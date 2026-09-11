// Package platform owns SaaS accounts. These credentials and sessions are
// separate from New API users inside each workspace.
package platform

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/mail"
	"net/url"
	"os"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type User struct {
	ID           int64     `json:"id" gorm:"primaryKey"`
	Email        string    `json:"email" gorm:"size:254;not null;uniqueIndex"`
	PasswordHash string    `json:"-" gorm:"type:text;not null"`
	Role         string    `json:"role" gorm:"size:16;not null"`
	TenantCount  int       `json:"tenant_count" gorm:"not null"`
	CreatedAt    time.Time `json:"created_at"`
}

func (User) TableName() string { return "platform_users" }

type Session struct {
	TokenHash         string    `gorm:"size:64;primaryKey"`
	UserID            int64     `gorm:"not null;index"`
	CreatedAt         time.Time `gorm:"not null"`
	LastSeen          time.Time `gorm:"not null"`
	ExpiresAt         time.Time `gorm:"not null;index"`
	CredentialVersion string    `gorm:"size:64;not null;default:''"`
}

func (Session) TableName() string { return "platform_sessions" }

type AuthAttempt struct {
	Key      string `gorm:"size:64;primaryKey"`
	Window   int64  `gorm:"primaryKey;autoIncrement:false"`
	Attempts int    `gorm:"not null"`
}

func (AuthAttempt) TableName() string { return "platform_auth_attempts" }

const sessionCookie = "new_api_platform_session"

func randomSecret() (string, error) {
	var value [32]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(value[:]), nil
}

