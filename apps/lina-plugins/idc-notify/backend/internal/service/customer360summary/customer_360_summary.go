// Package customer360summary reports notification counts without exposing
// recipient content or provider evidence to the core plugin.
package customer360summary

import (
	"context"
	"strings"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-plugin-idc-core/backend/cap/customer360cap"
)

type Contributor struct{ databaseSource func() gdb.DB }

func NewLazy(source func() gdb.DB) (*Contributor, error) {
	if source == nil {
		return nil, gerror.New("notification customer 360 contributor requires database source")
	}
	return &Contributor{databaseSource: source}, nil
}

func (*Contributor) Codes() []customer360cap.Code {
	return []customer360cap.Code{customer360cap.CodeNotifications}
}

func (c *Contributor) BatchCount(
	ctx context.Context,
	customerNos []string,
) ([]customer360cap.Count, error) {
	if len(customerNos) == 0 || len(customerNos) > customer360cap.MaximumBatchSize {
		return nil, gerror.New("notification customer 360 batch size is invalid")
	}
	database := c.databaseSource()
	if database == nil {
		return nil, gerror.New("notification customer 360 database is unavailable")
	}
	for _, customerNo := range customerNos {
		if customerNo == "" {
			return nil, gerror.New("notification customer 360 customer number is required")
		}
	}
	values := make([]string, len(customerNos))
	arguments := make([]any, len(customerNos))
	for index, customerNo := range customerNos {
		values[index], arguments[index] = "(?)", customerNo
	}
	query := `WITH requested(customer_no) AS (VALUES ` + strings.Join(values, ",") + `)
SELECT r.customer_no,COUNT(t.task_no) AS total FROM requested r
LEFT JOIN plugin_idc_notify_delivery_task t ON t.tenant_id=0 AND t.customer_no=r.customer_no
GROUP BY r.customer_no`
	rows, err := database.GetAll(ctx, query, arguments...)
	if err != nil {
		return nil, gerror.Wrap(err, "count notification customer 360 summary")
	}
	if len(rows) != len(customerNos) {
		return nil, gerror.New("notification customer 360 summary is incomplete")
	}
	result := make([]customer360cap.Count, 0, len(rows))
	for _, row := range rows {
		result = append(result, customer360cap.Count{
			CustomerNo: row["customer_no"].String(), Code: customer360cap.CodeNotifications,
			Total: row["total"].Int64(),
		})
	}
	return result, nil
}
