package serveroperation

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-plugin-idc-core/backend/cap/serveroperationresourcecap"
)

type channelWorkRow struct {
	OperationNo, ServiceNo, ActionCode, Parameters, ChannelNo, PoolNo string
	ProviderTaskNo                                                    *string
	StatusCode, RequestDigest, TaskEvidenceDigest                     string
	BusinessStatus                                                    string
	ExpectedServiceVersion, ServiceVersion, Version                   int64
	EffectiveAt, ObservedAt                                           time.Time
	QuoteNo, OwnerCustomerNo, PayerCustomerNo, CurrencyCode           string
	Direction, TargetConfigurationNo, TargetVersionNo                 string
	TargetBillingRuleNo                                               string
	TargetRecurring, TargetSubtotal, TargetNet, TargetTax             int64
	ReservationNo                                                     string
}

type channelWorkerOutput struct {
	Name      string    `json:"name"`
	Policy    string    `json:"policy"`
	SecretRef string    `json:"secretRef"`
	ExpiresAt time.Time `json:"expiresAt,omitempty"`
}

func channelTaskRequest(row *channelWorkRow) (serveroperationresourcecap.TaskRequest, error) {
	request := serveroperationresourcecap.TaskRequest{OperationNo: row.OperationNo,
		ServiceNo: row.ServiceNo, ChannelNo: row.ChannelNo, PoolNo: row.PoolNo,
		Action:                 serveroperationresourcecap.Action(row.ActionCode),
		ExpectedServiceVersion: row.ExpectedServiceVersion,
		Parameters:             json.RawMessage(row.Parameters)}
	digest, err := serveroperationresourcecap.TaskRequestDigest(request)
	if err != nil || digest != row.RequestDigest || row.ServiceVersion != row.ExpectedServiceVersion {
		return serveroperationresourcecap.TaskRequest{}, serveroperationresourcecap.ErrTaskConflict
	}
	request.RequestDigest = digest
	return request, nil
}

func (w *workerImpl) processChannelPricedOperations(
	ctx context.Context,
	limit int,
) (selected, dispatched, reconciled, manual, failures int, batchErr error) {
	rows, err := w.loadChannelWork(ctx, limit)
	if err != nil {
		return 0, 0, 0, 0, 1, err
	}
	for index := range rows {
		row := &rows[index]
		selected++
		if row.StatusCode == "scheduled" {
			if err = w.materializeChannelSchedule(ctx, row); err != nil {
				failures++
				batchErr = err
			} else {
				reconciled++
			}
			continue
		}
		request, requestErr := channelTaskRequest(row)
		if requestErr != nil {
			if err = w.freezeChannelOperation(ctx, row.OperationNo,
				"channel.operation.intent_conflict", row.TaskEvidenceDigest); err != nil {
				failures++
				batchErr = err
			} else {
				manual++
			}
			continue
		}
		var task *serveroperationresourcecap.Task
		if row.StatusCode == "ready" {
			task, err = w.source.CreateTask(ctx, request)
			if err == nil && (task == nil ||
				serveroperationresourcecap.ValidateTaskForRequest(*task, request) != nil) {
				err = serveroperationresourcecap.ErrSourceUnavailable
			}
			if err == nil {
				err = w.attachChannelTask(ctx, row, task)
			}
			if err == nil && row.ActionCode == "ip_change" {
				err = w.consumeChannelIPReservation(ctx, row, task)
			}
			if err != nil || task == nil || serveroperationresourcecap.ValidateTaskForRequest(*task, request) != nil {
				if errors.Is(err, serveroperationresourcecap.ErrCapabilityUnavailable) ||
					errors.Is(err, serveroperationresourcecap.ErrTaskConflict) ||
					errors.Is(err, serveroperationresourcecap.ErrIPReservationConflict) {
					if freezeErr := w.freezeChannelOperation(ctx, row.OperationNo,
						"channel.operation.dispatch_conflict", row.TaskEvidenceDigest); freezeErr == nil {
						manual++
						continue
					}
				}
				failures++
				batchErr = err
				continue
			}
			dispatched++
			if task.Status == serveroperationresourcecap.TaskStatusPending ||
				task.Status == serveroperationresourcecap.TaskStatusRunning {
				continue
			}
			row, err = w.loadChannelWorkByOperation(ctx, row.OperationNo)
			if err != nil || row == nil {
				failures++
				batchErr = err
				continue
			}
		} else {
			if row.ProviderTaskNo == nil {
				if err = w.freezeChannelOperation(ctx, row.OperationNo,
					"channel.operation.task_missing", row.TaskEvidenceDigest); err != nil {
					failures++
					batchErr = err
				} else {
					manual++
				}
				continue
			}
			task, err = w.source.ReadTask(ctx, serveroperationresourcecap.TaskReadRequest{
				ProviderTaskNo: *row.ProviderTaskNo, Request: request})
			if err != nil || task == nil || serveroperationresourcecap.ValidateTaskForRequest(*task, request) != nil {
				if errors.Is(err, serveroperationresourcecap.ErrTaskConflict) ||
					errors.Is(err, serveroperationresourcecap.ErrTaskUnavailable) {
					if freezeErr := w.freezeChannelOperation(ctx, row.OperationNo,
						"channel.operation.task_unavailable", row.TaskEvidenceDigest); freezeErr == nil {
						manual++
						continue
					}
				}
				failures++
				batchErr = err
				continue
			}
			reconciled++
		}
		isManual, err := w.convergeChannelTask(ctx, row, task)
		if err != nil {
			failures++
			batchErr = err
		} else if isManual {
			manual++
		}
	}
	return selected, dispatched, reconciled, manual, failures, batchErr
}

