package tenant

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNormalizeHost(t *testing.T) {
	host, ok := NormalizeHost("Alpha.Example.TEST:443")
	assert.True(t, ok)
	assert.Equal(t, "alpha.example.test", host)
	_, ok = NormalizeHost("127.0.0.1")
	assert.False(t, ok)
	_, ok = NormalizeHost("https://example.test")
	assert.False(t, ok)
}

func TestWildcardHost(t *testing.T) {
	host, ok := WildcardHost("acme", "example.test")
	assert.True(t, ok)
	assert.Equal(t, "acme.example.test", host)
	assert.Equal(t, "acme", PrefixForDomain(host, "example.test"))
	assert.Empty(t, PrefixForDomain("example.test", "example.test"))
	_, ok = WildcardHost("Acme", "example.test")
	assert.False(t, ok)
	assert.Equal(t, "_newapi-verify.api.example.test", VerifyTXTName("api.example.test"))
}
