package platform

import (
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm/clause"
)

func (s *Server) beginWeChat(c *gin.Context) {
	if !s.Auth.WeChat.Enabled {
		writeError(c, http.StatusNotFound, "login_method_disabled")
		return
	}
	if !s.authRateLimit(c, "wechat:"+c.ClientIP()) {
		return
	}
	var input struct {
		Redirect string `json:"redirect"`
	}
	if c.ShouldBindJSON(&input) != nil {
		writeError(c, http.StatusBadRequest, "invalid_authentication_request")
		return
	}
	intent := "login"
	if strings.HasSuffix(c.FullPath(), "/link") {
		if !requireRecentSession(c) {
			return
		}
		intent = "link"
	} else if strings.HasSuffix(c.FullPath(), "/verify") {
		intent = "verify"
	}
	token, err := s.createAuthFlow(c, AuthFlow{Purpose: "wechat", Provider: "wechat", Intent: intent, Redirect: input.Redirect}, s.Auth.WeChat.Server)
	if err != nil {
		s.authFlowFailure(c, "wechat")
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "flow_token": token})
}

func (s *Server) finishWeChat(c *gin.Context) {
	if !s.Auth.WeChat.Enabled {
		writeError(c, http.StatusNotFound, "login_method_disabled")
		return
	}
	var input struct {
		FlowToken string `json:"flow_token"`
		Code      string `json:"code"`
	}
	if c.ShouldBindJSON(&input) != nil {
		s.authFlowFailure(c, "wechat")
		return
	}
	flow, err := s.consumeAuthFlow(c, input.FlowToken, "wechat", "wechat")
	input.Code = strings.TrimSpace(input.Code)
	if err != nil || len(input.Code) < 4 || len(input.Code) > 64 || strings.ContainsAny(input.Code, "\r\n") {
		s.authFlowFailure(c, "wechat")
		return
	}
	var server string
	if common.UnmarshalJsonStr(flow.Payload, &server) != nil || server != s.Auth.WeChat.Server {
		s.authFlowFailure(c, "wechat")
		return
	}
	if flow.Intent != "login" {
		if _, err := s.flowSessionUser(c, flow); err != nil {
			s.authFlowFailure(c, "wechat")
			return
		}
	}
	if !s.authRateLimit(c, "wechat-finish:"+c.ClientIP()) {
		return
	}
	// The bridge contract consumes short-lived codes. Also reject reuse across
	// platform replicas throughout the maximum five-minute code lifetime.
	used := AuthFlow{TokenHash: digest("wechat-code:" + server + ":" + input.Code), Purpose: "wechat_used", ExpiresAt: time.Now().UTC().Add(authFlowTTL)}
	result := s.DB.WithContext(c.Request.Context()).Clauses(clause.OnConflict{DoNothing: true}).Create(&used)
	if result.Error != nil || result.RowsAffected != 1 {
		s.authFlowFailure(c, "wechat")
		return
	}
	request, err := http.NewRequestWithContext(c.Request.Context(), http.MethodGet, server+"/api/wechat/user?code="+url.QueryEscape(input.Code), nil)
	if err != nil {
		s.authFlowFailure(c, "wechat")
		return
	}
	request.Header.Set("Authorization", s.Auth.WeChat.Token)
	response, err := s.HTTPClient.Do(request)
	if err != nil {
		s.authFlowFailure(c, "wechat")
		return
	}
	defer response.Body.Close()
	var identity struct {
		Success bool   `json:"success"`
		Data    string `json:"data"`
	}
	if response.StatusCode != http.StatusOK || common.DecodeJson(io.LimitReader(response.Body, 1<<20), &identity) != nil || !identity.Success {
		s.authFlowFailure(c, "wechat")
		return
	}
	user, err := s.externalIdentity(c, flow, digest("wechat:"+server), identity.Data, "WeChat")
	if err != nil {
		s.authFlowFailure(c, "wechat")
		return
	}
	c.Set("platform_auth_redirect", flow.Redirect)
	action := "auth.wechat_login"
	if flow.Intent == "verify" {
		action = "auth.reauthenticate"
	}
	if flow.Intent == "link" {
		action = "auth.wechat_linked"
	}
	s.issueSession(c, user, action)
}
