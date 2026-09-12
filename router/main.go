package router

import (
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/tenant"

	"github.com/gin-gonic/gin"
)

func tenantFrontendPath(c *gin.Context) string {
	path, query, hasQuery := strings.Cut(c.Request.RequestURI, "?")
	if identity, err := tenant.FromContext(c.Request.Context()); err == nil && identity.Slug != "" {
		prefix := "/t/" + identity.Slug
		if path != prefix && !strings.HasPrefix(path, prefix+"/") && strings.HasPrefix(path, "/") {
			path = prefix + path
		}
	}
	if hasQuery {
		return path + "?" + query
	}
	return path
}

func SetRouter(router *gin.Engine, assets WebAssets) {
	SetApiRouter(router)
	SetDashboardRouter(router)
	SetRelayRouter(router)
	SetTaskPluginProtocolRouter(router)
	SetVideoRouter(router)
	SetTaskRouter(router)
	pluginDispatcher := SetPluginRouter(router)
	frontendBaseUrl := os.Getenv("FRONTEND_BASE_URL")
	if common.IsMasterNode && frontendBaseUrl != "" {
		frontendBaseUrl = ""
		common.SysLog("FRONTEND_BASE_URL is ignored on master node")
	}
	if frontendBaseUrl == "" {
		SetWebRouter(router, assets, pluginDispatcher)
	} else {
		frontendBaseUrl = strings.TrimSuffix(frontendBaseUrl, "/")
		router.NoRoute(
			pluginDispatcher,
			middleware.RouteTag("web"),
			middleware.AccessTokenAudit(),
			func(c *gin.Context) {
				c.Redirect(http.StatusMovedPermanently, fmt.Sprintf("%s%s", frontendBaseUrl, tenantFrontendPath(c)))
			},
		)
	}
}
