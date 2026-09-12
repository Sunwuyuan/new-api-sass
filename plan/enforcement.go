package plan

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/tenant"
	"gorm.io/gorm"
)

const PlatformFooter = `Hosted with New API SaaS · Powered by New API / QuantumNous`

type contextKey struct{}

func WithContext(ctx context.Context, view View) context.Context {
	return context.WithValue(ctx, contextKey{}, view)
}

func FromContext(ctx context.Context) (View, bool) {
	view, ok := ctx.Value(contextKey{}).(View)
	return view, ok
}

// Current reads the authoritative plan for writes; request snapshots are only
// for rendering. A downgrade or suspension takes effect without restarting.
func Current(ctx context.Context, db *gorm.DB) (View, error) {
	identity, err := tenant.FromContext(ctx)
	if err != nil {
		return View{}, err
	}
	var workspace tenant.Workspace
	query := db.WithContext(ctx).Session(&gorm.Session{NewDB: true})
	if err := query.First(&workspace, identity.ID).Error; err != nil {
		return View{}, err
	}
	return ForWorkspace(query, workspace, time.Now())
}

// EnforceResourceCapacity runs inside GORM's create transaction. Locking the
// tenant row serializes counting and inserting across every application replica.
func EnforceResourceCapacity(db *gorm.DB, identity tenant.Identity, additional int) error {
	if db.DryRun || additional == 0 {
		return nil
	}
	table := db.Statement.Table
	if table != "users" && table != "tokens" && table != "channels" {
		return nil
	}
	query := db.Session(&gorm.Session{NewDB: true})
	if err := query.Model(&tenant.Workspace{}).Where("id = ?", identity.ID).
		UpdateColumn("name", gorm.Expr("name")).Error; err != nil {
		return err
	}
	view, err := Current(db.Statement.Context, query)
	if err != nil {
		return err
	}
	limit := view.Limits.Users
	if table == "tokens" {
		limit = view.Limits.Tokens
	} else if table == "channels" {
		limit = view.Limits.Channels
	}
	var count int64
	model := reflect.New(db.Statement.Schema.ModelType).Interface()
	if err := query.Model(model).Count(&count).Error; err != nil {
		return err
	}
	if int64(additional) > limit-count {
		return &tenant.HTTPError{Status: 403, Code: "tenant_resource_limit_exceeded", Message: fmt.Sprintf("Hosting plan allows at most %d %s", limit, table)}
	}
	return nil
}

func ValidateOption(ctx context.Context, db *gorm.DB, key string) error {
	if strings.HasPrefix(key, "performance_setting.") {
		return &tenant.HTTPError{Status: 403, Code: "platform_managed_operation", Message: "Host performance settings are managed by the platform operator"}
	}
	if key != "Footer" && key != "SystemName" && key != "Logo" {
		return nil
	}
	view, err := Current(ctx, db)
	if err != nil {
		return err
	}
	if key == "Footer" && !view.Capabilities.RemovePlatformFooter {
		return ErrCapability
	}
	if key != "Footer" && !view.Capabilities.CustomBranding {
		return &tenant.HTTPError{Status: 403, Code: "plan_capability_denied", Message: "Your hosting plan does not allow custom branding"}
	}
	return nil
}
