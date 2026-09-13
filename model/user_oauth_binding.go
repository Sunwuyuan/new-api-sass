package model

import context "context"

import (
	"errors"
	"time"

	"gorm.io/gorm"
)

// UserOAuthBinding stores the binding relationship between users and custom OAuth providers
type UserOAuthBinding struct {
	TenantID       int64     `json:"-" gorm:"not null;index;uniqueIndex:ux_user_provider,priority:1;uniqueIndex:ux_provider_userid,priority:1"`
	Id             int       `json:"id" gorm:"primaryKey"`
	UserId         int       `json:"user_id" gorm:"not null;uniqueIndex:ux_user_provider"`                                    // User ID - one binding per user per provider
	ProviderId     int       `json:"provider_id" gorm:"not null;uniqueIndex:ux_user_provider;uniqueIndex:ux_provider_userid"` // Custom OAuth provider ID
	ProviderUserId string    `json:"provider_user_id" gorm:"type:varchar(256);not null;uniqueIndex:ux_provider_userid"`       // User ID from OAuth provider - one OAuth account per provider
	CreatedAt      time.Time `json:"created_at"`
}

func (UserOAuthBinding) TableName() string {
	return "user_oauth_bindings"
}

// GetUserOAuthBindingsByUserId returns all OAuth bindings for a user
func GetUserOAuthBindingsByUserId(tenantCtx context.Context, userId int) ([]*UserOAuthBinding, error) {
	var bindings []*UserOAuthBinding
	err := DB.WithContext(tenantCtx).Where("user_id = ?", userId).Find(&bindings).Error
	return bindings, err
}

// GetUserOAuthBinding returns a specific binding for a user and provider
func GetUserOAuthBinding(tenantCtx context.Context, userId, providerId int) (*UserOAuthBinding, error) {
	var binding UserOAuthBinding
	err := DB.WithContext(tenantCtx).Where("user_id = ? AND provider_id = ?", userId, providerId).First(&binding).Error
	if err != nil {
		return nil, err
	}
	return &binding, nil
}

// GetUserByOAuthBinding finds a user by provider ID and provider user ID
func GetUserByOAuthBinding(tenantCtx context.Context, providerId int, providerUserId string) (*User, error) {
	var binding UserOAuthBinding
	err := DB.WithContext(tenantCtx).Where("provider_id = ? AND provider_user_id = ?", providerId, providerUserId).First(&binding).Error
	if err != nil {
		return nil, err
	}

	var user User
	err = DB.WithContext(tenantCtx).First(&user, binding.UserId).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// IsProviderUserIdTaken checks if a provider user ID is already bound to any user
func IsProviderUserIdTaken(tenantCtx context.Context, providerId int, providerUserId string) bool {
	var count int64
	DB.WithContext(tenantCtx).Model(&UserOAuthBinding{}).Where("provider_id = ? AND provider_user_id = ?", providerId, providerUserId).Count(&count)
	return count > 0
}

// CreateUserOAuthBinding creates a new OAuth binding
func CreateUserOAuthBinding(tenantCtx context.Context, binding *UserOAuthBinding) error {
	if binding.UserId == 0 {
		return errors.New("user ID is required")
	}
	if binding.ProviderId == 0 {
		return errors.New("provider ID is required")
	}
	if binding.ProviderUserId == "" {
		return errors.New("provider user ID is required")
	}

	// Check if this provider user ID is already taken
	if IsProviderUserIdTaken(tenantCtx, binding.ProviderId, binding.ProviderUserId) {
		return errors.New("this OAuth account is already bound to another user")
	}

	binding.CreatedAt = time.Now()
	return DB.WithContext(tenantCtx).Create(binding).Error
}

// CreateUserOAuthBindingWithTx creates a new OAuth binding within a transaction
func CreateUserOAuthBindingWithTx(tx *gorm.DB, binding *UserOAuthBinding) error {
	if binding.UserId == 0 {
		return errors.New("user ID is required")
	}
	if binding.ProviderId == 0 {
		return errors.New("provider ID is required")
	}
	if binding.ProviderUserId == "" {
		return errors.New("provider user ID is required")
	}

	// Check if this provider user ID is already taken (use tx to check within the same transaction)
	var count int64
	tx.Model(&UserOAuthBinding{}).Where("provider_id = ? AND provider_user_id = ?", binding.ProviderId, binding.ProviderUserId).Count(&count)
	if count > 0 {
		return errors.New("this OAuth account is already bound to another user")
	}

	binding.CreatedAt = time.Now()
	return tx.Create(binding).Error
}

// UpdateUserOAuthBinding updates an existing OAuth binding (e.g., rebind to different OAuth account)
func UpdateUserOAuthBinding(tenantCtx context.Context, userId, providerId int, newProviderUserId string) error {
	// Check if the new provider user ID is already taken by another user
	var existingBinding UserOAuthBinding
	err := DB.WithContext(tenantCtx).Where("provider_id = ? AND provider_user_id = ?", providerId, newProviderUserId).First(&existingBinding).Error
	if err == nil && existingBinding.UserId != userId {
		return errors.New("this OAuth account is already bound to another user")
	}

	// Check if user already has a binding for this provider
	var binding UserOAuthBinding
	err = DB.WithContext(tenantCtx).Where("user_id = ? AND provider_id = ?", userId, providerId).First(&binding).Error
	if err != nil {
		// No existing binding, create new one
		return CreateUserOAuthBinding(tenantCtx, &UserOAuthBinding{
			UserId:         userId,
			ProviderId:     providerId,
			ProviderUserId: newProviderUserId,
		})
	}

	// Update existing binding
	return DB.WithContext(tenantCtx).Model(&binding).Update("provider_user_id", newProviderUserId).Error
}

// DeleteUserOAuthBinding deletes an OAuth binding
func DeleteUserOAuthBinding(tenantCtx context.Context, userId, providerId int) error {
	return DB.WithContext(tenantCtx).Where("user_id = ? AND provider_id = ?", userId, providerId).Delete(&UserOAuthBinding{}).Error
}

func deleteUserOAuthBindingsByUserId(tx *gorm.DB, userId int) error {
	return tx.Where("user_id = ?", userId).Delete(&UserOAuthBinding{}).Error
}

// GetBindingCountByProviderId returns the number of bindings for a provider
func GetBindingCountByProviderId(tenantCtx context.Context, providerId int) (int64, error) {
	var count int64
	err := DB.WithContext(tenantCtx).Model(&UserOAuthBinding{}).Where("provider_id = ?", providerId).Count(&count).Error
	return count, err
}
