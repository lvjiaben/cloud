// Package smtp provides the installation-owned, TLS-only SMTP channel.
package smtp

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net"
	"net/mail"
	netsmtp "net/smtp"
	"net/textproto"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-plugin-idc-core/backend/cap/secretcap"
	"lina-plugin-idc-notify/backend/cap/channelcap"
)

const (
	ProviderCode     = "smtp_tls_v1"
	hostField        = "host"
	portField        = "port"
	securityField    = "security"
	usernameField    = "username"
	passwordField    = "password"
	fromAddressField = "from_address"
	fromNameField    = "from_name"
	allowlistEnv     = "LINAPRO_IDC_NOTIFY_SMTP_ALLOWLIST"
	maximumMessage   = 256 * 1024
)

var declaration struct {
	sync.Once
	err error
}

// Declare registers the TLS-only SMTP adapter exactly once.
func Declare() error {
	declaration.Do(func() {
		declaration.err = channelcap.Declare(channelcap.Descriptor{
			ProviderCode: ProviderCode,
			Kind:         channelcap.KindEmail,
			Fields: []channelcap.Field{
				{Key: hostField, LabelKey: "notification.smtp.host", Required: true, MaxLength: 253},
				{Key: portField, LabelKey: "notification.smtp.port", Required: true, MaxLength: 3},
				{Key: securityField, LabelKey: "notification.smtp.security", Required: true, MaxLength: 16},
				{Key: usernameField, LabelKey: "notification.smtp.username", Required: true, MaxLength: 320},
				{Key: passwordField, LabelKey: "notification.smtp.password", Secret: true, Required: true, MaxLength: 4096},
				{Key: fromAddressField, LabelKey: "notification.smtp.from_address", Required: true, MaxLength: 320},
				{Key: fromNameField, LabelKey: "notification.smtp.from_name", MaxLength: 128},
			},
			Factory: func(accessor secretcap.Accessor) (channelcap.Adapter, error) {
				return newAdapter(accessor, parseAllowlist(os.Getenv(allowlistEnv)), &net.Dialer{Timeout: 15 * time.Second})
			},
		})
	})
	return declaration.err
}

type adapter struct {
	accessor secretcap.Accessor
	allowed  map[string]struct{}
	dialer   *net.Dialer
	purpose  secretcap.Purpose
}

type configuration struct {
	host        string
	port        string
	security    string
	username    string
	fromAddress string
	fromName    string
}

type smtpError struct {
	stage string
	err   error
}

func (e *smtpError) Error() string { return e.stage + ": " + e.err.Error() }
func (e *smtpError) Unwrap() error { return e.err }

func newAdapter(accessor secretcap.Accessor, allowed map[string]struct{}, dialer *net.Dialer) (*adapter, error) {
	if accessor == nil || allowed == nil || dialer == nil {
		return nil, gerror.New("smtp dependencies are unavailable")
	}
	purpose, err := secretcap.NewPurpose("notify.send")
	if err != nil {
		return nil, err
	}
	return &adapter{accessor: accessor, allowed: allowed, dialer: dialer, purpose: purpose}, nil
}

func (a *adapter) ValidateConfiguration(_ context.Context, nonSecret map[string]string, secrets map[string][]byte) error {
	if _, err := a.parseConfiguration(nonSecret); err != nil {
		return err
	}
	if len(secrets[passwordField]) == 0 || len(secrets[passwordField]) > 4096 {
		return gerror.New("smtp credential is invalid")
	}
	return nil
}

func (a *adapter) ValidateDestination(destination []byte) error {
	_, err := parseMailbox(destination)
	return err
}

func (a *adapter) Deliver(ctx context.Context, input channelcap.Delivery) (channelcap.Result, error) {
	config, err := a.parseConfiguration(input.NonSecretConfig)
	if err != nil || len(input.Subject)+len(input.Body) == 0 || len(input.Subject)+len(input.Body) > maximumMessage || hasHeaderBreak(input.Subject) {
		return smtpResult(input.TaskNo, channelcap.ResultConfigurationError, false, ""), nil
	}
	recipient, err := parseMailbox(input.Destination)
	if err != nil {
		return smtpResult(input.TaskNo, channelcap.ResultInvalidRecipient, false, ""), nil
	}
	request, err := secretcap.NewAccessRequest(input.Credential, a.purpose)
	if err != nil {
		return smtpResult(input.TaskNo, channelcap.ResultConfigurationError, false, ""), nil
	}
	result := smtpResult(input.TaskNo, channelcap.ResultUnknown, false, "")
	accessErr := a.accessor.Access(ctx, request, func(plaintext []byte) error {
		var secrets map[string][]byte
		if err := json.Unmarshal(plaintext, &secrets); err != nil {
			return err
		}
		defer clearSecrets(secrets)
		password := secrets[passwordField]
		if len(password) == 0 {
			return errors.New("smtp credential is unavailable")
		}
		err := a.send(ctx, config, password, recipient, input.Subject, input.Body, input.TaskNo)
		result = classifySMTP(input.TaskNo, err)
		return nil
	})
	if accessErr != nil {
		return smtpResult(input.TaskNo, channelcap.ResultConfigurationError, false, ""), nil
	}
	return result, nil
}

