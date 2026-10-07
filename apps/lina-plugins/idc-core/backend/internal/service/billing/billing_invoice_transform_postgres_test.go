// This file verifies invoice merge and whole-item split evidence against an
// isolated real PostgreSQL schema.

package billing

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/gogf/gf/v2/database/gdb"

	"lina-core/pkg/bizerr"
)

func TestInvoiceTransformPostgreSQLMergesWithoutRewritingSources(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	database := configureBillingServiceIntegrationDB(t, ctx)
	seedBillingService(t, ctx, database)
	seedInvoiceTransform(t, ctx, database)
	service, err := NewInvoiceTransformService(database)
	if err != nil {
		t.Fatalf("create invoice transform service: %v", err)
	}

	input := InvoiceMergeInput{
		InvoiceNos:     []string{"merge-invoice-b", "merge-invoice-a"},
		IdempotencyKey: "merge-idempotency-0001", RequestedBy: 101,
	}
	merged, err := service.MergeInvoices(ctx, input)
	if err != nil || merged == nil || merged.Reused || merged.Type != InvoiceTransformMerge ||
		merged.CustomerNo != "customer-ledger" || merged.CurrencyCode != "CNY" ||
		len(merged.SourceInvoiceNos) != 2 || len(merged.DestinationInvoices) != 1 ||
		merged.DestinationInvoices[0].TotalAmountMinor != 1_450 {
		t.Fatalf("merge invoices: result=%+v err=%v", merged, err)
	}
	if strings.Join(merged.SourceInvoiceNos, ",") != "merge-invoice-a,merge-invoice-b" {
		t.Fatalf("merge source order=%v", merged.SourceInvoiceNos)
	}
	replay, err := service.MergeInvoices(ctx, input)
	if err != nil || replay == nil || !replay.Reused || replay.TransformNo != merged.TransformNo ||
		replay.DestinationInvoices[0].InvoiceNo != merged.DestinationInvoices[0].InvoiceNo {
		t.Fatalf("replay invoice merge: result=%+v err=%v", replay, err)
	}
	changed := input
	changed.InvoiceNos = []string{"merge-invoice-a", "split-invoice"}
	if _, err = service.MergeInvoices(ctx, changed); !bizerr.Is(err, CodeInvoiceConflict) {
		t.Fatalf("changed merge replay should conflict: %v", err)
	}
	if _, err = service.MergeInvoices(ctx, InvoiceMergeInput{
		InvoiceNos: input.InvoiceNos, IdempotencyKey: "merge-idempotency-0002", RequestedBy: 101,
	}); !bizerr.Is(err, CodeInvoiceConflict) {
		t.Fatalf("cancelled sources were transformed twice: %v", err)
	}
	assertInvoiceMergeEvidence(t, ctx, database, merged)
	if _, err = database.Exec(ctx, `
UPDATE plugin_idc_core_billing_invoice_transform
SET requested_by = requested_by + 1 WHERE transform_no = ?`, merged.TransformNo); err == nil {
		t.Fatal("immutable invoice transform accepted mutation")
	}
}

