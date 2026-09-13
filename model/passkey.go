package model

import context "context"

import (
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/system_setting"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
	"gorm.io/gorm"
)

var (
	ErrPasskeyNotFound         = errors.New("passkey credential not found")
	ErrFriendlyPasskeyNotFound = errors.New("Passkey 验证失败，请重试或联系管理员")
)

type PasskeyCredential struct {
	TenantID        int64          `json:"-" gorm:"not null;index;uniqueIndex:tenant_passkey_credential_user_i_d,priority:1;uniqueIndex:tenant_passkey_credential_credential_i_d,priority:1"`
	ID              int            `json:"id" gorm:"primaryKey"`
	UserID          int            `json:"user_id" gorm:"uniqueIndex:tenant_passkey_credential_user_i_d;not null"`
	RPID            *string        `json:"rp_id,omitempty" gorm:"column:rp_id;type:varchar(253)"`
	CredentialID    string         `json:"credential_id" gorm:"type:varchar(512);uniqueIndex:tenant_passkey_credential_credential_i_d;not null"` // base64 encoded
	PublicKey       string         `json:"public_key" gorm:"type:text;not null"`                                                                 // base64 encoded
	AttestationType string         `json:"attestation_type" gorm:"type:varchar(255)"`
	AAGUID          string         `json:"aaguid" gorm:"type:varchar(512)"` // base64 encoded
	SignCount       uint32         `json:"sign_count" gorm:"default:0"`
	CloneWarning    bool           `json:"clone_warning"`
	UserPresent     bool           `json:"user_present"`
	UserVerified    bool           `json:"user_verified"`
	BackupEligible  bool           `json:"backup_eligible"`
	BackupState     bool           `json:"backup_state"`
	Transports      string         `json:"transports" gorm:"type:text"`
	Attachment      string         `json:"attachment" gorm:"type:varchar(32)"`
	LastUsedAt      *time.Time     `json:"last_used_at"`
	CreatedAt       time.Time      `json:"created_at"`
	UpdatedAt       time.Time      `json:"updated_at"`
	DeletedAt       gorm.DeletedAt `json:"-" gorm:"index"`
}

func (p *PasskeyCredential) TransportList() []protocol.AuthenticatorTransport {
	if p == nil || strings.TrimSpace(p.Transports) == "" {
		return nil
	}
	var transports []string
	if err := common.Unmarshal([]byte(p.Transports), &transports); err != nil {
		return nil
	}
	result := make([]protocol.AuthenticatorTransport, 0, len(transports))
	for _, transport := range transports {
		result = append(result, protocol.AuthenticatorTransport(transport))
	}
	return result
}

func (p *PasskeyCredential) SetTransports(list []protocol.AuthenticatorTransport) {
	if len(list) == 0 {
		p.Transports = ""
		return
	}
	stringList := make([]string, len(list))
	for i, transport := range list {
		stringList[i] = string(transport)
	}
	encoded, err := common.Marshal(stringList)
	if err != nil {
		return
	}
	p.Transports = string(encoded)
}

func (p *PasskeyCredential) ToWebAuthnCredential() webauthn.Credential {
	flags := webauthn.CredentialFlags{
		UserPresent:    p.UserPresent,
		UserVerified:   p.UserVerified,
		BackupEligible: p.BackupEligible,
		BackupState:    p.BackupState,
	}

	credID, _ := base64.StdEncoding.DecodeString(p.CredentialID)
	pubKey, _ := base64.StdEncoding.DecodeString(p.PublicKey)
	aaguid, _ := base64.StdEncoding.DecodeString(p.AAGUID)

	return webauthn.Credential{
		ID:              credID,
		PublicKey:       pubKey,
		AttestationType: p.AttestationType,
		Transport:       p.TransportList(),
		Flags:           flags,
		Authenticator: webauthn.Authenticator{
			AAGUID:       aaguid,
			SignCount:    p.SignCount,
			CloneWarning: p.CloneWarning,
			Attachment:   protocol.AuthenticatorAttachment(p.Attachment),
		},
	}
}