const channelWorkSelect = `SELECT operation.operation_no,operation.service_no,
operation.action_code,operation.parameters::TEXT AS parameters,operation.resource_channel_no AS channel_no,
operation.resource_pool_no AS pool_no,operation.provider_task_no,operation.status_code,
operation.request_digest,operation.task_evidence_digest,operation.expected_service_version,
entitlement.version AS service_version,operation.version,quote.effective_at,operation.observed_at,
business.status_code AS business_status,
quote.quote_no,quote.service_owner_customer_no AS owner_customer_no,quote.payer_customer_no,
quote.currency_code,quote.direction_code AS direction,
quote.target_configuration_snapshot_no AS target_configuration_no,quote.target_version_no,
quote.target_billing_rule_no,quote.target_recurring_amount_minor AS target_recurring,
quote.target_subtotal_minor AS target_subtotal,quote.target_net_amount_minor AS target_net,
quote.target_tax_amount_minor AS target_tax,COALESCE(quote.resource_reservation_no,'') AS reservation_no
FROM plugin_idc_core_channel_service_operation operation
JOIN plugin_idc_core_channel_service_quote quote
 ON quote.tenant_id=operation.tenant_id AND quote.service_no=operation.service_no
 AND quote.action_code=operation.action_code
JOIN plugin_idc_core_business_operation business
 ON business.tenant_id=operation.tenant_id AND business.operation_no=operation.operation_no
 AND business.quote_no=quote.quote_no
JOIN plugin_idc_core_service_entitlement entitlement
 ON entitlement.tenant_id=operation.tenant_id AND entitlement.entitlement_no=operation.service_no
`

func (w *workerImpl) loadChannelWork(ctx context.Context, limit int) ([]channelWorkRow, error) {
	var rows []channelWorkRow
	if err := w.database.GetScan(ctx, &rows, channelWorkSelect+`WHERE operation.tenant_id=0 AND operation.status_code IN
 ('scheduled','ready','pending','running','result_unknown')
 AND (operation.status_code<>'scheduled' OR quote.effective_at<=CURRENT_TIMESTAMP)
ORDER BY CASE WHEN operation.status_code='scheduled' THEN quote.effective_at ELSE operation.updated_at END,
	operation.id LIMIT ?`, limit); err != nil {
		return nil, gerror.Wrap(err, "load channel operation work")
	}
	return rows, nil
}

func (w *workerImpl) loadChannelWorkByOperation(
	ctx context.Context,
	operationNo string,
) (*channelWorkRow, error) {
	var row *channelWorkRow
	if err := w.database.GetScan(ctx, &row, channelWorkSelect+`
WHERE operation.tenant_id=0 AND operation.operation_no=?`, operationNo); err != nil {
		return nil, gerror.Wrap(err, "reload channel operation work")
	}
	return row, nil
}

