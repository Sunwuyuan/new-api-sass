package common

import (
	"bufio"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"net/smtp"
	"strconv"
	"strings"
	"testing"
	"time"

	testtenant "github.com/QuantumNous/new-api/internal/testtenant"
	"github.com/stretchr/testify/require"
)

type fakeSMTPServer struct {
	listener          net.Listener
	host              string
	port              int
	cert              tls.Certificate
	advertiseSTARTTLS bool
	authMechanisms    []string
	messages          chan string
	authCommands      chan string
	startTLSCommands  chan string
}

func newFakeSMTPServer(t *testing.T) *fakeSMTPServer {
	return newFakeSMTPServerWithSTARTTLSAdvertisement(t, true)
}

func newFakeSMTPServerWithSTARTTLSAdvertisement(t *testing.T, advertiseSTARTTLS bool) *fakeSMTPServer {
	t.Helper()

	cert, err := newTestTLSCertificate()
	require.NoError(t, err)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	host, portText, err := net.SplitHostPort(listener.Addr().String())
	require.NoError(t, err)
	port, err := strconv.Atoi(portText)
	require.NoError(t, err)

	server := &fakeSMTPServer{
		listener:          listener,
		host:              host,
		port:              port,
		cert:              cert,
		advertiseSTARTTLS: advertiseSTARTTLS,
		authMechanisms:    []string{"PLAIN", "LOGIN"},
		messages:          make(chan string, 1),
		authCommands:      make(chan string, 1),
		startTLSCommands:  make(chan string, 1),
	}
	go server.serve()
	return server
}

func newFakeImplicitTLSSMTPServer(t *testing.T) *fakeSMTPServer {
	t.Helper()

	cert, err := newTestTLSCertificate()
	require.NoError(t, err)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	host, portText, err := net.SplitHostPort(listener.Addr().String())
	require.NoError(t, err)
	port, err := strconv.Atoi(portText)
	require.NoError(t, err)

	server := &fakeSMTPServer{
		listener:          tls.NewListener(listener, &tls.Config{Certificates: []tls.Certificate{cert}}),
		host:              host,
		port:              port,
		cert:              cert,
		advertiseSTARTTLS: false,
		authMechanisms:    []string{"PLAIN", "LOGIN"},
		messages:          make(chan string, 1),
		authCommands:      make(chan string, 1),
		startTLSCommands:  make(chan string, 1),
	}
	go server.serve()
	return server
}

func (s *fakeSMTPServer) close() {
	_ = s.listener.Close()
}

func (s *fakeSMTPServer) serve() {
	conn, err := s.listener.Accept()
	if err != nil {
		return
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(5 * time.Second))

	rw := bufio.NewReadWriter(bufio.NewReader(conn), bufio.NewWriter(conn))
	if err := writeSMTPLine(rw, "220 fake.smtp.local ESMTP"); err != nil {
		return
	}

	encrypted := false
	for {
		line, err := rw.ReadString('\n')
		if err != nil {
			return
		}
		command := strings.TrimRight(line, "\r\n")
		upperCommand := strings.ToUpper(command)

		switch {
		case strings.HasPrefix(upperCommand, "EHLO"):
			if err := writeSMTPLine(rw, "250-fake.smtp.local"); err != nil {
				return
			}
			if !encrypted && s.advertiseSTARTTLS {
				if err := writeSMTPLine(rw, "250-STARTTLS"); err != nil {
					return
				}
			}
			if len(s.authMechanisms) > 0 {
				if err := writeSMTPLine(rw, "250 AUTH "+strings.Join(s.authMechanisms, " ")); err != nil {
					return
				}
			} else if err := writeSMTPLine(rw, "250 8BITMIME"); err != nil {
				return
			}
		case upperCommand == "STARTTLS":
			if encrypted || !s.advertiseSTARTTLS {
				if err := writeSMTPLine(rw, "502 5.5.1 STARTTLS not supported"); err != nil {
					return
				}
				continue
			}
			select {
			case s.startTLSCommands <- command:
			default:
			}
			if err := writeSMTPLine(rw, "220 2.0.0 Ready to start TLS"); err != nil {
				return
			}
			tlsConn := tls.Server(conn, &tls.Config{Certificates: []tls.Certificate{s.cert}})
			if err := tlsConn.Handshake(); err != nil {
				return
			}
			conn = tlsConn
			rw = bufio.NewReadWriter(bufio.NewReader(conn), bufio.NewWriter(conn))
			encrypted = true
		case strings.HasPrefix(upperCommand, "AUTH"):
			select {
			case s.authCommands <- command:
			default:
			}
			if err := writeSMTPLine(rw, "235 2.7.0 Authentication successful"); err != nil {
				return
			}
		case strings.HasPrefix(upperCommand, "MAIL FROM:"):
			if err := writeSMTPLine(rw, "250 2.1.0 Sender OK"); err != nil {
				return
			}
		case strings.HasPrefix(upperCommand, "RCPT TO:"):
			if err := writeSMTPLine(rw, "250 2.1.5 Recipient OK"); err != nil {
				return
			}
		case upperCommand == "DATA":
			if err := writeSMTPLine(rw, "354 End data with <CR><LF>.<CR><LF>"); err != nil {
				return
			}
			var data strings.Builder
			for {
				dataLine, err := rw.ReadString('\n')
				if err != nil {
					return
				}
				if strings.TrimRight(dataLine, "\r\n") == "." {
					break
				}
				data.WriteString(dataLine)
			}
			s.messages <- data.String()
			if err := writeSMTPLine(rw, "250 2.0.0 Queued"); err != nil {
				return
			}
		case upperCommand == "QUIT":
			_ = writeSMTPLine(rw, "221 2.0.0 Bye")
			return
		default:
			if err := writeSMTPLine(rw, "502 5.5.1 Command not implemented"); err != nil {
				return
			}
		}
	}
}

