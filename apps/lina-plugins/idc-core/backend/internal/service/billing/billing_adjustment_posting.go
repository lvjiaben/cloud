// This file posts approved manual receipts and invoice waivers into distinct
// immutable ledger and allocation evidence, then derives invoice state.

package billing

import (
	"context"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/internal/dao"
	"lina-plugin-idc-core/backend/internal/model/do"
	"lina-plugin-idc-core/backend/internal/model/entity"
)

func (s *serviceImpl) postFinancialAdjustment(
	ctx context.Context,
	request *entity.BillingAdjustmentRequest,
	invoice *entity.BillingInvoice,
	creditStatement *creditStatementRow,
	reused bool,
) (*FinancialAdjustment, error) {
	if request == nil || request.StatusCode == string(AdjustmentStatusRejected) ||
		request.StatusCode == string(AdjustmentStatusPosted) {
		return nil, bizerr.NewCode(CodeInvoiceConflict)
	}
	merchantNo, err := s.loadMerchantNo(ctx, true)
	if err != nil {
		return nil, err
	}
	var transaction *transactionRow
	var posting *FinancialAdjustment
	switch request.AdjustmentTypeCode {
	case adjustmentTypeManualReceipt:
		transaction, posting, err = s.postManualReceipt(
			ctx, request, invoice, creditStatement, merchantNo,
		)
	case adjustmentTypeInvoiceWaiver:
		transaction, posting, err = s.postInvoiceWaiver(ctx, request, invoice, merchantNo)
	case adjustmentTypeMarketingGrantPost, adjustmentTypeMarketingGrantReverse:
		transaction, posting, err = s.postMarketingGrantAdjustment(ctx, request, merchantNo)
	default:
		return nil, bizerr.NewCode(CodeInvoiceConflict)
	}
	if err != nil {
		return nil, err
	}
	now, err := databaseTime(ctx, s.database)
	if err != nil {
		return nil, err
	}
	if err = updateAdjustmentStatus(
		ctx, s.database, request, string(AdjustmentStatusPosted),
		transaction.TransactionNo, transaction.TransactionType, &now,
	); err != nil {
		return nil, err
	}
	posting.AdjustmentNo = request.AdjustmentNo
	posting.AdjustmentType = request.AdjustmentTypeCode
	posting.Status = AdjustmentStatusPosted
	posting.CustomerNo = request.CustomerNo
	posting.InvoiceNo = nullableString(request.InvoiceNo)
	posting.CreditStatementNo = request.CreditStatementNo
	posting.CurrencyCode = request.CurrencyCode
	posting.AmountMinor = request.AmountMinor
	posting.ApprovalRequired = request.ApprovalRequired
	posting.RequestedBy = request.RequestedBy
	posting.LedgerTransactionNo = transaction.TransactionNo
	posting.Reused = reused
	return posting, nil
}

