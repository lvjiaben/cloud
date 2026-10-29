// This file verifies deterministic settlement evidence and generic order,
// inventory, and trial quantity conservation with unit and PostgreSQL tests.

package billing

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gogf/gf/v2/database/gdb"

	"lina-plugin-idc-core/backend/internal/model/entity"
)

type financialSettlementCreditReservationStub struct{}

func (financialSettlementCreditReservationStub) ReserveCredit(
	context.Context,
	CreditReserveInput,
) (*CreditExposure, error) {
	return nil, errors.New("unexpected credit reservation")
}

type financialSettlementServiceStub struct{}

func (financialSettlementServiceStub) SettleOrder(
	context.Context,
	string,
) (*FinancialSettlement, error) {
	return nil, errors.New("unexpected order settlement")
}

func (financialSettlementServiceStub) SettleOrderWithCredit(
	context.Context,
	CreditOrderSettlementInput,
) (*FinancialSettlement, error) {
	return nil, errors.New("unexpected credit settlement")
}

func TestFinancialSettlementFinalizationIsDeterministic(t *testing.T) {
	settledAt := time.Date(2026, time.August, 11, 9, 30, 0, 123000, time.UTC)
	prepared := preparedFinancialSettlement{
		settlementNo: deterministicFinancialSettlementNo("ord_settlement_unit_0001"),
		orderNo:      "ord_settlement_unit_0001", invoiceNo: "inv_settlement_unit_0001",
		customerNo: "cus_settlement_unit_0001", currencyCode: "CNY",
		basis: SettlementBasisFundedInvoice, totalAmountMinor: 100,
		paidAmountMinor: 100, identityEvidenceDigest: strings.Repeat("a", 64),
		settledAt: settledAt,
		sources: []preparedFinancialSettlementSource{
			{
				sourceType: FinancialSettlementSourceBalance, sourceReference: "balance-payment-1",
				paymentAllocationNo: "bpa_settlement_unit_0001", amountMinor: 40,
				evidenceDigest: strings.Repeat("b", 64),
			},
			{
				sourceType:          FinancialSettlementSourceChannelPayment,
				sourceReference:     "channel-payment-1",
				paymentAllocationNo: "bpa_settlement_unit_0002", amountMinor: 60,
				evidenceDigest: strings.Repeat("c", 64),
			},
		},
	}
	if err := finalizePreparedFinancialSettlement(&prepared); err != nil {
		t.Fatalf("finalize settlement: %v", err)
	}
	if !canonicalDigestPattern.MatchString(prepared.sourceSnapshotDigest) ||
		!canonicalDigestPattern.MatchString(prepared.contentDigest) ||
		!financialSettlementBasisMatchesSources(prepared) {
		t.Fatalf("invalid finalized settlement: %+v", prepared)
	}
	firstDigest := prepared.contentDigest
	firstSourceNumbers := []string{
		prepared.sources[0].settlementSourceNo,
		prepared.sources[1].settlementSourceNo,
	}
	if err := finalizePreparedFinancialSettlement(&prepared); err != nil {
		t.Fatalf("refinalize settlement: %v", err)
	}
	if prepared.contentDigest != firstDigest ||
		prepared.sources[0].settlementSourceNo != firstSourceNumbers[0] ||
		prepared.sources[1].settlementSourceNo != firstSourceNumbers[1] {
		t.Fatalf("settlement finalization was not deterministic: %+v", prepared)
	}
	projection := projectPreparedFinancialSettlement(prepared, true)
	if projection.SettlementNo != prepared.settlementNo || !projection.Reused ||
		len(projection.Sources) != 2 || projection.PaidAmountMinor != 100 {
		t.Fatalf("unexpected settlement projection: %+v", projection)
	}
}

func TestFinancialSettlementBasisConservationRejectsInvalidSources(t *testing.T) {
	valid := preparedFinancialSettlement{
		basis: SettlementBasisFullDiscount,
		sources: []preparedFinancialSettlementSource{{
			sourceType: FinancialSettlementSourceFullDiscount,
		}},
	}
	if !financialSettlementBasisMatchesSources(valid) {
		t.Fatal("valid full-discount settlement was rejected")
	}
	invalid := valid
	invalid.sources = []preparedFinancialSettlementSource{{
		sourceType: FinancialSettlementSourceManualReceipt, amountMinor: 1,
	}}
	if financialSettlementBasisMatchesSources(invalid) {
		t.Fatal("payment evidence was accepted as a zero-value full discount")
	}
	invalid = preparedFinancialSettlement{
		basis: SettlementBasisFundedInvoice, totalAmountMinor: 100,
		paidAmountMinor: 99,
		sources: []preparedFinancialSettlementSource{{
			sourceType: FinancialSettlementSourceBalance, amountMinor: 99,
		}},
	}
	if financialSettlementBasisMatchesSources(invalid) {
		t.Fatal("underfunded invoice was accepted as financially settled")
	}
}