func writeSMTPLine(rw *bufio.ReadWriter, line string) error {
	_, err := rw.WriteString(line + "\r\n")
	if err != nil {
		return err
	}
	return rw.Flush()
}

func newTestTLSCertificate() (tls.Certificate, error) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return tls.Certificate{}, err
	}

	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName: "aixinexchange01.aixin-chip.com",
		},
		NotBefore:   time.Now().Add(-time.Hour),
		NotAfter:    time.Now().Add(time.Hour),
		KeyUsage:    x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:    []string{"aixinexchange01", "aixinexchange01.aixin-chip.com"},
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &privateKey.PublicKey, privateKey)
	if err != nil {
		return tls.Certificate{}, err
	}

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(privateKey)})
	return tls.X509KeyPair(certPEM, keyPEM)
}

func withSMTPSettings(t *testing.T) {
	t.Helper()
	originalSMTPServer := TenantState(testtenant.Context()).SMTPServer
	originalSMTPPort := TenantState(testtenant.Context()).SMTPPort
	originalSMTPSSLEnabled := TenantState(testtenant.Context()).SMTPSSLEnabled
	originalSMTPStartTLSEnabled := TenantState(testtenant.Context()).SMTPStartTLSEnabled
	originalSMTPInsecureSkipVerify := TenantState(testtenant.Context()).SMTPInsecureSkipVerify
	originalSMTPForceAuthLogin := TenantState(testtenant.Context()).SMTPForceAuthLogin
	originalSMTPAccount := TenantState(testtenant.Context()).SMTPAccount
	originalSMTPFrom := TenantState(testtenant.Context()).SMTPFrom
	originalSMTPToken := TenantState(testtenant.Context()).SMTPToken
	originalSystemName := TenantState(testtenant.Context()).SystemName

	t.Cleanup(func() {
		TenantState(testtenant.Context()).SMTPServer = originalSMTPServer
		TenantState(testtenant.Context()).SMTPPort = originalSMTPPort
		TenantState(testtenant.Context()).SMTPSSLEnabled = originalSMTPSSLEnabled
		TenantState(testtenant.Context()).SMTPStartTLSEnabled = originalSMTPStartTLSEnabled
		TenantState(testtenant.Context()).SMTPInsecureSkipVerify = originalSMTPInsecureSkipVerify
		TenantState(testtenant.Context()).SMTPForceAuthLogin = originalSMTPForceAuthLogin
		TenantState(testtenant.Context()).SMTPAccount = originalSMTPAccount
		TenantState(testtenant.Context()).SMTPFrom = originalSMTPFrom
		TenantState(testtenant.Context()).SMTPToken = originalSMTPToken
		TenantState(testtenant.Context()).SystemName = originalSystemName
	})
}

