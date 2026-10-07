package productfulfillment

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/gogf/gf/v2/database/gdb"
)

type cancellationRow struct {
	IntentNo, IntentStatus, TaskNo, TaskStatus, LeaseToken string
	SubmitAttempts                                         int
	CancelRequested                                        bool
}

type cancellationPlan struct {
	cancelIntents, cancelTasks, requestTasks, attentionTasks, manualIntents []string
}

type cancellationPlanPayloads struct {
	cancelIntents, cancelTasks, requestTasks, attentionTasks, manualIntents []byte
}

// ConsumeOrderCancelled projects an already committed order-owner fact into
// generic fulfillment. It neither initiates an order cancellation nor decides
// any refund. Database locks and frozen task evidence decide whether an intent
// can be cancelled safely or must stop for manual handling.
func (s *serviceImpl) ConsumeOrderCancelled(ctx context.Context, input OrderCancellationInput) (*OrderCancellationResult, error) {
	input.OrderNo = strings.TrimSpace(input.OrderNo)
	input.CancellationNo = strings.TrimSpace(input.CancellationNo)
	if ctx == nil || ctx.Err() != nil || !managementReferencePattern.MatchString(input.OrderNo) || !managementReferencePattern.MatchString(input.CancellationNo) {
		return nil, errorsNew("product_fulfillment_action_invalid")
	}
	output := &OrderCancellationResult{OrderNo: input.OrderNo}
	err := s.database.Transaction(ctx, func(txCtx context.Context, _ gdb.TX) error {
		var authority *struct{ OrderNo, CancellationNo string }
		if err := s.database.GetScan(txCtx, &authority, `SELECT cancellation.order_no,cancellation.cancellation_no FROM plugin_idc_core_commerce_order_cancellation cancellation JOIN plugin_idc_core_commerce_order orders ON orders.tenant_id=cancellation.tenant_id AND orders.order_no=cancellation.order_no WHERE cancellation.tenant_id=0 AND cancellation.cancellation_no=? AND cancellation.order_no=? AND cancellation.status_code='completed' LIMIT 1 FOR UPDATE OF cancellation,orders`, input.CancellationNo, input.OrderNo); err != nil || authority == nil {
			return errorsNew("product_fulfillment_order_evidence_unavailable")
		}
		rows := make([]cancellationRow, 0, 1001)
		if err := s.database.GetScan(txCtx, &rows, `SELECT intent.intent_no,intent.status_code AS intent_status,COALESCE(task.task_no,'') AS task_no,COALESCE(task.status_code,'') AS task_status,COALESCE(task.lease_token,'') AS lease_token,COALESCE(task.submit_attempt_count,0) AS submit_attempts,COALESCE(task.cancel_requested,FALSE) AS cancel_requested FROM plugin_idc_core_product_fulfillment_intent intent LEFT JOIN plugin_idc_core_product_fulfillment_task task ON task.tenant_id=intent.tenant_id AND task.intent_no=intent.intent_no WHERE intent.tenant_id=0 AND intent.order_no=? ORDER BY intent.intent_no LIMIT 1001 FOR UPDATE OF intent`, input.OrderNo); err != nil || len(rows) > 1000 {
			return errorsNew("product_fulfillment_order_evidence_unavailable")
		}
		if len(rows) == 0 {
			output.Reused = true
			return nil
		}
		if _, err := s.database.Exec(txCtx, `SELECT task.task_no FROM plugin_idc_core_product_fulfillment_task task JOIN plugin_idc_core_product_fulfillment_intent intent ON intent.tenant_id=task.tenant_id AND intent.intent_no=task.intent_no WHERE task.tenant_id=0 AND intent.order_no=? ORDER BY task.task_no FOR UPDATE OF task`, input.OrderNo); err != nil {
			return err
		}
		plan, planErr := planCancellation(rows)
		if planErr != nil {
			return planErr
		}
		cancelIntents, cancelTasks := plan.cancelIntents, plan.cancelTasks
		requestTasks, attentionTasks, manualIntents := plan.requestTasks, plan.attentionTasks, plan.manualIntents
		payloads, marshalErr := marshalCancellationPlan(plan)
		if marshalErr != nil {
			return marshalErr
		}
		if len(cancelIntents) > 0 {
			reason, reasonErr := s.systemReasons.ResolveSystemTransitionReason(SystemReasonOrderCancelled)
			if reasonErr != nil {
				return reasonErr
			}
			if _, err := s.database.Exec(txCtx, `INSERT INTO plugin_idc_core_product_fulfillment_system_transition(tenant_id,transition_no,intent_no,event_code,reason_code,reason_label_zh_cn,reason_label_en_us) SELECT 0,'pfs_'||substr(md5('cancelled'||intent_no),1,32),intent_no,'cancelled',?,?,? FROM jsonb_array_elements_text(?::jsonb) value(intent_no) ON CONFLICT(intent_no,event_code) DO NOTHING`, SystemReasonOrderCancelled, reason.LabelZhCN, reason.LabelEnUS, payloads.cancelIntents); err != nil {
				return err
			}
			updated, err := s.database.Exec(txCtx, `UPDATE plugin_idc_core_product_fulfillment_intent SET status_code='canceled',version=version+1,updated_at=CURRENT_TIMESTAMP WHERE tenant_id=0 AND intent_no IN(SELECT jsonb_array_elements_text(?::jsonb)) AND status_code IN('manual_pending','review_pending','gate_pending','queued','processing')`, payloads.cancelIntents)
			if err != nil || affected(updated) != int64(len(cancelIntents)) {
				return errorsNew("product_fulfillment_task_conflict")
			}
		}
		if len(cancelTasks) > 0 {
			updated, err := s.database.Exec(txCtx, `UPDATE plugin_idc_core_product_fulfillment_task SET status_code='canceled',phase_code='frozen_unknown',cancel_requested=TRUE,next_attempt_at=CURRENT_TIMESTAMP,version=version+1,updated_at=CURRENT_TIMESTAMP WHERE tenant_id=0 AND task_no IN(SELECT jsonb_array_elements_text(?::jsonb)) AND status_code IN('pending_query','pending_submit','retry_wait') AND lease_token='' AND submit_attempt_count=0`, payloads.cancelTasks)
			if err != nil || affected(updated) != int64(len(cancelTasks)) {
				return errorsNew("product_fulfillment_task_conflict")
			}
		}
		if len(requestTasks) > 0 {
			updated, err := s.database.Exec(txCtx, `UPDATE plugin_idc_core_product_fulfillment_task SET cancel_requested=TRUE,version=version+1,updated_at=CURRENT_TIMESTAMP WHERE tenant_id=0 AND task_no IN(SELECT jsonb_array_elements_text(?::jsonb)) AND status_code IN('querying','submitting') AND cancel_requested=FALSE`, payloads.requestTasks)
			if err != nil || affected(updated) != int64(len(requestTasks)) {
				return errorsNew("product_fulfillment_task_conflict")
			}
		}
		if len(attentionTasks) > 0 {
			updated, err := s.database.Exec(txCtx, `UPDATE plugin_idc_core_product_fulfillment_task SET status_code='cancellation_attention',phase_code='frozen_unknown',cancel_requested=TRUE,next_attempt_at=CURRENT_TIMESTAMP,version=version+1,updated_at=CURRENT_TIMESTAMP WHERE tenant_id=0 AND task_no IN(SELECT jsonb_array_elements_text(?::jsonb)) AND status_code IN('pending_query','pending_submit','retry_wait','accepted','running','result_unknown') AND lease_token=''`, payloads.attentionTasks)
			if err != nil || affected(updated) != int64(len(attentionTasks)) {
				return errorsNew("product_fulfillment_task_conflict")
			}
		}
		if len(manualIntents) > 0 {
			reason, reasonErr := s.systemReasons.ResolveSystemTransitionReason(SystemReasonCancellationRequiresAttention)
			if reasonErr != nil {
				return reasonErr
			}
			if _, err := s.database.Exec(txCtx, `INSERT INTO plugin_idc_core_product_fulfillment_system_transition(tenant_id,transition_no,intent_no,event_code,reason_code,reason_label_zh_cn,reason_label_en_us) SELECT 0,'pfs_'||substr(md5('manual-cancel'||intent_no),1,32),intent_no,'moved_to_manual',?,?,? FROM jsonb_array_elements_text(?::jsonb) value(intent_no) ON CONFLICT(intent_no,event_code) DO NOTHING`, SystemReasonCancellationRequiresAttention, reason.LabelZhCN, reason.LabelEnUS, payloads.manualIntents); err != nil {
				return err
			}
			updated, err := s.database.Exec(txCtx, `UPDATE plugin_idc_core_product_fulfillment_intent SET status_code='manual_pending',version=version+1,updated_at=CURRENT_TIMESTAMP WHERE tenant_id=0 AND intent_no IN(SELECT jsonb_array_elements_text(?::jsonb)) AND status_code IN('queued','processing')`, payloads.manualIntents)
			if err != nil || affected(updated) != int64(len(manualIntents)) {
				return errorsNew("product_fulfillment_task_conflict")
			}
		}
		output.Cancelled, output.AwaitingQuery, output.Manual = len(cancelIntents), len(requestTasks), len(manualIntents)
		output.Reused = output.Cancelled+output.AwaitingQuery+output.Manual == 0
		return nil
	})
	return output, err
}

