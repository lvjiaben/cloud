// This file verifies tax-invoice source projections against isolated PostgreSQL schemas.

package taxinvoicesource

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"os"
	"slices"
	"strings"
	"testing"

	_ "github.com/gogf/gf/contrib/drivers/pgsql/v2"
	"github.com/gogf/gf/v2/database/gdb"

	"lina-plugin-idc-core/backend/cap/identitycap"
	"lina-plugin-idc-core/backend/cap/taxinvoicecap"
)

const taxInvoiceSourcePostgreSQLDSNEnvironment = "LINAPRO_IDC_CORE_TEST_DSN"

type customerReaderStub struct {
	subject *identitycap.CustomerSubject
}

func (s customerReaderStub) CurrentCustomer(context.Context) (*identitycap.CustomerSubject, error) {
	copyValue := *s.subject
	return &copyValue, nil
}

type allocationUsageStub struct {
	rows []taxinvoicecap.InvoiceUsage
}

func (s allocationUsageStub) ReadAllocatedUsage(
	_ context.Context,
	customerNo string,
	invoiceNos []string,
) ([]taxinvoicecap.InvoiceUsage, error) {
	if customerNo != "customer-1" || !slices.Contains(invoiceNos, "invoice-1") {
		if customerNo != "customer-1" {
			return nil, errors.New("unexpected tax invoice usage request")
		}
	}
	result := make([]taxinvoicecap.InvoiceUsage, 0, len(s.rows))
	for _, row := range s.rows {
		if slices.Contains(invoiceNos, row.BillingInvoiceNo) {
			result = append(result, row)
		}
	}
	return result, nil
}

func TestCurrentCustomerTaxInvoiceSourcePostgreSQL(t *testing.T) {
	database, cleanup := newTaxInvoiceSourceTestDatabase(t)
	defer cleanup()
	ctx := context.Background()
	serviceValue, err := New(database, customerReaderStub{subject: testCustomerSubject()})
	if err != nil {
		t.Fatalf("construct tax invoice source: %v", err)
	}
	service := serviceValue.(*serviceImpl)
	usage := allocationUsageStub{rows: []taxinvoicecap.InvoiceUsage{{
		BillingInvoiceNo: "invoice-1", AllocatedGrossMinor: 100,
		Items: []taxinvoicecap.ItemUsage{{
			BillingInvoiceItemNo: "item-1", AllocatedGrossMinor: 100,
			AllocatedNetMinor: 94, AllocatedTaxMinor: 6,
		}},
	}}}

	options, err := service.ListCurrentCustomerOptions(ctx, usage)
	if err != nil {
		t.Fatalf("list tax invoice request options: %v", err)
	}
	invoiceOption := findInvoiceOption(options.Invoices, "invoice-1")
	if options.CustomerNo != "customer-1" || len(options.Profiles) != 2 ||
		options.Profiles[0].ProfileNo != "profile-default" || len(options.Invoices) != 3 ||
		invoiceOption == nil || invoiceOption.SourceGrossMinor != 900 ||
		invoiceOption.AlreadyAllocatedMinor != 100 || invoiceOption.RemainingGrossMinor != 800 {
		t.Fatalf("unexpected current customer options: %+v", options)
	}

	snapshot, err := service.PrepareCurrentCustomerRequest(
		ctx, usage, "profile-default",
		[]taxinvoicecap.Selection{{BillingInvoiceNo: "invoice-1", RequestedGrossMinor: 401}},
	)
	if err != nil {
		t.Fatalf("prepare tax invoice request source: %v", err)
	}
	defer snapshot.Clear()
	if snapshot.CustomerNo != "customer-1" || snapshot.Buyer.ProfileNo != "profile-default" ||
		snapshot.Buyer.Version != 3 || snapshot.Buyer.BankAccountSecretRef != "secret://bank-1" ||
		snapshot.Seller.MerchantNo != "merchant-local" || snapshot.CurrencyCode != "CNY" ||
		snapshot.GrossAmountMinor != 401 || snapshot.NetAmountMinor != 377 ||
		snapshot.TaxAmountMinor != 24 || len(snapshot.Invoices) != 1 ||
		len(snapshot.Invoices[0].Items) != 2 ||
		snapshot.Invoices[0].Items[1].GrossAmountMinor != 201 ||
		snapshot.Invoices[0].Items[0].TaxClassificationCode != "cloud_service" ||
		snapshot.Invoices[0].Items[0].TaxRatePPM != 60000 {
		t.Fatalf("unexpected prepared tax invoice source: %+v", snapshot)
	}

	if _, err = service.PrepareCurrentCustomerRequest(
		ctx, usage, "profile-other",
		[]taxinvoicecap.Selection{{BillingInvoiceNo: "invoice-1", RequestedGrossMinor: 1}},
	); err == nil {
		t.Fatal("another customer's invoice profile was accepted")
	}
	if _, err = service.PrepareCurrentCustomerRequest(
		ctx, usage, "profile-default",
		[]taxinvoicecap.Selection{{BillingInvoiceNo: "invoice-other", RequestedGrossMinor: 1}},
	); err == nil {
		t.Fatal("another customer's paid billing invoice was accepted")
	}
	for _, scenario := range []struct {
		invoiceNo string
		itemNo    string
	}{
		{invoiceNo: "invoice-renewal", itemNo: "item-renewal"},
		{invoiceNo: "invoice-derived", itemNo: "item-derived"},
	} {
		prepared, prepareErr := service.PrepareCurrentCustomerRequest(
			ctx, usage, "profile-default",
			[]taxinvoicecap.Selection{{BillingInvoiceNo: scenario.invoiceNo, RequestedGrossMinor: 100}},
		)
		if prepareErr != nil {
			t.Fatalf("prepare %s tax source: %v", scenario.invoiceNo, prepareErr)
		}
		if len(prepared.Invoices) != 1 || len(prepared.Invoices[0].Items) != 1 ||
			prepared.Invoices[0].Items[0].BillingInvoiceItemNo != scenario.itemNo ||
			prepared.Invoices[0].Items[0].TaxClassificationCode != "cloud_service" {
			t.Fatalf("unexpected %s tax source: %+v", scenario.invoiceNo, prepared)
		}
		prepared.Clear()
	}
}

