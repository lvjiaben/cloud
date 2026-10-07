package baota

import (
	"encoding/json"
	"errors"
	"testing"

	"lina-plugin-idc-resource/backend/cap/resourcecap"
)

type recordingExecutor struct {
	requests  []resourcecap.ConfidentialHTTPRequest
	responses []resourcecap.ConfidentialHTTPResponse
	failPath  string
}

func (e *recordingExecutor) Execute(_ string, _ uint8, payload json.RawMessage) (json.RawMessage, error) {
	var request resourcecap.ConfidentialHTTPRequest
	if json.Unmarshal(payload, &request) != nil {
		return nil, errors.New("invalid instruction")
	}
	e.requests = append(e.requests, request)
	if request.Path == e.failPath {
		return nil, errors.New("supplier transport unavailable")
	}
	response := e.responses[len(e.requests)-1]
	return json.Marshal(response)
}

func TestProvisionQueriesExactDomainBeforeCreatingAndReplayDoesNotDuplicate(t *testing.T) {
	executor := &recordingExecutor{responses: []resourcecap.ConfidentialHTTPResponse{
		{StatusCode: 200, Data: json.RawMessage(`{"data":[]}`)},
		{StatusCode: 200, Data: json.RawMessage(`{"siteStatus":true,"siteId":7}`)},
		{StatusCode: 200, Data: json.RawMessage(`{"data":[{"id":7,"name":"shop.example.com","path":"/www/wwwroot/shop.example.com","status":"1"}]}`)},
	}}
	payload, _ := json.Marshal(provisionRequest())
	for attempt := 0; attempt < 2; attempt++ {
		resultJSON, err := ExecuteResource(executor, "ctx-site", resourcecap.DynamicProviderMethodExecute, payload)
		if err != nil {
			t.Fatal(err)
		}
		var result resourcecap.OperationResult
		if json.Unmarshal(resultJSON, &result) != nil || result.Status != resourcecap.TaskStatusSucceeded || result.ExternalResourceRef != "7" {
			t.Fatalf("attempt %d did not return the same site: %s", attempt+1, resultJSON)
		}
	}
	addCount := 0
	for _, request := range executor.requests {
		if request.Form["request_time"] != "" || request.Form["request_token"] != "" {
			t.Fatalf("guest attempted to sign the request: %#v", request.Form)
		}
		if request.Path == "/site" {
			addCount++
		}
	}
	if addCount != 1 || executor.requests[0].Path != "/data" || executor.requests[0].Form["search"] != "shop.example.com" {
		t.Fatalf("query-first provisioning repeated creation: %#v", executor.requests)
	}
}

func TestProvisionPostSubmissionAmbiguityIsManualAndNotRetryable(t *testing.T) {
	for _, test := range []struct {
		name      string
		failPath  string
		addResult resourcecap.ConfidentialHTTPResponse
	}{
		{name: "transport", failPath: "/site"},
		{name: "server error", addResult: resourcecap.ConfidentialHTTPResponse{StatusCode: 500, Data: json.RawMessage(`{"status":false}`)}},
		{name: "missing site reference", addResult: resourcecap.ConfidentialHTTPResponse{StatusCode: 200, Data: json.RawMessage(`{"siteStatus":true}`)}},
	} {
		t.Run(test.name, func(t *testing.T) {
			executor := &recordingExecutor{failPath: test.failPath, responses: []resourcecap.ConfidentialHTTPResponse{
				{StatusCode: 200, Data: json.RawMessage(`{"data":[]}`)}, test.addResult,
			}}
			payload, _ := json.Marshal(provisionRequest())
			resultJSON, err := ExecuteResource(executor, "ctx-unknown", resourcecap.DynamicProviderMethodExecute, payload)
			if err != nil {
				t.Fatal(err)
			}
			var result resourcecap.OperationResult
			if json.Unmarshal(resultJSON, &result) != nil || result.Status != resourcecap.TaskStatusUnknown || result.Failure == nil || !result.Failure.ResultUnknown || result.Failure.Retryable {
				t.Fatalf("post-create ambiguity was unsafe: %s", resultJSON)
			}
			if len(executor.requests) != 2 {
				t.Fatalf("ambiguous creation was retried: %#v", executor.requests)
			}
		})
	}
}

func TestProvisionExplicitClientRejectionFails(t *testing.T) {
	executor := &recordingExecutor{responses: []resourcecap.ConfidentialHTTPResponse{
		{StatusCode: 200, Data: json.RawMessage(`{"data":[]}`)},
		{StatusCode: 422, Data: json.RawMessage(`{"siteStatus":false}`)},
	}}
	payload, _ := json.Marshal(provisionRequest())
	resultJSON, err := ExecuteResource(executor, "ctx-rejected", resourcecap.DynamicProviderMethodExecute, payload)
	if err != nil {
		t.Fatal(err)
	}
	var result resourcecap.OperationResult
	if json.Unmarshal(resultJSON, &result) != nil || result.Status != resourcecap.TaskStatusFailed || result.Failure == nil || result.Failure.ResultUnknown {
		t.Fatalf("explicit rejection did not fail safely: %s", resultJSON)
	}
}

func provisionRequest() resourcecap.DynamicOperationRequest {
	plan, _ := BuildSitePlan(json.RawMessage(`{"deployment_type":"php_site","php_version":"82"}`), json.RawMessage(`{"primary_domain":"shop.example.com","additional_domains":"www.example.com"}`))
	return resourcecap.DynamicOperationRequest{RequestID: "request-site-1", Capability: resourcecap.CapabilityProvision, Parameters: plan}
}