func TestInvoiceTransformPostgreSQLSplitsEveryWholeItemOnce(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	database := configureBillingServiceIntegrationDB(t, ctx)
	seedBillingService(t, ctx, database)
	seedInvoiceTransform(t, ctx, database)
	service, err := NewInvoiceTransformService(database)
	if err != nil {
		t.Fatalf("create invoice transform service: %v", err)
	}

	if _, err = service.SplitInvoice(ctx, InvoiceSplitInput{
		InvoiceNo: "split-invoice", IdempotencyKey: "split-missing-item-0001", RequestedBy: 102,
		Groups: []InvoiceSplitGroup{{InvoiceItemNos: []string{"split-item-a"}}, {InvoiceItemNos: []string{"split-item-b"}}},
	}); !bizerr.Is(err, CodeInvoiceConflict) {
		t.Fatalf("split omitted source item: %v", err)
	}
	if _, err = service.SplitInvoice(ctx, InvoiceSplitInput{
		InvoiceNo: "split-invoice", IdempotencyKey: "split-duplicate-item-1", RequestedBy: 102,
		Groups: []InvoiceSplitGroup{{InvoiceItemNos: []string{"split-item-a"}}, {InvoiceItemNos: []string{"split-item-a", "split-item-c"}}},
	}); !bizerr.Is(err, CodeInvoiceInvalid) {
		t.Fatalf("split duplicated source item: %v", err)
	}

	input := InvoiceSplitInput{
		InvoiceNo: "split-invoice", IdempotencyKey: "split-idempotency-0001", RequestedBy: 102,
		Groups: []InvoiceSplitGroup{
			{InvoiceItemNos: []string{"split-item-c", "split-item-a"}},
			{InvoiceItemNos: []string{"split-item-b"}},
		},
	}
	split, err := service.SplitInvoice(ctx, input)
	if err != nil || split == nil || split.Reused || split.Type != InvoiceTransformSplit ||
		len(split.SourceInvoiceNos) != 1 || split.SourceInvoiceNos[0] != "split-invoice" ||
		len(split.DestinationInvoices) != 2 {
		t.Fatalf("split invoice: result=%+v err=%v", split, err)
	}
	totals := []int64{
		split.DestinationInvoices[0].TotalAmountMinor,
		split.DestinationInvoices[1].TotalAmountMinor,
	}
	sort.Slice(totals, func(left int, right int) bool { return totals[left] < totals[right] })
	if totals[0] != 200 || totals[1] != 400 {
		t.Fatalf("split destination totals=%v", totals)
	}
	replay, err := service.SplitInvoice(ctx, input)
	if err != nil || replay == nil || !replay.Reused || replay.TransformNo != split.TransformNo {
		t.Fatalf("replay invoice split: result=%+v err=%v", replay, err)
	}
	assertInvoiceSplitEvidence(t, ctx, database, split)
}

func TestInvoiceTransformPostgreSQLSerializesConcurrentReplay(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	database := configureBillingServiceIntegrationDB(t, ctx)
	seedBillingService(t, ctx, database)
	seedInvoiceTransform(t, ctx, database)
	service, err := NewInvoiceTransformService(database)
	if err != nil {
		t.Fatalf("create invoice transform service: %v", err)
	}
	input := InvoiceMergeInput{
		InvoiceNos:     []string{"merge-invoice-a", "merge-invoice-b"},
		IdempotencyKey: "merge-concurrency-0001", RequestedBy: 103,
	}
	const workers = 8
	results := make([]*InvoiceTransform, workers)
	errors := make([]error, workers)
	start := make(chan struct{})
	var wait sync.WaitGroup
	for index := range results {
		wait.Add(1)
		go func(index int) {
			defer wait.Done()
			<-start
			results[index], errors[index] = service.MergeInvoices(ctx, input)
		}(index)
	}
	close(start)
	wait.Wait()
	created := 0
	transformNo := ""
	for index, result := range results {
		if errors[index] != nil || result == nil {
			t.Fatalf("concurrent merge[%d]: result=%+v err=%v", index, result, errors[index])
		}
		if transformNo == "" {
			transformNo = result.TransformNo
		} else if result.TransformNo != transformNo {
			t.Fatalf("concurrent merge produced %q and %q", transformNo, result.TransformNo)
		}
		if !result.Reused {
			created++
		}
	}
	if created != 1 {
		t.Fatalf("concurrent merge created count=%d want=1", created)
	}
	count, err := database.GetValue(ctx, `
SELECT count(*) FROM plugin_idc_core_billing_invoice_transform
WHERE transform_no = ?`, transformNo)
	if err != nil || count.Int() != 1 {
		t.Fatalf("concurrent merge header count=%d err=%v", count.Int(), err)
	}
}

