// Package closureprecheck contributes compliance-owned aggregate facts to the
// core customer closure coordinator.
package closureprecheck

import (
	"context"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-plugin-idc-core/backend/cap/customerclosurecap"
)

type Contributor struct {
	database       gdb.DB
	databaseSource func() gdb.DB
}

func New(database gdb.DB) (*Contributor, error) {
	if database == nil {
		return nil, gerror.New("compliance closure contributor requires database")
	}
	return &Contributor{database: database}, nil
}

// NewLazy defers host database lookup until runtime so declaration-only tests
// do not require a process configuration file.
func NewLazy(source func() gdb.DB) (*Contributor, error) {
	if source == nil {
		return nil, gerror.New("compliance closure contributor requires database source")
	}
	return &Contributor{databaseSource: source}, nil
}

func (*Contributor) Codes() []customerclosurecap.Code {
	return []customerclosurecap.Code{
		customerclosurecap.CodeActiveContracts,
		customerclosurecap.CodeLegalRetention,
	}
}

type aggregateRow struct {
	CustomerNo string `json:"customer_no" orm:"customer_no"`
	Count      int    `json:"count" orm:"count"`
}

func (c *Contributor) BatchCount(ctx context.Context, customerNos []string, checkedAt time.Time) ([]customerclosurecap.Count, error) {
	if c == nil || checkedAt.IsZero() || len(customerNos) == 0 || len(customerNos) > customerclosurecap.MaximumBatchSize {
		return nil, gerror.New("compliance closure contributor batch is invalid")
	}
	database := c.database
	if database == nil && c.databaseSource != nil {
		database = c.databaseSource()
	}
	if database == nil {
		return nil, gerror.New("compliance closure contributor database is unavailable")
	}
	var activeRows []aggregateRow
	if err := database.GetScan(ctx, &activeRows, `SELECT customer_no, COUNT(1)::int AS count FROM plugin_idc_compliance_contract WHERE tenant_id=0 AND customer_no IN(?) AND status_code IN ('draft','pending_launch','signing','reissuing') GROUP BY customer_no`, customerNos); err != nil {
		return nil, gerror.Wrap(err, "count active customer contracts")
	}
	var retentionRows []aggregateRow
	if err := database.GetScan(ctx, &retentionRows, `SELECT customer_no, SUM(item_count)::int AS count FROM (SELECT c.customer_no, COUNT(1)::bigint AS item_count FROM plugin_idc_compliance_contract_file f JOIN plugin_idc_compliance_contract c ON c.tenant_id=0 AND c.contract_no=f.contract_no WHERE f.tenant_id=0 AND c.customer_no IN(?) AND f.status_code NOT IN ('deleted','deletion_pending') AND (f.legal_hold OR f.immutable OR f.retention_until IS NULL OR f.retention_until>?) GROUP BY c.customer_no UNION ALL SELECT vc.customer_no, COUNT(1)::bigint AS item_count FROM plugin_idc_compliance_enterprise_material m JOIN plugin_idc_compliance_verification_case vc ON vc.tenant_id=0 AND vc.case_no=m.case_no WHERE m.tenant_id=0 AND vc.customer_no IN(?) AND m.status_code NOT IN ('deleted','deletion_pending') AND (m.legal_hold OR m.retention_until IS NULL OR m.retention_until>?) GROUP BY vc.customer_no) retained GROUP BY customer_no`, customerNos, checkedAt, customerNos, checkedAt); err != nil {
		return nil, gerror.Wrap(err, "count retained customer compliance evidence")
	}
	active := make(map[string]int, len(activeRows))
	retained := make(map[string]int, len(retentionRows))
	for _, row := range activeRows {
		active[row.CustomerNo] = row.Count
	}
	for _, row := range retentionRows {
		retained[row.CustomerNo] = row.Count
	}
	result := make([]customerclosurecap.Count, 0, len(customerNos)*2)
	for _, customerNo := range customerNos {
		result = append(result,
			customerclosurecap.Count{CustomerNo: customerNo, Code: customerclosurecap.CodeActiveContracts, Count: active[customerNo]},
			customerclosurecap.Count{CustomerNo: customerNo, Code: customerclosurecap.CodeLegalRetention, Count: retained[customerNo]},
		)
	}
	return result, nil
}
