package platform

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
)

// AuthConfig is a process-wide snapshot. It never reads workspace options or
// exposes secrets through the public status response.
type AuthConfig struct {
	Name              string
	Logo              string
	Registration      bool
	PasswordLogin     bool
	OAuthRegistration bool
	Passkey           bool
	AgreementURL      string
	PrivacyURL        string
	Providers         map[string]OAuthProvider
	WeChat            WeChatConfig
}

type OAuthProvider struct {
	Slug          string   `json:"slug"`
	Name          string   `json:"name"`
	ClientID      string   `json:"client_id"`
	ClientSecret  string   `json:"client_secret"`
	Issuer        string   `json:"issuer"`
	Authorization string   `json:"authorization_endpoint"`
	Token         string   `json:"token_endpoint"`
	UserInfo      string   `json:"userinfo_endpoint"`
	Scopes        []string `json:"scopes"`
	SubjectPath   string   `json:"subject_path"`
	NamePath      string   `json:"name_path"`
	AuthMethod    string   `json:"token_auth_method"`
}

type WeChatConfig struct {
	Enabled bool
	Server  string
	Token   string
	QRCode  string
}

var providerSlug = regexp.MustCompile(`^[a-z][a-z0-9-]{0,47}$`)

func loadAuthConfig() (AuthConfig, error) {
	cfg := AuthConfig{
		Name: "New API SaaS", Logo: "/logo.png",
		Registration: true, PasswordLogin: true, OAuthRegistration: true,
		Providers: make(map[string]OAuthProvider),
	}
	for key, target := range map[string]*bool{
		"PLATFORM_REGISTER_ENABLED":       &cfg.Registration,
		"PLATFORM_PASSWORD_LOGIN_ENABLED": &cfg.PasswordLogin,
		"PLATFORM_OAUTH_REGISTER_ENABLED": &cfg.OAuthRegistration,
		"PLATFORM_PASSKEY_ENABLED":        &cfg.Passkey,
		"PLATFORM_WECHAT_ENABLED":         &cfg.WeChat.Enabled,
	} {
		if value, exists := os.LookupEnv(key); exists {
			parsed, err := strconv.ParseBool(value)
			if err != nil {
				return cfg, fmt.Errorf("%s must be a boolean", key)
			}
			*target = parsed
		}
	}
	if name := strings.TrimSpace(os.Getenv("PLATFORM_NAME")); name != "" {
		cfg.Name = name
	}
	if logo := strings.TrimSpace(os.Getenv("PLATFORM_LOGO")); logo != "" {
		cfg.Logo = logo
	}
	cfg.AgreementURL = strings.TrimSpace(os.Getenv("PLATFORM_AGREEMENT_URL"))
	cfg.PrivacyURL = strings.TrimSpace(os.Getenv("PLATFORM_PRIVACY_URL"))
	for _, value := range []string{cfg.Logo, cfg.AgreementURL, cfg.PrivacyURL} {
		if value != "" && !safePublicURL(value) {
			return cfg, errors.New("platform branding and legal URLs must be HTTPS or local absolute paths")
		}
	}
	providers := []OAuthProvider{
		{Slug: "github", Name: "GitHub", Authorization: "https://github.com/login/oauth/authorize", Token: "https://github.com/login/oauth/access_token", UserInfo: "https://api.github.com/user", Scopes: []string{"read:user"}, SubjectPath: "id", NamePath: "login", AuthMethod: "client_secret_post"},
		{Slug: "discord", Name: "Discord", Authorization: "https://discord.com/oauth2/authorize", Token: "https://discord.com/api/v10/oauth2/token", UserInfo: "https://discord.com/api/v10/users/@me", Scopes: []string{"identify"}, SubjectPath: "id", NamePath: "username", AuthMethod: "client_secret_post"},
		{Slug: "linuxdo", Name: "LinuxDO", Authorization: "https://connect.linux.do/oauth2/authorize", Token: "https://connect.linux.do/oauth2/token", UserInfo: "https://connect.linux.do/api/user", SubjectPath: "id", NamePath: "username", AuthMethod: "client_secret_basic"},
		{Slug: "oidc", Name: "OIDC", Issuer: strings.TrimRight(os.Getenv("PLATFORM_OIDC_ISSUER"), "/"), Scopes: []string{"openid", "profile"}},
		{Slug: "logto", Name: "厚浪云", Issuer: strings.TrimRight(os.Getenv("PLATFORM_LOGTO_ISSUER"), "/"), Scopes: []string{"openid", "profile", "email"}},
		{Slug: "telegram", Name: "Telegram", Issuer: "https://oauth.telegram.org", Scopes: []string{"openid", "profile"}, AuthMethod: "client_secret_basic"},
	}
	if name := strings.TrimSpace(os.Getenv("PLATFORM_OIDC_NAME")); name != "" {
		providers[3].Name = name
	}
	if name := strings.TrimSpace(os.Getenv("PLATFORM_LOGTO_NAME")); name != "" {
		providers[4].Name = name
	}
	for _, provider := range providers {
		prefix := "PLATFORM_" + strings.ToUpper(provider.Slug)
		provider.ClientID = strings.TrimSpace(os.Getenv(prefix + "_CLIENT_ID"))
		provider.ClientSecret = strings.TrimSpace(os.Getenv(prefix + "_CLIENT_SECRET"))
		enabled := provider.ClientID != "" || provider.ClientSecret != ""
		if value, exists := os.LookupEnv(prefix + "_ENABLED"); exists {
			var err error
			enabled, err = strconv.ParseBool(value)
			if err != nil {
				return cfg, fmt.Errorf("%s_ENABLED must be a boolean", prefix)
			}
		}
		if !enabled {
			continue
		}
		if err := provider.validate(); err != nil {
			return cfg, fmt.Errorf("%s configuration is incomplete or invalid", prefix)
		}
		cfg.Providers[provider.Slug] = provider
	}
	if raw := os.Getenv("PLATFORM_CUSTOM_OAUTH_PROVIDERS"); raw != "" {
		var custom []OAuthProvider
		if common.UnmarshalJsonStr(raw, &custom) != nil || len(custom) > 20 {
			return cfg, errors.New("invalid PLATFORM_CUSTOM_OAUTH_PROVIDERS")
		}
		for _, provider := range custom {
			if slices.Contains([]string{"github", "discord", "oidc", "logto", "linuxdo", "telegram", "wechat", "passkey"}, provider.Slug) {
				return cfg, errors.New("custom platform OAuth provider uses a reserved slug")
			}
			if _, exists := cfg.Providers[provider.Slug]; exists {
				return cfg, errors.New("duplicate platform OAuth provider slug")
			}
			if err := provider.validate(); err != nil {
				return cfg, errors.New("invalid custom platform OAuth provider configuration")
			}
			cfg.Providers[provider.Slug] = provider
		}
	}
	cfg.WeChat.Server = strings.TrimRight(os.Getenv("PLATFORM_WECHAT_SERVER"), "/")
	cfg.WeChat.Token = os.Getenv("PLATFORM_WECHAT_TOKEN")
	cfg.WeChat.QRCode = os.Getenv("PLATFORM_WECHAT_QRCODE")
	if cfg.WeChat.Enabled && (!safeEndpoint(cfg.WeChat.Server) || cfg.WeChat.Token == "" || !safePublicURL(cfg.WeChat.QRCode)) {
		return cfg, errors.New("platform WeChat requires a server, token and QR code URL")
	}
	if !cfg.PasswordLogin && len(cfg.Providers) == 0 && !cfg.WeChat.Enabled && !cfg.Passkey {
		return cfg, errors.New("enable at least one platform login method")
	}
	return cfg, nil
}

