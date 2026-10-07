// This file appends completed refund allocations and their reversals. Every
// refund remains bound to one invoice item and one effective payment allocation.

package billing

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/internal/dao"
	"lina-plugin-idc-core/backend/internal/model/do"
	"lina-plugin-idc-core/backend/internal/model/entity"
)

func (s *serviceImpl) ApplyRefund(
	ctx context.Context,
	input InvoiceRefundInput,
) (*Invoice, error) {
	normalized, err := normalizeInvoiceRefund(input)
	if err != nil {
		return nil, err
	}
	var result *Invoice
	err = s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		var applyErr error
		result, applyErr = s.applyRefundLocked(ctx, normalized)
		return applyErr
	})
	return result, normalizeInvoiceError(err)
}

// applyRefundLocked appends one completed refund inside the caller's existing
// transaction. It is shared by the public invoice service and the online
// refund execution owner so channel evidence, ledger posting, allocation, and
// invoice state always commit or roll back together.
func (s *serviceImpl) applyRefundLocked(
	ctx context.Context,
	input InvoiceRefundInput,
) (*Invoice, error) {
	invoice, err := lockInvoice(ctx, s.database, input.InvoiceNo)
	if err != nil {
		return nil, err
	}
	if invoice == nil {
		return nil, bizerr.NewCode(CodeInvoiceNotFound)
	}
	aggregate, err := loadInvoiceAggregate(ctx, s.database, invoice.InvoiceNo)
	if err != nil {
		return nil, err
	}
	if err = validateInvoiceAggregate(invoice, aggregate); err != nil {
		return nil, err
	}
	digest, err := refundAllocationDigest(input, "", "apply")
	if err != nil {
		return nil, err
	}
	existing, err := loadRefundReplay(
		ctx, s.database, input.InvoiceNo, input.InvoiceItemNo,
		input.SourceType, input.SourceReference, "apply",
	)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		if existing.PaymentAllocationNo != input.PaymentAllocationNo ||
			existing.LedgerTransactionNo != input.LedgerTransactionNo ||
			existing.AmountMinor != input.AmountMinor || existing.ContentDigest != digest ||
			existing.OriginalRefundAllocationNo != "" {
			return nil, bizerr.NewCode(CodeInvoiceConflict)
		}
		return projectInvoice(invoice, existing.RefundAllocationNo, true, false), nil
	}
	if err = s.validateRefundApplication(ctx, invoice, input); err != nil {
		return nil, err
	}
	allocationNo, err := newBillingNumber(s.random, "bra_")
	if err != nil {
		return nil, err
	}
	if err = insertRefundAllocation(
		ctx, s.database, invoice.CustomerNo, invoice.CurrencyCode, input,
		"apply", "", digest, allocationNo,
	); err != nil {
		return nil, err
	}
	var result *Invoice
	if err = s.refreshInvoiceAfterAllocation(ctx, invoice, allocationNo, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func (s *serviceImpl) validateRefundApplication(
	ctx context.Context,
	invoice *entity.BillingInvoice,
	input InvoiceRefundInput,
) error {
	if !invoiceAcceptsRefund(invoice) || input.AmountMinor > invoice.RefundableAmountMinor {
		return bizerr.NewCode(CodeInvoiceConflict)
	}
	item, err := loadInvoiceItemAmounts(ctx, s.database, invoice.InvoiceNo, input.InvoiceItemNo)
	if err != nil {
		return err
	}
	if item == nil || item.RefundMinor < 0 ||
		input.AmountMinor > item.RefundableCapMinor-item.RefundMinor {
		return bizerr.NewCode(CodeInvoiceConflict)
	}
	payment, err := loadPaymentAllocation(ctx, s.database, input.PaymentAllocationNo, true)
	if err != nil {
		return err
	}
	if payment == nil || payment.Direction != "apply" ||
		payment.InvoiceNo != invoice.InvoiceNo || payment.CustomerNo != invoice.CustomerNo ||
		payment.CurrencyCode != invoice.CurrencyCode ||
		(payment.InvoiceItemNo != "" && payment.InvoiceItemNo != input.InvoiceItemNo) {
		return bizerr.NewCode(CodeInvoiceConflict)
	}
	effectivePayment, err := effectivePaymentAllocationMinor(ctx, s.database, payment.AllocationNo)
	if err != nil {
		return err
	}
	refundedPayment, err := effectiveRefundAgainstPaymentMinor(ctx, s.database, payment.AllocationNo)
	if err != nil {
		return err
	}
	if refundedPayment < 0 || input.AmountMinor > effectivePayment-refundedPayment {
		return bizerr.NewCode(CodeInvoiceConflict)
	}
	transaction, err := lockInvoiceLedgerTransaction(ctx, s.database, input.LedgerTransactionNo)
	if err != nil {
		return err
	}
	if transaction.CurrencyCode != invoice.CurrencyCode ||
		transaction.TransactionType != refundLedgerType(input.SourceType) ||
		transaction.OriginalTransaction != "" {
		return bizerr.NewCode(CodeInvoiceConflict)
	}
	if err = requireUnreversedLedgerTransaction(ctx, s.database, transaction.TransactionNo); err != nil {
		return err
	}
	usedMinor, err := usedLedgerAllocationMinor(ctx, s.database, transaction.TransactionNo)
	if err != nil {
		return err
	}
	if input.AmountMinor > transaction.AmountMinor-usedMinor {
		return bizerr.NewCode(CodeInvoiceConflict)
	}
	return nil
}

func (s *serviceImpl) ReverseRefund(
	ctx context.Context,
	input InvoiceRefundReversalInput,
) (*Invoice, error) {
	normalized, err := normalizeInvoiceRefundReversal(input)
	if err != nil {
		return nil, err
	}
	preloaded, err := loadRefundAllocation(
		ctx, s.database, normalized.OriginalRefundAllocationNo, false,
	)
	if err != nil {
		return nil, normalizeInvoiceError(err)
	}
	if preloaded == nil || preloaded.Direction != "apply" {
		return nil, bizerr.NewCode(CodeInvoiceNotFound)
	}
	var result *Invoice
	err = s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		invoice, loadErr := lockInvoice(ctx, s.database, preloaded.InvoiceNo)
		if loadErr != nil {
			return loadErr
		}
		if invoice == nil {
			return bizerr.NewCode(CodeInvoiceNotFound)
		}
		original, loadErr := loadRefundAllocation(
			ctx, s.database, normalized.OriginalRefundAllocationNo, true,
		)
		if loadErr != nil {
			return loadErr
		}
		if original == nil || original.Direction != "apply" ||
			original.InvoiceNo != invoice.InvoiceNo || original.CustomerNo != invoice.CustomerNo ||
			original.CurrencyCode != invoice.CurrencyCode {
			return bizerr.NewCode(CodeInvoiceIntegrity)
		}
		aggregate, aggregateErr := loadInvoiceAggregate(ctx, s.database, invoice.InvoiceNo)
		if aggregateErr != nil {
			return aggregateErr
		}
		if integrityErr := validateInvoiceAggregate(invoice, aggregate); integrityErr != nil {
			return integrityErr
		}
		sourceType := RefundSourceType(original.SourceType)
		digest, digestErr := refundReversalDigest(normalized, original, sourceType)
		if digestErr != nil {
			return digestErr
		}
		existing, loadErr := loadRefundReplay(
			ctx, s.database, original.InvoiceNo, original.InvoiceItemNo,
			sourceType, normalized.SourceReference, "reverse",
		)
		if loadErr != nil {
			return loadErr
		}
		if existing != nil {
			if existing.OriginalRefundAllocationNo != original.RefundAllocationNo ||
				existing.LedgerTransactionNo != normalized.LedgerTransactionNo ||
				existing.AmountMinor != normalized.AmountMinor || existing.ContentDigest != digest {
				return bizerr.NewCode(CodeInvoiceConflict)
			}
			result = projectInvoice(invoice, existing.RefundAllocationNo, true, false)
			return nil
		}
		effectiveMinor, effectiveErr := effectiveRefundAllocationMinor(
			ctx, s.database, original.RefundAllocationNo,
		)
		if effectiveErr != nil {
			return effectiveErr
		}
		if normalized.AmountMinor > effectiveMinor {
			return bizerr.NewCode(CodeInvoiceConflict)
		}
		transaction, transactionErr := lockInvoiceLedgerTransaction(
			ctx, s.database, normalized.LedgerTransactionNo,
		)
		if transactionErr != nil {
			return transactionErr
		}
		if transaction.TransactionType != "reversal" ||
			transaction.OriginalTransaction != original.LedgerTransactionNo ||
			transaction.CurrencyCode != invoice.CurrencyCode {
			return bizerr.NewCode(CodeInvoiceConflict)
		}
		usedMinor, usedErr := usedLedgerAllocationMinor(ctx, s.database, transaction.TransactionNo)
		if usedErr != nil {
			return usedErr
		}
		if normalized.AmountMinor > transaction.AmountMinor-usedMinor {
			return bizerr.NewCode(CodeInvoiceConflict)
		}
		allocationNo, numberErr := newBillingNumber(s.random, "bra_")
		if numberErr != nil {
			return numberErr
		}
		refundInput := InvoiceRefundInput{
			InvoiceNo: original.InvoiceNo, InvoiceItemNo: original.InvoiceItemNo,
			PaymentAllocationNo: original.PaymentAllocationNo, SourceType: sourceType,
			SourceReference:     normalized.SourceReference,
			LedgerTransactionNo: normalized.LedgerTransactionNo,
			AmountMinor:         normalized.AmountMinor,
		}
		if insertErr := insertRefundAllocation(
			ctx, s.database, invoice.CustomerNo, invoice.CurrencyCode, refundInput,
			"reverse", original.RefundAllocationNo, digest, allocationNo,
		); insertErr != nil {
			return insertErr
		}
		return s.refreshInvoiceAfterAllocation(ctx, invoice, allocationNo, &result)
	})
	return result, normalizeInvoiceError(err)
}

