// These tests prove that actual responses and owner preflight diagnostics survive
// resource and core persistence without issuing another creation automatically.
package commerce

import (
	"strings"
	"testing"

	productfulfillmentservice "lina-plugin-idc-core/backend/internal/service/productfulfillment"
)

func TestFulfillmentRetainsResponseAndPreflightDiagnosticsPostgreSQL(t *testing.T) {
	for _, noResponse := range []bool{false, true} {
		t.Run(map[bool]string{false: "HTTP body", true: "failure before HTTP"}[noResponse], func(t *testing.T) {
			prepared := prepareCrossPluginServerScenario(t)
			scenario := prepared.checkout
			ctx, db := scenario.ctx, scenario.database
			configureResourceChainDatabase(t, ctx, db)
			extensions, job, source := crossPluginRuntime(t, ctx)
			extensions.unknownFirst = !noResponse
			extensions.unknownNoResponse = noResponse
			expected := "Bad Gateway"
			if noResponse {
				expected = "request_preparation"
			}
			sealer, err := productfulfillmentservice.NewProviderEvidenceSealer("test-key-v1", []byte("0123456789abcdef0123456789abcdef"))
			if err != nil {
				t.Fatal(err)
			}
			worker, err := productfulfillmentservice.NewWorker(db, extensions, &fulfillmentWorkerContexts{}, sealer, fulfillmentSystemReasons(t), source, nil, nil)
			if err != nil {
				t.Fatal(err)
			}
			service, err := productfulfillmentservice.NewFullConfigured(db, structuredFulfillmentEvidenceValidator{}, fulfillmentGateStub{}, fulfillmentSystemReasons(t), fulfillmentAuthorizerStub{}, fulfillmentAuditStub{}, fulfillmentActorsStub{}, []byte("0123456789abcdef0123456789abcdef"), source)
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
			expectedStatus := "result_unknown"
			if noResponse {
				expectedStatus = "failed"
			}
			detail, err := service.Detail(ctx, intent.String())
			if err != nil || detail.Execution == nil || !strings.Contains(detail.Execution.ResponseExcerpt, expected) || detail.Execution.StatusCode != expectedStatus || detail.CanRecoverPreparation != noResponse || (!noResponse && detail.CanRequestRefund) {
				t.Fatalf("uncertain response was lost or incorrectly made recoverable: %+v %v", detail, err)
			}
			count, err := db.GetValue(ctx, `SELECT count(*) FROM plugin_idc_resource_task WHERE capability_code='server.provision'`)
			if err != nil || count.Int() != 1 {
				t.Fatal("uncertain creation was repeated")
			}
		})
	}
}