func TestInvoiceTransformPostgreSQLRejectsCrossOwnerOrCurrency(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	database := configureBillingServiceIntegrationDB(t, ctx)
	seedBillingService(t, ctx, database)
	seedInvoiceTransform(t, ctx, database)
	service, err := NewInvoiceTransformService(database)
	if err != nil {
		t.Fatalf("create invoice transform service: %v", err)
	}
	for name, invoiceNos := range map[string][]string{
		"customer": {"merge-invoice-a", "other-customer-invoice"},
		"currency": {"merge-invoice-a", "other-currency-invoice"},
		"status":   {"merge-invoice-a", "draft-invoice"},
	} {
		t.Run(name, func(t *testing.T) {
			_, mergeErr := service.MergeInvoices(ctx, InvoiceMergeInput{
				InvoiceNos: invoiceNos, IdempotencyKey: "reject-" + name + "-invoice-0001", RequestedBy: 104,
			})
			if !bizerr.Is(mergeErr, CodeInvoiceConflict) {
				t.Fatalf("unsafe merge accepted: %v", mergeErr)
			}
		})
	}
}

func TestInvoiceTransformPostgreSQLRefundsDerivedMergeAndSplitItems(t *testing.T) {
	t.Run("merge", func(t *testing.T) {
		ctx := context.Background()
		database := configureBillingServiceIntegrationDB(t, ctx)
		seedBillingService(t, ctx, database)
		seedInvoiceTransform(t, ctx, database)
		transforms, err := NewInvoiceTransformService(database)
		if err != nil {
			t.Fatal(err)
		}
		merged, err := transforms.MergeInvoices(ctx, InvoiceMergeInput{
			InvoiceNos:     []string{"merge-invoice-a", "merge-invoice-b"},
			IdempotencyKey: "merge-refund-idempotency-0001", RequestedBy: 105,
		})
		if err != nil || merged == nil || len(merged.DestinationInvoices) != 1 {
			t.Fatalf("merge before refund: result=%+v err=%v", merged, err)
		}
		refundTransformedInvoice(t, ctx, database,
			merged.DestinationInvoices[0].InvoiceNo, "merge-derived-refund", true)
		assertTransformRefundEvidence(t, ctx, database, merged.TransformNo,
			[]string{"merge-invoice-a", "merge-invoice-b"}, 3, 4)
	})

	t.Run("split", func(t *testing.T) {
		ctx := context.Background()
		database := configureBillingServiceIntegrationDB(t, ctx)
		seedBillingService(t, ctx, database)
		seedInvoiceTransform(t, ctx, database)
		transforms, err := NewInvoiceTransformService(database)
		if err != nil {
			t.Fatal(err)
		}
		split, err := transforms.SplitInvoice(ctx, InvoiceSplitInput{
			InvoiceNo: "split-invoice", IdempotencyKey: "split-refund-idempotency-0001",
			RequestedBy: 106,
			Groups: []InvoiceSplitGroup{
				{InvoiceItemNos: []string{"split-item-a", "split-item-c"}},
				{InvoiceItemNos: []string{"split-item-b"}},
			},
		})
		if err != nil || split == nil || len(split.DestinationInvoices) != 2 {
			t.Fatalf("split before refund: result=%+v err=%v", split, err)
		}
		for index, destination := range split.DestinationInvoices {
			refundTransformedInvoice(t, ctx, database, destination.InvoiceNo,
				fmt.Sprintf("split-derived-refund-%02d", index+1), index == 0)
		}
		assertTransformRefundEvidence(t, ctx, database, split.TransformNo,
			[]string{"split-invoice"}, 3, 4)
	})
}

