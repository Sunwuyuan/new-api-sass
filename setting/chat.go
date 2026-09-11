package setting

import context "context"

import (
	"github.com/QuantumNous/new-api/common"
)

var Chats = []map[string]string{
	//{
	//	"ChatGPT Next Web 官方示例": "https://app.nextchat.dev/#/?settings={\"key\":\"{key}\",\"url\":\"{address}\"}",
	//},
	{
		"Cherry Studio": "cherrystudio://providers/api-keys?v=1&data={cherryConfig}",
	},
	{
		"AionUI": "aionui://provider/add?v=1&data={aionuiConfig}",
	},
	{
		"流畅阅读": "fluentread",
	},
	{
		"CC Switch": "ccswitch",
	},
	{
		"DeepChat": "deepchat://provider/install?v=1&data={deepchatConfig}",
	},
	{
		"AQBot": "aqbot://providers?{aqbotConfig}",
	},
	{
		"Lobe Chat 官方示例": "https://chat-preview.lobehub.com/?settings={\"keyVaults\":{\"openai\":{\"apiKey\":\"{key}\",\"baseURL\":\"{address}/v1\"}}}",
	},
	{
		"AI as Workspace": "https://aiaw.app/set-provider?provider={\"type\":\"openai\",\"settings\":{\"apiKey\":\"{key}\",\"baseURL\":\"{address}/v1\",\"compatibility\":\"strict\"}}",
	},
	{
		"AMA 问天": "ama://set-api-key?server={address}&key={key}",
	},
	{
		"OpenCat": "opencat://team/join?domain={address}&token={key}",
	},
}

func UpdateChatsByJsonString(tenantCtx context.Context, jsonString string) error {
	chats := make([]map[string]string, 0)
	if err := common.Unmarshal([]byte(jsonString), &chats); err != nil {
		return err
	}
	UpdateTenantSettings(tenantCtx, func(state *WorkspaceState) { state.Chats = chats })
	return nil
}

func Chats2JsonString(tenantCtx context.Context) string {
	jsonBytes, err := common.Marshal(TenantState(tenantCtx).Chats)
	if err != nil {
		common.SysLog("error marshalling chats: " + err.Error())
		return "[]"
	}
	return string(jsonBytes)
}
