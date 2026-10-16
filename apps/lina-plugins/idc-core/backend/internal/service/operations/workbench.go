// Package operations provides the bounded read model used by the IDC
// operations workbench. It deliberately reads projections in one SQL round
// trip and never loads business rows one by one.
package operations

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"
	"lina-core/pkg/plugin/capability/authcap/authz"
	"lina-core/pkg/plugin/capability/bizctxcap"
	"lina-core/pkg/plugin/capability/sessioncap"
)

type QueueCount struct {
	Code  string `json:"code"`
	Count int64  `json:"count"`
}

type CurrencyMetric struct {
	CurrencyCode        string `json:"currencyCode"`
	InvoiceCount        int64  `json:"invoiceCount"`
	TotalAmountMinor    int64  `json:"totalAmountMinor"`
	PaidAmountMinor     int64  `json:"paidAmountMinor"`
	RefundedAmountMinor int64  `json:"refundedAmountMinor"`
	RevenueMinor        int64  `json:"revenueMinor"`
	ReceivableMinor     int64  `json:"receivableMinor"`
	CashFlowMinor       int64  `json:"cashFlowMinor"`
	BalanceMinor        int64  `json:"balanceMinor"`
	CostMinor           int64  `json:"costMinor"`
	CostKnown           bool   `json:"costKnown"`
	GrossProfitMinor    int64  `json:"grossProfitMinor"`
	CustomerCount       int64  `json:"customerCount"`
	OrderCount          int64  `json:"orderCount"`
	ProductCount        int64  `json:"productCount"`
	SalesCount          int64  `json:"salesCount"`
	AffiliateCount      int64  `json:"affiliateCount"`
	SupplierSuccess     int64  `json:"supplierSuccess"`
	SupplierFailure     int64  `json:"supplierFailure"`
}

type Snapshot struct {
	GeneratedAt time.Time
	Watermark   time.Time
	Queues      []QueueCount
	Currencies  []CurrencyMetric
}

// RefreshRequest lets callers send the last watermark they received. The
// query still returns the complete bounded projection, while Changed tells
// the caller whether any source row changed since that point.
type RefreshRequest struct{ Since time.Time }

type RefreshResult struct {
	Snapshot Snapshot
	Changed  bool
}

type ReportQuery struct {
	From         time.Time
	To           time.Time
	CurrencyCode string
}

type Report struct {
	From, To   time.Time
	Currencies []CurrencyMetric
}

type TaskQuery struct {
	Status string
	Limit  int
}

type TaskItem struct {
	TaskNo, TaskType, SubjectType, SubjectNo, Status, ErrorCode, ErrorSummary string
	AttemptCount                                                              int
	CreatedAt, UpdatedAt                                                      time.Time
}

type TaskAlert struct {
	Code, Severity string
	Count          int64
	OldestAt       time.Time
}

// Signal is a redacted health observation published by a domain owner. It
// carries no provider secrets or raw payloads and is bounded to one code per
// kind. Other plugins must use this contract instead of reading core tables.
type Signal struct {
	Kind           string
	Code           string
	Status         string
	Count          int64
	AgeSeconds     int64
	ValueMinor     int64
	ThresholdMinor int64
	ObservedAt     time.Time
	SourceRevision string
	Details        map[string]any
}

type OperationalAlert struct {
	Kind, Code, Severity string
	Count, AgeSeconds    int64
	ObservedAt           time.Time
}

type Service interface {
	// LoadData returns currency-isolated operating metrics under broad staff scope.
	LoadData(ctx context.Context, currencyCode string) (Data, error)
	// ListDataRecords returns at most 100 records using the metric predicates and target-domain permission.
	ListDataRecords(ctx context.Context, query DataQuery) (DataRecords, error)
	LoadSnapshot(ctx context.Context) (Snapshot, error)
	Refresh(ctx context.Context, request RefreshRequest) (RefreshResult, error)
	// RefreshInternal runs the scheduled, platform-owned refresh without a user
	// request. It is intentionally separate from Refresh so an HTTP caller can
	// never obtain the trusted background path by omitting identity context.
	RefreshInternal(ctx context.Context) (RefreshResult, error)
	LoadReport(ctx context.Context, query ReportQuery) (Report, error)
	ListTasks(ctx context.Context, query TaskQuery) ([]TaskItem, error)
	ListTaskAlerts(ctx context.Context) ([]TaskAlert, error)
	UpsertSignal(ctx context.Context, signal Signal) error
	ListOperationalAlerts(ctx context.Context) ([]OperationalAlert, error)
}

