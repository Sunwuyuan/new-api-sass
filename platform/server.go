package platform

import (
	"context"
	"errors"
	"html"
	"net/http"
	"net/mail"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/plan"
	"github.com/QuantumNous/new-api/tenant"
	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/gin-gonic/gin"
	"github.com/go-webauthn/webauthn/webauthn"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Server struct {
	DB               *gorm.DB
	Origin           string
	Secure           bool
	DummyHash        string
	Auth             AuthConfig
	HTTPClient       *http.Client
	Passkey          *webauthn.WebAuthn
	authMu           sync.Mutex
	Mail             MailSettings
	providerMu       sync.Mutex
	providers        map[string]*oidc.Provider
	InitializeTenant func(context.Context) error
}

type RootActivation struct {
	ID        int64     `gorm:"primaryKey"`
	TenantID  int64     `gorm:"not null;index"`
	UserID    int       `gorm:"not null"`
	TokenHash string    `gorm:"size:64;uniqueIndex;not null"`
	ExpiresAt time.Time `gorm:"not null;index"`
	UsedAt    *time.Time
}

func (RootActivation) TableName() string { return "tenant_root_activations" }

func New(db *gorm.DB) (*Server, error) {
	origin, secure, err := configuredOrigin()
	if err != nil {
		return nil, err
	}
	config, err := loadAuthConfig()
	if err != nil {
		return nil, err
	}
	if err := migrateExternalAccountEmails(db); err != nil {
		return nil, err
	}
	if err := db.AutoMigrate(&User{}, &Session{}, &AuthAttempt{}, &AuthFlow{}, &OAuthIdentity{}, &PasskeyCredential{}, &RootActivation{}, &AdminGuard{}, &Audit{}, &Redemption{}, &RedemptionUse{}, &Setting{}, &EmailChallenge{}); err != nil {
		return nil, err
	}
	if err := db.Model(&User{}).Where("email_verified_at IS NULL").Update("email_verified_at", gorm.Expr("created_at")).Error; err != nil {
		return nil, err
	}
	if err := db.Clauses(clause.OnConflict{DoNothing: true}).Create(&AdminGuard{ID: 1}).Error; err != nil {
		return nil, err
	}
	if err := plan.Migrate(db); err != nil {
		return nil, err
	}
	random, err := randomSecret()
	if err != nil {
		return nil, err
	}
	hash, err := common.HashAccountPassword(random)
	if err != nil {
		return nil, err
	}
	s := &Server{DB: db, Origin: origin, Secure: secure, DummyHash: hash, Auth: config,
		Mail:       bootstrapMailFromEnv(defaultMailSettings()),
		HTTPClient: &http.Client{Timeout: 15 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }},
		providers:  make(map[string]*oidc.Provider),
	}
	if err := s.loadStoredSettings(); err != nil {
		return nil, err
	}
	if err := s.configurePasskey(); err != nil {
		return nil, err
	}
	if err := s.bootstrapAdmin(); err != nil {
		return nil, err
	}
	common.SendPlatformMail = s.sendWorkspaceMail
	return s, nil
}

// Older platform accounts required an email. External identities have no
// verified platform email, so retain email uniqueness while allowing NULL.
// GORM AutoMigrate does not relax the old NOT NULL constraint by itself.
func migrateExternalAccountEmails(db *gorm.DB) error {
	if !db.Migrator().HasTable(&User{}) {
		return nil
	}
	columns, err := db.Migrator().ColumnTypes(&User{})
	if err != nil {
		return err
	}
	for _, column := range columns {
		if column.Name() != "email" {
			continue
		}
		nullable, known := column.Nullable()
		if !known {
			return errors.New("cannot determine platform email nullability")
		}
		if nullable {
			return nil
		}
		if !common.UsingMainDatabase(common.DatabaseTypeSQLite) {
			return db.Migrator().AlterColumn(&User{}, "Email")
		}
		// The SQLite driver rebuilds the table with supported SQLite syntax.
		// Its rebuild omits secondary indexes and triggers; restore their exact
		// definitions in the same transaction, including administrator additions.
		return db.Transaction(func(tx *gorm.DB) error {
			var objects []struct{ SQL string }
			if err := tx.Raw("SELECT sql FROM sqlite_master WHERE tbl_name = ? AND type IN ? AND sql IS NOT NULL ORDER BY name", "platform_users", []string{"index", "trigger"}).Scan(&objects).Error; err != nil {
				return err
			}
			if err := tx.Migrator().AlterColumn(&User{}, "Email"); err != nil {
				return err
			}
			for _, object := range objects {
				if err := tx.Exec(object.SQL).Error; err != nil {
					return err
				}
			}
			return nil
		})
	}
	return errors.New("platform account email column is missing")
}

