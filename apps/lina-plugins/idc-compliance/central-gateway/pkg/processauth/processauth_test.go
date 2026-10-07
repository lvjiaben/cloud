// This file verifies the complete installation-local process-authentication
// frame, canonicalization rules, key lifecycle, and persistence-safe digests.

package processauth

import (
	"bytes"
	"errors"
	"strings"
	"testing"
	"time"
)

const testInstallationID = "linst_1111111111111111111111111111111111111111111111111111111111111111"

func TestGenerateRotateRevokeAndSign(t *testing.T) {
	file := generateTestFile(t, 1)
	oldID := file.ActiveKeyID
	file, err := Rotate(file, repeatedReader(2))
	if err != nil || file.ActiveKeyID == oldID {
		t.Fatalf("rotate file=%+v err=%v", file, err)
	}
	ring, err := New(file)
	if err != nil {
		t.Fatalf("new rotated ring: %v", err)
	}
	t.Cleanup(ring.Clear)
	now := time.Unix(1_800_000_000, 0).UTC()
	businessKey := generateTestBusinessKey(t, 3)
	credentials, err := ring.Sign(
		testInstallationID, "installation.read", "GET", "/internal/v1/installation",
		PayloadModeEmpty, nil, businessKey, now, MaximumAuthenticationWindow, repeatedReader(4),
	)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	verified, err := ring.Verify(
		testInstallationID, "installation.read", "GET", "/internal/v1/installation",
		PayloadModeEmpty, nil, credentials, now, MaximumAuthenticationWindow,
	)
	if err != nil || verified.InstallationID != testInstallationID ||
		verified.OperationScope != "installation.read" || verified.PayloadMode != PayloadModeEmpty {
		t.Fatalf("verified=%+v err=%v", verified, err)
	}

	file, err = Revoke(file, oldID)
	if err != nil {
		t.Fatalf("revoke previous key: %v", err)
	}
	for _, key := range file.Keys {
		if key.ID == oldID && (key.State != KeyStateRevoked || key.Secret != "") {
			t.Fatalf("revoked key retained secret: %+v", key)
		}
	}
}

func TestJSONCanonicalizationAndBusinessRecoverySurviveRotation(t *testing.T) {
	file := generateTestFile(t, 5)
	ring, err := New(file)
	if err != nil {
		t.Fatalf("new ring: %v", err)
	}
	now := time.Unix(1_800_000_000, 0).UTC()
	businessKey := generateTestBusinessKey(t, 6)
	first, err := ring.Sign(
		testInstallationID, "contract.create", "POST", "/internal/v1/contracts?a=1&b=%E4%B8%AD",
		PayloadModeJSON, []byte(`{ "z": 2.0, "a": [true, null] }`), businessKey,
		now, 45*time.Second, repeatedReader(7),
	)
	if err != nil {
		t.Fatalf("sign first: %v", err)
	}
	firstVerified, err := ring.Verify(
		testInstallationID, "contract.create", "POST", "/internal/v1/contracts?a=1&b=%E4%B8%AD",
		PayloadModeJSON, []byte(`{"a":[true,null],"z":2}`), first, now, 45*time.Second,
	)
	if err != nil {
		t.Fatalf("verify canonical equivalent: %v", err)
	}
	ring.Clear()
	file, err = Rotate(file, repeatedReader(8))
	if err != nil {
		t.Fatalf("rotate: %v", err)
	}
	ring, err = New(file)
	if err != nil {
		t.Fatalf("new rotated ring: %v", err)
	}
	t.Cleanup(ring.Clear)
	second, err := ring.Sign(
		testInstallationID, "contract.create", "POST", "/internal/v1/contracts?a=1&b=%E4%B8%AD",
		PayloadModeJSON, []byte(`{"a":[true,null],"z":2}`), businessKey,
		now.Add(time.Second), 45*time.Second, repeatedReader(9),
	)
	if err != nil {
		t.Fatalf("sign rotated: %v", err)
	}
	secondVerified, err := ring.Verify(
		testInstallationID, "contract.create", "POST", "/internal/v1/contracts?a=1&b=%E4%B8%AD",
		PayloadModeJSON, []byte(`{"z":2,"a":[true,null]}`), second,
		now.Add(time.Second), 45*time.Second,
	)
	if err != nil {
		t.Fatalf("verify rotated: %v", err)
	}
	if firstVerified.IdempotencyHMAC != secondVerified.IdempotencyHMAC ||
		firstVerified.BusinessContentSHA256 != secondVerified.BusinessContentSHA256 ||
		firstVerified.FrameSHA256 == secondVerified.FrameSHA256 {
		t.Fatalf("unexpected recovery digests first=%+v second=%+v", firstVerified, secondVerified)
	}
}

