// This file verifies staff non-acceptance facts against real isolated resource,
// finance and core persistence. Providers are simulated; live orders are untouched.
package commerce

import (
	"context"
	"errors"
	"github.com/gogf/gf/v2/frame/g"
	"strings"
	"testing"

	"github.com/gogf/gf/v2/database/gdb"
	billingservice "lina-plugin-idc-core/backend/internal/service/billing"
	productfulfillmentservice "lina-plugin-idc-core/backend/internal/service/productfulfillment"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
)

func TestUnknownFulfillmentStaffVerificationPostgreSQL(t *testing.T) {
	for _, refund := range []bool{false, true} {
		name := "corrected_recovery"
		if refund {
			name = "refund"
		}
		t.Run(name, func(t *testing.T) {
			prepared := prepareCrossPluginServerScenario(t)
			scenario := prepared.checkout
			ctx, db := scenario.ctx, scenario.database
			configureResourceChainDatabase(t, ctx, db)
			extensions, job, source := crossPluginRuntime(t, ctx)
			extensions.unknownFirst = true
			corrected := &correctionSource{Source: source}
			sealer, err := productfulfillmentservice.NewProviderEvidenceSealer("test-key-v1", []byte("0123456789abcdef0123456789abcdef"))
			if err != nil {
				t.Fatal(err)
			}
			worker, err := productfulfillmentservice.NewWorker(db, extensions, &fulfillmentWorkerContexts{}, sealer, fulfillmentSystemReasons(t), corrected, nil, nil)
			if err != nil {
				t.Fatal(err)
			}
			service, err := productfulfillmentservice.NewFullConfigured(g.DB(), structuredFulfillmentEvidenceValidator{}, fulfillmentGateStub{}, fulfillmentSystemReasons(t), fulfillmentAuthorizerStub{}, fulfillmentAuditStub{}, fulfillmentActorsStub{}, []byte("0123456789abcdef0123456789abcdef"), corrected)
			if err != nil {
				t.Fatal(err)
			}
			run := func() {
				t.Helper()
				advanceFulfillmentTaskClock(t, scenario, prepared.order.OrderNo)
				if _, runErr := worker.Run(ctx, 1); runErr != nil {
					t.Fatal(runErr)
				}
			}
			run()
			run()
			if err = job(ctx); err != nil {
				t.Fatal(err)
			}
			run()
			intent, err := db.GetValue(ctx, `SELECT intent_no FROM plugin_idc_core_product_fulfillment_intent WHERE order_no=?`, prepared.order.OrderNo)
			if err != nil {
				t.Fatal(err)
			}
			detail, err := service.Detail(ctx, intent.String())
			if err != nil || !detail.CanVerifyNonAcceptance || detail.CanRecoverPreparation || detail.CanRequestRefund {
				t.Fatalf("unknown action projection: %+v %v", detail, err)
			}
			input := productfulfillmentservice.NonAcceptanceInput{ActionInput: productfulfillmentservice.ActionInput{IntentNo: intent.String(), ExpectedVersion: detail.Version, IdempotencyKey: "verify-unknown-no-creation-0001", Note: "Checked the upstream instance list and task history; neither exists"}, ConfirmedNoCreation: true}
			for _, mutate := range []func(*productfulfillmentservice.NonAcceptanceInput){func(i *productfulfillmentservice.NonAcceptanceInput) { i.ConfirmedNoCreation = false }, func(i *productfulfillmentservice.NonAcceptanceInput) { i.Note = "short" }, func(i *productfulfillmentservice.NonAcceptanceInput) { i.ExpectedVersion++ }} {
				invalid := input
				mutate(&invalid)
				if _, err = service.VerifyNonAcceptance(ctx, invalid); err == nil {
					t.Fatal("invalid verification accepted")
				}
			}
			for _, authorization := range []productfulfillmentservice.Authorization{{ActorUserID: 101, RequiredPermission: productfulfillmentservice.PermissionManualConclude, DataScope: "platform_all", AllCustomers: true}, {ActorUserID: 101, RequiredPermission: productfulfillmentservice.PermissionPreparationRecover, DataScope: "assigned", AllowedCustomerNos: []string{"customer-outside-scope"}}} {
				denied, setupErr := productfulfillmentservice.NewFullConfigured(g.DB(), structuredFulfillmentEvidenceValidator{}, fulfillmentGateStub{}, fulfillmentSystemReasons(t), recoveryRestrictedAuthorizer{authorization: authorization}, fulfillmentAuditStub{}, fulfillmentActorsStub{}, []byte("0123456789abcdef0123456789abcdef"), corrected)
				if setupErr != nil {
					t.Fatal(setupErr)
				}
				if _, err = denied.VerifyNonAcceptance(ctx, input); err == nil {
					t.Fatal("verification bypassed permission/customer scope")
				}
			}
			// A known accepted task cannot be overwritten by a checkbox assertion.
			rollback := errors.New("rollback isolated acceptance fixture")
			err = g.DB().Transaction(ctx, func(txCtx context.Context, _ gdb.TX) error {
				if _, insertErr := g.DB().Exec(txCtx, `INSERT INTO plugin_idc_resource_upstream_task_binding(tenant_id,binding_no,task_no,channel_no,external_task_ref,status_code,last_observed_at) SELECT 0,'verified-binding-test',task_no,channel_no,'accepted-upstream-task','accepted',CURRENT_TIMESTAMP FROM plugin_idc_resource_task WHERE task_no=?`, detail.Execution.ResourceTaskNo); insertErr != nil {
					return insertErr
				}
				if _, verifyErr := service.VerifyNonAcceptance(txCtx, input); verifyErr == nil {
					t.Fatal("known accepted task was verified as absent")
				}
				return rollback
			})
			if !errors.Is(err, rollback) {
				t.Fatal(err)
			}
			failing, err := productfulfillmentservice.NewFullConfigured(g.DB(), structuredFulfillmentEvidenceValidator{}, fulfillmentGateStub{}, fulfillmentSystemReasons(t), fulfillmentAuthorizerStub{}, recoveryFailingAudit{}, fulfillmentActorsStub{}, []byte("0123456789abcdef0123456789abcdef"), corrected)
			if err != nil {
				t.Fatal(err)
			}
			if _, err = failing.VerifyNonAcceptance(ctx, input); err == nil || !strings.Contains(err.Error(), "audit unavailable") {
				t.Fatal("verification survived audit failure")
			}
			var rollbackProof struct{ Core, Resource, Outcomes int }
			if err = db.GetScan(ctx, &rollbackProof, `SELECT (SELECT count(*) FROM plugin_idc_core_fulfillment_non_acceptance_verification) AS core,(SELECT count(*) FROM plugin_idc_resource_provision_non_acceptance_verification) AS resource,(SELECT count(*) FROM plugin_idc_core_product_fulfillment_resource_outcome_outbox) AS outcomes`); err != nil || rollbackProof.Core != 0 || rollbackProof.Resource != 0 || rollbackProof.Outcomes != 0 {
				t.Fatalf("cross-owner audit rollback: %+v %v", rollbackProof, err)
			}
			verified, err := service.VerifyNonAcceptance(ctx, input)
			if err != nil || verified.StatusCode != "not_completed" || !verified.CanRecoverPreparation || !verified.CanRequestRefund || verified.CanVerifyNonAcceptance || len(verified.NonAcceptanceVerifications) != 1 {
				t.Fatalf("verified action projection: %+v %v", verified, err)
			}
			if replay, replayErr := service.VerifyNonAcceptance(ctx, input); replayErr != nil || replay.Version != verified.Version {
				t.Fatalf("verification replay: %+v %v", replay, replayErr)
			}
			conflicting := input
			conflicting.Note += " changed"
			if _, err = service.VerifyNonAcceptance(ctx, conflicting); err == nil {
				t.Fatal("conflicting idempotency payload accepted")
			}
			var retained struct {
				Status                 string
				Unknown                bool
				UnknownAttempts, Tasks int
			}
			if err = db.GetScan(ctx, &retained, `SELECT status_code AS status,result_unknown AS unknown,(SELECT count(*) FROM plugin_idc_core_product_fulfillment_attempt WHERE result_status_code='result_unknown') AS unknown_attempts,(SELECT count(*) FROM plugin_idc_resource_task) AS tasks FROM plugin_idc_resource_task WHERE task_no=?`, detail.Execution.ResourceTaskNo); err != nil || retained.Status != "result_unknown" || !retained.Unknown || retained.UnknownAttempts != 1 || retained.Tasks != 1 || extensions.invocationCount(resourcecap.DynamicProviderMethodExecute) != 1 {
				t.Fatalf("unknown history or provider calls changed: %+v %v", retained, err)
			}
			for _, statement := range []string{`UPDATE plugin_idc_core_fulfillment_non_acceptance_verification SET safe_note='rewritten administrator history'`, `DELETE FROM plugin_idc_resource_provision_non_acceptance_verification`} {
				if _, immutabilityErr := db.Exec(ctx, statement); immutabilityErr == nil {
					t.Fatal("immutable verification history changed")
				}
			}
			outcomes, err := productfulfillmentservice.NewResourceOutcomeWorker(db, corrected)
			if err != nil {
				t.Fatal(err)
			}
			if report, reportErr := outcomes.Run(ctx, 1); reportErr != nil || report.Reported != 1 {
				t.Fatalf("verified failure outcome: %+v %v", report, reportErr)
			}
			recovery := productfulfillmentservice.ActionInput{IntentNo: intent.String(), ExpectedVersion: verified.Version, IdempotencyKey: "verified-recovery-command-0001", ReasonCode: "preparation_resolved", Note: "Corrected upstream mapping after administrator verification"}
			if refund {
				staff, setupErr := billingservice.NewManagementRefundService(db, refundManagementAuthorizerStub{actor: 9002}, &refundRecentAuthStub{actor: 9002}, refundBusinessAuditStub{}, newFinancialSettlementCreditService(t, scenario))
				if setupErr != nil {
					t.Fatal(setupErr)
				}
				request := billingservice.FailedFulfillmentRefundInput{IntentNo: intent.String(), ExpectedVersion: verified.Version, IdempotencyKey: "verified-failed-refund-0001", Note: "Administrator checked upstream and verified no delivery"}
				created, createErr := staff.RefundFailedFulfillment(ctx, request)
				if createErr != nil || created.AmountMinor != prepared.order.TotalAmountMinor || created.Status != "pending_review" {
					t.Fatalf("verified refund: %+v %v", created, createErr)
				}
				if _, err = service.RecoverPreparation(ctx, recovery); err == nil {
					t.Fatal("recovery bypassed refund reservation")
				}
				return
			}
			editor, err := service.InspectRecoveryConfiguration(ctx, intent.String(), nil)
			if err != nil {
				t.Fatal(err)
			}
			recovery.RecoveryConfiguration = &editor.Selection
			if _, err = service.RecoverPreparation(ctx, recovery); err != nil {
				t.Fatal(err)
			}
			run()
			run()
			if err = job(ctx); err != nil {
				t.Fatal(err)
			}
			run()
			completed, err := service.Detail(ctx, intent.String())
			if err != nil || completed.StatusCode != "completed" || len(completed.RecoveryConfigurations) != 1 {
				t.Fatalf("verified corrected recovery: %+v %v", completed, err)
			}
			var final struct{ Settlements, Tasks, Resources, Entitlements int }
			if err = db.GetScan(ctx, &final, `SELECT (SELECT count(*) FROM plugin_idc_core_financial_settlement WHERE order_no=?) AS settlements,(SELECT count(*) FROM plugin_idc_resource_task) AS tasks,(SELECT count(DISTINCT local_resource_no) FROM plugin_idc_resource_task) AS resources,(SELECT count(*) FROM plugin_idc_core_service_entitlement WHERE source_intent_no=?) AS entitlements`, prepared.order.OrderNo, intent.String()); err != nil || final.Settlements != 1 || final.Tasks != 2 || final.Resources != 1 || final.Entitlements != 1 {
				t.Fatalf("paid identity duplicated: %+v %v", final, err)
			}
		})
	}
}
