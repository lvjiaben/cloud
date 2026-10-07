package tencentess

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"testing"
	"time"

	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common"

	"lina-platform/idc-compliance-gateway/internal/suppliercallback"
)

const (
	testCallbackKey   = "TencentEssEncryptTestKey12345678"
	testCallbackToken = "buyer-installation-callback-token"
	testCallbackMsgID = "yDwFdUUckpswdrniUuzcbXw8N43W6Kcz"
)

type fakeCallbackResolver struct {
	reference []byte
	result    suppliercallback.ResolvedReference
	err       error
	calls     int
}

func (f *fakeCallbackResolver) ResolveExternalReference(
	_ context.Context,
	_ string,
	reference []byte,
) (suppliercallback.ResolvedReference, error) {
	f.calls++
	f.reference = append(f.reference[:0], reference...)
	return f.result, f.err
}

func TestCallbackVerifierAuthenticatesDecryptsAndResolvesLocalRoute(t *testing.T) {
	now := time.Unix(1_800_000_000, 0).UTC()
	resolver := &fakeCallbackResolver{result: suppliercallback.ResolvedReference{
		RouteNo: "lwr_0123456789abcdef0123456789abcdef", OperationScope: OperationFlowCreate,
	}}
	verifier := testCallbackVerifier(t, resolver, now)
	body, signature := signedEncryptedCallback(t, now, "ALL", "organization-id")
	result, err := verifier.Verify(t.Context(), suppliercallback.RawCallback{
		Provider: suppliercallback.ProviderTencentESSBasic,
		Body:     body, Signature: signature,
	})
	if err != nil || result.RouteNo != resolver.result.RouteNo ||
		result.OperationScope != OperationFlowCreate || result.Status != suppliercallback.StatusSucceeded ||
		string(result.ExternalEventID) != testCallbackMsgID ||
		!result.SupplierObservedAt.Equal(now) || len(result.EvidenceFrame) == 0 ||
		string(resolver.reference) != testFlowID || resolver.calls != 1 {
		t.Fatalf("unexpected verified callback: result=%+v resolver=%+v err=%v", result, resolver, err)
	}
	clear(result.ExternalEventID)
	clear(result.EvidenceFrame)
}

func TestCallbackVerifierRejectsTamperingBeforeRouteLookup(t *testing.T) {
	now := time.Unix(1_800_000_000, 0).UTC()
	resolver := &fakeCallbackResolver{result: suppliercallback.ResolvedReference{
		RouteNo: "lwr_0123456789abcdef0123456789abcdef", OperationScope: OperationFlowCreate,
	}}
	verifier := testCallbackVerifier(t, resolver, now)
	body, signature := signedEncryptedCallback(t, now, "ALL", "organization-id")
	body[len(body)-2] ^= 1
	if _, err := verifier.Verify(t.Context(), suppliercallback.RawCallback{
		Provider: suppliercallback.ProviderTencentESSBasic, Body: body, Signature: signature,
	}); err == nil || resolver.calls != 0 {
		t.Fatalf("tampered callback reached route lookup: calls=%d err=%v", resolver.calls, err)
	}
}

func TestCallbackVerifierRejectsWrongInstallationSubject(t *testing.T) {
	now := time.Unix(1_800_000_000, 0).UTC()
	resolver := &fakeCallbackResolver{result: suppliercallback.ResolvedReference{
		RouteNo: "lwr_0123456789abcdef0123456789abcdef", OperationScope: OperationFlowCreate,
	}}
	verifier := testCallbackVerifier(t, resolver, now)
	body, signature := signedEncryptedCallback(t, now, "PART", "another-organization")
	if _, err := verifier.Verify(t.Context(), suppliercallback.RawCallback{
		Provider: suppliercallback.ProviderTencentESSBasic, Body: body, Signature: signature,
	}); err == nil || resolver.calls != 0 {
		t.Fatalf("wrong subject callback reached route lookup: calls=%d err=%v", resolver.calls, err)
	}
}

