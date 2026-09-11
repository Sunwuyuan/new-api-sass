package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/plan"
	"github.com/QuantumNous/new-api/platform"
	"github.com/QuantumNous/new-api/router"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/tenant"
	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type saasBrowser struct {
	handler http.Handler
	cookie  *http.Cookie
	csrf    string
	bearer  string
}

func (b *saasBrowser) request(t *testing.T, method, path string, body any, output any) *httptest.ResponseRecorder {
	t.Helper()
	encoded, err := common.Marshal(body)
	require.NoError(t, err)
	req := httptest.NewRequest(method, path, bytes.NewReader(encoded))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Requested-With", "NewAPIPlatform")
	req.Header.Set("Origin", "http://localhost:3000")
	req.Header.Set("X-CSRF-Token", b.csrf)
	if b.cookie != nil {
		req.AddCookie(b.cookie)
	}
	if b.bearer != "" {
		req.Header.Set("Authorization", "Bearer "+b.bearer)
	}
	res := httptest.NewRecorder()
	b.handler.ServeHTTP(res, req)
	if output != nil {
		require.NoError(t, common.Unmarshal(res.Body.Bytes(), output))
	}
	return res
}

func (b *saasBrowser) login(t *testing.T, email, password string) {
	t.Helper()
	var result struct {
		Success   bool
		CSRFToken string `json:"csrf_token"`
	}
	res := b.request(t, http.MethodPost, "/platform/api/login", map[string]string{"email": email, "password": password}, &result)
	require.Equal(t, http.StatusOK, res.Code)
	require.True(t, result.Success)
	b.csrf = result.CSRFToken
	for _, cookie := range res.Result().Cookies() {
		if cookie.Name == "new_api_platform_session" {
			assert.True(t, cookie.HttpOnly)
			assert.Equal(t, "/platform/api", cookie.Path)
			b.cookie = cookie
		}
	}
	require.NotNil(t, b.cookie)
}

func saasPassword(t *testing.T) string {
	t.Helper()
	var data [24]byte
	_, err := rand.Read(data[:])
	require.NoError(t, err)
	return hex.EncodeToString(data[:])
}

