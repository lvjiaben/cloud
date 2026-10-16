package renewal

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"testing"

	_ "github.com/gogf/gf/contrib/drivers/pgsql/v2"
	"github.com/gogf/gf/v2/database/gdb"

	"lina-core/pkg/bizerr"
	"lina-core/pkg/plugin/capability/capmodel"
	"lina-plugin-idc-core/backend/internal/service/billing"
	"lina-plugin-idc-core/backend/internal/service/catalog"
	"lina-plugin-idc-core/backend/internal/service/commerce"
	"lina-plugin-idc-core/backend/internal/service/configurationcontract"
	"lina-plugin-idc-core/backend/internal/service/principal"
	"lina-plugin-idc-core/backend/internal/service/purchaseconfigurationfile"
	"lina-plugin-idc-core/backend/internal/service/salesmanagement"
)

func TestRenewalQuoteItemInsertMatchesGenericCommerceSchemaPostgreSQL(t *testing.T) {
	ctx, database := newRenewalPostgreSQLDatabase(t)
	legacyColumns, err := database.GetValue(ctx, `SELECT COUNT(*) FROM information_schema.columns
WHERE table_schema=current_schema() AND table_name='plugin_idc_core_commerce_quote_item_snapshot'
  AND column_name IN ('product_type_code','server_quantity','binding_no','inventory_snapshot_no')`)
	if err != nil || legacyColumns.Int() != 0 {
		t.Fatalf("legacy renewal quote columns remain: count=%d err=%v", legacyColumns.Int(), err)
	}
	prepared := postgresNumberedParameters(renewalQuoteItemInsertSQL)
	if _, err = database.Exec(ctx, "PREPARE renewal_quote_item_insert_contract AS "+prepared); err != nil {
		t.Fatalf("generic renewal quote item insert does not match the migrated schema: %v", err)
	}
	if _, err = database.Exec(ctx, "DEALLOCATE renewal_quote_item_insert_contract"); err != nil {
		t.Fatal(err)
	}
}

func newRenewalPostgreSQLDatabase(t *testing.T) (context.Context, gdb.DB) {
	t.Helper()
	dsn := strings.TrimSpace(os.Getenv("LINAPRO_IDC_CORE_TEST_DSN"))
	if dsn == "" {
		t.Skip("set LINAPRO_IDC_CORE_TEST_DSN")
	}
	ctx := context.Background()
	admin, err := gdb.New(gdb.ConfigNode{Link: dsn})
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close(ctx)
	var suffix [8]byte
	if _, err = rand.Read(suffix[:]); err != nil {
		t.Fatal(err)
	}
	schema := "idc_renewal_quote_test_" + hex.EncodeToString(suffix[:])
	if _, err = admin.Exec(ctx, `CREATE SCHEMA "`+schema+`"`); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if strings.HasPrefix(schema, "idc_renewal_quote_test_") {
			_, _ = admin.Exec(ctx, `DROP SCHEMA "`+schema+`" CASCADE`)
		} else {
			t.Errorf("refusing to drop unexpected renewal schema %q", schema)
		}
	})
	separator := "?"
	if strings.Contains(dsn, "?") {
		separator = "&"
	}
	database, err := gdb.New(gdb.ConfigNode{Link: dsn + separator + "search_path=" + url.QueryEscape(schema)})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close(ctx) })
	if _, err = database.Exec(ctx, renewalMigrationHostFixture); err != nil {
		t.Fatal(err)
	}
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve renewal migration path")
	}
	root := filepath.Join(filepath.Dir(filename), "..", "..", "..", "..", "manifest", "sql")
	paths, err := filepath.Glob(filepath.Join(root, "[0-9][0-9][0-9]-*.sql"))
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(paths)
	for _, path := range paths {
		content, readErr := os.ReadFile(path)
		if readErr != nil {
			t.Fatal(readErr)
		}
		if _, err = database.Exec(ctx, string(content)); err != nil {
			t.Fatalf("execute %s: %v", filepath.Base(path), err)
		}
	}
	return ctx, database
}

func postgresNumberedParameters(query string) string {
	var result strings.Builder
	result.Grow(len(query) + 64)
	parameter := 0
	for _, character := range query {
		if character == '?' {
			parameter++
			result.WriteByte('$')
			result.WriteString(strconv.Itoa(parameter))
			continue
		}
		result.WriteRune(character)
	}
	return result.String()
}

const renewalMigrationHostFixture = `
CREATE TABLE sys_dict_type (
 id INT GENERATED ALWAYS AS IDENTITY PRIMARY KEY, tenant_id INT NOT NULL DEFAULT 0,
 name VARCHAR(128) NOT NULL DEFAULT '', type VARCHAR(128) NOT NULL DEFAULT '',
 status SMALLINT NOT NULL DEFAULT 1, is_builtin SMALLINT NOT NULL DEFAULT 0,
 allow_tenant_override BOOL NOT NULL DEFAULT FALSE, remark VARCHAR(512) NOT NULL DEFAULT '',
 created_at TIMESTAMPTZ, updated_at TIMESTAMPTZ, UNIQUE (tenant_id,type)
);
CREATE TABLE sys_dict_data (
 id INT GENERATED ALWAYS AS IDENTITY PRIMARY KEY, tenant_id INT NOT NULL DEFAULT 0,
 dict_type VARCHAR(128) NOT NULL DEFAULT '', label VARCHAR(128) NOT NULL DEFAULT '',
 value VARCHAR(128) NOT NULL DEFAULT '', sort INT NOT NULL DEFAULT 0,
 tag_style VARCHAR(128) NOT NULL DEFAULT '', status SMALLINT NOT NULL DEFAULT 1,
 is_builtin SMALLINT NOT NULL DEFAULT 0, created_at TIMESTAMPTZ, updated_at TIMESTAMPTZ,
 UNIQUE (tenant_id,dict_type,value)
);`

