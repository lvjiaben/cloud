// This file verifies ISO virtual-media mutations against a controlled upstream.

package zjmfcloud

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"sync"
	"sync/atomic"
	"testing"

	"lina-plugin-idc-resource/backend/cap/resourcecap"
)

func TestVirtualMediaMountAndUnmountMappings(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name          string
		parameters    string
		method        string
		beforeISO     string
		afterISO      string
		wantNodeReads int64
		wantWriteForm url.Values
	}{
		{
			name: "mount", parameters: `{"action":"mount","image_ref":77}`,
			method: http.MethodPost, beforeISO: "0", afterISO: "77", wantNodeReads: 1,
			wantWriteForm: url.Values{"iso": {"77"}},
		},
		{
			name: "unmount", parameters: `{"action":"unmount"}`,
			method: http.MethodDelete, beforeISO: "77", afterISO: "0",
			wantWriteForm: url.Values{"iso": {"77"}},
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			var loginCount int
			var loginMu sync.Mutex
			login := loginHandler(t, &loginCount, &loginMu, "token-one")
			var detailReads atomic.Int64
			var nodeReads atomic.Int64
			var writes atomic.Int64
			fixture := newProviderFixture(t, http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				switch {
				case request.URL.Path == "/administrator/v1/login":
					login(writer, request)
				case request.Method == http.MethodGet && request.URL.Path == "/administrator/v1/clouds/41":
					iso := test.beforeISO
					if detailReads.Add(1) > 1 {
						iso = test.afterISO
					}
					writeJSON(t, writer, http.StatusOK,
						`{"data":{"id":41,"node_id":9,"iso":`+iso+`,"password":"must-not-leak"}}`)
				case request.Method == http.MethodGet && request.URL.Path == "/administrator/v1/node_isos":
					nodeReads.Add(1)
					if request.URL.Query().Encode() != "id=9&type=node" {
						t.Errorf("node ISO query=%v", request.URL.Query())
					}
					writeJSON(t, writer, http.StatusOK,
						`{"data":[{"name":"linux","info":[{"id":77,"name":"recovery.iso","path":"must-not-leak"}]}]}`)
				default:
					writes.Add(1)
					if request.Method != test.method || request.URL.Path != "/administrator/v1/clouds/41/iso" {
						t.Errorf("write=%s %s", request.Method, request.URL.Path)
					}
					body, err := io.ReadAll(request.Body)
					if err != nil {
						t.Errorf("read form: %v", err)
					}
					gotForm, err := url.ParseQuery(string(body))
					if err != nil || gotForm.Encode() != test.wantWriteForm.Encode() {
						t.Errorf("form=%v, want %v, err=%v", gotForm, test.wantWriteForm, err)
					}
					writeJSON(t, writer, http.StatusOK, `{"status":"success"}`)
				}
			}))
			request := resourcecap.OperationRequest{
				RequestID:  "request-virtual-media-" + test.name,
				Capability: resourcecap.CapabilityVirtualMedia, ResourceRef: "41",
				Parameters: []byte(test.parameters),
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
			if detailReads.Load() != 2 || nodeReads.Load() != test.wantNodeReads || writes.Load() != 1 {
				t.Fatalf("detail reads=%d node reads=%d writes=%d",
					detailReads.Load(), nodeReads.Load(), writes.Load())
			}
		})
	}
}

func TestVirtualMediaTaskAcceptanceSkipsPostconditionRead(t *testing.T) {
	t.Parallel()
	var loginCount int
	var loginMu sync.Mutex
	login := loginHandler(t, &loginCount, &loginMu, "token-one")
	var detailReads atomic.Int64
	fixture := newProviderFixture(t, http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/administrator/v1/login":
			login(writer, request)
		case "/administrator/v1/clouds/41":
			detailReads.Add(1)
			writeJSON(t, writer, http.StatusOK, `{"data":{"node_id":9,"iso":0}}`)
		case "/administrator/v1/node_isos":
			writeJSON(t, writer, http.StatusOK, `{"data":[{"info":[{"id":77,"name":"recovery.iso"}]}]}`)
		case "/administrator/v1/clouds/41/iso":
			writeJSON(t, writer, http.StatusAccepted, `{"data":{"taskid":8803}}`)
		default:
			http.NotFound(writer, request)
		}
	}))
	request := resourcecap.OperationRequest{
		RequestID: "request-virtual-media-task", Capability: resourcecap.CapabilityVirtualMedia,
		ResourceRef: "41", Parameters: []byte(`{"action":"mount","image_ref":77}`),
	}
	result, err := fixture.provider.Execute(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != resourcecap.TaskStatusAccepted || result.ExternalTaskRef != "8803" || detailReads.Load() != 1 {
		t.Fatalf("result=%+v detail reads=%d", result, detailReads.Load())
	}
}