func (p OAuthProvider) validate() error {
	if !providerSlug.MatchString(p.Slug) || strings.TrimSpace(p.Name) == "" || len(p.Name) > 128 || p.ClientID == "" || p.ClientSecret == "" {
		return errors.New("invalid OAuth provider")
	}
	if p.AuthMethod != "" && p.AuthMethod != "client_secret_basic" && p.AuthMethod != "client_secret_post" {
		return errors.New("invalid token authentication method")
	}
	if p.Issuer != "" {
		if !safeEndpoint(p.Issuer) || !slices.Contains(p.Scopes, "openid") {
			return errors.New("invalid OIDC issuer or scopes")
		}
		return nil
	}
	if !safeEndpoint(p.Authorization) || !safeEndpoint(p.Token) || !safeEndpoint(p.UserInfo) || p.SubjectPath == "" {
		return errors.New("invalid OAuth endpoints")
	}
	return nil
}

func safeEndpoint(value string) bool {
	u, err := url.Parse(value)
	return err == nil && u.Host != "" && u.User == nil && u.RawQuery == "" && u.Fragment == "" &&
		(u.Scheme == "https" || u.Scheme == "http" && (u.Hostname() == "localhost" || u.Hostname() == "127.0.0.1" || u.Hostname() == "::1"))
}

