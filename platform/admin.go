package platform

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/plan"
	"github.com/QuantumNous/new-api/tenant"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// AdminGuard serializes governance changes across replicas, including the
// count-and-update operation that preserves the last enabled administrator.
type AdminGuard struct {
	ID int64 `gorm:"primaryKey;autoIncrement:false"`
}

func (AdminGuard) TableName() string { return "platform_admin_guard" }

type Audit struct {
	ID        int64     `json:"id" gorm:"primaryKey"`
	ActorID   int64     `json:"actor_id" gorm:"not null;index"`
	Action    string    `json:"action" gorm:"size:64;not null;index"`
	TargetID  int64     `json:"target_id" gorm:"not null;index"`
	Details   string    `json:"details" gorm:"type:text;not null"`
	CreatedAt time.Time `json:"created_at" gorm:"index"`
}

func (Audit) TableName() string { return "platform_audits" }

func audit(tx *gorm.DB, actorID int64, action string, targetID int64, details any) error {
	encoded, err := common.Marshal(details)
	if err != nil {
		return err
	}
	return tx.Create(&Audit{ActorID: actorID, Action: action, TargetID: targetID, Details: string(encoded)}).Error
}

func isAdmin(user User) bool { return user.Role == "admin" || user.Role == "root" }

func lockForUpdate(tx *gorm.DB) *gorm.DB {
	if common.UsingMainDatabase(common.DatabaseTypeSQLite) {
		return tx
	}
	return tx.Clauses(clause.Locking{Strength: "UPDATE"})
}

// An UPDATE acquires a row write lock on every supported database, including
// SQLite. Read after acquiring it so concurrent credential/role changes win.
func activeUser(tx *gorm.DB, expected User) (User, error) {
	if err := tx.Model(&User{}).Where("id = ?", expected.ID).
		UpdateColumn("session_version", gorm.Expr("session_version")).Error; err != nil {
		return User{}, err
	}
	var user User
	if err := lockForUpdate(tx).First(&user, expected.ID).Error; err != nil {
		return User{}, err
	}
	if user.Status != "active" || user.SessionVersion != expected.SessionVersion || user.PasswordHash != expected.PasswordHash {
		return User{}, &tenant.HTTPError{Status: http.StatusUnauthorized, Code: "platform_login_required"}
	}
	return user, nil
}

func (s *Server) adminTransaction(c *gin.Context, action func(*gorm.DB) error) error {
	return s.DB.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&AdminGuard{}).Where("id = ?", 1).UpdateColumn("id", gorm.Expr("id")).Error; err != nil {
			return err
		}
		user, err := activeUser(tx, c.MustGet("platform_user").(User))
		if err != nil {
			return err
		}
		if !isAdmin(user) || user.MustChangePassword {
			return &tenant.HTTPError{Status: http.StatusForbidden, Code: "platform_admin_required"}
		}
		return action(tx)
	})
}

func transactionError(c *gin.Context, err error) {
	var failure *tenant.HTTPError
	if errors.As(err, &failure) {
		writeError(c, failure.Status, failure.Code)
		return
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		writeError(c, http.StatusNotFound, "platform_record_not_found")
		return
	}
	writeError(c, http.StatusServiceUnavailable, "platform_unavailable")
}

type pagination struct {
	Page     int   `json:"page"`
	PageSize int   `json:"page_size"`
	Total    int64 `json:"total"`
}

func pageQuery(c *gin.Context) (pagination, bool) {
	page, err := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, sizeErr := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if err != nil || sizeErr != nil || page < 1 || page > 1000000 || size < 1 || size > 100 || len(c.Query("search")) > 128 {
		writeError(c, http.StatusBadRequest, "invalid_page")
		return pagination{}, false
	}
	return pagination{Page: page, PageSize: size}, true
}

func (s *Server) users(c *gin.Context) {
	page, ok := pageQuery(c)
	if !ok {
		return
	}
	query := s.DB.WithContext(c.Request.Context()).Model(&User{})
	if search := strings.TrimSpace(c.Query("search")); search != "" {
		query = query.Where("email LIKE ?", "%"+search+"%")
	}
	if status := c.Query("status"); status != "" {
		if status != "active" && status != "disabled" {
			writeError(c, http.StatusBadRequest, "invalid_user_status")
			return
		}
		query = query.Where("status = ?", status)
	}
	if role := c.Query("role"); role != "" {
		if role != "admin" && role != "root" && role != "user" {
			writeError(c, http.StatusBadRequest, "invalid_user_role")
			return
		}
		query = query.Where("role = ?", role)
	}
	var users []User
	if err := query.Count(&page.Total).Error; err != nil {
		transactionError(c, err)
		return
	}
	if err := query.Order("id desc").Offset((page.Page - 1) * page.PageSize).Limit(page.PageSize).Find(&users).Error; err != nil {
		transactionError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "users": users, "pagination": page})
}

