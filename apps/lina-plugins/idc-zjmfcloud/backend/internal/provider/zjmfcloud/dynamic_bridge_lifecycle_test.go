// This file verifies the managed execution path, distinct acknowledgement ids,
// query-only uncertainty, and confidential principal slots rather than only
// exercising the source Provider implementation.
package zjmfcloud

import (
	"context"
	"encoding/json"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
	"net/http"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
)

type lifecycleExecutor struct {
	t        *testing.T
	response string
	status   int
	calls    []resourcecap.ConfidentialHTTPRequest
}

func (e *lifecycleExecutor) Execute(_ string, _ uint8, payload json.RawMessage) (json.RawMessage, error) {
	e.t.Helper()
	assertGovernedHTTPObjectKeys(e.t, payload)
	if strings.Contains(string(payload), `"password":`) || strings.Contains(string(payload), "opaque-secret") {
		e.t.Fatal("secret crossed guest wire")
	}
	var request resourcecap.ConfidentialHTTPRequest
	if err := json.Unmarshal(payload, &request); err != nil {
		return nil, err
	}
	if len(request.FormFields) > 0 {
		request.Form = make(map[string]string, len(request.FormFields))
		for _, item := range request.FormFields {
			request.Form[item.Field] = item.Value
		}
	}
	e.calls = append(e.calls, request)
	if request.Kind == resourcecap.ConfidentialHTTPKindAuthenticate {
		return json.RawMessage(`{"StatusCode":200,"Authenticated":true}`), nil
	}
	status := e.status
	if status == 0 {
		status = http.StatusOK
	}
	return json.RawMessage(`{"StatusCode":` + strconv.Itoa(status) + `,"Data":` + e.response + `}`), nil
}

