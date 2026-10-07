package productfulfillment

import (
	"bytes"
	"encoding/json"
	"testing"

	"lina-plugin-idc-core/backend/cap/productfulfillmentcap"
)

func TestWorkerStateIsQueryFirstAndNeverRepeatsUnknownSubmit(t *testing.T) {
	tests := []struct {
		name, previous, wantTask, wantIntent   string
		action                                 productfulfillmentcap.Action
		status                                 productfulfillmentcap.ResultStatus
		invokeFailed, providerBoundaryCrossed  bool
		takeover, cancelRequested, wantNext    bool
		queryAttempts, submitAttempts, maximum int
	}{
		{"query missing persists submit for another lease", "pending_query", "pending_submit", "processing", productfulfillmentcap.ActionQuery, productfulfillmentcap.StatusNotFound, false, false, false, false, true, 1, 0, 1},
		{"submit not accepted retries within bound", "pending_submit", "retry_wait", "processing", productfulfillmentcap.ActionSubmit, productfulfillmentcap.StatusNotAccepted, false, false, false, false, true, 1, 1, 3},
		{"submit not accepted stops safely at bound", "pending_submit", "failed", "manual_pending", productfulfillmentcap.ActionSubmit, productfulfillmentcap.StatusNotAccepted, false, false, false, false, false, 3, 3, 3},
		{"query transport failure retries", "pending_query", "retry_wait", "processing", productfulfillmentcap.ActionQuery, "", true, true, false, false, true, 1, 0, 3},
		{"query transport failure stops at fixed ten", "pending_query", "failed", "manual_pending", productfulfillmentcap.ActionQuery, "", true, true, false, false, false, 10, 0, 99},
		{"query failure after acceptance keeps accepted", "accepted", "accepted", "processing", productfulfillmentcap.ActionQuery, "", true, true, false, false, true, 1, 1, 3},
		{"submit transport failure is unknown", "pending_submit", "result_unknown", "processing", productfulfillmentcap.ActionSubmit, "", true, true, false, false, false, 1, 1, 3},
		{"submit preparation failure returns to query", "pending_submit", "retry_wait", "processing", productfulfillmentcap.ActionSubmit, "", true, false, false, false, true, 1, 1, 3},
		{"submit unknown after takeover converges to manual", "pending_submit", "manual_takeover", "manual_pending", productfulfillmentcap.ActionSubmit, "", true, true, true, false, false, 1, 1, 3},
		{"accepted is queried later", "pending_submit", "accepted", "processing", productfulfillmentcap.ActionSubmit, productfulfillmentcap.StatusAccepted, false, false, false, false, true, 1, 1, 3},
		{"unknown is never retried", "pending_query", "result_unknown", "processing", productfulfillmentcap.ActionQuery, productfulfillmentcap.StatusResultUnknown, false, false, false, false, false, 1, 0, 3},
		{"unknown after takeover converges to manual", "pending_query", "manual_takeover", "manual_pending", productfulfillmentcap.ActionQuery, productfulfillmentcap.StatusResultUnknown, false, false, true, false, false, 1, 0, 3},
		{"accepted then not found never resubmits", "accepted", "result_unknown", "processing", productfulfillmentcap.ActionQuery, productfulfillmentcap.StatusNotFound, false, false, false, false, false, 2, 1, 3},
		{"success remains authoritative during takeover", "running", "succeeded", "completed", productfulfillmentcap.ActionQuery, productfulfillmentcap.StatusSucceeded, false, false, true, false, false, 2, 1, 3},
		{"nonterminal result converges to takeover", "running", "manual_takeover", "manual_pending", productfulfillmentcap.ActionQuery, productfulfillmentcap.StatusRunning, false, false, true, false, false, 2, 1, 3},
		{"cancel before submit boundary is safe", "pending_submit", "canceled", "canceled", productfulfillmentcap.ActionSubmit, "", true, false, false, true, false, 1, 1, 3},
		{"cancel query not found is safe", "pending_query", "canceled", "canceled", productfulfillmentcap.ActionQuery, productfulfillmentcap.StatusNotFound, false, true, false, true, false, 1, 0, 3},
		{"cancel submit not accepted is safe", "pending_submit", "canceled", "canceled", productfulfillmentcap.ActionSubmit, productfulfillmentcap.StatusNotAccepted, false, true, false, true, false, 1, 1, 3},
		{"cancel submit unknown needs attention", "pending_submit", "cancellation_attention", "manual_pending", productfulfillmentcap.ActionSubmit, productfulfillmentcap.StatusResultUnknown, false, true, false, true, false, 1, 1, 3},
		{"cancel accepted needs attention", "pending_submit", "cancellation_attention", "manual_pending", productfulfillmentcap.ActionSubmit, productfulfillmentcap.StatusAccepted, false, true, false, true, false, 1, 1, 3},
		{"success remains authoritative during cancellation", "running", "succeeded", "completed", productfulfillmentcap.ActionQuery, productfulfillmentcap.StatusSucceeded, false, true, false, true, false, 2, 1, 3},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var result *productfulfillmentcap.Result
			if !test.invokeFailed {
				result = &productfulfillmentcap.Result{Version: productfulfillmentcap.RequestVersion, Status: test.status}
			}
			phase := "pre_submit_query"
			if test.previous == "pending_submit" {
				phase = "submit_allowed"
			} else if test.previous == "accepted" || test.previous == "running" {
				phase = "post_submit_query"
			}
			decision := decideWorkerResult(test.previous, phase, test.action, result, test.invokeFailed, test.providerBoundaryCrossed, test.takeover, test.cancelRequested, test.queryAttempts, test.submitAttempts)
			if decision.TaskStatus != test.wantTask || decision.IntentStatus != test.wantIntent || decision.ScheduleNext != test.wantNext {
				t.Fatalf("decision=%+v", decision)
			}
			if !test.takeover && decision.TaskStatus == "manual_takeover" {
				t.Fatal("worker must never manufacture an employee takeover")
			}
		})
	}
}

func TestSafeProviderCustomerConfigurationRemovesFileTokens(t *testing.T) {
	declaration := `{"version":"v1","fields":[{"key":"model","type":"text","label":{"zhCN":"型号","enUS":"Model"}},{"key":"identity_document","type":"file","label":{"zhCN":"证件","enUS":"Document"},"uploadPolicy":"customer.identity.v1"}],"uploadPolicies":[{"id":"customer.identity.v1","maximumFiles":1,"maximumFileBytes":1024,"maximumTotalBytes":1024,"allowedMimeTypes":["application/pdf"],"allowedExtensions":[".pdf"]}]}`
	result, err := safeProviderCustomerConfiguration(declaration, json.RawMessage(`{"model":"phone-pro","identity_document":["CFU_22222222222222222222222222222222"]}`))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(result, []byte("CFU_")) || bytes.Contains(result, []byte("identity_document")) || !bytes.Contains(result, []byte("phone-pro")) {
		t.Fatalf("unsafe provider customer configuration: %s", result)
	}
}
