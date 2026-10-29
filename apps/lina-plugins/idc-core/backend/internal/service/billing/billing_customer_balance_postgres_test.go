// This file verifies staff gift/cash separation, frozen-fund protection,
// idempotency and approval-time balance revalidation in isolated PostgreSQL.

package billing

import (
	"context"
	"lina-core/pkg/bizerr"
	"os"
	"strings"
	"testing"
	"time"
)

func TestCustomerBalanceAdjustmentPostgreSQLSeparatesGiftsAndProtectsHeldCash(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	database := configureBillingServiceIntegrationDB(t, ctx)
	seedBillingService(t, ctx, database)
	adjustments, err := NewFinancialAdjustmentService(database)
	if err != nil {
		t.Fatal(err)
	}
	balances, err := NewBalanceService(database)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = adjustments.ConfigureApprovalPolicy(ctx, AdjustmentPolicyInput{CurrencyCode: "CNY", ReviewThresholdMinor: 1, ConfiguredBy: 101}); err != nil {
		t.Fatal(err)
	}
	// The migration must also be safe to replay, including its constraints.
	paths := billingServiceMigrationPaths(t)
	for _, path := range paths {
		if strings.HasSuffix(path, "133-direct-admin-financial-actions.sql") {
			migration, readErr := os.ReadFile(path)
			if readErr != nil {
				t.Fatal(readErr)
			}
			if _, err = database.Exec(ctx, string(migration)); err != nil {
				t.Fatal(err)
			}
		}
	}
	receipt, err := adjustments.CreateManualReceipt(ctx, ManualReceiptInput{
		CustomerNo: "customer-ledger", CurrencyCode: "CNY", AmountMinor: 1000,
		ReasonCode: "offline_transfer", ProofReference: "private-receipt-test-001", ReceivingAccountReference: "cash-cny",
		BusinessPurposeCode: "balance_recharge", EvidenceDigest: strings.Repeat("a", 64), IdempotencyKey: "balance-receipt-test-001", RequestedBy: 101,
	})
	if err != nil || receipt.Balance == nil || receipt.Balance.BalanceMinor != 1000 {
		t.Fatalf("receipt: %+v %v", receipt, err)
	}
	giftInput := CustomerBalanceAdjustmentInput{CustomerNo: "customer-ledger", CurrencyCode: "CNY", AmountMinor: 500,
		ReasonCode: "manual_gift", EvidenceDigest: strings.Repeat("b", 64), IdempotencyKey: "balance-gift-test-001", RequestedBy: 101}
	gift, err := adjustments.CreateCustomerBalanceAdjustment(ctx, giftInput)
	if err != nil || gift.Balance == nil || gift.Balance.BalanceMinor != 500 || gift.Balance.TransactionType != "gift" {
		t.Fatalf("gift: %+v %v", gift, err)
	}
	replay, err := adjustments.CreateCustomerBalanceAdjustment(ctx, giftInput)
	if err != nil || !replay.Reused || replay.LedgerTransactionNo != gift.LedgerTransactionNo {
		t.Fatalf("gift replay: %+v %v", replay, err)
	}
	hold, err := balances.CreateHold(ctx, HoldCreateInput{CustomerNo: "customer-ledger", CurrencyCode: "CNY", AmountMinor: 700,
		SourceType: "checkout", SourceReference: "balance-admin-hold", SourceSnapshotDigest: strings.Repeat("c", 64), ExpiresAt: time.Now().Add(time.Hour)})
	if err != nil || hold.RemainingAmountMinor != 700 {
		t.Fatalf("hold: %+v %v", hold, err)
	}
	debit := giftInput
	debit.Deduct = true
	debit.ReasonCode = "manual_deduction"
	debit.IdempotencyKey = "balance-debit-test-001"
	debit.AmountMinor = 301
	if _, err = adjustments.CreateCustomerBalanceAdjustment(ctx, debit); !bizerr.Is(err, CodeInsufficientBalance) {
		t.Fatalf("spent frozen funds: %v", err)
	}
	debit.AmountMinor = 300
	deducted, err := adjustments.CreateCustomerBalanceAdjustment(ctx, debit)
	if err != nil || deducted.Balance.BalanceMinor != 700 || deducted.Balance.HeldBalanceMinor != 700 || deducted.Balance.AvailableBalanceMinor != 0 {
		t.Fatalf("deduct: %+v %v", deducted, err)
	}
	replay, err = adjustments.CreateCustomerBalanceAdjustment(ctx, debit)
	if err != nil || !replay.Reused || replay.LedgerTransactionNo != deducted.LedgerTransactionNo {
		t.Fatalf("debit replay: %+v %v", replay, err)
	}
	debit.IdempotencyKey = "balance-debit-test-002"
	debit.AmountMinor = 1
	if _, err = adjustments.CreateCustomerBalanceAdjustment(ctx, debit); !bizerr.Is(err, CodeInsufficientBalance) {
		t.Fatalf("spent gift or overdrawn cash: %v", err)
	}
}

