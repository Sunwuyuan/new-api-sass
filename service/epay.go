package service

import context "context"

import (
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/setting/system_setting"
)

func GetCallbackAddress(tenantCtx context.Context) string {
	if operation_setting.TenantState(tenantCtx).CustomCallbackAddress == "" {
		return system_setting.TenantState(tenantCtx).ServerAddress
	}
	return operation_setting.TenantState(tenantCtx).CustomCallbackAddress
}
