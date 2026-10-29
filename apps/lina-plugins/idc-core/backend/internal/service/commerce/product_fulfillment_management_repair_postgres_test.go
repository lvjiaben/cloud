// This file verifies local management repair on a fully paid cross-plugin
// server, preserving the actual provider task, money and immutable workflow.
package commerce

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/gogf/gf/v2/database/gdb"
	"lina-plugin-idc-core/backend/cap/idccap"
	productfulfillmentservice "lina-plugin-idc-core/backend/internal/service/productfulfillment"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
)

// managementRepairAudit retains only the safe business transition for assertions.
type managementRepairAudit struct {
	fulfillmentAuditStub
	events []idccap.BusinessAuditTransition
}

func (a *managementRepairAudit) AppendBusinessTransition(_ context.Context, event idccap.BusinessAuditTransition) error {
	a.events = append(a.events, event)
	return nil
}

func assertLocalManagementRepair(t *testing.T, prepared genericWorkerScenario, extensions *crossPluginBusinessExtensions, worker productfulfillmentservice.EntitlementCreatedWorker) {
	t.Helper()
	ctx, db := prepared.checkout.ctx, prepared.checkout.database
	_, file, _, _ := runtime.Caller(0)
	migration, err := os.ReadFile(filepath.Join(filepath.Dir(file), "..", "..", "..", "..", "manifest", "sql", "143-local-management-repair.sql"))
	if err != nil {
		t.Fatal(err)
	}
	// A second execution must preserve the same command policy and one seed.
	if _, err = db.Exec(ctx, string(migration)); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(ctx, string(migration)); err != nil {
		t.Fatal(err)
	}
	intent, err := db.GetValue(ctx, `SELECT intent_no FROM plugin_idc_core_product_fulfillment_intent WHERE order_no=?`, prepared.order.OrderNo)
	if err != nil {
		t.Fatal(err)
	}
	intentNo := intent.String()
	before, err := prepared.service.Detail(ctx, intentNo)
	if err != nil || before.CanRepairManagement {
		t.Fatalf("completed link eligible: %+v %v", before, err)
	}
	protected := func() string {
		t.Helper()
		value, readErr := db.GetValue(ctx, `SELECT jsonb_build_object('intent',to_jsonb(intent),'task',to_jsonb(task),'order',to_jsonb(orders),'settlement',to_jsonb(settlement))::text
  FROM plugin_idc_core_product_fulfillment_intent intent
  JOIN plugin_idc_core_product_fulfillment_task task ON task.intent_no=intent.intent_no
  JOIN plugin_idc_core_commerce_order orders ON orders.order_no=intent.order_no
  JOIN plugin_idc_core_financial_settlement settlement ON settlement.settlement_no=intent.settlement_no
  WHERE intent.intent_no=?`, intentNo)
		if readErr != nil {
			t.Fatal(readErr)
		}
		return value.String()
	}
	original := protected()
	// The owner already committed its local binding, but the notification reply
	// was lost. Only this test-owned outbox event is changed to model that fault.
	if _, err = db.Exec(ctx, `UPDATE plugin_idc_core_entitlement_created_outbox SET status_code='manual_attention',attempt_count=10,owner_reference='',receipt_evidence_digest='',last_error_code='password=PRIVATE',lease_token='',lease_expires_at=NULL WHERE intent_no=?`, intentNo); err != nil {
		t.Fatal(err)
	}
	current, err := prepared.service.Detail(ctx, intentNo)
	if err != nil || !current.CanRepairManagement || current.ManagementPreparation.FailureCode != "" {
		t.Fatalf("repair candidate: %+v %v", current, err)
	}
	input := productfulfillmentservice.ManagementRepairInput{IntentNo: intentNo, ExpectedVersion: current.Version, IdempotencyKey: "local-management-repair-test-0001"}
	newService := func(auth productfulfillmentservice.ManagementAuthorizer, audit idccap.BusinessAuditWriter) productfulfillmentservice.Service {
		t.Helper()
		service, setupErr := productfulfillmentservice.NewFullConfigured(db, fulfillmentFrozenStub{}, fulfillmentGateStub{}, fulfillmentSystemReasons(t), auth, audit, fulfillmentActorsStub{}, []byte("0123456789abcdef0123456789abcdef"), nil)
		if setupErr != nil {
			t.Fatal(setupErr)
		}
		return service
	}
	for _, authorization := range []productfulfillmentservice.Authorization{
		fulfillmentAuthorization(productfulfillmentservice.PermissionManualTakeover),
		{ActorUserID: 101, RequiredPermission: productfulfillmentservice.PermissionPreparationRecover, DataScope: "assigned_customers", AllowedCustomerNos: []string{"customer-other"}},
	} {
		if _, err = newService(recoveryRestrictedAuthorizer{authorization: authorization}, fulfillmentAuditStub{}).RepairManagement(ctx, input); err == nil {
			t.Fatal("repair bypassed permission or customer scope")
		}
	}
	stale := input
	stale.ExpectedVersion++
	if _, err = prepared.service.RepairManagement(ctx, stale); err == nil {
		t.Fatal("stale workflow accepted")
	}
	// Each invalid state is rolled back, including refund reservation which must
	// block both the candidate projection and the actual command.
	rollback := errors.New("rollback repair boundary fixture")
	for _, statement := range []string{
		`UPDATE plugin_idc_core_entitlement_created_outbox SET status_code='delivering',lease_token='busy',lease_expires_at=CURRENT_TIMESTAMP+INTERVAL '1 minute'`,
		`UPDATE plugin_idc_core_entitlement_created_outbox SET owner_reference='confirmed'`,
		`UPDATE plugin_idc_core_entitlement_created_outbox SET receipt_evidence_digest=repeat('a',64)`,
		`UPDATE plugin_idc_core_service_entitlement SET status_code='terminated'`,
	} {
		err = db.Transaction(ctx, func(txCtx context.Context, _ gdb.TX) error {
			if _, e := db.Exec(txCtx, statement); e != nil {
				return e
			}
			blocked, e := prepared.service.Detail(txCtx, intentNo)
			if e != nil {
				return e
			}
			if blocked.CanRepairManagement {
				t.Fatal("invalid local state advertised repair")
			}
			if _, e = prepared.service.RepairManagement(txCtx, input); e == nil {
				t.Fatal("invalid local state accepted repair")
			}
			return rollback
		})
		if !errors.Is(err, rollback) {
			t.Fatal(err)
		}
	}
	if _, err = newService(fulfillmentAuthorizerStub{}, recoveryFailingAudit{}).RepairManagement(ctx, input); err == nil {
		t.Fatal("repair survived audit failure")
	}
	untouched, err := prepared.service.Detail(ctx, intentNo)
	if err != nil || !untouched.CanRepairManagement || untouched.ManagementPreparation.AttemptCount != 10 {
		t.Fatalf("audit rollback: %+v %v", untouched, err)
	}
	audit := &managementRepairAudit{}
	service := newService(fulfillmentAuthorizerStub{}, audit)
	queued, err := service.RepairManagement(ctx, input)
	if err != nil || queued.CanRepairManagement || queued.StatusCode != "completed" || queued.Version != current.Version || queued.ManagementPreparation.StatusCode != "preparing" || queued.ManagementPreparation.AttemptCount != 0 {
		t.Fatalf("repair queue: %+v %v cause=%v", queued, err, errors.Unwrap(err))
	}
	replay, err := service.RepairManagement(ctx, input)
	if err != nil || replay.Version != current.Version || len(audit.events) != 1 {
		t.Fatalf("repair replay: %+v %v", replay, err)
	}
	conflicting := input
	conflicting.ExpectedVersion++
	if _, err = service.RepairManagement(ctx, conflicting); err == nil {
		t.Fatal("same key accepted changed request")
	}
	another := input
	another.IdempotencyKey = "local-management-repair-test-0002"
	if _, err = service.RepairManagement(ctx, another); err == nil {
		t.Fatal("second command accepted while pending")
	}
	report, err := worker.Run(ctx, 1)
	if err != nil || report.Delivered != 1 {
		t.Fatalf("repair local delivery: %+v %v", report, err)
	}
	ready, err := service.Detail(ctx, intentNo)
	if err != nil || ready.ManagementPreparation.StatusCode != "ready" || ready.CanRepairManagement {
		t.Fatalf("local readiness: %+v %v", ready, err)
	}
	if original != protected() || extensions.invocationCount(resourcecap.DynamicProviderMethodExecute) != 1 || extensions.invocationCount(resourcecap.DynamicProviderMethodBuildProvisionParameters) != 1 {
		t.Fatal("repair changed workflow, financial snapshot or provider invocation count")
	}
	encoded, err := json.Marshal(audit.events[0])
	if err != nil || strings.Contains(string(encoded), "PRIVATE") {
		t.Fatal("repair audit exposed untrusted error")
	}
	count, err := db.GetValue(ctx, `SELECT count(*) FROM plugin_idc_core_product_fulfillment_command WHERE intent_no=? AND action_code='repair_management'`, intentNo)
	if err != nil || count.Int() != 1 {
		t.Fatalf("repair commands=%v %v", count, err)
	}
}
