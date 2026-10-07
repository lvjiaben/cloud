// Package contractcompensation owns the paid-order stop and refund handoff
// that follows a supplier-authoritative failure of a required contract.
package contractcompensation

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"reflect"
	"regexp"
	"strings"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-plugin-idc-core/backend/cap/contractcap"
	"lina-plugin-idc-core/backend/cap/idccap"
)

var (
	businessNoPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,63}$`)
	digestPattern     = regexp.MustCompile(`^[0-9a-f]{64}$`)
)

const maximumCompensationRows = 1000

type serviceImpl struct {
	database gdb.DB
	audit    idccap.BusinessAuditWriter
}

type orderRow struct {
	OrderNo          string     `orm:"order_no"`
	CustomerNo       string     `orm:"customer_no"`
	CurrencyCode     string     `orm:"currency_code"`
	StatusCode       string     `orm:"status_code"`
	RequiresContract bool       `orm:"requires_contract"`
	PaidAt           *time.Time `orm:"paid_at"`
}

type invoiceRow struct {
	InvoiceNo             string `orm:"invoice_no"`
	CustomerNo            string `orm:"customer_no"`
	CurrencyCode          string `orm:"currency_code"`
	StatusCode            string `orm:"status_code"`
	PaidAmountMinor       int64  `orm:"paid_amount_minor"`
	RefundableAmountMinor int64  `orm:"refundable_amount_minor"`
	RefundedAmountMinor   int64  `orm:"refunded_amount_minor"`
}

type fulfillmentIntentRow struct {
	IntentNo   string `orm:"intent_no"`
	StatusCode string `orm:"status_code"`
}

type fulfillmentTaskRow struct {
	TaskNo            string `orm:"task_no"`
	IntentNo          string `orm:"intent_no"`
	StatusCode        string `orm:"status_code"`
	LeaseToken        string `orm:"lease_token"`
	SubmitAttempts    int    `orm:"submit_attempt_count"`
	TakeoverRequested bool   `orm:"takeover_requested"`
}

type reservationRow struct {
	ReservationNo    string     `orm:"reservation_no"`
	StatusCode       string     `orm:"status_code"`
	ConsumedAt       *time.Time `orm:"consumed_at"`
	ConsumedQuantity int        `orm:"consumed_quantity"`
}

type entitlementRow struct {
	EntitlementNo string `orm:"entitlement_no"`
	IntentNo      string `orm:"source_intent_no"`
	StatusCode    string `orm:"status_code"`
}

type fulfillmentPlan struct {
	CancelIntents        []string
	CancelTasks          []string
	ManualIntents        []string
	ManualTasks          []string
	RequestTakeoverTasks []string
}

type compensationRow struct {
	CompensationNo  string    `orm:"compensation_no"`
	ContractNo      string    `orm:"contract_no"`
	OrderNo         string    `orm:"order_no"`
	TerminalEventNo string    `orm:"terminal_event_no"`
	TerminalStatus  string    `orm:"terminal_status_code"`
	PolicyNo        string    `orm:"policy_no"`
	PolicyVersion   int64     `orm:"policy_version"`
	Deadline        time.Time `orm:"handling_deadline"`
	RequestDigest   string    `orm:"request_digest"`
	StatusCode      string    `orm:"status_code"`
	ReasonCode      string    `orm:"reason_code"`
	RefundRequestNo *string   `orm:"refund_request_no"`
}

var _ contractcap.FailureCompensator = (*serviceImpl)(nil)

// New constructs the core-owned command. The audit writer participates in the
// same GoFrame transaction through the transaction context.
func New(database gdb.DB, audit idccap.BusinessAuditWriter) (contractcap.FailureCompensator, error) {
	if nilDependency(database) || nilDependency(audit) {
		return nil, gerror.New("contract failure compensation dependencies are invalid")
	}
	return &serviceImpl{database: database, audit: audit}, nil
}

// CompensateContractFailure is exact-replay safe. It never calls a payment
// channel and never reports a refund as completed before posted refund truth.
func (s *serviceImpl) CompensateContractFailure(
	ctx context.Context,
	input contractcap.FailureCompensationInput,
) (result *contractcap.FailureCompensationResult, err error) {
	normalized, requestDigest, err := normalizeInput(input)
	if err != nil || ctx == nil || ctx.Err() != nil {
		return nil, gerror.New("contract failure compensation input is invalid")
	}
	err = s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		existing, loadErr := s.loadCompensation(ctx, normalized.ContractNo, true)
		if loadErr != nil {
			return loadErr
		}
		if existing != nil {
			if !compensationMatches(existing, normalized, requestDigest) {
				return gerror.New("contract failure compensation conflicts with existing request")
			}
			result = project(existing, true)
			return nil
		}

		order, loadErr := s.lockOrder(ctx, normalized.OrderNo)
		if loadErr != nil {
			return loadErr
		}
		invoice, loadErr := s.lockInvoice(ctx, normalized.OrderNo)
		if loadErr != nil {
			return loadErr
		}
		if order == nil || invoice == nil || order.CustomerNo != invoice.CustomerNo ||
			order.CurrencyCode != invoice.CurrencyCode {
			return gerror.New("contract failure compensation order graph is unavailable")
		}

		now, loadErr := databaseNow(ctx, s.database)
		if loadErr != nil {
			return loadErr
		}
		if normalized.TriggerCode == "deadline" && now.Before(normalized.HandlingDeadline) {
			return gerror.New("contract failure compensation deadline has not arrived")
		}

		intents, tasks, reservations, entitlements, loadErr := s.lockOrderGraph(ctx, order.OrderNo)
		if loadErr != nil {
			return loadErr
		}
		plan, reason := automaticSafetyPlan(order, invoice, intents, tasks, reservations, entitlements)
		if reason != "" {
			if containErr := s.applyFulfillmentContainment(ctx, plan); containErr != nil {
				return containErr
			}
			row, insertErr := s.insertCompensation(
				ctx, normalized, requestDigest, order, invoice, "manual_review", reason, "", now,
			)
			if insertErr != nil {
				return insertErr
			}
			if auditErr := s.appendAudit(ctx, row, normalized.TriggerCode, "manual_review", reason); auditErr != nil {
				return auditErr
			}
			result = project(row, false)
			return nil
		}

		compensationNo := deterministicNo("ccp_", requestDigest)
		refundRequestNo := ""
		statusCode := "completed"
		if invoice.PaidAmountMinor > 0 {
			refundRequestNo = deterministicNo("rfr_", requestDigest)
			statusCode = "refund_pending"
		}
		row, insertErr := s.insertCompensation(
			ctx, normalized, requestDigest, order, invoice, statusCode,
			"required_contract_terminal", refundRequestNo, now,
		)
		if insertErr != nil {
			return insertErr
		}
		if row.CompensationNo != compensationNo {
			return gerror.New("contract failure compensation identity is inconsistent")
		}
		if mutateErr := s.applySafeStop(
			ctx, order, invoice, plan, reservations,
			requestDigest, refundRequestNo, now,
		); mutateErr != nil {
			return mutateErr
		}
		if auditErr := s.appendAudit(
			ctx, row, normalized.TriggerCode, statusCode, "required_contract_terminal",
		); auditErr != nil {
			return auditErr
		}
		result = project(row, false)
		return nil
	})
	if err != nil {
		return nil, gerror.Wrap(err, "compensate required contract failure")
	}
	return result, nil
}

func normalizeInput(input contractcap.FailureCompensationInput) (contractcap.FailureCompensationInput, string, error) {
	input.ContractNo = strings.TrimSpace(input.ContractNo)
	input.OrderNo = strings.TrimSpace(input.OrderNo)
	input.TerminalEventNo = strings.TrimSpace(input.TerminalEventNo)
	input.TerminalStatus = strings.TrimSpace(input.TerminalStatus)
	input.PolicyNo = strings.TrimSpace(input.PolicyNo)
	input.TriggerCode = strings.TrimSpace(input.TriggerCode)
	input.HandlingDeadline = input.HandlingDeadline.UTC().Truncate(time.Microsecond)
	if !businessNoPattern.MatchString(input.ContractNo) || len(input.ContractNo) > 64 ||
		!businessNoPattern.MatchString(input.OrderNo) || len(input.OrderNo) > 40 ||
		!businessNoPattern.MatchString(input.TerminalEventNo) ||
		!businessNoPattern.MatchString(input.PolicyNo) || len(input.PolicyNo) > 40 ||
		input.PolicyVersion < 1 || input.HandlingDeadline.IsZero() ||
		(input.TriggerCode != "deadline" && input.TriggerCode != "admin_cancel") {
		return input, "", gerror.New("contract failure compensation identity is invalid")
	}
	switch input.TerminalStatus {
	case "rejected", "expired", "revoked", "failed":
	default:
		return input, "", gerror.New("contract failure terminal status is invalid")
	}
	digest := sha256Hex(strings.Join([]string{
		"contract-failure-compensation-v1", input.ContractNo, input.OrderNo,
		input.TerminalEventNo, input.TerminalStatus, input.PolicyNo,
		fmt.Sprintf("%d", input.PolicyVersion), input.HandlingDeadline.Format(time.RFC3339Nano),
	}, "\x00"))
	return input, digest, nil
}

func (s *serviceImpl) lockOrder(ctx context.Context, orderNo string) (*orderRow, error) {
	var rows []orderRow
	if err := s.database.GetScan(ctx, &rows, `
