package productfulfillment

import (
	"context"
	"encoding/json"
	"sort"
	"strings"

	"github.com/gogf/gf/v2/database/gdb"
)

type gateTaskPlan struct {
	TaskNo           string `json:"task_no" orm:"task_no"`
	IntentNo         string `json:"intent_no" orm:"intent_no"`
	MaximumAttempts  int    `json:"maximum_attempts" orm:"maximum_attempts"`
	RetryBaseSeconds int    `json:"retry_base_seconds" orm:"retry_base_seconds"`
}

type gateLockedIntent struct {
	IntentNo, OrderItemNo, StatusCode string
}

type gateReleaseEvidence struct {
	IntentNo, StatusCode, TaskNo                                                 string
	MaximumAttempts, RetryBaseSeconds, TaskMaximumAttempts, TaskRetryBaseSeconds int
	SystemTransition                                                             bool
}

func (s *serviceImpl) ReleaseOwnerGates(ctx context.Context, input OwnerGateReleaseInput) (*SettlementResult, error) {
	itemNos, err := normalizeGateReleaseInput(ctx, &input, s.gates != nil && s.frozen != nil)
	if err != nil {
		return nil, err
	}

	var output *SettlementResult
	err = s.database.Transaction(ctx, func(txCtx context.Context, _ gdb.TX) error {
		var transactionErr error
		output, transactionErr = s.releaseOwnerGatesTransaction(txCtx, input.OrderNo, itemNos)
		return transactionErr
	})
	return output, err
}

func normalizeGateReleaseInput(ctx context.Context, input *OwnerGateReleaseInput, dependenciesReady bool) ([]string, error) {
	input.OrderNo = strings.TrimSpace(input.OrderNo)
	itemNos := append([]string(nil), input.OrderItemNos...)
	if ctx == nil || ctx.Err() != nil || !managementReferencePattern.MatchString(input.OrderNo) || len(itemNos) == 0 || len(itemNos) > 100 || !dependenciesReady {
		return nil, errorsNew("product_fulfillment_gate_release_invalid")
	}
	for index := range itemNos {
		itemNos[index] = strings.TrimSpace(itemNos[index])
		if !managementReferencePattern.MatchString(itemNos[index]) {
			return nil, errorsNew("product_fulfillment_gate_release_invalid")
		}
	}
	sort.Strings(itemNos)
	for index := 1; index < len(itemNos); index++ {
		if itemNos[index] == itemNos[index-1] {
			return nil, errorsNew("product_fulfillment_gate_release_invalid")
		}
	}
	return itemNos, nil
}

func (s *serviceImpl) releaseOwnerGatesTransaction(ctx context.Context, orderNo string, itemNos []string) (*SettlementResult, error) {
	rawItems, err := json.Marshal(itemNos)
	if err != nil {
		return nil, err
	}
	settlementValue, err := s.database.GetValue(ctx, `SELECT settlement_no FROM plugin_idc_core_financial_settlement WHERE tenant_id=0 AND order_no=? LIMIT 1`, orderNo)
	if err != nil || settlementValue.String() == "" {
		return nil, errorsNew("product_fulfillment_settlement_unavailable")
	}
	authority, err := s.lockSettlement(ctx, SettlementInput{SettlementNo: settlementValue.String(), OrderNo: orderNo})
	if err != nil {
		return nil, err
	}
	items, err := s.loadFrozenItems(ctx, authority, itemNos)
	if err != nil {
		return nil, err
	}
	if err = s.validateFrozenConfigurations(ctx, items); err != nil {
		return nil, err
	}
	itemEvidence, err := selectedGateEvidence(items, itemNos)
	if err != nil {
		return nil, err
	}
	var locked []gateLockedIntent
	if err = s.database.GetScan(ctx, &locked, `SELECT intent_no,order_item_no,status_code FROM plugin_idc_core_product_fulfillment_intent WHERE tenant_id=0 AND order_no=? AND strategy_code='payment_confirmed_plugin' AND order_item_no IN (SELECT jsonb_array_elements_text(?::jsonb)) ORDER BY intent_no LIMIT 1001 FOR UPDATE`, orderNo, rawItems); err != nil || len(locked) < len(itemNos) || len(locked) > 1000 {
		return nil, errorsNew("product_fulfillment_gate_release_conflict")
	}
	candidates, stopped := gateReleaseCandidates(locked, itemEvidence)
	readyItems, err := s.readyGateItems(ctx, orderNo, candidates)
	if err != nil {
		return nil, err
	}
	if err = s.moveStoppedGateItemsToManual(ctx, orderNo, stopped); err != nil {
		return nil, err
	}
	plans, err := s.queueReadyGateItems(ctx, orderNo, readyItems)
	if err != nil {
		return nil, err
	}
	if err = s.verifyGateRelease(ctx, orderNo, rawItems, locked); err != nil {
		return nil, err
	}
	return &SettlementResult{OrderNo: orderNo, IntentCount: len(locked), TaskCount: len(plans), ManualCount: len(stopped), Reused: len(plans) == 0 && len(stopped) == 0}, nil
}