func safePublicURL(value string) bool {
	if strings.ContainsAny(value, "\\\r\n") {
		return false
	}
	u, err := url.Parse(value)
	return err == nil && u.User == nil && (u.Scheme == "https" && u.Host != "" || u.Scheme == "" && u.Host == "" && strings.HasPrefix(value, "/") && !strings.HasPrefix(value, "//"))
}

func (p OAuthProvider) scope() string {
	// Client and issuer changes must not relink a different identity namespace.
	return digest(p.Slug + "\x00" + p.ClientID + "\x00" + p.Issuer + "\x00" + p.UserInfo)
}

func (s *Server) status(c *gin.Context) {
	cfg := s.snapshotAuth()
	custom := make([]gin.H, 0)
	slugs := make([]string, 0, len(cfg.Providers))
	for slug := range cfg.Providers {
		slugs = append(slugs, slug)
	}
	slices.Sort(slugs)
	verificationProviders := make([]string, 0)
	for _, slug := range slugs {
		p := cfg.Providers[slug]
		if p.Issuer != "" {
			verificationProviders = append(verificationProviders, slug)
		}
		if slices.Contains([]string{"github", "discord", "oidc", "logto", "linuxdo", "telegram"}, slug) {
			continue
		}
		custom = append(custom, gin.H{"slug": slug, "name": p.Name})
	}
	_, github := cfg.Providers["github"]
	_, discord := cfg.Providers["discord"]
	_, linuxdo := cfg.Providers["linuxdo"]
	_, oidc := cfg.Providers["oidc"]
	_, logto := cfg.Providers["logto"]
	_, telegram := cfg.Providers["telegram"]
	mail := s.mailPublicStatus()
	c.JSON(http.StatusOK, gin.H{
		"success": true, "system_name": cfg.Name, "logo": cfg.Logo,
		"register_enabled": cfg.Registration && (cfg.PasswordLogin || cfg.OAuthRegistration && (len(cfg.Providers) > 0 || cfg.WeChat.Enabled)), "password_login_enabled": cfg.PasswordLogin,
		"password_register_enabled": cfg.Registration && cfg.PasswordLogin, "oauth_register_enabled": cfg.Registration && cfg.OAuthRegistration,
		"github_oauth": github, "discord_oauth": discord, "linuxdo_oauth": linuxdo,
		"oidc_enabled": oidc, "oidc_display_name": cfg.Providers["oidc"].Name,
		"logto_oauth": logto, "logto_display_name": cfg.Providers["logto"].Name, "telegram_oauth": telegram,
		"custom_oauth_providers": custom, "passkey_login": cfg.Passkey,
		"reauthentication_providers": verificationProviders,
		"wechat_login":               cfg.WeChat.Enabled, "wechat_qrcode": cfg.WeChat.QRCode,
		"user_agreement_enabled": cfg.AgreementURL != "", "privacy_policy_enabled": cfg.PrivacyURL != "",
		"user_agreement_url": cfg.AgreementURL, "privacy_policy_url": cfg.PrivacyURL,
		"email_verification": mail["email_verification"], "mail": mail,
	})
}
