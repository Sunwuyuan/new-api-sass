package platform

import (
	"errors"
	"maps"
	"net/http"
	"os"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/tenant"
	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Setting struct {
	Key   string `gorm:"primaryKey;size:64"`
	Value string `gorm:"type:text;not null"`
}

func (Setting) TableName() string { return "platform_settings" }

type storedProvider struct {
	Slug         string   `json:"slug"`
	Name         string   `json:"name"`
	Enabled      bool     `json:"enabled"`
	ClientID     string   `json:"client_id"`
	ClientSecret string   `json:"client_secret"`
	Issuer       string   `json:"issuer"`
	AuthMethod   string   `json:"token_auth_method"`
	Scopes       []string `json:"scopes"`
}

func (s *Server) loadStoredSettings() error {
	var rows []Setting
	if err := s.DB.Find(&rows).Error; err != nil {
		return err
	}
	s.authMu.Lock()
	defer s.authMu.Unlock()
	for _, row := range rows {
		switch row.Key {
		case "mail":
			mailCfg := s.Mail
			if err := common.UnmarshalJsonStr(row.Value, &mailCfg); err != nil {
				return err
			}
			if mailCfg.BaseURL == "" {
				mailCfg.BaseURL = defaultAmailURL
			}
			if mailCfg.ProviderID == "" {
				mailCfg.ProviderID = "auto"
			}
			s.Mail = mailCfg
		case "auth":
			var stored []storedProvider
			if err := common.UnmarshalJsonStr(row.Value, &stored); err != nil {
				return err
			}
			if err := applyStoredProviders(&s.Auth, stored); err != nil {
				return err
			}
		}
	}
	return nil
}

func applyStoredProviders(cfg *AuthConfig, stored []storedProvider) error {
	for _, item := range stored {
		if !providerSlug.MatchString(item.Slug) {
			return errors.New("invalid stored OAuth provider")
		}
		if !item.Enabled {
			delete(cfg.Providers, item.Slug)
			continue
		}
		current, exists := cfg.Providers[item.Slug]
		if !exists {
			current = builtinProvider(item.Slug)
			current.Slug = item.Slug
		}
		if item.Name != "" {
			current.Name = item.Name
		}
		if item.ClientID != "" {
			current.ClientID = item.ClientID
		}
		if item.ClientSecret != "" {
			current.ClientSecret = item.ClientSecret
		}
		if item.Issuer != "" {
			current.Issuer = strings.TrimRight(item.Issuer, "/")
		}
		if item.AuthMethod != "" {
			current.AuthMethod = item.AuthMethod
		}
		if len(item.Scopes) > 0 {
			current.Scopes = item.Scopes
		}
		if err := current.validate(); err != nil {
			return err
		}
		cfg.Providers[item.Slug] = current
	}
	if !cfg.PasswordLogin && len(cfg.Providers) == 0 && !cfg.WeChat.Enabled && !cfg.Passkey {
		return errors.New("enable at least one platform login method")
	}
	return nil
}

func builtinProvider(slug string) OAuthProvider {
	switch slug {
	case "github":
		return OAuthProvider{Slug: "github", Name: "GitHub", Authorization: "https://github.com/login/oauth/authorize", Token: "https://github.com/login/oauth/access_token", UserInfo: "https://api.github.com/user", Scopes: []string{"read:user"}, SubjectPath: "id", NamePath: "login", AuthMethod: "client_secret_post"}
	case "discord":
		return OAuthProvider{Slug: "discord", Name: "Discord", Authorization: "https://discord.com/oauth2/authorize", Token: "https://discord.com/api/v10/oauth2/token", UserInfo: "https://discord.com/api/v10/users/@me", Scopes: []string{"identify"}, SubjectPath: "id", NamePath: "username", AuthMethod: "client_secret_post"}
	case "linuxdo":
		return OAuthProvider{Slug: "linuxdo", Name: "LinuxDO", Authorization: "https://connect.linux.do/oauth2/authorize", Token: "https://connect.linux.do/oauth2/token", UserInfo: "https://connect.linux.do/api/user", SubjectPath: "id", NamePath: "username", AuthMethod: "client_secret_basic"}
	case "oidc":
		return OAuthProvider{Slug: "oidc", Name: "OIDC", Scopes: []string{"openid", "profile"}}
	case "logto":
		return OAuthProvider{Slug: "logto", Name: "厚浪云", Scopes: []string{"openid", "profile", "email"}}
	case "telegram":
		return OAuthProvider{Slug: "telegram", Name: "Telegram", Issuer: "https://oauth.telegram.org", Scopes: []string{"openid", "profile"}, AuthMethod: "client_secret_basic"}
	default:
		return OAuthProvider{Slug: slug, Name: slug}
	}
}

func (s *Server) snapshotAuth() AuthConfig {
	s.authMu.Lock()
	defer s.authMu.Unlock()
	cfg := s.Auth
	cfg.Providers = maps.Clone(s.Auth.Providers)
	return cfg
}

func (s *Server) adminSettings(c *gin.Context) {
	auth := s.snapshotAuth()
	s.authMu.Lock()
	mailCfg := s.Mail
	s.authMu.Unlock()
	providers := make([]gin.H, 0)
	seen := map[string]bool{}
	for _, slug := range []string{"github", "logto", "discord", "linuxdo", "oidc", "telegram"} {
		p, enabled := auth.Providers[slug]
		if !enabled {
			p = builtinProvider(slug)
		}
		providers = append(providers, gin.H{
			"slug": p.Slug, "name": p.Name, "enabled": enabled,
			"client_id": p.ClientID, "issuer": p.Issuer, "configured": p.ClientSecret != "",
		})
		seen[slug] = true
	}
	for slug, p := range auth.Providers {
		if seen[slug] {
			continue
		}
		providers = append(providers, gin.H{
			"slug": p.Slug, "name": p.Name, "enabled": true,
			"client_id": p.ClientID, "issuer": p.Issuer, "configured": p.ClientSecret != "",
		})
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"auth": gin.H{
			"registration":       auth.Registration,
			"password_login":     auth.PasswordLogin,
			"oauth_registration": auth.OAuthRegistration,
			"passkey":            auth.Passkey,
			"name":               auth.Name,
			"logo":               auth.Logo,
			"agreement_url":      auth.AgreementURL,
			"privacy_url":        auth.PrivacyURL,
			"providers":          providers,
		},
		"mail": gin.H{
			"enabled": mailCfg.Enabled, "base_url": mailCfg.BaseURL, "provider_id": mailCfg.ProviderID,
			"from": mailCfg.From, "from_name": mailCfg.FromName,
			"email_verification": mailCfg.EmailVerification, "notifications": mailCfg.Notifications,
			"configured": mailCfg.APIKey != "",
		},
	})
}

