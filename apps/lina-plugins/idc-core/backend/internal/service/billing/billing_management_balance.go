// This file binds customer gift/debit requests to exact staff permissions,
// optional operation proofs, and transactional business audits.

package billing

import (
	"context"
	"github.com/gogf/gf/v2/database/gdb"
	"lina-plugin-idc-core/backend/internal/service/reauth"
)

func (s *managementFinanceCommandImpl) prepareBalanceAdjustment(ctx context.Context, intent ManagementBalanceAdjustmentIntent) (CustomerBalanceAdjustmentInput, reauth.Binding, error) {
	actor, err := s.authorizer.AuthorizeBalanceAdjustmentCreate(ctx)
	if err != nil {
		return CustomerBalanceAdjustmentInput{}, reauth.Binding{}, err
	}
	input := CustomerBalanceAdjustmentInput{CustomerNo: intent.CustomerNo, CurrencyCode: intent.CurrencyCode,
		AmountMinor: intent.AmountMinor, Deduct: intent.Deduct, ReasonCode: intent.ReasonCode, EvidenceDigest: intent.EvidenceDigest,
		IdempotencyKey: intent.RequestNo, RequestedBy: actor}
	command, err := normalizeCustomerBalanceAdjustment(input)
	if err != nil {
		return CustomerBalanceAdjustmentInput{}, reauth.Binding{}, err
	}
	input.ReasonCode, input.EvidenceDigest = command.ReasonCode, command.EvidenceDigest
	digest, err := adjustmentRequestDigest(command)
	return input, reauth.Binding{Purpose: reauth.PurposeHighValueFunds, TargetDigest: digest}, err
}

func (s *managementFinanceCommandImpl) IssueBalanceAdjustmentProof(ctx context.Context, intent ManagementBalanceAdjustmentIntent) (*reauth.ProofContext, error) {
	_, binding, err := s.prepareBalanceAdjustment(ctx, intent)
	if err != nil {
		return nil, err
	}
	return s.recentAuth.Issue(ctx, binding)
}

func (s *managementFinanceCommandImpl) CreateBalanceAdjustment(ctx context.Context, intent ManagementBalanceAdjustmentIntent, token string) (*FinancialAdjustment, error) {
	input, binding, err := s.prepareBalanceAdjustment(ctx, intent)
	if err != nil {
		return nil, err
	}
	if err = s.consumeProof(ctx, input.RequestedBy, token, binding); err != nil {
		return nil, err
	}
	var result *FinancialAdjustment
	err = s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		var createErr error
		result, createErr = s.adjustments.CreateCustomerBalanceAdjustment(ctx, input)
		if createErr != nil {
			return createErr
		}
		return s.appendAdjustmentAudit(ctx, "customer_balance", intent.RequestNo, input.ReasonCode, result)
	})
	if err != nil {
		return nil, normalizeInvoiceError(err)
	}
	return result, nil
}
