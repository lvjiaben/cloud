// This file proves staff choices after rejected delivery using isolated schemas
// and a simulated provider. No test sends a supplier request or modifies live orders.

package commerce

import (
	entspi "lina-plugin-idc-core/backend/cap/entitlementcreatedcap/spi"
	"lina-plugin-idc-core/backend/cap/provisionresourcecap"
	billingservice "lina-plugin-idc-core/backend/internal/service/billing"
	productfulfillmentservice "lina-plugin-idc-core/backend/internal/service/productfulfillment"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestRejectedDeliveryStaffChoicePostgreSQL(t *testing.T) {
	for _, localFailure := range []bool{false, true} {
		for _, refund := range []bool{false, true} {
			name := "reprovision"
			if refund {
				name = "refund"
			}
			if localFailure {
				name = "local-not-sent-" + name
			}
			t.Run(name, func(t *testing.T) {
				prepared := prepareCrossPluginServerScenario(t)
				scenario := prepared.checkout
				ctx, db := scenario.ctx, scenario.database
				configureResourceChainDatabase(t, ctx, db)
				extensions, job, source := crossPluginRuntime(t, ctx)
				extensions.rejectFirst = !localFailure
				extensions.unknownNoResponse = localFailure
				failureCode := "upstream_request_rejected"
				if localFailure {
					failureCode = provisionresourcecap.FailureRequestNotSent
				}
				sealer, err := productfulfillmentservice.NewProviderEvidenceSealer("test-key-v1", []byte("0123456789abcdef0123456789abcdef"))
				if err != nil {
					t.Fatal(err)
				}
				worker, err := productfulfillmentservice.NewWorker(db, extensions, &fulfillmentWorkerContexts{}, sealer, fulfillmentSystemReasons(t), source, nil, nil)
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
				detail, err := prepared.service.Detail(ctx, intent.String())
				if err != nil || !detail.CanRecoverPreparation || detail.StatusCode != "not_completed" {
					var diag []struct{ TaskNo, StatusCode, FailureCode, PhaseCode, LastResultCode string }
					if diagErr := db.GetScan(ctx, &diag, `SELECT task_no,status_code,failure_code,'' AS phase_code,'' AS last_result_code FROM plugin_idc_resource_task`); diagErr != nil {
						t.Fatal(diagErr)
					}
					t.Fatalf("rejected detail status=%s execution=%+v resource=%+v invocations=%+v err=%v", detail.StatusCode, detail.Execution, diag, extensions.invocations, err)
				}
				if detail.Execution == nil || detail.Execution.FailureCode != failureCode {
					t.Fatalf("resource rejection reason did not reach authorized detail: %+v", detail.Execution)
				}
				outcomes, err := productfulfillmentservice.NewResourceOutcomeWorker(db, source)
				if err != nil {
					t.Fatal(err)
				}
				reported, err := outcomes.Run(ctx, 1)
				if err != nil || reported.Reported != 1 {
					t.Fatalf("failed outcome=%+v %v", reported, err)
				}
				input := productfulfillmentservice.ActionInput{IntentNo: intent.String(), ExpectedVersion: detail.Version, IdempotencyKey: "staff-rejected-recovery-0001", ReasonCode: "preparation_resolved", Note: "Original delivery prerequisites repaired"}
				if refund {
					staff, setupErr := billingservice.NewManagementRefundService(db, refundManagementAuthorizerStub{actor: 9002}, &refundRecentAuthStub{actor: 9002}, refundBusinessAuditStub{}, newFinancialSettlementCreditService(t, scenario))
					if setupErr != nil {
						t.Fatal(setupErr)
					}
					request := billingservice.FailedFulfillmentRefundInput{IntentNo: intent.String(), ExpectedVersion: detail.Version, IdempotencyKey: "staff-failed-refund-0001", Note: "Provider rejected creation; customer will be refunded"}

					denied, setupErr := billingservice.NewManagementRefundService(db, refundManagementAuthorizerStub{actor: 0}, &refundRecentAuthStub{actor: 9002}, refundBusinessAuditStub{}, newFinancialSettlementCreditService(t, scenario))
					if setupErr != nil {
						t.Fatal(setupErr)
					}
					if _, denyErr := denied.RefundFailedFulfillment(ctx, request); denyErr == nil {
						t.Fatal("refund accepted without staff actor")
					}
					stale := request
					stale.ExpectedVersion++
					if _, staleErr := staff.RefundFailedFulfillment(ctx, stale); staleErr == nil {
						t.Fatal("stale refund accepted")
					}
					failing, setupErr := billingservice.NewManagementRefundService(db, refundManagementAuthorizerStub{actor: 9002}, &refundRecentAuthStub{actor: 9002}, recoveryFailingAudit{}, newFinancialSettlementCreditService(t, scenario))
					if setupErr != nil {
						t.Fatal(setupErr)
					}
					if _, auditErr := failing.RefundFailedFulfillment(ctx, request); auditErr == nil {
						t.Fatal("refund survived audit failure")
					}
					count, countErr := db.GetValue(ctx, `SELECT COUNT(*) FROM plugin_idc_core_refund_request WHERE order_no=?`, prepared.order.OrderNo)
					if countErr != nil || count.Int() != 0 {
						t.Fatalf("failed command persisted refund: %v %v", count, countErr)
					}
					created, createErr := staff.RefundFailedFulfillment(ctx, request)
					if createErr != nil || created.Status != "pending_review" || created.AmountMinor != prepared.order.TotalAmountMinor {
						t.Fatalf("refund=%+v %v", created, createErr)
					}
					replay, replayErr := staff.RefundFailedFulfillment(ctx, request)
					if replayErr != nil || !replay.Reused || replay.RefundNo != created.RefundNo {
						t.Fatalf("refund replay=%+v %v", replay, replayErr)
					}

					duplicate := request
					duplicate.IdempotencyKey += "-duplicate"
					if _, duplicateErr := staff.RefundFailedFulfillment(ctx, duplicate); duplicateErr == nil {
						t.Fatal("duplicate item refund accepted")
					}
					if _, retryErr := prepared.service.RecoverPreparation(ctx, input); retryErr == nil {
						t.Fatal("refund and reprovision both accepted")
					}
					return
				}
				next, err := prepared.service.RecoverPreparation(ctx, input)
				if err != nil {
					t.Fatal(err)
				}
				replay, err := prepared.service.RecoverPreparation(ctx, input)
				if err != nil || replay.Version != next.Version {
					t.Fatalf("recovery replay=%+v %v", replay, err)
				}
				run()
				run()
				if err = job(ctx); err != nil {
					t.Fatal(err)
				}
				run()
				completed, err := prepared.service.Detail(ctx, intent.String())
				if err != nil || completed.StatusCode != "completed" {
					t.Fatalf("completion=%+v %v", completed, err)
				}
				reported, err = outcomes.Run(ctx, 4)
				if err != nil || reported.Reported != 1 {
					t.Fatalf("success outcome=%+v %v", reported, err)
				}
				var proof struct{ Tasks, Failed, Success, Resources, Settlements, Entitlements, Outcomes int }
				err = db.GetScan(ctx, &proof, `SELECT COUNT(*) AS tasks,COUNT(*) FILTER(WHERE status_code='failed') AS failed,COUNT(*) FILTER(WHERE status_code='succeeded') AS success,COUNT(DISTINCT local_resource_no) AS resources,(SELECT COUNT(*) FROM plugin_idc_core_financial_settlement WHERE order_no=?) AS settlements,(SELECT COUNT(*) FROM plugin_idc_core_service_entitlement WHERE source_intent_no=?) AS entitlements,(SELECT COUNT(*) FROM plugin_idc_core_product_fulfillment_resource_outcome_outbox WHERE status_code='reported') AS outcomes FROM plugin_idc_resource_task WHERE owner_metadata->>'operationNo'=?`, prepared.order.OrderNo, intent.String(), intent.String())
				if err != nil || proof.Tasks != 2 || proof.Failed != 1 || proof.Success != 1 || proof.Resources != 1 || proof.Settlements != 1 || proof.Entitlements != 1 || proof.Outcomes != 2 || extensions.invocationCount(resourcecap.DynamicProviderMethodExecute) != 2 {
					t.Fatalf("proof=%+v %v", proof, err)
				}
				router, routerErr := entspi.Freeze().Build(ctx)
				if routerErr != nil {
					t.Fatal(routerErr)
				}
				entitlements, setupErr := productfulfillmentservice.NewEntitlementCreatedWorker(db, router)
				if setupErr != nil {
					t.Fatal(setupErr)
				}
				delivered, deliveryErr := entitlements.Run(ctx, 1)
				if deliveryErr != nil || delivered.Delivered != 1 {
					t.Fatalf("recovered entitlement projection=%+v %v", delivered, deliveryErr)
				}

				_, filename, _, ok := runtime.Caller(0)
				if !ok {
					t.Fatal("locate recovery migrations")
				}
				for _, migration := range []string{"../../../../manifest/sql/139-rejected-fulfillment-recovery.sql", "../../../../manifest/sql/140-fulfillment-configuration-correction.sql", "../../../../../idc-resource/manifest/sql/037-rejected-provision-recovery.sql"} {
					sql, readErr := os.ReadFile(filepath.Join(filepath.Dir(filename), migration))
					if readErr != nil {
						t.Fatal(readErr)
					}
					for replay := 0; replay < 2; replay++ {
						if _, migrationErr := db.Exec(ctx, string(sql)); migrationErr != nil {
							t.Fatalf("migration replay %s: %v", migration, migrationErr)
						}
					}
				}
				// Original paid order and invoice still hold the same amount.
				status, err := db.GetValue(ctx, `SELECT status_code FROM plugin_idc_core_commerce_order WHERE order_no=?`, prepared.order.OrderNo)
				if err != nil || !strings.EqualFold(status.String(), "paid") {
					t.Fatalf("order changed: %v %v", status, err)
				}
			})
		}
	}
}
