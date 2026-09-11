/**
此文件为旧版支付设置文件，如需增加新的参数、变量等，请在 payment_setting.go 中添加
This file is the old version of the payment settings file. If you need to add new parameters, variables, etc., please add them in payment_setting.go
*/

package operation_setting

import context "context"

import (
	"github.com/QuantumNous/new-api/common"
)

var PayAddress = ""
var CustomCallbackAddress = ""
var EpayId = ""
var EpayKey = ""
var Price = 7.3
var MinTopUp = 1
var USDExchangeRate = 7.3

var PayMethods = []map[string]string{
	{
		"name": "支付宝",
		"icon": "SiAlipay",
		"type": "alipay",
	},
	{
		"name": "微信",
		"icon": "SiWechat",
		"type": "wxpay",
	},
	{
		"name":      "自定义1",
		"icon":      "LuCreditCard",
		"type":      "custom1",
		"min_topup": "50",
	},
}

func UpdatePayMethodsByJsonString(tenantCtx context.Context, jsonString string) error {
	methods := make([]map[string]string, 0)
	if err := common.Unmarshal([]byte(jsonString), &methods); err != nil {
		return err
	}
	UpdateTenantSettings(tenantCtx, func(state *WorkspaceState) { state.PayMethods = methods })
	return nil
}

func PayMethods2JsonString(tenantCtx context.Context) string {
	jsonBytes, err := common.Marshal(TenantState(tenantCtx).PayMethods)
	if err != nil {
		return "[]"
	}
	return string(jsonBytes)
}

func ContainsPayMethod(tenantCtx context.Context, method string) bool {
	for _, payMethod := range TenantState(tenantCtx).PayMethods {
		if payMethod["type"] == method {
			return true
		}
	}
	return false
}