func digest(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func csrfToken(cookie string) string { return digest("new-api/platform/csrf/v1:" + cookie) }

func writeError(c *gin.Context, status int, code string) {
	c.AbortWithStatusJSON(status, gin.H{"success": false, "code": code, "message": code})
}

func (s *Server) authRateLimit(c *gin.Context, email string) bool {
	window := time.Now().Unix() / 900
	for _, key := range []string{"ip:" + c.ClientIP(), "account:" + email} {
		entry := AuthAttempt{Key: digest(key), Window: window}
		query := s.DB.WithContext(c.Request.Context())
		if err := query.Clauses(clause.OnConflict{DoNothing: true}).Create(&entry).Error; err != nil {
			writeError(c, http.StatusServiceUnavailable, "platform_unavailable")
			return false
		}
		result := query.Model(&AuthAttempt{}).Where(&AuthAttempt{Key: entry.Key, Window: window}).Where("attempts < ?", 20).
			UpdateColumn("attempts", gorm.Expr("attempts + 1"))
		if result.Error != nil || result.RowsAffected != 1 {
			c.Header("Retry-After", "900")
			writeError(c, http.StatusTooManyRequests, "authentication_rate_limited")
			return false
		}
	}
	return true
}

// Every browser mutation requires a non-simple header and an exact origin
// check. Authenticated mutations additionally require a session-bound CSRF token.
func (s *Server) BrowserSecurity(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	c.Header("X-Content-Type-Options", "nosniff")
	if c.Request.Method == http.MethodGet || c.Request.Method == http.MethodHead {
		c.Next()
		return
	}
	if c.GetHeader("X-Requested-With") != "NewAPIPlatform" {
		writeError(c, http.StatusForbidden, "csrf_invalid")
		return
	}
	if origins := c.Request.Header.Values("Origin"); len(origins) > 1 || len(origins) == 1 && origins[0] != s.Origin {
		writeError(c, http.StatusForbidden, "csrf_invalid")
		return
	}
	if c.GetHeader("Sec-Fetch-Site") == "cross-site" {
		writeError(c, http.StatusForbidden, "csrf_invalid")
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 16384)
	c.Next()
}

func (s *Server) authenticate(c *gin.Context) {
	raw, err := c.Cookie(sessionCookie)
	if err != nil || len(raw) != 64 {
		writeError(c, http.StatusUnauthorized, "platform_login_required")
		return
	}
	var session Session
	now := time.Now().UTC()
	query := s.DB.WithContext(c.Request.Context())
	err = query.Where("token_hash = ? AND expires_at > ? AND last_seen > ?", digest(raw), now, now.Add(-30*time.Minute)).First(&session).Error
	if err != nil {
		writeError(c, http.StatusUnauthorized, "platform_login_required")
		return
	}
	var user User
	if query.First(&user, session.UserID).Error != nil {
		writeError(c, http.StatusUnauthorized, "platform_login_required")
		return
	}
	if subtle.ConstantTimeCompare([]byte(session.CredentialVersion), []byte(digest(user.PasswordHash))) != 1 {
		writeError(c, http.StatusUnauthorized, "platform_login_required")
		return
	}
	if c.Request.Method != http.MethodGet && c.Request.Method != http.MethodHead {
		if subtle.ConstantTimeCompare([]byte(c.GetHeader("X-CSRF-Token")), []byte(csrfToken(raw))) != 1 {
			writeError(c, http.StatusForbidden, "csrf_invalid")
			return
		}
	}
	if err := query.Model(&Session{}).Where("token_hash = ?", session.TokenHash).Update("last_seen", now).Error; err != nil {
		writeError(c, http.StatusServiceUnavailable, "platform_unavailable")
		return
	}
	c.Set("platform_user", user)
	c.Set("platform_session", session)
	c.Set("platform_csrf", csrfToken(raw))
	c.Next()
}

func requireAdmin(c *gin.Context) {
	user := c.MustGet("platform_user").(User)
	if user.Role != "admin" {
		writeError(c, http.StatusForbidden, "platform_admin_required")
		return
	}
	if c.Request.Method != http.MethodGet {
		session := c.MustGet("platform_session").(Session)
		if time.Since(session.CreatedAt) > 5*time.Minute {
			writeError(c, http.StatusUnauthorized, "recent_login_required")
			return
		}
	}
	c.Next()
}

type credentials struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func validateCredentials(input *credentials, newPassword bool) error {
	input.Email = strings.ToLower(strings.TrimSpace(input.Email))
	address, err := mail.ParseAddress(input.Email)
	if err != nil || address.Address != input.Email || len(input.Email) > 254 {
		return errors.New("invalid email")
	}
	if len(input.Password) > 512 || !utf8.ValidString(input.Password) {
		return errors.New("invalid password")
	}
	if newPassword && (utf8.RuneCountInString(input.Password) < 15 || utf8.RuneCountInString(input.Password) > 128) {
		return errors.New("password must contain 15 to 128 characters")
	}
	if newPassword {
		return common.ValidateNewAccountPassword(input.Password)
	}
	return nil
}

func (s *Server) register(c *gin.Context) {
	var input credentials
	if c.ShouldBindJSON(&input) != nil || validateCredentials(&input, true) != nil {
		writeError(c, http.StatusBadRequest, "invalid_email_or_password_length")
		return
	}
	if !s.authRateLimit(c, input.Email) {
		return
	}
	hash, err := common.HashAccountPassword(input.Password)
	if err != nil {
		writeError(c, http.StatusServiceUnavailable, "platform_unavailable")
		return
	}
	user := User{Email: input.Email, PasswordHash: hash, Role: "user"}
	if err := s.DB.WithContext(c.Request.Context()).Clauses(clause.OnConflict{DoNothing: true}).Create(&user).Error; err != nil {
		writeError(c, http.StatusServiceUnavailable, "platform_unavailable")
		return
	}
	// The same result for existing addresses avoids a registration oracle.
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (s *Server) login(c *gin.Context) {
	var input credentials
	if c.ShouldBindJSON(&input) != nil || validateCredentials(&input, false) != nil {
		writeError(c, http.StatusUnauthorized, "invalid_credentials")
		return
	}
	if !s.authRateLimit(c, input.Email) {
		return
	}
	var user User
	err := s.DB.WithContext(c.Request.Context()).Where("email = ?", input.Email).First(&user).Error
	hash := s.DummyHash
	if err == nil {
		hash = user.PasswordHash
	}
	valid := common.ValidatePasswordAndHash(input.Password, hash)
	if err != nil || !valid {
		common.SysLog("platform authentication failed: account_ref=" + digest(input.Email))
		writeError(c, http.StatusUnauthorized, "invalid_credentials")
		return
	}
	raw, err := randomSecret()
	if err != nil {
		writeError(c, http.StatusServiceUnavailable, "platform_unavailable")
		return
	}
	now := time.Now().UTC()
	session := Session{TokenHash: digest(raw), UserID: user.ID, CreatedAt: now, LastSeen: now, ExpiresAt: now.Add(8 * time.Hour), CredentialVersion: digest(user.PasswordHash)}
	err = s.DB.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		if previous, _ := c.Cookie(sessionCookie); len(previous) == 64 {
			if err := tx.Where("token_hash = ?", digest(previous)).Delete(&Session{}).Error; err != nil {
				return err
			}
		}
		return tx.Create(&session).Error
	})
	if err != nil {
		writeError(c, http.StatusServiceUnavailable, "platform_unavailable")
		return
	}
	http.SetCookie(c.Writer, &http.Cookie{Name: sessionCookie, Value: raw, Path: "/platform/api", HttpOnly: true, Secure: s.Secure, SameSite: http.SameSiteStrictMode, MaxAge: 8 * 3600, Expires: session.ExpiresAt})
	c.JSON(http.StatusOK, gin.H{"success": true, "user": user, "csrf_token": csrfToken(raw)})
}

