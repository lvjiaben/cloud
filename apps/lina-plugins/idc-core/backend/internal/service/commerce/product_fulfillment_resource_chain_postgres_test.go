package commerce

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"

	"lina-core/pkg/plugin/capability"
	"lina-core/pkg/plugin/capability/capmodel"
	"lina-core/pkg/plugin/capability/plugincap"
	"lina-core/pkg/plugin/pluginhost"
	"lina-plugin-idc-core/backend/cap/entitlementcreatedcap"
	entspi "lina-plugin-idc-core/backend/cap/entitlementcreatedcap/spi"
	"lina-plugin-idc-core/backend/cap/provisionresourcecap"
	provisionspi "lina-plugin-idc-core/backend/cap/provisionresourcecap/spi"
	"lina-plugin-idc-core/backend/cap/resourceinterfaceprojectioncap"
	projectionspi "lina-plugin-idc-core/backend/cap/resourceinterfaceprojectioncap/spi"
	"lina-plugin-idc-core/backend/cap/secretcap"
	secretspi "lina-plugin-idc-core/backend/cap/secretcap/spi"
	billingservice "lina-plugin-idc-core/backend/internal/service/billing"
	productfulfillmentservice "lina-plugin-idc-core/backend/internal/service/productfulfillment"
	"lina-plugin-idc-core/backend/internal/service/purchaseconfigurationfile"
	_ "lina-plugin-idc-resource/backend"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
)

