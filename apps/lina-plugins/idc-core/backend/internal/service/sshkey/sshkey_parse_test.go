// This file verifies strict public-key parsing and private-key rejection.

package sshkey

import (
	"crypto/ed25519"
	"crypto/rand"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"

	"lina-core/pkg/bizerr"
)

func TestParsePublicKeyAcceptsOnePlainAuthorizedKey(t *testing.T) {
	public, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	key, err := ssh.NewPublicKey(public)
	if err != nil {
		t.Fatal(err)
	}
	input := strings.TrimSpace(string(ssh.MarshalAuthorizedKey(key))) + " customer@example"
	parsed, err := parsePublicKey(input)
	if err != nil {
		t.Fatalf("valid public key rejected: %v", err)
	}
	if parsed.algorithm != ssh.KeyAlgoED25519 || parsed.fingerprint != ssh.FingerprintSHA256(key) ||
		parsed.canonical != strings.TrimSpace(string(ssh.MarshalAuthorizedKey(key))) ||
		strings.Contains(parsed.canonical, "customer@example") {
		t.Fatalf("unexpected canonical projection: %+v", parsed)
	}
}

func TestParsePublicKeyRejectsPrivateAndAmbiguousInputs(t *testing.T) {
	public, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	key, err := ssh.NewPublicKey(public)
	if err != nil {
		t.Fatal(err)
	}
	line := strings.TrimSpace(string(ssh.MarshalAuthorizedKey(key)))
	tests := []struct {
		name    string
		value   string
		private bool
	}{
		{name: "OpenSSH private key", value: "-----BEGIN OPENSSH PRIVATE KEY-----\nsecret", private: true},
		{name: "PEM private key", value: "-----BEGIN RSA PRIVATE KEY-----\nsecret", private: true},
		{name: "PuTTY private key", value: "PuTTY-User-Key-File-3: ssh-rsa\nEncryption: none", private: true},
		{name: "authorized options", value: `command="touch /tmp/unsafe" ` + line},
		{name: "multiple keys", value: line + "\n" + line},
		{name: "empty", value: ""},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			_, parseErr := parsePublicKey(testCase.value)
			if parseErr == nil {
				t.Fatal("unsafe SSH input accepted")
			}
			if testCase.private && !bizerr.Is(parseErr, CodePrivateKeyRejected) {
				t.Fatalf("private key returned wrong error: %v", parseErr)
			}
		})
	}
}

func TestPublicKeyProjectionNeverContainsMaterial(t *testing.T) {
	row := keyRow{KeyNo: "SSH_0123456789abcdef0123456789abcdef", Name: "Production",
		AlgorithmCode: ssh.KeyAlgoED25519, PublicKey: "ssh-ed25519 sensitive-material",
		FingerprintSHA256: "SHA256:abcdefghijklmnopqrstuv", Version: 1, CreatedAt: timeForTest()}
	item := project(row)
	if strings.Contains(strings.Join([]string{item.KeyNo, item.Name, item.Algorithm, item.Fingerprint}, " "), row.PublicKey) {
		t.Fatal("safe public-key projection exposed key material")
	}
}

func timeForTest() (value time.Time) { return time.Unix(1_787_184_000, 0).UTC() }