type serviceImpl struct {
	database      gdb.DB
	bizCtx        bizctxcap.Service
	sessions      sessioncap.Service
	authorization authz.Service
}

func New(database gdb.DB, bizCtx bizctxcap.Service, sessions sessioncap.Service, authorization authz.Service) (Service, error) {
	if database == nil {
		return nil, gerror.New("operations database is required")
	}
	if bizCtx == nil {
		return nil, gerror.New("operations business context is required")
	}
	if sessions == nil {
		return nil, gerror.New("operations sessions capability is required")
	}
	if authorization == nil {
		return nil, gerror.New("operations authorization is required")
	}
	return &serviceImpl{database: database, bizCtx: bizCtx, sessions: sessions, authorization: authorization}, nil
}

func (s *serviceImpl) LoadSnapshot(ctx context.Context) (Snapshot, error) {
	result, err := s.Refresh(ctx, RefreshRequest{})
	return result.Snapshot, err
}

func (s *serviceImpl) Refresh(ctx context.Context, request RefreshRequest) (RefreshResult, error) {
	if err := s.authorizeRead(ctx); err != nil {
		return RefreshResult{}, err
	}
	return s.refresh(ctx, request)
}

func (s *serviceImpl) RefreshInternal(ctx context.Context) (RefreshResult, error) {
	return s.refresh(ctx, RefreshRequest{})
}

