// These tests reproduce response loss before guest projection and ensure that
// retained operator diagnostics cannot contain secrets or trigger another write.
package composition

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"lina-core/pkg/plugin/pluginhost"
	"lina-plugin-idc-core/backend/cap/secretcap"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
	"lina-plugin-idc-resource/backend/internal/service/channelconfig"
)

func TestHTTPDiagnosticSurvivesProjectionAndMasksSecrets(t *testing.T) {
	for _, status := range []int{200, 400, 422, 500} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				calls++
				w.WriteHeader(status)
				if _, err := io.WriteString(w, `{"status":400,"msg":"镜像在当前节点未下载","password":"private-root","nested":{"access_token":"private-session"},"echo":"private-root","configurationEcho":"private-session","escapedEcho":"private-\u0072oot","counter":1234567890123456789}`); err != nil {
					t.Error(err)
				}
			}))
			t.Cleanup(server.Close)
			var observations []resourcecap.Evidence
			h := &confidentialHTTPHandler{providerMethod: resourcecap.DynamicProviderMethodExecute, configuration: channelconfig.Configuration{PluginID: "idc-zjmfcloud", Config: resourcecap.ProviderConfig{Endpoint: server.URL, Settings: json.RawMessage(`{}`)}}, environment: resourcecap.ProviderEnv{HTTPClient: confidentialHTTPClient{client: server.Client()}}, observeResponse: func(e resourcecap.Evidence) {
				if e.Kind != businessHTTPAttemptObserved {
					observations = append(observations, e)
				}
			}}
			handle, handleErr := secretcap.NewHandle("secret_response_diagnostic")
			if handleErr != nil {
				t.Fatal(handleErr)
			}
			h.configuration.Config.SecretBindings = []resourcecap.SecretBinding{{Name: "password", Handle: handle}}
			h.environment.Secrets = confidentialHTTPAccessor{"secret_response_diagnostic": []byte("private-session")}
			_, err := h.perform(context.Background(), resourcecap.ConfidentialHTTPRequest{Kind: resourcecap.ConfidentialHTTPKindRequest, Method: http.MethodPost, Path: "/clouds", Form: map[string]string{"rootpass": "private-root"}, ResponseFields: []string{"status", "data"}}, nil)
			if err != nil || calls != 1 || len(observations) != 1 {
				t.Fatalf("response observation lost: calls=%d evidence=%d error=%v", calls, len(observations), err)
			}
			summary := observations[0].Summary
			if !strings.Contains(summary, "镜像在当前节点未下载") || !strings.Contains(summary, `"httpStatus":`) || strings.Contains(summary, "private-root") || strings.Contains(summary, "private-session") || observations[0].Validate() != nil {
				t.Fatal("diagnostic lost message or exposed secret")
			}
			if !strings.Contains(summary, "1234567890123456789") {
				t.Fatal("numeric response precision lost")
			}
		})
	}
}

func TestHTTPDiagnosticSkipsLoginAndBoundsOversizedResponse(t *testing.T) {
	var observations []resourcecap.Evidence
	h := &confidentialHTTPHandler{providerMethod: resourcecap.DynamicProviderMethodExecute, observeResponse: func(e resourcecap.Evidence) {
		if e.Kind != businessHTTPAttemptObserved {
			observations = append(observations, e)
		}
	}}
	h.observeHTTPResponse(context.Background(), resourcecap.ConfidentialHTTPRequest{Kind: resourcecap.ConfidentialHTTPKindAuthenticate}, 200, []byte(`"private-login-token"`), nil)
	if len(observations) != 0 {
		t.Fatal("login response retained")
	}
	h.observeHTTPResponse(context.Background(), resourcecap.ConfidentialHTTPRequest{Kind: resourcecap.ConfidentialHTTPKindRequest, Method: http.MethodPost, Path: "/clouds"}, 502, []byte(strings.Repeat("unexpected response ", 10000)), nil)
	if len(observations) != 1 || len(observations[0].Summary) > 4096 || !strings.Contains(observations[0].Summary, "TRUNCATED") {
		t.Fatal("oversized response not explicitly bounded")
	}
}

