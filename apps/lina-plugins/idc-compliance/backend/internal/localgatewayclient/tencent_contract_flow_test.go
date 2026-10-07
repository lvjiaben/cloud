// This file verifies exact signed local-gateway contract flow and termination requests.

package localgatewayclient

import (
	"crypto/rand"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"lina-platform/idc-compliance-gateway/pkg/processauth"
	"lina-plugin-idc-compliance/backend/internal/localgatewayauth"
)

func TestTencentContractLinkAndRecheckUseReadOnlySignedRoutes(t *testing.T) {
	file, err := processauth.Generate(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := localgatewayauth.New(file, clientTestInstallationID, 30*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	calls := 0
	httpClient := &http.Client{Timeout: time.Second, Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		calls++
		body := `{"routeNo":"lwr_0123456789abcdef0123456789abcdef","status":"succeeded","providerStatus":"ALL","reconciliationStatus":"none","routeVersion":3,"supplierObservedUnix":1800000000,"decisionCode":"applied"}`
		expectedScope := processauth.TencentContractFlowRecheckScope
		if request.URL.RequestURI() == processauth.TencentContractSigningLinksPath {
			expectedScope = processauth.TencentContractSigningLinkCreateScope
			body = `{"url":"https://tencent.example.test/sign","expiresUnix":1800001800}`
		}
		if request.Method != http.MethodPost || request.Header.Get(processauth.HeaderOperationScope) != expectedScope ||
			request.Header.Get(processauth.HeaderSignature) == "" {
			t.Fatalf("unexpected signed request path=%s scope=%s", request.URL.RequestURI(), request.Header.Get(processauth.HeaderOperationScope))
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
	})}
	client, err := New(defaultBaseURL, signer, httpClient)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(client.Close)
	link, err := client.CreateTencentContractSigningLink(t.Context(), processauth.TencentContractSigningLinkRequest{
		RouteNo:  "lwr_0123456789abcdef0123456789abcdef",
		Approver: processauth.TencentContractApprover{SequenceNo: 1, RecipientRef: "recipient", PartyCode: "customer", SubjectTypeCode: "individual", Name: "张三", Mobile: "13800138000"},
		JumpURL:  "https://customer.example.test/account/contracts?signing_return=1",
	})
	if err != nil || link.URL == "" {
		t.Fatalf("link=%+v err=%v", link, err)
	}
	state, err := client.RecheckTencentContractFlow(t.Context(), processauth.TencentContractFlowRecheckRequest{RouteNo: "lwr_0123456789abcdef0123456789abcdef"})
	if err != nil || state.Status != "succeeded" || calls != 2 {
		t.Fatalf("state=%+v calls=%d err=%v", state, calls, err)
	}
}

func TestTencentContractTerminationUsesIndependentSignedWriteRoute(t *testing.T) {
	file, err := processauth.Generate(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := localgatewayauth.New(file, clientTestInstallationID, 30*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	httpClient := &http.Client{Timeout: time.Second, Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.Method != http.MethodPost ||
			request.URL.RequestURI() != processauth.TencentContractTerminationsPath ||
			request.Header.Get(processauth.HeaderOperationScope) != processauth.TencentContractTerminationCreateScope ||
			request.Header.Get(processauth.HeaderSignature) == "" {
			t.Fatalf("unexpected termination request path=%s scope=%s",
				request.URL.RequestURI(), request.Header.Get(processauth.HeaderOperationScope))
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(
			`{"routeNo":"lwr_1123456789abcdef0123456789abcdef","status":"signing","recovered":false}`,
		)), Header: make(http.Header)}, nil
	})}
	client, err := New(defaultBaseURL, signer, httpClient)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(client.Close)
	result, err := client.CreateTencentContractTermination(t.Context(), processauth.TencentContractTerminationRequest{
		ContractNo:      "CTR_0123456789ABCDEF",
		OriginalRouteNo: "lwr_0123456789abcdef0123456789abcdef",
		Reason:          "双方协商一致解除", DeadlineUnix: time.Now().UTC().Add(7 * 24 * time.Hour).Unix(),
	}, "contract-termination-test-0001")
	if err != nil || result.RouteNo != "lwr_1123456789abcdef0123456789abcdef" {
		t.Fatalf("termination result=%+v err=%v", result, err)
	}
}