func (s *serviceImpl) refresh(ctx context.Context, request RefreshRequest) (RefreshResult, error) {
	if ctx == nil {
		return RefreshResult{}, gerror.New("operations context is required")
	}
	if err := ctx.Err(); err != nil {
		return RefreshResult{}, err
	}
	rows, err := s.database.GetAll(ctx, `
SELECT 'pending_orders' AS code, COUNT(*) AS total, NULL::text AS currency_code, 0::bigint AS invoice_count, 0::bigint AS total_amount_minor, 0::bigint AS paid_amount_minor, 0::bigint AS refunded_amount_minor, NULL::timestamptz AS watermark
  FROM plugin_idc_core_commerce_order WHERE tenant_id=0 AND status_code IN ('pending_payment','payment_review')
UNION ALL SELECT 'open_tasks', COUNT(*), NULL, 0, 0, 0, 0, NULL FROM plugin_idc_core_async_task WHERE tenant_id=0 AND status_code IN ('pending','running','retry_wait')
UNION ALL SELECT 'open_risks', COUNT(*), NULL, 0, 0, 0, 0, NULL FROM plugin_idc_core_customer_risk_record WHERE tenant_id=0 AND status_code IN ('open','acknowledged')
UNION ALL SELECT 'pending_invoices', COUNT(*), NULL, 0, 0, 0, 0, NULL FROM plugin_idc_core_billing_invoice WHERE tenant_id=0 AND status_code IN ('pending_payment','partially_paid','overdue')
UNION ALL SELECT 'pending_services', COUNT(*), NULL, 0, 0, 0, 0, NULL FROM plugin_idc_core_service_instance WHERE tenant_id=0 AND runtime_status_code IN ('pending_provision','provisioning')
UNION ALL SELECT 'paused_services', COUNT(*), NULL, 0, 0, 0, 0, NULL FROM plugin_idc_core_service_instance WHERE tenant_id=0 AND runtime_status_code IN ('suspended','unknown')
UNION ALL SELECT 'paused_products', COUNT(*), NULL, 0, 0, 0, 0, NULL FROM plugin_idc_core_catalog_product WHERE tenant_id=0 AND status_code='suspended'
UNION ALL SELECT 'expiring_services', COUNT(*), NULL, 0, 0, 0, 0, NULL FROM plugin_idc_core_service_instance WHERE tenant_id=0 AND entitlement_expires_at IS NOT NULL AND entitlement_expires_at > CURRENT_TIMESTAMP AND entitlement_expires_at <= CURRENT_TIMESTAMP + INTERVAL '7 days'
UNION ALL SELECT 'open_tickets', COUNT(*), NULL, 0, 0, 0, 0, NULL FROM plugin_idc_core_support_ticket WHERE tenant_id=0 AND closed_at IS NULL
UNION ALL SELECT 'overdue_tickets', COUNT(*), NULL, 0, 0, 0, 0, NULL FROM plugin_idc_core_support_ticket t WHERE t.tenant_id=0 AND t.closed_at IS NULL AND (t.sla_risk_code='breached' OR EXISTS (SELECT 1 FROM plugin_idc_core_support_ticket_sla_round r WHERE r.tenant_id=0 AND r.ticket_no=t.ticket_no AND r.round_no=t.current_round AND r.stopped_at IS NULL AND r.resolution_due_at < CURRENT_TIMESTAMP))
UNION ALL SELECT 'pending_refunds', COUNT(*), NULL, 0, 0, 0, 0, NULL FROM plugin_idc_core_refund_request WHERE tenant_id=0 AND status_code IN ('pending_review','approved','awaiting_balance_consent','executing','result_unknown','manual_confirmation')
UNION ALL SELECT 'payment_exceptions', COUNT(*), NULL, 0, 0, 0, 0, NULL FROM plugin_idc_core_commerce_order WHERE tenant_id=0 AND status_code IN ('payment_failed','payment_review','payment_unknown')
UNION ALL SELECT 'pending_ip_changes', COUNT(*), NULL, 0, 0, 0, 0, NULL FROM plugin_idc_core_server_ip_change_intent WHERE tenant_id=0 AND reservation_status_code IN ('reserved','unknown')
UNION ALL SELECT 'pending_identity', COUNT(*), NULL, 0, 0, 0, 0, NULL FROM plugin_idc_core_customer_individual_profile WHERE tenant_id=0 AND identity_status_code IN ('pending','unverified')
UNION ALL SELECT 'pending_enterprise_identity', COUNT(*), NULL, 0, 0, 0, 0, NULL FROM plugin_idc_core_customer_enterprise_profile WHERE tenant_id=0 AND verification_status_code IN ('pending','unverified')
UNION ALL SELECT 'contract_exceptions', COUNT(*), NULL, 0, 0, 0, 0, NULL FROM plugin_idc_core_contract_failure_compensation WHERE tenant_id=0 AND status_code IN ('refund_pending','manual_review')
UNION ALL SELECT 'pending_contracts', COUNT(*), NULL, 0, 0, 0, 0, NULL FROM plugin_idc_core_async_task WHERE tenant_id=0 AND task_type ILIKE '%contract%' AND status_code IN ('pending','running','retry_wait','result_unknown','manual_takeover')
UNION ALL SELECT 'pending_commission_settlements', COUNT(*), NULL, 0, 0, 0, 0, NULL FROM plugin_idc_core_sales_commission_settlement WHERE tenant_id=0 AND status_code IN ('pending_review','approved','batched','manual_review')
UNION ALL SELECT 'notification_failures', COUNT(*), NULL, 0, 0, 0, 0, NULL FROM plugin_idc_core_async_task WHERE tenant_id=0 AND task_type ILIKE '%notif%' AND status_code IN ('failed','dead_letter','result_unknown')
UNION ALL SELECT 'resource_alerts', COUNT(*), NULL, 0, 0, 0, 0, NULL FROM plugin_idc_core_operations_signal WHERE tenant_id=0 AND signal_kind IN ('inventory_freshness','adapter_health') AND status_code IN ('warning','critical','unknown')
UNION ALL SELECT 'open_operations', COUNT(*), NULL, 0, 0, 0, 0, NULL FROM plugin_idc_core_business_operation WHERE tenant_id=0 AND status_code IN ('pending','queued','executing','result_unknown','manual_review')
UNION ALL SELECT 'currency', 0, currency_code, COUNT(*), COALESCE(SUM(total_amount_minor),0), COALESCE(SUM(paid_amount_minor),0), COALESCE(SUM(refunded_amount_minor),0), NULL
  FROM plugin_idc_core_billing_invoice WHERE tenant_id=0 GROUP BY currency_code
UNION ALL SELECT 'watermark', 0, NULL, 0, 0, 0, 0,
  GREATEST(COALESCE((SELECT MAX(updated_at) FROM plugin_idc_core_commerce_order WHERE tenant_id=0), TIMESTAMPTZ 'epoch'),
           COALESCE((SELECT MAX(updated_at) FROM plugin_idc_core_async_task WHERE tenant_id=0), TIMESTAMPTZ 'epoch'),
           COALESCE((SELECT MAX(updated_at) FROM plugin_idc_core_customer_risk_record WHERE tenant_id=0), TIMESTAMPTZ 'epoch'),
           COALESCE((SELECT MAX(updated_at) FROM plugin_idc_core_billing_invoice WHERE tenant_id=0), TIMESTAMPTZ 'epoch'),
           COALESCE((SELECT MAX(updated_at) FROM plugin_idc_core_service_instance WHERE tenant_id=0), TIMESTAMPTZ 'epoch'),
           COALESCE((SELECT MAX(updated_at) FROM plugin_idc_core_support_ticket WHERE tenant_id=0), TIMESTAMPTZ 'epoch'),
           COALESCE((SELECT MAX(updated_at) FROM plugin_idc_core_refund_request WHERE tenant_id=0), TIMESTAMPTZ 'epoch'),
           COALESCE((SELECT MAX(updated_at) FROM plugin_idc_core_server_ip_change_intent WHERE tenant_id=0), TIMESTAMPTZ 'epoch'),
           COALESCE((SELECT MAX(updated_at) FROM plugin_idc_core_customer_individual_profile WHERE tenant_id=0), TIMESTAMPTZ 'epoch'),
           COALESCE((SELECT MAX(updated_at) FROM plugin_idc_core_customer_enterprise_profile WHERE tenant_id=0), TIMESTAMPTZ 'epoch'),
           COALESCE((SELECT MAX(updated_at) FROM plugin_idc_core_contract_failure_compensation WHERE tenant_id=0), TIMESTAMPTZ 'epoch'),
           COALESCE((SELECT MAX(updated_at) FROM plugin_idc_core_operations_signal WHERE tenant_id=0), TIMESTAMPTZ 'epoch'),
           COALESCE((SELECT MAX(updated_at) FROM plugin_idc_core_business_operation WHERE tenant_id=0), TIMESTAMPTZ 'epoch'),
           COALESCE((SELECT MAX(updated_at) FROM plugin_idc_core_catalog_product WHERE tenant_id=0), TIMESTAMPTZ 'epoch'),
           COALESCE((SELECT GREATEST(MAX(requested_at), COALESCE(MAX(completed_at), TIMESTAMPTZ 'epoch')) FROM plugin_idc_core_sales_commission_settlement WHERE tenant_id=0), TIMESTAMPTZ 'epoch'))
ORDER BY code, currency_code`,
	)
	if err != nil {
		return RefreshResult{}, gerror.Wrap(err, "load operations workbench snapshot")
	}
	queues := make([]QueueCount, 0, 4)
	currencies := make([]CurrencyMetric, 0, 4)
	watermark := time.Unix(0, 0).UTC()
	for _, row := range rows {
		code := row["code"].String()
		if code == "watermark" {
			watermark = row["watermark"].Time().UTC()
			continue
		}
		if code == "currency" {
			currencies = append(currencies, CurrencyMetric{CurrencyCode: row["currency_code"].String(), InvoiceCount: row["invoice_count"].Int64(), TotalAmountMinor: row["total_amount_minor"].Int64(), PaidAmountMinor: row["paid_amount_minor"].Int64(), RefundedAmountMinor: row["refunded_amount_minor"].Int64()})
			continue
		}
		queues = append(queues, QueueCount{Code: code, Count: row["total"].Int64()})
	}
	generatedAt := time.Now().UTC()
	snapshot := Snapshot{GeneratedAt: generatedAt, Watermark: watermark, Queues: queues, Currencies: currencies}
	if err := s.persistSnapshot(ctx, snapshot); err != nil {
		return RefreshResult{}, err
	}
	return RefreshResult{Snapshot: snapshot, Changed: request.Since.IsZero() || watermark.After(request.Since)}, nil
}

