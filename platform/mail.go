package platform

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/mail"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/plan"
	"github.com/QuantumNous/new-api/tenant"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const defaultAmailURL = "https://amail-service.192325.xyz"

type MailSettings struct {
	Enabled           bool   `json:"enabled"`
	BaseURL           string `json:"base_url"`
	APIKey            string `json:"api_key"`
	ProviderID        string `json:"provider_id"`
	From              string `json:"from"`
	FromName          string `json:"from_name"`
	EmailVerification bool   `json:"email_verification"`
	Notifications     bool   `json:"notifications"`
}

type EmailChallenge struct {
	KeyHash   string    `gorm:"size:64;primaryKey"`
	CodeHash  string    `gorm:"size:64;not null"`
	ExpiresAt time.Time `gorm:"not null;index"`
	Attempts  int       `gorm:"not null"`
}

func (EmailChallenge) TableName() string { return "platform_email_challenges" }

func defaultMailSettings() MailSettings {
	return MailSettings{BaseURL: defaultAmailURL, ProviderID: "auto", FromName: "New API SaaS"}
}

func (s *Server) sendWorkspaceMail(tenantCtx context.Context, subject, receiver, content string) error {
	s.authMu.Lock()
	mailCfg := s.Mail
	s.authMu.Unlock()
	view, err := plan.Current(tenantCtx, s.DB)
	if err != nil {
		return err
	}
	if !mailCfg.Enabled || !view.Capabilities.PlatformEmail {
		return errors.New("platform mail is not available on this hosting plan")
	}
	complete, err := plan.ReserveEmail(tenantCtx, s.DB, view.Limits.Emails, time.Now())
	if err != nil {
		return err
	}
	sent := false
	defer func() {
		if complete != nil {
			_ = complete(sent)
		}
	}()
	if err := s.deliverMail(tenantCtx, mailCfg, receiver, subject, content); err != nil {
		return err
	}
	sent = true
	return nil
}

func (s *Server) sendPlatformMail(ctx context.Context, to, subject, html string) error {
	s.authMu.Lock()
	mailCfg := s.Mail
	s.authMu.Unlock()
	if !mailCfg.Enabled {
		return errors.New("platform mail is not configured")
	}
	return s.deliverMail(ctx, mailCfg, to, subject, html)
}

func (s *Server) deliverMail(ctx context.Context, cfg MailSettings, to, subject, html string) error {
	if strings.TrimSpace(cfg.APIKey) == "" {
		return errors.New("platform mail is not configured")
	}
	base := strings.TrimRight(cfg.BaseURL, "/")
	if base == "" {
		base = defaultAmailURL
	}
	if !safeEndpoint(base) {
		return errors.New("platform mail endpoint is invalid")
	}
	from := strings.TrimSpace(cfg.From)
	if from == "" {
		return errors.New("platform mail from address is required")
	}
	fromHeader := from
	if name := strings.TrimSpace(cfg.FromName); name != "" {
		fromHeader = name + " <" + from + ">"
	}
	provider := strings.TrimSpace(cfg.ProviderID)
	if provider == "" {
		provider = "auto"
	}
	body, err := common.Marshal(gin.H{
		"provider_id": provider,
		"from":        fromHeader,
		"to":          []string{to},
		"subject":     subject,
		"html":        html,
	})
	if err != nil {
		return err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/emails", bytes.NewReader(body))
	if err != nil {
		return err
	}
	request.Header.Set("Authorization", "Bearer "+cfg.APIKey)
	request.Header.Set("Content-Type", "application/json")
	response, err := s.HTTPClient.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	payload, _ := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		message := strings.TrimSpace(string(payload))
		if message == "" {
			message = response.Status
		}
		return fmt.Errorf("platform mail delivery failed: %s", message)
	}
	return nil
}

func (s *Server) notify(ctx context.Context, to, subject, html string) {
	s.authMu.Lock()
	enabled := s.Mail.Enabled && s.Mail.Notifications && to != ""
	s.authMu.Unlock()
	if !enabled {
		return
	}
	if err := s.sendPlatformMail(ctx, to, subject, html); err != nil {
		common.SysError("platform notification failed: " + err.Error())
	}
}

