package serveroperation

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-plugin-idc-core/backend/cap/secretcap"
	"lina-plugin-idc-core/backend/cap/serveroperationresourcecap"
)

type workRow struct {
	ExecutionNo              string
	OperationNo              string
	ServiceNo                string
	ActionCode               string
	ChannelNo                string
	PoolNo                   string
	ExpectedServiceVersion   int64
	OriginalRuntimeStatus    string `orm:"original_runtime_status_code"`
	Parameters               string
	SecretBindings           string
	ExecutionStatus          string `orm:"execution_status_code"`
	ProviderTaskNo           *string
	RequestDigest            string
	CapabilityEvidenceNo     *string
	CapabilityEvidenceDigest *string
	TaskEvidenceDigest       *string
	ObservedAt               *time.Time
	ExecutionVersion         int64
	OperationStatus          string
	OperationVersion         int64
	ServiceRuntimeStatus     string
	ProviderTaskStatus       string
	ServiceVersion           int64
}

type persistedSecretOutput struct {
	Name      string    `json:"name"`
	Policy    string    `json:"policy"`
	Ref       string    `json:"ref"`
	ExpiresAt time.Time `json:"expiresAt,omitempty"`
}

func (w *workerImpl) Run(ctx context.Context, limit int) (*Batch, error) {
	if w == nil || w.database == nil || w.source == nil || ctx == nil || ctx.Err() != nil ||
		limit < 1 || limit > maximumBatch {
		return nil, ErrInvalidInput
	}
	if err := w.materializeDueResizeDowngrades(ctx, limit); err != nil {
		return nil, err
	}
	channelSelected, channelDispatched, channelReconciled, channelManual,
		channelFailures, channelErr := w.processChannelPricedOperations(ctx, limit)
	remaining := limit - channelSelected
	var rows []workRow
	var err error
	if remaining > 0 {
		rows, err = loadWork(ctx, w.database, remaining)
		if err != nil {
			return nil, err
		}
	}
	report := &Batch{Selected: channelSelected + len(rows),
		Dispatched: channelDispatched, Reconciled: channelReconciled,
		ManualReview: channelManual, Failures: channelFailures}
	batchErr := channelErr
	for index := range rows {
		if ctx.Err() != nil {
			return report, ctx.Err()
		}
		row := &rows[index]
		request, requestErr := buildRequest(row)
		if requestErr != nil {
			if w.moveToManualReview(ctx, row.OperationNo, "server_operation.intent_invalid", "") != nil {
				report.Failures++
			} else {
				report.ManualReview++
			}
			continue
		}
		if row.ExecutionStatus == "ready" {
			manual, dispatchErr := w.dispatch(ctx, row, request)
			if dispatchErr != nil {
				report.Failures++
			} else if manual {
				report.ManualReview++
			} else {
				report.Dispatched++
			}
			continue
		}
		manual, reconcileErr := w.reconcile(ctx, row, request)
		if reconcileErr != nil {
			report.Failures++
		} else if manual {
			report.ManualReview++
		} else {
			report.Reconciled++
		}
	}
	bound, manual, bindingErr := w.reconcileAccessoryBindings(ctx, limit)
	report.Bound += bound
	report.ManualReview += manual
	if bindingErr != nil {
		report.Failures++
		batchErr = bindingErr
	}
	renewed, paymentPending, renewalErr := w.reconcileAccessoryRenewals(ctx, limit)
	report.Renewed += renewed
	report.PaymentPending += paymentPending
	if renewalErr != nil {
		report.Failures++
		batchErr = renewalErr
	}
	ipReconciled, ipManual, ipErr := w.reconcileIPReservations(ctx, limit)
	report.Reconciled += ipReconciled
	report.ManualReview += ipManual
	if ipErr != nil {
		report.Failures++
		batchErr = ipErr
	}
	if report.Failures > 0 {
		if batchErr != nil {
			return report, gerror.Wrap(batchErr, "one or more server operations remain unavailable")
		}
		return report, gerror.New("one or more server operations remain unavailable")
	}
	return report, nil
}

