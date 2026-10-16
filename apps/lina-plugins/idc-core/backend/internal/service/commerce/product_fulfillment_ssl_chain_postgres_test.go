package commerce

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"net/http"
	"strings"
	"sync"
	"testing"

	"lina-core/pkg/plugin/capability/capmodel"
	"lina-core/pkg/plugin/capability/plugincap"
	"lina-core/pkg/plugin/pluginbridge/protocol"
	"lina-core/pkg/plugin/pluginhost"
	"lina-plugin-idc-core/backend/cap/supplierinterfacecap"
	"lina-plugin-idc-core/backend/internal/service/configurationcontract"
	productfulfillmentservice "lina-plugin-idc-core/backend/internal/service/productfulfillment"
	yunsussl "lina-plugin-yunsu-ssl/backend"
)

func TestPaidSSLCertificateCrossesYunsuGuestAndCreatesManagedEntitlementPostgreSQL(t *testing.T) {
	prepared := prepareGenericWorkerScenarioWithFrozenConfiguration(t, true, true, 1, freezeYunsuSSLConfiguration(t))
	supplier := &sslChainSupplier{}
	extensions := &sslChainExtensions{handler: yunsussl.NewGuestHandler(supplier)}
	contexts := &fulfillmentWorkerContexts{}
	sealer, err := productfulfillmentservice.NewProviderEvidenceSealer(
		"test-key-v1", []byte("0123456789abcdef0123456789abcdef"),
	)
	if err != nil {
		t.Fatal(err)
	}
	worker, err := productfulfillmentservice.NewWorker(
		prepared.checkout.database, extensions, contexts, sealer, fulfillmentSystemReasons(t), nil, nil, nil,
	)
	if err != nil {
		t.Fatal(err)
	}

	first, err := worker.Run(prepared.checkout.ctx, 1)
	if err != nil || first.Selected != 1 || first.Scheduled != 1 || supplier.requestCount() != 0 {
		t.Fatalf("SSL query-first batch=%+v supplier_calls=%d err=%v", first, supplier.requestCount(), err)
	}
	advanceFulfillmentTaskClock(t, prepared.checkout, prepared.order.OrderNo)
	second, err := worker.Run(prepared.checkout.ctx, 1)
	if err != nil || second.Selected != 1 || second.Scheduled != 1 || supplier.requestCount() != 1 {
		t.Fatalf("SSL submit batch=%+v supplier_calls=%d err=%v", second, supplier.requestCount(), err)
	}
	advanceFulfillmentTaskClock(t, prepared.checkout, prepared.order.OrderNo)
	third, err := worker.Run(prepared.checkout.ctx, 1)
	if err != nil || third.Selected != 1 || third.Completed != 1 || supplier.requestCount() != 2 {
		t.Fatalf("SSL issued reconciliation=%+v supplier_calls=%d err=%v", third, supplier.requestCount(), err)
	}

	requests := supplier.snapshot()
	if requests[0].Path != "/payment/balance" || requests[1].Path != "/order/sync" ||
		requests[0].EndpointField != "endpoint" || requests[0].SecretHeaders["X-API-Key"] != "api_key" ||
		requests[0].SecretHeaders["X-API-Secret"] != "api_secret" {
		t.Fatalf("unexpected governed SSL supplier requests: %+v", requests)
	}
	if strings.Contains(string(requests[0].Body), "PRIVATE KEY") {
		t.Fatal("SSL supplier request contained a private key")
	}

	var evidence struct {
		IntentStatus, TaskStatus, DeliveryStatus, EntitlementStatus  string
		PluginID, ExternalReference, PrimaryDomain, ValidationMethod string
		Entitlements, ProviderInvocations                            int
	}
	extensions.mu.Lock()
	evidence.ProviderInvocations = len(extensions.invocations)
	extensions.mu.Unlock()
	err = prepared.checkout.database.GetScan(prepared.checkout.ctx, &evidence, `SELECT
intent.status_code AS intent_status,task.status_code AS task_status,delivery.status_code AS delivery_status,
entitlement.status_code AS entitlement_status,entitlement.plugin_id,
task.external_reference,configuration.typed_value_snapshot->>'primary_domain' AS primary_domain,
configuration.typed_value_snapshot->>'validation_method' AS validation_method,
(SELECT count(*) FROM plugin_idc_core_service_entitlement counted WHERE counted.order_no=intent.order_no) AS entitlements
FROM plugin_idc_core_product_fulfillment_intent intent
JOIN plugin_idc_core_product_fulfillment_task task ON task.intent_no=intent.intent_no
JOIN plugin_idc_core_product_delivery_record delivery ON delivery.order_item_no=intent.order_item_no
JOIN plugin_idc_core_service_entitlement entitlement ON entitlement.source_intent_no=intent.intent_no
JOIN plugin_idc_core_commerce_structured_configuration_snapshot configuration
  ON configuration.snapshot_no=intent.configuration_snapshot_no
WHERE intent.order_no=?`, prepared.order.OrderNo)
	if err != nil || evidence.IntentStatus != "completed" || evidence.TaskStatus != "succeeded" ||
		evidence.DeliveryStatus != "completed" || evidence.EntitlementStatus != "active" ||
		evidence.PluginID != "yunsu-ssl" || evidence.ExternalReference != "YS-CHAIN-10001" ||
		evidence.PrimaryDomain != "example.com" || evidence.ValidationMethod != "dns" ||
		evidence.Entitlements != 1 || evidence.ProviderInvocations != 3 {
		t.Fatalf("SSL end-to-end evidence=%+v err=%v", evidence, err)
	}
}

