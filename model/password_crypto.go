package model

import context "context"

import (
	"errors"
	"fmt"

	"github.com/QuantumNous/new-api/common"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const activeLoginEncryptionKeySlot = "active"

// LoginEncryptionKey stores internal key material used by the browser login
// protocol. It is deliberately separate from administrator-facing options.
type LoginEncryptionKey struct {
	TenantID      int64  `json:"-" gorm:"not null;index;uniqueIndex:tenant_login_encryption_key_slot,priority:1"`
	ID            uint   `json:"-" gorm:"primaryKey"`
	Slot          string `json:"-" gorm:"type:varchar(32);not null;uniqueIndex:tenant_login_encryption_key_slot"`
	PrivateKeyPEM string `json:"-" gorm:"type:text;not null"`
}

// InitPasswordEncryption loads the shared login-encryption key from its
// dedicated store. Concurrent replicas converge through the unique slot.
func InitPasswordEncryption(tenantCtx context.Context) error {
	var stored LoginEncryptionKey
	queryErr := DB.WithContext(tenantCtx).Where("slot = ?", activeLoginEncryptionKeySlot).First(&stored).Error
	if queryErr == nil {
		if err := common.LoadPasswordEncryptionPrivateKey(tenantCtx, stored.PrivateKeyPEM); err != nil {
			return fmt.Errorf("load persisted password encryption key: %w", err)
		}
		return nil
	}
	if !errors.Is(queryErr, gorm.ErrRecordNotFound) {
		return fmt.Errorf("read password encryption key: %w", queryErr)
	}

	privateKeyPEM, err := common.GeneratePasswordEncryptionPrivateKey()
	if err != nil {
		return err
	}
	candidate := LoginEncryptionKey{
		Slot:          activeLoginEncryptionKeySlot,
		PrivateKeyPEM: privateKeyPEM,
	}
	if err := DB.WithContext(tenantCtx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "tenant_id"}, {Name: "slot"}},
		DoNothing: true,
	}).Create(&candidate).Error; err != nil {
		return fmt.Errorf("persist password encryption key: %w", err)
	}

	if err := DB.WithContext(tenantCtx).Where("slot = ?", activeLoginEncryptionKeySlot).First(&stored).Error; err != nil {
		return fmt.Errorf("reload password encryption key: %w", err)
	}
	if err := common.LoadPasswordEncryptionPrivateKey(tenantCtx, stored.PrivateKeyPEM); err != nil {
		return fmt.Errorf("load persisted password encryption key: %w", err)
	}
	return nil
}
