package model

import "github.com/QuantumNous/new-api/tenant"

import context "context"

type Setup struct {
	tenant.Row
	ID            uint   `json:"id" gorm:"primaryKey"`
	Version       string `json:"version" gorm:"type:varchar(50);not null"`
	InitializedAt int64  `json:"initialized_at" gorm:"type:bigint;not null"`
}

func GetSetup(tenantCtx context.Context) *Setup {
	var setup Setup
	err := DB.WithContext(tenantCtx).First(&setup).Error
	if err != nil {
		return nil
	}
	return &setup
}