// TestPaidServerSettlementCrossesResourcePluginAndProjectsEntitlementPostgreSQL
// proves one continuous paid-server path. It deliberately drives the resource
// plugin through its public source declaration and registered host job instead
// of importing any resource-plugin internal package.
func TestPaidServerSettlementCrossesResourcePluginAndProjectsEntitlementPostgreSQL(t *testing.T) {
	prepared := prepareCrossPluginServerScenario(t)
	ctx, database := prepared.checkout.ctx, prepared.checkout.database
	configureResourceChainDatabase(t, ctx, database)

	extensions, resourceJob, source := crossPluginRuntime(t, ctx)

	sealer, err := productfulfillmentservice.NewProviderEvidenceSealer(
		"test-key-v1", []byte("0123456789abcdef0123456789abcdef"),
	)
	if err != nil {
		t.Fatal(err)
	}
	worker, err := productfulfillmentservice.NewWorker(
		database, extensions, &fulfillmentWorkerContexts{}, sealer,
		fulfillmentSystemReasons(t), source, nil, nil,
	)
	if err != nil {
		t.Fatal(err)
	}

	first, err := worker.Run(ctx, 1)
	if err != nil || first.Selected != 1 || first.Scheduled != 1 {
		t.Fatalf("query-first resource fulfillment=%+v err=%v", first, err)
	}
	advanceFulfillmentTaskClock(t, prepared.checkout, prepared.order.OrderNo)
	second, err := worker.Run(ctx, 1)
	if err != nil || second.Selected != 1 || second.Scheduled != 1 {
		t.Fatalf("resource task creation=%+v err=%v", second, err)
	}
	if err = resourceJob(ctx); err != nil {
		t.Fatalf("execute resource plugin task job: %v", err)
	}
	advanceFulfillmentTaskClock(t, prepared.checkout, prepared.order.OrderNo)
	third, err := worker.Run(ctx, 1)
	if err != nil || third.Selected != 1 || third.Completed != 1 {
		var diagnostic struct {
			IntentStatus, CoreTaskStatus, CorePhase, ResourceTaskStatus string
			CoreFailure, ResourceFailure                                string
		}
		diagnosticErr := database.GetScan(ctx, &diagnostic, `SELECT intent.status_code AS intent_status,
core_task.status_code AS core_task_status,core_task.phase_code AS core_phase,
core_task.last_result_code AS core_failure,resource_task.status_code AS resource_task_status,
resource_task.failure_code AS resource_failure
FROM plugin_idc_core_product_fulfillment_intent intent
JOIN plugin_idc_core_product_fulfillment_task core_task ON core_task.intent_no=intent.intent_no
LEFT JOIN plugin_idc_resource_task resource_task ON resource_task.owner_metadata->>'operationNo'=intent.intent_no
WHERE intent.order_no=?`, prepared.order.OrderNo)
		t.Fatalf("resource result reconciliation=%+v diagnostic=%+v diagnostic_err=%v err=%v", third, diagnostic, diagnosticErr, err)
	}

	router, err := entspi.Freeze().Build(ctx)
	if err != nil {
		t.Fatalf("build entitlement owner router: %v", err)
	}
	outcomeWorker, err := productfulfillmentservice.NewResourceOutcomeWorker(database, source)
	if err != nil {
		t.Fatal(err)
	}
	reported, err := outcomeWorker.Run(ctx, 1)
	if err != nil || reported.Reported != 1 {
		t.Fatalf("report resource outcome=%+v err=%v", reported, err)
	}
	entitlementWorker, err := productfulfillmentservice.NewEntitlementCreatedWorker(database, router)
	if err != nil {
		t.Fatal(err)
	}
	delivered, err := entitlementWorker.Run(ctx, 1)
	if err != nil || delivered.Delivered != 1 {
		var event entitlementcreatedcap.Event
		event.Version = entitlementcreatedcap.ContractVersion
		loadErr := database.GetScan(ctx, &event, `SELECT event_no,entitlement_no,intent_no,customer_no,product_no,
version_no AS product_version_no,product_name_zh_cn,product_name_en_us,configuration_snapshot_no,
interface_instance_no,plugin_id,plugin_release_version,contract_version,declaration_digest,owner_id
FROM plugin_idc_core_entitlement_created_outbox WHERE status_code='retry_wait' LIMIT 1`)
		sink, _ := router.Resolve(event.OwnerID)
		_, directErr := sink.Deliver(ctx, event)
		t.Fatalf("project server entitlement=%+v event=%+v load_err=%v direct_err=%v err=%v", delivered, event, loadErr, directErr, err)
	}
	var evidence struct {
		SettlementBasis, IntentStatus, CoreTaskStatus, ResourceTaskStatus string
		MappingStatus, ExternalResourceRef, EntitlementStatus             string
		ResourceProjectionRows, OutcomeReceipts, DeliveredEvents          int
	}
	err = database.GetScan(ctx, &evidence, `SELECT
settlement.basis_code AS settlement_basis,intent.status_code AS intent_status,
core_task.status_code AS core_task_status,resource_task.status_code AS resource_task_status,
mapping.status_code AS mapping_status,mapping.external_resource_ref,
entitlement.status_code AS entitlement_status,
(SELECT COUNT(*) FROM plugin_idc_resource_server_entitlement_projection projection
 WHERE projection.entitlement_no=entitlement.entitlement_no) AS resource_projection_rows,
(SELECT COUNT(*) FROM plugin_idc_resource_provision_outcome_receipt receipt
 WHERE receipt.task_no=resource_task.task_no) AS outcome_receipts,
(SELECT COUNT(*) FROM plugin_idc_core_entitlement_created_outbox event
 WHERE event.entitlement_no=entitlement.entitlement_no AND event.status_code='delivered') AS delivered_events
FROM plugin_idc_core_financial_settlement settlement
JOIN plugin_idc_core_product_fulfillment_intent intent ON intent.settlement_no=settlement.settlement_no
JOIN plugin_idc_core_product_fulfillment_task core_task ON core_task.intent_no=intent.intent_no
JOIN plugin_idc_resource_task resource_task ON resource_task.owner_metadata->>'operationNo'=intent.intent_no
JOIN plugin_idc_resource_mapping mapping ON mapping.local_resource_no=resource_task.local_resource_no
JOIN plugin_idc_core_service_entitlement entitlement ON entitlement.source_intent_no=intent.intent_no
WHERE settlement.settlement_no=?`, prepared.settleNo)
	if err != nil || evidence.SettlementBasis != "funded_invoice" || evidence.IntentStatus != "completed" ||
		evidence.CoreTaskStatus != "succeeded" || evidence.ResourceTaskStatus != "succeeded" ||
		evidence.MappingStatus != "active" || evidence.ExternalResourceRef != "upstream-cross-plugin-server-1" ||
		evidence.EntitlementStatus != "active" || evidence.ResourceProjectionRows != 1 ||
		evidence.OutcomeReceipts != 1 || evidence.DeliveredEvents != 1 ||
		extensions.invocationCount(resourcecap.DynamicProviderMethodBuildProvisionParameters) != 1 ||
		extensions.invocationCount(resourcecap.DynamicProviderMethodExecute) != 1 {
		t.Fatalf("cross-plugin server evidence=%+v invocations=%+v err=%v", evidence, extensions.invocations, err)
	}
}