func sixDigitCode() (string, error) {
	var raw [4]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	return fmt.Sprintf("%06d", binary.BigEndian.Uint32(raw[:])%1000000), nil
}

func (s *Server) issueEmailChallenge(c *gin.Context, purpose, email string) error {
	code, err := sixDigitCode()
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	challenge := EmailChallenge{KeyHash: digest(purpose + ":" + email), CodeHash: digest(code), ExpiresAt: now.Add(15 * time.Minute)}
	if err := s.DB.WithContext(c.Request.Context()).Save(&challenge).Error; err != nil {
		return err
	}
	s.authMu.Lock()
	name := s.Auth.Name
	s.authMu.Unlock()
	subject := name + " email verification"
	content := fmt.Sprintf("<p>Your verification code is <strong>%s</strong>.</p><p>It expires in 15 minutes. If you did not request this, ignore the message.</p>", code)
	if purpose == "email-change" {
		subject = name + " email change confirmation"
		content = fmt.Sprintf("<p>Your verification code is <strong>%s</strong>.</p><p>It expires in 15 minutes. Someone is changing the email address of your account to this address. If you did not request this, ignore the message.</p>", code)
	}
	return s.sendPlatformMail(c.Request.Context(), email, subject, content)
}

func (s *Server) consumeEmailChallenge(c *gin.Context, purpose, email, code string) error {
	if utf8.RuneCountInString(code) != 6 {
		return errAuthFlow
	}
	now := time.Now().UTC()
	return s.DB.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		var challenge EmailChallenge
		if err := lockForUpdate(tx).First(&challenge, "key_hash = ?", digest(purpose+":"+email)).Error; err != nil {
			return errAuthFlow
		}
		if !challenge.ExpiresAt.After(now) || challenge.Attempts >= 5 {
			_ = tx.Delete(&challenge).Error
			return errAuthFlow
		}
		if err := tx.Model(&challenge).Update("attempts", gorm.Expr("attempts + 1")).Error; err != nil {
			return err
		}
		if challenge.CodeHash != digest(code) {
			return errAuthFlow
		}
		return tx.Delete(&challenge).Error
	})
}

