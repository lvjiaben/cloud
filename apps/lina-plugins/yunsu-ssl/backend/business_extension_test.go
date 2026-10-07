package backend

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"errors"
	"net"
	"strings"
	"testing"

	"lina-core/pkg/plugin/capability/capmodel"
	"lina-core/pkg/plugin/pluginbridge/protocol"
	"lina-plugin-idc-core/backend/cap/productfulfillmentcap"
	"lina-plugin-idc-core/backend/cap/supplierinterfacecap"
)

type supplierInterfaceStub struct {
	contexts []context.Context
	requests []supplierinterfacecap.OutboundHTTPRequest
	results  []*supplierinterfacecap.OutboundHTTPResponse
	err      error
}

func (*supplierInterfaceStub) GetConfiguration(context.Context, string) (*supplierinterfacecap.ConfigurationView, error) {
	return nil, errors.New("not used")
}

func (*supplierInterfaceStub) ListConfigurationFiles(context.Context, string, string) (*supplierinterfacecap.ConfigurationFileList, error) {
	return nil, errors.New("not used")
}

func (*supplierInterfaceStub) ReadConfigurationFile(context.Context, string, supplierinterfacecap.ConfigurationFileReadRequest) (*supplierinterfacecap.ConfigurationFileChunk, error) {
	return nil, errors.New("not used")
}

func (*supplierInterfaceStub) AccessSecret(context.Context, string, string, func([]byte) error) error {
	return errors.New("dynamic secret access must not be used")
}

func (s *supplierInterfaceStub) ExecuteOutboundHTTP(ctx context.Context, _ string, request supplierinterfacecap.OutboundHTTPRequest) (*supplierinterfacecap.OutboundHTTPResponse, error) {
	s.contexts = append(s.contexts, ctx)
	s.requests = append(s.requests, request)
	if s.err != nil {
		return nil, s.err
	}
	if len(s.results) == 0 {
		return nil, errors.New("missing result")
	}
	result := s.results[0]
	s.results = s.results[1:]
	return result, nil
}

func (*supplierInterfaceStub) ExecuteOutboundSession(context.Context, string, supplierinterfacecap.OutboundSessionRequest) (*supplierinterfacecap.OutboundSessionResponse, error) {
	return nil, errors.New("not used")
}

func TestCandidateDiscoveryUsesSecretFieldBindingsWithoutSecretValues(t *testing.T) {
	interfaces := &supplierInterfaceStub{results: []*supplierinterfacecap.OutboundHTTPResponse{{StatusCode: 200, Data: json.RawMessage(`{"success":true,"data":[{"product_id":"ssl-dv-1","name":"Domain SSL","brand":"Yunsu","type":"DV","file_validation_only":0,"dns_validation_only":0}]}`)}}}
	response, err := NewGuestHandler(interfaces)(extensionEnvelope(t, candidateMethod, capmodel.BusinessExtensionInvocationPurposeNewBusiness,
		json.RawMessage(`{"schemaVersion":"v1","currentSelections":[],"requests":[{"fieldKey":"plan_id","parentValues":[],"selectedValues":[]}]}`)))
	if err != nil || response.StatusCode != 200 || len(interfaces.requests) != 1 {
		t.Fatalf("response=%#v calls=%d err=%v", response, len(interfaces.requests), err)
	}
	request := interfaces.requests[0]
	if request.Path != "/product/list" || request.SecretHeaders["X-API-Key"] != "api_key" || request.SecretHeaders["X-API-Secret"] != "api_secret" {
		t.Fatalf("unexpected confidential request: %#v", request)
	}
	encoded, _ := json.Marshal(request)
	if strings.Contains(string(encoded), "plain-secret") {
		t.Fatal("secret value crossed the guest boundary")
	}
	var bridge capmodel.BusinessExtensionBridgeResponse
	var candidates candidateResponse
	if json.Unmarshal(response.Body, &bridge) != nil || json.Unmarshal(bridge.Payload, &candidates) != nil || len(candidates.Items) != 1 || len(candidates.Items[0].Candidates) != 1 || !strings.Contains(string(bridge.Payload), "Yunsu Domain SSL (DV)") {
		t.Fatalf("unexpected candidate projection: %s", response.Body)
	}
	product := productConfiguration{PlanID: candidates.Items[0].Candidates[0].Value, CertificateScope: "ordinary"}
	if hydrateProductConfiguration(&product) != nil || product.SupplierPlanID != "ssl-dv-1" || !product.DNSValidationSupported || !product.FileValidationSupported || product.CertificateValidationLevel != "dv" {
		t.Fatalf("supplier product facts were not frozen: %#v", product)
	}
}