func prepareCrossPluginServerScenario(t *testing.T) genericWorkerScenario {
	t.Helper()
	scenario := newCheckoutTestScenarioWithSetup(t, "customer-commerce-service-1", 101, 1, false, 0,
		func(ctx context.Context, database gdb.DB) {
			prepareStructuredCheckoutDatabase(t, ctx, database)
			applyCrossPluginCoreMigrations(t, ctx, database)
			configureCrossPluginServerCatalog(t, ctx, database)
		})
	quoteItem := loadStructuredQuoteItem(t, scenario, scenario.quote.QuoteNo)
	seedCrossPluginServerCheckoutEvidence(t, scenario, quoteItem)
	consumer, err := purchaseconfigurationfile.NewOrderFileConsumer(scenario.database, scenario.service.principal)
	if err != nil {
		t.Fatal(err)
	}
	if err = BindStructuredConfigurationSnapshots(scenario.service, structuredCheckoutFreezerStub{}, consumer); err != nil {
		t.Fatal(err)
	}
	insertVerifiedIndividualPaymentIdentity(t, scenario, "customer-commerce-service-1")
	checkout, err := scenario.service.CheckoutAccountQuote(scenario.ctx, CheckoutRequest{
		QuoteNo: scenario.quote.QuoteNo, IdempotencyKey: "cross-plugin-server-checkout-0001",
	})
	if err != nil {
		t.Fatal(err)
	}
	seedFinancialSettlementMerchant(t, scenario)
	balance, _ := billingservice.NewBalanceService(scenario.database)
	if _, err = balance.PostConfirmedRecharge(scenario.ctx, billingservice.ConfirmedRechargeInput{
		CustomerNo: "customer-commerce-service-1", CurrencyCode: "CNY", AmountMinor: checkout.TotalAmountMinor,
		SourceReference: "cross-plugin-server-recharge", SourceSnapshotDigest: strings.Repeat("d", 64),
	}); err != nil {
		t.Fatal(err)
	}
	invoices, _ := billingservice.NewInvoiceService(scenario.database)
	if _, err = invoices.PayFromBalance(scenario.ctx, billingservice.InvoiceBalancePaymentInput{
		InvoiceNo: checkout.InvoiceNo, SourceReference: "cross-plugin-server-payment",
		SourceSnapshotDigest: strings.Repeat("e", 64),
	}); err != nil {
		t.Fatal(err)
	}
	settlement, err := newFinancialSettlementTestService(t, scenario).SettleOrder(scenario.ctx, checkout.OrderNo)
	if err != nil {
		t.Fatal(err)
	}
	files, err := purchaseconfigurationfile.NewOrderEvidenceVerifier(scenario.database)
	if err != nil {
		t.Fatal(err)
	}
	service, err := productfulfillmentservice.NewFullConfigured(
		scenario.database, structuredFulfillmentEvidenceValidator{files: files}, fulfillmentGateStub{},
		fulfillmentSystemReasons(t), fulfillmentAuthorizerStub{}, fulfillmentAuditStub{}, fulfillmentActorsStub{},
		[]byte("0123456789abcdef0123456789abcdef"),
	)
	if err != nil {
		t.Fatal(err)
	}
	materialized, err := service.ConsumeFinanciallySettled(scenario.ctx, productfulfillmentservice.SettlementInput{
		SettlementNo: settlement.SettlementNo, OrderNo: checkout.OrderNo,
	})
	if err != nil || materialized.TaskCount != 1 {
		t.Fatalf("materialize cross-plugin server fulfillment=%+v err=%v", materialized, err)
	}
	return genericWorkerScenario{checkout: scenario, order: checkout, service: service, settleNo: settlement.SettlementNo}
}

func seedCrossPluginServerCheckoutEvidence(t *testing.T, scenario checkoutTestScenario, quoteItem structuredQuoteItemFixture) {
	t.Helper()
	_, err := scenario.database.Exec(scenario.ctx, `INSERT INTO plugin_idc_core_commerce_structured_configuration_snapshot
(tenant_id,snapshot_no,customer_no,uploader_user_id,quote_no,quote_item_no,product_no,version_no,
 configuration_revision,interface_instance_no,interface_configuration_revision,customer_declaration_digest,
 candidate_snapshot_digest,customer_value_digest,candidate_snapshot,typed_value_snapshot,content_digest)
VALUES (0,'SCF_33333333333333333333333333333333','customer-commerce-service-1',101,?,?,?,?,
 'cfg-structured-1','pfi_11111111111111111111111111111111',1,repeat('8',64),repeat('9',64),repeat('a',64),
 '{}'::jsonb,'{"operating_system":"ubuntu-2204"}'::jsonb,repeat('b',64))`,
		scenario.quote.QuoteNo, quoteItem.QuoteItemNo, quoteItem.ProductNo, quoteItem.VersionNo)
	if err != nil {
		t.Fatalf("seed frozen server purchase configuration: %v", err)
	}
}

