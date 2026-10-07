package commerce

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gogf/gf/v2/database/gdb"

	"lina-core/pkg/plugin/capability/capmodel"
	"lina-core/pkg/plugin/capability/plugincap"
	"lina-core/pkg/plugin/pluginhost"
	"lina-plugin-idc-core/backend/cap/productfulfillmentcap"
	billingservice "lina-plugin-idc-core/backend/internal/service/billing"
	"lina-plugin-idc-core/backend/internal/service/catalog"
	"lina-plugin-idc-core/backend/internal/service/fulfillment"
	"lina-plugin-idc-core/backend/internal/service/outbox"
	"lina-plugin-idc-core/backend/internal/service/productautomation"
	productfulfillmentservice "lina-plugin-idc-core/backend/internal/service/productfulfillment"
	"lina-plugin-idc-core/backend/internal/service/purchaseconfigurationfile"
)

type fulfillmentWorkerExtensions struct {
	mu          sync.Mutex
	invocations []pluginhost.BusinessExtensionMethodInvokeInput
}

// genericOnlyAddonConsumer keeps generic worker fixtures focused on ordinary
// purchases; paid accessory routing has its own transactional coverage.
type genericOnlyAddonConsumer struct{}

func (genericOnlyAddonConsumer) ConsumeAddonSettlement(context.Context, fulfillment.SettlementEvent) (*fulfillment.Delivery, bool, error) {
	return nil, false, nil
}

// structuredFulfillmentEvidenceValidator keeps these worker tests focused on
// the immutable generic order evidence handed to fulfillment. File ownership
// is verified by the dedicated purchase-configuration tests.
type structuredFulfillmentEvidenceValidator struct {
	files purchaseconfigurationfile.OrderEvidenceVerifier
}

func (v structuredFulfillmentEvidenceValidator) Validate(ctx context.Context, input productautomation.FrozenCustomerConfigurationInput) (*productautomation.FrozenCustomerConfigurationEvidence, error) {
	rows, err := v.ValidateBatch(ctx, []productautomation.FrozenCustomerConfigurationInput{input})
	if err != nil {
		return nil, err
	}
	return &rows[0], nil
}

func (structuredFulfillmentEvidenceValidator) ValidateBatch(_ context.Context, inputs []productautomation.FrozenCustomerConfigurationInput) ([]productautomation.FrozenCustomerConfigurationEvidence, error) {
	rows := make([]productautomation.FrozenCustomerConfigurationEvidence, len(inputs))
	for index, input := range inputs {
		rows[index] = productautomation.FrozenCustomerConfigurationEvidence{
			CustomerValueDigest:     input.CustomerValueDigest,
			CandidateSnapshotDigest: input.CandidateSnapshotDigest,
		}
	}
	return rows, nil
}

func fulfillmentSystemReasons(t *testing.T) productfulfillmentservice.SystemTransitionReasonCatalog {
	t.Helper()
	catalog, err := productfulfillmentservice.NewSystemTransitionReasonCatalog(map[string]productfulfillmentservice.SystemTransitionReason{
		productfulfillmentservice.SystemReasonOrderCancelled: {
			LabelZhCN: "订单已取消", LabelEnUS: "Order cancelled",
		},
		productfulfillmentservice.SystemReasonCancellationRequiresAttention: {
			LabelZhCN: "取消时供应商可能已受理，已停止自动处理",
			LabelEnUS: "The supplier may have accepted the request; automatic processing stopped",
		},
		productfulfillmentservice.SystemReasonNewBusinessStopped: {
			LabelZhCN: "插件已停止新业务，已转人工处理",
			LabelEnUS: "The plugin stopped new business; fulfillment moved to manual handling",
		},
	})
	if err != nil {
		t.Fatalf("create system reason catalog: %v", err)
	}
	return catalog
}

func (*fulfillmentWorkerExtensions) ListProviders(context.Context, plugincap.BusinessExtensionProviderListInput) (*capmodel.PageResult[*capmodel.BusinessExtensionProviderEvidence], error) {
	return nil, errors.New("not used")
}
func (*fulfillmentWorkerExtensions) GetProviderExact(context.Context, plugincap.BusinessExtensionProviderExactInput) (*capmodel.BusinessExtensionProviderEvidence, error) {
	return nil, errors.New("not used")
}
func (*fulfillmentWorkerExtensions) GetProvidersExact(context.Context, []plugincap.BusinessExtensionProviderExactInput) ([]*capmodel.BusinessExtensionProviderEvidence, error) {
	return nil, errors.New("not used")
}
func (*fulfillmentWorkerExtensions) WithNewBusinessAdmission(context.Context, pluginhost.BusinessExtensionAdmissionInput, capmodel.BusinessExtensionAdmissionAction) error {
	return errors.New("not used")
}
func (*fulfillmentWorkerExtensions) WithConfidentialOutboundContext(context.Context, pluginhost.ConfidentialOutboundContextInput, pluginhost.ConfidentialOutboundAction) error {
	return errors.New("not used")
}
func (e *fulfillmentWorkerExtensions) InvokeProviderMethod(_ context.Context, input pluginhost.BusinessExtensionMethodInvokeInput) (*capmodel.BusinessExtensionMethodResult, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.invocations = append(e.invocations, input)
	if bytes.Contains(input.Payload, []byte("CFU_")) || bytes.Contains(input.Payload, []byte("private/")) || bytes.Contains(input.Payload, []byte("http://")) || bytes.Contains(input.Payload, []byte("https://")) {
		return nil, errors.New("provider payload exposed a file token or storage locator")
	}
	request := &productfulfillmentcap.Request{}
	if err := json.Unmarshal(input.Payload, request); err != nil {
		return nil, err
	}
	result := &productfulfillmentcap.Result{Version: productfulfillmentcap.RequestVersion}
	if len(e.invocations) == 1 {
		result.Status = productfulfillmentcap.StatusNotFound
	} else {
		result.Status = productfulfillmentcap.StatusSucceeded
		result.ExternalReference = "supplier-order:1001"
		result.Evidence = json.RawMessage(`{"supplier":"accepted","reference":"1001"}`)
	}
	raw, err := json.Marshal(result)
	if err != nil {
		return nil, err
	}
	return &capmodel.BusinessExtensionMethodResult{Version: capmodel.BusinessExtensionInvocationProtocolV1, Payload: raw}, nil
}

type fulfillmentWorkerContexts struct {
	mu       sync.Mutex
	inputs   []productautomation.ExistingBusinessRuntimeContextInput
	releases int
}