func TestProvisionBuildsYunsuOrderAfterExistingBusinessAdmission(t *testing.T) {
	interfaces := &supplierInterfaceStub{results: []*supplierinterfacecap.OutboundHTTPResponse{{StatusCode: 200, Data: json.RawMessage(`{"success":true,"order_id":"YS-10001"}`)}}}
	response, err := NewGuestHandler(interfaces)(extensionEnvelope(t, provisionMethod, capmodel.BusinessExtensionInvocationPurposeExistingBusiness, validRequest(t, "")))
	if err != nil || response.StatusCode != 200 || len(interfaces.requests) != 1 {
		t.Fatalf("response=%#v calls=%d err=%v", response, len(interfaces.requests), err)
	}
	request := interfaces.requests[0]
	if request.Path != "/payment/balance" || request.EndpointField != "endpoint" {
		t.Fatalf("unexpected order request: %#v", request)
	}
	var body map[string]any
	if json.Unmarshal(request.Body, &body) != nil || body["product_id"] != "ssl-dv-1" || body["domain"] != "example.com,www.example.com" {
		t.Fatalf("unexpected order payload: %s", request.Body)
	}
	result := decodeResult(t, response)
	if result.Status != productfulfillmentcap.StatusAccepted || result.ExternalReference != "YS-10001" {
		t.Fatalf("unexpected fulfillment result: %#v", result)
	}
}

func TestProvisionIsRejectedForUnpaidNewBusinessInvocation(t *testing.T) {
	interfaces := &supplierInterfaceStub{}
	response, err := NewGuestHandler(interfaces)(extensionEnvelope(t, provisionMethod, capmodel.BusinessExtensionInvocationPurposeNewBusiness, validRequest(t, "")))
	if err != nil || response.StatusCode != 400 || len(interfaces.requests) != 0 {
		t.Fatalf("unpaid request crossed supplier boundary: response=%#v calls=%d err=%v", response, len(interfaces.requests), err)
	}
}

func TestManagedServiceDetailUsesExistingBusinessRuntimeOnly(t *testing.T) {
	interfaces := &supplierInterfaceStub{results: []*supplierinterfacecap.OutboundHTTPResponse{{
		StatusCode: 200,
		Data:       json.RawMessage(`{"success":true,"data":{"status":"awaiting_validation","dcv_info":{"method":"dns","record_type":"CNAME","host":"_ssl.example.com","value":"proof.example.net"}}}`),
	}}}
	response, err := NewGuestHandler(interfaces)(extensionEnvelope(t, serviceDetailMethod, capmodel.BusinessExtensionInvocationPurposeExistingBusiness, serviceActionPayload(t, "detail", "")))
	if err != nil || response.StatusCode != 200 || len(interfaces.requests) != 1 || interfaces.requests[0].Path != "/order/detail" {
		t.Fatalf("response=%#v requests=%#v err=%v", response, interfaces.requests, err)
	}
	if !strings.Contains(string(response.Body), "proof.example.net") {
		t.Fatalf("supplier detail was not projected: %s", response.Body)
	}
}

