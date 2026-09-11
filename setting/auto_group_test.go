package setting

import (
	"fmt"
	"testing"

	testtenant "github.com/QuantumNous/new-api/internal/testtenant"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUpdateMaxTokenAutoGroupsAcceptsAnyPositiveInteger(t *testing.T) {
	original := GetMaxTokenAutoGroups(testtenant.Context())
	t.Cleanup(func() {
		require.NoError(t, UpdateMaxTokenAutoGroups(testtenant.Context(), fmt.Sprintf("%d", original)))
	})

	require.NoError(t, UpdateMaxTokenAutoGroups(testtenant.Context(), "123456"))
	assert.Equal(t, 123456, GetMaxTokenAutoGroups(testtenant.Context()))
}

func TestUpdateMaxTokenAutoGroupsRejectsInvalidValuesWithoutChangingState(t *testing.T) {
	original := GetMaxTokenAutoGroups(testtenant.Context())
	for _, value := range []string{"", "0", "-1", "1.5", "not-a-number"} {
		t.Run(value, func(t *testing.T) {
			assert.Error(t, UpdateMaxTokenAutoGroups(testtenant.Context(), value))
			assert.Equal(t, original, GetMaxTokenAutoGroups(testtenant.Context()))
		})
	}
}
