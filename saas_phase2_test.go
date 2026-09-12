package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/plan"
	"github.com/QuantumNous/new-api/platform"
	"github.com/QuantumNous/new-api/tenant"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type platformCodeBatch struct {
	Codes       []string
	Redemptions []platform.Redemption
}

func createPlatformCodes(t *testing.T, admin *saasBrowser, planID int64, months, uses, count int) platformCodeBatch {
	t.Helper()
	var batch platformCodeBatch
	res := admin.request(t, http.MethodPost, "/platform/api/admin/redemptions", map[string]any{"plan_id": planID, "duration_months": months, "max_uses": uses, "count": count}, &batch)
	require.Equal(t, http.StatusCreated, res.Code, res.Body.String())
	require.Len(t, batch.Codes, count)
	require.Len(t, batch.Redemptions, count)
	return batch
}

func resetPlatformAuthAttempts(t *testing.T) {
	t.Helper()
	require.NoError(t, model.DB.Where("1 = 1").Delete(&platform.AuthAttempt{}).Error)
}

func testSaaSPhase2(t *testing.T, saas *platform.Server, server http.Handler, admin *saasBrowser, adminPassword string) {
	resetPlatformAuthAttempts(t)
	t.Cleanup(func() { resetPlatformAuthAttempts(t) })
	alicePassword, bobPassword := saasPassword(t), saasPassword(t)
	alice, bob := &saasBrowser{handler: server}, &saasBrowser{handler: server}
	for _, fixture := range []struct {
		browser  *saasBrowser
		email    string
		password string
	}{{alice, "phase2-alice@example.test", alicePassword}, {bob, "phase2-bob@example.test", bobPassword}} {
		res := fixture.browser.request(t, http.MethodPost, "/platform/api/register", map[string]string{"email": fixture.email, "password": fixture.password, "role": "admin"}, nil)
		require.Equal(t, http.StatusOK, res.Code)
		fixture.browser.login(t, fixture.email, fixture.password)
	}
	var aliceUser, bobUser platform.User
	require.NoError(t, model.DB.Where("email = ?", "phase2-alice@example.test").First(&aliceUser).Error)
	require.NoError(t, model.DB.Where("email = ?", "phase2-bob@example.test").First(&bobUser).Error)
	assert.Equal(t, "user", aliceUser.Role, "registration cannot set its own role")
	var alpha, beta struct{ Tenant tenant.Workspace }
	require.Equal(t, http.StatusCreated, alice.request(t, http.MethodPost, "/platform/api/tenants", map[string]string{"name": "Phase Two Alpha", "slug": "phase2-alpha"}, &alpha).Code)
	require.Equal(t, http.StatusCreated, bob.request(t, http.MethodPost, "/platform/api/tenants", map[string]string{"name": "Phase Two Beta", "slug": "phase2-beta"}, &beta).Code)

	t.Run("users cannot reach any administrator endpoint or enumerate foreign workspaces", func(t *testing.T) {
		for _, path := range []string{"/users", "/tenants", "/plans", "/redemptions", "/redemptions/1/uses", "/audits"} {
			assert.Equal(t, http.StatusForbidden, alice.request(t, http.MethodGet, "/platform/api/admin"+path, nil, nil).Code, path)
		}
		for _, path := range []string{"/users/1", "/tenants/1/plan", "/tenants/1/status", "/plans/1", "/redemptions", "/redemptions/1/disable"} {
			assert.Equal(t, http.StatusForbidden, alice.request(t, http.MethodPost, "/platform/api/admin"+path, map[string]string{}, nil).Code, path)
		}
		var listed struct {
			Tenants    []any
			Pagination struct{ Total int }
		}
		require.Equal(t, http.StatusOK, alice.request(t, http.MethodGet, "/platform/api/tenants?search=phase2-beta", nil, &listed).Code)
		assert.Empty(t, listed.Tenants)
		assert.Zero(t, listed.Pagination.Total)
		var users struct {
			Users      []platform.User
			Pagination struct{ Total int }
		}
		require.Equal(t, http.StatusOK, admin.request(t, http.MethodGet, "/platform/api/admin/users?search=phase2-&page_size=1&page=2&status=active", nil, &users).Code)
		assert.Len(t, users.Users, 1)
		assert.Equal(t, 2, users.Pagination.Total)
		assert.Equal(t, http.StatusBadRequest, admin.request(t, http.MethodGet, "/platform/api/admin/users?page_size=101", nil, nil).Code)
	})

	t.Run("Lite capacity and the three capability tiers are enforced", func(t *testing.T) {
		var plans struct{ Plans []plan.View }
		require.Equal(t, http.StatusOK, alice.request(t, http.MethodGet, "/platform/api/plans", nil, &plans).Code)
		require.Len(t, plans.Plans, 3)
		byName := map[string]plan.View{}
		for _, p := range plans.Plans {
			byName[p.Name] = p
		}
		assert.EqualValues(t, 20000, byName["Standard"].Limits.Requests)
		assert.EqualValues(t, 50, byName["Standard"].Limits.Users)
		assert.EqualValues(t, 200, byName["Standard"].Limits.Tokens)
		assert.EqualValues(t, 20, byName["Standard"].Limits.Channels)
		assert.Equal(t, plan.Capabilities{MaxWorkspaces: 1}, byName["Lite"].Capabilities)
		assert.Equal(t, plan.Capabilities{MaxWorkspaces: 3, CustomBranding: true}, byName["Standard"].Capabilities)
		assert.Equal(t, plan.Capabilities{MaxWorkspaces: 10, CustomBranding: true, RemovePlatformFooter: true}, byName["Pro"].Capabilities)
		assert.Equal(t, http.StatusConflict, alice.request(t, http.MethodPost, "/platform/api/tenants", map[string]string{"name": "Too many", "slug": "phase2-limit"}, nil).Code)
		ctx := tenant.WithContext(t.Context(), tenant.Identity{ID: alpha.Tenant.ID, Slug: alpha.Tenant.Slug})
		assert.Error(t, plan.ValidateOption(ctx, model.DB, "SystemName"))
		assert.Error(t, plan.ValidateOption(ctx, model.DB, "Logo"))
		assert.ErrorIs(t, plan.ValidateOption(ctx, model.DB, "Footer"), plan.ErrCapability)
		batch := createPlatformCodes(t, admin, byName["Standard"].ID, 1, 1, 1)
		var result struct{ Assignment plan.Assignment }
		require.Equal(t, http.StatusBadRequest, bob.request(t, http.MethodPost, "/platform/api/redeem", map[string]any{"tenant_id": alpha.Tenant.ID, "code": batch.Codes[0]}, nil).Code)
		require.Equal(t, http.StatusOK, alice.request(t, http.MethodPost, "/platform/api/redeem", map[string]any{"tenant_id": alpha.Tenant.ID, "code": batch.Codes[0]}, &result).Code)
		assert.Equal(t, "redeem", result.Assignment.Source)
		assert.Equal(t, aliceUser.ID, result.Assignment.PlatformUserID)
		assert.Equal(t, batch.Redemptions[0].ID, *result.Assignment.RedemptionID)
		assert.NoError(t, plan.ValidateOption(ctx, model.DB, "SystemName"))
		assert.NoError(t, plan.ValidateOption(ctx, model.DB, "Logo"))
		assert.ErrorIs(t, plan.ValidateOption(ctx, model.DB, "Footer"), plan.ErrCapability)
		var listed struct {
			MaxWorkspaces int `json:"max_workspaces"`
		}
		require.Equal(t, http.StatusOK, alice.request(t, http.MethodGet, "/platform/api/tenants", nil, &listed).Code)
		assert.Equal(t, 3, listed.MaxWorkspaces)
	})

	t.Run("renewal preserves calendar months and replay does not spend a use", func(t *testing.T) {
		resetPlatformAuthAttempts(t)
		base := time.Date(time.Now().Year()+1, time.January, 31, 12, 0, 0, 0, time.UTC)
		require.NoError(t, model.DB.Model(&tenant.Workspace{}).Where("id = ?", alpha.Tenant.ID).Update("plan_expires_at", base).Error)
		batch := createPlatformCodes(t, admin, 3, 1, 2, 1)
		input := map[string]any{"tenant_id": alpha.Tenant.ID, "code": batch.Codes[0]}
		var result struct{ Assignment plan.Assignment }
		require.Equal(t, http.StatusOK, alice.request(t, http.MethodPost, "/platform/api/redeem", input, &result).Code)
		expected := time.Date(base.Year(), time.March, 0, 12, 0, 0, 0, time.UTC)
		assert.True(t, expected.Equal(result.Assignment.ExpiresAt))
		assert.Equal(t, http.StatusBadRequest, alice.request(t, http.MethodPost, "/platform/api/redeem", input, nil).Code)
		var entry platform.Redemption
		require.NoError(t, model.DB.First(&entry, batch.Redemptions[0].ID).Error)
		assert.Equal(t, 1, entry.Used)
		assert.NotEqual(t, batch.Codes[0], entry.CodeHash)
		require.Equal(t, http.StatusOK, bob.request(t, http.MethodPost, "/platform/api/redeem", map[string]any{"tenant_id": beta.Tenant.ID, "code": batch.Codes[0]}, nil).Code)
		var uses struct {
			Uses       []platform.RedemptionUse
			Pagination struct{ Total int }
		}
		require.Equal(t, http.StatusOK, admin.request(t, http.MethodGet, fmt.Sprintf("/platform/api/admin/redemptions/%d/uses?page_size=1", entry.ID), nil, &uses).Code)
		assert.Equal(t, 2, uses.Pagination.Total)
		assert.Len(t, uses.Uses, 1)
		assert.NotZero(t, uses.Uses[0].AssignmentID)
		listed := admin.request(t, http.MethodGet, "/platform/api/admin/redemptions?status=exhausted", nil, nil)
		assert.NotContains(t, listed.Body.String(), batch.Codes[0])
		assert.NotContains(t, listed.Body.String(), entry.CodeHash)
	})

	t.Run("disabled expired unknown foreign and suspended redemptions share a response", func(t *testing.T) {
		resetPlatformAuthAttempts(t)
		batch := createPlatformCodes(t, admin, 2, 1, 1, 3)
		require.Equal(t, http.StatusOK, admin.request(t, http.MethodPost, fmt.Sprintf("/platform/api/admin/redemptions/%d/disable", batch.Redemptions[0].ID), nil, nil).Code)
		require.NoError(t, model.DB.Model(&platform.Redemption{}).Where("id = ?", batch.Redemptions[1].ID).Update("expires_at", time.Now().Add(-time.Minute)).Error)
		var response string
		for _, code := range []string{batch.Codes[0], batch.Codes[1], "unknown", fmt.Sprintf("%064x", 0)} {
			res := alice.request(t, http.MethodPost, "/platform/api/redeem", map[string]any{"tenant_id": alpha.Tenant.ID, "code": code}, nil)
			require.Equal(t, http.StatusBadRequest, res.Code)
			if response == "" {
				response = res.Body.String()
			}
			assert.JSONEq(t, response, res.Body.String())
		}
		res := bob.request(t, http.MethodPost, "/platform/api/redeem", map[string]any{"tenant_id": alpha.Tenant.ID, "code": batch.Codes[2]}, nil)
		assert.JSONEq(t, response, res.Body.String())
		require.Equal(t, http.StatusOK, admin.request(t, http.MethodPost, fmt.Sprintf("/platform/api/admin/tenants/%d/status", alpha.Tenant.ID), map[string]string{"status": "suspended"}, nil).Code)
		res = alice.request(t, http.MethodPost, "/platform/api/redeem", map[string]any{"tenant_id": alpha.Tenant.ID, "code": batch.Codes[2]}, nil)
		assert.JSONEq(t, response, res.Body.String())
		assert.Equal(t, http.StatusForbidden, alice.request(t, http.MethodGet, "/t/phase2-alpha/api/status", nil, nil).Code)
		require.Equal(t, http.StatusOK, admin.request(t, http.MethodPost, fmt.Sprintf("/platform/api/admin/tenants/%d/status", alpha.Tenant.ID), map[string]string{"status": "active"}, nil).Code)
		var assigned struct{ Assignment plan.Assignment }
		require.Equal(t, http.StatusOK, alice.request(t, http.MethodPost, "/platform/api/redeem", map[string]any{"tenant_id": alpha.Tenant.ID, "code": batch.Codes[2]}, &assigned).Code)
		assert.WithinDuration(t, time.Now().UTC().AddDate(0, 1, 0), assigned.Assignment.ExpiresAt, 5*time.Second, "switching from Standard to Pro starts now")
		ctx := tenant.WithContext(t.Context(), tenant.Identity{ID: alpha.Tenant.ID, Slug: alpha.Tenant.Slug})
		assert.NoError(t, plan.ValidateOption(ctx, model.DB, "Footer"))
		var codes []platform.Redemption
		require.NoError(t, model.DB.Where("id IN ?", []int64{batch.Redemptions[0].ID, batch.Redemptions[1].ID}).Find(&codes).Error)
		for _, entry := range codes {
			assert.Zero(t, entry.Used)
		}
	})

	t.Run("audit failure rolls back the plan assignment and code use", func(t *testing.T) {
		batch := createPlatformCodes(t, admin, 3, 1, 1, 1)
		var before, after tenant.Workspace
		require.NoError(t, model.DB.First(&before, alpha.Tenant.ID).Error)
		err := model.DB.Callback().Create().Before("gorm:create").Register("phase2:fail_audit", func(tx *gorm.DB) {
			if tx.Statement.Table == "platform_audits" {
				tx.AddError(errors.New("audit unavailable"))
			}
		})
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, model.DB.Callback().Create().Remove("phase2:fail_audit")) })
		assert.Equal(t, http.StatusBadRequest, alice.request(t, http.MethodPost, "/platform/api/redeem", map[string]any{"tenant_id": alpha.Tenant.ID, "code": batch.Codes[0]}, nil).Code)
		var entry platform.Redemption
		require.NoError(t, model.DB.First(&entry, batch.Redemptions[0].ID).Error)
		assert.Zero(t, entry.Used)
		require.NoError(t, model.DB.First(&after, alpha.Tenant.ID).Error)
		assert.Equal(t, before.PlanID, after.PlanID)
		assert.True(t, before.PlanExpiresAt.Equal(*after.PlanExpiresAt))
		var assignments int64
		require.NoError(t, model.DB.Model(&plan.Assignment{}).Where("redemption_id = ?", entry.ID).Count(&assignments).Error)
		assert.Zero(t, assignments)
	})

	t.Run("concurrent redemption cannot exceed its global use limit", func(t *testing.T) {
		resetPlatformAuthAttempts(t)
		batch := createPlatformCodes(t, admin, 3, 1, 1, 1)
		var requests []*http.Request
		for _, fixture := range []struct {
			browser  *saasBrowser
			tenantID int64
		}{{alice, alpha.Tenant.ID}, {bob, beta.Tenant.ID}} {
			body, err := common.Marshal(map[string]any{"tenant_id": fixture.tenantID, "code": batch.Codes[0]})
			require.NoError(t, err)
			req := httptest.NewRequest(http.MethodPost, "/platform/api/redeem", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Origin", "http://localhost:3000")
			req.Header.Set("X-Requested-With", "NewAPIPlatform")
			req.Header.Set("X-CSRF-Token", fixture.browser.csrf)
			req.AddCookie(fixture.browser.cookie)
			requests = append(requests, req)
		}
		var wg sync.WaitGroup
		responses := make(chan int, len(requests))
		for _, req := range requests {
			wg.Go(func() { res := httptest.NewRecorder(); server.ServeHTTP(res, req); responses <- res.Code })
		}
		wg.Wait()
		close(responses)
		var statuses []int
		for code := range responses {
			statuses = append(statuses, code)
		}
		assert.ElementsMatch(t, []int{http.StatusOK, http.StatusBadRequest}, statuses)
		var entry platform.Redemption
		require.NoError(t, model.DB.First(&entry, batch.Redemptions[0].ID).Error)
		assert.Equal(t, 1, entry.Used)
		var uses int64
		require.NoError(t, model.DB.Model(&platform.RedemptionUse{}).Where("redemption_id = ?", entry.ID).Count(&uses).Error)
		assert.EqualValues(t, 1, uses)
	})

	t.Run("renewal reads the latest expiry after a concurrent manual assignment", func(t *testing.T) {
		if common.UsingMainDatabase(common.DatabaseTypeSQLite) {
			t.Skip("SQLite serializes all writes; this interleaving requires row-level MVCC")
		}
		resetPlatformAuthAttempts(t)
		base := time.Date(time.Now().Year()+1, time.January, 15, 12, 0, 0, 0, time.UTC)
		require.NoError(t, model.DB.Model(&tenant.Workspace{}).Where("id = ?", alpha.Tenant.ID).Updates(map[string]any{"plan_id": 3, "plan_expires_at": base}).Error)
		batch := createPlatformCodes(t, admin, 3, 1, 1, 1)
		ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
		defer cancel()
		ready, resume := make(chan struct{}), make(chan struct{})
		var pause, release sync.Once
		defer release.Do(func() { close(resume) })
		err := model.DB.Callback().Update().Before("gorm:update").Register("phase2:renewal_interleaving", func(tx *gorm.DB) {
			if tx.Statement.Table != "tenants" || tx.Statement.Context != ctx {
				return
			}
			pause.Do(func() {
				close(ready)
				select {
				case <-resume:
				case <-ctx.Done():
					tx.AddError(ctx.Err())
				}
			})
		})
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, model.DB.Callback().Update().Remove("phase2:renewal_interleaving")) })
		body, err := common.Marshal(map[string]any{"tenant_id": alpha.Tenant.ID, "code": batch.Codes[0]})
		require.NoError(t, err)
		req := httptest.NewRequest(http.MethodPost, "/platform/api/redeem", bytes.NewReader(body)).WithContext(ctx)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Origin", saas.Origin)
		req.Header.Set("X-Requested-With", "NewAPIPlatform")
		req.Header.Set("X-CSRF-Token", alice.csrf)
		req.AddCookie(alice.cookie)
		completed := make(chan *httptest.ResponseRecorder, 1)
		finished := make(chan struct{})
		go func() {
			defer close(finished)
			res := httptest.NewRecorder()
			server.ServeHTTP(res, req)
			completed <- res
		}()
		defer func() { release.Do(func() { close(resume) }); cancel(); <-finished }()
		select {
		case <-ready:
		case res := <-completed:
			t.Fatalf("redemption completed before the controlled interleaving: %d", res.Code)
		case <-ctx.Done():
			t.Fatal("redemption did not reach workspace locking")
		}
		require.Equal(t, http.StatusOK, admin.request(t, http.MethodPost, fmt.Sprintf("/platform/api/admin/tenants/%d/plan", alpha.Tenant.ID), map[string]int{"plan_id": 3, "months": 1}, nil).Code)
		release.Do(func() { close(resume) })
		res := <-completed
		require.Equal(t, http.StatusOK, res.Code)
		var result struct{ Assignment plan.Assignment }
		require.NoError(t, common.Unmarshal(res.Body.Bytes(), &result))
		assert.True(t, base.AddDate(0, 2, 0).Equal(result.Assignment.ExpiresAt), "both renewals must add a month, including on MySQL REPEATABLE READ")
	})

	t.Run("bootstrap preserves administrator role changes across restarts", func(t *testing.T) {
		resetPlatformAuthAttempts(t)
		t.Cleanup(func() {
			require.NoError(t, model.DB.Model(&platform.User{}).Where("id = ?", 1).Updates(map[string]any{"role": "admin", "session_version": gorm.Expr("session_version + 1")}).Error)
			require.NoError(t, model.DB.Model(&platform.User{}).Where("id = ?", bobUser.ID).Updates(map[string]any{"role": "user", "session_version": gorm.Expr("session_version + 1")}).Error)
			resetPlatformAuthAttempts(t)
			admin.login(t, "administrator@example.test", adminPassword)
			bob.login(t, *bobUser.Email, bobPassword)
		})
		require.Equal(t, http.StatusOK, admin.request(t, http.MethodPost, fmt.Sprintf("/platform/api/admin/users/%d", bobUser.ID), map[string]string{"action": "promote"}, nil).Code)
		require.Equal(t, http.StatusOK, admin.request(t, http.MethodPost, "/platform/api/admin/users/1", map[string]string{"action": "demote"}, nil).Code)
		// Startup migration runs before runtime tenant callbacks are installed.
		// Reopen the real database to exercise the same ordering on restart.
		restarted, err := gorm.Open(model.DB.Dialector, &gorm.Config{Logger: model.DB.Logger})
		require.NoError(t, err)
		pool, err := restarted.DB()
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, pool.Close()) })
		_, err = platform.New(restarted)
		require.NoError(t, err, "bootstrap credentials must not override explicitly managed roles or prevent startup when another administrator remains")
		var original platform.User
		require.NoError(t, model.DB.First(&original, 1).Error)
		assert.Equal(t, "user", original.Role)
		assert.Equal(t, http.StatusUnauthorized, admin.request(t, http.MethodGet, "/platform/api/admin/users", nil, nil).Code)
		bob.login(t, *bobUser.Email, bobPassword)
		assert.Equal(t, http.StatusOK, bob.request(t, http.MethodGet, "/platform/api/admin/users", nil, nil).Code)
	})

	t.Run("plan edits and expiry immediately change workspace capacity", func(t *testing.T) {
		var original plan.Plan
		require.NoError(t, model.DB.First(&original, 3).Error)
		view, err := original.View()
		require.NoError(t, err)
		t.Cleanup(func() {
			require.NoError(t, model.DB.Model(&plan.Plan{}).Where("id = ?", 3).Updates(map[string]any{"limits": original.Limits, "capabilities": original.Capabilities}).Error)
		})
		view.Capabilities.MaxWorkspaces = 2
		view.Limits.Requests = 22000
		require.Equal(t, http.StatusOK, admin.request(t, http.MethodPost, "/platform/api/admin/plans/3", view, nil).Code)
		require.Equal(t, http.StatusOK, admin.request(t, http.MethodPost, fmt.Sprintf("/platform/api/admin/tenants/%d/plan", alpha.Tenant.ID), map[string]int{"plan_id": 3, "months": 1}, nil).Code)
		var second struct{ Tenant tenant.Workspace }
		require.Equal(t, http.StatusCreated, alice.request(t, http.MethodPost, "/platform/api/tenants", map[string]string{"name": "Second", "slug": "phase2-second"}, &second).Code)
		assert.Equal(t, http.StatusConflict, alice.request(t, http.MethodPost, "/platform/api/tenants", map[string]string{"name": "Over capacity", "slug": "phase2-capacity"}, nil).Code)
		require.NoError(t, model.DB.Model(&tenant.Workspace{}).Where("id = ?", alpha.Tenant.ID).Update("plan_expires_at", time.Now().Add(-time.Minute)).Error)
		var listed struct {
			MaxWorkspaces  int `json:"max_workspaces"`
			WorkspaceCount int `json:"workspace_count"`
		}
		require.Equal(t, http.StatusOK, alice.request(t, http.MethodGet, "/platform/api/tenants", nil, &listed).Code)
		assert.Equal(t, 1, listed.MaxWorkspaces)
		assert.Equal(t, 2, listed.WorkspaceCount, "existing workspaces survive a downgrade")
		assert.Equal(t, http.StatusForbidden, alice.request(t, http.MethodGet, "/t/phase2-alpha/api/status", nil, nil).Code)
		batch := createPlatformCodes(t, admin, 3, 1, 1, 1)
		require.Equal(t, http.StatusOK, alice.request(t, http.MethodPost, "/platform/api/redeem", map[string]any{"tenant_id": alpha.Tenant.ID, "code": batch.Codes[0]}, nil).Code)
		assert.Equal(t, http.StatusOK, alice.request(t, http.MethodGet, "/t/phase2-alpha/api/status", nil, nil).Code)
		view.Capabilities.MaxWorkspaces = 0
		assert.Equal(t, http.StatusBadRequest, admin.request(t, http.MethodPost, "/platform/api/admin/plans/3", view, nil).Code)
	})

	t.Run("disabled roles and forced password changes invalidate platform sessions", func(t *testing.T) {
		resetPlatformAuthAttempts(t)
		path := fmt.Sprintf("/platform/api/admin/users/%d", bobUser.ID)
		require.Equal(t, http.StatusOK, admin.request(t, http.MethodPost, path, map[string]string{"action": "disable"}, nil).Code)
		assert.Equal(t, http.StatusUnauthorized, bob.request(t, http.MethodGet, "/platform/api/session", nil, nil).Code)
		failed := bob.request(t, http.MethodPost, "/platform/api/login", map[string]string{"email": *bobUser.Email, "password": bobPassword}, nil)
		assert.Equal(t, http.StatusUnauthorized, failed.Code)
		assert.Contains(t, failed.Body.String(), "invalid_credentials")
		require.Equal(t, http.StatusOK, admin.request(t, http.MethodPost, path, map[string]string{"action": "enable"}, nil).Code)
		bob.login(t, *bobUser.Email, bobPassword)
		require.Equal(t, http.StatusOK, admin.request(t, http.MethodPost, path, map[string]string{"action": "promote"}, nil).Code)
		assert.Equal(t, http.StatusUnauthorized, bob.request(t, http.MethodGet, "/platform/api/admin/users", nil, nil).Code)
		bob.login(t, *bobUser.Email, bobPassword)
		assert.Equal(t, http.StatusOK, bob.request(t, http.MethodGet, "/platform/api/admin/users", nil, nil).Code)
		require.Equal(t, http.StatusOK, admin.request(t, http.MethodPost, path, map[string]string{"action": "demote"}, nil).Code)
		assert.Equal(t, http.StatusUnauthorized, bob.request(t, http.MethodGet, "/platform/api/admin/users", nil, nil).Code)
		bob.login(t, *bobUser.Email, bobPassword)
		require.Equal(t, http.StatusOK, admin.request(t, http.MethodPost, path, map[string]string{"action": "require_password_change"}, nil).Code)
		assert.Equal(t, http.StatusUnauthorized, bob.request(t, http.MethodGet, "/platform/api/session", nil, nil).Code)
		bob.login(t, *bobUser.Email, bobPassword)
		for _, endpoint := range []string{"/tenants", "/admin/users"} {
			assert.Equal(t, http.StatusForbidden, bob.request(t, http.MethodGet, "/platform/api"+endpoint, nil, nil).Code)
		}
		assert.Equal(t, http.StatusForbidden, bob.request(t, http.MethodPost, "/platform/api/redeem", map[string]string{}, nil).Code)
		changed := saasPassword(t)
		assert.Equal(t, http.StatusUnauthorized, bob.request(t, http.MethodPost, "/platform/api/password", map[string]string{"current_password": changed, "new_password": changed}, nil).Code)
		require.Equal(t, http.StatusOK, bob.request(t, http.MethodPost, "/platform/api/password", map[string]string{"current_password": bobPassword, "new_password": changed}, nil).Code)
		assert.Equal(t, http.StatusUnauthorized, bob.request(t, http.MethodGet, "/platform/api/session", nil, nil).Code)
		bob.login(t, *bobUser.Email, changed)
		assert.Equal(t, http.StatusOK, bob.request(t, http.MethodGet, "/platform/api/tenants", nil, nil).Code)
	})

	t.Run("last administrator protection and recent authentication are enforced", func(t *testing.T) {
		resetPlatformAuthAttempts(t)
		for _, action := range []string{"disable", "demote"} {
			assert.Equal(t, http.StatusConflict, admin.request(t, http.MethodPost, "/platform/api/admin/users/1", map[string]string{"action": action}, nil).Code)
		}
		require.NoError(t, model.DB.Model(&platform.Session{}).Where("user_id = ?", 1).Update("created_at", time.Now().Add(-6*time.Minute)).Error)
		res := admin.request(t, http.MethodPost, "/platform/api/admin/redemptions", map[string]any{}, nil)
		assert.Equal(t, http.StatusUnauthorized, res.Code)
		assert.Contains(t, res.Body.String(), "recent_login_required")
		old := *admin
		assert.Equal(t, http.StatusUnauthorized, admin.request(t, http.MethodPost, "/platform/api/reauthenticate", map[string]string{"password": saasPassword(t)}, nil).Code)
		var signed struct {
			CSRFToken string `json:"csrf_token"`
		}
		res = admin.request(t, http.MethodPost, "/platform/api/reauthenticate", map[string]string{"password": adminPassword}, &signed)
		require.Equal(t, http.StatusOK, res.Code)
		for _, cookie := range res.Result().Cookies() {
			if cookie.Name == "new_api_platform_session" {
				admin.cookie = cookie
			}
		}
		admin.csrf = signed.CSRFToken
		assert.NotEqual(t, old.csrf, admin.csrf)
		assert.NotEqual(t, old.cookie.Value, admin.cookie.Value)
		assert.Equal(t, http.StatusUnauthorized, old.request(t, http.MethodGet, "/platform/api/session", nil, nil).Code)
		forged := *admin
		forged.csrf = old.csrf
		assert.Equal(t, http.StatusForbidden, forged.request(t, http.MethodPost, "/platform/api/admin/users/1", map[string]string{"action": "demote"}, nil).Code)
		createPlatformCodes(t, admin, 1, 1, 1, 1)
		var audits []platform.Audit
		require.NoError(t, model.DB.Order("id").Find(&audits).Error)
		actions := make([]string, 0, len(audits))
		for _, event := range audits {
			actions = append(actions, event.Action)
			assert.NotContains(t, event.Details, adminPassword)
			assert.NotContains(t, event.Details, admin.cookie.Value)
			assert.NotContains(t, event.Details, admin.csrf)
		}
		for _, action := range []string{"plan.manual", "plan.redeem", "redemption.create", "redemption.disable", "plan.update", "user.disable", "user.promote", "user.require_password_change", "auth.password_changed", "auth.reauthenticate"} {
			assert.Contains(t, actions, action)
		}
	})

	t.Run("origin validation rejects forged hosts and missing source headers", func(t *testing.T) {
		for _, tc := range []struct {
			name, origin, referer, host, fetch string
			want                               int
		}{
			{"matching", saas.Origin, "", "localhost:3000", "same-origin", http.StatusOK},
			{"referer fallback", "", saas.Origin + "/platform", "localhost:3000", "same-origin", http.StatusOK},
			{"missing", "", "", "localhost:3000", "", http.StatusForbidden},
			{"foreign", "https://attacker.test", "", "attacker.test", "", http.StatusForbidden},
			{"suffix", "http://localhost:3000.attacker.test", "", "localhost:3000", "", http.StatusForbidden},
			{"cross-site", saas.Origin, "", "localhost:3000", "cross-site", http.StatusForbidden},
		} {
			t.Run(tc.name, func(t *testing.T) {
				req := httptest.NewRequest(http.MethodPost, "/platform/api/logout", nil)
				req.Host = tc.host
				req.Header.Set("X-Requested-With", "NewAPIPlatform")
				if tc.origin != "" {
					req.Header.Set("Origin", tc.origin)
				}
				if tc.referer != "" {
					req.Header.Set("Referer", tc.referer)
				}
				if tc.fetch != "" {
					req.Header.Set("Sec-Fetch-Site", tc.fetch)
				}
				// Exercise browser security directly without ending the fixture's session.
				r := httptest.NewRecorder()
				context, _ := gin.CreateTestContext(r)
				context.Request = req
				saas.BrowserSecurity(context)
				assert.Equal(t, tc.want, r.Code)
			})
		}
	})

	t.Run("redemption attempts are bounded per account", func(t *testing.T) {
		resetPlatformAuthAttempts(t)
		key := sha256.Sum256([]byte("account:redeem:" + fmt.Sprint(aliceUser.ID)))
		require.NoError(t, model.DB.Create(&platform.AuthAttempt{Key: hex.EncodeToString(key[:]), Window: time.Now().Unix() / 900, Attempts: 20}).Error)
		res := alice.request(t, http.MethodPost, "/platform/api/redeem", map[string]string{}, nil)
		assert.Equal(t, http.StatusTooManyRequests, res.Code)
		assert.Equal(t, "900", res.Header().Get("Retry-After"))
	})
}
