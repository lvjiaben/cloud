// This file verifies loopback routing, process signatures, and safe response projection.

package localgatewayclient

import (
	"crypto/rand"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"lina-platform/idc-compliance-gateway/pkg/processauth"
	"lina-plugin-idc-compliance/backend/internal/localgatewayauth"
)

const clientTestInstallationID = "linst_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

func TestReopenSendsServerDerivedEmployeeEvidence(t *testing.T) {
	file, err := processauth.Generate(rand.Reader)
	if err != nil {
		t.Fatalf("generate keyring: %v", err)
	}
	signer, err := localgatewayauth.New(file, clientTestInstallationID, 30*time.Second)
	if err != nil {
		t.Fatalf("new signer: %v", err)
	}
	seen := false
	httpClient := &http.Client{
		Timeout: time.Second,
		Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			var body struct {
				EmployeePrincipalNo string `json:"employeePrincipalNo"`
				EvidenceSetSHA256   string `json:"evidenceSetSha256"`
			}
			decodeError := json.NewDecoder(request.Body).Decode(&body)
			seen = decodeError == nil && request.Method == http.MethodPost &&
				request.URL.RequestURI() == processauth.SupplierSubjectReopeningsPath &&
				request.Header.Get(processauth.HeaderOperationScope) == processauth.SupplierSubjectReopenScope &&
				body.EmployeePrincipalNo == "staff:42" && body.EvidenceSetSHA256 == strings.Repeat("b", 64)
			return &http.Response{
				StatusCode: http.StatusOK,
				Body: io.NopCloser(strings.NewReader(
					`{"operationNo":"lsi_1123456789abcdef0123456789abcdef","bindingNo":"lsb_1123456789abcdef0123456789abcdef","replayed":false}`,
				)),
				Header: make(http.Header),
			}, nil
		}),
	}
	client, err := New(defaultBaseURL, signer, httpClient)
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	t.Cleanup(client.Close)
	result, err := client.Reopen(
		t.Context(), "tencent_essbasic",
		"lsb_0123456789abcdef0123456789abcdef", "supplier_reauthorized",
		[]byte("reviewed-authorization-result"), "staff:42", strings.Repeat("b", 64),
		"reopen-operation-0001",
	)
	if err != nil || !seen || result.BindingNo != "lsb_1123456789abcdef0123456789abcdef" {
		t.Fatalf("seen=%v result=%+v err=%v", seen, result, err)
	}
}

func TestRevokeUsesSignedLoopbackRouteAndSafeProjection(t *testing.T) {
	file, err := processauth.Generate(rand.Reader)
	if err != nil {
		t.Fatalf("generate keyring: %v", err)
	}
	signer, err := localgatewayauth.New(file, clientTestInstallationID, 30*time.Second)
	if err != nil {
		t.Fatalf("new signer: %v", err)
	}
	seen := false
	httpClient := &http.Client{
		Timeout: time.Second,
		Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			seen = request.Method == http.MethodPost &&
				request.URL.RequestURI() == processauth.SupplierSubjectRevocationsPath &&
				request.Header.Get(processauth.HeaderOperationScope) == processauth.SupplierSubjectRevokeScope &&
				request.Header.Get(processauth.HeaderSignature) != ""
			return &http.Response{
				StatusCode: http.StatusOK,
				Body: io.NopCloser(strings.NewReader(
					`{"operationNo":"lsi_0123456789abcdef0123456789abcdef","bindingNo":"lsb_0123456789abcdef0123456789abcdef","replayed":false}`,
				)),
				Header: make(http.Header),
			}, nil
		}),
	}
	client, err := New(defaultBaseURL, signer, httpClient)
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	t.Cleanup(client.Close)
	result, err := client.Revoke(
		t.Context(), "tencent_essbasic",
		"lsb_0123456789abcdef0123456789abcdef",
		"supplier_authorization_revoked", "staff:42", strings.Repeat("a", 64),
		"revoke-operation-0001",
	)
	if err != nil || !seen || result.OperationNo != "lsi_0123456789abcdef0123456789abcdef" ||
		result.BindingNo != "lsb_0123456789abcdef0123456789abcdef" {
		t.Fatalf("seen=%v result=%+v err=%v", seen, result, err)
	}
	if client.RecognizerAvailable("tencent_essbasic") || client.RecognizerAvailable("nuonuo_invoice") {
		t.Fatal("retired contract provider or closed invoice gate is incorrect")
	}
}
