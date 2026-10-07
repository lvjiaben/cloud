// This file verifies signed Tencent onboarding requests and secret-free responses.

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

func TestTencentOnboardingStatusUsesExactSignedReadRoute(t *testing.T) {
	seen := false
	client := onboardingTestClient(t, func(request *http.Request) string {
		if request.Method == http.MethodGet &&
			request.URL.RequestURI() == processauth.TencentOnboardingPath &&
			request.Header.Get(processauth.HeaderOperationScope) == processauth.TencentOnboardingReadScope &&
			request.Header.Get(processauth.HeaderSignature) != "" {
			seen = true
		}
		return "{\"state\":\"ready\",\"organizationReady\":false,\"operatorReady\":false,\"capabilities\":[]}"
	})
	result, err := client.TencentOnboardingStatus(t.Context())
	if err != nil || !seen || result.State != "ready" || result.ConsoleURL != "" {
		t.Fatalf("seen=%v result=%+v err=%v", seen, result, err)
	}
}

func TestTencentOnboardingLinkSendsOnlyLegalNameAndReturnsOfficialURL(t *testing.T) {
	var body map[string]any
	seen := false
	client := onboardingTestClient(t, func(request *http.Request) string {
		decodeErr := json.NewDecoder(request.Body).Decode(&body)
		if decodeErr == nil && request.Method == http.MethodPost &&
			request.URL.RequestURI() == processauth.TencentOnboardingLinksPath &&
			request.Header.Get(processauth.HeaderOperationScope) == processauth.TencentOnboardingLinkScope &&
			request.Header.Get(processauth.HeaderSignature) != "" {
			seen = true
		}
		return "{\"state\":\"enterprise_pending\",\"consoleUrl\":\"https://qian.tencent.com/one-use\",\"organizationReady\":false,\"operatorReady\":false,\"capabilities\":[]}"
	})
	result, err := client.CreateTencentOnboardingLink(
		t.Context(), "示例 IDC 有限公司", "onboarding-request-0001",
	)
	if err != nil || !seen || result.ConsoleURL != "https://qian.tencent.com/one-use" ||
		len(body) != 1 || body["legalName"] != "示例 IDC 有限公司" {
		t.Fatalf("seen=%v body=%+v result=%+v err=%v", seen, body, result, err)
	}
}

func TestTencentOnboardingRecheckUsesExactScopeAndSuppressesOfficialURL(t *testing.T) {
	seen := false
	client := onboardingTestClient(t, func(request *http.Request) string {
		if request.Method == http.MethodPost &&
			request.URL.RequestURI() == processauth.TencentOnboardingRechecksPath &&
			request.Header.Get(processauth.HeaderOperationScope) == processauth.TencentOnboardingRecheckScope &&
			request.Header.Get(processauth.HeaderSignature) != "" {
			seen = true
		}
		return `{"state":"active","consoleUrl":"https://qian.tencent.com/unused","organizationReady":true,"operatorReady":true,"bindingNo":"lsb_0123456789abcdef0123456789abcdef","capabilities":[]}`
	})
	result, err := client.RecheckTencentOnboarding(
		t.Context(), "示例 IDC 有限公司", "recheck-request-0001",
	)
	if err != nil || !seen || result.State != "active" || result.ConsoleURL != "" {
		t.Fatalf("seen=%v result=%+v err=%v", seen, result, err)
	}
}

func onboardingTestClient(
	t *testing.T,
	responseBody func(*http.Request) string,
) *Client {
	t.Helper()
	file, err := processauth.Generate(rand.Reader)
	if err != nil {
		t.Fatalf("generate keyring: %v", err)
	}
	signer, err := localgatewayauth.New(file, clientTestInstallationID, 30*time.Second)
	if err != nil {
		t.Fatalf("new signer: %v", err)
	}
	httpClient := &http.Client{
		Timeout: time.Second,
		Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(responseBody(request))),
				Header:     make(http.Header),
			}, nil
		}),
	}
	client, err := New(defaultBaseURL, signer, httpClient)
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	t.Cleanup(client.Close)
	return client
}