func (s *serviceImpl) persistSnapshot(ctx context.Context, snapshot Snapshot) error {
	if s == nil || s.database == nil {
		return gerror.New("operations database is required")
	}
	queues, err := json.Marshal(snapshot.Queues)
	if err != nil {
		return gerror.Wrap(err, "encode operations queues")
	}
	currencies, err := json.Marshal(snapshot.Currencies)
	if err != nil {
		return gerror.Wrap(err, "encode operations currencies")
	}
	_, err = s.database.Exec(ctx, `
INSERT INTO plugin_idc_core_operations_read_model
    (tenant_id, model_key, generated_at, watermark, queues, currencies, revision)
VALUES (0, 'default', ?, ?, CAST(? AS JSONB), CAST(? AS JSONB), 1)
ON CONFLICT (tenant_id, model_key) DO UPDATE SET
    generated_at = EXCLUDED.generated_at,
    watermark = EXCLUDED.watermark,
    queues = EXCLUDED.queues,
    currencies = EXCLUDED.currencies,
    revision = plugin_idc_core_operations_read_model.revision + 1
WHERE plugin_idc_core_operations_read_model.watermark IS DISTINCT FROM EXCLUDED.watermark
   OR plugin_idc_core_operations_read_model.queues IS DISTINCT FROM EXCLUDED.queues
   OR plugin_idc_core_operations_read_model.currencies IS DISTINCT FROM EXCLUDED.currencies`, snapshot.GeneratedAt, snapshot.Watermark, string(queues), string(currencies))
	if err != nil {
		return gerror.Wrap(err, "persist operations read model")
	}
	return nil
}