func TestPostedRefundTaxInvoiceSourcePostgreSQL(t *testing.T) {
	database, cleanup := newTaxInvoiceSourceTestDatabase(t)
	defer cleanup()
	ctx := context.Background()
	serviceValue, err := New(database, customerReaderStub{subject: testCustomerSubject()})
	if err != nil {
		t.Fatalf("construct posted refund source: %v", err)
	}
	service := serviceValue.(*serviceImpl)
	if _, err = database.Exec(ctx, `
INSERT INTO plugin_idc_core_billing_ledger_transaction
    (tenant_id, transaction_no, currency_code, transaction_type_code, posted_at)
VALUES (0, 'ledger-refund-1', 'CNY', 'refund', CURRENT_TIMESTAMP);
INSERT INTO plugin_idc_core_billing_refund_allocation
	    (tenant_id, refund_allocation_no, invoice_no, invoice_item_no, customer_no,
	     currency_code, direction_code, amount_minor, refund_source_type_code, refund_source_reference,
     ledger_transaction_no, ledger_transaction_type_code, content_digest)
VALUES
    (0, 'refund-allocation-1', 'invoice-1', 'item-1', 'customer-1', 'CNY',
	     'apply', 500, 'manual', 'refund-operation-1', 'ledger-refund-1', 'refund', repeat('a', 64)),
    (0, 'refund-allocation-2', 'invoice-1', 'item-2', 'customer-1', 'CNY',
	     'apply', 100, 'manual', 'refund-operation-1', 'ledger-refund-1', 'refund', repeat('b', 64));
INSERT INTO plugin_idc_core_billing_refund_allocation
    (tenant_id, invoice_no, invoice_item_no, direction_code, amount_minor,
     original_refund_allocation_no)
VALUES (0, 'invoice-1', 'item-1', 'reverse', 200, 'refund-allocation-1');`); err != nil {
		t.Fatalf("seed posted refund source: %v", err)
	}
	refund, err := service.GetPostedRefund(ctx, "manual:refund-operation-1")
	if err != nil {
		t.Fatalf("load posted refund source: %v", err)
	}
	if refund.CustomerNo != "customer-1" || refund.CurrencyCode != "CNY" ||
		refund.PostedAt.IsZero() || len(refund.Allocations) != 2 ||
		refund.Allocations[0].AmountMinor != 300 || refund.Allocations[1].AmountMinor != 100 {
		t.Fatalf("unexpected posted refund: %#v", refund)
	}
	if _, err = service.GetPostedRefund(ctx, "manual:refund-operation-missing"); err == nil {
		t.Fatal("missing posted refund was accepted")
	}
	if _, err = database.Exec(ctx, `
INSERT INTO plugin_idc_core_billing_ledger_transaction
    (tenant_id, transaction_no, currency_code, transaction_type_code, posted_at)
VALUES (0, 'ledger-refund-2', 'CNY', 'refund', CURRENT_TIMESTAMP);
INSERT INTO plugin_idc_core_billing_refund_allocation
    (tenant_id, refund_allocation_no, invoice_no, invoice_item_no, customer_no,
     currency_code, direction_code, amount_minor, refund_source_type_code, refund_source_reference,
     ledger_transaction_no, ledger_transaction_type_code, content_digest)
VALUES
    (0, 'refund-allocation-3', 'invoice-1', 'item-1', 'customer-1', 'CNY',
     'apply', 50, 'manual', 'refund-operation-2', 'ledger-refund-2', 'refund', repeat('f', 64));
INSERT INTO plugin_idc_core_billing_ledger_transaction
    (tenant_id, transaction_no, currency_code, transaction_type_code, posted_at)
VALUES (0, 'ledger-refund-reversed', 'CNY', 'refund', CURRENT_TIMESTAMP);
INSERT INTO plugin_idc_core_billing_refund_allocation
    (tenant_id, refund_allocation_no, invoice_no, invoice_item_no, customer_no,
     currency_code, direction_code, amount_minor, refund_source_type_code, refund_source_reference,
     ledger_transaction_no, ledger_transaction_type_code, content_digest)
VALUES
    (0, 'refund-allocation-reversed', 'invoice-1', 'item-1', 'customer-1', 'CNY',
     'apply', 50, 'manual', 'refund-operation-reversed', 'ledger-refund-reversed', 'refund', repeat('1', 64));
INSERT INTO plugin_idc_core_billing_refund_allocation
    (tenant_id, invoice_no, invoice_item_no, direction_code, amount_minor,
     original_refund_allocation_no)
VALUES (0, 'invoice-1', 'item-1', 'reverse', 50, 'refund-allocation-reversed');`); err != nil {
		t.Fatalf("seed second posted refund source: %v", err)
	}
	if _, err = database.Exec(ctx, `
INSERT INTO plugin_idc_core_billing_ledger_transaction
    (tenant_id, transaction_no, currency_code, transaction_type_code, posted_at)
VALUES (0, 'ledger-refund-namespaced', 'CNY', 'refund', CURRENT_TIMESTAMP);
INSERT INTO plugin_idc_core_billing_refund_allocation
    (tenant_id, refund_allocation_no, invoice_no, invoice_item_no, customer_no,
     currency_code, direction_code, amount_minor, refund_source_type_code, refund_source_reference,
     ledger_transaction_no, ledger_transaction_type_code, content_digest)
VALUES
    (0, 'refund-allocation-namespaced', 'invoice-1', 'item-2', 'customer-1', 'CNY',
     'apply', 25, 'balance_credit', 'refund-operation-2',
     'ledger-refund-namespaced', 'refund', repeat('2', 64));`); err != nil {
		t.Fatalf("seed namespaced posted refund source: %v", err)
	}
	firstPage, err := service.ListPostedRefundOperations(ctx, "", 1)
	if err != nil || len(firstPage) != 1 ||
		firstPage[0].RefundOperationNo != "manual:refund-operation-1" || firstPage[0].Cursor == "" {
		t.Fatalf("unexpected first posted refund feed page: rows=%+v err=%v", firstPage, err)
	}
	secondPage, err := service.ListPostedRefundOperations(ctx, firstPage[0].Cursor, 1)
	if err != nil || len(secondPage) != 1 ||
		secondPage[0].RefundOperationNo != "manual:refund-operation-2" ||
		secondPage[0].Cursor == firstPage[0].Cursor {
		t.Fatalf("unexpected second posted refund feed page: rows=%+v err=%v", secondPage, err)
	}
	thirdPage, err := service.ListPostedRefundOperations(ctx, secondPage[0].Cursor, 1)
	if err != nil || len(thirdPage) != 1 ||
		thirdPage[0].RefundOperationNo != "balance_credit:refund-operation-2" {
		t.Fatalf("source namespaces were merged in the refund feed: rows=%+v err=%v", thirdPage, err)
	}
	fourthPage, err := service.ListPostedRefundOperations(ctx, thirdPage[0].Cursor, 1)
	if err != nil || len(fourthPage) != 0 {
		t.Fatalf("fully reversed refund remained in feed: rows=%+v err=%v", fourthPage, err)
	}
	if namespaced, namespacedErr := service.GetPostedRefund(
		ctx, "balance_credit:refund-operation-2",
	); namespacedErr != nil || len(namespaced.Allocations) != 1 || namespaced.Allocations[0].AmountMinor != 25 {
		t.Fatalf("namespaced posted refund=%+v err=%v", namespaced, namespacedErr)
	}
	if _, err = service.ListPostedRefundOperations(ctx, "not-a-cursor", 1); err == nil {
		t.Fatal("invalid posted refund feed cursor was accepted")
	}

	if _, err = database.Exec(ctx, `
INSERT INTO plugin_idc_core_billing_ledger_transaction
    (tenant_id, transaction_no, currency_code, transaction_type_code, posted_at)
VALUES (0, 'ledger-refund-mixed', 'CNY', 'refund', CURRENT_TIMESTAMP);
INSERT INTO plugin_idc_core_billing_refund_allocation
	    (tenant_id, refund_allocation_no, invoice_no, invoice_item_no, customer_no,
	     currency_code, direction_code, amount_minor, refund_source_type_code, refund_source_reference,
     ledger_transaction_no, ledger_transaction_type_code, content_digest)
VALUES
    (0, 'refund-mixed-1', 'invoice-1', 'item-1', 'customer-1', 'CNY',
	     'apply', 10, 'manual', 'refund-operation-mixed', 'ledger-refund-mixed', 'refund', repeat('c', 64)),
    (0, 'refund-mixed-2', 'invoice-other', 'item-other', 'customer-2', 'CNY',
	     'apply', 10, 'manual', 'refund-operation-mixed', 'ledger-refund-mixed', 'refund', repeat('d', 64));`); err != nil {
		t.Fatalf("seed mixed posted refund source: %v", err)
	}
	if _, err = service.GetPostedRefund(ctx, "manual:refund-operation-mixed"); err == nil {
		t.Fatal("mixed-customer posted refund was accepted")
	}

	if _, err = database.Exec(ctx, `
INSERT INTO plugin_idc_core_billing_ledger_transaction
    (tenant_id, transaction_no, currency_code, transaction_type_code, posted_at)
VALUES (0, 'ledger-refund-bounded', 'CNY', 'refund', CURRENT_TIMESTAMP);
INSERT INTO plugin_idc_core_billing_refund_allocation
	    (tenant_id, refund_allocation_no, invoice_no, invoice_item_no, customer_no,
	     currency_code, direction_code, amount_minor, refund_source_type_code, refund_source_reference,
     ledger_transaction_no, ledger_transaction_type_code, content_digest)
SELECT 0, 'refund-bound-' || value, 'invoice-1', 'item-1', 'customer-1', 'CNY',
	       'apply', 1, 'manual', 'refund-operation-bounded', 'ledger-refund-bounded', 'refund', repeat('e', 64)
FROM generate_series(1, 201) AS value;`); err != nil {
		t.Fatalf("seed bounded posted refund source: %v", err)
	}
	if _, err = service.GetPostedRefund(ctx, "manual:refund-operation-bounded"); err == nil {
		t.Fatal("oversized posted refund source was accepted")
	}
}

