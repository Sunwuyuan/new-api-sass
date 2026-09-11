package system_setting

import context "context"

var ServerAddress = "http://localhost:3000"
var TaskPublicAddress = ""
var WorkerUrl = ""
var WorkerValidKey = ""
var WorkerAllowHttpImageRequestEnabled = false

func EnableWorker(tenantCtx context.Context) bool {
	return TenantState(tenantCtx).WorkerUrl != ""
}