func (s *Server) logout(c *gin.Context) {
	session := c.MustGet("platform_session").(Session)
	if err := s.DB.WithContext(c.Request.Context()).Where("token_hash = ?", session.TokenHash).Delete(&Session{}).Error; err != nil {
		writeError(c, http.StatusServiceUnavailable, "platform_unavailable")
		return
	}
	http.SetCookie(c.Writer, &http.Cookie{Name: sessionCookie, Path: "/platform/api", HttpOnly: true, Secure: s.Secure, SameSite: http.SameSiteStrictMode, MaxAge: -1})
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (s *Server) changePassword(c *gin.Context) {
	var input struct {
		CurrentPassword string `json:"current_password"`
		NewPassword     string `json:"new_password"`
	}
	user := c.MustGet("platform_user").(User)
	if c.ShouldBindJSON(&input) != nil || validateCredentials(&credentials{Email: user.Email, Password: input.NewPassword}, true) != nil {
		writeError(c, http.StatusBadRequest, "invalid_new_password")
		return
	}
	if !s.authRateLimit(c, user.Email) {
		return
	}
	if !common.ValidatePasswordAndHash(input.CurrentPassword, user.PasswordHash) {
		common.SysLog(fmt.Sprintf("platform password change rejected: user_id=%d", user.ID))
		writeError(c, http.StatusUnauthorized, "invalid_credentials")
		return
	}
	if input.NewPassword == input.CurrentPassword {
		writeError(c, http.StatusBadRequest, "password_unchanged")
		return
	}
	hash, err := common.HashAccountPassword(input.NewPassword)
	if err != nil {
		writeError(c, http.StatusServiceUnavailable, "platform_unavailable")
		return
	}
	err = s.DB.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&User{}).Where("id = ? AND password_hash = ?", user.ID, user.PasswordHash).Update("password_hash", hash)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return gorm.ErrRecordNotFound
		}
		return tx.Where("user_id = ?", user.ID).Delete(&Session{}).Error
	})
	if err != nil {
		writeError(c, http.StatusConflict, "platform_unavailable")
		return
	}
	common.SysLog(fmt.Sprintf("platform password changed; sessions revoked: user_id=%d", user.ID))
	http.SetCookie(c.Writer, &http.Cookie{Name: sessionCookie, Path: "/platform/api", HttpOnly: true, Secure: s.Secure, SameSite: http.SameSiteStrictMode, MaxAge: -1})
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (s *Server) bootstrapAdmin() error {
	email := os.Getenv("PLATFORM_ADMIN_EMAIL")
	password := os.Getenv("PLATFORM_ADMIN_PASSWORD")
	if email == "" && password == "" {
		var count int64
		if err := s.DB.Model(&User{}).Where("role = ?", "admin").Count(&count).Error; err != nil {
			return err
		}
		if count == 0 {
			return errors.New("set PLATFORM_ADMIN_EMAIL and PLATFORM_ADMIN_PASSWORD to create the first platform administrator")
		}
		return nil
	}
	input := credentials{Email: email, Password: password}
	if err := validateCredentials(&input, true); err != nil {
		return err
	}
	var existing User
	err := s.DB.Where("email = ?", input.Email).First(&existing).Error
	if err == nil {
		if existing.Role != "admin" {
			return errors.New("bootstrap email already belongs to a non-administrator")
		}
		return nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	hash, err := common.HashAccountPassword(input.Password)
	if err != nil {
		return err
	}
	return s.DB.Create(&User{Email: input.Email, PasswordHash: hash, Role: "admin"}).Error
}

func configuredOrigin() (string, bool, error) {
	origin := strings.TrimRight(os.Getenv("PLATFORM_ORIGIN"), "/")
	if origin == "" {
		origin = "http://localhost:3000"
	}
	u, err := url.Parse(origin)
	if err != nil || u.Host == "" || u.Path != "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return "", false, errors.New("PLATFORM_ORIGIN must be an absolute origin")
	}
	if u.Scheme != "https" && (u.Scheme != "http" || u.Hostname() != "localhost" && u.Hostname() != "127.0.0.1" && u.Hostname() != "::1") {
		return "", false, errors.New("PLATFORM_ORIGIN requires HTTPS except for local development")
	}
	normalized, err := common.NormalizeOrigin(origin)
	return normalized, u.Scheme == "https", err
}