func refundTransformedInvoice(
	t *testing.T,
	ctx context.Context,
	database gdb.DB,
	invoiceNo string,
	suffix string,
	splitFirstItem bool,
) {
	t.Helper()
	var invoice struct {
		TotalMinor int64 `orm:"total_amount_minor"`
	}
	if err := database.GetScan(ctx, &invoice, `
SELECT total_amount_minor FROM plugin_idc_core_billing_invoice
WHERE invoice_no=?`, invoiceNo); err != nil || invoice.TotalMinor <= 0 {
		t.Fatalf("load transformed invoice %s: invoice=%+v err=%v", invoiceNo, invoice, err)
	}
	balance, err := NewBalanceService(database)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = balance.PostConfirmedRecharge(ctx, ConfirmedRechargeInput{
		CustomerNo: "customer-ledger", CurrencyCode: "CNY", AmountMinor: invoice.TotalMinor,
		SourceReference: suffix + "-recharge", SourceSnapshotDigest: strings.Repeat("7", 64),
	}); err != nil {
		t.Fatalf("fund transformed invoice %s: %v", invoiceNo, err)
	}
	invoices, err := NewInvoiceService(database)
	if err != nil {
		t.Fatal(err)
	}
	paid, err := invoices.PayFromBalance(ctx, InvoiceBalancePaymentInput{
		InvoiceNo: invoiceNo, SourceReference: suffix + "-payment",
		SourceSnapshotDigest: strings.Repeat("8", 64),
	})
	if err != nil || paid == nil || paid.Invoice == nil ||
		paid.Invoice.Status != InvoiceStatusPaid || paid.Invoice.AllocationNo == "" {
		t.Fatalf("pay transformed invoice %s: result=%+v err=%v", invoiceNo, paid, err)
	}
	var items []struct {
		InvoiceItemNo string `orm:"invoice_item_no"`
		Refundable    int64  `orm:"refundable_cap_minor"`
	}
	if err = database.GetScan(ctx, &items, `
SELECT invoice_item_no,refundable_cap_minor
FROM plugin_idc_core_billing_invoice_item
WHERE invoice_no=? ORDER BY line_number,id`, invoiceNo); err != nil || len(items) == 0 {
		t.Fatalf("load transformed refund items %s: items=%+v err=%v", invoiceNo, items, err)
	}
	sequence := 0
	for itemIndex, item := range items {
		tranches := []int64{item.Refundable}
		if splitFirstItem && itemIndex == 0 && item.Refundable > 1 {
			tranches = []int64{item.Refundable / 2, item.Refundable - item.Refundable/2}
		}
		for _, amount := range tranches {
			sequence++
			transactionNo := postTestRefundLedger(
				t, ctx, balance.(*serviceImpl), amount,
				fmt.Sprintf("%s-ledger-%02d", suffix, sequence),
			)
			if _, err = invoices.ApplyRefund(ctx, InvoiceRefundInput{
				InvoiceNo: invoiceNo, InvoiceItemNo: item.InvoiceItemNo,
				PaymentAllocationNo: paid.Invoice.AllocationNo,
				SourceType:          RefundSourceBalanceCredit,
				SourceReference:     fmt.Sprintf("%s-allocation-%02d", suffix, sequence),
				LedgerTransactionNo: transactionNo, AmountMinor: amount,
			}); err != nil {
				t.Fatalf("refund transformed invoice %s item %s tranche %d: %v",
					invoiceNo, item.InvoiceItemNo, sequence, err)
			}
		}
	}
	var final struct {
		Status          string `orm:"status_code"`
		RefundedMinor   int64  `orm:"refunded_amount_minor"`
		RefundableMinor int64  `orm:"refundable_amount_minor"`
		RefundCap       int64  `orm:"refund_cap"`
	}
	if err = database.GetScan(ctx, &final, `
SELECT invoice.status_code,invoice.refunded_amount_minor,invoice.refundable_amount_minor,
       (SELECT SUM(item.refundable_cap_minor)
        FROM plugin_idc_core_billing_invoice_item item
        WHERE item.invoice_no=invoice.invoice_no) AS refund_cap
FROM plugin_idc_core_billing_invoice invoice
WHERE invoice.invoice_no=?`, invoiceNo); err != nil {
		t.Fatalf("load transformed refund result %s: %v", invoiceNo, err)
	}
	if final.Status != string(InvoiceStatusRefunded) || final.RefundableMinor != 0 ||
		final.RefundedMinor != final.RefundCap {
		t.Fatalf("unexpected transformed refund result %s: %+v", invoiceNo, final)
	}
}

