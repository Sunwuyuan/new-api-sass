package router

import (
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/platform"
	"github.com/QuantumNous/new-api/tenant"
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
	// Keep callback codes out of resource referrers even when an edge proxy
	// replaces the response header with its own default policy.
	platformPage := []byte(strings.Replace(string(assets.IndexPage), "<head>", `<head><meta name="referrer" content="no-referrer" />`, 1))
	outer.NoRoute(static.Serve("/", frontendFS), func(c *gin.Context) {
		path := c.Request.URL.Path
		if path == "/" || strings.HasPrefix(path, "/platform") {
			c.Header("Cache-Control", "no-store")
			c.Header("Referrer-Policy", "no-referrer")
			c.Header("X-Frame-Options", "DENY")
			c.Header("Content-Security-Policy", "frame-ancestors 'none'")
			if strings.HasPrefix(path, "/platform/oauth/") && c.Request.URL.RawQuery != "" {
				// Edge-injected Speculation-Rules can fetch before HTML meta policies
				// take effect. Move callback parameters to a fragment before serving
				// any document: browsers never send fragments in resource referrers.
				c.Redirect(http.StatusSeeOther, c.Request.URL.EscapedPath()+"#"+c.Request.URL.RawQuery)
				return
			}
			c.Data(http.StatusOK, "text/html; charset=utf-8", platformPage)
			return
		}
		if slug := tenant.SlugFromPath(path); slug != "" && path == "/t/"+slug {
			c.Params = gin.Params{{Key: "slug", Value: slug}, {Key: "path", Value: "/"}}
			saas.Resolve(workspace)(c)
			return
		}
		if slug := saas.WorkspaceSlugFromRequest(c); slug != "" && !tenant.IsBrowserDocument(c.Request) {
			c.Params = gin.Params{{Key: "slug", Value: slug}, {Key: "path", Value: path}}
			saas.Resolve(workspace)(c)
			return
		}
		c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"success": false, "code": "tenant_context_required"})
	})
}
