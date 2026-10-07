// This file verifies lifecycle write mappings and ambiguous-result safety.

package zjmfcloud

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"

	"lina-plugin-idc-core/backend/cap/secretcap"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
)

type interceptClient struct {
	base     resourcecap.HTTPClient
	path     string
	method   string
	err      error
	attempts atomic.Int64
}

func (c *interceptClient) Do(request *http.Request) (*http.Response, error) {
	if request.URL.Path == c.path && (c.method == "" || request.Method == c.method) {
		c.attempts.Add(1)
		return nil, c.err
	}
	return c.base.Do(request)
}

func TestLifecycleWriteReturnsNormalizedTaskAndResourceReferences(t *testing.T) {
	t.Parallel()
	var loginCount int
	var mu sync.Mutex
	login := loginHandler(t, &loginCount, &mu, "token-one")
	var actionCount atomic.Int64
	fixture := newProviderFixture(t, http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/administrator/v1/login" {
			login(writer, request)
			return
		}
		if request.URL.Path != "/administrator/v1/clouds/41/on" || request.Method != http.MethodPost {
			t.Errorf("unexpected lifecycle request %s %s", request.Method, request.URL.Path)
		}
		if request.Header.Get("access-token") != "token-one" {
			t.Error("lifecycle request omitted the access token")
		}
		actionCount.Add(1)
		writeJSON(t, writer, http.StatusAccepted, `{"taskid":7001,"password":"must-not-leak"}`)
	}))
	result, err := fixture.provider.Execute(context.Background(), resourcecap.OperationRequest{
		RequestID: "request-7001", Capability: resourcecap.CapabilityStart,
		ResourceRef: "41", Parameters: []byte(`{}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != resourcecap.TaskStatusAccepted || result.ExternalTaskRef != "7001" ||
		result.ExternalResourceRef != "41" {
		t.Fatalf("unexpected operation result: %+v", result)
	}
	if err := result.Validate(); err != nil {
		t.Fatalf("validate operation result: %v", err)
	}
	if actionCount.Load() != 1 {
		t.Fatalf("action attempts=%d, want 1", actionCount.Load())
	}
}

func TestCoreLifecycleMappingsUseExactVerifiedRoutes(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		capability resourcecap.CapabilityCode
		parameters string
		method     string
		path       string
		form       string
	}{
		{name: "start", capability: resourcecap.CapabilityStart, parameters: `{}`, method: http.MethodPost, path: "/administrator/v1/clouds/41/on"},
		{name: "stop graceful", capability: resourcecap.CapabilityStop, parameters: `{}`, method: http.MethodPost, path: "/administrator/v1/clouds/41/off"},
		{name: "stop hard", capability: resourcecap.CapabilityStop, parameters: `{"mode":"hard"}`, method: http.MethodPost, path: "/administrator/v1/clouds/41/hardoff"},
		{name: "reboot graceful", capability: resourcecap.CapabilityReboot, parameters: `{"mode":"graceful"}`, method: http.MethodPost, path: "/administrator/v1/clouds/41/reboot"},
		{name: "reboot hard", capability: resourcecap.CapabilityReboot, parameters: `{"mode":"hard"}`, method: http.MethodPost, path: "/administrator/v1/clouds/41/hard_reboot"},
		{name: "suspend", capability: resourcecap.CapabilitySuspend, parameters: `{"reason":"due","free_gpu":true}`, method: http.MethodPost, path: "/administrator/v1/clouds/41/suspend", form: "free_gpu=true&type=due"},
		{name: "resume", capability: resourcecap.CapabilityResume, parameters: `{}`, method: http.MethodPost, path: "/administrator/v1/clouds/41/unsuspend"},
		{name: "resize", capability: resourcecap.CapabilityResize, parameters: `{"cpu":4,"memory":8192,"in_bw":100,"out_bw":100,"system_disk_size":80}`, method: http.MethodPut, path: "/administrator/v1/clouds/41", form: "cpu=4&in_bw=100&memory=8192&out_bw=100&system_disk_size=80"},
		{name: "terminate", capability: resourcecap.CapabilityTerminate, parameters: `{}`, method: http.MethodDelete, path: "/administrator/v1/clouds/41"},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			var (
				loginCount  int
				mu          sync.Mutex
				actionCount atomic.Int64
			)
			login := loginHandler(t, &loginCount, &mu, "token-one")
			fixture := newProviderFixture(t, http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				if request.URL.Path == "/administrator/v1/login" {
					login(writer, request)
					return
				}
				actionCount.Add(1)
				if request.Method != test.method || request.URL.Path != test.path || request.URL.RawQuery != "" {
					t.Errorf("lifecycle request=%s %s, want %s %s", request.Method, request.URL.RequestURI(), test.method, test.path)
				}
				if err := request.ParseForm(); err != nil {
					t.Errorf("parse lifecycle form: %v", err)
				} else if request.Form.Encode() != test.form {
					t.Errorf("lifecycle form=%q, want %q", request.Form.Encode(), test.form)
				}
				writeJSON(t, writer, http.StatusAccepted, `{"taskid":7101,"password":"must-not-leak"}`)
			}))
			result, err := fixture.provider.Execute(context.Background(), resourcecap.OperationRequest{
				RequestID:   "request-lifecycle-" + strconv.Itoa(len(test.name)),
				Capability:  test.capability,
				ResourceRef: "41",
				Parameters:  []byte(test.parameters),
			})
			if err != nil {
				t.Fatal(err)
			}
			if result.Status != resourcecap.TaskStatusAccepted || result.ExternalTaskRef != "7101" ||
				result.ExternalResourceRef != "41" {
				t.Fatalf("unexpected lifecycle result: %+v", result)
			}
			if actionCount.Load() != 1 {
				t.Fatalf("lifecycle action count=%d, want exactly 1", actionCount.Load())
			}
		})
	}
}

func TestAmbiguousWriteTransportFailureIsUnknownAndNeverRetried(t *testing.T) {
	t.Parallel()
	var loginCount int
	var mu sync.Mutex
	login := loginHandler(t, &loginCount, &mu, "token-one")
	fixture := newProviderFixture(t, http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/administrator/v1/login" {
			login(writer, request)
			return
		}
		writeJSON(t, writer, http.StatusInternalServerError, `{}`)
	}))
	client := &interceptClient{
		base: fixture.client, path: "/administrator/v1/clouds/41/on", err: errors.New("simulated disconnect"),
	}
	fixture.provider.env.HTTPClient = client
	result, err := fixture.provider.Execute(context.Background(), resourcecap.OperationRequest{
		RequestID: "request-unknown", Capability: resourcecap.CapabilityStart,
		ResourceRef: "41", Parameters: []byte(`{}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != resourcecap.TaskStatusUnknown || result.Failure == nil ||
		!result.Failure.ResultUnknown || result.Failure.Retryable {
		t.Fatalf("ambiguous write did not fail closed: %+v", result)
	}
	if client.attempts.Load() != 1 {
		t.Fatalf("ambiguous write attempts=%d, want exactly 1", client.attempts.Load())
	}
}

func TestInitialAuthenticationFailureDoesNotProduceAnUnknownWrite(t *testing.T) {
	t.Parallel()
	for _, statusCode := range []int{http.StatusRequestTimeout, http.StatusServiceUnavailable} {
		statusCode := statusCode
		t.Run(strconv.Itoa(statusCode), func(t *testing.T) {
			t.Parallel()
			var actionCalls atomic.Int64
			fixture := newProviderFixture(t, http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				if request.URL.Path == "/administrator/v1/login" {
					writeJSON(t, writer, statusCode, `{"error":"discard-me"}`)
					return
				}
				actionCalls.Add(1)
				writeJSON(t, writer, http.StatusAccepted, `{"taskid":7001}`)
			}))
			result, err := fixture.provider.Execute(context.Background(), resourcecap.OperationRequest{
				RequestID:   "authentication-preflight-" + strconv.Itoa(statusCode),
				Capability:  resourcecap.CapabilityStart,
				ResourceRef: "41",
				Parameters:  []byte(`{}`),
			})
			var callError *resourcecap.ProviderCallError
			if result != nil || !errors.As(err, &callError) || !callError.Retryable ||
				callError.ResultUnknown {
				t.Fatalf("preflight status %d result=%+v err=%v", statusCode, result, err)
			}
			wantCode := resourcecap.ProviderCallErrorCodeTransport
			if statusCode == http.StatusRequestTimeout {
				wantCode = resourcecap.ProviderCallErrorCodeTimeout
			}
			if callError.Code != wantCode {
				t.Fatalf("preflight status %d code=%s, want %s", statusCode, callError.Code, wantCode)
			}
			if actionCalls.Load() != 0 {
				t.Fatalf("preflight failure reached the write endpoint %d times", actionCalls.Load())
			}
		})
	}
}