func assertTransformRefundEvidence(
	t *testing.T,
	ctx context.Context,
	database gdb.DB,
	transformNo string,
	sourceInvoiceNos []string,
	expectedDerivations int64,
	expectedRefundAllocations int64,
) {
	t.Helper()
	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(sourceInvoiceNos)), ",")
	args := make([]any, 0, len(sourceInvoiceNos)+1)
	args = append(args, transformNo)
	for _, source := range sourceInvoiceNos {
		args = append(args, source)
	}
	var evidence struct {
		Derivations       int64 `orm:"derivations"`
		RefundAllocations int64 `orm:"refund_allocations"`
		SourceAllocations int64 `orm:"source_allocations"`
	}
	query := fmt.Sprintf(`
SELECT
  (SELECT COUNT(*) FROM plugin_idc_core_billing_invoice_item_derivation
   WHERE transform_no=?) AS derivations,
  (SELECT COUNT(*)
   FROM plugin_idc_core_billing_refund_allocation refund
   JOIN plugin_idc_core_billing_invoice_transform_destination destination
     ON destination.destination_invoice_no=refund.invoice_no
   WHERE destination.transform_no=?) AS refund_allocations,
  (SELECT COUNT(*) FROM plugin_idc_core_billing_payment_allocation
   WHERE invoice_no IN (%s)) AS source_allocations`, placeholders)
	queryArgs := make([]any, 0, len(args)+1)
	queryArgs = append(queryArgs, transformNo, transformNo)
	queryArgs = append(queryArgs, args[1:]...)
	if err := database.GetScan(ctx, &evidence, query, queryArgs...); err != nil {
		t.Fatalf("load transformed refund evidence: %v", err)
	}
	if evidence.Derivations != expectedDerivations ||
		evidence.RefundAllocations != expectedRefundAllocations ||
		evidence.SourceAllocations != 0 {
		t.Fatalf("unexpected transformed refund evidence: %+v", evidence)
	}
}

func assertInvoiceMergeEvidence(
	t *testing.T,
	ctx context.Context,
	database gdb.DB,
	result *InvoiceTransform,
) {
	t.Helper()
	var state struct {
		CancelledSources int   `orm:"cancelled_sources"`
		SourceItems      int   `orm:"source_items"`
		DerivedItems     int   `orm:"derived_items"`
		Derivations      int   `orm:"derivations"`
		SourceTotal      int64 `orm:"source_total"`
		DestinationTotal int64 `orm:"destination_total"`
	}
	if err := database.GetScan(ctx, &state, `
SELECT
  (SELECT count(*) FROM plugin_idc_core_billing_invoice
   WHERE invoice_no IN ('merge-invoice-a', 'merge-invoice-b') AND status_code = 'cancelled') AS cancelled_sources,
  (SELECT count(*) FROM plugin_idc_core_billing_invoice_item
   WHERE invoice_no IN ('merge-invoice-a', 'merge-invoice-b') AND item_type_code = 'manual') AS source_items,
  (SELECT count(*) FROM plugin_idc_core_billing_invoice_item
   WHERE invoice_no = ?) AS derived_items,
  (SELECT count(*) FROM plugin_idc_core_billing_invoice_item_derivation
   WHERE transform_no = ?) AS derivations,
  (SELECT sum(total_amount_minor) FROM plugin_idc_core_billing_invoice
   WHERE invoice_no IN ('merge-invoice-a', 'merge-invoice-b')) AS source_total,
  (SELECT total_amount_minor FROM plugin_idc_core_billing_invoice
   WHERE invoice_no = ?) AS destination_total`,
		result.DestinationInvoices[0].InvoiceNo, result.TransformNo,
		result.DestinationInvoices[0].InvoiceNo); err != nil {
		t.Fatalf("load merge evidence: %v", err)
	}
	if state.CancelledSources != 2 || state.SourceItems != 3 || state.DerivedItems != 3 ||
		state.Derivations != 3 || state.SourceTotal != 1_450 || state.DestinationTotal != 1_450 {
		t.Fatalf("merge evidence=%+v", state)
	}
}

