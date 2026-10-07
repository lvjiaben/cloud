package notification

import (
	"context"
	"encoding/json"
	"strconv"

	"github.com/gogf/gf/v2/database/gdb"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/secretcap"
	"lina-plugin-idc-notify/backend/cap/channelcap"
	modeldo "lina-plugin-idc-notify/backend/internal/model/do"
)

func (s *serviceImpl) ProcessQueries(ctx context.Context, limit int) error {
	if limit <= 0 || limit > maximumWorkerBatch {
		return bizerr.NewCode(CodeInvalid)
	}
	expired, err := s.expireQueries(ctx, limit)
	if err != nil {
		return publicError(err)
	}
	for index := range expired {
		if err = s.appendDeliveryAudit(ctx, expired[index].TaskNo, "query_expired", "system", 0,
			"provider_query_deadline", expired[index].StatusCode, string(DeliveryManualReview), expired[index].Version); err != nil {
			return publicError(err)
		}
	}
	rows, err := s.claimQueries(ctx, limit)
	if err != nil {
		return publicError(err)
	}
	for index := range rows {
		if err = s.processQuery(ctx, &rows[index]); err != nil {
			return publicError(err)
		}
	}
	return nil
}

func (s *serviceImpl) expireQueries(ctx context.Context, limit int) ([]deliveryRow, error) {
	var rows []deliveryRow
	err := s.database.GetScan(ctx, &rows, `
WITH expired AS (
    SELECT tenant_id,task_no FROM plugin_idc_notify_delivery_task
    WHERE tenant_id=? AND status_code IN ('submitted','result_unknown') AND external_boundary_crossed=TRUE
      AND provider_query_deadline IS NOT NULL AND provider_query_deadline <= CURRENT_TIMESTAMP
    ORDER BY provider_query_deadline,task_no FOR UPDATE SKIP LOCKED LIMIT ?
)
UPDATE plugin_idc_notify_delivery_task t
SET status_code='manual_review',updated_at=CURRENT_TIMESTAMP,version=version+1,
    attempt_lease=NULL,attempt_lease_until=NULL
FROM expired
WHERE t.tenant_id=expired.tenant_id AND t.task_no=expired.task_no
RETURNING t.*`, notificationTenantID, limit)
	return rows, err
}

func (s *serviceImpl) claimQueries(ctx context.Context, limit int) ([]deliveryRow, error) {
	leaseID := stableNumber("nquery_", strconv.FormatInt(s.now().UTC().UnixNano(), 10))
	var rows []deliveryRow
	err := s.database.GetScan(ctx, &rows, `
WITH due AS (
    SELECT tenant_id,task_no FROM plugin_idc_notify_delivery_task
    WHERE tenant_id=? AND status_code IN ('submitted','result_unknown') AND external_boundary_crossed=TRUE
      AND provider_query_deadline > CURRENT_TIMESTAMP
      AND updated_at <= CURRENT_TIMESTAMP - INTERVAL '1 minute'
      AND (attempt_lease_until IS NULL OR attempt_lease_until < CURRENT_TIMESTAMP)
    ORDER BY updated_at,task_no FOR UPDATE SKIP LOCKED LIMIT ?
)
UPDATE plugin_idc_notify_delivery_task t
SET attempt_lease=?,attempt_lease_until=CURRENT_TIMESTAMP + INTERVAL '2 minutes',updated_at=CURRENT_TIMESTAMP
FROM due
WHERE t.tenant_id=due.tenant_id AND t.task_no=due.task_no
RETURNING t.*`, notificationTenantID, limit, leaseID)
	return rows, err
}

func (s *serviceImpl) processQuery(ctx context.Context, task *deliveryRow) error {
	if task == nil || task.ProviderMessageRef == nil || *task.ProviderMessageRef == "" {
		return s.moveQueryToManualReview(ctx, task, "provider_reference_unavailable")
	}
	channel, err := s.loadDeliveryChannel(ctx, task.ChannelCode)
	if err != nil || channel == nil || channel.ProviderCode == nil || channel.CredentialSecretRef == nil {
		return s.moveQueryToManualReview(ctx, task, "query_configuration_unavailable")
	}
	binding, exists := s.adapters[*channel.ProviderCode]
	if !exists {
		return s.moveQueryToManualReview(ctx, task, "query_adapter_unavailable")
	}
	handle, err := secretcap.NewHandle(*channel.CredentialSecretRef)
	if err != nil {
		return s.moveQueryToManualReview(ctx, task, "query_credential_unavailable")
	}
	configuration := make(map[string]string)
	if json.Unmarshal([]byte(channel.NonSecretConfiguration), &configuration) != nil {
		return s.moveQueryToManualReview(ctx, task, "query_configuration_unavailable")
	}
	result, queryErr := binding.adapter.Query(ctx, channelcap.Query{
		TaskNo: task.TaskNo, ProviderMessageRef: *task.ProviderMessageRef,
		Credential: handle, NonSecretConfig: configuration,
	})
	if queryErr != nil {
		result = channelcap.Result{Class: channelcap.ResultUnknown, CrossedExternalBorder: true,
			EvidenceDigest: digestText("query_error", task.TaskNo, strconv.FormatInt(task.Version, 10))}
	}
	if !isDigest(result.EvidenceDigest) {
		result.EvidenceDigest = digestText("query_result", task.TaskNo, strconv.FormatInt(task.Version, 10), string(result.Class))
	}
	return s.applyQueryResult(ctx, task, result)
}