func (s *Server) updateAdminSettings(c *gin.Context) {
	var input struct {
		Auth *struct {
			Registration      *bool            `json:"registration"`
			PasswordLogin     *bool            `json:"password_login"`
			OAuthRegistration *bool            `json:"oauth_registration"`
			Passkey           *bool            `json:"passkey"`
			Name              *string          `json:"name"`
			Logo              *string          `json:"logo"`
			AgreementURL      *string          `json:"agreement_url"`
			PrivacyURL        *string          `json:"privacy_url"`
			Providers         []storedProvider `json:"providers"`
		} `json:"auth"`
		Mail *MailSettings `json:"mail"`
	}
	if c.ShouldBindJSON(&input) != nil || input.Auth == nil && input.Mail == nil {
		writeError(c, http.StatusBadRequest, "invalid_platform_settings")
		return
	}
	err := s.adminTransaction(c, func(tx *gorm.DB) error {
		if input.Auth != nil {
			current := s.snapshotAuth()
			if input.Auth.Registration != nil {
				current.Registration = *input.Auth.Registration
			}
			if input.Auth.PasswordLogin != nil {
				current.PasswordLogin = *input.Auth.PasswordLogin
			}
			if input.Auth.OAuthRegistration != nil {
				current.OAuthRegistration = *input.Auth.OAuthRegistration
			}
			if input.Auth.Passkey != nil {
				current.Passkey = *input.Auth.Passkey
			}
			if input.Auth.Name != nil {
				current.Name = strings.TrimSpace(*input.Auth.Name)
			}
			if input.Auth.Logo != nil {
				current.Logo = strings.TrimSpace(*input.Auth.Logo)
			}
			if input.Auth.AgreementURL != nil {
				current.AgreementURL = strings.TrimSpace(*input.Auth.AgreementURL)
			}
			if input.Auth.PrivacyURL != nil {
				current.PrivacyURL = strings.TrimSpace(*input.Auth.PrivacyURL)
			}
			for _, value := range []string{current.Logo, current.AgreementURL, current.PrivacyURL} {
				if value != "" && !safePublicURL(value) {
					return &tenant.HTTPError{Status: http.StatusBadRequest, Code: "invalid_platform_settings"}
				}
			}
			if err := applyStoredProviders(&current, input.Auth.Providers); err != nil {
				return &tenant.HTTPError{Status: http.StatusBadRequest, Code: "invalid_platform_settings"}
			}
			encoded, err := common.Marshal(input.Auth.Providers)
			if err != nil {
				return err
			}
			if err := upsertSetting(tx, "auth", string(encoded)); err != nil {
				return err
			}
			s.authMu.Lock()
			s.Auth = current
			s.providers = make(map[string]*oidc.Provider)
			s.authMu.Unlock()
		}
		if input.Mail != nil {
			mailCfg := input.Mail
			if mailCfg.BaseURL == "" {
				mailCfg.BaseURL = defaultAmailURL
			}
			if mailCfg.ProviderID == "" {
				mailCfg.ProviderID = "auto"
			}
			if mailCfg.APIKey == "" {
				s.authMu.Lock()
				mailCfg.APIKey = s.Mail.APIKey
				s.authMu.Unlock()
			}
			if mailCfg.Enabled && (mailCfg.APIKey == "" || mailCfg.From == "" || !safeEndpoint(mailCfg.BaseURL)) {
				return &tenant.HTTPError{Status: http.StatusBadRequest, Code: "invalid_platform_settings"}
			}
			stored := *mailCfg
			stored.APIKey = mailCfg.APIKey
			encoded, err := common.Marshal(stored)
			if err != nil {
				return err
			}
			if err := upsertSetting(tx, "mail", string(encoded)); err != nil {
				return err
			}
			s.authMu.Lock()
			s.Mail = *mailCfg
			s.authMu.Unlock()
		}
		return audit(tx, c.MustGet("platform_user").(User).ID, "settings.update", 0, gin.H{
			"auth": input.Auth != nil, "mail": input.Mail != nil,
		})
	})
	if err != nil {
		transactionError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func upsertSetting(tx *gorm.DB, key, value string) error {
	return tx.Clauses(clause.OnConflict{UpdateAll: true}).Create(&Setting{Key: key, Value: value}).Error
}

func bootstrapMailFromEnv(cfg MailSettings) MailSettings {
	if url := strings.TrimSpace(os.Getenv("PLATFORM_MAIL_BASE_URL")); url != "" {
		cfg.BaseURL = strings.TrimRight(url, "/")
	}
	if key := strings.TrimSpace(os.Getenv("PLATFORM_MAIL_API_KEY")); key != "" {
		cfg.APIKey = key
		cfg.Enabled = true
	}
	if from := strings.TrimSpace(os.Getenv("PLATFORM_MAIL_FROM")); from != "" {
		cfg.From = from
	}
	if name := strings.TrimSpace(os.Getenv("PLATFORM_MAIL_FROM_NAME")); name != "" {
		cfg.FromName = name
	}
	if provider := strings.TrimSpace(os.Getenv("PLATFORM_MAIL_PROVIDER_ID")); provider != "" {
		cfg.ProviderID = provider
	}
	return cfg
}