func TestUnauthorizedWriteIsUnknownAndNeverRetried(t *testing.T) {
	t.Parallel()
	var loginCount atomic.Int64
	var actionCount atomic.Int64
	fixture := newProviderFixture(t, http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/administrator/v1/login" {
			count := loginCount.Add(1)
			writeJSON(t, writer, http.StatusCreated, `"token-`+strconv.FormatInt(count, 10)+`"`)
			return
		}
		actionCount.Add(1)
		if request.Header.Get("Access-Token") != "token-1" {
			t.Error("write used an unexpected token")
		}
		writeJSON(t, writer, http.StatusUnauthorized, `{"error":"expired","password":"discard-me"}`)
	}))
	result, err := fixture.provider.Execute(context.Background(), resourcecap.OperationRequest{
		RequestID: "request-unauthorized-write", Capability: resourcecap.CapabilityStart,
		ResourceRef: "41", Parameters: []byte(`{}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != resourcecap.TaskStatusUnknown || result.Failure == nil ||
		!result.Failure.ResultUnknown || loginCount.Load() != 1 || actionCount.Load() != 1 {
		t.Fatalf("unexpected unauthorized write result=%+v login=%d action=%d",
			result, loginCount.Load(), actionCount.Load())
	}
}

func TestUnauthorizedWriteRefreshesOnlyForTheNextOwnerAttempt(t *testing.T) {
	t.Parallel()
	var loginCount atomic.Int64
	var actionCount atomic.Int64
	fixture := newProviderFixture(t, http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/administrator/v1/login" {
			count := loginCount.Add(1)
			writeJSON(t, writer, http.StatusCreated, `"token-`+strconv.FormatInt(count, 10)+`"`)
			return
		}
		call := actionCount.Add(1)
		if call == 1 {
			writeJSON(t, writer, http.StatusUnauthorized, `{"error":"rejected","token":"discard-me"}`)
			return
		}
		if request.Header.Get("Access-Token") != "token-2" {
			t.Error("next owner attempt did not use a refreshed token")
		}
		writeJSON(t, writer, http.StatusAccepted, `{"taskid":7002}`)
	}))
	request := resourcecap.OperationRequest{
		RequestID: "request-refresh-next-write", Capability: resourcecap.CapabilityStart,
		ResourceRef: "41", Parameters: []byte(`{}`),
	}
	first, err := fixture.provider.Execute(context.Background(), request)
	if err != nil || first == nil || first.Status != resourcecap.TaskStatusUnknown {
		t.Fatalf("first unauthorized write result=%+v err=%v", first, err)
	}
	request.RequestID = "request-refresh-next-write-2"
	second, err := fixture.provider.Execute(context.Background(), request)
	if err != nil || second == nil || second.Status != resourcecap.TaskStatusAccepted ||
		second.ExternalTaskRef != "7002" {
		t.Fatalf("next owner write result=%+v err=%v", second, err)
	}
	if loginCount.Load() != 2 || actionCount.Load() != 2 {
		t.Fatalf("separate unauthorized writes login=%d action=%d, want exactly two each",
			loginCount.Load(), actionCount.Load())
	}
}

func TestDocumentedRejectionIsFailedWithoutRawErrorBody(t *testing.T) {
	t.Parallel()
	var loginCount int
	var mu sync.Mutex
	login := loginHandler(t, &loginCount, &mu, "token-one")
	fixture := newProviderFixture(t, http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/administrator/v1/login" {
			login(writer, request)
			return
		}
		writeJSON(t, writer, http.StatusBadRequest, `{"error":"apiKey=must-not-leak"}`)
	}))
	result, err := fixture.provider.Execute(context.Background(), resourcecap.OperationRequest{
		RequestID: "request-rejected", Capability: resourcecap.CapabilityStart,
		ResourceRef: "41", Parameters: []byte(`{}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != resourcecap.TaskStatusFailed || result.Failure == nil || result.Failure.Retryable {
		t.Fatalf("unexpected rejected result: %+v", result)
	}
	if result.Failure.Message == "apiKey=must-not-leak" {
		t.Fatal("raw upstream error body escaped into the normalized failure")
	}
}

func TestChangeIPAlwaysRequiresOwnerReconciliation(t *testing.T) {
	t.Parallel()
	for name, body := range map[string]string{
		"empty response": `{}`,
		"task response":  `{"taskid":7102}`,
	} {
		name, body := name, body
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			var loginCount int
			var mu sync.Mutex
			login := loginHandler(t, &loginCount, &mu, "token-one")
			var actionCount atomic.Int64
			fixture := newProviderFixture(t, http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				if request.URL.Path == "/administrator/v1/login" {
					login(writer, request)
					return
				}
				actionCount.Add(1)
				if request.Method != http.MethodPut || request.URL.Path != "/administrator/v1/clouds/41/ip" {
					t.Errorf("IP change request=%s %s", request.Method, request.URL.RequestURI())
				}
				if err := request.ParseForm(); err != nil {
					t.Errorf("parse IP change form: %v", err)
				}
				if request.Form.Encode() != "ip_group=9&num=2" || request.Form.Has("ip_num") {
					t.Errorf("IP change form=%v, want normalized num and ip_group", request.Form)
				}
				writeJSON(t, writer, http.StatusOK, body)
			}))
			result, err := fixture.provider.Execute(context.Background(), resourcecap.OperationRequest{
				RequestID: "request-ip-change-" + strconv.Itoa(len(name)), Capability: resourcecap.CapabilityChangeIP,
				ResourceRef: "41", Parameters: []byte(`{"ip_num":2,"ip_group":9}`),
			})
			if err != nil {
				t.Fatal(err)
			}
			if result.Status != resourcecap.TaskStatusUnknown || result.Failure == nil || !result.Failure.ResultUnknown {
				t.Fatalf("IP change escaped owner reconciliation: %+v", result)
			}
			if actionCount.Load() != 1 {
				t.Fatalf("IP change attempts=%d, want exactly 1", actionCount.Load())
			}
		})
	}
}