type crossPluginProjectionSink struct{}

func (crossPluginProjectionSink) Upsert(context.Context, resourceinterfaceprojectioncap.Projection) error {
	return nil
}

var crossPluginRuntimeState = struct {
	once       sync.Once
	extensions *crossPluginBusinessExtensions
	secrets    *crossPluginSecretSource
	job        pluginhost.JobHandler
	source     provisionresourcecap.Source
	err        error
}{
	extensions: &crossPluginBusinessExtensions{},
	secrets:    newCrossPluginSecretSource(),
}

func crossPluginRuntime(t *testing.T, ctx context.Context) (*crossPluginBusinessExtensions, pluginhost.JobHandler, provisionresourcecap.Source) {
	t.Helper()
	state := &crossPluginRuntimeState
	state.once.Do(func() {
		if err := secretspi.Declare(func(context.Context) (secretcap.Source, error) { return state.secrets, nil }); err != nil {
			state.err = err
			return
		}
		if err := projectionspi.DeclareSink(func(context.Context) (resourceinterfaceprojectioncap.Sink, error) {
			return crossPluginProjectionSink{}, nil
		}); err != nil {
			state.err = err
			return
		}
		state.job = registerCrossPluginResourceJobs(t, ctx, state.extensions)
		snapshot := provisionspi.Freeze()
		if snapshot.Factory() == nil {
			state.err = errors.New("resource plugin did not publish its provisioning source")
			return
		}
		state.source, state.err = snapshot.Factory()(ctx)
	})
	if state.err != nil || state.job == nil || state.source == nil {
		t.Fatalf("compose cross-plugin runtime: %v", state.err)
	}
	state.extensions.reset()
	state.secrets.reset()
	return state.extensions, state.job, state.source
}

type crossPluginJobRegistrar struct {
	extensions pluginhost.BusinessExtensionRegistrar
	jobs       map[string]pluginhost.JobHandler
}

func (r *crossPluginJobRegistrar) Add(ctx context.Context, pattern, name string, handler pluginhost.JobHandler) error {
	return r.AddWithMetadata(ctx, pattern, name, name, "", handler)
}

func (r *crossPluginJobRegistrar) AddWithMetadata(_ context.Context, _, name, _, _ string, handler pluginhost.JobHandler) error {
	if handler == nil || name == "" {
		return errors.New("invalid cross-plugin job")
	}
	if r.jobs == nil {
		r.jobs = make(map[string]pluginhost.JobHandler)
	}
	if _, exists := r.jobs[name]; exists {
		return errors.New("duplicate cross-plugin job")
	}
	r.jobs[name] = handler
	return nil
}

func (*crossPluginJobRegistrar) IsPrimaryNode() bool           { return true }
func (*crossPluginJobRegistrar) Services() capability.Services { return nil }
func (r *crossPluginJobRegistrar) BusinessExtensions() pluginhost.BusinessExtensionRegistrar {
	return r.extensions
}

func registerCrossPluginResourceJobs(t *testing.T, ctx context.Context, extensions pluginhost.BusinessExtensionRegistrar) pluginhost.JobHandler {
	t.Helper()
	plugin, ok := pluginhost.GetSourcePlugin("idc-resource")
	if !ok {
		t.Fatal("idc-resource source plugin is not registered")
	}
	registrations := plugin.GetJobRegistrars()
	if len(registrations) != 1 || registrations[0] == nil || registrations[0].Handler == nil {
		t.Fatalf("unexpected idc-resource job declarations: %+v", registrations)
	}
	registrar := &crossPluginJobRegistrar{extensions: extensions, jobs: make(map[string]pluginhost.JobHandler)}
	if err := registrations[0].Handler(ctx, registrar); err != nil {
		t.Fatalf("compose idc-resource jobs: %v", err)
	}
	handler := registrar.jobs["idc-resource-provider-tasks"]
	if handler == nil {
		t.Fatal("idc-resource provider task job is missing")
	}
	return handler
}

