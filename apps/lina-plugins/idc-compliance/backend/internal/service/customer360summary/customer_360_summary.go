// Package customer360summary reports compliance-owned counts to the core
// customer 360 coordinator without exposing compliance rows.
package customer360summary

import (
	"context"
	"strings"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-plugin-idc-core/backend/cap/customer360cap"
)

type Contributor struct {
	database       gdb.DB
	databaseSource func() gdb.DB
}

func NewLazy(source func() gdb.DB) (*Contributor, error) {
	if source == nil {
		return nil, gerror.New("compliance customer 360 contributor requires database source")
	}
	return &Contributor{databaseSource: source}, nil
}

func (*Contributor) Codes() []customer360cap.Code {
	return []customer360cap.Code{customer360cap.CodeContracts, customer360cap.CodeTaxInvoices}
}

func (c *Contributor) BatchCount(
	ctx context.Context,
	customerNos []string,
) ([]customer360cap.Count, error) {
	if len(customerNos) == 0 || len(customerNos) > customer360cap.MaximumBatchSize {
		return nil, gerror.New("compliance customer 360 batch size is invalid")
	}
	database := c.database
	if database == nil && c.databaseSource != nil {
		database = c.databaseSource()
	}
	if database == nil {
		return nil, gerror.New("compliance customer 360 database is unavailable")
	}
	for _, customerNo := range customerNos {
		if customerNo == "" {
			return nil, gerror.New("compliance customer 360 customer number is required")
		}
	}
	values := make([]string, len(customerNos))
	arguments := make([]any, len(customerNos))
	for index, customerNo := range customerNos {
		values[index], arguments[index] = "(?)", customerNo
	}
	query := `WITH requested(customer_no) AS (VALUES ` + strings.Join(values, ",") + `)
SELECT r.customer_no,'contracts' AS code,COUNT(c.id) AS total FROM requested r
LEFT JOIN plugin_idc_compliance_contract c ON c.tenant_id=0 AND c.customer_no=r.customer_no GROUP BY r.customer_no
UNION ALL SELECT r.customer_no,'tax_invoices',COUNT(i.id) FROM requested r
LEFT JOIN plugin_idc_compliance_tax_invoice i ON i.tenant_id=0 AND i.customer_no=r.customer_no GROUP BY r.customer_no`
	rows, err := database.GetAll(ctx, query, arguments...)
	if err != nil {
		return nil, gerror.Wrap(err, "count compliance customer 360 summaries")
	}
	if len(rows) != len(customerNos)*2 {
		return nil, gerror.New("compliance customer 360 summary is incomplete")
	}
	result := make([]customer360cap.Count, 0, len(rows))
	for _, row := range rows {
		result = append(result, customer360cap.Count{
			CustomerNo: row["customer_no"].String(), Code: customer360cap.Code(row["code"].String()),
			Total: row["total"].Int64(),
		})
	}
	return result, nil
}
