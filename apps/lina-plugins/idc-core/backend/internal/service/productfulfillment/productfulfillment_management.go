package productfulfillment

import (
	"context"
	"database/sql"
	"encoding/json"
	"math"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/gogf/gf/v2/database/gdb"

	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/internal/service/customerprojection"
)

const (
	actionReviewApprove       = "review_approve"
	actionReviewReject        = "review_reject"
	actionTakeover            = "takeover"
	actionConcludeCompleted   = "manual_conclude_completed"
	actionConcludeUnable      = "manual_conclude_unable"
	maximumManagementPageSize = 100
)

var managementReferencePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.:-]{0,127}$`)

type actionAuthorization func(context.Context, string) (Authorization, error)

func (s *serviceImpl) ApproveAndStart(ctx context.Context, input ActionInput) (*IntentDetail, error) {
	return s.executeAction(ctx, input, actionReviewApprove, s.authorizer.AuthorizeReviewApprove)
}

func (s *serviceImpl) RejectToManual(ctx context.Context, input ActionInput) (*IntentDetail, error) {
	return s.executeAction(ctx, input, actionReviewReject, s.authorizer.AuthorizeReviewReject)
}

func (s *serviceImpl) Takeover(ctx context.Context, input ActionInput) (*IntentDetail, error) {
	return s.executeAction(ctx, input, actionTakeover, s.authorizer.AuthorizeTakeover)
}

func (s *serviceImpl) ConcludeManual(ctx context.Context, input ManualConclusionInput) (*IntentDetail, error) {
	action := actionConcludeCompleted
	if input.Outcome == "unable_to_complete" {
		action = actionConcludeUnable
	} else if input.Outcome != "completed" {
		if s.authorizer == nil {
			return nil, errorsNew("product_fulfillment_management_unavailable")
		}
		if _, err := s.authorizer.AuthorizeManualConclude(ctx, input.IntentNo); err != nil {
			return nil, err
		}
		return nil, errorsNew("product_fulfillment_action_invalid")
	}
	return s.executeAction(ctx, input.ActionInput, action, s.authorizer.AuthorizeManualConclude)
}

func (s *serviceImpl) executeAction(ctx context.Context, input ActionInput, action string, authorize actionAuthorization) (*IntentDetail, error) {
	if s.authorizer == nil || s.audit == nil || s.gates == nil {
		return nil, errorsNew("product_fulfillment_management_unavailable")
	}
	authorization, err := authorize(ctx, input.IntentNo)
	if err != nil {
		return nil, err
	}
	input.IntentNo, input.IdempotencyKey = strings.TrimSpace(input.IntentNo), strings.TrimSpace(input.IdempotencyKey)
	input.ReasonCode, input.Note = strings.TrimSpace(input.ReasonCode), strings.TrimSpace(input.Note)
	if authorization.ActorUserID <= 0 || authorization.RequiredPermission == "" ||
		authorization.RequiredPermission != permissionForAction(action) || strings.TrimSpace(authorization.DataScope) == "" ||
		!managementReferencePattern.MatchString(input.IntentNo) || input.ExpectedVersion < 1 ||
		len(input.IdempotencyKey) < 16 || len(input.IdempotencyKey) > 128 ||
		!managementReferencePattern.MatchString(input.IdempotencyKey) ||
		!managementReferencePattern.MatchString(input.ReasonCode) || !validSafeNote(input.Note, action == actionConcludeCompleted || action == actionConcludeUnable) {
		return nil, errorsNew("product_fulfillment_action_invalid")
	}
	requestDigest, err := digestJSON(struct {
		Action, IntentNo, IdempotencyKey, ReasonCode, Note string
		ExpectedVersion                                    int64
	}{action, input.IntentNo, input.IdempotencyKey, input.ReasonCode, input.Note, input.ExpectedVersion})
	if err != nil {
		return nil, err
	}
	var detail *IntentDetail
	err = s.database.Transaction(ctx, func(txCtx context.Context, _ gdb.TX) error {
		current, loadErr := s.loadDetailAuthorized(txCtx, input.IntentNo, true, authorization)
		if loadErr != nil || current == nil {
			return errorsNew("product_fulfillment_not_accessible")
		}
		replayed, replayErr := s.commandReplay(txCtx, action, input, requestDigest)
		if replayErr != nil {
			return replayErr
		}
		if replayed {
			detail = current
			return nil
		}
		if current.Version != input.ExpectedVersion {
			return errorsNew("product_fulfillment_action_conflict")
		}
		reason, validErr := s.validateReason(txCtx, action, input.ReasonCode)
		if validErr != nil {
			return validErr
		}
		if applyErr := s.applyAction(txCtx, current, input, action, authorization); applyErr != nil {
			return applyErr
		}
		var actualAfter struct {
			StatusCode        string
			TakeoverRequested bool
		}
		if loadErr := s.database.GetScan(txCtx, &actualAfter, `SELECT intent.status_code,COALESCE(task.takeover_requested,FALSE) AS takeover_requested FROM plugin_idc_core_product_fulfillment_intent intent LEFT JOIN plugin_idc_core_product_fulfillment_task task ON task.tenant_id=intent.tenant_id AND task.intent_no=intent.intent_no WHERE intent.tenant_id=0 AND intent.intent_no=? LIMIT 1`, input.IntentNo); loadErr != nil {
			return loadErr
		}
		auditStatus := actualAfter.StatusCode
		if actualAfter.TakeoverRequested && actualAfter.StatusCode != "manual_pending" {
			auditStatus = "takeover_requested"
		}
		commandNo := deterministicNo("pfc_", action, input.IdempotencyKey)
		if _, insertErr := s.database.Exec(txCtx, `INSERT INTO plugin_idc_core_product_fulfillment_command(tenant_id,command_no,intent_no,action_code,idempotency_key,request_digest,reason_code,reason_label_zh_cn,reason_label_en_us,safe_note,actor_user_id,resulting_version) VALUES(0,?,?,?,?,?,?,?,?,?,?,?)`, commandNo, input.IntentNo, action, input.IdempotencyKey, requestDigest, input.ReasonCode, reason.LabelZhCN, reason.LabelEnUS, input.Note, authorization.ActorUserID, input.ExpectedVersion+1); insertErr != nil {
			return insertErr
		}
		if auditErr := s.audit.AppendBusinessTransition(txCtx, idccap.BusinessAuditTransition{
			IdempotencyScope: "product-fulfillment." + action, IdempotencyKey: input.IdempotencyKey,
			OperationCode: "product-fulfillment." + action, TargetType: "product_fulfillment_intent",
			TargetNo: input.IntentNo, CustomerNo: current.CustomerNo, ResultCode: "succeeded",
			ReasonCode: input.ReasonCode, RiskCode: "high", RequiredPermission: authorization.RequiredPermission,
			BeforeSnapshot: map[string]any{"status": current.internalStatus, "version": current.Version},
			AfterSnapshot:  map[string]any{"status": auditStatus, "version": current.Version + 1, "data_scope": authorization.DataScope},
		}); auditErr != nil {
			return auditErr
		}
		detail, loadErr = s.loadDetail(txCtx, input.IntentNo, false)
		return loadErr
	})
	return detail, err
}

func (s *serviceImpl) applyAction(ctx context.Context, current *IntentDetail, input ActionInput, action string, authorization Authorization) error {
	switch action {
	case actionReviewApprove:
		if current.internalStatus != "review_pending" || current.StrategyCode != StrategyApprovalThenPlugin {
			return errorsNew("product_fulfillment_action_conflict")
		}
		if err := s.validateOwnerRelease(ctx, current.OrderNo, current.OrderItemNo); err != nil {
			return err
		}
		result, err := s.database.Exec(ctx, `UPDATE plugin_idc_core_product_fulfillment_intent SET status_code='queued',approved_by=?,approved_at=CURRENT_TIMESTAMP,version=version+1,updated_at=CURRENT_TIMESTAMP WHERE tenant_id=0 AND intent_no=? AND version=? AND status_code='review_pending'`, authorization.ActorUserID, current.IntentNo, current.Version)
		if err != nil || affected(result) != 1 {
			return errorsNew("product_fulfillment_action_conflict")
		}
		taskNo := deterministicNo("pfk_", current.IntentNo)
		_, err = s.database.Exec(ctx, `INSERT INTO plugin_idc_core_product_fulfillment_task(tenant_id,task_no,intent_no,status_code,maximum_attempts,retry_base_seconds) SELECT 0,? ,intent_no,'pending_query',maximum_attempts,retry_base_seconds FROM plugin_idc_core_product_fulfillment_intent WHERE tenant_id=0 AND intent_no=? ON CONFLICT(intent_no) DO NOTHING`, taskNo, current.IntentNo)
		if err != nil {
			return err
		}
		var taskCount int
		if err = s.database.GetScan(ctx, &taskCount, `SELECT count(*) FROM plugin_idc_core_product_fulfillment_task task JOIN plugin_idc_core_product_fulfillment_intent intent ON intent.intent_no=task.intent_no WHERE task.tenant_id=0 AND task.task_no=? AND task.intent_no=? AND task.maximum_attempts=intent.maximum_attempts AND task.retry_base_seconds=intent.retry_base_seconds`, taskNo, current.IntentNo); err != nil || taskCount != 1 {
			return errorsNew("product_fulfillment_task_conflict")
		}
		return nil
	case actionReviewReject:
		if current.internalStatus != "review_pending" {
			return errorsNew("product_fulfillment_action_conflict")
		}
		return updateIntentStatus(ctx, s.database, current, "manual_pending")
	case actionTakeover:
		if current.internalStatus != "queued" && current.internalStatus != "processing" {
			return errorsNew("product_fulfillment_action_conflict")
		}
		var task *struct {
			StatusCode, LeaseToken string
			Version                int64
		}
		if err := s.database.GetScan(ctx, &task, `SELECT status_code,lease_token,version FROM plugin_idc_core_product_fulfillment_task WHERE tenant_id=0 AND intent_no=? LIMIT 1 FOR UPDATE`, current.IntentNo); err != nil || task == nil {
			return errorsNew("product_fulfillment_action_conflict")
		}
		inFlight := (task.StatusCode == "querying" || task.StatusCode == "submitting") && task.LeaseToken != ""
		if inFlight {
			taskResult, err := s.database.Exec(ctx, `UPDATE plugin_idc_core_product_fulfillment_task SET takeover_requested=TRUE,version=version+1,updated_at=CURRENT_TIMESTAMP WHERE tenant_id=0 AND intent_no=? AND version=? AND status_code=? AND lease_token=? AND takeover_requested=FALSE`, current.IntentNo, task.Version, task.StatusCode, task.LeaseToken)
			if err != nil || affected(taskResult) != 1 {
				return errorsNew("product_fulfillment_action_conflict")
			}
			result, err := s.database.Exec(ctx, `UPDATE plugin_idc_core_product_fulfillment_intent SET version=version+1,updated_at=CURRENT_TIMESTAMP WHERE tenant_id=0 AND intent_no=? AND version=?`, current.IntentNo, current.Version)
			if err != nil || affected(result) != 1 {
				return errorsNew("product_fulfillment_action_conflict")
			}
			return nil
		}
		taskResult, err := s.database.Exec(ctx, `UPDATE plugin_idc_core_product_fulfillment_task SET status_code='manual_takeover',phase_code='frozen_unknown',takeover_requested=TRUE,lease_token='',lease_expires_at=NULL,version=version+1,updated_at=CURRENT_TIMESTAMP WHERE tenant_id=0 AND intent_no=? AND version=? AND status_code IN ('pending_query','pending_submit','accepted','running','retry_wait','result_unknown')`, current.IntentNo, task.Version)
		if err != nil || affected(taskResult) != 1 {
			return errorsNew("product_fulfillment_action_conflict")
		}
		return updateIntentStatus(ctx, s.database, current, "manual_pending")
	case actionConcludeCompleted, actionConcludeUnable:
		if current.internalStatus != "manual_pending" {
			return errorsNew("product_fulfillment_action_conflict")
		}
		outcome, status := "completed", "completed"
		if action == actionConcludeUnable {
			outcome, status = "unable", "incomplete"
		}
		if _, err := s.database.Exec(ctx, `INSERT INTO plugin_idc_core_product_fulfillment_manual_conclusion(tenant_id,conclusion_no,intent_no,outcome_code,reason_code,note,actor_user_id,intent_version) VALUES(0,?,?,?,?,?,?,?)`, deterministicNo("pfn_", current.IntentNo), current.IntentNo, outcome, input.ReasonCode, input.Note, authorization.ActorUserID, current.Version+1); err != nil {
			return err
		}
		return updateIntentStatus(ctx, s.database, current, status)
	default:
		return errorsNew("product_fulfillment_action_invalid")
	}
}

func (s *serviceImpl) validateOwnerRelease(ctx context.Context, orderNo, orderItemNo string) error {
	ready, err := evaluateOwnerGates(ctx, s.gates, orderNo, []string{orderItemNo})
	if err != nil || !ready[orderItemNo] {
		return errorsNew("product_fulfillment_owner_gate_blocked")
	}
	var state string
	if err := s.database.GetScan(ctx, &state, `SELECT interface.new_business_state FROM plugin_idc_core_product_fulfillment_intent intent JOIN plugin_idc_core_product_automation_interface_instance interface ON interface.tenant_id=intent.tenant_id AND interface.interface_instance_no=intent.interface_instance_no WHERE intent.tenant_id=0 AND intent.order_no=? AND intent.order_item_no=? LIMIT 1`, orderNo, orderItemNo); err != nil || state != "accepting" {
		return errorsNew("product_fulfillment_new_business_stopped")
	}
	return nil
}

func updateIntentStatus(ctx context.Context, database gdb.DB, current *IntentDetail, status string) error {
	result, err := database.Exec(ctx, `UPDATE plugin_idc_core_product_fulfillment_intent SET status_code=?,version=version+1,updated_at=CURRENT_TIMESTAMP WHERE tenant_id=0 AND intent_no=? AND version=?`, status, current.IntentNo, current.Version)
	if err != nil || affected(result) != 1 {
		return errorsNew("product_fulfillment_action_conflict")
	}
	return nil
}

func affected(result sql.Result) int64 {
	if result == nil {
		return 0
	}
	count, err := result.RowsAffected()
	if err != nil {
		return 0
	}
	return count
}

func (s *serviceImpl) commandReplay(ctx context.Context, action string, input ActionInput, digest string) (bool, error) {
	var row *struct{ IntentNo, RequestDigest string }
	if err := s.database.GetScan(ctx, &row, `SELECT intent_no,request_digest FROM plugin_idc_core_product_fulfillment_command WHERE tenant_id=0 AND action_code=? AND idempotency_key=? LIMIT 1`, action, input.IdempotencyKey); err != nil {
		return false, err
	}
	if row == nil {
		return false, nil
	}
	if row.IntentNo != input.IntentNo || row.RequestDigest != digest {
		return false, errorsNew("product_fulfillment_idempotency_conflict")
	}
	return true, nil
}

func (s *serviceImpl) validateReason(ctx context.Context, action, reason string) (*ReasonOption, error) {
	var value *ReasonOption
	if err := s.database.GetScan(ctx, &value, `SELECT reason_code AS code,label_zh_cn,label_en_us FROM plugin_idc_core_product_fulfillment_reason_option WHERE tenant_id=0 AND action_code=? AND reason_code=? AND status_code='active' LIMIT 1`, action, reason); err != nil || value == nil {
		return nil, errorsNew("product_fulfillment_reason_invalid")
	}
	return value, nil
}

func permissionForAction(action string) string {
	switch action {
	case actionReviewApprove:
		return PermissionReviewApprove
	case actionReviewReject:
		return PermissionReviewReject
	case actionTakeover:
		return PermissionManualTakeover
	case actionConcludeCompleted, actionConcludeUnable:
		return PermissionManualConclude
	}
	return ""
}

func (s *serviceImpl) Detail(ctx context.Context, intentNo string) (*IntentDetail, error) {
	if s.authorizer == nil {
		return nil, errorsNew("product_fulfillment_management_unavailable")
	}
	authorization, err := s.authorizer.AuthorizeDetail(ctx, intentNo)
	if err != nil {
		return nil, err
	}
	if authorization.ActorUserID <= 0 || authorization.RequiredPermission != PermissionTaskQuery || strings.TrimSpace(authorization.DataScope) == "" {
		return nil, errorsNew("product_fulfillment_not_accessible")
	}
	if !managementReferencePattern.MatchString(strings.TrimSpace(intentNo)) {
		return nil, errorsNew("product_fulfillment_not_accessible")
	}
	return s.loadDetailAuthorized(ctx, strings.TrimSpace(intentNo), false, authorization)
}

func (s *serviceImpl) loadDetailAuthorized(ctx context.Context, intentNo string, lock bool, authorization Authorization) (*IntentDetail, error) {
	if !validAuthorizationScope(authorization) {
		return nil, errorsNew("product_fulfillment_not_accessible")
	}
	scopeJSON, err := json.Marshal(authorization.AllowedCustomerNos)
	if err != nil {
		return nil, errorsNew("product_fulfillment_not_accessible")
	}
	query := `SELECT intent_no,order_no,order_item_no,customer_no,product_name_zh_cn,product_name_en_us,strategy_code,granularity_code,initial_status_code,status_code,quantity,unit_sequence,version,financially_settled_at,updated_at,EXISTS(SELECT 1 FROM plugin_idc_core_product_fulfillment_task task WHERE task.tenant_id=0 AND task.intent_no=intent.intent_no AND task.takeover_requested) AS takeover_requested FROM plugin_idc_core_product_fulfillment_intent intent WHERE tenant_id=0 AND intent_no=? AND (? OR customer_no IN(SELECT jsonb_array_elements_text(?::jsonb))) LIMIT 1`
	if lock {
		query += ` FOR UPDATE`
	}
	return s.scanDetail(ctx, query, intentNo, authorization.AllCustomers, scopeJSON)
}

func (s *serviceImpl) loadDetail(ctx context.Context, intentNo string, lock bool) (*IntentDetail, error) {
	query := `SELECT intent_no,order_no,order_item_no,customer_no,product_name_zh_cn,product_name_en_us,strategy_code,granularity_code,initial_status_code,status_code,quantity,unit_sequence,version,financially_settled_at,updated_at,EXISTS(SELECT 1 FROM plugin_idc_core_product_fulfillment_task task WHERE task.tenant_id=0 AND task.intent_no=intent.intent_no AND task.takeover_requested) AS takeover_requested FROM plugin_idc_core_product_fulfillment_intent intent WHERE tenant_id=0 AND intent_no=? LIMIT 1`
	if lock {
		query += ` FOR UPDATE`
	}
	return s.scanDetail(ctx, query, intentNo)
}

func (s *serviceImpl) scanDetail(ctx context.Context, query string, args ...any) (*IntentDetail, error) {
	var row *intentDetailRow
	if err := s.database.GetScan(ctx, &row, query, args...); err != nil || row == nil {
		return nil, errorsNew("product_fulfillment_not_accessible")
	}
	if safeIntentStatus(row.StatusCode) == "" || safeIntentStatus(row.InitialStatusCode) == "" {
		return nil, errorsNew("product_fulfillment_management_unavailable")
	}
	detail := row.safe()
	customers, err := customerprojection.Load(ctx, s.database, []string{row.CustomerNo})
	if err != nil {
		return nil, errorsNew("product_fulfillment_management_unavailable")
	}
	detail.CustomerCode = customers[row.CustomerNo].CustomerCode
	detail.CustomerDisplayName = customers[row.CustomerNo].DisplayName
	detail.CustomerPrimaryPhone = customers[row.CustomerNo].PrimaryPhone
	options, err := s.loadReasonOptions(ctx)
	if err != nil {
		return nil, err
	}
	detail.ReasonOptions = options
	detail.Events, err = s.loadSafeEvents(ctx, row)
	if err != nil {
		return nil, err
	}
	return detail, nil
}

func validAuthorizationScope(authorization Authorization) bool {
	if strings.TrimSpace(authorization.DataScope) == "" || (!authorization.AllCustomers && (len(authorization.AllowedCustomerNos) == 0 || len(authorization.AllowedCustomerNos) > 1000)) {
		return false
	}
	for _, customerNo := range authorization.AllowedCustomerNos {
		if !managementReferencePattern.MatchString(customerNo) {
			return false
		}
	}
	return true
}

type intentDetailRow struct {
	IntentNo, OrderNo, OrderItemNo, CustomerNo                   string
	ProductNameZhCN, ProductNameEnUS                             string
	StrategyCode, GranularityCode, InitialStatusCode, StatusCode string
	Quantity, UnitSequence                                       int
	Version                                                      int64
	FinanciallySettledAt, UpdatedAt                              time.Time
	TakeoverRequested                                            bool
}

func (row *intentDetailRow) safe() *IntentDetail {
	return &IntentDetail{IntentNo: row.IntentNo, OrderNo: row.OrderNo, OrderItemNo: row.OrderItemNo,
		CustomerNo: row.CustomerNo, ProductNameZhCN: row.ProductNameZhCN, ProductNameEnUS: row.ProductNameEnUS,
		StrategyCode: row.StrategyCode, GranularityCode: row.GranularityCode, StatusCode: safeIntentStatus(row.StatusCode),
		Quantity: row.Quantity, UnitSequence: row.UnitSequence, Version: row.Version,
		FinanciallySettledAt: row.FinanciallySettledAt, UpdatedAt: row.UpdatedAt,
		CanReviewApprove: row.StatusCode == "review_pending", CanReviewReject: row.StatusCode == "review_pending",
		CanTakeover:       !row.TakeoverRequested && (row.StatusCode == "queued" || row.StatusCode == "processing"),
		CanManualConclude: row.StatusCode == "manual_pending", internalStatus: row.StatusCode}
}

func safeIntentStatus(status string) string {
	switch status {
	case "review_pending":
		return "awaiting_review"
	case "manual_pending", "gate_pending", "queued", "processing":
		return "processing"
	case "completed":
		return "completed"
	case "incomplete":
		return "not_completed"
	case "canceled":
		return "cancelled"
	default:
		return ""
	}
}

func (s *serviceImpl) loadSafeEvents(ctx context.Context, row *intentDetailRow) ([]IntentEvent, error) {
	type eventRow struct {
		TypeCode, ReasonCode, ReasonLabelZhCN, ReasonLabelEnUS, SafeNote string
		ActorUserID                                                      int
		OccurredAt                                                       time.Time
	}
	rows := make([]eventRow, 0, 65)
	if err := s.database.GetScan(ctx, &rows, `SELECT type_code,reason_code,reason_label_zh_cn,reason_label_en_us,safe_note,actor_user_id,occurred_at FROM (
SELECT CASE action_code WHEN 'review_approve' THEN 'review_approved' WHEN 'review_reject' THEN 'review_rejected'
WHEN 'takeover' THEN 'taken_over' WHEN 'manual_conclude_completed' THEN 'manual_completed'
WHEN 'manual_conclude_unable' THEN 'manual_unable_to_complete' END AS type_code,
reason_code,reason_label_zh_cn,reason_label_en_us,safe_note,actor_user_id,created_at AS occurred_at
FROM plugin_idc_core_product_fulfillment_command WHERE tenant_id=0 AND intent_no=?
UNION ALL
SELECT event_code,reason_code,reason_label_zh_cn,reason_label_en_us,'',0,created_at
FROM plugin_idc_core_product_fulfillment_system_transition WHERE tenant_id=0 AND intent_no=?
UNION ALL
SELECT 'automation_started','','','','',0,task.created_at FROM plugin_idc_core_product_fulfillment_task task
WHERE task.tenant_id=0 AND task.intent_no=?
UNION ALL
SELECT CASE attempt.result_status_code WHEN 'succeeded' THEN 'automation_completed' WHEN 'failed' THEN 'automation_failed'
ELSE 'automation_processing' END,'','','','',0,attempt.finished_at
FROM plugin_idc_core_product_fulfillment_attempt attempt JOIN plugin_idc_core_product_fulfillment_task task ON task.task_no=attempt.task_no
WHERE task.tenant_id=0 AND task.intent_no=? AND attempt.finished_at IS NOT NULL
) event ORDER BY occurred_at,type_code LIMIT 65`, row.IntentNo, row.IntentNo, row.IntentNo, row.IntentNo); err != nil || len(rows) > 64 {
		return nil, errorsNew("product_fulfillment_management_unavailable")
	}
	actorSet := make(map[int]struct{}, len(rows))
	actorIDs := make([]int, 0, len(rows))
	for _, event := range rows {
		if event.ActorUserID <= 0 {
			continue
		}
		if _, ok := actorSet[event.ActorUserID]; !ok {
			actorSet[event.ActorUserID] = struct{}{}
			actorIDs = append(actorIDs, event.ActorUserID)
		}
	}
	names := map[int]string{}
	var err error
	if len(actorIDs) > 0 {
		names, err = s.actors.DisplayNames(ctx, actorIDs)
		if err != nil || len(names) != len(actorIDs) {
			return nil, errorsNew("product_fulfillment_management_unavailable")
		}
	}
	result := make([]IntentEvent, 0, len(rows)+1)
	result = append(result, IntentEvent{TypeCode: "intent_created", StatusCode: safeIntentStatus(row.InitialStatusCode), OccurredAt: row.FinanciallySettledAt})
	for _, event := range rows {
		safeStatus := safeCommandStatus(event.TypeCode)
		if safeStatus == "" {
			return nil, errorsNew("product_fulfillment_management_unavailable")
		}
		name := ""
		if event.ActorUserID > 0 {
			name = strings.TrimSpace(names[event.ActorUserID])
			if name == "" {
				return nil, errorsNew("product_fulfillment_management_unavailable")
			}
		}
		result = append(result, IntentEvent{TypeCode: event.TypeCode, StatusCode: safeStatus, ReasonCode: event.ReasonCode, ReasonLabelZhCN: event.ReasonLabelZhCN, ReasonLabelEnUS: event.ReasonLabelEnUS, OperatorDisplayName: name, SafeNote: event.SafeNote, OccurredAt: event.OccurredAt})
	}
	return result, nil
}

func safeCommandStatus(action string) string {
	switch action {
	case "review_approved", "taken_over", "moved_to_manual", "automation_started", "automation_processing":
		return "processing"
	case "review_rejected":
		return "processing"
	case "manual_completed", "automation_completed":
		return "completed"
	case "manual_unable_to_complete", "automation_failed":
		return "not_completed"
	case "cancelled":
		return "cancelled"
	}
	return ""
}

func (s *serviceImpl) loadReasonOptions(ctx context.Context) (map[string][]ReasonOption, error) {
	var rows []struct{ ActionCode, ReasonCode, LabelZhCN, LabelEnUS string }
	if err := s.database.GetScan(ctx, &rows, `SELECT action_code,reason_code,label_zh_cn,label_en_us FROM plugin_idc_core_product_fulfillment_reason_option WHERE tenant_id=0 AND status_code='active' ORDER BY action_code,sort_order,reason_code LIMIT 101`); err != nil || len(rows) > 100 {
		return nil, errorsNew("product_fulfillment_management_unavailable")
	}
	result := make(map[string][]ReasonOption)
	for _, row := range rows {
		result[row.ActionCode] = append(result[row.ActionCode], ReasonOption{Code: row.ReasonCode, LabelZhCN: row.LabelZhCN, LabelEnUS: row.LabelEnUS})
	}
	return result, nil
}

func (s *serviceImpl) List(ctx context.Context, input ListInput) (*IntentPage, error) {
	if s.authorizer == nil {
		return nil, errorsNew("product_fulfillment_management_unavailable")
	}
	authorization, err := s.authorizer.AuthorizeList(ctx)
	if err != nil {
		return nil, err
	}
	if authorization.ActorUserID <= 0 || authorization.RequiredPermission != PermissionTaskList || strings.TrimSpace(authorization.DataScope) == "" {
		return nil, errorsNew("product_fulfillment_not_accessible")
	}
	if !authorization.AllCustomers && (len(authorization.AllowedCustomerNos) == 0 || len(authorization.AllowedCustomerNos) > 1000) {
		return nil, errorsNew("product_fulfillment_not_accessible")
	}
	for _, customerNo := range authorization.AllowedCustomerNos {
		if !managementReferencePattern.MatchString(customerNo) {
			return nil, errorsNew("product_fulfillment_not_accessible")
		}
	}
	scopeJSON, err := json.Marshal(authorization.AllowedCustomerNos)
	if err != nil {
		return nil, errorsNew("product_fulfillment_not_accessible")
	}
	input, err = normalizeIntentListInput(input)
	if err != nil {
		return nil, err
	}
	rows := make([]intentDetailRow, 0, input.Limit+1)
	scopeValues := append([]string(nil), authorization.AllowedCustomerNos...)
	sort.Strings(scopeValues)
	scopeDigest, err := digestJSON(struct {
		Code      string
		All       bool
		Customers []string
	}{authorization.DataScope, authorization.AllCustomers, scopeValues})
	if err != nil {
		return nil, errorsNew("product_fulfillment_not_accessible")
	}
	cursorDigest := fulfillmentCursorDigest(input, scopeDigest)
	boundary := ""
	if input.Cursor != "" {
		boundary, err = s.cursor.decode(input.Cursor, cursorDigest)
		if err != nil {
			return nil, errorsNew("product_fulfillment_action_invalid")
		}
	}
	const filtered = ` FROM plugin_idc_core_product_fulfillment_intent intent