func (s *Server) verifyEmail(c *gin.Context) {
	var input struct {
		Email string `json:"email"`
		Code  string `json:"code"`
	}
	if c.ShouldBindJSON(&input) != nil {
		writeError(c, http.StatusBadRequest, "invalid_authentication_request")
		return
	}
	address, err := mail.ParseAddress(strings.TrimSpace(input.Email))
	if err != nil {
		s.authFlowFailure(c, "email")
		return
	}
	email := strings.ToLower(address.Address)
	if !s.authRateLimit(c, "email-verify:"+email) {
		return
	}
	if s.consumeEmailChallenge(c, "email", email, strings.TrimSpace(input.Code)) != nil {
		s.authFlowFailure(c, "email")
		return
	}
	now := time.Now().UTC()
	result := s.DB.WithContext(c.Request.Context()).Model(&User{}).
		Where("email = ? AND email_verified_at IS NULL", email).
		Update("email_verified_at", now)
	if result.Error != nil || result.RowsAffected != 1 {
		s.authFlowFailure(c, "email")
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (s *Server) resendVerification(c *gin.Context) {
	var input struct {
		Email string `json:"email"`
	}
	if c.ShouldBindJSON(&input) != nil {
		writeError(c, http.StatusBadRequest, "invalid_authentication_request")
		return
	}
	address, err := mail.ParseAddress(strings.TrimSpace(input.Email))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": true})
		return
	}
	email := strings.ToLower(address.Address)
	if !s.authRateLimit(c, "email-resend:"+email) {
		return
	}
	s.authMu.Lock()
	enabled := s.Mail.Enabled && s.Mail.EmailVerification
	s.authMu.Unlock()
	var user User
	if enabled && s.DB.WithContext(c.Request.Context()).Where("email = ?", email).First(&user).Error == nil && user.EmailVerifiedAt == nil {
		if err := s.issueEmailChallenge(c, "email", email); err != nil {
			common.SysError("platform verification email failed: " + err.Error())
		}
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (s *Server) mailPublicStatus() gin.H {
	s.authMu.Lock()
	defer s.authMu.Unlock()
	return gin.H{
		"enabled":            s.Mail.Enabled,
		"email_verification": s.Mail.Enabled && s.Mail.EmailVerification,
		"notifications":      s.Mail.Enabled && s.Mail.Notifications,
	}
}

func parseEmailInput(value string) (string, bool) {
	trimmed := strings.TrimSpace(value)
	address, err := mail.ParseAddress(trimmed)
	if err != nil || address.Address != trimmed || len(trimmed) > 254 {
		return "", false
	}
	return strings.ToLower(address.Address), true
}

// Changing the account email requires a recent session and proof of ownership
// of the new address; the old address is notified after the change.
func (s *Server) startEmailChange(c *gin.Context) {
	if !requireRecentSession(c) {
		return
	}
	user := c.MustGet("platform_user").(User)
	s.authMu.Lock()
	enabled := s.Mail.Enabled
	s.authMu.Unlock()
	if !enabled {
		writeError(c, http.StatusBadRequest, "platform_mail_unavailable")
		return
	}
	var input struct {
		Email string `json:"email"`
	}
	email, ok := "", false
	if c.ShouldBindJSON(&input) == nil {
		email, ok = parseEmailInput(input.Email)
	}
	if !ok {
		writeError(c, http.StatusBadRequest, "invalid_email")
		return
	}
	if user.Email != nil && *user.Email == email {
		writeError(c, http.StatusBadRequest, "email_unchanged")
		return
	}
	if !s.authRateLimit(c, "email-change:"+strconv.FormatInt(user.ID, 10)) {
		return
	}
	var taken int64
	if s.DB.WithContext(c.Request.Context()).Model(&User{}).Where("email = ?", email).Count(&taken).Error != nil {
		writeError(c, http.StatusServiceUnavailable, "platform_unavailable")
		return
	}
	if taken > 0 {
		writeError(c, http.StatusConflict, "email_taken")
		return
	}
	if err := s.issueEmailChallenge(c, "email-change", email); err != nil {
		common.SysError("platform email change mail failed: " + err.Error())
		writeError(c, http.StatusServiceUnavailable, "platform_mail_unavailable")
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (s *Server) finishEmailChange(c *gin.Context) {
	if !requireRecentSession(c) {
		return
	}
	user := c.MustGet("platform_user").(User)
	var input struct {
		Email string `json:"email"`
		Code  string `json:"code"`
	}
	email, ok := "", false
	if c.ShouldBindJSON(&input) == nil {
		email, ok = parseEmailInput(input.Email)
	}
	if !ok {
		writeError(c, http.StatusBadRequest, "invalid_email")
		return
	}
	if !s.authRateLimit(c, "email-change:"+strconv.FormatInt(user.ID, 10)) {
		return
	}
	if s.consumeEmailChallenge(c, "email-change", email, strings.TrimSpace(input.Code)) != nil {
		writeError(c, http.StatusUnauthorized, "invalid_verification_code")
		return
	}
	now := time.Now().UTC()
	err := s.DB.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		if _, err := activeUser(tx, user); err != nil {
			return err
		}
		var taken int64
		if err := tx.Model(&User{}).Where("email = ? AND id <> ?", email, user.ID).Count(&taken).Error; err != nil {
			return err
		}
		if taken > 0 {
			return &tenant.HTTPError{Status: http.StatusConflict, Code: "email_taken"}
		}
		result := tx.Model(&User{}).Where("id = ? AND session_version = ?", user.ID, user.SessionVersion).
			Updates(map[string]any{"email": email, "email_verified_at": now})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return gorm.ErrRecordNotFound
		}
		return audit(tx, user.ID, "auth.email_changed", user.ID, gin.H{"email": email})
	})
	if err != nil {
		transactionError(c, err)
		return
	}
	if user.Email != nil {
		s.notify(c.Request.Context(), *user.Email, "Email address changed",
			"<p>Your account email was changed to <strong>"+html.EscapeString(email)+"</strong>.</p><p>If you did not make this change, contact the platform administrator immediately.</p>")
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}
