package platform

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/tenant"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

var (
	lookupTXT   = net.LookupTXT
	lookupCNAME = net.LookupCNAME
)

func (s *Server) requestHost(r *http.Request) string {
	host, ok := tenant.NormalizeHost(r.Host)
	if ok {
		return host
	}
	return ""
}

func (s *Server) platformHost() string {
	parsed, err := url.Parse(s.Origin)
	if err != nil {
		return ""
	}
	host, ok := tenant.NormalizeHost(parsed.Host)
	if !ok {
		return ""
	}
	return host
}

func (s *Server) IsPlatformHost(host string) bool {
	return host != "" && host == s.platformHost()
}

func (s *Server) WorkspaceByHost(c *gin.Context) (tenant.Workspace, bool) {
	host := s.requestHost(c.Request)
	if host == "" || s.IsPlatformHost(host) {
		return tenant.Workspace{}, false
	}
	var binding tenant.Host
	if s.DB.WithContext(c.Request.Context()).Where("host = ? AND status = ?", host, tenant.HostVerified).First(&binding).Error != nil {
		return tenant.Workspace{}, false
	}
	var workspace tenant.Workspace
	if s.DB.WithContext(c.Request.Context()).First(&workspace, binding.TenantID).Error != nil {
		return tenant.Workspace{}, false
	}
	return workspace, true
}

func (s *Server) publicURL(host, path string) string {
	parsed, err := url.Parse(s.Origin)
	if err != nil {
		return ""
	}
	normalized, ok := tenant.NormalizeHost(host)
	if !ok {
		return ""
	}
	if port := parsed.Port(); port != "" {
		parsed.Host = net.JoinHostPort(normalized, port)
	} else {
		parsed.Host = normalized
	}
	parsed.Path = tenant.GatewayPath(path)
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return strings.TrimSuffix(parsed.String(), "/")
}

func (s *Server) verifyTarget() string {
	host := s.platformHost()
	if host == "" {
		return ""
	}
	return "verify." + host
}

func (s *Server) hostView(binding tenant.Host) gin.H {
	view := gin.H{
		"id": binding.ID, "tenant_id": binding.TenantID, "kind": binding.Kind,
		"host": binding.Host, "prefix": binding.Prefix, "wildcard_domain_id": binding.WildcardDomainID,
		"status": binding.Status, "verification_method": binding.VerificationMethod,
		"verified_at": binding.VerifiedAt, "created_at": binding.CreatedAt,
		"url": s.publicURL(binding.Host, "/"),
	}
	if binding.Kind == tenant.HostCustom && binding.Status != tenant.HostVerified {
		view["verification_token"] = binding.VerificationToken
		view["txt_name"] = tenant.VerifyTXTName(binding.Host)
		view["txt_value"] = binding.VerificationToken
		view["cname_host"] = binding.Host
		view["cname_target"] = binding.VerificationToken + "." + s.verifyTarget()
	}
	return view
}

func (s *Server) hostsForWorkspaces(db *gorm.DB, ids []int64) (map[int64][]tenant.Host, error) {
	hosts := make([]tenant.Host, 0)
	if len(ids) == 0 {
		return map[int64][]tenant.Host{}, nil
	}
	if err := db.Where("tenant_id IN ?", ids).Order("id").Find(&hosts).Error; err != nil {
		return nil, err
	}
	grouped := make(map[int64][]tenant.Host, len(ids))
	for _, binding := range hosts {
		grouped[binding.TenantID] = append(grouped[binding.TenantID], binding)
	}
	return grouped, nil
}

func (s *Server) primaryHost(hosts []tenant.Host) (tenant.Host, bool) {
	for _, binding := range hosts {
		if binding.Status == tenant.HostVerified && binding.Kind == tenant.HostWildcard {
			return binding, true
		}
	}
	for _, binding := range hosts {
		if binding.Status == tenant.HostVerified {
			return binding, true
		}
	}
	return tenant.Host{}, false
}

func (s *Server) workspaceURL(hosts []tenant.Host, path string) string {
	binding, ok := s.primaryHost(hosts)
	if !ok {
		return ""
	}
	return s.publicURL(binding.Host, path)
}

