// This file centralizes metric sources so summaries and paginated drill-downs
// cannot silently diverge. It reads only IDC-owned tables and host-governed SPI.

package operations

import (
	"context"
	"strings"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"lina-core/pkg/bizerr"
	"lina-core/pkg/plugin/capability/authcap/authz"
	"lina-core/pkg/plugin/capability/sessioncap/sessionspi"
)

// DataMetric identifies a supported metric source, never an arbitrary table.
type DataMetric string

// DataStatus identifies cross-domain dashboard filters, not persisted statuses.
type DataStatus string

const (
	// StatusToday selects customers registered on the merchant's current day.
	StatusToday DataStatus = "today"
	// StatusOnline selects customers with an active governed session.
	StatusOnline DataStatus = "online"
	// StatusPending selects unresolved and unclosed tickets.
	StatusPending DataStatus = "pending"
	// StatusProcessed selects resolved or closed tickets.
	StatusProcessed DataStatus = "processed"
)

// Metric identifiers are shared between aggregation and drill-down validation.
const (
	MetricIncome    DataMetric = "income"
	MetricOrders    DataMetric = "orders"
	MetricCustomers DataMetric = "customers"
	MetricServers   DataMetric = "servers"
	MetricTickets   DataMetric = "tickets"
	MetricProducts  DataMetric = "products"
)

// DataQuery describes one bounded drill-down with half-open time filters.
type DataQuery struct {
	Metric                          DataMetric
	StartAt, EndAt, AsOf            int64
	CurrencyCode, Status, ProductNo string
	Page, PageSize                  int
}

// metricSource contains controlled SQL and bind values; none are client SQL.
type metricSource struct {
	from, where, event, id, label, customer, amount, quantity, currency, status string
	args                                                                        []any
}

// source constructs identical row sets for aggregate and record projections.
func (s *serviceImpl) source(ctx context.Context, q DataQuery) (metricSource, error) {
	src := metricSource{where: "r.tenant_id=0", customer: "''", amount: "0", quantity: "0", currency: "''", status: "''"}
	switch q.Metric {
	case MetricIncome:
		src.from = `(SELECT tx.tenant_id,tx.transaction_no,tx.source_reference,tx.currency_code,tx.transaction_type_code,tx.posted_at,SUM(e.amount_minor) amount_minor
   FROM plugin_idc_core_billing_ledger_transaction tx
   JOIN plugin_idc_core_billing_ledger_entry e ON e.tenant_id=tx.tenant_id AND e.transaction_no=tx.transaction_no AND e.currency_code=tx.currency_code
   WHERE tx.tenant_id=0 AND tx.transaction_type_code IN ('recharge','payment','manual_receipt','credit') AND e.entry_role_code='cash' AND e.direction_code='debit'
   GROUP BY tx.tenant_id,tx.transaction_no,tx.source_reference,tx.currency_code,tx.transaction_type_code,tx.posted_at) r`
		src.id, src.label, src.event, src.amount, src.currency, src.status = "r.transaction_no", "r.source_reference", "r.posted_at", "r.amount_minor", "r.currency_code", "r.transaction_type_code"
	case MetricOrders:
		src.from = "plugin_idc_core_commerce_order r"
		src.id, src.label, src.event, src.customer, src.amount, src.currency, src.status = "r.order_no", "r.order_no", "r.paid_at", "r.customer_no", "r.total_amount_minor", "r.currency_code", "r.status_code"
		src.where += " AND r.paid_at IS NOT NULL"
	case MetricProducts:
		src.from = `plugin_idc_core_commerce_order_item r JOIN plugin_idc_core_commerce_order o ON o.tenant_id=r.tenant_id AND o.order_no=r.order_no
   JOIN plugin_idc_core_commerce_order_item_quantity_snapshot qty ON qty.tenant_id=r.tenant_id AND qty.order_item_no=r.order_item_no`
		src.id, src.label, src.event, src.customer, src.amount, src.quantity, src.currency, src.status = "r.order_no", "r.product_name_zh_cn", "o.paid_at", "o.customer_no", "r.total_amount_minor", "qty.quantity", "r.currency_code", "o.status_code"
		src.where += " AND o.paid_at IS NOT NULL"
		if q.ProductNo != "" {
			src.where += " AND r.product_no=?"
			src.args = append(src.args, q.ProductNo)
		}
	case MetricCustomers:
		src.from = "plugin_idc_core_customer_subject r"
		src.id, src.label, src.event, src.customer, src.status = "r.customer_no", "r.display_name", "r.created_at", "r.customer_no", "r.status_code"
		if DataStatus(q.Status) == StatusOnline {
			predicate, err := s.onlineCustomerPredicate(ctx)
			if err != nil {
				return src, err
			}
			src.where += " AND " + predicate
		}
	case MetricServers:
		src.from = "plugin_idc_core_service_instance r"
		src.id, src.label, src.event, src.customer, src.status = "r.service_no", "r.customer_alias", "r.entitlement_expires_at", "r.customer_no", "r.entitlement_status_code"
		src.where += " AND r.resource_owner_code='idc-resource' AND r.product_type_code IN ('cloud_server','bare_metal') AND r.entitlement_status_code='active'"
	case MetricTickets:
		src.from = "plugin_idc_core_support_ticket r"
		src.id, src.label, src.event, src.customer, src.status = "r.ticket_no", "r.subject", "r.created_at", "r.customer_no", "r.base_stage"
		if DataStatus(q.Status) == StatusPending {
			src.where += " AND r.base_stage NOT IN ('resolved','closed')"
		}
		if DataStatus(q.Status) == StatusProcessed {
			src.where += " AND r.base_stage IN ('resolved','closed')"
		}
	default:
		return src, bizerr.NewCode(CodeDataInvalid)
	}
	if q.CurrencyCode != "" && (q.Metric == MetricIncome || q.Metric == MetricOrders || q.Metric == MetricProducts) {
		src.where += " AND " + src.currency + "=?"
		src.args = append(src.args, q.CurrencyCode)
	}
	if q.StartAt > 0 {
		src.where += " AND " + src.event + ">=?"
		src.args = append(src.args, time.UnixMilli(q.StartAt))
	}
	if q.EndAt > 0 {
		src.where += " AND " + src.event + "<?"
		src.args = append(src.args, time.UnixMilli(q.EndAt))
	}
	return src, nil
}

