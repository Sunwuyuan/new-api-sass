package operation_setting

import context "context"

import "strings"

var DemoSiteEnabled = false
var SelfUseModeEnabled = false

var AutomaticDisableKeywords = []string{
	"Your credit balance is too low",
	"This organization has been disabled.",
	"You exceeded your current quota",
	"Permission denied",
	"The security token included in the request is invalid",
	"Operation not allowed",
	"Your account is not authorized",
}

func AutomaticDisableKeywordsToString(tenantCtx context.Context) string {
	return strings.Join(TenantState(tenantCtx).AutomaticDisableKeywords, "\n")
}

func AutomaticDisableKeywordsFromString(tenantCtx context.Context, s string) {
	keywords := []string{}
	ak := strings.SplitSeq(s, "\n")
	for k := range ak {
		k = strings.TrimSpace(k)
		k = strings.ToLower(k)
		if k != "" {
			keywords = append(keywords, k)
		}
	}
	UpdateTenantSettings(tenantCtx, func(state *WorkspaceState) { state.AutomaticDisableKeywords = keywords })
}
