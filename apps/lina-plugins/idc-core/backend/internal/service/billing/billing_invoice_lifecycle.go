// This file owns invoice locking, lifecycle derivation, and aggregate header
// persistence. Status is always derived from immutable allocation facts and
// the database clock, never accepted from a caller.

package billing

import (
	"context"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/internal/model/entity"
)

type invoiceAggregate struct {
	PaymentMinor       int64 `orm:"payment_minor"`
	WaiverMinor        int64 `orm:"waiver_minor"`
	RefundMinor        int64 `orm:"refund_minor"`
	RefundableCapMinor int64 `orm:"refundable_cap_minor"`
	PaymentRows        int64 `orm:"payment_rows"`
	WaiverRows         int64 `orm:"waiver_rows"`
	RefundRows         int64 `orm:"refund_rows"`
}

type invoiceDerivedState struct {
	Status          InvoiceStatus
	PaidMinor       int64
	WaivedMinor     int64
	RefundableMinor int64
	RefundedMinor   int64
	SettledAt       *time.Time
}

func (s *serviceImpl) IssueDraft(ctx context.Context, invoiceNo string) (*Invoice, error) {
	normalized, err := normalizeInvoiceNo(invoiceNo)
	if err != nil {
		return nil, err
	}
	var result *Invoice
	err = s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		invoice, loadErr := lockInvoice(ctx, s.database, normalized)
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
		if invoice.StatusCode == string(InvoiceStatusPendingPayment) ||
			invoice.StatusCode == string(InvoiceStatusOverdue) {
			if invoice.IssuedAt == nil {
				return bizerr.NewCode(CodeInvoiceIntegrity)
			}
			result = projectInvoice(invoice, "", true, false)
			return nil
		}
		if invoice.StatusCode != string(InvoiceStatusDraft) || aggregate.PaymentRows != 0 ||
			aggregate.WaiverRows != 0 ||
			aggregate.RefundRows != 0 || invoice.IssuedAt != nil || invoice.CancelledAt != nil {
			return bizerr.NewCode(CodeInvoiceConflict)
		}
		now, nowErr := databaseTime(ctx, s.database)
		if nowErr != nil {
			return nowErr
		}
		status := InvoiceStatusPendingPayment
		if invoice.PaymentDueAt == nil {
			return bizerr.NewCode(CodeInvoiceIntegrity)
		}
		if !invoice.PaymentDueAt.After(now) {
			status = InvoiceStatusOverdue
		}
		if persistErr := updateInvoiceLifecycle(
			ctx, s.database, invoice, status, 0, 0, 0, 0, &now, nil, nil,
		); persistErr != nil {
			return persistErr
		}
		invoice.StatusCode = string(status)
		invoice.IssuedAt = &now
		invoice.Version++
		result = projectInvoice(invoice, "", false, false)
		return nil
	})
	return result, normalizeInvoiceError(err)
}

func (s *serviceImpl) MarkOverdue(ctx context.Context, invoiceNo string) (*Invoice, error) {
	normalized, err := normalizeInvoiceNo(invoiceNo)
	if err != nil {
		return nil, err
	}
	var result *Invoice
	err = s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		invoice, loadErr := lockInvoice(ctx, s.database, normalized)
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
		if invoice.StatusCode == string(InvoiceStatusOverdue) {
			result = projectInvoice(invoice, "", true, false)
			return nil
		}
		if invoice.StatusCode != string(InvoiceStatusPendingPayment) &&
			invoice.StatusCode != string(InvoiceStatusPartiallyPaid) {
			return bizerr.NewCode(CodeInvoiceConflict)
		}
		now, nowErr := databaseTime(ctx, s.database)
		if nowErr != nil {
			return nowErr
		}
		if invoice.PaymentDueAt == nil || invoice.PaymentDueAt.After(now) {
			return bizerr.NewCode(CodeInvoiceConflict)
		}
		if persistErr := updateInvoiceLifecycle(
			ctx, s.database, invoice, InvoiceStatusOverdue,
			invoice.PaidAmountMinor, invoice.WaivedAmountMinor, 0, invoice.RefundedAmountMinor,
			invoice.IssuedAt, nil, nil,
		); persistErr != nil {
			return persistErr
		}
		invoice.StatusCode = string(InvoiceStatusOverdue)
		invoice.RefundableAmountMinor = 0
		invoice.SettledAt = nil
		invoice.Version++
		result = projectInvoice(invoice, "", false, false)
		return nil
	})
	return result, normalizeInvoiceError(err)
}

