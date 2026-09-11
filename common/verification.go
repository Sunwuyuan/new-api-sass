package common

import context "context"

import (
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

type verificationValue struct {
	code string
	time time.Time
}

const (
	EmailVerificationPurpose = "v"
	PasswordResetPurpose     = "r"
)

var verificationMutex sync.Mutex
var verificationMap map[string]verificationValue
var verificationMapMaxSize = 10
var VerificationValidMinutes = 10

func GenerateVerificationCode(length int) string {
	code := uuid.New().String()
	code = strings.Replace(code, "-", "", -1)
	if length == 0 {
		return code
	}
	return code[:length]
}

func RegisterVerificationCodeWithKey(tenantCtx context.Context, key string, code string, purpose string) {
	TenantState(tenantCtx).verificationMutex.Lock()
	defer TenantState(tenantCtx).verificationMutex.Unlock()
	TenantState(tenantCtx).verificationMap[purpose+key] = verificationValue{
		code: code,
		time: time.Now(),
	}
	if len(TenantState(tenantCtx).verificationMap) > TenantState(tenantCtx).verificationMapMaxSize {
		removeExpiredPairs(tenantCtx)
	}
}

func VerifyCodeWithKey(tenantCtx context.Context, key string, code string, purpose string) bool {
	TenantState(tenantCtx).verificationMutex.Lock()
	defer TenantState(tenantCtx).verificationMutex.Unlock()
	value, okay := TenantState(tenantCtx).verificationMap[purpose+key]
	now := time.Now()
	if !okay || int(now.Sub(value.time).Seconds()) >= VerificationValidMinutes*60 {
		return false
	}
	return code == value.code
}

func DeleteKey(tenantCtx context.Context, key string, purpose string) {
	TenantState(tenantCtx).verificationMutex.Lock()
	defer TenantState(tenantCtx).verificationMutex.Unlock()
	delete(TenantState(tenantCtx).verificationMap, purpose+key)
}

// no lock inside, so the caller must lock the verificationMap before calling!
func removeExpiredPairs(tenantCtx context.Context) {
	now := time.Now()
	for key := range TenantState(tenantCtx).verificationMap {
		if int(now.Sub(TenantState(tenantCtx).verificationMap[key].time).Seconds()) >= VerificationValidMinutes*60 {
			delete(TenantState(tenantCtx).verificationMap, key)
		}
	}
}

func init() {
	verificationMutex.Lock()
	defer verificationMutex.Unlock()
	verificationMap = make(map[string]verificationValue)
}