func TestDangerousOrConfidentialOperationsFailBeforeNetwork(t *testing.T) {
	t.Parallel()
	var requests atomic.Int64
	fixture := newProviderFixture(t, http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		writeJSON(t, writer, http.StatusInternalServerError, `{}`)
	}))
	unexpectedSecret, err := secretcap.NewHandle("sec_zjmf_unexpected_operation")
	if err != nil {
		t.Fatal(err)
	}
	tests := []resourcecap.OperationRequest{
		{
			RequestID: "terminate-force", Capability: resourcecap.CapabilityTerminate,
			ResourceRef: "41", Parameters: []byte(`{"force":true}`),
		},
		{
			RequestID: "provision-batch", Capability: resourcecap.CapabilityProvision,
			Parameters: []byte(`{"cpu":2,"memory":2048,"os":5,"num":2}`),
		},
		{
			RequestID: "console-output", Capability: resourcecap.CapabilityConsole,
			ResourceRef: "41", Parameters: []byte(`{"raw_ticket":"forbidden"}`),
		},
		{
			RequestID: "hidden-start-secret", Capability: resourcecap.CapabilityStart,
			ResourceRef: "41", Parameters: []byte(`{}`),
			SecretBindings: []resourcecap.SecretBinding{{Name: "hidden", Handle: unexpectedSecret}},
		},
	}
	for _, request := range tests {
		if _, err := fixture.provider.Execute(context.Background(), request); err == nil {
			t.Errorf("capability %s was accepted", request.Capability)
		}
	}
	if requests.Load() != 0 {
		t.Fatalf("invalid operations made %d network requests", requests.Load())
	}
}