func (s *Server) attachWorkspaceHosts(views []gin.H, grouped map[int64][]tenant.Host) {
	for i, view := range views {
		workspace, _ := view["tenant"].(tenant.Workspace)
		hosts := grouped[workspace.ID]
		items := make([]gin.H, 0, len(hosts))
		for _, binding := range hosts {
			items = append(items, s.hostView(binding))
		}
		views[i]["hosts"] = items
		views[i]["primary_url"] = s.workspaceURL(hosts, "/")
	}
}

func (s *Server) refreshServerAddress(tx *gorm.DB, workspace tenant.Workspace) error {
	var hosts []tenant.Host
	if err := tx.Where("tenant_id = ?", workspace.ID).Order("id").Find(&hosts).Error; err != nil {
		return err
	}
	address := s.Origin
	if url := s.workspaceURL(hosts, "/"); url != "" {
		address = url
	}
	ctx := tenant.WithContext(tx.Statement.Context, tenant.Identity{ID: workspace.ID, Slug: workspace.Slug})
	return tx.WithContext(ctx).Model(&model.Option{}).Where("key = ?", "ServerAddress").Update("value", address).Error
}

func (s *Server) enabledWildcard(tx *gorm.DB, id int64) (tenant.WildcardDomain, error) {
	var domain tenant.WildcardDomain
	query := tx.Where("enabled = ?", true)
	if id > 0 {
		if err := query.First(&domain, id).Error; err != nil {
			return domain, &tenant.HTTPError{Status: http.StatusBadRequest, Code: "invalid_wildcard_domain"}
		}
		return domain, nil
	}
	if err := query.Order("id").First(&domain).Error; err != nil {
		return domain, err
	}
	return domain, nil
}

func (s *Server) reservedPrefixes(domain string) []string {
	reserved := []string{"www", "mail", "platform", "admin"}
	if prefix := tenant.PrefixForDomain(s.platformHost(), domain); prefix != "" {
		reserved = append(reserved, prefix)
	}
	return reserved
}

func (s *Server) provisionWildcardHost(tx *gorm.DB, workspace tenant.Workspace, prefix string, domainID int64) (tenant.Host, error) {
	domain, err := s.enabledWildcard(tx, domainID)
	if err != nil {
		return tenant.Host{}, err
	}
	prefix = strings.TrimSpace(strings.ToLower(prefix))
	if prefix == "" {
		prefix = workspace.Slug
	}
	if !tenant.ValidPrefix(prefix) {
		return tenant.Host{}, &tenant.HTTPError{Status: http.StatusBadRequest, Code: "invalid_workspace_host"}
	}
	for _, name := range s.reservedPrefixes(domain.Domain) {
		if prefix == name {
			return tenant.Host{}, &tenant.HTTPError{Status: http.StatusBadRequest, Code: "invalid_workspace_host"}
		}
	}
	host, ok := tenant.WildcardHost(prefix, domain.Domain)
	if !ok {
		return tenant.Host{}, &tenant.HTTPError{Status: http.StatusBadRequest, Code: "invalid_workspace_host"}
	}
	var taken int64
	if err := tx.Model(&tenant.Host{}).Where("host = ? OR (wildcard_domain_id = ? AND prefix = ?)", host, domain.ID, prefix).Count(&taken).Error; err != nil {
		return tenant.Host{}, err
	}
	if taken > 0 {
		return tenant.Host{}, &tenant.HTTPError{Status: http.StatusConflict, Code: "workspace_host_taken"}
	}
	now := time.Now().UTC()
	binding := tenant.Host{
		TenantID: workspace.ID, Kind: tenant.HostWildcard, Host: host, Prefix: prefix,
		WildcardDomainID: &domain.ID, Status: tenant.HostVerified, VerifiedAt: &now,
	}
	if err := tx.Create(&binding).Error; err != nil {
		return tenant.Host{}, err
	}
	return binding, nil
}

func newVerifyToken() (string, error) {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(raw[:]), nil
}

func (s *Server) wildcardDomains(c *gin.Context) {
	var domains []tenant.WildcardDomain
	query := s.DB.WithContext(c.Request.Context()).Where("enabled = ?", true).Order("id")
	if strings.Contains(c.FullPath(), "/admin/") {
		query = s.DB.WithContext(c.Request.Context()).Order("id")
	}
	if query.Find(&domains).Error != nil {
		writeError(c, http.StatusServiceUnavailable, "platform_unavailable")
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "wildcard_domains": domains})
}