func (s *Server) Routes(router *gin.Engine) {
	api := router.Group("/platform/api", s.BrowserSecurity)
	api.GET("/status", s.status)
	api.POST("/register", s.register)
	api.POST("/login", s.login)
	api.POST("/verify-email", s.verifyEmail)
	api.POST("/resend-verification", s.resendVerification)
	api.POST("/oauth/:provider/start", s.beginOAuth)
	api.POST("/oauth/:provider/finish", s.finishOAuth)
	api.POST("/wechat/start", s.beginWeChat)
	api.POST("/wechat/finish", s.finishWeChat)
	api.POST("/passkey/login/begin", s.beginPasskeyLogin)
	api.POST("/passkey/login/finish", s.finishPasskeyLogin)
	api.GET("/plans", s.plans)
	auth := api.Group("", s.authenticate)
	auth.GET("/session", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"success": true, "user": c.MustGet("platform_user"), "csrf_token": c.MustGet("platform_csrf"), "authenticated_at": c.MustGet("platform_session").(Session).CreatedAt})
	})
	auth.POST("/logout", s.logout)
	auth.POST("/password", s.changePassword)
	auth.POST("/email/change/start", s.startEmailChange)
	auth.POST("/email/change/finish", s.finishEmailChange)
	auth.POST("/reauthenticate", s.reauthenticate)
	auth.GET("/auth-methods", s.authMethods)
	auth.POST("/oauth/:provider/link", s.beginOAuth)
	auth.POST("/oauth/:provider/verify", s.beginOAuth)
	auth.POST("/wechat/link", s.beginWeChat)
	auth.POST("/wechat/verify", s.beginWeChat)
	auth.POST("/passkey/verify/begin", s.beginPasskeyLogin)
	auth.POST("/passkey/verify/finish", s.finishPasskeyLogin)
	auth.POST("/passkey/register/begin", s.beginPasskeyRegistration)
	auth.POST("/passkey/register/finish", s.finishPasskeyRegistration)
	auth.POST("/passkey/:id/delete", s.deletePasskey)
	auth.GET("/tenants", s.tenants)
	auth.GET("/tenants/:id", s.workspaceDetail)
	auth.GET("/usage", s.usageSummary)
	auth.POST("/tenants", s.createTenant)
	auth.POST("/tenants/:id", s.updateWorkspace)
	auth.POST("/tenants/:id/administrator", s.updateWorkspaceAdministrator)
	auth.POST("/redeem", s.redeem)
	admin := auth.Group("/admin", requireAdmin)
	admin.GET("/users", s.users)
	admin.POST("/users/:id", s.updateUser)
	admin.GET("/settings", s.adminSettings)
	admin.POST("/settings", s.updateAdminSettings)
	admin.GET("/tenants", s.tenants)
	admin.GET("/tenants/:id", s.workspaceDetail)
	admin.GET("/usage", s.usageSummary)
	admin.POST("/tenants/:id/plan", s.assignPlan)
	admin.POST("/tenants/:id/status", s.setTenantStatus)
	admin.POST("/tenants/:id/owner", s.transferOwner)
	admin.POST("/tenants/:id", s.updateWorkspace)
	admin.POST("/tenants/:id/administrator", s.updateWorkspaceAdministrator)
	admin.GET("/plans", s.plans)
	admin.POST("/plans/:id", s.updatePlan)
	admin.GET("/redemptions", s.redemptions)
	admin.POST("/redemptions", s.createRedemptions)
	admin.POST("/redemptions/:id/disable", s.disableRedemption)
	admin.GET("/redemptions/:id/uses", s.redemptionUses)
	admin.GET("/audits", s.audits)
}

func (s *Server) plans(c *gin.Context) {
	var entries []plan.Plan
	if s.DB.WithContext(c.Request.Context()).Order("id").Find(&entries).Error != nil {
		writeError(c, http.StatusServiceUnavailable, "platform_unavailable")
		return
	}
	views := make([]plan.View, 0, len(entries))
	for _, p := range entries {
		view, err := p.View()
		if err != nil {
			writeError(c, http.StatusServiceUnavailable, "invalid_plan")
			return
		}
		views = append(views, view)
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "plans": views})
}

