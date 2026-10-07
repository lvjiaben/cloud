// This file verifies instance NAT forwarding mutations against a controlled upstream.

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

func TestNATCreateAndDeleteMappings(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		parameters string
		method     string
		path       string
		query      string
		form       url.Values
		before     string
		after      string
	}{
		{
			name: "create", parameters: `{"action":"create","name":"ssh-alt","external_port":10022,"internal_port":22,"protocol":"tcp"}`,
			method: http.MethodPost, path: "/administrator/v1/clouds/41/nat_acl",
			form:   url.Values{"name": {"ssh-alt"}, "ext_port": {"10022"}, "int_port": {"22"}, "protocol": {"1"}},
			before: `{"data":[]}`, after: `{"data":[{"id":71,"name":"ssh-alt","ext_port":10022,"int_port":22,"protocol":1}]}`,
		},
		{
			name: "delete", parameters: `{"action":"delete","nat_ref":71}`,
			method: http.MethodDelete, path: "/administrator/v1/nat_acl/71", query: "hostid=41",
			form:   url.Values{},
			before: `{"data":[{"id":71,"name":"ssh-alt","ext_port":10022,"int_port":22,"protocol":1}]}`, after: `{"data":[]}`,
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			var loginCount int
			var loginMu sync.Mutex
			login := loginHandler(t, &loginCount, &loginMu, "token-one")
			var reads atomic.Int64
			var writes atomic.Int64
			fixture := newProviderFixture(t, http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				switch {
				case request.URL.Path == "/administrator/v1/login":
					login(writer, request)
				case request.Method == http.MethodGet && request.URL.Path == "/administrator/v1/clouds/41/nat_acl":
					if request.URL.Query().Encode() != "list_type=all" {
						t.Errorf("list query=%v", request.URL.Query())
					}
					body := test.before
					if reads.Add(1) > 1 {
						body = test.after
					}
					writeJSON(t, writer, http.StatusOK, body)
				default:
					writes.Add(1)
					if request.Method != test.method || request.URL.Path != test.path || request.URL.RawQuery != test.query {
						t.Errorf("write=%s %s?%s, want %s %s?%s",
							request.Method, request.URL.Path, request.URL.RawQuery, test.method, test.path, test.query)
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
				RequestID: "request-nat-" + test.name, Capability: resourcecap.CapabilityNAT,
				ResourceRef: "41", Parameters: []byte(test.parameters),
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
			if reads.Load() != 2 || writes.Load() != 1 {
				t.Fatalf("reads=%d writes=%d", reads.Load(), writes.Load())
			}
		})
	}
}

func TestNATTaskAcceptanceSkipsImmediatePostconditionRead(t *testing.T) {
	t.Parallel()
	var loginCount int
	var loginMu sync.Mutex
	login := loginHandler(t, &loginCount, &loginMu, "token-one")
	var reads atomic.Int64
	fixture := newProviderFixture(t, http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch {
		case request.URL.Path == "/administrator/v1/login":
			login(writer, request)
		case request.Method == http.MethodGet && request.URL.Path == "/administrator/v1/clouds/41/nat_acl":
			reads.Add(1)
			writeJSON(t, writer, http.StatusOK, `{"data":[]}`)
		case request.Method == http.MethodPost && request.URL.Path == "/administrator/v1/clouds/41/nat_acl":
			writeJSON(t, writer, http.StatusAccepted, `{"data":{"taskid":8802}}`)
		default:
			http.NotFound(writer, request)
		}
	}))
	request := resourcecap.OperationRequest{
		RequestID: "request-nat-task", Capability: resourcecap.CapabilityNAT, ResourceRef: "41",
		Parameters: []byte(`{"action":"create","name":"web-alt","external_port":10443,"internal_port":443,"protocol":"tcp"}`),
	}
	result, err := fixture.provider.Execute(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != resourcecap.TaskStatusAccepted || result.ExternalTaskRef != "8802" || reads.Load() != 1 {
		t.Fatalf("result=%+v reads=%d", result, reads.Load())
	}
}

func TestNATInvalidOrUnownedInputNeverWrites(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		parameters string
		list       string
	}{
		{name: "unknown-action", parameters: `{"action":"gateway-delete","nat_ref":71}`},
		{name: "automatic-external-port", parameters: `{"action":"create","name":"ssh","external_port":0,"internal_port":22,"protocol":"tcp"}`},
		{name: "reserved-external-port", parameters: `{"action":"create","name":"ssh","external_port":22,"internal_port":22,"protocol":"tcp"}`},
		{name: "port-overflow", parameters: `{"action":"create","name":"ssh","external_port":65536,"internal_port":22,"protocol":"tcp"}`},
		{name: "numeric-protocol", parameters: `{"action":"create","name":"ssh","external_port":10022,"internal_port":22,"protocol":1}`},
		{name: "create-with-ref", parameters: `{"action":"create","nat_ref":71,"name":"ssh","external_port":10022,"internal_port":22,"protocol":"tcp"}`},
		{name: "delete-with-force", parameters: `{"action":"delete","nat_ref":71,"force":true}`},
		{name: "unowned-delete", parameters: `{"action":"delete","nat_ref":71}`, list: `{"data":[{"id":72,"name":"other","ext_port":10023,"int_port":23,"protocol":1}]}`},
		{name: "duplicate-delete-reference", parameters: `{"action":"delete","nat_ref":71}`, list: `{"data":[{"id":71,"name":"one","ext_port":10023,"int_port":23,"protocol":1},{"id":71,"name":"two","ext_port":10024,"int_port":24,"protocol":1}]}`},
		{name: "overlapping-port", parameters: `{"action":"create","name":"new","external_port":10022,"internal_port":22,"protocol":"udp"}`, list: `{"data":[{"id":71,"name":"both","ext_port":10022,"int_port":22,"protocol":3}]}`},
		{name: "malformed-list", parameters: `{"action":"delete","nat_ref":71}`, list: `{"data":[{"id":71,"name":"bad","ext_port":70000,"int_port":22,"protocol":1}]}`},
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
				case "/administrator/v1/clouds/41/nat_acl":
					writeJSON(t, writer, http.StatusOK, test.list)
				default:
					t.Errorf("unexpected write %s %s", request.Method, request.URL.Path)
				}
			}))
			request := resourcecap.OperationRequest{
				RequestID: "request-nat-invalid-" + test.name, Capability: resourcecap.CapabilityNAT,
				ResourceRef: "41", Parameters: []byte(test.parameters),
			}
			if _, err := fixture.provider.Execute(context.Background(), request); err == nil {
				t.Fatal("expected input or ownership rejection")
			}
			if test.list == "" && requests.Load() != 0 {
				t.Fatalf("syntax rejection made %d network requests", requests.Load())
			}
		})
	}
}

