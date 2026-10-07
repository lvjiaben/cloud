// This file verifies temporary-traffic package mutations against a controlled upstream.

package zjmfcloud

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"

	"lina-plugin-idc-resource/backend/cap/resourcecap"
)

func TestTrafficPackageMappingAndPostcondition(t *testing.T) {
	t.Parallel()
	var loginCount int
	var loginMu sync.Mutex
	login := loginHandler(t, &loginCount, &loginMu, "token-one")
	var detailReads atomic.Int64
	var writes atomic.Int64
	fixture := newProviderFixture(t, http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch {
		case request.URL.Path == "/administrator/v1/login":
			login(writer, request)
		case request.Method == http.MethodGet && request.URL.Path == "/administrator/v1/clouds/41":
			temporary := "20"
			if detailReads.Add(1) > 1 {
				temporary = "120"
			}
			writeJSON(t, writer, http.StatusOK,
				`{"data":{"traffic_quota":1000,"tmp_traffic":`+temporary+`,"rootpassword":"must-not-leak"}}`)
		case request.Method == http.MethodPut && request.URL.Path == "/administrator/v1/clouds/41/temp_traffic":
			writes.Add(1)
			if err := request.ParseForm(); err != nil || request.PostForm.Encode() != "traffic=100" {
				t.Errorf("form=%v err=%v", request.PostForm, err)
			}
			writeJSON(t, writer, http.StatusOK, `{"status":"success"}`)
		default:
			http.NotFound(writer, request)
		}
	}))
	request := resourcecap.OperationRequest{
		RequestID: "request-traffic-package-add", Capability: resourcecap.CapabilityTrafficPackage,
		ResourceRef: "41", Parameters: []byte(`{"action":"add","traffic_gb":100}`),
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
	if detailReads.Load() != 2 || writes.Load() != 1 {
		t.Fatalf("detail reads=%d writes=%d", detailReads.Load(), writes.Load())
	}
}

func TestTrafficPackageTaskAcceptanceSkipsPostconditionRead(t *testing.T) {
	t.Parallel()
	var loginCount int
	var loginMu sync.Mutex
	login := loginHandler(t, &loginCount, &loginMu, "token-one")
	var detailReads atomic.Int64
	fixture := newProviderFixture(t, http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch {
		case request.URL.Path == "/administrator/v1/login":
			login(writer, request)
		case request.Method == http.MethodGet && request.URL.Path == "/administrator/v1/clouds/41":
			detailReads.Add(1)
			writeJSON(t, writer, http.StatusOK, `{"data":{"traffic_quota":1000,"tmp_traffic":20}}`)
		case request.Method == http.MethodPut && request.URL.Path == "/administrator/v1/clouds/41/temp_traffic":
			writeJSON(t, writer, http.StatusAccepted, `{"data":{"taskid":8804}}`)
		default:
			http.NotFound(writer, request)
		}
	}))
	request := resourcecap.OperationRequest{
		RequestID: "request-traffic-package-task", Capability: resourcecap.CapabilityTrafficPackage,
		ResourceRef: "41", Parameters: []byte(`{"action":"add","traffic_gb":100}`),
	}
	result, err := fixture.provider.Execute(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != resourcecap.TaskStatusAccepted || result.ExternalTaskRef != "8804" || detailReads.Load() != 1 {
		t.Fatalf("result=%+v detail reads=%d", result, detailReads.Load())
	}
}

func TestTrafficPackageInvalidInputOrStateNeverWrites(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		parameters string
		detail     string
	}{
		{name: "unknown-action", parameters: `{"action":"set","traffic_gb":100}`},
		{name: "missing-amount", parameters: `{"action":"add"}`},
		{name: "zero", parameters: `{"action":"add","traffic_gb":0}`},
		{name: "leading-zero", parameters: `{"action":"add","traffic_gb":"0100"}`},
		{name: "over-limit", parameters: `{"action":"add","traffic_gb":1000001}`},
		{name: "force", parameters: `{"action":"add","traffic_gb":100,"force":true}`},
		{name: "unlimited-instance", parameters: `{"action":"add","traffic_gb":100}`, detail: `{"data":{"traffic_quota":0,"tmp_traffic":20}}`},
		{name: "missing-temporary", parameters: `{"action":"add","traffic_gb":100}`, detail: `{"data":{"traffic_quota":1000}}`},
		{name: "fractional-temporary", parameters: `{"action":"add","traffic_gb":100}`, detail: `{"data":{"traffic_quota":1000,"tmp_traffic":1.5}}`},
		{name: "signed-quota", parameters: `{"action":"add","traffic_gb":100}`, detail: `{"data":{"traffic_quota":"+1000","tmp_traffic":20}}`},
		{name: "signed-temporary", parameters: `{"action":"add","traffic_gb":100}`, detail: `{"data":{"traffic_quota":1000,"tmp_traffic":"+20"}}`},
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
				default:
					t.Errorf("unexpected write %s %s", request.Method, request.URL.Path)
				}
			}))
			request := resourcecap.OperationRequest{
				RequestID:  "request-traffic-invalid-" + test.name,
				Capability: resourcecap.CapabilityTrafficPackage, ResourceRef: "41",
				Parameters: []byte(test.parameters),
			}
			if _, err := fixture.provider.Execute(context.Background(), request); err == nil {
				t.Fatal("expected input or state rejection")
			}
			if test.detail == "" && requests.Load() != 0 {
				t.Fatalf("syntax rejection made %d network requests", requests.Load())
			}
		})
	}
}

func TestTrafficPackageUnprovenAndUnauthorizedWritesRemainUnknownWithoutReplay(t *testing.T) {
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
				switch {
				case request.URL.Path == "/administrator/v1/login":
					login(writer, request)
				case request.Method == http.MethodGet && request.URL.Path == "/administrator/v1/clouds/41":
					detailReads.Add(1)
					writeJSON(t, writer, http.StatusOK, `{"data":{"traffic_quota":1000,"tmp_traffic":20}}`)
				default:
					writes.Add(1)
					writeJSON(t, writer, status, `{"status":"success"}`)
				}
			}))
			request := resourcecap.OperationRequest{
				RequestID: "request-traffic-unproven", Capability: resourcecap.CapabilityTrafficPackage,
				ResourceRef: "41", Parameters: []byte(`{"action":"add","traffic_gb":100}`),
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

func TestTrafficPackageTransportAmbiguityReconcilesWithoutReplay(t *testing.T) {
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
			temporary := "20"
			if detailReads.Add(1) > 1 {
				temporary = "120"
			}
			writeJSON(t, writer, http.StatusOK,
				`{"data":{"traffic_quota":1000,"tmp_traffic":`+temporary+`}}`)
		default:
			http.NotFound(writer, request)
		}
	}))
	client := &interceptClient{
		base: fixture.client, path: "/administrator/v1/clouds/41/temp_traffic", method: http.MethodPut,
		err: errors.New("simulated disconnect after send"),
	}
	fixture.provider.env.HTTPClient = client
	request := resourcecap.OperationRequest{
		RequestID: "request-traffic-disconnect", Capability: resourcecap.CapabilityTrafficPackage,
		ResourceRef: "41", Parameters: []byte(`{"action":"add","traffic_gb":100}`),
	}
	result, err := fixture.provider.Execute(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != resourcecap.TaskStatusSucceeded || client.attempts.Load() != 1 || detailReads.Load() != 2 {
		t.Fatalf("result=%+v attempts=%d reads=%d", result, client.attempts.Load(), detailReads.Load())
	}
}