func TestVirtualMediaInvalidOrUnownedInputNeverWrites(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		parameters string
		detail     string
		nodeISOs   string
	}{
		{name: "unknown-action", parameters: `{"action":"delete","image_ref":77}`},
		{name: "mount-missing-ref", parameters: `{"action":"mount"}`},
		{name: "mount-force", parameters: `{"action":"mount","image_ref":77,"force":true}`},
		{name: "unmount-with-ref", parameters: `{"action":"unmount","image_ref":77}`},
		{name: "replace-mounted", parameters: `{"action":"mount","image_ref":77}`, detail: `{"data":{"node_id":9,"iso":78}}`},
		{name: "unmount-empty", parameters: `{"action":"unmount"}`, detail: `{"data":{"node_id":9,"iso":0}}`},
		{name: "foreign-iso", parameters: `{"action":"mount","image_ref":77}`, detail: `{"data":{"node_id":9,"iso":0}}`, nodeISOs: `{"data":[{"info":[{"id":78,"name":"other.iso"}]}]}`},
		{name: "duplicate-iso", parameters: `{"action":"mount","image_ref":77}`, detail: `{"data":{"node_id":9,"iso":0}}`, nodeISOs: `{"data":[{"info":[{"id":77,"name":"one.iso"},{"id":77,"name":"two.iso"}]}]}`},
		{name: "malformed-iso", parameters: `{"action":"mount","image_ref":77}`, detail: `{"data":{"node_id":9,"iso":0}}`, nodeISOs: `{"data":[{"info":[{"id":"0077","name":"bad.iso"}]}]}`},
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
				case "/administrator/v1/node_isos":
					writeJSON(t, writer, http.StatusOK, test.nodeISOs)
				default:
					t.Errorf("unexpected write %s %s", request.Method, request.URL.Path)
				}
			}))
			request := resourcecap.OperationRequest{
				RequestID:  "request-virtual-media-invalid-" + test.name,
				Capability: resourcecap.CapabilityVirtualMedia, ResourceRef: "41",
				Parameters: []byte(test.parameters),
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

func TestVirtualMediaUnprovenAndUnauthorizedWritesRemainUnknownWithoutReplay(t *testing.T) {
	t.Parallel()
	for _, status := range []int{http.StatusOK, http.StatusUnauthorized} {
		status := status
		t.Run(http.StatusText(status), func(t *testing.T) {
			t.Parallel()
			var loginCount int
			var loginMu sync.Mutex
			login := loginHandler(t, &loginCount, &loginMu, "token-one")
			var detailReads atomic.Int64
			var writes atomic.Int64
			fixture := newProviderFixture(t, http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				switch request.URL.Path {
				case "/administrator/v1/login":
					login(writer, request)
				case "/administrator/v1/clouds/41":
					detailReads.Add(1)
					writeJSON(t, writer, http.StatusOK, `{"data":{"node_id":9,"iso":0}}`)
				case "/administrator/v1/node_isos":
					writeJSON(t, writer, http.StatusOK, `{"data":[{"info":[{"id":77,"name":"recovery.iso"}]}]}`)
				default:
					writes.Add(1)
					writeJSON(t, writer, status, `{"status":"success"}`)
				}
			}))
			request := resourcecap.OperationRequest{
				RequestID: "request-virtual-media-unproven", Capability: resourcecap.CapabilityVirtualMedia,
				ResourceRef: "41", Parameters: []byte(`{"action":"mount","image_ref":77}`),
			}
			result, err := fixture.provider.Execute(context.Background(), request)
			if err != nil {
				t.Fatal(err)
			}
			wantLogins := 1
			if status == http.StatusUnauthorized {
				wantLogins = 2
			}
			if result.Status != resourcecap.TaskStatusUnknown || result.Failure == nil ||
				!result.Failure.ResultUnknown || writes.Load() != 1 || detailReads.Load() != 2 ||
				loginCount != wantLogins {
				t.Fatalf("result=%+v writes=%d reads=%d logins=%d", result, writes.Load(), detailReads.Load(), loginCount)
			}
		})
	}
}

func TestVirtualMediaTransportAmbiguityReconcilesWithoutReplay(t *testing.T) {
	t.Parallel()
	var loginCount int
	var loginMu sync.Mutex
	login := loginHandler(t, &loginCount, &loginMu, "token-one")
	var detailReads atomic.Int64
	fixture := newProviderFixture(t, http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/administrator/v1/login":
			login(writer, request)
		case "/administrator/v1/clouds/41":
			iso := "0"
			if detailReads.Add(1) > 1 {
				iso = "77"
			}
			writeJSON(t, writer, http.StatusOK, `{"data":{"node_id":9,"iso":`+iso+`}}`)
		case "/administrator/v1/node_isos":
			writeJSON(t, writer, http.StatusOK, `{"data":[{"info":[{"id":77,"name":"recovery.iso"}]}]}`)
		default:
			http.NotFound(writer, request)
		}
	}))
	client := &interceptClient{
		base: fixture.client, path: "/administrator/v1/clouds/41/iso", method: http.MethodPost,
		err: errors.New("simulated disconnect after send"),
	}
	fixture.provider.env.HTTPClient = client
	request := resourcecap.OperationRequest{
		RequestID: "request-virtual-media-disconnect", Capability: resourcecap.CapabilityVirtualMedia,
		ResourceRef: "41", Parameters: []byte(`{"action":"mount","image_ref":77}`),
	}
	result, err := fixture.provider.Execute(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != resourcecap.TaskStatusSucceeded || client.attempts.Load() != 1 || detailReads.Load() != 2 {
		t.Fatalf("result=%+v attempts=%d detail reads=%d", result, client.attempts.Load(), detailReads.Load())
	}
}