func (a *adapter) Query(_ context.Context, input channelcap.Query) (channelcap.Result, error) {
	return smtpResult(input.TaskNo, channelcap.ResultUnknown, true, input.ProviderMessageRef), nil
}

func (a *adapter) parseConfiguration(values map[string]string) (configuration, error) {
	host := strings.ToLower(strings.TrimSpace(values[hostField]))
	port := strings.TrimSpace(values[portField])
	security := strings.TrimSpace(values[securityField])
	username := strings.TrimSpace(values[usernameField])
	fromAddress, err := mail.ParseAddress(strings.TrimSpace(values[fromAddressField]))
	if err != nil || fromAddress.Name != "" || fromAddress.Address != strings.TrimSpace(values[fromAddressField]) || username == "" || hasHeaderBreak(username) {
		return configuration{}, gerror.New("smtp configuration is invalid")
	}
	if host == "" || strings.ContainsAny(host, " /\\\t\r\n") || net.ParseIP(host) == nil && !validHostname(host) {
		return configuration{}, gerror.New("smtp host is invalid")
	}
	if security != "implicit_tls" && security != "starttls" || security == "implicit_tls" && port != "465" || security == "starttls" && port != "587" {
		return configuration{}, gerror.New("smtp transport security is invalid")
	}
	endpoint := net.JoinHostPort(host, port)
	if _, ok := a.allowed[endpoint]; !ok {
		return configuration{}, gerror.New("smtp endpoint is not deployment-approved")
	}
	fromName := strings.TrimSpace(values[fromNameField])
	if hasHeaderBreak(fromName) {
		return configuration{}, gerror.New("smtp sender name is invalid")
	}
	return configuration{host: host, port: port, security: security, username: username, fromAddress: fromAddress.Address, fromName: fromName}, nil
}

func (a *adapter) send(ctx context.Context, config configuration, password []byte, recipient, subject, body, taskNo string) error {
	address := net.JoinHostPort(config.host, config.port)
	connection, err := a.dialer.DialContext(ctx, "tcp", address)
	if err != nil {
		return &smtpError{stage: "connect", err: err}
	}
	defer connection.Close()
	deadline := time.Now().Add(30 * time.Second)
	if contextDeadline, ok := ctx.Deadline(); ok && contextDeadline.Before(deadline) {
		deadline = contextDeadline
	}
	_ = connection.SetDeadline(deadline)
	tlsConfig := &tls.Config{MinVersion: tls.VersionTLS12, ServerName: config.host}
	var client *netsmtp.Client
	if config.security == "implicit_tls" {
		tlsConnection := tls.Client(connection, tlsConfig)
		if err = tlsConnection.HandshakeContext(ctx); err != nil {
			return &smtpError{stage: "tls", err: err}
		}
		client, err = netsmtp.NewClient(tlsConnection, config.host)
	} else {
		client, err = netsmtp.NewClient(connection, config.host)
		if err == nil {
			if ok, _ := client.Extension("STARTTLS"); !ok {
				_ = client.Close()
				return &smtpError{stage: "tls", err: errors.New("STARTTLS is unavailable")}
			}
			err = client.StartTLS(tlsConfig)
		}
	}
	if err != nil {
		return &smtpError{stage: "tls", err: err}
	}
	defer client.Close()
	if err = client.Auth(netsmtp.PlainAuth("", config.username, string(password), config.host)); err != nil {
		return &smtpError{stage: "auth", err: err}
	}
	if err = client.Mail(config.fromAddress); err != nil {
		return &smtpError{stage: "mail", err: err}
	}
	if err = client.Rcpt(recipient); err != nil {
		return &smtpError{stage: "recipient", err: err}
	}
	writer, err := client.Data()
	if err != nil {
		return &smtpError{stage: "data", err: err}
	}
	message := composeMessage(config, recipient, subject, body, taskNo)
	if _, err = io.Copy(writer, bytes.NewReader(message)); err != nil {
		_ = writer.Close()
		return &smtpError{stage: "body", err: err}
	}
	if err = writer.Close(); err != nil {
		return &smtpError{stage: "commit", err: err}
	}
	if err = client.Quit(); err != nil {
		// A successful DATA completion is authoritative acceptance; a later QUIT
		// failure must not turn it into an automatic resend.
		return nil
	}
	return nil
}