func (s *Server) createWildcardDomain(c *gin.Context) {
	var input struct {
		Domain  string `json:"domain"`
		Enabled *bool  `json:"enabled"`
	}
	if c.ShouldBindJSON(&input) != nil {
		writeError(c, http.StatusBadRequest, "invalid_wildcard_domain")
		return
	}
	domain, ok := tenant.NormalizeHost(input.Domain)
	if !ok || !strings.Contains(domain, ".") && domain != "localhost" {
		writeError(c, http.StatusBadRequest, "invalid_wildcard_domain")
		return
	}
	enabled := true
	if input.Enabled != nil {
		enabled = *input.Enabled
	}
	err := s.adminTransaction(c, func(tx *gorm.DB) error {
		var taken int64
		if err := tx.Model(&tenant.WildcardDomain{}).Where("domain = ?", domain).Count(&taken).Error; err != nil {
			return err
		}
		if taken > 0 {
			return &tenant.HTTPError{Status: http.StatusConflict, Code: "wildcard_domain_in_use"}
		}
		row := tenant.WildcardDomain{Domain: domain, Enabled: enabled}
		if err := tx.Create(&row).Error; err != nil {
			return err
		}
		return audit(tx, c.MustGet("platform_user").(User).ID, "wildcard_domain.create", row.ID, gin.H{"domain": domain})
	})
	if err != nil {
		transactionError(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"success": true})
}

func (s *Server) updateWildcardDomain(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	var input struct {
		Domain  string `json:"domain"`
		Enabled *bool  `json:"enabled"`
	}
	if err != nil || id <= 0 || c.ShouldBindJSON(&input) != nil {
		writeError(c, http.StatusBadRequest, "invalid_wildcard_domain")
		return
	}
	err = s.adminTransaction(c, func(tx *gorm.DB) error {
		var row tenant.WildcardDomain
		if err := tx.First(&row, id).Error; err != nil {
			return err
		}
		updates := map[string]any{}
		if domain := strings.TrimSpace(input.Domain); domain != "" {
			normalized, ok := tenant.NormalizeHost(domain)
			if !ok {
				return &tenant.HTTPError{Status: http.StatusBadRequest, Code: "invalid_wildcard_domain"}
			}
			var used int64
			if err := tx.Model(&tenant.Host{}).Where("wildcard_domain_id = ?", row.ID).Count(&used).Error; err != nil {
				return err
			}
			if used > 0 && normalized != row.Domain {
				return &tenant.HTTPError{Status: http.StatusConflict, Code: "wildcard_domain_in_use"}
			}
			updates["domain"] = normalized
		}
		if input.Enabled != nil {
			updates["enabled"] = *input.Enabled
		}
		if len(updates) == 0 {
			return &tenant.HTTPError{Status: http.StatusBadRequest, Code: "invalid_wildcard_domain"}
		}
		if err := tx.Model(&row).Updates(updates).Error; err != nil {
			return err
		}
		return audit(tx, c.MustGet("platform_user").(User).ID, "wildcard_domain.update", row.ID, updates)
	})
	if err != nil {
		transactionError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (s *Server) deleteWildcardDomain(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		writeError(c, http.StatusBadRequest, "invalid_wildcard_domain")
		return
	}
	err = s.adminTransaction(c, func(tx *gorm.DB) error {
		var used int64
		if err := tx.Model(&tenant.Host{}).Where("wildcard_domain_id = ?", id).Count(&used).Error; err != nil {
			return err
		}
		if used > 0 {
			return &tenant.HTTPError{Status: http.StatusConflict, Code: "wildcard_domain_in_use"}
		}
		result := tx.Delete(&tenant.WildcardDomain{}, id)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}
		return audit(tx, c.MustGet("platform_user").(User).ID, "wildcard_domain.delete", id, gin.H{})
	})
	if err != nil {
		transactionError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (s *Server) listWorkspaceHosts(c *gin.Context) {
	workspace, ok := s.loadOwnedWorkspace(c)
	if !ok {
		return
	}
	var hosts []tenant.Host
	if s.DB.WithContext(c.Request.Context()).Where("tenant_id = ?", workspace.ID).Order("id").Find(&hosts).Error != nil {
		writeError(c, http.StatusServiceUnavailable, "platform_unavailable")
		return
	}
	items := make([]gin.H, 0, len(hosts))
	for _, binding := range hosts {
		items = append(items, s.hostView(binding))
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "hosts": items, "primary_url": s.workspaceURL(hosts, "/")})
}

func (s *Server) createWorkspaceHost(c *gin.Context) {
	workspace, ok := s.loadOwnedWorkspace(c)
	if !ok {
		return
	}
	var input struct {
		Kind             string `json:"kind"`
		Prefix           string `json:"prefix"`
		WildcardDomainID int64  `json:"wildcard_domain_id"`
		Host             string `json:"host"`
		Method           string `json:"method"`
	}
	if c.ShouldBindJSON(&input) != nil {
		writeError(c, http.StatusBadRequest, "invalid_workspace_host")
		return
	}
	actor := c.MustGet("platform_user").(User)
	var created tenant.Host
	err := s.DB.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		var count int64
		if err := tx.Model(&tenant.Host{}).Where("tenant_id = ?", workspace.ID).Count(&count).Error; err != nil {
			return err
		}
		if count >= tenant.MaxHostsPerWorkspace {
			return &tenant.HTTPError{Status: http.StatusBadRequest, Code: "workspace_host_limit_reached"}
		}
		switch strings.TrimSpace(input.Kind) {
		case tenant.HostWildcard, "":
			binding, err := s.provisionWildcardHost(tx, workspace, input.Prefix, input.WildcardDomainID)
			if err != nil {
				return err
			}
			created = binding
		case tenant.HostCustom:
			binding, err := s.provisionCustomHost(tx, workspace, input.Host, input.Method)
			if err != nil {
				return err
			}
			created = binding
		default:
			return &tenant.HTTPError{Status: http.StatusBadRequest, Code: "invalid_workspace_host"}
		}
		if err := s.refreshServerAddress(tx, workspace); err != nil {
			return err
		}
		return audit(tx, actor.ID, "workspace.host.create", workspace.ID, gin.H{"host": created.Host, "kind": created.Kind})
	})
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			writeError(c, http.StatusBadRequest, "invalid_wildcard_domain")
			return
		}
		transactionError(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"success": true, "host": s.hostView(created)})
}