WHERE tenant_id=0 AND (? OR customer_no IN(SELECT jsonb_array_elements_text(?::jsonb)))
  AND (?='' OR order_no=?) AND (?='' OR customer_no=?) AND (?='' OR strategy_code=?)
  AND (?='' OR (?='awaiting_review' AND status_code='review_pending')
       OR (?='processing' AND status_code IN ('manual_pending','gate_pending','queued','processing'))
       OR (?='completed' AND status_code='completed') OR (?='not_completed' AND status_code='incomplete')
       OR (?='cancelled' AND status_code='canceled'))`
	args := []any{authorization.AllCustomers, scopeJSON, input.OrderNo, input.OrderNo, input.CustomerNo, input.CustomerNo, input.Mode, input.Mode, input.SafeStatus, input.SafeStatus, input.SafeStatus, input.SafeStatus, input.SafeStatus, input.SafeStatus}
	var total, offset int
	if input.Page > 0 {
		offset = (input.Page - 1) * input.Limit
	}
	err = s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		if _, readErr := s.database.Exec(ctx, "SET TRANSACTION ISOLATION LEVEL REPEATABLE READ, READ ONLY"); readErr != nil {
			return readErr
		}
		count, readErr := s.database.GetValue(ctx, "SELECT COUNT(*)"+filtered, args...)
		if readErr != nil {
			return readErr
		}
		total = count.Int()
		if offset >= total {
			return nil
		}
		pageArgs := append(append([]any(nil), args...), boundary, boundary, input.Limit+1, offset)
		return s.database.GetScan(ctx, &rows, `SELECT intent_no,order_no,order_item_no,customer_no,product_name_zh_cn,product_name_en_us,
       strategy_code,granularity_code,status_code,quantity,unit_sequence,version,financially_settled_at,updated_at,
       EXISTS(SELECT 1 FROM plugin_idc_core_product_fulfillment_task task
              WHERE task.tenant_id=0 AND task.intent_no=intent.intent_no AND task.takeover_requested) AS takeover_requested`+filtered+
			` AND (?='' OR intent_no>?) ORDER BY intent_no LIMIT ? OFFSET ?`, pageArgs...)
	})
	if err != nil {
		return nil, errorsNew("product_fulfillment_management_unavailable")
	}
	page := &IntentPage{Items: make([]IntentDetail, 0, min(len(rows), input.Limit)), Total: total}
	pageRows := rows
	if len(pageRows) > input.Limit {
		pageRows = pageRows[:input.Limit]
	}
	customerNos := make([]string, 0, len(pageRows))
	for _, row := range pageRows {
		customerNos = append(customerNos, row.CustomerNo)
	}
	customers, projectionErr := customerprojection.Load(ctx, s.database, customerNos)
	if projectionErr != nil {
		return nil, errorsNew("product_fulfillment_management_unavailable")
	}
	for index := 0; index < len(rows) && index < input.Limit; index++ {
		if safeIntentStatus(rows[index].StatusCode) == "" {
			return nil, errorsNew("product_fulfillment_management_unavailable")
		}
		item := rows[index].safe()
		customer := customers[item.CustomerNo]
		item.CustomerCode, item.CustomerDisplayName, item.CustomerPrimaryPhone = customer.CustomerCode, customer.DisplayName, customer.PrimaryPhone
		page.Items = append(page.Items, *item)
	}
	if len(rows) > input.Limit {
		page.NextCursor, err = s.cursor.encode(rows[input.Limit-1].IntentNo, cursorDigest)
		if err != nil {
			return nil, errorsNew("product_fulfillment_management_unavailable")
		}
	}
	return page, nil
}

func normalizeIntentListInput(input ListInput) (ListInput, error) {
	if input.Limit == 0 {
		input.Limit = 20
	}
	if input.Limit < 1 || input.Limit > maximumManagementPageSize || input.Page < 0 ||
		(input.Page > 0 && (input.Cursor != "" || input.Page-1 > math.MaxInt/input.Limit)) ||
		(input.OrderNo != "" && !managementReferencePattern.MatchString(input.OrderNo)) ||
		(input.CustomerNo != "" && !managementReferencePattern.MatchString(input.CustomerNo)) || !validManagementMode(input.Mode) ||
		(input.SafeStatus != "" && !validSafeStatus(input.SafeStatus)) {
		return ListInput{}, errorsNew("product_fulfillment_action_invalid")
	}
	return input, nil
}

func validSafeStatus(value string) bool {
	return value == "awaiting_review" || value == "processing" || value == "completed" || value == "not_completed" || value == "cancelled"
}

func validManagementMode(value string) bool {
	return value == "" || value == StrategyManual || value == StrategyApprovalThenPlugin || value == StrategyPaymentConfirmedPlugin
}

func validSafeNote(value string, required bool) bool {
	count := utf8.RuneCountInString(value)
	if count > 512 || (required && count < 10) {
		return false
	}
	for _, r := range value {
		if unicode.IsControl(r) {
			return false
		}
	}
	return true
}

var _ ManagementService = (*serviceImpl)(nil)