func (c *fulfillmentWorkerContexts) IssueExistingBusinessRuntimeContext(_ context.Context, input productautomation.ExistingBusinessRuntimeContextInput) (*productautomation.ExistingBusinessRuntimeContext, error) {
	c.mu.Lock()
	c.inputs = append(c.inputs, input)
	c.mu.Unlock()
	return &productautomation.ExistingBusinessRuntimeContext{ContextID: "src_0123456789abcdef0123456789abcdef", Release: func() {
		c.mu.Lock()
		c.releases++
		c.mu.Unlock()
	}}, nil
}

type genericWorkerScenario struct {
	checkout checkoutTestScenario
	order    *Checkout
	service  productfulfillmentservice.Service
	settleNo string
}

func prepareGenericWorkerScenario(t *testing.T) genericWorkerScenario {
	return prepareGenericWorkerScenarioOptions(t, true, true)
}

func prepareGenericWorkerScenarioWithMaterialization(t *testing.T, materialize bool) genericWorkerScenario {
	return prepareGenericWorkerScenarioOptions(t, materialize, false)
}

func prepareGenericWorkerScenarioOptions(t *testing.T, materialize bool, ongoingManagement bool) genericWorkerScenario {
	return prepareGenericWorkerScenarioOptionsWithQuantity(t, materialize, ongoingManagement, 1)
}

func prepareGenericWorkerScenarioOptionsWithQuantity(t *testing.T, materialize bool, ongoingManagement bool, quantity int) genericWorkerScenario {
	return prepareGenericWorkerScenarioWithFrozenConfiguration(t, materialize, ongoingManagement, quantity, nil)
}