func TestVerifyRejectsTamperingTimeAndScope(t *testing.T) {
	file := generateTestFile(t, 10)
	ring, err := New(file)
	if err != nil {
		t.Fatalf("new ring: %v", err)
	}
	t.Cleanup(ring.Clear)
	now := time.Unix(1_800_000_000, 0).UTC()
	credentials, err := ring.Sign(
		testInstallationID, "resource.update", "PUT", "/internal/v1/resources/r1",
		PayloadModeJSON, []byte(`{"value":1}`), generateTestBusinessKey(t, 11),
		now, MaximumAuthenticationWindow, repeatedReader(12),
	)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	tests := []struct {
		name          string
		installation  string
		scope         string
		body          []byte
		databaseNow   time.Time
		maximumWindow time.Duration
	}{
		{name: "body", installation: testInstallationID, scope: "resource.update", body: []byte(`{"value":2}`), databaseNow: now, maximumWindow: MaximumAuthenticationWindow},
		{name: "scope", installation: testInstallationID, scope: "resource.delete", body: []byte(`{"value":1}`), databaseNow: now, maximumWindow: MaximumAuthenticationWindow},
		{name: "expired", installation: testInstallationID, scope: "resource.update", body: []byte(`{"value":1}`), databaseNow: now.Add(61 * time.Second), maximumWindow: MaximumAuthenticationWindow},
		{name: "oversized-window", installation: testInstallationID, scope: "resource.update", body: []byte(`{"value":1}`), databaseNow: now, maximumWindow: 61 * time.Second},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, verifyErr := ring.Verify(
				test.installation, test.scope, "PUT", "/internal/v1/resources/r1",
				PayloadModeJSON, test.body, credentials, test.databaseNow, test.maximumWindow,
			); verifyErr == nil {
				t.Fatal("tampered or expired frame was accepted")
			}
		})
	}

	future, err := ring.Sign(
		testInstallationID, "resource.update", "PUT", "/internal/v1/resources/r1",
		PayloadModeJSON, []byte(`{"value":1}`), generateTestBusinessKey(t, 13),
		now.Add(31*time.Second), 20*time.Second, repeatedReader(14),
	)
	if err != nil {
		t.Fatalf("sign future frame: %v", err)
	}
	if _, err = ring.Verify(
		testInstallationID, "resource.update", "PUT", "/internal/v1/resources/r1",
		PayloadModeJSON, []byte(`{"value":1}`), future, now, MaximumAuthenticationWindow,
	); !errors.Is(err, ErrRejected) {
		t.Fatalf("future frame err=%v", err)
	}
}

func TestCanonicalizationRejectsAliasesDuplicatesAndWrongMode(t *testing.T) {
	file := generateTestFile(t, 15)
	ring, err := New(file)
	if err != nil {
		t.Fatalf("new ring: %v", err)
	}
	t.Cleanup(ring.Clear)
	now := time.Unix(1_800_000_000, 0).UTC()
	businessKey := generateTestBusinessKey(t, 16)
	tests := []struct {
		name   string
		target string
		mode   PayloadMode
		body   []byte
	}{
		{name: "duplicate-query", target: "/internal/v1/items?a=1&a=2", mode: PayloadModeEmpty},
		{name: "unsorted-query", target: "/internal/v1/items?b=2&a=1", mode: PayloadModeEmpty},
		{name: "lowercase-escape", target: "/internal/v1/items?a=%e4%b8%ad", mode: PayloadModeEmpty},
		{name: "plus-alias", target: "/internal/v1/items?a=hello+world", mode: PayloadModeEmpty},
		{name: "path-escape", target: "/internal/v1/%69tems", mode: PayloadModeEmpty},
		{name: "path-dot", target: "/internal/v1/../items", mode: PayloadModeEmpty},
		{name: "duplicate-json", target: "/internal/v1/items", mode: PayloadModeJSON, body: []byte(`{"a":1,"a":2}`)},
		{name: "empty-mode-body", target: "/internal/v1/items", mode: PayloadModeEmpty, body: []byte(`{}`)},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, signErr := ring.Sign(
				testInstallationID, "items.read", "GET", test.target, test.mode, test.body,
				businessKey, now, 30*time.Second, repeatedReader(17),
			); !errors.Is(signErr, ErrInvalid) {
				t.Fatalf("invalid canonical input err=%v", signErr)
			}
		})
	}
}

func TestInstallationIdentifiersAreCanonicalAndRandomSized(t *testing.T) {
	installationID, err := GenerateInstallationID(repeatedReader(18))
	if err != nil {
		t.Fatalf("generate installation id: %v", err)
	}
	if !strings.HasPrefix(installationID, "linst_") || len(installationID) != 70 {
		t.Fatalf("installation id=%q", installationID)
	}
	parsed, err := ParseInstallationID([]byte(installationID + "\n"))
	if err != nil || parsed != installationID {
		t.Fatalf("parsed=%q err=%v", parsed, err)
	}
	if _, err = ParseInstallationID([]byte("linst_short")); !errors.Is(err, ErrInvalid) {
		t.Fatalf("short installation id err=%v", err)
	}
}

func TestParseRejectsUnknownFieldsAndNonCanonicalSecret(t *testing.T) {
	for _, content := range [][]byte{
		[]byte(`{"version":"v1","activeKeyId":"lgk_1111111111111111","keys":[],"other":true}`),
		[]byte(`{"version":"v1","activeKeyId":"lgk_1111111111111111","businessIdempotencySecret":"bad","keys":[{"id":"lgk_1111111111111111","state":"active","secret":"not-a-key"}]}`),
	} {
		if _, err := Parse(content); !errors.Is(err, ErrInvalid) {
			t.Fatalf("content accepted: %s err=%v", content, err)
		}
	}
}

func TestMarshalRoundTrip(t *testing.T) {
	file := generateTestFile(t, 19)
	content, err := Marshal(file)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	parsed, err := Parse(content)
	if err != nil || parsed.ActiveKeyID != file.ActiveKeyID || len(parsed.Keys) != 1 ||
		parsed.BusinessIdempotencySecret != file.BusinessIdempotencySecret {
		t.Fatalf("parsed=%+v err=%v", parsed, err)
	}
}

func generateTestFile(t *testing.T, value byte) File {
	t.Helper()
	file, err := Generate(repeatedReader(value))
	if err != nil {
		t.Fatalf("generate keyring: %v", err)
	}
	return file
}

func generateTestBusinessKey(t *testing.T, value byte) string {
	t.Helper()
	key, err := GenerateBusinessIdempotencyKey(repeatedReader(value))
	if err != nil {
		t.Fatalf("generate business key: %v", err)
	}
	return key
}

func repeatedReader(value byte) *bytes.Reader {
	return bytes.NewReader(bytes.Repeat([]byte{value}, 512))
}