SELECT order_no, customer_no, currency_code, status_code, requires_contract, paid_at
FROM plugin_idc_core_commerce_order
WHERE tenant_id = 0 AND order_no = ?
FOR UPDATE`, orderNo); err != nil {
		return nil, gerror.Wrap(err, "lock contract compensation order")
	}
	if len(rows) != 1 {
		return nil, nil
	}
	return &rows[0], nil
}

func (s *serviceImpl) lockInvoice(ctx context.Context, orderNo string) (*invoiceRow, error) {
	var rows []invoiceRow
	if err := s.database.GetScan(ctx, &rows, `
SELECT invoice_no, customer_no, currency_code, status_code,
       paid_amount_minor, refundable_amount_minor, refunded_amount_minor
FROM plugin_idc_core_billing_invoice
WHERE tenant_id = 0 AND source_code = 'order' AND order_no = ?
FOR UPDATE`, orderNo); err != nil {
		return nil, gerror.Wrap(err, "lock contract compensation invoice")
	}
	if len(rows) != 1 {
		return nil, nil
	}
	return &rows[0], nil
}

func (s *serviceImpl) lockOrderGraph(
	ctx context.Context,
	orderNo string,
) ([]fulfillmentIntentRow, []fulfillmentTaskRow, []reservationRow, []entitlementRow, error) {
	var intents []fulfillmentIntentRow
	if err := s.database.GetScan(ctx, &intents, `
