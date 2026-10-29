// This file seeds historical pending adjustments without invoking the new
// immediate-posting path, so recovery is verified against real legacy rows.

package billing

import (
	"context"
	"github.com/gogf/gf/v2/database/gdb"
	"testing"
)

func seedLegacyReceipt(t *testing.T, ctx context.Context, service FinancialAdjustmentService, input ManualReceiptInput) (*FinancialAdjustment, error) {
	t.Helper()
	command, err := normalizeManualReceipt(input)
	if err != nil {
		return nil, err
	}
	return seedLegacyAdjustment(t, ctx, service, command)
}

func seedLegacyWaiver(t *testing.T, ctx context.Context, service FinancialAdjustmentService, input InvoiceWaiverInput) (*FinancialAdjustment, error) {
	t.Helper()
	command, err := normalizeInvoiceWaiver(input)
	if err != nil {
		return nil, err
	}
	return seedLegacyAdjustment(t, ctx, service, command)
}

func seedLegacyBalance(t *testing.T, ctx context.Context, service FinancialAdjustmentService, input CustomerBalanceAdjustmentInput) (*FinancialAdjustment, error) {
	t.Helper()
	command, err := normalizeCustomerBalanceAdjustment(input)
	if err != nil {
		return nil, err
	}
	return seedLegacyAdjustment(t, ctx, service, command)
}

func seedLegacyAdjustment(t *testing.T, ctx context.Context, service FinancialAdjustmentService, command adjustmentCommand) (*FinancialAdjustment, error) {
	t.Helper()
	owner := service.(*serviceImpl)
	var result *FinancialAdjustment
	err := owner.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		if _, _, err := resolveAdjustmentTarget(ctx, owner.database, &command); err != nil {
			return err
		}
		if command.InvoiceNo == "" && command.CreditStatementNo == "" {
			if _, err := owner.ensureAndLockAccounts(ctx, command.CurrencyCode, true, customerAdjustmentAccountSpec(command)); err != nil {
				return err
			}
		}
		digest, err := adjustmentRequestDigest(command)
		if err != nil {
			return err
		}
		request, err := owner.insertAdjustmentRequest(ctx, command, adjustmentPolicySnapshot{ApprovalRequired: true}, digest)
		if err != nil {
			return err
		}
		result = projectAdjustmentRequest(request, false)
		return nil
	})
	return result, err
}
