package platform

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/tenant"
	"github.com/gin-gonic/gin"
	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
	"gorm.io/gorm"
)

type PasskeyCredential struct {
	KeyHash   string     `json:"id" gorm:"size:64;primaryKey"`
	UserID    int64      `json:"-" gorm:"not null;index"`
	RPID      string     `json:"-" gorm:"size:253;not null"`
	Data      string     `json:"-" gorm:"type:text;not null"`
	Version   int64      `json:"-" gorm:"not null"`
	CreatedAt time.Time  `json:"created_at"`
	LastUsed  *time.Time `json:"last_used_at"`
}

func (PasskeyCredential) TableName() string { return "platform_passkeys" }

type passkeyUser struct {
	User        User
	Credentials []webauthn.Credential
}

// A workspace user with the same numeric ID is a different WebAuthn user.
func (u passkeyUser) WebAuthnID() []byte {
	return []byte("new-api:platform:" + strconv.FormatInt(u.User.ID, 10))
}
func (u passkeyUser) WebAuthnName() string                       { return u.User.AccountName() }
func (u passkeyUser) WebAuthnDisplayName() string                { return u.User.AccountName() }
func (u passkeyUser) WebAuthnCredentials() []webauthn.Credential { return u.Credentials }

func (s *Server) configurePasskey() error {
	if !s.Auth.Passkey {
		return nil
	}
	origin, err := url.Parse(s.Origin)
	if err != nil {
		return err
	}
	s.Passkey, err = webauthn.New(&webauthn.Config{
		RPID: origin.Hostname(), RPDisplayName: s.Auth.Name, RPOrigins: []string{s.Origin}, RPTopOrigins: []string{s.Origin},
		RPTopOriginVerificationMode: protocol.TopOriginExplicitVerificationMode,
		AttestationPreference:       protocol.PreferNoAttestation,
		AuthenticatorSelection:      protocol.AuthenticatorSelection{UserVerification: protocol.VerificationRequired, ResidentKey: protocol.ResidentKeyRequirementRequired},
		Timeouts: webauthn.TimeoutsConfig{
			Login:        webauthn.TimeoutConfig{Enforce: true, Timeout: authFlowTTL},
			Registration: webauthn.TimeoutConfig{Enforce: true, Timeout: authFlowTTL},
		},
	})
	return err
}

func (s *Server) passkeyEnabled(c *gin.Context) bool {
	if !s.Auth.Passkey || s.Passkey == nil {
		writeError(c, http.StatusNotFound, "login_method_disabled")
		return false
	}
	return true
}

func (s *Server) beginPasskeyLogin(c *gin.Context) {
	if !s.passkeyEnabled(c) || !s.authRateLimit(c, "passkey:"+c.ClientIP()) {
		return
	}
	intent, purpose := "login", "passkey_login"
	if strings.Contains(c.FullPath(), "/verify/") {
		intent, purpose = "verify", "passkey_verify"
	}
	options, session, err := s.Passkey.BeginDiscoverableLogin(webauthn.WithUserVerification(protocol.VerificationRequired))
	if err != nil {
		s.authFlowFailure(c, "passkey")
		return
	}
	token, err := s.createAuthFlow(c, AuthFlow{Purpose: purpose, Provider: "passkey", Intent: intent}, session)
	if err != nil {
		s.authFlowFailure(c, "passkey")
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "flow_token": token, "options": options, "rp_ids": []string{s.Passkey.Config.RPID}})
}

type passkeyFinishRequest struct {
	FlowToken  string          `json:"flow_token"`
	Credential json.RawMessage `json:"credential"`
}