func TestCustomerBalanceAdjustmentPostgreSQLRechecksFundsAtApproval(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	database := configureBillingServiceIntegrationDB(t, ctx)
	seedBillingService(t, ctx, database)
	adjustments, err := NewFinancialAdjustmentService(database)
	if err != nil {
		t.Fatal(err)
	}
	balances, err := NewBalanceService(database)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = balances.PostConfirmedRecharge(ctx, ConfirmedRechargeInput{CustomerNo: "customer-ledger", CurrencyCode: "CNY", AmountMinor: 1000, SourceReference: "review-balance-recharge", SourceSnapshotDigest: strings.Repeat("a", 64)}); err != nil {
		t.Fatal(err)
	}
	if _, err = adjustments.ConfigureApprovalPolicy(ctx, AdjustmentPolicyInput{CurrencyCode: "CNY", ReviewThresholdMinor: 500, ConfiguredBy: 101}); err != nil {
		t.Fatal(err)
	}
	input := CustomerBalanceAdjustmentInput{CustomerNo: "customer-ledger", CurrencyCode: "CNY", AmountMinor: 800, Deduct: true, ReasonCode: "manual_deduction", EvidenceDigest: strings.Repeat("b", 64), IdempotencyKey: "review-balance-debit-001", RequestedBy: 101}
	pending, err := seedLegacyBalance(t, ctx, adjustments, input)
	if err != nil || pending.Status != AdjustmentStatusPendingReview || pending.LedgerTransactionNo != "" {
		t.Fatalf("pending: %+v %v", pending, err)
	}
	review := AdjustmentReviewInput{AdjustmentNo: pending.AdjustmentNo, Decision: AdjustmentDecisionApprove, ReviewerID: 101, CommentDigest: strings.Repeat("c", 64), EvidenceDigest: strings.Repeat("d", 64)}
	if _, err = balances.ConsumeAvailableBalance(ctx, BalanceConsumptionInput{CustomerNo: "customer-ledger", CurrencyCode: "CNY", AmountMinor: 300, SourceReference: "spend-before-review", SourceSnapshotDigest: strings.Repeat("e", 64)}); err != nil {
		t.Fatal(err)
	}
	if _, err = adjustments.ReviewAdjustment(ctx, review); !bizerr.Is(err, CodeInsufficientBalance) {
		t.Fatalf("approval overdrew cash: %v", err)
	}
	// A rejected posting transaction must leave the request pending for recovery.
	result, err := adjustments.CreateCustomerBalanceAdjustment(ctx, input)
	if err != nil || !result.Reused || result.Status != AdjustmentStatusPendingReview || result.LedgerTransactionNo != "" {
		t.Fatalf("failed review persisted effects: %+v %v", result, err)
	}
	// A gift above the same threshold is reviewed into its own account and
	// cannot become cash or be counted as an offline receipt.
	input.Deduct = false
	input.AmountMinor = 900
	input.ReasonCode = "manual_gift"
	input.IdempotencyKey = "review-gift-credit-001"
	gift, err := seedLegacyBalance(t, ctx, adjustments, input)
	if err != nil || gift.Status != AdjustmentStatusPendingReview {
		t.Fatalf("pending gift: %+v %v", gift, err)
	}
	review.AdjustmentNo = gift.AdjustmentNo
	posted, err := adjustments.ReviewAdjustment(ctx, review)
	if err != nil || posted.Balance == nil || posted.Balance.BalanceMinor != 900 || posted.Balance.TransactionType != "gift" {
		t.Fatalf("reviewed gift: %+v %v", posted, err)
	}

}
