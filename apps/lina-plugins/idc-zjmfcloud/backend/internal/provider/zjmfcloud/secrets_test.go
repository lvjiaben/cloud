// This file verifies confidential form encoding and fail-closed output projection.

package zjmfcloud

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"

	"lina-plugin-idc-core/backend/cap/secretcap"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
)

func TestConsoleConnectionRequiresTrustedHTTPSOrWSSHost(t *testing.T) {
	t.Parallel()
	fixture := newProviderFixture(t, http.NotFoundHandler())
	fixture.provider.settings.ConsoleHosts = map[string]struct{}{"console.example.com": {}}
	valid := []string{
		"wss://console.example.com/socket?ticket=opaque",
		"https://" + fixture.provider.endpoint.Host + "/console",
	}
	invalid := []string{
		"javascript:alert(1)",
		"file:///etc/passwd",
		"http://console.example.com/socket",
		"wss://untrusted.example.com/socket",
		"wss://user:password@console.example.com/socket",
		"wss://console.example.com/socket#fragment",
		"wss://console.example.com:0/socket",
	}
	for _, value := range valid {
		raw, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		if !fixture.provider.validConsoleConnection(raw) {
			t.Errorf("trusted console URL %q was rejected", value)
		}
	}
	for _, value := range invalid {
		raw, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		if fixture.provider.validConsoleConnection(raw) {
			t.Errorf("unsafe console URL %q was accepted", value)
		}
	}
}

func TestConfidentialFormEncodingPreservesUTF8WithoutStringConversion(t *testing.T) {
	t.Parallel()
	body, err := encodeConfidentialForm(confidentialFormPair{
		name: "password", value: []byte("密 码+&"),
	})
	if err != nil {
		t.Fatal(err)
	}
	defer clear(body)
	if string(body) != "password=%E5%AF%86+%E7%A0%81%2B%26" {
		t.Fatalf("unexpected confidential form encoding: %s", body)
	}
	for _, value := range [][]byte{nil, []byte("line\nbreak"), {0xff, 0xfe}} {
		if encoded, encodeErr := encodeConfidentialForm(confidentialFormPair{
			name: "password", value: value,
		}); encodeErr == nil {
			clear(encoded)
			t.Errorf("invalid confidential value %x was accepted", value)
		}
	}
}

func TestResetPasswordRejectsMissingOrUnavailableSecretBeforeNetwork(t *testing.T) {
	t.Parallel()
	var requests atomic.Int64
	fixture := newProviderFixture(t, http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		writeJSON(t, writer, http.StatusInternalServerError, `{}`)
	}))
	missing, err := secretcap.NewHandle("sec_zjmf_missing_operation_password")
	if err != nil {
		t.Fatal(err)
	}
	tests := []resourcecap.OperationRequest{
		{
			RequestID: "password-binding-missing", Capability: resourcecap.CapabilityResetPassword,
			ResourceRef: "41", Parameters: []byte(`{}`),
		},
		{
			RequestID: "password-binding-wrong-name", Capability: resourcecap.CapabilityResetPassword,
			ResourceRef: "41", Parameters: []byte(`{}`),
			SecretBindings: []resourcecap.SecretBinding{{Name: "other", Handle: missing}},
		},
	}
	for _, request := range tests {
		if _, executeErr := fixture.provider.Execute(context.Background(), request); executeErr == nil {
			t.Errorf("request %s accepted an invalid password binding", request.RequestID)
		}
	}

	unavailable := resourcecap.OperationRequest{
		RequestID: "password-secret-unavailable", Capability: resourcecap.CapabilityResetPassword,
		ResourceRef: "41", Parameters: []byte(`{}`),
		SecretBindings: []resourcecap.SecretBinding{{Name: resetPasswordBinding, Handle: missing}},
	}
	_, err = fixture.provider.Execute(context.Background(), unavailable)
	var callError *resourcecap.ProviderCallError
	if !errors.As(err, &callError) || callError.Code != resourcecap.ProviderCallErrorCodeInternal ||
		callError.Retryable || callError.ResultUnknown {
		t.Fatalf("unavailable operation secret error=%v, want safe internal classification", err)
	}
	if requests.Load() != 0 {
		t.Fatalf("invalid password inputs made %d network requests", requests.Load())
	}
}