func (w *workerImpl) materializeChannelSchedule(ctx context.Context, row *channelWorkRow) error {
	if row.ServiceVersion != row.ExpectedServiceVersion {
		return w.freezeChannelOperation(ctx, row.OperationNo,
			"channel.resize.schedule_conflict", row.TaskEvidenceDigest)
	}
	return w.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		now, err := databaseNow(ctx, w.database)
		if err != nil {
			return err
		}
		if err = execOne(ctx, w.database, `UPDATE plugin_idc_core_channel_service_operation
SET status_code='ready',version=version+1,updated_at=?
WHERE tenant_id=0 AND operation_no=? AND status_code='scheduled'`, now, row.OperationNo); err != nil {
			return err
		}
		if err = execOne(ctx, w.database, `UPDATE plugin_idc_core_business_operation
SET status_code='ready',version=version+1,updated_at=?
WHERE tenant_id=0 AND operation_no=? AND status_code='quoted'`, now, row.OperationNo); err != nil {
			return err
		}
		return insertChannelWorkerEvent(ctx, w.database, row.OperationNo, row.ServiceNo,
			"quoted", "ready", "channel.resize.scheduled_effective", row.TaskEvidenceDigest, now)
	})
}

func (w *workerImpl) attachChannelTask(ctx context.Context, row *channelWorkRow,
	task *serveroperationresourcecap.Task) error {
	outputs, err := encodeChannelWorkerOutputs(task.SecretOutputs)
	if err != nil {
		return err
	}
	return w.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		if err := execOne(ctx, w.database, `UPDATE plugin_idc_core_channel_service_operation
SET provider_task_no=?,status_code='pending',failure_code=?,task_evidence_digest=?,output_bindings=?::JSONB,
observed_at=?,completed_at=NULL,version=version+1,updated_at=CURRENT_TIMESTAMP
WHERE tenant_id=0 AND operation_no=? AND status_code='ready' AND provider_task_no IS NULL`,
			task.ProviderTaskNo, task.FailureCode, task.EvidenceDigest,
			string(outputs), task.ObservedAt, row.OperationNo); err != nil {
			return err
		}
		if err = execOne(ctx, w.database, `UPDATE plugin_idc_core_business_operation
SET provider_task_no=?,status_code='executing',execution_started_at=?,version=version+1,
updated_at=CURRENT_TIMESTAMP WHERE tenant_id=0 AND operation_no=? AND status_code='ready'`,
			task.ProviderTaskNo, task.ObservedAt, row.OperationNo); err != nil {
			return err
		}
		return insertChannelWorkerEvent(ctx, w.database, row.OperationNo, row.ServiceNo,
			"ready", "executing", "channel.operation.task_created", task.EvidenceDigest,
			task.ObservedAt.UTC())
	})
}

func (w *workerImpl) consumeChannelIPReservation(ctx context.Context, row *channelWorkRow,
	task *serveroperationresourcecap.Task) error {
	if w.ipCapacity == nil || row.ReservationNo == "" || task == nil {
		return serveroperationresourcecap.ErrIPReservationConflict
	}
	reservation, err := w.ipCapacity.ConsumeIP(ctx,
		serveroperationresourcecap.IPReservationConsumeRequest{
			ReservationNo: row.ReservationNo, ProviderTaskNo: task.ProviderTaskNo})
	if err != nil || reservation == nil || reservation.Status != serveroperationresourcecap.IPReservationConsumed {
		if err != nil {
			return err
		}
		return serveroperationresourcecap.ErrIPReservationConflict
	}
	return nil
}

