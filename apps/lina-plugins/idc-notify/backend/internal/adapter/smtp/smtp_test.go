// This file verifies SMTP configuration hardening, message construction, and result classification.

package smtp

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/textproto"
	"strings"
	"testing"

	"lina-plugin-idc-core/backend/cap/secretcap"
	"lina-plugin-idc-notify/backend/cap/channelcap"
)

type accessorStub struct{ payload []byte }

func (stub accessorStub) Access(_ context.Context, request secretcap.AccessRequest, consume secretcap.Consumer) error {
	if err := secretcap.ValidateAccess(request, consume); err != nil {
		return err
	}
	value := append([]byte(nil), stub.payload...)
	defer clear(value)
	return consume(value)
}

func TestValidateConfigurationRequiresApprovedTLSModeAndEndpoint(t *testing.T) {
	credential, err := json.Marshal(map[string][]byte{passwordField: []byte("smtp-password")})
	if err != nil {
		t.Fatalf("encode SMTP credential: %v", err)
	}
	adapter, err := newAdapter(accessorStub{payload: credential}, parseAllowlist("mail.example.com:465,relay.example.com:587"), &net.Dialer{})
	if err != nil {
		t.Fatalf("construct SMTP adapter: %v", err)
	}
	valid := map[string]string{
		hostField: "mail.example.com", portField: "465", securityField: "implicit_tls",
		usernameField: "mailer@example.com", fromAddressField: "mailer@example.com", fromNameField: "IDC 通知",
	}
	if err = adapter.ValidateConfiguration(t.Context(), valid, map[string][]byte{passwordField: []byte("smtp-password")}); err != nil {
		t.Fatalf("valid approved SMTP configuration was rejected: %v", err)
	}
	for name, mutate := range map[string]func(map[string]string){
		"unapproved endpoint": func(values map[string]string) { values[hostField] = "other.example.com" },
		"plaintext mode":      func(values map[string]string) { values[securityField] = "plain" },
		"wrong TLS port":      func(values map[string]string) { values[portField] = "587" },
		"header injection":    func(values map[string]string) { values[fromNameField] = "IDC\r\nBcc: attacker@example.com" },
	} {
		t.Run(name, func(t *testing.T) {
			candidate := make(map[string]string, len(valid))
			for key, value := range valid {
				candidate[key] = value
			}
			mutate(candidate)
			if err := adapter.ValidateConfiguration(t.Context(), candidate, map[string][]byte{passwordField: []byte("smtp-password")}); err == nil {
				t.Fatal("unsafe SMTP configuration was accepted")
			}
		})
	}
}

func TestComposeMessagePreventsHeaderInjectionAndUsesStableMessageID(t *testing.T) {
	config := configuration{host: "mail.example.com", fromAddress: "mailer@example.com", fromName: "IDC 通知"}
	message := string(composeMessage(config, "customer@example.com", "服务到期提醒", "第一行\n第二行", "ntask_123"))
	for _, required := range []string{
		"To: customer@example.com\r\n",
		"Message-ID: <ntask_123@mail.example.com>\r\n",
		"Content-Type: text/plain; charset=UTF-8\r\n",
		"第一行\r\n第二行",
	} {
		if !strings.Contains(message, required) {
			t.Fatalf("SMTP message is missing %q: %s", required, message)
		}
	}
	if err := (&adapter{}).ValidateDestination([]byte("customer@example.com\r\nBcc: attacker@example.com")); err == nil {
		t.Fatal("SMTP recipient header injection was accepted")
	}
}

func TestClassifySMTPDistinguishesDefinitiveAndAmbiguousFailures(t *testing.T) {
	tests := []struct {
		name    string
		err     error
		class   channelcap.ResultClass
		crossed bool
	}{
		{name: "accepted", class: channelcap.ResultDelivered, crossed: true},
		{name: "temporary provider response", err: &smtpError{stage: "data", err: &textproto.Error{Code: 451, Msg: "busy"}}, class: channelcap.ResultDefinitiveNotReceivedTemporary, crossed: true},
		{name: "invalid recipient", err: &smtpError{stage: "recipient", err: &textproto.Error{Code: 550, Msg: "unknown"}}, class: channelcap.ResultInvalidRecipient, crossed: true},
		{name: "connection failed before provider", err: &smtpError{stage: "connect", err: errors.New("offline")}, class: channelcap.ResultConfigurationError, crossed: false},
		{name: "ambiguous body failure", err: &smtpError{stage: "body", err: errors.New("closed")}, class: channelcap.ResultUnknown, crossed: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := classifySMTP("ntask_1", test.err)
			if result.Class != test.class || result.CrossedExternalBorder != test.crossed || len(result.EvidenceDigest) != 64 {
				t.Fatalf("unexpected SMTP result: %+v", result)
			}
		})
	}
}