func marshalCancellationPlan(plan cancellationPlan) (cancellationPlanPayloads, error) {
	var result cancellationPlanPayloads
	values := []struct {
		source []string
		target *[]byte
	}{
		{plan.cancelIntents, &result.cancelIntents},
		{plan.cancelTasks, &result.cancelTasks},
		{plan.requestTasks, &result.requestTasks},
		{plan.attentionTasks, &result.attentionTasks},
		{plan.manualIntents, &result.manualIntents},
	}
	for _, value := range values {
		raw, err := json.Marshal(value.source)
		if err != nil {
			return cancellationPlanPayloads{}, err
		}
		*value.target = raw
	}
	return result, nil
}

func planCancellation(rows []cancellationRow) (cancellationPlan, error) {
	plan := cancellationPlan{}
	for _, current := range rows {
		if current.IntentStatus == "completed" || current.IntentStatus == "incomplete" || current.IntentStatus == "canceled" {
			continue
		}
		if current.TaskNo == "" {
			plan.cancelIntents = append(plan.cancelIntents, current.IntentNo)
			continue
		}
		switch current.TaskStatus {
		case "querying", "submitting":
			if !current.CancelRequested {
				plan.requestTasks = append(plan.requestTasks, current.TaskNo)
			}
		case "pending_query", "pending_submit", "retry_wait":
			if current.SubmitAttempts == 0 && current.LeaseToken == "" {
				plan.cancelTasks = append(plan.cancelTasks, current.TaskNo)
				plan.cancelIntents = append(plan.cancelIntents, current.IntentNo)
			} else {
				plan.attentionTasks = append(plan.attentionTasks, current.TaskNo)
				plan.manualIntents = append(plan.manualIntents, current.IntentNo)
			}
		case "accepted", "running", "result_unknown":
			plan.attentionTasks = append(plan.attentionTasks, current.TaskNo)
			plan.manualIntents = append(plan.manualIntents, current.IntentNo)
		case "succeeded", "failed", "manual_takeover", "cancellation_attention", "canceled":
		default:
			return cancellationPlan{}, errorsNew("product_fulfillment_task_conflict")
		}
	}
	return plan, nil
}