func TestNATUnprovenOrDuplicatePostconditionRemainsUnknownWithoutReplay(t *testing.T) {
	t.Parallel()
	var loginCount int
	var loginMu sync.Mutex
	login := loginHandler(t, &loginCount, &loginMu, "token-one")
	var reads atomic.Int64
	var writes atomic.Int64
	fixture := newProviderFixture(t, http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch {
		case request.URL.Path == "/administrator/v1/login":
			login(writer, request)
		case request.Method == http.MethodGet && request.URL.Path == "/administrator/v1/clouds/41/nat_acl":
			reads.Add(1)
			writeJSON(t, writer, http.StatusOK, `{"data":[]}`)
		default:
			writes.Add(1)
			writeJSON(t, writer, http.StatusOK, `{"status":"success"}`)
		}
	}))
	request := resourcecap.OperationRequest{
		RequestID: "request-nat-unproven", Capability: resourcecap.CapabilityNAT, ResourceRef: "41",
		Parameters: []byte(`{"action":"create","name":"ssh-alt","external_port":10022,"internal_port":22,"protocol":"tcp"}`),
	}
	result, err := fixture.provider.Execute(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != resourcecap.TaskStatusUnknown || result.Failure == nil || !result.Failure.ResultUnknown ||
		writes.Load() != 1 || reads.Load() != 2 {
		t.Fatalf("result=%+v writes=%d reads=%d", result, writes.Load(), reads.Load())
	}
}

func TestNATUnauthorizedWriteIsNeverReplayed(t *testing.T) {
	t.Parallel()
	var loginCount int
	var loginMu sync.Mutex
	login := loginHandler(t, &loginCount, &loginMu, "token-one")
	var reads atomic.Int64
	var writes atomic.Int64
	fixture := newProviderFixture(t, http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch {
		case request.URL.Path == "/administrator/v1/login":
			login(writer, request)
		case request.Method == http.MethodGet && request.URL.Path == "/administrator/v1/clouds/41/nat_acl":
			reads.Add(1)
			writeJSON(t, writer, http.StatusOK, `{"data":[]}`)
		default:
			writes.Add(1)
			writeJSON(t, writer, http.StatusUnauthorized, `{"message":"expired"}`)
		}
	}))
	request := resourcecap.OperationRequest{
		RequestID: "request-nat-unauthorized", Capability: resourcecap.CapabilityNAT, ResourceRef: "41",
		Parameters: []byte(`{"action":"create","name":"ssh-alt","external_port":10022,"internal_port":22,"protocol":"tcp"}`),
	}
	result, err := fixture.provider.Execute(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != resourcecap.TaskStatusUnknown || writes.Load() != 1 || reads.Load() != 2 || loginCount != 2 {
		t.Fatalf("result=%+v writes=%d reads=%d logins=%d", result, writes.Load(), reads.Load(), loginCount)
	}
}

func TestNATTransportAmbiguityReconcilesWithoutReplay(t *testing.T) {
	t.Parallel()
	var loginCount int
	var loginMu sync.Mutex
	login := loginHandler(t, &loginCount, &loginMu, "token-one")
	var reads atomic.Int64
	fixture := newProviderFixture(t, http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/administrator/v1/login":
			login(writer, request)
		case "/administrator/v1/clouds/41/nat_acl":
			body := `{"data":[]}`
			if reads.Add(1) > 1 {
				body = `{"data":[{"id":71,"name":"ssh-alt","ext_port":10022,"int_port":22,"protocol":1}]}`
			}
			writeJSON(t, writer, http.StatusOK, body)
		default:
			http.NotFound(writer, request)
		}
	}))
	client := &interceptClient{
		base: fixture.client, path: "/administrator/v1/clouds/41/nat_acl", method: http.MethodPost,
		err: errors.New("simulated disconnect after send"),
	}
	fixture.provider.env.HTTPClient = client
	request := resourcecap.OperationRequest{
		RequestID: "request-nat-disconnect", Capability: resourcecap.CapabilityNAT, ResourceRef: "41",
		Parameters: []byte(`{"action":"create","name":"ssh-alt","external_port":10022,"internal_port":22,"protocol":"tcp"}`),
	}
	result, err := fixture.provider.Execute(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != resourcecap.TaskStatusSucceeded || client.attempts.Load() != 1 || reads.Load() != 2 {
		t.Fatalf("result=%+v attempts=%d reads=%d", result, client.attempts.Load(), reads.Load())
	}
}