func (s *serviceImpl) LoadReport(ctx context.Context, query ReportQuery) (Report, error) {
	if err := s.authorizeRead(ctx); err != nil {
		return Report{}, err
	}
	if ctx == nil {
		return Report{}, gerror.New("operations context is required")
	}
	if query.To.IsZero() {
		query.To = time.Now().UTC()
	}
	if query.From.IsZero() {
		query.From = query.To.Add(-30 * 24 * time.Hour)
	}
	if query.From.After(query.To) || query.To.Sub(query.From) > 366*24*time.Hour {
		return Report{}, gerror.New("operations report range is invalid")
	}
	if query.CurrencyCode != "" {
		query.CurrencyCode = strings.ToUpper(strings.TrimSpace(query.CurrencyCode))
		if len(query.CurrencyCode) != 3 {
			return Report{}, gerror.New("operations report currency is invalid")
		}
	}
	rows, err := s.database.GetAll(ctx, `
WITH invoice_metrics AS (
    SELECT currency_code, COUNT(*) AS invoice_count,
           COALESCE(SUM(total_amount_minor),0) AS total_amount_minor,
           COALESCE(SUM(total_amount_minor - paid_amount_minor),0) AS receivable_minor,
           COALESCE(SUM(paid_amount_minor),0) AS paid_amount_minor,
           COALESCE(SUM(refunded_amount_minor),0) AS refunded_amount_minor,
           COUNT(DISTINCT customer_no) AS customer_count,
           COUNT(DISTINCT order_no) FILTER (WHERE order_no IS NOT NULL) AS order_count
      FROM plugin_idc_core_billing_invoice
     WHERE tenant_id=0 AND created_at >= ? AND created_at < ?
       AND (? = '' OR currency_code = ?)
     GROUP BY currency_code
), cash_metrics AS (
    SELECT tx.currency_code,
           COALESCE(SUM(CASE WHEN e.direction_code='debit' THEN e.amount_minor ELSE -e.amount_minor END),0) AS cash_flow_minor
      FROM plugin_idc_core_billing_ledger_transaction tx
      JOIN plugin_idc_core_billing_ledger_entry e ON e.transaction_no=tx.transaction_no AND e.currency_code=tx.currency_code AND e.tenant_id=0
     WHERE tx.tenant_id=0 AND e.entry_role_code='cash' AND tx.posted_at >= ? AND tx.posted_at < ?
     GROUP BY tx.currency_code
), balance_metrics AS (
    SELECT currency_code, COALESCE(SUM(CASE WHEN normal_balance_code='debit' THEN closing_balance_minor ELSE -closing_balance_minor END),0) AS balance_minor
      FROM (SELECT DISTINCT ON (account_no) account_no, currency_code, normal_balance_code, closing_balance_minor
              FROM plugin_idc_core_billing_balance_snapshot
             WHERE tenant_id=0 ORDER BY account_no, sequence_no DESC, id DESC) latest
     GROUP BY currency_code
), cost_metrics AS (
    SELECT oi.currency_code,
           COALESCE(SUM(CASE WHEN c.status_code IN ('frozen','corrected') THEN c.total_cost_minor ELSE 0 END),0) AS cost_minor,
           BOOL_AND(c.status_code IN ('frozen','corrected')) AS cost_known
      FROM plugin_idc_core_commerce_order_cost_snapshot c
      JOIN plugin_idc_core_commerce_order_item oi ON oi.order_item_no=c.order_item_no AND oi.tenant_id=0
     WHERE c.tenant_id=0 AND c.created_at >= ? AND c.created_at < ?
     GROUP BY oi.currency_code
), product_metrics AS (
    SELECT i.currency_code, COUNT(DISTINCT oi.product_no) AS product_count
      FROM plugin_idc_core_billing_invoice_item i
      JOIN plugin_idc_core_commerce_order_item oi ON oi.order_item_no=i.order_item_no AND oi.tenant_id=0
     WHERE i.tenant_id=0 AND i.created_at >= ? AND i.created_at < ?
     GROUP BY i.currency_code
), commission_metrics AS (
    SELECT currency_code,
           COUNT(DISTINCT sales_assignment_no) FILTER (WHERE sales_assignment_no IS NOT NULL) AS sales_count,
           COUNT(DISTINCT affiliate_attribution_no) FILTER (WHERE affiliate_attribution_no IS NOT NULL) AS affiliate_count
      FROM plugin_idc_core_commission_accrual
     WHERE tenant_id=0 AND created_at >= ? AND created_at < ?
     GROUP BY currency_code
), supplier_metrics AS (
    SELECT i.currency_code,
           COUNT(*) FILTER (WHERE a.result_status_code='succeeded') AS supplier_success,
           COUNT(*) FILTER (WHERE a.result_status_code='failed') AS supplier_failure
      FROM plugin_idc_core_product_fulfillment_attempt a
      JOIN plugin_idc_core_product_fulfillment_task t ON t.task_no=a.task_no
      JOIN plugin_idc_core_product_fulfillment_intent i ON i.intent_no=t.intent_no AND i.tenant_id=0
     WHERE a.tenant_id=0 AND a.finished_at >= ? AND a.finished_at < ?
     GROUP BY i.currency_code
)
SELECT i.currency_code, i.invoice_count, i.total_amount_minor, i.paid_amount_minor,
       i.refunded_amount_minor, i.receivable_minor,
       COALESCE(c.cash_flow_minor,0) AS cash_flow_minor,
       COALESCE(b.balance_minor,0) AS balance_minor,
       COALESCE(k.cost_minor,0) AS cost_minor, COALESCE(k.cost_known,FALSE) AS cost_known,
       i.customer_count, i.order_count, COALESCE(p.product_count,0) AS product_count,
       COALESCE(m.sales_count,0) AS sales_count, COALESCE(m.affiliate_count,0) AS affiliate_count,
       COALESCE(u.supplier_success,0) AS supplier_success, COALESCE(u.supplier_failure,0) AS supplier_failure
  FROM invoice_metrics i
  LEFT JOIN cash_metrics c USING (currency_code)
  LEFT JOIN balance_metrics b USING (currency_code)
  LEFT JOIN cost_metrics k USING (currency_code)
  LEFT JOIN product_metrics p USING (currency_code)
  LEFT JOIN commission_metrics m USING (currency_code)
  LEFT JOIN supplier_metrics u USING (currency_code)
 ORDER BY i.currency_code`, query.From, query.To, query.CurrencyCode, query.CurrencyCode,
		query.From, query.To, query.From, query.To, query.From, query.To, query.From, query.To, query.From, query.To)
	if err != nil {
		return Report{}, gerror.Wrap(err, "load operations report")
	}
	result := Report{From: query.From.UTC(), To: query.To.UTC(), Currencies: make([]CurrencyMetric, 0, len(rows))}
	for _, row := range rows {
		metric := CurrencyMetric{CurrencyCode: row["currency_code"].String(), InvoiceCount: row["invoice_count"].Int64(), TotalAmountMinor: row["total_amount_minor"].Int64(), PaidAmountMinor: row["paid_amount_minor"].Int64(), RefundedAmountMinor: row["refunded_amount_minor"].Int64(), RevenueMinor: row["total_amount_minor"].Int64(), ReceivableMinor: row["receivable_minor"].Int64(), CashFlowMinor: row["cash_flow_minor"].Int64(), BalanceMinor: row["balance_minor"].Int64(), CostMinor: row["cost_minor"].Int64(), CostKnown: row["cost_known"].Bool(), CustomerCount: row["customer_count"].Int64(), OrderCount: row["order_count"].Int64(), ProductCount: row["product_count"].Int64(), SalesCount: row["sales_count"].Int64(), AffiliateCount: row["affiliate_count"].Int64(), SupplierSuccess: row["supplier_success"].Int64(), SupplierFailure: row["supplier_failure"].Int64()}
		if metric.CostKnown {
			metric.GrossProfitMinor = metric.PaidAmountMinor - metric.RefundedAmountMinor - metric.CostMinor
		}
		result.Currencies = append(result.Currencies, metric)
	}
	return result, nil
}