func TestProvisionRejectsMissingPasswordBeforeNetwork(t *testing.T) {
	t.Parallel()
	var requests atomic.Int64
	fixture := newProviderFixture(t, http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		writeJSON(t, writer, http.StatusInternalServerError, `{}`)
	}))
	_, err := fixture.provider.Execute(context.Background(), resourcecap.OperationRequest{
		RequestID: "provision-password-missing", Capability: resourcecap.CapabilityProvision,
		Parameters: []byte(`{"cpu":2,"memory":2048,"os":5,"client":91,"hostname":"srv-001.example"}`),
	})
	if err == nil {
		t.Fatal("provision accepted a missing password binding")
	}
	if requests.Load() != 0 {
		t.Fatalf("missing provision password made %d network requests", requests.Load())
	}
}

func TestReinstallMissingGeneratedPasswordFreezesWithoutSecretHandle(t *testing.T) {
	t.Parallel()
	var loginCount int
	var mu sync.Mutex
	login := loginHandler(t, &loginCount, &mu, "token-one")
	fixture := newProviderFixture(t, http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/administrator/v1/login" {
			login(writer, request)
			return
		}
		writeJSON(t, writer, http.StatusAccepted, `{"data":{"taskid":7201,"user":"root"}}`)
	}))
	request := resourcecap.OperationRequest{
		RequestID: "request-reinstall-incomplete", Capability: resourcecap.CapabilityReinstall,
		ResourceRef: "41", Parameters: []byte(`{"os":9}`),
	}
	result, err := fixture.provider.Execute(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != resourcecap.TaskStatusUnknown || result.Failure == nil ||
		!result.Failure.ResultUnknown || len(result.SecretOutputs) != 0 ||
		len(fixture.sink.requestSnapshot()) != 0 {
		t.Fatalf("incomplete reinstall response did not fail closed: %+v", result)
	}
}

func TestConsoleTokenWithoutConnectionAddressFreezesWithoutSecretHandle(t *testing.T) {
	t.Parallel()
	var loginCount int
	var mu sync.Mutex
	login := loginHandler(t, &loginCount, &mu, "token-one")
	fixture := newProviderFixture(t, http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/administrator/v1/login" {
			login(writer, request)
			return
		}
		writeJSON(t, writer, http.StatusOK, `{"data":{"token":"orphan-ticket","vnc_pass":"secret"}}`)
	}))
	request := resourcecap.OperationRequest{
		RequestID: "request-console-incomplete", Capability: resourcecap.CapabilityConsole,
		ResourceRef: "41", Parameters: []byte(`{}`),
	}
	result, err := fixture.provider.Execute(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != resourcecap.TaskStatusUnknown || result.Failure == nil ||
		!result.Failure.ResultUnknown || len(result.SecretOutputs) != 0 ||
		len(fixture.sink.requestSnapshot()) != 0 {
		t.Fatalf("incomplete console response did not fail closed: %+v", result)
	}
}

func TestConsoleFromUntrustedHostFreezesWithoutSecretHandle(t *testing.T) {
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
			`{"data":{"vnc_url":"wss://untrusted.example.com/socket","token":"orphan-ticket"}}`)
	}))
	result, err := fixture.provider.Execute(context.Background(), resourcecap.OperationRequest{
		RequestID: "request-console-untrusted-host", Capability: resourcecap.CapabilityConsole,
		ResourceRef: "41", Parameters: []byte(`{}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != resourcecap.TaskStatusUnknown || result.Failure == nil ||
		!result.Failure.ResultUnknown || len(result.SecretOutputs) != 0 ||
		len(fixture.sink.requestSnapshot()) != 0 {
		t.Fatalf("untrusted console host did not fail closed: %+v", result)
	}
}
