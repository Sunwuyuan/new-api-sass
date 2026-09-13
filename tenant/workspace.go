package tenant

import "time"

type Workspace struct {
	ID                  int64      `json:"id" gorm:"primaryKey"`
	Slug                string     `json:"slug" gorm:"size:48;not null;uniqueIndex"`
	Name                string     `json:"name" gorm:"size:128;not null"`
	OwnerPlatformUserID int64      `json:"owner_platform_user_id" gorm:"not null;index"`
	PlanID              int64      `json:"plan_id" gorm:"not null"`
	Status              string     `json:"status" gorm:"size:16;not null"`
	PlanExpiresAt       *time.Time `json:"plan_expires_at"`
	CreatedAt           time.Time  `json:"created_at"`
	UpdatedAt           time.Time  `json:"updated_at"`
}

func (Workspace) TableName() string { return "tenants" }