func (w *workerImpl) convergeChannelTask(ctx context.Context, row *channelWorkRow,
	task *serveroperationresourcecap.Task) (bool, error) {
	if task.ObservedAt.Before(row.ObservedAt) {
		return true, w.freezeChannelOperation(ctx, row.OperationNo,
			"channel.operation.task_regressed", task.EvidenceDigest)
	}
	switch task.Status {
	case serveroperationresourcecap.TaskStatusPending,
		serveroperationresourcecap.TaskStatusRunning,
		serveroperationresourcecap.TaskStatusResultUnknown:
		return task.Status == serveroperationresourcecap.TaskStatusResultUnknown,
			w.observeChannelTask(ctx, row, task)
	case serveroperationresourcecap.TaskStatusSucceeded,
		serveroperationresourcecap.TaskStatusFailed,
		serveroperationresourcecap.TaskStatusManualReview:
		return task.Status == serveroperationresourcecap.TaskStatusManualReview,
			w.finishChannelTask(ctx, row, task)
	default:
		return false, serveroperationresourcecap.ErrSourceUnavailable
	}
}

func (w *workerImpl) observeChannelTask(ctx context.Context, row *channelWorkRow,
	task *serveroperationresourcecap.Task) error {
	businessStatus := "executing"
	if task.Status == serveroperationresourcecap.TaskStatusResultUnknown {
		businessStatus = "manual_review"
	}
	return w.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		if err := execOne(ctx, w.database, `UPDATE plugin_idc_core_channel_service_operation
SET status_code=?,failure_code=?,task_evidence_digest=?,observed_at=?,version=version+1,
updated_at=CURRENT_TIMESTAMP WHERE tenant_id=0 AND operation_no=? AND version=?`,
			task.Status, task.FailureCode, task.EvidenceDigest, task.ObservedAt,
			row.OperationNo, row.Version); err != nil {
			return err
		}
		_, err := w.database.Exec(ctx, `UPDATE plugin_idc_core_business_operation
SET status_code=?,last_error_code=?,version=version+1,updated_at=CURRENT_TIMESTAMP
WHERE tenant_id=0 AND operation_no=? AND status_code IN ('executing','manual_review')`,
			businessStatus, task.FailureCode, row.OperationNo)
		if err != nil || row.BusinessStatus == businessStatus {
			return err
		}
		return insertChannelWorkerEvent(ctx, w.database, row.OperationNo, row.ServiceNo,
			row.BusinessStatus, businessStatus, "channel.operation.provider_observed",
			task.EvidenceDigest, task.ObservedAt.UTC())
	})
}

func (w *workerImpl) finishChannelTask(ctx context.Context, row *channelWorkRow,
	task *serveroperationresourcecap.Task) error {
	return w.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		now := task.CompletedAt.UTC()
		if now.IsZero() {
			now = task.ObservedAt.UTC()
		}
		outputs, err := encodeChannelWorkerOutputs(task.SecretOutputs)
		if err != nil {
			return err
		}
		if task.Status == serveroperationresourcecap.TaskStatusSucceeded {
			if row.ActionCode == "resize" {
				if err = w.finishChannelResizeSuccess(ctx, row, task, now); err != nil {
					return err
				}
			} else if err = execOne(ctx, w.database, `UPDATE plugin_idc_core_service_entitlement
SET version=version+1,updated_at=? WHERE tenant_id=0 AND entitlement_no=? AND version=?`,
				now, row.ServiceNo, row.ExpectedServiceVersion); err != nil {
				return err
			}
		}
		businessStatus := string(task.Status)
		if task.Status == serveroperationresourcecap.TaskStatusManualReview {
			businessStatus = "manual_review"
		}
		if task.Status == serveroperationresourcecap.TaskStatusFailed {
			refunding, refundErr := w.createChannelResizeRefund(ctx, row, task, now)
			if refundErr != nil {
				return refundErr
			}
			if refunding {
				businessStatus = "refunding"
			}
		}
		completed := any(now)
		if task.Status == serveroperationresourcecap.TaskStatusManualReview {
			completed = nil
		}
		if err = execOne(ctx, w.database, `UPDATE plugin_idc_core_channel_service_operation
SET status_code=?,failure_code=?,task_evidence_digest=?,output_bindings=?::JSONB,
observed_at=?,completed_at=?,version=version+1,updated_at=?
WHERE tenant_id=0 AND operation_no=? AND status_code IN ('pending','running','result_unknown')`,
			task.Status, task.FailureCode, task.EvidenceDigest, string(outputs),
			task.ObservedAt, completed, now, row.OperationNo); err != nil {
			return err
		}
		if _, err = w.database.Exec(ctx, `UPDATE plugin_idc_core_business_operation
SET status_code=?,last_error_code=?,completed_at=?,version=version+1,updated_at=?
WHERE tenant_id=0 AND operation_no=? AND status_code IN ('executing','manual_review')`,
			businessStatus, task.FailureCode, completed, now, row.OperationNo); err != nil {
			return err
		}
		return insertChannelWorkerEvent(ctx, w.database, row.OperationNo, row.ServiceNo,
			row.BusinessStatus, businessStatus, "channel.operation.provider_reconciled",
			task.EvidenceDigest, now)
	})
}

