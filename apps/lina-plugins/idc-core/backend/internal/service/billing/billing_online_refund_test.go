// This file verifies online-refund normalization, allocation, and impact classification.

package billing

import (
	"math"
	"strings"
	"testing"

	"lina-core/pkg/bizerr"
)

func TestRefundNumberedPageBounds(t *testing.T) {
	for _, input := range []RefundDirectoryFilter{{Page: -1}, {Page: math.MaxInt, Limit: 100}, {Page: 1, Cursor: "old-cursor"}, {Page: 1, Limit: 101}} {
		if _, err := normalizeRefundDirectoryFilter(input); err == nil {
			t.Fatalf("accepted %+v", input)
		}
	}
	for _, size := range []int{10, 20, 50, 100} {
		if _, err := normalizeRefundDirectoryFilter(RefundDirectoryFilter{Page: 3, Limit: size}); err != nil {
			t.Fatalf("size %d: %v", size, err)
		}
	}
}

func TestBuildRefundBatchStatementUsesOneStatementForManyRows(t *testing.T) {
	for _, rowCount := range []int{1, 100} {
		rows := make([][]any, rowCount)
		for index := range rows {
			rows[index] = []any{index, "refund", int64(index + 1)}
		}
		statement, args, err := buildRefundBatchStatement("INSERT INTO refund_fixture VALUES ", rows)
		if err != nil {
			t.Fatalf("build %d refund rows: %v", rowCount, err)
		}
		if strings.Count(statement, "INSERT INTO") != 1 ||
			strings.Count(statement, "(") != rowCount || len(args) != rowCount*3 {
			t.Fatalf("refund batch is not one set statement: rows=%d sql=%q args=%d", rowCount, statement, len(args))
		}
	}
}

func TestNormalizeRefundSelectionBindsOptionalEntitlementsToOrderItem(t *testing.T) {
	valid, err := normalizeRefundPreviewInput(RefundPreviewInput{
		OrderNo: "order-refund-001",
		Selections: []RefundSelectionInput{
			{InvoiceItemNo: "invoice-item-1", EntitlementNos: []string{"entitlement-2", "entitlement-1"}, Quantity: 2, AmountMinor: 200},
		},
	})
	if err != nil || len(valid.Selections) != 1 || valid.Selections[0].EntitlementNos[0] != "entitlement-1" {
		t.Fatalf("normalize entitlement selections: value=%+v err=%v", valid, err)
	}
	_, err = normalizeRefundPreviewInput(RefundPreviewInput{
		OrderNo: "order-refund-001",
		Selections: []RefundSelectionInput{
			{InvoiceItemNo: "invoice-item-1", Quantity: 1, AmountMinor: 100},
			{InvoiceItemNo: "invoice-item-1", EntitlementNos: []string{"entitlement-1"}, Quantity: 1, AmountMinor: 100},
		},
	})
	if !bizerr.Is(err, CodeRefundInvalid) {
		t.Fatalf("overlapping order-item and entitlement selection error=%v", err)
	}
	partialEntitlementImpact, err := normalizeRefundPreviewInput(RefundPreviewInput{OrderNo: "order-refund-001", Selections: []RefundSelectionInput{{
		InvoiceItemNo: "invoice-item-1", EntitlementNos: []string{"entitlement-1"},
		Quantity: 2, AmountMinor: 100,
	}}})
	if err != nil || len(partialEntitlementImpact.Selections) != 1 ||
		partialEntitlementImpact.Selections[0].Quantity != 2 ||
		len(partialEntitlementImpact.Selections[0].EntitlementNos) != 1 {
		t.Fatalf("order-item quantity must stay independent from optional entitlement impact: value=%+v err=%v", partialEntitlementImpact, err)
	}
	_, err = normalizeRefundPreviewInput(RefundPreviewInput{OrderNo: "order-refund-001", Selections: []RefundSelectionInput{{
		InvoiceItemNo: "invoice-item-1", EntitlementNos: []string{"entitlement-1", "entitlement-2"},
		Quantity: 1, AmountMinor: 100,
	}}})
	if !bizerr.Is(err, CodeRefundInvalid) {
		t.Fatalf("more exact entitlement impacts than refunded quantity error=%v", err)
	}
}

