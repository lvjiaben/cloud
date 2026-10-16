// This file freezes payment-time ownership before commission rules are applied.
// Historical assignment windows and append-only manual revisions prevent a late
// worker or refund from reassigning revenue to the customer's current owner.

package commission

import (
	"context"
	"time"

	"lina-plugin-idc-core/backend/internal/dao"
	"lina-plugin-idc-core/backend/internal/model/do"
	"lina-plugin-idc-core/backend/internal/model/entity"
)

// freezePaymentFacts runs under the invoice lock and in the receipt transaction.
// No-rule and lawful zero-amount payments still retain independent sales facts.
func (c *consumer) freezePaymentFacts(ctx context.Context, invoice *entity.BillingInvoice, settlementNo, digest, operation string, paid int64, at time.Time) ([]beneficiary, error) {
	owners, err := c.loadBeneficiaries(ctx, invoice.CustomerNo, at)
	if err != nil {
		return nil, err
	}
	if _, err = c.database.Model(dao.SalesPaymentFact.Table()).Ctx(ctx).Data(do.SalesPaymentFact{
		TenantId: 0, InvoiceNo: invoice.InvoiceNo, OrderNo: invoice.OrderNo,
		CustomerNo: invoice.CustomerNo, CurrencyCode: invoice.CurrencyCode,
		OperationTypeCode: operation, ReceivedAmountMinor: paid, ReceivedAt: &at,
		SettlementNo: settlementNo, SourceDigest: digest,
	}).Insert(); err != nil {
		return nil, err
	}
	rows := make([]do.SalesPaymentOwner, 0, len(owners))
	for _, owner := range owners {
		row := do.SalesPaymentOwner{TenantId: 0, InvoiceNo: invoice.InvoiceNo, BeneficiaryTypeCode: owner.Kind, BeneficiaryReference: owner.Reference}
		if owner.Assignment != "" {
			row.SalesAssignmentNo = &owner.Assignment
		}
		if owner.Attribution != "" {
			row.AffiliateAttributionNo = &owner.Attribution
		}
		if owner.ManualReferrerID > 0 {
			row.ManualReferrerId = &owner.ManualReferrerID
		}
		rows = append(rows, row)
	}
	if len(rows) > 0 {
		if _, err = c.database.Model(dao.SalesPaymentOwner.Table()).Ctx(ctx).Data(rows).Insert(); err != nil {
			return nil, err
		}
	}
	return owners, nil
}

// loadBeneficiaries reads only relationships effective at the payment instant.
// A manual NULL is an explicit cancellation; it blocks legacy link fallback.
func (c *consumer) loadBeneficiaries(ctx context.Context, customer string, at time.Time) ([]beneficiary, error) {
	var rows []struct {
		Kind, Reference, Assignment, Attribution string
		ManualReferrerID                         int64 `orm:"manual_referrer_id"`
	}
	err := c.database.GetScan(ctx, &rows, `WITH manual AS (
 SELECT id,referrer_customer_no FROM plugin_idc_core_sales_customer_referrer
 WHERE tenant_id=0 AND customer_no=? AND effective_at<=? ORDER BY version DESC LIMIT 1
 )
 SELECT 'sales' AS kind,m.staff_user_id::text AS reference,a.assignment_no AS assignment,'' AS attribution,0::bigint AS manual_referrer_id
 FROM plugin_idc_core_sales_customer_assignment a JOIN plugin_idc_core_sales_group_member m ON m.tenant_id=a.tenant_id AND m.membership_no=a.membership_no
 WHERE a.tenant_id=0 AND a.customer_no=? AND a.effective_at<=? AND (a.ended_at IS NULL OR a.ended_at>?)
 AND m.effective_at<=? AND (m.ended_at IS NULL OR m.ended_at>?)
 UNION ALL
 SELECT 'affiliate',referrer_customer_no,'','',id FROM manual WHERE referrer_customer_no IS NOT NULL AND referrer_customer_no<>?
 UNION ALL
 SELECT 'affiliate',referrer_customer_no,'',attribution_no,0::bigint FROM plugin_idc_core_sales_affiliate_attribution
 WHERE tenant_id=0 AND referred_customer_no=? AND referrer_customer_no<>referred_customer_no AND status_code='eligible'
 AND attributed_at<=? AND (expires_at IS NULL OR expires_at>?) AND NOT EXISTS(SELECT 1 FROM manual)`, customer, at, customer, at, at, at, at, customer, customer, at, at)
	if err != nil {
		return nil, err
	}
	result := make([]beneficiary, 0, len(rows))
	seen := map[string]bool{}
	for _, row := range rows {
		if seen[row.Kind] {
			return nil, permanent("commission_attribution_ambiguous")
		}
		seen[row.Kind] = true
		result = append(result, beneficiary{Kind: row.Kind, Reference: row.Reference, Assignment: row.Assignment, Attribution: row.Attribution, ManualReferrerID: row.ManualReferrerID})
	}
	return result, nil
}