type crossPluginBusinessExtensions struct {
	mu            sync.Mutex
	registrations map[string]pluginhost.ConfidentialOutboundContextInput
	invocations   map[string]int
}

func (e *crossPluginBusinessExtensions) reset() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.registrations = make(map[string]pluginhost.ConfidentialOutboundContextInput)
	e.invocations = make(map[string]int)
}

func (*crossPluginBusinessExtensions) ListProviders(context.Context, plugincap.BusinessExtensionProviderListInput) (*capmodel.PageResult[*capmodel.BusinessExtensionProviderEvidence], error) {
	return nil, errors.New("unused")
}
func (*crossPluginBusinessExtensions) GetProviderExact(context.Context, plugincap.BusinessExtensionProviderExactInput) (*capmodel.BusinessExtensionProviderEvidence, error) {
	return nil, errors.New("unused")
}
func (*crossPluginBusinessExtensions) GetProvidersExact(context.Context, []plugincap.BusinessExtensionProviderExactInput) ([]*capmodel.BusinessExtensionProviderEvidence, error) {
	return nil, errors.New("unused")
}
func (*crossPluginBusinessExtensions) WithNewBusinessAdmission(context.Context, pluginhost.BusinessExtensionAdmissionInput, capmodel.BusinessExtensionAdmissionAction) error {
	return errors.New("unused")
}
func (e *crossPluginBusinessExtensions) WithConfidentialOutboundContext(ctx context.Context, input pluginhost.ConfidentialOutboundContextInput, action pluginhost.ConfidentialOutboundAction) error {
	e.mu.Lock()
	if e.registrations == nil {
		e.registrations = make(map[string]pluginhost.ConfidentialOutboundContextInput)
	}
	e.registrations[input.ProviderMethod] = input
	e.mu.Unlock()
	return action(ctx, &capmodel.BusinessExtensionRuntimeContext{ContextID: "opaque-cross-plugin-context"})
}
func (e *crossPluginBusinessExtensions) InvokeProviderMethod(_ context.Context, input pluginhost.BusinessExtensionMethodInvokeInput) (*capmodel.BusinessExtensionMethodResult, error) {
	e.mu.Lock()
	if e.invocations == nil {
		e.invocations = make(map[string]int)
	}
	e.invocations[input.Method]++
	e.mu.Unlock()
	var payload []byte
	var err error
	switch input.Method {
	case resourcecap.DynamicProviderMethodBuildProvisionParameters:
		payload, err = json.Marshal(map[string]any{
			"num": 1, "cpu": 2, "memory": 4096, "os": "3", "client": 88,
			"hostname": "server-cross-plugin-1",
		})
	case resourcecap.DynamicProviderMethodExecute:
		payload, err = json.Marshal(resourcecap.OperationResult{
			Status:              resourcecap.TaskStatusSucceeded,
			ExternalResourceRef: "upstream-cross-plugin-server-1",
		})
	default:
		return nil, errors.New("unexpected cross-plugin provider method")
	}
	return &capmodel.BusinessExtensionMethodResult{
		Version: capmodel.BusinessExtensionInvocationProtocolV1, Payload: payload,
	}, err
}
func (e *crossPluginBusinessExtensions) invocationCount(method string) int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.invocations[method]
}

type crossPluginSecretSource struct {
	mu      sync.Mutex
	values  map[string][]byte
	results map[string]secretcap.CreateResult
}

