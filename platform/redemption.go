package platform

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/plan"
	"github.com/QuantumNous/new-api/tenant"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// Codes are bearer entitlements: only a digest is persisted, and plaintext is
// returned once on creation. Workspace wallet redemptions are unrelated.
type Redemption struct {
	ID             int64      `json:"id" gorm:"primaryKey"`
	CodeHash       string     `json:"-" gorm:"size:64;uniqueIndex;not null"`
	CodeHint       string     `json:"code_hint" gorm:"size:8;not null;index"`
	PlanID         int64      `json:"plan_id" gorm:"not null;index"`
	DurationMonths int        `json:"duration_months" gorm:"not null"`
	Status         string     `json:"status" gorm:"size:16;not null;index"`
	MaxUses        int        `json:"max_uses" gorm:"not null"`
	Used           int        `json:"used" gorm:"not null"`
	ExpiresAt      *time.Time `json:"expires_at" gorm:"index"`
	CreatedBy      int64      `json:"created_by" gorm:"not null;index"`
	CreatedAt      time.Time  `json:"created_at"`
}

func (Redemption) TableName() string { return "platform_redemptions" }

type RedemptionUse struct {
	ID             int64     `json:"id" gorm:"primaryKey"`
	RedemptionID   int64     `json:"redemption_id" gorm:"not null;uniqueIndex:platform_redemption_workspace,priority:1"`
	TenantID       int64     `json:"tenant_id" gorm:"not null;uniqueIndex:platform_redemption_workspace,priority:2"`
	PlatformUserID int64     `json:"platform_user_id" gorm:"not null;index"`
	AssignmentID   int64     `json:"assignment_id" gorm:"not null;uniqueIndex"`
	CreatedAt      time.Time `json:"created_at"`
}

func (RedemptionUse) TableName() string { return "platform_redemption_uses" }

var errRedemption = errors.New("redemption unavailable")

func consumeRedemption(tx *gorm.DB, code string, tenantID, expectedPlanID int64, actor User) (plan.Assignment, error) {
	var entry Redemption
	if err := tx.Where("code_hash = ?", digest(strings.ToLower(strings.TrimSpace(code)))).First(&entry).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return plan.Assignment{}, errRedemption
		}
		return plan.Assignment{}, err
	}
	if expectedPlanID > 0 && entry.PlanID != expectedPlanID {
		return plan.Assignment{}, errRedemption
	}
	// Conditional increment enforces the global use limit even when two
	// different owners redeem simultaneously. Any later failure rolls back.
	result := tx.Model(&Redemption{}).Where("id = ? AND status = ? AND used < max_uses AND (expires_at IS NULL OR expires_at > ?)", entry.ID, "active", time.Now().UTC()).UpdateColumn("used", gorm.Expr("used + 1"))
	if result.Error != nil {
		return plan.Assignment{}, result.Error
	}
	if result.RowsAffected != 1 {
		return plan.Assignment{}, errRedemption
	}
	assignment, err := assignWorkspacePlan(tx, tenantID, entry.PlanID, entry.DurationMonths, actor, "redeem", &entry.ID)
	if err != nil {
		return plan.Assignment{}, err
	}
	return assignment, tx.Create(&RedemptionUse{RedemptionID: entry.ID, TenantID: tenantID, PlatformUserID: actor.ID, AssignmentID: assignment.ID}).Error
}

