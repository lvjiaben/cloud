package billing

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
)

func (s *managementFinanceDirectoryImpl) GetManagementFinancialOverview(
	ctx context.Context,
) (*ManagementFinancialOverview, error) {
	if _, err := s.authorizer.AuthorizeFinancialOverview(ctx); err != nil {
		return nil, err
	}
	result := &ManagementFinancialOverview{Receivables: []ManagementReceivableSummary{}}
	err := s.database.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		if _, setErr := tx.Exec("SET TRANSACTION ISOLATION LEVEL REPEATABLE READ, READ ONLY"); setErr != nil {
			return setErr
		}
		var invoiceCounts struct {
			Open    int `orm:"open_count"`
			Overdue int `orm:"overdue_count"`
		}
		if loadErr := tx.GetScan(&invoiceCounts, `
SELECT COUNT(*) FILTER (WHERE status_code IN ('pending_payment','partially_paid','overdue')) AS open_count,
       COUNT(*) FILTER (WHERE status_code = 'overdue') AS overdue_count
FROM plugin_idc_core_billing_invoice
WHERE tenant_id = 0`); loadErr != nil {
			return gerror.Wrap(loadErr, "load management invoice counts")
		}
		result.OpenInvoiceCount = invoiceCounts.Open
		result.OverdueInvoiceCount = invoiceCounts.Overdue
		pendingAdjustments, loadErr := tx.GetValue(`
SELECT COUNT(*) FROM plugin_idc_core_billing_adjustment_request
WHERE tenant_id = 0 AND status_code = 'pending_review'`)
		if loadErr != nil {
			return gerror.Wrap(loadErr, "load pending financial adjustments")
		}
		result.PendingAdjustmentCount = pendingAdjustments.Int()
		rows := make([]ManagementReceivableSummary, 0, maximumManagementCurrencyCount+1)
		if loadErr := tx.GetScan(&rows, `
SELECT currency_code,
       SUM(total_amount_minor - paid_amount_minor - waived_amount_minor
           - credit_amount_minor) AS outstanding_minor
FROM plugin_idc_core_billing_invoice
WHERE tenant_id = 0 AND status_code IN ('pending_payment','partially_paid','overdue')
GROUP BY currency_code ORDER BY currency_code ASC LIMIT ?`, maximumManagementCurrencyCount+1); loadErr != nil {
			return gerror.Wrap(loadErr, "load management receivables")
		}
		if len(rows) > maximumManagementCurrencyCount {
			return bizerr.NewCode(CodeDirectoryIntegrity)
		}
		for _, row := range rows {
			if row.OutstandingMinor < 0 {
				return bizerr.NewCode(CodeDirectoryIntegrity)
			}
		}
		result.Receivables = rows
		return nil
	})
	if err != nil {
		if _, ok := bizerr.As(err); ok {
			return nil, err
		}
		return nil, bizerr.WrapCode(err, CodeDirectoryUnavailable)
	}
	return result, nil
}
