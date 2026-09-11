package model_setting

import context "context"

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
	"sync"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/config"
)

type ChatCompletionsToResponsesPolicy struct {
	Enabled       bool     `json:"enabled"`
	AllChannels   bool     `json:"all_channels"`
	ChannelIDs    []int    `json:"channel_ids,omitempty"`
	ChannelTypes  []int    `json:"channel_types,omitempty"`
	ModelPatterns []string `json:"model_patterns,omitempty"`
}

func (p ChatCompletionsToResponsesPolicy) IsChannelEnabled(channelID int, channelType int) bool {
	if !p.Enabled {
		return false
	}
	if p.AllChannels {
		return true
	}

	if channelID > 0 && len(p.ChannelIDs) > 0 && slices.Contains(p.ChannelIDs, channelID) {
		return true
	}
	if channelType > 0 && len(p.ChannelTypes) > 0 && slices.Contains(p.ChannelTypes, channelType) {
		return true
	}
	return false
}

type GlobalSettings struct {
	PassThroughRequestEnabled bool     `json:"pass_through_request_enabled"`
	ThinkingModelBlacklist    []string `json:"thinking_model_blacklist"`
	// EffortTailModelIDs lists real model IDs that sit inside the GPT/o-series
	// family whitelist but whose names already end in an effort word.
	EffortTailModelIDs               []string                         `json:"effort_tail_model_ids"`
	ChatCompletionsToResponsesPolicy ChatCompletionsToResponsesPolicy `json:"chat_completions_to_responses_policy"`
}

// 默认配置
var defaultOpenaiSettings = GlobalSettings{
	PassThroughRequestEnabled: false,
	ThinkingModelBlacklist: []string{
		"moonshotai/kimi-k2-thinking",
		"kimi-k2-thinking",
	},
	EffortTailModelIDs: []string{
		"gpt-5.1-codex-max",
		"qwen-image-edit-max",
		"qwen-max",
		"stable-diffusion-3-medium",
		"yi-medium",
	},
	ChatCompletionsToResponsesPolicy: ChatCompletionsToResponsesPolicy{
		Enabled:     false,
		AllChannels: true,
	},
}

// 全局实例
var globalSettings = defaultOpenaiSettings

func init() {
	// 注册到全局配置管理器
	config.GlobalConfig.Register("global", &globalSettings)
}

func GetGlobalSettings(tenantCtx context.Context) *GlobalSettings {
	return config.GlobalConfig.ForTenant(tenantCtx).Get("global").(*GlobalSettings)
}

const thinkingBlacklistRegexPrefix = "re:"

type thinkingBlacklistCompiled struct {
	source  string
	exact   []string
	regexes []*regexp.Regexp
}

var (
	thinkingBlacklistMu    sync.RWMutex
	thinkingBlacklistCache thinkingBlacklistCompiled
)

func thinkingBlacklistSourceKey(entries []string) string {
	return strings.Join(entries, "\x00")
}

func compiledThinkingBlacklist(tenantCtx context.Context) ([]string, []*regexp.Regexp) {
	entries := (*(config.GlobalConfig.ForTenant(tenantCtx).Get("global").(*GlobalSettings))).ThinkingModelBlacklist
	key := thinkingBlacklistSourceKey(entries)

	TenantState(tenantCtx).thinkingBlacklistMu.RLock()
	if TenantState(tenantCtx).thinkingBlacklistCache.source == key {
		exact, regexes := TenantState(tenantCtx).thinkingBlacklistCache.exact, TenantState(tenantCtx).thinkingBlacklistCache.regexes
		TenantState(tenantCtx).thinkingBlacklistMu.RUnlock()
		return exact, regexes
	}
	TenantState(tenantCtx).thinkingBlacklistMu.RUnlock()

	TenantState(tenantCtx).thinkingBlacklistMu.Lock()
	defer TenantState(tenantCtx).thinkingBlacklistMu.Unlock()
	if TenantState(tenantCtx).thinkingBlacklistCache.source == key {
		return TenantState(tenantCtx).thinkingBlacklistCache.exact, TenantState(tenantCtx).thinkingBlacklistCache.regexes
	}

	exact := make([]string, 0, len(entries))
	var regexes []*regexp.Regexp
	for _, entry := range entries {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		if after, ok := strings.CutPrefix(entry, thinkingBlacklistRegexPrefix); ok {
			pattern := after
			if pattern == "" {
				common.SysError(fmt.Sprintf("invalid thinking_model_blacklist regex %q: pattern is empty", entry))
				continue
			}
			re, err := regexp.Compile(pattern)
			if err != nil {
				common.SysError(fmt.Sprintf("invalid thinking_model_blacklist regex %q: %v", entry, err))
				continue
			}
			regexes = append(regexes, re)
			continue
		}
		exact = append(exact, entry)
	}
	TenantState(tenantCtx).thinkingBlacklistCache = thinkingBlacklistCompiled{source: key, exact: exact, regexes: regexes}
	return exact, regexes
}

// ShouldPreserveThinkingSuffix reports whether the full model name is exempt
// from host thinking-suffix and @-modifier parsing. Exact blacklist entries
// match the complete name; entries prefixed with re: are Go regular expressions
// matched with MatchString against the same full name.
func ShouldPreserveThinkingSuffix(tenantCtx context.Context, modelName string) bool {
	target := strings.TrimSpace(modelName)
	if target == "" {
		return false
	}

	exact, regexes := compiledThinkingBlacklist(tenantCtx)
	if slices.Contains(exact, target) {
		return true
	}
	for _, re := range regexes {
		if re.MatchString(target) {
			return true
		}
	}
	return false
}

// ShouldPreserveEffortTail reports whether modelName is a real model ID whose
// name already ends in an effort word. Entries match the complete name and the
// de-namespaced bare name.
func ShouldPreserveEffortTail(tenantCtx context.Context, modelName string) bool {
	target := strings.TrimSpace(modelName)
	if target == "" {
		return false
	}
	bare := target
	if slash := strings.LastIndex(target, "/"); slash >= 0 {
		bare = target[slash+1:]
	}

	for _, entry := range (*(config.GlobalConfig.ForTenant(tenantCtx).Get("global").(*GlobalSettings))).EffortTailModelIDs {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		if entry == target || entry == bare {
			return true
		}
	}
	return false
}
