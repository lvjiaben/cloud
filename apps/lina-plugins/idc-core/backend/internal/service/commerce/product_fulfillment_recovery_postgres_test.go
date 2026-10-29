// This file proves audited resource preparation recovery without suppliers.
// Each scenario owns its paid order, schema, failed attempts and cleanup.

package commerce

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/cap/provisionresourcecap"
	billingservice "lina-plugin-idc-core/backend/internal/service/billing"
	productfulfillmentservice "lina-plugin-idc-core/backend/internal/service/productfulfillment"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
)

// preparationBlockedSource fails before the resource owner's idempotent task creation.
type preparationBlockedSource struct {
	provisionresourcecap.Source
	blocked bool
	calls   int
}

func (s *preparationBlockedSource) CreateTask(ctx context.Context, request provisionresourcecap.TaskRequest, corrections ...*provisionresourcecap.RecoveryConfiguration) (*provisionresourcecap.Task, error) {
	s.calls++
	if s.blocked {
		return nil, errors.New("local preparation unavailable")
	}
	return s.Source.CreateTask(ctx, request, corrections...)
}

// recoveryFailingAudit proves that no workflow change survives an audit failure.
type recoveryFailingAudit struct{ fulfillmentAuditStub }

func (recoveryFailingAudit) AppendBusinessTransition(context.Context, idccap.BusinessAuditTransition) error {
	return errors.New("audit unavailable")
}

// recoveryRestrictedAuthorizer exercises the command's exact permission and scope.
type recoveryRestrictedAuthorizer struct {
	fulfillmentAuthorizerStub
	authorization productfulfillmentservice.Authorization
}

func (a recoveryRestrictedAuthorizer) AuthorizeRecovery(context.Context, string) (productfulfillmentservice.Authorization, error) {
	return a.authorization, nil
}

