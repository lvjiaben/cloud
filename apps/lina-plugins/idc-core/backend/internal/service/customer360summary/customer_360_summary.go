// Package customer360summary composes bounded customer summary counts without
// reading tables owned by another plugin.
package customer360summary

import (
	"context"
	"reflect"
	"strings"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-plugin-idc-core/backend/cap/customer360cap"
)

var requiredCodes = []customer360cap.Code{
	customer360cap.CodeOrders,
	customer360cap.CodeBills,
	customer360cap.CodeTransactions,
	customer360cap.CodeServers,
	customer360cap.CodeContracts,
	customer360cap.CodeTaxInvoices,
	customer360cap.CodeTickets,
	customer360cap.CodeNotifications,
	customer360cap.CodeAudits,
}

// Summary is a complete, display-safe customer 360 count projection.
type Summary struct {
	Orders        int64
	Bills         int64
	Transactions  int64
	Servers       int64
	Contracts     int64
	TaxInvoices   int64
	Tickets       int64
	Notifications int64
	Audits        int64
	Balances      []Balance
}

// Balance is a display-safe customer balance projection. Internal ledger
// account numbers are deliberately excluded from the management overview.
type Balance struct {
	AccountTypeCode       string
	CurrencyCode          string
	StatusCode            string
	BalanceMinor          int64
	HeldBalanceMinor      int64
	AvailableBalanceMinor int64
	UpdatedAt             time.Time
}

type balanceReader interface {
	Balances(context.Context, string) ([]Balance, error)
}

// Reader loads complete summaries. Missing, duplicate, negative, or foreign
// contributor output fails closed instead of showing misleading zeros.
type Reader interface {
	Get(context.Context, string) (Summary, error)
}

type reader struct {
	contributors []customer360cap.Contributor
	balances     balanceReader
}

func New(contributors []customer360cap.Contributor) (Reader, error) {
	if len(contributors) == 0 {
		return nil, gerror.New("customer 360 contributors are required")
	}
	covered := make(map[customer360cap.Code]struct{}, len(requiredCodes))
	var balances balanceReader
	for _, contributor := range contributors {
		if nilInterface(contributor) || len(contributor.Codes()) == 0 {
			return nil, gerror.New("customer 360 contributor is invalid")
		}
		for _, code := range contributor.Codes() {
			if _, exists := covered[code]; exists {
				return nil, gerror.New("customer 360 contributor ownership is duplicated")
			}
			covered[code] = struct{}{}
		}
		if candidate, ok := contributor.(balanceReader); ok {
			if balances != nil {
				return nil, gerror.New("customer 360 balance ownership is duplicated")
			}
			balances = candidate
		}
	}
	if len(covered) != len(requiredCodes) {
		return nil, gerror.New("customer 360 contributor coverage is incomplete")
	}
	for _, code := range requiredCodes {
		if _, exists := covered[code]; !exists {
			return nil, gerror.New("customer 360 required summary has no owner")
		}
	}
	return &reader{
		contributors: append([]customer360cap.Contributor(nil), contributors...),
		balances:     balances,
	}, nil
}

