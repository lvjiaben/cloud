package v10

import (
	"encoding/json"
	"strings"
	"testing"

	"lina-plugin-idc-resource/backend/cap/resourcecap"
)

type recordedStep struct {
	sequence uint8
	request  resourcecap.ConfidentialHTTPRequest
}

type recordingExecutor struct {
	steps []recordedStep
}

func (e *recordingExecutor) Execute(_ string, sequence uint8, payload json.RawMessage) (json.RawMessage, error) {
	var request resourcecap.ConfidentialHTTPRequest
	if err := json.Unmarshal(payload, &request); err != nil {
		return nil, err
	}
	e.steps = append(e.steps, recordedStep{sequence: sequence, request: request})
	response := resourcecap.ConfidentialHTTPResponse{StatusCode: 200}
	switch request.Kind {
	case resourcecap.ConfidentialHTTPKindAuthenticate:
		response.Authenticated = true
	case resourcecap.ConfidentialHTTPKindRequest:
		switch request.Path {
		case "/api/v1/product":
			response.Data = json.RawMessage(`{"status":200,"data":{"list":[{"id":81,"name":"KVM 云服务器"}]}}`)
		case "/console/v1/cart":
			if request.Method == "DELETE" {
				response.Data = json.RawMessage(`{"status":200,"data":{}}`)
			} else {
				response.Data = json.RawMessage(`{"status":200}`)
			}
		case "/console/v1/cart/settle":
			response.Data = json.RawMessage(`{"status":200,"data":{"order_id":71,"host_ids":[91],"amount":"10.00"}}`)
		case "/console/v1/pay":
			response.Data = json.RawMessage(`{"status":200}`)
		default:
			response.Data = json.RawMessage(`{"status":200}`)
		}
	}
	return json.Marshal(response)
}

func TestConfigurationCandidatesUseV10AuthAndProductAPI(t *testing.T) {
	executor := &recordingExecutor{}
	payload := json.RawMessage(`{"schemaVersion":"v1","currentSelections":[],"requests":[{"fieldKey":"upstream_product_id","parentValues":[],"selectedValues":[]}]}`)
	result, err := ExecuteDynamic(executor, "ctx-v10-products", resourcecap.DynamicProviderMethodConfigurationCandidates, payload)
	if err != nil || len(executor.steps) != 2 || executor.steps[0].request.Path != "/api/v1/auth" ||
		executor.steps[1].request.Path != "/api/v1/product" || executor.steps[1].request.Method != "GET" {
		t.Fatalf("V10 discovery steps=%+v result=%s err=%v", executor.steps, result, err)
	}
	if !json.Valid(result) || !containsAll(string(result), `"upstream_product_id"`, `"81"`, `"KVM 云服务器"`) {
		t.Fatalf("unexpected V10 product projection: %s", result)
	}
}

func containsAll(value string, fragments ...string) bool {
	for _, fragment := range fragments {
		if !strings.Contains(value, fragment) {
			return false
		}
	}
	return true
}

func TestProvisionUsesQueryFirstConsoleCartWorkflow(t *testing.T) {
	executor := &recordingExecutor{}
	request := resourcecap.DynamicOperationRequest{
		RequestID: "req-v10-provision-0001", Capability: resourcecap.CapabilityProvision,
		Parameters:             json.RawMessage(`{"pid":"81","billingcycle":"monthly","configoption":{"1001":"2001","1002":"2002","1003":"2003"},"downstream_client_id":"42","create_sub_account":1,"hostname":"srv-42.internal"}`),
		ConfidentialInputNames: []string{"downstream_token"},
		ProviderCorrelation:    &resourcecap.ProviderCorrelation{DownstreamID: 1001, DownstreamURL: "https://platform.example.test/api/plugins/idc-resource/provider-callback"},
	}
	payload, _ := json.Marshal(request)
	result, err := ExecuteDynamic(executor, "ctx-v10-provision", resourcecap.DynamicProviderMethodExecute, payload)
	if err != nil {
		t.Fatal(err)
	}
	paths := make([]string, 0, len(executor.steps))
	for _, step := range executor.steps {
		paths = append(paths, step.request.Path)
	}
	want := []string{"/api/v1/auth", "/console/v1/cart", "/console/v1/cart", "/console/v1/cart/settle", "/console/v1/pay"}
	if len(paths) != len(want) {
		t.Fatalf("unexpected V10 provision sequence: %v", paths)
	}
	for index := range want {
		if paths[index] != want[index] {
			t.Fatalf("unexpected V10 provision sequence: %v", paths)
		}
	}
	if executor.steps[1].request.Method != "DELETE" || executor.steps[1].request.SecretFormFields["downstream_token"] != "downstream_token" ||
		executor.steps[2].request.Form["config_options[configoption][1001]"] != "2001" ||
		executor.steps[3].request.SecretFormFields["downstream_token"] != "downstream_token" {
		t.Fatalf("V10 query-first or confidential form binding is incomplete: %+v", executor.steps)
	}
	var operation resourcecap.OperationResult
	if json.Unmarshal(result, &operation) != nil || operation.Status != resourcecap.TaskStatusSucceeded || operation.ExternalResourceRef != "91" {
		t.Fatalf("unexpected V10 provision result: %s", result)
	}
}

func TestConfigurationHTMLBecomesStructuredCandidates(t *testing.T) {
	groups, err := projectConfigurationHTML(`<div><label for="region">地区</label><select id="region" name="config_options[configoption][1001]"><option value="">请选择</option><option value="2001">华南</option></select></div><div><select aria-label="操作系统" name="config_options[configoption][1003]"><option value="3001">Ubuntu 22.04</option></select></div>`)
	if err != nil || len(groups) != 2 || groups[0].id != "1001" || groups[0].label != "地区" ||
		len(groups[0].options) != 1 || groups[0].options[0].Value != "2001" || groups[1].label != "操作系统" {
		t.Fatalf("unexpected V10 HTML candidate projection: groups=%+v err=%v", groups, err)
	}
	items := optionCandidates("operating_system", []candidateSelection{{FieldKey: "upstream_os_option_id", Values: []string{"1003"}}}, groups)
	if len(items) != 1 || items[0].Value != "3001" || items[0].LabelZhCN != "Ubuntu 22.04" {
		t.Fatalf("unexpected operating system candidates: %+v", items)
	}
}