func selectedGateEvidence(items []frozenItem, itemNos []string) (map[string]frozenItem, error) {
	itemSet := make(map[string]struct{}, len(itemNos))
	for _, itemNo := range itemNos {
		itemSet[itemNo] = struct{}{}
	}
	evidence := make(map[string]frozenItem, len(itemNos))
	for _, item := range items {
		if _, selected := itemSet[item.OrderItemNo]; !selected {
			continue
		}
		if item.StrategyCode != StrategyPaymentConfirmedPlugin {
			return nil, errorsNew("product_fulfillment_gate_release_conflict")
		}
		evidence[item.OrderItemNo] = item
	}
	if len(evidence) != len(itemNos) {
		return nil, errorsNew("product_fulfillment_gate_release_conflict")
	}
	return evidence, nil
}

func gateReleaseCandidates(locked []gateLockedIntent, evidence map[string]frozenItem) ([]string, []string) {
	pending := make(map[string]struct{})
	for _, intent := range locked {
		if intent.StatusCode == "gate_pending" {
			pending[intent.OrderItemNo] = struct{}{}
		}
	}
	candidates, stopped := make([]string, 0, len(pending)), make([]string, 0, len(pending))
	for itemNo := range pending {
		if evidence[itemNo].InterfaceNewBusinessState == "accepting" {
			candidates = append(candidates, itemNo)
		} else {
			stopped = append(stopped, itemNo)
		}
	}
	sort.Strings(candidates)
	sort.Strings(stopped)
	return candidates, stopped
}

func (s *serviceImpl) readyGateItems(ctx context.Context, orderNo string, candidates []string) ([]string, error) {
	if len(candidates) == 0 {
		return nil, nil
	}
	ready, err := evaluateOwnerGates(ctx, s.gates, orderNo, candidates)
	if err != nil {
		return nil, err
	}
	readyItems := make([]string, 0, len(candidates))
	for _, itemNo := range candidates {
		if ready[itemNo] {
			readyItems = append(readyItems, itemNo)
		}
	}
	return readyItems, nil
}

func (s *serviceImpl) moveStoppedGateItemsToManual(ctx context.Context, orderNo string, stopped []string) error {
	if len(stopped) == 0 {
		return nil
	}
	rawStopped, err := json.Marshal(stopped)
	if err != nil {
		return err
	}
	reason, err := s.systemReasons.ResolveSystemTransitionReason(SystemReasonNewBusinessStopped)
	if err != nil {
		return err
	}
	_, err = s.database.Exec(ctx, `WITH changed AS (UPDATE plugin_idc_core_product_fulfillment_intent SET status_code='manual_pending',version=version+1,updated_at=CURRENT_TIMESTAMP WHERE tenant_id=0 AND order_no=? AND status_code='gate_pending' AND order_item_no IN (SELECT jsonb_array_elements_text(?::jsonb)) RETURNING intent_no) INSERT INTO plugin_idc_core_product_fulfillment_system_transition(tenant_id,transition_no,intent_no,event_code,reason_code,reason_label_zh_cn,reason_label_en_us) SELECT 0,'pfs_'||substr(md5(E'\\000'||intent_no),1,32),intent_no,'moved_to_manual',?,?,? FROM changed ON CONFLICT(intent_no,event_code) DO NOTHING`, orderNo, rawStopped, SystemReasonNewBusinessStopped, reason.LabelZhCN, reason.LabelEnUS)
	return err
}

