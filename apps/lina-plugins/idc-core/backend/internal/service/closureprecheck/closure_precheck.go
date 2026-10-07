// Package closureprecheck composes the complete, fail-closed customer closure
// precheck without letting the customer profile service query foreign domains.
package closureprecheck

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-plugin-idc-core/backend/cap/customerclosurecap"
	"lina-plugin-idc-core/backend/internal/service/customer"
)

const maximumAggregateCount = 1_000_000_000

// Service coordinates immutable startup contributors and emits the customer
// domain's versioned projection.
type Service struct {
	database     gdb.DB
	contributors []customerclosurecap.Contributor
}

func New(database gdb.DB, contributors []customerclosurecap.Contributor) (*Service, error) {
	if database == nil || len(contributors) == 0 {
		return nil, gerror.New("customer closure precheck requires database and contributors")
	}
	return &Service{database: database, contributors: append([]customerclosurecap.Contributor(nil), contributors...)}, nil
}

// BatchCheck obtains one PostgreSQL timestamp and requires every owner to
// return exactly one result per customer and code.
func (s *Service) BatchCheck(ctx context.Context, customerNos []string) ([]customer.ClosurePrecheckResult, error) {
	if s == nil || len(customerNos) == 0 || len(customerNos) > customerclosurecap.MaximumBatchSize {
		return nil, gerror.New("customer closure precheck batch is invalid")
	}
	seenCustomers := make(map[string]struct{}, len(customerNos))
	for _, customerNo := range customerNos {
		if customerNo == "" {
			return nil, gerror.New("customer closure precheck customer is invalid")
		}
		if _, duplicate := seenCustomers[customerNo]; duplicate {
			return nil, gerror.New("customer closure precheck customer is duplicated")
		}
		seenCustomers[customerNo] = struct{}{}
	}
	var checkedAt time.Time
	if err := s.database.GetScan(ctx, &checkedAt, "SELECT CURRENT_TIMESTAMP"); err != nil || checkedAt.IsZero() {
		return nil, gerror.Wrap(err, "read customer closure precheck database time")
	}
	type key struct {
		customerNo string
		code       customerclosurecap.Code
	}
	counts := make(map[key]int, len(customerNos)*6)
	declared := make(map[customerclosurecap.Code]struct{}, 6)
	for _, contributor := range s.contributors {
		if contributor == nil {
			return nil, gerror.New("customer closure precheck contributor is unavailable")
		}
		owned := make(map[customerclosurecap.Code]struct{}, len(contributor.Codes()))
		for _, code := range contributor.Codes() {
			if _, duplicate := declared[code]; duplicate {
				return nil, gerror.New("customer closure precheck code has duplicate owners")
			}
			declared[code], owned[code] = struct{}{}, struct{}{}
		}
		items, err := contributor.BatchCount(ctx, append([]string(nil), customerNos...), checkedAt)
		if err != nil {
			return nil, gerror.Wrap(err, "run customer closure precheck contributor")
		}
		for _, item := range items {
			if _, ok := seenCustomers[item.CustomerNo]; !ok {
				return nil, gerror.New("customer closure precheck returned an unknown customer")
			}
			if _, ok := owned[item.Code]; !ok || item.Count < 0 || item.Count > maximumAggregateCount {
				return nil, gerror.New("customer closure precheck returned an invalid count")
			}
			itemKey := key{customerNo: item.CustomerNo, code: item.Code}
			if _, duplicate := counts[itemKey]; duplicate {
				return nil, gerror.New("customer closure precheck returned a duplicate count")
			}
			counts[itemKey] = item.Count
		}
		for customerNo := range seenCustomers {
			for code := range owned {
				if _, ok := counts[key{customerNo: customerNo, code: code}]; !ok {
					return nil, gerror.New("customer closure precheck returned an incomplete result")
				}
			}
		}
	}
	if len(declared) != 6 {
		return nil, gerror.New("customer closure precheck coverage is incomplete")
	}
	results := make([]customer.ClosurePrecheckResult, 0, len(customerNos))
	for _, customerNo := range customerNos {
		findings := make([]customer.ClosureFinding, 0, 6)
		for code := range declared {
			count := counts[key{customerNo: customerNo, code: code}]
			if count > 0 {
				findings = append(findings, customer.ClosureFinding{Code: customer.ClosureFindingCode(code), Count: count})
			}
		}
		sort.Slice(findings, func(i, j int) bool { return findings[i].Code < findings[j].Code })
		results = append(results, customer.ClosurePrecheckResult{
			CustomerNo: customerNo, ContractVersion: customerclosurecap.ContractVersion,
			CheckedAt: checkedAt, Findings: findings,
		})
	}
	return results, nil
}