func findInvoiceOption(
	options []taxinvoicecap.InvoiceOption,
	invoiceNo string,
) *taxinvoicecap.InvoiceOption {
	for index := range options {
		if options[index].BillingInvoiceNo == invoiceNo {
			return &options[index]
		}
	}
	return nil
}

func newTaxInvoiceSourceTestDatabase(t *testing.T) (gdb.DB, func()) {
	t.Helper()
	ctx := context.Background()
	dsn := strings.TrimSpace(os.Getenv(taxInvoiceSourcePostgreSQLDSNEnvironment))
	if dsn == "" {
		t.Skipf("set %s to run tax invoice source PostgreSQL tests", taxInvoiceSourcePostgreSQLDSNEnvironment)
	}
	admin, err := gdb.New(gdb.ConfigNode{Link: dsn})
	if err != nil {
		t.Fatalf("open tax invoice source admin database: %v", err)
	}
	var randomValue [8]byte
	if _, err = rand.Read(randomValue[:]); err != nil {
		t.Fatal(err)
	}
	schema := "linapro_tax_invoice_source_test_" + hex.EncodeToString(randomValue[:])
	quoted := `"` + schema + `"`
	if _, err = admin.Exec(ctx, "CREATE SCHEMA "+quoted); err != nil {
		t.Fatal(err)
	}
	scopedDSN := dsn + "?search_path=" + schema
	if strings.Contains(dsn, "?") {
		scopedDSN = dsn + "&search_path=" + schema
	}
	database, err := gdb.New(gdb.ConfigNode{Link: scopedDSN})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(ctx, taxInvoiceSourceBaseSchema); err != nil {
		t.Fatalf("create tax invoice source schema: %v", err)
	}
	if _, err = database.Exec(ctx, taxInvoiceSourceFixtures); err != nil {
		t.Fatalf("seed tax invoice source fixtures: %v", err)
	}
	cleanup := func() {
		if closeErr := database.Close(ctx); closeErr != nil {
			t.Errorf("close tax invoice source database: %v", closeErr)
		}
		if !strings.HasPrefix(schema, "linapro_tax_invoice_source_test_") {
			t.Errorf("refusing unexpected schema %q", schema)
		} else if _, dropErr := admin.Exec(ctx, "DROP SCHEMA "+quoted+" CASCADE"); dropErr != nil {
			t.Errorf("drop tax invoice source schema: %v", dropErr)
		}
		if closeErr := admin.Close(ctx); closeErr != nil {
			t.Errorf("close tax invoice source admin database: %v", closeErr)
		}
	}
	return database, cleanup
}