func TestResetPasswordReadsOnlyTheDynamicSecretBinding(t *testing.T) {
	t.Parallel()
	var loginCount int
	var mu sync.Mutex
	login := loginHandler(t, &loginCount, &mu, "token-one")
	fixture := newProviderFixture(t, http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/administrator/v1/login" {
			login(writer, request)
			return
		}
		if request.URL.Path != "/administrator/v1/clouds/41/password" || request.Method != http.MethodPut {
			t.Errorf("unexpected password request %s %s", request.Method, request.URL.Path)
		}
		if err := request.ParseForm(); err != nil {
			t.Errorf("parse password form: %v", err)
		}
		if request.Form.Get("password") != "N3w p@ss+word" || len(request.Form) != 1 {
			t.Error("password write did not receive exactly the scoped dynamic secret")
		}
		writeJSON(t, writer, http.StatusAccepted, `{"taskid":7100}`)
	}))
	password, err := secretcap.NewHandle("sec_zjmf_operation_password")
	if err != nil {
		t.Fatal(err)
	}
	fixture.secrets.put(password, []byte("N3w p@ss+word"))
	request := resourcecap.OperationRequest{
		RequestID: "request-password-7100", Capability: resourcecap.CapabilityResetPassword,
		ResourceRef: "41", Parameters: []byte(`{}`),
		SecretBindings: []resourcecap.SecretBinding{{Name: resetPasswordBinding, Handle: password}},
	}
	result, err := fixture.provider.Execute(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != resourcecap.TaskStatusAccepted || result.ExternalTaskRef != "7100" ||
		len(result.SecretOutputs) != 0 {
		t.Fatalf("unexpected password result status=%s task=%s outputs=%d",
			result.Status, result.ExternalTaskRef, len(result.SecretOutputs))
	}
	if err = result.ValidateFor(request); err != nil {
		t.Fatalf("validate password result: %v", err)
	}
	if fixture.secrets.readCount(password) != 1 || len(fixture.sink.requestSnapshot()) != 0 {
		t.Fatal("password input did not remain on the one-way accessor path")
	}
}

