package model_setting

import (
	"bytes"
	"testing"

	testtenant "github.com/QuantumNous/new-api/internal/testtenant"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestShouldPreserveThinkingSuffixExactAndRegex(t *testing.T) {
	settings := GetGlobalSettings(testtenant.Context())
	original := append([]string(nil), settings.ThinkingModelBlacklist...)
	t.Cleanup(func() { settings.ThinkingModelBlacklist = original })

	assert.True(t, ShouldPreserveThinkingSuffix(testtenant.Context(), "kimi-k2-thinking"))
	assert.True(t, ShouldPreserveThinkingSuffix(testtenant.Context(), "moonshotai/kimi-k2-thinking"))
	assert.False(t, ShouldPreserveThinkingSuffix(testtenant.Context(), "m@sha256:abc"))

	settings.ThinkingModelBlacklist = []string{
		"kimi-k2-thinking",
		"re:[",
		"re:",
		"re:.*@sha256:.*",
	}

	var logged bytes.Buffer
	previous := gin.DefaultErrorWriter
	gin.DefaultErrorWriter = &logged
	t.Cleanup(func() { gin.DefaultErrorWriter = previous })

	assert.True(t, ShouldPreserveThinkingSuffix(testtenant.Context(), "kimi-k2-thinking"))
	assert.True(t, ShouldPreserveThinkingSuffix(testtenant.Context(), "m@sha256:abc"))
	assert.False(t, ShouldPreserveThinkingSuffix(testtenant.Context(), "m@sha256"))
	assert.False(t, ShouldPreserveThinkingSuffix(testtenant.Context(), "qwen3-max@thinking:on"))
	require.Contains(t, logged.String(), `invalid thinking_model_blacklist regex "re:["`)
	require.Contains(t, logged.String(), `invalid thinking_model_blacklist regex "re:"`)

	settings.ThinkingModelBlacklist = []string{"re:^beta@"}
	assert.False(t, ShouldPreserveThinkingSuffix(testtenant.Context(), "m@sha256:abc"))
	assert.True(t, ShouldPreserveThinkingSuffix(testtenant.Context(), "beta@sha256:abc"))
	assert.False(t, ShouldPreserveThinkingSuffix(testtenant.Context(), "alpha@sha256:abc"))
}
