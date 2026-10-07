package billing

import (
	"context"
	"strings"
	"testing"

	"github.com/gogf/gf/v2/database/gdb"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/internal/service/principal"
)

type financialPrincipalStub struct{ audience *principal.Audience }

func (s financialPrincipalStub) Current(context.Context) (*principal.Audience, error) {
	return s.audience, nil
}

func TestAccountFinanceDirectoryPostgreSQLScopesAndPaginates(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	database := configureBillingServiceIntegrationDB(t, ctx)
	seedBillingService(t, ctx, database)
	seedInvoiceTransform(t, ctx, database)
	balanceService, err := NewBalanceService(database)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = balanceService.PostConfirmedRecharge(ctx, ConfirmedRechargeInput{
		CustomerNo: "customer-ledger", CurrencyCode: "CNY", AmountMinor: 10_000,
		SourceReference: "directory-recharge", SourceSnapshotDigest: strings.Repeat("d", 64),
	}); err != nil {
		t.Fatalf("seed directory balance: %v", err)
	}
	seedGenericRenewalDirectory(t, ctx, database)
	reader, err := NewAccountFinanceReader(database, financialPrincipalStub{audience: &principal.Audience{
		UserID: 77, Code: principal.AudienceCustomer,
		Status: principal.AudienceStatusActive, CustomerNo: "customer-ledger",
	}}, []byte(strings.Repeat("b", 32)))
	if err != nil {
		t.Fatal(err)
	}
	overview, err := reader.GetAccountFinancialOverview(ctx)
	if err != nil || len(overview.Balances) != 1 || overview.Balances[0].AvailableBalanceMinor != 10_000 ||
		overview.RechargeAvailable || overview.RechargeReasonCode != "payment_capability_unavailable" {
		t.Fatalf("overview=%+v err=%v", overview, err)
	}
	first, err := reader.ListAccountInvoices(ctx, InvoiceDirectoryFilter{Limit: 2})
	if err != nil || len(first.Invoices) != 2 || first.NextCursor == "" {
		t.Fatalf("first invoice page=%+v err=%v", first, err)
	}
	for _, invoice := range first.Invoices {
		if invoice.CustomerNo != "" || invoice.InvoiceNo == "other-customer-invoice" {
			t.Fatalf("account invoice projection leaked ownership: %+v", invoice)
		}
	}
	second, err := reader.ListAccountInvoices(ctx, InvoiceDirectoryFilter{
		Limit: 2, Cursor: first.NextCursor,
	})
	if err != nil || len(second.Invoices) == 0 {
		t.Fatalf("second invoice page=%+v err=%v", second, err)
	}
	if _, err = reader.ListAccountInvoices(ctx, InvoiceDirectoryFilter{
		Limit: 2, Cursor: first.NextCursor, StatusCode: "pending_payment",
	}); !bizerr.Is(err, CodeDirectoryInvalid) {
		t.Fatalf("invoice cursor crossed filter boundary: %v", err)
	}
	detail, err := reader.GetAccountInvoice(ctx, "merge-invoice-a", InvoiceItemDirectoryFilter{Limit: 1})
	if err != nil || len(detail.Items) != 1 || detail.NextCursor == "" || !detail.Invoice.CanPayFromBalance {
		t.Fatalf("invoice detail=%+v err=%v", detail, err)
	}
	detailNext, err := reader.GetAccountInvoice(ctx, "merge-invoice-a", InvoiceItemDirectoryFilter{
		Limit: 1, Cursor: detail.NextCursor,
	})
	if err != nil || len(detailNext.Items) != 1 || detailNext.Items[0].LineNumber != 2 {
		t.Fatalf("next invoice item page=%+v err=%v", detailNext, err)
	}
	if _, err = reader.GetAccountInvoice(ctx, "other-customer-invoice", InvoiceItemDirectoryFilter{}); !bizerr.Is(err, CodeDirectoryNotAccessible) {
		t.Fatalf("foreign invoice existence was exposed: %v", err)
	}
	transactions, err := reader.ListAccountTransactions(ctx, TransactionDirectoryFilter{})
	if err != nil || len(transactions.Transactions) != 1 ||
		transactions.Transactions[0].ChangeMinor != 10_000 {
		t.Fatalf("transactions=%+v err=%v", transactions, err)
	}
	renewals, err := reader.ListAccountRenewalServices(ctx, RenewalServiceDirectoryFilter{})
	if err != nil || len(renewals.Services) != 1 {
		t.Fatalf("renewal entitlements=%+v err=%v", renewals, err)
	}
	renewal := renewals.Services[0]
	if renewal.EntitlementNo != "entitlement-renewable" || renewal.ProductNameZhCN != "冻结长期权益" ||
		renewal.RenewalModeCode != "local_entitlement" || renewal.EntitlementStatusCode != "active" ||
		!renewal.AutoRenewEnabled || !renewal.CanConfigureAutomaticRenewal {
		t.Fatalf("renewal entitlement lost frozen authority: %+v", renewal)
	}
	overview, err = reader.GetAccountFinancialOverview(ctx)
	if err != nil || overview.AutomaticRenewalEnabled != 1 {
		t.Fatalf("automatic renewal entitlement count=%+v err=%v", overview, err)
	}
}

