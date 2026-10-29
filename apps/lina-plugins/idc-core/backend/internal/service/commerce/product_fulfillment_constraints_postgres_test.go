package commerce

import (
	"context"
	"strings"
	"sync"
	"testing"

	"github.com/gogf/gf/v2/database/gdb"

	"lina-plugin-idc-core/backend/cap/idccap"
	billingservice "lina-plugin-idc-core/backend/internal/service/billing"
	"lina-plugin-idc-core/backend/internal/service/productautomation"
	productfulfillmentservice "lina-plugin-idc-core/backend/internal/service/productfulfillment"
)

type fulfillmentFrozenStub struct{}

func (fulfillmentFrozenStub) Validate(context.Context, productautomation.FrozenCustomerConfigurationInput) (*productautomation.FrozenCustomerConfigurationEvidence, error) {
	return &productautomation.FrozenCustomerConfigurationEvidence{}, nil
}
func (fulfillmentFrozenStub) ValidateBatch(_ context.Context, inputs []productautomation.FrozenCustomerConfigurationInput) ([]productautomation.FrozenCustomerConfigurationEvidence, error) {
	return make([]productautomation.FrozenCustomerConfigurationEvidence, len(inputs)), nil
}

type fulfillmentGateStub struct{}

func (fulfillmentGateStub) EvaluateFulfillmentRelease(_ context.Context, _ string, itemNos []string) (map[string]bool, error) {
	result := make(map[string]bool, len(itemNos))
	for _, itemNo := range itemNos {
		result[itemNo] = true
	}
	return result, nil
}

type fulfillmentAuditStub struct{}

func (fulfillmentAuditStub) AppendBusinessTransition(context.Context, idccap.BusinessAuditTransition) error {
	return nil
}
func (fulfillmentAuditStub) AppendBusinessBatchTransition(context.Context, idccap.BusinessAuditBatchTransition) (idccap.BusinessAuditAppendResult, error) {
	return idccap.BusinessAuditAppendResult{}, nil
}

type fulfillmentActorsStub struct{}

func (fulfillmentActorsStub) DisplayNames(_ context.Context, ids []int) (map[int]string, error) {
	result := make(map[int]string, len(ids))
	for _, id := range ids {
		result[id] = "operator"
	}
	return result, nil
}

type fulfillmentAuthorizerStub struct{}

func fulfillmentAuthorization(permission string) productfulfillmentservice.Authorization {
	return productfulfillmentservice.Authorization{ActorUserID: 101, RequiredPermission: permission, DataScope: "platform_all", AllCustomers: true}
}
func (fulfillmentAuthorizerStub) AuthorizeList(context.Context) (productfulfillmentservice.Authorization, error) {
	return fulfillmentAuthorization(productfulfillmentservice.PermissionTaskList), nil
}
func (fulfillmentAuthorizerStub) AuthorizeDetail(context.Context, string) (productfulfillmentservice.Authorization, error) {
	return fulfillmentAuthorization(productfulfillmentservice.PermissionTaskQuery), nil
}
func (fulfillmentAuthorizerStub) AuthorizeReviewApprove(context.Context, string) (productfulfillmentservice.Authorization, error) {
	return fulfillmentAuthorization(productfulfillmentservice.PermissionReviewApprove), nil
}
func (fulfillmentAuthorizerStub) AuthorizeReviewReject(context.Context, string) (productfulfillmentservice.Authorization, error) {
	return fulfillmentAuthorization(productfulfillmentservice.PermissionReviewReject), nil
}
func (fulfillmentAuthorizerStub) AuthorizeTakeover(context.Context, string) (productfulfillmentservice.Authorization, error) {
	return fulfillmentAuthorization(productfulfillmentservice.PermissionManualTakeover), nil
}
func (fulfillmentAuthorizerStub) AuthorizeRecovery(context.Context, string) (productfulfillmentservice.Authorization, error) {
	return fulfillmentAuthorization(productfulfillmentservice.PermissionPreparationRecover), nil
}
func (fulfillmentAuthorizerStub) AuthorizeManualConclude(context.Context, string) (productfulfillmentservice.Authorization, error) {
	return fulfillmentAuthorization(productfulfillmentservice.PermissionManualConclude), nil
}