func TestProvisionUsesScopedPasswordAndDiscardsResponseCredentials(t *testing.T) {
	t.Parallel()
	var loginCount int
	var mu sync.Mutex
	login := loginHandler(t, &loginCount, &mu, "token-one")
	fixture := newProviderFixture(t, http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/administrator/v1/login" {
			login(writer, request)
			return
		}
		if request.URL.Path != "/administrator/v1/clouds" || request.Method != http.MethodPost {
			t.Errorf("unexpected provision request %s %s", request.Method, request.URL.Path)
		}
		if err := request.ParseForm(); err != nil {
			t.Errorf("parse provision form: %v", err)
		}
		if request.Form.Get("client") != "91" || request.Form.Get("hostname") != "srv-001.example" ||
			request.Form.Get("rootpass") != "G3nerated p@ss+word" || request.Form.Get("num") != "1" ||
			request.Form.Has("password") {
			t.Errorf("provision form did not preserve the verified contract: %v", request.Form)
		}
		writeJSON(t, writer, http.StatusAccepted,
			`{"data":{"id":42,"taskid":8001,"rootpassword":"discard-me","osuser":"root"}}`)
	}))
	password, err := secretcap.NewHandle("sec_zjmf_provision_password")
	if err != nil {
		t.Fatal(err)
	}
	fixture.secrets.put(password, []byte("G3nerated p@ss+word"))
	request := resourcecap.OperationRequest{
		RequestID: "request-provision-8001", Capability: resourcecap.CapabilityProvision,
		Parameters:     []byte(`{"cpu":2,"memory":2048,"os":5,"client":91,"hostname":"srv-001.example"}`),
		SecretBindings: []resourcecap.SecretBinding{{Name: resetPasswordBinding, Handle: password}},
	}
	result, err := fixture.provider.Execute(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != resourcecap.TaskStatusAccepted || result.ExternalTaskRef != "8001" ||
		result.ExternalResourceRef != "42" || len(result.SecretOutputs) != 0 {
		t.Fatalf("unexpected provision result: %+v", result)
	}
	if err := result.ValidateFor(request); err != nil {
		t.Fatalf("validate provision result: %v", err)
	}
	if fixture.secrets.readCount(password) != 1 || len(fixture.sink.requestSnapshot()) != 0 {
		t.Fatal("provision password did not remain on the one-way input path")
	}
}

func TestRescueOperationsUseScopedTemporaryPasswordAndReturnTasks(t *testing.T) {
	t.Parallel()
	var (
		loginCount int
		mu         sync.Mutex
		calls      atomic.Int64
	)
	login := loginHandler(t, &loginCount, &mu, "token-one")
	fixture := newProviderFixture(t, http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/administrator/v1/login" {
			login(writer, request)
			return
		}
		if request.URL.Path != "/administrator/v1/clouds/41/rescue" {
			t.Errorf("unexpected rescue path %s", request.URL.Path)
		}
		switch calls.Add(1) {
		case 1:
			if request.Method != http.MethodPost {
				t.Errorf("enter rescue method=%s, want POST", request.Method)
			}
			if err := request.ParseForm(); err != nil {
				t.Errorf("parse rescue form: %v", err)
			}
			if request.Form.Get("type") != "1" ||
				request.Form.Get("temp_pass") != "T3mp p@ss+word" ||
				request.Form.Has("system") || request.Form.Has(rescuePasswordBinding) ||
				len(request.Form) != 2 {
				t.Errorf("enter rescue form violated the verified contract: %v", request.Form)
			}
			writeJSON(t, writer, http.StatusAccepted,
				`{"data":{"taskid":7301,"temp_pass":"discard-me"}}`)
		case 2:
			if request.Method != http.MethodDelete {
				t.Errorf("exit rescue method=%s, want DELETE", request.Method)
			}
			writeJSON(t, writer, http.StatusAccepted, `{"data":{"taskid":7302}}`)
		default:
			t.Errorf("unexpected extra rescue request")
			writeJSON(t, writer, http.StatusInternalServerError, `{}`)
		}
	}))
	temporaryPassword, err := secretcap.NewHandle("sec_zjmf_rescue_password")
	if err != nil {
		t.Fatal(err)
	}
	fixture.secrets.put(temporaryPassword, []byte("T3mp p@ss+word"))
	enter := resourcecap.OperationRequest{
		RequestID: "request-rescue-enter-7301", Capability: resourcecap.CapabilityEnterRescue,
		ResourceRef: "41", Parameters: []byte(`{"system":"linux"}`),
		SecretBindings: []resourcecap.SecretBinding{{Name: rescuePasswordBinding, Handle: temporaryPassword}},
	}
	result, err := fixture.provider.Execute(context.Background(), enter)
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != resourcecap.TaskStatusAccepted || result.ExternalTaskRef != "7301" ||
		result.ExternalResourceRef != "41" || len(result.SecretOutputs) != 0 {
		t.Fatalf("unexpected enter rescue result: %+v", result)
	}
	if err = result.ValidateFor(enter); err != nil {
		t.Fatalf("validate enter rescue result: %v", err)
	}
	if fixture.secrets.readCount(temporaryPassword) != 1 {
		t.Fatal("temporary rescue password did not remain on the one-way accessor path")
	}
	exit := resourcecap.OperationRequest{
		RequestID: "request-rescue-exit-7302", Capability: resourcecap.CapabilityExitRescue,
		ResourceRef: "41", Parameters: []byte(`{}`),
	}
	result, err = fixture.provider.Execute(context.Background(), exit)
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != resourcecap.TaskStatusAccepted || result.ExternalTaskRef != "7302" ||
		result.ExternalResourceRef != "41" || len(result.SecretOutputs) != 0 {
		t.Fatalf("unexpected exit rescue result: %+v", result)
	}
	if err = result.ValidateFor(exit); err != nil {
		t.Fatalf("validate exit rescue result: %v", err)
	}
}