func TestNormalizeRefundSubmissionRequiresDetailOnlyForOther(t *testing.T) {
	digest := strings.Repeat("a", 64)
	if _, _, _, err := normalizeRefundSubmission(RefundSubmissionInput{
		PreviewDigest: digest, Reason: RefundReasonOther, IdempotencyKey: "refund-key-001",
	}); !bizerr.Is(err, CodeRefundInvalid) {
		t.Fatalf("empty other detail error=%v", err)
	}
	reason, detail, keyDigest, err := normalizeRefundSubmission(RefundSubmissionInput{
		PreviewDigest: digest, Reason: RefundReasonDuplicatePurchase,
		IdempotencyKey: "refund-key-001",
	})
	if err != nil || reason != RefundReasonDuplicatePurchase || detail != "" ||
		!canonicalDigestPattern.MatchString(keyDigest) {
		t.Fatalf("normalized submission reason=%s detail=%q key=%q err=%v", reason, detail, keyDigest, err)
	}
}

func TestAllocateRefundSourcesPreservesOriginalFundingAndBounds(t *testing.T) {
	itemOne := "invoice-item-1"
	sources := []refundPaymentSourceRow{
		{AllocationNo: "allocation-item", InvoiceItemNo: &itemOne, PaymentSourceTypeCode: "balance", PaymentSourceReference: "balance-1", RemainingAmountMinor: 80},
		{AllocationNo: "allocation-shared", PaymentSourceTypeCode: "channel_payment", PaymentSourceReference: "attempt-1", RemainingAmountMinor: 100},
	}
	remaining := map[string]int64{"allocation-item": 80, "allocation-shared": 100}
	allocated, err := allocateRefundSources(120, itemOne, sources, remaining)
	if err != nil || len(allocated) != 2 || allocated[0].AmountMinor != 80 ||
		allocated[0].ExecutionOwner != "core" || allocated[1].AmountMinor != 40 ||
		allocated[1].ExecutionOwner != "payment" || remaining["allocation-shared"] != 60 {
		t.Fatalf("allocated sources=%+v remaining=%+v err=%v", allocated, remaining, err)
	}
	if _, err = allocateRefundSources(61, "invoice-item-2", sources, remaining); !bizerr.Is(err, CodeRefundNotEligible) {
		t.Fatalf("over-allocation error=%v", err)
	}
}

func TestRefundEntitlementImpactIsGenericAndExact(t *testing.T) {
	item := refundInvoiceItemRow{InvoiceItemNo: "invoice-item-1", OrderItemNo: "order-item-1", RemainingAmountMinor: 500, RemainingQuantity: 2}
	selection := RefundSelectionInput{InvoiceItemNo: item.InvoiceItemNo, EntitlementNos: []string{"entitlement-1"}, Quantity: 1, AmountMinor: 100}
	preview, flags, err := buildRefundPreviewItem(selection, item, map[string]refundEntitlementRow{
		"entitlement-1": {EntitlementNo: "entitlement-1", OrderItemNo: item.OrderItemNo, Version: 3, StatusCode: "active"},
	})
	if err != nil || len(preview.Impacts) != 1 || preview.Impacts[0].EntitlementNo != "entitlement-1" ||
		!preview.Impacts[0].RequiresTermination || !flags.requiresTermination || !flags.hasResourceImpact {
		t.Fatalf("generic entitlement preview=%+v flags=%+v err=%v", preview, flags, err)
	}
}

func TestCreditRefundPostedSourceTypeFailsClosed(t *testing.T) {
	tests := []struct {
		name string
		row  creditOnlineRefundCommandRow
		want RefundSourceType
	}{
		{name: "unbilled credit release", row: creditOnlineRefundCommandRow{
			CreditComponent: string(CreditRefundUnbilled), PaymentSourceType: "credit",
		}, want: RefundSourceCreditRelease},
		{name: "repaid channel", row: creditOnlineRefundCommandRow{
			CreditComponent: string(CreditRefundRepaid), PaymentSourceType: "channel_payment",
			Destination: "original_channel",
		}, want: RefundSourceChannel},
		{name: "repaid balance fallback", row: creditOnlineRefundCommandRow{
			CreditComponent: string(CreditRefundRepaid), PaymentSourceType: "channel_payment",
			Destination: "same_currency_balance",
		}, want: RefundSourceBalanceCredit},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := creditRefundPostedSourceType(test.row)
			if err != nil || got != test.want {
				t.Fatalf("source type=%q want=%q err=%v", got, test.want, err)
			}
		})
	}
	for _, row := range []creditOnlineRefundCommandRow{
		{CreditComponent: "unknown", PaymentSourceType: "credit"},
		{CreditComponent: string(CreditRefundRepaid), PaymentSourceType: "unknown",
			Destination: "original_channel"},
	} {
		if got, err := creditRefundPostedSourceType(row); !bizerr.Is(err, CodeRefundIntegrity) {
			t.Fatalf("unknown source type=%q error=%v", got, err)
		}
	}
}
