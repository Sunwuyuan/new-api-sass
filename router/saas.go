package router

import (
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/platform"
	"github.com/gin-contrib/static"
	"github.com/gin-gonic/gin"
)

func SetSaaSRouter(outer *gin.Engine, saas *platform.Server, assets WebAssets) {
	outer.ContextWithFallback = true
	workspace := gin.New()
	workspace.ContextWithFallback = true
	workspace.Use(gin.Recovery(), middleware.RequestId(), middleware.I18n(), middleware.Version())
	if err := middleware.ConfigureTrustedProxies(workspace); err != nil {
		panic(err)
	}
	workspace.POST("/api/saas/activate", saas.BrowserSecurity, saas.ActivateRoot)
	SetRouter(workspace, assets)
	saas.Routes(outer)
	outer.Any("/t/:slug/*path", saas.Resolve(workspace))
	frontendFS := common.EmbedFolder(assets.BuildFS, "web/dist")
	outer.NoRoute(static.Serve("/", frontendFS), func(c *gin.Context) {
		path := c.Request.URL.Path
		if path != "/" && !strings.HasPrefix(path, "/platform") {
			c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"success": false, "code": "tenant_context_required"})
			return
		}
		c.Header("Cache-Control", "no-store")
		c.Data(http.StatusOK, "text/html; charset=utf-8", assets.IndexPage)
	})
}