func TestManagedServiceMutationRejectsNewBusinessPurpose(t *testing.T) {
	interfaces := &supplierInterfaceStub{}
	response, err := NewGuestHandler(interfaces)(extensionEnvelope(t, serviceCancelMethod, capmodel.BusinessExtensionInvocationPurposeNewBusiness, serviceActionPayload(t, "cancel", "")))
	if err != nil || response.StatusCode != 400 || len(interfaces.requests) != 0 {
		t.Fatalf("new-business action crossed supplier boundary: response=%#v requests=%#v err=%v", response, interfaces.requests, err)
	}
}

func TestQueryMapsIssuedCertificateToSucceeded(t *testing.T) {
	interfaces := &supplierInterfaceStub{results: []*supplierinterfacecap.OutboundHTTPResponse{{StatusCode: 200, Data: json.RawMessage(`{"success":true,"data":{"status":"issued","expires_at":"2027-08-28"}}`)}}}
	response, err := NewGuestHandler(interfaces)(extensionEnvelope(t, queryMethod, capmodel.BusinessExtensionInvocationPurposeExistingBusiness, validRequest(t, "YS-10001")))
	if err != nil || response.StatusCode != 200 || len(interfaces.requests) != 1 || interfaces.requests[0].Path != "/order/sync" {
		t.Fatalf("response=%#v requests=%#v err=%v", response, interfaces.requests, err)
	}
	result := decodeResult(t, response)
	if result.Status != productfulfillmentcap.StatusSucceeded || result.ExternalReference != "YS-10001" || !strings.Contains(string(result.Evidence), "issued") {
		t.Fatalf("unexpected query result: %#v", result)
	}
}

func TestQueryKeepsUnknownSupplierStatusUnknown(t *testing.T) {
	interfaces := &supplierInterfaceStub{results: []*supplierinterfacecap.OutboundHTTPResponse{{StatusCode: 200, Data: json.RawMessage(`{"success":true,"data":{"status":"supplier_new_state"}}`)}}}
	response, err := NewGuestHandler(interfaces)(extensionEnvelope(t, queryMethod, capmodel.BusinessExtensionInvocationPurposeExistingBusiness, validRequest(t, "YS-10001")))
	if err != nil || response.StatusCode != 200 {
		t.Fatalf("response=%#v err=%v", response, err)
	}
	if result := decodeResult(t, response); result.Status != productfulfillmentcap.StatusResultUnknown {
		t.Fatalf("unknown supplier status was guessed: %#v", result)
	}
}

func TestIPAddressCannotUseOrdinaryDomainProduct(t *testing.T) {
	interfaces := &supplierInterfaceStub{}
	payload := validRequest(t, "")
	var request productfulfillmentcap.Request
	if json.Unmarshal(payload, &request) != nil {
		t.Fatal("decode request")
	}
	request.ProductConfiguration = testProductConfiguration(t, "ordinary", "dv", true, true)
	request.CustomerConfiguration = certificateCustomerConfiguration(t, []string{"192.0.2.10"}, "file", false)
	payload, _ = json.Marshal(request)
	response, err := NewGuestHandler(interfaces)(extensionEnvelope(t, provisionMethod, capmodel.BusinessExtensionInvocationPurposeExistingBusiness, payload))
	if err != nil || response.StatusCode != 500 || len(interfaces.requests) != 0 {
		t.Fatalf("invalid domain type crossed supplier boundary: response=%#v calls=%d err=%v", response, len(interfaces.requests), err)
	}
}

func extensionEnvelope(t *testing.T, method string, purpose capmodel.BusinessExtensionInvocationPurpose, payload json.RawMessage) *protocol.BridgeRequestEnvelopeV1 {
	t.Helper()
	invocation := &capmodel.BusinessExtensionBridgeRequest{
		Version: capmodel.BusinessExtensionInvocationProtocolV1, OwnerPluginID: ownerPluginID, ProviderPluginID: pluginID,
		Point: capmodel.BusinessExtensionProductFulfillment, ContractVersion: productfulfillmentcap.ContractVersion,
		Method: method, ReleaseVersion: "v0.3.2", ReleaseDigest: strings.Repeat("a", 64), DeclarationDigest: strings.Repeat("b", 64),
		Purpose: purpose, RuntimeContext: &capmodel.BusinessExtensionRuntimeContext{ContextID: "src_0123456789abcdef0123456789abcdef"}, Payload: payload,
	}
	body, err := json.Marshal(invocation)
	if err != nil {
		t.Fatal(err)
	}
	return &protocol.BridgeRequestEnvelopeV1{PluginID: pluginID, Route: &protocol.RouteMatchSnapshotV1{RequestType: capmodel.BusinessExtensionBridgeRequestType}, Request: &protocol.HTTPRequestSnapshotV1{Body: body}}
}

