package common

import context "context"

import (
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"net/smtp"
	"slices"
	"strings"
	"time"
)

func generateMessageID(tenantCtx context.Context) (string, error) {
	split := strings.Split(TenantState(tenantCtx).SMTPFrom, "@")
	if len(split) < 2 {
		return "", fmt.Errorf("invalid SMTP account")
	}
	domain := strings.Split(TenantState(tenantCtx).SMTPFrom, "@")[1]
	return fmt.Sprintf("<%d.%s@%s>", time.Now().UnixNano(), GetRandomString(12), domain), nil
}

func shouldUseSMTPLoginAuth(tenantCtx context.Context) bool {
	if TenantState(tenantCtx).SMTPForceAuthLogin {
		return true
	}
	return isOutlookServer(TenantState(tenantCtx).SMTPAccount) || slices.Contains(EmailLoginAuthServerList, TenantState(tenantCtx).SMTPServer)
}

func getSMTPAuth(tenantCtx context.Context) smtp.Auth {
	return AutoSMTPAuth(tenantCtx, TenantState(tenantCtx).SMTPAccount, TenantState(tenantCtx).SMTPToken)
}

func shouldAuthenticateSMTP(tenantCtx context.Context) bool {
	return TenantState(tenantCtx).SMTPAccount != "" && TenantState(tenantCtx).SMTPToken != ""
}

func smtpTLSConfig(tenantCtx context.Context) *tls.Config {
	return &tls.Config{
		ServerName:         TenantState(tenantCtx).SMTPServer,
		InsecureSkipVerify: TenantState(tenantCtx).SMTPInsecureSkipVerify, // #nosec G402 -- admin-controlled SMTP compatibility option.
	}
}

func newSMTPClient(tenantCtx context.Context, addr string) (*smtp.Client, error) {
	if TenantState(tenantCtx).SMTPSSLEnabled || (TenantState(tenantCtx).SMTPPort == 465 && !TenantState(tenantCtx).SMTPStartTLSEnabled) {
		conn, err := tls.Dial("tcp", addr, smtpTLSConfig(tenantCtx))
		if err != nil {
			return nil, err
		}
		client, err := smtp.NewClient(conn, TenantState(tenantCtx).SMTPServer)
		if err != nil {
			_ = conn.Close()
			return nil, err
		}
		return client, nil
	}

	client, err := smtp.Dial(addr)
	if err != nil {
		return nil, err
	}

	if TenantState(tenantCtx).SMTPStartTLSEnabled {
		startTLSSupported, _ := client.Extension("STARTTLS")
		if !startTLSSupported {
			_ = client.Close()
			return nil, fmt.Errorf("SMTP server does not support STARTTLS")
		}
		if err := client.StartTLS(smtpTLSConfig(tenantCtx)); err != nil {
			_ = client.Close()
			return nil, err
		}
	}

	return client, nil
}

// SendPlatformMail is set by the SaaS host. It must never import this package
// cycle; workspace SMTP stays the default when the option is off.
var SendPlatformMail func(tenantCtx context.Context, subject string, receiver string, content string) error

func SendEmail(tenantCtx context.Context, subject string, receiver string, content string) error {
	if TenantState(tenantCtx).PlatformMailEnabled {
		if SendPlatformMail == nil {
			return fmt.Errorf("platform mail is not configured")
		}
		return SendPlatformMail(tenantCtx, subject, receiver, content)
	}
	if TenantState(tenantCtx).SMTPFrom == "" { // for compatibility
		UpdateTenantSettings(tenantCtx, func(state *WorkspaceState) {
			if state.SMTPFrom == "" {
				state.SMTPFrom = state.SMTPAccount
			}
		})
	}
	id, err2 := generateMessageID(tenantCtx)
	if err2 != nil {
		return err2
	}
	if TenantState(tenantCtx).SMTPServer == "" && TenantState(tenantCtx).SMTPAccount == "" {
		return fmt.Errorf("SMTP 服务器未配置")
	}
	encodedSubject := fmt.Sprintf("=?UTF-8?B?%s?=", base64.StdEncoding.EncodeToString([]byte(subject)))
	mail := []byte(fmt.Sprintf("To: %s\r\n"+
		"From: %s <%s>\r\n"+
		"Subject: %s\r\n"+
		"Date: %s\r\n"+
		"Message-ID: %s\r\n"+ // 添加 Message-ID 头
		"Content-Type: text/html; charset=UTF-8\r\n\r\n%s\r\n",
		receiver, TenantState(tenantCtx).SystemName, TenantState(tenantCtx).SMTPFrom, encodedSubject, time.Now().Format(time.RFC1123Z), id, content))
	auth := getSMTPAuth(tenantCtx)
	addr := fmt.Sprintf("%s:%d", TenantState(tenantCtx).SMTPServer, TenantState(tenantCtx).SMTPPort)
	to := strings.Split(receiver, ";")
	var err error
	client, err := newSMTPClient(tenantCtx, addr)
	if err != nil {
		return err
	}
	defer client.Close()
	if shouldAuthenticateSMTP(tenantCtx) {
		if err = client.Auth(auth); err != nil {
			return err
		}
	}
	if err = client.Mail(TenantState(tenantCtx).SMTPFrom); err != nil {
		return err
	}
	for _, receiver := range to {
		if err = client.Rcpt(receiver); err != nil {
			return err
		}
	}
	w, err := client.Data()
	if err != nil {
		return err
	}
	_, err = w.Write(mail)
	if err != nil {
		return err
	}
	err = w.Close()
	if err != nil {
		return err
	}
	err = client.Quit()
	if err != nil {
		SysError(fmt.Sprintf("failed to send email to %s: %v", receiver, err))
	}
	return err
}
