// Package customer360summary reports server counts from the resource-owned
// customer ownership projection without exposing resource or Provider data.
package customer360summary

import (
	"context"
	"strings"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-plugin-idc-core/backend/cap/customer360cap"
)

type resultQuery interface {
	GetAll(context.Context, string, ...any) (gdb.Result, error)
}

type Contributor struct{ databaseSource func() gdb.DB }

func NewLazy(source func() gdb.DB) (*Contributor, error) {
	if source == nil {
		return nil, gerror.New("resource customer 360 contributor requires database source")
	}
	return &Contributor{databaseSource: source}, nil
}

func (*Contributor) Codes() []customer360cap.Code {
	return []customer360cap.Code{customer360cap.CodeServers}
}

// BatchCount uses one complete-set query for the entire bounded customer
// batch. Non-server entitlements cannot enter the resource-owned projection.
func (contributor *Contributor) BatchCount(
	ctx context.Context,
	customerNos []string,
) ([]customer360cap.Count, error) {
	if contributor == nil || contributor.databaseSource == nil {
		return nil, gerror.New("resource customer 360 contributor is unavailable")
	}
	database := contributor.databaseSource()
	if database == nil {
		return nil, gerror.New("resource customer 360 database is unavailable")
	}
	return batchCount(ctx, database, customerNos)
}

func batchCount(
	ctx context.Context,
	database resultQuery,
	customerNos []string,
) ([]customer360cap.Count, error) {
	if ctx == nil || ctx.Err() != nil || database == nil || len(customerNos) == 0 ||
		len(customerNos) > customer360cap.MaximumBatchSize {
		return nil, gerror.New("resource customer 360 batch is invalid")
	}
	seen := make(map[string]struct{}, len(customerNos))
	values := make([]string, len(customerNos))
	arguments := make([]any, len(customerNos))
	for index, customerNo := range customerNos {
		if customerNo == "" {
			return nil, gerror.New("resource customer 360 customer number is required")
		}
		if _, duplicated := seen[customerNo]; duplicated {
			return nil, gerror.New("resource customer 360 customer number is duplicated")
		}
		seen[customerNo] = struct{}{}
		values[index], arguments[index] = "(?)", customerNo
	}
	rows, err := database.GetAll(ctx, `WITH requested(customer_no) AS (VALUES `+
		strings.Join(values, ",")+`)
SELECT requested.customer_no,
       COUNT(projection.entitlement_no) FILTER (WHERE mapping.mapping_no IS NOT NULL) AS total
FROM requested
LEFT JOIN plugin_idc_resource_server_entitlement_projection projection
  ON projection.tenant_id=0 AND projection.customer_no=requested.customer_no
LEFT JOIN plugin_idc_resource_mapping mapping
  ON mapping.tenant_id=projection.tenant_id
 AND mapping.mapping_no=projection.mapping_no
 AND mapping.local_resource_no=projection.local_resource_no
 AND mapping.status_code<>'released'
GROUP BY requested.customer_no
ORDER BY requested.customer_no`, arguments...)
	if err != nil {
		return nil, gerror.Wrap(err, "count resource customer 360 servers")
	}
	if len(rows) != len(customerNos) {
		return nil, gerror.New("resource customer 360 summary is incomplete")
	}
	result := make([]customer360cap.Count, 0, len(rows))
	for _, row := range rows {
		result = append(result, customer360cap.Count{
			CustomerNo: row["customer_no"].String(),
			Code:       customer360cap.CodeServers,
			Total:      row["total"].Int64(),
		})
	}
	return result, nil
}