func (s *serviceImpl) postMarketingGrantAdjustment(
	ctx context.Context,
	request *entity.BillingAdjustmentRequest,
	merchantNo string,
) (*transactionRow, *FinancialAdjustment, error) {
	if request == nil || request.InvoiceNo != nil || request.CreditStatementNo != "" ||
		request.AccountNo == nil || request.AccountTypeCode == nil ||
		*request.AccountTypeCode != AccountTypeCustomerGift ||
		*request.AccountNo != deterministicAccountNo(
			"customer", request.CustomerNo, AccountTypeCustomerGift, request.CurrencyCode,
		) || !validBusinessReference(request.ProofReference, 40) {
		return nil, nil, bizerr.NewCode(CodeInvoiceIntegrity)
	}
	if customerErr := s.requireActiveCustomer(ctx, request.CustomerNo); customerErr != nil {
		return nil, nil, customerErr
	}
	giftSpec := accountSpec{
		OwnerType: "customer", OwnerReference: request.CustomerNo,
		CustomerNo: request.CustomerNo, AccountType: AccountTypeCustomerGift,
		NormalBalance: "credit",
	}
	expenseSpec := merchantMarketingExpenseSpec(merchantNo)
	accounts, err := s.ensureAndLockAccounts(
		ctx, request.CurrencyCode, true, giftSpec, expenseSpec,
	)
	if err != nil {
		return nil, nil, err
	}
	giftAccountNo := deterministicAccountNo(
		giftSpec.OwnerType, giftSpec.OwnerReference, giftSpec.AccountType, request.CurrencyCode,
	)
	expenseAccountNo := deterministicAccountNo(
		expenseSpec.OwnerType, expenseSpec.OwnerReference, expenseSpec.AccountType, request.CurrencyCode,
	)
	transactionType := "gift"
	originalTransactionNo := ""
	entries := []entrySemantic{
		{AccountNo: expenseAccountNo, Direction: "debit", Role: "expense",
			AmountMinor: request.AmountMinor, Reason: request.ReasonCode},
		{AccountNo: giftAccountNo, Direction: "credit", Role: "gift",
			AmountMinor: request.AmountMinor, Reason: request.ReasonCode},
	}
	if request.AdjustmentTypeCode == adjustmentTypeMarketingGrantReverse {
		transactionType = "reversal"
		originalTransactionNo, err = s.loadMarketingGrantPostingTransaction(ctx, request)
		if err != nil {
			return nil, nil, err
		}
		entries = []entrySemantic{
			{AccountNo: giftAccountNo, Direction: "debit", Role: "gift",
				AmountMinor: request.AmountMinor, Reason: request.ReasonCode},
			{AccountNo: expenseAccountNo, Direction: "credit", Role: "expense",
				AmountMinor: request.AmountMinor, Reason: request.ReasonCode},
		}
	}
	transaction, _, err := s.postLocked(
		ctx, transactionType, request.CurrencyCode, request.AdjustmentTypeCode,
		request.AdjustmentNo, request.EvidenceDigest, originalTransactionNo, entries, accounts,
	)
	if err != nil {
		return nil, nil, err
	}
	balance, held, err := loadCurrentBalanceAndHeld(ctx, s.database, giftAccountNo)
	if err != nil {
		return nil, nil, err
	}
	return transaction, &FinancialAdjustment{Balance: &Posting{
		TransactionNo: transaction.TransactionNo, TransactionType: transaction.TransactionType,
		CurrencyCode: request.CurrencyCode, AmountMinor: request.AmountMinor,
		CustomerNo: request.CustomerNo, BalanceMinor: balance, HeldBalanceMinor: held,
		AvailableBalanceMinor: balance - held,
	}}, nil
}

func (s *serviceImpl) loadMarketingGrantPostingTransaction(
	ctx context.Context,
	request *entity.BillingAdjustmentRequest,
) (string, error) {
	if request == nil || request.AdjustmentTypeCode != adjustmentTypeMarketingGrantReverse {
		return "", bizerr.NewCode(CodeInvoiceIntegrity)
	}
	var rows []struct {
		TransactionNo string `orm:"transaction_no"`
	}
	err := s.database.GetScan(ctx, &rows, `
SELECT ledger_transaction_no AS transaction_no
FROM plugin_idc_core_billing_adjustment_request
WHERE tenant_id = 0
  AND adjustment_type_code = 'marketing_grant_post'
  AND status_code = 'posted'
  AND proof_reference = ?
  AND customer_no = ?
  AND currency_code = ?
ORDER BY posted_at DESC
LIMIT 2
FOR UPDATE`, request.ProofReference, request.CustomerNo, request.CurrencyCode)
	if err != nil {
		return "", gerror.Wrap(err, "load marketing value-grant posting transaction")
	}
	if len(rows) != 1 || !validBusinessReference(rows[0].TransactionNo, 40) {
		return "", bizerr.NewCode(CodeInvoiceIntegrity)
	}
	return rows[0].TransactionNo, nil
}

