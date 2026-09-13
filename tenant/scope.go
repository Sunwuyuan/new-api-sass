package tenant

import (
	"fmt"
	"reflect"
	"regexp"
	"strings"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Row is embedded by every workspace-owned model. TenantID is deliberately
// absent from client JSON; only the database callback assigns ownership.
type Row struct {
	TenantID int64 `json:"-" gorm:"not null;index"`
}

// Scope is installed on both the main and log databases after migrations.
// GlobalTables is an explicit allowlist: a new business table fails closed.
type Scope struct {
	GlobalTables map[string]bool
	BeforeCreate func(*gorm.DB, Identity, int) error
}

func (s Scope) Name() string { return "tenant_scope" }

func (s Scope) Initialize(db *gorm.DB) error {
	callbacks := []struct {
		register func(string, func(*gorm.DB)) error
		handler  func(*gorm.DB)
	}{
		{db.Callback().Query().Before("gorm:query").Register, s.query},
		{db.Callback().Row().Before("gorm:row").Register, s.query},
		{db.Callback().Create().Before("gorm:before_create").Register, s.create},
		{db.Callback().Update().Before("gorm:before_update").Register, s.update},
		{db.Callback().Delete().Before("gorm:before_delete").Register, s.query},
		{db.Callback().Raw().Before("gorm:raw").Register, s.raw},
	}
	for _, callback := range callbacks {
		if err := callback.register("tenant:scope", callback.handler); err != nil {
			return err
		}
	}
	return nil
}

var joinedTable = regexp.MustCompile(`(?i)\bjoin\s+["` + "`" + `]?([a-z_][a-z_0-9]*)["` + "`" + `]?`)

func (s Scope) query(db *gorm.DB) {
	if db.Error != nil {
		return
	}
	if db.Statement.SQL.Len() != 0 {
		// Database time is used for cross-replica task leases. These constant
		// expressions cannot read business rows or execute additional SQL.
		switch db.Statement.SQL.String() {
		case "SELECT EXTRACT(EPOCH FROM NOW())::bigint", "SELECT strftime('%s','now')", "SELECT UNIX_TIMESTAMP()":
			return
		}
		db.AddError(fmt.Errorf("raw business queries are forbidden; use a scoped GORM query"))
		return
	}
	global := s.GlobalTables[db.Statement.Table]
	if global && len(db.Statement.Joins) == 0 {
		return
	}
	identity, err := FromContext(db.Statement.Context)
	if err != nil {
		db.AddError(err)
		return
	}
	if !global && (db.Statement.TableExpr == nil || !strings.Contains(db.Statement.TableExpr.SQL, "(?)")) {
		db.Statement.AddClause(clause.Where{Exprs: []clause.Expression{
			clause.Eq{Column: clause.Column{Table: clause.CurrentTable, Name: "tenant_id"}, Value: identity.ID},
		}})
	}
	for _, join := range db.Statement.Joins {
		for _, match := range joinedTable.FindAllStringSubmatch(join.Name, -1) {
			if s.GlobalTables[match[1]] {
				continue
			}
			db.Statement.AddClause(clause.Where{Exprs: []clause.Expression{
				clause.Eq{Column: clause.Column{Table: match[1], Name: "tenant_id"}, Value: identity.ID},
			}})
		}
	}
}

func (s Scope) create(db *gorm.DB) {
	if db.Error != nil || s.GlobalTables[db.Statement.Table] {
		return
	}
	identity, err := FromContext(db.Statement.Context)
	if err != nil {
		db.AddError(err)
		return
	}
	if db.Statement.Schema == nil {
		db.AddError(ErrMissing)
		return
	}
	field := db.Statement.Schema.LookUpField("TenantID")
	if field == nil {
		db.AddError(fmt.Errorf("business model %s has no tenant_id", db.Statement.Table))
		return
	}
	value := db.Statement.ReflectValue
	if value.Kind() != reflect.Struct && value.Kind() != reflect.Map && value.Kind() != reflect.Slice && value.Kind() != reflect.Array {
		db.AddError(fmt.Errorf("business inserts require a model struct"))
		return
	}
	count := 1
	if value.Kind() == reflect.Slice || value.Kind() == reflect.Array {
		count = value.Len()
	}
	if s.BeforeCreate != nil {
		if err := s.BeforeCreate(db, identity, count); err != nil {
			db.AddError(err)
			return
		}
	}
	for i := range count {
		row := value
		if value.Kind() == reflect.Slice || value.Kind() == reflect.Array {
			row = value.Index(i)
		}
		if values, ok := row.Interface().(map[string]any); ok {
			if values == nil {
				db.AddError(fmt.Errorf("business insert cannot use a nil map"))
				return
			}
			for _, key := range []string{field.Name, field.DBName} {
				if owner, exists := values[key]; exists && owner != identity.ID {
					db.AddError(ErrMismatch)
					return
				}
			}
			delete(values, field.Name)
			values[field.DBName] = identity.ID
			continue
		}
		if reflect.Indirect(row).Kind() != reflect.Struct {
			db.AddError(fmt.Errorf("business inserts require a model struct"))
			return
		}
		current, zero := field.ValueOf(db.Statement.Context, row)
		if !zero && current != identity.ID {
			db.AddError(ErrMismatch)
			return
		}
		if err := field.Set(db.Statement.Context, row, identity.ID); err != nil {
			db.AddError(err)
			return
		}
	}
	// A partial insert must not fall back to the imported tenant's database
	// default. Ownership is mandatory even when callers use Select or Omit.
	if len(db.Statement.Selects) > 0 {
		db.Statement.Selects = append(db.Statement.Selects, field.DBName)
	}
	omits := make([]string, 0, len(db.Statement.Omits))
	for _, column := range db.Statement.Omits {
		if column == "*" {
			db.AddError(fmt.Errorf("business inserts cannot omit every column"))
			return
		}
		if strings.EqualFold(column, field.DBName) || strings.EqualFold(column, field.Name) {
			continue
		}
		omits = append(omits, column)
	}
	db.Statement.Omits = omits
	// Save's upsert fallback may otherwise update a row owned by another
	// tenant when the caller supplies its primary key. Reject that fallback.
	if conflict, ok := db.Statement.Clauses["ON CONFLICT"]; ok {
		if onConflict, ok := conflict.Expression.(clause.OnConflict); ok {
			if onConflict.UpdateAll || onConflict.OnConstraint != "" {
				db.AddError(fmt.Errorf("unrestricted business upsert is forbidden"))
				return
			}
			for _, assignment := range onConflict.DoUpdates {
				if assignment.Column.Name == "tenant_id" {
					db.AddError(ErrMismatch)
					return
				}
			}
			if len(onConflict.Columns) > 0 {
				hasTenant := false
				for _, column := range onConflict.Columns {
					hasTenant = hasTenant || column.Name == "tenant_id"
				}
				if !hasTenant {
					onConflict.Columns = append([]clause.Column{{Name: "tenant_id"}}, onConflict.Columns...)
				}
			}
			// A caller-supplied surrogate ID could collide on MySQL even when
			// the intended conflict target is a tenant-qualified unique key.
			if len(onConflict.DoUpdates) > 0 {
				for _, primary := range db.Statement.Schema.PrimaryFields {
					if !primary.AutoIncrement {
						continue
					}
					for i := range count {
						row := value
						if value.Kind() == reflect.Slice || value.Kind() == reflect.Array {
							row = value.Index(i)
						}
						if values, ok := row.Interface().(map[string]any); ok {
							for _, key := range []string{primary.Name, primary.DBName} {
								if id := values[key]; id != nil && !reflect.ValueOf(id).IsZero() {
									db.AddError(fmt.Errorf("business upsert must not supply a surrogate primary key"))
									return
								}
							}
							continue
						}
						if _, zero := primary.ValueOf(db.Statement.Context, row); !zero {
							db.AddError(fmt.Errorf("business upsert must not supply a surrogate primary key"))
							return
						}
					}
				}
			}
			db.Statement.AddClause(onConflict)
		}
	}
}

func (s Scope) update(db *gorm.DB) {
	s.query(db)
	if db.Error != nil || s.GlobalTables[db.Statement.Table] {
		return
	}
	identity, _ := FromContext(db.Statement.Context)
	if values, ok := db.Statement.Dest.(map[string]any); ok {
		for key := range values {
			if strings.EqualFold(key, "tenant_id") || strings.EqualFold(key, "TenantID") {
				db.AddError(ErrMismatch)
				return
			}
		}
	}
	if db.Statement.Schema != nil {
		field := db.Statement.Schema.LookUpField("TenantID")
		if field != nil && db.Statement.ReflectValue.Kind() == reflect.Struct {
			value, zero := field.ValueOf(db.Statement.Context, db.Statement.ReflectValue)
			if !zero && value != identity.ID {
				db.AddError(ErrMismatch)
				return
			}
		}
	}
	db.Statement.Omits = append(db.Statement.Omits, "tenant_id")
}

func (s Scope) raw(db *gorm.DB) {
	// SQL migrations run before this plugin is installed. Runtime mutations
	// must use GORM so tenancy cannot be bypassed with Exec or raw SQL.
	db.AddError(fmt.Errorf("raw SQL is forbidden on the scoped database"))
}
