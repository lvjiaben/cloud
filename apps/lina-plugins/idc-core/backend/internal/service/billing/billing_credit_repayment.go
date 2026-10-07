package billing

import (
	"context"
	"time"

	"github.com/gogf/gf/v2/database/gdb"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/billingcap"
	"lina-plugin-idc-core/backend/internal/service/principal"
)

func (s *channelPaymentOwnerService) PrepareCreditRepayment(ctx context.Context, input billingcap.PrepareCreditRepayment) (*billingcap.PreparedCreditRepayment, error) {
	if err := billingcap.ValidatePrepareCreditRepayment(input); err != nil {
		return nil, err
	}
	if s.principal == nil {
		return nil, bizerr.NewCode(CodeCreditNotFound)
	}
	audience, err := s.principal.Current(ctx)
	if err != nil || audience == nil || audience.Code != principal.AudienceCustomer || audience.Status != principal.AudienceStatusActive || audience.UserID <= 0 || audience.CustomerNo == "" {
		return nil, bizerr.NewCode(CodeCreditNotFound)
	}
	var result *billingcap.PreparedCreditRepayment
	err = s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		statement, loadErr := loadCreditStatementForUpdate(ctx, s.database, input.StatementNo)
		if loadErr != nil {
			return loadErr
		}
		if statement == nil || statement.CustomerNo != audience.CustomerNo || statement.WrittenOffMinor != 0 || input.AmountMinor > statement.OutstandingMinor || (statement.StatusCode != "open" && statement.StatusCode != "partially_paid" && statement.StatusCode != "overdue") {
			return bizerr.NewCode(CodeCreditConflict)
		}
		now, loadErr := databaseTime(ctx, s.database)
		if loadErr != nil {
			return loadErr
		}
		result = &billingcap.PreparedCreditRepayment{PaymentAttemptNo: input.PaymentAttemptNo, StatementNo: statement.StatementNo, CustomerRefDigest: billingcap.PrincipalReferenceDigest(audience.UserID), CurrencyCode: statement.CurrencyCode, AmountMinor: input.AmountMinor, ExpiresAt: now.Add(30 * time.Minute).UTC()}
		return billingcap.ValidatePreparedCreditRepayment(*result)
	})
	return result, normalizeInvoiceError(err)
}

func (s *channelPaymentOwnerService) ConfirmCreditRepayment(ctx context.Context, input billingcap.ConfirmedCreditRepayment) (*billingcap.CreditRepaymentResult, error) {
	if err := billingcap.ValidateConfirmedCreditRepayment(input); err != nil {
		return nil, err
	}
	var result *billingcap.CreditRepaymentResult
	err := s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		statement, loadErr := loadCreditStatementForUpdate(ctx, s.database, input.StatementNo)
		if loadErr != nil {
			return loadErr
		}
		if statement == nil || statement.CurrencyCode != input.CurrencyCode || statement.WrittenOffMinor != 0 || input.AmountMinor > statement.OutstandingMinor || (statement.StatusCode != "open" && statement.StatusCode != "partially_paid" && statement.StatusCode != "overdue") {
			return bizerr.NewCode(CodeCreditConflict)
		}
		if loadErr = s.ledger.requireActiveCustomer(ctx, statement.CustomerNo); loadErr != nil {
			return loadErr
		}
		if loadErr = s.ledger.lockSource(ctx, "credit_repayment", input.PaymentAttemptNo); loadErr != nil {
			return loadErr
		}
		existing, loadErr := loadTransactionBySource(ctx, s.database, "credit_repayment", input.PaymentAttemptNo)
		if loadErr != nil {
			return loadErr
		}
		merchantNo, loadErr := s.ledger.loadMerchantNo(ctx, existing == nil)
		if loadErr != nil {
			return loadErr
		}
		cashSpec, receivableSpec := merchantCashSpec(merchantNo), merchantReceivableSpec(merchantNo)
		accounts, loadErr := s.ledger.ensureAndLockAccounts(ctx, input.CurrencyCode, existing == nil, cashSpec, receivableSpec)
		if loadErr != nil {
			return loadErr
		}
		cashAccountNo := deterministicAccountNo(cashSpec.OwnerType, cashSpec.OwnerReference, cashSpec.AccountType, input.CurrencyCode)
		receivableAccountNo := deterministicAccountNo(receivableSpec.OwnerType, receivableSpec.OwnerReference, receivableSpec.AccountType, input.CurrencyCode)
		transaction, _, loadErr := s.ledger.postLocked(ctx, "credit", input.CurrencyCode, "credit_repayment", input.PaymentAttemptNo, input.EvidenceDigest, "", []entrySemantic{
			{AccountNo: cashAccountNo, Direction: "debit", Role: "cash", AmountMinor: input.AmountMinor, Reason: "credit_repayment"},
			{AccountNo: receivableAccountNo, Direction: "credit", Role: "receivable", AmountMinor: input.AmountMinor, Reason: "credit_repayment"},
		}, accounts)
		if loadErr != nil {
			return loadErr
		}
		repayment, loadErr := s.ledger.ApplyConfirmedCreditStatementRepayment(ctx, CreditStatementRepaymentInput{StatementNo: statement.StatementNo, FundingSourceType: "online_payment", FundingSourceReference: input.PaymentAttemptNo, LedgerTransactionNo: transaction.TransactionNo, AmountMinor: input.AmountMinor, SourceSnapshotDigest: input.EvidenceDigest, ConfirmationEvidenceDigest: input.EvidenceDigest})
		if loadErr != nil {
			return loadErr
		}
		result = &billingcap.CreditRepaymentResult{PaymentAttemptNo: input.PaymentAttemptNo, StatementNo: repayment.StatementNo, CurrencyCode: repayment.CurrencyCode, AmountMinor: repayment.AmountMinor, TransactionNo: transaction.TransactionNo, RepaymentNo: repayment.RepaymentNo, StatementStatusCode: repayment.StatementStatusCode, StatementOutstandingMinor: repayment.StatementOutstandingMinor, Reused: existing != nil || repayment.Reused}
		return billingcap.ValidateCreditRepaymentResult(*result)
	})
	return result, creditRepaymentResultError(err)
}