SELECT intent_no, status_code
FROM plugin_idc_core_product_fulfillment_intent
WHERE tenant_id = 0 AND order_no = ?
ORDER BY intent_no
FOR UPDATE`, orderNo); err != nil {
		return nil, nil, nil, nil, gerror.Wrap(err, "lock contract compensation intents")
	}
	var tasks []fulfillmentTaskRow
	if err := s.database.GetScan(ctx, &tasks, `
SELECT task.task_no, task.intent_no, task.status_code, task.lease_token,
       task.submit_attempt_count, task.takeover_requested
FROM plugin_idc_core_product_fulfillment_task task
JOIN plugin_idc_core_product_fulfillment_intent intent
  ON intent.tenant_id = task.tenant_id AND intent.intent_no = task.intent_no
WHERE task.tenant_id = 0 AND intent.order_no = ?
ORDER BY task.task_no
FOR UPDATE OF task`, orderNo); err != nil {
		return nil, nil, nil, nil, gerror.Wrap(err, "lock contract compensation tasks")
	}
	var reservations []reservationRow
	if err := s.database.GetScan(ctx, &reservations, `
SELECT reservation_no, status_code, consumed_at, consumed_quantity
FROM plugin_idc_core_commerce_inventory_reservation
WHERE tenant_id = 0 AND order_no = ?
	ORDER BY id