func insertRefundAllocation(
	ctx context.Context,
	database gdb.DB,
	customerNo string,
	currency string,
	input InvoiceRefundInput,
	direction string,
	originalAllocationNo string,
	digest string,
	allocationNo string,
) error {
	var originalPointer *string
	if originalAllocationNo != "" {
		originalCopy := originalAllocationNo
		originalPointer = &originalCopy
	}
	transactionType := refundLedgerType(input.SourceType)
	if direction == "reverse" {
		transactionType = "reversal"
	}
	row := do.BillingRefundAllocation{
		TenantId: 0, RefundAllocationNo: allocationNo, InvoiceNo: input.InvoiceNo,
		InvoiceItemNo: input.InvoiceItemNo, CustomerNo: customerNo, CurrencyCode: currency,
		PaymentAllocationNo: input.PaymentAllocationNo, RefundSourceTypeCode: input.SourceType,
		RefundSourceReference: input.SourceReference, LedgerTransactionNo: input.LedgerTransactionNo,
		LedgerTransactionTypeCode: transactionType, DirectionCode: direction,
		OriginalRefundAllocationNo: originalPointer, AmountMinor: input.AmountMinor,
		ContentDigest: digest,
	}
	if _, err := database.Model(dao.BillingRefundAllocation.Table()).Ctx(ctx).Data(row).Insert(); err != nil {
		return gerror.Wrap(err, "insert invoice refund allocation")
	}
	return nil
}