// The real governed host validates every object key before the owner's HTTP
// handler runs. Checking the serialized boundary prevents a permissive fake
// transport from hiding invalid supplier bracket notation again.
func assertGovernedHTTPObjectKeys(t *testing.T, payload json.RawMessage) {
	t.Helper()
	var value any
	if err := json.Unmarshal(payload, &value); err != nil {
		t.Fatal(err)
	}
	pattern := regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$`)
	var visit func(any)
	visit = func(item any) {
		switch typed := item.(type) {
		case map[string]any:
			for key, nested := range typed {
				if !pattern.MatchString(key) {
					t.Fatalf("governed host rejects object key %q before HTTP", key)
				}
				visit(nested)
			}
		case []any:
			for _, nested := range typed {
				visit(nested)
			}
		}
	}
	visit(value)
}

// A definite validation failure must survive the guest protocol as failed.
func TestManagedProvisionPreservesValidationRejection(t *testing.T) {
	for _, status := range []int{http.StatusBadRequest, http.StatusUnprocessableEntity} {
		t.Run(strconv.Itoa(status), func(t *testing.T) {
			executor := &lifecycleExecutor{t: t, status: status, response: `{}`}
			payload := json.RawMessage(`{"RequestID":"rejected-create-1","Capability":"server.provision","Parameters":{"area":2,"node":5,"os":97,"client":166,"cpu":2,"memory":2000,"system_disk_size":22,"data_disk_size":22,"hostname":"srv-1009be52e6696761.internal"},"ConfidentialInputNames":["password"]}`)
			raw, err := ExecuteDynamicResource(executor, "context", resourcecap.DynamicProviderMethodExecute, payload)
			if err != nil {
				t.Fatal(err)
			}
			var result resourcecap.OperationResult
			if json.Unmarshal(raw, &result) != nil || result.Validate() != nil || result.Status != resourcecap.TaskStatusFailed ||
				result.Failure == nil || result.Failure.Code != "upstream_request_rejected" || result.Failure.ResultUnknown || result.Failure.Retryable {
				t.Fatalf("validation rejection was lost: %s", raw)
			}
			if len(executor.calls) != 2 || result.ExternalTaskRef != "" || result.ExternalResourceRef != "" {
				t.Fatal("rejected provisioning was retried or invented an acceptance identifier")
			}
		})
	}
}
func TestManagedProvisionKeepsCloudAndTaskIdentitiesDistinct(t *testing.T) {
	payload, err := json.Marshal(resourcecap.DynamicOperationRequest{RequestID: "native-create-1", Capability: resourcecap.CapabilityProvision, Parameters: json.RawMessage(`{"area":1,"node":2,"os":3,"client":4,"cpu":1,"memory":1024,"system_disk_size":20,"hostname":"native-test-01"}`), ConfidentialInputNames: []string{"password"}})
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name, reply string
		status      resourcecap.TaskStatus
		task, cloud string
	}{
		{"accepted", `{"data":{"id":41,"taskid":71}}`, resourcecap.TaskStatusAccepted, "71", "41"},
		{"cloud id alone is unknown", `{"data":{"id":41}}`, resourcecap.TaskStatusUnknown, "", "41"},
		{"task without cloud reconciles", `{"data":{"taskid":71}}`, resourcecap.TaskStatusUnknown, "71", ""},
		{"empty response cannot succeed", `{}`, resourcecap.TaskStatusUnknown, "", ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			executor := &lifecycleExecutor{t: t, response: test.reply}
			raw, err := ExecuteDynamicResource(executor, "context", resourcecap.DynamicProviderMethodExecute, payload)
			if err != nil {
				t.Fatal(err)
			}
			var result resourcecap.OperationResult
			if json.Unmarshal(raw, &result) != nil || result.Validate() != nil || result.Status != test.status || result.ExternalTaskRef != test.task || result.ExternalResourceRef != test.cloud {
				t.Fatalf("managed acceptance=%s err=%v", raw, err)
			}
			if len(executor.calls) != 2 || len(executor.calls[1].SecretFormBindings) != 1 || executor.calls[1].SecretFormBindings[0].Field != "rootpass" {
				t.Fatal("initial password slot was not bound to rootpass")
			}
		})
	}
}

// Business validation codes belong to the envelope, independently of HTTP 200.
// Contradictory acceptance identifiers must still stop automatic replay.
func TestManagedProvisionRecognizesBusinessRejection(t *testing.T) {
	for _, sample := range []struct {
		name, body string
		want       resourcecap.TaskStatus
	}{
		{"validation", `{"status":400,"msg":"镜像在当前节点未下载"}`, resourcecap.TaskStatusFailed},
		{"invalid parameters", `{"status":"422","msg":"参数错误","data":{}}`, resourcecap.TaskStatusFailed},
		{"contradictory receipt", `{"status":400,"data":{"id":41,"taskid":71}}`, resourcecap.TaskStatusUnknown},
		{"root receipt cannot disappear", `{"status":400,"taskid":71,"data":{}}`, resourcecap.TaskStatusUnknown},
		{"internal failure uncertain", `{"status":500,"msg":"Internal error"}`, resourcecap.TaskStatusUnknown},
	} {
		t.Run(sample.name, func(t *testing.T) {
			executor := &lifecycleExecutor{t: t, response: sample.body}
			payload := json.RawMessage(`{"RequestID":"business-rejection-1","Capability":"server.provision","Parameters":{"area":2,"node":5,"os":97,"client":166,"cpu":2,"memory":2000,"system_disk_size":22,"hostname":"srv-business-rejection.internal"},"ConfidentialInputNames":["password"]}`)
			raw, err := ExecuteDynamicResource(executor, "context", resourcecap.DynamicProviderMethodExecute, payload)
			var result resourcecap.OperationResult
			if err != nil || json.Unmarshal(raw, &result) != nil || result.Status != sample.want || result.Validate() != nil {
				t.Fatalf("business rejection classification lost: status=%s expected=%s error=%v", result.Status, sample.want, err)
			}
			if len(executor.calls) != 2 {
				t.Fatal("operation was automatically repeated")
			}
		})
	}
}
func TestManagedTaskQueryChecksExactTaskAndCloud(t *testing.T) {
	payload := json.RawMessage(`{"ExternalTaskRef":"71","ResourceRef":"41"}`)
	for _, test := range []struct {
		reply  string
		status resourcecap.TaskStatus
	}{
		{`{"data":{"id":71,"hostid":41,"status":2}}`, resourcecap.TaskStatusSucceeded},
		{`{"data":{"id":72,"hostid":41,"status":2}}`, resourcecap.TaskStatusUnknown},
		{`{"data":{"id":71,"hostid":42,"status":2}}`, resourcecap.TaskStatusUnknown},
	} {
		executor := &lifecycleExecutor{t: t, response: test.reply}
		raw, err := ExecuteDynamicResource(executor, "context", resourcecap.DynamicProviderMethodQueryTask, payload)
		var result resourcecap.OperationResult
		if err != nil || json.Unmarshal(raw, &result) != nil || result.Status != test.status {
			t.Fatalf("managed task=%s err=%v", raw, err)
		}
	}
}
func TestManagedPrincipalUsesOnlyConfidentialSlotsAndExplicitAbsence(t *testing.T) {
	for _, test := range []struct {
		name, method, reply string
		names               []string
		exists              bool
		ref                 string
		invalid             bool
	}{
		{"missing", resourcecap.DynamicProviderMethodLookupPrincipal, `{"id":0}`, []string{"username"}, false, "", false},
		{"found", resourcecap.DynamicProviderMethodLookupPrincipal, `{"id":49}`, []string{"username"}, true, "49", false},
		{"missing id is uncertain", resourcecap.DynamicProviderMethodLookupPrincipal, `{}`, []string{"username"}, false, "", true},
		{"created", resourcecap.DynamicProviderMethodCreatePrincipal, `{"id":49}`, []string{"username", "password"}, false, "49", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			executor := &lifecycleExecutor{t: t, response: test.reply}
			payload, _ := json.Marshal(resourcecap.DynamicPrincipalRequest{RequestID: "principal-test-1", ConfidentialInputNames: test.names})
			raw, err := ExecuteDynamicResource(executor, "context", test.method, payload)
			if test.invalid {
				if err == nil {
					t.Fatal("uncertain lookup allowed another create")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if test.method == resourcecap.DynamicProviderMethodLookupPrincipal {
				var result resourcecap.PrincipalLookupResult
				if json.Unmarshal(raw, &result) != nil || result.Exists != test.exists || result.ExternalPrincipalRef != test.ref || result.Validate() != nil {
					t.Fatal("invalid lookup projection")
				}
			} else {
				var result resourcecap.PrincipalCreateResult
				if json.Unmarshal(raw, &result) != nil || result.ExternalPrincipalRef != test.ref || result.Validate() != nil {
					t.Fatal("invalid create projection")
				}
			}
			if len(executor.calls) != 2 || len(executor.calls[1].SecretFormBindings) != len(test.names) {
				t.Fatal("principal secret slots not bound")
			}
			if strings.Contains(string(raw), "username") || strings.Contains(string(raw), "password") {
				t.Fatal("principal secrets returned")
			}
		})
	}
}
func TestManagedResourceProjectsIPAddressesAndBlocksTransitionalPowerState(t *testing.T) {
	executor := &lifecycleExecutor{t: t, response: `{"data":{"id":41,"status":"task","ip_num":1,"ip":[{"id":1,"ipaddress":"203.0.113.9","password":"must-not-leak"}],"rootpassword":"must-not-leak"}}`}
	raw, err := ExecuteDynamicResource(executor, "context", resourcecap.DynamicProviderMethodQueryResource, json.RawMessage(`{"ExternalResourceRef":"41"}`))
	var result resourcecap.ResourceSnapshot
	if err != nil || json.Unmarshal(raw, &result) != nil || result.State != resourcecap.ResourceStatePending || !strings.Contains(string(result.Data), `"addresses":["203.0.113.9"]`) || strings.Contains(string(raw), "must-not-leak") {
		t.Fatalf("unsafe managed observation %s %v", raw, err)
	}
	for _, request := range executor.calls {
		for _, field := range request.ResponseFields {
			if strings.Contains(field, "password") {
				t.Fatal("guest requested a secret response field")
			}
		}
	}
}

func TestManagedConfidentialOutputActionsRejectBeforeUpstreamWrite(t *testing.T) {
	for _, capability := range []resourcecap.CapabilityCode{resourcecap.CapabilityConsole, resourcecap.CapabilityReinstall} {
		executor := &lifecycleExecutor{t: t, response: `{"id":41}`}
		payload, err := json.Marshal(resourcecap.DynamicOperationRequest{RequestID: "unsupported-output-1", Capability: capability, ResourceRef: "41", Parameters: json.RawMessage(`{"os":3}`)})
		if err != nil {
			t.Fatal(err)
		}
		if _, err = ExecuteDynamicResource(executor, "context", resourcecap.DynamicProviderMethodExecute, payload); err == nil {
			t.Fatal("unsupported output accepted")
		}
		for _, call := range executor.calls {
			if call.Kind != resourcecap.ConfidentialHTTPKindAuthenticate {
				t.Fatal("unsupported output reached upstream business route")
			}
		}
	}
}

// The isolated guest has its own dispatch path. It must refuse the withdrawn
// quantity contract before any confidential outbound invocation, including login.
func TestManagedChangeIPRejectsBeforeAuthentication(t *testing.T) {
	executor := &lifecycleExecutor{t: t, response: `{"taskid":71}`}
	payload := json.RawMessage(`{"RequestID":"old-ip-queued","Capability":"server.ip.change","ResourceRef":"41","Parameters":{"ip_num":2}}`)
	if result, err := ExecuteDynamicResource(executor, "context", resourcecap.DynamicProviderMethodExecute, payload); err == nil || result != nil || len(executor.calls) != 0 {
		t.Fatal("managed replacement reached authentication or supplier", err)
	}
	raw, err := ExecuteDynamicResource(executor, "context", resourcecap.DynamicProviderMethodDiscoverCapabilities, json.RawMessage(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	var snapshot resourcecap.CapabilitySnapshot
	if err = json.Unmarshal(raw, &snapshot); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, capability := range snapshot.Capabilities {
		if capability.Code == resourcecap.CapabilityChangeIP {
			found = true
			if capability.Support != resourcecap.SupportLevelUnsupported {
				t.Fatal("managed capabilities still advertise address replacement")
			}
		}
	}
	if !found || len(executor.calls) != 0 {
		t.Fatal("managed capability withdrawal missing or not pure")
	}
}

// The real confidential host filters response keys before the guest sees data.
// Guest facts therefore need both an explicit outbound allowlist and projection.
func TestManagedResourceFactsCrossOnlyExplicitHostAllowlist(t *testing.T) {
	executor := &lifecycleExecutor{t: t, response: `{"data":{"id":41,"status":"on","hostname":"guest-1","os_name":"Debian 12","osuser":"root","port":22,"cpu":2,"memory":2048,"rescue":0,"network_type":"public","bootorder":"dc","password":"must-not-leak","username":"upstream-account"}}`}
	raw, err := ExecuteDynamicResource(executor, "context", resourcecap.DynamicProviderMethodQueryResource, json.RawMessage(`{"ExternalResourceRef":"41"}`))
	if err != nil {
		t.Fatal(err)
	}
	if len(executor.calls) != 2 {
		t.Fatal("resource facts introduced extra calls")
	}
	for _, field := range []string{"hostname", "os_name", "osuser", "port", "cpu", "memory", "rescue", "network_type", "bootorder"} {
		if !slices.Contains(executor.calls[1].ResponseFields, field) {
			t.Fatalf("host would strip required guest fact %s", field)
		}
	}
	for _, field := range []string{"password", "rootpassword", "username", "vncpass"} {
		if slices.Contains(executor.calls[1].ResponseFields, field) {
			t.Fatal("secret/account field crossed host allowlist")
		}
	}
	var snapshot resourcecap.ResourceSnapshot
	if json.Unmarshal(raw, &snapshot) != nil || !strings.Contains(string(snapshot.Data), `"boot_order":"dc"`) || !strings.Contains(string(snapshot.Data), `"login_username":"root"`) || !strings.Contains(string(snapshot.Data), `"cpu_cores":2`) || strings.Contains(string(snapshot.Data), "must-not-leak") || strings.Contains(string(snapshot.Data), "upstream-account") {
		t.Fatalf("guest fact projection incorrect: %s", raw)
	}
}

// Both source and managed routes reject restoration when list state has no
// authoritative readiness matrix. Deletion remains a separate identity action.
func TestSnapshotRestoreUnverifiedBeforeEitherTransport(t *testing.T) {
	for _, capability := range []resourcecap.CapabilityCode{resourcecap.CapabilityManageSnapshot, resourcecap.CapabilityBackup} {
		executor := &lifecycleExecutor{t: t, response: `{"taskid":71}`}
		payload, err := json.Marshal(resourcecap.DynamicOperationRequest{RequestID: "old-restore", Capability: capability, ResourceRef: "41", Parameters: json.RawMessage(`{"action":"restore","snapshot_ref":77}`)})
		if err != nil {
			t.Fatal(err)
		}
		if _, err = ExecuteDynamicResource(executor, "context", resourcecap.DynamicProviderMethodExecute, payload); err == nil || len(executor.calls) != 0 {
			t.Fatal("managed restore used unverified readiness")
		}
		fixture := newProviderFixture(t, http.HandlerFunc(func(http.ResponseWriter, *http.Request) { t.Error("source restore called network") }))
		if result, err := fixture.provider.Execute(context.Background(), resourcecap.OperationRequest{RequestID: "old-restore", Capability: capability, ResourceRef: "41", Parameters: json.RawMessage(`{"action":"restore","snapshot_ref":77}`)}); err == nil || result != nil {
			t.Fatal("source restore used unverified readiness")
		}
	}
}