// onlineCustomerPredicate composes the host-owned session predicate with the
// plugin's active identity binding; EXISTS naturally deduplicates customers.
func (s *serviceImpl) onlineCustomerPredicate(ctx context.Context) (string, error) {
	scope, err := sessionspi.Scope(s.sessions)
	if err != nil {
		return "", err
	}
	model, empty, err := scope.BuildOnlineUserExists(ctx, "b.sys_user_id")
	if err != nil {
		return "", err
	}
	if empty {
		return "1=0", nil
	}
	// Build the correlated subquery without executing it outside its binding alias.
	predicate, args := model.Builder().Where("EXISTS ?", model).Build()
	sql := gdb.FormatSqlWithArgs(predicate, args)
	return `EXISTS (SELECT 1 FROM plugin_idc_core_customer_identity_binding b
 JOIN plugin_idc_core_principal_audience a ON a.tenant_id=b.tenant_id AND a.sys_user_id=b.sys_user_id AND a.audience_code='customer' AND a.status_code='active'
 WHERE b.tenant_id=r.tenant_id AND b.customer_no=r.customer_no AND b.status_code='active' AND (` + sql + `))`, nil
}

// ListDataRecords applies both operating scope and the target domain permission.
func (s *serviceImpl) ListDataRecords(ctx context.Context, q DataQuery) (result DataRecords, err error) {
	defer func() { err = dataError(err) }()
	if err = s.authorizeData(ctx); err != nil {
		return result, bizerr.NewCode(CodeDataDenied)
	}
	q, err = s.normalizeDataQuery(ctx, q)
	if err != nil {
		return result, err
	}
	allowed, err := s.canReadRecords(ctx, q.Metric)
	if err != nil {
		return result, err
	}
	if !allowed {
		return result, bizerr.NewCode(CodeDataDenied)
	}
	src, err := s.source(ctx, q)
	if err != nil {
		return result, err
	}
	total, err := s.database.GetValue(ctx, "SELECT COUNT(*) FROM "+src.from+" WHERE "+src.where, src.args...)
	if err != nil {
		return result, err
	}
	result.Total = total.Int64()
	result.Items = make([]DataRecord, 0, q.PageSize)
	args := append(append([]any{}, src.args...), q.PageSize, (q.Page-1)*q.PageSize)
	order := src.event + " DESC," + src.id + " DESC"
	if q.Metric == MetricProducts {
		// Several item rows may share an order; preserve stable page boundaries.
		order += ",r.order_item_no DESC"
	}
	rows, err := s.database.GetAll(ctx, "SELECT "+strings.Join([]string{src.id + " id", src.label + " label", src.customer + " customer_no", src.amount + " amount_minor", src.quantity + " quantity", src.currency + " currency_code", src.status + " status_code", src.event + " occurred_at"}, ",")+",c.display_name customer_name FROM "+src.from+" LEFT JOIN plugin_idc_core_customer_subject c ON c.tenant_id=0 AND c.customer_no="+src.customer+" WHERE "+src.where+" ORDER BY "+order+" LIMIT ? OFFSET ?", args...)
	if err != nil {
		return result, err
	}
	for _, row := range rows {
		result.Items = append(result.Items, DataRecord{ID: row["id"].String(), Label: row["label"].String(), CustomerNo: row["customer_no"].String(), CustomerName: row["customer_name"].String(), AmountMinor: row["amount_minor"].String(), Quantity: row["quantity"].Int64(), CurrencyCode: row["currency_code"].String(), StatusCode: row["status_code"].String(), OccurredAt: row["occurred_at"].Time().UnixMilli()})
	}
	return result, nil
}

