// This file creates a single immutable upgrade invoice and settles it without advancing service expiry.

package entitlementupgrade

import (
	"context"
	"github.com/gogf/gf/v2/database/gdb"
	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/internal/model/do"
	"lina-plugin-idc-core/backend/internal/model/entity"
	"lina-plugin-idc-core/backend/internal/service/billing"
	"time"
)

const (
	statusQuoted    = "quoted"
	statusPending   = "pending_payment"
	statusReady     = "ready"
	statusExecuting = "executing"
	statusRetry     = "retry_wait"
	statusSucceeded = "succeeded"
	statusManual    = "manual_review"
	statusCancelled = "cancelled"
)

// Confirm replays the original invoice and rejects any service revision change before creating a payable intent.
func (s *serviceImpl) Confirm(ctx context.Context, no string) (*View, error) {
	customer, err := s.customer(ctx)
	if err != nil {
		return nil, err
	}
	var out *View
	err = s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		row, e := s.load(ctx, no, customer)
		if e != nil {
			return e
		}
		if row == nil {
			return bizerr.NewCode(CodeInvalid)
		}
		if row.StatusCode != statusQuoted {
			out = view(row)
			return nil
		}
		state, e := s.state(ctx, row.EntitlementNo, customer)
		if e != nil {
			return e
		}
		if state.Version != row.SourceVersion || state.ConfigurationSnapshotNo != row.SourceConfigurationNo || !state.ExpiresAt.Equal(*row.PeriodExpiresAt) || !row.QuoteExpiresAt.After(time.Now()) {
			return bizerr.NewCode(CodeConflict)
		}
		invoice := "UIN_" + row.IdempotencyDigest[:32]
		item := "UIT_" + row.IdempotencyDigest[:32]
		status := statusPending
		invoiceStatus := "pending_payment"
		var settled *time.Time
		if row.ChargeTotalMinor == 0 {
			status = statusReady
			invoiceStatus = "paid"
			now := time.Now().UTC()
			settled = &now
		}
		if _, e = s.database.Exec(ctx, `INSERT INTO plugin_idc_core_billing_invoice
 (tenant_id,invoice_no,customer_no,source_code,currency_code,status_code,item_count,subtotal_minor,net_amount_minor,tax_amount_minor,total_amount_minor,source_snapshot_digest,content_digest,payment_due_at,issued_at,settled_at)
 VALUES(0,?,?,'change',?,?,1,?,?,?,?,?,?,?,CURRENT_TIMESTAMP,?)`, invoice, customer, row.CurrencyCode, invoiceStatus, row.ChargeNetMinor, row.ChargeNetMinor, row.ChargeTaxMinor, row.ChargeTotalMinor, row.ContentDigest, row.ContentDigest, row.QuoteExpiresAt, settled); e != nil {
			return e
		}
		if _, e = s.database.Exec(ctx, `INSERT INTO plugin_idc_core_billing_invoice_item
 (tenant_id,invoice_item_no,invoice_no,customer_no,currency_code,line_number,item_type_code,source_reference,description_zh_cn,description_en_us,subtotal_minor,net_amount_minor,tax_amount_minor,total_amount_minor,refundable_cap_minor,content_digest)
 VALUES(0,?,?,?,?,1,'change',?,'服务器配置升级','Server configuration upgrade',?,?,?,?,?,?)`, item, invoice, customer, row.CurrencyCode, no, row.ChargeNetMinor, row.ChargeNetMinor, row.ChargeTaxMinor, row.ChargeTotalMinor, row.ChargeTotalMinor, row.ContentDigest); e != nil {
			return e
		}
		data := do.EntitlementUpgrade{InvoiceNo: invoice, StatusCode: status}
		if settled != nil {
			data.PaidAt = settled
		}
		if _, e = s.database.Model(do.EntitlementUpgrade{}).Ctx(ctx).Where(do.EntitlementUpgrade{UpgradeNo: no, CustomerNo: customer, StatusCode: statusQuoted}).Data(data).Update(); e != nil {
			return e
		}
		row, e = s.load(ctx, no, customer)
		if e != nil {
			return e
		}
		out = view(row)
		return nil
	})
	if err != nil {
		return nil, bizerr.WrapCode(err, CodeConflict)
	}
	return out, nil
}

// Pay derives the exact invoice and same-currency debit from the locked upgrade; a replay never creates another bill.
func (s *serviceImpl) Pay(ctx context.Context, no string) (*View, error) {
	customer, err := s.customer(ctx)
	if err != nil {
		return nil, err
	}
	var out *View
	err = s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		row, e := s.load(ctx, no, customer)
		if e != nil {
			return e
		}
		if row == nil || row.InvoiceNo == "" {
			return bizerr.NewCode(CodeInvalid)
		}
		if row.StatusCode != statusPending {
			out = view(row)
			return nil
		}
		var invoice *entity.BillingInvoice
		if e = s.database.Model(do.BillingInvoice{}).Ctx(ctx).Where(do.BillingInvoice{TenantId: 0, InvoiceNo: row.InvoiceNo, CustomerNo: customer, CurrencyCode: row.CurrencyCode}).LockUpdate().Scan(&invoice); e != nil {
			return e
		}
		if invoice == nil || invoice.TotalAmountMinor != row.ChargeTotalMinor {
			return bizerr.NewCode(CodeConflict)
		}
		// A payment through the financial page already passed the same SQL guard.
		// Reconcile that fact without creating a second payment source or debit.
		alreadyPaid := invoice.StatusCode == string(billing.InvoiceStatusPaid) && invoice.PaidAmountMinor == row.ChargeTotalMinor
		if !alreadyPaid {
			state, stateErr := s.state(ctx, row.EntitlementNo, customer)
			if stateErr != nil {
				return stateErr
			}
			if state.Version != row.SourceVersion || !state.ExpiresAt.Equal(*row.PeriodExpiresAt) || !row.QuoteExpiresAt.After(time.Now()) {
				return bizerr.NewCode(CodeConflict)
			}
			payment, paymentErr := s.invoices.PayFromBalance(ctx, billing.InvoiceBalancePaymentInput{InvoiceNo: row.InvoiceNo, SourceReference: "upgrade:" + no, SourceSnapshotDigest: row.ContentDigest})
			if paymentErr != nil {
				return paymentErr
			}
			if payment == nil || payment.Invoice == nil || payment.Invoice.Status != billing.InvoiceStatusPaid || payment.Invoice.CustomerNo != customer {
				return bizerr.NewCode(CodeConflict)
			}
		}
		now := time.Now().UTC()
		if _, e = s.database.Model(do.EntitlementUpgrade{}).Ctx(ctx).Where(do.EntitlementUpgrade{UpgradeNo: no, CustomerNo: customer, StatusCode: statusPending}).Data(do.EntitlementUpgrade{StatusCode: statusReady, PaidAt: &now}).Update(); e != nil {
			return e
		}
		row, e = s.load(ctx, no, customer)
		if e != nil {
			return e
		}
		out = view(row)
		return nil
	})
	if err != nil {
		return nil, bizerr.WrapCode(err, CodeConflict)
	}
	return out, nil
}
