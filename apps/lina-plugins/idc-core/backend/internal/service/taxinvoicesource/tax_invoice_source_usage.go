// This file validates compliance-owned prior allocation projections.
package taxinvoicesource

import (
	"context"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-plugin-idc-core/backend/cap/taxinvoicecap"
)

// readUsage reads bounded prior usage through the compliance-owned contract and
// rejects rows outside the already-authorized invoice set.
func readUsage(
	ctx context.Context,
	reader taxinvoicecap.AllocationUsageReader,
	customerNo string,
	invoiceNos []string,
	requireItems bool,
) (map[string]taxinvoicecap.InvoiceUsage, error) {
	if len(invoiceNos) == 0 {
		return map[string]taxinvoicecap.InvoiceUsage{}, nil
	}
	rows, err := reader.ReadAllocatedUsage(ctx, customerNo, append([]string(nil), invoiceNos...))
	if err != nil {
		return nil, gerror.Wrap(err, "read compliance tax invoice allocation usage")
	}
	allowed := make(map[string]struct{}, len(invoiceNos))
	for _, invoiceNo := range invoiceNos {
		allowed[invoiceNo] = struct{}{}
	}
	result := make(map[string]taxinvoicecap.InvoiceUsage, len(rows))
	for _, row := range rows {
		if _, ok := allowed[row.BillingInvoiceNo]; !ok || row.AllocatedGrossMinor < 0 {
			return nil, gerror.New("compliance tax invoice allocation usage is inconsistent")
		}
		if _, duplicate := result[row.BillingInvoiceNo]; duplicate {
			return nil, gerror.New("compliance tax invoice allocation usage is duplicated")
		}
		var gross, net, tax int64
		itemNos := make(map[string]struct{}, len(row.Items))
		for _, item := range row.Items {
			if !businessNoPattern.MatchString(item.BillingInvoiceItemNo) ||
				item.AllocatedGrossMinor < 0 || item.AllocatedNetMinor < 0 ||
				item.AllocatedTaxMinor < 0 ||
				item.AllocatedGrossMinor != item.AllocatedNetMinor+item.AllocatedTaxMinor {
				return nil, gerror.New("compliance tax invoice item usage is inconsistent")
			}
			if _, duplicate := itemNos[item.BillingInvoiceItemNo]; duplicate {
				return nil, gerror.New("compliance tax invoice item usage is duplicated")
			}
			itemNos[item.BillingInvoiceItemNo] = struct{}{}
			if addOverflow(gross, item.AllocatedGrossMinor) || addOverflow(net, item.AllocatedNetMinor) ||
				addOverflow(tax, item.AllocatedTaxMinor) {
				return nil, gerror.New("compliance tax invoice item usage overflows")
			}
			gross += item.AllocatedGrossMinor
			net += item.AllocatedNetMinor
			tax += item.AllocatedTaxMinor
		}
		if requireItems && gross != row.AllocatedGrossMinor {
			return nil, gerror.New("compliance tax invoice allocation usage total is inconsistent")
		}
		result[row.BillingInvoiceNo] = row
	}
	return result, nil
}