func (w *workerImpl) dispatch(ctx context.Context, row *workRow,
	request serveroperationresourcecap.TaskRequest) (bool, error) {
	task, err := w.source.CreateTask(ctx, request)
	if err != nil {
		if errors.Is(err, serveroperationresourcecap.ErrCapabilityUnavailable) ||
			errors.Is(err, serveroperationresourcecap.ErrTaskConflict) {
			return true, w.moveToManualReview(ctx, row.OperationNo,
				"server_operation.capability_unavailable", "")
		}
		return false, err
	}
	if task == nil || serveroperationresourcecap.ValidateTaskForRequest(*task, request) != nil {
		return false, serveroperationresourcecap.ErrSourceUnavailable
	}
	if err = w.attachTask(ctx, row.OperationNo, request, task); err != nil {
		return false, err
	}
	if task.Status == serveroperationresourcecap.TaskStatusPending ||
		task.Status == serveroperationresourcecap.TaskStatusRunning {
		return false, nil
	}
	return w.converge(ctx, row.OperationNo, request, task)
}

func (w *workerImpl) reconcile(ctx context.Context, row *workRow,
	request serveroperationresourcecap.TaskRequest) (bool, error) {
	if row.ProviderTaskNo == nil {
		return true, w.moveToManualReview(ctx, row.OperationNo,
			"server_operation.task_missing", "")
	}
	task, err := w.source.ReadTask(ctx, serveroperationresourcecap.TaskReadRequest{
		ProviderTaskNo: *row.ProviderTaskNo, Request: request,
	})
	if err != nil {
		if errors.Is(err, serveroperationresourcecap.ErrTaskConflict) ||
			errors.Is(err, serveroperationresourcecap.ErrTaskUnavailable) {
			return true, w.moveToManualReview(ctx, row.OperationNo,
				"server_operation.task_unavailable", "")
		}
		return false, err
	}
	if task == nil || serveroperationresourcecap.ValidateTaskForRequest(*task, request) != nil {
		return false, serveroperationresourcecap.ErrSourceUnavailable
	}
	return w.converge(ctx, row.OperationNo, request, task)
}

func (w *workerImpl) converge(ctx context.Context, operationNo string,
	request serveroperationresourcecap.TaskRequest, task *serveroperationresourcecap.Task) (bool, error) {
	row, err := loadWorkByOperation(ctx, w.database, operationNo)
	if err != nil {
		return false, err
	}
	if row == nil || validateTaskIdentity(row, request, task) != nil {
		return true, w.moveToManualReview(ctx, operationNo,
			"server_operation.task_mismatch", task.EvidenceDigest)
	}
	if row.ObservedAt != nil && databaseTime(task.ObservedAt).Before(databaseTime(*row.ObservedAt)) {
		return true, w.moveToManualReview(ctx, operationNo,
			"server_operation.task_regressed", task.EvidenceDigest)
	}
	switch task.Status {
	case serveroperationresourcecap.TaskStatusPending,
		serveroperationresourcecap.TaskStatusRunning,
		serveroperationresourcecap.TaskStatusResultUnknown:
		return false, w.observeTask(ctx, operationNo, request, task)
	case serveroperationresourcecap.TaskStatusSucceeded,
		serveroperationresourcecap.TaskStatusFailed,
		serveroperationresourcecap.TaskStatusManualReview:
		manual := task.Status == serveroperationresourcecap.TaskStatusManualReview
		return manual, w.finishTask(ctx, operationNo, request, task)
	default:
		return false, serveroperationresourcecap.ErrSourceUnavailable
	}
}

func loadWork(ctx context.Context, database gdb.DB, limit int) ([]workRow, error) {
	var rows []workRow
	if err := database.GetScan(ctx, &rows, workSelect+`
WHERE execution.tenant_id = 0
  AND execution.status_code IN ('ready', 'pending', 'running', 'result_unknown')
ORDER BY execution.updated_at, execution.id LIMIT ?`, limit); err != nil {
		return nil, gerror.Wrap(err, "list server operation work")
	}
	return rows, nil
}

