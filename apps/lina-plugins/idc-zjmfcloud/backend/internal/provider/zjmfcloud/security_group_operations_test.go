// This file verifies security-group relation mutations against a controlled upstream.

package zjmfcloud

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"sync"
	"sync/atomic"
	"testing"

	"lina-plugin-idc-resource/backend/cap/resourcecap"
)

func TestSecurityGroupAttachAndDetachMappings(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name             string
		parameters       string
		method           string
		path             string
		form             url.Values
		beforeSecurity   string
		afterSecurity    string
		wantGroupListGet int64
	}{
		{
			name: "attach", parameters: `{"action":"attach","security_group_ref":77}`,
			method: http.MethodPost, path: "/administrator/v1/security_groups/77/links",
			form:             url.Values{"cloud": {"41"}, "type": {"1"}},
			beforeSecurity:   "0",
			afterSecurity:    "77",
			wantGroupListGet: 1,
		},
		{
			name: "detach", parameters: `{"action":"detach"}`,
			method: http.MethodDelete, path: "/administrator/v1/clouds/41/security_groups",
			form:           url.Values{},
			beforeSecurity: "77",
			afterSecurity:  "0",
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			var (
				loginCount int
				loginMu    sync.Mutex
			)
			login := loginHandler(t, &loginCount, &loginMu, "token-one")
			var (
				detailReads atomic.Int64
				groupReads  atomic.Int64
				writes      atomic.Int64
			)
			fixture := newProviderFixture(t, http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				switch {
				case request.URL.Path == "/administrator/v1/login":
					login(writer, request)
				case request.Method == http.MethodGet && request.URL.Path == "/administrator/v1/clouds/41":
					security := test.beforeSecurity
					if detailReads.Add(1) > 1 {
						security = test.afterSecurity
					}
					writeJSON(t, writer, http.StatusOK,
						`{"data":{"id":41,"user_id":9,"type":"host","security":`+security+`,"password":"must-not-leak"}}`)
				case request.Method == http.MethodGet && request.URL.Path == "/administrator/v1/security_groups":
					groupReads.Add(1)
					if request.URL.Query().Encode() != "list_type=all&type=host&user=9" {
						t.Errorf("group query=%v", request.URL.Query())
					}
					writeJSON(t, writer, http.StatusOK,
						`{"data":{"data":[{"id":77,"type":"host","name":"default","secret":"must-not-leak"}]}}`)
				default:
					writes.Add(1)
					if request.Method != test.method || request.URL.Path != test.path {
						t.Errorf("write=%s %s, want %s %s", request.Method, request.URL.Path, test.method, test.path)
					}
					if err := request.ParseForm(); err != nil {
						t.Errorf("parse form: %v", err)
					}
					if request.PostForm.Encode() != test.form.Encode() {
						t.Errorf("form=%v, want %v", request.PostForm, test.form)
					}
					writeJSON(t, writer, http.StatusOK, `{"status":"success"}`)
				}
			}))
			request := resourcecap.OperationRequest{
				RequestID:   "request-security-group-" + test.name,
				Capability:  resourcecap.CapabilitySecurityGroup,
				ResourceRef: "41",
				Parameters:  []byte(test.parameters),
			}
			result, err := fixture.provider.Execute(context.Background(), request)
			if err != nil {
				t.Fatal(err)
			}
			if result.Status != resourcecap.TaskStatusSucceeded || result.ExternalResourceRef != "41" ||
				len(result.Evidence) != 1 || result.Evidence[0].Kind != "upstream_postcondition" {
				t.Fatalf("unexpected result: %+v", result)
			}
			if err := result.ValidateFor(request); err != nil {
				t.Fatalf("validate result: %v", err)
			}
			if detailReads.Load() != 2 || groupReads.Load() != test.wantGroupListGet || writes.Load() != 1 {
				t.Fatalf("reads=%d groups=%d writes=%d", detailReads.Load(), groupReads.Load(), writes.Load())
			}
		})
	}
}

func TestSecurityGroupTaskAcceptanceSkipsImmediatePostconditionRead(t *testing.T) {
	t.Parallel()
	var (
		loginCount int
		loginMu    sync.Mutex
	)
	login := loginHandler(t, &loginCount, &loginMu, "token-one")
	var detailReads atomic.Int64
	fixture := newProviderFixture(t, http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/administrator/v1/login":
			login(writer, request)
		case "/administrator/v1/clouds/41":
			detailReads.Add(1)
			writeJSON(t, writer, http.StatusOK, `{"data":{"user_id":9,"type":"host","security":0}}`)
		case "/administrator/v1/security_groups":
			writeJSON(t, writer, http.StatusOK, `{"data":[{"id":77,"type":"host"}]}`)
		case "/administrator/v1/security_groups/77/links":
			writeJSON(t, writer, http.StatusAccepted, `{"data":{"taskid":8801}}`)
		default:
			http.NotFound(writer, request)
		}
	}))
	request := resourcecap.OperationRequest{
		RequestID: "request-security-group-task", Capability: resourcecap.CapabilitySecurityGroup,
		ResourceRef: "41", Parameters: []byte(`{"action":"attach","security_group_ref":"77"}`),
	}
	result, err := fixture.provider.Execute(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != resourcecap.TaskStatusAccepted || result.ExternalTaskRef != "8801" || detailReads.Load() != 1 {
		t.Fatalf("result=%+v detail reads=%d", result, detailReads.Load())
	}
}

