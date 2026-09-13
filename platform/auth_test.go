package platform

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/plan"
	"github.com/QuantumNous/new-api/tenant"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/go-jose/go-jose/v4"
	"github.com/go-webauthn/webauthn/protocol/webauthncbor"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// PLATFORM_TEST_DSN must name a disposable, empty database. The fixture resets
// only SaaS tables, and runs the same HTTP/protocol contracts on all dialects.
type authFixture struct {
	server  *Server
	handler http.Handler
	nextIP  int
}

type authBrowser struct {
	fixture *authFixture
	cookies map[string]*http.Cookie
	csrf    string
	ip      string
}

func newAuthFixture(t *testing.T, seeds ...func(*gorm.DB)) *authFixture {
	t.Helper()
	gin.SetMode(gin.TestMode)
	for _, entry := range os.Environ() {
		key, _, _ := strings.Cut(entry, "=")
		if strings.HasPrefix(key, "PLATFORM_") && key != "PLATFORM_TEST_DSN" {
			t.Setenv(key, "")
			require.NoError(t, os.Unsetenv(key))
		}
	}
	t.Setenv("PLATFORM_ORIGIN", "https://platform.example.test")
	t.Setenv("PLATFORM_ADMIN_EMAIL", "admin@example.test")
	t.Setenv("PLATFORM_ADMIN_PASSWORD", "Only a synthetic bootstrap passphrase 2026!")
	t.Setenv("PLATFORM_PASSKEY_ENABLED", "true")
	previousType := common.MainDatabaseType()
	dsn := os.Getenv("PLATFORM_TEST_DSN")
	var dialect gorm.Dialector
	switch {
	case strings.HasPrefix(dsn, "postgres://"):
		dialect = postgres.Open(dsn)
		common.SetMainDatabaseType(common.DatabaseTypePostgreSQL)
	case dsn != "":
		dialect = mysql.Open(dsn)
		common.SetMainDatabaseType(common.DatabaseTypeMySQL)
	default:
		dialect = sqlite.Open(filepath.Join(t.TempDir(), "platform.sqlite") + "?_pragma=busy_timeout(30000)&_txlock=immediate")
		common.SetMainDatabaseType(common.DatabaseTypeSQLite)
	}
	db, err := gorm.Open(dialect, &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	previousDB, previousLogDB := model.DB, model.LOG_DB
	t.Cleanup(func() {
		model.DB, model.LOG_DB = previousDB, previousLogDB
		common.SetMainDatabaseType(previousType)
		require.NoError(t, sqlDB.Close())
	})
	require.NoError(t, db.Migrator().DropTable(&AuthFlow{}, &OAuthIdentity{}, &PasskeyCredential{}, &Session{}, &AuthAttempt{}, &RedemptionUse{}, &Redemption{}, &RootActivation{}, &Audit{}, &AdminGuard{}, &User{}, &plan.Assignment{}, &plan.Usage{}, &tenant.Workspace{}, &plan.Plan{}))
	for _, seed := range seeds {
		seed(db)
	}
	s, err := New(db)
	require.NoError(t, err)
	model.DB, model.LOG_DB = db, db
	require.NoError(t, model.EnforceTenantScopes())
	router := gin.New()
	s.Routes(router)
	return &authFixture{server: s, handler: router}
}

func (f *authFixture) browser() *authBrowser {
	f.nextIP++
	return &authBrowser{fixture: f, cookies: make(map[string]*http.Cookie), ip: fmt.Sprintf("192.0.2.%d:10000", f.nextIP)}
}

func (b *authBrowser) request(t *testing.T, method, path string, body, output any) *httptest.ResponseRecorder {
	t.Helper()
	encoded, err := common.Marshal(body)
	require.NoError(t, err)
	request := httptest.NewRequest(method, "/platform/api"+path, bytes.NewReader(encoded))
	request.RemoteAddr = b.ip
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Origin", b.fixture.server.Origin)
	request.Header.Set("X-Requested-With", "NewAPIPlatform")
	request.Header.Set("X-CSRF-Token", b.csrf)
	for _, cookie := range b.cookies {
		request.AddCookie(cookie)
	}
	response := httptest.NewRecorder()
	b.fixture.handler.ServeHTTP(response, request)
	for _, cookie := range response.Result().Cookies() {
		if cookie.MaxAge < 0 {
			delete(b.cookies, cookie.Name)
		} else {
			b.cookies[cookie.Name] = cookie
		}
	}
	var session struct {
		CSRF string `json:"csrf_token"`
	}
	require.NoError(t, common.Unmarshal(response.Body.Bytes(), &session))
	if session.CSRF != "" {
		b.csrf = session.CSRF
	}
	if output != nil {
		require.NoError(t, common.Unmarshal(response.Body.Bytes(), output))
	}
	return response
}

func (f *authFixture) account(t *testing.T, name string) (*authBrowser, User) {
	t.Helper()
	b := f.browser()
	password := "Synthetic platform test phrase for " + name
	email := name + "@example.test"
	require.Equal(t, http.StatusOK, b.request(t, http.MethodPost, "/register", credentials{email, password}, nil).Code)
	var session struct{ User User }
	require.Equal(t, http.StatusOK, b.request(t, http.MethodPost, "/login", credentials{email, password}, &session).Code)
	require.NotNil(t, b.cookies[sessionCookie])
	return b, session.User
}

type oauthGrant struct {
	challenge      string
	nonce          string
	subject        string
	claims         map[string]any
	wrongSignature bool
}

type oauthProviderFixture struct {
	server         *httptest.Server
	mu             sync.Mutex
	grants         map[string]oauthGrant
	access         map[string]string
	key            *ecdsa.PrivateKey
	platformOrigin string
	clientID       string
	tokenCalls     int
}

func newOAuthProvider(t *testing.T, f *authFixture, slug string, oidc bool) *oauthProviderFixture {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	p := &oauthProviderFixture{grants: make(map[string]oauthGrant), access: make(map[string]string), key: key, platformOrigin: f.server.Origin, clientID: "synthetic-client"}
	p.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p.mu.Lock()
		defer p.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		var response any
		switch r.URL.Path {
		case "/.well-known/openid-configuration":
			response = gin.H{"issuer": p.server.URL, "authorization_endpoint": p.server.URL + "/authorize", "token_endpoint": p.server.URL + "/token", "jwks_uri": p.server.URL + "/keys", "id_token_signing_alg_values_supported": []string{"ES256"}, "response_types_supported": []string{"code"}, "subject_types_supported": []string{"public"}}
		case "/keys":
			response = jose.JSONWebKeySet{Keys: []jose.JSONWebKey{{Key: &p.key.PublicKey, KeyID: "test-key", Algorithm: "ES256", Use: "sig"}}}
		case "/token":
			p.tokenCalls++
			if r.ParseForm() != nil {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			clientID, secret, ok := r.BasicAuth()
			assert.True(t, ok)
			assert.Equal(t, p.clientID, clientID)
			assert.Equal(t, "synthetic-client-secret", secret)
			grant, found := p.grants[r.Form.Get("code")]
			delete(p.grants, r.Form.Get("code"))
			challenge := sha256.Sum256([]byte(r.Form.Get("code_verifier")))
			if !found || base64.RawURLEncoding.EncodeToString(challenge[:]) != grant.challenge || r.Form.Get("redirect_uri") != p.platformOrigin+"/platform/oauth/"+slug {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			access := "synthetic-access-" + r.Form.Get("code")
			p.access[access] = grant.subject
			result := gin.H{"access_token": access, "token_type": "Bearer", "expires_in": 300}
			if oidc {
				signingKey := p.key
				if grant.wrongSignature {
					signingKey, err = ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
					if !assert.NoError(t, err) {
						return
					}
				}
				signer, signingErr := jose.NewSigner(jose.SigningKey{Algorithm: jose.ES256, Key: signingKey}, (&jose.SignerOptions{}).WithHeader("kid", "test-key"))
				if !assert.NoError(t, signingErr) {
					return
				}
				encoded, marshalErr := common.Marshal(grant.claims)
				if !assert.NoError(t, marshalErr) {
					return
				}
				signed, signingErr := signer.Sign(encoded)
				if !assert.NoError(t, signingErr) {
					return
				}
				result["id_token"], signingErr = signed.CompactSerialize()
				if !assert.NoError(t, signingErr) {
					return
				}
			}
			response = result
		case "/userinfo":
			subject, found := p.access[strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")]
			if !found {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			response = gin.H{"id": subject, "name": "External member", "email": "owner@example.test"}
		default:
			w.WriteHeader(http.StatusNotFound)
			return
		}
		encoded, marshalErr := common.Marshal(response)
		if assert.NoError(t, marshalErr) {
			_, _ = w.Write(encoded)
		}
	}))
	t.Cleanup(p.server.Close)
	config := OAuthProvider{Slug: slug, Name: "Contract provider", ClientID: p.clientID, ClientSecret: "synthetic-client-secret", Authorization: p.server.URL + "/authorize", Token: p.server.URL + "/token", UserInfo: p.server.URL + "/userinfo", SubjectPath: "id", NamePath: "name"}
	if oidc {
		config.Issuer = p.server.URL
		config.Scopes = []string{"openid", "profile"}
	}
	f.server.Auth.Providers[slug] = config
	return p
}

func (p *oauthProviderFixture) begin(t *testing.T, b *authBrowser, slug, intent, subject string) (string, string) {
	t.Helper()
	var result struct {
		AuthorizationURL string `json:"authorization_url"`
	}
	response := b.request(t, http.MethodPost, "/oauth/"+slug+"/"+intent, gin.H{"redirect": "/platform/usage"}, &result)
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	authorization, err := url.Parse(result.AuthorizationURL)
	require.NoError(t, err)
	values := authorization.Query()
	require.Equal(t, "S256", values.Get("code_challenge_method"))
	require.Equal(t, "code", values.Get("response_type"))
	require.Equal(t, p.platformOrigin+"/platform/oauth/"+slug, values.Get("redirect_uri"))
	state := values.Get("state")
	require.Len(t, state, 64)
	require.True(t, b.cookies[authFlowCookie].HttpOnly)
	require.True(t, b.cookies[authFlowCookie].Secure)
	if intent == "verify" {
		assert.Equal(t, "0", values.Get("max_age"))
		assert.Equal(t, "login", values.Get("prompt"))
	}
	code := "synthetic-code-" + state[:12]
	p.mu.Lock()
	p.grants[code] = oauthGrant{challenge: values.Get("code_challenge"), nonce: values.Get("nonce"), subject: subject, claims: map[string]any{"iss": p.server.URL, "aud": p.clientID, "sub": subject, "nonce": values.Get("nonce"), "iat": time.Now().Unix(), "exp": time.Now().Add(time.Minute).Unix(), "auth_time": time.Now().Unix(), "name": "External member"}}
	p.mu.Unlock()
	return state, code
}

func TestPlatformOAuthContracts(t *testing.T) {
	f := newAuthFixture(t)
	provider := newOAuthProvider(t, f, "contract", false)
	owner, original := f.account(t, "owner")
	b := f.browser()
	state, code := provider.begin(t, b, "contract", "start", "external-member")
	input := gin.H{"state": state, "code": code}
	assert.Equal(t, http.StatusUnauthorized, f.browser().request(t, http.MethodPost, "/oauth/contract/finish", input, nil).Code)
	var result struct {
		User     User
		Redirect string
	}
	require.Equal(t, http.StatusOK, b.request(t, http.MethodPost, "/oauth/contract/finish", input, &result).Code)
	assert.NotEqual(t, original.ID, result.User.ID, "an external email claim cannot link a platform account")
	assert.Nil(t, result.User.Email)
	assert.Equal(t, "External member", result.User.DisplayName)
	assert.False(t, result.User.HasPassword)
	assert.Equal(t, "/platform/usage", result.Redirect)
	externalID := result.User.ID
	assert.True(t, b.cookies[sessionCookie].HttpOnly)
	assert.True(t, b.cookies[sessionCookie].Secure)
	assert.Equal(t, "/platform/api", b.cookies[sessionCookie].Path)
	assert.Equal(t, http.StatusUnauthorized, b.request(t, http.MethodPost, "/oauth/contract/finish", input, nil).Code)
	assert.Equal(t, 1, provider.tokenCalls)
	t.Run("provider and client namespaces cannot spoof the same subject", func(t *testing.T) {
		other := newOAuthProvider(t, f, "other-idp", false)
		browser := f.browser()
		state, code := other.begin(t, browser, "other-idp", "start", "external-member")
		require.Equal(t, 200, browser.request(t, http.MethodPost, "/oauth/other-idp/finish", gin.H{"state": state, "code": code}, &result).Code)
		otherID := result.User.ID
		assert.NotEqual(t, externalID, otherID)
		config := f.server.Auth.Providers["other-idp"]
		config.ClientID = "different-synthetic-client"
		f.server.Auth.Providers["other-idp"] = config
		other.mu.Lock()
		other.clientID = config.ClientID
		other.mu.Unlock()
		browser = f.browser()
		state, code = other.begin(t, browser, "other-idp", "start", "external-member")
		require.Equal(t, 200, browser.request(t, http.MethodPost, "/oauth/other-idp/finish", gin.H{"state": state, "code": code}, &result).Code)
		assert.NotEqual(t, otherID, result.User.ID)
		assert.NotEqual(t, externalID, result.User.ID)
	})

	t.Run("identity reuse and disabled registration", func(t *testing.T) {
		f.server.Auth.OAuthRegistration = false
		defer func() { f.server.Auth.OAuthRegistration = true }()
		for _, subject := range []string{"external-member", "new-member"} {
			browser := f.browser()
			state, code := provider.begin(t, browser, "contract", "start", subject)
			response := browser.request(t, http.MethodPost, "/oauth/contract/finish", gin.H{"state": state, "code": code}, &result)
			if subject == "external-member" {
				require.Equal(t, 200, response.Code)
				assert.Equal(t, externalID, result.User.ID)
			} else {
				assert.Equal(t, 401, response.Code)
				assert.Nil(t, browser.cookies[sessionCookie])
			}
		}
	})
	t.Run("expiry provider mismatch configuration change and csrf", func(t *testing.T) {
		for _, kind := range []string{"expiry", "provider", "config", "csrf"} {
			t.Run(kind, func(t *testing.T) {
				browser := f.browser()
				state, code := provider.begin(t, browser, "contract", "start", "boundary")
				path := "/oauth/contract/finish"
				if kind == "expiry" {
					require.NoError(t, f.server.DB.Model(&AuthFlow{}).Where("token_hash = ?", digest(state)).Update("expires_at", time.Now().UTC().Add(-time.Second)).Error)
				}
				if kind == "provider" {
					other := f.server.Auth.Providers["contract"]
					other.Slug = "other"
					f.server.Auth.Providers["other"] = other
					path = "/oauth/other/finish"
				}
				if kind == "config" {
					previous := f.server.Auth.Providers["contract"]
					next := previous
					next.ClientSecret = "rotated-test-secret"
					f.server.Auth.Providers["contract"] = next
					defer func() { f.server.Auth.Providers["contract"] = previous }()
				}
				if kind == "csrf" {
					request := httptest.NewRequest(http.MethodPost, "/platform/api"+path, strings.NewReader(`{}`))
					request.Header.Set("Origin", "https://attacker.example.test")
					request.Header.Set("X-Requested-With", "NewAPIPlatform")
					response := httptest.NewRecorder()
					f.handler.ServeHTTP(response, request)
					assert.Equal(t, http.StatusForbidden, response.Code)
					return
				}
				assert.Equal(t, 401, browser.request(t, http.MethodPost, path, gin.H{"state": state, "code": code}, nil).Code)
				assert.Nil(t, browser.cookies[sessionCookie])
			})
		}
	})
	t.Run("link needs session csrf fresh proof and an unclaimed identity", func(t *testing.T) {
		state, code := provider.begin(t, owner, "contract", "link", "external-member")
		assert.Equal(t, 401, owner.request(t, http.MethodPost, "/oauth/contract/finish", gin.H{"state": state, "code": code}, nil).Code)
		state, code = provider.begin(t, owner, "contract", "link", "owner-external")
		csrf := owner.csrf
		owner.csrf = "wrong-csrf"
		assert.Equal(t, 401, owner.request(t, http.MethodPost, "/oauth/contract/finish", gin.H{"state": state, "code": code}, nil).Code)
		owner.csrf = csrf
		state, code = provider.begin(t, owner, "contract", "link", "owner-external")
		oldCookie := owner.cookies[sessionCookie].Value
		require.Equal(t, 200, owner.request(t, http.MethodPost, "/oauth/contract/finish", gin.H{"state": state, "code": code}, &result).Code)
		assert.Equal(t, original.ID, result.User.ID)
		assert.NotEqual(t, oldCookie, owner.cookies[sessionCookie].Value)
		assert.Equal(t, 400, owner.request(t, http.MethodPost, "/oauth/contract/verify", gin.H{}, nil).Code)
		require.NoError(t, f.server.DB.Model(&Session{}).Where("token_hash = ?", digest(owner.cookies[sessionCookie].Value)).Update("created_at", time.Now().Add(-6*time.Minute)).Error)
		assert.Equal(t, 401, owner.request(t, http.MethodPost, "/oauth/contract/link", gin.H{}, nil).Code)
	})
	t.Run("disabled accounts and forced password change cannot bypass controls", func(t *testing.T) {
		require.NoError(t, f.server.DB.Model(&User{}).Where("id = ?", externalID).Update("status", "disabled").Error)
		browser := f.browser()
		state, code := provider.begin(t, browser, "contract", "start", "external-member")
		assert.Equal(t, 401, browser.request(t, http.MethodPost, "/oauth/contract/finish", gin.H{"state": state, "code": code}, nil).Code)
		assert.Nil(t, browser.cookies[sessionCookie])
		require.NoError(t, f.server.DB.Model(&User{}).Where("id = ?", original.ID).Update("must_change_password", true).Error)
		browser = f.browser()
		state, code = provider.begin(t, browser, "contract", "start", "owner-external")
		require.Equal(t, 200, browser.request(t, http.MethodPost, "/oauth/contract/finish", gin.H{"state": state, "code": code}, &result).Code)
		assert.True(t, result.User.MustChangePassword)
		assert.Equal(t, 403, browser.request(t, http.MethodGet, "/tenants", nil, nil).Code)
		assert.Equal(t, 403, browser.request(t, http.MethodPost, "/passkey/register/begin", gin.H{}, nil).Code)
	})
	var audits []Audit
	require.NoError(t, f.server.DB.Find(&audits).Error)
	encoded, err := common.Marshal(audits)
	require.NoError(t, err)
	for _, secret := range []string{code, state, "synthetic-client-secret", "synthetic-access-", b.cookies[sessionCookie].Value, owner.csrf} {
		assert.NotContains(t, string(encoded), secret)
	}
}

func TestPlatformOIDCVerification(t *testing.T) {
	f := newAuthFixture(t)
	p := newOAuthProvider(t, f, "oidc", true)
	b := f.browser()
	state, code := p.begin(t, b, "oidc", "start", "oidc-member")
	var result struct{ User User }
	require.Equal(t, 200, b.request(t, http.MethodPost, "/oauth/oidc/finish", gin.H{"state": state, "code": code}, &result).Code)
	identityID := result.User.ID
	for _, failure := range []string{"issuer", "audience", "nonce", "signature", "expiry", "azp", "missing_issued_at", "future_issued_at", "missing_auth_time", "old_auth_time", "future_auth_time", "other_identity", "valid"} {
		t.Run(failure, func(t *testing.T) {
			require.NoError(t, f.server.DB.Where("1 = 1").Delete(&AuthAttempt{}).Error)
			state, code := p.begin(t, b, "oidc", "verify", "oidc-member")
			p.mu.Lock()
			grant := p.grants[code]
			switch failure {
			case "issuer":
				grant.claims["iss"] = "https://wrong.example.test"
			case "audience":
				grant.claims["aud"] = "another-client"
			case "nonce":
				grant.claims["nonce"] = "wrong-nonce"
			case "signature":
				grant.wrongSignature = true
			case "expiry":
				grant.claims["exp"] = time.Now().Add(-time.Minute).Unix()
			case "azp":
				grant.claims["aud"] = []string{p.clientID, "other"}
				grant.claims["azp"] = "other"
			case "missing_auth_time":
				delete(grant.claims, "auth_time")
			case "missing_issued_at":
				delete(grant.claims, "iat")
			case "future_issued_at":
				grant.claims["iat"] = time.Now().Add(time.Hour).Unix()
			case "old_auth_time":
				grant.claims["auth_time"] = time.Now().Add(-time.Hour).Unix()
			case "future_auth_time":
				grant.claims["auth_time"] = time.Now().Add(time.Hour).Unix()
			case "other_identity":
				grant.claims["sub"] = "another-member"
			}
			p.grants[code] = grant
			p.mu.Unlock()
			previous := b.cookies[sessionCookie].Value
			response := b.request(t, http.MethodPost, "/oauth/oidc/finish", gin.H{"state": state, "code": code}, &result)
			if failure == "valid" {
				require.Equal(t, 200, response.Code, response.Body.String())
				assert.Equal(t, identityID, result.User.ID)
				assert.NotEqual(t, previous, b.cookies[sessionCookie].Value)
				old := f.browser()
				old.cookies[sessionCookie] = &http.Cookie{Name: sessionCookie, Value: previous}
				assert.Equal(t, 401, old.request(t, http.MethodGet, "/session", nil, nil).Code)
			} else {
				assert.Equal(t, 401, response.Code)
				assert.Equal(t, previous, b.cookies[sessionCookie].Value)
			}
			assert.Equal(t, 401, b.request(t, http.MethodPost, "/oauth/oidc/finish", gin.H{"state": state, "code": code}, nil).Code)
		})
	}
}

// The test authenticator uses an actual P-256 key, CBOR attestation, and signed
// authenticator data. Verification failures exercise the WebAuthn protocol.
type platformAuthenticator struct {
	key    *ecdsa.PrivateKey
	id     []byte
	handle string
	origin string
	rpID   string
}

func newPlatformAuthenticator(t *testing.T, userID int64) *platformAuthenticator {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	id := make([]byte, 32)
	_, err = rand.Read(id)
	require.NoError(t, err)
	return &platformAuthenticator{key: key, id: id, handle: fmt.Sprintf("new-api:platform:%d", userID), origin: "https://platform.example.test", rpID: "platform.example.test"}
}

func (a *platformAuthenticator) response(t *testing.T, challenge string, registration bool, counter uint32, failure string) map[string]any {
	t.Helper()
	kind := "webauthn.get"
	if registration {
		kind = "webauthn.create"
	}
	origin, handle, rpID := a.origin, a.handle, a.rpID
	if failure == "challenge" {
		challenge = "different-challenge"
	}
	if failure == "origin" {
		origin = "https://attacker.example.test"
	}
	if failure == "handle" {
		handle = "1"
	}
	if failure == "rp" {
		rpID = "tenant.example.test"
	}
	clientData, err := common.Marshal(gin.H{"type": kind, "challenge": challenge, "origin": origin, "crossOrigin": failure == "cross_origin"})
	require.NoError(t, err)
	rpHash := sha256.Sum256([]byte(rpID))
	authData := append([]byte(nil), rpHash[:]...)
	flags := byte(0x05)
	if failure == "uv" {
		flags = 0x01
	}
	if registration {
		flags |= 0x40
	}
	authData = append(authData, flags)
	authData = binary.BigEndian.AppendUint32(authData, counter)
	encode := base64.RawURLEncoding.EncodeToString
	response := map[string]any{"clientDataJSON": encode(clientData)}
	if registration {
		publicKey, marshalErr := webauthncbor.Marshal(map[int]any{1: 2, 3: -7, -1: 1, -2: a.key.X.FillBytes(make([]byte, 32)), -3: a.key.Y.FillBytes(make([]byte, 32))})
		require.NoError(t, marshalErr)
		authData = append(authData, make([]byte, 16)...)
		authData = binary.BigEndian.AppendUint16(authData, uint16(len(a.id)))
		authData = append(authData, a.id...)
		authData = append(authData, publicKey...)
		attestation, marshalErr := webauthncbor.Marshal(map[string]any{"fmt": "none", "attStmt": map[string]any{}, "authData": authData})
		require.NoError(t, marshalErr)
		response["attestationObject"] = encode(attestation)
		response["transports"] = []string{"internal"}
	} else {
		clientHash := sha256.Sum256(clientData)
		signed := append(append([]byte(nil), authData...), clientHash[:]...)
		hash := sha256.Sum256(signed)
		signature, signErr := ecdsa.SignASN1(rand.Reader, a.key, hash[:])
		require.NoError(t, signErr)
		if failure == "signature" {
			signature[len(signature)-1] ^= 1
		}
		response["signature"] = encode(signature)
		response["authenticatorData"] = encode(authData)
		response["userHandle"] = encode([]byte(handle))
	}
	return map[string]any{"id": encode(a.id), "rawId": encode(a.id), "type": "public-key", "authenticatorAttachment": "platform", "response": response, "clientExtensionResults": map[string]any{}}
}

func passkeyChallenge(t *testing.T, b *authBrowser, intent string) (string, string) {
	t.Helper()
	response := b.request(t, http.MethodPost, "/passkey/"+intent+"/begin", gin.H{}, nil)
	require.Equal(t, 200, response.Code, response.Body.String())
	flow := gjson.GetBytes(response.Body.Bytes(), "flow_token").String()
	challenge := gjson.GetBytes(response.Body.Bytes(), "options.publicKey.challenge").String()
	require.NotEmpty(t, flow)
	require.NotEmpty(t, challenge)
	if intent == "register" {
		assert.Equal(t, "required", gjson.GetBytes(response.Body.Bytes(), "options.publicKey.authenticatorSelection.userVerification").String())
		assert.Equal(t, "required", gjson.GetBytes(response.Body.Bytes(), "options.publicKey.authenticatorSelection.residentKey").String())
	} else {
		assert.Equal(t, "required", gjson.GetBytes(response.Body.Bytes(), "options.publicKey.userVerification").String())
	}
	return flow, challenge
}

func TestPlatformPasskeyContracts(t *testing.T) {
	f := newAuthFixture(t)
	owner, user := f.account(t, "passkey-owner")
	other, _ := f.account(t, "passkey-other")
	a := newPlatformAuthenticator(t, user.ID)
	t.Run("registration rejects invalid protocol and stale proof", func(t *testing.T) {
		for _, failure := range []string{"challenge", "origin", "rp", "cross_origin", "uv"} {
			flow, challenge := passkeyChallenge(t, owner, "register")
			input := gin.H{"flow_token": flow, "credential": a.response(t, challenge, true, 0, failure)}
			assert.Equal(t, 401, owner.request(t, http.MethodPost, "/passkey/register/finish", input, nil).Code, failure)
			assert.Equal(t, 401, owner.request(t, http.MethodPost, "/passkey/register/finish", input, nil).Code)
		}
		flow, challenge := passkeyChallenge(t, owner, "register")
		require.NoError(t, f.server.DB.Model(&Session{}).Where("token_hash = ?", digest(owner.cookies[sessionCookie].Value)).Update("created_at", time.Now().Add(-6*time.Minute)).Error)
		assert.Equal(t, 401, owner.request(t, http.MethodPost, "/passkey/register/finish", gin.H{"flow_token": flow, "credential": a.response(t, challenge, true, 0, "")}, nil).Code)
		assert.Equal(t, 401, owner.request(t, http.MethodPost, "/passkey/register/begin", gin.H{}, nil).Code)
		require.NoError(t, f.server.DB.Model(&Session{}).Where("token_hash = ?", digest(owner.cookies[sessionCookie].Value)).Update("created_at", time.Now()).Error)
	})
	flow, challenge := passkeyChallenge(t, owner, "register")
	oldCookie := owner.cookies[sessionCookie].Value
	input := gin.H{"flow_token": flow, "credential": a.response(t, challenge, true, 0, "")}
	require.Equal(t, 200, owner.request(t, http.MethodPost, "/passkey/register/finish", input, nil).Code)
	assert.NotEqual(t, oldCookie, owner.cookies[sessionCookie].Value)
	assert.Equal(t, 401, owner.request(t, http.MethodPost, "/passkey/register/finish", input, nil).Code)
	var methods struct{ Passkeys []PasskeyCredential }
	require.Equal(t, 200, owner.request(t, http.MethodGet, "/auth-methods", nil, &methods).Code)
	require.Len(t, methods.Passkeys, 1)
	keyID := methods.Passkeys[0].KeyHash

	t.Run("signed discoverable login and replay", func(t *testing.T) {
		browser := f.browser()
		flow, challenge := passkeyChallenge(t, browser, "login")
		var session struct{ User User }
		input := gin.H{"flow_token": flow, "credential": a.response(t, challenge, false, 1, "")}
		require.Equal(t, 200, browser.request(t, http.MethodPost, "/passkey/login/finish", input, &session).Code)
		assert.Equal(t, user.ID, session.User.ID)
		assert.Equal(t, 401, browser.request(t, http.MethodPost, "/passkey/login/finish", input, nil).Code)
	})
	for _, failure := range []string{"challenge", "origin", "rp", "cross_origin", "uv", "handle", "signature", "counter", "expiry", "browser"} {
		t.Run("login rejects "+failure, func(t *testing.T) {
			browser := f.browser()
			flow, challenge := passkeyChallenge(t, browser, "login")
			counter := uint32(2)
			if failure == "counter" {
				counter = 1
			}
			if failure == "expiry" {
				require.NoError(t, f.server.DB.Model(&AuthFlow{}).Where("token_hash = ?", digest(flow)).Update("expires_at", time.Now().UTC().Add(-time.Second)).Error)
			}
			if failure == "browser" {
				delete(browser.cookies, authFlowCookie)
			}
			assert.Equal(t, 401, browser.request(t, http.MethodPost, "/passkey/login/finish", gin.H{"flow_token": flow, "credential": a.response(t, challenge, false, counter, failure)}, nil).Code)
			assert.Nil(t, browser.cookies[sessionCookie])
		})
	}
	t.Run("verification stays on the current account and rotates the session", func(t *testing.T) {
		flow, challenge := passkeyChallenge(t, other, "verify")
		assert.Equal(t, 401, other.request(t, http.MethodPost, "/passkey/verify/finish", gin.H{"flow_token": flow, "credential": a.response(t, challenge, false, 2, "")}, nil).Code)
		flow, challenge = passkeyChallenge(t, owner, "verify")
		oldCookie := owner.cookies[sessionCookie].Value
		require.Equal(t, 200, owner.request(t, http.MethodPost, "/passkey/verify/finish", gin.H{"flow_token": flow, "credential": a.response(t, challenge, false, 2, "")}, nil).Code)
		assert.NotEqual(t, oldCookie, owner.cookies[sessionCookie].Value)
	})
	t.Run("deletion requires ownership and retains a usable login method", func(t *testing.T) {
		assert.Equal(t, 404, other.request(t, http.MethodPost, "/passkey/"+keyID+"/delete", gin.H{}, nil).Code)
		f.server.Auth.PasswordLogin = false
		assert.Equal(t, 400, owner.request(t, http.MethodPost, "/passkey/"+keyID+"/delete", gin.H{}, nil).Code)
		f.server.Auth.PasswordLogin = true
		second := f.browser()
		flow, challenge := passkeyChallenge(t, second, "login")
		require.Equal(t, 200, second.request(t, http.MethodPost, "/passkey/login/finish", gin.H{"flow_token": flow, "credential": a.response(t, challenge, false, 3, "")}, nil).Code)
		require.Equal(t, 200, owner.request(t, http.MethodPost, "/passkey/"+keyID+"/delete", gin.H{}, nil).Code)
		assert.Equal(t, 401, second.request(t, http.MethodGet, "/session", nil, nil).Code)
		assert.Equal(t, 200, owner.request(t, http.MethodGet, "/session", nil, nil).Code)
		flow, challenge = passkeyChallenge(t, second, "login")
		assert.Equal(t, 401, second.request(t, http.MethodPost, "/passkey/login/finish", gin.H{"flow_token": flow, "credential": a.response(t, challenge, false, 4, "")}, nil).Code)
	})
}

func TestPlatformWeChatAndStatus(t *testing.T) {
	f := newAuthFixture(t)
	calls := 0
	bridge := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		assert.Equal(t, "/api/wechat/user", r.URL.Path)
		assert.Equal(t, "synthetic-bridge-secret", r.Header.Get("Authorization"))
		assert.NotEmpty(t, r.URL.Query().Get("code"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true,"data":"wechat-subject"}`))
	}))
	t.Cleanup(bridge.Close)
	f.server.Auth.WeChat = WeChatConfig{Enabled: true, Server: bridge.URL, Token: "synthetic-bridge-secret", QRCode: "https://example.test/qr.png"}
	browser := f.browser()
	for _, tc := range []struct {
		code   string
		status int
	}{{"synthetic-wx-code", 200}, {"synthetic-wx-code", 401}, {"another-wx-code", 200}} {
		code := tc.code
		var flow struct {
			Token string `json:"flow_token"`
		}
		require.Equal(t, 200, browser.request(t, http.MethodPost, "/wechat/start", gin.H{}, &flow).Code)
		request := gin.H{"flow_token": flow.Token, "code": code}
		assert.Equal(t, 401, f.browser().request(t, http.MethodPost, "/wechat/finish", request, nil).Code)
		response := browser.request(t, http.MethodPost, "/wechat/finish", request, nil)
		assert.Equal(t, tc.status, response.Code)
		assert.Equal(t, 401, browser.request(t, http.MethodPost, "/wechat/finish", request, nil).Code)
	}
	assert.Equal(t, 2, calls, "reused codes must not reach the bridge again")
	status := browser.request(t, http.MethodGet, "/status", nil, nil)
	assert.NotContains(t, status.Body.String(), "synthetic-bridge-secret")
	assert.NotContains(t, status.Body.String(), bridge.URL)
	assert.True(t, gjson.GetBytes(status.Body.Bytes(), "wechat_login").Bool())
	f.server.Auth.WeChat.Enabled = false
	f.server.Auth.PasswordLogin = false
	status = browser.request(t, http.MethodGet, "/status", nil, nil)
	assert.False(t, gjson.GetBytes(status.Body.Bytes(), "register_enabled").Bool())
	assert.False(t, gjson.GetBytes(status.Body.Bytes(), "wechat_login").Bool())
	assert.Equal(t, 404, browser.request(t, http.MethodPost, "/wechat/start", gin.H{}, nil).Code)
	assert.Equal(t, 403, browser.request(t, http.MethodPost, "/login", credentials{}, nil).Code)
	f.server.Auth.Passkey = false
	assert.Equal(t, 404, browser.request(t, http.MethodPost, "/passkey/login/begin", gin.H{}, nil).Code)
	assert.Equal(t, 404, browser.request(t, http.MethodPost, "/oauth/github/start", gin.H{}, nil).Code)
}

func TestPlatformWorkspaceViews(t *testing.T) {
	f := newAuthFixture(t)
	owner, user := f.account(t, "usage-owner")
	stranger, strangerUser := f.account(t, "usage-stranger")
	admin := f.browser()
	require.Equal(t, 200, admin.request(t, http.MethodPost, "/login", credentials{"admin@example.test", os.Getenv("PLATFORM_ADMIN_PASSWORD")}, nil).Code)
	now := time.Now().UTC()
	expired, soon := now.Add(-time.Hour), now.Add(48*time.Hour)
	workspaces := []tenant.Workspace{
		{Slug: "owned", Name: "Owned", OwnerPlatformUserID: user.ID, PlanID: 1, Status: "active", PlanExpiresAt: &soon},
		{Slug: "expired", Name: "Expired", OwnerPlatformUserID: user.ID, PlanID: 3, Status: "active", PlanExpiresAt: &expired},
		{Slug: "suspended", Name: "Suspended", OwnerPlatformUserID: user.ID, PlanID: 2, Status: "suspended"},
		{Slug: "foreign", Name: "Foreign", OwnerPlatformUserID: strangerUser.ID, PlanID: 2, Status: "active"},
	}
	require.NoError(t, f.server.DB.Create(&workspaces).Error)
	month := now.Format("2006-01")
	previousMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC).AddDate(0, -1, 0).Format("2006-01")
	usages := []plan.Usage{{TenantID: workspaces[0].ID, Month: month, Requests: 10000}, {TenantID: workspaces[0].ID, Month: previousMonth, Requests: 80}, {TenantID: workspaces[1].ID, Month: month, Requests: 200}, {TenantID: workspaces[3].ID, Month: month, Requests: 500}}
	require.NoError(t, f.server.DB.Create(&usages).Error)
	assignment := plan.Assignment{TenantID: workspaces[0].ID, PlanID: 1, PlatformUserID: user.ID, AdministratorID: 1, Source: "manual", ExpiresAt: soon}
	require.NoError(t, f.server.DB.Create(&assignment).Error)
	path := fmt.Sprintf("/tenants/%d", workspaces[0].ID)
	var detail struct {
		Tenant      tenant.Workspace
		Usage       plan.Usage
		History     []plan.Usage
		Assignments []plan.Assignment
	}
	require.Equal(t, 200, owner.request(t, http.MethodGet, path, nil, &detail).Code)
	assert.Equal(t, workspaces[0].ID, detail.Tenant.ID)
	assert.Equal(t, int64(10000), detail.Usage.Requests)
	assert.Len(t, detail.History, 2)
	require.Len(t, detail.Assignments, 1)
	assert.Equal(t, "manual", detail.Assignments[0].Source)
	assert.Equal(t, 404, stranger.request(t, http.MethodGet, path, nil, nil).Code)
	assert.Equal(t, 403, stranger.request(t, http.MethodGet, "/admin"+path, nil, nil).Code)
	assert.Equal(t, 200, admin.request(t, http.MethodGet, "/admin"+path, nil, nil).Code)
	for _, scope := range []struct {
		browser                      *authBrowser
		path                         string
		workspaces, requests, active int64
	}{{owner, "/usage", 3, 10200, 1}, {admin, "/admin/usage", 4, 10700, 2}} {
		response := scope.browser.request(t, http.MethodGet, scope.path, nil, nil)
		require.Equal(t, 200, response.Code, response.Body.String())
		body := response.Body.Bytes()
		assert.Equal(t, scope.workspaces, gjson.GetBytes(body, "summary.workspaces").Int())
		assert.Equal(t, scope.requests, gjson.GetBytes(body, "summary.requests").Int())
		assert.Equal(t, scope.active, gjson.GetBytes(body, "summary.active").Int())
		for _, key := range []string{"suspended", "expired", "expiring_soon", "exhausted"} {
			assert.Equal(t, int64(1), gjson.GetBytes(body, "summary."+key).Int(), key)
		}
		assert.Equal(t, int64(80), gjson.GetBytes(body, "history.0.requests").Int())
		assert.Equal(t, int64(1), gjson.GetBytes(body, `plans.#(name=="Lite").workspaces`).Int())
	}
	assert.Equal(t, 403, owner.request(t, http.MethodGet, "/admin/usage", nil, nil).Code)
}

// The previous release's platform schema, including its email constraint.
type legacyPlatformUser struct {
	ID                 int64  `gorm:"primaryKey"`
	Email              string `gorm:"size:254;uniqueIndex;not null"`
	PasswordHash       string `gorm:"type:text;not null"`
	Role               string `gorm:"size:16;not null"`
	Status             string `gorm:"size:16;not null;default:active"`
	MustChangePassword bool
	SessionVersion     int64 `gorm:"not null;default:1"`
	TenantCount        int   `gorm:"not null"`
	CreatedAt          time.Time
}

func (legacyPlatformUser) TableName() string { return "platform_users" }

func TestPlatformExternalEmailMigration(t *testing.T) {
	before := legacyPlatformUser{ID: 40, Email: "legacy@example.test", PasswordHash: "existing-synthetic-hash", Role: "user", Status: "active", SessionVersion: 3, TenantCount: 2, CreatedAt: time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)}
	f := newAuthFixture(t, func(db *gorm.DB) {
		require.NoError(t, db.AutoMigrate(&legacyPlatformUser{}))
		require.NoError(t, db.Create(&before).Error)
		require.NoError(t, db.Exec("CREATE INDEX platform_legacy_role ON platform_users (role)").Error)
		if common.UsingMainDatabase(common.DatabaseTypeSQLite) {
			require.NoError(t, db.Exec("CREATE TRIGGER platform_nonnegative_count BEFORE UPDATE OF tenant_count ON platform_users WHEN NEW.tenant_count < 0 BEGIN SELECT RAISE(ABORT, 'negative workspace count'); END").Error)
		}
	})
	for range 2 {
		// Re-open as startup does, before tenant-scope callbacks are installed.
		db, err := gorm.Open(f.server.DB.Dialector, &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
		require.NoError(t, err)
		_, err = New(db)
		require.NoError(t, err)
		var after User
		require.NoError(t, db.First(&after, before.ID).Error)
		require.NotNil(t, after.Email)
		assert.Equal(t, before.Email, *after.Email)
		assert.Equal(t, before.PasswordHash, after.PasswordHash)
		assert.Equal(t, before.Role, after.Role)
		assert.Equal(t, before.SessionVersion, after.SessionVersion)
		assert.Equal(t, before.TenantCount, after.TenantCount)
		assert.True(t, before.CreatedAt.Equal(after.CreatedAt))
		assert.True(t, db.Migrator().HasIndex(&User{}, "platform_legacy_role"))
		if common.UsingMainDatabase(common.DatabaseTypeSQLite) {
			assert.Error(t, db.Model(&User{}).Where("id = ?", before.ID).Update("tenant_count", -1).Error)
		}
		for _, name := range []string{"External one", "External two"} {
			require.NoError(t, db.Create(&User{DisplayName: name, Role: "user"}).Error)
		}
		assert.Error(t, db.Create(&User{Email: &before.Email, Role: "user"}).Error, "existing email uniqueness survives upgrades")
		sqlDB, err := db.DB()
		require.NoError(t, err)
		require.NoError(t, sqlDB.Close())
	}
}

func TestPlatformPublicAuthConfiguration(t *testing.T) {
	f := newAuthFixture(t)
	for _, slug := range []string{"GITHUB", "DISCORD", "LINUXDO", "OIDC", "TELEGRAM"} {
		t.Setenv("PLATFORM_"+slug+"_CLIENT_ID", "synthetic-config-client")
		t.Setenv("PLATFORM_"+slug+"_CLIENT_SECRET", "synthetic-config-secret")
	}
	t.Setenv("PLATFORM_OIDC_ISSUER", "https://identity.example.test")
	t.Setenv("PLATFORM_CUSTOM_OAUTH_PROVIDERS", `[{"slug":"company","name":"Company SSO","client_id":"synthetic-custom-client","client_secret":"synthetic-custom-secret","issuer":"https://company.example.test","scopes":["openid","profile"]}]`)
	config, err := loadAuthConfig()
	require.NoError(t, err)
	f.server.Auth = config
	response := f.browser().request(t, http.MethodGet, "/status", nil, nil)
	for _, flag := range []string{"github_oauth", "discord_oauth", "linuxdo_oauth", "oidc_enabled", "telegram_oauth", "passkey_login"} {
		assert.True(t, gjson.GetBytes(response.Body.Bytes(), flag).Bool(), flag)
	}
	assert.Equal(t, "Company SSO", gjson.GetBytes(response.Body.Bytes(), "custom_oauth_providers.0.name").String())
	assert.NotContains(t, response.Body.String(), "synthetic-")
	assert.Equal(t, []string{"company", "oidc", "telegram"}, []string{gjson.GetBytes(response.Body.Bytes(), "reauthentication_providers.0").String(), gjson.GetBytes(response.Body.Bytes(), "reauthentication_providers.1").String(), gjson.GetBytes(response.Body.Bytes(), "reauthentication_providers.2").String()})
	t.Setenv("PLATFORM_GITHUB_ENABLED", "false")
	config, err = loadAuthConfig()
	require.NoError(t, err)
	f.server.Auth = config
	assert.False(t, gjson.GetBytes(f.browser().request(t, http.MethodGet, "/status", nil, nil).Body.Bytes(), "github_oauth").Bool())
	t.Setenv("PLATFORM_OIDC_ISSUER", "http://identity.example.test")
	_, err = loadAuthConfig()
	assert.Error(t, err, "non-local identity endpoints require HTTPS")
}

type platformMailCapture struct {
	mu   sync.Mutex
	html map[string]string
}

func newPlatformMailCapture(t *testing.T, f *authFixture) *platformMailCapture {
	t.Helper()
	capture := &platformMailCapture{html: make(map[string]string)}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload struct {
			To   []string `json:"to"`
			HTML string   `json:"html"`
		}
		if err := common.DecodeJson(r.Body, &payload); err != nil || len(payload.To) != 1 {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		capture.mu.Lock()
		capture.html[payload.To[0]] = payload.HTML
		capture.mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)
	f.server.Mail = MailSettings{Enabled: true, BaseURL: server.URL, APIKey: "synthetic-mail-key", From: "no-reply@example.test", Notifications: true}
	return capture
}

var mailCodePattern = regexp.MustCompile(`<strong>(\d{6})</strong>`)

func (m *platformMailCapture) code(t *testing.T, to string) string {
	t.Helper()
	m.mu.Lock()
	defer m.mu.Unlock()
	matches := mailCodePattern.FindStringSubmatch(m.html[to])
	require.NotEmpty(t, matches, "no verification mail for "+to)
	return matches[1]
}

func TestPlatformEmailChange(t *testing.T) {
	f := newAuthFixture(t)
	mailbox := newPlatformMailCapture(t, f)
	alpha, user := f.account(t, "alpha")
	_, _ = f.account(t, "beta")

	// Validation failures never send mail.
	response := alpha.request(t, http.MethodPost, "/email/change/start", gin.H{"email": "not-an-email"}, nil)
	assert.Equal(t, http.StatusBadRequest, response.Code)
	assert.Equal(t, "invalid_email", gjson.GetBytes(response.Body.Bytes(), "code").String())
	response = alpha.request(t, http.MethodPost, "/email/change/start", gin.H{"email": "alpha@example.test"}, nil)
	assert.Equal(t, http.StatusBadRequest, response.Code)
	assert.Equal(t, "email_unchanged", gjson.GetBytes(response.Body.Bytes(), "code").String())
	response = alpha.request(t, http.MethodPost, "/email/change/start", gin.H{"email": "beta@example.test"}, nil)
	assert.Equal(t, http.StatusConflict, response.Code)
	assert.Equal(t, "email_taken", gjson.GetBytes(response.Body.Bytes(), "code").String())

	const updated = "alpha-renamed@example.test"
	require.Equal(t, http.StatusOK, alpha.request(t, http.MethodPost, "/email/change/start", gin.H{"email": updated}, nil).Code)
	code := mailbox.code(t, updated)
	wrong := "000000"
	if code == wrong {
		wrong = "000001"
	}
	response = alpha.request(t, http.MethodPost, "/email/change/finish", gin.H{"email": updated, "code": wrong}, nil)
	assert.Equal(t, http.StatusUnauthorized, response.Code)
	assert.Equal(t, "invalid_verification_code", gjson.GetBytes(response.Body.Bytes(), "code").String())
	require.Equal(t, http.StatusOK, alpha.request(t, http.MethodPost, "/email/change/finish", gin.H{"email": updated, "code": code}, nil).Code)

	var stored User
	require.NoError(t, f.server.DB.First(&stored, user.ID).Error)
	require.NotNil(t, stored.Email)
	assert.Equal(t, updated, *stored.Email)
	assert.NotNil(t, stored.EmailVerifiedAt)
	// The session survives an email change and reports the new address.
	var session struct{ User User }
	require.Equal(t, http.StatusOK, alpha.request(t, http.MethodGet, "/session", nil, &session).Code)
	require.NotNil(t, session.User.Email)
	assert.Equal(t, updated, *session.User.Email)
	// The old address is notified and codes are single-use.
	mailbox.mu.Lock()
	_, notified := mailbox.html["alpha@example.test"]
	mailbox.mu.Unlock()
	assert.True(t, notified, "old address receives a change notification")
	response = alpha.request(t, http.MethodPost, "/email/change/finish", gin.H{"email": updated, "code": code}, nil)
	assert.Equal(t, http.StatusUnauthorized, response.Code)

	// A stale session must re-authenticate before starting another change.
	require.NoError(t, f.server.DB.Model(&Session{}).Where("user_id = ?", user.ID).Update("created_at", time.Now().Add(-10*time.Minute)).Error)
	response = alpha.request(t, http.MethodPost, "/email/change/start", gin.H{"email": "alpha-third@example.test"}, nil)
	assert.Equal(t, http.StatusUnauthorized, response.Code)
	assert.Equal(t, "recent_login_required", gjson.GetBytes(response.Body.Bytes(), "code").String())
}

func TestPlatformPasswordLength(t *testing.T) {
	f := newAuthFixture(t)
	browser := f.browser()
	response := browser.request(t, http.MethodPost, "/register", credentials{"length@example.test", "Kx9#mQ2vL"}, nil)
	assert.Equal(t, http.StatusBadRequest, response.Code)
	assert.Equal(t, "invalid_email_or_password_length", gjson.GetBytes(response.Body.Bytes(), "code").String())
	require.Equal(t, http.StatusOK, browser.request(t, http.MethodPost, "/register", credentials{"length@example.test", "Kx9#mQ2vLz"}, nil).Code)
	require.Equal(t, http.StatusOK, browser.request(t, http.MethodPost, "/login", credentials{"length@example.test", "Kx9#mQ2vLz"}, nil).Code)
}
