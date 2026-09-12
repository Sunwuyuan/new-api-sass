package platform

import (
	"context"
	"crypto/subtle"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/QuantumNous/new-api/common"
	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
	"golang.org/x/oauth2"
	"gorm.io/gorm"
)

type OAuthIdentity struct {
	KeyHash       string    `json:"-" gorm:"size:64;primaryKey"`
	UserID        int64     `json:"-" gorm:"not null;uniqueIndex:platform_user_provider"`
	ProviderScope string    `json:"-" gorm:"size:64;not null;uniqueIndex:platform_user_provider"`
	Provider      string    `json:"provider" gorm:"size:48;not null"`
	CreatedAt     time.Time `json:"created_at"`
}

func (OAuthIdentity) TableName() string { return "platform_oauth_identities" }

type oauthFlowPayload struct {
	Verifier   string `json:"verifier"`
	Nonce      string `json:"nonce"`
	ConfigHash string `json:"config_hash"`
	FreshAfter int64  `json:"fresh_after,omitempty"`
}

func (s *Server) oidcProvider(ctx context.Context, p OAuthProvider) (*oidc.Provider, error) {
	s.providerMu.Lock()
	provider := s.providers[p.Issuer]
	s.providerMu.Unlock()
	if provider != nil {
		return provider, nil
	}
	provider, err := oidc.NewProvider(oidc.ClientContext(ctx, s.HTTPClient), p.Issuer)
	if err != nil {
		return nil, err
	}
	endpoint := provider.Endpoint()
	var discovery struct {
		JWKSURL string `json:"jwks_uri"`
	}
	if provider.Claims(&discovery) != nil || !safeEndpoint(discovery.JWKSURL) || !safeEndpoint(endpoint.AuthURL) || !safeEndpoint(endpoint.TokenURL) {
		return nil, errAuthFlow
	}
	s.providerMu.Lock()
	s.providers[p.Issuer] = provider
	s.providerMu.Unlock()
	return provider, nil
}

func (s *Server) oauthClient(ctx context.Context, p OAuthProvider) (*oauth2.Config, error) {
	endpoint := oauth2.Endpoint{AuthURL: p.Authorization, TokenURL: p.Token, AuthStyle: oauth2.AuthStyleInHeader}
	if p.Issuer != "" {
		provider, err := s.oidcProvider(ctx, p)
		if err != nil {
			return nil, err
		}
		endpoint = provider.Endpoint()
		endpoint.AuthStyle = oauth2.AuthStyleInHeader
	}
	if p.AuthMethod == "client_secret_post" {
		endpoint.AuthStyle = oauth2.AuthStyleInParams
	}
	return &oauth2.Config{ClientID: p.ClientID, ClientSecret: p.ClientSecret, RedirectURL: s.Origin + "/platform/oauth/" + p.Slug, Endpoint: endpoint, Scopes: p.Scopes}, nil
}

func (s *Server) beginOAuth(c *gin.Context) {
	p, exists := s.snapshotAuth().Providers[c.Param("provider")]
	if !exists {
		writeError(c, http.StatusNotFound, "login_method_disabled")
		return
	}
	if !s.authRateLimit(c, "oauth:"+p.Slug+":"+c.ClientIP()) {
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
		// OAuth userinfo alone cannot prove when the user authenticated.
		// Fresh external proof requires a verified OIDC auth_time claim.
		if p.Issuer == "" {
			writeError(c, http.StatusBadRequest, "reauthentication_method_unavailable")
			return
		}
	}
	client, err := s.oauthClient(c.Request.Context(), p)
	if err != nil {
		s.authFlowFailure(c, p.Slug)
		return
	}
	encoded, err := common.Marshal(p)
	if err != nil {
		s.authFlowFailure(c, p.Slug)
		return
	}
	nonce, err := randomSecret()
	if err != nil {
		s.authFlowFailure(c, p.Slug)
		return
	}
	payload := oauthFlowPayload{Verifier: oauth2.GenerateVerifier(), Nonce: nonce, ConfigHash: digest(string(encoded))}
	if intent == "verify" {
		payload.FreshAfter = time.Now().UTC().Unix()
	}
	state, err := s.createAuthFlow(c, AuthFlow{Purpose: "oauth", Provider: p.Slug, Intent: intent, Redirect: input.Redirect}, payload)
	if err != nil {
		s.authFlowFailure(c, p.Slug)
		return
	}
	options := []oauth2.AuthCodeOption{oauth2.S256ChallengeOption(payload.Verifier)}
	if p.Issuer != "" {
		options = append(options, oauth2.SetAuthURLParam("nonce", nonce))
	}
	if intent == "verify" && p.Issuer != "" {
		options = append(options, oauth2.SetAuthURLParam("prompt", "login"), oauth2.SetAuthURLParam("max_age", "0"))
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "authorization_url": client.AuthCodeURL(state, options...)})
}

