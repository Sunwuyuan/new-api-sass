// Package plan implements monthly SaaS hosting limits, independently of the
// gateway's upstream token billing and each workspace's user wallets.
package plan

import (
	"context"
	"errors"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/tenant"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var ErrLimit = errors.New("monthly request limit reached; ask the platform administrator to upgrade your plan")
var ErrInactive = errors.New("workspace is suspended or its monthly plan has expired")
var ErrCapability = &tenant.HTTPError{Status: 403, Code: "plan_capability_denied", Message: "Your hosting plan requires the platform footer"}

type Limits struct {
	Requests int64 `json:"requests"`
	Users    int64 `json:"users"`
	Tokens   int64 `json:"tokens"`
	Channels int64 `json:"channels"`
}

type Capabilities struct {
	RemovePlatformFooter bool `json:"remove_platform_footer"`
	CustomBranding       bool `json:"custom_branding"`
	MaxWorkspaces        int  `json:"max_workspaces"`
}

type Plan struct {
	ID           int64  `json:"id" gorm:"primaryKey"`
	Name         string `json:"name" gorm:"size:32;not null;uniqueIndex"`
	Price        string `json:"price" gorm:"size:64;not null"`
	Limits       string `json:"-" gorm:"type:text;not null"`
	Capabilities string `json:"-" gorm:"type:text;not null"`
}

type View struct {
	ID           int64        `json:"id"`
	Name         string       `json:"name"`
	Price        string       `json:"price"`
	Limits       Limits       `json:"limits"`
	Capabilities Capabilities `json:"capabilities"`
}

func (p Plan) View() (View, error) {
	v := View{ID: p.ID, Name: p.Name, Price: p.Price}
	if err := common.UnmarshalJsonStr(p.Limits, &v.Limits); err != nil {
		return v, err
	}
	if err := common.UnmarshalJsonStr(p.Capabilities, &v.Capabilities); err != nil {
		return v, err
	}
	if v.Limits.Requests <= 0 || v.Limits.Users <= 0 || v.Limits.Tokens <= 0 || v.Limits.Channels <= 0 {
		return v, errors.New("hosting plan contains invalid limits")
	}
	if v.Capabilities.MaxWorkspaces < 1 || v.Capabilities.MaxWorkspaces > 1000 {
		return v, errors.New("hosting plan contains invalid workspace capacity")
	}
	return v, nil
}

type Usage struct {
	TenantID int64  `json:"tenant_id" gorm:"primaryKey;autoIncrement:false"`
	Month    string `json:"month" gorm:"size:7;primaryKey"`
	Requests int64  `json:"requests" gorm:"not null"`
}

func (Usage) TableName() string { return "tenant_usage" }

type Assignment struct {
	ID              int64     `json:"id" gorm:"primaryKey"`
	TenantID        int64     `json:"tenant_id" gorm:"not null;index"`
	PlanID          int64     `json:"plan_id" gorm:"not null"`
	AdministratorID int64     `json:"administrator_id" gorm:"not null"`
	PlatformUserID  int64     `json:"platform_user_id" gorm:"not null;default:0"`
	Source          string    `json:"source" gorm:"size:16;not null;default:manual"`
	RedemptionID    *int64    `json:"redemption_id" gorm:"index"`
	ExpiresAt       time.Time `json:"expires_at"`
	CreatedAt       time.Time `json:"created_at"`
}

func (Assignment) TableName() string { return "plan_assignments" }

func Migrate(db *gorm.DB) error {
	if err := db.AutoMigrate(&Plan{}, &tenant.Workspace{}, &Usage{}, &Assignment{}); err != nil {
		return err
	}
	if common.UsingMainDatabase(common.DatabaseTypePostgreSQL) {
		var existing, standard int64
		if err := db.Model(&Plan{}).Count(&existing).Error; err != nil {
			return err
		}
		if err := db.Model(&Plan{}).Where("name = ?", "Standard").Count(&standard).Error; err != nil {
			return err
		}
		if existing > 0 && standard == 0 {
			// Phase 1 seeded explicit IDs, which do not advance a PostgreSQL
			// sequence. Advance it before GORM allocates the new Standard ID.
			// SQLite/MySQL already advance their allocator for explicit IDs.
			if err := db.Exec("SELECT setval(pg_get_serial_sequence('plans', 'id'), GREATEST((SELECT MAX(id) FROM plans), nextval(pg_get_serial_sequence('plans', 'id'))))").Error; err != nil {
				return err
			}
		}
	}
	for _, entry := range []View{
		{Name: "Lite", Price: "Free", Limits: Limits{Requests: 1000, Users: 5, Tokens: 20, Channels: 3}, Capabilities: Capabilities{MaxWorkspaces: 1}},
		{Name: "Pro", Price: "Contact administrator", Limits: Limits{Requests: 100000, Users: 1000, Tokens: 10000, Channels: 100}, Capabilities: Capabilities{RemovePlatformFooter: true, CustomBranding: true, MaxWorkspaces: 10}},
		{Name: "Standard", Price: "Contact administrator", Limits: Limits{Requests: 20000, Users: 50, Tokens: 200, Channels: 20}, Capabilities: Capabilities{CustomBranding: true, MaxWorkspaces: 3}},
	} {
		limits, err := common.Marshal(entry.Limits)
		if err != nil {
			return err
		}
		capabilities, err := common.Marshal(entry.Capabilities)
		if err != nil {
			return err
		}
		p := Plan{Name: entry.Name, Price: entry.Price, Limits: string(limits), Capabilities: string(capabilities)}
		if err := db.Where("name = ?", p.Name).FirstOrCreate(&p).Error; err != nil {
			return err
		}
		// Backfill only absent capabilities. Existing Pro IDs, prices, limits,
		// and explicit administrator capability overrides survive upgrades.
		var stored, defaults map[string]any
		if err := common.UnmarshalJsonStr(p.Capabilities, &stored); err != nil {
			return err
		}
		if stored == nil {
			return errors.New("hosting plan capabilities must be an object")
		}
		if err := common.Unmarshal(capabilities, &defaults); err != nil {
			return err
		}
		changed := false
		for key, value := range defaults {
			if _, exists := stored[key]; !exists {
				stored[key] = value
				changed = true
			}
		}
		if changed {
			encoded, err := common.Marshal(stored)
			if err != nil {
				return err
			}
			if err := db.Model(&p).Update("capabilities", string(encoded)).Error; err != nil {
				return err
			}
		}
	}
	return db.Model(&Assignment{}).Where("platform_user_id = ?", 0).
		UpdateColumn("platform_user_id", gorm.Expr("administrator_id")).Error
}

func ForWorkspace(db *gorm.DB, workspace tenant.Workspace, now time.Time) (View, error) {
	if workspace.Status != "active" || workspace.PlanExpiresAt != nil && !workspace.PlanExpiresAt.After(now) {
		return View{}, ErrInactive
	}
	var p Plan
	if err := db.First(&p, workspace.PlanID).Error; err != nil {
		return View{}, err
	}
	return p.View()
}

// Reserve uses a conditional update so concurrent replicas cannot overshoot
// the monthly ceiling. Failed, unbilled requests release their reservation.
func Reserve(ctx context.Context, db *gorm.DB, limit int64, now time.Time) (func(bool) error, error) {
	identity, err := tenant.FromContext(ctx)
	if err != nil {
		return nil, err
	}
	month := now.UTC().Format("2006-01")
	usage := Usage{TenantID: identity.ID, Month: month}
	query := db.WithContext(ctx)
	if err := query.Clauses(clause.OnConflict{DoNothing: true}).Create(&usage).Error; err != nil {
		return nil, err
	}
	result := query.Model(&Usage{}).Where("tenant_id = ? AND month = ? AND requests < ?", identity.ID, month, limit).
		UpdateColumn("requests", gorm.Expr("requests + 1"))
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected != 1 {
		return nil, ErrLimit
	}
	return func(success bool) error {
		if success {
			return nil
		}
		return db.WithContext(context.WithoutCancel(ctx)).Model(&Usage{}).
			Where("tenant_id = ? AND month = ? AND requests > 0", identity.ID, month).
			UpdateColumn("requests", gorm.Expr("requests - 1")).Error
	}, nil
}
