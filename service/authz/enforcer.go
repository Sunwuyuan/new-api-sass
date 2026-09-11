package authz

import context "context"

import (
	"fmt"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/casbin/casbin/v2"
	casbinmodel "github.com/casbin/casbin/v2/model"
	"gorm.io/gorm"
)

var (
	enforcerMu sync.RWMutex
	enforcer   *casbin.SyncedEnforcer
)

const modelText = `
[request_definition]
r = sub, obj, act

[policy_definition]
p = sub, obj, act, eft

[policy_effect]
e = some(where (p.eft == allow))

[matchers]
m = r.sub == p.sub && r.obj == p.obj && r.act == p.act && p.eft == "allow"
`

func Init(db *gorm.DB) error {
	if common.IsMasterNode {
		if err := seedBuiltInRoles(db); err != nil {
			return err
		}
		if err := resetBuiltInRolePolicies(db); err != nil {
			return err
		}
	}

	m, err := casbinmodel.NewModelFromString(modelText)
	if err != nil {
		return err
	}
	e, err := casbin.NewSyncedEnforcer(m, newGormAdapter(db))
	if err != nil {
		return err
	}
	e.EnableAutoSave(true)

	TenantState(db.Statement.Context).enforcerMu.Lock()
	TenantState(db.Statement.Context).enforcer = e
	TenantState(db.Statement.Context).enforcerMu.Unlock()

	if !common.IsMasterNode {
		return nil
	}
	return seedDefaultPolicies(db.Statement.Context)
}

func currentEnforcer(tenantCtx context.Context) *casbin.SyncedEnforcer {
	TenantState(tenantCtx).enforcerMu.RLock()
	defer TenantState(tenantCtx).enforcerMu.RUnlock()
	return TenantState(tenantCtx).enforcer
}

func ReloadPolicy(tenantCtx context.Context) error {
	TenantState(tenantCtx).enforcerMu.Lock()
	defer TenantState(tenantCtx).enforcerMu.Unlock()
	if TenantState(tenantCtx).enforcer == nil {
		return fmt.Errorf("authz enforcer is not initialized")
	}
	return TenantState(tenantCtx).enforcer.LoadPolicy()
}

// StartPolicySync periodically reloads the authorization policy from the database.
// The enforcer keeps an in-memory snapshot, and permission changes are written
// straight to the DB (see SetUserPermissionsInTx) with only the local node's
// snapshot refreshed afterwards. Without this loop other instances in a
// multi-node deployment would keep serving stale permissions (including not
// honoring a revoked grant) until restart. Mirrors model.SyncOptions polling.
func StartPolicySync(tenantCtx context.Context, frequency int) {
	if frequency <= 0 {
		return
	}
	for {
		time.Sleep(time.Duration(frequency) * time.Second)
		if err := ReloadPolicy(tenantCtx); err != nil {
			common.SysError("failed to reload authz policy: " + err.Error())
		}
	}
}