func (s *Server) createRedemptions(c *gin.Context) {
	var input struct {
		PlanID         int64      `json:"plan_id"`
		DurationMonths int        `json:"duration_months"`
		Count          int        `json:"count"`
		MaxUses        int        `json:"max_uses"`
		ExpiresAt      *time.Time `json:"expires_at"`
	}
	now := time.Now().UTC()
	if c.ShouldBindJSON(&input) != nil || input.PlanID <= 0 || input.DurationMonths < 1 || input.DurationMonths > 36 || input.Count < 1 || input.Count > 100 || input.MaxUses < 1 || input.MaxUses > 1000 || input.ExpiresAt != nil && (!input.ExpiresAt.After(now) || input.ExpiresAt.After(now.AddDate(10, 0, 0))) {
		writeError(c, http.StatusBadRequest, "invalid_redemption_batch")
		return
	}
	actor := c.MustGet("platform_user").(User)
	codes := make([]string, 0, input.Count)
	entries := make([]Redemption, 0, input.Count)
	for range input.Count {
		code, err := randomSecret()
		if err != nil {
			transactionError(c, err)
			return
		}
		codes = append(codes, code)
		entries = append(entries, Redemption{CodeHash: digest(code), CodeHint: code[len(code)-8:], PlanID: input.PlanID, DurationMonths: input.DurationMonths, Status: "active", MaxUses: input.MaxUses, ExpiresAt: input.ExpiresAt, CreatedBy: actor.ID})
	}
	err := s.adminTransaction(c, func(tx *gorm.DB) error {
		var p plan.Plan
		if err := tx.First(&p, input.PlanID).Error; err != nil {
			return err
		}
		if _, err := p.View(); err != nil {
			return err
		}
		if err := tx.Create(&entries).Error; err != nil {
			return err
		}
		return audit(tx, actor.ID, "redemption.create", 0, gin.H{"count": input.Count, "plan_id": input.PlanID, "months": input.DurationMonths, "max_uses": input.MaxUses, "expires_at": input.ExpiresAt})
	})
	if err != nil {
		transactionError(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"success": true, "codes": codes, "redemptions": entries})
}

func (s *Server) redemptions(c *gin.Context) {
	page, ok := pageQuery(c)
	if !ok {
		return
	}
	query := s.DB.WithContext(c.Request.Context()).Model(&Redemption{})
	if search := strings.TrimSpace(c.Query("search")); search != "" {
		query = query.Where("code_hint LIKE ?", "%"+search+"%")
	}
	now := time.Now().UTC()
	switch c.Query("status") {
	case "":
	case "active":
		query = query.Where("status = ? AND used < max_uses AND (expires_at IS NULL OR expires_at > ?)", "active", now)
	case "disabled":
		query = query.Where("status = ?", "disabled")
	case "exhausted":
		query = query.Where("status = ? AND used >= max_uses", "active")
	case "expired":
		query = query.Where("status = ? AND used < max_uses AND expires_at <= ?", "active", now)
	default:
		writeError(c, http.StatusBadRequest, "invalid_redemption_status")
		return
	}
	var items []Redemption
	if err := query.Count(&page.Total).Error; err != nil {
		transactionError(c, err)
		return
	}
	if err := query.Order("id desc").Offset((page.Page - 1) * page.PageSize).Limit(page.PageSize).Find(&items).Error; err != nil {
		transactionError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "redemptions": items, "pagination": page})
}

func (s *Server) disableRedemption(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		writeError(c, http.StatusBadRequest, "invalid_redemption_status")
		return
	}
	err = s.adminTransaction(c, func(tx *gorm.DB) error {
		var entry Redemption
		if err := tx.First(&entry, id).Error; err != nil {
			return err
		}
		if err := tx.Model(&entry).Update("status", "disabled").Error; err != nil {
			return err
		}
		return audit(tx, c.MustGet("platform_user").(User).ID, "redemption.disable", id, nil)
	})
	if err != nil {
		transactionError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (s *Server) redemptionUses(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		writeError(c, http.StatusBadRequest, "invalid_redemption_status")
		return
	}
	page, ok := pageQuery(c)
	if !ok {
		return
	}
	query := s.DB.WithContext(c.Request.Context()).Model(&RedemptionUse{}).Where("redemption_id = ?", id)
	var items []RedemptionUse
	if err := query.Count(&page.Total).Error; err != nil {
		transactionError(c, err)
		return
	}
	if err := query.Order("id desc").Offset((page.Page - 1) * page.PageSize).Limit(page.PageSize).Find(&items).Error; err != nil {
		transactionError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "uses": items, "pagination": page})
}