func freezeYunsuSSLConfiguration(t *testing.T) func(checkoutTestScenario, structuredQuoteItemFixture) {
	t.Helper()
	planContent, err := json.Marshal(map[string]any{
		"version": "v1", "supplierPlanId": "ssl-dv-chain-1", "certificateValidationLevel": "dv",
		"dnsValidationSupported": true, "fileValidationSupported": true,
	})
	if err != nil {
		t.Fatal(err)
	}
	product, err := json.Marshal(map[string]any{
		"plan_id": "ysp1." + base64.RawURLEncoding.EncodeToString(planContent), "certificate_scope": "ordinary",
	})
	if err != nil {
		t.Fatal(err)
	}
	customer, err := json.Marshal(map[string]any{
		"primary_domain": "example.com", "validation_method": "dns",
		"email": "ssl-owner@example.com", "phone": "+8613800138000", "csr": sslChainCSR(t),
	})
	if err != nil {
		t.Fatal(err)
	}
	customerDeclaration := `{"version":"v1","fields":[` +
		`{"key":"primary_domain","type":"single_line","required":true,"label":{"zhCN":"主域名或 IP","enUS":"Primary domain or IP"},"maximumLength":255},` +
		`{"key":"validation_method","type":"single_select","required":true,"label":{"zhCN":"域名验证方式","enUS":"Domain validation method"},"staticCandidates":[{"value":"dns","label":{"zhCN":"DNS 验证","enUS":"DNS validation"}},{"value":"file","label":{"zhCN":"文件验证","enUS":"File validation"}}]},` +
		`{"key":"email","type":"single_line","required":true,"label":{"zhCN":"联系邮箱","enUS":"Contact email"},"textFormat":"email","maximumLength":255},` +
		`{"key":"phone","type":"single_line","required":true,"label":{"zhCN":"联系电话","enUS":"Contact phone"},"textFormat":"phone","maximumLength":64},` +
		`{"key":"csr","type":"multi_line","required":true,"label":{"zhCN":"证书签名请求（CSR）","enUS":"Certificate signing request (CSR)"},"maximumLength":32768}]}`
	declaration, err := configurationcontract.DecodeDeclaration(customerDeclaration)
	if err != nil {
		t.Fatalf("decode SSL customer declaration: %v", err)
	}
	if _, err = configurationcontract.DecodeCompiledValues(declaration, customer); err != nil {
		t.Fatalf("decode SSL customer values: %v", err)
	}
	return func(scenario checkoutTestScenario, _ structuredQuoteItemFixture) {
		tables := []string{
			"plugin_idc_core_product_automation_interface_instance",
			"plugin_idc_core_catalog_product_version_fulfillment_binding",
			"plugin_idc_core_catalog_product_version_fulfillment_policy",
			"plugin_idc_core_catalog_product_version_configuration",
			"plugin_idc_core_commerce_structured_configuration_snapshot",
		}
		for _, table := range tables {
			if _, alterErr := scenario.database.Exec(scenario.ctx, `ALTER TABLE `+table+` DISABLE TRIGGER USER`); alterErr != nil {
				t.Fatalf("disable %s SSL fixture guards: %v", table, alterErr)
			}
		}
		updates := []struct {
			statement string
			args      []any
		}{
			{`UPDATE plugin_idc_core_product_automation_interface_instance
SET display_name='Yunsu SSL A',plugin_id='yunsu-ssl',plugin_release_version='v0.3.1',
release_digest=repeat('a',64),declaration_digest=repeat('b',64)
WHERE interface_instance_no='pfi_11111111111111111111111111111111'`, nil},
			{`UPDATE plugin_idc_core_catalog_product_version_fulfillment_binding
SET interface_display_name_snapshot='Yunsu SSL A',plugin_id='yunsu-ssl',plugin_release_version='v0.3.1',
release_digest=repeat('a',64),declaration_digest=repeat('b',64)
WHERE version_no='version-commerce-service-1'`, nil},
			{`UPDATE plugin_idc_core_catalog_product_version_fulfillment_policy
SET strategy_code='payment_confirmed_plugin',granularity_code='per_order_item',ongoing_management=TRUE,
renewal_mode_code='local_entitlement',entitlement_owner_id='idc-core',provision_method='provision',query_method='query',
maximum_attempts=10,retry_base_seconds=30,policy_digest=repeat('f',64),created_by=101
WHERE version_no='version-commerce-service-1'`, nil},
			{`UPDATE plugin_idc_core_catalog_product_version_configuration
SET merchant_value_snapshot=?::jsonb,merchant_value_digest=repeat('d',64),
customer_schema_version='v1',customer_declaration_snapshot=?::jsonb
WHERE version_no='version-commerce-service-1'`, []any{string(product), customerDeclaration}},
			{`UPDATE plugin_idc_core_commerce_structured_configuration_snapshot
SET typed_value_snapshot=?::jsonb,customer_value_digest=repeat('a',64)
WHERE snapshot_no='SCF_33333333333333333333333333333333'`, []any{string(customer)}},
		}
		for _, update := range updates {
			if _, updateErr := scenario.database.Exec(scenario.ctx, update.statement, update.args...); updateErr != nil {
				t.Fatalf("freeze SSL product evidence: %v", updateErr)
			}
		}
		for _, table := range tables {
			if _, alterErr := scenario.database.Exec(scenario.ctx, `ALTER TABLE `+table+` ENABLE TRIGGER USER`); alterErr != nil {
				t.Fatalf("restore %s SSL fixture guards: %v", table, alterErr)
			}
		}
	}
}

