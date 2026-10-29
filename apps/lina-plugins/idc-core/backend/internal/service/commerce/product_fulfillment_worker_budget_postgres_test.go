// This file verifies persisted query/submit counters converge at the bounded retry limit.

package commerce

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"lina-core/pkg/plugin/capability/capmodel"
	"lina-core/pkg/plugin/pluginhost"
	"lina-plugin-idc-core/backend/cap/productfulfillmentcap"
	productfulfillmentservice "lina-plugin-idc-core/backend/internal/service/productfulfillment"
)

// refusingWorkerExtensions proves retry limits with an upstream that always explicitly refuses writes.
type refusingWorkerExtensions struct {
	fulfillmentWorkerExtensions
	calls int
}

func (e *refusingWorkerExtensions) InvokeProviderMethod(_ context.Context, input pluginhost.BusinessExtensionMethodInvokeInput) (*capmodel.BusinessExtensionMethodResult, error) {
	var request productfulfillmentcap.Request
	if err := json.Unmarshal(input.Payload, &request); err != nil {
		return nil, err
	}
	e.calls++
	status := productfulfillmentcap.StatusNotFound
	if input.Method == "provision" {
		status = productfulfillmentcap.StatusNotAccepted
	}
	payload, err := json.Marshal(productfulfillmentcap.Result{Version: productfulfillmentcap.RequestVersion, Status: status})
	if err != nil {
		return nil, err
	}
	return &capmodel.BusinessExtensionMethodResult{Version: capmodel.BusinessExtensionInvocationProtocolV1, Payload: payload}, nil
}

func TestProductFulfillmentWorkerPersistedAttemptBudgetPostgreSQL(t *testing.T) {
	prepared := prepareGenericWorkerScenario(t)
	scenario := prepared.checkout
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve budget migration")
	}
	migration, err := os.ReadFile(filepath.Join(filepath.Dir(file), "..", "..", "..", "..", "manifest", "sql", "136-fulfillment-attempt-budget.sql"))
	if err != nil {
		t.Fatal(err)
	}
	for pass := 0; pass < 2; pass++ {
		if _, err = scenario.database.Exec(scenario.ctx, string(migration)); err != nil {
			t.Fatal(err)
		}
	}
	sealer, err := productfulfillmentservice.NewProviderEvidenceSealer("test-key-v1", []byte("0123456789abcdef0123456789abcdef"))
	if err != nil {
		t.Fatal(err)
	}
	extensions := &refusingWorkerExtensions{}
	worker, err := productfulfillmentservice.NewWorker(scenario.database, extensions, &fulfillmentWorkerContexts{}, sealer, fulfillmentSystemReasons(t), nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	for attempt := 0; attempt < 20; attempt++ {
		advanceFulfillmentTaskClock(t, scenario, prepared.order.OrderNo)
		report, runErr := worker.Run(scenario.ctx, 1)
		if runErr != nil || report.Selected != 1 {
			t.Fatalf("attempt %d stranded: report=%+v err=%v", attempt+1, report, runErr)
		}
	}
	var evidence struct {
		TaskStatus, IntentStatus string
		Queries, Submits, Total  int
	}
	if err = scenario.database.GetScan(scenario.ctx, &evidence, `SELECT task.status_code AS task_status,intent.status_code AS intent_status,task.query_attempt_count AS queries,task.submit_attempt_count AS submits,task.attempt_count AS total FROM plugin_idc_core_product_fulfillment_task task JOIN plugin_idc_core_product_fulfillment_intent intent USING(intent_no) WHERE intent.order_no=?`, prepared.order.OrderNo); err != nil {
		t.Fatal(err)
	}
	if evidence.TaskStatus != "failed" || evidence.IntentStatus != "manual_pending" || evidence.Queries != 10 || evidence.Submits != 10 || evidence.Total != 20 || extensions.calls != 20 {
		t.Fatalf("retry budget did not converge: %+v calls=%d", evidence, extensions.calls)
	}
	report, err := worker.Run(scenario.ctx, 1)
	if err != nil || report.Selected != 0 || extensions.calls != 20 {
		t.Fatalf("exhausted task invoked again: %+v calls=%d err=%v", report, extensions.calls, err)
	}
}