type renewalPostgreSQLPrincipal struct {
	audience *principal.Audience
}

func (s *renewalPostgreSQLPrincipal) Current(context.Context) (*principal.Audience, error) {
	return s.audience, nil
}

type renewalPostgreSQLPurchaseRuntime struct{}

func (renewalPostgreSQLPurchaseRuntime) CheckPublicPurchaseAvailability(context.Context, string) (bool, error) {
	return true, nil
}

func (renewalPostgreSQLPurchaseRuntime) ResolvePublicPurchaseCandidates(
	context.Context,
	string,
	*capmodel.ConfigurationSchemaDeclaration,
	[]configurationcontract.ConfigurationValue,
) (configurationcontract.ResolvedCandidates, error) {
	return configurationcontract.ResolvedCandidates{}, nil
}

type renewalPostgreSQLFileVerifier struct {
	purchaseconfigurationfile.Service
}

func (renewalPostgreSQLFileVerifier) VerifyFields(
	context.Context,
	[]purchaseconfigurationfile.VerifyInput,
) ([]purchaseconfigurationfile.VerifiedField, error) {
	return nil, nil
}

type renewalPostgreSQLSettlementStub struct{}

func (renewalPostgreSQLSettlementStub) SettleOrder(context.Context, string) (*billing.FinancialSettlement, error) {
	return nil, errors.New("not used by checkout")
}

func (renewalPostgreSQLSettlementStub) SettleOrderWithCredit(
	context.Context,
	billing.CreditOrderSettlementInput,
) (*billing.FinancialSettlement, error) {
	return nil, errors.New("not used by checkout")
}

func TestGenericConfigurationPriceRenewalFromCheckoutPostgreSQL(t *testing.T) {
	ctx, database := newRenewalPostgreSQLDatabase(t)
	publicReader, principalReader, fixture := newGenericRenewalCheckout(t, ctx, database)

	pricing, err := salesmanagement.NewPricing(database)
	if err != nil {
		t.Fatal(err)
	}
	marketingEvaluator, err := newDefaultQuoteAdjustments(database)
	if err != nil {
		t.Fatal(err)
	}
	evaluator, err := commerce.NewAgentQuoteAdjustmentEvaluator(pricing, marketingEvaluator)
	if err != nil {
		t.Fatal(err)
	}
	renewalService, err := NewWithQuoteAdjustments(ctx, database, principalReader, publicReader, evaluator)
	if err != nil {
		t.Fatal(err)
	}
	renewalQuote, err := renewalService.QuoteCurrentCustomer(ctx, QuoteInput{
		EntitlementNo:  fixture.EntitlementNo,
		RenewalCycles:  1,
		IdempotencyKey: "renewal-configured-quote-0001",
	})
	if err != nil || renewalQuote == nil || renewalQuote.SubtotalMinor != 15_000 ||
		renewalQuote.TotalAmountMinor != 15_000 {
		t.Fatalf("quote configured entitlement renewal: quote=%+v err=%v", renewalQuote, err)
	}
	assertGenericRenewalPriceEvidence(t, ctx, database, renewalQuote, fixture)
	// The same production-injected owner must replace renewal prices even when
	// the negotiated amount exceeds the current public renewal configuration.
	if _, err = database.Exec(ctx, `INSERT INTO plugin_idc_core_sales_purchase_price
 (tenant_id,price_no,customer_no,product_no,currency_code,charge_type_code,period_unit_code,period_count,adjustment_type_code,fixed_price_minor,status_code,version,content_digest)
 SELECT 0,'APR_renewal_markup','CUS_renewal_owner',product_no,currency_code,charge_type_code,period_unit_code,period_count,'fixed_price',20000,'active',1,repeat('a',64)
 FROM plugin_idc_core_commerce_quote_item_snapshot WHERE quote_item_no=?`, fixture.QuoteItemNo); err != nil {
		t.Fatal(err)
	}
	markedUp, err := renewalService.QuoteCurrentCustomer(ctx, QuoteInput{EntitlementNo: fixture.EntitlementNo, RenewalCycles: 1, IdempotencyKey: "renewal-agent-markup-0001"})
	if err != nil || markedUp == nil || markedUp.TotalAmountMinor != 20000 || markedUp.DiscountMinor != 0 {
		t.Fatalf("agent renewal quote=%+v err=%v", markedUp, err)
	}
	evidence, err := database.GetOne(ctx, `SELECT a.original_period_amount_minor,a.target_period_amount_minor FROM plugin_idc_core_commerce_quote_adjustment_snapshot a JOIN plugin_idc_core_commerce_quote_item_snapshot i USING (quote_item_no) WHERE i.quote_no=?`, markedUp.QuoteNo)
	if err != nil || evidence["original_period_amount_minor"].Int64() != 15000 || evidence["target_period_amount_minor"].Int64() != 20000 {
		t.Fatalf("renewal negotiated evidence=%v err=%v", evidence, err)
	}
	// A stopped sale invalidates both fresh quotes and unconfirmed earlier
	// quotes. Reopening preserves the original quote for a valid checkout.
	if _, err = database.Exec(ctx, `UPDATE plugin_idc_core_catalog_product SET status_code='suspended' WHERE product_no='prd_renewal_ssl'`); err != nil {
		t.Fatal(err)
	}
	if quote, err := renewalService.QuoteCurrentCustomer(ctx, QuoteInput{EntitlementNo: fixture.EntitlementNo, RenewalCycles: 1, IdempotencyKey: "stopped-renewal-quote"}); !bizerr.Is(err, CodePriceUnavailable) || quote != nil {
		t.Fatalf("stopped product quoted: quote=%+v err=%v", quote, err)
	}
	confirmationInput := ConfirmInput{QuoteNo: markedUp.QuoteNo, IdempotencyKey: "stopped-renewal-confirm"}
	if operation, err := renewalService.ConfirmCurrentCustomer(ctx, confirmationInput); !bizerr.Is(err, CodePriceUnavailable) || operation != nil {
		t.Fatalf("old quote confirmed after stop: operation=%+v err=%v", operation, err)
	}
	// The stopped sale is reopened only in a fresh fixture; this test keeps the
	// earlier quote immutable and verifies the pre-confirmation rejection.

	principalReader.audience = &principal.Audience{
		UserID: 202, Code: principal.AudienceCustomer, Status: principal.AudienceStatusActive,
		CustomerNo: "CUS_renewal_other", Version: 1,
	}
	if _, err = renewalService.QuoteCurrentCustomer(ctx, QuoteInput{
		EntitlementNo:  fixture.EntitlementNo,
		RenewalCycles:  1,
		IdempotencyKey: "renewal-cross-customer-0001",
	}); !bizerr.Is(err, CodeNotAccessible) {
		t.Fatalf("cross-customer renewal returned %v", err)
	}
}