func NewPasskeyCredentialFromWebAuthn(userID int, credential *webauthn.Credential) *PasskeyCredential {
	if credential == nil {
		return nil
	}
	passkey := &PasskeyCredential{
		UserID:          userID,
		CredentialID:    base64.StdEncoding.EncodeToString(credential.ID),
		PublicKey:       base64.StdEncoding.EncodeToString(credential.PublicKey),
		AttestationType: credential.AttestationType,
		AAGUID:          base64.StdEncoding.EncodeToString(credential.Authenticator.AAGUID),
		SignCount:       credential.Authenticator.SignCount,
		CloneWarning:    credential.Authenticator.CloneWarning,
		UserPresent:     credential.Flags.UserPresent,
		UserVerified:    credential.Flags.UserVerified,
		BackupEligible:  credential.Flags.BackupEligible,
		BackupState:     credential.Flags.BackupState,
		Attachment:      string(credential.Authenticator.Attachment),
	}
	passkey.SetTransports(credential.Transport)
	return passkey
}

func GetPasskeyByUserID(tenantCtx context.Context, userID int) (*PasskeyCredential, error) {
	if userID == 0 {
		common.SysLog("GetPasskeyByUserID: empty user ID")
		return nil, ErrFriendlyPasskeyNotFound
	}
	var credential PasskeyCredential
	if err := DB.WithContext(tenantCtx).Where("user_id = ?", userID).First(&credential).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// 未找到记录是正常情况（用户未绑定），返回 ErrPasskeyNotFound 而不记录日志
			return nil, ErrPasskeyNotFound
		}
		// 只有真正的数据库错误才记录日志
		common.SysLog(fmt.Sprintf("GetPasskeyByUserID: database error for user %d: %v", userID, err))
		return nil, ErrFriendlyPasskeyNotFound
	}
	return &credential, nil
}

func GetPasskeyByCredentialID(tenantCtx context.Context, credentialID []byte) (*PasskeyCredential, error) {
	if len(credentialID) == 0 {
		common.SysLog("GetPasskeyByCredentialID: empty credential ID")
		return nil, ErrFriendlyPasskeyNotFound
	}

	credIDStr := base64.StdEncoding.EncodeToString(credentialID)
	var credential PasskeyCredential
	if err := DB.WithContext(tenantCtx).Where("credential_id = ?", credIDStr).First(&credential).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			common.SysLog(fmt.Sprintf("GetPasskeyByCredentialID: passkey not found for credential ID length %d", len(credentialID)))
			return nil, ErrFriendlyPasskeyNotFound
		}
		common.SysLog(fmt.Sprintf("GetPasskeyByCredentialID: database error for credential ID: %v", err))
		return nil, ErrFriendlyPasskeyNotFound
	}

	return &credential, nil
}

// UpdatePasskeyAssertionState persists only fields produced by a successful
// assertion. Registration identity (credential ID, public key, AAGUID,
// transports and attestation metadata) is immutable on this path.
func UpdatePasskeyAssertionState(tenantCtx context.Context, userID int, credential *webauthn.Credential, lastUsedAt time.Time, rpID string) error {
	if userID <= 0 || credential == nil || len(credential.ID) == 0 || lastUsedAt.IsZero() || rpID == "" {
		return fmt.Errorf("Passkey 保存失败，请重试")
	}
	credentialID := base64.StdEncoding.EncodeToString(credential.ID)
	TenantState(tenantCtx).passkeyOptionMutex.Lock()
	defer TenantState(tenantCtx).passkeyOptionMutex.Unlock()
	return DB.WithContext(tenantCtx).Transaction(func(tx *gorm.DB) error {
		if err := validatePasskeyRPIDWithTx(tx, rpID); err != nil {
			return err
		}
		var stored PasskeyCredential
		if err := lockForUpdate(tx).Where("user_id = ? AND credential_id = ?", userID, credentialID).First(&stored).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrPasskeyNotFound
			}
			return err
		}
		// Compare in Go: the database collation may fold historical case.
		if stored.RPID != nil && *stored.RPID != "" && *stored.RPID != rpID {
			return system_setting.ErrPasskeyRPIDUnavailable
		}
		result := tx.Model(&PasskeyCredential{}).
			Where("user_id = ? AND credential_id = ?", userID, credentialID).
			Where("rp_id IS NULL OR rp_id = ? OR rp_id = ?", "", rpID).
			Updates(map[string]any{
				"rp_id":           rpID,
				"sign_count":      credential.Authenticator.SignCount,
				"clone_warning":   credential.Authenticator.CloneWarning,
				"user_present":    credential.Flags.UserPresent,
				"user_verified":   credential.Flags.UserVerified,
				"backup_eligible": credential.Flags.BackupEligible,
				"backup_state":    credential.Flags.BackupState,
				"last_used_at":    lastUsedAt,
			})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return ErrPasskeyNotFound
		}
		return nil
	})
}

