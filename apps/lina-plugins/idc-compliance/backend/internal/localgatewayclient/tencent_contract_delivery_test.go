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

func TestTencentContractDeliveriesUseSeparateSignedScopes(t *testing.T) {
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
		if request.Header.Get(processauth.HeaderSignature) == "" {
			t.Fatalf("unexpected unsigned delivery request path=%s", request.URL.RequestURI())
		}
		body := `{"items":[{"deliveryNo":"ldv_0123456789abcdef0123456789abcdef","routeNo":"lwr_0123456789abcdef0123456789abcdef","routeVersion":2,"status":"running","providerStatus":"PART","reconciliationStatus":"none","sourceKind":"callback","decisionCode":"applied","supplierObservedUnix":1800000000,"observedUnix":1800000001}]}`
		expectedScope := processauth.TencentContractDeliveryReadScope
		if request.URL.Path == processauth.TencentContractDeliveryPagesPath {
			if request.Method != http.MethodGet || request.URL.RawQuery != "limit=50" {
				t.Fatalf("unexpected delivery page request=%s %s", request.Method, request.URL.RequestURI())
			}
		} else if request.URL.RequestURI() == processauth.TencentContractDeliveryAcknowledgementsPath {
			if request.Method != http.MethodPost {
				t.Fatalf("unexpected delivery acknowledgement method=%s", request.Method)
			}
			expectedScope = processauth.TencentContractDeliveryAcknowledgeScope
			var input processauth.TencentContractDeliveryAcknowledgementRequest
			if decodeErr := json.NewDecoder(request.Body).Decode(&input); decodeErr != nil ||
				len(input.DeliveryNos) != 1 || input.DeliveryNos[0] != "ldv_0123456789abcdef0123456789abcdef" {
				t.Fatalf("unexpected delivery acknowledgement input=%+v err=%v", input, decodeErr)
			}
			body = `{"acknowledged":1}`
		} else {
			t.Fatalf("unexpected delivery path=%s", request.URL.RequestURI())
		}
		if request.Header.Get(processauth.HeaderOperationScope) != expectedScope {
			t.Fatalf("delivery scope=%q want=%q", request.Header.Get(processauth.HeaderOperationScope), expectedScope)
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
	})}
	client, err := New(defaultBaseURL, signer, httpClient)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(client.Close)
	items, err := client.ListTencentContractDeliveries(t.Context(), 50)
	if err != nil || len(items) != 1 || items[0].ProviderStatus != "PART" {
		t.Fatalf("delivery items=%+v err=%v", items, err)
	}
	if err = client.AcknowledgeTencentContractDeliveries(
		t.Context(), []string{items[0].DeliveryNo}, "contract-delivery-batch-1",
	); err != nil || calls != 2 {
		t.Fatalf("acknowledge deliveries calls=%d err=%v", calls, err)
	}
}