// newGenericRenewalCheckout materializes a configured paid service fixture;
// tests of later lifecycle phases explicitly set their starting entitlement facts.
func newGenericRenewalCheckout(t *testing.T, ctx context.Context, database gdb.DB) (catalog.CatalogReader, *renewalPostgreSQLPrincipal, renewableCheckoutFixture) {
	t.Helper()
	seedGenericRenewalProduct(t, ctx, database)

	principalReader := &renewalPostgreSQLPrincipal{audience: &principal.Audience{
		UserID: 101, Code: principal.AudienceCustomer, Status: principal.AudienceStatusActive,
		CustomerNo: "CUS_renewal_owner", Version: 1,
	}}
	publicReader, err := catalog.NewPublicCatalogReaderConfigured(
		database,
		[]byte("0123456789abcdef0123456789abcdef"),
		renewalPostgreSQLPurchaseRuntime{},
		renewalPostgreSQLFileVerifier{},
	)
	if err != nil {
		t.Fatal(err)
	}
	if err = catalog.BindPublicBillingLabelResources(publicReader, catalog.PublicBillingLabelResources{
		"charge.one_time": {ZhCN: "一次性", EnUS: "One-time"},
		"period.hour":     {ZhCN: "小时", EnUS: "hour"},
		"period.day":      {ZhCN: "天", EnUS: "day"},
		"period.month":    {ZhCN: "个月", EnUS: "month"},
		"period.year":     {ZhCN: "年", EnUS: "year"},
	}); err != nil {
		t.Fatal(err)
	}
	configuration, err := publicReader.GetPurchaseConfiguration(ctx, "renewal-ssl-certificate")
	if err != nil || configuration == nil || len(configuration.Fields) != 1 ||
		len(configuration.Fields[0].Options) != 1 {
		t.Fatalf("load non-server purchase configuration: configuration=%+v err=%v", configuration, err)
	}
	values := []catalog.PublicPurchaseConfigurationValue{{
		FieldToken: configuration.Fields[0].FieldToken,
		Type:       "single_select",
		OptionTokens: []string{
			configuration.Fields[0].Options[0].OptionToken,
		},
	}}
	quotedConfiguration, err := publicReader.QuotePurchaseConfiguration(
		ctx,
		catalog.PublicPurchaseConfigurationQuoteInput{
			PublicPurchaseConfigurationInput: catalog.PublicPurchaseConfigurationInput{
				PublicSlug:            "renewal-ssl-certificate",
				ConfigurationRevision: configuration.ConfigurationRevision,
				Values:                values,
			},
			CurrencyCode: "CNY",
		},
	)
	if err != nil || quotedConfiguration == nil || len(quotedConfiguration.Terms) != 1 {
		t.Fatalf("quote non-server purchase configuration: quote=%+v err=%v", quotedConfiguration, err)
	}
	purchase := catalog.PurchaseConfigurationInput{
		PublicSlug:            "renewal-ssl-certificate",
		ConfigurationRevision: configuration.ConfigurationRevision,
		BillingTermToken:      quotedConfiguration.Terms[0].BillingTermToken,
		CurrencyCode:          "CNY",
		PurchaseCycles:        1,
		Values:                values,
	}
	resolver, err := catalog.PurchaseResolverFromPublicReader(publicReader)
	if err != nil {
		t.Fatal(err)
	}
	resolved, err := resolver.ResolvePurchaseConfiguration(ctx, purchase)
	if err != nil {
		t.Fatalf("resolve configured purchase: %v", err)
	}
	assembler, err := catalog.PurchaseQuoteAssemblerFromPublicReader(publicReader)
	if err != nil {
		t.Fatal(err)
	}
	if lines, assembleErr := assembler.AssemblePurchaseQuote(ctx, catalog.PurchaseQuoteInput{
		CurrencyCode: "CNY",
		Lines: []catalog.PurchaseQuoteLineInput{{
			ReferenceNo: "renewal-test", ProductNo: resolved.ProductNo,
			VersionNo: resolved.VersionNo, BillingRuleNo: resolved.BillingRuleNo,
			Quantity: 1, PurchaseCycles: 1, PriceSelections: resolved.PriceSelections,
		}},
	}); assembleErr != nil || len(lines) != 1 {
		t.Fatalf("assemble configured purchase: resolved=%+v lines=%+v err=%v", resolved, lines, assembleErr)
	}
	commerceService, err := commerce.New(
		database,
		resolver,
		principalReader,
		renewalPostgreSQLSettlementStub{},
	)
	if err != nil {
		t.Fatal(err)
	}
	freezer, err := catalog.PurchaseConfigurationFreezerFromPublicReader(publicReader)
	if err != nil {
		t.Fatal(err)
	}
	files, err := purchaseconfigurationfile.NewOrderFileConsumer(database, principalReader)
	if err != nil {
		t.Fatal(err)
	}
	if err = commerce.BindStructuredConfigurationSnapshots(commerceService, freezer, files); err != nil {
		t.Fatal(err)
	}
	cart, err := commerceService.OpenAccountCart(ctx, "CNY")
	if err != nil {
		t.Fatal(err)
	}
	cart, err = commerceService.AddAccountItem(ctx, commerce.AddItemInput{
		ExpectedCartVersion: cart.Version,
		Quantity:            1,
		Purchase:            purchase,
	})
	if err != nil {
		t.Fatal(err)
	}
	checkoutQuote, err := commerceService.QuoteAccountCart(ctx, commerce.QuoteRequest{
		ExpectedCartVersion: cart.Version,
	})
	if err != nil || checkoutQuote == nil || len(checkoutQuote.Items) != 1 ||
		checkoutQuote.TotalAmountMinor != 12_500 {
		currentCart, currentErr := commerceService.GetAccountCart(ctx)
		t.Fatalf("quote configured non-server product: quote=%+v err=%v cart=%+v cartErr=%v", checkoutQuote, err, currentCart, currentErr)
	}
	frozen, err := commerceService.FreezeAccountQuoteItemConfiguration(
		ctx,
		commerce.FreezeQuoteItemConfigurationInput{
			QuoteItemNo: checkoutQuote.Items[0].QuoteItemNo,
			Purchase: catalog.PublicPurchaseConfigurationInput{
				PublicSlug:            "renewal-ssl-certificate",
				ConfigurationRevision: configuration.ConfigurationRevision,
				Values:                values,
			},
		},
	)
	if err != nil || frozen == nil || !frozen.ConfigurationFrozen ||
		!strings.HasPrefix(frozen.SnapshotNo, "SCF_") {
		t.Fatalf("freeze structured configuration: frozen=%+v err=%v", frozen, err)
	}
	checkout, err := commerceService.CheckoutAccountQuote(ctx, commerce.CheckoutRequest{
		QuoteNo: checkoutQuote.QuoteNo, IdempotencyKey: "renewal-configured-checkout-0001",
	})
	if err != nil || checkout == nil || checkout.TotalAmountMinor != 12_500 {
		t.Fatalf("checkout configured non-server product: checkout=%+v err=%v", checkout, err)
	}

	fixture := establishRenewableEntitlementFromCheckout(
		t, ctx, database, checkout, checkoutQuote.Items[0].QuoteItemNo, frozen.SnapshotNo,
	)
	if !strings.HasPrefix(fixture.PriceConfigurationSnapshotNo, "CFG_") ||
		fixture.StructuredConfigurationSnapshotNo != frozen.SnapshotNo {
		t.Fatalf("checkout did not retain separate CFG and SCF snapshots: %+v", fixture)
	}

	return publicReader, principalReader, fixture
}