func (s *Server) tenants(c *gin.Context) {
	user := c.MustGet("platform_user").(User)
	page, ok := pageQuery(c)
	if !ok {
		return
	}
	query := s.DB.WithContext(c.Request.Context()).Model(&tenant.Workspace{})
	if !strings.Contains(c.FullPath(), "/admin/") {
		query = query.Where("owner_platform_user_id = ?", user.ID)
	}
	if search := strings.TrimSpace(c.Query("search")); search != "" {
		query = query.Where("name LIKE ? OR slug LIKE ?", "%"+search+"%", "%"+search+"%")
	}
	now := time.Now().UTC()
	switch c.Query("status") {
	case "":
	case "active":
		query = query.Where("status = ? AND (plan_expires_at IS NULL OR plan_expires_at > ?)", "active", now)
	case "suspended":
		query = query.Where("status = ?", "suspended")
	case "expired":
		query = query.Where("status = ? AND plan_expires_at <= ?", "active", now)
	default:
		writeError(c, http.StatusBadRequest, "invalid_workspace_status")
		return
	}
	var tenants []tenant.Workspace
	if query.Count(&page.Total).Error != nil || query.Order("id desc").Offset((page.Page-1)*page.PageSize).Limit(page.PageSize).Find(&tenants).Error != nil {
		writeError(c, http.StatusServiceUnavailable, "platform_unavailable")
		return
	}
	month := now.Format("2006-01")
	tenantIDs := make([]int64, 0, len(tenants))
	ownerIDs := make([]int64, 0, len(tenants))
	for _, workspace := range tenants {
		tenantIDs = append(tenantIDs, workspace.ID)
		ownerIDs = append(ownerIDs, workspace.OwnerPlatformUserID)
	}
	var usages []plan.Usage
	var owners []User
	db := s.DB.WithContext(c.Request.Context())
	if len(tenants) > 0 && (db.Where("tenant_id IN ? AND month = ?", tenantIDs, month).Find(&usages).Error != nil || db.Where("id IN ?", ownerIDs).Find(&owners).Error != nil) {
		writeError(c, http.StatusServiceUnavailable, "platform_unavailable")
		return
	}
	usageByID := make(map[int64]plan.Usage, len(usages))
	ownerByID := make(map[int64]string, len(owners))
	for _, usage := range usages {
		usageByID[usage.TenantID] = usage
	}
	for _, owner := range owners {
		ownerByID[owner.ID] = owner.AccountName()
	}
	views := make([]gin.H, 0, len(tenants))
	for _, workspace := range tenants {
		usage, ok := usageByID[workspace.ID]
		if !ok {
			usage = plan.Usage{TenantID: workspace.ID, Month: month}
		}
		views = append(views, gin.H{"tenant": workspace, "usage": usage, "owner_email": ownerByID[workspace.OwnerPlatformUserID]})
	}
	response := gin.H{"success": true, "tenants": views, "pagination": page}
	if !strings.Contains(c.FullPath(), "/admin/") {
		available, err := liteAvailable(db, user.ID)
		if err != nil {
			transactionError(c, err)
			return
		}
		response["lite_available"] = available
		response["workspace_count"] = user.TenantCount
	}
	c.JSON(http.StatusOK, response)
}

