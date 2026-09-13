package controller

import context "context"

import (
	"strings"

	"github.com/QuantumNous/new-api/setting/system_setting"
)

func paymentReturnPath(tenantCtx context.Context, suffix string) string {
	base := strings.TrimRight(system_setting.TenantState(tenantCtx).ServerAddress, "/")
	return base + suffix
}
