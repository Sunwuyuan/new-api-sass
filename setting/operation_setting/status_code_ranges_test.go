package operation_setting

import (
	"testing"

	testtenant "github.com/QuantumNous/new-api/internal/testtenant"
	"github.com/stretchr/testify/require"
)

func TestParseHTTPStatusCodeRanges_CommaSeparated(t *testing.T) {
	ranges, err := ParseHTTPStatusCodeRanges("401,403,500-599")
	require.NoError(t, err)
	require.Equal(t, []StatusCodeRange{
		{Start: 401, End: 401},
		{Start: 403, End: 403},
		{Start: 500, End: 599},
	}, ranges)
}

func TestParseHTTPStatusCodeRanges_MergeAndNormalize(t *testing.T) {
	ranges, err := ParseHTTPStatusCodeRanges("500-505,504,401,403,402")
	require.NoError(t, err)
	require.Equal(t, []StatusCodeRange{
		{Start: 401, End: 403},
		{Start: 500, End: 505},
	}, ranges)
}

func TestParseHTTPStatusCodeRanges_Invalid(t *testing.T) {
	_, err := ParseHTTPStatusCodeRanges("99,600,foo,500-400,500-")
	require.Error(t, err)
}

func TestParseHTTPStatusCodeRanges_NoComma_IsInvalid(t *testing.T) {
	_, err := ParseHTTPStatusCodeRanges("401 403")
	require.Error(t, err)
}

func TestShouldDisableByStatusCode(t *testing.T) {
	orig := TenantState(testtenant.Context()).AutomaticDisableStatusCodeRanges
	t.Cleanup(func() { TenantState(testtenant.Context()).AutomaticDisableStatusCodeRanges = orig })

	TenantState(testtenant.Context()).AutomaticDisableStatusCodeRanges = []StatusCodeRange{
		{Start: 401, End: 403},
		{Start: 500, End: 599},
	}

	require.True(t, ShouldDisableByStatusCode(testtenant.Context(), 401))
	require.True(t, ShouldDisableByStatusCode(testtenant.Context(), 403))
	require.False(t, ShouldDisableByStatusCode(testtenant.Context(), 404))
	require.True(t, ShouldDisableByStatusCode(testtenant.Context(), 500))
	require.False(t, ShouldDisableByStatusCode(testtenant.Context(), 200))
}

func TestShouldRetryByStatusCode(t *testing.T) {
	orig := TenantState(testtenant.Context()).AutomaticRetryStatusCodeRanges
	t.Cleanup(func() { TenantState(testtenant.Context()).AutomaticRetryStatusCodeRanges = orig })

	TenantState(testtenant.Context()).AutomaticRetryStatusCodeRanges = []StatusCodeRange{
		{Start: 429, End: 429},
		{Start: 500, End: 599},
	}

	require.True(t, ShouldRetryByStatusCode(testtenant.Context(), 429))
	require.True(t, ShouldRetryByStatusCode(testtenant.Context(), 500))
	require.False(t, ShouldRetryByStatusCode(testtenant.Context(), 504))
	require.False(t, ShouldRetryByStatusCode(testtenant.Context(), 524))
	require.False(t, ShouldRetryByStatusCode(testtenant.Context(), 400))
	require.False(t, ShouldRetryByStatusCode(testtenant.Context(), 200))
}

func TestShouldRetryByStatusCode_DefaultMatchesLegacyBehavior(t *testing.T) {
	require.False(t, ShouldRetryByStatusCode(testtenant.Context(), 200))
	require.False(t, ShouldRetryByStatusCode(testtenant.Context(), 400))
	require.True(t, ShouldRetryByStatusCode(testtenant.Context(), 401))
	require.False(t, ShouldRetryByStatusCode(testtenant.Context(), 408))
	require.True(t, ShouldRetryByStatusCode(testtenant.Context(), 429))
	require.True(t, ShouldRetryByStatusCode(testtenant.Context(), 500))
	require.False(t, ShouldRetryByStatusCode(testtenant.Context(), 504))
	require.False(t, ShouldRetryByStatusCode(testtenant.Context(), 524))
	require.True(t, ShouldRetryByStatusCode(testtenant.Context(), 599))
}

func TestIsAlwaysSkipRetryStatusCode(t *testing.T) {
	require.True(t, IsAlwaysSkipRetryStatusCode(504))
	require.True(t, IsAlwaysSkipRetryStatusCode(524))
	require.False(t, IsAlwaysSkipRetryStatusCode(500))
}
