package finance

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"

	"lina-plugin-idc-resource/backend/cap/resourcecap"
)

const testDownstreamToken = "supplier-callback-token-value"

type nativeFinanceSupplier struct {
	mu           sync.Mutex
	server       *httptest.Server
	invoiceID    string
	hostID       string
	addCount     int
	settleCount  int
	creditCount  int
	settleStatus int
	settleBody   string
	creditStatus int
	creditBody   string
}

func newNativeFinanceSupplier(t *testing.T) *nativeFinanceSupplier {
	t.Helper()
	supplier := &nativeFinanceSupplier{}
	supplier.server = httptest.NewTLSServer(http.HandlerFunc(supplier.serveHTTP))
	t.Cleanup(supplier.server.Close)
	return supplier
}

func (s *nativeFinanceSupplier) serveHTTP(response http.ResponseWriter, request *http.Request) {
	response.Header().Set("Content-Type", "application/json")
	if request.URL.Path == "/zjmf_api_login" {
		_ = request.ParseForm()
		if request.Form.Get("username") != "merchant-user" || request.Form.Get("password") != "merchant-password" {
			response.WriteHeader(http.StatusUnauthorized)
			_, _ = response.Write([]byte(`{"status":401}`))
			return
		}
		_, _ = response.Write([]byte(`{"status":200,"jwt":"supplier-session"}`))
		return
	}
	if request.Header.Get("Authorization") != "Bearer supplier-session" {
		response.WriteHeader(http.StatusUnauthorized)
		_, _ = response.Write([]byte(`{"status":401}`))
		return
	}
	_ = request.ParseForm()
	s.mu.Lock()
	defer s.mu.Unlock()
	switch request.URL.Path {
	case "/cart/clear":
		if request.Form.Get("downstream_token") != testDownstreamToken || request.Form.Get("downstream_id") != "1001" || request.Form.Get("downstream_url") != "https://platform.example.test/api/plugins/idc-resource/provider-callback" {
			_, _ = response.Write([]byte(`{"status":422}`))
			return
		}
		if s.hostID != "" {
			_, _ = response.Write([]byte(`{"status":400,"hostid":91,"domainstatus":"Active"}`))
			return
		}
		if s.invoiceID != "" {
			_, _ = response.Write([]byte(`{"status":200,"invoiceid":71,"hostid":91}`))
			return
		}
		_, _ = response.Write([]byte(`{"status":200}`))
	case "/user_info":
		_, _ = response.Write([]byte(`{"status":200,"user":{"currency":1}}`))
	case "/cart/add_to_shop":
		if request.Form.Get("pid") != "42" || request.Form.Get("configoption[11]") != "102" || request.Form.Get("password") != "" {
			_, _ = response.Write([]byte(`{"status":422}`))
			return
		}
		s.addCount++
		_, _ = response.Write([]byte(`{"status":200}`))
	case "/cart/settle":
		if request.Form.Get("downstream_token") != testDownstreamToken {
			_, _ = response.Write([]byte(`{"status":422}`))
			return
		}
		s.settleCount++
		s.invoiceID = "71"
		if s.settleStatus != 0 {
			response.WriteHeader(s.settleStatus)
		}
		if s.settleBody != "" {
			_, _ = response.Write([]byte(s.settleBody))
		} else {
			_, _ = response.Write([]byte(`{"status":200,"data":{"invoiceid":71}}`))
		}
	case "/apply_credit":
		if request.Form.Get("downstream_token") != testDownstreamToken || request.Form.Get("invoiceid") != "71" {
			_, _ = response.Write([]byte(`{"status":422}`))
			return
		}
		s.creditCount++
		s.hostID = "91"
		if s.creditStatus != 0 {
			response.WriteHeader(s.creditStatus)
		}
		if s.creditBody != "" {
			_, _ = response.Write([]byte(s.creditBody))
		} else {
			_, _ = response.Write([]byte(`{"status":1001,"data":{"hostid":[91]}}`))
		}
	default:
		response.WriteHeader(http.StatusNotFound)
	}
}

