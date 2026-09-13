package tenant

import (
	"net"
	"strings"
	"time"

	"golang.org/x/net/idna"
)

const MaxHostsPerWorkspace = 20

type WildcardDomain struct {
	ID        int64     `json:"id" gorm:"primaryKey"`
	Domain    string    `json:"domain" gorm:"size:253;not null;uniqueIndex"`
	Enabled   bool      `json:"enabled"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (WildcardDomain) TableName() string { return "platform_wildcard_domains" }

type Host struct {
	ID                 int64      `json:"id" gorm:"primaryKey"`
	TenantID           int64      `json:"tenant_id" gorm:"not null;index"`
	Kind               string     `json:"kind" gorm:"size:16;not null"`
	Host               string     `json:"host" gorm:"size:253;not null;uniqueIndex"`
	Prefix             string     `json:"prefix" gorm:"size:48"`
	WildcardDomainID   *int64     `json:"wildcard_domain_id" gorm:"index"`
	Status             string     `json:"status" gorm:"size:16;not null"`
	VerificationMethod string     `json:"verification_method" gorm:"size:16"`
	VerificationToken  string     `json:"verification_token" gorm:"size:64"`
	VerifiedAt         *time.Time `json:"verified_at"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
}

func (Host) TableName() string { return "tenant_hosts" }

const (
	HostWildcard = "wildcard"
	HostCustom   = "custom"
	HostPending  = "pending"
	HostVerified = "verified"
	VerifyTXT    = "txt"
	VerifyCNAME  = "cname"
)

func NormalizeHost(value string) (string, bool) {
	value = strings.TrimSpace(strings.ToLower(value))
	if value == "" {
		return "", false
	}
	if strings.Contains(value, "://") {
		return "", false
	}
	if host, port, err := net.SplitHostPort(value); err == nil && port != "" {
		value = host
	}
	if strings.ContainsAny(value, "/:@ ") {
		return "", false
	}
	value = strings.TrimSuffix(value, ".")
	ascii, err := idna.Lookup.ToASCII(value)
	if err != nil || ascii == "" || len(ascii) > 253 || net.ParseIP(ascii) != nil {
		return "", false
	}
	for label := range strings.SplitSeq(ascii, ".") {
		if label == "" || label[0] == '-' || label[len(label)-1] == '-' {
			return "", false
		}
		for i := range len(label) {
			c := label[i]
			if c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '-' {
				continue
			}
			return "", false
		}
	}
	return ascii, true
}

func ValidPrefix(prefix string) bool { return ValidSlug(prefix) }

func WildcardHost(prefix, domain string) (string, bool) {
	if !ValidPrefix(prefix) {
		return "", false
	}
	host, ok := NormalizeHost(prefix + "." + domain)
	if !ok || !strings.HasSuffix(host, "."+domain) {
		return "", false
	}
	return host, true
}

func PrefixForDomain(host, domain string) string {
	suffix := "." + domain
	if !strings.HasSuffix(host, suffix) {
		return ""
	}
	prefix := strings.TrimSuffix(host, suffix)
	if prefix == "" || strings.Contains(prefix, ".") || !ValidPrefix(prefix) {
		return ""
	}
	return prefix
}

func VerifyTXTName(host string) string { return "_newapi-verify." + host }