func (s *Server) finishOAuth(c *gin.Context) {
	p, exists := s.snapshotAuth().Providers[c.Param("provider")]
	if !exists {
		writeError(c, http.StatusNotFound, "login_method_disabled")
		return
	}
	var input struct {
		State string `json:"state"`
		Code  string `json:"code"`
		Error string `json:"error"`
	}
	if c.ShouldBindJSON(&input) != nil || len(input.Code) > 4096 {
		s.authFlowFailure(c, p.Slug)
		return
	}
	flow, err := s.consumeAuthFlow(c, input.State, "oauth", p.Slug)
	if err != nil || input.Code == "" || input.Error != "" {
		s.authFlowFailure(c, p.Slug)
		return
	}
	var payload oauthFlowPayload
	encoded, encodeErr := common.Marshal(p)
	if common.UnmarshalJsonStr(flow.Payload, &payload) != nil || encodeErr != nil || payload.ConfigHash != digest(string(encoded)) {
		s.authFlowFailure(c, p.Slug)
		return
	}
	if flow.Intent != "login" {
		if _, err := s.flowSessionUser(c, flow); err != nil {
			s.authFlowFailure(c, p.Slug)
			return
		}
	}
	if !s.authRateLimit(c, "oauth-finish:"+p.Slug+":"+c.ClientIP()) {
		return
	}
	subject, name, err := s.exchangeOAuthIdentity(c.Request.Context(), p, input.Code, payload)
	if err != nil {
		s.authFlowFailure(c, p.Slug)
		return
	}
	user, err := s.externalIdentity(c, flow, p.scope(), subject, name)
	if err != nil {
		s.authFlowFailure(c, p.Slug)
		return
	}
	c.Set("platform_auth_redirect", flow.Redirect)
	action := "auth.oauth_login"
	if flow.Intent == "verify" {
		action = "auth.reauthenticate"
	}
	if flow.Intent == "link" {
		action = "auth.oauth_linked"
	}
	s.issueSession(c, user, action)
}