func TestSendEmailUsesExplicitStartTLSWithInsecureCertificate(t *testing.T) {
	server := newFakeSMTPServer(t)
	defer server.close()
	withSMTPSettings(t)

	TenantState(testtenant.Context()).SMTPServer = server.host
	TenantState(testtenant.Context()).SMTPPort = server.port
	TenantState(testtenant.Context()).SMTPSSLEnabled = false
	TenantState(testtenant.Context()).SMTPStartTLSEnabled = true
	TenantState(testtenant.Context()).SMTPInsecureSkipVerify = true
	TenantState(testtenant.Context()).SMTPForceAuthLogin = false
	TenantState(testtenant.Context()).SMTPAccount = "sender@example.com"
	TenantState(testtenant.Context()).SMTPFrom = "sender@example.com"
	TenantState(testtenant.Context()).SMTPToken = "secret"
	TenantState(testtenant.Context()).SystemName = "New API"

	err := SendEmail(testtenant.Context(), "Verification", "receiver@example.com", "<p>123456</p>")
	require.NoError(t, err)

	select {
	case message := <-server.messages:
		require.Contains(t, message, "Subject: =?UTF-8?B?")
		require.Contains(t, message, "<p>123456</p>")
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for SMTP DATA")
	}
}

func TestSendEmailExplicitStartTLSRequiresServerSupport(t *testing.T) {
	server := newFakeSMTPServerWithSTARTTLSAdvertisement(t, false)
	defer server.close()
	withSMTPSettings(t)

	TenantState(testtenant.Context()).SMTPServer = server.host
	TenantState(testtenant.Context()).SMTPPort = server.port
	TenantState(testtenant.Context()).SMTPSSLEnabled = false
	TenantState(testtenant.Context()).SMTPStartTLSEnabled = true
	TenantState(testtenant.Context()).SMTPInsecureSkipVerify = true
	TenantState(testtenant.Context()).SMTPForceAuthLogin = false
	TenantState(testtenant.Context()).SMTPAccount = "sender@example.com"
	TenantState(testtenant.Context()).SMTPFrom = "sender@example.com"
	TenantState(testtenant.Context()).SMTPToken = "secret"
	TenantState(testtenant.Context()).SystemName = "New API"

	err := SendEmail(testtenant.Context(), "Verification", "receiver@example.com", "<p>123456</p>")
	require.Error(t, err)
	require.Contains(t, err.Error(), "STARTTLS")
}

func TestSendEmailDoesNotAutoUpgradeWhenStartTLSDisabled(t *testing.T) {
	server := newFakeSMTPServerWithSTARTTLSAdvertisement(t, true)
	defer server.close()
	withSMTPSettings(t)

	TenantState(testtenant.Context()).SMTPServer = server.host
	TenantState(testtenant.Context()).SMTPPort = server.port
	TenantState(testtenant.Context()).SMTPSSLEnabled = false
	TenantState(testtenant.Context()).SMTPStartTLSEnabled = false
	TenantState(testtenant.Context()).SMTPInsecureSkipVerify = false
	TenantState(testtenant.Context()).SMTPForceAuthLogin = false
	TenantState(testtenant.Context()).SMTPAccount = "sender@example.com"
	TenantState(testtenant.Context()).SMTPFrom = "sender@example.com"
	TenantState(testtenant.Context()).SMTPToken = "secret"
	TenantState(testtenant.Context()).SystemName = "New API"

	err := SendEmail(testtenant.Context(), "Verification", "receiver@example.com", "<p>123456</p>")
	require.NoError(t, err)

	select {
	case command := <-server.startTLSCommands:
		t.Fatalf("unexpected SMTP STARTTLS command: %s", command)
	default:
	}

	select {
	case message := <-server.messages:
		require.Contains(t, message, "<p>123456</p>")
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for SMTP DATA")
	}
}

func TestSMTPPlainAuthRejectsRemotePlaintextConnection(t *testing.T) {
	server := newFakeSMTPServerWithSTARTTLSAdvertisement(t, false)
	defer server.close()
	withSMTPSettings(t)

	TenantState(testtenant.Context()).SMTPServer = "smtp.example.com"
	TenantState(testtenant.Context()).SMTPPort = server.port
	TenantState(testtenant.Context()).SMTPSSLEnabled = false
	TenantState(testtenant.Context()).SMTPStartTLSEnabled = false
	TenantState(testtenant.Context()).SMTPInsecureSkipVerify = false
	TenantState(testtenant.Context()).SMTPForceAuthLogin = false
	TenantState(testtenant.Context()).SMTPAccount = "sender@example.com"
	TenantState(testtenant.Context()).SMTPFrom = "sender@example.com"
	TenantState(testtenant.Context()).SMTPToken = "secret"

	conn, err := net.Dial("tcp", net.JoinHostPort(server.host, strconv.Itoa(server.port)))
	require.NoError(t, err)
	client, err := smtp.NewClient(conn, TenantState(testtenant.Context()).SMTPServer)
	require.NoError(t, err)

	err = client.Auth(getSMTPAuth(testtenant.Context()))
	require.Error(t, err)
	require.Contains(t, err.Error(), "unencrypted connection")

	select {
	case command := <-server.authCommands:
		t.Fatalf("unexpected SMTP auth command: %s", command)
	default:
	}
}

