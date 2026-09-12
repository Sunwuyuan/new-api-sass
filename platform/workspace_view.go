package platform

import (
	"net/http"
	"strconv"
	"strings"
	"time"

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
	view, err := hosting.View()
	if err != nil {
		writeError(c, http.StatusServiceUnavailable, "invalid_plan")
		return
	}
	now := time.Now().UTC()
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
	c.JSON(http.StatusOK, gin.H{"success": true, "tenant": workspace, "owner_name": owner.AccountName(), "plan": view, "usage": usage, "history": history, "assignments": assignments})
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
		exhaustedIDs := db.Model(&plan.Usage{}).Select("tenant_id").Where("month = ? AND requests >= ?", month, view.Limits.Requests)
		if err := s.ownedWorkspaces(c).Where("plan_id = ? AND id IN (?)", hosting.ID, exhaustedIDs).Count(&exhausted).Error; err != nil {
			transactionError(c, err)
			return
		}
		summary.Exhausted += exhausted
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
