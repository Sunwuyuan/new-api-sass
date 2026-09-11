// Package testtenant supplies an explicit workspace to legacy gateway fixtures.
// Production code has no default tenant or unscoped fallback.
package testtenant

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"

	"github.com/QuantumNous/new-api/tenant"
	"github.com/gin-gonic/gin"
)

func Context() context.Context {
	return tenant.WithContext(context.Background(), tenant.Identity{ID: 1, Slug: "test"})
}

func NewRequest(method, target string, body io.Reader) *http.Request {
	return httptest.NewRequestWithContext(Context(), method, target, body)
}

func HTTPRequest(method, target string, body io.Reader) (*http.Request, error) {
	return http.NewRequestWithContext(Context(), method, target, body)
}

func Middleware(c *gin.Context) {
	c.Request = c.Request.WithContext(tenant.WithContext(c.Request.Context(), tenant.Identity{ID: 1, Slug: "test"}))
	c.Next()
}

func NewRouter() *gin.Engine {
	router := gin.New()
	router.ContextWithFallback = true
	router.Use(Middleware)
	return router
}

func DefaultRouter() *gin.Engine {
	router := gin.Default()
	router.ContextWithFallback = true
	router.Use(Middleware)
	return router
}

func CreateTestContext(w http.ResponseWriter) (*gin.Context, *gin.Engine) {
	c, router := gin.CreateTestContext(w)
	router.ContextWithFallback = true
	router.Use(Middleware)
	c.Request = NewRequest(http.MethodGet, "/", nil)
	return c, router
}

func GinContext() *gin.Context {
	c, _ := CreateTestContext(httptest.NewRecorder())
	return c
}