func encodeChannelWorkerOutputs(outputs []serveroperationresourcecap.SecretOutput) ([]byte, error) {
	result := make([]channelWorkerOutput, len(outputs))
	for index, output := range outputs {
		result[index] = channelWorkerOutput{Name: output.Name, Policy: string(output.Policy),
			SecretRef: output.Handle.OpaqueRef(), ExpiresAt: output.ExpiresAt}
	}
	return json.Marshal(result)
}

func (w *workerImpl) freezeChannelOperation(ctx context.Context, operationNo, code, evidence string) error {
	return w.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		var current *struct {
			ServiceNo, ChannelStatus, BusinessStatus string
		}
		if err := w.database.GetScan(ctx, &current, `SELECT operation.service_no,
operation.status_code AS channel_status,business.status_code AS business_status
FROM plugin_idc_core_channel_service_operation operation
JOIN plugin_idc_core_business_operation business
 ON business.tenant_id=operation.tenant_id AND business.operation_no=operation.operation_no
WHERE operation.tenant_id=0 AND operation.operation_no=?
FOR UPDATE OF operation,business`, operationNo); err != nil {
			return err
		}
		if current == nil {
			return ErrOperationConflict
		}
		if current.ChannelStatus == "manual_review" && current.BusinessStatus == "manual_review" {
			return nil
		}
		if err := execOne(ctx, w.database, `UPDATE plugin_idc_core_channel_service_operation
SET status_code='manual_review',failure_code=?,task_evidence_digest=CASE WHEN ?='' THEN task_evidence_digest ELSE ? END,
version=version+1,updated_at=CURRENT_TIMESTAMP WHERE tenant_id=0 AND operation_no=?
 AND status_code IN ('scheduled','ready','pending','running','result_unknown')`,
			code, evidence, evidence, operationNo); err != nil {
			return err
		}
		if current.BusinessStatus == "manual_review" {
			return nil
		}
		if err := execOne(ctx, w.database, `UPDATE plugin_idc_core_business_operation
SET status_code='manual_review',last_error_code=?,version=version+1,updated_at=CURRENT_TIMESTAMP
WHERE tenant_id=0 AND operation_no=? AND status_code=?`,
			code, operationNo, current.BusinessStatus); err != nil {
			return err
		}
		now, err := databaseNow(ctx, w.database)
		if err != nil {
			return err
		}
		return insertChannelWorkerEvent(ctx, w.database, operationNo, current.ServiceNo,
			current.BusinessStatus, "manual_review", code, evidence, now)
	})
}