func composeMessage(config configuration, recipient, subject, body, taskNo string) []byte {
	from := config.fromAddress
	if config.fromName != "" {
		from = mime.QEncoding.Encode("UTF-8", config.fromName) + " <" + config.fromAddress + ">"
	}
	var output bytes.Buffer
	writer := bufio.NewWriter(&output)
	_, _ = fmt.Fprintf(writer, "From: %s\r\nTo: %s\r\nSubject: %s\r\n", from, recipient, mime.QEncoding.Encode("UTF-8", subject))
	_, _ = fmt.Fprintf(writer, "Message-ID: <%s@%s>\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\nContent-Transfer-Encoding: 8bit\r\n\r\n", taskNo, config.host)
	_, _ = writer.WriteString(strings.ReplaceAll(strings.ReplaceAll(body, "\r\n", "\n"), "\n", "\r\n"))
	_ = writer.Flush()
	return output.Bytes()
}

func classifySMTP(taskNo string, err error) channelcap.Result {
	if err == nil {
		return smtpResult(taskNo, channelcap.ResultDelivered, true, taskNo)
	}
	var failure *smtpError
	if !errors.As(err, &failure) {
		return smtpResult(taskNo, channelcap.ResultUnknown, true, "")
	}
	var protocol *textproto.Error
	if errors.As(failure.err, &protocol) {
		if failure.stage == "recipient" && protocol.Code >= 500 {
			return smtpResult(taskNo, channelcap.ResultInvalidRecipient, true, "")
		}
		if failure.stage == "auth" || failure.stage == "mail" || failure.stage == "tls" {
			return smtpResult(taskNo, channelcap.ResultConfigurationError, true, "")
		}
		if protocol.Code >= 400 && protocol.Code < 500 {
			return smtpResult(taskNo, channelcap.ResultDefinitiveNotReceivedTemporary, true, "")
		}
		if protocol.Code >= 500 {
			return smtpResult(taskNo, channelcap.ResultDefinitiveNotReceivedPermanent, true, "")
		}
	}
	if failure.stage == "connect" || failure.stage == "tls" || failure.stage == "auth" {
		return smtpResult(taskNo, channelcap.ResultConfigurationError, false, "")
	}
	return smtpResult(taskNo, channelcap.ResultUnknown, true, "")
}

func smtpResult(taskNo string, class channelcap.ResultClass, crossed bool, reference string) channelcap.Result {
	digest := sha256.Sum256([]byte("smtp-v1\x00" + taskNo + "\x00" + string(class) + "\x00" + reference))
	return channelcap.Result{Class: class, ProviderMessageRef: reference, EvidenceDigest: hex.EncodeToString(digest[:]), CrossedExternalBorder: crossed}
}

func parseMailbox(value []byte) (string, error) {
	if len(value) == 0 || len(value) > 320 || strings.ContainsAny(string(value), "\r\n") {
		return "", gerror.New("smtp recipient is invalid")
	}
	raw := string(value)
	address, err := mail.ParseAddress(raw)
	if err != nil || address.Name != "" || address.Address != raw {
		return "", gerror.New("smtp recipient is invalid")
	}
	return address.Address, nil
}

func parseAllowlist(value string) map[string]struct{} {
	result := make(map[string]struct{})
	for _, raw := range strings.Split(value, ",") {
		raw = strings.TrimSpace(raw)
		host, port, err := net.SplitHostPort(raw)
		if err != nil || host == "" || port != "465" && port != "587" {
			continue
		}
		result[net.JoinHostPort(strings.ToLower(host), port)] = struct{}{}
	}
	return result
}

func validHostname(value string) bool {
	if len(value) == 0 || len(value) > 253 || strings.HasPrefix(value, ".") || strings.HasSuffix(value, ".") {
		return false
	}
	for _, label := range strings.Split(value, ".") {
		if label == "" || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for _, character := range label {
			if (character < 'a' || character > 'z') && (character < '0' || character > '9') && character != '-' {
				return false
			}
		}
	}
	return true
}

func hasHeaderBreak(value string) bool { return strings.ContainsAny(value, "\r\n") }

func clearSecrets(values map[string][]byte) {
	for key, value := range values {
		clear(value)
		delete(values, key)
	}
}