// A body read error must not discard the HTTP status and the bytes already
// received, or conceal a gateway response behind a generic connection error.
func TestHTTPDiagnosticRetainsPartialResponse(t *testing.T) {
	for _, status := range []int{http.StatusOK, http.StatusBadRequest, http.StatusUnprocessableEntity} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Length", "1000")
				w.WriteHeader(status)
				if _, err := io.WriteString(w, "upstream disconnected before acknowledgement"); err != nil {
					t.Error(err)
				}
			}))
			t.Cleanup(server.Close)
			var observations []resourcecap.Evidence
			h := &confidentialHTTPHandler{providerMethod: resourcecap.DynamicProviderMethodExecute, configuration: channelconfig.Configuration{PluginID: "idc-zjmfcloud", Config: resourcecap.ProviderConfig{Endpoint: server.URL, Settings: json.RawMessage(`{}`)}}, environment: resourcecap.ProviderEnv{HTTPClient: confidentialHTTPClient{client: server.Client()}}, observeResponse: func(e resourcecap.Evidence) {
				if e.Kind != businessHTTPAttemptObserved {
					observations = append(observations, e)
				}
			}}
			result, err := h.perform(context.Background(), resourcecap.ConfidentialHTTPRequest{Kind: resourcecap.ConfidentialHTTPKindRequest, Method: http.MethodPost, Path: "/clouds", ResponseFields: []string{"status", "data"}}, nil)
			if len(observations) != 1 || !strings.Contains(observations[0].Summary, "upstream disconnected before acknowledgement") {
				t.Fatal("partial response was lost")
			}
			if status == http.StatusOK && err == nil {
				t.Fatal("partial success body was accepted")
			}
			if status != http.StatusOK && (err != nil || result == nil || result.StatusCode != status) {
				t.Fatal("known HTTP rejection was lost after partial read")
			}
		})
	}
}

func TestHTTPDiagnosticMasksNumericSecretEcho(t *testing.T) {
	result := sanitizeHTTPDiagnosticValue(json.Number("1234567890123456"), 0, []string{"1234567890123456"})
	if result != diagnosticRedaction {
		t.Fatal("numeric credential echo retained")
	}
}

func TestDynamicInvocationRetainsOwnerHTTPResponse(t *testing.T) {
	proof, err := json.Marshal(resourcecap.OperationResult{Status: resourcecap.TaskStatusUnknown, Failure: &resourcecap.ProviderFailure{Code: "upstream_result_unknown", Message: "unconfirmed response", ResultUnknown: true}})
	if err != nil {
		t.Fatal(err)
	}
	registrar := &verificationClockRegistrar{proof: proof, step: &resourcecap.ConfidentialHTTPRequest{Kind: resourcecap.ConfidentialHTTPKindRequest}}
	evidence := resourcecap.Evidence{Kind: resourcecap.EvidenceKindHTTPResponse, Summary: `{"httpStatus":200,"response":{"status":400,"msg":"node image not downloaded"}}`, ObservedAt: time.Now().UTC()}
	p := &dynamicProvider{extensions: registrar, configuration: channelconfig.Configuration{Config: resourcecap.ProviderConfig{Settings: json.RawMessage(`{}`)}}, outbound: func(_ context.Context, _ channelconfig.Configuration, _ string, _ []resourcecap.SecretBinding, observers ...func(resourcecap.Evidence)) (pluginhost.ConfidentialOutboundHandler, func(), error) {
		return func(context.Context, *pluginhost.ConfidentialOutboundRequest) (*pluginhost.ConfidentialOutboundResult, error) {
			observers[0](evidence)
			return &pluginhost.ConfidentialOutboundResult{Payload: []byte(`{"StatusCode":200}`)}, nil
		}, func() {}, nil
	}}
	var result resourcecap.OperationResult
	if err = p.invoke(context.Background(), resourcecap.DynamicProviderMethodExecute, struct{}{}, nil, &result); err != nil || len(result.Evidence) != 1 || result.Evidence[0].Summary != evidence.Summary {
		t.Fatalf("owner response did not survive guest result: %v", err)
	}
}