func (w *workerImpl) finishChannelResizeSuccess(ctx context.Context, row *channelWorkRow,
	task *serveroperationresourcecap.Task, now time.Time) error {
	priceNo, _ := deterministicReference("crp_", row.QuoteNo+":target")
	var tax *struct {
		PolicyNo, Treatment, Category, Scope, Mode string
		Rate                                       *int
		Quantum                                    int64
		PriceIncludesTax                           bool
	}
	if err := w.database.GetScan(ctx, &tax, `SELECT tax_policy_no AS policy_no,
tax_treatment_code AS treatment,tax_rate_basis_points AS rate,tax_category_code AS category,
rounding_scope_code AS scope,rounding_mode_code AS mode,rounding_quantum_minor AS quantum,
price_includes_tax FROM plugin_idc_core_catalog_tax_policy
WHERE tenant_id=0 AND version_no=? AND currency_code=? AND status_code='active'
ORDER BY id LIMIT 1`, row.TargetVersionNo, row.CurrencyCode); err != nil {
		return err
	}
	if tax == nil {
		return ErrOperationConflict
	}
	pricingDigest, _ := digestValue(struct {
		QuoteNo, Evidence string
		Amount            int64
	}{
		row.QuoteNo, task.EvidenceDigest, row.TargetRecurring})
	contentDigest, _ := digestValue(struct{ PriceNo, ConfigNo, Evidence string }{
		priceNo, row.TargetConfigurationNo, task.EvidenceDigest})
	componentCount := 1
	if row.TargetTax > 0 {
		componentCount++
	}
	if _, err := w.database.Exec(ctx, `INSERT INTO plugin_idc_core_service_price_snapshot(
tenant_id,price_snapshot_no,service_no,subject_type_code,subject_no,customer_no,phase_code,
product_no,version_no,billing_rule_no,configuration_snapshot_no,currency_code,purchase_cycles,
period_unit_code,period_count,renewal_mode_code,capability_evidence_no,capability_evidence_digest,
capability_evidence_observed_at,capability_evidence_expires_at,tax_policy_no,price_includes_tax,
tax_treatment_code,tax_rate_basis_points,tax_category_code,rounding_scope_code,rounding_mode_code,
rounding_quantum_minor,component_count,subtotal_minor,discount_minor,net_amount_minor,tax_amount_minor,
total_amount_minor,pricing_rule_digest,content_digest)
SELECT 0,?,NULL,'entitlement',?,?, 'resize_target',product_no,?,?,?,?,1,
entitlement.period_unit_code,entitlement.period_count,'provider_confirmed',?,?,?,?,?,?,?,?,?,?,?,?,?, ?,0,?,?,?,?,?
FROM plugin_idc_core_catalog_product_version version
JOIN plugin_idc_core_service_entitlement entitlement ON entitlement.tenant_id=version.tenant_id
 AND entitlement.entitlement_no=?
WHERE version.tenant_id=0 AND version.version_no=?`, priceNo, row.ServiceNo,
		row.OwnerCustomerNo, row.TargetVersionNo, row.TargetBillingRuleNo,
		row.TargetConfigurationNo, row.CurrencyCode, task.CapabilityEvidenceNo,
		task.CapabilityEvidenceDigest, task.ObservedAt, task.ObservedAt.Add(24*time.Hour),
		tax.PolicyNo, tax.PriceIncludesTax, tax.Treatment, tax.Rate, tax.Category, tax.Scope,
		tax.Mode, tax.Quantum, componentCount, row.TargetSubtotal, row.TargetNet,
		row.TargetTax, row.TargetRecurring, pricingDigest, contentDigest,
		row.ServiceNo, row.TargetVersionNo); err != nil {
		return gerror.Wrap(err, "insert channel resize target price")
	}
	baseNo, _ := deterministicReference("crm_", priceNo+":base")
	if _, err := w.database.Exec(ctx, `INSERT INTO plugin_idc_core_service_price_component_snapshot(
tenant_id,component_snapshot_no,price_snapshot_no,service_no,subject_type_code,subject_no,
line_number,component_type_code,charge_phase_code,source_reference,quantity,
unit_amount_minor,amount_minor,source_digest,content_digest)
VALUES(0,?,?,NULL,'entitlement',?,1,'base','periodic',?,1,?,?,?,?)`, baseNo,
		priceNo, row.ServiceNo, row.QuoteNo, row.TargetNet, row.TargetNet,
		pricingDigest, contentDigest); err != nil {
		return err
	}
	if row.TargetTax > 0 {
		taxNo, _ := deterministicReference("crm_", priceNo+":tax")
		if _, err := w.database.Exec(ctx, `INSERT INTO plugin_idc_core_service_price_component_snapshot(
tenant_id,component_snapshot_no,price_snapshot_no,service_no,subject_type_code,subject_no,
line_number,component_type_code,charge_phase_code,source_reference,quantity,
unit_amount_minor,amount_minor,source_digest,content_digest)
VALUES(0,?,?,NULL,'entitlement',?,2,'tax','total',?,1,?,?,?,?)`, taxNo, priceNo,
			row.ServiceNo, tax.PolicyNo, row.TargetTax, row.TargetTax,
			pricingDigest, contentDigest); err != nil {
			return err
		}
	}
	return execOne(ctx, w.database, `UPDATE plugin_idc_core_service_entitlement
SET version_no=?,billing_rule_no=?,current_price_snapshot_no=?,version=version+1,updated_at=?
WHERE tenant_id=0 AND entitlement_no=? AND version=?`, row.TargetVersionNo,
		row.TargetBillingRuleNo, priceNo, now, row.ServiceNo, row.ExpectedServiceVersion)
}