func TestNewFinancialSettlementServiceRejectsMissingDependencies(t *testing.T) {
	if _, err := newFinancialSettlementService(nil, nil, nil); err == nil {
		t.Fatal("financial settlement service accepted missing dependencies")
	}
}

func TestFinancialSettlementChannelEvidenceRequiresSameSourceReference(t *testing.T) {
	invoice := &entity.BillingInvoice{
		InvoiceNo: "inv_settlement_unit_channel", CustomerNo: "cus_settlement_unit_channel",
		CurrencyCode: "CNY",
	}
	row := settlementAllocationEvidenceRow{
		AllocationNo: "bpa_settlement_unit_channel", InvoiceNo: invoice.InvoiceNo,
		CustomerNo: invoice.CustomerNo, CurrencyCode: invoice.CurrencyCode,
		SourceType: string(PaymentSourceChannel), SourceReference: "channel-attempt-0001",
		LedgerTransactionNo:   "btx_settlement_unit_channel",
		LedgerTransactionType: paymentLedgerType(PaymentSourceChannel), Direction: "apply",
		AmountMinor: 100, AllocationDigest: strings.Repeat("a", 64),
		TransactionSourceType:      "payment_attempt",
		TransactionSourceReference: "channel-attempt-0001",
		TransactionSnapshotDigest:  strings.Repeat("b", 64),
		TransactionContentDigest:   strings.Repeat("c", 64),
		TransactionEntryCount:      2, TransactionDebitMinor: 100,
		TransactionCreditMinor: 100,
	}
	if err := validateSettlementAllocationBase(invoice, row); err != nil {
		t.Fatalf("valid channel evidence was rejected: %v", err)
	}
	if err := validateSettlementApplySemantics(row); err != nil {
		t.Fatalf("valid channel source semantics were rejected: %v", err)
	}
	row.TransactionSourceReference = "different-channel-attempt"
	if err := validateSettlementAllocationBase(invoice, row); err == nil {
		t.Fatal("channel evidence accepted a mismatched ledger source reference")
	}
}

func TestFinancialSettlementOrderItemsUseGenericQuantity(t *testing.T) {
	order := &entity.CommerceOrder{
		ItemCount: 2, SubtotalMinor: 600, DiscountMinor: 50,
		NetAmountMinor: 550, TaxAmountMinor: 0, TotalAmountMinor: 550,
	}
	summary := settlementOrderItemSummary{
		ItemCount: 2, TotalQuantity: 5, SubtotalMinor: 600,
		DiscountMinor: 50, NetAmountMinor: 550, TaxAmountMinor: 0,
		TotalAmountMinor: 550,
	}
	if !financialSettlementOrderItemsMatch(summary, order) {
		t.Fatal("valid non-server generic quantities were rejected")
	}
	summary.TotalQuantity = 0
	if financialSettlementOrderItemsMatch(summary, order) {
		t.Fatal("empty generic quantity snapshot was accepted")
	}
}

func TestFinancialSettlementInventoryUsesGenericQuantity(t *testing.T) {
	items := settlementOrderItemSummary{ItemCount: 2, TotalQuantity: 5}
	inventory := settlementInventorySummary{
		SnapshotCount: 2, TrackedItemCount: 1, TrackedQuantity: 4,
		ReservationCount: 1, HeldReservations: 1, ReservedQuantity: 4,
		MatchingReservations: 1,
	}
	if !financialSettlementInventoryMatches(inventory, items) {
		t.Fatal("mixed tracked and untracked generic inventory was rejected")
	}
	inventory.ReservedQuantity = 1
	if financialSettlementInventoryMatches(inventory, items) {
		t.Fatal("a tracked reservation with the wrong generic quantity was accepted")
	}
	inventory = settlementInventorySummary{SnapshotCount: 2}
	if !financialSettlementInventoryMatches(inventory, items) {
		t.Fatal("inventory-disabled order items incorrectly required reservations")
	}
	inventory.ReservationCount = 1
	if financialSettlementInventoryMatches(inventory, items) {
		t.Fatal("inventory-disabled order items accepted an unexpected reservation")
	}
}

