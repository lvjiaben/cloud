// This file returns bounded reservation usage without per-row database queries.

package taxinvoicerequest

import (
	"context"
	"sort"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-plugin-idc-compliance/backend/internal/dao"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/cap/taxinvoicecap"
)

type allocationUsageRow struct {
	BillingInvoiceNo    string `orm:"billing_invoice_no"`
	AllocatedGrossMinor int64  `orm:"allocated_gross_minor"`
}

type itemUsageRow struct {
	BillingInvoiceNo     string `orm:"billing_invoice_no"`
	BillingInvoiceItemNo string `orm:"billing_invoice_item_no"`
	AllocatedGrossMinor  int64  `orm:"allocated_gross_minor"`
	AllocatedNetMinor    int64  `orm:"allocated_net_minor"`
	AllocatedTaxMinor    int64  `orm:"allocated_tax_minor"`
}

// ReadAllocatedUsage counts every submitted state except an explicit
// pre-issuance rejection or cancellation. Failed and result-unknown requests
// deliberately remain reserved until a later explicit decision.
func (s *serviceImpl) ReadAllocatedUsage(
	ctx context.Context,
	customerNo string,
	billingInvoiceNos []string,
) ([]taxinvoicecap.InvoiceUsage, error) {
	if s == nil || ctx == nil || ctx.Err() != nil || !validBusinessNo(customerNo, 64) ||
		len(billingInvoiceNos) == 0 || len(billingInvoiceNos) > taxinvoicecap.MaximumInvoices {
		return nil, gerror.New("tax invoice allocation usage input is invalid")
	}
	numbers := append([]string(nil), billingInvoiceNos...)
	sort.Strings(numbers)
	for index, number := range numbers {
		if !validBusinessNo(number, 64) || index > 0 && number == numbers[index-1] {
			clear(numbers)
			return nil, gerror.New("tax invoice allocation usage scope is invalid")
		}
	}
	defer clear(numbers)
	activeStatuses := []string{string(StatusRejected), string(StatusCanceled)}
	var allocations []allocationUsageRow
	err := s.database.Model(dao.TaxInvoiceAllocation.Table()).Ctx(ctx).As("allocation").
		InnerJoin(dao.TaxInvoice.Table()+" invoice",
			"invoice.tenant_id = allocation.tenant_id AND invoice.tax_invoice_no = allocation.tax_invoice_no").
		Fields(`allocation.billing_invoice_no,
SUM(allocation.allocated_gross_minor) AS allocated_gross_minor`).
		Where("invoice.tenant_id", idccap.DefaultTenantID).
		Where("invoice.customer_no", customerNo).
		WhereNotIn("invoice.status_code", activeStatuses).
		WhereIn("allocation.billing_invoice_no", numbers).
		Group("allocation.billing_invoice_no").Scan(&allocations)
	if err != nil {
		return nil, gerror.Wrap(err, "read tax invoice allocation usage")
	}
	var items []itemUsageRow
	err = s.database.Model(dao.TaxInvoiceItem.Table()).Ctx(ctx).As("item").
		InnerJoin(dao.TaxInvoiceAllocation.Table()+" allocation",
			"allocation.tenant_id = item.tenant_id AND allocation.allocation_no = item.allocation_no AND allocation.tax_invoice_no = item.tax_invoice_no").
		InnerJoin(dao.TaxInvoice.Table()+" invoice",
			"invoice.tenant_id = allocation.tenant_id AND invoice.tax_invoice_no = allocation.tax_invoice_no").
		Fields(`allocation.billing_invoice_no, item.billing_invoice_item_no,
SUM(item.gross_amount_minor) AS allocated_gross_minor,
SUM(item.net_amount_minor) AS allocated_net_minor,
SUM(item.tax_amount_minor) AS allocated_tax_minor`).
		Where("invoice.tenant_id", idccap.DefaultTenantID).
		Where("invoice.customer_no", customerNo).
		WhereNotIn("invoice.status_code", activeStatuses).
		WhereIn("allocation.billing_invoice_no", numbers).
		Group("allocation.billing_invoice_no, item.billing_invoice_item_no").Scan(&items)
	if err != nil {
		return nil, gerror.Wrap(err, "read tax invoice item usage")
	}
	resultByInvoice := make(map[string]*taxinvoicecap.InvoiceUsage, len(allocations))
	for _, row := range allocations {
		if !validBusinessNo(row.BillingInvoiceNo, 64) || row.AllocatedGrossMinor <= 0 {
			return nil, gerror.New("stored tax invoice allocation usage is invalid")
		}
		resultByInvoice[row.BillingInvoiceNo] = &taxinvoicecap.InvoiceUsage{
			BillingInvoiceNo: row.BillingInvoiceNo, AllocatedGrossMinor: row.AllocatedGrossMinor,
		}
	}
	for _, row := range items {
		usage := resultByInvoice[row.BillingInvoiceNo]
		if usage == nil || !validBusinessNo(row.BillingInvoiceItemNo, 64) ||
			row.AllocatedGrossMinor <= 0 || row.AllocatedNetMinor < 0 || row.AllocatedTaxMinor < 0 ||
			row.AllocatedGrossMinor != row.AllocatedNetMinor+row.AllocatedTaxMinor {
			return nil, gerror.New("stored tax invoice item usage is invalid")
		}
		usage.Items = append(usage.Items, taxinvoicecap.ItemUsage{
			BillingInvoiceItemNo: row.BillingInvoiceItemNo,
			AllocatedGrossMinor:  row.AllocatedGrossMinor,
			AllocatedNetMinor:    row.AllocatedNetMinor,
			AllocatedTaxMinor:    row.AllocatedTaxMinor,
		})
	}
	result := make([]taxinvoicecap.InvoiceUsage, 0, len(resultByInvoice))
	for _, number := range numbers {
		usage := resultByInvoice[number]
		if usage == nil {
			continue
		}
		sort.Slice(usage.Items, func(left, right int) bool {
			return usage.Items[left].BillingInvoiceItemNo < usage.Items[right].BillingInvoiceItemNo
		})
		var itemGross int64
		for _, item := range usage.Items {
			itemGross += item.AllocatedGrossMinor
		}
		if itemGross != usage.AllocatedGrossMinor {
			return nil, gerror.New("stored tax invoice allocation and item usage disagree")
		}
		result = append(result, *usage)
	}
	return result, nil
}