func (r *reader) Get(ctx context.Context, customerNo string) (Summary, error) {
	if r == nil || customerNo == "" {
		return Summary{}, gerror.New("customer 360 summary request is invalid")
	}
	counts := make(map[customer360cap.Code]int64, len(requiredCodes))
	for _, contributor := range r.contributors {
		items, err := contributor.BatchCount(ctx, []string{customerNo})
		if err != nil {
			return Summary{}, gerror.Wrap(err, "load customer 360 domain summary")
		}
		if len(items) != len(contributor.Codes()) {
			return Summary{}, gerror.New("customer 360 domain summary is incomplete")
		}
		owned := make(map[customer360cap.Code]struct{}, len(contributor.Codes()))
		for _, code := range contributor.Codes() {
			owned[code] = struct{}{}
		}
		for _, item := range items {
			if item.CustomerNo != customerNo || item.Total < 0 {
				return Summary{}, gerror.New("customer 360 domain summary is inconsistent")
			}
			if _, belongs := owned[item.Code]; !belongs {
				return Summary{}, gerror.New("customer 360 contributor returned a foreign summary")
			}
			if _, duplicate := counts[item.Code]; duplicate {
				return Summary{}, gerror.New("customer 360 domain summary is duplicated")
			}
			counts[item.Code] = item.Total
		}
	}
	if len(counts) != len(requiredCodes) {
		return Summary{}, gerror.New("customer 360 summary is incomplete")
	}
	balances := []Balance{}
	if r.balances != nil {
		var err error
		balances, err = r.balances.Balances(ctx, customerNo)
		if err != nil {
			return Summary{}, gerror.Wrap(err, "load customer 360 balances")
		}
	}
	return Summary{
		Orders: counts[customer360cap.CodeOrders], Bills: counts[customer360cap.CodeBills],
		Transactions: counts[customer360cap.CodeTransactions], Servers: counts[customer360cap.CodeServers],
		Contracts: counts[customer360cap.CodeContracts], TaxInvoices: counts[customer360cap.CodeTaxInvoices],
		Tickets: counts[customer360cap.CodeTickets], Notifications: counts[customer360cap.CodeNotifications],
		Audits: counts[customer360cap.CodeAudits], Balances: balances,
	}, nil
}

func nilInterface(value any) bool {
	if value == nil {
		return true
	}
	reflected := reflect.ValueOf(value)
	switch reflected.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return reflected.IsNil()
	default:
		return false
	}
}

// CoreContributor owns counts stored by the core plugin.
type CoreContributor struct {
	database       gdb.DB
	databaseSource func() gdb.DB
}

func NewCoreContributor(database gdb.DB) (*CoreContributor, error) {
	if database == nil {
		return nil, gerror.New("core customer 360 contributor requires database")
	}
	return &CoreContributor{database: database}, nil
}

func NewLazyCoreContributor(source func() gdb.DB) (*CoreContributor, error) {
	if source == nil {
		return nil, gerror.New("core customer 360 contributor requires database source")
	}
	return &CoreContributor{databaseSource: source}, nil
}

func (*CoreContributor) Codes() []customer360cap.Code {
	return []customer360cap.Code{
		customer360cap.CodeOrders, customer360cap.CodeBills, customer360cap.CodeTransactions,
		customer360cap.CodeTickets, customer360cap.CodeAudits,
	}
}