func newCrossPluginSecretSource() *crossPluginSecretSource {
	return &crossPluginSecretSource{values: make(map[string][]byte), results: make(map[string]secretcap.CreateResult)}
}
func (s *crossPluginSecretSource) reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.values = make(map[string][]byte)
	s.results = make(map[string]secretcap.CreateResult)
}
func (s *crossPluginSecretSource) Accessor(secretcap.Scope) (secretcap.Accessor, error) {
	return s, nil
}
func (s *crossPluginSecretSource) Manager(secretcap.Scope) (secretcap.Manager, error) { return s, nil }
func (s *crossPluginSecretSource) Access(_ context.Context, request secretcap.AccessRequest, consume secretcap.Consumer) error {
	s.mu.Lock()
	value := append([]byte(nil), s.values[request.Handle.OpaqueRef()]...)
	s.mu.Unlock()
	if len(value) == 0 {
		return errors.New("cross-plugin secret is unavailable")
	}
	return consume(value)
}
func (s *crossPluginSecretSource) Create(_ context.Context, key string, produce secretcap.PlaintextProducer) (secretcap.CreateResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if result, ok := s.results[key]; ok {
		result.Reused = true
		return result, nil
	}
	var value []byte
	if err := produce(func(plaintext []byte) error {
		value = append([]byte(nil), plaintext...)
		return nil
	}); err != nil {
		return secretcap.CreateResult{}, err
	}
	encoded := hex.EncodeToString([]byte(key))
	if len(encoded) > 48 {
		encoded = encoded[:48]
	}
	handle, err := secretcap.NewHandle("cross-plugin-secret-" + encoded)
	if err != nil {
		return secretcap.CreateResult{}, err
	}
	result := secretcap.CreateResult{Handle: handle, Metadata: secretcap.Metadata{Configured: true, UpdatedAt: time.Now().UTC()}}
	s.values[handle.OpaqueRef()] = value
	s.results[key] = result
	return result, nil
}
func (s *crossPluginSecretSource) CreateGenerated(ctx context.Context, key, policy string) (secretcap.CreateResult, error) {
	if policy != secretcap.GeneratedSecretPolicyURLSafe256 {
		return secretcap.CreateResult{}, errors.New("unsupported generated secret policy")
	}
	return s.Create(ctx, key, func(consume secretcap.Consumer) error {
		return consume([]byte("CrossPluginGeneratedPassword-2026"))
	})
}
func (s *crossPluginSecretSource) RecoverCreate(_ context.Context, key string) (secretcap.CreateResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	result, ok := s.results[key]
	if !ok {
		return secretcap.CreateResult{}, errors.New("cross-plugin secret result is unavailable")
	}
	result.Reused = true
	return result, nil
}
func (*crossPluginSecretSource) Replace(context.Context, secretcap.Handle, time.Time, string, secretcap.PlaintextProducer) (secretcap.MutationResult, error) {
	return secretcap.MutationResult{}, errors.New("unused")
}
func (s *crossPluginSecretSource) Metadata(_ context.Context, handle secretcap.Handle) (secretcap.Metadata, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, result := range s.results {
		if result.Handle.Equal(handle) {
			return result.Metadata, nil
		}
	}
	return secretcap.Metadata{}, errors.New("cross-plugin secret is unavailable")
}
func (*crossPluginSecretSource) Revoke(context.Context, secretcap.Handle, time.Time) (secretcap.Metadata, error) {
	return secretcap.Metadata{}, errors.New("unused")
}
func (*crossPluginSecretSource) Rotate(context.Context, int) (secretcap.RotationResult, error) {
	return secretcap.RotationResult{}, errors.New("unused")
}

func configureResourceChainDatabase(t *testing.T, ctx context.Context, database gdb.DB) {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve cross-plugin test path")
	}
	paths, err := filepath.Glob(filepath.Join(filepath.Dir(file), "..", "..", "..", "..", "..", "idc-resource", "manifest", "sql", "[0-9][0-9][0-9]-*.sql"))
	if err != nil || len(paths) == 0 {
		t.Fatalf("resolve resource migrations: %v", err)
	}
	for _, path := range paths {
		content, readErr := os.ReadFile(path)
		if readErr != nil {
			t.Fatalf("read resource migration %s: %v", filepath.Base(path), readErr)
		}
		if _, err = database.Exec(ctx, string(content)); err != nil {
			t.Fatalf("execute resource migration %s: %v", filepath.Base(path), err)
		}
	}
	schema, err := database.GetValue(ctx, `SELECT current_schema()`)
	if err != nil {
		t.Fatal(err)
	}
	original := gdb.GetAllConfig()
	t.Cleanup(func() {
		if restoreErr := gdb.SetConfig(original); restoreErr != nil {
			t.Errorf("restore cross-plugin database config: %v", restoreErr)
		}
	})
	dsn := strings.TrimSpace(os.Getenv(commerceIntegrationDSNEnv))
	separator := "?"
	if strings.Contains(dsn, "?") {
		separator = "&"
	}
	if err = gdb.SetConfig(gdb.Config{gdb.DefaultGroupName: gdb.ConfigGroup{{Link: dsn + separator + "search_path=" + schema.String()}}}); err != nil {
		t.Fatal(err)
	}
	actual, err := g.DB().GetValue(ctx, `SELECT current_schema()`)
	if err != nil || actual.String() != schema.String() {
		t.Fatalf("resource DAO schema=%q want=%q err=%v", actual.String(), schema.String(), err)
	}
	seedCrossPluginResourceRoute(t, ctx, database)
}