func TestInvalidRescueInputsFailBeforeNetwork(t *testing.T) {
	t.Parallel()
	var requests atomic.Int64
	fixture := newProviderFixture(t, http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		writeJSON(t, writer, http.StatusInternalServerError, `{}`)
	}))
	password, err := secretcap.NewHandle("sec_zjmf_invalid_rescue_password")
	if err != nil {
		t.Fatal(err)
	}
	fixture.secrets.put(password, []byte("temporary-secret"))
	tests := []resourcecap.OperationRequest{
		{
			RequestID: "rescue-missing-secret", Capability: resourcecap.CapabilityEnterRescue,
			ResourceRef: "41", Parameters: []byte(`{"system":"linux"}`),
		},
		{
			RequestID: "rescue-unknown-system", Capability: resourcecap.CapabilityEnterRescue,
			ResourceRef: "41", Parameters: []byte(`{"system":"other"}`),
			SecretBindings: []resourcecap.SecretBinding{{Name: rescuePasswordBinding, Handle: password}},
		},
		{
			RequestID: "rescue-secret-on-exit", Capability: resourcecap.CapabilityExitRescue,
			ResourceRef: "41", Parameters: []byte(`{}`),
			SecretBindings: []resourcecap.SecretBinding{{Name: rescuePasswordBinding, Handle: password}},
		},
	}
	for _, request := range tests {
		if _, err := fixture.provider.Execute(context.Background(), request); err == nil {
			t.Errorf("invalid rescue request %s was accepted", request.RequestID)
		}
	}
	if requests.Load() != 0 {
		t.Fatalf("invalid rescue inputs made %d network requests", requests.Load())
	}
}

func TestReinstallEncryptsGeneratedCredentialBeforeReturning(t *testing.T) {
	t.Parallel()
	var loginCount int
	var mu sync.Mutex
	login := loginHandler(t, &loginCount, &mu, "token-one")
	fixture := newProviderFixture(t, http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/administrator/v1/login" {
			login(writer, request)
			return
		}
		if request.URL.Path != "/administrator/v1/clouds/41/reinstall" || request.Method != http.MethodPut {
			t.Errorf("unexpected reinstall request %s %s", request.Method, request.URL.Path)
		}
		if err := request.ParseForm(); err != nil {
			t.Errorf("parse reinstall form: %v", err)
		}
		if request.Form.Get("os") != "9" || request.Form.Get("port") != "2222" ||
			request.Form.Get("format_data_disk") != "1" {
			t.Error("reinstall form was not normalized")
		}
		writeJSON(t, writer, http.StatusAccepted,
			`{"data":{"id":41,"taskid":7200,"user":"root","password":"generated-secret","port":2222,"apiKey":"discard-me"}}`)
	}))
	request := resourcecap.OperationRequest{
		RequestID: "request-reinstall-7200", Capability: resourcecap.CapabilityReinstall,
		ResourceRef: "41", Parameters: []byte(`{"os":9,"port":2222,"format_data_disk":1}`),
	}
	result, err := fixture.provider.Execute(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != resourcecap.TaskStatusAccepted || result.ExternalTaskRef != "7200" ||
		len(result.SecretOutputs) != 1 {
		t.Fatalf("unexpected reinstall result status=%s task=%s outputs=%d",
			result.Status, result.ExternalTaskRef, len(result.SecretOutputs))
	}
	if err = result.ValidateFor(request); err != nil {
		t.Fatalf("validate reinstall result: %v", err)
	}
	if result.Validate() == nil {
		t.Fatal("context-free validation accepted a confidential output")
	}
	output := result.SecretOutputs[0]
	plaintext, ok := fixture.sink.load(output.Handle)
	if !ok {
		t.Fatal("reinstall credential was not stored by the owner sink")
	}
	defer clear(plaintext)
	var credential map[string]any
	if json.Unmarshal(plaintext, &credential) != nil || credential["username"] != "root" ||
		credential["password"] != "generated-secret" || credential["apiKey"] != nil {
		t.Fatal("reinstall credential projection was incomplete or retained an unapproved field")
	}
	requests := fixture.sink.requestSnapshot()
	if len(requests) != 1 || requests[0].Name != reinstallOutputName ||
		requests[0].Policy != resourcecap.SecretOutputPolicyOneTimeCredential || !requests[0].ExpiresAt.IsZero() {
		t.Fatal("reinstall output did not use the one-time owner policy")
	}
}