func (s *Server) finishPasskeyLogin(c *gin.Context) {
	if !s.passkeyEnabled(c) {
		return
	}
	var input passkeyFinishRequest
	if c.ShouldBindJSON(&input) != nil {
		s.authFlowFailure(c, "passkey")
		return
	}
	purpose := "passkey_login"
	if strings.Contains(c.FullPath(), "/verify/") {
		purpose = "passkey_verify"
	}
	flow, err := s.consumeAuthFlow(c, input.FlowToken, purpose, "passkey")
	if err != nil {
		s.authFlowFailure(c, "passkey")
		return
	}
	if flow.Intent == "verify" {
		if _, err := s.flowSessionUser(c, flow); err != nil {
			s.authFlowFailure(c, "passkey")
			return
		}
	}
	var session webauthn.SessionData
	if common.UnmarshalJsonStr(flow.Payload, &session) != nil || session.UserVerification != protocol.VerificationRequired || session.RelyingPartyID != s.Passkey.Config.RPID {
		s.authFlowFailure(c, "passkey")
		return
	}
	parsed, err := protocol.ParseCredentialRequestResponseBytes(input.Credential)
	if err != nil || parsed.Response.CollectedClientData.CrossOrigin {
		s.authFlowFailure(c, "passkey")
		return
	}
	var stored PasskeyCredential
	var user User
	lookup := func(rawID, handle []byte) (webauthn.User, error) {
		db := s.DB.WithContext(c.Request.Context())
		if db.Where("key_hash = ? AND rp_id = ?", digest(string(rawID)), s.Passkey.Config.RPID).First(&stored).Error != nil || db.First(&user, stored.UserID).Error != nil || user.Status != "active" {
			return nil, errAuthFlow
		}
		wrapper := passkeyUser{User: user}
		if !bytes.Equal(handle, wrapper.WebAuthnID()) || flow.Intent == "verify" && flow.UserID != user.ID {
			return nil, errAuthFlow
		}
		var credential webauthn.Credential
		if common.UnmarshalJsonStr(stored.Data, &credential) != nil {
			return nil, errAuthFlow
		}
		wrapper.Credentials = []webauthn.Credential{credential}
		return wrapper, nil
	}
	_, credential, err := s.Passkey.ValidatePasskeyLogin(lookup, session, parsed)
	if err != nil || credential.Authenticator.CloneWarning {
		s.authFlowFailure(c, "passkey")
		return
	}
	encoded, err := common.Marshal(credential)
	if err != nil {
		s.authFlowFailure(c, "passkey")
		return
	}
	err = s.DB.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		if _, err := activeUser(tx, user); err != nil {
			return err
		}
		result := tx.Model(&PasskeyCredential{}).Where("key_hash = ? AND user_id = ? AND version = ?", stored.KeyHash, user.ID, stored.Version).
			Updates(map[string]any{"data": string(encoded), "last_used": time.Now().UTC(), "version": gorm.Expr("version + 1")})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return errAuthFlow
		}
		return nil
	})
	if err != nil {
		s.authFlowFailure(c, "passkey")
		return
	}
	action := "auth.passkey_login"
	if flow.Intent == "verify" {
		action = "auth.reauthenticate"
	}
	s.issueSession(c, user, action)
}

func (s *Server) beginPasskeyRegistration(c *gin.Context) {
	if !s.passkeyEnabled(c) || !requireRecentSession(c) {
		return
	}
	user := c.MustGet("platform_user").(User)
	if !s.authRateLimit(c, "passkey-register:"+strconv.FormatInt(user.ID, 10)) {
		return
	}
	var records []PasskeyCredential
	if err := s.DB.WithContext(c.Request.Context()).Where("user_id = ? AND rp_id = ?", user.ID, s.Passkey.Config.RPID).Find(&records).Error; err != nil {
		s.authFlowFailure(c, "passkey")
		return
	}
	if len(records) >= 10 {
		writeError(c, http.StatusBadRequest, "passkey_limit_reached")
		return
	}
	credentials := make([]webauthn.Credential, 0, len(records))
	for _, record := range records {
		var credential webauthn.Credential
		if common.UnmarshalJsonStr(record.Data, &credential) != nil {
			s.authFlowFailure(c, "passkey")
			return
		}
		credentials = append(credentials, credential)
	}
	options, session, err := s.Passkey.BeginRegistration(passkeyUser{User: user, Credentials: credentials},
		webauthn.WithResidentKeyRequirement(protocol.ResidentKeyRequirementRequired),
		webauthn.WithExclusions(webauthn.Credentials(credentials).CredentialDescriptors()))
	if err != nil {
		s.authFlowFailure(c, "passkey")
		return
	}
	token, err := s.createAuthFlow(c, AuthFlow{Purpose: "passkey_register", Provider: "passkey", Intent: "register"}, session)
	if err != nil {
		s.authFlowFailure(c, "passkey")
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "flow_token": token, "options": options})
}

