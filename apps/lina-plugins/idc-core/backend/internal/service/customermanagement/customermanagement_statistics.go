// This file aggregates authoritative billing and ticket facts for one visible customer.

package customermanagement

import (
	"context"
	"time"

	"github.com/gogf/gf/v2/errors/gerror"
	"lina-plugin-idc-core/backend/internal/dao"
	"lina-plugin-idc-core/backend/internal/service/billing"
	"lina-plugin-idc-core/backend/internal/service/supportdesk"
)

// allocationDirection identifies applied financial facts; reversals subtract them.
type allocationDirection string

const allocationApplied allocationDirection = "apply"

// CurrencyStatistics keeps monetary totals separate for each currency.
type CurrencyStatistics struct {
	CurrencyCode  string
	PaidMinor     int64
	RefundedMinor int64
	UnpaidMinor   int64
	UnpaidCount   int64
}

// TicketStatistics uses the support owner's base stages, including completed tickets.
type TicketStatistics struct {
	Total      int64
	Processing int64
	Replied    int64
	Closed     int64
}

// ProductCategoryStatistics counts purchased units by their authoritative delivery type and owner.
type ProductCategoryStatistics struct {
	ProductType string
	PluginID    string
	Total       int64
}

// maximumProductCategories bounds the customer overview classification response.
const maximumProductCategories = 64

// OperationsStatistics is a bounded aggregate; no historical secrets or configuration is exposed.
type OperationsStatistics struct {
	Currencies        []CurrencyStatistics
	ProductCategories []ProductCategoryStatistics
	Tickets           TicketStatistics
	Products          int64
	Expiring          int64
	Expired           int64
}

func loadOperationsStatistics(ctx context.Context, customerNo string) (OperationsStatistics, error) {
	result := OperationsStatistics{Currencies: []CurrencyStatistics{}, ProductCategories: []ProductCategoryStatistics{}}
	if err := dao.BillingInvoice.DB().GetScan(ctx, &result.Currencies, `
WITH amounts AS (
 SELECT currency_code, SUM(CASE direction_code WHEN ? THEN amount_minor ELSE -amount_minor END) AS paid_minor,0 AS refunded_minor,0 AS unpaid_minor,0 AS unpaid_count
 FROM plugin_idc_core_billing_payment_allocation WHERE tenant_id=0 AND customer_no=? AND payment_source_type_code IN (?,?,?) GROUP BY currency_code
 UNION ALL
 SELECT currency_code,0,SUM(CASE direction_code WHEN ? THEN amount_minor ELSE -amount_minor END),0,0
 FROM plugin_idc_core_billing_refund_allocation WHERE tenant_id=0 AND customer_no=? GROUP BY currency_code
 UNION ALL
 SELECT currency_code,0,0,SUM(total_amount_minor-paid_amount_minor-waived_amount_minor),COUNT(*)
 FROM plugin_idc_core_billing_invoice WHERE tenant_id=0 AND customer_no=? AND status_code IN (?,?,?) AND total_amount_minor>paid_amount_minor+waived_amount_minor GROUP BY currency_code
)
SELECT currency_code,SUM(paid_minor) AS paid_minor,SUM(refunded_minor) AS refunded_minor,SUM(unpaid_minor) AS unpaid_minor,SUM(unpaid_count) AS unpaid_count FROM amounts GROUP BY currency_code ORDER BY currency_code LIMIT 65`, allocationApplied, customerNo, billing.PaymentSourceBalance, billing.PaymentSourceChannel, billing.PaymentSourceManualReceipt, allocationApplied, customerNo, customerNo, billing.InvoiceStatusPendingPayment, billing.InvoiceStatusPartiallyPaid, billing.InvoiceStatusOverdue); err != nil {
		return result, gerror.Wrap(err, "load customer financial statistics")
	}
	if len(result.Currencies) > 64 {
		return result, gerror.New("customer currency statistics exceeds limit")
	}
	if err := dao.SupportTicket.DB().GetScan(ctx, &result.Tickets, `SELECT COUNT(*) AS total,SUM(CASE WHEN base_stage IN (?,?,?) THEN 1 ELSE 0 END) AS processing,SUM(CASE WHEN base_stage IN (?,?) THEN 1 ELSE 0 END) AS replied,SUM(CASE WHEN base_stage=? THEN 1 ELSE 0 END) AS closed FROM plugin_idc_core_support_ticket WHERE tenant_id=0 AND customer_no=?`, supportdesk.StageNew, supportdesk.StageInProgress, supportdesk.StageWaitingUpstream, supportdesk.StageWaitingCustomer, supportdesk.StageResolved, supportdesk.StageClosed, customerNo); err != nil {
		return result, gerror.Wrap(err, "load customer ticket statistics")
	}

	now := time.Now().UTC()
	// Count and lifecycle buckets share the exact purchased-product projection used by the list.
	query := `WITH products AS (
 SELECT COALESCE(s.entitlement_expires_at,e.expires_at) AS expires_at,
 COALESCE(s.entitlement_status_code,e.status_code,o.status_code) AS status` + customerProductsFrom + `
 ) SELECT COUNT(*) AS products,
 SUM(CASE WHEN expires_at>? AND expires_at<=? AND status NOT IN (?,?,?) THEN 1 ELSE 0 END) AS expiring,
 SUM(CASE WHEN expires_at<=? AND status NOT IN (?,?,?) THEN 1 ELSE 0 END) AS expired FROM products`
	if err := dao.CommerceOrderItem.DB().GetScan(ctx, &result, query, customerNo, now, now.AddDate(0, 0, 30), productLifecycleTerminated, productLifecycleCancelled, productLifecycleRefunded, now, productLifecycleTerminated, productLifecycleCancelled, productLifecycleRefunded); err != nil {
		return result, gerror.Wrap(err, "load customer purchased product statistics")
	}
	// A single grouped projection includes pending purchases and excludes renewals and other customers.
	categoriesQuery := `WITH products AS (SELECT ` + customerProductTypeProjection + ` AS product_type,` + customerProductPluginProjection + ` AS plugin_id` + customerProductsFrom + `)
 SELECT product_type,plugin_id,COUNT(*) AS total FROM products GROUP BY product_type,plugin_id ORDER BY product_type,plugin_id LIMIT ?`
	if err := dao.CommerceOrderItem.DB().GetScan(ctx, &result.ProductCategories, categoriesQuery, customerNo, maximumProductCategories+1); err != nil {
		return result, gerror.Wrap(err, "load customer product category statistics")
	}
	if len(result.ProductCategories) > maximumProductCategories {
		return result, gerror.New("customer product category statistics exceeds limit")
	}
	return result, nil
}
