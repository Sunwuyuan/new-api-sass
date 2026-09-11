package model

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	"github.com/QuantumNous/new-api/tenant"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// These optional engines must point to a disposable database. The fixture
// refuses to touch an existing tokens table and removes only its own table.
func TestMigrateTokenTenantScope(t *testing.T) {
	for _, engine := range []string{"sqlite", "mysql", "postgres"} {
		t.Run(engine, func(t *testing.T) {
			dsn := os.Getenv("TEST_MYSQL_DSN")
			if engine == "postgres" {
				dsn = os.Getenv("TEST_POSTGRES_DSN")
			}
			if engine != "sqlite" && dsn == "" {
				t.Skip("disposable database DSN is not configured")
			}
			for _, legacy := range []bool{false, true} {
				name := "fresh"
				if legacy {
					name = "released_schema"
				}
				t.Run(name, func(t *testing.T) {
					db := tokenMigrationDatabase(t, engine, dsn)
					var raw [24]byte
					_, err := rand.Read(raw[:])
					require.NoError(t, err)
					key := hex.EncodeToString(raw[:])
					if legacy {
						type releasedToken struct {
							Id     int    `gorm:"primaryKey"`
							Key    string `gorm:"type:char(48);uniqueIndex:idx_tokens_key"`
							UserId int
							Name   string
						}
						require.NoError(t, db.Table("tokens").AutoMigrate(&releasedToken{}))
						require.NoError(t, db.Table("tokens").Create(&releasedToken{Key: key, UserId: 7, Name: "preserved"}).Error)
					}
					for range 2 {
						require.NoError(t, MigrateTenantSchema(db, []any{&Token{}}))
						require.NoError(t, db.AutoMigrate(&Token{}))
					}
					require.NoError(t, db.Use(tenant.Scope{}))
					a := db.WithContext(tenant.WithContext(context.Background(), tenant.Identity{ID: 1, Slug: "imported"}))
					b := db.WithContext(tenant.WithContext(context.Background(), tenant.Identity{ID: 2, Slug: "other"}))
					if legacy {
						var row Token
						require.NoError(t, a.Where(map[string]any{"key": key}).First(&row).Error)
						assert.Equal(t, "preserved", row.Name)
						assert.Equal(t, 7, row.UserId)
						assert.EqualValues(t, 1, row.TenantID)
					} else {
						require.NoError(t, a.Create(&Token{Key: key, UserId: 7, Name: "preserved"}).Error)
					}
					assert.Error(t, a.Create(&Token{Key: key, UserId: 8}).Error)
					require.NoError(t, b.Create(&Token{Key: key, UserId: 8, Name: "independent"}).Error)
					var rows []Token
					require.NoError(t, b.Find(&rows).Error)
					require.Len(t, rows, 1)
					assert.Equal(t, "independent", rows[0].Name)
					assert.ErrorIs(t, db.Find(&rows).Error, tenant.ErrMissing)
				})
			}
		})
	}
}

func tokenMigrationDatabase(t *testing.T, engine, dsn string) *gorm.DB {
	t.Helper()
	var dialector gorm.Dialector = sqlite.Open(filepath.Join(t.TempDir(), "tokens.sqlite"))
	if engine == "mysql" {
		dialector = mysql.Open(dsn)
	} else if engine == "postgres" {
		dialector = postgres.New(postgres.Config{DSN: dsn, PreferSimpleProtocol: true})
	}
	db, err := gorm.Open(dialector, &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
	require.False(t, db.Migrator().HasTable("tokens"), "use an empty disposable database")
	t.Cleanup(func() {
		_, err := sqlDB.Exec("DROP TABLE IF EXISTS tokens")
		require.NoError(t, err)
	})
	return db
}

func TestTokenTenantMigrationPostgreSQLConstraints(t *testing.T) {
	dsn := os.Getenv("TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("disposable PostgreSQL database DSN is not configured")
	}
	for _, tc := range []struct {
		name       string
		constraint string
		deferrable bool
		reject     bool
	}{
		{name: "legacy index constraint", constraint: tokenKeyIndex},
		{name: "GORM constraint", constraint: gormTokenKeyConstraint},
		{name: "PostgreSQL constraint", constraint: postgresTokenKeyConstraint},
		{name: "unknown constraint preserved", constraint: "custom_tokens_key", reject: true},
		{name: "deferrable constraint preserved", constraint: postgresTokenKeyConstraint, deferrable: true, reject: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db := tokenMigrationDatabase(t, "postgres", dsn)
			require.NoError(t, db.AutoMigrate(&Token{}))
			query := "ALTER TABLE tokens ADD CONSTRAINT ? UNIQUE (key)"
			if tc.deferrable {
				query += " DEFERRABLE INITIALLY DEFERRED"
			}
			require.NoError(t, db.Exec(query, clause.Column{Name: tc.constraint}).Error)
			err := MigrateTenantSchema(db, []any{&Token{}})
			if tc.reject {
				require.Error(t, err)
				assert.True(t, db.Migrator().HasConstraint(&Token{}, tc.constraint))
				return
			}
			require.NoError(t, err)
			require.NoError(t, db.AutoMigrate(&Token{}))
			assert.False(t, db.Migrator().HasConstraint(&Token{}, tc.constraint))
			assert.True(t, db.Migrator().HasIndex(&Token{}, "tenant_token_key"))
		})
	}
}