func TestNormalizeCallbackStatusPreservesOfficialTerminalClasses(t *testing.T) {
	tests := map[string]suppliercallback.Status{
		"INIT": suppliercallback.StatusRunning, "PART": suppliercallback.StatusRunning,
		"WILLEXPIRE": suppliercallback.StatusRunning, "ALL": suppliercallback.StatusSucceeded,
		"RELIEVED": suppliercallback.StatusSucceeded, "REJECT": suppliercallback.StatusFailed,
		"CANCEL": suppliercallback.StatusFailed, "DEADLINE": suppliercallback.StatusFailed,
		"INVALID": suppliercallback.StatusFailed, "EXCEPTION": suppliercallback.StatusFailed,
	}
	for input, expected := range tests {
		actual, ok := normalizeCallbackStatus(input)
		if !ok || actual != expected {
			t.Fatalf("status %s normalized to %s valid=%v", input, actual, ok)
		}
	}
	if _, ok := normalizeCallbackStatus("UNKNOWN"); ok {
		t.Fatal("unknown Tencent callback status was accepted")
	}
}

func testCallbackVerifier(
	t *testing.T,
	resolver callbackReferenceResolver,
	now time.Time,
) *CallbackVerifier {
	t.Helper()
	config := Config{
		APIVersion: APIVersion, SecretID: "secret-id", SecretKey: "secret-key", AppID: "app-id",
		CallbackVerifyToken: testCallbackToken, CallbackURLKey: testCallbackKey,
		Capabilities: []Capability{CapabilityCallback},
	}
	credential := common.NewCredential(config.SecretID, config.SecretKey)
	client, err := newClient(config, inertAPI(), credential, &fakeSubjectSource{payload: []byte(
		`{"proxyOrganizationOpenId":"organization-id","proxyOperatorOpenId":"operator-id"}`,
	)}, nil)
	if err != nil {
		t.Fatalf("construct callback client: %v", err)
	}
	t.Cleanup(client.Close)
	verifier, err := NewCallbackVerifier(config, client, resolver)
	if err != nil {
		t.Fatalf("construct callback verifier: %v", err)
	}
	verifier.now = func() time.Time { return now }
	t.Cleanup(verifier.Close)
	return verifier
}

func signedEncryptedCallback(
	t *testing.T,
	now time.Time,
	status string,
	organization string,
) ([]byte, []byte) {
	t.Helper()
	releaseFlowID := ""
	if status == "RELIEVED" {
		releaseFlowID = "ReLeAsEfLoW0123456789AbCdEfGhIjK"
	}
	plaintext, err := json.Marshal(callbackEnvelope{
		MessageID: testCallbackMsgID, MessageType: callbackMessageType,
		MessageVersion: callbackMessageVersion,
		MessageData: mustJSON(t, flowStatusCallback{
			ApplicationID: "app-id", ProxyOrganizationOpenID: organization,
			ProxyOperatorOpenID: "operator-id", FlowID: testFlowID, FlowStatus: status,
			CreateOn: now.Add(-time.Hour).Unix(), Deadline: now.Add(24 * time.Hour).Unix(),
			OccurTime: now.Unix(), ReleaseFlowID: releaseFlowID,
		}),
	})
	if err != nil {
		t.Fatalf("marshal callback: %v", err)
	}
	key := []byte(testCallbackKey)
	padding := aes.BlockSize - len(plaintext)%aes.BlockSize
	padded := append(append([]byte(nil), plaintext...), make([]byte, padding)...)
	for index := len(padded) - padding; index < len(padded); index++ {
		padded[index] = byte(padding)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		t.Fatalf("construct callback cipher: %v", err)
	}
	ciphertext := make([]byte, len(padded))
	cipher.NewCBCEncrypter(block, key[:aes.BlockSize]).CryptBlocks(ciphertext, padded)
	body, err := json.Marshal(encryptedCallback{Encrypt: base64.StdEncoding.EncodeToString(ciphertext)})
	if err != nil {
		t.Fatalf("marshal encrypted callback: %v", err)
	}
	mac := hmac.New(sha256.New, []byte(testCallbackToken))
	_, _ = mac.Write(body)
	signature := []byte("sha256=" + hex.EncodeToString(mac.Sum(nil)))
	clear(plaintext)
	clear(padded)
	clear(ciphertext)
	return body, signature
}

func mustJSON(t *testing.T, value any) json.RawMessage {
	t.Helper()
	payload, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal callback data: %v", err)
	}
	return payload
}