func seedGenericRenewalDirectory(t *testing.T, ctx context.Context, database gdb.DB) {
	t.Helper()
	if _, err := database.Exec(ctx, `
CREATE TABLE plugin_idc_core_product_fulfillment_intent (
    tenant_id INT NOT NULL DEFAULT 0,
    intent_no VARCHAR(40) PRIMARY KEY,
    product_name_zh_cn VARCHAR(200) NOT NULL,
    product_name_en_us VARCHAR(200) NOT NULL
);
CREATE TABLE plugin_idc_core_service_entitlement (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    tenant_id INT NOT NULL DEFAULT 0,
    entitlement_no VARCHAR(40) NOT NULL UNIQUE,
    source_intent_no VARCHAR(40) NOT NULL,
    customer_no VARCHAR(40) NOT NULL,
    currency_code CHAR(3) NOT NULL,
    status_code VARCHAR(24) NOT NULL,
    renewal_mode_code VARCHAR(32) NOT NULL,
    charge_type_code VARCHAR(16) NOT NULL,
    period_unit_code VARCHAR(16) NOT NULL,
    period_count INT NOT NULL,
    expires_at TIMESTAMPTZ,
    current_renewal_policy_no VARCHAR(40),
    current_renewal_policy_version BIGINT,
    version BIGINT NOT NULL DEFAULT 1,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
INSERT INTO plugin_idc_core_product_fulfillment_intent
    (intent_no,product_name_zh_cn,product_name_en_us)
VALUES
    ('intent-renewable','冻结长期权益','Frozen long-term entitlement'),
    ('intent-once','一次性交付','One-time delivery'),
    ('intent-unavailable','不可续期长期权益','Non-renewable entitlement'),
    ('intent-terminated','已终止权益','Terminated entitlement');
INSERT INTO plugin_idc_core_service_entitlement
    (entitlement_no,source_intent_no,customer_no,currency_code,status_code,
     renewal_mode_code,charge_type_code,period_unit_code,period_count,expires_at,
     current_renewal_policy_no,current_renewal_policy_version)
VALUES
    ('entitlement-renewable','intent-renewable','customer-ledger','CNY','active',
         'local_entitlement','recurring','month',1,CURRENT_TIMESTAMP + INTERVAL '30 days','policy-renewable',1),
    ('entitlement-once','intent-once','customer-ledger','CNY','active',
     'local_entitlement','one_time','once',1,NULL,NULL,NULL),
    ('entitlement-unavailable','intent-unavailable','customer-ledger','CNY','active',
         'unavailable','recurring','month',1,CURRENT_TIMESTAMP + INTERVAL '30 days',NULL,NULL),
    ('entitlement-terminated','intent-terminated','customer-ledger','CNY','terminated',
         'local_entitlement','recurring','month',1,CURRENT_TIMESTAMP + INTERVAL '30 days',NULL,NULL);
INSERT INTO plugin_idc_core_service_renewal_policy
    (policy_no,entitlement_no,policy_version,auto_renew_enabled,renewal_cycles,
     lead_seconds,maximum_attempts,retry_interval_seconds,
     failure_notification_enabled,configured_by,content_digest)
VALUES ('policy-renewable','entitlement-renewable',1,TRUE,1,604800,3,21600,TRUE,77,repeat('a',64));`); err != nil {
		t.Fatalf("seed generic renewal directory: %v", err)
	}
}

func TestAccountFinanceDirectoryRejectsNonCustomerPrincipal(t *testing.T) {
	reader := &financeDirectoryImpl{principal: financialPrincipalStub{audience: &principal.Audience{
		UserID: 1, Code: principal.AudienceStaff, Status: principal.AudienceStatusActive,
	}}}
	if _, err := reader.ListAccountInvoices(context.Background(), InvoiceDirectoryFilter{}); !bizerr.Is(err, CodeDirectoryNotAccessible) {
		t.Fatalf("staff principal entered customer financial directory: %v", err)
	}
}

func TestRenewableServiceCanConfigureUsesFrozenEntitlementFacts(t *testing.T) {
	t.Parallel()
	valid := renewalServiceDirectoryRow{
		RenewalModeCode: "provider_confirmed", PeriodUnitCode: "month",
		EntitlementStatusCode: "expired",
	}
	if !renewableServiceCanConfigure(valid) {
		t.Fatal("renewable long-term entitlement should allow policy configuration")
	}
	for name, mutate := range map[string]func(*renewalServiceDirectoryRow){
		"one-time":    func(row *renewalServiceDirectoryRow) { row.PeriodUnitCode = "once" },
		"unavailable": func(row *renewalServiceDirectoryRow) { row.RenewalModeCode = "unavailable" },
		"terminated":  func(row *renewalServiceDirectoryRow) { row.EntitlementStatusCode = "terminated" },
	} {
		t.Run(name, func(t *testing.T) {
			row := valid
			mutate(&row)
			if renewableServiceCanConfigure(row) {
				t.Fatalf("%s entitlement should not allow policy configuration", name)
			}
		})
	}
}