func TestTrialFinancialSettlementUsesGenericQuantity(t *testing.T) {
	items := settlementOrderItemSummary{
		ItemCount: 2, TrialCount: 2, TotalQuantity: 5,
	}
	if !trialFinancialSettlementItemsEligible(items) {
		t.Fatal("valid multi-item generic trial quantities were rejected")
	}
	items.TotalQuantity = maximumFinancialSettlementSources + 1
	if trialFinancialSettlementItemsEligible(items) {
		t.Fatal("unbounded generic trial quantity was accepted")
	}
}

func TestFinancialSettlementGenericInventoryPostgreSQL(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("LINAPRO_IDC_CORE_TEST_DSN"))
	if dsn == "" {
		t.Skip("set LINAPRO_IDC_CORE_TEST_DSN to run PostgreSQL settlement tests")
	}
	ctx := context.Background()
	admin, err := gdb.New(gdb.ConfigNode{Link: dsn})
	if err != nil {
		t.Fatalf("create settlement test database: %v", err)
	}
	t.Cleanup(func() {
		if closeErr := admin.Close(ctx); closeErr != nil {
			t.Errorf("close settlement admin database: %v", closeErr)
		}
	})
	var suffix [8]byte
	if _, err = rand.Read(suffix[:]); err != nil {
		t.Fatalf("generate settlement test schema: %v", err)
	}
	schema := "linapro_billing_settlement_" + hex.EncodeToString(suffix[:])
	if _, err = admin.Exec(ctx, `CREATE SCHEMA "`+schema+`"`); err != nil {
		t.Fatalf("create settlement test schema: %v", err)
	}
	t.Cleanup(func() {
		if _, dropErr := admin.Exec(ctx, `DROP SCHEMA "`+schema+`" CASCADE`); dropErr != nil {
			t.Errorf("drop settlement test schema: %v", dropErr)
		}
	})
	separator := "?"
	if strings.Contains(dsn, "?") {
		separator = "&"
	}
	database, err := gdb.New(gdb.ConfigNode{Link: dsn + separator + "search_path=" + schema})
	if err != nil {
		t.Fatalf("create scoped settlement test database: %v", err)
	}
	t.Cleanup(func() {
		if closeErr := database.Close(ctx); closeErr != nil {
			t.Errorf("close scoped settlement database: %v", closeErr)
		}
	})
	if _, err = database.Exec(ctx, financialSettlementGenericInventoryFixtureSQL); err != nil {
		t.Fatalf("create generic settlement fixture: %v", err)
	}
	issuedAt := time.Now().UTC()
	order := &entity.CommerceOrder{
		OrderNo: "order-generic", QuoteNo: "quote-generic", CustomerNo: "customer-generic",
		CurrencyCode: "CNY", ItemCount: 2, SubtotalMinor: 500, NetAmountMinor: 500,
		TotalAmountMinor: 500, RiskDecisionCode: "approved", StatusCode: "pending_payment",
	}
	invoiceOrderNo := order.OrderNo
	invoice := &entity.BillingInvoice{
		InvoiceNo: "invoice-generic", OrderNo: &invoiceOrderNo, SourceCode: "order",
		CustomerNo: order.CustomerNo, CurrencyCode: order.CurrencyCode, ItemCount: 2,
		SubtotalMinor: 500, NetAmountMinor: 500, TotalAmountMinor: 500,
		IssuedAt: &issuedAt,
	}
	summary, err := validateFinancialSettlementCommercialFacts(ctx, database, order, invoice)
	if err != nil || summary.TotalQuantity != 5 {
		t.Fatalf("validate mixed generic inventory summary=%+v err=%v", summary, err)
	}
	if _, err = database.Exec(ctx, `
INSERT INTO plugin_idc_core_commerce_inventory_reservation(
    tenant_id, order_no, order_item_no, reserved_quantity, status_code
) VALUES (0, 'order-generic', 'item-untracked', 1, 'held')`); err != nil {
		t.Fatalf("insert invalid untracked reservation: %v", err)
	}
	if _, err = validateFinancialSettlementCommercialFacts(ctx, database, order, invoice); err == nil {
		t.Fatal("settlement accepted a reservation for inventory-disabled goods")
	}
}