// Run this contract against each real engine by setting SAAS_TEST_DSN and
// SAAS_TEST_LOG_DSN to empty, disposable databases. The default is real SQLite.
func TestSaaSContracts(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("SQL_DSN", os.Getenv("SAAS_TEST_DSN"))
	t.Setenv("LOG_SQL_DSN", os.Getenv("SAAS_TEST_LOG_DSN"))
	t.Setenv("PLATFORM_ORIGIN", "http://localhost:3000")
	t.Setenv("PLATFORM_ADMIN_EMAIL", "administrator@example.test")
	adminPassword := saasPassword(t)
	t.Setenv("PLATFORM_ADMIN_PASSWORD", adminPassword)
	common.SQLitePath = filepath.Join(t.TempDir(), "saas.sqlite")
	common.IsMasterNode = true
	common.RedisEnabled = false
	common.MemoryCacheEnabled = true
	common.SessionSecret = saasPassword(t)
	common.PasswordLoginEncryptionEnabled = false
	require.NoError(t, model.InitDB())
	require.NoError(t, model.InitLogDB())
	t.Cleanup(func() { require.NoError(t, model.CloseDB()) })
	saas, err := platform.New(model.DB)
	require.NoError(t, err)
	require.NoError(t, model.EnforceTenantScopes())
	require.NoError(t, i18n.Init())
	saas.InitializeTenant = initializeWorkspace
	server := gin.New()
	server.ContextWithFallback = true
	server.Use(gin.Recovery())
	router.SetSaaSRouter(server, saas, router.WebAssets{BuildFS: buildFS, IndexPage: indexPage})
	admin := &saasBrowser{handler: server}
	admin.login(t, "administrator@example.test", adminPassword)
	owner := &saasBrowser{handler: server}
	ownerPassword := saasPassword(t)
	require.Equal(t, http.StatusOK, owner.request(t, http.MethodPost, "/platform/api/register", map[string]string{"email": "owner@example.test", "password": ownerPassword}, nil).Code)
	owner.login(t, "owner@example.test", ownerPassword)
	stranger := &saasBrowser{handler: server}
	strangerPassword := saasPassword(t)
	require.Equal(t, http.StatusOK, stranger.request(t, http.MethodPost, "/platform/api/register", map[string]string{"email": "stranger@example.test", "password": strangerPassword}, nil).Code)
	stranger.login(t, "stranger@example.test", strangerPassword)

	var alpha, beta tenant.Workspace
	roots := make(map[string]*saasBrowser)
	activations := make(map[string]string)
	for _, slug := range []string{"alpha", "beta"} {
		var created struct {
			Tenant tenant.Workspace
			URL    string `json:"root_activation_url"`
		}
		require.Equal(t, http.StatusCreated, owner.request(t, http.MethodPost, "/platform/api/tenants", map[string]string{"name": slug, "slug": slug}, &created).Code)
		fragment := strings.SplitN(created.URL, "#", 2)
		require.Len(t, fragment, 2)
		values, err := url.ParseQuery(fragment[1])
		require.NoError(t, err)
		activations[slug] = values.Get("token")
		root := &saasBrowser{handler: server}
		password := saasPassword(t)
		activation := map[string]string{"token": activations[slug], "password": password}
		require.Equal(t, http.StatusOK, root.request(t, http.MethodPost, "/t/"+slug+"/api/saas/activate", activation, nil).Code)
		require.Equal(t, http.StatusBadRequest, root.request(t, http.MethodPost, "/t/"+slug+"/api/saas/activate", activation, nil).Code)
		var signed struct {
			Success bool
			Data    service.AuthBundle
		}
		login := root.request(t, http.MethodPost, "/t/"+slug+"/api/user/login", map[string]string{"username": "root", "password": password}, &signed)
		require.Equal(t, http.StatusOK, login.Code)
		require.True(t, signed.Success)
		root.bearer = signed.Data.AccessToken
		require.NotEmpty(t, root.bearer)
		for _, cookie := range login.Result().Cookies() {
			assert.True(t, strings.HasPrefix(cookie.Path, "/t/"+slug+"/"))
			if cookie.Name == service.RefreshCookieName {
				root.cookie = cookie
			}
		}
		roots[slug] = root
		if slug == "alpha" {
			alpha = created.Tenant
		} else {
			beta = created.Tenant
		}
	}
	alphaCtx := tenant.WithContext(context.Background(), tenant.Identity{ID: alpha.ID, Slug: alpha.Slug})
	betaCtx := tenant.WithContext(context.Background(), tenant.Identity{ID: beta.ID, Slug: beta.Slug})

	t.Run("platform ownership and credential isolation", func(t *testing.T) {
		var visible struct{ Tenants []any }
		require.Equal(t, http.StatusOK, stranger.request(t, http.MethodGet, "/platform/api/tenants", nil, &visible).Code)
		assert.Empty(t, visible.Tenants)
		assert.Equal(t, http.StatusForbidden, owner.request(t, http.MethodPost, fmt.Sprintf("/platform/api/admin/tenants/%d/plan", alpha.ID), map[string]int{"plan_id": 2, "months": 1}, nil).Code)
		assert.Equal(t, http.StatusUnauthorized, roots["alpha"].request(t, http.MethodGet, "/t/beta/api/user/self", nil, nil).Code)
		assert.Equal(t, http.StatusUnauthorized, owner.request(t, http.MethodGet, "/t/alpha/api/user/self", nil, nil).Code)
		assert.Equal(t, http.StatusNotFound, owner.request(t, http.MethodGet, "/api/user/self", nil, nil).Code)
		forged := *owner
		forged.csrf = "wrong"
		assert.Equal(t, http.StatusForbidden, forged.request(t, http.MethodPost, "/platform/api/tenants", map[string]string{"name": "csrf", "slug": "csrf"}, nil).Code)
		req := httptest.NewRequest(http.MethodPost, "/t/alpha/api/saas/activate", strings.NewReader(`{}`))
		rec := httptest.NewRecorder()
		server.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusForbidden, rec.Code)
	})

	t.Run("database scope and ID probes fail closed", func(t *testing.T) {
		var users []model.User
		assert.ErrorIs(t, model.DB.Find(&users).Error, tenant.ErrMissing)
		assert.Error(t, model.DB.WithContext(alphaCtx).Raw("SELECT * FROM users").Scan(&users).Error)
		a := model.Token{UserId: 1, Key: saasPassword(t), Name: "alpha-private", Status: common.TokenStatusEnabled, UnlimitedQuota: true}
		require.NoError(t, model.DB.WithContext(alphaCtx).Create(&a).Error)
		assert.Equal(t, alpha.ID, a.TenantID)
		var token model.Token
		assert.ErrorIs(t, model.DB.WithContext(betaCtx).First(&token, a.Id).Error, gorm.ErrRecordNotFound)
		changed := model.DB.WithContext(betaCtx).Model(&model.Token{}).Where("id = ?", a.Id).Update("name", "stolen")
		require.NoError(t, changed.Error)
		assert.Zero(t, changed.RowsAffected)
		removed := model.DB.WithContext(betaCtx).Delete(&model.Token{}, a.Id)
		require.NoError(t, removed.Error)
		assert.Zero(t, removed.RowsAffected)
		assert.Equal(t, http.StatusNotFound, roots["beta"].request(t, http.MethodGet, fmt.Sprintf("/t/beta/api/token/%d", a.Id), nil, nil).Code)
		assert.Equal(t, http.StatusNotFound, roots["beta"].request(t, http.MethodDelete, fmt.Sprintf("/t/beta/api/token/%d", a.Id), nil, nil).Code)
		assert.ErrorIs(t, model.DB.WithContext(alphaCtx).Model(&model.Token{}).Where("id = ?", a.Id).Update("tenant_id", beta.ID).Error, tenant.ErrMismatch)
		stolen := a
		stolen.TenantID = beta.ID
		stolen.Name = "stolen"
		assert.Error(t, model.DB.WithContext(betaCtx).Save(&stolen).Error)
		assert.Error(t, model.DB.WithContext(betaCtx).Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "key"}}, DoUpdates: clause.AssignmentColumns([]string{"name"})}).Create(&stolen).Error)
		var original model.Token
		require.NoError(t, model.DB.WithContext(alphaCtx).First(&original, a.Id).Error)
		assert.Equal(t, "alpha-private", original.Name)
		b := model.Token{UserId: 2, Key: a.Key, Name: "beta-private", Status: common.TokenStatusEnabled}
		require.NoError(t, model.DB.WithContext(betaCtx).Create(&b).Error)
		forgedToken := model.Token{TenantID: beta.ID, UserId: 1, Key: saasPassword(t)}
		assert.ErrorIs(t, model.DB.WithContext(alphaCtx).Create(&forgedToken).Error, tenant.ErrMismatch)
		log := model.Log{UserId: 1, Content: "alpha-private"}
		require.NoError(t, model.LOG_DB.WithContext(alphaCtx).Create(&log).Error)
		assert.ErrorIs(t, model.LOG_DB.WithContext(betaCtx).Where("content = ?", log.Content).First(&model.Log{}).Error, gorm.ErrRecordNotFound)
		selected := model.Option{Key: "selected-ownership", Value: "beta-only"}
		require.NoError(t, model.DB.WithContext(betaCtx).Select("Key", "Value").Create(&selected).Error)
		omitted := model.Option{Key: "omitted-ownership", Value: "beta-only"}
		require.NoError(t, model.DB.WithContext(betaCtx).Omit("TenantID").Create(&omitted).Error)
		var options []model.Option
		require.NoError(t, model.DB.WithContext(betaCtx).Where(map[string]any{"key": []string{selected.Key, omitted.Key}}).Find(&options).Error)
		assert.Len(t, options, 2)
		require.NoError(t, model.DB.WithContext(alphaCtx).Where(map[string]any{"key": []string{selected.Key, omitted.Key}}).Find(&options).Error)
		assert.Empty(t, options)
		require.NoError(t, model.DB.WithContext(betaCtx).Model(&model.Option{}).Create(map[string]any{"key": "map-insert", "value": "beta-only"}).Error)
		assert.ErrorIs(t, model.DB.WithContext(alphaCtx).Where(model.Option{Key: "map-insert"}).First(&model.Option{}).Error, gorm.ErrRecordNotFound)
		assert.ErrorIs(t, model.DB.WithContext(betaCtx).Model(&model.Option{}).Create(map[string]any{"tenant_id": alpha.ID, "key": "forged-map"}).Error, tenant.ErrMismatch)
	})

	t.Run("log cleanup preserves other workspaces and audit trails", func(t *testing.T) {
		cutoff := time.Now().Unix() - 3600
		for _, ctx := range []context.Context{alphaCtx, betaCtx} {
			require.NoError(t, model.LOG_DB.WithContext(ctx).Create(&model.Log{CreatedAt: cutoff - 1, Content: "old consume"}).Error)
			model.RecordAuditLogContext(ctx, nil, model.AuditLog{CreatedAt: cutoff - 1, ActorRole: common.RoleRootUser, Category: model.AuditCategoryOperation, Action: "saas_scope_check", Content: "audit retained"})
		}
		_, err := model.DeleteOldLogBatch(alphaCtx, cutoff, 100)
		require.NoError(t, err)
		var count int64
		require.NoError(t, model.LOG_DB.WithContext(alphaCtx).Model(&model.Log{}).Where("created_at < ?", cutoff).Count(&count).Error)
		assert.Zero(t, count)
		require.NoError(t, model.LOG_DB.WithContext(betaCtx).Model(&model.Log{}).Where("content = ?", "old consume").Count(&count).Error)
		assert.EqualValues(t, 1, count)
		for _, ctx := range []context.Context{alphaCtx, betaCtx} {
			require.NoError(t, model.LOG_DB.WithContext(ctx).Model(&model.AuditLog{}).Where("action = ?", "saas_scope_check").Count(&count).Error)
			assert.EqualValues(t, 1, count)
		}
	})

	t.Run("settings footer capabilities and resource caps", func(t *testing.T) {
		original := common.TenantState(alphaCtx)
		require.NoError(t, model.UpdateOption(alphaCtx, "SystemName", "Only Alpha"))
		assert.Equal(t, "alpha", original.SystemName, "in-flight requests retain an immutable settings snapshot")
		assert.Equal(t, "Only Alpha", common.TenantState(alphaCtx).SystemName)
		assert.Equal(t, "beta", common.TenantState(betaCtx).SystemName)
		assert.ErrorIs(t, model.UpdateOption(alphaCtx, "Footer", ""), plan.ErrCapability)
		assert.Equal(t, http.StatusForbidden, roots["alpha"].request(t, http.MethodPost, "/t/alpha/api/performance/gc", nil, nil).Code)
		assert.Error(t, model.UpdateOption(alphaCtx, "performance_setting.disk_cache_path", "/unavailable"))
		assert.Equal(t, http.StatusForbidden, roots["alpha"].request(t, http.MethodPut, "/t/alpha/api/option/", map[string]string{"key": "Footer", "value": ""}, nil).Code)
		var status struct {
			Data struct {
				Locked bool   `json:"platform_footer_locked"`
				Footer string `json:"platform_footer"`
			}
		}
		require.Equal(t, http.StatusOK, roots["alpha"].request(t, http.MethodGet, "/t/alpha/api/status", nil, &status).Code)
		assert.True(t, status.Data.Locked)
		assert.NotEmpty(t, status.Data.Footer)
		view, err := plan.Current(alphaCtx, model.DB)
		require.NoError(t, err)
		var count int64
		require.NoError(t, model.DB.WithContext(alphaCtx).Model(&model.Token{}).Count(&count).Error)
		for range view.Limits.Tokens - count {
			require.NoError(t, model.DB.WithContext(alphaCtx).Create(&model.Token{UserId: 1, Key: saasPassword(t)}).Error)
		}
		var limitError *tenant.HTTPError
		require.ErrorAs(t, model.DB.WithContext(alphaCtx).Create(&model.Token{UserId: 1, Key: saasPassword(t)}).Error, &limitError)
		assert.Equal(t, "tenant_resource_limit_exceeded", limitError.Code)
		require.NoError(t, model.DB.WithContext(betaCtx).Create(&model.Token{UserId: 2, Key: saasPassword(t)}).Error)
	})

	t.Run("new workspace channels relay immediately and unbilled failures do not count", func(t *testing.T) {
		var reject atomic.Bool
		upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			if reject.Load() {
				w.WriteHeader(http.StatusBadRequest)
				_, _ = w.Write([]byte(`{"error":{"message":"Rejected without charge","type":"invalid_request_error"}}`))
				return
			}
			_, _ = w.Write([]byte(`{"id":"chatcmpl-saas","object":"chat.completion","model":"gpt-4o","choices":[{"index":0,"message":{"role":"assistant","content":"Workspace upstream accepted"},"finish_reason":"stop"}],"usage":{"prompt_tokens":8,"completion_tokens":4,"total_tokens":12}}`))
		}))
		defer upstream.Close()
		var root model.User
		require.NoError(t, model.DB.WithContext(betaCtx).Where("username = ?", "root").First(&root).Error)
		require.NoError(t, model.IncreaseUserQuota(betaCtx, root.Id, 5000000, true))
		var created struct{ Success bool }
		response := roots["beta"].request(t, http.MethodPost, "/t/beta/api/channel/", map[string]any{
			"mode": "single", "channel": map[string]any{
				"type": 1, "name": "beta-upstream", "key": saasPassword(t),
				"base_url": upstream.URL, "models": "gpt-4o", "group": "default", "status": 1,
			},
		}, &created)
		require.Equal(t, http.StatusOK, response.Code)
		require.True(t, created.Success)
		token := model.Token{UserId: root.Id, Key: saasPassword(t), Name: "beta-relay", Status: common.TokenStatusEnabled, UnlimitedQuota: true, ExpiredTime: -1}
		require.NoError(t, model.DB.WithContext(betaCtx).Create(&token).Error)
		client := &saasBrowser{handler: server, bearer: token.GetFullKey()}
		request := map[string]any{"model": "gpt-4o", "messages": []map[string]string{{"role": "user", "content": "acceptance"}}, "max_tokens": 16}
		response = client.request(t, http.MethodPost, "/t/beta/v1/chat/completions", request, nil)
		require.Equal(t, http.StatusOK, response.Code, response.Body.String())
		assert.Contains(t, response.Body.String(), "Workspace upstream accepted")
		var usage plan.Usage
		month := time.Now().UTC().Format("2006-01")
		require.NoError(t, model.DB.Where("tenant_id = ? AND month = ?", beta.ID, month).First(&usage).Error)
		assert.EqualValues(t, 1, usage.Requests)
		reject.Store(true)
		response = client.request(t, http.MethodPost, "/t/beta/v1/chat/completions", request, nil)
		require.Equal(t, http.StatusBadRequest, response.Code, response.Body.String())
		require.NoError(t, model.DB.Where("tenant_id = ? AND month = ?", beta.ID, month).First(&usage).Error)
		assert.EqualValues(t, 1, usage.Requests, "an unbilled upstream failure releases its reservation")
		anonymous := &saasBrowser{handler: server}
		for _, path := range []string{"/t/beta/not-a-relay-route", "/t/beta/logo.png"} {
			assert.Equal(t, http.StatusNotFound, anonymous.request(t, http.MethodPost, path, nil, nil).Code)
		}
		require.NoError(t, model.DB.Where("tenant_id = ? AND month = ?", beta.ID, month).First(&usage).Error)
		assert.EqualValues(t, 1, usage.Requests, "frontend POSTs cannot consume a workspace's gateway allowance")
		assert.Equal(t, http.StatusUnauthorized, client.request(t, http.MethodPost, "/t/alpha/v1/chat/completions", request, nil).Code)
	})

	t.Run("monthly limit is atomic and manual upgrade recovers", func(t *testing.T) {
		view, err := plan.Current(alphaCtx, model.DB)
		require.NoError(t, err)
		now := time.Now().UTC()
		month := now.Format("2006-01")
		require.NoError(t, model.DB.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "tenant_id"}, {Name: "month"}},
			DoUpdates: clause.AssignmentColumns([]string{"requests"}),
		}).Create(&plan.Usage{TenantID: alpha.ID, Month: month, Requests: view.Limits.Requests - 1}).Error)
		type reservation struct {
			finish func(bool) error
			err    error
		}
		results := make(chan reservation, 2)
		var wg sync.WaitGroup
		for range 2 {
			wg.Go(func() {
				finish, err := plan.Reserve(alphaCtx, model.DB, view.Limits.Requests, now)
				results <- reservation{finish, err}
			})
		}
		wg.Wait()
		close(results)
		accepted, rejected := 0, 0
		for result := range results {
			if result.err != nil {
				require.ErrorIs(t, result.err, plan.ErrLimit)
				rejected++
				continue
			}
			accepted++
			require.NoError(t, result.finish(true))
		}
		assert.Equal(t, 1, accepted)
		assert.Equal(t, 1, rejected)
		assert.Equal(t, http.StatusTooManyRequests, roots["alpha"].request(t, http.MethodPost, "/t/alpha/v1/chat/completions", map[string]string{"model": "test"}, nil).Code)
		require.Equal(t, http.StatusOK, admin.request(t, http.MethodPost, fmt.Sprintf("/platform/api/admin/tenants/%d/plan", alpha.ID), map[string]int{"plan_id": 2, "months": 1}, nil).Code)
		require.NoError(t, model.UpdateOption(alphaCtx, "Footer", "<p>Custom Alpha</p>"))
		meter := gin.New()
		meter.Any("/t/:slug/*path", saas.Resolve(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("X-Test-Billed") == "true" {
				tenant.MarkBilled(r.Context())
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			if r.Header.Get("X-Test-Failed") == "true" {
				w.WriteHeader(http.StatusBadGateway)
				return
			}
			if r.Header.Get("X-Test-Empty") == "true" {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"ok":true}`))
		})))
		for _, header := range []string{"", "X-Test-Failed", "X-Test-Billed", "X-Test-Empty"} {
			req := httptest.NewRequest(http.MethodPost, "/t/alpha/v1/chat/completions", nil)
			if header != "" {
				req.Header.Set(header, "true")
			}
			res := httptest.NewRecorder()
			meter.ServeHTTP(res, req)
			assert.NotEqual(t, http.StatusTooManyRequests, res.Code)
		}
		var usage plan.Usage
		require.NoError(t, model.DB.Where("tenant_id = ? AND month = ?", alpha.ID, month).First(&usage).Error)
		assert.Equal(t, view.Limits.Requests+3, usage.Requests)
		require.Equal(t, http.StatusOK, admin.request(t, http.MethodPost, fmt.Sprintf("/platform/api/admin/tenants/%d/status", alpha.ID), map[string]string{"status": "suspended"}, nil).Code)
		assert.Equal(t, http.StatusForbidden, roots["alpha"].request(t, http.MethodGet, "/t/alpha/api/status", nil, nil).Code)
		require.Equal(t, http.StatusOK, admin.request(t, http.MethodPost, fmt.Sprintf("/platform/api/admin/tenants/%d/status", alpha.ID), map[string]string{"status": "active"}, nil).Code)
	})

	t.Run("activation expiry and platform password changes revoke sessions", func(t *testing.T) {
		var created struct {
			Tenant tenant.Workspace
			URL    string `json:"root_activation_url"`
		}
		require.Equal(t, http.StatusCreated, owner.request(t, http.MethodPost, "/platform/api/tenants", map[string]string{"name": "Expired", "slug": "expired"}, &created).Code)
		require.NoError(t, model.DB.Model(&platform.RootActivation{}).Where("tenant_id = ?", created.Tenant.ID).Update("expires_at", time.Now().Add(-time.Minute)).Error)
		parts := strings.SplitN(created.URL, "#", 2)
		require.Len(t, parts, 2)
		values, err := url.ParseQuery(parts[1])
		require.NoError(t, err)
		assert.Equal(t, http.StatusBadRequest, owner.request(t, http.MethodPost, "/t/expired/api/saas/activate", map[string]string{"token": values.Get("token"), "password": saasPassword(t)}, nil).Code)
		otherSession := &saasBrowser{handler: server}
		otherSession.login(t, "owner@example.test", ownerPassword)
		changedPassword := saasPassword(t)
		assert.Equal(t, http.StatusUnauthorized, owner.request(t, http.MethodPost, "/platform/api/password", map[string]string{"current_password": saasPassword(t), "new_password": changedPassword}, nil).Code)
		require.Equal(t, http.StatusOK, owner.request(t, http.MethodPost, "/platform/api/password", map[string]string{"current_password": ownerPassword, "new_password": changedPassword}, nil).Code)
		assert.Equal(t, http.StatusUnauthorized, otherSession.request(t, http.MethodGet, "/platform/api/session", nil, nil).Code)
		assert.Equal(t, http.StatusUnauthorized, owner.request(t, http.MethodGet, "/platform/api/session", nil, nil).Code)
		owner.login(t, "owner@example.test", changedPassword)
		require.Equal(t, http.StatusOK, owner.request(t, http.MethodPost, "/platform/api/logout", nil, nil).Code)
		assert.Equal(t, http.StatusUnauthorized, owner.request(t, http.MethodGet, "/platform/api/session", nil, nil).Code)
		require.NoError(t, model.DB.Model(&platform.Session{}).Where("user_id = ?", 1).Update("last_seen", time.Now().Add(-time.Hour)).Error)
		assert.Equal(t, http.StatusUnauthorized, admin.request(t, http.MethodGet, "/platform/api/session", nil, nil).Code)
	})
}

func TestSaaSRedisIsolation(t *testing.T) {
	address := os.Getenv("SAAS_TEST_REDIS_ADDR")
	if address == "" {
		t.Skip("set SAAS_TEST_REDIS_ADDR for real Redis acceptance")
	}
	client := redis.NewClient(&redis.Options{Addr: address, DB: 14})
	client.AddHook(tenant.RedisScope{})
	t.Cleanup(func() { require.NoError(t, client.Close()) })
	a := tenant.WithContext(context.Background(), tenant.Identity{ID: 701, Slug: "alpha"})
	b := tenant.WithContext(context.Background(), tenant.Identity{ID: 702, Slug: "beta"})
	key := "saas-test:" + saasPassword(t)
	t.Cleanup(func() { client.Del(a, key); client.Del(b, key) })
	require.NoError(t, client.Set(a, key, "alpha", time.Minute).Err())
	require.NoError(t, client.Set(b, key, "beta", time.Minute).Err())
	assert.Equal(t, "alpha", client.Get(a, key).Val())
	assert.Equal(t, "beta", client.Get(b, key).Val())
	assert.ErrorIs(t, client.Get(context.Background(), key).Err(), tenant.ErrMissing)
	assert.ErrorIs(t, client.Get(b, tenant.MustKey(a, key)).Err(), tenant.ErrMismatch)
	_, err := client.Pipelined(a, func(pipe redis.Pipeliner) error { pipe.Get(a, key); return nil })
	require.NoError(t, err)
	script := redis.NewScript(`return redis.call('GET', KEYS[1])`)
	value, err := script.Run(b, client, []string{key}).Result()
	require.NoError(t, err)
	assert.Equal(t, "beta", value)
	iterator := client.Scan(a, 0, "saas-test:*", 100).Iterator()
	var keys []string
	for iterator.Next(context.Background()) {
		keys = append(keys, iterator.Val())
	}
	require.NoError(t, iterator.Err())
	assert.Contains(t, keys, key)
	require.NoError(t, client.Del(a, key).Err())
	assert.Equal(t, "beta", client.Get(b, key).Val())
}