func (s *Server) updateUser(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	var input struct {
		Action string `json:"action"`
	}
	if err != nil || id <= 0 || c.ShouldBindJSON(&input) != nil {
		writeError(c, http.StatusBadRequest, "invalid_user_action")
		return
	}
	actor := c.MustGet("platform_user").(User)
	err = s.adminTransaction(c, func(tx *gorm.DB) error {
		var target User
		if err := lockForUpdate(tx).First(&target, id).Error; err != nil {
			return err
		}
		updates := map[string]any{"session_version": gorm.Expr("session_version + 1")}
		switch input.Action {
		case "enable":
			updates["status"] = "active"
		case "disable":
			updates["status"] = "disabled"
		case "promote":
			if target.Role == "root" {
				return &tenant.HTTPError{Status: http.StatusBadRequest, Code: "invalid_user_action"}
			}
			updates["role"] = "admin"
		case "demote":
			updates["role"] = "user"
		case "require_password_change":
			updates["must_change_password"] = true
		case "revoke_sessions":
		default:
			return &tenant.HTTPError{Status: http.StatusBadRequest, Code: "invalid_user_action"}
		}
		if isAdmin(target) && target.Status == "active" && (input.Action == "disable" || input.Action == "demote") {
			var remaining int64
			if err := tx.Model(&User{}).Where("role IN ? AND status = ? AND id <> ?", []string{"admin", "root"}, "active", id).Count(&remaining).Error; err != nil {
				return err
			}
			if remaining == 0 {
				return &tenant.HTTPError{Status: http.StatusConflict, Code: "last_platform_admin"}
			}
		}
		if err := tx.Model(&User{}).Where("id = ?", id).Updates(updates).Error; err != nil {
			return err
		}
		if err := tx.Where("user_id = ?", id).Delete(&Session{}).Error; err != nil {
			return err
		}
		return audit(tx, actor.ID, "user."+input.Action, id, gin.H{"previous_role": target.Role, "previous_status": target.Status})
	})
	if err != nil {
		transactionError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (s *Server) updatePlan(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	var input plan.View
	if err != nil || id <= 0 || c.ShouldBindJSON(&input) != nil || strings.TrimSpace(input.Price) == "" || len(input.Price) > 64 {
		writeError(c, http.StatusBadRequest, "invalid_plan")
		return
	}
	for _, limit := range []int64{input.Limits.Requests, input.Limits.Users, input.Limits.Tokens, input.Limits.Channels} {
		if limit < 1 || limit > 1000000000 {
			writeError(c, http.StatusBadRequest, "invalid_plan")
			return
		}
	}
	limits, err := common.Marshal(input.Limits)
	if err != nil {
		transactionError(c, err)
		return
	}
	capabilities, err := common.Marshal(input.Capabilities)
	if err != nil {
		transactionError(c, err)
		return
	}
	p := plan.Plan{ID: id, Price: strings.TrimSpace(input.Price), Limits: string(limits), Capabilities: string(capabilities)}
	if _, err := p.View(); err != nil {
		writeError(c, http.StatusBadRequest, "invalid_plan")
		return
	}
	err = s.adminTransaction(c, func(tx *gorm.DB) error {
		var previous plan.Plan
		if err := tx.First(&previous, id).Error; err != nil {
			return err
		}
		if err := tx.Model(&p).Updates(map[string]any{"price": p.Price, "limits": p.Limits, "capabilities": p.Capabilities}).Error; err != nil {
			return err
		}
		before, err := previous.View()
		if err != nil {
			return err
		}
		input.Name = previous.Name
		input.ID = id
		return audit(tx, c.MustGet("platform_user").(User).ID, "plan.update", id, gin.H{"before": before, "after": input})
	})
	if err != nil {
		transactionError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (s *Server) audits(c *gin.Context) {
	page, ok := pageQuery(c)
	if !ok {
		return
	}
	query := s.DB.WithContext(c.Request.Context()).Model(&Audit{})
	if search := strings.TrimSpace(c.Query("search")); search != "" {
		query = query.Where("action LIKE ?", "%"+search+"%")
	}
	var items []Audit
	if err := query.Count(&page.Total).Error; err != nil {
		transactionError(c, err)
		return
	}
	if err := query.Order("id desc").Offset((page.Page - 1) * page.PageSize).Limit(page.PageSize).Find(&items).Error; err != nil {
		transactionError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "audits": items, "pagination": page})
}
