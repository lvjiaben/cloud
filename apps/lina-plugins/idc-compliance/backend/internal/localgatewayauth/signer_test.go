// This file verifies that the source plugin signs the exact frame accepted by
// the installation-local gateway process.

package localgatewayauth

import (
	"bytes"
	"crypto/rand"
	"net/http"
	"strconv"
	"testing"
	"time"

	"lina-platform/idc-compliance-gateway/pkg/processauth"
)

const testInstallationID = "linst_2222222222222222222222222222222222222222222222222222222222222222"

func TestSignerAndGatewayRingInteroperate(t *testing.T) {
	file, err := processauth.Generate(rand.Reader)
	if err != nil {
		t.Fatalf("generate keyring: %v", err)
	}
	signer, err := New(file, testInstallationID, 45*time.Second)
	if err != nil {
		t.Fatalf("new signer: %v", err)
	}
	t.Cleanup(signer.Close)
	now := time.Unix(1_800_000_000, 0).UTC()
	signer.now = func() time.Time { return now }
	request, err := http.NewRequest(
		http.MethodPost,
		"http://127.0.0.1:9088/internal/v1/tasks?source=plugin",
		bytes.NewReader([]byte(`{"b":2,"a":1}`)),
	)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	businessKey, err := processauth.GenerateBusinessIdempotencyKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate business key: %v", err)
	}
	if err = signer.Sign(
		request, "task.create", processauth.PayloadModeJSON,
		[]byte(`{"b":2,"a":1}`), businessKey,
	); err != nil {
		t.Fatalf("sign: %v", err)
	}
	credentials := credentialsFromHeaders(t, request)
	verifier, err := processauth.New(file)
	if err != nil {
		t.Fatalf("new verifier: %v", err)
	}
	t.Cleanup(verifier.Clear)
	verified, err := verifier.Verify(
		testInstallationID, "task.create", request.Method, request.URL.RequestURI(),
		processauth.PayloadModeJSON, []byte(`{"a":1,"b":2}`), credentials,
		now, 45*time.Second,
	)
	if err != nil || verified.InstallationID != testInstallationID {
		t.Fatalf("verified=%+v err=%v", verified, err)
	}
}

func TestInstallationReadUsesFixedScopeAndFreshBusinessKey(t *testing.T) {
	file, err := processauth.Generate(rand.Reader)
	if err != nil {
		t.Fatalf("generate keyring: %v", err)
	}
	signer, err := New(file, testInstallationID, 30*time.Second)
	if err != nil {
		t.Fatalf("new signer: %v", err)
	}
	t.Cleanup(signer.Close)
	request, err := http.NewRequest(
		http.MethodGet, "http://127.0.0.1:9088/internal/v1/installation", nil,
	)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	if err = signer.SignInstallationRead(request); err != nil {
		t.Fatalf("sign installation read: %v", err)
	}
	credentials := credentialsFromHeaders(t, request)
	if credentials.OperationScope != "installation.read" ||
		credentials.InstallationID != testInstallationID ||
		credentials.BusinessIdempotencyKey == "" {
		t.Fatalf("credentials=%+v", credentials)
	}
}

func TestSupplierSubjectWritesUseExactRouteScopes(t *testing.T) {
	file, err := processauth.Generate(rand.Reader)
	if err != nil {
		t.Fatalf("generate keyring: %v", err)
	}
	signer, err := New(file, testInstallationID, 30*time.Second)
	if err != nil {
		t.Fatalf("new signer: %v", err)
	}
	t.Cleanup(signer.Close)
	body := []byte(`{"providerCode":"tencent_essbasic"}`)
	businessKey, err := processauth.GenerateBusinessIdempotencyKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate business key: %v", err)
	}
	for _, test := range []struct {
		path  string
		scope string
	}{
		{processauth.SupplierSubjectBindingsPath, processauth.SupplierSubjectBindScope},
		{processauth.SupplierSubjectRevocationsPath, processauth.SupplierSubjectRevokeScope},
		{processauth.SupplierSubjectReopeningsPath, processauth.SupplierSubjectReopenScope},
	} {
		request, requestErr := http.NewRequest(
			http.MethodPost, "http://127.0.0.1:9088"+test.path, bytes.NewReader(body),
		)
		if requestErr != nil {
			t.Fatalf("new request: %v", requestErr)
		}
		if err = signer.SignSupplierSubjectWrite(request, body, businessKey); err != nil {
			t.Fatalf("sign %s: %v", test.path, err)
		}
		if got := credentialsFromHeaders(t, request).OperationScope; got != test.scope {
			t.Fatalf("path %s scope=%q want=%q", test.path, got, test.scope)
		}
	}
	request, err := http.NewRequest(
		http.MethodPost, "http://127.0.0.1:9088"+processauth.SupplierSubjectBindingsPath+"?unsafe=1", nil,
	)
	if err != nil {
		t.Fatalf("new query request: %v", err)
	}
	if err = signer.SignSupplierSubjectWrite(request, body, businessKey); err == nil {
		t.Fatal("supplier-subject signer accepted a query-bearing route")
	}
}

func credentialsFromHeaders(t *testing.T, request *http.Request) processauth.Credentials {
	t.Helper()
	var credentials processauth.Credentials
	credentials.Protocol = request.Header.Get(processauth.HeaderProtocol)
	credentials.InstallationID = request.Header.Get(processauth.HeaderInstallationID)
	credentials.OperationScope = request.Header.Get(processauth.HeaderOperationScope)
	credentials.KeyID = request.Header.Get(processauth.HeaderKeyID)
	credentials.Nonce = request.Header.Get(processauth.HeaderNonce)
	credentials.BusinessIdempotencyKey = request.Header.Get(processauth.HeaderBusinessIdempotency)
	credentials.ContentSHA256 = request.Header.Get(processauth.HeaderContentSHA)
	credentials.BusinessContentSHA256 = request.Header.Get(processauth.HeaderBusinessContentSHA)
	credentials.Signature = request.Header.Get(processauth.HeaderSignature)
	issued, err := strconv.ParseInt(request.Header.Get(processauth.HeaderTimestampMS), 10, 64)
	if err != nil {
		t.Fatalf("parse issued timestamp: %v", err)
	}
	expires, err := strconv.ParseInt(request.Header.Get(processauth.HeaderExpiresMS), 10, 64)
	if err != nil {
		t.Fatalf("parse expiry timestamp: %v", err)
	}
	credentials.TimestampMS = issued
	credentials.ExpiresMS = expires
	return credentials
}
