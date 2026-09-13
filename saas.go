package main

import (
	"context"
	"fmt"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/controller"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/oauth"
	perfmetrics "github.com/QuantumNous/new-api/pkg/perf_metrics"
	"github.com/QuantumNous/new-api/plan"
	"github.com/QuantumNous/new-api/platform"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/service/authz"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/QuantumNous/new-api/tenant"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var saasPlatform *platform.Server

func initializeWorkspace(ctx context.Context) error {
	ctx = context.WithoutCancel(ctx)
	ratio_setting.InitRatioSettings(ctx)
	model.InitOptionMap(ctx)
	service.InitHttpClient(ctx)
	if err := model.InitializeUserAuthVersions(ctx); err != nil {
		return err
	}
	if err := model.InitializeExternalIdentityClaims(ctx); err != nil {
		return err
	}
	if err := authz.Init(model.DB.WithContext(ctx)); err != nil {
		return err
	}
	if common.PasswordLoginEncryptionEnabled {
		if err := model.InitPasswordEncryption(ctx); err != nil {
			return err
		}
	}
	if err := oauth.LoadCustomProviders(ctx); err != nil {
		return err
	}
	if err := controller.RefreshTenantPlugins(ctx); err != nil {
		return err
	}
	model.InitChannelCache(ctx)
	return nil
}

func importStandaloneWorkspace() error {
	var existing int64
	if err := model.DB.Model(&tenant.Workspace{}).Count(&existing).Error; err != nil || existing != 0 {
		return err
	}
	var users int64
	if err := model.DB.Model(&model.User{}).Where("tenant_id = ?", 1).Count(&users).Error; err != nil || users == 0 {
		return err
	}
	var admin platform.User
	if err := model.DB.Where("role = ?", "admin").Order("id").First(&admin).Error; err != nil {
		return err
	}
	workspace := tenant.Workspace{ID: 1, Slug: "imported", Name: "New API", OwnerPlatformUserID: admin.ID, PlanID: 1, Status: "active"}
	return model.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&workspace).Error; err != nil {
			return err
		}
		if err := tx.Model(&platform.User{}).Where("id = ?", admin.ID).UpdateColumn("tenant_count", gorm.Expr("tenant_count + 1")).Error; err != nil {
			return err
		}
		// Redirects and callback links must use the new workspace entry point.
		address := model.Option{TenantID: 1, Key: "ServerAddress", Value: saasPlatform.Origin}
		return tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "tenant_id"}, {Name: "key"}}, DoUpdates: clause.AssignmentColumns([]string{"value"})}).Create(&address).Error
	})
}

// One scheduler enumerates tenant IDs and dispatches the existing job types.
// Neither a tenant process nor a long-lived per-tenant worker is created.
func runWorkspaceScheduler(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	lastMaintenance := make(map[int64]time.Time)
	runnerID := fmt.Sprintf("%s-%s", common.NodeName, common.GetRandomString(8))
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
		var workspaces []tenant.Workspace
		if err := model.DB.WithContext(ctx).Where("status = ?", "active").Find(&workspaces).Error; err != nil {
			common.SysError("list scheduled workspaces: " + err.Error())
			continue
		}
		for _, workspace := range workspaces {
			if err := plan.DowngradeExpired(model.DB.WithContext(ctx), &workspace, time.Now()); err != nil {
				common.SysError("downgrade expired workspace: " + err.Error())
				continue
			}
			tenantCtx := tenant.WithContext(ctx, tenant.Identity{ID: workspace.ID, Slug: workspace.Slug})
			if err := saasPlatform.EnsureInitialized(tenantCtx); err != nil {
				common.SysError("initialize scheduled workspace: " + err.Error())
				continue
			}
			model.RefreshTenantSettings(tenantCtx)
			model.InitChannelCache(tenantCtx)
			if err := authz.ReloadPolicy(tenantCtx); err != nil {
				common.SysError("reload workspace permissions: " + err.Error())
			}
			if err := controller.RefreshTenantPlugins(tenantCtx); err != nil {
				common.SysError("reload workspace plugins: " + err.Error())
			}
			model.SaveQuotaDataCache(tenantCtx)
			perfmetrics.FlushTenant(tenantCtx)
			maintenance := time.Since(lastMaintenance[workspace.ID]) >= 10*time.Minute
			if maintenance {
				lastMaintenance[workspace.ID] = time.Now()
			}
			if common.IsMasterNode {
				service.RunTenantJobs(tenantCtx, runnerID, maintenance)
			}
		}
	}
}