// CoreContributor owns the four closure facts whose rows belong to the core plugin.
type CoreContributor struct {
	database       gdb.DB
	databaseSource func() gdb.DB
}

func NewCoreContributor(database gdb.DB) (*CoreContributor, error) {
	if database == nil {
		return nil, gerror.New("core closure contributor requires database")
	}
	return &CoreContributor{database: database}, nil
}

// NewLazyCoreContributor defers host database lookup until runtime so package
// compilation and declaration tests do not require a process config file.
func NewLazyCoreContributor(source func() gdb.DB) (*CoreContributor, error) {
	if source == nil {
		return nil, gerror.New("core closure contributor requires database source")
	}
	return &CoreContributor{databaseSource: source}, nil
}

func (*CoreContributor) Codes() []customerclosurecap.Code {
	return []customerclosurecap.Code{
		customerclosurecap.CodeUnsettledFunds,
		customerclosurecap.CodeActiveServices,
		customerclosurecap.CodeUnsettledInvoices,
		customerclosurecap.CodeInProgressOperations,
	}
}

type aggregateRow struct {
	CustomerNo string `json:"customer_no" orm:"customer_no"`
	Count      int    `json:"count" orm:"count"`
}

func (c *CoreContributor) BatchCount(ctx context.Context, customerNos []string, _ time.Time) ([]customerclosurecap.Count, error) {
	if c == nil || len(customerNos) == 0 || len(customerNos) > customerclosurecap.MaximumBatchSize {
		return nil, gerror.New("core closure contributor batch is invalid")
	}
	database := c.database
	if database == nil && c.databaseSource != nil {
		database = c.databaseSource()
	}
	if database == nil {
		return nil, gerror.New("core closure contributor database is unavailable")
	}
	counts := make(map[customerclosurecap.Code]map[string]int, 4)
	for _, code := range c.Codes() {
		counts[code] = make(map[string]int, len(customerNos))
	}
	queries := []struct {
		code  customerclosurecap.Code
		query string
	}{
		{customerclosurecap.CodeActiveServices, `SELECT customer_no, COUNT(1)::int AS count FROM plugin_idc_core_service_entitlement WHERE tenant_id=0 AND customer_no IN(?) AND status_code IN ('active','suspended') GROUP BY customer_no`},
		{customerclosurecap.CodeUnsettledInvoices, `SELECT customer_no, COUNT(1)::int AS count FROM plugin_idc_core_billing_invoice WHERE tenant_id=0 AND customer_no IN(?) AND status_code IN ('draft','pending_payment','partially_paid','overdue','partially_refunded','bad_debt') GROUP BY customer_no`},
		{customerclosurecap.CodeInProgressOperations, `SELECT customer_no, COUNT(1)::int AS count FROM plugin_idc_core_business_operation WHERE tenant_id=0 AND customer_no IN(?) AND status_code IN ('quoted','awaiting_financial_settlement','financially_settled','ready','executing','compensating','refunding','manual_review') GROUP BY customer_no`},
		{customerclosurecap.CodeUnsettledFunds, `SELECT customer_no, SUM(item_count)::int AS count FROM (SELECT a.customer_no, COUNT(1)::bigint AS item_count FROM plugin_idc_core_billing_ledger_account a JOIN LATERAL (SELECT s.closing_balance_minor FROM plugin_idc_core_billing_balance_snapshot s WHERE s.tenant_id=0 AND s.account_no=a.account_no ORDER BY s.sequence_no DESC LIMIT 1) latest ON TRUE WHERE a.tenant_id=0 AND a.customer_no IN(?) AND a.status_code IN ('active','frozen') AND latest.closing_balance_minor <> 0 GROUP BY a.customer_no UNION ALL SELECT h.customer_no, COUNT(1)::bigint AS item_count FROM plugin_idc_core_billing_fund_hold h WHERE h.tenant_id=0 AND h.customer_no IN(?) AND h.status_code IN ('active','partially_consumed') GROUP BY h.customer_no) unsettled GROUP BY customer_no`},
	}
	for _, item := range queries {
		var rows []aggregateRow
		args := []any{customerNos}
		if item.code == customerclosurecap.CodeUnsettledFunds {
			args = append(args, customerNos)
		}
		if err := database.GetScan(ctx, &rows, item.query, args...); err != nil {
			return nil, gerror.Wrap(err, fmt.Sprintf("count %s for customer closure", item.code))
		}
		for _, row := range rows {
			counts[item.code][row.CustomerNo] = row.Count
		}
	}
	result := make([]customerclosurecap.Count, 0, len(customerNos)*4)
	for _, customerNo := range customerNos {
		for _, code := range c.Codes() {
			result = append(result, customerclosurecap.Count{CustomerNo: customerNo, Code: code, Count: counts[code][customerNo]})
		}
	}
	return result, nil
}