func (s *Server) provisionCustomHost(tx *gorm.DB, workspace tenant.Workspace, raw, method string) (tenant.Host, error) {
	host, ok := tenant.NormalizeHost(raw)
	if !ok || host == s.platformHost() || !strings.Contains(host, ".") && host != "localhost" {
		return tenant.Host{}, &tenant.HTTPError{Status: http.StatusBadRequest, Code: "invalid_workspace_host"}
	}
	var wildcards []tenant.WildcardDomain
	if err := tx.Find(&wildcards).Error; err != nil {
		return tenant.Host{}, err
	}
	for _, domain := range wildcards {
		if host == domain.Domain || tenant.PrefixForDomain(host, domain.Domain) != "" {
			return tenant.Host{}, &tenant.HTTPError{Status: http.StatusBadRequest, Code: "invalid_workspace_host"}
		}
	}
	method = strings.TrimSpace(strings.ToLower(method))
	if method == "" {
		method = tenant.VerifyTXT
	}
	if method != tenant.VerifyTXT && method != tenant.VerifyCNAME {
		return tenant.Host{}, &tenant.HTTPError{Status: http.StatusBadRequest, Code: "invalid_workspace_host"}
	}
	var taken int64
	if err := tx.Model(&tenant.Host{}).Where("host = ?", host).Count(&taken).Error; err != nil {
		return tenant.Host{}, err
	}
	if taken > 0 {
		return tenant.Host{}, &tenant.HTTPError{Status: http.StatusConflict, Code: "workspace_host_taken"}
	}
	token, err := newVerifyToken()
	if err != nil {
		return tenant.Host{}, err
	}
	binding := tenant.Host{
		TenantID: workspace.ID, Kind: tenant.HostCustom, Host: host,
		Status: tenant.HostPending, VerificationMethod: method, VerificationToken: token,
	}
	if err := tx.Create(&binding).Error; err != nil {
		return tenant.Host{}, err
	}
	return binding, nil
}