func (s *serviceImpl) postManualReceipt(
	ctx context.Context,
	request *entity.BillingAdjustmentRequest,
	invoice *entity.BillingInvoice,
	creditStatement *creditStatementRow,
	merchantNo string,
) (*transactionRow, *FinancialAdjustment, error) {
	cashSpec := merchantCashSpec(merchantNo)
	cashAccountNo := deterministicAccountNo(
		cashSpec.OwnerType, cashSpec.OwnerReference, cashSpec.AccountType, request.CurrencyCode,
	)
	if request.InvoiceNo == nil && request.CreditStatementNo == "" {
		if customerErr := s.requireActiveCustomer(ctx, request.CustomerNo); customerErr != nil {
			return nil, nil, customerErr
		}
		balanceSpec := customerBalanceSpec(request.CustomerNo)
		accounts, err := s.ensureAndLockAccounts(
			ctx, request.CurrencyCode, true, cashSpec, balanceSpec,
		)
		if err != nil {
			return nil, nil, err
		}
		balanceAccountNo := deterministicAccountNo(
			balanceSpec.OwnerType, balanceSpec.OwnerReference,
			balanceSpec.AccountType, request.CurrencyCode,
		)
		transaction, _, err := s.postLocked(
			ctx, "manual_receipt", request.CurrencyCode,
			"manual_receipt", request.AdjustmentNo, request.EvidenceDigest, "",
			[]entrySemantic{
				{AccountNo: cashAccountNo, Direction: "debit", Role: "cash",
					AmountMinor: request.AmountMinor, Reason: request.ReasonCode},
				{AccountNo: balanceAccountNo, Direction: "credit", Role: "balance",
					AmountMinor: request.AmountMinor, Reason: request.ReasonCode},
			}, accounts,
		)
		if err != nil {
			return nil, nil, err
		}
		balance, held, err := loadCurrentBalanceAndHeld(ctx, s.database, balanceAccountNo)
		if err != nil {
			return nil, nil, err
		}
		return transaction, &FinancialAdjustment{Balance: &Posting{
			TransactionNo: transaction.TransactionNo, TransactionType: transaction.TransactionType,
			CurrencyCode: request.CurrencyCode, AmountMinor: request.AmountMinor,
			CustomerNo: request.CustomerNo, BalanceMinor: balance, HeldBalanceMinor: held,
			AvailableBalanceMinor: balance - held,
		}}, nil
	}
	if request.CreditStatementNo != "" {
		if creditStatement == nil ||
			creditStatement.StatementNo != request.CreditStatementNo ||
			creditStatement.CustomerNo != request.CustomerNo ||
			creditStatement.CurrencyCode != request.CurrencyCode ||
			(creditStatement.StatusCode != "open" &&
				creditStatement.StatusCode != "partially_paid" &&
				creditStatement.StatusCode != "overdue") ||
			creditStatement.WrittenOffMinor != 0 ||
			request.AmountMinor > creditStatement.OutstandingMinor {
			return nil, nil, bizerr.NewCode(CodeCreditConflict)
		}
		if customerErr := s.requireActiveCustomer(ctx, request.CustomerNo); customerErr != nil {
			return nil, nil, customerErr
		}
		receivableSpec := merchantReceivableSpec(merchantNo)
		accounts, err := s.ensureAndLockAccounts(
			ctx, request.CurrencyCode, true, cashSpec, receivableSpec,
		)
		if err != nil {
			return nil, nil, err
		}
		receivableAccountNo := deterministicAccountNo(
			receivableSpec.OwnerType, receivableSpec.OwnerReference,
			receivableSpec.AccountType, request.CurrencyCode,
		)
		transaction, _, err := s.postLocked(
			ctx, "manual_receipt", request.CurrencyCode,
			"credit_repayment", request.AdjustmentNo, request.EvidenceDigest, "",
			[]entrySemantic{
				{AccountNo: cashAccountNo, Direction: "debit", Role: "cash",
					AmountMinor: request.AmountMinor, Reason: request.ReasonCode},
				{AccountNo: receivableAccountNo, Direction: "credit", Role: "receivable",
					AmountMinor: request.AmountMinor, Reason: request.ReasonCode},
			}, accounts,
		)
		if err != nil {
			return nil, nil, err
		}
		repayment, err := s.ApplyConfirmedCreditStatementRepayment(
			ctx, CreditStatementRepaymentInput{
				StatementNo:                creditStatement.StatementNo,
				FundingSourceType:          "manual_receipt",
				FundingSourceReference:     request.AdjustmentNo,
				LedgerTransactionNo:        transaction.TransactionNo,
				AmountMinor:                request.AmountMinor,
				SourceSnapshotDigest:       request.EvidenceDigest,
				ConfirmationEvidenceDigest: request.EvidenceDigest,
			},
		)
		if err != nil {
			return nil, nil, err
		}
		return transaction, &FinancialAdjustment{CreditRepayment: repayment}, nil
	}
	if invoice == nil || invoice.InvoiceNo != *request.InvoiceNo {
		return nil, nil, bizerr.NewCode(CodeInvoiceIntegrity)
	}
	aggregate, err := loadInvoiceAggregate(ctx, s.database, invoice.InvoiceNo)
	if err != nil {
		return nil, nil, err
	}
	if err = validateInvoiceAggregate(invoice, aggregate); err != nil {
		return nil, nil, err
	}
	if !invoiceAcceptsPayment(invoice) || request.AmountMinor >
		invoice.TotalAmountMinor-aggregate.PaymentMinor-aggregate.WaiverMinor {
		return nil, nil, bizerr.NewCode(CodeInvoiceConflict)
	}
	receivableSpec := merchantReceivableSpec(merchantNo)
	accounts, err := s.ensureAndLockAccounts(
		ctx, request.CurrencyCode, true, cashSpec, receivableSpec,
	)
	if err != nil {
		return nil, nil, err
	}
	receivableAccountNo := deterministicAccountNo(
		receivableSpec.OwnerType, receivableSpec.OwnerReference,
		receivableSpec.AccountType, request.CurrencyCode,
	)
	transaction, _, err := s.postLocked(
		ctx, "manual_receipt", request.CurrencyCode,
		"manual_receipt", request.AdjustmentNo, request.EvidenceDigest, "",
		[]entrySemantic{
			{AccountNo: cashAccountNo, Direction: "debit", Role: "cash",
				AmountMinor: request.AmountMinor, InvoiceNo: invoice.InvoiceNo,
				Reason: request.ReasonCode},
			{AccountNo: receivableAccountNo, Direction: "credit", Role: "receivable",
				AmountMinor: request.AmountMinor, InvoiceNo: invoice.InvoiceNo,
				Reason: request.ReasonCode},
		}, accounts,
	)
	if err != nil {
		return nil, nil, err
	}
	paid, err := s.ApplyPayment(ctx, InvoicePaymentInput{
		InvoiceNo: invoice.InvoiceNo, SourceType: PaymentSourceManualReceipt,
		SourceReference: request.AdjustmentNo, LedgerTransactionNo: transaction.TransactionNo,
		AmountMinor: request.AmountMinor,
	})
	if err != nil {
		return nil, nil, err
	}
	return transaction, &FinancialAdjustment{
		AllocationNo: paid.AllocationNo, Invoice: paid,
	}, nil
}

