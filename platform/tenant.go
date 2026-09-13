package platform

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/plan"
	"github.com/QuantumNous/new-api/tenant"
	"github.com/gin-gonic/gin"
)

type initialization struct {
	sync.Mutex
	Ready bool
}

var initializedTenants tenant.Registry[*initialization]

func (s *Server) EnsureInitialized(ctx context.Context) error {
	state, err := initializedTenants.Get(ctx, func() *initialization { return &initialization{} })
	if err != nil {
		return err
	}
	state.Lock()
	defer state.Unlock()
	if state.Ready || s.InitializeTenant == nil {
		return nil
	}
	if err := s.InitializeTenant(ctx); err != nil {
		return err
	}
	state.Ready = true
	return nil
}

// Resolve serves every workspace through one shared gateway handler. Only the
// request context changes; no process, DB handle, or global current tenant is
// switched when another request arrives. The workspace is selected by Host.
func (s *Server) Resolve(gateway http.Handler) gin.HandlerFunc {
	return func(c *gin.Context) {
		workspace, ok := s.WorkspaceByHost(c)
		if !ok {
			writeError(c, http.StatusNotFound, "workspace_not_found")
			return
		}
		s.ServeWorkspace(c, workspace, c.Request.URL.Path, gateway)
	}
}

func (s *Server) ServeWorkspace(c *gin.Context, workspace tenant.Workspace, path string, gateway http.Handler) {
	now := time.Now()
	db := s.DB.WithContext(c.Request.Context())
	if err := plan.DowngradeExpired(db, &workspace, now); err != nil {
		writeError(c, http.StatusServiceUnavailable, "workspace_unavailable")
		return
	}
	view, err := plan.ForWorkspace(db, workspace, now)
	if err != nil {
		writeError(c, http.StatusForbidden, "workspace_inactive")
		return
	}
	ctx := tenant.WithContext(c.Request.Context(), tenant.Identity{ID: workspace.ID, Slug: workspace.Slug})
	if err := s.EnsureInitialized(ctx); err != nil {
		common.SysError("workspace initialization failed: " + err.Error())
		writeError(c, http.StatusServiceUnavailable, "workspace_unavailable")
		return
	}
	ctx = plan.WithContext(ctx, view)
	ctx, meter := tenant.WithMeter(ctx)
	request := c.Request.Clone(ctx)
	request.URL.Path = tenant.GatewayPath(path)
	request.URL.RawPath = ""
	request.RequestURI = request.URL.RequestURI()
	if strings.HasPrefix(request.URL.Path, "/api/performance/") || strings.HasPrefix(request.URL.Path, "/api/system-info/") {
		writeError(c, http.StatusForbidden, "platform_managed_operation")
		return
	}
	if request.URL.Path == "/api/saas/plan" && request.Method == http.MethodGet {
		c.JSON(http.StatusOK, gin.H{"success": true, "tenant": workspace, "plan": view})
		return
	}
	var complete func(bool) error
	if gatewayMutation(request) {
		complete, err = plan.Reserve(ctx, s.DB, view.Limits.Requests, time.Now())
		if err != nil {
			if errors.Is(err, plan.ErrLimit) {
				c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": gin.H{"code": "tenant_monthly_limit_exceeded", "type": "hosting_limit", "message": err.Error()}})
			} else {
				writeError(c, http.StatusServiceUnavailable, "usage_meter_unavailable")
			}
			return
		}
	}
	finished := false
	if complete != nil {
		defer func() {
			success := finished && (c.Writer.Status() == http.StatusSwitchingProtocols || c.Writer.Status() >= 200 && c.Writer.Status() < 300)
			if err := complete(success || meter.Billed()); err != nil {
				common.SysError("workspace usage rollback failed: " + err.Error())
			}
		}()
	}
	gateway.ServeHTTP(c.Writer, request)
	finished = true
}

func gatewayMutation(request *http.Request) bool {
	path := request.URL.Path
	if strings.HasSuffix(path, "/count_tokens") {
		return false
	}
	if strings.HasPrefix(path, "/api/playground/") {
		return request.Method == http.MethodPost
	}
	if strings.HasPrefix(path, "/api/") || strings.HasPrefix(path, "/assets/") || strings.HasPrefix(path, "/static/") {
		return false
	}
	return request.Method == http.MethodPost || strings.HasPrefix(path, "/v1/realtime")
}