func TestConsoleReturnsOnlyAnEncryptedShortLivedTicket(t *testing.T) {
	t.Parallel()
	var loginCount int
	var mu sync.Mutex
	login := loginHandler(t, &loginCount, &mu, "token-one")
	fixture := newProviderFixture(t, http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/administrator/v1/login" {
			login(writer, request)
			return
		}
		if request.URL.Path != "/administrator/v1/clouds/41/vnc" || request.Method != http.MethodPost {
			t.Errorf("unexpected console request %s %s", request.Method, request.URL.Path)
		}
		writeJSON(t, writer, http.StatusOK,
			`{"data":{"vnc_url":"wss://`+request.Host+`/socket","token":"ticket-secret","vnc_pass":"console-secret","api_key":"discard-me"}}`)
	}))
	request := resourcecap.OperationRequest{
		RequestID: "request-console", Capability: resourcecap.CapabilityConsole,
		ResourceRef: "41", Parameters: []byte(`{}`),
	}
	result, err := fixture.provider.Execute(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != resourcecap.TaskStatusSucceeded || result.ExternalResourceRef != "41" ||
		len(result.SecretOutputs) != 1 {
		t.Fatalf("unexpected console result status=%s resource=%s outputs=%d",
			result.Status, result.ExternalResourceRef, len(result.SecretOutputs))
	}
	output := result.SecretOutputs[0]
	if output.Policy != resourcecap.SecretOutputPolicyShortLivedTicket ||
		!output.ExpiresAt.Equal(fixture.clock.Now().Add(consoleTicketLifetime)) {
		t.Fatal("console output did not use a bounded short-lived policy")
	}
	if err = result.ValidateFor(request); err != nil {
		t.Fatalf("validate console result: %v", err)
	}
	plaintext, ok := fixture.sink.load(output.Handle)
	if !ok {
		t.Fatal("console ticket was not stored by the owner sink")
	}
	defer clear(plaintext)
	var ticket map[string]any
	if json.Unmarshal(plaintext, &ticket) != nil || ticket["token"] != "ticket-secret" ||
		ticket["vnc_password"] != "console-secret" || ticket["api_key"] != nil {
		t.Fatal("console ticket projection was incomplete or retained an unapproved field")
	}
}

func TestAcceptedConsoleWithoutTerminalEvidenceRemainsUnknown(t *testing.T) {
	t.Parallel()
	var loginCount int
	var mu sync.Mutex
	login := loginHandler(t, &loginCount, &mu, "token-one")
	fixture := newProviderFixture(t, http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/administrator/v1/login" {
			login(writer, request)
			return
		}
		writeJSON(t, writer, http.StatusAccepted,
			`{"data":{"vnc_url":"wss://console.invalid/socket","token":"not-ready"}}`)
	}))
	result, err := fixture.provider.Execute(context.Background(), resourcecap.OperationRequest{
		RequestID: "request-console-accepted", Capability: resourcecap.CapabilityConsole,
		ResourceRef: "41", Parameters: []byte(`{}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != resourcecap.TaskStatusUnknown || result.Failure == nil ||
		!result.Failure.ResultUnknown || len(result.SecretOutputs) != 0 ||
		len(fixture.sink.requestSnapshot()) != 0 {
		t.Fatalf("accepted console response was treated as terminal: %+v", result)
	}
}

func TestConfidentialSinkFailureFreezesAcceptedWriteWithoutAHandle(t *testing.T) {
	t.Parallel()
	var loginCount int
	var mu sync.Mutex
	login := loginHandler(t, &loginCount, &mu, "token-one")
	fixture := newProviderFixture(t, http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/administrator/v1/login" {
			login(writer, request)
			return
		}
		writeJSON(t, writer, http.StatusOK,
			`{"data":{"vnc_url":"wss://`+request.Host+`/socket","token":"ticket-secret"}}`)
	}))
	fixture.sink.fail = true
	result, err := fixture.provider.Execute(context.Background(), resourcecap.OperationRequest{
		RequestID: "request-console-store-failure", Capability: resourcecap.CapabilityConsole,
		ResourceRef: "41", Parameters: []byte(`{}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != resourcecap.TaskStatusUnknown || result.Failure == nil ||
		!result.Failure.ResultUnknown || len(result.SecretOutputs) != 0 {
		t.Fatalf("sink failure did not freeze the accepted write: status=%s outputs=%d",
			result.Status, len(result.SecretOutputs))
	}
}

func TestTaskStatusMatrixAndResourceProjection(t *testing.T) {
	t.Parallel()
	var loginCount int
	var mu sync.Mutex
	login := loginHandler(t, &loginCount, &mu, "token-one")
	fixture := newProviderFixture(t, http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/administrator/v1/login" {
			login(writer, request)
			return
		}
		if request.URL.Path == "/administrator/v1/clouds/41" {
			writeJSON(t, writer, http.StatusOK, `{"data":{"status":"on","ip_num":2,"password":"must-not-leak","rootpassword":"must-not-leak","vnc_token":"must-not-leak"}}`)
			return
		}
		status := request.URL.Path[len(request.URL.Path)-1:]
		writeJSON(t, writer, http.StatusOK, `{"id":`+status+`,"hostid":41,"status":`+status+`,"progress":73,"msg":"apiKey=must-not-leak"}`)
	}))
	want := map[string]resourcecap.TaskStatus{
		"1": resourcecap.TaskStatusRunning,
		"2": resourcecap.TaskStatusSucceeded,
		"3": resourcecap.TaskStatusFailed,
		"4": resourcecap.TaskStatusTerminated,
		"5": resourcecap.TaskStatusCanceled,
		"6": resourcecap.TaskStatusUnknown,
	}
	for taskReference, wantStatus := range want {
		result, err := fixture.provider.QueryTask(context.Background(), resourcecap.TaskQuery{
			ExternalTaskRef: taskReference, ResourceRef: "41",
		})
		if err != nil {
			t.Errorf("query task %s: %v", taskReference, err)
			continue
		}
		if result.Status != wantStatus {
			t.Errorf("task %s status=%s, want %s", taskReference, result.Status, wantStatus)
		}
		if taskReference != "6" && (result.Progress == nil || *result.Progress != 73) {
			t.Errorf("task %s progress=%v, want 73", taskReference, result.Progress)
		}
		if taskReference == "6" && result.Progress != nil {
			t.Errorf("unknown task %s retained untrusted progress=%v", taskReference, result.Progress)
		}
		if err := result.Validate(); err != nil {
			t.Errorf("task %s result validation: %v", taskReference, err)
		}
	}
	snapshot, err := fixture.provider.QueryResource(context.Background(), resourcecap.ResourceQuery{
		ExternalResourceRef: "41",
	})
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.State != resourcecap.ResourceStateActive || !bytesContains(snapshot.Data, []byte(`"ip_count":2`)) ||
		bytesContains(snapshot.Data, []byte("must-not-leak")) || bytesContains(snapshot.Data, []byte("password")) {
		t.Fatalf("unsafe resource projection: state=%s data=%s", snapshot.State, snapshot.Data)
	}
}