func (s *Server) verifyWorkspaceHost(c *gin.Context) {
	workspace, ok := s.loadOwnedWorkspace(c)
	if !ok {
		return
	}
	id, err := strconv.ParseInt(c.Param("hid"), 10, 64)
	if err != nil || id <= 0 {
		writeError(c, http.StatusBadRequest, "invalid_workspace_host")
		return
	}
	var binding tenant.Host
	if s.DB.WithContext(c.Request.Context()).Where("id = ? AND tenant_id = ?", id, workspace.ID).First(&binding).Error != nil {
		writeError(c, http.StatusNotFound, "platform_record_not_found")
		return
	}
	if binding.Kind != tenant.HostCustom {
		writeError(c, http.StatusBadRequest, "invalid_workspace_host")
		return
	}
	if binding.Status == tenant.HostVerified {
		c.JSON(http.StatusOK, gin.H{"success": true, "host": s.hostView(binding)})
		return
	}
	if !s.domainVerified(binding) {
		writeError(c, http.StatusBadRequest, "workspace_host_unverified")
		return
	}
	now := time.Now().UTC()
	actor := c.MustGet("platform_user").(User)
	err = s.DB.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&binding).Updates(map[string]any{"status": tenant.HostVerified, "verified_at": now}).Error; err != nil {
			return err
		}
		binding.Status = tenant.HostVerified
		binding.VerifiedAt = &now
		if err := s.refreshServerAddress(tx, workspace); err != nil {
			return err
		}
		return audit(tx, actor.ID, "workspace.host.verify", workspace.ID, gin.H{"host": binding.Host})
	})
	if err != nil {
		transactionError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "host": s.hostView(binding)})
}

func (s *Server) domainVerified(binding tenant.Host) bool {
	token := binding.VerificationToken
	if token == "" {
		return false
	}
	if binding.VerificationMethod != tenant.VerifyCNAME {
		records, err := lookupTXT(tenant.VerifyTXTName(binding.Host))
		if err == nil {
			for _, record := range records {
				value := strings.TrimSpace(strings.Trim(record, `"`))
				value, _ = strings.CutPrefix(value, "newapi-site-verification=")
				if value == token {
					return true
				}
			}
		}
		if binding.VerificationMethod == tenant.VerifyTXT {
			return false
		}
	}
	target := strings.TrimSuffix(strings.ToLower(token+"."+s.verifyTarget()), ".")
	cname, err := lookupCNAME(binding.Host)
	if err != nil {
		return false
	}
	cname = strings.TrimSuffix(strings.ToLower(cname), ".")
	if cname == target {
		return true
	}
	var aliases []tenant.Host
	if s.DB.Where("tenant_id = ? AND status = ? AND id <> ?", binding.TenantID, tenant.HostVerified, binding.ID).Find(&aliases).Error != nil {
		return false
	}
	for _, alias := range aliases {
		if cname == alias.Host {
			return true
		}
	}
	return false
}

func (s *Server) deleteWorkspaceHost(c *gin.Context) {
	workspace, ok := s.loadOwnedWorkspace(c)
	if !ok {
		return
	}
	id, err := strconv.ParseInt(c.Param("hid"), 10, 64)
	if err != nil || id <= 0 {
		writeError(c, http.StatusBadRequest, "invalid_workspace_host")
		return
	}
	actor := c.MustGet("platform_user").(User)
	err = s.DB.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		var binding tenant.Host
		if err := tx.Where("id = ? AND tenant_id = ?", id, workspace.ID).First(&binding).Error; err != nil {
			return err
		}
		if err := tx.Delete(&binding).Error; err != nil {
			return err
		}
		if err := s.refreshServerAddress(tx, workspace); err != nil {
			return err
		}
		return audit(tx, actor.ID, "workspace.host.delete", workspace.ID, gin.H{"host": binding.Host})
	})
	if err != nil {
		transactionError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (s *Server) loadOwnedWorkspace(c *gin.Context) (tenant.Workspace, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		writeError(c, http.StatusNotFound, "platform_record_not_found")
		return tenant.Workspace{}, false
	}
	var workspace tenant.Workspace
	if err := s.ownedWorkspaces(c).First(&workspace, id).Error; err != nil {
		transactionError(c, err)
		return tenant.Workspace{}, false
	}
	return workspace, true
}
