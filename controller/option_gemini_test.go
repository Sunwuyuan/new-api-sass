package controller

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	testtenant "github.com/QuantumNous/new-api/internal/testtenant"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUpdateOptionRejectsInvalidGeminiSafetyThreshold(t *testing.T) {
	response := httptest.NewRecorder()
	context, _ := testtenant.CreateTestContext(response)
	context.Request = testtenant.NewRequest(
		http.MethodPut,
		"/api/option/",
		strings.NewReader(`{"key":"gemini.safety_settings","value":"{\"default\":\"BLOCK_SOME\"}"}`),
	)

	UpdateOption(context)

	assert.Equal(t, http.StatusOK, response.Code)
	var payload struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
	}
	require.NoError(t, common.Unmarshal(response.Body.Bytes(), &payload))
	assert.False(t, payload.Success)
	assert.Contains(t, payload.Message, "BLOCK_SOME")
}