func loadWorkByOperation(ctx context.Context, database gdb.DB, operationNo string) (*workRow, error) {
	var row *workRow
	if err := database.GetScan(ctx, &row, workSelect+`
WHERE execution.tenant_id = 0 AND execution.operation_no = ?`, operationNo); err != nil {
		return nil, gerror.Wrap(err, "load server operation work")
	}
	return row, nil
}

const workSelect = `SELECT execution.execution_no, execution.operation_no,
execution.service_no, execution.action_code, execution.channel_no, execution.pool_no,
execution.expected_service_version, execution.original_runtime_status_code,
execution.parameters::TEXT AS parameters,
execution.secret_bindings::TEXT AS secret_bindings,
execution.status_code AS execution_status_code, execution.provider_task_no,
execution.request_digest, execution.capability_evidence_no,
execution.capability_evidence_digest, execution.task_evidence_digest,
execution.observed_at, execution.version AS execution_version,
operation.status_code AS operation_status, operation.version AS operation_version,
service.runtime_status_code AS service_runtime_status,
service.provider_task_status_code AS provider_task_status,
service.version AS service_version
FROM plugin_idc_core_server_operation_execution execution
JOIN plugin_idc_core_business_operation operation
 ON operation.tenant_id = execution.tenant_id AND operation.operation_no = execution.operation_no
JOIN plugin_idc_core_service_instance service
 ON service.tenant_id = execution.tenant_id AND service.service_no = execution.service_no `

func buildRequest(row *workRow) (serveroperationresourcecap.TaskRequest, error) {
	if row == nil {
		return serveroperationresourcecap.TaskRequest{}, ErrInvalidInput
	}
	var persisted []persistedSecretBinding
	if json.Unmarshal([]byte(row.SecretBindings), &persisted) != nil {
		return serveroperationresourcecap.TaskRequest{}, ErrInvalidInput
	}
	bindings := make([]serveroperationresourcecap.SecretBinding, len(persisted))
	for index, item := range persisted {
		handle, err := secretcap.NewHandle(item.Ref)
		if err != nil {
			return serveroperationresourcecap.TaskRequest{}, ErrInvalidInput
		}
		bindings[index] = serveroperationresourcecap.SecretBinding{Name: item.Name, Handle: handle}
	}
	request := serveroperationresourcecap.TaskRequest{OperationNo: row.OperationNo,
		ServiceNo: row.ServiceNo, ChannelNo: row.ChannelNo, PoolNo: row.PoolNo,
		Action:                 serveroperationresourcecap.Action(row.ActionCode),
		ExpectedServiceVersion: row.ExpectedServiceVersion,
		Parameters:             json.RawMessage(row.Parameters), SecretBindings: bindings,
		RequestDigest: row.RequestDigest}
	if serveroperationresourcecap.ValidateTaskRequest(request) != nil {
		return serveroperationresourcecap.TaskRequest{}, ErrInvalidInput
	}
	return request, nil
}

func validateTaskIdentity(row *workRow, request serveroperationresourcecap.TaskRequest,
	task *serveroperationresourcecap.Task) error {
	if row == nil || task == nil || row.ProviderTaskNo == nil ||
		*row.ProviderTaskNo != task.ProviderTaskNo || row.RequestDigest != request.RequestDigest ||
		row.CapabilityEvidenceNo == nil || *row.CapabilityEvidenceNo != task.CapabilityEvidenceNo ||
		row.CapabilityEvidenceDigest == nil ||
		*row.CapabilityEvidenceDigest != task.CapabilityEvidenceDigest {
		return ErrOperationConflict
	}
	return nil
}