const financialSettlementGenericInventoryFixtureSQL = `
CREATE TABLE plugin_idc_core_commerce_order_item(
    tenant_id INT NOT NULL, order_no TEXT NOT NULL, order_item_no TEXT NOT NULL,
    charge_type_code TEXT NOT NULL, subtotal_minor BIGINT NOT NULL,
    discount_minor BIGINT NOT NULL, net_amount_minor BIGINT NOT NULL,
    tax_amount_minor BIGINT NOT NULL, total_amount_minor BIGINT NOT NULL
);
CREATE TABLE plugin_idc_core_commerce_order_item_quantity_snapshot(
    tenant_id INT NOT NULL, order_no TEXT NOT NULL, order_item_no TEXT NOT NULL,
    quantity INT NOT NULL
);
CREATE TABLE plugin_idc_core_billing_invoice_item(
    tenant_id INT NOT NULL, invoice_no TEXT NOT NULL, subtotal_minor BIGINT NOT NULL,
    discount_minor BIGINT NOT NULL, net_amount_minor BIGINT NOT NULL,
    tax_amount_minor BIGINT NOT NULL, total_amount_minor BIGINT NOT NULL
);
CREATE TABLE plugin_idc_core_commerce_quote_snapshot(
    tenant_id INT NOT NULL, quote_no TEXT NOT NULL, customer_no TEXT NOT NULL,
    purpose_code TEXT NOT NULL
);
CREATE TABLE plugin_idc_core_commerce_order_item_inventory_snapshot(
    order_item_no TEXT PRIMARY KEY, tracking_enabled BOOLEAN NOT NULL
);
CREATE TABLE plugin_idc_core_commerce_inventory_reservation(
    tenant_id INT NOT NULL, order_no TEXT NOT NULL, order_item_no TEXT NOT NULL,
    reserved_quantity INT NOT NULL, status_code TEXT NOT NULL
);
INSERT INTO plugin_idc_core_commerce_order_item VALUES
    (0,'order-generic','item-tracked','paid',400,0,400,0,400),
    (0,'order-generic','item-untracked','paid',100,0,100,0,100);
INSERT INTO plugin_idc_core_commerce_order_item_quantity_snapshot VALUES
    (0,'order-generic','item-tracked',4),
    (0,'order-generic','item-untracked',1);
INSERT INTO plugin_idc_core_billing_invoice_item VALUES
    (0,'invoice-generic',400,0,400,0,400),
    (0,'invoice-generic',100,0,100,0,100);
INSERT INTO plugin_idc_core_commerce_quote_snapshot
    VALUES (0,'quote-generic','customer-generic','purchase');
INSERT INTO plugin_idc_core_commerce_order_item_inventory_snapshot VALUES
    ('item-tracked',TRUE),('item-untracked',FALSE);
INSERT INTO plugin_idc_core_commerce_inventory_reservation
    VALUES (0,'order-generic','item-tracked',4,'held');
`

// Optional documents must not prevent a real, authorized manual receipt from
// settling its invoice; malformed supplied references and missing actors fail.
func TestManualReceiptSettlementAcceptsOptionalDocuments(t *testing.T) {
	var (
		digest   = strings.Repeat("a", 64)
		required = false
		actor    = 901
		purpose  = adjustmentTypeManualReceipt
		now      = time.Now()
	)
	row := settlementAllocationEvidenceRow{AdjustmentEvidenceDigest: &digest, AdjustmentRequestDigest: &digest, AdjustmentApprovalRequired: &required, AdjustmentRequestedBy: &actor, AdjustmentBusinessPurpose: &purpose, AdjustmentPostedAt: &now}
	if err := validateManualReceiptSettlementReview(row); err != nil {
		t.Fatalf("receipt without optional documents: %v", err)
	}
	badReference := " "
	row.AdjustmentProofReference = &badReference
	if err := validateManualReceiptSettlementReview(row); err == nil {
		t.Fatal("malformed supplied proof accepted")
	}
	row.AdjustmentProofReference = nil
	row.AdjustmentRequestedBy = nil
	if err := validateManualReceiptSettlementReview(row); err == nil {
		t.Fatal("receipt without operator accepted")
	}
}