func TestSecurityGroupInvalidOrUnownedInputNeverWrites(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		parameters string
		detail     string
		groups     string
	}{
		{name: "unknown-action", parameters: `{"action":"delete","security_group_ref":77}`},
		{name: "attach-missing-ref", parameters: `{"action":"attach"}`},
		{name: "attach-force", parameters: `{"action":"attach","security_group_ref":77,"force":true}`},
		{name: "detach-with-ref", parameters: `{"action":"detach","security_group_ref":77}`},
		{name: "already-attached", parameters: `{"action":"attach","security_group_ref":77}`, detail: `{"data":{"user_id":9,"type":"host","security":77}}`},
		{name: "replace-existing-group", parameters: `{"action":"attach","security_group_ref":77}`, detail: `{"data":{"user_id":9,"type":"host","security":78}}`},
		{name: "foreign-group", parameters: `{"action":"attach","security_group_ref":77}`, detail: `{"data":{"user_id":9,"type":"host","security":0}}`, groups: `{"data":[{"id":78,"type":"host"}]}`},
		{name: "wrong-type", parameters: `{"action":"attach","security_group_ref":77}`, detail: `{"data":{"user_id":9,"type":"host","security":0}}`, groups: `{"data":[{"id":77,"type":"lightHost"}]}`},
		{name: "duplicate-group", parameters: `{"action":"attach","security_group_ref":77}`, detail: `{"data":{"user_id":9,"type":"host","security":0}}`, groups: `{"data":[{"id":77,"type":"host"},{"id":77,"type":"host"}]}`},
		{name: "unsafe-resource-type", parameters: `{"action":"attach","security_group_ref":77}`, detail: `{"data":{"user_id":9,"type":"host&user=8","security":0}}`, groups: `{"data":[{"id":77,"type":"host"}]}`},
		{name: "detach-unbound", parameters: `{"action":"detach"}`, detail: `{"data":{"user_id":9,"type":"host","security":0}}`},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			var requests atomic.Int64
			fixture := newProviderFixture(t, http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				requests.Add(1)
				switch request.URL.Path {
				case "/administrator/v1/login":
					writeJSON(t, writer, http.StatusOK, `{"data":{"token":"token-one"}}`)
				case "/administrator/v1/clouds/41":
					writeJSON(t, writer, http.StatusOK, test.detail)
				case "/administrator/v1/security_groups":
					writeJSON(t, writer, http.StatusOK, test.groups)
				default:
					t.Errorf("unexpected write %s %s", request.Method, request.URL.Path)
				}
			}))
			request := resourcecap.OperationRequest{
				RequestID:   "request-security-group-invalid-" + test.name,
				Capability:  resourcecap.CapabilitySecurityGroup,
				ResourceRef: "41", Parameters: []byte(test.parameters),
			}
			if _, err := fixture.provider.Execute(context.Background(), request); err == nil {
				t.Fatal("expected input or ownership rejection")
			}
			if test.detail == "" && requests.Load() != 0 {
				t.Fatalf("syntax rejection made %d network requests", requests.Load())
			}
		})
	}
}

func TestSecurityGroupUnprovenPostconditionRemainsUnknownWithoutReplay(t *testing.T) {
	t.Parallel()
	var (
		loginCount int
		loginMu    sync.Mutex
	)
	login := loginHandler(t, &loginCount, &loginMu, "token-one")
	var (
		detailReads atomic.Int64
		writes      atomic.Int64
	)
	fixture := newProviderFixture(t, http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/administrator/v1/login":
			login(writer, request)
		case "/administrator/v1/clouds/41":
			detailReads.Add(1)
			writeJSON(t, writer, http.StatusOK, `{"data":{"user_id":9,"type":"host","security":0}}`)
		case "/administrator/v1/security_groups":
			writeJSON(t, writer, http.StatusOK, `{"data":[{"id":77,"type":"host"}]}`)
		case "/administrator/v1/security_groups/77/links":
			writes.Add(1)
			writeJSON(t, writer, http.StatusOK, `{"status":"success"}`)
		default:
			http.NotFound(writer, request)
		}
	}))
	request := resourcecap.OperationRequest{
		RequestID: "request-security-group-unproven", Capability: resourcecap.CapabilitySecurityGroup,
		ResourceRef: "41", Parameters: []byte(`{"action":"attach","security_group_ref":77}`),
	}
	result, err := fixture.provider.Execute(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != resourcecap.TaskStatusUnknown || result.Failure == nil || !result.Failure.ResultUnknown {
		t.Fatalf("unexpected result: %+v", result)
	}
	if writes.Load() != 1 || detailReads.Load() != 2 {
		t.Fatalf("writes=%d detail reads=%d", writes.Load(), detailReads.Load())
	}
}

