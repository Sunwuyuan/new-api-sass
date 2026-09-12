package router

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/tenant"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestTenantFrontendPathKeepsWorkspacePrefix(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)

	context.Request = httptest.NewRequest(http.MethodGet, "/dashboard?next=1", nil)
	assert.Equal(t, "/dashboard?next=1", tenantFrontendPath(context))

	context.Request = context.Request.WithContext(tenant.WithContext(context.Request.Context(), tenant.Identity{ID: 1, Slug: "alpha"}))
	assert.Equal(t, "/t/alpha/dashboard?next=1", tenantFrontendPath(context))

	prefixed := httptest.NewRequest(http.MethodGet, "/t/alpha/dashboard", nil)
	context.Request = prefixed.WithContext(context.Request.Context())
	assert.Equal(t, "/t/alpha/dashboard", tenantFrontendPath(context))
}