func TestNewSMTPClientHonorsExplicitStartTLSWhenPortIs465(t *testing.T) {
	server := newFakeSMTPServer(t)
	defer server.close()
	withSMTPSettings(t)

	TenantState(testtenant.Context()).SMTPServer = server.host
	TenantState(testtenant.Context()).SMTPPort = 465
	TenantState(testtenant.Context()).SMTPSSLEnabled = false
	TenantState(testtenant.Context()).SMTPStartTLSEnabled = true
	TenantState(testtenant.Context()).SMTPInsecureSkipVerify = true

	client, err := newSMTPClient(testtenant.Context(), fmt.Sprintf("%s:%d", server.host, server.port))
	require.NoError(t, err)
	defer client.Close()

	select {
	case command := <-server.startTLSCommands:
		require.Equal(t, "STARTTLS", command)
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for SMTP STARTTLS")
	}
}

func TestNewSMTPClientKeepsImplicitTLSForLegacyPort465(t *testing.T) {
	server := newFakeImplicitTLSSMTPServer(t)
	defer server.close()
	withSMTPSettings(t)

	TenantState(testtenant.Context()).SMTPServer = server.host
	TenantState(testtenant.Context()).SMTPPort = 465
	TenantState(testtenant.Context()).SMTPSSLEnabled = false
	TenantState(testtenant.Context()).SMTPStartTLSEnabled = false
	TenantState(testtenant.Context()).SMTPInsecureSkipVerify = true

	client, err := newSMTPClient(testtenant.Context(), fmt.Sprintf("%s:%d", server.host, server.port))
	require.NoError(t, err)
	defer client.Close()
}

func TestSendEmailSkipsAuthWhenCredentialsAreEmpty(t *testing.T) {
	server := newFakeSMTPServerWithSTARTTLSAdvertisement(t, false)
	defer server.close()
	withSMTPSettings(t)

	TenantState(testtenant.Context()).SMTPServer = server.host
	TenantState(testtenant.Context()).SMTPPort = server.port
	TenantState(testtenant.Context()).SMTPSSLEnabled = false
	TenantState(testtenant.Context()).SMTPStartTLSEnabled = false
	TenantState(testtenant.Context()).SMTPInsecureSkipVerify = false
	TenantState(testtenant.Context()).SMTPForceAuthLogin = false
	TenantState(testtenant.Context()).SMTPAccount = ""
	TenantState(testtenant.Context()).SMTPFrom = "sender@example.com"
	TenantState(testtenant.Context()).SMTPToken = ""
	TenantState(testtenant.Context()).SystemName = "New API"

	err := SendEmail(testtenant.Context(), "Verification", "receiver@example.com", "<p>123456</p>")
	require.NoError(t, err)

	select {
	case command := <-server.authCommands:
		t.Fatalf("unexpected SMTP auth command: %s", command)
	default:
	}

	select {
	case message := <-server.messages:
		require.Contains(t, message, "<p>123456</p>")
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for SMTP DATA")
	}
}

func TestSendEmailSkipsAuthWhenCredentialsAreIncomplete(t *testing.T) {
	server := newFakeSMTPServerWithSTARTTLSAdvertisement(t, false)
	defer server.close()
	withSMTPSettings(t)

	TenantState(testtenant.Context()).SMTPServer = server.host
	TenantState(testtenant.Context()).SMTPPort = server.port
	TenantState(testtenant.Context()).SMTPSSLEnabled = false
	TenantState(testtenant.Context()).SMTPStartTLSEnabled = false
	TenantState(testtenant.Context()).SMTPInsecureSkipVerify = false
	TenantState(testtenant.Context()).SMTPForceAuthLogin = false
	TenantState(testtenant.Context()).SMTPAccount = "sender@example.com"
	TenantState(testtenant.Context()).SMTPFrom = "sender@example.com"
	TenantState(testtenant.Context()).SMTPToken = ""
	TenantState(testtenant.Context()).SystemName = "New API"

	err := SendEmail(testtenant.Context(), "Verification", "receiver@example.com", "<p>123456</p>")
	require.NoError(t, err)

	select {
	case command := <-server.authCommands:
		t.Fatalf("unexpected SMTP auth command: %s", command)
	default:
	}

	select {
	case message := <-server.messages:
		require.Contains(t, message, "<p>123456</p>")
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for SMTP DATA")
	}
}