func upsertPasskeyCredentialWithTx(tx *gorm.DB, credential *PasskeyCredential) error {
	if err := tx.Unscoped().Where("user_id = ?", credential.UserID).Delete(&PasskeyCredential{}).Error; err != nil {
		common.SysLog(fmt.Sprintf("UpsertPasskeyCredential: failed to delete existing credential for user %d: %v", credential.UserID, err))
		return fmt.Errorf("Passkey 保存失败，请重试")
	}
	if err := tx.Create(credential).Error; err != nil {
		common.SysLog(fmt.Sprintf("UpsertPasskeyCredential: failed to create credential for user %d: %v", credential.UserID, err))
		return fmt.Errorf("Passkey 保存失败，请重试")
	}
	return nil
}

// UpsertPasskeyCredentialWithAuthVersion is reserved for enrollment changes;
// assertion sign-count updates must use UpdatePasskeyAssertionState.
func UpsertPasskeyCredentialWithAuthVersion(tenantCtx context.Context, credential *PasskeyCredential) error {
	return upsertPasskeyCredentialWithAuthVersion(tenantCtx, credential, nil)
}

func RegisterPasskeyForSession(tenantCtx context.Context, identity AuthSessionIdentity, credential *PasskeyCredential) error {
	return upsertPasskeyCredentialWithAuthVersion(tenantCtx, credential, &identity)
}

func upsertPasskeyCredentialWithAuthVersion(tenantCtx context.Context, credential *PasskeyCredential, identity *AuthSessionIdentity) error {
	if credential == nil || credential.UserID <= 0 {
		return fmt.Errorf("Passkey 保存失败，请重试")
	}
	TenantState(tenantCtx).passkeyOptionMutex.Lock()
	defer TenantState(tenantCtx).passkeyOptionMutex.Unlock()
	if err := DB.WithContext(tenantCtx).Transaction(func(tx *gorm.DB) error {
		if identity != nil && (credential.RPID == nil || *credential.RPID == "") {
			return system_setting.ErrPasskeyRPIDUnavailable
		}
		if credential.RPID != nil && *credential.RPID != "" {
			if err := validatePasskeyRPIDWithTx(tx, *credential.RPID); err != nil {
				return err
			}
		}
		if identity != nil {
			if identity.UserID != credential.UserID {
				return ErrUserSessionInactive
			}
			if err := ValidateAuthSessionWithTx(tx, *identity); err != nil {
				return err
			}
		}
		if _, err := IncrementUserAuthVersionWithTx(tx, credential.UserID); err != nil {
			return err
		}
		return upsertPasskeyCredentialWithTx(tx, credential)
	}); err != nil {
		return err
	}
	return PublishUserAuthCache(tenantCtx, credential.UserID)
}

func DeletePasskeyByUserIDWithAuthVersion(tenantCtx context.Context, userID int) error {
	return deletePasskeyWithAuthVersion(tenantCtx, userID, nil)
}

func DeletePasskeyForSession(tenantCtx context.Context, identity AuthSessionIdentity) error {
	return deletePasskeyWithAuthVersion(tenantCtx, identity.UserID, &identity)
}

func deletePasskeyWithAuthVersion(tenantCtx context.Context, userID int, identity *AuthSessionIdentity) error {
	if userID == 0 {
		return fmt.Errorf("删除失败，请重试")
	}
	if err := DB.WithContext(tenantCtx).Transaction(func(tx *gorm.DB) error {
		if identity != nil {
			if err := ValidateAuthSessionWithTx(tx, *identity); err != nil {
				return err
			}
		}
		var credential PasskeyCredential
		if err := lockForUpdate(tx).Where("user_id = ?", userID).First(&credential).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrPasskeyNotFound
			}
			return err
		}
		if _, err := IncrementUserAuthVersionWithTx(tx, userID); err != nil {
			return err
		}
		result := tx.Unscoped().Delete(&credential)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return ErrPasskeyNotFound
		}
		return nil
	}); err != nil {
		return err
	}
	return PublishUserAuthCache(tenantCtx, userID)
}