func (s *serviceImpl) ListTasks(ctx context.Context, query TaskQuery) ([]TaskItem, error) {
	if err := s.authorizeRead(ctx); err != nil {
		return nil, err
	}
	if ctx == nil {
		return nil, gerror.New("operations context is required")
	}
	if query.Limit == 0 {
		query.Limit = 50
	}
	if query.Limit < 1 || query.Limit > 200 {
		return nil, gerror.New("operations task limit is invalid")
	}
	allowed := map[string]bool{"pending": true, "running": true, "retry_wait": true, "succeeded": true, "failed": true, "dead_letter": true, "result_unknown": true, "manual_takeover": true}
	if query.Status != "" && !allowed[query.Status] {
		return nil, gerror.New("operations task status is invalid")
	}
	rows, err := s.database.GetAll(ctx, `
SELECT task_no,task_type,subject_type,subject_no,status_code,attempt_count,last_error_code,last_error_summary,created_at,updated_at
FROM plugin_idc_core_async_task
WHERE tenant_id=0 AND (? = '' OR status_code = ?)
ORDER BY updated_at DESC,id DESC LIMIT ?`, query.Status, query.Status, query.Limit)
	if err != nil {
		return nil, gerror.Wrap(err, "list operations tasks")
	}
	result := make([]TaskItem, 0, len(rows))
	for _, row := range rows {
		result = append(result, TaskItem{TaskNo: row["task_no"].String(), TaskType: row["task_type"].String(), SubjectType: row["subject_type"].String(), SubjectNo: row["subject_no"].String(), Status: row["status_code"].String(), AttemptCount: row["attempt_count"].Int(), ErrorCode: row["last_error_code"].String(), ErrorSummary: row["last_error_summary"].String(), CreatedAt: row["created_at"].Time().UTC(), UpdatedAt: row["updated_at"].Time().UTC()})
	}
	return result, nil
}

