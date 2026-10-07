// This file appends payment allocations and their reversals, then derives the
// invoice header from immutable allocation totals in the same transaction.

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

func (s *serviceImpl) ApplyPayment(
	ctx context.Context,
	input InvoicePaymentInput,
) (*Invoice, error) {
	normalized, err := normalizeInvoicePayment(input)
	if err != nil {
		return nil, err
	}
	var result *Invoice
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
		digest, digestErr := paymentAllocationDigest(normalized, "", "apply")
		if digestErr != nil {
			return digestErr
		}
		existing, loadErr := loadPaymentReplay(
			ctx, s.database, normalized.InvoiceNo, normalized.InvoiceItemNo,
			normalized.SourceType, normalized.SourceReference, "apply",
		)
		if loadErr != nil {
			return loadErr
		}
		if existing != nil {
			if existing.LedgerTransactionNo != normalized.LedgerTransactionNo ||
				existing.AmountMinor != normalized.AmountMinor || existing.ContentDigest != digest ||
				existing.OriginalAllocationNo != "" {
				return bizerr.NewCode(CodeInvoiceConflict)
			}
			result = projectInvoice(invoice, existing.AllocationNo, true, false)
			return nil
		}
		if !invoiceAcceptsPayment(invoice) ||
			normalized.AmountMinor > invoice.TotalAmountMinor-aggregate.PaymentMinor-aggregate.WaiverMinor {
			return bizerr.NewCode(CodeInvoiceConflict)
		}
		if normalized.InvoiceItemNo != "" {
			item, itemErr := loadInvoiceItemAmounts(
				ctx, s.database, invoice.InvoiceNo, normalized.InvoiceItemNo,
			)
			if itemErr != nil {
				return itemErr
			}
			if item == nil || item.PaymentMinor < 0 ||
				normalized.AmountMinor > item.TotalMinor-item.PaymentMinor {
				return bizerr.NewCode(CodeInvoiceConflict)
			}
		}
		transaction, transactionErr := lockInvoiceLedgerTransaction(
			ctx, s.database, normalized.LedgerTransactionNo,
		)
		if transactionErr != nil {
			return transactionErr
		}
		if transaction.CurrencyCode != invoice.CurrencyCode ||
			transaction.TransactionType != paymentLedgerType(normalized.SourceType) ||
			transaction.OriginalTransaction != "" {
			return bizerr.NewCode(CodeInvoiceConflict)
		}
		if transactionErr = requireUnreversedLedgerTransaction(
			ctx, s.database, transaction.TransactionNo,
		); transactionErr != nil {
			return transactionErr
		}
		usedMinor, usedErr := usedLedgerAllocationMinor(ctx, s.database, transaction.TransactionNo)
		if usedErr != nil {
			return usedErr
		}
		if usedMinor < 0 || normalized.AmountMinor > transaction.AmountMinor-usedMinor {
			return bizerr.NewCode(CodeInvoiceConflict)
		}
		allocationNo, numberErr := newBillingNumber(s.random, "bpa_")
		if numberErr != nil {
			return numberErr
		}
		if insertErr := insertPaymentAllocation(
			ctx, s.database, invoice.InvoiceNo, normalized.InvoiceItemNo,
			invoice.CustomerNo, invoice.CurrencyCode, normalized.SourceType,
			normalized.SourceReference, normalized.LedgerTransactionNo, "apply", "",
			normalized.AmountMinor, digest, allocationNo,
		); insertErr != nil {
			return insertErr
		}
		return s.refreshInvoiceAfterAllocation(ctx, invoice, allocationNo, &result)
	})
	return result, normalizeInvoiceError(err)
}