func sslChainCSR(t *testing.T) string {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	request, err := x509.CreateCertificateRequest(rand.Reader, &x509.CertificateRequest{
		Subject: pkix.Name{CommonName: "example.com"}, DNSNames: []string{"example.com"},
	}, key)
	if err != nil {
		t.Fatal(err)
	}
	return string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE REQUEST", Bytes: request}))
}

type sslChainExtensions struct {
	mu          sync.Mutex
	handler     func(*protocol.BridgeRequestEnvelopeV1) (*protocol.BridgeResponseEnvelopeV1, error)
	invocations []pluginhost.BusinessExtensionMethodInvokeInput
}

func (*sslChainExtensions) ListProviders(context.Context, plugincap.BusinessExtensionProviderListInput) (*capmodel.PageResult[*capmodel.BusinessExtensionProviderEvidence], error) {
	return nil, errors.New("unused")
}
func (*sslChainExtensions) GetProviderExact(context.Context, plugincap.BusinessExtensionProviderExactInput) (*capmodel.BusinessExtensionProviderEvidence, error) {
	return nil, errors.New("unused")
}
func (*sslChainExtensions) GetProvidersExact(context.Context, []plugincap.BusinessExtensionProviderExactInput) ([]*capmodel.BusinessExtensionProviderEvidence, error) {
	return nil, errors.New("unused")
}
func (*sslChainExtensions) WithNewBusinessAdmission(context.Context, pluginhost.BusinessExtensionAdmissionInput, capmodel.BusinessExtensionAdmissionAction) error {
	return errors.New("unused")
}
func (*sslChainExtensions) WithConfidentialOutboundContext(context.Context, pluginhost.ConfidentialOutboundContextInput, pluginhost.ConfidentialOutboundAction) error {
	return errors.New("unused")
}
func (e *sslChainExtensions) InvokeProviderMethod(_ context.Context, input pluginhost.BusinessExtensionMethodInvokeInput) (*capmodel.BusinessExtensionMethodResult, error) {
	e.mu.Lock()
	e.invocations = append(e.invocations, input)
	e.mu.Unlock()
	invocation := capmodel.BusinessExtensionBridgeRequest{
		Version: capmodel.BusinessExtensionInvocationProtocolV1, OwnerPluginID: "idc-core",
		ProviderPluginID: input.PluginID, Point: input.Point, ContractVersion: input.ContractVersion,
		Method: input.Method, ReleaseVersion: input.ReleaseVersion, ReleaseDigest: input.ReleaseDigest,
		DeclarationDigest: input.DeclarationDigest, Purpose: input.Purpose,
		RuntimeContext: input.RuntimeContext, Payload: input.Payload,
	}
	body, err := json.Marshal(invocation)
	if err != nil {
		return nil, err
	}
	response, err := e.handler(&protocol.BridgeRequestEnvelopeV1{
		PluginID: input.PluginID,
		Route:    &protocol.RouteMatchSnapshotV1{RequestType: capmodel.BusinessExtensionBridgeRequestType},
		Request:  &protocol.HTTPRequestSnapshotV1{Body: body},
	})
	if err != nil || response == nil || response.StatusCode != http.StatusOK {
		return nil, errors.New("Yunsu SSL guest invocation failed")
	}
	var result capmodel.BusinessExtensionBridgeResponse
	if err = json.Unmarshal(response.Body, &result); err != nil {
		return nil, err
	}
	return &capmodel.BusinessExtensionMethodResult{Version: result.Version, Payload: result.Payload}, nil
}