func (s *serviceImpl) ListTaskAlerts(ctx context.Context) ([]TaskAlert, error) {
	if err := s.authorizeRead(ctx); err != nil {
		return nil, err
	}
	if ctx == nil {
		return nil, gerror.New("operations context is required")
	}
	if s == nil || s.database == nil {
		return nil, gerror.New("operations database is required")
	}
	rows, err := s.database.GetAll(ctx, `
SELECT code, severity, total, oldest_at FROM (
SELECT CASE
         WHEN status_code IN ('failed','dead_letter') THEN 'task_failed'
         WHEN status_code='result_unknown' THEN 'task_result_unknown'
         ELSE 'task_stuck_running'
       END AS code,
       CASE
         WHEN status_code='result_unknown' THEN 'critical'
         WHEN status_code IN ('failed','dead_letter') THEN 'high'
         ELSE 'medium'
       END AS severity,
       COUNT(*) AS total, MIN(updated_at) AS oldest_at
FROM plugin_idc_core_async_task
WHERE tenant_id=0 AND (status_code IN ('failed','dead_letter','result_unknown') OR (status_code='running' AND updated_at < CURRENT_TIMESTAMP - INTERVAL '15 minutes'))
GROUP BY 1,2
UNION ALL
SELECT signal_kind || ':' || signal_code AS code,
       CASE status_code WHEN 'critical' THEN 'critical' WHEN 'unknown' THEN 'critical' ELSE 'high' END AS severity,
       count_value AS total, observed_at AS oldest_at
FROM plugin_idc_core_operations_signal
WHERE tenant_id=0 AND status_code IN ('warning','critical','unknown')
) alerts
ORDER BY CASE severity WHEN 'critical' THEN 1 WHEN 'high' THEN 2 ELSE 3 END, code`,
	)
	if err != nil {
		return nil, gerror.Wrap(err, "list operations task alerts")
	}
	result := make([]TaskAlert, 0, len(rows))
	for _, row := range rows {
		result = append(result, TaskAlert{Code: row["code"].String(), Severity: row["severity"].String(), Count: row["total"].Int64(), OldestAt: row["oldest_at"].Time().UTC()})
	}
	return result, nil
}

