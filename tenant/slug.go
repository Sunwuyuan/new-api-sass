package tenant

import (
	"net/http"
	"net/url"
	"regexp"
	"strings"
)

var slugPattern = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]{0,46}[a-z0-9])?$`)

func ValidSlug(slug string) bool {
	return slug != "" && slugPattern.MatchString(slug)
}

func SlugFromPath(pathname string) string {
	path, _, _ := strings.Cut(pathname, "?")
	rest, found := strings.CutPrefix(path, "/t/")
	if !found {
		return ""
	}
	slug, _, _ := strings.Cut(rest, "/")
	if !ValidSlug(slug) {
		return ""
	}
	return slug
}

func SlugFromHeader(value string) string {
	value = strings.TrimSpace(value)
	if !ValidSlug(value) {
		return ""
	}
	return value
}

func GatewayPath(path string) string {
	if path == "" {
		return "/"
	}
	if strings.HasPrefix(path, "/") {
		return path
	}
	return "/" + path
}

func IsBrowserDocument(r *http.Request) bool {
	dest := r.Header.Get("Sec-Fetch-Dest")
	if dest == "document" || dest == "iframe" || dest == "frame" {
		return true
	}
	if dest != "" {
		return false
	}
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		return false
	}
	return strings.Contains(r.Header.Get("Accept"), "text/html")
}

func AllowReferer(r *http.Request, ref *url.URL, origin string) bool {
	if ref.Host == "" {
		return true
	}
	if hostEqual(ref.Host, r.Host) {
		return true
	}
	if origin != "" {
		if parsed, err := url.Parse(origin); err == nil && hostEqual(ref.Host, parsed.Host) {
			return true
		}
	}
	if raw := r.Header.Get("Origin"); raw != "" {
		if parsed, err := url.Parse(raw); err == nil && hostEqual(ref.Host, parsed.Host) {
			return true
		}
	}
	if forwarded := forwardedHost(r.Header.Get("X-Forwarded-Host")); forwarded != "" && hostEqual(ref.Host, forwarded) {
		return true
	}
	return false
}

func forwardedHost(value string) string {
	host, _, _ := strings.Cut(value, ",")
	return strings.TrimSpace(host)
}

func hostEqual(a, b string) bool {
	return strings.EqualFold(strings.TrimSuffix(a, "."), strings.TrimSuffix(b, "."))
}
