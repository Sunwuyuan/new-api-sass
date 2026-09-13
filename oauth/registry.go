package oauth

import context "context"

import (
	"fmt"
	"maps"
	"sync"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
)

var (
	providers = make(map[string]Provider)
	mu        sync.RWMutex
	// customProviderSlugs tracks which providers are custom (can be unregistered)
	customProviderSlugs     = make(map[string]bool)
	customProviderConflicts = make(map[string]bool)
)

// Register registers an OAuth provider with the given name
func Register(tenantCtx context.Context, name string, provider Provider) {
	TenantState(tenantCtx).mu.Lock()
	defer TenantState(tenantCtx).mu.Unlock()
	TenantState(tenantCtx).providers[name] = provider
}

// RegisterCustom registers a custom OAuth provider (can be unregistered later)
func RegisterCustom(tenantCtx context.Context, name string, provider Provider) error {
	TenantState(tenantCtx).mu.Lock()
	defer TenantState(tenantCtx).mu.Unlock()
	if TenantState(tenantCtx).providers[name] != nil && !TenantState(tenantCtx).customProviderSlugs[name] {
		TenantState(tenantCtx).customProviderConflicts[name] = true
		return fmt.Errorf("custom OAuth provider %q conflicts with a built-in provider; rename the custom provider", name)
	}
	TenantState(tenantCtx).providers[name] = provider
	TenantState(tenantCtx).customProviderSlugs[name] = true
	return nil
}

func HasCustomProviderConflict(tenantCtx context.Context, name string) bool {
	TenantState(tenantCtx).mu.RLock()
	defer TenantState(tenantCtx).mu.RUnlock()
	return TenantState(tenantCtx).customProviderConflicts[name]
}

// Unregister removes a provider from the registry
func Unregister(tenantCtx context.Context, name string) {
	TenantState(tenantCtx).mu.Lock()
	defer TenantState(tenantCtx).mu.Unlock()
	delete(TenantState(tenantCtx).providers, name)
	delete(TenantState(tenantCtx).customProviderSlugs, name)
}

// GetProvider returns the OAuth provider for the given name
func GetProvider(tenantCtx context.Context, name string) Provider {
	TenantState(tenantCtx).mu.RLock()
	defer TenantState(tenantCtx).mu.RUnlock()
	return TenantState(tenantCtx).providers[name]
}

// GetAllProviders returns all registered OAuth providers
func GetAllProviders(tenantCtx context.Context) map[string]Provider {
	TenantState(tenantCtx).mu.RLock()
	defer TenantState(tenantCtx).mu.RUnlock()
	result := make(map[string]Provider, len(TenantState(tenantCtx).providers))
	maps.Copy(result, TenantState(tenantCtx).providers)
	return result
}

// GetEnabledCustomProviders returns all enabled custom OAuth providers
func GetEnabledCustomProviders(tenantCtx context.Context) []*GenericOAuthProvider {
	TenantState(tenantCtx).mu.RLock()
	defer TenantState(tenantCtx).mu.RUnlock()
	var result []*GenericOAuthProvider
	for name, provider := range TenantState(tenantCtx).providers {
		if TenantState(tenantCtx).customProviderSlugs[name] {
			if gp, ok := provider.(*GenericOAuthProvider); ok && gp.IsEnabled(tenantCtx) {
				result = append(result, gp)
			}
		}
	}
	return result
}

// IsProviderRegistered checks if a provider is registered
func IsProviderRegistered(tenantCtx context.Context, name string) bool {
	TenantState(tenantCtx).mu.RLock()
	defer TenantState(tenantCtx).mu.RUnlock()
	_, ok := TenantState(tenantCtx).providers[name]
	return ok
}

// IsCustomProvider checks if a provider is a custom provider
func IsCustomProvider(tenantCtx context.Context, name string) bool {
	TenantState(tenantCtx).mu.RLock()
	defer TenantState(tenantCtx).mu.RUnlock()
	return TenantState(tenantCtx).customProviderSlugs[name]
}

// LoadCustomProviders loads all custom OAuth providers from the database
func LoadCustomProviders(tenantCtx context.Context) error {
	// First, unregister all existing custom providers
	TenantState(tenantCtx).mu.Lock()
	for name := range TenantState(tenantCtx).customProviderSlugs {
		delete(TenantState(tenantCtx).providers, name)
	}
	TenantState(tenantCtx).customProviderSlugs = make(map[string]bool)
	TenantState(tenantCtx).customProviderConflicts = make(map[string]bool)
	TenantState(tenantCtx).mu.Unlock()

	// Load all custom providers from database
	customProviders, err := model.GetAllCustomOAuthProviders(tenantCtx)
	if err != nil {
		common.SysError("Failed to load custom OAuth providers: " + err.Error())
		return err
	}

	// Register each custom provider
	var conflict error
	for _, config := range customProviders {
		provider := NewGenericOAuthProvider(config)
		if err := RegisterCustom(tenantCtx, config.Slug, provider); err != nil {
			common.SysError(err.Error())
			conflict = err
			continue
		}
		common.SysLog("Loaded custom OAuth provider: " + config.Name + " (" + config.Slug + ")")
	}

	common.SysLog(fmt.Sprintf("Loaded %d custom OAuth providers", len(customProviders)))
	return conflict
}

// ReloadCustomProviders reloads all custom OAuth providers from the database
func ReloadCustomProviders(tenantCtx context.Context) error {
	return LoadCustomProviders(tenantCtx)
}

// RegisterOrUpdateCustomProvider registers or updates a single custom provider
func RegisterOrUpdateCustomProvider(tenantCtx context.Context, config *model.CustomOAuthProvider) {
	provider := NewGenericOAuthProvider(config)
	if err := RegisterCustom(tenantCtx, config.Slug, provider); err != nil {
		common.SysError(err.Error())
	}
}

// UnregisterCustomProvider unregisters a custom provider by slug
func UnregisterCustomProvider(tenantCtx context.Context, slug string) {
	TenantState(tenantCtx).mu.Lock()
	defer TenantState(tenantCtx).mu.Unlock()
	if TenantState(tenantCtx).customProviderSlugs[slug] {
		delete(TenantState(tenantCtx).providers, slug)
		delete(TenantState(tenantCtx).customProviderSlugs, slug)
	}
	delete(TenantState(tenantCtx).customProviderConflicts, slug)
}

// RegisterDefault is used only by package initialization.
func RegisterDefault(name string, provider Provider) { providers[name] = provider }