func (w *workerImpl) attachTask(ctx context.Context, operationNo string,
	request serveroperationresourcecap.TaskRequest, task *serveroperationresourcecap.Task) error {
	return w.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		row, err := loadLockedWork(ctx, w.database, operationNo)
		if err != nil || row == nil {
			return errors.Join(ErrOperationConflict, err)
		}
		if row.ExecutionStatus != "ready" || row.OperationStatus != "ready" ||
			row.ServiceVersion != row.ExpectedServiceVersion ||
			row.ServiceRuntimeStatus != row.OriginalRuntimeStatus {
			return ErrOperationConflict
		}
		now, err := databaseNow(ctx, w.database)
		if err != nil {
			return err
		}
		status, providerStatus := "pending", "pending"
		if task.Status == serveroperationresourcecap.TaskStatusRunning {
			status, providerStatus = "running", "executing"
		}
		if err = execOne(ctx, w.database, `UPDATE plugin_idc_core_server_operation_execution
SET status_code = ?, provider_task_no = ?, capability_evidence_no = ?,
capability_evidence_digest = ?, task_evidence_digest = ?, observed_at = ?,
version = version + 1, updated_at = ?
WHERE tenant_id = 0 AND operation_no = ? AND status_code = 'ready' AND version = ?`,
			status, task.ProviderTaskNo, task.CapabilityEvidenceNo,
			task.CapabilityEvidenceDigest, task.EvidenceDigest, databaseTime(task.ObservedAt), now,
			operationNo, row.ExecutionVersion); err != nil {
			return gerror.Wrap(err, "attach server operation task")
		}
		if err = execOne(ctx, w.database, `UPDATE plugin_idc_core_business_operation
SET status_code = 'executing', execution_started_at = ?, version = version + 1, updated_at = ?
WHERE tenant_id = 0 AND operation_no = ? AND status_code = 'ready' AND version = ?`,
			now, now, operationNo, row.OperationVersion); err != nil {
			return gerror.Wrap(err, "start server business operation")
		}
		if row.ActionCode == string(serveroperationresourcecap.ActionResize) {
			if err = execOne(ctx, w.database, `UPDATE plugin_idc_core_server_resize_intent
SET status_code='executing',updated_at=? WHERE tenant_id=0 AND operation_no=? AND status_code='ready'`, now, operationNo); err != nil {
				return gerror.Wrap(err, "start server resize intent")
			}
		}
		runtimeStatus := transitionalRuntime(row.ActionCode, row.OriginalRuntimeStatus)
		if err = execOne(ctx, w.database, `UPDATE plugin_idc_core_service_instance
SET runtime_status_code = ?, provider_task_status_code = ?,
version = version + 1, updated_at = ?
WHERE tenant_id = 0 AND service_no = ? AND version = ?`, runtimeStatus,
			providerStatus, now, row.ServiceNo, row.ServiceVersion); err != nil {
			return gerror.Wrap(err, "start server operation projection")
		}
		from := "ready"
		sequence, err := nextOperationEventSequence(ctx, w.database, operationNo)
		if err != nil {
			return err
		}
		return insertEvent(ctx, w.database, operationNo, row.ServiceNo, sequence, &from,
			"executing", "server_operation.task_created", "system", "server-operation-worker",
			task.EvidenceDigest, now)
	})
}

func loadLockedWork(ctx context.Context, database gdb.DB, operationNo string) (*workRow, error) {
	var row *workRow
	if err := database.GetScan(ctx, &row, workSelect+`
WHERE execution.tenant_id = 0 AND execution.operation_no = ?
FOR UPDATE OF execution, operation, service`, operationNo); err != nil {
		return nil, gerror.Wrap(err, "lock server operation work")
	}
	return row, nil
}

