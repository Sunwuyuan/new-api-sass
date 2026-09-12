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
// Existing rows over the current ceiling stay; new inserts are rejected.
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
	var workspace tenant.Workspace
	if err := query.First(&workspace, identity.ID).Error; err != nil {
		return err
	}
	now := time.Now()
	view, err := ForWorkspace(query, workspace, now)
	if err != nil {
		return err
	}
	if table == "tokens" || table == "channels" {
		if Expired(workspace, now) {
			return ErrExpiredResources
		}
		return nil
	}
	limit := view.Limits.Users
	if limit == 0 {
		return nil
	}
	var count int64
	model := reflect.New(db.Statement.Schema.ModelType).Interface()
	if err := query.Model(model).Count(&count).Error; err != nil {
		return err
	}
	if int64(additional) > limit-count {
		return &tenant.HTTPError{Status: 403, Code: "tenant_resource_limit_exceeded", Message: fmt.Sprintf("Hosting plan allows at most %d users", limit)}
	}
	return nil
}

func ValidateOption(ctx context.Context, db *gorm.DB, key string) error {
	if strings.HasPrefix(key, "performance_setting.") {
		return &tenant.HTTPError{Status: 403, Code: "platform_managed_operation", Message: "Host performance settings are managed by the platform operator"}
	}
	if !gatedOption(key) {
		return nil
	}
	view, err := Current(ctx, db)
	if err != nil {
		return err
	}
	if denied, message := capabilityDenied(view, key); denied {
		return &tenant.HTTPError{Status: 403, Code: "plan_capability_denied", Message: message}
	}
	return nil
}

func gatedOption(key string) bool {
	switch key {
	case "Footer", "SystemName", "Logo", "PlatformMailEnabled",
		"DrawingEnabled", "TaskEnabled", "TaskPluginEnabled":
		return true
	}
	return optionClass(key) != ""
}

func capabilityDenied(view View, key string) (bool, string) {
	if key == "Footer" && !view.Capabilities.RemovePlatformFooter {
		return true, "Your hosting plan requires the platform footer"
	}
	if (key == "SystemName" || key == "Logo") && !view.Capabilities.CustomBranding {
		return true, "Your hosting plan does not allow custom branding"
	}
	if key == "PlatformMailEnabled" && !view.Capabilities.PlatformEmail {
		return true, "Your hosting plan does not include platform email"
	}
	if (key == "DrawingEnabled" || key == "TaskEnabled" || key == "TaskPluginEnabled") && !view.Capabilities.TaskPlugins {
		return true, "Your hosting plan does not include image, video or task plugins"
	}
	switch optionClass(key) {
	case "oauth":
		if !view.Capabilities.WorkspaceOAuth {
			return true, "Your hosting plan does not include workspace sign-in providers"
		}
	case "passkey":
		if !view.Capabilities.Passkey {
			return true, "Your hosting plan does not include Passkeys"
		}
	case "topup":
		if !view.Capabilities.Topup {
			return true, "Your hosting plan does not include wallet top-up"
		}
	case "affiliate":
		if !view.Capabilities.Affiliate {
			return true, "Your hosting plan does not include affiliate rewards"
		}
	case "models":
		if !view.Capabilities.CustomModels {
			return true, "Your hosting plan does not include custom model pricing"
		}
	}
	return false, ""
}

func optionClass(key string) string {
	switch {
	case strings.HasPrefix(key, "GitHub"), strings.HasPrefix(key, "LinuxDO"),
		strings.HasPrefix(key, "Telegram"), strings.HasPrefix(key, "WeChat"),
		strings.HasPrefix(key, "discord"), strings.HasPrefix(key, "Discord"),
		strings.HasPrefix(key, "oidc"), strings.HasPrefix(key, "OIDC"),
		strings.Contains(key, "OAuth"), strings.Contains(key, "oauth"):
		return "oauth"
	case strings.Contains(strings.ToLower(key), "passkey"):
		return "passkey"
	case strings.HasPrefix(key, "Stripe"), strings.HasPrefix(key, "Creem"),
		strings.HasPrefix(key, "Waffo"), strings.HasPrefix(key, "Pay"),
		strings.HasPrefix(key, "TopUp"), strings.HasPrefix(key, "Epay"),
		strings.Contains(strings.ToLower(key), "payment"):
		return "topup"
	case strings.HasPrefix(key, "Aff"), strings.Contains(strings.ToLower(key), "affiliate"):
		return "affiliate"
	case key == "ModelRatio" || key == "CompletionRatio" || key == "CacheRatio" ||
		key == "ModelPrice" || key == "GroupRatio" || key == "UserUsableGroups" ||
		strings.HasPrefix(key, "billing") || strings.Contains(key, "model_ratio") ||
		strings.Contains(key, "ModelRatio"):
		return "models"
	}
	return ""
}