func assertInvoiceSplitEvidence(
	t *testing.T,
	ctx context.Context,
	database gdb.DB,
	result *InvoiceTransform,
) {
	t.Helper()
	var state struct {
		SourceCancelled int   `orm:"source_cancelled"`
		SourceItems     int   `orm:"source_items"`
		DerivedItems    int   `orm:"derived_items"`
		Derivations     int   `orm:"derivations"`
		SourceTotal     int64 `orm:"source_total"`
		DestinationSum  int64 `orm:"destination_sum"`
	}
	if err := database.GetScan(ctx, &state, `
SELECT
  (SELECT count(*) FROM plugin_idc_core_billing_invoice
   WHERE invoice_no = 'split-invoice' AND status_code = 'cancelled') AS source_cancelled,
  (SELECT count(*) FROM plugin_idc_core_billing_invoice_item
   WHERE invoice_no = 'split-invoice' AND item_type_code = 'manual') AS source_items,
  (SELECT count(*) FROM plugin_idc_core_billing_invoice_item_derivation derivation
   JOIN plugin_idc_core_billing_invoice_item item
     ON item.invoice_item_no = derivation.destination_invoice_item_no
   WHERE derivation.transform_no = ?) AS derived_items,
  (SELECT count(*) FROM plugin_idc_core_billing_invoice_item_derivation
   WHERE transform_no = ?) AS derivations,
  (SELECT total_amount_minor FROM plugin_idc_core_billing_invoice
   WHERE invoice_no = 'split-invoice') AS source_total,
  (SELECT sum(invoice.total_amount_minor)
   FROM plugin_idc_core_billing_invoice_transform_destination destination
   JOIN plugin_idc_core_billing_invoice invoice
     ON invoice.invoice_no = destination.destination_invoice_no
   WHERE destination.transform_no = ?) AS destination_sum`,
		result.TransformNo, result.TransformNo, result.TransformNo); err != nil {
		t.Fatalf("load split evidence: %v", err)
	}
	if state.SourceCancelled != 1 || state.SourceItems != 3 || state.DerivedItems != 3 ||
		state.Derivations != 3 || state.SourceTotal != 600 || state.DestinationSum != 600 {
		t.Fatalf("split evidence=%+v", state)
	}
}