func (s *Server) exchangeOAuthIdentity(ctx context.Context, p OAuthProvider, code string, payload oauthFlowPayload) (string, string, error) {
	ctx = oidc.ClientContext(ctx, s.HTTPClient)
	client, err := s.oauthClient(ctx, p)
	if err != nil {
		return "", "", err
	}
	token, err := client.Exchange(ctx, code, oauth2.VerifierOption(payload.Verifier))
	if err != nil {
		return "", "", errAuthFlow
	}
	if p.Issuer != "" {
		rawIDToken, ok := token.Extra("id_token").(string)
		if !ok {
			return "", "", errAuthFlow
		}
		provider, err := s.oidcProvider(ctx, p)
		if err != nil {
			return "", "", err
		}
		verified, err := provider.Verifier(&oidc.Config{ClientID: p.ClientID}).Verify(ctx, rawIDToken)
		if err != nil || verified.Subject == "" || verified.IssuedAt.IsZero() || verified.IssuedAt.After(time.Now().Add(time.Minute)) || subtle.ConstantTimeCompare([]byte(verified.Nonce), []byte(payload.Nonce)) != 1 {
			return "", "", errAuthFlow
		}
		var claims struct {
			Name            string `json:"name"`
			Username        string `json:"preferred_username"`
			AuthorizedParty string `json:"azp"`
			AuthTime        int64  `json:"auth_time"`
		}
		if verified.Claims(&claims) != nil || (len(verified.Audience) > 1 && claims.AuthorizedParty != p.ClientID) || (claims.AuthorizedParty != "" && claims.AuthorizedParty != p.ClientID) {
			return "", "", errAuthFlow
		}
		if payload.FreshAfter > 0 && (claims.AuthTime < payload.FreshAfter-30 || claims.AuthTime > time.Now().Add(30*time.Second).Unix()) {
			return "", "", errAuthFlow
		}
		name := claims.Name
		if name == "" {
			name = claims.Username
		}
		return verified.Subject, name, nil
	}
	if token.AccessToken == "" || !strings.EqualFold(token.Type(), "Bearer") {
		return "", "", errAuthFlow
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, p.UserInfo, nil)
	if err != nil {
		return "", "", err
	}
	request.Header.Set("Authorization", "Bearer "+token.AccessToken)
	request.Header.Set("Accept", "application/json")
	response, err := s.HTTPClient.Do(request)
	if err != nil {
		return "", "", errAuthFlow
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return "", "", errAuthFlow
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil || !gjson.ValidBytes(body) {
		return "", "", errAuthFlow
	}
	id := gjson.GetBytes(body, p.SubjectPath)
	if id.Type != gjson.String && id.Type != gjson.Number {
		return "", "", errAuthFlow
	}
	subject := id.Str
	if id.Type == gjson.Number {
		subject = id.Raw
		for _, digit := range subject {
			if digit < '0' || digit > '9' {
				return "", "", errAuthFlow
			}
		}
		if subject == "0" {
			return "", "", errAuthFlow
		}
	}
	if p.Slug == "linuxdo" && (!gjson.GetBytes(body, "active").Bool() || gjson.GetBytes(body, "silenced").Bool()) {
		return "", "", errAuthFlow
	}
	return subject, gjson.GetBytes(body, p.NamePath).String(), nil
}

// Provider subjects form a separate namespace. Email claims never merge an
// account: linking requires the existing platform session and fresh proof.
func (s *Server) externalIdentity(c *gin.Context, flow AuthFlow, scope, subject, name string) (User, error) {
	if subject == "" || len(subject) > 512 || !utf8.ValidString(subject) {
		return User{}, errAuthFlow
	}
	name = strings.TrimSpace(name)
	if name == "" {
		name = flow.Provider
	}
	if !utf8.ValidString(name) {
		return User{}, errAuthFlow
	}
	if utf8.RuneCountInString(name) > 128 {
		name = string([]rune(name)[:128])
	}
	var user User
	var err error
	if flow.Intent != "login" {
		user, err = s.flowSessionUser(c, flow)
		if err != nil {
			return User{}, err
		}
	}
	key := digest(scope + "\x00" + subject)
	err = s.DB.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		var identity OAuthIdentity
		err := tx.First(&identity, "key_hash = ?", key).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		found := err == nil
		if flow.Intent != "login" {
			user, err = activeUser(tx, user)
			if err != nil {
				return err
			}
			if found && identity.UserID != user.ID {
				return errAuthFlow
			}
			if flow.Intent == "verify" {
				if !found {
					return errAuthFlow
				}
				return nil
			}
			if flow.Intent != "link" {
				return errAuthFlow
			}
			if found {
				return nil
			}
		} else if found {
			return tx.First(&user, identity.UserID).Error
		} else {
			auth := s.snapshotAuth()
			if !auth.Registration || !auth.OAuthRegistration {
				return errAuthFlow
			}
			verified := time.Now().UTC()
			user = User{DisplayName: name, Role: "user", Status: "active", SessionVersion: 1, EmailVerifiedAt: &verified}
			if err := tx.Create(&user).Error; err != nil {
				return err
			}
			if err := audit(tx, user.ID, "auth.oauth_registered", user.ID, gin.H{"provider": flow.Provider}); err != nil {
				return err
			}
		}
		identity = OAuthIdentity{KeyHash: key, UserID: user.ID, ProviderScope: scope, Provider: flow.Provider}
		if err := tx.Create(&identity).Error; err != nil {
			return err
		}
		return audit(tx, user.ID, "auth.identity_linked", user.ID, gin.H{"provider": flow.Provider})
	})
	if err != nil || user.Status != "active" {
		return User{}, errAuthFlow
	}
	return user, nil
}