func invoiceAcceptsRefund(invoice *entity.BillingInvoice) bool {
	if invoice == nil || invoice.IssuedAt == nil || invoice.SettledAt == nil ||
		invoice.CancelledAt != nil || invoice.PaidAmountMinor != invoice.TotalAmountMinor ||
		invoice.CreditAmountMinor != 0 || invoice.RefundableAmountMinor <= 0 {
		return false
	}
	status := InvoiceStatus(invoice.StatusCode)
	return status == InvoiceStatusPaid || status == InvoiceStatusPartiallyRefunded
}

func refundLedgerType(source RefundSourceType) string {
	if source == RefundSourceMigration {
		return "migration"
	}
	return "refund"
}

func refundAllocationDigest(
	input InvoiceRefundInput,
	originalAllocationNo string,
	direction string,
) (string, error) {
	return contentDigest(struct {
		InvoiceNo                  string           `json:"invoiceNo"`
		InvoiceItemNo              string           `json:"invoiceItemNo"`
		PaymentAllocationNo        string           `json:"paymentAllocationNo"`
		SourceType                 RefundSourceType `json:"sourceType"`
		SourceReference            string           `json:"sourceReference"`
		LedgerTransactionNo        string           `json:"ledgerTransactionNo"`
		Direction                  string           `json:"direction"`
		OriginalRefundAllocationNo string           `json:"originalRefundAllocationNo"`
		AmountMinor                int64            `json:"amountMinor"`
	}{
		input.InvoiceNo, input.InvoiceItemNo, input.PaymentAllocationNo,
		input.SourceType, input.SourceReference, input.LedgerTransactionNo,
		direction, originalAllocationNo, input.AmountMinor,
	})
}

func refundReversalDigest(
	input InvoiceRefundReversalInput,
	original *refundAllocationRow,
	sourceType RefundSourceType,
) (string, error) {
	return refundAllocationDigest(InvoiceRefundInput{
		InvoiceNo: original.InvoiceNo, InvoiceItemNo: original.InvoiceItemNo,
		PaymentAllocationNo: original.PaymentAllocationNo, SourceType: sourceType,
		SourceReference: input.SourceReference, LedgerTransactionNo: input.LedgerTransactionNo,
		AmountMinor: input.AmountMinor,
	}, original.RefundAllocationNo, "reverse")
}
