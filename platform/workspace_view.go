package platform

import (
	"context"
	"net/http"
	"net/mail"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/plan"
	"github.com/QuantumNous/new-api/tenant"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// Platform ownership is checked before any workspace details or usage are
// returned. This query only accesses global SaaS tables, never relay records.
func (s *Server) ownedWorkspaces(c *gin.Context) *gorm.DB {
	query := s.DB.WithContext(c.Request.Context()).Model(&tenant.Workspace{})
	if !strings.Contains(c.FullPath(), "/admin/") {
		query = query.Where("owner_platform_user_id = ?", c.MustGet("platform_user").(User).ID)
	}
	return query
}

func (s *Server) workspaceDetail(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		writeError(c, http.StatusNotFound, "platform_record_not_found")
		return
	}
	var workspace tenant.Workspace
	if err := s.ownedWorkspaces(c).First(&workspace, id).Error; err != nil {
		transactionError(c, err)
		return
	}
	db := s.DB.WithContext(c.Request.Context())
	var owner User
	var hosting plan.Plan
	if db.First(&hosting, workspace.PlanID).Error != nil || db.First(&owner, workspace.OwnerPlatformUserID).Error != nil {
		writeError(c, http.StatusServiceUnavailable, "platform_unavailable")
		return
	}
	now := time.Now().UTC()
	if err := plan.DowngradeExpired(db, &workspace, now); err != nil {
		writeError(c, http.StatusServiceUnavailable, "platform_unavailable")
		return
	}
	view, err := plan.ForWorkspace(db, workspace, now)
	if err != nil {
		view, err = hosting.View()
	}
	if err != nil {
		writeError(c, http.StatusServiceUnavailable, "invalid_plan")
		return
	}
	month := now.Format("2006-01")
	history := make([]plan.Usage, 0)
	// Show real recorded months; the UI can distinguish no recorded requests.
	firstMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC).AddDate(0, -5, 0).Format("2006-01")
	if db.Where("tenant_id = ? AND month >= ? AND month <= ?", id, firstMonth, month).Order("month").Find(&history).Error != nil {
		writeError(c, http.StatusServiceUnavailable, "platform_unavailable")
		return
	}
	usage := plan.Usage{TenantID: id, Month: month}
	for _, item := range history {
		if item.Month == month {
			usage = item
		}
	}
	var assignments []plan.Assignment
	if db.Where("tenant_id = ?", id).Order("id desc").Limit(20).Find(&assignments).Error != nil {
		writeError(c, http.StatusServiceUnavailable, "platform_unavailable")
		return
	}
	var hosts []tenant.Host
	if db.Where("tenant_id = ?", id).Order("id").Find(&hosts).Error != nil {
		writeError(c, http.StatusServiceUnavailable, "platform_unavailable")
		return
	}
	hostViews := make([]gin.H, 0, len(hosts))
	for _, binding := range hosts {
		hostViews = append(hostViews, s.hostView(binding))
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "tenant": workspace, "owner_name": owner.AccountName(), "plan": view, "usage": usage, "history": history, "assignments": assignments, "administrators": workspaceAdministrators(db, c.Request.Context(), workspace), "setup_complete": model.GetSetup(tenant.WithContext(c.Request.Context(), tenant.Identity{ID: workspace.ID, Slug: workspace.Slug})) != nil, "hosts": hostViews, "primary_url": s.workspaceURL(hosts, "/")})
}

func workspaceAdministrators(db *gorm.DB, ctx context.Context, workspace tenant.Workspace) []gin.H {
	ctx = tenant.WithContext(ctx, tenant.Identity{ID: workspace.ID, Slug: workspace.Slug})
	var users []model.User
	if db.WithContext(ctx).Where("role >= ?", common.RoleAdminUser).Order("role desc, id").Find(&users).Error != nil {
		return []gin.H{}
	}
	views := make([]gin.H, 0, len(users))
	for _, user := range users {
		role := "admin"
		if user.Role >= common.RoleRootUser {
			role = "root"
		}
		views = append(views, gin.H{
			"id": user.Id, "username": user.Username, "display_name": user.DisplayName,
			"email": user.Email, "status": user.Status, "role": role,
		})
	}
	return views
}