func (w *workerImpl) observeTask(ctx context.Context, operationNo string,
	request serveroperationresourcecap.TaskRequest, task *serveroperationresourcecap.Task) error {
	return w.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		row, err := loadLockedWork(ctx, w.database, operationNo)
		if err != nil || row == nil || validateTaskIdentity(row, request, task) != nil ||
			row.OperationStatus != "executing" {
			return errors.Join(ErrOperationConflict, err)
		}
		if row.ObservedAt != nil && databaseTime(task.ObservedAt).Before(databaseTime(*row.ObservedAt)) {
			return ErrOperationConflict
		}
		now, err := databaseNow(ctx, w.database)
		if err != nil {
			return err
		}
		providerStatus := "pending"
		if task.Status == serveroperationresourcecap.TaskStatusRunning {
			providerStatus = "executing"
		} else if task.Status == serveroperationresourcecap.TaskStatusResultUnknown {
			providerStatus = "result_unknown"
		}
		if err = execOne(ctx, w.database, `UPDATE plugin_idc_core_server_operation_execution
SET status_code = ?, task_evidence_digest = ?, last_failure_code = ?, observed_at = ?,
version = version + 1, updated_at = ?
WHERE tenant_id = 0 AND operation_no = ? AND version = ?
  AND status_code IN ('pending', 'running', 'result_unknown')`, task.Status,
			task.EvidenceDigest, task.FailureCode, databaseTime(task.ObservedAt), now,
			operationNo, row.ExecutionVersion); err != nil {
			return gerror.Wrap(err, "observe server operation task")
		}
		runtimeStatus := transitionalRuntime(row.ActionCode, row.OriginalRuntimeStatus)
		if task.Status == serveroperationresourcecap.TaskStatusResultUnknown &&
			(row.ActionCode == "start" || row.ActionCode == "stop" || row.ActionCode == "reinstall") {
			runtimeStatus = "unknown"
		}
		if err = execOne(ctx, w.database, `UPDATE plugin_idc_core_service_instance
SET runtime_status_code = ?, provider_task_status_code = ?, version = version + 1, updated_at = ?
WHERE tenant_id = 0 AND service_no = ? AND version = ?`, runtimeStatus,
			providerStatus, now, row.ServiceNo, row.ServiceVersion); err != nil {
			return gerror.Wrap(err, "observe server operation projection")
		}
		if row.ActionCode == string(serveroperationresourcecap.ActionChangeIP) &&
			task.Status == serveroperationresourcecap.TaskStatusResultUnknown {
			if err = execOne(ctx, w.database, `UPDATE plugin_idc_core_server_ip_change_intent
SET reservation_status_code='unknown',updated_at=?
WHERE tenant_id=0 AND operation_no=? AND reservation_status_code='reserved'`, now, operationNo); err != nil {
				return gerror.Wrap(err, "retain IP reservation for unknown result")
			}
		}
		if row.ActionCode == string(serveroperationresourcecap.ActionResize) &&
			task.Status == serveroperationresourcecap.TaskStatusResultUnknown {
			if err = execOne(ctx, w.database, `UPDATE plugin_idc_core_server_resize_intent
SET status_code='result_unknown',updated_at=? WHERE tenant_id=0 AND operation_no=? AND status_code IN ('executing','result_unknown')`, now, operationNo); err != nil {
				return gerror.Wrap(err, "freeze unknown server resize result")
			}
		}
		return nil
	})
}