FOR UPDATE`, orderNo); err != nil {
		return nil, nil, nil, nil, gerror.Wrap(err, "lock contract compensation inventory")
	}
	var entitlements []entitlementRow
	if err := s.database.GetScan(ctx, &entitlements, `
SELECT entitlement_no, source_intent_no, status_code
FROM plugin_idc_core_service_entitlement
WHERE tenant_id = 0 AND order_no = ?
ORDER BY entitlement_no
FOR UPDATE`, orderNo); err != nil {
		return nil, nil, nil, nil, gerror.Wrap(err, "lock contract compensation entitlements")
	}
	if len(intents) > maximumCompensationRows || len(tasks) > maximumCompensationRows ||
		len(reservations) > maximumCompensationRows || len(entitlements) > maximumCompensationRows {
		return nil, nil, nil, nil, gerror.New("contract compensation order graph exceeds limit")
	}
	return intents, tasks, reservations, entitlements, nil
}

func automaticSafetyPlan(
	order *orderRow,
	invoice *invoiceRow,
	intents []fulfillmentIntentRow,
	tasks []fulfillmentTaskRow,
	reservations []reservationRow,
	entitlements []entitlementRow,
) (fulfillmentPlan, string) {
	plan := fulfillmentPlan{}
	if order == nil || !order.RequiresContract || order.PaidAt == nil || order.StatusCode != "paid" {
		return plan, "order_not_safely_paid"
	}
	reason := ""
	if invoice == nil || (invoice.StatusCode != "paid" && invoice.StatusCode != "credit_settled") ||
		invoice.RefundedAmountMinor != 0 || invoice.RefundableAmountMinor != invoice.PaidAmountMinor {
		reason = "financial_state_not_refundable"
	}
	if len(intents) == 0 {
		return plan, "fulfillment_started_or_unknown"
	}
	if len(entitlements) > 0 && reason == "" {
		reason = "fulfillment_started_or_unknown"
	}
	taskByIntent := make(map[string]fulfillmentTaskRow, len(tasks))
	for _, task := range tasks {
		if _, exists := taskByIntent[task.IntentNo]; exists {
			return plan, "fulfillment_graph_inconsistent"
		}
		taskByIntent[task.IntentNo] = task
	}
	for _, row := range reservations {
		if (row.StatusCode != "held" && row.StatusCode != "committed") ||
			row.ConsumedAt != nil || row.ConsumedQuantity != 0 {
			if reason == "" {
				reason = "inventory_consumed_or_unknown"
			}
		}
	}
	for _, intent := range intents {
		task, hasTask := taskByIntent[intent.IntentNo]
		intentReason := appendIntentContainment(&plan, intent, task, hasTask)
		if intentReason == "fulfillment_graph_inconsistent" {
			return fulfillmentPlan{}, "fulfillment_graph_inconsistent"
		}
		if reason == "" {
			reason = intentReason
		}
	}
	return plan, reason
}

func appendIntentContainment(
	plan *fulfillmentPlan,
	intent fulfillmentIntentRow,
	task fulfillmentTaskRow,
	hasTask bool,
) string {
	switch intent.StatusCode {
	case "manual_pending", "review_pending", "gate_pending":
		if hasTask {
			return "fulfillment_graph_inconsistent"
		}
		plan.CancelIntents = append(plan.CancelIntents, intent.IntentNo)
		return ""
	case "queued", "processing":
		if !hasTask {
			return "fulfillment_graph_inconsistent"
		}
		return appendTaskContainment(plan, intent.IntentNo, task)
	case "completed", "incomplete":
		return "fulfillment_started_or_unknown"
	case "canceled":
		return ""
	default:
		return "fulfillment_graph_inconsistent"
	}
}

func appendTaskContainment(
	plan *fulfillmentPlan,
	intentNo string,
	task fulfillmentTaskRow,
) string {
	switch task.StatusCode {
	case "pending_query", "pending_submit", "retry_wait":
		if task.SubmitAttempts == 0 && task.LeaseToken == "" {
			plan.CancelTasks = append(plan.CancelTasks, task.TaskNo)
			plan.CancelIntents = append(plan.CancelIntents, intentNo)
			return ""
		}
		plan.ManualTasks = append(plan.ManualTasks, task.TaskNo)
		plan.ManualIntents = append(plan.ManualIntents, intentNo)
		return "fulfillment_started_or_unknown"
	case "querying", "submitting":
		if task.LeaseToken == "" {
			return "fulfillment_graph_inconsistent"
		}
		if !task.TakeoverRequested {
			plan.RequestTakeoverTasks = append(plan.RequestTakeoverTasks, task.TaskNo)
		}
		return "fulfillment_started_or_unknown"
	case "accepted", "running", "result_unknown":
		if task.LeaseToken != "" {
			return "fulfillment_graph_inconsistent"
		}
		plan.ManualTasks = append(plan.ManualTasks, task.TaskNo)
		plan.ManualIntents = append(plan.ManualIntents, intentNo)
		return "fulfillment_started_or_unknown"
	case "succeeded", "failed", "manual_takeover", "cancellation_attention", "canceled":
		return "fulfillment_started_or_unknown"
	default:
		return "fulfillment_graph_inconsistent"
	}
}

func (s *serviceImpl) applySafeStop(
	ctx context.Context,
	order *orderRow,
	invoice *invoiceRow,
	plan fulfillmentPlan,
	reservations []reservationRow,
	requestDigest string,
	refundRequestNo string,
	now time.Time,
) error {
	if err := s.applyFulfillmentContainment(ctx, plan); err != nil {
		return err
	}
	if len(reservations) > 0 {
		result, err := s.database.Exec(ctx, `