func (s *serviceImpl) ReversePayment(
	ctx context.Context,
	input InvoicePaymentReversalInput,
) (*Invoice, error) {
	normalized, err := normalizeInvoicePaymentReversal(input)
	if err != nil {
		return nil, err
	}
	preloaded, err := loadPaymentAllocation(
		ctx, s.database, normalized.OriginalAllocationNo, false,
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
		original, loadErr := loadPaymentAllocation(
			ctx, s.database, normalized.OriginalAllocationNo, true,
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
		sourceType := PaymentSourceType(original.SourceType)
		digest, digestErr := paymentReversalDigest(normalized, original, sourceType)
		if digestErr != nil {
			return digestErr
		}
		existing, loadErr := loadPaymentReplay(
			ctx, s.database, original.InvoiceNo, original.InvoiceItemNo,
			sourceType, normalized.SourceReference, "reverse",
		)
		if loadErr != nil {
			return loadErr
		}
		if existing != nil {
			if existing.OriginalAllocationNo != original.AllocationNo ||
				existing.LedgerTransactionNo != normalized.LedgerTransactionNo ||
				existing.AmountMinor != normalized.AmountMinor || existing.ContentDigest != digest {
				return bizerr.NewCode(CodeInvoiceConflict)
			}
			result = projectInvoice(invoice, existing.AllocationNo, true, false)
			return nil
		}
		effectiveMinor, effectiveErr := effectivePaymentAllocationMinor(
			ctx, s.database, original.AllocationNo,
		)
		if effectiveErr != nil {
			return effectiveErr
		}
		refundedMinor, refundErr := effectiveRefundAgainstPaymentMinor(
			ctx, s.database, original.AllocationNo,
		)
		if refundErr != nil {
			return refundErr
		}
		if normalized.AmountMinor > effectiveMinor ||
			effectiveMinor-normalized.AmountMinor < refundedMinor {
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
		allocationNo, numberErr := newBillingNumber(s.random, "bpa_")
		if numberErr != nil {
			return numberErr
		}
		if insertErr := insertPaymentAllocation(
			ctx, s.database, invoice.InvoiceNo, original.InvoiceItemNo,
			invoice.CustomerNo, invoice.CurrencyCode, sourceType,
			normalized.SourceReference, normalized.LedgerTransactionNo, "reverse",
			original.AllocationNo, normalized.AmountMinor, digest, allocationNo,
		); insertErr != nil {
			return insertErr
		}
		return s.refreshInvoiceAfterAllocation(ctx, invoice, allocationNo, &result)
	})
	return result, normalizeInvoiceError(err)
}

func (s *serviceImpl) refreshInvoiceAfterAllocation(
	ctx context.Context,
	invoice *entity.BillingInvoice,
	allocationNo string,
	result **Invoice,
) error {
	aggregate, err := loadInvoiceAggregate(ctx, s.database, invoice.InvoiceNo)
	if err != nil {
		return err
	}
	now, err := databaseTime(ctx, s.database)
	if err != nil {
		return err
	}
	state, err := deriveInvoiceState(invoice, aggregate, now)
	if err != nil {
		return err
	}
	becamePaid := invoice.StatusCode != string(InvoiceStatusPaid) &&
		state.Status == InvoiceStatusPaid
	if err = updateInvoiceLifecycle(
		ctx, s.database, invoice, state.Status, state.PaidMinor,
		state.WaivedMinor, state.RefundableMinor, state.RefundedMinor, invoice.IssuedAt,
		state.SettledAt, nil,
	); err != nil {
		return err
	}
	invoice.StatusCode = string(state.Status)
	invoice.PaidAmountMinor = state.PaidMinor
	invoice.WaivedAmountMinor = state.WaivedMinor
	invoice.RefundableAmountMinor = state.RefundableMinor
	invoice.RefundedAmountMinor = state.RefundedMinor
	invoice.SettledAt = state.SettledAt
	invoice.Version++
	*result = projectInvoice(invoice, allocationNo, false, becamePaid)
	return nil
}

func insertPaymentAllocation(
	ctx context.Context,
	database gdb.DB,
	invoiceNo string,
	itemNo string,
	customerNo string,
	currency string,
	sourceType PaymentSourceType,
	sourceReference string,
	transactionNo string,
	direction string,
	originalAllocationNo string,
	amountMinor int64,
	digest string,
	allocationNo string,
) error {
	var itemNoPointer *string
	if itemNo != "" {
		itemNoCopy := itemNo
		itemNoPointer = &itemNoCopy
	}
	var originalPointer *string
	if originalAllocationNo != "" {
		originalCopy := originalAllocationNo
		originalPointer = &originalCopy
	}
	transactionType := paymentLedgerType(sourceType)
	if direction == "reverse" {
		transactionType = "reversal"
	}
	row := do.BillingPaymentAllocation{
		TenantId: 0, AllocationNo: allocationNo, InvoiceNo: invoiceNo,
		InvoiceItemNo: itemNoPointer, CustomerNo: customerNo, CurrencyCode: currency,
		PaymentSourceTypeCode: sourceType, PaymentSourceReference: sourceReference,
		LedgerTransactionNo: transactionNo, LedgerTransactionTypeCode: transactionType,
		DirectionCode: direction, OriginalAllocationNo: originalPointer,
		AmountMinor: amountMinor, ContentDigest: digest,
	}
	if _, err := database.Model(dao.BillingPaymentAllocation.Table()).Ctx(ctx).Data(row).Insert(); err != nil {
		return gerror.Wrap(err, "insert invoice payment allocation")
	}
	return nil
}

func invoiceAcceptsPayment(invoice *entity.BillingInvoice) bool {
	if invoice == nil || invoice.IssuedAt == nil || invoice.CancelledAt != nil ||
		invoice.PaymentDueAt == nil || invoice.TotalAmountMinor <= 0 ||
		invoice.CreditAmountMinor != 0 {
		return false
	}
	status := InvoiceStatus(invoice.StatusCode)
	return status == InvoiceStatusPendingPayment || status == InvoiceStatusPartiallyPaid ||
		status == InvoiceStatusOverdue
}

func paymentLedgerType(source PaymentSourceType) string {
	switch source {
	case PaymentSourceChannel:
		return "payment"
	case PaymentSourceBalance:
		return "balance_payment"
	case PaymentSourceCredit:
		return "credit"
	case PaymentSourceManualReceipt:
		return "manual_receipt"
	case PaymentSourceMigration:
		return "migration"
	default:
		return ""
	}
}

func paymentAllocationDigest(
	input InvoicePaymentInput,
	originalAllocationNo string,
	direction string,
) (string, error) {
	return contentDigest(struct {
		InvoiceNo            string            `json:"invoiceNo"`
		InvoiceItemNo        string            `json:"invoiceItemNo"`
		SourceType           PaymentSourceType `json:"sourceType"`
		SourceReference      string            `json:"sourceReference"`
		LedgerTransactionNo  string            `json:"ledgerTransactionNo"`
		Direction            string            `json:"direction"`
		OriginalAllocationNo string            `json:"originalAllocationNo"`
		AmountMinor          int64             `json:"amountMinor"`
	}{
		input.InvoiceNo, input.InvoiceItemNo, input.SourceType,
		input.SourceReference, input.LedgerTransactionNo, direction,
		originalAllocationNo, input.AmountMinor,
	})
}

func paymentReversalDigest(
	input InvoicePaymentReversalInput,
	original *paymentAllocationRow,
	sourceType PaymentSourceType,
) (string, error) {
	return paymentAllocationDigest(InvoicePaymentInput{
		InvoiceNo: original.InvoiceNo, InvoiceItemNo: original.InvoiceItemNo,
		SourceType: sourceType, SourceReference: input.SourceReference,
		LedgerTransactionNo: input.LedgerTransactionNo, AmountMinor: input.AmountMinor,
	}, original.AllocationNo, "reverse")
}