func (s *serviceImpl) CancelUnpaid(ctx context.Context, invoiceNo string) (*Invoice, error) {
	normalized, err := normalizeInvoiceNo(invoiceNo)
	if err != nil {
		return nil, err
	}
	var result *Invoice
	err = s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		invoice, loadErr := lockInvoice(ctx, s.database, normalized)
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
		if invoice.StatusCode == string(InvoiceStatusCancelled) {
			result = projectInvoice(invoice, "", true, false)
			return nil
		}
		cancelable := invoice.StatusCode == string(InvoiceStatusDraft) ||
			invoice.StatusCode == string(InvoiceStatusPendingPayment) ||
			invoice.StatusCode == string(InvoiceStatusOverdue)
		if !cancelable || aggregate.PaymentRows != 0 || aggregate.WaiverRows != 0 ||
			aggregate.RefundRows != 0 ||
			invoice.PaidAmountMinor != 0 || invoice.RefundableAmountMinor != 0 ||
			invoice.RefundedAmountMinor != 0 || invoice.WaivedAmountMinor != 0 ||
			invoice.CreditAmountMinor != 0 ||
			invoice.SettledAt != nil {
			return bizerr.NewCode(CodeInvoiceConflict)
		}
		now, nowErr := databaseTime(ctx, s.database)
		if nowErr != nil {
			return nowErr
		}
		if persistErr := updateInvoiceLifecycle(
			ctx, s.database, invoice, InvoiceStatusCancelled, 0, 0, 0, 0,
			invoice.IssuedAt, nil, &now,
		); persistErr != nil {
			return persistErr
		}
		invoice.StatusCode = string(InvoiceStatusCancelled)
		invoice.CancelledAt = &now
		invoice.Version++
		result = projectInvoice(invoice, "", false, false)
		return nil
	})
	return result, normalizeInvoiceError(err)
}

func (s *serviceImpl) RecognizeBadDebt(
	ctx context.Context,
	input InvoiceBadDebtInput,
) (*Invoice, error) {
	normalized, err := normalizeInvoiceBadDebt(input)
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
		transaction, transactionErr := lockInvoiceLedgerTransaction(
			ctx, s.database, normalized.LedgerTransactionNo,
		)
		if transactionErr != nil {
			return transactionErr
		}
		if transaction.CurrencyCode != invoice.CurrencyCode {
			return bizerr.NewCode(CodeInvoiceConflict)
		}
		outstandingMinor := invoice.TotalAmountMinor - aggregate.PaymentMinor - aggregate.WaiverMinor
		if writeoffErr := validateBadDebtWriteoff(
			ctx, s.database, invoice.InvoiceNo, transaction, outstandingMinor,
		); writeoffErr != nil {
			return writeoffErr
		}
		if invoice.StatusCode == string(InvoiceStatusBadDebt) {
			result = projectInvoice(invoice, "", true, false)
			return nil
		}
		status := InvoiceStatus(invoice.StatusCode)
		if status != InvoiceStatusPendingPayment && status != InvoiceStatusPartiallyPaid &&
			status != InvoiceStatusOverdue || invoice.IssuedAt == nil ||
			aggregate.RefundMinor != 0 || outstandingMinor <= 0 {
			return bizerr.NewCode(CodeInvoiceConflict)
		}
		now, nowErr := databaseTime(ctx, s.database)
		if nowErr != nil {
			return nowErr
		}
		if invoice.PaymentDueAt == nil || invoice.PaymentDueAt.After(now) {
			return bizerr.NewCode(CodeInvoiceConflict)
		}
		if persistErr := updateInvoiceLifecycle(
			ctx, s.database, invoice, InvoiceStatusBadDebt,
			aggregate.PaymentMinor, aggregate.WaiverMinor, 0, 0, invoice.IssuedAt, nil, nil,
		); persistErr != nil {
			return persistErr
		}
		invoice.StatusCode = string(InvoiceStatusBadDebt)
		invoice.PaidAmountMinor = aggregate.PaymentMinor
		invoice.RefundableAmountMinor = 0
		invoice.RefundedAmountMinor = 0
		invoice.SettledAt = nil
		invoice.Version++
		result = projectInvoice(invoice, "", false, false)
		return nil
	})
	return result, normalizeInvoiceError(err)
}