UPDATE plugin_idc_core_commerce_inventory_reservation
SET status_code = 'released', release_reason_code = 'contract_failed',
    released_at = ?, released_quantity = reserved_quantity,
    version = version + 1, updated_at = ?
WHERE tenant_id = 0 AND order_no = ? AND status_code IN ('held','committed')
  AND consumed_at IS NULL AND consumed_quantity = 0`, now, now, order.OrderNo)
		if err != nil || affected(result) != int64(len(reservations)) {
			return gerror.New("release contract-gated inventory failed")
		}
	}
	orderStatus := "refunded"
	if refundRequestNo != "" {
		orderStatus = "refund_pending"
	}
	result, err := s.database.Exec(ctx, `
UPDATE plugin_idc_core_commerce_order
SET status_code = ?, version = version + 1, updated_at = ?
WHERE tenant_id = 0 AND order_no = ? AND status_code = 'paid'
  AND paid_at IS NOT NULL AND requires_contract`, orderStatus, now, order.OrderNo)
	if err != nil || affected(result) != 1 {
		return gerror.New("move contract-gated order to refund failed")
	}
	if refundRequestNo == "" {
		return nil
	}
	_, err = s.database.Exec(ctx, `
INSERT INTO plugin_idc_core_billing_refund_request
    (tenant_id, refund_request_no, compensation_no, order_no, invoice_no,
     customer_no, currency_code, amount_minor, status_code,
     idempotency_digest, created_at, updated_at)
VALUES (0, ?, ?, ?, ?, ?, ?, ?, 'ready', ?, ?, ?)`,
		refundRequestNo, deterministicNo("ccp_", requestDigest), order.OrderNo,
		invoice.InvoiceNo, order.CustomerNo, order.CurrencyCode, invoice.PaidAmountMinor,
		requestDigest, now, now)
	if err != nil {
		return gerror.Wrap(err, "create unique contract compensation refund request")
	}
	eventDigest := sha256Hex("contract-refund-requested\x00" + requestDigest)
	_, err = s.database.Exec(ctx, `
INSERT INTO plugin_idc_core_outbox_event
    (tenant_id, event_no, event_type, event_version, topic, aggregate_type,
     aggregate_no, correlation_no, deduplication_digest, payload, metadata,
     occurred_at, available_at, created_at, updated_at)