func TestSecurityGroupUnauthorizedWriteIsNeverReplayed(t *testing.T) {
	t.Parallel()
	var (
		loginCount int
		loginMu    sync.Mutex
	)
	login := loginHandler(t, &loginCount, &loginMu, "token-one")
	var (
		detailReads atomic.Int64
		writes      atomic.Int64
	)
	fixture := newProviderFixture(t, http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/administrator/v1/login":
			login(writer, request)
		case "/administrator/v1/clouds/41":
			detailReads.Add(1)
			writeJSON(t, writer, http.StatusOK, `{"data":{"user_id":9,"type":"host","security":0}}`)
		case "/administrator/v1/security_groups":
			writeJSON(t, writer, http.StatusOK, `{"data":[{"id":77,"type":"host"}]}`)
		case "/administrator/v1/security_groups/77/links":
			writes.Add(1)
			writeJSON(t, writer, http.StatusUnauthorized, `{"message":"expired"}`)
		default:
			http.NotFound(writer, request)
		}
	}))
	request := resourcecap.OperationRequest{
		RequestID: "request-security-group-unauthorized", Capability: resourcecap.CapabilitySecurityGroup,
		ResourceRef: "41", Parameters: []byte(`{"action":"attach","security_group_ref":77}`),
	}
	result, err := fixture.provider.Execute(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != resourcecap.TaskStatusUnknown || result.Failure == nil || !result.Failure.ResultUnknown {
		t.Fatalf("unexpected result: %+v", result)
	}
	if writes.Load() != 1 || detailReads.Load() != 2 || loginCount != 2 {
		t.Fatalf("writes=%d detail reads=%d logins=%d", writes.Load(), detailReads.Load(), loginCount)
	}
}

func TestSecurityGroupDocumentedRejectionDoesNotRunPostconditionRead(t *testing.T) {
	t.Parallel()
	var (
		loginCount int
		loginMu    sync.Mutex
	)
	login := loginHandler(t, &loginCount, &loginMu, "token-one")
	var (
		detailReads atomic.Int64
		writes      atomic.Int64
	)
	fixture := newProviderFixture(t, http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/administrator/v1/login":
			login(writer, request)
		case "/administrator/v1/clouds/41":
			detailReads.Add(1)
			writeJSON(t, writer, http.StatusOK, `{"data":{"user_id":9,"type":"host","security":0}}`)
		case "/administrator/v1/security_groups":
			writeJSON(t, writer, http.StatusOK, `{"data":[{"id":77,"type":"host"}]}`)
		case "/administrator/v1/security_groups/77/links":
			writes.Add(1)
			writeJSON(t, writer, http.StatusBadRequest, `{"message":"invalid relation"}`)
		default:
			http.NotFound(writer, request)
		}
	}))
	request := resourcecap.OperationRequest{
		RequestID: "request-security-group-rejected", Capability: resourcecap.CapabilitySecurityGroup,
		ResourceRef: "41", Parameters: []byte(`{"action":"attach","security_group_ref":77}`),
	}
	result, err := fixture.provider.Execute(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != resourcecap.TaskStatusFailed || result.Failure == nil || result.Failure.ResultUnknown {
		t.Fatalf("unexpected result: %+v", result)
	}
	if writes.Load() != 1 || detailReads.Load() != 1 {
		t.Fatalf("writes=%d detail reads=%d", writes.Load(), detailReads.Load())
	}
}

func TestSecurityGroupTransportAmbiguityReconcilesWithoutReplay(t *testing.T) {
	t.Parallel()
	var (
		loginCount int
		loginMu    sync.Mutex
	)
	login := loginHandler(t, &loginCount, &loginMu, "token-one")
	var detailReads atomic.Int64
	fixture := newProviderFixture(t, http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/administrator/v1/login":
			login(writer, request)
		case "/administrator/v1/clouds/41":
			security := "0"
			if detailReads.Add(1) > 1 {
				security = "77"
			}
			writeJSON(t, writer, http.StatusOK,
				`{"data":{"user_id":9,"type":"host","security":`+security+`}}`)
		case "/administrator/v1/security_groups":
			writeJSON(t, writer, http.StatusOK, `{"data":[{"id":77,"type":"host"}]}`)
		default:
			http.NotFound(writer, request)
		}
	}))
	client := &interceptClient{
		base: fixture.client, path: "/administrator/v1/security_groups/77/links",
		err: errors.New("simulated disconnect after send"),
	}
	fixture.provider.env.HTTPClient = client
	request := resourcecap.OperationRequest{
		RequestID: "request-security-group-disconnect", Capability: resourcecap.CapabilitySecurityGroup,
		ResourceRef: "41", Parameters: []byte(`{"action":"attach","security_group_ref":77}`),
	}
	result, err := fixture.provider.Execute(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != resourcecap.TaskStatusSucceeded || len(result.Evidence) != 1 ||
		result.Evidence[0].Kind != "upstream_postcondition" {
		t.Fatalf("unexpected reconciled result: %+v", result)
	}
	if client.attempts.Load() != 1 || detailReads.Load() != 2 {
		t.Fatalf("write attempts=%d detail reads=%d", client.attempts.Load(), detailReads.Load())
	}
}