type tlsConfidentialExecutor struct {
	client          *http.Client
	baseURL         string
	secrets         map[string]string
	guestPayloads   [][]byte
	failAfterSettle bool
	failPath        string
}

func (e *tlsConfidentialExecutor) Execute(_ string, _ uint8, payload json.RawMessage) (json.RawMessage, error) {
	e.guestPayloads = append(e.guestPayloads, append([]byte(nil), payload...))
	var instruction resourcecap.ConfidentialHTTPRequest
	if json.Unmarshal(payload, &instruction) != nil {
		return nil, errors.New("invalid confidential instruction")
	}
	form := url.Values{}
	for key, value := range instruction.Form {
		form.Set(key, value)
	}
	if instruction.Kind == resourcecap.ConfidentialHTTPKindAuthenticate {
		for upstreamField, secretName := range instruction.CredentialFields {
			form.Set(upstreamField, e.secrets[secretName])
		}
	} else {
		for upstreamField, secretName := range instruction.SecretFormFields {
			form.Set(upstreamField, e.secrets[secretName])
		}
	}
	request, err := http.NewRequest(instruction.Method, e.baseURL+instruction.Path, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if instruction.UseAuthentication {
		request.Header.Set("Authorization", "Bearer supplier-session")
	}
	if instruction.Path == e.failPath {
		return nil, errors.New("supplier transport unavailable before settlement")
	}
	response, err := e.client.Do(request)
	if err != nil {
		return nil, err
	}
	body, readErr := io.ReadAll(response.Body)
	_ = response.Body.Close()
	if readErr != nil {
		return nil, readErr
	}
	if instruction.Path == "/cart/settle" && e.failAfterSettle {
		return nil, errors.New("settlement response lost after supplier accepted request")
	}
	result := resourcecap.ConfidentialHTTPResponse{StatusCode: response.StatusCode, Data: body}
	if instruction.Kind == resourcecap.ConfidentialHTTPKindAuthenticate && response.StatusCode == http.StatusOK {
		result.Authenticated = true
	}
	return json.Marshal(result)
}

func TestProvisionPreSettlementTransportLossFailsSafely(t *testing.T) {
	supplier := newNativeFinanceSupplier(t)
	executor := &tlsConfidentialExecutor{client: supplier.server.Client(), baseURL: supplier.server.URL, failPath: "/cart/add_to_shop", secrets: map[string]string{
		"username": "merchant-user", "password": "merchant-password", "downstream_token": testDownstreamToken,
	}}
	payload, _ := json.Marshal(provisionDynamicRequest())
	resultJSON, err := ExecuteDynamic(executor, "ctx-safe-failure", resourcecap.DynamicProviderMethodExecute, payload)
	if err != nil {
		t.Fatal(err)
	}
	var result resourcecap.OperationResult
	if json.Unmarshal(resultJSON, &result) != nil || result.Status != resourcecap.TaskStatusFailed || result.Failure == nil || result.Failure.ResultUnknown {
		t.Fatalf("pre-settlement failure became ambiguous: %s", resultJSON)
	}
	if supplier.addCount != 0 || supplier.settleCount != 0 || supplier.creditCount != 0 {
		t.Fatalf("pre-settlement failure reached a supplier side effect: add=%d settle=%d credit=%d", supplier.addCount, supplier.settleCount, supplier.creditCount)
	}
}

func TestProvisionRejectsEverySecretSlotExceptDownstreamToken(t *testing.T) {
	executor := &recordingExecutor{results: []resourcecap.ConfidentialHTTPResponse{{StatusCode: 200, Authenticated: true}}}
	request := provisionDynamicRequest()
	request.ConfidentialInputNames = []string{"password", "downstream_token"}
	payload, _ := json.Marshal(request)
	if _, err := ExecuteDynamic(executor, "ctx-invalid-secret", resourcecap.DynamicProviderMethodExecute, payload); err == nil {
		t.Fatal("provision accepted a guest-visible password secret slot")
	}
}

func TestProvisionUsesConfidentialTLSProxyAndReplayRecoversExistingHost(t *testing.T) {
	supplier := newNativeFinanceSupplier(t)
	executor := &tlsConfidentialExecutor{client: supplier.server.Client(), baseURL: supplier.server.URL, secrets: map[string]string{
		"username": "merchant-user", "password": "merchant-password", "downstream_token": testDownstreamToken,
	}}
	request := provisionDynamicRequest()
	payload, _ := json.Marshal(request)
	for attempt := 0; attempt < 2; attempt++ {
		resultJSON, err := ExecuteDynamic(executor, "ctx-provision", resourcecap.DynamicProviderMethodExecute, payload)
		if err != nil {
			t.Fatal(err)
		}
		var result resourcecap.OperationResult
		if json.Unmarshal(resultJSON, &result) != nil || result.Status != resourcecap.TaskStatusSucceeded || result.ExternalResourceRef != "91" {
			t.Fatalf("attempt %d did not recover the same host: %s", attempt+1, resultJSON)
		}
	}
	if supplier.addCount != 1 || supplier.settleCount != 1 || supplier.creditCount != 1 {
		t.Fatalf("replay repeated supplier side effects: add=%d settle=%d credit=%d", supplier.addCount, supplier.settleCount, supplier.creditCount)
	}
	for _, payload := range executor.guestPayloads {
		if strings.Contains(string(payload), testDownstreamToken) || strings.Contains(string(payload), "merchant-password") || strings.Contains(string(payload), "merchant-user") {
			t.Fatalf("secret entered guest-visible memory: %s", payload)
		}
	}
}

func TestProvisionSettlementTransportLossIsUnknownAndNeverRetried(t *testing.T) {
	supplier := newNativeFinanceSupplier(t)
	executor := &tlsConfidentialExecutor{client: supplier.server.Client(), baseURL: supplier.server.URL, failAfterSettle: true, secrets: map[string]string{
		"username": "merchant-user", "password": "merchant-password", "downstream_token": testDownstreamToken,
	}}
	payload, _ := json.Marshal(provisionDynamicRequest())
	resultJSON, err := ExecuteDynamic(executor, "ctx-unknown", resourcecap.DynamicProviderMethodExecute, payload)
	if err != nil {
		t.Fatal(err)
	}
	var result resourcecap.OperationResult
	if json.Unmarshal(resultJSON, &result) != nil || result.Status != resourcecap.TaskStatusUnknown || result.Failure == nil || !result.Failure.ResultUnknown || result.Failure.Retryable {
		t.Fatalf("post-settlement ambiguity was not held for manual reconciliation: %s", resultJSON)
	}
	if supplier.addCount != 1 || supplier.settleCount != 1 || supplier.creditCount != 0 {
		t.Fatalf("ambiguous settlement was automatically repeated or paid: add=%d settle=%d credit=%d", supplier.addCount, supplier.settleCount, supplier.creditCount)
	}
}

func TestProvisionSettlementServerErrorIsUnknown(t *testing.T) {
	supplier := newNativeFinanceSupplier(t)
	supplier.settleStatus = http.StatusInternalServerError
	supplier.settleBody = `{"status":500}`
	result := executeProvisionAgainstSupplier(t, supplier)
	assertUnknownProvisionResult(t, result)
	if supplier.settleCount != 1 || supplier.creditCount != 0 {
		t.Fatalf("server-error settlement was retried or paid: settle=%d credit=%d", supplier.settleCount, supplier.creditCount)
	}
}

func TestProvisionSettlementSuccessWithoutReferenceIsUnknown(t *testing.T) {
	supplier := newNativeFinanceSupplier(t)
	supplier.settleBody = `{"status":200,"data":{}}`
	result := executeProvisionAgainstSupplier(t, supplier)
	assertUnknownProvisionResult(t, result)
	if supplier.settleCount != 1 || supplier.creditCount != 0 {
		t.Fatalf("reference-free settlement was retried or paid: settle=%d credit=%d", supplier.settleCount, supplier.creditCount)
	}
}

func TestProvisionCreditServerErrorIsUnknown(t *testing.T) {
	supplier := newNativeFinanceSupplier(t)
	supplier.creditStatus = http.StatusInternalServerError
	supplier.creditBody = `{"status":500}`
	result := executeProvisionAgainstSupplier(t, supplier)
	assertUnknownProvisionResult(t, result)
	if supplier.settleCount != 1 || supplier.creditCount != 1 {
		t.Fatalf("credit server error changed submission count: settle=%d credit=%d", supplier.settleCount, supplier.creditCount)
	}
}

func executeProvisionAgainstSupplier(t *testing.T, supplier *nativeFinanceSupplier) resourcecap.OperationResult {
	t.Helper()
	executor := &tlsConfidentialExecutor{client: supplier.server.Client(), baseURL: supplier.server.URL, secrets: map[string]string{
		"username": "merchant-user", "password": "merchant-password", "downstream_token": testDownstreamToken,
	}}
	payload, _ := json.Marshal(provisionDynamicRequest())
	resultJSON, err := ExecuteDynamic(executor, "ctx-post-settlement", resourcecap.DynamicProviderMethodExecute, payload)
	if err != nil {
		t.Fatal(err)
	}
	var result resourcecap.OperationResult
	if json.Unmarshal(resultJSON, &result) != nil {
		t.Fatalf("decode operation result: %s", resultJSON)
	}
	return result
}

func assertUnknownProvisionResult(t *testing.T, result resourcecap.OperationResult) {
	t.Helper()
	if result.Status != resourcecap.TaskStatusUnknown || result.Failure == nil || !result.Failure.ResultUnknown || result.Failure.Retryable {
		t.Fatalf("post-settlement ambiguity was not held for manual reconciliation: %#v", result)
	}
}

func provisionDynamicRequest() resourcecap.DynamicOperationRequest {
	return resourcecap.DynamicOperationRequest{
		RequestID: "request-1001", Capability: resourcecap.CapabilityProvision,
		Parameters:             json.RawMessage(`{"pid":"42","billingcycle":"monthly","host":"server-1","qty":1,"configoption":{"11":"102"}}`),
		ConfidentialInputNames: []string{"downstream_token"},
		ProviderCorrelation:    &resourcecap.ProviderCorrelation{DownstreamID: 1001, DownstreamURL: "https://platform.example.test/api/plugins/idc-resource/provider-callback"},
	}
}

// Native upstream CartController returns both invoiceid and hostid for unpaid
// downstream orders; the host must not bypass apply_credit settlement.
func TestProvisionUnpaidHostReferenceStillAppliesCredit(t *testing.T) {
	supplier := newNativeFinanceSupplier(t)
	supplier.settleBody = `{"status":200,"data":{"invoiceid":71,"hostid":[91]}}`
	result := executeProvisionAgainstSupplier(t, supplier)
	if result.Status != resourcecap.TaskStatusSucceeded || result.ExternalResourceRef != "91" || supplier.creditCount != 1 {
		t.Fatalf("unpaid native order bypassed settlement: status=%s host=%s credits=%d", result.Status, result.ExternalResourceRef, supplier.creditCount)
	}
}

func TestProvisionUnpaidHostReferenceWithRejectedCreditNeverSucceeds(t *testing.T) {
	supplier := newNativeFinanceSupplier(t)
	supplier.settleBody = `{"status":200,"data":{"invoiceid":71,"hostid":[91]}}`
	supplier.creditBody = `{"status":400,"msg":"insufficient credit"}`
	result := executeProvisionAgainstSupplier(t, supplier)
	if result.Status != resourcecap.TaskStatusFailed || supplier.creditCount != 1 {
		t.Fatalf("unpaid native order became provisioned: status=%s credits=%d", result.Status, supplier.creditCount)
	}
}

func TestProvisionPartialCreditResponseRemainsUnknown(t *testing.T) {
	supplier := newNativeFinanceSupplier(t)
	supplier.settleBody = `{"status":200,"data":{"invoiceid":71,"hostid":[91]}}`
	supplier.creditBody = `{"status":200,"data":{"invoiceid":71,"hostid":[91]}}`
	result := executeProvisionAgainstSupplier(t, supplier)
	assertUnknownProvisionResult(t, result)
	if supplier.creditCount != 1 {
		t.Fatal("credit was retried or skipped")
	}
}