VALUES (0, ?, 'billing.refund.requested', 1, 'idc.payment.refund',
        'billing_refund_request', ?, ?, ?,
        jsonb_build_object(
            'refundRequestNo', ?::text, 'orderNo', ?::text, 'invoiceNo', ?::text,
            'customerNo', ?::text, 'currencyCode', ?::text, 'amountMinor', ?::bigint
        ),
        jsonb_build_object(
            'owner', 'idc-core',
            'reason', 'required_contract_terminal'
        ),
        ?, ?, ?, ?)`,
		deterministicNo("evt_", eventDigest), refundRequestNo,
		deterministicNo("ccp_", requestDigest), eventDigest,
		refundRequestNo, order.OrderNo, invoice.InvoiceNo, order.CustomerNo,
		order.CurrencyCode, invoice.PaidAmountMinor,
		now, now, now, now)
	return gerror.Wrap(err, "enqueue contract compensation refund request")
}

// applyFulfillmentContainment stops only work that has not crossed the
// supplier boundary. In-flight work is asked to converge to takeover, while
// accepted or unknown work is frozen for manual handling without erasing its
// provider evidence.
func (s *serviceImpl) applyFulfillmentContainment(ctx context.Context, plan fulfillmentPlan) error {
	if len(plan.CancelIntents) > 0 {
		raw, err := json.Marshal(plan.CancelIntents)
		if err != nil {
			return gerror.Wrap(err, "encode contract compensation canceled intents")
		}
		if _, err = s.database.Exec(ctx, `
INSERT INTO plugin_idc_core_product_fulfillment_system_transition
    (tenant_id,transition_no,intent_no,event_code,reason_code,reason_label_zh_cn,reason_label_en_us)
SELECT 0,'pfs_'||substr(md5('contract-terminal-cancelled'||intent_no),1,32),intent_no,
       'cancelled','required_contract_terminal','必签合同已终止','Required contract terminated'
FROM jsonb_array_elements_text(?::jsonb) value(intent_no)
ON CONFLICT(intent_no,event_code) DO NOTHING`, raw); err != nil {
			return gerror.Wrap(err, "record contract compensation canceled intents")
		}
		result, updateErr := s.database.Exec(ctx, `
UPDATE plugin_idc_core_product_fulfillment_intent
SET status_code='canceled',version=version+1,updated_at=CURRENT_TIMESTAMP
WHERE tenant_id=0 AND intent_no IN(SELECT jsonb_array_elements_text(?::jsonb))
  AND status_code IN('manual_pending','review_pending','gate_pending','queued','processing')`, raw)
		if updateErr != nil || affected(result) != int64(len(plan.CancelIntents)) {
			return gerror.New("cancel contract-gated fulfillment intents failed")
		}
	}
	if len(plan.CancelTasks) > 0 {
		raw, err := json.Marshal(plan.CancelTasks)
		if err != nil {
			return gerror.Wrap(err, "encode contract compensation canceled tasks")
		}
		result, updateErr := s.database.Exec(ctx, `
UPDATE plugin_idc_core_product_fulfillment_task
SET status_code='canceled',phase_code='frozen_unknown',cancel_requested=TRUE,
    next_attempt_at=CURRENT_TIMESTAMP,version=version+1,updated_at=CURRENT_TIMESTAMP
WHERE tenant_id=0 AND task_no IN(SELECT jsonb_array_elements_text(?::jsonb))
  AND status_code IN('pending_query','pending_submit','retry_wait')
  AND lease_token='' AND submit_attempt_count=0`, raw)
		if updateErr != nil || affected(result) != int64(len(plan.CancelTasks)) {
			return gerror.New("cancel contract-gated fulfillment tasks failed")
		}
	}
	if len(plan.ManualIntents) > 0 {
		raw, err := json.Marshal(plan.ManualIntents)
		if err != nil {
			return gerror.Wrap(err, "encode contract compensation manual intents")
		}
		if _, err = s.database.Exec(ctx, `
INSERT INTO plugin_idc_core_product_fulfillment_system_transition
    (tenant_id,transition_no,intent_no,event_code,reason_code,reason_label_zh_cn,reason_label_en_us)