func lockInvoice(
	ctx context.Context,
	database gdb.DB,
	invoiceNo string,
) (*entity.BillingInvoice, error) {
	var invoice *entity.BillingInvoice
	if err := database.GetScan(ctx, &invoice, `
SELECT id, tenant_id, invoice_no, customer_no, order_no, source_code,
       currency_code, status_code, item_count, subtotal_minor, discount_minor,
       net_amount_minor, tax_amount_minor, total_amount_minor, paid_amount_minor,
       waived_amount_minor, credit_amount_minor,
       refundable_amount_minor, refunded_amount_minor, source_snapshot_digest,
       content_digest, payment_due_at, issued_at, settled_at, cancelled_at,
       version, created_at, updated_at
FROM plugin_idc_core_billing_invoice
WHERE tenant_id = 0 AND invoice_no = ?
FOR UPDATE`, invoiceNo); err != nil {
		return nil, gerror.Wrap(err, "lock billing invoice")
	}
	return invoice, nil
}

func loadInvoiceAggregate(
	ctx context.Context,
	database gdb.DB,
	invoiceNo string,
) (invoiceAggregate, error) {
	var aggregate invoiceAggregate
	if err := database.GetScan(ctx, &aggregate, `
SELECT
  COALESCE((SELECT SUM(CASE direction_code WHEN 'apply' THEN amount_minor ELSE -amount_minor END)
    FROM plugin_idc_core_billing_payment_allocation
    WHERE tenant_id = 0 AND invoice_no = ?), 0) AS payment_minor,
  COALESCE((SELECT SUM(amount_minor)
    FROM plugin_idc_core_billing_invoice_waiver_allocation
    WHERE tenant_id = 0 AND invoice_no = ?), 0) AS waiver_minor,
  COALESCE((SELECT SUM(CASE direction_code WHEN 'apply' THEN amount_minor ELSE -amount_minor END)
    FROM plugin_idc_core_billing_refund_allocation
    WHERE tenant_id = 0 AND invoice_no = ?), 0) AS refund_minor,
  COALESCE((SELECT SUM(refundable_cap_minor)
    FROM plugin_idc_core_billing_invoice_item
    WHERE tenant_id = 0 AND invoice_no = ?), 0) AS refundable_cap_minor,
  (SELECT COUNT(*) FROM plugin_idc_core_billing_payment_allocation
    WHERE tenant_id = 0 AND invoice_no = ?) AS payment_rows,
  (SELECT COUNT(*) FROM plugin_idc_core_billing_invoice_waiver_allocation
    WHERE tenant_id = 0 AND invoice_no = ?) AS waiver_rows,
  (SELECT COUNT(*) FROM plugin_idc_core_billing_refund_allocation
    WHERE tenant_id = 0 AND invoice_no = ?) AS refund_rows`,
		invoiceNo, invoiceNo, invoiceNo, invoiceNo, invoiceNo, invoiceNo, invoiceNo); err != nil {
		return invoiceAggregate{}, gerror.Wrap(err, "load billing invoice aggregates")
	}
	return aggregate, nil
}