func TestTaskObservationSequenceKeepsOutOfOrderAndDuplicateFactsExplicit(t *testing.T) {
	t.Parallel()
	var loginCount int
	var mu sync.Mutex
	login := loginHandler(t, &loginCount, &mu, "token-one")
	statuses := []int{1, 0, 2, 2}
	var queryCount atomic.Int64
	fixture := newProviderFixture(t, http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/administrator/v1/login" {
			login(writer, request)
			return
		}
		if request.Method != http.MethodGet || request.URL.Path != "/administrator/v1/tasks/88" {
			t.Errorf("task observation request=%s %s", request.Method, request.URL.RequestURI())
			writeJSON(t, writer, http.StatusNotFound, `{}`)
			return
		}
		index := int(queryCount.Add(1)) - 1
		if index >= len(statuses) {
			t.Errorf("unexpected task observation %d", index+1)
			writeJSON(t, writer, http.StatusInternalServerError, `{}`)
			return
		}
		writeJSON(t, writer, http.StatusOK, `{"id":88,"hostid":41,"status":`+strconv.Itoa(statuses[index])+`}`)
	}))
	want := []resourcecap.TaskStatus{
		resourcecap.TaskStatusRunning,
		resourcecap.TaskStatusAccepted,
		resourcecap.TaskStatusSucceeded,
		resourcecap.TaskStatusSucceeded,
	}
	for index := range want {
		result, err := fixture.provider.QueryTask(context.Background(), resourcecap.TaskQuery{
			ExternalTaskRef: "88",
			ResourceRef:     "41",
		})
		if err != nil {
			t.Fatalf("task observation %d failed: %v", index+1, err)
		}
		if result.Status != want[index] {
			t.Fatalf("task observation %d status=%s, want %s", index+1, result.Status, want[index])
		}
	}
	if queryCount.Load() != int64(len(statuses)) {
		t.Fatalf("task observation count=%d, want %d", queryCount.Load(), len(statuses))
	}
}

func TestTaskObservationRejectsMismatchedTaskOrResourceIdentity(t *testing.T) {
	t.Parallel()
	for name, body := range map[string]string{
		"task":     `{"id":999,"hostid":41,"status":2}`,
		"resource": `{"id":7,"hostid":999,"status":2}`,
	} {
		name, body := name, body
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			var loginCount int
			var mu sync.Mutex
			login := loginHandler(t, &loginCount, &mu, "token-one")
			fixture := newProviderFixture(t, http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				if request.URL.Path == "/administrator/v1/login" {
					login(writer, request)
					return
				}
				writeJSON(t, writer, http.StatusOK, body)
			}))
			_, err := fixture.provider.QueryTask(context.Background(), resourcecap.TaskQuery{
				ExternalTaskRef: "7", ResourceRef: "41",
			})
			if !errors.Is(err, errInvalidResponse) {
				t.Fatalf("mismatched %s identity error=%v, want invalid response", name, err)
			}
		})
	}
}

func bytesContains(haystack []byte, needle []byte) bool {
	for index := 0; index+len(needle) <= len(haystack); index++ {
		match := true
		for offset := range needle {
			if haystack[index+offset] != needle[offset] {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}