SELECT 0,'pfs_'||substr(md5('contract-terminal-manual'||intent_no),1,32),intent_no,
       'moved_to_manual','required_contract_manual','合同已终止，履约需人工处理','Contract terminated; fulfillment requires manual handling'
FROM jsonb_array_elements_text(?::jsonb) value(intent_no)
ON CONFLICT(intent_no,event_code) DO NOTHING`, raw); err != nil {
			return gerror.Wrap(err, "record contract compensation manual intents")
		}
		result, updateErr := s.database.Exec(ctx, `
UPDATE plugin_idc_core_product_fulfillment_intent
SET status_code='manual_pending',version=version+1,updated_at=CURRENT_TIMESTAMP
WHERE tenant_id=0 AND intent_no IN(SELECT jsonb_array_elements_text(?::jsonb))
  AND status_code IN('queued','processing')`, raw)
		if updateErr != nil || affected(result) != int64(len(plan.ManualIntents)) {
			return gerror.New("move contract-gated fulfillment intents to manual failed")
		}
	}
	if len(plan.ManualTasks) > 0 {
		raw, err := json.Marshal(plan.ManualTasks)
		if err != nil {
			return gerror.Wrap(err, "encode contract compensation manual tasks")
		}
		result, updateErr := s.database.Exec(ctx, `
UPDATE plugin_idc_core_product_fulfillment_task
SET status_code='manual_takeover',phase_code='frozen_unknown',takeover_requested=TRUE,
    lease_token='',lease_expires_at=NULL,version=version+1,updated_at=CURRENT_TIMESTAMP
WHERE tenant_id=0 AND task_no IN(SELECT jsonb_array_elements_text(?::jsonb))
  AND status_code IN('pending_query','pending_submit','retry_wait','accepted','running','result_unknown')
  AND lease_token=''`, raw)
		if updateErr != nil || affected(result) != int64(len(plan.ManualTasks)) {
			return gerror.New("move contract-gated fulfillment tasks to manual failed")
		}
	}
	if len(plan.RequestTakeoverTasks) > 0 {
		raw, err := json.Marshal(plan.RequestTakeoverTasks)
		if err != nil {
			return gerror.Wrap(err, "encode contract compensation takeover requests")
		}
		result, updateErr := s.database.Exec(ctx, `
UPDATE plugin_idc_core_product_fulfillment_task
SET takeover_requested=TRUE,version=version+1,updated_at=CURRENT_TIMESTAMP
WHERE tenant_id=0 AND task_no IN(SELECT jsonb_array_elements_text(?::jsonb))
  AND status_code IN('querying','submitting') AND lease_token<>'' AND takeover_requested=FALSE`, raw)
		if updateErr != nil || affected(result) != int64(len(plan.RequestTakeoverTasks)) {
			return gerror.New("request contract-gated fulfillment takeover failed")
		}
	}
	return nil
}

func (s *serviceImpl) insertCompensation(
	ctx context.Context,
	input contractcap.FailureCompensationInput,
	requestDigest string,
	order *orderRow,
	invoice *invoiceRow,
	statusCode string,
	reasonCode string,
	refundRequestNo string,
	now time.Time,
) (*compensationRow, error) {
	compensationNo := deterministicNo("ccp_", requestDigest)
	var refundPointer any
	if refundRequestNo != "" {
		refundPointer = refundRequestNo
	}
	var completedAt any
	if statusCode == "completed" {
		completedAt = now
	}
	_, err := s.database.Exec(ctx, `
INSERT INTO plugin_idc_core_contract_failure_compensation
    (tenant_id, compensation_no, contract_no, order_no, customer_no, invoice_no,
     terminal_event_no, terminal_status_code, policy_no, policy_version,
     handling_deadline, request_digest, status_code, reason_code,
     refund_request_no, completed_at, created_at, updated_at)
