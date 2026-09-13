package platform

import (
	"errors"
	"net"
	"net/http"
	"strconv"
	"testing"

	"github.com/QuantumNous/new-api/tenant"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAdminWildcardDomains(t *testing.T) {
	f := newAuthFixture(t)
	admin := f.browser()
	require.Equal(t, http.StatusOK, admin.request(t, http.MethodPost, "/login", credentials{"admin@example.test", "Only a synthetic bootstrap passphrase 2026!"}, nil).Code)
	require.Equal(t, http.StatusCreated, admin.request(t, http.MethodPost, "/admin/wildcard-domains", map[string]any{"domain": "example.test", "enabled": true}, nil).Code)
	assert.Equal(t, http.StatusConflict, admin.request(t, http.MethodPost, "/admin/wildcard-domains", map[string]any{"domain": "example.test"}, nil).Code)
	var listed struct {
		Domains []tenant.WildcardDomain `json:"wildcard_domains"`
	}
	require.Equal(t, http.StatusOK, admin.request(t, http.MethodGet, "/admin/wildcard-domains", nil, &listed).Code)
	require.Len(t, listed.Domains, 1)
	assert.Equal(t, "example.test", listed.Domains[0].Domain)
	assert.True(t, listed.Domains[0].Enabled)
	id := strconv.FormatInt(listed.Domains[0].ID, 10)
	require.Equal(t, http.StatusOK, admin.request(t, http.MethodPost, "/admin/wildcard-domains/"+id, map[string]any{"enabled": false}, nil).Code)
	require.Equal(t, http.StatusOK, admin.request(t, http.MethodGet, "/wildcard-domains", nil, &listed).Code)
	assert.Empty(t, listed.Domains)
	require.Equal(t, http.StatusOK, admin.request(t, http.MethodPost, "/admin/wildcard-domains/"+id+"/delete", map[string]any{}, nil).Code)
}

func TestCustomDomainTXTVerification(t *testing.T) {
	f := newAuthFixture(t)
	workspace := tenant.Workspace{Slug: "alpha", Name: "Alpha", OwnerPlatformUserID: 1, PlanID: 1, Status: "active"}
	require.NoError(t, f.server.DB.Create(&workspace).Error)
	binding := tenant.Host{
		TenantID: workspace.ID, Kind: tenant.HostCustom, Host: "api.customer.test",
		Status: tenant.HostPending, VerificationMethod: tenant.VerifyTXT, VerificationToken: "site-token",
	}
	require.NoError(t, f.server.DB.Create(&binding).Error)
	lookupTXT = func(name string) ([]string, error) {
		if name == tenant.VerifyTXTName("api.customer.test") {
			return []string{`newapi-site-verification=site-token`}, nil
		}
		return nil, errors.New("nxdomain")
	}
	t.Cleanup(func() { lookupTXT = net.LookupTXT })
	assert.True(t, f.server.domainVerified(binding))
	lookupTXT = func(string) ([]string, error) { return []string{"other"}, nil }
	assert.False(t, f.server.domainVerified(binding))
}