func seedInvoiceTransform(t *testing.T, ctx context.Context, database gdb.DB) {
	t.Helper()
	if _, err := database.Exec(ctx, `
INSERT INTO plugin_idc_core_billing_invoice
    (invoice_no, customer_no, source_code, currency_code, status_code, item_count,
     subtotal_minor, discount_minor, net_amount_minor, tax_amount_minor, total_amount_minor,
     paid_amount_minor, refundable_amount_minor, refunded_amount_minor,
     source_snapshot_digest, content_digest, payment_due_at, issued_at, settled_at,
     version, created_at, updated_at)
VALUES
    ('merge-invoice-a', 'customer-ledger', 'manual', 'CNY', 'pending_payment', 2,
     1000, 100, 900, 0, 900, 0, 0, 0, repeat('1',64), repeat('2',64),
     CURRENT_TIMESTAMP + INTERVAL '1 hour', CURRENT_TIMESTAMP, NULL, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    ('merge-invoice-b', 'customer-ledger', 'manual', 'CNY', 'overdue', 1,
     500, 0, 500, 50, 550, 0, 0, 0, repeat('3',64), repeat('4',64),
     CURRENT_TIMESTAMP - INTERVAL '1 hour', CURRENT_TIMESTAMP - INTERVAL '2 hours', NULL, 1,
     CURRENT_TIMESTAMP - INTERVAL '3 hours', CURRENT_TIMESTAMP),
    ('split-invoice', 'customer-ledger', 'manual', 'CNY', 'pending_payment', 3,
     600, 0, 600, 0, 600, 0, 0, 0, repeat('5',64), repeat('6',64),
     CURRENT_TIMESTAMP + INTERVAL '2 hours', CURRENT_TIMESTAMP, NULL, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    ('other-customer-invoice', 'customer-ledger-other', 'manual', 'CNY', 'pending_payment', 1,
     100, 0, 100, 0, 100, 0, 0, 0, repeat('7',64), repeat('8',64),
     CURRENT_TIMESTAMP + INTERVAL '1 hour', CURRENT_TIMESTAMP, NULL, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    ('other-currency-invoice', 'customer-ledger', 'manual', 'USD', 'pending_payment', 1,
     100, 0, 100, 0, 100, 0, 0, 0, repeat('9',64), repeat('a',64),
     CURRENT_TIMESTAMP + INTERVAL '1 hour', CURRENT_TIMESTAMP, NULL, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
	('draft-invoice', 'customer-ledger', 'manual', 'CNY', 'draft', 1,
	 100, 0, 100, 0, 100, 0, 0, 0, repeat('b',64), repeat('c',64),
	 CURRENT_TIMESTAMP + INTERVAL '1 hour', NULL, NULL, 1,
     CURRENT_TIMESTAMP, CURRENT_TIMESTAMP);
INSERT INTO plugin_idc_core_billing_invoice_item
    (invoice_item_no, invoice_no, customer_no, currency_code, line_number,
     item_type_code, source_reference, description_zh_cn, description_en_us,
     subtotal_minor, discount_minor, net_amount_minor, tax_amount_minor,
     total_amount_minor, refundable_cap_minor, content_digest)
VALUES
    ('merge-item-a1', 'merge-invoice-a', 'customer-ledger', 'CNY', 1, 'manual', 'merge-a1', '合并一', 'Merge one', 600, 50, 550, 0, 550, 500, repeat('d',64)),
    ('merge-item-a2', 'merge-invoice-a', 'customer-ledger', 'CNY', 2, 'manual', 'merge-a2', '合并二', 'Merge two', 400, 50, 350, 0, 350, 300, repeat('e',64)),
    ('merge-item-b1', 'merge-invoice-b', 'customer-ledger', 'CNY', 1, 'manual', 'merge-b1', '合并三', 'Merge three', 500, 0, 500, 50, 550, 500, repeat('f',64)),
    ('split-item-a', 'split-invoice', 'customer-ledger', 'CNY', 1, 'manual', 'split-a', '拆分一', 'Split one', 100, 0, 100, 0, 100, 100, repeat('1',64)),
    ('split-item-b', 'split-invoice', 'customer-ledger', 'CNY', 2, 'manual', 'split-b', '拆分二', 'Split two', 200, 0, 200, 0, 200, 200, repeat('2',64)),
    ('split-item-c', 'split-invoice', 'customer-ledger', 'CNY', 3, 'manual', 'split-c', '拆分三', 'Split three', 300, 0, 300, 0, 300, 300, repeat('3',64)),
    ('other-customer-item', 'other-customer-invoice', 'customer-ledger-other', 'CNY', 1, 'manual', 'other-customer', '其他客户', 'Other customer', 100, 0, 100, 0, 100, 100, repeat('4',64)),
    ('other-currency-item', 'other-currency-invoice', 'customer-ledger', 'USD', 1, 'manual', 'other-currency', '其他币种', 'Other currency', 100, 0, 100, 0, 100, 100, repeat('5',64)),
	('draft-item', 'draft-invoice', 'customer-ledger', 'CNY', 1, 'manual', 'draft-item-source', '草稿', 'Draft', 100, 0, 100, 0, 100, 100, repeat('6',64))`); err != nil {
		t.Fatalf("seed invoice transform: %v", err)
	}
}