VALUES (0, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		compensationNo, input.ContractNo, order.OrderNo, order.CustomerNo, invoice.InvoiceNo,
		input.TerminalEventNo, input.TerminalStatus, input.PolicyNo, input.PolicyVersion,
		input.HandlingDeadline, requestDigest, statusCode, reasonCode,
		refundPointer, completedAt, now, now)
	if err != nil {
		return nil, gerror.Wrap(err, "insert contract failure compensation")
	}
	return s.loadCompensation(ctx, input.ContractNo, false)
}

func (s *serviceImpl) loadCompensation(
	ctx context.Context,
	contractNo string,
	lock bool,
) (*compensationRow, error) {
	query := `
SELECT compensation_no, contract_no, order_no, terminal_event_no,
       terminal_status_code, policy_no, policy_version, handling_deadline,
       request_digest, status_code, reason_code, refund_request_no
FROM plugin_idc_core_contract_failure_compensation
WHERE tenant_id = 0 AND contract_no = ?`
	if lock {
		query += " FOR UPDATE"
	}
	var rows []compensationRow
	if err := s.database.GetScan(ctx, &rows, query, contractNo); err != nil {
		return nil, gerror.Wrap(err, "load contract failure compensation")
	}
	if len(rows) == 0 {
		return nil, nil
	}
	if len(rows) != 1 {
		return nil, gerror.New("contract failure compensation is ambiguous")
	}
	return &rows[0], nil
}

func (s *serviceImpl) appendAudit(
	ctx context.Context,
	row *compensationRow,
	triggerCode string,
	resultCode string,
	reasonCode string,
) error {
	return s.audit.AppendBusinessTransition(ctx, idccap.BusinessAuditTransition{
		IdempotencyScope: "core.contract-failure-compensation",
		IdempotencyKey:   row.RequestDigest,
		OperationCode:    "core.contract-failure-compensation",
		TargetType:       "commerce_order",
		TargetNo:         row.OrderNo,
		ResultCode:       resultCode,
		ReasonCode:       reasonCode,
		RiskCode:         "critical",
		CorrelationNo:    row.CompensationNo,
		BeforeSnapshot: map[string]any{
			"contract_no":     row.ContractNo,
			"terminal_status": row.TerminalStatus,
			"trigger":         triggerCode,
		},
		AfterSnapshot: map[string]any{
			"compensation_status":    row.StatusCode,
			"refund_request_created": row.RefundRequestNo != nil,
		},
	})
}

func compensationMatches(
	row *compensationRow,
	input contractcap.FailureCompensationInput,
	requestDigest string,
) bool {
	return row != nil && row.ContractNo == input.ContractNo && row.OrderNo == input.OrderNo &&
		row.TerminalEventNo == input.TerminalEventNo && row.TerminalStatus == input.TerminalStatus &&
		row.PolicyNo == input.PolicyNo && row.PolicyVersion == input.PolicyVersion &&
		row.Deadline.Equal(input.HandlingDeadline) && row.RequestDigest == requestDigest
}

func project(row *compensationRow, reused bool) *contractcap.FailureCompensationResult {
	result := &contractcap.FailureCompensationResult{
		CompensationNo: row.CompensationNo,
		OrderNo:        row.OrderNo,
		StatusCode:     row.StatusCode,
		ReasonCode:     row.ReasonCode,
		Reused:         reused,
	}
	if row.RefundRequestNo != nil {
		result.RefundRequestNo = *row.RefundRequestNo
	}
	return result
}

func databaseNow(ctx context.Context, database gdb.DB) (time.Time, error) {
	var row struct {
		Now time.Time `orm:"database_now"`
	}
	if err := database.GetScan(ctx, &row, `SELECT CURRENT_TIMESTAMP AS database_now`); err != nil || row.Now.IsZero() {
		return time.Time{}, gerror.New("read contract compensation database time failed")
	}
	return row.Now.UTC(), nil
}

func deterministicNo(prefix, digest string) string {
	if !digestPattern.MatchString(digest) {
		digest = sha256Hex(digest)
	}
	return prefix + digest[:32]
}

func sha256Hex(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func affected(result sql.Result) int64 {
	if result == nil {
		return -1
	}
	count, err := result.RowsAffected()
	if err != nil {
		return -1
	}
	return count
}

func nilDependency(value any) bool {
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