const taxInvoiceSourceBaseSchema = `
CREATE TABLE plugin_idc_core_customer_invoice_profile (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    tenant_id INT NOT NULL, profile_no VARCHAR(40) NOT NULL,
    customer_no VARCHAR(40) NOT NULL, title_type_code VARCHAR(16) NOT NULL,
    invoice_title VARCHAR(512) NOT NULL, tax_registration_no VARCHAR(128) NOT NULL,
    registered_address VARCHAR(512) NOT NULL, registered_phone VARCHAR(32) NOT NULL,
    bank_name VARCHAR(256) NOT NULL, bank_account_masked VARCHAR(128) NOT NULL,
    bank_account_secret_ref VARCHAR(256) NOT NULL, is_default BOOLEAN NOT NULL,
    status_code VARCHAR(16) NOT NULL, version BIGINT NOT NULL
);
CREATE TABLE plugin_idc_core_merchant_profile (
    tenant_id INT NOT NULL, merchant_no VARCHAR(40) NOT NULL,
    legal_name VARCHAR(256) NOT NULL, country_code CHAR(2) NOT NULL,
    enabled BOOLEAN NOT NULL, version BIGINT NOT NULL
);
CREATE TABLE plugin_idc_core_billing_invoice (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    tenant_id INT NOT NULL, invoice_no VARCHAR(40) NOT NULL,
    customer_no VARCHAR(40) NOT NULL, order_no VARCHAR(40),
    currency_code CHAR(3) NOT NULL, status_code VARCHAR(24) NOT NULL,
    paid_amount_minor BIGINT NOT NULL, refunded_amount_minor BIGINT NOT NULL,
    content_digest CHAR(64) NOT NULL, settled_at TIMESTAMPTZ,
    cancelled_at TIMESTAMPTZ, version BIGINT NOT NULL
);
CREATE TABLE plugin_idc_core_billing_invoice_item (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    tenant_id INT NOT NULL, invoice_item_no VARCHAR(40) NOT NULL,
    invoice_no VARCHAR(40) NOT NULL, order_item_no VARCHAR(40),
    item_type_code VARCHAR(24) NOT NULL, source_reference VARCHAR(128) NOT NULL,
    line_number INT NOT NULL, description_zh_cn VARCHAR(256) NOT NULL,
    description_en_us VARCHAR(256) NOT NULL, net_amount_minor BIGINT NOT NULL,
    tax_amount_minor BIGINT NOT NULL, total_amount_minor BIGINT NOT NULL,
    content_digest CHAR(64) NOT NULL
);
CREATE TABLE plugin_idc_core_billing_invoice_item_derivation (
    tenant_id INT NOT NULL, destination_invoice_item_no VARCHAR(40) NOT NULL,
    source_invoice_item_no VARCHAR(40) NOT NULL
);
CREATE TABLE plugin_idc_core_billing_refund_allocation (
	id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    tenant_id INT NOT NULL, invoice_no VARCHAR(40) NOT NULL,
    invoice_item_no VARCHAR(40) NOT NULL, direction_code VARCHAR(8) NOT NULL,
	amount_minor BIGINT NOT NULL, refund_allocation_no VARCHAR(64),
	customer_no VARCHAR(64), currency_code CHAR(3),
	refund_source_type_code VARCHAR(24), refund_source_reference VARCHAR(64), ledger_transaction_no VARCHAR(64),
	ledger_transaction_type_code VARCHAR(32), content_digest CHAR(64),
	original_refund_allocation_no VARCHAR(64)
);
CREATE TABLE plugin_idc_core_billing_ledger_transaction (
	tenant_id INT NOT NULL, transaction_no VARCHAR(64) NOT NULL,
	currency_code CHAR(3) NOT NULL, transaction_type_code VARCHAR(32) NOT NULL,
	posted_at TIMESTAMPTZ NOT NULL
);
CREATE TABLE plugin_idc_core_commerce_order_item (
    tenant_id INT NOT NULL, order_item_no VARCHAR(40) NOT NULL,
    tax_treatment_code VARCHAR(16) NOT NULL, tax_rate_basis_points INT,
    tax_category_code VARCHAR(64) NOT NULL
);
CREATE TABLE plugin_idc_core_business_operation (
    tenant_id INT NOT NULL, invoice_no VARCHAR(40) NOT NULL,
    price_snapshot_no VARCHAR(40) NOT NULL
);
CREATE TABLE plugin_idc_core_service_price_snapshot (
    tenant_id INT NOT NULL, price_snapshot_no VARCHAR(40) NOT NULL,
    tax_treatment_code VARCHAR(16) NOT NULL, tax_rate_basis_points INT,
    tax_category_code VARCHAR(64) NOT NULL
);`

