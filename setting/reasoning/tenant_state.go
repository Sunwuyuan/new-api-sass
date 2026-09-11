// TenantState keeps mutable workspace settings and caches isolated.
package reasoning

import (
	context "context"
	tenant "github.com/QuantumNous/new-api/tenant"
)

type WorkspaceState struct {
	DeepSeekV4EffortSuffixes      []string
	EffortSuffixes                []string
	OpenAIEffortSuffixes          []string
	ParseDeepSeekV4ThinkingSuffix func(modelName string) (baseModel string, thinkingType string, effort string, ok bool)
	TrimEffortSuffixWithSuffixes  func(modelName string, suffixes []string) (string, string, bool)
	TrimGeminiThinkingSuffix      func(modelName string) (string, bool)
}

var workspaceStates tenant.Registry[*WorkspaceState]

func TenantState(ctx context.Context) *WorkspaceState {
	value, err := workspaceStates.Get(ctx, func() *WorkspaceState {
		return &WorkspaceState{
			DeepSeekV4EffortSuffixes:      tenant.Clone(DeepSeekV4EffortSuffixes),
			EffortSuffixes:                tenant.Clone(EffortSuffixes),
			OpenAIEffortSuffixes:          tenant.Clone(OpenAIEffortSuffixes),
			ParseDeepSeekV4ThinkingSuffix: tenant.Clone(ParseDeepSeekV4ThinkingSuffix),
			TrimEffortSuffixWithSuffixes:  tenant.Clone(TrimEffortSuffixWithSuffixes),
			TrimGeminiThinkingSuffix:      tenant.Clone(TrimGeminiThinkingSuffix),
		}
	})
	if err != nil {
		panic(err)
	}
	return value
}