type renewableCheckoutFixture struct {
	QuoteItemNo                       string
	EntitlementNo                     string
	PriceConfigurationSnapshotNo      string
	StructuredConfigurationSnapshotNo string
}

func establishRenewableEntitlementFromCheckout(
	t *testing.T,
	ctx context.Context,
	database gdb.DB,
	checkout *commerce.Checkout,
	quoteItemNo string,
	structuredConfigurationSnapshotNo string,
) renewableCheckoutFixture {
	t.Helper()
	var row struct {
		OrderItemNo                  string `orm:"order_item_no"`
		PriceConfigurationSnapshotNo string `orm:"configuration_snapshot_no"`
		ProductNo                    string `orm:"product_no"`
		VersionNo                    string `orm:"version_no"`
		ProductNameZhCN              string `orm:"product_name_zh_cn"`
		ProductNameEnUS              string `orm:"product_name_en_us"`
	}
	if err := database.GetScan(ctx, &row, `SELECT order_item_no,configuration_snapshot_no,
product_no,version_no,product_name_zh_cn,product_name_en_us
FROM plugin_idc_core_commerce_order_item
WHERE tenant_id=0 AND order_no=? AND quote_item_no=?`, checkout.OrderNo, quoteItemNo); err != nil {
		t.Fatal(err)
	}
	if row.OrderItemNo == "" || row.PriceConfigurationSnapshotNo == "" {
		t.Fatalf("checkout order item did not freeze generic price configuration: %+v", row)
	}
	if value, err := database.GetValue(ctx, `SELECT count(*)
FROM plugin_idc_core_commerce_structured_configuration_snapshot
WHERE tenant_id=0 AND snapshot_no=? AND order_no=? AND order_item_no=?`,
		structuredConfigurationSnapshotNo, checkout.OrderNo, row.OrderItemNo); err != nil || value.Int() != 1 {
		t.Fatalf("checkout did not bind structured configuration to order: count=%d err=%v", value.Int(), err)
	}
	if _, err := database.Exec(ctx, `ALTER TABLE plugin_idc_core_billing_ledger_transaction DISABLE TRIGGER USER`); err != nil {
		t.Fatalf("disable ledger fixture triggers: %v", err)
	}
	if _, err := database.Exec(ctx, `INSERT INTO plugin_idc_core_billing_ledger_transaction
(tenant_id,transaction_no,transaction_type_code,currency_code,source_type_code,source_reference,
 source_snapshot_digest,entry_count,debit_total_minor,credit_total_minor,content_digest,posted_by)
SELECT 0,'TXN_renewal_configured','payment','CNY','migration','renewal-configured-payment',
       repeat('d',64),2,total_amount_minor,total_amount_minor,repeat('e',64),0
FROM plugin_idc_core_billing_invoice WHERE tenant_id=0 AND invoice_no=?`, checkout.InvoiceNo); err != nil {
		t.Fatalf("seed checkout payment transaction: %v", err)
	}
	if _, err := database.Exec(ctx, `ALTER TABLE plugin_idc_core_billing_ledger_transaction ENABLE TRIGGER USER`); err != nil {
		t.Fatalf("restore ledger fixture triggers: %v", err)
	}
	if _, err := database.Exec(ctx, `ALTER TABLE plugin_idc_core_billing_payment_allocation DISABLE TRIGGER USER`); err != nil {
		t.Fatalf("disable payment allocation fixture triggers: %v", err)
	}
	if _, err := database.Exec(ctx, `INSERT INTO plugin_idc_core_billing_payment_allocation
(tenant_id,allocation_no,invoice_no,customer_no,currency_code,payment_source_type_code,
 payment_source_reference,ledger_transaction_no,ledger_transaction_type_code,direction_code,
 amount_minor,content_digest)
SELECT 0,'PAL_renewal_configured',invoice_no,customer_no,currency_code,'migration',
       'renewal-configured-payment','TXN_renewal_configured','payment','apply',total_amount_minor,
       repeat('f',64)
FROM plugin_idc_core_billing_invoice WHERE tenant_id=0 AND invoice_no=?`, checkout.InvoiceNo); err != nil {
		t.Fatalf("seed checkout payment allocation: %v", err)
	}
	if _, err := database.Exec(ctx, `ALTER TABLE plugin_idc_core_billing_payment_allocation ENABLE TRIGGER USER`); err != nil {
		t.Fatalf("restore payment allocation fixture triggers: %v", err)
	}
	if _, err := database.Exec(ctx, `UPDATE plugin_idc_core_billing_invoice
SET status_code='paid',paid_amount_minor=total_amount_minor,
    refundable_amount_minor=total_amount_minor,settled_at=CURRENT_TIMESTAMP,updated_at=CURRENT_TIMESTAMP
WHERE tenant_id=0 AND invoice_no=?`, checkout.InvoiceNo); err != nil {
		t.Fatalf("mark checkout invoice paid: %v", err)
	}
	if err := database.Transaction(ctx, func(_ context.Context, tx gdb.TX) error {
		if _, txErr := tx.Exec(`INSERT INTO plugin_idc_core_financial_settlement
(tenant_id,settlement_no,order_no,invoice_no,customer_no,currency_code,basis_code,
 total_amount_minor,paid_amount_minor,credit_amount_minor,source_count,
 identity_evidence_digest,source_snapshot_digest,content_digest,settled_at)
SELECT 0,'SET_renewal_configured',orders.order_no,invoice.invoice_no,orders.customer_no,
       orders.currency_code,'funded_invoice',orders.total_amount_minor,orders.total_amount_minor,0,1,
       repeat('1',64),repeat('2',64),repeat('3',64),CURRENT_TIMESTAMP
FROM plugin_idc_core_commerce_order orders
JOIN plugin_idc_core_billing_invoice invoice ON invoice.order_no=orders.order_no
WHERE orders.tenant_id=0 AND orders.order_no=? AND invoice.invoice_no=?`, checkout.OrderNo, checkout.InvoiceNo); txErr != nil {
			return txErr
		}
		_, txErr := tx.Exec(`INSERT INTO plugin_idc_core_financial_settlement_source
(tenant_id,settlement_source_no,settlement_no,order_no,invoice_no,customer_no,currency_code,
 source_type_code,source_reference,payment_allocation_no,amount_minor,evidence_digest,content_digest)
SELECT 0,'FSS_renewal_configured',settlement_no,order_no,invoice_no,customer_no,currency_code,
       'channel_payment','renewal-configured-payment','PAL_renewal_configured',paid_amount_minor,
       repeat('a',64),repeat('b',64)
FROM plugin_idc_core_financial_settlement
WHERE tenant_id=0 AND settlement_no='SET_renewal_configured'`)
		return txErr
	}); err != nil {
		t.Fatalf("establish paid checkout settlement: %v", err)
	}
	if _, err := database.Exec(ctx, `INSERT INTO plugin_idc_core_product_delivery_record
(tenant_id,delivery_no,order_no,order_item_no,customer_no,product_no,version_no,quantity,
 strategy_code,policy_digest,configuration_snapshot_no,interface_instance_no,plugin_id,
 plugin_release_version,contract_version,declaration_digest,ongoing_management,
 renewal_mode_code,renewal_method,entitlement_owner_id,status_code,started_at,content_digest)
VALUES (0,'DLV_renewal_configured',?,?, 'CUS_renewal_owner',?,?,1,
        'payment_confirmed_plugin',repeat('4',64),?,'pfi_11111111111111111111111111111111',
        'ssl-fulfillment','1.0.0','v1',repeat('5',64),TRUE,
        'local_entitlement','','ssl-fulfillment','processing',CURRENT_TIMESTAMP,repeat('6',64))`,
		checkout.OrderNo, row.OrderItemNo, row.ProductNo, row.VersionNo,
		structuredConfigurationSnapshotNo); err != nil {
		t.Fatalf("establish checkout delivery: %v", err)
	}
	if _, err := database.Exec(ctx, `INSERT INTO plugin_idc_core_product_fulfillment_intent
(tenant_id,intent_no,settlement_no,order_no,order_item_no,customer_no,invoice_no,currency_code,
 product_no,version_no,product_name_zh_cn,product_name_en_us,quantity,unit_sequence,
 strategy_code,granularity_code,ongoing_management,renewal_mode_code,renewal_method,
 entitlement_owner_id,initial_status_code,status_code,configuration_snapshot_no,
 interface_instance_no,interface_configuration_revision,plugin_id,plugin_release_version,
 contract_version,release_digest,declaration_digest,provision_method,query_method,
 maximum_attempts,retry_base_seconds,policy_digest,evidence_digest,financially_settled_at)
VALUES (0,'PFT_renewal_configured','SET_renewal_configured',?,?, 'CUS_renewal_owner',?,'CNY',
        ?,?,?,?,1,1,'payment_confirmed_plugin','per_order_item',TRUE,
        'local_entitlement','','ssl-fulfillment','queued','queued',?,
        'pfi_11111111111111111111111111111111',1,'ssl-fulfillment','1.0.0','v1',
        repeat('7',64),repeat('5',64),'provision','query',10,30,repeat('4',64),repeat('8',64),
        CURRENT_TIMESTAMP)`, checkout.OrderNo, row.OrderItemNo, checkout.InvoiceNo,
		row.ProductNo, row.VersionNo, row.ProductNameZhCN, row.ProductNameEnUS,
		structuredConfigurationSnapshotNo); err != nil {
		t.Fatalf("complete checkout fulfillment: %v", err)
	}
	if _, err := database.Exec(ctx, `ALTER TABLE plugin_idc_core_product_fulfillment_intent DISABLE TRIGGER trg_idc_guard_fulfillment_intent`); err != nil {
		t.Fatalf("disable fulfillment history guard: %v", err)
	}
	if _, err := database.Exec(ctx, `ALTER TABLE plugin_idc_core_product_fulfillment_intent DISABLE TRIGGER trg_idc_check_fulfillment_intent_terminal`); err != nil {
		t.Fatalf("disable fulfillment terminal guard: %v", err)
	}
	if _, err := database.Exec(ctx, `UPDATE plugin_idc_core_product_fulfillment_intent
SET status_code='completed',version=version+1,updated_at=CURRENT_TIMESTAMP
WHERE tenant_id=0 AND intent_no='PFT_renewal_configured'`); err != nil {
		t.Fatalf("complete checkout fulfillment intent: %v", err)
	}
	if _, err := database.Exec(ctx, `ALTER TABLE plugin_idc_core_product_fulfillment_intent ENABLE TRIGGER trg_idc_guard_fulfillment_intent`); err != nil {
		t.Fatalf("restore fulfillment history guard: %v", err)
	}
	if _, err := database.Exec(ctx, `ALTER TABLE plugin_idc_core_product_fulfillment_intent ENABLE TRIGGER trg_idc_check_fulfillment_intent_terminal`); err != nil {
		t.Fatalf("restore fulfillment terminal guard: %v", err)
	}
	var entitlementNo string
	if value, err := database.GetValue(ctx, `SELECT entitlement_no
FROM plugin_idc_core_service_entitlement
WHERE tenant_id=0 AND source_intent_no='PFT_renewal_configured'`); err != nil || value.IsNil() {
		t.Fatalf("load generated entitlement: value=%v err=%v", value, err)
	} else {
		entitlementNo = value.String()
	}
	if _, err := database.Exec(ctx, `UPDATE plugin_idc_core_service_entitlement
SET expires_at=CURRENT_TIMESTAMP+INTERVAL '1 month',updated_at=CURRENT_TIMESTAMP
WHERE tenant_id=0 AND entitlement_no=?`, entitlementNo); err != nil {
		t.Fatalf("set renewable entitlement expiry: %v", err)
	}
	return renewableCheckoutFixture{
		QuoteItemNo:                       quoteItemNo,
		EntitlementNo:                     entitlementNo,
		PriceConfigurationSnapshotNo:      row.PriceConfigurationSnapshotNo,
		StructuredConfigurationSnapshotNo: structuredConfigurationSnapshotNo,
	}
}