func (c *CoreContributor) BatchCount(
	ctx context.Context,
	customerNos []string,
) ([]customer360cap.Count, error) {
	if len(customerNos) == 0 || len(customerNos) > customer360cap.MaximumBatchSize {
		return nil, gerror.New("core customer 360 batch size is invalid")
	}
	database := c.database
	if database == nil && c.databaseSource != nil {
		database = c.databaseSource()
	}
	if database == nil {
		return nil, gerror.New("core customer 360 database is unavailable")
	}
	for _, customerNo := range customerNos {
		if customerNo == "" {
			return nil, gerror.New("core customer 360 customer number is required")
		}
	}
	values := make([]string, len(customerNos))
	arguments := make([]any, len(customerNos))
	for index, customerNo := range customerNos {
		values[index] = "(?)"
		arguments[index] = customerNo
	}
	query := `WITH requested(customer_no) AS (VALUES ` + strings.Join(values, ",") + `),
customer_transactions AS (
  SELECT DISTINCT a.customer_no,e.transaction_no
  FROM plugin_idc_core_billing_ledger_entry e
  INNER JOIN plugin_idc_core_billing_ledger_account a
    ON a.tenant_id=e.tenant_id AND a.account_no=e.account_no
  WHERE e.tenant_id=0 AND a.customer_no IS NOT NULL
)
SELECT r.customer_no,'orders' AS code,COUNT(o.id) AS total FROM requested r
LEFT JOIN plugin_idc_core_commerce_order o ON o.tenant_id=0 AND o.customer_no=r.customer_no GROUP BY r.customer_no
UNION ALL SELECT r.customer_no,'bills',COUNT(i.id) FROM requested r
LEFT JOIN plugin_idc_core_billing_invoice i ON i.tenant_id=0 AND i.customer_no=r.customer_no GROUP BY r.customer_no
UNION ALL SELECT r.customer_no,'transactions',COUNT(t.transaction_no) FROM requested r
LEFT JOIN customer_transactions t ON t.customer_no=r.customer_no GROUP BY r.customer_no
UNION ALL SELECT r.customer_no,'tickets',COUNT(k.id) FROM requested r
LEFT JOIN plugin_idc_core_support_ticket k ON k.tenant_id=0 AND k.customer_no=r.customer_no GROUP BY r.customer_no
UNION ALL SELECT r.customer_no,'audits',COUNT(a.id) FROM requested r
LEFT JOIN plugin_idc_core_business_audit_event a ON a.tenant_id=0 AND a.customer_no=r.customer_no GROUP BY r.customer_no`
	rows, err := database.GetAll(ctx, query, arguments...)
	if err != nil {
		return nil, gerror.Wrap(err, "count core customer 360 summaries")
	}
	if len(rows) != len(customerNos)*len(c.Codes()) {
		return nil, gerror.New("core customer 360 summary is incomplete")
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

// Balances returns all bounded balance accounts for one customer in one query.
func (c *CoreContributor) Balances(ctx context.Context, customerNo string) ([]Balance, error) {
	if customerNo == "" {
		return nil, gerror.New("core customer 360 customer number is required")
	}
	database := c.database
	if database == nil && c.databaseSource != nil {
		database = c.databaseSource()
	}
	if database == nil {
		return nil, gerror.New("core customer 360 database is unavailable")
	}
	type balanceRow struct {
		AccountTypeCode  string    `orm:"account_type_code"`
		CurrencyCode     string    `orm:"currency_code"`
		StatusCode       string    `orm:"status_code"`
		BalanceMinor     int64     `orm:"balance_minor"`
		HeldBalanceMinor int64     `orm:"held_balance_minor"`
		UpdatedAt        time.Time `orm:"updated_at"`
	}
	rows := make([]balanceRow, 0, 65)
	err := database.GetScan(ctx, &rows, `
SELECT account.account_type_code,account.currency_code,account.status_code,
       COALESCE(snapshot.closing_balance_minor,0) AS balance_minor,
       COALESCE(hold.held_balance_minor,0) AS held_balance_minor,
       COALESCE(snapshot.created_at,account.updated_at) AS updated_at
FROM plugin_idc_core_billing_ledger_account account
LEFT JOIN LATERAL (
  SELECT closing_balance_minor,created_at
  FROM plugin_idc_core_billing_balance_snapshot
  WHERE tenant_id=0 AND account_no=account.account_no
  ORDER BY sequence_no DESC LIMIT 1
) snapshot ON TRUE
LEFT JOIN LATERAL (
  SELECT COALESCE(SUM(amount_minor-consumed_amount_minor-released_amount_minor),0) AS held_balance_minor
  FROM plugin_idc_core_billing_fund_hold
  WHERE tenant_id=0 AND account_no=account.account_no
    AND status_code IN ('active','partially_consumed') AND expires_at>CURRENT_TIMESTAMP
) hold ON TRUE
WHERE account.tenant_id=0 AND account.customer_no=?
ORDER BY account.currency_code,account.account_type_code,account.id
LIMIT 65`, customerNo)
	if err != nil {
		return nil, gerror.Wrap(err, "load core customer 360 balances")
	}
	if len(rows) > 64 {
		return nil, gerror.New("core customer 360 balance result exceeds limit")
	}
	result := make([]Balance, 0, len(rows))
	for _, row := range rows {
		if row.AccountTypeCode == "" || row.CurrencyCode == "" || row.StatusCode == "" ||
			row.BalanceMinor < 0 || row.HeldBalanceMinor < 0 || row.HeldBalanceMinor > row.BalanceMinor ||
			row.UpdatedAt.IsZero() {
			return nil, gerror.New("core customer 360 balance projection is invalid")
		}
		result = append(result, Balance{
			AccountTypeCode: row.AccountTypeCode, CurrencyCode: row.CurrencyCode,
			StatusCode: row.StatusCode, BalanceMinor: row.BalanceMinor,
			HeldBalanceMinor:      row.HeldBalanceMinor,
			AvailableBalanceMinor: row.BalanceMinor - row.HeldBalanceMinor,
			UpdatedAt:             row.UpdatedAt.UTC(),
		})
	}
	return result, nil
}