func (s *serviceImpl) postInvoiceWaiver(
	ctx context.Context,
	request *entity.BillingAdjustmentRequest,
	invoice *entity.BillingInvoice,
	merchantNo string,
) (*transactionRow, *FinancialAdjustment, error) {
	if request.InvoiceNo == nil || invoice == nil || invoice.InvoiceNo != *request.InvoiceNo {
		return nil, nil, bizerr.NewCode(CodeInvoiceIntegrity)
	}
	aggregate, err := loadInvoiceAggregate(ctx, s.database, invoice.InvoiceNo)
	if err != nil {
		return nil, nil, err
	}
	if err = validateInvoiceAggregate(invoice, aggregate); err != nil {
		return nil, nil, err
	}
	if !invoiceAcceptsPayment(invoice) || request.AmountMinor >
		invoice.TotalAmountMinor-aggregate.PaymentMinor-aggregate.WaiverMinor {
		return nil, nil, bizerr.NewCode(CodeInvoiceConflict)
	}
	receivableSpec := merchantReceivableSpec(merchantNo)
	expenseSpec := merchantWaiverExpenseSpec(merchantNo)
	accounts, err := s.ensureAndLockAccounts(
		ctx, request.CurrencyCode, true, receivableSpec, expenseSpec,
	)
	if err != nil {
		return nil, nil, err
	}
	receivableAccountNo := deterministicAccountNo(
		receivableSpec.OwnerType, receivableSpec.OwnerReference,
		receivableSpec.AccountType, request.CurrencyCode,
	)
	expenseAccountNo := deterministicAccountNo(
		expenseSpec.OwnerType, expenseSpec.OwnerReference,
		expenseSpec.AccountType, request.CurrencyCode,
	)
	transaction, _, err := s.postLocked(
		ctx, "adjustment", request.CurrencyCode,
		"invoice_waiver", request.AdjustmentNo, request.EvidenceDigest, "",
		[]entrySemantic{
			{AccountNo: expenseAccountNo, Direction: "debit", Role: "expense",
				AmountMinor: request.AmountMinor, InvoiceNo: invoice.InvoiceNo,
				Reason: request.ReasonCode},
			{AccountNo: receivableAccountNo, Direction: "credit", Role: "receivable",
				AmountMinor: request.AmountMinor, InvoiceNo: invoice.InvoiceNo,
				Reason: request.ReasonCode},
		}, accounts,
	)
	if err != nil {
		return nil, nil, err
	}
	allocationNo, err := newBillingNumber(s.random, "bwa_")
	if err != nil {
		return nil, nil, err
	}
	digest, err := contentDigest(struct {
		AllocationNo  string `json:"allocationNo"`
		AdjustmentNo  string `json:"adjustmentNo"`
		InvoiceNo     string `json:"invoiceNo"`
		TransactionNo string `json:"transactionNo"`
		AmountMinor   int64  `json:"amountMinor"`
	}{allocationNo, request.AdjustmentNo, invoice.InvoiceNo, transaction.TransactionNo, request.AmountMinor})
	if err != nil {
		return nil, nil, err
	}
	now, err := databaseTime(ctx, s.database)
	if err != nil {
		return nil, nil, err
	}
	row := do.BillingInvoiceWaiverAllocation{
		TenantId: 0, WaiverAllocationNo: allocationNo, AdjustmentNo: request.AdjustmentNo,
		InvoiceNo: invoice.InvoiceNo, CustomerNo: invoice.CustomerNo,
		CurrencyCode: invoice.CurrencyCode, LedgerTransactionNo: transaction.TransactionNo,
		LedgerTransactionTypeCode: "adjustment", AdjustmentTypeCode: adjustmentTypeInvoiceWaiver,
		AmountMinor: request.AmountMinor, ContentDigest: digest, CreatedAt: &now,
	}
	if _, err = s.database.Model(dao.BillingInvoiceWaiverAllocation.Table()).Ctx(ctx).
		Data(row).Insert(); err != nil {
		return nil, nil, gerror.Wrap(err, "insert billing invoice waiver allocation")
	}
	var projected *Invoice
	if err = s.refreshInvoiceAfterAllocation(ctx, invoice, allocationNo, &projected); err != nil {
		return nil, nil, err
	}
	return transaction, &FinancialAdjustment{
		AllocationNo: allocationNo, Invoice: projected,
	}, nil
}