var slugPattern = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]{0,46}[a-z0-9])?$`)

func (s *Server) createTenant(c *gin.Context) {
	var input struct {
		Slug        string `json:"slug"`
		Name        string `json:"name"`
		Username    string `json:"username"`
		DisplayName string `json:"display_name"`
		Email       string `json:"email"`
		Password    string `json:"password"`
		PlanID      int64  `json:"plan_id"`
		Code        string `json:"code"`
	}
	if c.ShouldBindJSON(&input) != nil || !slugPattern.MatchString(input.Slug) || strings.TrimSpace(input.Name) == "" || len([]rune(input.Name)) > 128 {
		writeError(c, http.StatusBadRequest, "invalid_workspace")
		return
	}
	username := strings.TrimSpace(input.Username)
	display := strings.TrimSpace(input.DisplayName)
	email := strings.TrimSpace(input.Email)
	if username == "" || utf8.RuneCountInString(username) > model.UserNameMaxLength {
		writeError(c, http.StatusBadRequest, "invalid_workspace_administrator")
		return
	}
	if display == "" {
		display = username
	}
	if utf8.RuneCountInString(display) > 20 {
		writeError(c, http.StatusBadRequest, "invalid_workspace_administrator")
		return
	}
	if email != "" {
		address, err := mail.ParseAddress(email)
		if err != nil || address.Address != email || utf8.RuneCountInString(email) > 50 {
			writeError(c, http.StatusBadRequest, "invalid_workspace_administrator")
			return
		}
	}
	if utf8.RuneCountInString(input.Password) < 10 || utf8.RuneCountInString(input.Password) > 128 || common.ValidateNewAccountPassword(input.Password) != nil {
		writeError(c, http.StatusBadRequest, "invalid_new_password")
		return
	}
	user := c.MustGet("platform_user").(User)
	workspace := tenant.Workspace{Slug: input.Slug, Name: strings.TrimSpace(input.Name), OwnerPlatformUserID: user.ID, Status: "active"}
	hash, err := common.HashAccountPassword(input.Password)
	if err != nil {
		writeError(c, http.StatusServiceUnavailable, "platform_unavailable")
		return
	}
	err = s.DB.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		if _, err := activeUser(tx, user); err != nil {
			return err
		}
		selected, err := selectedCreatePlan(tx, input.PlanID)
		if err != nil {
			return err
		}
		workspace.PlanID = selected.ID
		if selected.Name == "Lite" {
			ok, err := liteAvailable(tx, user.ID)
			if err != nil {
				return err
			}
			if !ok {
				return errLiteTaken
			}
		} else {
			if len(strings.TrimSpace(input.Code)) != 64 {
				return errRedemption
			}
		}
		if err := tx.Model(&User{}).Where("id = ?", user.ID).UpdateColumn("tenant_count", gorm.Expr("tenant_count + 1")).Error; err != nil {
			return err
		}
		if err := tx.Create(&workspace).Error; err != nil {
			return err
		}
		if selected.Name != "Lite" {
			assignment, err := consumeRedemption(tx, input.Code, workspace.ID, selected.ID, user)
			if err != nil {
				return err
			}
			expires := assignment.ExpiresAt
			workspace.PlanID = assignment.PlanID
			workspace.PlanExpiresAt = &expires
		}
		ctx := tenant.WithContext(c.Request.Context(), tenant.Identity{ID: workspace.ID, Slug: workspace.Slug})
		root := model.User{
			Username:    username,
			Password:    hash,
			DisplayName: display,
			Email:       email,
			Role:        common.RoleRootUser,
			Status:      common.UserStatusEnabled,
			AuthVersion: 1,
			Quota:       0,
		}
		if err := tx.WithContext(ctx).Create(&root).Error; err != nil {
			return err
		}
		for key, value := range map[string]string{"SystemName": workspace.Name, "ServerAddress": s.Origin + "/t/" + workspace.Slug} {
			if err := tx.WithContext(ctx).Create(&model.Option{Key: key, Value: value}).Error; err != nil {
				return err
			}
		}
		return audit(tx, user.ID, "workspace.create", workspace.ID, gin.H{"slug": workspace.Slug, "plan_id": workspace.PlanID})
	})
	if err != nil {
		if errors.Is(err, errRedemption) {
			writeError(c, http.StatusBadRequest, "redemption_unavailable")
			return
		}
		var failure *tenant.HTTPError
		if errors.As(err, &failure) {
			transactionError(c, err)
			return
		}
		writeError(c, http.StatusConflict, "workspace_unavailable_or_limit_reached")
		return
	}
	if user.Email != nil {
		s.notify(c.Request.Context(), *user.Email, "Workspace ready",
			"<p>Your workspace <strong>"+html.EscapeString(workspace.Name)+"</strong> is ready.</p><p>Open <a href=\""+html.EscapeString(s.Origin)+"/t/"+html.EscapeString(workspace.Slug)+"/setup\">setup</a> and sign in with the administrator account you created.</p>")
	}
	c.JSON(http.StatusCreated, gin.H{"success": true, "tenant": workspace, "setup_url": "/t/" + workspace.Slug + "/setup"})
}

func (s *Server) ActivateRoot(c *gin.Context) {
	var input struct {
		Token    string `json:"token"`
		Password string `json:"password"`
	}
	identity, err := tenant.FromContext(c.Request.Context())
	if err != nil || c.ShouldBindJSON(&input) != nil || len(input.Token) != 64 || len([]rune(input.Password)) < 10 || common.ValidateNewAccountPassword(input.Password) != nil {
		writeError(c, http.StatusBadRequest, "invalid_activation")
		return
	}
	if !s.authRateLimit(c, "activation:"+strconv.FormatInt(identity.ID, 10)) {
		return
	}
	hash, err := common.HashAccountPassword(input.Password)
	if err != nil {
		writeError(c, http.StatusBadRequest, "invalid_activation")
		return
	}
	now := time.Now().UTC()
	err = s.DB.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		var activation RootActivation
		if err := tx.Where("tenant_id = ? AND token_hash = ? AND used_at IS NULL AND expires_at > ?", identity.ID, digest(input.Token), now).First(&activation).Error; err != nil {
			return err
		}
		result := tx.Model(&RootActivation{}).Where("id = ? AND used_at IS NULL AND expires_at > ?", activation.ID, now).Update("used_at", now)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return errors.New("activation already used")
		}
		if err := tx.Model(&model.User{}).Where("id = ?", activation.UserID).Update("password", hash).Error; err != nil {
			return err
		}
		_, err := model.IncrementUserAuthVersionWithTx(tx, activation.UserID)
		return err
	})
	if err != nil {
		writeError(c, http.StatusBadRequest, "invalid_activation")
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (s *Server) assignPlan(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	var input struct {
		PlanID int64 `json:"plan_id"`
		Months int   `json:"months"`
	}
	if err != nil || id <= 0 || c.ShouldBindJSON(&input) != nil || input.Months < 1 || input.Months > 36 {
		writeError(c, http.StatusBadRequest, "invalid_plan_assignment")
		return
	}
	user := c.MustGet("platform_user").(User)
	var assignment plan.Assignment
	err = s.adminTransaction(c, func(tx *gorm.DB) error {
		var err error
		assignment, err = assignWorkspacePlan(tx, id, input.PlanID, input.Months, user, "manual", nil)
		return err
	})
	if err != nil {
		transactionError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "assignment": assignment})
}

func (s *Server) setTenantStatus(c *gin.Context) {
	var input struct {
		Status string `json:"status"`
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 || c.ShouldBindJSON(&input) != nil || input.Status != "active" && input.Status != "suspended" {
		writeError(c, http.StatusBadRequest, "invalid_workspace_status")
		return
	}
	err = s.adminTransaction(c, func(tx *gorm.DB) error {
		var workspace tenant.Workspace
		if err := tx.First(&workspace, id).Error; err != nil {
			return err
		}
		if err := tx.Model(&workspace).Update("status", input.Status).Error; err != nil {
			return err
		}
		return audit(tx, c.MustGet("platform_user").(User).ID, "workspace."+input.Status, id, nil)
	})
	if err != nil {
		transactionError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// One free Lite workspace per owner. Paid workspaces are created with a
// redemption code and do not consume this entitlement.
func liteAvailable(db *gorm.DB, userID int64) (bool, error) {
	var lite plan.Plan
	if err := db.Where("name = ?", "Lite").First(&lite).Error; err != nil {
		return false, err
	}
	var count int64
	if err := db.Model(&tenant.Workspace{}).Where("owner_platform_user_id = ? AND plan_id = ? AND plan_expires_at IS NULL", userID, lite.ID).Count(&count).Error; err != nil {
		return false, err
	}
	return count == 0, nil
}

func selectedCreatePlan(db *gorm.DB, planID int64) (plan.Plan, error) {
	var selected plan.Plan
	query := db
	if planID > 0 {
		query = query.Where("id = ?", planID)
	} else {
		query = query.Where("name = ?", "Lite")
	}
	if err := query.First(&selected).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return selected, &tenant.HTTPError{Status: http.StatusBadRequest, Code: "invalid_plan"}
		}
		return selected, err
	}
	if _, err := selected.View(); err != nil {
		return selected, err
	}
	return selected, nil
}

var errLiteTaken = &tenant.HTTPError{Status: http.StatusConflict, Code: "lite_workspace_already_exists"}