func validateInvoiceAggregate(invoice *entity.BillingInvoice, aggregate invoiceAggregate) error {
	if invoice == nil || invoice.PaymentDueAt == nil || invoice.Version < 1 ||
		invoice.CreditAmountMinor < 0 ||
		aggregate.PaymentMinor < 0 || aggregate.RefundMinor < 0 ||
		aggregate.WaiverMinor < 0 ||
		aggregate.RefundableCapMinor < 0 || invoice.PaidAmountMinor != aggregate.PaymentMinor ||
		invoice.WaivedAmountMinor != aggregate.WaiverMinor ||
		invoice.RefundedAmountMinor != aggregate.RefundMinor ||
		invoice.RefundableAmountMinor < 0 ||
		invoice.RefundableAmountMinor+invoice.RefundedAmountMinor > invoice.PaidAmountMinor {
		return bizerr.NewCode(CodeInvoiceIntegrity)
	}
	expectedRefundable := int64(0)
	status := InvoiceStatus(invoice.StatusCode)
	if status == InvoiceStatusCreditSettled {
		if invoice.TotalAmountMinor <= 0 || invoice.CreditAmountMinor != invoice.TotalAmountMinor ||
			invoice.PaidAmountMinor != 0 || invoice.WaivedAmountMinor != 0 ||
			invoice.RefundableAmountMinor != 0 || invoice.RefundedAmountMinor != 0 ||
			invoice.SettledAt == nil {
			return bizerr.NewCode(CodeInvoiceIntegrity)
		}
		return nil
	}
	if invoice.CreditAmountMinor != 0 {
		return bizerr.NewCode(CodeInvoiceIntegrity)
	}
	if status == InvoiceStatusPaid || status == InvoiceStatusPartiallyRefunded ||
		status == InvoiceStatusRefunded {
		expectedRefundable = minimumInt64(aggregate.PaymentMinor, aggregate.RefundableCapMinor) -
			aggregate.RefundMinor
	}
	if expectedRefundable < 0 || invoice.RefundableAmountMinor != expectedRefundable {
		return bizerr.NewCode(CodeInvoiceIntegrity)
	}
	return nil
}

func deriveInvoiceState(
	invoice *entity.BillingInvoice,
	aggregate invoiceAggregate,
	now time.Time,
) (invoiceDerivedState, error) {
	if invoice.CreditAmountMinor != 0 || aggregate.PaymentMinor < 0 || aggregate.WaiverMinor < 0 ||
		aggregate.PaymentMinor > invoice.TotalAmountMinor-aggregate.WaiverMinor ||
		aggregate.RefundMinor < 0 || aggregate.RefundMinor > aggregate.PaymentMinor {
		return invoiceDerivedState{}, bizerr.NewCode(CodeInvoiceConflict)
	}
	state := invoiceDerivedState{
		PaidMinor: aggregate.PaymentMinor, WaivedMinor: aggregate.WaiverMinor,
		RefundedMinor: aggregate.RefundMinor,
	}
	if aggregate.RefundMinor > 0 {
		if aggregate.PaymentMinor+aggregate.WaiverMinor != invoice.TotalAmountMinor {
			return invoiceDerivedState{}, bizerr.NewCode(CodeInvoiceConflict)
		}
		state.RefundableMinor = minimumInt64(
			aggregate.PaymentMinor, aggregate.RefundableCapMinor,
		) - aggregate.RefundMinor
		if state.RefundableMinor < 0 {
			return invoiceDerivedState{}, bizerr.NewCode(CodeInvoiceConflict)
		}
		state.Status = InvoiceStatusPartiallyRefunded
		if state.RefundableMinor == 0 {
			state.Status = InvoiceStatusRefunded
		}
		state.SettledAt = invoice.SettledAt
		if state.SettledAt == nil {
			state.SettledAt = &now
		}
		return state, nil
	}
	if aggregate.PaymentMinor+aggregate.WaiverMinor == invoice.TotalAmountMinor {
		state.Status = InvoiceStatusPaid
		state.RefundableMinor = minimumInt64(aggregate.PaymentMinor, aggregate.RefundableCapMinor)
		state.SettledAt = invoice.SettledAt
		if state.SettledAt == nil {
			state.SettledAt = &now
		}
		return state, nil
	}
	state.Status = InvoiceStatusPendingPayment
	if aggregate.PaymentMinor+aggregate.WaiverMinor > 0 {
		state.Status = InvoiceStatusPartiallyPaid
	}
	if invoice.StatusCode == string(InvoiceStatusBadDebt) {
		state.Status = InvoiceStatusBadDebt
	} else if invoice.PaymentDueAt != nil && !invoice.PaymentDueAt.After(now) {
		state.Status = InvoiceStatusOverdue
	}
	return state, nil
}

