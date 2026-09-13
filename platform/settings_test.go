package platform

import (
	"net/http"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPlatformAdminSettingsSave(t *testing.T) {
	f := newAuthFixture(t)
	admin := f.browser()
	require.Equal(t, http.StatusOK, admin.request(t, http.MethodPost, "/login", credentials{"admin@example.test", os.Getenv("PLATFORM_ADMIN_PASSWORD")}, nil).Code)
	payload := map[string]any{
		"auth": map[string]any{
			"registration":       false,
			"password_login":     true,
			"oauth_registration": true,
			"passkey":            false,
			"providers": []map[string]any{
				{"slug": "github", "name": "GitHub", "enabled": false, "client_id": "", "issuer": "", "configured": false, "client_secret": ""},
			},
		},
		"mail": map[string]any{
			"enabled": false, "from": "", "from_name": "New API SaaS",
			"base_url": "https://amail-service.192325.xyz", "provider_id": "auto",
			"api_key": "", "email_verification": false, "notifications": false,
		},
	}
	res := admin.request(t, http.MethodPost, "/admin/settings", payload, nil)
	require.Equal(t, http.StatusOK, res.Code, res.Body.String())
	require.Equal(t, http.StatusOK, admin.request(t, http.MethodPost, "/admin/settings", payload, nil).Code, "repeat save must upsert")
	var settings struct {
		Auth struct {
			Registration bool `json:"registration"`
		}
	}
	require.Equal(t, http.StatusOK, admin.request(t, http.MethodGet, "/admin/settings", nil, &settings).Code)
	assert.False(t, settings.Auth.Registration)
}
