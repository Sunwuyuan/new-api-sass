package model

import (
	"context"
	"fmt"
	"reflect"
	"slices"
	"strings"

	"github.com/QuantumNous/new-api/plan"
	"github.com/QuantumNous/new-api/tenant"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// BusinessModels is also the migration inventory. New workspace-owned tables
// must carry TenantID; the runtime callback rejects unregistered model shapes.
func BusinessModels() []any {
	return []any{
		&Channel{}, &Token{}, &User{}, &UserSession{}, &AuthFlow{},
		&ExternalIdentityClaim{}, &PasskeyCredential{}, &Option{},
		&LoginEncryptionKey{}, &Redemption{}, &Ability{}, &Log{}, &AuditLog{},
		&Midjourney{}, &TopUp{}, &QuotaData{}, &Task{}, &TaskPlugin{}, &Model{},
		&Vendor{}, &PrefillGroup{}, &Setup{}, &TwoFA{}, &TwoFABackupCode{},
		&Checkin{}, &SubscriptionOrder{}, &UserSubscription{},
		&SubscriptionPreConsumeRecord{}, &SubscriptionPlan{}, &CustomOAuthProvider{},
		&UserOAuthBinding{}, &PerfMetric{}, &SystemTask{}, &SystemTaskLock{},
		&CasbinRule{}, &AuthzRole{},
	}
}

// MigrateTenantSchema runs before scopes are installed. Existing standalone
// rows are retained under tenant 1, which the platform bootstrap explicitly
// assigns to its administrator as the imported workspace.
func MigrateTenantSchema(db *gorm.DB, models []any) error {
	for _, value := range models {
		statement := &gorm.Statement{DB: db}
		if err := statement.Parse(value); err != nil {
			return err
		}
		if !db.Migrator().HasTable(value) {
			continue
		}
		var tokenConstraints []tokenKeyUniqueConstraint
		if _, ok := value.(*Token); ok && db.Dialector.Name() == "postgres" {
			constraints, err := inspectTokenKeyUniqueConstraints(db, statement.Schema.Table)
			if err != nil {
				return err
			}
			if err := validateTokenKeyUniqueConstraints(constraints); err != nil {
				return err
			}
			tokenConstraints = constraints
			index, err := inspectTokenKeyIndex(db, statement.Schema.Table)
			if err != nil {
				return err
			}
			if index.exists && !index.definitionValid {
				return fmt.Errorf("legacy token key index has an unsupported definition")
			}
		}
		indexes, err := db.Migrator().GetIndexes(value)
		if err != nil {
			return err
		}
		// Never silently discard custom uniqueness rules. Only the released
		// schema's indexes are replaced with the model's tenant-qualified
		// equivalents; operators must explicitly migrate custom definitions.
		for _, index := range indexes {
			unique, _ := index.Unique()
			primary, _ := index.PrimaryKey()
			if !unique || primary || slices.Contains(index.Columns(), "tenant_id") {
				continue
			}
			known := false
			for _, target := range statement.Schema.ParseIndexes() {
				if target.Class != "UNIQUE" {
					continue
				}
				var columns []string
				for _, field := range target.Fields {
					if field.DBName != "tenant_id" {
						columns = append(columns, field.DBName)
					}
				}
				// PostgreSQL's GORM index inventory does not guarantee column
				// order. Uniqueness depends on this column set, not its order.
				slices.Sort(columns)
				storedColumns := slices.Clone(index.Columns())
				slices.Sort(storedColumns)
				if !slices.Equal(columns, storedColumns) {
					continue
				}
				known = index.Name() == target.Name
				if len(columns) == 1 {
					known = known || index.Name() == db.NamingStrategy.IndexName(statement.Schema.Table, columns[0]) ||
						index.Name() == db.NamingStrategy.UniqueName(statement.Schema.Table, columns[0]) ||
						index.Name() == statement.Schema.Table+"_"+columns[0]+"_key" ||
						strings.HasPrefix(index.Name(), "sqlite_autoindex")
				}
				if known {
					break
				}
			}
			if !known {
				return fmt.Errorf("unsupported unique index %q on %s: migrate this custom index to include tenant_id before startup", index.Name(), statement.Schema.Table)
			}
		}
		if !db.Migrator().HasColumn(value, "tenant_id") {
			table := statement.Quote(statement.Schema.Table)
			column := statement.Quote("tenant_id")
			if err := db.Exec("ALTER TABLE " + table + " ADD COLUMN " + column + " BIGINT NOT NULL DEFAULT 1").Error; err != nil {
				return fmt.Errorf("add tenant scope to %s: %w", table, err)
			}
		}
		// GORM's PostgreSQL GetIndexes excludes constraint-backed indexes.
		// Remove the validated legacy token constraints explicitly so its
		// column migrator cannot mistake them for uni_tokens_key.
		for _, constraint := range tokenConstraints {
			if err := db.Migrator().DropConstraint(value, constraint.Name); err != nil {
				return err
			}
		}
		// Drop constraints before their backing indexes. PostgreSQL refuses
		// DROP INDEX for an index owned by a UNIQUE constraint.
		for _, field := range statement.Schema.Fields {
			name := db.NamingStrategy.UniqueName(statement.Schema.Table, field.DBName)
			if db.Migrator().HasConstraint(value, name) {
				if err := db.Migrator().DropConstraint(value, name); err != nil {
					return err
				}
			}
		}
		// Convert global uniqueness to (tenant_id, ...) before AutoMigrate.
		indexes, err = db.Migrator().GetIndexes(value)
		if err != nil {
			return err
		}
		for _, index := range indexes {
			unique, _ := index.Unique()
			primary, _ := index.PrimaryKey()
			if !unique || primary {
				continue
			}
			if !slices.Contains(index.Columns(), "tenant_id") && !strings.HasPrefix(index.Name(), "sqlite_autoindex") {
				if db.Migrator().HasConstraint(value, index.Name()) {
					if err := db.Migrator().DropConstraint(value, index.Name()); err != nil {
						return err
					}
				} else if err := db.Migrator().DropIndex(value, index.Name()); err != nil {
					return err
				}
			}
		}
	}
	for _, value := range []any{&Option{}, &SystemTaskLock{}, &Ability{}} {
		if err := migrateTenantPrimaryKey(db, value); err != nil {
			return err
		}
	}
	return nil
}

func migrateTenantPrimaryKey(db *gorm.DB, value any) error {
	statement := &gorm.Statement{DB: db}
	if err := statement.Parse(value); err != nil {
		return err
	}
	name := statement.Schema.Table
	backup := "saas_legacy_" + name + "_v1"
	if !db.Migrator().HasTable(backup) {
		if !db.Migrator().HasTable(value) {
			return nil
		}
		columns, err := db.Migrator().ColumnTypes(value)
		if err != nil {
			return err
		}
		for _, column := range columns {
			primary, _ := column.PrimaryKey()
			if column.Name() == "tenant_id" && primary {
				return nil
			}
		}
		indexes, err := db.Migrator().GetIndexes(value)
		if err != nil {
			return err
		}
		knownIndexes := statement.Schema.ParseIndexes()
		for _, index := range indexes {
			primary, _ := index.PrimaryKey()
			if _, known := knownIndexes[index.Name()]; !primary && !known && !strings.HasPrefix(index.Name(), "sqlite_autoindex") {
				return fmt.Errorf("custom index %q on %s requires an explicit migration before rebuilding its primary key", index.Name(), name)
			}
		}
		if err := db.Migrator().RenameTable(name, backup); err != nil {
			return err
		}
	}
	// Renaming a table keeps its index names. SQLite and PostgreSQL share
	// those names across tables, so release the old secondary indexes before
	// creating the replacement. The backup remains available after failures.
	indexes, err := db.Migrator().GetIndexes(backup)
	if err != nil {
		return err
	}
	for _, index := range indexes {
		primary, _ := index.PrimaryKey()
		if primary || strings.HasPrefix(index.Name(), "sqlite_autoindex") {
			continue
		}
		if err := db.Migrator().DropIndex(backup, index.Name()); err != nil {
			return err
		}
	}
	if !db.Migrator().HasTable(name) {
		if err := db.Migrator().CreateTable(value); err != nil {
			return err
		}
	}
	rows := reflect.New(reflect.SliceOf(reflect.TypeOf(value).Elem())).Interface()
	if err := db.Table(backup).Find(rows).Error; err != nil {
		return err
	}
	if reflect.ValueOf(rows).Elem().Len() > 0 {
		if err := db.Table(name).Clauses(clause.OnConflict{DoNothing: true}).CreateInBatches(rows, 100).Error; err != nil {
			return err
		}
	}
	// Until the copy succeeds, the backup is a durable recovery marker. Once
	// copied, remove it so later restarts cannot resurrect deleted options.
	return db.Migrator().DropTable(backup)
}

// EnforceTenantScopes is called only after schema migration and platform
// bootstrap. There is deliberately no public per-query bypass flag.
func EnforceTenantScopes() error {
	globals := map[string]bool{
		"platform_users": true, "platform_sessions": true, "platform_auth_attempts": true,
		"tenants": true, "plans": true, "tenant_usage": true,
		"plan_assignments": true, "tenant_root_activations": true,
		"platform_admin_guard": true, "platform_audits": true,
		"platform_redemptions": true, "platform_redemption_uses": true,
		"system_instances": true,
	}
	if err := DB.Use(tenant.Scope{GlobalTables: globals, BeforeCreate: plan.EnforceResourceCapacity}); err != nil {
		return err
	}
	if LOG_DB != DB {
		return LOG_DB.Use(tenant.Scope{GlobalTables: globals, BeforeCreate: plan.EnforceResourceCapacity})
	}
	return nil
}

func TenantDB(ctx context.Context) *gorm.DB { return DB.WithContext(ctx) }