func TestSendEmailUsesNTLMWhenServerOnlySupportsNTLM(t *testing.T) {
	server := newFakeSMTPServer(t)
	server.authMechanisms = []string{"NTLM"}
	defer server.close()
	withSMTPSettings(t)

	TenantState(testtenant.Context()).SMTPServer = server.host
	TenantState(testtenant.Context()).SMTPPort = server.port
	TenantState(testtenant.Context()).SMTPSSLEnabled = false
	TenantState(testtenant.Context()).SMTPStartTLSEnabled = true
	TenantState(testtenant.Context()).SMTPInsecureSkipVerify = true
	TenantState(testtenant.Context()).SMTPForceAuthLogin = false
	TenantState(testtenant.Context()).SMTPAccount = "no-reply"
	TenantState(testtenant.Context()).SMTPFrom = "no-reply@example.com"
	TenantState(testtenant.Context()).SMTPToken = "secret"
	TenantState(testtenant.Context()).SystemName = "New API"

	err := SendEmail(testtenant.Context(), "Verification", "receiver@example.com", "<p>123456</p>")
	require.NoError(t, err)

	select {
	case command := <-server.authCommands:
		require.True(t, strings.HasPrefix(command, "AUTH NTLM "), "unexpected auth command: %s", command)
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for SMTP AUTH")
	}
}

func TestSendEmailUsesNTLMForMicrosoftAccountWhenServerOnlySupportsNTLM(t *testing.T) {
	server := newFakeSMTPServer(t)
	server.authMechanisms = []string{"NTLM"}
	defer server.close()
	withSMTPSettings(t)

	TenantState(testtenant.Context()).SMTPServer = server.host
	TenantState(testtenant.Context()).SMTPPort = server.port
	TenantState(testtenant.Context()).SMTPSSLEnabled = false
	TenantState(testtenant.Context()).SMTPStartTLSEnabled = true
	TenantState(testtenant.Context()).SMTPInsecureSkipVerify = true
	TenantState(testtenant.Context()).SMTPForceAuthLogin = false
	TenantState(testtenant.Context()).SMTPAccount = "no-reply@contoso.onmicrosoft.com"
	TenantState(testtenant.Context()).SMTPFrom = "no-reply@contoso.onmicrosoft.com"
	TenantState(testtenant.Context()).SMTPToken = "secret"
	TenantState(testtenant.Context()).SystemName = "New API"

	err := SendEmail(testtenant.Context(), "Verification", "receiver@example.com", "<p>123456</p>")
	require.NoError(t, err)

	select {
	case command := <-server.authCommands:
		require.True(t, strings.HasPrefix(command, "AUTH NTLM "), "unexpected auth command: %s", command)
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for SMTP AUTH")
	}
}

func TestSendEmailExplicitStartTLSRejectsUntrustedCertificateByDefault(t *testing.T) {
	server := newFakeSMTPServer(t)
	defer server.close()
	withSMTPSettings(t)

	TenantState(testtenant.Context()).SMTPServer = server.host
	TenantState(testtenant.Context()).SMTPPort = server.port
	TenantState(testtenant.Context()).SMTPSSLEnabled = false
	TenantState(testtenant.Context()).SMTPStartTLSEnabled = true
	TenantState(testtenant.Context()).SMTPInsecureSkipVerify = false
	TenantState(testtenant.Context()).SMTPForceAuthLogin = false
	TenantState(testtenant.Context()).SMTPAccount = "sender@example.com"
	TenantState(testtenant.Context()).SMTPFrom = "sender@example.com"
	TenantState(testtenant.Context()).SMTPToken = "secret"
	TenantState(testtenant.Context()).SystemName = "New API"

	err := SendEmail(testtenant.Context(), "Verification", "receiver@example.com", "<p>123456</p>")
	require.Error(t, err)
	require.Contains(t, fmt.Sprint(err), "certificate")
}