func TestProductFulfillmentIntentConstraintsPostgreSQL(t *testing.T) {
	scenario := newCheckoutTestScenario(t, "customer-commerce-service-1", 101, 2, false, 0)
	insertVerifiedIndividualPaymentIdentity(t, scenario, "customer-commerce-service-1")
	checkout, err := scenario.service.CheckoutAccountQuote(scenario.ctx, CheckoutRequest{QuoteNo: scenario.quote.QuoteNo, IdempotencyKey: "fulfillment-constraint-checkout-0001"})
	if err != nil {
		t.Fatalf("checkout: %v", err)
	}
	seedFinancialSettlementMerchant(t, scenario)
	balance, err := billingservice.NewBalanceService(scenario.database)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = balance.PostConfirmedRecharge(scenario.ctx, billingservice.ConfirmedRechargeInput{CustomerNo: "customer-commerce-service-1", CurrencyCode: "CNY", AmountMinor: checkout.TotalAmountMinor, SourceReference: "fulfillment-constraint-recharge", SourceSnapshotDigest: strings.Repeat("a", 64)}); err != nil {
		t.Fatal(err)
	}
	invoices, err := billingservice.NewInvoiceService(scenario.database)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = invoices.PayFromBalance(scenario.ctx, billingservice.InvoiceBalancePaymentInput{InvoiceNo: checkout.InvoiceNo, SourceReference: "fulfillment-constraint-payment", SourceSnapshotDigest: strings.Repeat("b", 64)}); err != nil {
		t.Fatal(err)
	}
	settlement, err := newFinancialSettlementTestService(t, scenario).SettleOrder(scenario.ctx, checkout.OrderNo)
	if err != nil {
		t.Fatal(err)
	}
	_, err = scenario.database.Exec(scenario.ctx, `INSERT INTO plugin_idc_core_product_fulfillment_intent(tenant_id,intent_no,settlement_no,order_no,order_item_no,customer_no,invoice_no,currency_code,product_no,version_no,product_name_zh_cn,product_name_en_us,quantity,unit_sequence,strategy_code,granularity_code,initial_status_code,status_code,policy_digest,evidence_digest,financially_settled_at) SELECT 0,'pft_constraint_manual',?,item.order_no,item.order_item_no,orders.customer_no,?,orders.currency_code,item.product_no,item.version_no,item.product_name_zh_cn,item.product_name_en_us,quantity.quantity,1,'manual','per_order_item','manual_pending','manual_pending',repeat('c',64),repeat('d',64),? FROM plugin_idc_core_commerce_order_item item JOIN plugin_idc_core_commerce_order orders ON orders.order_no=item.order_no JOIN plugin_idc_core_commerce_order_item_quantity_snapshot quantity ON quantity.order_item_no=item.order_item_no WHERE item.order_no=? ORDER BY item.line_number LIMIT 1`, settlement.SettlementNo, checkout.InvoiceNo, settlement.SettledAt, checkout.OrderNo)
	if err != nil {
		t.Fatalf("seed intent: %v", err)
	}
	if _, err = scenario.database.Exec(scenario.ctx, `INSERT INTO plugin_idc_core_product_fulfillment_task(tenant_id,task_no,intent_no,status_code,maximum_attempts,retry_base_seconds) VALUES(0,'pfk_constraint_manual','pft_constraint_manual','pending_query',10,30)`); err != nil {
		t.Fatalf("seed task: %v", err)
	}
	assertFulfillmentTransactionRejected(t, scenario.ctx, scenario.database, `UPDATE plugin_idc_core_product_fulfillment_task SET phase_code='frozen_unknown',version=version+1 WHERE task_no='pfk_constraint_manual'`)
	assertFulfillmentTransactionRejected(t, scenario.ctx, scenario.database, `UPDATE plugin_idc_core_product_fulfillment_task SET submit_attempt_count=1,attempt_count=1,version=version+1 WHERE task_no='pfk_constraint_manual'; INSERT INTO plugin_idc_core_product_fulfillment_attempt(tenant_id,task_no,attempt_no,action_code,started_at,evidence_digest) VALUES(0,'pfk_constraint_manual',1,'submit',CURRENT_TIMESTAMP,repeat('e',64))`)
	if err = scenario.database.Transaction(scenario.ctx, func(txCtx context.Context, _ gdb.TX) error {
		if _, txErr := scenario.database.Exec(txCtx, `UPDATE plugin_idc_core_product_fulfillment_task SET status_code='querying',lease_token='pfl_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa',lease_expires_at=CURRENT_TIMESTAMP+INTERVAL '1 minute',attempt_count=1,query_attempt_count=1,version=version+1 WHERE task_no='pfk_constraint_manual'`); txErr != nil {
			return txErr
		}
		if _, txErr := scenario.database.Exec(txCtx, `INSERT INTO plugin_idc_core_product_fulfillment_attempt(tenant_id,task_no,attempt_no,action_code,started_at,evidence_digest) VALUES(0,'pfk_constraint_manual',1,'query',CURRENT_TIMESTAMP,repeat('e',64))`); txErr != nil {
			return txErr
		}
		if _, txErr := scenario.database.Exec(txCtx, `UPDATE plugin_idc_core_product_fulfillment_attempt SET finished_at=CURRENT_TIMESTAMP,result_code='provider_result',result_evidence_digest=repeat('f',64),result_status_code='not_found',external_reference='',safe_result_evidence='{"status":"not_found"}'::jsonb,safe_summary='provider_result' WHERE task_no='pfk_constraint_manual' AND attempt_no=1`); txErr != nil {
			return txErr
		}
		if _, txErr := scenario.database.Exec(txCtx, `INSERT INTO plugin_idc_core_product_fulfillment_attempt_provider_evidence(tenant_id,task_no,attempt_no,key_id,nonce,ciphertext,evidence_digest) VALUES(0,'pfk_constraint_manual',1,'key-v1',decode(repeat('aa',12),'hex'),decode(repeat('bb',17),'hex'),repeat('1',64))`); txErr != nil {
			return txErr
		}
		_, txErr := scenario.database.Exec(txCtx, `UPDATE plugin_idc_core_product_fulfillment_task SET status_code='pending_submit',phase_code='submit_allowed',lease_token='',lease_expires_at=NULL,version=version+1 WHERE task_no='pfk_constraint_manual'`)
		return txErr
	}); err != nil {
		t.Fatalf("persist protected provider evidence: %v", err)
	}
	assertFulfillmentTransactionRejected(t, scenario.ctx, scenario.database, `UPDATE plugin_idc_core_product_fulfillment_attempt_provider_evidence SET evidence_digest=repeat('2',64) WHERE task_no='pfk_constraint_manual' AND attempt_no=1`)
	assertFulfillmentTransactionRejected(t, scenario.ctx, scenario.database, `DELETE FROM plugin_idc_core_product_fulfillment_attempt_provider_evidence WHERE task_no='pfk_constraint_manual' AND attempt_no=1`)
	if _, err = scenario.database.Exec(scenario.ctx, `UPDATE plugin_idc_core_product_fulfillment_task SET status_code='manual_takeover',phase_code='frozen_unknown',takeover_requested=TRUE,version=version+1 WHERE task_no='pfk_constraint_manual'`); err != nil {
		t.Fatalf("valid task takeover: %v", err)
	}
	assertFulfillmentTransactionRejected(t, scenario.ctx, scenario.database, `UPDATE plugin_idc_core_product_fulfillment_task SET takeover_requested=FALSE,version=version+1 WHERE task_no='pfk_constraint_manual'`)
	assertFulfillmentTransactionRejected(t, scenario.ctx, scenario.database, `UPDATE plugin_idc_core_product_fulfillment_intent SET approved_by=101 WHERE intent_no='pft_constraint_manual'`)
	assertFulfillmentTransactionRejected(t, scenario.ctx, scenario.database, `UPDATE plugin_idc_core_product_fulfillment_intent SET status_code='queued' WHERE intent_no='pft_constraint_manual'`)
	assertFulfillmentTransactionRejected(t, scenario.ctx, scenario.database, `UPDATE plugin_idc_core_product_fulfillment_intent SET status_code='completed',version=version+1 WHERE intent_no='pft_constraint_manual'`)
	assertFulfillmentTransactionRejected(t, scenario.ctx, scenario.database, `INSERT INTO plugin_idc_core_product_fulfillment_manual_conclusion(tenant_id,conclusion_no,intent_no,outcome_code,reason_code,note,actor_user_id,intent_version) VALUES(0,'pfn_constraint_bad','pft_constraint_manual','unable','manual_fulfillment_unable','unable after manual review',101,2); UPDATE plugin_idc_core_product_fulfillment_intent SET status_code='completed',version=2 WHERE intent_no='pft_constraint_manual'`)
	if err = scenario.database.Transaction(scenario.ctx, func(ctx context.Context, _ gdb.TX) error {
		if _, txErr := scenario.database.Exec(ctx, `INSERT INTO plugin_idc_core_product_fulfillment_manual_conclusion(tenant_id,conclusion_no,intent_no,outcome_code,reason_code,note,actor_user_id,intent_version) VALUES(0,'pfn_constraint_ok','pft_constraint_manual','completed','manual_fulfillment_completed','completed by manual operator',101,2)`); txErr != nil {
			return txErr
		}
		_, txErr := scenario.database.Exec(ctx, `UPDATE plugin_idc_core_product_fulfillment_intent SET status_code='completed',version=2 WHERE intent_no='pft_constraint_manual'`)
		return txErr
	}); err != nil {
		t.Fatalf("valid conclusion: %v", err)
	}
}

