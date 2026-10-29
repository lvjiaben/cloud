// This file directly posts authorized staff gifts and cash deductions through
// immutable adjustments. Account locks serialize deductions with holds/payments.

package billing

import (
	"context"
	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/internal/model/entity"
)

// Customer balance postings use the established ledger semantics.
const (
	customerBalanceOwner      = "customer"
	merchantBalanceOwner      = "merchant"
	balanceDirectionCredit    = "credit"
	balanceDirectionDebit     = "debit"
	balanceRoleGift           = "gift"
	balanceRoleExpense        = "expense"
	balanceRoleCash           = "balance"
	balanceRoleAdjustment     = "adjustment"
	balanceAdjustmentClearing = "adjustment_clearing"
)

func normalizeCustomerBalanceAdjustment(input CustomerBalanceAdjustmentInput) (adjustmentCommand, error) {
	operation := adjustmentTypeBalanceCredit
	if input.Deduct {
		operation = adjustmentTypeBalanceDebit
	}
	input.ReasonCode, input.EvidenceDigest = adjustmentMetadata(input.ReasonCode, input.EvidenceDigest, operation, input.IdempotencyKey)
	if !exactlyTrimmed(input.CustomerNo, input.CurrencyCode, input.ReasonCode, input.EvidenceDigest, input.IdempotencyKey) ||
		!validBusinessReference(input.CustomerNo, 40) || !currencyPattern.MatchString(input.CurrencyCode) ||
		!validAmount(input.AmountMinor) || input.RequestedBy <= 0 || !adjustmentReasonPattern.MatchString(input.ReasonCode) ||
		!canonicalDigestPattern.MatchString(input.EvidenceDigest) || !validInvoiceTransformKey(input.IdempotencyKey) {
		return adjustmentCommand{}, invoiceInvalidError()
	}
	command := adjustmentCommand{AdjustmentType: adjustmentTypeBalanceCredit, CustomerNo: input.CustomerNo,
		CurrencyCode: input.CurrencyCode, AccountType: AccountTypeCustomerGift, AmountMinor: input.AmountMinor,
		ReasonCode: input.ReasonCode, EvidenceDigest: input.EvidenceDigest,
		IdempotencyDigest: invoiceTransformIdempotencyDigest(input.IdempotencyKey), RequestedBy: input.RequestedBy}
	if input.Deduct {
		command.AdjustmentType = adjustmentTypeBalanceDebit
		command.AccountType = AccountTypeCustomerBalance
	}
	return command, nil
}

func (s *serviceImpl) CreateCustomerBalanceAdjustment(ctx context.Context, input CustomerBalanceAdjustmentInput) (*FinancialAdjustment, error) {
	command, err := normalizeCustomerBalanceAdjustment(input)
	if err != nil {
		return nil, err
	}
	return s.createFinancialAdjustment(ctx, command)
}

// requireAvailableAdjustmentBalance runs both before accepting a deduction and
// again under the same account lock at posting, including delayed approvals.
func (s *serviceImpl) requireAvailableAdjustmentBalance(ctx context.Context, accountNo string, amount int64) error {
	balance, held, err := loadCurrentBalanceAndHeld(ctx, s.database, accountNo)
	if err != nil {
		return err
	}
	if balance-held < amount {
		return bizerr.NewCode(CodeInsufficientBalance)
	}
	return nil
}

func (s *serviceImpl) postCustomerBalanceAdjustment(ctx context.Context, request *entity.BillingAdjustmentRequest, merchantNo string) (*transactionRow, *FinancialAdjustment, error) {
	if request == nil || request.AccountNo == nil || request.AccountTypeCode == nil || request.InvoiceNo != nil || request.CreditStatementNo != "" {
		return nil, nil, bizerr.NewCode(CodeInvoiceIntegrity)
	}
	debit := request.AdjustmentTypeCode == adjustmentTypeBalanceDebit
	accountType := AccountTypeCustomerGift
	if debit {
		accountType = AccountTypeCustomerBalance
	}
	expected := deterministicAccountNo(customerBalanceOwner, request.CustomerNo, accountType, request.CurrencyCode)
	if *request.AccountNo != expected || *request.AccountTypeCode != accountType {
		return nil, nil, bizerr.NewCode(CodeInvoiceIntegrity)
	}
	if err := s.requireActiveCustomer(ctx, request.CustomerNo); err != nil {
		return nil, nil, err
	}
	customerSpec := customerAdjustmentAccountSpec(adjustmentCommand{CustomerNo: request.CustomerNo, AccountType: accountType})
	counterpart := merchantMarketingExpenseSpec(merchantNo)
	if debit {
		counterpart = accountSpec{OwnerType: merchantBalanceOwner, OwnerReference: merchantNo, AccountType: balanceAdjustmentClearing, NormalBalance: balanceDirectionCredit}
	}
	accounts, err := s.ensureAndLockAccounts(ctx, request.CurrencyCode, true, customerSpec, counterpart)
	if err != nil {
		return nil, nil, err
	}
	counterpartNo := deterministicAccountNo(counterpart.OwnerType, counterpart.OwnerReference, counterpart.AccountType, request.CurrencyCode)
	transactionType := balanceRoleGift
	entries := []entrySemantic{
		{AccountNo: counterpartNo, Direction: balanceDirectionDebit, Role: balanceRoleExpense, AmountMinor: request.AmountMinor, Reason: request.ReasonCode},
		{AccountNo: expected, Direction: balanceDirectionCredit, Role: balanceRoleGift, AmountMinor: request.AmountMinor, Reason: request.ReasonCode},
	}
	if debit {
		if err = s.requireAvailableAdjustmentBalance(ctx, expected, request.AmountMinor); err != nil {
			return nil, nil, err
		}
		transactionType = balanceRoleAdjustment
		entries = []entrySemantic{
			{AccountNo: expected, Direction: balanceDirectionDebit, Role: balanceRoleCash, AmountMinor: request.AmountMinor, Reason: request.ReasonCode},
			{AccountNo: counterpartNo, Direction: balanceDirectionCredit, Role: balanceRoleAdjustment, AmountMinor: request.AmountMinor, Reason: request.ReasonCode},
		}
	}
	transaction, _, err := s.postLocked(ctx, transactionType, request.CurrencyCode, request.AdjustmentTypeCode,
		request.AdjustmentNo, request.EvidenceDigest, "", entries, accounts)
	if err != nil {
		return nil, nil, err
	}
	balance, held, err := loadCurrentBalanceAndHeld(ctx, s.database, expected)
	if err != nil {
		return nil, nil, err
	}
	return transaction, &FinancialAdjustment{Balance: &Posting{TransactionNo: transaction.TransactionNo,
		TransactionType: transaction.TransactionType, CustomerNo: request.CustomerNo, CurrencyCode: request.CurrencyCode,
		AmountMinor: request.AmountMinor, BalanceMinor: balance, HeldBalanceMinor: held, AvailableBalanceMinor: balance - held}}, nil
}