func applyCrossPluginCoreMigrations(t *testing.T, ctx context.Context, database gdb.DB) {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve cross-plugin core migration path")
	}
	for _, name := range []string{"096-resource-owned-product-fulfillment.sql", "097-resource-interface-configuration-revision.sql"} {
		path := filepath.Join(filepath.Dir(file), "..", "..", "..", "..", "manifest", "sql", name)
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read core resource migration %s: %v", name, err)
		}
		if _, err = database.Exec(ctx, string(content)); err != nil {
			t.Fatalf("execute core resource migration %s: %v", name, err)
		}
	}
}

func seedCrossPluginResourceRoute(t *testing.T, ctx context.Context, database gdb.DB) {
	t.Helper()
	selector := `{"provision":{"provider":"zjmfcloud-v1","areas":{"cn-south":"1"},"nodes":{"premium":"2"},"node_groups":{},"images":{"ubuntu-2204":"3"},"defaults":{"network_type":"normal","ip_group":"4","traffic_type":3,"node_priority":1}}}`
	statements := []struct {
		sql  string
		args []any
	}{
		{sql: `INSERT INTO plugin_idc_resource_channel
(channel_no,provider_id,provider_plugin_id,provider_runtime_type,provider_contract_version,
 provider_release_version,provider_release_digest,provider_declaration_digest,display_name,endpoint,status_code)
VALUES ('channel-provision-1','zjmfcloud-v1','idc-zjmfcloud','dynamic','v1','v0.1.0',repeat('a',64),repeat('b',64),'Cross Plugin','https://provider.invalid','enabled')`},
		{sql: `INSERT INTO plugin_idc_resource_interface_channel_binding
(interface_instance_no,channel_no,binding_version,current_configuration_revision,product_plugin_id,
 product_plugin_release_version,product_contract_version,product_authorized_methods,product_release_digest,product_declaration_digest)
VALUES ('pfi_11111111111111111111111111111111','channel-provision-1',1,'rci_11111111111111111111111111111111',
 'idc-zjmfcloud','v0.1.0','v1',ARRAY['provision','query'],repeat('a',64),repeat('b',64))`},
		{sql: `INSERT INTO plugin_idc_resource_interface_configuration_revision
(revision_no,interface_instance_no,channel_no,endpoint,non_secret_settings,secret_bindings,created_by)
VALUES ('rci_11111111111111111111111111111111','pfi_11111111111111111111111111111111','channel-provision-1','https://provider.invalid','{}','{}',101)`},
		{sql: `INSERT INTO plugin_idc_resource_pool(pool_no,channel_no,display_name,resource_kind,status_code,selector)
VALUES ('pool-provision-1','channel-provision-1','Cross Plugin Pool','virtual_server','enabled',?::jsonb)`, args: []any{selector}},
		{sql: `INSERT INTO plugin_idc_resource_principal_mapping
(mapping_no,customer_no,channel_no,external_principal_ref,status_code,evidence_digest,observed_at)
VALUES ('principal-provision-1','customer-commerce-service-1','channel-provision-1','88','active',repeat('a',64),CURRENT_TIMESTAMP)`},
		{sql: `INSERT INTO plugin_idc_resource_capability_snapshot
(snapshot_no,channel_no,adapter_version,status_code,item_count,observed_at,completed_at)
			VALUES ('capability-provision-1','channel-provision-1','zjmfcloud-admin-v1','complete',?,CURRENT_TIMESTAMP,CURRENT_TIMESTAMP)`, args: []any{resourcecap.MaximumCapabilityCount}},
	}
	for _, statement := range statements {
		if _, err := database.Exec(ctx, statement.sql, statement.args...); err != nil {
			t.Fatalf("seed cross-plugin resource route: %v", err)
		}
	}
	constraints := `[{"name":"num","required":false,"allowedValues":["1"]},{"name":"cpu","required":true,"minimum":1},{"name":"memory","required":true,"minimum":128},{"name":"os","required":true},{"name":"client","required":true,"minimum":1},{"name":"hostname","required":true}]`
	for _, capabilityCode := range resourcecap.SupportedCapabilityCodes() {
		support, itemConstraints, reason := "unsupported", `[]`, "unused by cross-plugin fixture"
		if capabilityCode == resourcecap.CapabilityProvision {
			support, itemConstraints, reason = "conditional", constraints, ""
		}
		if _, err := database.Exec(ctx, `INSERT INTO plugin_idc_resource_capability_snapshot_item(snapshot_no,capability_code,support_code,constraints,unsupported_reason) VALUES ('capability-provision-1',?,?,CAST(? AS JSONB),?)`, capabilityCode, support, itemConstraints, reason); err != nil {
			t.Fatalf("seed cross-plugin capability %s: %v", capabilityCode, err)
		}
	}
}