func (s *Server) finishPasskeyRegistration(c *gin.Context) {
	if !s.passkeyEnabled(c) || !requireRecentSession(c) {
		return
	}
	var input passkeyFinishRequest
	if c.ShouldBindJSON(&input) != nil {
		s.authFlowFailure(c, "passkey")
		return
	}
	flow, err := s.consumeAuthFlow(c, input.FlowToken, "passkey_register", "passkey")
	if err != nil {
		s.authFlowFailure(c, "passkey")
		return
	}
	user, err := s.flowSessionUser(c, flow)
	if err != nil {
		s.authFlowFailure(c, "passkey")
		return
	}
	var session webauthn.SessionData
	if common.UnmarshalJsonStr(flow.Payload, &session) != nil || session.UserVerification != protocol.VerificationRequired || session.RelyingPartyID != s.Passkey.Config.RPID {
		s.authFlowFailure(c, "passkey")
		return
	}
	parsed, err := protocol.ParseCredentialCreationResponseBytes(input.Credential)
	if err != nil || parsed.Response.CollectedClientData.CrossOrigin {
		s.authFlowFailure(c, "passkey")
		return
	}
	credential, err := s.Passkey.CreateCredential(passkeyUser{User: user}, session, parsed)
	if err != nil {
		s.authFlowFailure(c, "passkey")
		return
	}
	encoded, err := common.Marshal(credential)
	if err != nil {
		s.authFlowFailure(c, "passkey")
		return
	}
	err = s.DB.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		if _, err := activeUser(tx, user); err != nil {
			return err
		}
		var count int64
		if tx.Model(&PasskeyCredential{}).Where("user_id = ?", user.ID).Count(&count).Error != nil || count >= 10 {
			return errAuthFlow
		}
		entry := PasskeyCredential{KeyHash: digest(string(credential.ID)), UserID: user.ID, RPID: s.Passkey.Config.RPID, Data: string(encoded), Version: 1}
		if err := tx.Create(&entry).Error; err != nil {
			return err
		}
		return audit(tx, user.ID, "auth.passkey_registered", user.ID, nil)
	})
	if err != nil {
		s.authFlowFailure(c, "passkey")
		return
	}
	s.issueSession(c, user, "auth.passkey_added")
}

func (s *Server) deletePasskey(c *gin.Context) {
	if !s.passkeyEnabled(c) || !requireRecentSession(c) {
		return
	}
	user := c.MustGet("platform_user").(User)
	id := c.Param("id")
	if len(id) != 64 {
		writeError(c, http.StatusBadRequest, "invalid_authentication_request")
		return
	}
	err := s.DB.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		if _, err := activeUser(tx, user); err != nil {
			return err
		}
		var passkeys int64
		if err := tx.Model(&PasskeyCredential{}).Where("user_id = ? AND rp_id = ?", user.ID, s.Passkey.Config.RPID).Count(&passkeys).Error; err != nil {
			return err
		}
		if passkeys <= 1 && !(s.Auth.PasswordLogin && user.PasswordHash != "") {
			var identities int64
			if err := tx.Model(&OAuthIdentity{}).Where("user_id = ? AND provider_scope IN ?", user.ID, s.identityScopes()).Count(&identities).Error; err != nil {
				return err
			}
			if identities == 0 {
				return &tenant.HTTPError{Status: http.StatusBadRequest, Code: "last_login_method"}
			}
		}
		result := tx.Where("key_hash = ? AND user_id = ? AND rp_id = ?", id, user.ID, s.Passkey.Config.RPID).Delete(&PasskeyCredential{})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return gorm.ErrRecordNotFound
		}
		if err := tx.Model(&user).UpdateColumn("session_version", gorm.Expr("session_version + 1")).Error; err != nil {
			return err
		}
		user.SessionVersion++
		if err := tx.Where("user_id = ?", user.ID).Delete(&Session{}).Error; err != nil {
			return err
		}
		return audit(tx, user.ID, "auth.passkey_deleted", user.ID, nil)
	})
	if err != nil {
		transactionError(c, err)
		return
	}
	s.issueSession(c, user, "auth.credentials_rotated")
}

func (s *Server) identityScopes() []string {
	scopes := make([]string, 0, len(s.Auth.Providers)+1)
	for _, p := range s.Auth.Providers {
		scopes = append(scopes, p.scope())
	}
	if s.Auth.WeChat.Enabled {
		scopes = append(scopes, digest("wechat:"+s.Auth.WeChat.Server))
	}
	return scopes
}

func (s *Server) authMethods(c *gin.Context) {
	user := c.MustGet("platform_user").(User)
	var identities []OAuthIdentity
	passkeys := make([]PasskeyCredential, 0)
	db := s.DB.WithContext(c.Request.Context())
	if db.Where("user_id = ? AND provider_scope IN ?", user.ID, s.identityScopes()).Find(&identities).Error != nil {
		writeError(c, http.StatusServiceUnavailable, "platform_unavailable")
		return
	}
	if s.Passkey != nil && db.Where("user_id = ? AND rp_id = ?", user.ID, s.Passkey.Config.RPID).Order("created_at").Find(&passkeys).Error != nil {
		writeError(c, http.StatusServiceUnavailable, "platform_unavailable")
		return
	}
	linked := make([]string, 0, len(identities))
	for _, identity := range identities {
		linked = append(linked, identity.Provider)
	}
	slices.Sort(linked)
	c.JSON(http.StatusOK, gin.H{"success": true, "providers": linked, "passkeys": passkeys, "has_password": user.PasswordHash != ""})
}