type sslChainSupplier struct {
	mu       sync.Mutex
	requests []supplierinterfacecap.OutboundHTTPRequest
}

func (*sslChainSupplier) GetConfiguration(context.Context, string) (*supplierinterfacecap.ConfigurationView, error) {
	return nil, errors.New("unused")
}
func (*sslChainSupplier) ListConfigurationFiles(context.Context, string, string) (*supplierinterfacecap.ConfigurationFileList, error) {
	return nil, errors.New("unused")
}
func (*sslChainSupplier) ReadConfigurationFile(context.Context, string, supplierinterfacecap.ConfigurationFileReadRequest) (*supplierinterfacecap.ConfigurationFileChunk, error) {
	return nil, errors.New("unused")
}
func (s *sslChainSupplier) ExecuteOutboundHTTP(_ context.Context, _ string, request supplierinterfacecap.OutboundHTTPRequest) (*supplierinterfacecap.OutboundHTTPResponse, error) {
	s.mu.Lock()
	s.requests = append(s.requests, request)
	s.mu.Unlock()
	switch request.Path {
	case "/payment/balance":
		return &supplierinterfacecap.OutboundHTTPResponse{StatusCode: http.StatusOK, Data: json.RawMessage(`{"success":true,"order_id":"YS-CHAIN-10001"}`)}, nil
	case "/order/sync":
		return &supplierinterfacecap.OutboundHTTPResponse{StatusCode: http.StatusOK, Data: json.RawMessage(`{"success":true,"data":{"status":"issued"}}`)}, nil
	default:
		return nil, errors.New("unexpected SSL supplier path")
	}
}
func (*sslChainSupplier) ExecuteOutboundSession(context.Context, string, supplierinterfacecap.OutboundSessionRequest) (*supplierinterfacecap.OutboundSessionResponse, error) {
	return nil, errors.New("unused")
}
func (*sslChainSupplier) AccessSecret(context.Context, string, string, func([]byte) error) error {
	return errors.New("unused")
}
func (s *sslChainSupplier) requestCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.requests)
}
func (s *sslChainSupplier) snapshot() []supplierinterfacecap.OutboundHTTPRequest {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]supplierinterfacecap.OutboundHTTPRequest(nil), s.requests...)
}