func updateInvoiceLifecycle(
	ctx context.Context,
	database gdb.DB,
	invoice *entity.BillingInvoice,
	status InvoiceStatus,
	paidMinor int64,
	waivedMinor int64,
	refundableMinor int64,
	refundedMinor int64,
	issuedAt *time.Time,
	settledAt *time.Time,
	cancelledAt *time.Time,
) error {
	updated, err := database.Exec(ctx, `
UPDATE plugin_idc_core_billing_invoice
SET status_code = ?, paid_amount_minor = ?, waived_amount_minor = ?, refundable_amount_minor = ?,
    refunded_amount_minor = ?, issued_at = ?, settled_at = ?, cancelled_at = ?,
    version = version + 1, updated_at = CURRENT_TIMESTAMP
WHERE tenant_id = 0 AND invoice_no = ? AND version = ?
  AND credit_amount_minor = 0`,
		status, paidMinor, waivedMinor, refundableMinor, refundedMinor, issuedAt, settledAt,
		cancelledAt, invoice.InvoiceNo, invoice.Version)
	if err != nil {
		return gerror.Wrap(err, "update billing invoice lifecycle")
	}
	affected, err := updated.RowsAffected()
	if err != nil {
		return gerror.Wrap(err, "read billing invoice lifecycle update")
	}
	if affected != 1 {
		return bizerr.NewCode(CodeInvoiceConflict)
	}
	return nil
}

func projectInvoice(
	invoice *entity.BillingInvoice,
	allocationNo string,
	reused bool,
	becamePaid bool,
) *Invoice {
	orderNo := ""
	if invoice.OrderNo != nil {
		orderNo = *invoice.OrderNo
	}
	dueAt := time.Time{}
	if invoice.PaymentDueAt != nil {
		dueAt = invoice.PaymentDueAt.UTC()
	}
	return &Invoice{
		InvoiceNo: invoice.InvoiceNo, OrderNo: orderNo, CustomerNo: invoice.CustomerNo,
		CurrencyCode: invoice.CurrencyCode, Status: InvoiceStatus(invoice.StatusCode),
		TotalAmountMinor: invoice.TotalAmountMinor, PaidAmountMinor: invoice.PaidAmountMinor,
		WaivedAmountMinor: invoice.WaivedAmountMinor,
		CreditAmountMinor: invoice.CreditAmountMinor,
		OutstandingAmountMinor: invoice.TotalAmountMinor - invoice.PaidAmountMinor -
			invoice.WaivedAmountMinor - invoice.CreditAmountMinor,
		RefundableAmountMinor: invoice.RefundableAmountMinor,
		RefundedAmountMinor:   invoice.RefundedAmountMinor, PaymentDueAt: dueAt,
		IssuedAt: invoice.IssuedAt, SettledAt: invoice.SettledAt,
		CancelledAt: invoice.CancelledAt, Version: invoice.Version,
		AllocationNo: allocationNo, Reused: reused, BecamePaid: becamePaid,
	}
}

func minimumInt64(left int64, right int64) int64 {
	if left < right {
		return left
	}
	return right
}

func invoiceInvalidError() error {
	return bizerr.NewCode(CodeInvoiceInvalid)
}

func normalizeInvoiceError(err error) error {
	if err == nil {
		return nil
	}
	if _, ok := bizerr.As(err); ok {
		return err
	}
	return bizerr.WrapCode(err, CodeInvoiceUnavailable)
}