func assertGenericRenewalPriceEvidence(
	t *testing.T,
	ctx context.Context,
	database gdb.DB,
	quote *Quote,
	fixture renewableCheckoutFixture,
) {
	t.Helper()
	var snapshot struct {
		ConfigurationSnapshotNo           string `orm:"configuration_snapshot_no"`
		StructuredConfigurationSnapshotNo string `orm:"structured_configuration_snapshot_no"`
		SubtotalMinor                     int64  `orm:"subtotal_minor"`
		TotalAmountMinor                  int64  `orm:"total_amount_minor"`
		ComponentCount                    int    `orm:"component_count"`
	}
	if err := database.GetScan(ctx, &snapshot, `SELECT configuration_snapshot_no,
structured_configuration_snapshot_no,subtotal_minor,total_amount_minor,component_count
FROM plugin_idc_core_service_price_snapshot
WHERE tenant_id=0 AND price_snapshot_no=? AND subject_type_code='entitlement' AND subject_no=?`,
		quote.PriceSnapshotNo, fixture.EntitlementNo); err != nil {
		t.Fatal(err)
	}
	if snapshot.ConfigurationSnapshotNo != fixture.PriceConfigurationSnapshotNo ||
		snapshot.StructuredConfigurationSnapshotNo != fixture.StructuredConfigurationSnapshotNo ||
		snapshot.SubtotalMinor != 15_000 || snapshot.TotalAmountMinor != 15_000 ||
		snapshot.ComponentCount != 2 {
		t.Fatalf("renewal price snapshot did not freeze CFG+SCF and complete price: %+v", snapshot)
	}
	var components struct {
		BaseCount       int   `orm:"base_count"`
		BaseAmount      int64 `orm:"base_amount"`
		DimensionCount  int   `orm:"dimension_count"`
		DimensionAmount int64 `orm:"dimension_amount"`
		BoundCFGCount   int   `orm:"bound_cfg_count"`
	}
	if err := database.GetScan(ctx, &components, `SELECT
count(*) FILTER (WHERE component_type_code='base') AS base_count,
COALESCE(sum(amount_minor) FILTER (WHERE component_type_code='base'),0) AS base_amount,
count(*) FILTER (WHERE configuration_price_dimension_no='cpd_11111111111111111111111111111111') AS dimension_count,
COALESCE(sum(amount_minor) FILTER (WHERE configuration_price_dimension_no='cpd_11111111111111111111111111111111'),0) AS dimension_amount,
count(*) FILTER (WHERE configuration_price_dimension_no='cpd_11111111111111111111111111111111'
                  AND configuration_snapshot_no=?) AS bound_cfg_count
FROM plugin_idc_core_service_price_component_snapshot
WHERE tenant_id=0 AND price_snapshot_no=? AND subject_no=?`,
		fixture.PriceConfigurationSnapshotNo, quote.PriceSnapshotNo, fixture.EntitlementNo); err != nil {
		t.Fatal(err)
	}
	if components.BaseCount != 1 || components.BaseAmount != 12_000 ||
		components.DimensionCount != 1 || components.DimensionAmount != 3_000 ||
		components.BoundCFGCount != 1 {
		t.Fatalf("renewal components did not retain base plus generic configured price: %+v", components)
	}
}

