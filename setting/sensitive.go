package setting

import context "context"

import "strings"

var CheckSensitiveEnabled = true
var CheckSensitiveOnPromptEnabled = true

//var CheckSensitiveOnCompletionEnabled = true

// StopOnSensitiveEnabled 如果检测到敏感词，是否立刻停止生成，否则替换敏感词
var StopOnSensitiveEnabled = true

// StreamCacheQueueLength 流模式缓存队列长度，0表示无缓存
var StreamCacheQueueLength = 0

// SensitiveWords 敏感词
// var SensitiveWords []string
var SensitiveWords = []string{
	"test_sensitive",
}

func SensitiveWordsToString(tenantCtx context.Context) string {
	return strings.Join(TenantState(tenantCtx).SensitiveWords, "\n")
}

func SensitiveWordsFromString(tenantCtx context.Context, s string) {
	words := []string{}
	sw := strings.SplitSeq(s, "\n")
	for w := range sw {
		w = strings.TrimSpace(w)
		if w != "" {
			words = append(words, w)
		}
	}
	UpdateTenantSettings(tenantCtx, func(state *WorkspaceState) { state.SensitiveWords = words })
}

func ShouldCheckPromptSensitive(tenantCtx context.Context) bool {
	return TenantState(tenantCtx).CheckSensitiveEnabled && TenantState(tenantCtx).CheckSensitiveOnPromptEnabled
}

//func ShouldCheckCompletionSensitive() bool {
//	return CheckSensitiveEnabled && CheckSensitiveOnCompletionEnabled
//}