func (s *Server) redeem(c *gin.Context) {
	actor := c.MustGet("platform_user").(User)
	if !s.authRateLimit(c, "redeem:"+strconv.FormatInt(actor.ID, 10)) {
		return
	}
	var input struct {
		Code     string `json:"code"`
		TenantID int64  `json:"tenant_id"`
	}
	if c.ShouldBindJSON(&input) != nil || input.TenantID <= 0 || len(strings.TrimSpace(input.Code)) != 64 {
		writeError(c, http.StatusBadRequest, "redemption_unavailable")
		return
	}
	var assignment plan.Assignment
	err := s.DB.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		if _, err := activeUser(tx, actor); err != nil {
			return err
		}
		var err error
		assignment, err = consumeRedemption(tx, input.Code, input.TenantID, 0, actor)
		return err
	})
	if err != nil {
		// Missing, expired, disabled, exhausted, replayed, foreign and suspended
		// workspaces all have the same response; code values never enter logs.
		writeError(c, http.StatusBadRequest, "redemption_unavailable")
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "assignment": assignment})
}

func assignWorkspacePlan(tx *gorm.DB, tenantID, planID int64, months int, actor User, source string, redemptionID *int64) (plan.Assignment, error) {
	if months < 1 || months > 36 {
		return plan.Assignment{}, errRedemption
	}
	if err := tx.Model(&tenant.Workspace{}).Where("id = ?", tenantID).UpdateColumn("name", gorm.Expr("name")).Error; err != nil {
		return plan.Assignment{}, err
	}
	var workspace tenant.Workspace
	// A plain SELECT on MySQL REPEATABLE READ can see the earlier snapshot
	// even after our UPDATE waited for a concurrent renewal. A locking read
	// observes its committed expiry; SQLite is already serialized by UPDATE.
	if err := lockForUpdate(tx).First(&workspace, tenantID).Error; err != nil {
		return plan.Assignment{}, err
	}
	if source == "redeem" && (workspace.OwnerPlatformUserID != actor.ID || workspace.Status != "active") {
		return plan.Assignment{}, errRedemption
	}
	var p plan.Plan
	if err := tx.First(&p, planID).Error; err != nil {
		return plan.Assignment{}, err
	}
	if _, err := p.View(); err != nil {
		return plan.Assignment{}, err
	}
	now := time.Now().UTC()
	base := now
	// Renewals extend the same plan. Switching tiers starts now, so a Lite
	// entitlement cannot extend a Pro term (or vice versa) for the wrong price.
	if workspace.PlanID == planID && workspace.PlanExpiresAt != nil && workspace.PlanExpiresAt.After(now) {
		base = workspace.PlanExpiresAt.UTC()
	}
	month := time.Date(base.Year(), base.Month()+time.Month(months), 1, base.Hour(), base.Minute(), base.Second(), base.Nanosecond(), time.UTC)
	expires := month.AddDate(0, 0, min(base.Day(), month.AddDate(0, 1, -1).Day())-1)
	if expires.After(now.AddDate(100, 0, 0)) {
		return plan.Assignment{}, errRedemption
	}
	assignment := plan.Assignment{TenantID: tenantID, PlanID: planID, PlatformUserID: actor.ID, Source: source, RedemptionID: redemptionID, ExpiresAt: expires}
	if source == "manual" {
		assignment.AdministratorID = actor.ID
	}
	if err := tx.Model(&workspace).Updates(map[string]any{"plan_id": planID, "plan_expires_at": expires}).Error; err != nil {
		return assignment, err
	}
	if err := tx.Create(&assignment).Error; err != nil {
		return assignment, err
	}
	return assignment, audit(tx, actor.ID, "plan."+source, tenantID, gin.H{"plan_id": planID, "months": months, "expires_at": expires, "assignment_id": assignment.ID, "redemption_id": redemptionID})
}