const taxInvoiceSourceFixtures = `
INSERT INTO plugin_idc_core_customer_invoice_profile
    (tenant_id, profile_no, customer_no, title_type_code, invoice_title,
     tax_registration_no, registered_address, registered_phone, bank_name,
     bank_account_masked, bank_account_secret_ref, is_default, status_code, version)
VALUES
    (0, 'profile-default', 'customer-1', 'enterprise', '客户企业',
     '913000000000000000', '注册地址', '01000000000', '开户银行',
     '****1234', 'secret://bank-1', TRUE, 'active', 3),
    (0, 'profile-second', 'customer-1', 'individual', '客户个人',
     '', '', '', '', '', '', FALSE, 'active', 1),
    (0, 'profile-other', 'customer-2', 'enterprise', '其他企业',
     '913000000000000001', '', '', '', '', '', TRUE, 'active', 1);
INSERT INTO plugin_idc_core_merchant_profile
    (tenant_id, merchant_no, legal_name, country_code, enabled, version)
VALUES (0, 'merchant-local', '本地 IDC 商家', 'CN', TRUE, 2);
INSERT INTO plugin_idc_core_billing_invoice
    (tenant_id, invoice_no, customer_no, order_no, currency_code, status_code,
     paid_amount_minor, refunded_amount_minor, content_digest, settled_at, version)
VALUES
    (0, 'invoice-1', 'customer-1', 'order-1', 'CNY', 'partially_refunded',
     1000, 100, repeat('a', 64), CURRENT_TIMESTAMP, 4),
	(0, 'invoice-renewal', 'customer-1', NULL, 'CNY', 'paid',
	 200, 0, repeat('1', 64), CURRENT_TIMESTAMP, 1),
	(0, 'invoice-derived', 'customer-1', NULL, 'CNY', 'paid',
	 100, 0, repeat('2', 64), CURRENT_TIMESTAMP, 1),
    (0, 'invoice-pending', 'customer-1', 'order-2', 'CNY', 'pending_payment',
     0, 0, repeat('b', 64), NULL, 1),
    (0, 'invoice-other', 'customer-2', 'order-3', 'CNY', 'paid',
     100, 0, repeat('c', 64), CURRENT_TIMESTAMP, 1);
INSERT INTO plugin_idc_core_billing_invoice_item
    (tenant_id, invoice_item_no, invoice_no, order_item_no, item_type_code,
     source_reference, line_number, description_zh_cn, description_en_us,
     net_amount_minor, tax_amount_minor, total_amount_minor, content_digest)
VALUES
    (0, 'item-1', 'invoice-1', 'order-item-1', 'order_item', 'order-item-1', 1,
     '服务器一', 'Server One', 566, 34, 600, repeat('d', 64)),
    (0, 'item-2', 'invoice-1', 'order-item-2', 'order_item', 'order-item-2', 2,
     '服务器二', 'Server Two', 377, 23, 400, repeat('e', 64)),
	(0, 'item-renewal', 'invoice-renewal', NULL, 'renewal', 'service-1', 1,
	 '续费服务器', 'Renewed Server', 188, 12, 200, repeat('1', 64)),
	(0, 'item-derived', 'invoice-derived', NULL, 'derived', 'item-2', 1,
	 '拆分服务器', 'Derived Server', 94, 6, 100, repeat('2', 64)),
    (0, 'item-other', 'invoice-other', 'order-item-other', 'order_item', 'order-item-other', 1,
     '其他服务器', 'Other Server', 94, 6, 100, repeat('f', 64));
INSERT INTO plugin_idc_core_commerce_order_item
    (tenant_id, order_item_no, tax_treatment_code, tax_rate_basis_points, tax_category_code)
VALUES
    (0, 'order-item-1', 'taxable', 600, 'cloud_service'),
    (0, 'order-item-2', 'taxable', 600, 'cloud_service'),
    (0, 'order-item-other', 'taxable', 600, 'cloud_service');
INSERT INTO plugin_idc_core_billing_invoice_item_derivation
	(tenant_id, destination_invoice_item_no, source_invoice_item_no)
VALUES (0, 'item-derived', 'item-2');
INSERT INTO plugin_idc_core_business_operation
	(tenant_id, invoice_no, price_snapshot_no)
VALUES (0, 'invoice-renewal', 'price-snapshot-1');
INSERT INTO plugin_idc_core_service_price_snapshot
	(tenant_id, price_snapshot_no, tax_treatment_code, tax_rate_basis_points, tax_category_code)
VALUES (0, 'price-snapshot-1', 'taxable', 600, 'cloud_service');
INSERT INTO plugin_idc_core_billing_refund_allocation
    (tenant_id, invoice_no, invoice_item_no, direction_code, amount_minor)
VALUES (0, 'invoice-1', 'item-1', 'apply', 100);`