func assertFulfillmentTransactionRejected(t *testing.T, ctx context.Context, database gdb.DB, statement string) {
	t.Helper()
	if err := database.Transaction(ctx, func(txCtx context.Context, _ gdb.TX) error { _, txErr := database.Exec(txCtx, statement); return txErr }); err == nil {
		t.Fatalf("expected rejected fulfillment SQL: %s", statement)
	}
}

func TestProductFulfillmentManualConclusionConcurrentReplayPostgreSQL(t *testing.T) {
	scenario := newCheckoutTestScenario(t, "customer-commerce-service-1", 101, 2, false, 0)
	insertVerifiedIndividualPaymentIdentity(t, scenario, "customer-commerce-service-1")
	checkout, err := scenario.service.CheckoutAccountQuote(scenario.ctx, CheckoutRequest{QuoteNo: scenario.quote.QuoteNo, IdempotencyKey: "fulfillment-action-checkout-0001"})
	if err != nil {
		t.Fatal(err)
	}
	seedFinancialSettlementMerchant(t, scenario)
	balance, _ := billingservice.NewBalanceService(scenario.database)
	if _, err = balance.PostConfirmedRecharge(scenario.ctx, billingservice.ConfirmedRechargeInput{CustomerNo: "customer-commerce-service-1", CurrencyCode: "CNY", AmountMinor: checkout.TotalAmountMinor, SourceReference: "fulfillment-action-recharge", SourceSnapshotDigest: strings.Repeat("e", 64)}); err != nil {
		t.Fatal(err)
	}
	invoices, _ := billingservice.NewInvoiceService(scenario.database)
	if _, err = invoices.PayFromBalance(scenario.ctx, billingservice.InvoiceBalancePaymentInput{InvoiceNo: checkout.InvoiceNo, SourceReference: "fulfillment-action-payment", SourceSnapshotDigest: strings.Repeat("f", 64)}); err != nil {
		t.Fatal(err)
	}
	settlement, err := newFinancialSettlementTestService(t, scenario).SettleOrder(scenario.ctx, checkout.OrderNo)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = scenario.database.Exec(scenario.ctx, `INSERT INTO plugin_idc_core_product_fulfillment_intent(tenant_id,intent_no,settlement_no,order_no,order_item_no,customer_no,invoice_no,currency_code,product_no,version_no,product_name_zh_cn,product_name_en_us,quantity,unit_sequence,strategy_code,granularity_code,initial_status_code,status_code,policy_digest,evidence_digest,financially_settled_at) SELECT 0,'pft_action_manual',?,item.order_no,item.order_item_no,orders.customer_no,?,orders.currency_code,item.product_no,item.version_no,item.product_name_zh_cn,item.product_name_en_us,quantity.quantity,1,'manual','per_order_item','manual_pending','manual_pending',repeat('1',64),repeat('2',64),? FROM plugin_idc_core_commerce_order_item item JOIN plugin_idc_core_commerce_order orders ON orders.order_no=item.order_no JOIN plugin_idc_core_commerce_order_item_quantity_snapshot quantity ON quantity.order_item_no=item.order_item_no WHERE item.order_no=? ORDER BY item.line_number LIMIT 1`, settlement.SettlementNo, checkout.InvoiceNo, settlement.SettledAt, checkout.OrderNo); err != nil {
		t.Fatal(err)
	}
	service, err := productfulfillmentservice.NewFullConfigured(scenario.database, fulfillmentFrozenStub{}, fulfillmentGateStub{}, fulfillmentSystemReasons(t), fulfillmentAuthorizerStub{}, fulfillmentAuditStub{}, fulfillmentActorsStub{}, []byte("0123456789abcdef0123456789abcdef"), nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, detailErr := service.Detail(scenario.ctx, "pft_action_manual"); detailErr != nil {
		t.Fatalf("detail before action: %v", detailErr)
	}
	start := make(chan struct{})
	errs := make(chan error, 20)
	var wait sync.WaitGroup
	for range 20 {
		wait.Add(1)
		go func() {
			defer wait.Done()
			<-start
			_, callErr := service.ConcludeManual(scenario.ctx, productfulfillmentservice.ManualConclusionInput{ActionInput: productfulfillmentservice.ActionInput{IntentNo: "pft_action_manual", ExpectedVersion: 1, IdempotencyKey: "manual-conclusion-replay-0001", ReasonCode: "manual_fulfillment_completed", Note: "completed after manual handling"}, Outcome: "completed"})
			errs <- callErr
		}()
	}
	close(start)
	wait.Wait()
	close(errs)
	for callErr := range errs {
		if callErr != nil {
			t.Errorf("concurrent replay: %v", callErr)
		}
	}
	var counts struct{ Commands, Conclusions int }
	if err = scenario.database.GetScan(scenario.ctx, &counts, `SELECT (SELECT count(*) FROM plugin_idc_core_product_fulfillment_command WHERE intent_no='pft_action_manual') commands,(SELECT count(*) FROM plugin_idc_core_product_fulfillment_manual_conclusion WHERE intent_no='pft_action_manual') conclusions`); err != nil || counts.Commands != 1 || counts.Conclusions != 1 {
		t.Fatalf("evidence=%+v err=%v", counts, err)
	}
	if _, err = scenario.database.Exec(scenario.ctx, `UPDATE plugin_idc_core_product_fulfillment_reason_option SET label_zh_cn='后来修改的标签',label_en_us='Later changed label' WHERE action_code='manual_conclude_completed' AND reason_code='manual_fulfillment_completed'`); err != nil {
		t.Fatal(err)
	}
	detail, err := service.Detail(scenario.ctx, "pft_action_manual")
	if err != nil || len(detail.Events) != 2 {
		t.Fatalf("detail history: %+v err=%v", detail, err)
	}
	event := detail.Events[1]
	if event.TypeCode != "manual_completed" || event.ReasonLabelZhCN != "人工履约已完成" || event.ReasonLabelEnUS != "Manual fulfillment completed" {
		t.Fatalf("historical reason label drifted: %+v", event)
	}
	if _, err = service.ConcludeManual(scenario.ctx, productfulfillmentservice.ManualConclusionInput{ActionInput: productfulfillmentservice.ActionInput{IntentNo: "pft_action_manual", ExpectedVersion: 2, IdempotencyKey: "manual-conclusion-invalid-0001", ReasonCode: "manual_fulfillment_completed", Note: "completed after manual handling"}, Outcome: "anything_else"}); err == nil {
		t.Fatal("unknown manual conclusion outcome must fail closed")
	}
}

func TestProductFulfillmentOrderCancellationUsesAuthoritativeFactPostgreSQL(t *testing.T) {
	scenario := newCheckoutTestScenario(t, "customer-commerce-service-1", 101, 1, false, 0)
	insertVerifiedIndividualPaymentIdentity(t, scenario, "customer-commerce-service-1")
	checkout, err := scenario.service.CheckoutAccountQuote(scenario.ctx, CheckoutRequest{QuoteNo: scenario.quote.QuoteNo, IdempotencyKey: "fulfillment-cancel-checkout-0001"})
	if err != nil {
		t.Fatal(err)
	}
	seedFinancialSettlementMerchant(t, scenario)
	balance, _ := billingservice.NewBalanceService(scenario.database)
	if _, err = balance.PostConfirmedRecharge(scenario.ctx, billingservice.ConfirmedRechargeInput{CustomerNo: "customer-commerce-service-1", CurrencyCode: "CNY", AmountMinor: checkout.TotalAmountMinor, SourceReference: "fulfillment-cancel-recharge", SourceSnapshotDigest: strings.Repeat("8", 64)}); err != nil {
		t.Fatal(err)
	}
	invoices, _ := billingservice.NewInvoiceService(scenario.database)
	if _, err = invoices.PayFromBalance(scenario.ctx, billingservice.InvoiceBalancePaymentInput{InvoiceNo: checkout.InvoiceNo, SourceReference: "fulfillment-cancel-payment", SourceSnapshotDigest: strings.Repeat("9", 64)}); err != nil {
		t.Fatal(err)
	}
	settlement, err := newFinancialSettlementTestService(t, scenario).SettleOrder(scenario.ctx, checkout.OrderNo)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = scenario.database.Exec(scenario.ctx, `INSERT INTO plugin_idc_core_product_fulfillment_intent(tenant_id,intent_no,settlement_no,order_no,order_item_no,customer_no,invoice_no,currency_code,product_no,version_no,product_name_zh_cn,product_name_en_us,quantity,unit_sequence,strategy_code,granularity_code,initial_status_code,status_code,configuration_snapshot_no,interface_instance_no,interface_configuration_revision,plugin_id,plugin_release_version,contract_version,release_digest,declaration_digest,provision_method,query_method,policy_digest,evidence_digest,financially_settled_at) SELECT 0,'pft_cancel_pending',?,item.order_no,item.order_item_no,orders.customer_no,?,orders.currency_code,item.product_no,item.version_no,item.product_name_zh_cn,item.product_name_en_us,quantity.quantity,1,'payment_confirmed_plugin','per_order_item','queued','queued','pcs_cancel_snapshot','pfi_0123456789abcdef0123456789abcdef',1,'phone-provider','1.0.0','v1',repeat('a',64),repeat('b',64),'fulfillment.submit','fulfillment.query',repeat('c',64),repeat('d',64),? FROM plugin_idc_core_commerce_order_item item JOIN plugin_idc_core_commerce_order orders ON orders.order_no=item.order_no JOIN plugin_idc_core_commerce_order_item_quantity_snapshot quantity ON quantity.order_item_no=item.order_item_no WHERE item.order_no=? LIMIT 1`, settlement.SettlementNo, checkout.InvoiceNo, settlement.SettledAt, checkout.OrderNo); err != nil {
		t.Fatal(err)
	}
	assertFulfillmentTransactionRejected(t, scenario.ctx, scenario.database, `INSERT INTO plugin_idc_core_product_fulfillment_task(tenant_id,task_no,intent_no,status_code,phase_code,attempt_count,query_attempt_count,maximum_attempts,retry_base_seconds,external_reference,version) VALUES(0,'pfk_cancel_invalid','pft_cancel_pending','accepted','post_submit_query',1,1,10,30,'provider-order-1',1)`)
	if _, err = scenario.database.Exec(scenario.ctx, `INSERT INTO plugin_idc_core_product_fulfillment_task(tenant_id,task_no,intent_no,status_code,maximum_attempts,retry_base_seconds) VALUES(0,'pfk_cancel_pending','pft_cancel_pending','pending_query',10,30)`); err != nil {
		t.Fatal(err)
	}
	if _, err = scenario.database.Exec(scenario.ctx, `INSERT INTO plugin_idc_core_commerce_order_cancellation(tenant_id,cancellation_no,order_no,customer_no,source_code,reason_code,request_digest,status_code,requested_by,completed_at) VALUES(0,'can_fulfillment_cancel_0001',?,'customer-commerce-service-1','admin','manual_rejected',repeat('e',64),'completed',101,CURRENT_TIMESTAMP)`, checkout.OrderNo); err != nil {
		t.Fatal(err)
	}
	service, err := productfulfillmentservice.NewFullConfigured(scenario.database, fulfillmentFrozenStub{}, fulfillmentGateStub{}, fulfillmentSystemReasons(t), fulfillmentAuthorizerStub{}, fulfillmentAuditStub{}, fulfillmentActorsStub{}, []byte("0123456789abcdef0123456789abcdef"), nil)
	if err != nil {
		t.Fatal(err)
	}
	start := make(chan struct{})
	results := make(chan *productfulfillmentservice.OrderCancellationResult, 20)
	errorsSeen := make(chan error, 20)
	var wait sync.WaitGroup
	for range 20 {
		wait.Add(1)
		go func() {
			defer wait.Done()
			<-start
			value, callErr := service.ConsumeOrderCancelled(scenario.ctx, productfulfillmentservice.OrderCancellationInput{OrderNo: checkout.OrderNo, CancellationNo: "can_fulfillment_cancel_0001"})
			results <- value
			errorsSeen <- callErr
		}()
	}
	close(start)
	wait.Wait()
	close(results)
	close(errorsSeen)
	for callErr := range errorsSeen {
		if callErr != nil {
			t.Fatalf("concurrent cancellation: %v", callErr)
		}
	}
	mutations := 0
	for result := range results {
		if result != nil && result.Cancelled == 1 && !result.Reused {
			mutations++
		}
	}
	if mutations != 1 {
		t.Fatalf("concurrent cancellation mutations=%d, want 1", mutations)
	}
	replay, err := service.ConsumeOrderCancelled(scenario.ctx, productfulfillmentservice.OrderCancellationInput{OrderNo: checkout.OrderNo, CancellationNo: "can_fulfillment_cancel_0001"})
	if err != nil || !replay.Reused {
		t.Fatalf("cancellation replay=%+v err=%v", replay, err)
	}
	var evidence struct {
		IntentStatus, TaskStatus, ReasonCode, LabelZhCN, LabelEnUS string
		Events                                                     int
	}
	if err = scenario.database.GetScan(scenario.ctx, &evidence, `SELECT intent.status_code intent_status,task.status_code task_status,transition.reason_code,transition.reason_label_zh_cn label_zh_cn,transition.reason_label_en_us label_en_us,(transition.intent_no IS NOT NULL)::int events FROM plugin_idc_core_product_fulfillment_intent intent JOIN plugin_idc_core_product_fulfillment_task task ON task.intent_no=intent.intent_no LEFT JOIN plugin_idc_core_product_fulfillment_system_transition transition ON transition.intent_no=intent.intent_no AND transition.event_code='cancelled' WHERE intent.intent_no='pft_cancel_pending'`); err != nil || evidence.IntentStatus != "canceled" || evidence.TaskStatus != "canceled" || evidence.Events != 1 || evidence.ReasonCode != productfulfillmentservice.SystemReasonOrderCancelled || evidence.LabelZhCN == "" || evidence.LabelEnUS == "" {
		t.Fatalf("cancellation evidence=%+v err=%v", evidence, err)
	}
	assertFulfillmentTransactionRejected(t, scenario.ctx, scenario.database, `UPDATE plugin_idc_core_product_fulfillment_task SET cancel_requested=FALSE,version=version+1 WHERE task_no='pfk_cancel_pending'`)
}