func configureCrossPluginServerCatalog(t *testing.T, ctx context.Context, database gdb.DB) {
	t.Helper()
	for _, table := range []string{
		"plugin_idc_core_catalog_product_version_configuration",
		"plugin_idc_core_catalog_product_version_fulfillment_binding",
		"plugin_idc_core_catalog_product_version_fulfillment_policy",
		"plugin_idc_core_product_automation_interface_instance",
		"plugin_idc_core_product_automation_interface_configuration_revision",
	} {
		if _, err := database.Exec(ctx, `ALTER TABLE `+table+` DISABLE TRIGGER USER`); err != nil {
			t.Fatalf("disable %s test guards: %v", table, err)
		}
		defer func(table string) {
			if _, err := database.Exec(ctx, `ALTER TABLE `+table+` ENABLE TRIGGER USER`); err != nil {
				t.Errorf("restore %s test guards: %v", table, err)
			}
		}(table)
	}
	product := `{"region":"cn-south","line":"premium","cpu":2,"memory":4096,"system_disk":40,"data_disk":0,"bandwidth":100,"upstream_product_id":"v10-product-41","upstream_region_option_id":"region-3"}`
	declaration := `{"version":"v1","fields":[{"key":"operating_system","type":"single_line","label":{"zhCN":"操作系统","enUS":"Operating system"}}]}`
	updates := []struct {
		sql  string
		args []any
	}{
		{sql: `UPDATE plugin_idc_core_catalog_product_version_configuration
SET merchant_value_snapshot=CAST(? AS JSONB),merchant_value_digest=repeat('d',64),customer_declaration_snapshot=CAST(? AS JSONB)
WHERE version_no='version-commerce-service-1'`, args: []any{product, declaration}},
		{sql: `UPDATE plugin_idc_core_product_automation_interface_configuration_revision
SET resource_owner_configuration_revision='rci_11111111111111111111111111111111'
WHERE interface_instance_no='pfi_11111111111111111111111111111111' AND revision=1`},
		{sql: `UPDATE plugin_idc_core_product_automation_interface_instance
SET plugin_id='idc-zjmfcloud',plugin_release_version='v0.1.0',contract_version='v1',
    release_digest=repeat('a',64),declaration_digest=repeat('b',64),
    interface_origin_code='resource',resource_owner_id='idc-resource',resource_channel_ref='CHN_cross_plugin_1'
WHERE interface_instance_no='pfi_11111111111111111111111111111111'`},
		{sql: `UPDATE plugin_idc_core_catalog_product_version_fulfillment_binding
SET plugin_id='idc-zjmfcloud',plugin_release_version='v0.1.0',contract_version='v1',
    release_digest=repeat('a',64),declaration_digest=repeat('b',64)
WHERE version_no='version-commerce-service-1'`},
		{sql: `UPDATE plugin_idc_core_catalog_product_version_fulfillment_policy
SET strategy_code='payment_confirmed_plugin',granularity_code='per_order_item',ongoing_management=TRUE,
    renewal_mode_code='local_entitlement',entitlement_owner_id='idc-resource',
    provision_method='provision',query_method='query',maximum_attempts=10,retry_base_seconds=30,
    policy_digest=repeat('f',64),created_by=101
WHERE version_no='version-commerce-service-1'`},
	}
	for _, update := range updates {
		if _, err := database.Exec(ctx, update.sql, update.args...); err != nil {
			t.Fatalf("freeze server-owned fulfillment evidence: %v", err)
		}
	}
}

func (sink crossPluginProjectionSink) RefreshUnused(ctx context.Context, projection resourceinterfaceprojectioncap.Projection, apply func(context.Context) error) error {
	if err := apply(ctx); err != nil {
		return err
	}
	return sink.Upsert(ctx, projection)
}