func validRequest(t *testing.T, externalReference string) json.RawMessage {
	t.Helper()
	request := &productfulfillmentcap.Request{
		Version: productfulfillmentcap.RequestVersion, IntentNo: "intent-10001", IdempotencyKey: "idem-10001", OrderNo: "order-10001", OrderItemNo: "item-10001",
		UnitSequence: 1, Quantity: 1, ProductNo: "product-10001", ProductVersionNo: "version-10001", ConfigurationRevision: "configuration-1",
		ProductConfiguration:       testProductConfiguration(t, "multi_domain", "dv", true, true),
		ProductConfigurationDigest: strings.Repeat("d", 64),
		CustomerConfiguration:      certificateCustomerConfiguration(t, []string{"example.com", "www.example.com"}, "dns", false),
		CustomerConfigurationID:    "configuration-snapshot-1", PreviousExternalReference: externalReference,
	}
	payload, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	return payload
}

func testProductConfiguration(t *testing.T, scope, validationLevel string, dns, file bool) json.RawMessage {
	t.Helper()
	token, err := encodeFrozenPlan(frozenCertificatePlan{
		Version: "v1", SupplierPlanID: "ssl-" + validationLevel + "-1", CertificateValidationLevel: validationLevel,
		DNSValidationSupported: dns, FileValidationSupported: file,
	})
	if err != nil {
		t.Fatal(err)
	}
	payload, err := json.Marshal(productConfiguration{PlanID: token, CertificateScope: scope})
	if err != nil {
		t.Fatal(err)
	}
	return payload
}

func certificateCustomerConfiguration(t *testing.T, domains []string, validation string, organization bool) json.RawMessage {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.CertificateRequest{Subject: pkix.Name{CommonName: domains[0]}}
	for _, domain := range domains {
		if address := net.ParseIP(domain); address != nil {
			template.IPAddresses = append(template.IPAddresses, address)
		} else {
			template.DNSNames = append(template.DNSNames, domain)
		}
	}
	der, err := x509.CreateCertificateRequest(rand.Reader, template, key)
	if err != nil {
		t.Fatal(err)
	}
	value := map[string]any{
		"primary_domain": domains[0], "validation_method": validation,
		"email": "ops@example.com", "phone": "13800138000",
		"csr": string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE REQUEST", Bytes: der})),
	}
	if len(domains) > 1 {
		value["additional_domains"] = strings.Join(domains[1:], ",")
	}
	if organization {
		value["contact_first_name"], value["contact_last_name"], value["contact_title"] = "Lin", "Operator", "Manager"
		value["organization_name"], value["organization_phone"] = "Example Limited", "13800138000"
		value["organization_address"], value["organization_city"], value["organization_state"] = "1 Example Road", "Shenzhen", "Guangdong"
		value["organization_country"], value["organization_postcode"], value["organization_registration_number"] = "CN", "518000", "REG-10001"
	}
	result, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func decodeResult(t *testing.T, response *protocol.BridgeResponseEnvelopeV1) productfulfillmentcap.Result {
	t.Helper()
	var bridge capmodel.BusinessExtensionBridgeResponse
	var result productfulfillmentcap.Result
	if json.Unmarshal(response.Body, &bridge) != nil || json.Unmarshal(bridge.Payload, &result) != nil {
		t.Fatalf("invalid bridge response: %s", response.Body)
	}
	return result
}