// authorizeRead fails closed for staff scopes that the current bounded
// projection cannot represent. Platform and tenant scopes are equivalent in
// this single-tenant plugin; department/self scopes must use domain lists.
func (s *serviceImpl) authorizeRead(ctx context.Context) error {
	if ctx == nil {
		return gerror.New("operations context is required")
	}
	if s == nil || s.bizCtx == nil {
		return gerror.New("operations business context is required")
	}
	current := s.bizCtx.Current(ctx)
	if current.UserID <= 0 || current.TenantID != 0 || current.ActingUserID != 0 || current.IsImpersonation || current.ActingAsTenant || current.DataScopeUnsupported {
		return gerror.New("operations data scope is unavailable")
	}
	if current.DataScope != 1 && current.DataScope != 2 {
		return gerror.New("operations data scope is not supported")
	}
	return nil
}

func (s *serviceImpl) UpsertSignal(ctx context.Context, signal Signal) error {
	if ctx == nil {
		return gerror.New("operations context is required")
	}
	if s == nil || s.database == nil {
		return gerror.New("operations database is required")
	}
	if err := validateSignal(signal); err != nil {
		return err
	}
	if signal.ObservedAt.IsZero() {
		signal.ObservedAt = time.Now().UTC()
	}
	details := signal.Details
	if details == nil {
		details = map[string]any{}
	}
	encoded, err := json.Marshal(details)
	if err != nil {
		return gerror.Wrap(err, "encode operations signal details")
	}
	_, err = s.database.Exec(ctx, `
INSERT INTO plugin_idc_core_operations_signal
    (tenant_id, signal_kind, signal_code, status_code, count_value, age_seconds, value_minor, threshold_minor, observed_at, source_revision, details, updated_at)
VALUES (0, ?, ?, ?, ?, ?, ?, ?, ?, ?, CAST(? AS JSONB), CURRENT_TIMESTAMP)
ON CONFLICT (tenant_id, signal_kind, signal_code) DO UPDATE SET
    status_code = EXCLUDED.status_code,
    count_value = EXCLUDED.count_value,
    age_seconds = EXCLUDED.age_seconds,
    value_minor = EXCLUDED.value_minor,
    threshold_minor = EXCLUDED.threshold_minor,
    observed_at = EXCLUDED.observed_at,
    source_revision = EXCLUDED.source_revision,
    details = EXCLUDED.details,
    updated_at = CURRENT_TIMESTAMP`, signal.Kind, signal.Code, signal.Status, signal.Count, signal.AgeSeconds, signal.ValueMinor, signal.ThresholdMinor, signal.ObservedAt, signal.SourceRevision, string(encoded))
	if err != nil {
		return gerror.Wrap(err, "persist operations signal")
	}
	return nil
}

func validateSignal(signal Signal) error {
	if signal.Kind != "queue_backlog" && signal.Kind != "adapter_health" && signal.Kind != "inventory_freshness" && signal.Kind != "performance" {
		return gerror.New("operations signal kind is invalid")
	}
	if strings.TrimSpace(signal.Code) == "" || len(signal.Code) > 96 {
		return gerror.New("operations signal code is invalid")
	}
	if signal.Status != "ok" && signal.Status != "warning" && signal.Status != "critical" && signal.Status != "unknown" {
		return gerror.New("operations signal status is invalid")
	}
	if signal.Count < 0 || signal.AgeSeconds < 0 || signal.ValueMinor < 0 || signal.ThresholdMinor < 0 {
		return gerror.New("operations signal values are invalid")
	}
	return nil
}

func (s *serviceImpl) ListOperationalAlerts(ctx context.Context) ([]OperationalAlert, error) {
	if ctx == nil {
		return nil, gerror.New("operations context is required")
	}
	rows, err := s.database.GetAll(ctx, `
SELECT signal_kind, signal_code, status_code, count_value, age_seconds, observed_at
FROM plugin_idc_core_operations_signal
WHERE tenant_id=0 AND status_code IN ('warning','critical','unknown')
ORDER BY CASE status_code WHEN 'critical' THEN 1 WHEN 'unknown' THEN 2 ELSE 3 END, observed_at ASC, id ASC
LIMIT 200`)
	if err != nil {
		return nil, gerror.Wrap(err, "list operations alerts")
	}
	alerts := make([]OperationalAlert, 0, len(rows))
	for _, row := range rows {
		alerts = append(alerts, OperationalAlert{Kind: row["signal_kind"].String(), Code: row["signal_code"].String(), Severity: row["status_code"].String(), Count: row["count_value"].Int64(), AgeSeconds: row["age_seconds"].Int64(), ObservedAt: row["observed_at"].Time().UTC()})
	}
	return alerts, nil
}