// authorizeData requires the active staff fact as well as host scope evidence;
// broad roles on a customer identity cannot grant access to operating metrics.
func (s *serviceImpl) authorizeData(ctx context.Context) error {
	if err := s.authorizeRead(ctx); err != nil {
		return bizerr.NewCode(CodeDataDenied)
	}
	if s.database == nil {
		return bizerr.NewCode(CodeDataUnavailable)
	}
	staff, err := s.database.GetValue(ctx, `SELECT COUNT(*) FROM plugin_idc_core_principal_audience a
 WHERE a.tenant_id=0 AND a.sys_user_id=? AND a.audience_code='staff' AND a.status_code='active'
 AND NOT EXISTS (SELECT 1 FROM plugin_idc_core_customer_identity_binding b
 WHERE b.tenant_id=a.tenant_id AND b.sys_user_id=a.sys_user_id AND b.status_code='active')`, s.bizCtx.Current(ctx).UserID)
	if err != nil {
		return err
	}
	if staff.Int() != 1 {
		return bizerr.NewCode(CodeDataDenied)
	}
	return nil
}

// normalizeDataQuery resolves merchant-calendar defaults once per request and
// pins all past-event metrics to the same snapshot used by the summary.
func (s *serviceImpl) normalizeDataQuery(ctx context.Context, q DataQuery) (DataQuery, error) {
	location := time.UTC
	if (q.Metric == MetricCustomers && DataStatus(q.Status) == StatusToday) || q.Metric == MetricServers {
		merchant, err := s.database.GetOne(ctx, "SELECT timezone FROM plugin_idc_core_merchant_profile WHERE tenant_id=0 AND enabled=true LIMIT 1")
		if err != nil {
			return q, err
		}
		if len(merchant) == 0 {
			return q, bizerr.NewCode(CodeDataUnavailable)
		}
		location, err = time.LoadLocation(merchant["timezone"].String())
		if err != nil {
			return q, err
		}
	}
	return normalizeDataWindow(q, time.Now(), location)
}

// normalizeDataWindow validates metric-specific filters and clips full-day
// chart intervals at asOf. Future chart days remain valid empty intervals.
func normalizeDataWindow(q DataQuery, now time.Time, location *time.Location) (DataQuery, error) {
	if q.Page < 1 || q.Page > 1000000 || q.PageSize < 1 || q.PageSize > 100 || q.StartAt < 0 || q.EndAt < 0 || q.AsOf < 0 || (q.EndAt > 0 && q.StartAt >= q.EndAt) {
		return q, bizerr.NewCode(CodeDataInvalid)
	}
	allowedStatus := q.Status == ""
	switch q.Metric {
	case MetricIncome, MetricOrders, MetricProducts:
		q.CurrencyCode = normalizeCurrencyCode(q.CurrencyCode)
		if !validCurrencyCode(q.CurrencyCode) {
			return q, bizerr.NewCode(CodeDataInvalid)
		}
	case MetricCustomers:
		allowedStatus = allowedStatus || DataStatus(q.Status) == StatusToday || DataStatus(q.Status) == StatusOnline
	case MetricServers:
	case MetricTickets:
		allowedStatus = allowedStatus || DataStatus(q.Status) == StatusPending || DataStatus(q.Status) == StatusProcessed
	default:
		return q, bizerr.NewCode(CodeDataInvalid)
	}
	if !allowedStatus || (q.ProductNo != "" && q.Metric != MetricProducts) || len(q.ProductNo) > 64 {
		return q, bizerr.NewCode(CodeDataInvalid)
	}
	if q.AsOf > now.UnixMilli() {
		return q, bizerr.NewCode(CodeDataInvalid)
	}
	if q.AsOf == 0 {
		q.AsOf = now.UnixMilli()
	}
	asOf := time.UnixMilli(q.AsOf).In(location)
	limit := q.AsOf
	if q.Metric == MetricServers {
		limit = asOf.AddDate(0, 0, 7).UnixMilli()
		q.StartAt = max(q.StartAt, q.AsOf)
	}
	if q.Metric == MetricCustomers && DataStatus(q.Status) == StatusToday && q.StartAt == 0 {
		q.StartAt = time.Date(asOf.Year(), asOf.Month(), asOf.Day(), 0, 0, 0, 0, location).UnixMilli()
	}
	if q.EndAt == 0 || q.EndAt > limit {
		q.EndAt = limit
	}
	// A future calendar point must stay empty instead of reading future facts.
	q.StartAt = min(q.StartAt, q.EndAt)
	return q, nil
}

// canReadRecords never treats aggregate access as permission to business rows.
func (s *serviceImpl) canReadRecords(ctx context.Context, metric DataMetric) (bool, error) {
	permissions := map[DataMetric]string{MetricIncome: "idc:admin:billing:transaction:list", MetricOrders: "idc:admin:commerce:order:list", MetricProducts: "idc:admin:commerce:order:list", MetricCustomers: "idc:admin:customer:list", MetricServers: "idc:admin:resource:server-batch:view", MetricTickets: "idc:admin:support:ticket:view-all"}
	permission, ok := permissions[metric]
	if !ok {
		return false, nil
	}
	if s.authorization == nil {
		return false, bizerr.NewCode(CodeDataUnavailable)
	}
	return s.authorization.HasPermission(ctx, authz.PermissionKey(permission))
}
