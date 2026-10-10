// This file verifies deterministic envelope vectors, tamper resistance, and error redaction.

package secret

import (
	"bytes"
	"context"
	"encoding/hex"
	"errors"
	"strings"
	"testing"
	"time"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/secretcap"
)

// TestAES256GCMKnownVector anchors the primitive to a published NIST vector.
func TestAES256GCMKnownVector(t *testing.T) {
	key := make([]byte, 32)
	nonce := make([]byte, 12)
	plaintext := make([]byte, 16)
	want := mustDecodeHex(t, "cea7403d4d606b6e074ec5d3baf39d18d0d1c8a799996bf0265b98b5d48ab919")
	got, err := sealWithNonce(key, nonce, plaintext, nil)
	if err != nil {
		t.Fatalf("seal known AES-256-GCM vector failed: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("known AES-256-GCM vector mismatch: got=%x want=%x", got, want)
	}
	opened, err := openSealed(key, nonce, got, nil)
	if err != nil || !bytes.Equal(opened, plaintext) {
		t.Fatalf("open known AES-256-GCM vector failed: plaintext=%x err=%v", opened, err)
	}
	zeroBytes(opened)
}

// TestEnvelopeRejectsTamperWrongKeyAndAAD proves both encryption layers bind
// tenant, owner, caller scope, reference, payload version, and key identity.
func TestEnvelopeRejectsTamperWrongKeyAndAAD(t *testing.T) {
	scope := Scope{TenantID: 0, OwnerPluginID: "provider-test", CallerScope: "provider.authentication"}
	secretRef := "SEC_0123456789abcdef0123456789abcdef"
	plaintext := []byte("unit-test-sensitive-token")
	active := KeyMaterial{ID: "master-v1", Key: bytes.Repeat([]byte{0x11}, dekSize)}
	randomBytes := bytes.Repeat([]byte{0x5a}, dekSize+2*gcmNonceSize)
	service := &serviceImpl{random: bytes.NewReader(randomBytes)}
	fence := requestFence{
		operation: requestOperationCreate, idempotencyDigest: digestString("unit-create"),
	}
	envelope, err := service.encryptEnvelope(active, scope, secretRef, 1, fence, plaintext)
	if err != nil {
		t.Fatalf("encrypt envelope failed: %v", err)
	}
	wrapper := keyWrapValue{
		masterKeyID: active.ID,
		nonce:       envelope.wrapNonce,
		ciphertext:  envelope.wrappedDEK,
	}
	dek, err := unwrapDEK(map[string][]byte{active.ID: active.Key}, scope, secretRef, 1, []keyWrapValue{wrapper})
	if err != nil {
		t.Fatalf("unwrap envelope DEK failed: %v", err)
	}
	opened, err := openSealed(dek, envelope.payloadNonce, envelope.ciphertext, payloadAAD(scope, secretRef, 1))
	if err != nil || !bytes.Equal(opened, plaintext) {
		t.Fatalf("open envelope failed: plaintext=%q err=%v", opened, err)
	}
	zeroBytes(opened)
	zeroBytes(dek)

	wrongTenant := scope
	wrongTenant.TenantID = 1
	wrongOwner := scope
	wrongOwner.OwnerPluginID = "provider-other"
	wrongCaller := scope
	wrongCaller.CallerScope = "provider.healthcheck"
	for _, testCase := range []struct {
		name      string
		scope     Scope
		secretRef string
		version   int
	}{
		{name: "tenant", scope: wrongTenant, secretRef: secretRef, version: 1},
		{name: "owner", scope: wrongOwner, secretRef: secretRef, version: 1},
		{name: "caller", scope: wrongCaller, secretRef: secretRef, version: 1},
		{name: "reference", scope: scope, secretRef: "SEC_1123456789abcdef0123456789abcdef", version: 1},
		{name: "version", scope: scope, secretRef: secretRef, version: 2},
	} {
		if _, err = unwrapDEK(
			map[string][]byte{active.ID: active.Key},
			testCase.scope,
			testCase.secretRef,
			testCase.version,
			[]keyWrapValue{wrapper},
		); err == nil {
			t.Fatalf("%s-substituted wrapper unexpectedly authenticated", testCase.name)
		}
	}
	wrongKeyIDWrapper := wrapper
	wrongKeyIDWrapper.masterKeyID = "master-v2"
	if _, err = unwrapDEK(
		map[string][]byte{wrongKeyIDWrapper.masterKeyID: active.Key},
		scope,
		secretRef,
		1,
		[]keyWrapValue{wrongKeyIDWrapper},
	); err == nil {
		t.Fatal("key-ID-substituted wrapper unexpectedly authenticated")
	}
	if _, err = unwrapDEK(
		map[string][]byte{active.ID: bytes.Repeat([]byte{0x22}, dekSize)},
		scope,
		secretRef,
		1,
		[]keyWrapValue{wrapper},
	); err == nil {
		t.Fatal("wrong master key unexpectedly authenticated")
	}
	tampered := append([]byte(nil), envelope.wrappedDEK...)
	tampered[len(tampered)-1] ^= 0x01
	if _, err = unwrapDEK(
		map[string][]byte{active.ID: active.Key},
		scope,
		secretRef,
		1,
		[]keyWrapValue{{masterKeyID: active.ID, nonce: envelope.wrapNonce, ciphertext: tampered}},
	); err == nil {
		t.Fatal("tampered wrapper unexpectedly authenticated")
	}

	dek, err = unwrapDEK(map[string][]byte{active.ID: active.Key}, scope, secretRef, 1, []keyWrapValue{wrapper})
	if err != nil {
		t.Fatalf("unwrap DEK for payload tamper test failed: %v", err)
	}
	tamperedPayload := append([]byte(nil), envelope.ciphertext...)
	tamperedPayload[0] ^= 0x80
	if _, err = openSealed(dek, envelope.payloadNonce, tamperedPayload, payloadAAD(scope, secretRef, 1)); err == nil {
		t.Fatal("tampered payload unexpectedly authenticated")
	}
	for _, testCase := range []struct {
		name      string
		scope     Scope
		secretRef string
		version   int
	}{
		{name: "tenant", scope: wrongTenant, secretRef: secretRef, version: 1},
		{name: "owner", scope: wrongOwner, secretRef: secretRef, version: 1},
		{name: "caller", scope: wrongCaller, secretRef: secretRef, version: 1},
		{name: "reference", scope: scope, secretRef: "SEC_1123456789abcdef0123456789abcdef", version: 1},
		{name: "version", scope: scope, secretRef: secretRef, version: 2},
	} {
		if _, err = openSealed(
			dek,
			envelope.payloadNonce,
			envelope.ciphertext,
			payloadAAD(testCase.scope, testCase.secretRef, testCase.version),
		); err == nil {
			t.Fatalf("%s-substituted payload unexpectedly authenticated", testCase.name)
		}
	}
	zeroBytes(dek)
}

// TestFingerprintBindsCompleteReplayContext prevents future envelope changes
// from authenticating only plaintext while omitting operation or write fences.
func TestFingerprintBindsCompleteReplayContext(t *testing.T) {
	key := bytes.Repeat([]byte{0x31}, dekSize)
	scope := Scope{TenantID: 0, OwnerPluginID: "provider-test", CallerScope: "provider.authentication"}
	secretRef := "SEC_0123456789abcdef0123456789abcdef"
	expectedAt := databaseTime(time.Unix(1_700_000_000, 123_456_000))
	base := requestFence{
		operation: requestOperationUpdate, idempotencyDigest: digestString("fingerprint-base"),
		parentVersion: 1, expectedUpdatedAt: expectedAt,
	}
	want, err := fingerprint(key, scope, secretRef, 2, base, []byte("same-sensitive-value"))
	if err != nil {
		t.Fatalf("create base replay fingerprint failed: %v", err)
	}
	testCases := []struct {
		name      string
		scope     Scope
		secretRef string
		version   int
		fence     requestFence
		plaintext []byte
	}{
		{name: "operation", scope: scope, secretRef: secretRef, version: 2, fence: requestFence{operation: requestOperationCreate, idempotencyDigest: base.idempotencyDigest}, plaintext: []byte("same-sensitive-value")},
		{name: "idempotency-digest", scope: scope, secretRef: secretRef, version: 2, fence: requestFence{operation: base.operation, idempotencyDigest: digestString("fingerprint-other"), parentVersion: 1, expectedUpdatedAt: expectedAt}, plaintext: []byte("same-sensitive-value")},
		{name: "parent-version", scope: scope, secretRef: secretRef, version: 2, fence: requestFence{operation: base.operation, idempotencyDigest: base.idempotencyDigest, parentVersion: 2, expectedUpdatedAt: expectedAt}, plaintext: []byte("same-sensitive-value")},
		{name: "expected-updated-at", scope: scope, secretRef: secretRef, version: 2, fence: requestFence{operation: base.operation, idempotencyDigest: base.idempotencyDigest, parentVersion: 1, expectedUpdatedAt: expectedAt.Add(time.Microsecond)}, plaintext: []byte("same-sensitive-value")},
		{name: "scope", scope: Scope{TenantID: 0, OwnerPluginID: scope.OwnerPluginID, CallerScope: "provider.backup"}, secretRef: secretRef, version: 2, fence: base, plaintext: []byte("same-sensitive-value")},
		{name: "reference", scope: scope, secretRef: "SEC_1123456789abcdef0123456789abcdef", version: 2, fence: base, plaintext: []byte("same-sensitive-value")},
		{name: "version", scope: scope, secretRef: secretRef, version: 3, fence: base, plaintext: []byte("same-sensitive-value")},
		{name: "plaintext", scope: scope, secretRef: secretRef, version: 2, fence: base, plaintext: []byte("different-sensitive-value")},
	}
	for _, testCase := range testCases {
		got, fingerprintErr := fingerprint(
			key,
			testCase.scope,
			testCase.secretRef,
			testCase.version,
			testCase.fence,
			testCase.plaintext,
		)
		if fingerprintErr != nil {
			t.Fatalf("create %s replay fingerprint failed: %v", testCase.name, fingerprintErr)
		}
		if constantTimeStringEqual(got, want) {
			t.Fatalf("%s was not bound into replay fingerprint", testCase.name)
		}
	}
}

// TestProviderAndAuthorizationErrorsDoNotLeak proves untrusted dependency
// messages are replaced rather than wrapped by the public boundary.
func TestProviderAndAuthorizationErrorsDoNotLeak(t *testing.T) {
	const sensitive = "sensitive-provider-error-token"
	service, err := newService(
		Config{},
		failingKeyProvider{err: errors.New(sensitive)},
		failingAuthorizer{err: errors.New(sensitive)},
		time.Now,
		bytes.NewReader(make([]byte, 128)),
	)
	if err != nil {
		t.Fatalf("construct redaction test service failed: %v", err)
	}
	scope := Scope{TenantID: 0, OwnerPluginID: "provider-test", CallerScope: "provider.authentication"}
	if err = service.authorize(context.Background(), scope, ActionCreate, secretcap.Handle{}, ""); !bizerr.Is(err, CodeAccessDenied) || strings.Contains(err.Error(), sensitive) {
		t.Fatalf("authorization error leaked dependency text: %v", err)
	}
	if _, err = service.activeKey(context.Background()); !bizerr.Is(err, CodeUnavailable) || strings.Contains(err.Error(), sensitive) {
		t.Fatalf("key-provider error leaked dependency text: %v", err)
	}
}

func mustDecodeHex(t *testing.T, value string) []byte {
	t.Helper()
	decoded, err := hex.DecodeString(value)
	if err != nil {
		t.Fatalf("decode test vector failed: %v", err)
	}
	return decoded
}

type failingKeyProvider struct {
	err error
}

func (p failingKeyProvider) Active(context.Context) (KeyMaterial, error) {
	return KeyMaterial{}, p.err
}

func (p failingKeyProvider) Resolve(context.Context, []string) (map[string][]byte, error) {
	return nil, p.err
}

type failingAuthorizer struct {
	err error
}

func (a failingAuthorizer) Authorize(context.Context, AuthorizationRequest) error {
	return a.err
}

// TestPublicSecretErrorDropsSensitiveCauses keeps internal SQL and cryptographic
// material out of the public service boundary while retaining local stable codes.
func TestPublicSecretErrorDropsSensitiveCauses(t *testing.T) {
	const canary = "sensitive-sql-wrapped-dek-nonce"
	raw := errors.New(canary)
	for _, code := range []*bizerr.Code{CodeInvalid, CodeAccessDenied, CodeNotConfigured,
		CodeIdempotencyConflict, CodeVersionConflict, CodeConsumerFailed, CodeUnavailable} {
		result := publicSecretError(bizerr.WrapCode(raw, code))
		if !bizerr.Is(result, code) || strings.Contains(result.Error(), canary) || errors.Is(result, raw) {
			t.Fatalf("unsafe secret business error: %v", result)
		}
	}
	result := publicSecretError(raw)
	if !bizerr.Is(result, CodeUnavailable) || strings.Contains(result.Error(), canary) || errors.Is(result, raw) {
		t.Fatalf("unsafe raw secret error: %v", result)
	}
	if publicSecretError(nil) != nil {
		t.Fatal("nil error became failure")
	}
}