func (s *serviceImpl) applyQueryResult(ctx context.Context, task *deliveryRow, result channelcap.Result) error {
	status := DeliveryResultUnknown
	switch result.Class {
	case channelcap.ResultDelivered:
		status = DeliveryDelivered
	case channelcap.ResultAccepted:
		status = DeliverySubmitted
	case channelcap.ResultDefinitiveNotReceivedTemporary:
		status = deliveryStatusForResult(result, task.RetrySequence)
	case channelcap.ResultUnknown:
		status = DeliveryResultUnknown
	default:
		status = DeliveryFailed
	}
	deadline := result.QueryDeadline
	deadlineSource := result.QueryDeadlineSource
	if deadline == nil {
		deadline = task.ProviderQueryDeadline
		deadlineSource = task.ProviderQueryDeadlineSource
	}
	if (status == DeliverySubmitted || status == DeliveryResultUnknown) && deadline == nil {
		status = DeliveryManualReview
	}
	queryNo := stableNumber("nqry_", task.TaskNo, strconv.FormatInt(task.Version, 10))
	data := modeldo.DeliveryTask{
		StatusCode: string(status), ResultClass: string(result.Class),
		ProviderQueryDeadline: nullableTimeValue(deadline), ProviderQueryDeadlineSource: deadlineSource,
		AttemptLease: gdb.Raw("NULL"), AttemptLeaseUntil: gdb.Raw("NULL"),
		Version: gdb.Raw("version + 1"),
	}
	if status == DeliveryRetryWait {
		next := task.RetrySequence + 1
		data.RetrySequence = next
		data.NextAttemptAt = gdb.Raw("CURRENT_TIMESTAMP + INTERVAL '" + retryIntervalLiteral(next) + "'")
	}
	if isTerminalDeliveryStatus(status) {
		data.CompletedAt = gdb.Raw("CURRENT_TIMESTAMP")
	}
	return s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		if _, err := s.database.Model("plugin_idc_notify_delivery_query").Ctx(ctx).Data(modeldo.DeliveryQuery{
			TenantId: notificationTenantID, QueryNo: queryNo,
			TaskNo: task.TaskNo, SourceCode: "provider_query",
			ResultClass: string(result.Class), EvidenceDigest: result.EvidenceDigest,
		}).InsertIgnore(); err != nil {
			return err
		}
		updated, err := s.database.Model(deliveryTable).Ctx(ctx).
			Where(tenantIDColumn, notificationTenantID).Where("task_no", task.TaskNo).
			Where("version", task.Version).WhereIn("status_code", []string{string(DeliverySubmitted), string(DeliveryResultUnknown)}).
			Data(data).Update()
		if err != nil || requireOne(updated) != nil {
			return bizerr.NewCode(CodeConflict)
		}
		return s.appendDeliveryAudit(ctx, task.TaskNo, "provider_query", "provider", 0,
			"provider_query", task.StatusCode, string(status), task.Version+1)
	})
}

func (s *serviceImpl) moveQueryToManualReview(ctx context.Context, task *deliveryRow, reason string) error {
	if task == nil {
		return nil
	}
	return s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		updated, err := s.database.Model(deliveryTable).Ctx(ctx).
			Where(tenantIDColumn, notificationTenantID).Where("task_no", task.TaskNo).
			Where("version", task.Version).WhereIn("status_code", []string{string(DeliverySubmitted), string(DeliveryResultUnknown)}).
			Data(modeldo.DeliveryTask{StatusCode: string(DeliveryManualReview), AttemptLease: gdb.Raw("NULL"),
				AttemptLeaseUntil: gdb.Raw("NULL"), Version: gdb.Raw("version + 1")}).Update()
		if err != nil || requireOne(updated) != nil {
			return bizerr.NewCode(CodeConflict)
		}
		return s.appendDeliveryAudit(ctx, task.TaskNo, "query_manual_review", "system", 0,
			reason, task.StatusCode, string(DeliveryManualReview), task.Version+1)
	})
}
