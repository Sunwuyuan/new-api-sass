package platform

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/mail"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/plan"
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

func (s *Server) issueEmailChallenge(c *gin.Context, email string) error {
	code, err := sixDigitCode()
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	challenge := EmailChallenge{KeyHash: digest("email:" + email), CodeHash: digest(code), ExpiresAt: now.Add(15 * time.Minute)}
	if err := s.DB.WithContext(c.Request.Context()).Save(&challenge).Error; err != nil {
		return err
	}
	s.authMu.Lock()
	name := s.Auth.Name
	s.authMu.Unlock()
	subject := name + " email verification"
	content := fmt.Sprintf("<p>Your verification code is <strong>%s</strong>.</p><p>It expires in 15 minutes. If you did not request this, ignore the message.</p>", code)
	return s.sendPlatformMail(c.Request.Context(), email, subject, content)
}

func (s *Server) consumeEmailChallenge(c *gin.Context, email, code string) error {
	if utf8.RuneCountInString(code) != 6 {
		return errAuthFlow
	}
	now := time.Now().UTC()
	return s.DB.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		var challenge EmailChallenge
		if err := lockForUpdate(tx).First(&challenge, "key_hash = ?", digest("email:"+email)).Error; err != nil {
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
	if s.consumeEmailChallenge(c, email, strings.TrimSpace(input.Code)) != nil {
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
		if err := s.issueEmailChallenge(c, email); err != nil {
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