func TestProductFulfillmentPreparationRecoveryPreservesHistoryPostgreSQL(t *testing.T) {
	prepared := prepareCrossPluginServerScenario(t)
	scenario := prepared.checkout
	configureResourceChainDatabase(t, scenario.ctx, scenario.database)
	extensions, resourceJob, source := crossPluginRuntime(t, scenario.ctx)
	blocked := &preparationBlockedSource{Source: source, blocked: true}
	sealer, err := productfulfillmentservice.NewProviderEvidenceSealer("test-key-v1", []byte("0123456789abcdef0123456789abcdef"))
	if err != nil {
		t.Fatal(err)
	}
	worker, err := productfulfillmentservice.NewWorker(scenario.database, extensions, &fulfillmentWorkerContexts{}, sealer, fulfillmentSystemReasons(t), blocked, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	for n := 0; n < 20; n++ {
		advanceFulfillmentTaskClock(t, scenario, prepared.order.OrderNo)
		report, runErr := worker.Run(scenario.ctx, 1)
		if runErr != nil || report.Selected != 1 {
			t.Fatalf("preparation attempt %d: %+v %v", n, report, runErr)
		}
	}
	intentValue, err := scenario.database.GetValue(scenario.ctx, `SELECT intent_no FROM plugin_idc_core_product_fulfillment_intent WHERE order_no=?`, prepared.order.OrderNo)
	if err != nil {
		t.Fatal(err)
	}
	intentNo := intentValue.String()
	detail, err := prepared.service.Detail(scenario.ctx, intentNo)
	if err != nil || !detail.CanRecoverPreparation {
		t.Fatalf("failed preparation not recoverable: %+v %v", detail, err)
	}
	input := productfulfillmentservice.ActionInput{IntentNo: intentNo, ExpectedVersion: detail.Version, IdempotencyKey: "recovery-preparation-test-0001", ReasonCode: "preparation_resolved", Note: "Local provisioning prerequisites repaired"}
	for _, authorization := range []productfulfillmentservice.Authorization{
		fulfillmentAuthorization(productfulfillmentservice.PermissionManualTakeover),
		{ActorUserID: 101, RequiredPermission: productfulfillmentservice.PermissionPreparationRecover, DataScope: "assigned_customers", AllowedCustomerNos: []string{"customer-other"}},
	} {
		restricted, setupErr := productfulfillmentservice.NewFullConfigured(scenario.database, fulfillmentFrozenStub{}, fulfillmentGateStub{}, fulfillmentSystemReasons(t), recoveryRestrictedAuthorizer{authorization: authorization}, fulfillmentAuditStub{}, fulfillmentActorsStub{}, []byte("0123456789abcdef0123456789abcdef"), nil)
		if setupErr != nil {
			t.Fatal(setupErr)
		}
		if _, restrictedErr := restricted.RecoverPreparation(scenario.ctx, input); restrictedErr == nil {
			t.Fatal("recovery bypassed its exact permission or customer scope")
		}
	}
	failing, err := productfulfillmentservice.NewFullConfigured(scenario.database, fulfillmentFrozenStub{}, fulfillmentGateStub{}, fulfillmentSystemReasons(t), fulfillmentAuthorizerStub{}, recoveryFailingAudit{}, fulfillmentActorsStub{}, []byte("0123456789abcdef0123456789abcdef"), nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = failing.RecoverPreparation(scenario.ctx, input); err == nil {
		t.Fatal("audit failure committed recovery")
	}
	unchanged, err := prepared.service.Detail(scenario.ctx, intentNo)
	if err != nil || unchanged.Version != detail.Version || !unchanged.CanRecoverPreparation {
		t.Fatalf("audit failure changed task: %+v %v", unchanged, err)
	}
	recovered, err := prepared.service.RecoverPreparation(scenario.ctx, input)
	if err != nil || recovered.Version != detail.Version+1 || recovered.CanRecoverPreparation {
		t.Fatalf("recovery: %+v %v", recovered, err)
	}
	replay, err := prepared.service.RecoverPreparation(scenario.ctx, input)
	if err != nil || replay.Version != recovered.Version {
		t.Fatalf("same request was not idempotent: %+v %v", replay, err)
	}
	var counts struct{ Total, Queries, Submits, QueryBaseline, SubmitBaseline, Recoveries, Commands int }
	if err = scenario.database.GetScan(scenario.ctx, &counts, `SELECT task.attempt_count AS total,task.query_attempt_count AS queries,task.submit_attempt_count AS submits,task.recovery_query_baseline AS query_baseline,task.recovery_submit_baseline AS submit_baseline,task.recovery_count AS recoveries,(SELECT count(*) FROM plugin_idc_core_product_fulfillment_command command WHERE command.intent_no=task.intent_no AND action_code='recover_preparation') AS commands FROM plugin_idc_core_product_fulfillment_task task WHERE intent_no=?`, intentNo); err != nil {
		t.Fatal(err)
	}
	if counts.Total != 20 || counts.Queries != 10 || counts.Submits != 10 || counts.QueryBaseline != 10 || counts.SubmitBaseline != 10 || counts.Recoveries != 1 || counts.Commands != 1 {
		t.Fatalf("history was reset or duplicated: %+v", counts)
	}
	// Repeated polling must not hide the three recovery boundaries or completion.
	for round := 0; round < 2; round++ {
		for attempt := 0; attempt < 20; attempt++ {
			advanceFulfillmentTaskClock(t, scenario, prepared.order.OrderNo)
			if _, runErr := worker.Run(scenario.ctx, 1); runErr != nil {
				t.Fatal(runErr)
			}
		}
		next, detailErr := prepared.service.Detail(scenario.ctx, intentNo)
		if detailErr != nil || !next.CanRecoverPreparation {
			t.Fatalf("next recovery unavailable: %+v %v", next, detailErr)
		}
		input.ExpectedVersion = next.Version
		input.IdempotencyKey += "x"
		if _, recoveryErr := prepared.service.RecoverPreparation(scenario.ctx, input); recoveryErr != nil {
			t.Fatal(recoveryErr)
		}
	}
	blocked.blocked = false
	first, err := worker.Run(scenario.ctx, 1)
	if err != nil || first.Selected != 1 || first.Scheduled != 1 || blocked.calls != 30 {
		t.Fatalf("recovery did not query first: %+v calls=%d %v", first, blocked.calls, err)
	}
	advanceFulfillmentTaskClock(t, scenario, prepared.order.OrderNo)
	if _, err = worker.Run(scenario.ctx, 1); err != nil {
		t.Fatal(err)
	}
	if err = resourceJob(scenario.ctx); err != nil {
		t.Fatal(err)
	}
	advanceFulfillmentTaskClock(t, scenario, prepared.order.OrderNo)
	completed, err := worker.Run(scenario.ctx, 1)
	if err != nil || completed.Completed != 1 || blocked.calls != 31 || extensions.invocationCount(resourcecap.DynamicProviderMethodExecute) != 1 {
		t.Fatalf("recovered provision: %+v calls=%d %v", completed, blocked.calls, err)
	}
	// Replaying the budget/recovery migrations also works with counters above ten.
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate recovery migrations")
	}
	for _, migration := range []string{"136-fulfillment-attempt-budget.sql", "137-fulfillment-preparation-recovery.sql"} {
		sql, readErr := os.ReadFile(filepath.Join(filepath.Dir(filename), "../../../../manifest/sql", migration))
		if readErr != nil {
			t.Fatal(readErr)
		}
		if _, migrationErr := scenario.database.Exec(scenario.ctx, string(sql)); migrationErr != nil {
			t.Fatalf("replay %s after recovery: %v", migration, migrationErr)
		}
	}
	input.IdempotencyKey = strings.ReplaceAll(input.IdempotencyKey, "0001", "0002")
	current, err := prepared.service.Detail(scenario.ctx, intentNo)
	if err != nil {
		t.Fatalf("latest audit projection: %+v %v", current, err)
	}
	wantEvents := []string{"intent_created", "automation_started", "automation_processing", "preparation_recovered", "automation_processing", "preparation_recovered", "automation_processing", "preparation_recovered", "automation_processing", "automation_completed"}
	if len(current.Events) != len(wantEvents) {
		t.Fatalf("polling obscured recovery history: %+v", current.Events)
	}
	for index, want := range wantEvents {
		if current.Events[index].TypeCode != want {
			t.Fatalf("event %d = %s, want %s", index, current.Events[index].TypeCode, want)
		}
	}
	attemptCount, err := scenario.database.GetValue(scenario.ctx, `SELECT count(*) FROM plugin_idc_core_product_fulfillment_attempt attempt JOIN plugin_idc_core_product_fulfillment_task task ON task.task_no=attempt.task_no WHERE task.intent_no=?`, intentNo)
	if err != nil || attemptCount.Int() != 63 {
		t.Fatalf("timeline projection discarded authoritative attempts: %v %v", attemptCount, err)
	}
	input.ExpectedVersion = current.Version
	if _, err = prepared.service.RecoverPreparation(scenario.ctx, input); err == nil {
		t.Fatal("completed resource was replayed")
	}
}

// recoveryStatusSource projects a resource task whose submission crossed the
// owner boundary; neither accepted nor unknown results may become preparation.
type recoveryStatusSource struct {
	provisionresourcecap.Source
	unknown bool
}

func (s recoveryStatusSource) CreateTask(ctx context.Context, request provisionresourcecap.TaskRequest, corrections ...*provisionresourcecap.RecoveryConfiguration) (*provisionresourcecap.Task, error) {
	task, err := s.Source.CreateTask(ctx, request, corrections...)
	if err == nil && task != nil && s.unknown {
		task.Status = provisionresourcecap.TaskStatusResultUnknown
	}
	return task, err
}

func TestProductFulfillmentRecoveryRejectsAcceptedAndUnknownPostgreSQL(t *testing.T) {
	for _, unknown := range []bool{false, true} {
		name := "accepted"
		if unknown {
			name = "unknown"
		}
		t.Run(name, func(t *testing.T) {
			prepared := prepareCrossPluginServerScenario(t)
			scenario := prepared.checkout
			configureResourceChainDatabase(t, scenario.ctx, scenario.database)
			extensions, _, source := crossPluginRuntime(t, scenario.ctx)
			sealer, err := productfulfillmentservice.NewProviderEvidenceSealer("test-key-v1", []byte("0123456789abcdef0123456789abcdef"))
			if err != nil {
				t.Fatal(err)
			}
			worker, err := productfulfillmentservice.NewWorker(scenario.database, extensions, &fulfillmentWorkerContexts{}, sealer, fulfillmentSystemReasons(t), recoveryStatusSource{Source: source, unknown: unknown}, nil, nil)
			if err != nil {
				t.Fatal(err)
			}
			for attempt := 0; attempt < 2; attempt++ {
				advanceFulfillmentTaskClock(t, scenario, prepared.order.OrderNo)
				report, runErr := worker.Run(scenario.ctx, 1)
				if runErr != nil || report.Selected != 1 {
					t.Fatalf("submit state: %+v %v", report, runErr)
				}
			}
			intent, err := scenario.database.GetValue(scenario.ctx, `SELECT intent_no FROM plugin_idc_core_product_fulfillment_intent WHERE order_no=?`, prepared.order.OrderNo)
			if err != nil {
				t.Fatal(err)
			}
			detail, err := prepared.service.Detail(scenario.ctx, intent.String())
			if err != nil || detail.CanRecoverPreparation {
				t.Fatalf("unsafe recovery was offered: %+v %v", detail, err)
			}

			staff, setupErr := billingservice.NewManagementRefundService(scenario.database, refundManagementAuthorizerStub{actor: 9002}, &refundRecentAuthStub{actor: 9002}, refundBusinessAuditStub{}, newFinancialSettlementCreditService(t, scenario))
			if setupErr != nil {
				t.Fatal(setupErr)
			}
			if _, refundErr := staff.RefundFailedFulfillment(scenario.ctx, billingservice.FailedFulfillmentRefundInput{IntentNo: intent.String(), ExpectedVersion: detail.Version, IdempotencyKey: "unsafe-refund-state-" + name, Note: "This unknown or accepted task must not refund"}); refundErr == nil {
				t.Fatal("unknown or accepted delivery was refunded")
			}
			input := productfulfillmentservice.ActionInput{IntentNo: intent.String(), ExpectedVersion: detail.Version, IdempotencyKey: "recovery-unsafe-state-" + name, ReasonCode: "preparation_resolved", Note: "Should be rejected after owner submission"}
			if _, err = prepared.service.RecoverPreparation(scenario.ctx, input); err == nil {
				t.Fatal("accepted/unknown resource replayed")
			}
			commands, err := scenario.database.GetValue(scenario.ctx, `SELECT count(*) FROM plugin_idc_core_product_fulfillment_command WHERE intent_no=?`, intent.String())
			if err != nil || commands.Int() != 0 {
				t.Fatalf("unsafe recovery appended command: %v %v", commands, err)
			}
		})
	}
}
