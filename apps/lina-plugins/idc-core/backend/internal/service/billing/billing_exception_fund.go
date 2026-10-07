package billing

import (
	"context"
	"time"

	"github.com/gogf/gf/v2/database/gdb"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/billingcap"
)

type exceptionTargetRow struct {
	OrderNo                 string     `orm:"order_no"`
	InvoiceNo               string     `orm:"invoice_no"`
	CustomerNo              string     `orm:"customer_no"`
	CustomerTypeCode        string     `orm:"customer_type_code"`
	CustomerStatus          string     `orm:"customer_status"`
	BindingStatus           string     `orm:"binding_status"`
	CurrencyCode            string     `orm:"currency_code"`
	OrderStatus             string     `orm:"order_status"`
	RiskDecisionCode        string     `orm:"risk_decision_code"`
	InvoiceStatus           string     `orm:"invoice_status"`
	TotalAmountMinor        int64      `orm:"total_amount_minor"`
	PaidAmountMinor         int64      `orm:"paid_amount_minor"`
	WaivedMinor             int64      `orm:"waived_amount_minor"`
	PaymentDueAt            time.Time  `orm:"payment_due_at"`
	SysUserID               int        `orm:"sys_user_id"`
	IdentityStatus          string     `orm:"identity_status"`
	IdentityExpiresAt       *time.Time `orm:"identity_expires_at"`
	ItemCount               int        `orm:"item_count"`
	HeldCount               int        `orm:"held_count"`
	ExistingAllocationCount int        `orm:"existing_allocation_count"`
}

// AllocateExceptionFund revalidates all current owner facts before applying
// one late payment to a different customer-created unpaid order.
func (s *channelPaymentOwnerService) AllocateExceptionFund(
	ctx context.Context,
	input billingcap.AllocateExceptionFund,
) (*billingcap.ExceptionFundAllocation, error) {
	if err := billingcap.ValidateAllocateExceptionFund(input); err != nil {
		return nil, err
	}
	var payment *billingcap.ChannelPaymentResult
	err := s.database.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		now, err := databaseTime(ctx, s.database)
		if err != nil {
			return err
		}
		var target *exceptionTargetRow
		if err = tx.Ctx(ctx).GetScan(&target, `
SELECT o.order_no,i.invoice_no,o.customer_no,customer.customer_type_code,
       customer.status_code AS customer_status,binding.status_code AS binding_status,
       o.currency_code,o.status_code AS order_status,o.risk_decision_code,
       i.status_code AS invoice_status,i.total_amount_minor,i.paid_amount_minor,
       i.waived_amount_minor,o.payment_due_at,binding.sys_user_id,
       CASE WHEN customer.customer_type_code='individual' THEN individual.identity_status_code
            ELSE enterprise.verification_status_code END AS identity_status,
       CASE WHEN customer.customer_type_code='individual' THEN individual.identity_expires_at
            ELSE enterprise.expires_at END AS identity_expires_at,
       o.item_count,
       (SELECT COUNT(*)::int FROM plugin_idc_core_commerce_inventory_reservation r
        WHERE r.tenant_id=0 AND r.order_no=o.order_no AND r.status_code='held'
          AND r.hold_expires_at>CURRENT_TIMESTAMP) AS held_count
FROM plugin_idc_core_commerce_order o
JOIN plugin_idc_core_billing_invoice i
  ON i.tenant_id=0 AND i.order_no=o.order_no AND i.source_code='order'
JOIN plugin_idc_core_customer_subject customer
  ON customer.tenant_id=0 AND customer.customer_no=o.customer_no
JOIN plugin_idc_core_customer_identity_binding binding
  ON binding.tenant_id=0 AND binding.customer_no=o.customer_no
LEFT JOIN plugin_idc_core_customer_individual_profile individual
  ON individual.tenant_id=0 AND individual.customer_no=o.customer_no
LEFT JOIN plugin_idc_core_customer_enterprise_profile enterprise
  ON enterprise.tenant_id=0 AND enterprise.customer_no=o.customer_no
WHERE o.tenant_id=0 AND o.order_no=?
FOR UPDATE OF o,i`, input.TargetOrderNo); err != nil {
			return err
		}
		if target != nil && target.InvoiceNo != "" {
			existing, loadErr := tx.Ctx(ctx).GetValue(`SELECT COUNT(*) FROM plugin_idc_core_billing_payment_allocation WHERE tenant_id=0 AND invoice_no=? AND payment_source_type_code='channel_payment' AND payment_source_reference=? AND currency_code=? AND amount_minor=?`, target.InvoiceNo, input.PaymentAttemptNo, input.CurrencyCode, input.AmountMinor)
			if loadErr != nil {
				return loadErr
			}
			target.ExistingAllocationCount = existing.Int()
		}
		replay := target != nil && target.ExistingAllocationCount == 1
		invalidOwner := target == nil || target.OrderNo == "" ||
			billingcap.PrincipalReferenceDigest(target.SysUserID) != input.CustomerRefDigest ||
			target.CurrencyCode != input.CurrencyCode
		invalidNewAllocation := !replay && (target == nil || target.CustomerStatus != "active" || target.BindingStatus != "active" ||
			target.OrderStatus != "pending_payment" || target.InvoiceStatus != "pending_payment" ||
			target.RiskDecisionCode == "manual_review" || target.IdentityStatus != "verified" ||
			(target.IdentityExpiresAt != nil && !target.IdentityExpiresAt.After(now)) ||
			target.ItemCount <= 0 || target.HeldCount != target.ItemCount || !target.PaymentDueAt.After(now) ||
			target.TotalAmountMinor-target.PaidAmountMinor-target.WaivedMinor != input.AmountMinor)
		if invalidOwner || invalidNewAllocation {
			return bizerr.NewCode(CodeInvoiceConflict)
		}
		payment, err = s.postConfirmedChannelPaymentLocked(ctx, billingcap.ConfirmedChannelPayment{
			PaymentAttemptNo: input.PaymentAttemptNo,
			InvoiceNo:        target.InvoiceNo, CurrencyCode: input.CurrencyCode,
			AmountMinor: input.AmountMinor, TotalAmountMinor: input.AmountMinor,
			EvidenceDigest: input.EvidenceDigest,
		})
		return err
	})
	if err != nil {
		return nil, normalizeInvoiceError(err)
	}
	if payment == nil {
		return nil, bizerr.NewCode(CodeInvoiceIntegrity)
	}
	if err = s.attachChannelPaymentSettlement(ctx, payment); err != nil {
		return nil, err
	}
	result := &billingcap.ExceptionFundAllocation{
		ExceptionNo: input.ExceptionNo, PaymentAttemptNo: input.PaymentAttemptNo,
		TargetOrderNo: input.TargetOrderNo, InvoiceNo: payment.InvoiceNo,
		AllocationNo: payment.AllocationNo, SettlementNo: payment.SettlementNo,
		CurrencyCode: input.CurrencyCode, AmountMinor: input.AmountMinor,
		Reused: payment.FundingReused || payment.SettlementReused,
	}
	if err = billingcap.ValidateExceptionFundAllocation(*result); err != nil {
		return nil, err
	}
	return result, nil
}
