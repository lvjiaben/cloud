// This file atomically spends same-currency customer balance and allocates the
// resulting immutable ledger transaction to the locked invoice.

package billing

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/internal/model/entity"
)

// PayFromBalance settles the exact current outstanding invoice amount. Ledger,
// allocation, balance snapshots, and invoice lifecycle are committed together.
func (s *serviceImpl) PayFromBalance(
	ctx context.Context,
	input InvoiceBalancePaymentInput,
) (*InvoiceBalancePayment, error) {
	normalized, err := normalizeInvoiceBalancePayment(input)
	if err != nil {
		return nil, err
	}
	var result *InvoiceBalancePayment
	err = s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		invoice, loadErr := lockInvoice(ctx, s.database, normalized.InvoiceNo)
		if loadErr != nil {
			return loadErr
		}
		if invoice == nil {
			return bizerr.NewCode(CodeInvoiceNotFound)
		}
		aggregate, aggregateErr := loadInvoiceAggregate(ctx, s.database, invoice.InvoiceNo)
		if aggregateErr != nil {
			return aggregateErr
		}
		if integrityErr := validateInvoiceAggregate(invoice, aggregate); integrityErr != nil {
			return integrityErr
		}
		if lockErr := s.lockSource(
			ctx, sourceInvoiceBalancePayment, normalized.SourceReference,
		); lockErr != nil {
			return lockErr
		}
		existingTransaction, loadErr := loadTransactionBySource(
			ctx, s.database, sourceInvoiceBalancePayment, normalized.SourceReference,
		)
		if loadErr != nil {
			return loadErr
		}
		existingAllocation, loadErr := loadPaymentReplay(
			ctx, s.database, invoice.InvoiceNo, "", PaymentSourceBalance,
			normalized.SourceReference, "apply",
		)
		if loadErr != nil {
			return loadErr
		}
		if existingTransaction != nil || existingAllocation != nil {
			return s.projectInvoiceBalancePaymentReplay(
				ctx, normalized, invoice, existingTransaction, existingAllocation, &result,
			)
		}
		if !invoiceAcceptsPayment(invoice) {
			return bizerr.NewCode(CodeInvoiceConflict)
		}
		outstandingMinor := invoice.TotalAmountMinor - aggregate.PaymentMinor - aggregate.WaiverMinor
		if !validAmount(outstandingMinor) {
			return bizerr.NewCode(CodeInvoiceConflict)
		}
		if customerErr := s.requireActiveCustomer(ctx, invoice.CustomerNo); customerErr != nil {
			return customerErr
		}
		merchantNo, merchantErr := s.loadMerchantNo(ctx, true)
		if merchantErr != nil {
			return merchantErr
		}
		balanceSpec := customerBalanceSpec(invoice.CustomerNo)
		revenueSpec := merchantRevenueSpec(merchantNo)
		accounts, accountErr := s.ensureAndLockAccounts(
			ctx, invoice.CurrencyCode, true, balanceSpec, revenueSpec,
		)
		if accountErr != nil {
			return accountErr
		}
		balanceAccountNo := deterministicAccountNo(
			balanceSpec.OwnerType, balanceSpec.OwnerReference,
			balanceSpec.AccountType, invoice.CurrencyCode,
		)
		revenueAccountNo := deterministicAccountNo(
			revenueSpec.OwnerType, revenueSpec.OwnerReference,
			revenueSpec.AccountType, invoice.CurrencyCode,
		)
		balance, held, balanceErr := loadCurrentBalanceAndHeld(
			ctx, s.database, balanceAccountNo,
		)
		if balanceErr != nil {
			return balanceErr
		}
		if balance-held < outstandingMinor {
			return bizerr.NewCode(CodeInsufficientBalance)
		}
		transaction, _, postErr := s.postLocked(
			ctx, "balance_payment", invoice.CurrencyCode,
			sourceInvoiceBalancePayment, normalized.SourceReference,
			normalized.SourceSnapshotDigest, "",
			[]entrySemantic{
				{AccountNo: balanceAccountNo, Direction: "debit", Role: "balance",
					AmountMinor: outstandingMinor, InvoiceNo: invoice.InvoiceNo,
					Reason: "invoice_balance_payment"},
				{AccountNo: revenueAccountNo, Direction: "credit", Role: "revenue",
					AmountMinor: outstandingMinor, InvoiceNo: invoice.InvoiceNo,
					Reason: "invoice_balance_payment"},
			}, accounts,
		)
		if postErr != nil {
			return postErr
		}
		paymentInput := InvoicePaymentInput{
			InvoiceNo: invoice.InvoiceNo, SourceType: PaymentSourceBalance,
			SourceReference:     normalized.SourceReference,
			LedgerTransactionNo: transaction.TransactionNo, AmountMinor: outstandingMinor,
		}
		allocationDigest, digestErr := paymentAllocationDigest(paymentInput, "", "apply")
		if digestErr != nil {
			return digestErr
		}
		allocationNo, numberErr := newBillingNumber(s.random, "bpa_")
		if numberErr != nil {
			return numberErr
		}
		if insertErr := insertPaymentAllocation(
			ctx, s.database, invoice.InvoiceNo, "", invoice.CustomerNo,
			invoice.CurrencyCode, PaymentSourceBalance, normalized.SourceReference,
			transaction.TransactionNo, "apply", "", outstandingMinor,
			allocationDigest, allocationNo,
		); insertErr != nil {
			return insertErr
		}
		var paidInvoice *Invoice
		if refreshErr := s.refreshInvoiceAfterAllocation(
			ctx, invoice, allocationNo, &paidInvoice,
		); refreshErr != nil {
			return refreshErr
		}
		balance, held, balanceErr = loadCurrentBalanceAndHeld(ctx, s.database, balanceAccountNo)
		if balanceErr != nil {
			return balanceErr
		}
		result = &InvoiceBalancePayment{
			Posting: &Posting{
				TransactionNo: transaction.TransactionNo, TransactionType: "balance_payment",
				CurrencyCode: invoice.CurrencyCode, AmountMinor: outstandingMinor,
				CustomerNo: invoice.CustomerNo, BalanceMinor: balance,
				HeldBalanceMinor: held, AvailableBalanceMinor: balance - held,
			},
			Invoice: paidInvoice,
		}
		return nil
	})
	return result, normalizeInvoiceError(err)
}