func merchantReceivableSpec(merchantNo string) accountSpec {
	return accountSpec{
		OwnerType: "merchant", OwnerReference: merchantNo,
		AccountType: "accounts_receivable", NormalBalance: "debit",
	}
}

func merchantWaiverExpenseSpec(merchantNo string) accountSpec {
	return accountSpec{
		OwnerType: "merchant", OwnerReference: merchantNo,
		AccountType: "waiver_expense", NormalBalance: "debit",
	}
}

func merchantMarketingExpenseSpec(merchantNo string) accountSpec {
	return accountSpec{
		OwnerType: "merchant", OwnerReference: merchantNo,
		AccountType: "marketing_expense", NormalBalance: "debit",
	}
}

func (s *serviceImpl) projectFinancialAdjustment(
	ctx context.Context,
	request *entity.BillingAdjustmentRequest,
	reused bool,
) (*FinancialAdjustment, error) {
	result := projectAdjustmentRequest(request, reused)
	if request.StatusCode != string(AdjustmentStatusPosted) {
		return result, nil
	}
	if request.InvoiceNo != nil {
		invoice, err := lockInvoice(ctx, s.database, *request.InvoiceNo)
		if err != nil {
			return nil, err
		}
		if invoice == nil {
			return nil, bizerr.NewCode(CodeInvoiceIntegrity)
		}
		result.Invoice = projectInvoice(invoice, "", true, false)
		if request.AdjustmentTypeCode == adjustmentTypeManualReceipt {
			allocation, loadErr := loadPaymentReplay(
				ctx, s.database, invoice.InvoiceNo, "", PaymentSourceManualReceipt,
				request.AdjustmentNo, "apply",
			)
			if loadErr != nil || allocation == nil {
				if loadErr != nil {
					return nil, loadErr
				}
				return nil, bizerr.NewCode(CodeInvoiceIntegrity)
			}
			result.AllocationNo = allocation.AllocationNo
		} else {
			allocationNo, loadErr := s.database.GetValue(ctx, `
SELECT waiver_allocation_no
FROM plugin_idc_core_billing_invoice_waiver_allocation
WHERE tenant_id = 0 AND adjustment_no = ?`, request.AdjustmentNo)
			if loadErr != nil || !validBusinessReference(allocationNo.String(), 40) {
				if loadErr != nil {
					return nil, gerror.Wrap(loadErr, "load billing waiver replay")
				}
				return nil, bizerr.NewCode(CodeInvoiceIntegrity)
			}
			result.AllocationNo = allocationNo.String()
		}
		return result, nil
	}
	if request.CreditStatementNo != "" {
		statement, err := loadCreditStatement(
			ctx, s.database, request.CreditStatementNo,
		)
		if err != nil {
			return nil, err
		}
		if statement == nil {
			return nil, bizerr.NewCode(CodeCreditIntegrity)
		}
		account, err := loadCreditAccountByNo(
			ctx, s.database, statement.CreditAccountNo, false,
		)
		if err != nil {
			return nil, err
		}
		repayment, err := loadCreditRepaymentBySource(
			ctx, s.database, "manual_receipt", request.AdjustmentNo,
		)
		if err != nil {
			return nil, err
		}
		if account == nil || repayment == nil ||
			repayment.StatementNo != statement.StatementNo ||
			repayment.LedgerTransactionNo != result.LedgerTransactionNo {
			return nil, bizerr.NewCode(CodeCreditIntegrity)
		}
		result.CreditRepayment = projectStatementRepayment(
			repayment, statement, account, true,
		)
		if result.CreditRepayment == nil {
			return nil, bizerr.NewCode(CodeCreditIntegrity)
		}
		return result, nil
	}
	if request.AccountNo == nil {
		return nil, bizerr.NewCode(CodeInvoiceIntegrity)
	}
	balance, held, err := loadCurrentBalanceAndHeld(ctx, s.database, *request.AccountNo)
	if err != nil {
		return nil, err
	}
	transactionType := "manual_receipt"
	if request.LedgerTransactionTypeCode != nil {
		transactionType = *request.LedgerTransactionTypeCode
	}
	result.Balance = &Posting{
		TransactionNo: result.LedgerTransactionNo, TransactionType: transactionType,
		CurrencyCode: request.CurrencyCode, AmountMinor: request.AmountMinor,
		CustomerNo: request.CustomerNo, BalanceMinor: balance, HeldBalanceMinor: held,
		AvailableBalanceMinor: balance - held, Reused: true,
	}
	return result, nil
}