func (s *Server) updateWorkspace(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	var input struct {
		Name string `json:"name"`
		Slug string `json:"slug"`
	}
	if err != nil || id <= 0 || c.ShouldBindJSON(&input) != nil {
		writeError(c, http.StatusBadRequest, "invalid_workspace")
		return
	}
	name := strings.TrimSpace(input.Name)
	slug := strings.TrimSpace(input.Slug)
	if name == "" || utf8.RuneCountInString(name) > 128 {
		writeError(c, http.StatusBadRequest, "invalid_workspace")
		return
	}
	if slug != "" && !tenant.ValidSlug(slug) {
		writeError(c, http.StatusBadRequest, "invalid_workspace")
		return
	}
	actor := c.MustGet("platform_user").(User)
	err = s.DB.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		query := tx.Where("id = ?", id)
		if !strings.Contains(c.FullPath(), "/admin/") {
			query = query.Where("owner_platform_user_id = ?", actor.ID)
		}
		var workspace tenant.Workspace
		if err := query.First(&workspace).Error; err != nil {
			return err
		}
		updates := map[string]any{"name": name}
		if slug != "" && slug != workspace.Slug {
			var taken int64
			if err := tx.Model(&tenant.Workspace{}).Where("slug = ? AND id <> ?", slug, workspace.ID).Count(&taken).Error; err != nil {
				return err
			}
			if taken > 0 {
				return &tenant.HTTPError{Status: http.StatusConflict, Code: "workspace_unavailable_or_limit_reached"}
			}
			updates["slug"] = slug
			workspace.Slug = slug
		}
		if err := tx.Model(&workspace).Updates(updates).Error; err != nil {
			return err
		}
		ctx := tenant.WithContext(c.Request.Context(), tenant.Identity{ID: workspace.ID, Slug: workspace.Slug})
		if err := tx.WithContext(ctx).Model(&model.Option{}).Where("key = ?", "SystemName").Update("value", name).Error; err != nil {
			return err
		}
		if err := s.refreshServerAddress(tx, workspace); err != nil {
			return err
		}
		return audit(tx, actor.ID, "workspace.update", id, gin.H{"name": name, "slug": workspace.Slug})
	})
	if err != nil {
		transactionError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (s *Server) transferOwner(c *gin.Context) {
	if !requireRecentSession(c) {
		return
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	var input struct {
		Email string `json:"email"`
	}
	if err != nil || id <= 0 || c.ShouldBindJSON(&input) != nil {
		writeError(c, http.StatusBadRequest, "invalid_workspace")
		return
	}
	address, err := mail.ParseAddress(strings.TrimSpace(input.Email))
	if err != nil {
		writeError(c, http.StatusBadRequest, "invalid_workspace")
		return
	}
	email := strings.ToLower(address.Address)
	err = s.adminTransaction(c, func(tx *gorm.DB) error {
		var workspace tenant.Workspace
		if err := tx.First(&workspace, id).Error; err != nil {
			return err
		}
		var owner User
		if err := tx.Where("email = ? AND status = ?", email, "active").First(&owner).Error; err != nil {
			return err
		}
		previousOwnerID := workspace.OwnerPlatformUserID
		if owner.ID == previousOwnerID {
			return &tenant.HTTPError{Status: http.StatusBadRequest, Code: "invalid_workspace"}
		}
		if workspace.PlanExpiresAt == nil {
			ok, err := liteAvailable(tx, owner.ID)
			if err != nil {
				return err
			}
			var lite plan.Plan
			if err := tx.Where("name = ?", "Lite").First(&lite).Error; err != nil {
				return err
			}
			if workspace.PlanID == lite.ID && !ok {
				return errLiteTaken
			}
		}
		if err := tx.Model(&workspace).Update("owner_platform_user_id", owner.ID).Error; err != nil {
			return err
		}
		released := tx.Model(&User{}).Where("id = ? AND tenant_count > 0", previousOwnerID).
			UpdateColumn("tenant_count", gorm.Expr("tenant_count - 1"))
		if released.Error != nil {
			return released.Error
		}
		if err := tx.Model(&User{}).Where("id = ?", owner.ID).
			UpdateColumn("tenant_count", gorm.Expr("tenant_count + 1")).Error; err != nil {
			return err
		}
		return audit(tx, c.MustGet("platform_user").(User).ID, "workspace.transfer", id, gin.H{"owner_id": owner.ID})
	})
	if err != nil {
		transactionError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (s *Server) updateWorkspaceAdministrator(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	var input struct {
		ID          int    `json:"id"`
		Username    string `json:"username"`
		DisplayName string `json:"display_name"`
		Email       string `json:"email"`
		Password    string `json:"password"`
	}
	if err != nil || id <= 0 || c.ShouldBindJSON(&input) != nil {
		writeError(c, http.StatusBadRequest, "invalid_workspace_administrator")
		return
	}
	var workspace tenant.Workspace
	if s.ownedWorkspaces(c).First(&workspace, id).Error != nil {
		transactionError(c, gorm.ErrRecordNotFound)
		return
	}
	ctx := tenant.WithContext(c.Request.Context(), tenant.Identity{ID: workspace.ID, Slug: workspace.Slug})
	updates := map[string]any{}
	if username := strings.TrimSpace(input.Username); username != "" {
		if utf8.RuneCountInString(username) > model.UserNameMaxLength {
			writeError(c, http.StatusBadRequest, "invalid_workspace_administrator")
			return
		}
		updates["username"] = username
	}
	if display := strings.TrimSpace(input.DisplayName); display != "" {
		if utf8.RuneCountInString(display) > 20 {
			writeError(c, http.StatusBadRequest, "invalid_workspace_administrator")
			return
		}
		updates["display_name"] = display
	}
	if email := strings.TrimSpace(input.Email); email != "" {
		if utf8.RuneCountInString(email) > 50 {
			writeError(c, http.StatusBadRequest, "invalid_workspace_administrator")
			return
		}
		updates["email"] = email
	}
	if input.Password != "" {
		if common.ValidateNewAccountPassword(input.Password) != nil {
			writeError(c, http.StatusBadRequest, "invalid_new_password")
			return
		}
		hash, err := common.HashAccountPassword(input.Password)
		if err != nil {
			writeError(c, http.StatusServiceUnavailable, "platform_unavailable")
			return
		}
		updates["password"] = hash
	}
	if len(updates) == 0 {
		writeError(c, http.StatusBadRequest, "invalid_workspace_administrator")
		return
	}
	err = s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		query := tx.Where("role >= ?", common.RoleAdminUser)
		if input.ID > 0 {
			query = query.Where("id = ?", input.ID)
		} else {
			query = query.Where("role = ?", common.RoleRootUser)
		}
		var admin model.User
		if err := query.First(&admin).Error; err != nil {
			return err
		}
		if err := tx.Model(&model.User{}).Where("id = ?", admin.Id).Updates(updates).Error; err != nil {
			return err
		}
		if _, ok := updates["password"]; ok {
			if _, err := model.IncrementUserAuthVersionWithTx(tx, admin.Id); err != nil {
				return err
			}
		}
		fields := make([]string, 0, len(updates))
		for key := range updates {
			fields = append(fields, key)
		}
		return audit(tx, c.MustGet("platform_user").(User).ID, "workspace.administrator", id, gin.H{"fields": fields, "user_id": admin.Id})
	})
	if err != nil {
		transactionError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (s *Server) usageSummary(c *gin.Context) {
	now := time.Now().UTC()
	month := now.Format("2006-01")
	db := s.DB.WithContext(c.Request.Context())
	var summary struct {
		Workspaces   int64 `json:"workspaces"`
		Active       int64 `json:"active"`
		Suspended    int64 `json:"suspended"`
		Expired      int64 `json:"expired"`
		ExpiringSoon int64 `json:"expiring_soon"`
		Requests     int64 `json:"requests"`
		Exhausted    int64 `json:"exhausted"`
	}
	// Standard CASE/COALESCE works on SQLite, MySQL 5.7 and PostgreSQL 9.6;
	// keep JSON plan limits in Go and avoid database-specific JSON operators.
	err := s.ownedWorkspaces(c).Select(`COUNT(*) AS workspaces,
COALESCE(SUM(CASE WHEN status = ? AND (plan_expires_at IS NULL OR plan_expires_at > ?) THEN 1 ELSE 0 END), 0) AS active,
COALESCE(SUM(CASE WHEN status = ? THEN 1 ELSE 0 END), 0) AS suspended,
COALESCE(SUM(CASE WHEN status = ? AND plan_expires_at <= ? THEN 1 ELSE 0 END), 0) AS expired,
COALESCE(SUM(CASE WHEN status = ? AND plan_expires_at > ? AND plan_expires_at <= ? THEN 1 ELSE 0 END), 0) AS expiring_soon`, "active", now, "suspended", "active", now, "active", now, now.AddDate(0, 0, 7)).Scan(&summary).Error
	if err != nil {
		transactionError(c, err)
		return
	}
	ids := s.ownedWorkspaces(c).Select("id")
	if db.Model(&plan.Usage{}).Select("COALESCE(SUM(requests), 0)").Where("month = ? AND tenant_id IN (?)", month, ids).Scan(&summary.Requests).Error != nil {
		writeError(c, http.StatusServiceUnavailable, "platform_unavailable")
		return
	}
	var plans []plan.Plan
	if db.Order("id").Find(&plans).Error != nil {
		writeError(c, http.StatusServiceUnavailable, "platform_unavailable")
		return
	}
	distribution := make([]gin.H, 0, len(plans))
	for _, hosting := range plans {
		view, err := hosting.View()
		if err != nil {
			writeError(c, http.StatusServiceUnavailable, "invalid_plan")
			return
		}
		var count, requests, exhausted int64
		if err := s.ownedWorkspaces(c).Where("plan_id = ?", hosting.ID).Count(&count).Error; err != nil {
			transactionError(c, err)
			return
		}
		planIDs := s.ownedWorkspaces(c).Where("plan_id = ?", hosting.ID).Select("id")
		if db.Model(&plan.Usage{}).Select("COALESCE(SUM(requests), 0)").Where("month = ? AND tenant_id IN (?)", month, planIDs).Scan(&requests).Error != nil {
			writeError(c, http.StatusServiceUnavailable, "platform_unavailable")
			return
		}
		if view.Limits.Requests > 0 {
			exhaustedIDs := db.Model(&plan.Usage{}).Select("tenant_id").Where("month = ? AND requests >= ?", month, view.Limits.Requests)
			if err := s.ownedWorkspaces(c).Where("plan_id = ? AND id IN (?)", hosting.ID, exhaustedIDs).Count(&exhausted).Error; err != nil {
				transactionError(c, err)
				return
			}
			summary.Exhausted += exhausted
		}
		distribution = append(distribution, gin.H{"plan_id": hosting.ID, "name": hosting.Name, "workspaces": count, "requests": requests})
	}
	firstMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC).AddDate(0, -5, 0).Format("2006-01")
	history := make([]struct {
		Month    string `json:"month"`
		Requests int64  `json:"requests"`
	}, 0)
	if db.Model(&plan.Usage{}).Select("month, SUM(requests) AS requests").Where("month >= ? AND month <= ? AND tenant_id IN (?)", firstMonth, month, ids).Group("month").Order("month").Scan(&history).Error != nil {
		writeError(c, http.StatusServiceUnavailable, "platform_unavailable")
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "month": month, "summary": summary, "plans": distribution, "history": history})
}
