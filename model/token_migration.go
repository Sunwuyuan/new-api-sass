package model

import (
	"fmt"
	"strings"

	"gorm.io/gorm"
)

const (
	tokenKeyIndex              = "idx_tokens_key"
	postgresTokenKeyConstraint = "tokens_key_key"
	gormTokenKeyConstraint     = "uni_tokens_key"
)

type tokenKeyUniqueConstraint struct {
	Name       string `gorm:"column:constraint_name"`
	Definition string `gorm:"column:constraint_definition"`
	Deferrable bool   `gorm:"column:is_deferrable"`
	Validated  bool   `gorm:"column:is_validated"`
}

type tokenKeyIndexState struct {
	exists          bool
	definitionValid bool
	standaloneValid bool
}

func inspectTokenKeyUniqueConstraints(db *gorm.DB, tableName string) ([]tokenKeyUniqueConstraint, error) {
	var constraints []tokenKeyUniqueConstraint
	if err := db.Raw(`
SELECT constraint_meta.conname AS constraint_name,
       pg_get_constraintdef(constraint_meta.oid) AS constraint_definition,
       constraint_meta.condeferrable AS is_deferrable,
       constraint_meta.convalidated AS is_validated
FROM pg_catalog.pg_constraint AS constraint_meta
WHERE constraint_meta.conrelid = to_regclass(?)
  AND constraint_meta.contype = 'u'
  AND cardinality(constraint_meta.conkey) = 1
  AND EXISTS (
      SELECT 1
      FROM pg_catalog.pg_attribute AS attribute_meta
      WHERE attribute_meta.attrelid = constraint_meta.conrelid
        AND attribute_meta.attnum = constraint_meta.conkey[1]
        AND attribute_meta.attname = ?
  )
ORDER BY constraint_meta.conname`, tableName, "key").Scan(&constraints).Error; err != nil {
		return nil, fmt.Errorf("inspect token key unique constraints: %w", err)
	}
	return constraints, nil
}

func validateTokenKeyUniqueConstraints(constraints []tokenKeyUniqueConstraint) error {
	for _, constraint := range constraints {
		switch constraint.Name {
		case tokenKeyIndex, postgresTokenKeyConstraint, gormTokenKeyConstraint:
		default:
			return fmt.Errorf(
				"tokens.key has unsupported unique constraint %q with definition %q",
				constraint.Name,
				constraint.Definition,
			)
		}
		if constraint.Deferrable || !constraint.Validated || strings.Contains(strings.ToUpper(constraint.Definition), "NULLS NOT DISTINCT") {
			return fmt.Errorf(
				"tokens.key unique constraint %q has unsupported definition %q",
				constraint.Name,
				constraint.Definition,
			)
		}
	}
	return nil
}

func inspectTokenKeyIndex(db *gorm.DB, tableName string) (tokenKeyIndexState, error) {
	var state struct {
		Exists          bool `gorm:"column:index_exists"`
		DefinitionValid bool `gorm:"column:definition_valid"`
		StandaloneValid bool `gorm:"column:standalone_valid"`
	}
	if err := db.Raw(`
SELECT count(*) > 0 AS index_exists,
       COALESCE(bool_or(
           index_meta.indisunique
           AND index_meta.indisvalid
           AND index_meta.indisready
           AND NOT index_meta.indisprimary
           AND index_meta.indpred IS NULL
           AND index_meta.indexprs IS NULL
           AND index_meta.indnatts = 1
           AND attribute_meta.attname = ?
       ), false) AS definition_valid,
       COALESCE(bool_or(
           index_meta.indisunique
           AND index_meta.indisvalid
           AND index_meta.indisready
           AND NOT index_meta.indisprimary
           AND index_meta.indpred IS NULL
           AND index_meta.indexprs IS NULL
           AND index_meta.indnatts = 1
           AND attribute_meta.attname = ?
           AND NOT EXISTS (
               SELECT 1
               FROM pg_catalog.pg_constraint AS constraint_meta
               WHERE constraint_meta.conindid = index_meta.indexrelid
           )
       ), false) AS standalone_valid
FROM pg_catalog.pg_index AS index_meta
JOIN pg_catalog.pg_class AS index_class
  ON index_class.oid = index_meta.indexrelid
LEFT JOIN pg_catalog.pg_attribute AS attribute_meta
  ON attribute_meta.attrelid = index_meta.indrelid
 AND attribute_meta.attnum = index_meta.indkey[0]
WHERE index_meta.indrelid = to_regclass(?)
  AND index_class.relname = ?`, "key", "key", tableName, tokenKeyIndex).Scan(&state).Error; err != nil {
		return tokenKeyIndexState{}, fmt.Errorf("inspect token key unique index: %w", err)
	}
	return tokenKeyIndexState{
		exists:          state.Exists,
		definitionValid: state.DefinitionValid,
		standaloneValid: state.StandaloneValid,
	}, nil
}
