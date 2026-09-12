package platform

import (
	"crypto/subtle"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const authFlowCookie = "new_api_platform_auth"
const authFlowTTL = 5 * time.Minute

var errAuthFlow = errors.New("invalid platform authentication flow")

type AuthFlow struct {
	TokenHash   string    `gorm:"size:64;primaryKey"`
	BrowserHash string    `gorm:"size:64;not null"`
	Purpose     string    `gorm:"size:32;not null"`
	Provider    string    `gorm:"size:48;not null"`
	Intent      string    `gorm:"size:16;not null"`
	UserID      int64     `gorm:"not null"`
	SessionHash string    `gorm:"size:64;not null"`
	Payload     string    `gorm:"type:text;not null"`
	Redirect    string    `gorm:"size:1024;not null"`
	ExpiresAt   time.Time `gorm:"not null;index"`
}

func (AuthFlow) TableName() string { return "platform_auth_flows" }

func platformRedirect(value string) string {
	if len(value) > 1024 || strings.ContainsAny(value, "\\\r\n") {
		return "/platform"
	}
	u, err := url.Parse(value)
	if err != nil || u.IsAbs() || u.Host != "" || u.User != nil || u.Fragment != "" {
		return "/platform"
	}
	if strings.Contains(u.Path, "/.") || strings.Contains(u.Path, "//") || strings.ContainsAny(u.Path, "\\\r\n") {
		return "/platform"
	}
	if u.Path == "/platform" || u.Path == "/platform/security" || u.Path == "/platform/plans" || u.Path == "/platform/usage" || u.Path == "/platform/redeem" || strings.HasPrefix(u.Path, "/platform/workspaces/") || u.Path == "/platform/admin" || strings.HasPrefix(u.Path, "/platform/admin/") {
		return u.String()
	}
	return "/platform"
}

func (s *Server) createAuthFlow(c *gin.Context, flow AuthFlow, payload any) (string, error) {
	raw, err := randomSecret()
	if err != nil {
		return "", err
	}
	browser, err := randomSecret()
	if err != nil {
		return "", err
	}
	encoded, err := common.Marshal(payload)
	if err != nil {
		return "", err
	}
	flow.TokenHash, flow.BrowserHash = digest(raw), digest(browser)
	flow.Payload = string(encoded)
	flow.ExpiresAt = time.Now().UTC().Add(authFlowTTL)
	flow.Redirect = platformRedirect(flow.Redirect)
	if flow.Intent != "login" {
		user := c.MustGet("platform_user").(User)
		session := c.MustGet("platform_session").(Session)
		flow.UserID, flow.SessionHash = user.ID, session.TokenHash
	}
	db := s.DB.WithContext(c.Request.Context())
	// Opportunistic expiry uses the existing request process, with no worker.
	if err := db.Where("expires_at <= ?", time.Now().UTC()).Delete(&AuthFlow{}).Error; err != nil {
		return "", err
	}
	if err := db.Create(&flow).Error; err != nil {
		return "", err
	}
	http.SetCookie(c.Writer, &http.Cookie{Name: authFlowCookie, Value: browser, Path: "/platform/api", HttpOnly: true, Secure: s.Secure, SameSite: http.SameSiteLaxMode, MaxAge: int(authFlowTTL.Seconds())})
	return raw, nil
}

// Delete conditionally before using a challenge. A failed verification is also
// terminal; neither retries nor another replica can reuse the same flow.
func (s *Server) consumeAuthFlow(c *gin.Context, raw, purpose, provider string) (AuthFlow, error) {
	browser, err := c.Cookie(authFlowCookie)
	if err != nil || len(raw) != 64 || len(browser) != 64 {
		return AuthFlow{}, errAuthFlow
	}
	var flow AuthFlow
	err = s.DB.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		query := tx.Where("token_hash = ? AND browser_hash = ? AND purpose = ? AND provider = ? AND expires_at > ?", digest(raw), digest(browser), purpose, provider, time.Now().UTC())
		if err := query.First(&flow).Error; err != nil {
			return errAuthFlow
		}
		result := tx.Where("token_hash = ? AND browser_hash = ? AND expires_at > ?", flow.TokenHash, flow.BrowserHash, time.Now().UTC()).Delete(&AuthFlow{})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return errAuthFlow
		}
		return nil
	})
	return flow, err
}

func (s *Server) flowSessionUser(c *gin.Context, flow AuthFlow) (User, error) {
	raw, err := c.Cookie(sessionCookie)
	if err != nil || len(raw) != 64 || digest(raw) != flow.SessionHash || subtle.ConstantTimeCompare([]byte(c.GetHeader("X-CSRF-Token")), []byte(csrfToken(raw))) != 1 {
		return User{}, errAuthFlow
	}
	var session Session
	now := time.Now().UTC()
	db := s.DB.WithContext(c.Request.Context())
	if db.Where("token_hash = ? AND user_id = ? AND expires_at > ? AND last_seen > ?", flow.SessionHash, flow.UserID, now, now.Add(-30*time.Minute)).First(&session).Error != nil {
		return User{}, errAuthFlow
	}
	var user User
	if db.First(&user, flow.UserID).Error != nil || user.Status != "active" || user.MustChangePassword || user.SessionVersion != session.UserVersion || digest(user.PasswordHash) != session.CredentialVersion {
		return User{}, errAuthFlow
	}
	if (flow.Intent == "link" || flow.Intent == "register") && now.Sub(session.CreatedAt) > authFlowTTL {
		return User{}, errAuthFlow
	}
	return user, nil
}

func requireRecentSession(c *gin.Context) bool {
	session := c.MustGet("platform_session").(Session)
	if time.Since(session.CreatedAt) > authFlowTTL {
		writeError(c, http.StatusUnauthorized, "recent_login_required")
		return false
	}
	return true
}

func (s *Server) authFlowFailure(c *gin.Context, method string) {
	// Provider responses can contain codes, tokens and personal information.
	// Log only the allowlisted method and a stable failure category.
	common.SysLog(fmt.Sprintf("platform authentication failed: method=%s ip=%q route=%q reason=verification_failed", method, c.ClientIP(), c.FullPath()))
	writeError(c, http.StatusUnauthorized, "authentication_failed")
}
