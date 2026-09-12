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
var ErrEmailLimit = errors.New("monthly platform email limit reached; ask the platform administrator to upgrade your plan")
var ErrInactive = errors.New("workspace is suspended")
var ErrCapability = &tenant.HTTPError{Status: 403, Code: "plan_capability_denied", Message: "Your hosting plan requires the platform footer"}
var ErrExpiredResources = &tenant.HTTPError{Status: 403, Code: "tenant_resource_limit_exceeded", Message: "Renew the hosting plan before creating tokens or channels"}

type Limits struct {
	Requests int64 `json:"requests"`
	Users    int64 `json:"users"`
	Tokens   int64 `json:"tokens"`
	Channels int64 `json:"channels"`
	Emails   int64 `json:"emails"`
}

type Capabilities struct {
	RemovePlatformFooter bool `json:"remove_platform_footer"`
	CustomBranding       bool `json:"custom_branding"`
	MaxWorkspaces        int  `json:"max_workspaces"`
	PlatformEmail        bool `json:"platform_email"`
	TaskPlugins          bool `json:"task_plugins"`
	DataExport           bool `json:"data_export"`
	WorkspaceOAuth       bool `json:"workspace_oauth"`
	Topup                bool `json:"topup"`
	Affiliate            bool `json:"affiliate"`
	Passkey              bool `json:"passkey"`
	CustomModels         bool `json:"custom_models"`
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
	if v.Limits.Requests < 0 || v.Limits.Users < 0 || v.Limits.Tokens < 0 || v.Limits.Channels < 0 || v.Limits.Emails < 0 {
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
	Emails   int64  `json:"emails" gorm:"not null;default:0"`
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

func Defaults() []View {
	return []View{
		{
			Name: "Lite", Price: "Free",
			Limits: Limits{Requests: 10000, Users: 1, Emails: 100},
			Capabilities: Capabilities{
				MaxWorkspaces: 1, PlatformEmail: true, TaskPlugins: true, DataExport: true, Passkey: true,
			},
		},
		{
			Name: "Pro", Price: "Contact administrator",
			Limits: Limits{Requests: 0, Users: 0},
			Capabilities: Capabilities{
				RemovePlatformFooter: true, CustomBranding: true, MaxWorkspaces: 20,
				PlatformEmail: true, TaskPlugins: true, DataExport: true, WorkspaceOAuth: true,
				Topup: true, Affiliate: true, Passkey: true, CustomModels: true,
			},
		},
		{
			Name: "Standard", Price: "Contact administrator",
			Limits: Limits{Requests: 100000, Users: 1000, Emails: 1000},
			Capabilities: Capabilities{
				CustomBranding: true, MaxWorkspaces: 5, PlatformEmail: true, TaskPlugins: true,
				DataExport: true, WorkspaceOAuth: true, Topup: true, Passkey: true,
			},
		},
	}
}

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
	for _, entry := range Defaults() {
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
		if err := backfillJSON(db, &p, "limits", p.Limits, string(limits)); err != nil {
			return err
		}
		if err := backfillJSON(db, &p, "capabilities", p.Capabilities, string(capabilities)); err != nil {
			return err
		}
		if err := syncBuiltinPlanPolicy(db, &p, entry); err != nil {
			return err
		}
	}
	return db.Model(&Assignment{}).Where("platform_user_id = ?", 0).
		UpdateColumn("platform_user_id", gorm.Expr("administrator_id")).Error
}

// Backfill only absent keys. Administrator overrides and existing numeric
// ceilings survive upgrades.
func backfillJSON(db *gorm.DB, p *Plan, column, storedJSON, defaultsJSON string) error {
	var stored, defaults map[string]any
	if err := common.UnmarshalJsonStr(storedJSON, &stored); err != nil {
		return err
	}
	if stored == nil {
		return errors.New("hosting plan " + column + " must be an object")
	}
	if err := common.UnmarshalJsonStr(defaultsJSON, &defaults); err != nil {
		return err
	}
	changed := false
	for key, value := range defaults {
		if _, exists := stored[key]; !exists {
			stored[key] = value
			changed = true
		}
	}
	if !changed {
		return nil
	}
	encoded, err := common.Marshal(stored)
	if err != nil {
		return err
	}
	return db.Model(p).Update(column, string(encoded)).Error
}

// Builtin request, user and email ceilings are product policy. Token and
// channel counts are not consumption gates and stay unlimited.
func syncBuiltinPlanPolicy(db *gorm.DB, p *Plan, entry View) error {
	if err := db.First(p, p.ID).Error; err != nil {
		return err
	}
	view, err := p.View()
	if err != nil {
		return err
	}
	if view.Limits.Requests == entry.Limits.Requests &&
		view.Limits.Users == entry.Limits.Users &&
		view.Limits.Emails == entry.Limits.Emails &&
		view.Limits.Tokens == 0 && view.Limits.Channels == 0 &&
		view.Capabilities.DataExport {
		return nil
	}
	view.Limits.Requests = entry.Limits.Requests
	view.Limits.Users = entry.Limits.Users
	view.Limits.Emails = entry.Limits.Emails
	view.Limits.Tokens = 0
	view.Limits.Channels = 0
	view.Capabilities.DataExport = true
	limits, err := common.Marshal(view.Limits)
	if err != nil {
		return err
	}
	capabilities, err := common.Marshal(view.Capabilities)
	if err != nil {
		return err
	}
	return db.Model(p).Updates(map[string]any{
		"limits": string(limits), "capabilities": string(capabilities),
	}).Error
}

func Expired(workspace tenant.Workspace, now time.Time) bool {
	return workspace.PlanExpiresAt != nil && !workspace.PlanExpiresAt.After(now)
}

func Lite(db *gorm.DB) (View, error) {
	var p Plan
	if err := db.Where("name = ?", "Lite").First(&p).Error; err != nil {
		return View{}, err
	}
	return p.View()
}

func ForWorkspace(db *gorm.DB, workspace tenant.Workspace, now time.Time) (View, error) {
	if workspace.Status != "active" {
		return View{}, ErrInactive
	}
	if Expired(workspace, now) {
		return Lite(db)
	}
	var p Plan
	if err := db.First(&p, workspace.PlanID).Error; err != nil {
		return View{}, err
	}
	return p.View()
}

// DowngradeExpired switches a lapsed paid workspace onto Lite without removing
// users, tokens or channels. PlanExpiresAt stays so the UI can ask for renewal.
func DowngradeExpired(db *gorm.DB, workspace *tenant.Workspace, now time.Time) error {
	if workspace == nil || workspace.Status != "active" || !Expired(*workspace, now) {
		return nil
	}
	var lite Plan
	if err := db.Where("name = ?", "Lite").First(&lite).Error; err != nil {
		return err
	}
	if workspace.PlanID == lite.ID {
		return nil
	}
	if err := db.Model(workspace).Update("plan_id", lite.ID).Error; err != nil {
		return err
	}
	workspace.PlanID = lite.ID
	return nil
}

// Reserve uses a conditional update so concurrent replicas cannot overshoot
// the monthly ceiling. Failed, unbilled requests release their reservation.
func Reserve(ctx context.Context, db *gorm.DB, limit int64, now time.Time) (func(bool) error, error) {
	return reserveColumn(ctx, db, "requests", limit, now, ErrLimit)
}

func ReserveEmail(ctx context.Context, db *gorm.DB, limit int64, now time.Time) (func(bool) error, error) {
	return reserveColumn(ctx, db, "emails", limit, now, ErrEmailLimit)
}

func reserveColumn(ctx context.Context, db *gorm.DB, column string, limit int64, now time.Time, exhausted error) (func(bool) error, error) {
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
	q := query.Model(&Usage{}).Where("tenant_id = ? AND month = ?", identity.ID, month)
	if limit > 0 {
		q = q.Where(column+" < ?", limit)
	}
	result := q.UpdateColumn(column, gorm.Expr(column+" + 1"))
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected != 1 {
		if limit > 0 {
			return nil, exhausted
		}
		return nil, errors.New("hosting plan usage counter missing")
	}
	return func(success bool) error {
		if success {
			return nil
		}
		return db.WithContext(context.WithoutCancel(ctx)).Model(&Usage{}).
			Where("tenant_id = ? AND month = ? AND "+column+" > 0", identity.ID, month).
			UpdateColumn(column, gorm.Expr(column+" - 1")).Error
	}, nil
}
