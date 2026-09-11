package model

import context "context"

import "github.com/QuantumNous/new-api/common"

// GetDBTimestamp returns a UNIX timestamp from database time.
// Falls back to application time on error.
func GetDBTimestamp(tenantCtx context.Context) int64 {
	var ts int64
	var err error
	switch {
	case common.UsingMainDatabase(common.DatabaseTypePostgreSQL):
		err = DB.WithContext(tenantCtx).Raw("SELECT EXTRACT(EPOCH FROM NOW())::bigint").Scan(&ts).Error
	case common.UsingMainDatabase(common.DatabaseTypeSQLite):
		err = DB.WithContext(tenantCtx).Raw("SELECT strftime('%s','now')").Scan(&ts).Error
	default:
		err = DB.WithContext(tenantCtx).Raw("SELECT UNIX_TIMESTAMP()").Scan(&ts).Error
	}
	if err != nil || ts <= 0 {
		return common.GetTimestamp()
	}
	return ts
}