func (s *serviceImpl) projectInvoiceBalancePaymentReplay(
	ctx context.Context,
	input InvoiceBalancePaymentInput,
	invoice *entity.BillingInvoice,
	transaction *transactionRow,
	allocation *paymentAllocationRow,
	result **InvoiceBalancePayment,
) error {
	if transaction == nil || allocation == nil ||
		transaction.TransactionType != "balance_payment" ||
		transaction.CurrencyCode != invoice.CurrencyCode ||
		transaction.SourceSnapshotDigest != input.SourceSnapshotDigest ||
		transaction.OriginalTransactionNo != "" ||
		allocation.InvoiceNo != invoice.InvoiceNo || allocation.InvoiceItemNo != "" ||
		allocation.CustomerNo != invoice.CustomerNo || allocation.CurrencyCode != invoice.CurrencyCode ||
		allocation.SourceType != string(PaymentSourceBalance) || allocation.Direction != "apply" ||
		allocation.OriginalAllocationNo != "" ||
		allocation.LedgerTransactionNo != transaction.TransactionNo ||
		allocation.AmountMinor != transaction.DebitTotalMinor {
		return bizerr.NewCode(CodeInvoiceConflict)
	}
	expectedDigest, err := paymentAllocationDigest(InvoicePaymentInput{
		InvoiceNo: invoice.InvoiceNo, SourceType: PaymentSourceBalance,
		SourceReference:     input.SourceReference,
		LedgerTransactionNo: transaction.TransactionNo,
		AmountMinor:         transaction.DebitTotalMinor,
	}, "", "apply")
	if err != nil {
		return err
	}
	if allocation.ContentDigest != expectedDigest {
		return bizerr.NewCode(CodeInvoiceIntegrity)
	}
	balanceAccountNo := deterministicAccountNo(
		"customer", invoice.CustomerNo, AccountTypeCustomerBalance, invoice.CurrencyCode,
	)
	balance, held, err := loadCurrentBalanceAndHeld(ctx, s.database, balanceAccountNo)
	if err != nil {
		return err
	}
	*result = &InvoiceBalancePayment{
		Posting: &Posting{
			TransactionNo: transaction.TransactionNo, TransactionType: "balance_payment",
			CurrencyCode: invoice.CurrencyCode, AmountMinor: transaction.DebitTotalMinor,
			CustomerNo: invoice.CustomerNo, BalanceMinor: balance,
			HeldBalanceMinor: held, AvailableBalanceMinor: balance - held, Reused: true,
		},
		Invoice: projectInvoice(invoice, allocation.AllocationNo, true, false), Reused: true,
	}
	return nil
}