func (s *serviceImpl) queueReadyGateItems(ctx context.Context, orderNo string, readyItems []string) ([]gateTaskPlan, error) {
	if len(readyItems) == 0 {
		return nil, nil
	}
	rawReady, err := json.Marshal(readyItems)
	if err != nil {
		return nil, err
	}
	plans := make([]gateTaskPlan, 0, len(readyItems))
	if err = s.database.GetScan(ctx, &plans, `UPDATE plugin_idc_core_product_fulfillment_intent SET status_code='queued',version=version+1,updated_at=CURRENT_TIMESTAMP WHERE tenant_id=0 AND order_no=? AND status_code='gate_pending' AND order_item_no IN (SELECT jsonb_array_elements_text(?::jsonb)) RETURNING intent_no,maximum_attempts,retry_base_seconds`, orderNo, rawReady); err != nil {
		return nil, err
	}
	for index := range plans {
		plans[index].TaskNo = deterministicNo("pfk_", plans[index].IntentNo)
	}
	rawPlans, err := json.Marshal(plans)
	if err != nil {
		return nil, err
	}
	_, err = s.database.Exec(ctx, `INSERT INTO plugin_idc_core_product_fulfillment_task(tenant_id,task_no,intent_no,status_code,maximum_attempts,retry_base_seconds) SELECT 0,task_no,intent_no,'pending_query',maximum_attempts,retry_base_seconds FROM jsonb_to_recordset(?::jsonb) AS x(task_no text,intent_no text,maximum_attempts int,retry_base_seconds int) ON CONFLICT(intent_no) DO NOTHING`, rawPlans)
	return plans, err
}

func (s *serviceImpl) verifyGateRelease(ctx context.Context, orderNo string, rawItems []byte, locked []gateLockedIntent) error {
	var evidence []gateReleaseEvidence
	if err := s.database.GetScan(ctx, &evidence, `SELECT intent.intent_no,intent.status_code,intent.maximum_attempts,intent.retry_base_seconds,COALESCE(task.task_no,'') AS task_no,COALESCE(task.maximum_attempts,0) AS task_maximum_attempts,COALESCE(task.retry_base_seconds,0) AS task_retry_base_seconds,(system_event.intent_no IS NOT NULL) AS system_transition FROM plugin_idc_core_product_fulfillment_intent intent LEFT JOIN plugin_idc_core_product_fulfillment_system_transition system_event ON system_event.intent_no=intent.intent_no AND system_event.event_code='moved_to_manual' LEFT JOIN plugin_idc_core_product_fulfillment_task task ON task.intent_no=intent.intent_no WHERE intent.tenant_id=0 AND intent.order_no=? AND intent.order_item_no IN (SELECT jsonb_array_elements_text(?::jsonb)) ORDER BY intent.intent_no LIMIT 1001`, orderNo, rawItems); err != nil || len(evidence) != len(locked) {
		return errorsNew("product_fulfillment_task_conflict")
	}
	for _, row := range evidence {
		requiresTask := row.StatusCode == "queued" || row.StatusCode == "processing" || row.StatusCode == "completed" || row.StatusCode == "incomplete" || (row.StatusCode == "manual_pending" && !row.SystemTransition)
		if row.SystemTransition && row.TaskNo != "" {
			return errorsNew("product_fulfillment_task_conflict")
		}
		if requiresTask && (row.TaskNo != deterministicNo("pfk_", row.IntentNo) || row.TaskMaximumAttempts != row.MaximumAttempts || row.TaskRetryBaseSeconds != row.RetryBaseSeconds) {
			return errorsNew("product_fulfillment_task_conflict")
		}
	}
	return nil
}
