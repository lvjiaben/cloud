// This file atomically posts a customer-balance credit repayment and applies
// it to the locked same-currency credit statement without caller-owned money facts.

package billing

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"

	"lina-core/pkg/bizerr"
)

// RepayCreditStatementFromBalance posts and allocates one bounded partial or
// full repayment. Source replay is validated by the immutable ledger and
// credit repayment owners inside the same database transaction.
func (s *serviceImpl) RepayCreditStatementFromBalance(
	ctx context.Context,
	input CreditBalanceRepaymentInput,
) (*CreditBalanceRepayment, error) {
	normalized, err := normalizeCreditBalanceRepayment(input)
	if err != nil {
		return nil, err
	}
	var result *CreditBalanceRepayment
	err = s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		identity, loadErr := loadCreditStatement(ctx, s.database, normalized.StatementNo)
		if loadErr != nil {
			return loadErr
		}
		if identity == nil {
			return bizerr.NewCode(CodeCreditNotFound)
		}
		account, loadErr := loadCreditAccountByNo(
			ctx, s.database, identity.CreditAccountNo, true,
		)
		if loadErr != nil {
			return loadErr
		}
		if account == nil || !validCreditAccountProjection(account) {
			return bizerr.NewCode(CodeCreditIntegrity)
		}
		statement, loadErr := loadCreditStatementForUpdate(
			ctx, s.database, normalized.StatementNo,
		)
		if loadErr != nil {
			return loadErr
		}
		if !validCreditStatementRepaymentTarget(CreditStatementRepaymentInput{
			StatementNo: normalized.StatementNo, AmountMinor: normalized.AmountMinor,
		}, statement, account) {
			return bizerr.NewCode(CodeCreditConflict)
		}
		if customerErr := s.requireActiveCustomer(ctx, account.CustomerNo); customerErr != nil {
			return customerErr
		}
		if lockErr := s.lockSource(ctx, "credit_repayment", normalized.SourceReference); lockErr != nil {
			return lockErr
		}
		transaction, loadErr := loadTransactionBySource(
			ctx, s.database, "credit_repayment", normalized.SourceReference,
		)
		if loadErr != nil {
			return loadErr
		}
		merchantNo, merchantErr := s.loadMerchantNo(ctx, true)
		if merchantErr != nil {
			return merchantErr
		}
		balanceSpec := customerBalanceSpec(account.CustomerNo)
		receivableSpec := merchantReceivableSpec(merchantNo)
		balanceAccountNo := deterministicAccountNo(
			balanceSpec.OwnerType, balanceSpec.OwnerReference,
			balanceSpec.AccountType, account.CurrencyCode,
		)
		if transaction == nil {
			accounts, accountErr := s.ensureAndLockAccounts(
				ctx, account.CurrencyCode, true, balanceSpec, receivableSpec,
			)
			if accountErr != nil {
				return accountErr
			}
			balance, held, balanceErr := loadCurrentBalanceAndHeld(
				ctx, s.database, balanceAccountNo,
			)
			if balanceErr != nil {
				return balanceErr
			}
			if balance-held < normalized.AmountMinor {
				return bizerr.NewCode(CodeInsufficientBalance)
			}
			receivableAccountNo := deterministicAccountNo(
				receivableSpec.OwnerType, receivableSpec.OwnerReference,
				receivableSpec.AccountType, account.CurrencyCode,
			)
			transaction, _, loadErr = s.postLocked(
				ctx, "balance_payment", account.CurrencyCode,
				"credit_repayment", normalized.SourceReference,
				normalized.SourceSnapshotDigest, "",
				[]entrySemantic{
					{AccountNo: balanceAccountNo, Direction: "debit", Role: "balance",
						AmountMinor: normalized.AmountMinor, Reason: "credit_repayment"},
					{AccountNo: receivableAccountNo, Direction: "credit", Role: "receivable",
						AmountMinor: normalized.AmountMinor, Reason: "credit_repayment"},
				}, accounts,
			)
			if loadErr != nil {
				return loadErr
			}
		}
		repayment, repaymentErr := s.ApplyConfirmedCreditStatementRepayment(
			ctx, CreditStatementRepaymentInput{
				StatementNo: normalized.StatementNo, FundingSourceType: "customer_balance",
				FundingSourceReference:     normalized.SourceReference,
				LedgerTransactionNo:        transaction.TransactionNo,
				AmountMinor:                normalized.AmountMinor,
				SourceSnapshotDigest:       normalized.SourceSnapshotDigest,
				ConfirmationEvidenceDigest: normalized.ConfirmationEvidenceDigest,
			},
		)
		if repaymentErr != nil {
			return repaymentErr
		}
		balance, held, balanceErr := loadCurrentBalanceAndHeld(
			ctx, s.database, balanceAccountNo,
		)
		if balanceErr != nil {
			return balanceErr
		}
		result = &CreditBalanceRepayment{
			Repayment: repayment, BalanceMinor: balance, HeldBalanceMinor: held,
			AvailableBalanceMinor: balance - held, Reused: repayment.Reused,
		}
		return nil
	})
	if err != nil {
		return nil, creditRepaymentResultError(err)
	}
	return result, nil
}

func creditRepaymentResultError(err error) error {
	if err == nil {
		return nil
	}
	if _, ok := bizerr.As(err); ok {
		return err
	}
	return bizerr.WrapCode(err, CodeCreditUnavailable)
}