func prepareGenericWorkerScenarioWithFrozenConfiguration(
	t *testing.T,
	materialize bool,
	ongoingManagement bool,
	quantity int,
	freeze func(checkoutTestScenario, structuredQuoteItemFixture),
) genericWorkerScenario {
	t.Helper()
	scenario := newCheckoutTestScenarioWithSetup(t, "customer-commerce-service-1", 101, quantity, false, 0,
		func(ctx context.Context, database gdb.DB) {
			prepareStructuredCheckoutDatabase(t, ctx, database)
			renewalMode := "unavailable"
			entitlementOwnerID := ""
			if ongoingManagement {
				renewalMode = "local_entitlement"
				entitlementOwnerID = "phone-fulfillment"
			}
			if _, err := database.Exec(ctx, `ALTER TABLE plugin_idc_core_catalog_product_version_fulfillment_policy DISABLE TRIGGER trg_idc_guard_fulfillment_policy`); err != nil {
				t.Fatal(err)
			}
			if _, err := database.Exec(ctx, `UPDATE plugin_idc_core_catalog_product_version_fulfillment_policy SET strategy_code='payment_confirmed_plugin',granularity_code='per_order_item',ongoing_management=?,renewal_mode_code=?,entitlement_owner_id=?,provision_method='provision',query_method='query',maximum_attempts=10,retry_base_seconds=30,policy_digest=repeat('f',64),created_by=101 WHERE tenant_id=0 AND version_no='version-commerce-service-1'`, ongoingManagement, renewalMode, entitlementOwnerID); err != nil {
				t.Fatal(err)
			}
			if _, err := database.Exec(ctx, `ALTER TABLE plugin_idc_core_catalog_product_version_fulfillment_policy ENABLE TRIGGER trg_idc_guard_fulfillment_policy`); err != nil {
				t.Fatal(err)
			}
		})
	quoteItem := loadStructuredQuoteItem(t, scenario, scenario.quote.QuoteNo)
	seedStructuredCheckoutEvidence(t, scenario, quoteItem.QuoteItemNo, quoteItem.ProductNo, quoteItem.VersionNo)
	if freeze != nil {
		freeze(scenario, quoteItem)
	}
	if quoteItem.VersionNo != "version-commerce-service-1" {
		t.Fatalf("unexpected structured version %q", quoteItem.VersionNo)
	}
	consumer, err := purchaseconfigurationfile.NewOrderFileConsumer(scenario.database, scenario.service.principal)
	if err != nil {
		t.Fatal(err)
	}
	if err = BindStructuredConfigurationSnapshots(scenario.service, structuredCheckoutFreezerStub{}, consumer); err != nil {
		t.Fatal(err)
	}
	insertVerifiedIndividualPaymentIdentity(t, scenario, "customer-commerce-service-1")
	checkout, err := scenario.service.CheckoutAccountQuote(scenario.ctx, CheckoutRequest{QuoteNo: scenario.quote.QuoteNo, IdempotencyKey: "generic-worker-checkout-0001"})
	if err != nil {
		t.Fatal(err)
	}
	seedFinancialSettlementMerchant(t, scenario)
	balance, _ := billingservice.NewBalanceService(scenario.database)
	if _, err = balance.PostConfirmedRecharge(scenario.ctx, billingservice.ConfirmedRechargeInput{CustomerNo: "customer-commerce-service-1", CurrencyCode: "CNY", AmountMinor: checkout.TotalAmountMinor, SourceReference: "generic-worker-recharge", SourceSnapshotDigest: strings.Repeat("d", 64)}); err != nil {
		t.Fatal(err)
	}
	invoices, _ := billingservice.NewInvoiceService(scenario.database)
	if _, err = invoices.PayFromBalance(scenario.ctx, billingservice.InvoiceBalancePaymentInput{InvoiceNo: checkout.InvoiceNo, SourceReference: "generic-worker-payment", SourceSnapshotDigest: strings.Repeat("e", 64)}); err != nil {
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
	service, err := productfulfillmentservice.NewFullConfigured(scenario.database, structuredFulfillmentEvidenceValidator{files: files}, fulfillmentGateStub{}, fulfillmentSystemReasons(t), fulfillmentAuthorizerStub{}, fulfillmentAuditStub{}, fulfillmentActorsStub{}, []byte("0123456789abcdef0123456789abcdef"))
	if err != nil {
		t.Fatal(err)
	}
	if materialize {
		materialized, consumeErr := service.ConsumeFinanciallySettled(scenario.ctx, productfulfillmentservice.SettlementInput{SettlementNo: settlement.SettlementNo, OrderNo: checkout.OrderNo})
		if consumeErr != nil || materialized.TaskCount != 1 {
			t.Fatalf("materialized=%+v err=%v", materialized, consumeErr)
		}
	}
	return genericWorkerScenario{checkout: scenario, order: checkout, service: service, settleNo: settlement.SettlementNo}
}

func TestGenericDeliveryReconcilesPartialTerminalResultAndCreatesSuccessfulEntitlementPostgreSQL(t *testing.T) {
	prepared := prepareGenericWorkerScenarioOptions(t, true, true)
	database, ctx := prepared.checkout.database, prepared.checkout.ctx
	var first struct {
		IntentNo    string `orm:"intent_no"`
		OrderItemNo string `orm:"order_item_no"`
	}
	if err := database.GetScan(ctx, &first, `SELECT intent_no,order_item_no
FROM plugin_idc_core_product_fulfillment_intent WHERE order_no=?`, prepared.order.OrderNo); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(ctx, `UPDATE plugin_idc_core_product_delivery_record SET quantity=2 WHERE order_item_no=?`, first.OrderItemNo); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(ctx, `ALTER TABLE plugin_idc_core_product_fulfillment_intent DISABLE TRIGGER trg_idc_guard_fulfillment_intent`); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(ctx, `ALTER TABLE plugin_idc_core_product_fulfillment_intent DISABLE TRIGGER trg_idc_check_fulfillment_intent_terminal`); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(ctx, `UPDATE plugin_idc_core_product_fulfillment_intent
SET quantity=2,granularity_code='per_quantity_unit' WHERE intent_no=?`, first.IntentNo); err != nil {
		t.Fatal(err)
	}
	secondIntentNo := "intent-partial-terminal-2"
	if _, err := database.Exec(ctx, `INSERT INTO plugin_idc_core_product_fulfillment_intent(
tenant_id,intent_no,settlement_no,order_no,order_item_no,customer_no,invoice_no,currency_code,
product_no,version_no,product_name_zh_cn,product_name_en_us,quantity,unit_sequence,strategy_code,
granularity_code,initial_status_code,status_code,configuration_snapshot_no,interface_instance_no,
interface_configuration_revision,plugin_id,plugin_release_version,contract_version,release_digest,
declaration_digest,provision_method,query_method,maximum_attempts,retry_base_seconds,policy_digest,
evidence_digest,financially_settled_at,ongoing_management,renewal_mode_code,renewal_method,entitlement_owner_id)
SELECT tenant_id,?,settlement_no,order_no,order_item_no,customer_no,invoice_no,currency_code,
product_no,version_no,product_name_zh_cn,product_name_en_us,2,2,strategy_code,
'per_quantity_unit',initial_status_code,'queued',configuration_snapshot_no,interface_instance_no,
interface_configuration_revision,plugin_id,plugin_release_version,contract_version,release_digest,
declaration_digest,provision_method,query_method,maximum_attempts,retry_base_seconds,policy_digest,
repeat('9',64),financially_settled_at,TRUE,renewal_mode_code,renewal_method,entitlement_owner_id
FROM plugin_idc_core_product_fulfillment_intent WHERE intent_no=?`, secondIntentNo, first.IntentNo); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(ctx, `UPDATE plugin_idc_core_product_fulfillment_intent SET status_code='completed' WHERE intent_no=?`, first.IntentNo); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(ctx, `UPDATE plugin_idc_core_product_fulfillment_intent SET status_code='canceled' WHERE intent_no=?`, secondIntentNo); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(ctx, `ALTER TABLE plugin_idc_core_product_fulfillment_intent ENABLE TRIGGER trg_idc_guard_fulfillment_intent`); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(ctx, `ALTER TABLE plugin_idc_core_product_fulfillment_intent ENABLE TRIGGER trg_idc_check_fulfillment_intent_terminal`); err != nil {
		t.Fatal(err)
	}
	var evidence struct {
		DeliveryStatus string `orm:"delivery_status"`
		Entitlements   int    `orm:"entitlements"`
	}
	if err := database.GetScan(ctx, &evidence, `SELECT delivery.status_code AS delivery_status,
(SELECT count(*) FROM plugin_idc_core_service_entitlement entitlement
 WHERE entitlement.delivery_no=delivery.delivery_no AND entitlement.source_intent_no=?) AS entitlements
FROM plugin_idc_core_product_delivery_record delivery WHERE delivery.order_item_no=?`, first.IntentNo, first.OrderItemNo); err != nil {
		t.Fatal(err)
	}
	if evidence.DeliveryStatus != "incomplete" || evidence.Entitlements != 1 {
		t.Fatalf("partial delivery evidence=%+v", evidence)
	}
}

func TestFinancialSettlementOutboxUsesOnlyGenericOwnerForPolicyItemPostgreSQL(t *testing.T) {
	prepared := prepareGenericWorkerScenarioWithMaterialization(t, false)
	scenario := prepared.checkout
	dispatcher, err := outbox.NewService(scenario.database, outbox.Config{LeaseDuration: time.Minute, MaximumClaimBatch: 10})
	if err != nil {
		t.Fatal(err)
	}
	routed, err := fulfillment.NewRoutedConsumer(prepared.service, genericOnlyAddonConsumer{})
	if err != nil {
		t.Fatal(err)
	}
	worker, err := fulfillment.NewWorker(dispatcher, routed)
	if err != nil {
		t.Fatal(err)
	}
	report, err := worker.Run(scenario.ctx, 1)
	if err != nil || report.Claimed != 1 || report.Published != 1 || report.Failed != 0 {
		t.Fatalf("outbox report=%+v err=%v", report, err)
	}
	var evidence struct{ Intents, Tasks, Deliveries, Entitlements, LegacyServices, LegacyOperations int }
	if err = scenario.database.GetScan(scenario.ctx, &evidence, `SELECT (SELECT count(*) FROM plugin_idc_core_product_fulfillment_intent WHERE order_no=?) intents,(SELECT count(*) FROM plugin_idc_core_product_fulfillment_task task JOIN plugin_idc_core_product_fulfillment_intent intent ON intent.intent_no=task.intent_no WHERE intent.order_no=?) tasks,(SELECT count(*) FROM plugin_idc_core_product_delivery_record WHERE order_no=?) deliveries,(SELECT count(*) FROM plugin_idc_core_service_entitlement WHERE order_no=?) entitlements,(SELECT count(*) FROM plugin_idc_core_service_instance WHERE order_no=?) legacy_services,(SELECT count(*) FROM plugin_idc_core_business_operation WHERE order_no=?) legacy_operations`, prepared.order.OrderNo, prepared.order.OrderNo, prepared.order.OrderNo, prepared.order.OrderNo, prepared.order.OrderNo, prepared.order.OrderNo); err != nil || evidence.Intents != 1 || evidence.Tasks != 1 || evidence.Deliveries != 1 || evidence.Entitlements != 0 || evidence.LegacyServices != 0 || evidence.LegacyOperations != 0 {
		t.Fatalf("owner routing evidence=%+v err=%v", evidence, err)
	}
	replayed, err := worker.Run(scenario.ctx, 1)
	if err != nil || replayed.Claimed != 0 {
		t.Fatalf("published event was claimed twice: %+v err=%v", replayed, err)
	}
}

func TestFinancialSettlementOutboxRoutesTwoGenericItemsExactlyOncePostgreSQL(t *testing.T) {
	scenario := newCheckoutTestScenario(t, "customer-commerce-service-1", 101, 1, false, 0)
	reader, err := catalog.NewPublicCatalogReader(scenario.database, []byte("0123456789abcdef0123456789abcdef"))
	if err != nil {
		t.Fatal(err)
	}
	bindTestPublicBillingLabels(t, reader)
	configuration, err := reader.GetPurchaseConfiguration(scenario.ctx, "generic-phone")
	if err != nil {
		t.Fatal(err)
	}
	quoted, err := reader.QuotePurchaseConfiguration(scenario.ctx, catalog.PublicPurchaseConfigurationQuoteInput{
		PublicPurchaseConfigurationInput: catalog.PublicPurchaseConfigurationInput{PublicSlug: "generic-phone", ConfigurationRevision: configuration.ConfigurationRevision},
		CurrencyCode:                     "CNY",
	})
	if err != nil || len(quoted.Terms) != 1 {
		t.Fatalf("second generic quote=%+v err=%v", quoted, err)
	}
	cart, err := scenario.service.GetAccountCart(scenario.ctx)
	if err != nil {
		t.Fatal(err)
	}
	cart, err = scenario.service.AddAccountItem(scenario.ctx, AddItemInput{ExpectedCartVersion: cart.Version, Quantity: 1, Purchase: catalog.PurchaseConfigurationInput{PublicSlug: "generic-phone", ConfigurationRevision: configuration.ConfigurationRevision, BillingTermToken: quoted.Terms[0].BillingTermToken, CurrencyCode: "CNY", PurchaseCycles: 1}})
	if err != nil {
		t.Fatal(err)
	}
	mixedQuote, err := scenario.service.QuoteAccountCart(scenario.ctx, QuoteRequest{ExpectedCartVersion: cart.Version})
	if err != nil || len(mixedQuote.Items) != 2 {
		t.Fatalf("mixed quote=%+v err=%v", mixedQuote, err)
	}
	insertVerifiedIndividualPaymentIdentity(t, scenario, "customer-commerce-service-1")
	checkout, err := scenario.service.CheckoutAccountQuote(scenario.ctx, CheckoutRequest{QuoteNo: mixedQuote.QuoteNo, IdempotencyKey: "mixed-owner-checkout-0001"})
	if err != nil {
		t.Fatal(err)
	}
	seedFinancialSettlementMerchant(t, scenario)
	balance, _ := billingservice.NewBalanceService(scenario.database)
	if _, err = balance.PostConfirmedRecharge(scenario.ctx, billingservice.ConfirmedRechargeInput{CustomerNo: "customer-commerce-service-1", CurrencyCode: "CNY", AmountMinor: checkout.TotalAmountMinor, SourceReference: "mixed-owner-recharge", SourceSnapshotDigest: strings.Repeat("6", 64)}); err != nil {
		t.Fatal(err)
	}
	invoices, _ := billingservice.NewInvoiceService(scenario.database)
	if _, err = invoices.PayFromBalance(scenario.ctx, billingservice.InvoiceBalancePaymentInput{InvoiceNo: checkout.InvoiceNo, SourceReference: "mixed-owner-payment", SourceSnapshotDigest: strings.Repeat("7", 64)}); err != nil {
		t.Fatal(err)
	}
	settlement, err := newFinancialSettlementTestService(t, scenario).SettleOrder(scenario.ctx, checkout.OrderNo)
	if err != nil {
		t.Fatal(err)
	}
	generic, err := productfulfillmentservice.NewSettlementConsumer(scenario.database, structuredFulfillmentEvidenceValidator{}, fulfillmentGateStub{})
	if err != nil {
		t.Fatal(err)
	}
	routed, err := fulfillment.NewRoutedConsumer(generic, genericOnlyAddonConsumer{})
	if err != nil {
		t.Fatal(err)
	}
	dispatcher, err := outbox.NewService(scenario.database, outbox.Config{LeaseDuration: time.Minute, MaximumClaimBatch: 10})
	if err != nil {
		t.Fatal(err)
	}
	worker, err := fulfillment.NewWorker(dispatcher, routed)
	if err != nil {
		t.Fatal(err)
	}
	report, err := worker.Run(scenario.ctx, 1)
	if err != nil || report.Published != 1 {
		failure, _ := scenario.database.GetValue(scenario.ctx, `SELECT last_error_code||':'||last_error_summary FROM plugin_idc_core_outbox_event WHERE aggregate_no=? ORDER BY id DESC LIMIT 1`, checkout.OrderNo)
		t.Fatalf("mixed route report=%+v settlement=%+v err=%v failure=%s", report, settlement, err, failure.String())
	}
	var evidence struct {
		OriginalIntents, PhoneIntents, LegacyServices, LegacyOperations int
	}
	if err = scenario.database.GetScan(scenario.ctx, &evidence, `SELECT
  (SELECT count(*) FROM plugin_idc_core_product_fulfillment_intent WHERE order_no=? AND product_no='product-commerce-service') original_intents,
  (SELECT count(*) FROM plugin_idc_core_product_fulfillment_intent WHERE order_no=? AND product_no='prd_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa') phone_intents,
  (SELECT count(*) FROM plugin_idc_core_service_instance WHERE order_no=?) legacy_services,
  (SELECT count(*) FROM plugin_idc_core_business_operation WHERE order_no=?) legacy_operations`, checkout.OrderNo, checkout.OrderNo, checkout.OrderNo, checkout.OrderNo); err != nil || evidence.OriginalIntents != 1 || evidence.PhoneIntents != 1 || evidence.LegacyServices != 0 || evidence.LegacyOperations != 0 {
		t.Fatalf("mixed owner evidence=%+v err=%v", evidence, err)
	}
}

func TestManualConsultingProductCompletesWithoutPluginOrResourceCallsPostgreSQL(t *testing.T) {
	scenario := newCheckoutTestScenario(t, "customer-commerce-service-1", 101, 2, false, 0)
	insertVerifiedIndividualPaymentIdentity(t, scenario, "customer-commerce-service-1")
	checkout, err := scenario.service.CheckoutAccountQuote(scenario.ctx, CheckoutRequest{
		QuoteNo: scenario.quote.QuoteNo, IdempotencyKey: "manual-consulting-checkout-0001",
	})
	if err != nil {
		t.Fatal(err)
	}
	seedFinancialSettlementMerchant(t, scenario)
	balance, err := billingservice.NewBalanceService(scenario.database)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = balance.PostConfirmedRecharge(scenario.ctx, billingservice.ConfirmedRechargeInput{
		CustomerNo: "customer-commerce-service-1", CurrencyCode: "CNY",
		AmountMinor: checkout.TotalAmountMinor, SourceReference: "manual-consulting-recharge",
		SourceSnapshotDigest: strings.Repeat("8", 64),
	}); err != nil {
		t.Fatal(err)
	}
	invoices, err := billingservice.NewInvoiceService(scenario.database)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = invoices.PayFromBalance(scenario.ctx, billingservice.InvoiceBalancePaymentInput{
		InvoiceNo: checkout.InvoiceNo, SourceReference: "manual-consulting-payment",
		SourceSnapshotDigest: strings.Repeat("9", 64),
	}); err != nil {
		t.Fatal(err)
	}
	settlement, err := newFinancialSettlementTestService(t, scenario).SettleOrder(scenario.ctx, checkout.OrderNo)
	if err != nil {
		t.Fatal(err)
	}
	service, err := productfulfillmentservice.NewFullConfigured(
		scenario.database, structuredFulfillmentEvidenceValidator{}, fulfillmentGateStub{},
		fulfillmentSystemReasons(t), fulfillmentAuthorizerStub{}, fulfillmentAuditStub{},
		fulfillmentActorsStub{}, []byte("0123456789abcdef0123456789abcdef"),
	)
	if err != nil {
		t.Fatal(err)
	}
	materialized, err := service.ConsumeFinanciallySettled(scenario.ctx, productfulfillmentservice.SettlementInput{
		SettlementNo: settlement.SettlementNo, OrderNo: checkout.OrderNo,
	})
	if err != nil || materialized.IntentCount != 1 || materialized.ManualCount != 1 ||
		materialized.TaskCount != 0 || materialized.ReviewCount != 0 {
		t.Fatalf("manual consulting materialization=%+v err=%v", materialized, err)
	}

	extensions := &fulfillmentWorkerExtensions{}
	contexts := &fulfillmentWorkerContexts{}
	sealer, err := productfulfillmentservice.NewProviderEvidenceSealer(
		"test-key-v1", []byte("0123456789abcdef0123456789abcdef"),
	)
	if err != nil {
		t.Fatal(err)
	}
	worker, err := productfulfillmentservice.NewWorker(
		scenario.database, extensions, contexts, sealer, fulfillmentSystemReasons(t), nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	batch, err := worker.Run(scenario.ctx, 8)
	if err != nil || batch.Selected != 0 {
		t.Fatalf("manual product crossed automated worker: batch=%+v err=%v", batch, err)
	}
	extensions.mu.Lock()
	invocations := len(extensions.invocations)
	extensions.mu.Unlock()
	contexts.mu.Lock()
	runtimeContexts := len(contexts.inputs)
	contexts.mu.Unlock()
	if invocations != 0 || runtimeContexts != 0 {
		t.Fatalf("manual product called plugin runtime: invocations=%d contexts=%d", invocations, runtimeContexts)
	}

	var intent struct {
		IntentNo string
		Version  int64
	}
	if err = scenario.database.GetScan(scenario.ctx, &intent, `SELECT intent_no,version
FROM plugin_idc_core_product_fulfillment_intent WHERE order_no=?`, checkout.OrderNo); err != nil {
		t.Fatal(err)
	}
	completed, err := service.ConcludeManual(scenario.ctx, productfulfillmentservice.ManualConclusionInput{
		ActionInput: productfulfillmentservice.ActionInput{
			IntentNo: intent.IntentNo, ExpectedVersion: intent.Version,
			IdempotencyKey: "manual-consulting-conclusion-0001",
			ReasonCode:     "manual_fulfillment_completed", Note: "咨询服务已经按约定全部交付完成",
		},
		Outcome: "completed",
	})
	if err != nil || completed == nil || completed.StatusCode != "completed" {
		t.Fatalf("manual consulting conclusion=%+v err=%v", completed, err)
	}
	var evidence struct {
		DeliveryStatus                                                   string
		AutomatedTasks, Entitlements, ConfigurationSnapshots, Interfaces int
	}
	if err = scenario.database.GetScan(scenario.ctx, &evidence, `SELECT
delivery.status_code AS delivery_status,
(SELECT count(*) FROM plugin_idc_core_product_fulfillment_task task JOIN plugin_idc_core_product_fulfillment_intent child ON child.intent_no=task.intent_no WHERE child.order_no=?) AS automated_tasks,
(SELECT count(*) FROM plugin_idc_core_service_entitlement WHERE order_no=?) AS entitlements,
(SELECT count(*) FROM plugin_idc_core_commerce_structured_configuration_snapshot WHERE order_no=?) AS configuration_snapshots,
(SELECT count(*) FROM plugin_idc_core_product_automation_interface_instance interface JOIN plugin_idc_core_catalog_product_version_fulfillment_binding binding ON binding.interface_instance_no=interface.interface_instance_no WHERE binding.version_no='version-commerce-service-1') AS interfaces
FROM plugin_idc_core_product_delivery_record delivery WHERE delivery.order_no=?`,
		checkout.OrderNo, checkout.OrderNo, checkout.OrderNo, checkout.OrderNo); err != nil ||
		evidence.DeliveryStatus != "completed" || evidence.AutomatedTasks != 0 ||
		evidence.Entitlements != 0 || evidence.ConfigurationSnapshots != 0 || evidence.Interfaces != 0 {
		t.Fatalf("manual consulting evidence=%+v err=%v", evidence, err)
	}
}

func TestProductFulfillmentWorkerUsesFixedQueryFirstPolicyPostgreSQL(t *testing.T) {
	prepared := prepareGenericWorkerScenario(t)
	scenario, checkout, service := prepared.checkout, prepared.order, prepared.service
	if _, err := scenario.database.Exec(scenario.ctx, `UPDATE plugin_idc_core_product_automation_interface_instance SET new_business_state='stopped',version=version+1 WHERE interface_instance_no='pfi_11111111111111111111111111111111'`); err != nil {
		t.Fatal(err)
	}
	replayed, err := service.ConsumeFinanciallySettled(scenario.ctx, productfulfillmentservice.SettlementInput{SettlementNo: prepared.settleNo, OrderNo: checkout.OrderNo})
	if err != nil || !replayed.Reused || replayed.TaskCount != 0 {
		t.Fatalf("settlement replay after stop=%+v err=%v", replayed, err)
	}
	extensions := &fulfillmentWorkerExtensions{}
	contexts := &fulfillmentWorkerContexts{}
	sealer, err := productfulfillmentservice.NewProviderEvidenceSealer("test-key-v1", []byte("0123456789abcdef0123456789abcdef"))
	if err != nil {
		t.Fatal(err)
	}
	worker, err := productfulfillmentservice.NewWorker(scenario.database, extensions, contexts, sealer, fulfillmentSystemReasons(t), nil)
	if err != nil {
		t.Fatal(err)
	}
	first, err := worker.Run(scenario.ctx, 1)
	if err != nil || first.Selected != 1 || first.Scheduled != 1 {
		t.Fatalf("query batch=%+v err=%v", first, err)
	}
	delayValue, delayErr := scenario.database.GetValue(scenario.ctx, `SELECT EXTRACT(EPOCH FROM (next_attempt_at-CURRENT_TIMESTAMP)) FROM plugin_idc_core_product_fulfillment_task WHERE intent_no IN(SELECT intent_no FROM plugin_idc_core_product_fulfillment_intent WHERE order_no=?)`, checkout.OrderNo)
	delaySeconds := delayValue.Float64()
	if delayErr != nil || delaySeconds < 29 {
		t.Fatalf("fixed retry delay=%v err=%v", delaySeconds, delayErr)
	}
	advanceFulfillmentTaskClock(t, scenario, checkout.OrderNo)
	second, err := worker.Run(scenario.ctx, 1)
	if err != nil || second.Selected != 1 || second.Completed != 1 {
		t.Fatalf("submit batch=%+v err=%v", second, err)
	}
	contexts.mu.Lock()
	if len(contexts.inputs) != 2 || contexts.releases != 2 {
		t.Fatalf("runtime contexts=%d releases=%d", len(contexts.inputs), contexts.releases)
	}
	for _, input := range contexts.inputs {
		if input.IntentNo == "" || input.OrderNo != checkout.OrderNo || input.OrderItemNo == "" || input.ConfigurationSnapshotNo != "SCF_33333333333333333333333333333333" {
			t.Fatalf("runtime scope=%+v", input)
		}
	}
	contexts.mu.Unlock()
	var evidence struct {
		IntentStatus, TaskStatus                                                   string
		Attempts, QueryAttempts, SubmitAttempts, Private, Deliveries, Entitlements int
	}
	if err = scenario.database.GetScan(scenario.ctx, &evidence, `SELECT intent.status_code intent_status,task.status_code task_status,task.attempt_count attempts,task.query_attempt_count query_attempts,task.submit_attempt_count submit_attempts,(SELECT count(*) FROM plugin_idc_core_product_fulfillment_attempt_provider_evidence protected WHERE protected.task_no=task.task_no) private,(SELECT count(*) FROM plugin_idc_core_product_delivery_record delivery WHERE delivery.order_no=intent.order_no AND delivery.status_code='completed') deliveries,(SELECT count(*) FROM plugin_idc_core_service_entitlement entitlement WHERE entitlement.order_no=intent.order_no AND entitlement.source_intent_no=intent.intent_no AND entitlement.status_code='active') entitlements FROM plugin_idc_core_product_fulfillment_intent intent JOIN plugin_idc_core_product_fulfillment_task task ON task.intent_no=intent.intent_no WHERE intent.order_no=?`, checkout.OrderNo); err != nil || evidence.IntentStatus != "completed" || evidence.TaskStatus != "succeeded" || evidence.Attempts != 2 || evidence.QueryAttempts != 1 || evidence.SubmitAttempts != 1 || evidence.Private != 1 || evidence.Deliveries != 1 || evidence.Entitlements != 1 {
		t.Fatalf("worker evidence=%+v err=%v", evidence, err)
	}
	replayBatch, err := worker.Run(scenario.ctx, 1)
	if err != nil || replayBatch.Selected != 0 {
		t.Fatalf("completed task was selected twice: batch=%+v err=%v", replayBatch, err)
	}
	if count, countErr := scenario.database.GetCount(scenario.ctx, `SELECT 1 FROM plugin_idc_core_service_entitlement WHERE order_no=?`, checkout.OrderNo); countErr != nil || count != 1 {
		t.Fatalf("entitlement replay count=%d err=%v", count, countErr)
	}
}

func TestProductFulfillmentWorkerDoesNotCreateEntitlementForDeliveryOnlyProductPostgreSQL(t *testing.T) {
	prepared := prepareGenericWorkerScenarioOptions(t, true, false)
	extensions := &fulfillmentWorkerExtensions{}
	contexts := &fulfillmentWorkerContexts{}
	sealer, err := productfulfillmentservice.NewProviderEvidenceSealer("test-key-v1", []byte("0123456789abcdef0123456789abcdef"))
	if err != nil {
		t.Fatal(err)
	}
	worker, err := productfulfillmentservice.NewWorker(prepared.checkout.database, extensions, contexts, sealer, fulfillmentSystemReasons(t), nil)
	if err != nil {
		t.Fatal(err)
	}
	first, err := worker.Run(prepared.checkout.ctx, 1)
	if err != nil || first.Scheduled != 1 {
		t.Fatalf("query batch=%+v err=%v", first, err)
	}
	advanceFulfillmentTaskClock(t, prepared.checkout, prepared.order.OrderNo)
	second, err := worker.Run(prepared.checkout.ctx, 1)
	if err != nil || second.Completed != 1 {
		t.Fatalf("submit batch=%+v err=%v", second, err)
	}
	var evidence struct{ Deliveries, Entitlements int }
	if err = prepared.checkout.database.GetScan(prepared.checkout.ctx, &evidence, `SELECT (SELECT count(*) FROM plugin_idc_core_product_delivery_record WHERE order_no=? AND status_code='completed') deliveries,(SELECT count(*) FROM plugin_idc_core_service_entitlement WHERE order_no=?) entitlements`, prepared.order.OrderNo, prepared.order.OrderNo); err != nil || evidence.Deliveries != 1 || evidence.Entitlements != 0 {
		t.Fatalf("delivery-only evidence=%+v err=%v", evidence, err)
	}
}

func TestProductFulfillmentWorkerTwentyConcurrentClaimsInvokeOncePostgreSQL(t *testing.T) {
	prepared := prepareGenericWorkerScenario(t)
	extensions := &fulfillmentWorkerExtensions{}
	contexts := &fulfillmentWorkerContexts{}
	sealer, err := productfulfillmentservice.NewProviderEvidenceSealer("test-key-v1", []byte("0123456789abcdef0123456789abcdef"))
	if err != nil {
		t.Fatal(err)
	}
	worker, err := productfulfillmentservice.NewWorker(prepared.checkout.database, extensions, contexts, sealer, fulfillmentSystemReasons(t), nil)
	if err != nil {
		t.Fatal(err)
	}
	start := make(chan struct{})
	errorsSeen := make(chan error, 20)
	selected := make(chan int, 20)
	var wait sync.WaitGroup
	for range 20 {
		wait.Add(1)
		go func() {
			defer wait.Done()
			<-start
			report, callErr := worker.Run(prepared.checkout.ctx, 1)
			errorsSeen <- callErr
			if report != nil {
				selected <- report.Selected
			}
		}()
	}
	close(start)
	wait.Wait()
	close(errorsSeen)
	close(selected)
	for callErr := range errorsSeen {
		if callErr != nil {
			t.Fatal(callErr)
		}
	}
	totalSelected := 0
	for count := range selected {
		totalSelected += count
	}
	extensions.mu.Lock()
	invocations := len(extensions.invocations)
	extensions.mu.Unlock()
	if totalSelected != 1 || invocations != 1 {
		t.Fatalf("selected=%d provider invocations=%d", totalSelected, invocations)
	}
	attemptValue, attemptErr := prepared.checkout.database.GetValue(prepared.checkout.ctx, `SELECT attempt_count FROM plugin_idc_core_product_fulfillment_task task JOIN plugin_idc_core_product_fulfillment_intent intent ON intent.intent_no=task.intent_no WHERE intent.order_no=?`, prepared.order.OrderNo)
	attempts := attemptValue.Int()
	if attemptErr != nil || attempts != 1 {
		t.Fatalf("attempts=%d err=%v", attempts, attemptErr)
	}
}

func TestProductFulfillmentWorkerRecoversExpiredSubmitWithoutRepeatingPostgreSQL(t *testing.T) {
	prepared := prepareGenericWorkerScenario(t)
	scenario := prepared.checkout
	extensions := &fulfillmentWorkerExtensions{}
	contexts := &fulfillmentWorkerContexts{}
	sealer, err := productfulfillmentservice.NewProviderEvidenceSealer("test-key-v1", []byte("0123456789abcdef0123456789abcdef"))
	if err != nil {
		t.Fatal(err)
	}
	worker, err := productfulfillmentservice.NewWorker(scenario.database, extensions, contexts, sealer, fulfillmentSystemReasons(t), nil)
	if err != nil {
		t.Fatal(err)
	}
	first, err := worker.Run(scenario.ctx, 1)
	if err != nil || first.Selected != 1 || first.Scheduled != 1 {
		t.Fatalf("query batch=%+v err=%v", first, err)
	}
	advanceFulfillmentTaskClock(t, scenario, prepared.order.OrderNo)
	if err = scenario.database.Transaction(scenario.ctx, func(txCtx context.Context, _ gdb.TX) error {
		var task *struct {
			TaskNo  string
			Version int64
		}
		if loadErr := scenario.database.GetScan(txCtx, &task, `SELECT task_no,version FROM plugin_idc_core_product_fulfillment_task WHERE tenant_id=0 AND status_code='pending_submit' LIMIT 1 FOR UPDATE`); loadErr != nil || task == nil {
			return loadErr
		}
		updated, updateErr := scenario.database.Exec(txCtx, `UPDATE plugin_idc_core_product_fulfillment_task SET status_code='submitting',lease_token='pfl_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa',lease_expires_at=CURRENT_TIMESTAMP+INTERVAL '0.2 seconds',attempt_count=attempt_count+1,submit_attempt_count=submit_attempt_count+1,version=version+1,updated_at=CURRENT_TIMESTAMP WHERE task_no=? AND version=?`, task.TaskNo, task.Version)
		changed := int64(0)
		if updateErr == nil {
			changed, _ = updated.RowsAffected()
		}
		if updateErr != nil || changed != 1 {
			return updateErr
		}
		_, insertErr := scenario.database.Exec(txCtx, `INSERT INTO plugin_idc_core_product_fulfillment_attempt(tenant_id,task_no,attempt_no,action_code,started_at,evidence_digest) VALUES(0,?,2,'submit',CURRENT_TIMESTAMP,repeat('a',64))`, task.TaskNo)
		return insertErr
	}); err != nil {
		t.Fatal(err)
	}
	time.Sleep(300 * time.Millisecond)
	recovered, err := worker.Run(scenario.ctx, 1)
	if err != nil || recovered.Selected != 0 {
		t.Fatalf("recovery batch=%+v err=%v", recovered, err)
	}
	extensions.mu.Lock()
	invocations := len(extensions.invocations)
	extensions.mu.Unlock()
	var evidence struct {
		TaskStatus, IntentStatus, ResultCode string
		Finished                             bool
	}
	if err = scenario.database.GetScan(scenario.ctx, &evidence, `SELECT task.status_code task_status,intent.status_code intent_status,attempt.result_code,(attempt.finished_at IS NOT NULL) finished FROM plugin_idc_core_product_fulfillment_task task JOIN plugin_idc_core_product_fulfillment_intent intent ON intent.intent_no=task.intent_no JOIN plugin_idc_core_product_fulfillment_attempt attempt ON attempt.task_no=task.task_no AND attempt.attempt_no=2 WHERE intent.order_no=?`, prepared.order.OrderNo); err != nil || evidence.TaskStatus != "result_unknown" || evidence.IntentStatus != "processing" || evidence.ResultCode != "worker_interrupted" || !evidence.Finished || invocations != 1 {
		t.Fatalf("expired submit evidence=%+v invocations=%d err=%v", evidence, invocations, err)
	}
}

func TestProductFulfillmentCancellationDuringExpiredSubmitRequiresAttentionPostgreSQL(t *testing.T) {
	prepared := prepareGenericWorkerScenario(t)
	scenario := prepared.checkout
	extensions := &fulfillmentWorkerExtensions{}
	contexts := &fulfillmentWorkerContexts{}
	sealer, err := productfulfillmentservice.NewProviderEvidenceSealer("test-key-v1", []byte("0123456789abcdef0123456789abcdef"))
	if err != nil {
		t.Fatal(err)
	}
	worker, err := productfulfillmentservice.NewWorker(scenario.database, extensions, contexts, sealer, fulfillmentSystemReasons(t), nil)
	if err != nil {
		t.Fatal(err)
	}
	first, err := worker.Run(scenario.ctx, 1)
	if err != nil || first.Selected != 1 || first.Scheduled != 1 {
		t.Fatalf("query batch=%+v err=%v", first, err)
	}
	advanceFulfillmentTaskClock(t, scenario, prepared.order.OrderNo)
	if err = seedExpiredSubmittingAttempt(scenario); err != nil {
		t.Fatal(err)
	}
	if _, err = scenario.database.Exec(scenario.ctx, `INSERT INTO plugin_idc_core_commerce_order_cancellation(tenant_id,cancellation_no,order_no,customer_no,source_code,reason_code,request_digest,status_code,requested_by,completed_at) VALUES(0,'can_worker_expired_submit',?,'customer-commerce-service-1','admin','manual_rejected',repeat('e',64),'completed',101,CURRENT_TIMESTAMP)`, prepared.order.OrderNo); err != nil {
		t.Fatal(err)
	}
	cancelled, err := prepared.service.ConsumeOrderCancelled(scenario.ctx, productfulfillmentservice.OrderCancellationInput{OrderNo: prepared.order.OrderNo, CancellationNo: "can_worker_expired_submit"})
	if err != nil || cancelled.AwaitingQuery != 1 {
		t.Fatalf("cancellation=%+v err=%v", cancelled, err)
	}
	time.Sleep(300 * time.Millisecond)
	recovered, err := worker.Run(scenario.ctx, 1)
	if err != nil || recovered.Selected != 0 {
		t.Fatalf("recovery batch=%+v err=%v", recovered, err)
	}
	var evidence struct {
		TaskStatus, IntentStatus, ReasonCode, LabelZhCN, LabelEnUS string
		Events                                                     int
	}
	if err = scenario.database.GetScan(scenario.ctx, &evidence, `SELECT task.status_code task_status,intent.status_code intent_status,transition.reason_code,transition.reason_label_zh_cn label_zh_cn,transition.reason_label_en_us label_en_us,(transition.intent_no IS NOT NULL)::int events FROM plugin_idc_core_product_fulfillment_task task JOIN plugin_idc_core_product_fulfillment_intent intent ON intent.intent_no=task.intent_no LEFT JOIN plugin_idc_core_product_fulfillment_system_transition transition ON transition.intent_no=intent.intent_no AND transition.event_code='moved_to_manual' WHERE intent.order_no=?`, prepared.order.OrderNo); err != nil || evidence.TaskStatus != "cancellation_attention" || evidence.IntentStatus != "manual_pending" || evidence.Events != 1 || evidence.ReasonCode != productfulfillmentservice.SystemReasonCancellationRequiresAttention || evidence.LabelZhCN == "" || evidence.LabelEnUS == "" {
		t.Fatalf("cancellation recovery evidence=%+v err=%v", evidence, err)
	}
	extensions.mu.Lock()
	invocations := len(extensions.invocations)
	extensions.mu.Unlock()
	if invocations != 1 {
		t.Fatalf("provider invocations=%d, want only the pre-submit query", invocations)
	}
}

func seedExpiredSubmittingAttempt(scenario checkoutTestScenario) error {
	return scenario.database.Transaction(scenario.ctx, func(txCtx context.Context, _ gdb.TX) error {
		var task *struct {
			TaskNo  string
			Version int64
		}
		if err := scenario.database.GetScan(txCtx, &task, `SELECT task_no,version FROM plugin_idc_core_product_fulfillment_task WHERE tenant_id=0 AND status_code='pending_submit' LIMIT 1 FOR UPDATE`); err != nil || task == nil {
			return err
		}
		updated, err := scenario.database.Exec(txCtx, `UPDATE plugin_idc_core_product_fulfillment_task SET status_code='submitting',lease_token='pfl_bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb',lease_expires_at=CURRENT_TIMESTAMP+INTERVAL '0.2 seconds',attempt_count=attempt_count+1,submit_attempt_count=submit_attempt_count+1,version=version+1,updated_at=CURRENT_TIMESTAMP WHERE task_no=? AND version=?`, task.TaskNo, task.Version)
		changed := int64(0)
		if err == nil {
			changed, _ = updated.RowsAffected()
		}
		if err != nil || changed != 1 {
			if err == nil {
				err = errors.New("expired submit fixture did not update exactly one task")
			}
			return err
		}
		_, err = scenario.database.Exec(txCtx, `INSERT INTO plugin_idc_core_product_fulfillment_attempt(tenant_id,task_no,attempt_no,action_code,started_at,evidence_digest) VALUES(0,?,2,'submit',CURRENT_TIMESTAMP,repeat('a',64))`, task.TaskNo)
		return err
	})
}

func advanceFulfillmentTaskClock(t *testing.T, scenario checkoutTestScenario, orderNo string) {
	t.Helper()
	if _, err := scenario.database.Exec(scenario.ctx, `ALTER TABLE plugin_idc_core_product_fulfillment_task DISABLE TRIGGER trg_idc_guard_fulfillment_task`); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if _, err := scenario.database.Exec(scenario.ctx, `ALTER TABLE plugin_idc_core_product_fulfillment_task ENABLE TRIGGER trg_idc_guard_fulfillment_task`); err != nil {
			t.Error(err)
		}
	}()
	if _, err := scenario.database.Exec(scenario.ctx, `UPDATE plugin_idc_core_product_fulfillment_task SET next_attempt_at=CURRENT_TIMESTAMP WHERE intent_no IN(SELECT intent_no FROM plugin_idc_core_product_fulfillment_intent WHERE order_no=?)`, orderNo); err != nil {
		t.Fatal(err)
	}
}
