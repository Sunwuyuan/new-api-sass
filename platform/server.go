package platform

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/plan"
	"github.com/QuantumNous/new-api/tenant"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type Server struct {
	DB               *gorm.DB
	Origin           string
	Secure           bool
	DummyHash        string
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
	if err := db.AutoMigrate(&User{}, &Session{}, &AuthAttempt{}, &RootActivation{}); err != nil {
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
	s := &Server{DB: db, Origin: origin, Secure: secure, DummyHash: hash}
	if err := s.bootstrapAdmin(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Server) Routes(router *gin.Engine) {
	api := router.Group("/platform/api", s.BrowserSecurity)
	api.POST("/register", s.register)
	api.POST("/login", s.login)
	api.GET("/plans", s.plans)
	auth := api.Group("", s.authenticate)
	auth.GET("/session", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"success": true, "user": c.MustGet("platform_user"), "csrf_token": c.MustGet("platform_csrf")})
	})
	auth.POST("/logout", s.logout)
	auth.POST("/password", s.changePassword)
	auth.GET("/tenants", s.tenants)
	auth.POST("/tenants", s.createTenant)
	admin := auth.Group("/admin", requireAdmin)
	admin.GET("/users", s.users)
	admin.GET("/tenants", s.tenants)
	admin.POST("/tenants/:id/plan", s.assignPlan)
	admin.POST("/tenants/:id/status", s.setTenantStatus)
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

func (s *Server) users(c *gin.Context) {
	var users []User
	if s.DB.WithContext(c.Request.Context()).Order("id desc").Limit(200).Find(&users).Error != nil {
		writeError(c, http.StatusServiceUnavailable, "platform_unavailable")
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "users": users})
}

func (s *Server) tenants(c *gin.Context) {
	user := c.MustGet("platform_user").(User)
	query := s.DB.WithContext(c.Request.Context())
	if !strings.Contains(c.FullPath(), "/admin/") {
		query = query.Where("owner_platform_user_id = ?", user.ID)
	}
	var tenants []tenant.Workspace
	if query.Order("id desc").Limit(200).Find(&tenants).Error != nil {
		writeError(c, http.StatusServiceUnavailable, "platform_unavailable")
		return
	}
	month := time.Now().UTC().Format("2006-01")
	views := make([]gin.H, 0, len(tenants))
	for _, workspace := range tenants {
		usage := plan.Usage{TenantID: workspace.ID, Month: month}
		if err := s.DB.WithContext(c.Request.Context()).Where("tenant_id = ? AND month = ?", workspace.ID, month).First(&usage).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			writeError(c, http.StatusServiceUnavailable, "platform_unavailable")
			return
		}
		views = append(views, gin.H{"tenant": workspace, "usage": usage})
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "tenants": views})
}

var slugPattern = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]{0,46}[a-z0-9])?$`)

func (s *Server) createTenant(c *gin.Context) {
	var input struct {
		Slug string `json:"slug"`
		Name string `json:"name"`
	}
	if c.ShouldBindJSON(&input) != nil || !slugPattern.MatchString(input.Slug) || strings.TrimSpace(input.Name) == "" || len([]rune(input.Name)) > 128 {
		writeError(c, http.StatusBadRequest, "invalid_workspace")
		return
	}
	user := c.MustGet("platform_user").(User)
	workspace := tenant.Workspace{Slug: input.Slug, Name: strings.TrimSpace(input.Name), OwnerPlatformUserID: user.ID, PlanID: 1, Status: "active"}
	activationSecret, err := randomSecret()
	if err != nil {
		writeError(c, http.StatusServiceUnavailable, "platform_unavailable")
		return
	}
	randomPassword, err := randomSecret()
	if err != nil {
		writeError(c, http.StatusServiceUnavailable, "platform_unavailable")
		return
	}
	hash, err := common.HashAccountPassword(randomPassword)
	if err != nil {
		writeError(c, http.StatusServiceUnavailable, "platform_unavailable")
		return
	}
	err = s.DB.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&User{}).Where("id = ? AND tenant_count < ?", user.ID, 10).UpdateColumn("tenant_count", gorm.Expr("tenant_count + 1"))
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return errors.New("workspace creation limit reached")
		}
		if err := tx.Create(&workspace).Error; err != nil {
			return err
		}
		ctx := tenant.WithContext(c.Request.Context(), tenant.Identity{ID: workspace.ID, Slug: workspace.Slug})
		root := model.User{Username: "root", Password: hash, DisplayName: "Root User", Role: common.RoleRootUser, Status: common.UserStatusEnabled, AuthVersion: 1, Quota: 0}
		if err := tx.WithContext(ctx).Create(&root).Error; err != nil {
			return err
		}
		if err := tx.WithContext(ctx).Create(&model.Setup{Version: common.Version, InitializedAt: time.Now().Unix()}).Error; err != nil {
			return err
		}
		for key, value := range map[string]string{"SystemName": workspace.Name, "ServerAddress": s.Origin + "/t/" + workspace.Slug} {
			if err := tx.WithContext(ctx).Create(&model.Option{Key: key, Value: value}).Error; err != nil {
				return err
			}
		}
		return tx.Create(&RootActivation{TenantID: workspace.ID, UserID: root.Id, TokenHash: digest(activationSecret), ExpiresAt: time.Now().UTC().Add(30 * time.Minute)}).Error
	})
	if err != nil {
		writeError(c, http.StatusConflict, "workspace_unavailable_or_limit_reached")
		return
	}
	c.JSON(http.StatusCreated, gin.H{"success": true, "tenant": workspace, "root_username": "root", "root_activation_url": "/t/" + workspace.Slug + "/activate#token=" + activationSecret})
}

func (s *Server) ActivateRoot(c *gin.Context) {
	var input struct {
		Token    string `json:"token"`
		Password string `json:"password"`
	}
	identity, err := tenant.FromContext(c.Request.Context())
	if err != nil || c.ShouldBindJSON(&input) != nil || len(input.Token) != 64 || len([]rune(input.Password)) < 15 || common.ValidateNewAccountPassword(input.Password) != nil {
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
		return tx.Model(&model.User{}).Where("id = ?", activation.UserID).Update("password", hash).Error
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
	expiresAt := time.Now().UTC().AddDate(0, input.Months, 0)
	err = s.DB.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		var p plan.Plan
		if err := tx.First(&p, input.PlanID).Error; err != nil {
			return err
		}
		if _, err := p.View(); err != nil {
			return err
		}
		result := tx.Model(&tenant.Workspace{}).Where("id = ?", id).Updates(map[string]any{"plan_id": p.ID, "plan_expires_at": expiresAt})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return gorm.ErrRecordNotFound
		}
		return tx.Create(&plan.Assignment{TenantID: id, PlanID: p.ID, AdministratorID: user.ID, ExpiresAt: expiresAt}).Error
	})
	if err != nil {
		writeError(c, http.StatusBadRequest, "invalid_plan_assignment")
		return
	}
	common.SysLog(fmt.Sprintf("platform plan assigned: administrator_id=%d tenant_id=%d plan_id=%d", user.ID, id, input.PlanID))
	c.JSON(http.StatusOK, gin.H{"success": true})
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
	result := s.DB.WithContext(c.Request.Context()).Model(&tenant.Workspace{}).Where("id = ?", id).Update("status", input.Status)
	if result.Error != nil || result.RowsAffected != 1 {
		writeError(c, http.StatusNotFound, "workspace_not_found")
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}
