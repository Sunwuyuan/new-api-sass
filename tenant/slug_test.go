package tenant

import (
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSlugFromPath(t *testing.T) {
	assert.Equal(t, "alpha", SlugFromPath("/t/alpha/dashboard"))
	assert.Equal(t, "alpha", SlugFromPath("/t/alpha"))
	assert.Equal(t, "alpha", SlugFromPath("/t/alpha/api/status?x=1"))
	assert.Empty(t, SlugFromPath("/platform"))
	assert.Empty(t, SlugFromPath("/t/Alpha/dashboard"))
	assert.Empty(t, SlugFromHeader("alpha/beta"))
	assert.Equal(t, "alpha", SlugFromHeader(" alpha "))
	assert.Equal(t, "/api/status", GatewayPath("api/status"))
	assert.Equal(t, "/api/status", GatewayPath("/api/status"))
}

func TestAllowReferer(t *testing.T) {
	req, err := http.NewRequest(http.MethodGet, "http://localhost:3000/api/status", nil)
	require.NoError(t, err)
	req.Host = "localhost:3000"
	ref, err := url.Parse("http://localhost:3000/t/alpha/dashboard")
	require.NoError(t, err)
	assert.True(t, AllowReferer(req, ref, "http://localhost:3000"))

	evil, err := url.Parse("https://evil.test/t/alpha/dashboard")
	require.NoError(t, err)
	assert.False(t, AllowReferer(req, evil, "http://localhost:3000"))

	proxied, err := http.NewRequest(http.MethodGet, "http://localhost:3000/api/status", nil)
	require.NoError(t, err)
	proxied.Host = "localhost:3000"
	proxied.Header.Set("X-Forwarded-Host", "localhost:5173")
	dev, err := url.Parse("http://localhost:5173/t/alpha/dashboard")
	require.NoError(t, err)
	assert.True(t, AllowReferer(proxied, dev, "http://localhost:3000"))
}

func TestIsBrowserDocument(t *testing.T) {
	page, err := http.NewRequest(http.MethodGet, "/dashboard", nil)
	require.NoError(t, err)
	page.Header.Set("Accept", "text/html")
	assert.True(t, IsBrowserDocument(page))

	api, err := http.NewRequest(http.MethodGet, "/api/status", nil)
	require.NoError(t, err)
	api.Header.Set("Accept", "application/json")
	assert.False(t, IsBrowserDocument(api))
}