func (w *workerImpl) finishTask(ctx context.Context, operationNo string,
	request serveroperationresourcecap.TaskRequest, task *serveroperationresourcecap.Task) error {
	return w.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		row, err := loadLockedWork(ctx, w.database, operationNo)
		if err != nil || row == nil || validateTaskIdentity(row, request, task) != nil ||
			row.OperationStatus != "executing" || task.CompletedAt.IsZero() {
			return errors.Join(ErrOperationConflict, err)
		}
		now, err := databaseNow(ctx, w.database)
		if err != nil {
			return err
		}
		operationStatus, providerStatus, runtimeStatus := "manual_review", "manual_review",
			manualRuntime(row.ActionCode, row.OriginalRuntimeStatus)
		failureCode, reason := task.FailureCode, "server_operation.manual_review"
		if task.Status == serveroperationresourcecap.TaskStatusSucceeded {
			operationStatus, providerStatus = "succeeded", "succeeded"
			runtimeStatus, failureCode, reason = successfulRuntime(row.ActionCode,
				row.OriginalRuntimeStatus), "", "server_operation.succeeded"
		} else if task.Status == serveroperationresourcecap.TaskStatusFailed {
			operationStatus, providerStatus, runtimeStatus = "failed", "failed", row.OriginalRuntimeStatus
			reason = "server_operation.failed"
		}
		if task.Status == serveroperationresourcecap.TaskStatusSucceeded {
			if err = markDeletedAccessoryEntitlement(ctx, w.database, row, request, task, now); err != nil {
				return err
			}
		}
		if failureCode == "" && operationStatus == "manual_review" {
			failureCode = "server_operation.manual_review"
		}
		outputs, err := encodeOutputs(task.SecretOutputs)
		if err != nil {
			return err
		}
		if err = execOne(ctx, w.database, `UPDATE plugin_idc_core_server_operation_execution
SET status_code = ?, task_evidence_digest = ?, last_failure_code = ?,
output_bindings = ?::JSONB, observed_at = ?, completed_at = ?,
version = version + 1, updated_at = ?
WHERE tenant_id = 0 AND operation_no = ? AND version = ?
  AND status_code IN ('pending', 'running', 'result_unknown')`, task.Status,
			task.EvidenceDigest, failureCode, string(outputs), databaseTime(task.ObservedAt),
			databaseTime(task.CompletedAt), now, operationNo, row.ExecutionVersion); err != nil {
			return gerror.Wrap(err, "finish server operation execution")
		}
		if err = execOne(ctx, w.database, `UPDATE plugin_idc_core_business_operation
SET status_code = ?, last_error_code = ?, completed_at = ?,
version = version + 1, updated_at = ?
WHERE tenant_id = 0 AND operation_no = ? AND status_code = 'executing' AND version = ?`,
			operationStatus, failureCode, databaseTime(task.CompletedAt), now, operationNo,
			row.OperationVersion); err != nil {
			return gerror.Wrap(err, "finish server business operation")
		}
		if row.ActionCode == string(serveroperationresourcecap.ActionResize) && task.Status == serveroperationresourcecap.TaskStatusSucceeded {
			if err = w.finishResizeSuccess(ctx, row, task, runtimeStatus, providerStatus, now); err != nil {
				return err
			}
		} else if err = execOne(ctx, w.database, `UPDATE plugin_idc_core_service_instance
SET runtime_status_code = ?, provider_task_status_code = ?, version = version + 1, updated_at = ?
WHERE tenant_id = 0 AND service_no = ? AND version = ?`, runtimeStatus,
			providerStatus, now, row.ServiceNo, row.ServiceVersion); err != nil {
			return gerror.Wrap(err, "finish server operation projection")
		}
		if row.ActionCode == string(serveroperationresourcecap.ActionResize) && task.Status != serveroperationresourcecap.TaskStatusSucceeded {
			var refunding bool
			if refunding, err = w.finishResizeFailure(ctx, row, task, now); err != nil {
				return err
			}
			if refunding {
				operationStatus = "refunding"
				reason = "server_resize.refund_pending"
			}
		}
		from := "executing"
		if row.ActionCode == string(serveroperationresourcecap.ActionChangeIP) &&
			task.Status == serveroperationresourcecap.TaskStatusManualReview {
			if err = execOne(ctx, w.database, `UPDATE plugin_idc_core_server_ip_change_intent
SET reservation_status_code='unknown',updated_at=?
WHERE tenant_id=0 AND operation_no=? AND reservation_status_code='reserved'`, now, operationNo); err != nil {
				return gerror.Wrap(err, "freeze IP reservation for manual review")
			}
		}
		sequence, err := nextOperationEventSequence(ctx, w.database, operationNo)
		if err != nil {
			return err
		}
		return insertEvent(ctx, w.database, operationNo, row.ServiceNo, sequence, &from,
			operationStatus, reason, "system", "server-operation-worker", task.EvidenceDigest, now)
	})
}