func seedGenericRenewalProduct(t *testing.T, ctx context.Context, database gdb.DB) {
	t.Helper()
	if _, err := database.Exec(ctx, genericRenewalProductSQL+genericRenewalProductPublicationSQL); err != nil {
		t.Fatalf("seed generic renewable product: %v", err)
	}
}

const genericRenewalProductSQL = `
INSERT INTO plugin_idc_core_customer_subject
    (customer_no,customer_type_code,status_code,display_name)
VALUES ('CUS_renewal_owner','individual','active','Renewal owner'),
       ('CUS_renewal_other','individual','active','Other customer');
INSERT INTO plugin_idc_core_catalog_category
    (category_no,status_code,name_zh_cn,name_en_us)
VALUES ('CTG_renewal_ssl','active','数字证书','Certificates');
INSERT INTO plugin_idc_core_catalog_product
    (product_no,category_no,status_code,public_slug,name_zh_cn,name_en_us)
VALUES ('prd_renewal_ssl','CTG_renewal_ssl','active','renewal-ssl-certificate',
        '通配符证书','Wildcard certificate');
INSERT INTO plugin_idc_core_catalog_product_version
    (version_no,product_no,version_number,status_code,source_code)
VALUES ('ver_renewal_ssl_v1','prd_renewal_ssl',1,'draft','manual');
INSERT INTO plugin_idc_core_catalog_product_version_inventory
    (version_no,tracking_enabled,sellable_quantity,version)
VALUES ('ver_renewal_ssl_v1',FALSE,0,1);
INSERT INTO plugin_idc_core_product_automation_interface_instance
    (tenant_id,interface_instance_no,display_name,plugin_id,extension_point_code,
     plugin_release_version,contract_version,authorized_methods,release_digest,
     declaration_digest,new_business_state,created_by,changed_by)
VALUES (0,'pfi_11111111111111111111111111111111','SSL fulfillment','ssl-fulfillment',
        'product-fulfillment','1.0.0','v1',ARRAY['provision','query'],repeat('7',64),
        repeat('5',64),'accepting',101,101);
INSERT INTO plugin_idc_core_product_automation_interface_configuration_revision
    (tenant_id,interface_instance_no,revision,schema_version,declaration_snapshot,
     public_value_snapshot,declaration_digest,value_digest,secret_field_count,created_by)
VALUES (0,'pfi_11111111111111111111111111111111',1,'v1','{}','{}',
        repeat('9',64),repeat('a',64),0,101);
INSERT INTO plugin_idc_core_catalog_product_version_fulfillment_binding
    (tenant_id,version_no,interface_instance_no,interface_display_name_snapshot,plugin_id,
     extension_point_code,plugin_release_version,contract_version,authorized_methods,
     release_digest,declaration_digest,bound_by)
VALUES (0,'ver_renewal_ssl_v1','pfi_11111111111111111111111111111111','SSL fulfillment',
        'ssl-fulfillment','product-fulfillment','1.0.0','v1',ARRAY['provision','query'],
        repeat('7',64),repeat('5',64),101);
INSERT INTO plugin_idc_core_catalog_product_version_configuration
    (tenant_id,version_no,interface_instance_no,interface_configuration_revision,
     product_schema_version,product_declaration_snapshot,merchant_value_snapshot,
     customer_schema_version,customer_declaration_snapshot,declaration_digest,
     merchant_value_digest,changed_by)
VALUES (0,'ver_renewal_ssl_v1','pfi_11111111111111111111111111111111',1,
        'v1','{}','{}','v1',
        '{"version":"v1","fields":[{"key":"certificate_type","type":"single_select","required":true,"label":{"zhCN":"证书类型","enUS":"Certificate type"},"staticCandidates":[{"value":"wildcard","label":{"zhCN":"通配符","enUS":"Wildcard"}}]}]}',
        repeat('b',64),repeat('c',64),101);
INSERT INTO plugin_idc_core_catalog_product_version_fulfillment_policy
    (version_no,strategy_code,granularity_code,provision_method,query_method,
     maximum_attempts,retry_base_seconds,policy_digest,created_by,ongoing_management,
     renewal_mode_code,renewal_method,entitlement_owner_id)
VALUES ('ver_renewal_ssl_v1','payment_confirmed_plugin','per_order_item','provision','query',
        10,30,repeat('4',64),101,TRUE,'local_entitlement','','ssl-fulfillment');
INSERT INTO plugin_idc_core_catalog_billing_rule
    (billing_rule_no,version_no,rule_code,charge_type_code,period_unit_code,period_count,status_code)
VALUES ('bil_renewal_ssl_month','ver_renewal_ssl_v1','monthly','recurring','month',1,'active');
INSERT INTO plugin_idc_core_catalog_configuration_price_dimension
    (tenant_id,dimension_no,version_no,configuration_version,field_key,candidate_value,
     field_label_zh_cn,field_label_en_us,candidate_label_zh_cn,candidate_label_en_us,status_code)
VALUES (0,'cpd_11111111111111111111111111111111','ver_renewal_ssl_v1',1,
        'certificate_type','wildcard','证书类型','Certificate type','通配符','Wildcard','active');
INSERT INTO plugin_idc_core_catalog_price
    (price_no,version_no,billing_rule_no,currency_code,amount_minor,setup_fee_minor,
     renewal_amount_minor,status_code)
VALUES ('PRC_renewal_ssl_base','ver_renewal_ssl_v1','bil_renewal_ssl_month','CNY',
        10000,0,12000,'active');
INSERT INTO plugin_idc_core_catalog_price
    (price_no,version_no,billing_rule_no,configuration_price_dimension_no,currency_code,
     amount_minor,setup_fee_minor,renewal_amount_minor,status_code)
VALUES ('PRC_renewal_ssl_wildcard','ver_renewal_ssl_v1','bil_renewal_ssl_month',
        'cpd_11111111111111111111111111111111','CNY',2500,0,3000,'active');
INSERT INTO plugin_idc_core_catalog_tax_policy
    (tax_policy_no,version_no,currency_code,price_includes_tax,tax_treatment_code,
     tax_rate_basis_points,tax_category_code,rounding_scope_code,rounding_mode_code,
     rounding_quantum_minor)
VALUES ('TAX_renewal_ssl','ver_renewal_ssl_v1','CNY',TRUE,'exempt',NULL,
		'goods','line','half_up',1);
`

const genericRenewalProductPublicationSQL = `
UPDATE plugin_idc_core_catalog_product_version
SET status_code='published',published_at=CURRENT_TIMESTAMP
WHERE version_no='ver_renewal_ssl_v1';
UPDATE plugin_idc_core_catalog_product
SET current_version_no='ver_renewal_ssl_v1'
WHERE product_no='prd_renewal_ssl';
`