func (w *workerImpl) createChannelResizeRefund(ctx context.Context, row *channelWorkRow,
	task *serveroperationresourcecap.Task, now time.Time) (bool, error) {
	if row.ActionCode != "resize" {
		return false, nil
	}
	var paid *struct {
		OrderNo, InvoiceNo, CustomerNo, CurrencyCode string
		Amount                                       int64
	}
	if err := w.database.GetScan(ctx, &paid, `SELECT COALESCE(operation.order_no,'') AS order_no,
COALESCE(operation.invoice_no,'') AS invoice_no,operation.payer_customer_no AS customer_no,
operation.currency_code,COALESCE(invoice.paid_amount_minor,0) AS amount
FROM plugin_idc_core_business_operation operation
LEFT JOIN plugin_idc_core_billing_invoice invoice
 ON invoice.tenant_id=operation.tenant_id AND invoice.invoice_no=operation.invoice_no
WHERE operation.tenant_id=0 AND operation.operation_no=?`, row.OperationNo); err != nil {
		return false, err
	}
	if paid == nil || paid.OrderNo == "" || paid.Amount <= 0 {
		return false, nil
	}
	handoffNo, _ := deterministicReference("rrh_", row.OperationNo)
	evidence := task.EvidenceDigest
	if evidence == "" {
		evidence, _ = digestValue(struct{ OperationNo, Failure string }{row.OperationNo, task.FailureCode})
	}
	if _, err := w.database.Exec(ctx, `INSERT INTO plugin_idc_core_server_resize_refund_handoff(
tenant_id,handoff_no,operation_no,order_no,invoice_no,customer_no,currency_code,amount_minor,
status_code,reason_code,evidence_digest,created_at,updated_at)
VALUES(0,?,?,?,?,?,?,?,'ready','channel_resize.provider_failed',?,?,?)
ON CONFLICT (operation_no) DO NOTHING`, handoffNo, row.OperationNo, paid.OrderNo,
		paid.InvoiceNo, paid.CustomerNo, paid.CurrencyCode, paid.Amount, evidence, now, now); err != nil {
		return false, err
	}
	_, err := w.database.Exec(ctx, `UPDATE plugin_idc_core_commerce_order
SET status_code='refund_pending',version=version+1,updated_at=? WHERE tenant_id=0 AND order_no=?
 AND status_code IN ('paid','fulfilling','partially_fulfilled','fulfilled')`, now, paid.OrderNo)
	return true, err
}

func insertChannelWorkerEvent(ctx context.Context, database gdb.DB,
	operationNo, serviceNo, from, to, reason, evidence string, now time.Time) error {
	var next *struct {
		Sequence int
	}
	if err := database.GetScan(ctx, &next, `SELECT COALESCE(MAX(event_sequence),0)::INT+1 AS sequence
FROM plugin_idc_core_business_operation_event WHERE tenant_id=0 AND operation_no=?`, operationNo); err != nil {
		return err
	}
	if next == nil || next.Sequence < 1 {
		return ErrOperationConflict
	}
	sequence := next.Sequence
	eventNo, _ := deterministicReference("cwe_", operationNo+":"+strconv.Itoa(sequence))
	content, _ := digestValue(struct {
		OperationNo, From, To, Evidence string
		Sequence                        int
	}{
		operationNo, from, to, evidence, sequence})
	_, err := database.Exec(ctx, `INSERT INTO plugin_idc_core_business_operation_event(
tenant_id,event_no,operation_no,service_no,subject_type_code,subject_no,event_sequence,
from_status_code,to_status_code,reason_code,actor_type_code,actor_reference,
evidence_digest,content_digest,created_at)
VALUES(0,?,?,NULL,'entitlement',?,?,?,?,?,'system','server-operation-worker',?,?,?)`,
		eventNo, operationNo, serviceNo, sequence, from, to, reason, evidence, content, now)
	return err
}