func (w *workerImpl) moveToManualReview(ctx context.Context, operationNo, failureCode, evidence string) error {
	return w.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		row, err := loadLockedWork(ctx, w.database, operationNo)
		if err != nil || row == nil {
			return errors.Join(ErrOperationConflict, err)
		}
		if row.ExecutionStatus == "manual_review" {
			return nil
		}
		now, err := databaseNow(ctx, w.database)
		if err != nil {
			return err
		}
		if evidence == "" {
			evidence, err = digestValue(struct{ OperationNo, Failure string }{operationNo, failureCode})
			if err != nil {
				return err
			}
		}
		if err = execOne(ctx, w.database, `UPDATE plugin_idc_core_server_operation_execution
SET status_code = 'manual_review', last_failure_code = ?, task_evidence_digest =
CASE WHEN provider_task_no IS NULL THEN NULL ELSE ? END,
observed_at = CASE WHEN provider_task_no IS NULL THEN NULL ELSE COALESCE(observed_at, ?) END,
completed_at = ?, version = version + 1, updated_at = ?
WHERE tenant_id = 0 AND operation_no = ? AND version = ?`, failureCode, evidence,
			now, now, now, operationNo, row.ExecutionVersion); err != nil {
			return gerror.Wrap(err, "move server operation execution to manual review")
		}
		if err = execOne(ctx, w.database, `UPDATE plugin_idc_core_business_operation
SET status_code = 'manual_review', last_error_code = ?,
execution_started_at = COALESCE(execution_started_at, ?),
version = version + 1, updated_at = ?
WHERE tenant_id = 0 AND operation_no = ? AND version = ?`, failureCode, now, now,
			operationNo, row.OperationVersion); err != nil {
			return gerror.Wrap(err, "move server business operation to manual review")
		}
		if row.ProviderTaskNo != nil {
			if err = execOne(ctx, w.database, `UPDATE plugin_idc_core_service_instance
SET runtime_status_code = ?, provider_task_status_code = 'manual_review',
version = version + 1, updated_at = ?
WHERE tenant_id = 0 AND service_no = ? AND version = ?`,
				manualRuntime(row.ActionCode, row.OriginalRuntimeStatus), now,
				row.ServiceNo, row.ServiceVersion); err != nil {
				return gerror.Wrap(err, "move server operation projection to manual review")
			}
		}
		from := row.OperationStatus
		sequence, err := nextOperationEventSequence(ctx, w.database, operationNo)
		if err != nil {
			return err
		}
		return insertEvent(ctx, w.database, operationNo, row.ServiceNo, sequence, &from,
			"manual_review", "server_operation.manual_review", "system",
			"server-operation-worker", evidence, now)
	})
}

func nextOperationEventSequence(ctx context.Context, database gdb.DB, operationNo string) (int, error) {
	value, err := database.GetValue(ctx, `SELECT COALESCE(MAX(event_sequence),0)+1
FROM plugin_idc_core_business_operation_event
WHERE tenant_id=0 AND operation_no=?`, operationNo)
	if err != nil {
		return 0, gerror.Wrap(err, "allocate server operation event sequence")
	}
	sequence := value.Int()
	if sequence < 1 || sequence > 1000000 {
		return 0, ErrOperationConflict
	}
	return sequence, nil
}

func encodeOutputs(outputs []serveroperationresourcecap.SecretOutput) ([]byte, error) {
	persisted := make([]persistedSecretOutput, len(outputs))
	for index, output := range outputs {
		persisted[index] = persistedSecretOutput{Name: output.Name, Policy: string(output.Policy),
			Ref: output.Handle.OpaqueRef(), ExpiresAt: output.ExpiresAt}
	}
	encoded, err := json.Marshal(persisted)
	if err != nil {
		return nil, gerror.Wrap(err, "encode server operation confidential outputs")
	}
	return encoded, nil
}

func transitionalRuntime(action, original string) string {
	switch action {
	case "start":
		return "resuming"
	case "stop", "reboot", "reinstall":
		return "changing"
	default:
		return original
	}
}

func successfulRuntime(action, original string) string {
	switch action {
	case "start", "reboot", "reinstall":
		return "running"
	case "stop":
		return "stopped"
	default:
		return original
	}
}

func manualRuntime(action, original string) string {
	if action == "start" || action == "stop" || action == "reinstall" {
		return "unknown"
	}
	return original
}

func databaseTime(value time.Time) time.Time {
	return value.UTC().Truncate(time.Microsecond)
}
