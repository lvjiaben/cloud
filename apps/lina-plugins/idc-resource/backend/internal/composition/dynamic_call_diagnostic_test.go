// These tests verify failure coordinates survive without a supplier body and
// that real login responses never enter retained operation diagnostics.
package composition

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/gogf/gf/v2/errors/gcode"
	"io"
	"lina-core/pkg/bizerr"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"lina-core/pkg/plugin/pluginhost"
	"lina-plugin-idc-core/backend/cap/secretcap"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
	"lina-plugin-idc-resource/backend/internal/service/channelconfig"
)

func TestCallDiagnosticRetainsReviewedAdmissionCodeWithoutRawCause(t *testing.T) {
	failure := bizerr.NewCode(bizerr.MustDefine("PLUGIN_LIFECYCLE_ALREADY_RUNNING", "private runtime explanation", gcode.CodeOperationFailed))
	evidence := newCallDiagnostic(callDiagnosticDispatch, failure, nil, nil, nil)
	if evidence.Validate() != nil || !strings.Contains(evidence.Summary, "PLUGIN_LIFECYCLE_ALREADY_RUNNING") || strings.Contains(evidence.Summary, "private runtime") || strings.Contains(evidence.Summary, "businessRequestStarted") {
		t.Fatal("admission failure lost its safe category or invented submission evidence")
	}
}

type diagnosticFailedHTTPClient struct{ calls int }

func (c *diagnosticFailedHTTPClient) Do(*http.Request) (*http.Response, error) {
	c.calls++
	return nil, errors.New("dial https://private-user:private-value@example.test: connection refused")
}

func TestCallDiagnosticRetainsNoResponseWithoutRawTransportError(t *testing.T) {
	client := &diagnosticFailedHTTPClient{}
	var evidence []resourcecap.Evidence
	h := &confidentialHTTPHandler{
		providerMethod: resourcecap.DynamicProviderMethodExecute,
		login:          &resourcecap.ConfidentialHTTPRequest{Kind: resourcecap.ConfidentialHTTPKindAuthenticate, Method: http.MethodPost, Path: "/login"},
		configuration:  channelconfig.Configuration{Config: resourcecap.ProviderConfig{Endpoint: "https://example.test", Settings: diagnosticLoginSettings()}},
		environment:    resourcecap.ProviderEnv{HTTPClient: client, AuthenticationSessions: &confidentialHTTPSessions{token: []byte("private-session")}},
		observeResponse: func(item resourcecap.Evidence) {
			if item.Kind != businessHTTPAttemptObserved {
				evidence = append(evidence, item)
			}
		},
	}
	payload := json.RawMessage(`{"kind":"request","method":"POST","path":"/clouds","responseFields":["data","id"],"useAuthentication":true}`)
	if _, err := h.Execute(t.Context(), &pluginhost.ConfidentialOutboundRequest{Sequence: 2, Payload: payload}); err == nil {
		t.Fatal("transport unexpectedly succeeded")
	}
	if client.calls != 1 || len(evidence) != 1 {
		t.Fatal("transport retried or failed to record diagnostic")
	}
	assertCallDiagnostic(t, evidence[0], callDiagnosticTransport, true, false)
	if strings.Contains(evidence[0].Summary, "private-") || strings.Contains(evidence[0].Summary, "example.test") {
		t.Fatal("raw transport error or session leaked")
	}
}

func TestCallDiagnosticCapturesLocalValidationWithoutAuthentication(t *testing.T) {
	var evidence []resourcecap.Evidence
	h := &confidentialHTTPHandler{providerMethod: resourcecap.DynamicProviderMethodExecute, observeResponse: func(item resourcecap.Evidence) {
		if item.Kind != businessHTTPAttemptObserved {
			evidence = append(evidence, item)
		}
	}}
	if _, err := h.Execute(t.Context(), &pluginhost.ConfidentialOutboundRequest{Sequence: 2, Payload: []byte(`{"kind":"request","method":"POST","path":"/clouds","formFields":[{"field":"rootpass","value":"private-value"}],"responseFields":["data"],"useAuthentication":true}`)}); err == nil {
		t.Fatal("invalid public credentials accepted")
	}
	if len(evidence) != 1 {
		t.Fatal("local failure has no diagnostic")
	}
	assertCallDiagnostic(t, evidence[0], callDiagnosticPreparation, false, false)
	if strings.Contains(evidence[0].Summary, "private-value") {
		t.Fatal("public credential retained")
	}
}

func TestCallDiagnosticActualLoginNeverRetainsAuthenticationBody(t *testing.T) {
	for _, rejected := range []bool{false, true} {
		t.Run(map[bool]string{false: "accepted", true: "rejected"}[rejected], func(t *testing.T) {
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				calls++
				body := `"private-session-token"`
				if rejected {
					body = `{"status":400,"msg":"private-login-response"}`
				}
				if _, err := io.WriteString(w, body); err != nil {
					t.Error(err)
				}
			}))
			t.Cleanup(server.Close)
			var evidence []resourcecap.Evidence
			username, err := secretcap.NewHandle("secret_diag_username")
			if err != nil {
				t.Fatal(err)
			}
			password, err := secretcap.NewHandle("secret_diag_password")
			if err != nil {
				t.Fatal(err)
			}
			h := &confidentialHTTPHandler{
				providerMethod: resourcecap.DynamicProviderMethodExecute,
				configuration:  channelconfig.Configuration{Config: resourcecap.ProviderConfig{Endpoint: server.URL, Settings: diagnosticLoginSettings(), SecretBindings: []resourcecap.SecretBinding{{Name: "username", Handle: username}, {Name: "password", Handle: password}}}},
				environment:    resourcecap.ProviderEnv{HTTPClient: confidentialHTTPClient{client: server.Client()}, Secrets: confidentialHTTPAccessor{"secret_diag_username": []byte("private-user"), "secret_diag_password": []byte("private-password")}, AuthenticationSessions: &confidentialHTTPSessions{}},
				observeResponse: func(item resourcecap.Evidence) {
					if item.Kind != businessHTTPAttemptObserved {
						evidence = append(evidence, item)
					}
				},
			}
			_, callErr := h.Execute(t.Context(), &pluginhost.ConfidentialOutboundRequest{Sequence: 1, Payload: []byte(`{"kind":"authenticate","method":"POST","path":"/login"}`)})
			if (callErr != nil) != rejected || calls != 1 {
				t.Fatal("login result changed or duplicated")
			}
			if !rejected && len(evidence) != 0 {
				t.Fatal("successful login response was retained")
			}
			if rejected {
				if len(evidence) != 1 {
					t.Fatal("authentication failure was lost")
				}
				assertCallDiagnostic(t, evidence[0], callDiagnosticAuthentication, false, false)
				if strings.Contains(evidence[0].Summary, "private-") {
					t.Fatal("login body retained")
				}
			}
		})
	}
}

func diagnosticLoginSettings() json.RawMessage {
	return json.RawMessage(`{"__host":{"policy":{"mode":"login_session","mappings":[{"role":"password","slot":"password"},{"role":"username","slot":"username"}],"path":"/login","readPath":["$"],"header":"Access-Token","ttlSeconds":3600}}}`)
}

func assertCallDiagnostic(t *testing.T, evidence resourcecap.Evidence, stage callDiagnosticStage, started, responded bool) {
	t.Helper()
	var value struct {
		Stage     callDiagnosticStage `json:"stage"`
		Started   *bool               `json:"businessRequestStarted"`
		Responded *bool               `json:"responseReceived"`
		Code      string              `json:"errorCode"`
	}
	if evidence.Kind != resourcecap.EvidenceKindCallDiagnostic || evidence.Validate() != nil || json.Unmarshal([]byte(evidence.Summary), &value) != nil || value.Stage != stage || value.Started == nil || *value.Started != started || value.Responded == nil || *value.Responded != responded || value.Code == "" {
		t.Fatalf("missing or invalid call diagnostic: %+v", evidence)
	}
}

func TestCallDiagnosticRetainsHostHandoffFailure(t *testing.T) {
	proof, err := json.Marshal(resourcecap.OperationResult{Status: resourcecap.TaskStatusUnknown, Failure: &resourcecap.ProviderFailure{Code: "upstream_result_unknown", Message: "unconfirmed", ResultUnknown: true}})
	if err != nil {
		t.Fatal(err)
	}
	registrar := &verificationClockRegistrar{proof: proof}
	p := &dynamicProvider{extensions: registrar, configuration: channelconfig.Configuration{Config: resourcecap.ProviderConfig{Settings: []byte(`{}`)}}, outbound: func(context.Context, channelconfig.Configuration, string, []resourcecap.SecretBinding, ...func(resourcecap.Evidence)) (pluginhost.ConfidentialOutboundHandler, func(), error) {
		return func(context.Context, *pluginhost.ConfidentialOutboundRequest) (*pluginhost.ConfidentialOutboundResult, error) {
			t.Fatal("business handoff must not occur")
			return nil, nil
		}, func() {}, nil
	}}
	var result resourcecap.OperationResult
	if err = p.invoke(t.Context(), resourcecap.DynamicProviderMethodExecute, struct{}{}, nil, &result); err != nil || result.Status != resourcecap.TaskStatusUnknown || result.Failure == nil || !result.Failure.ResultUnknown || len(result.Evidence) != 1 || !strings.Contains(result.Evidence[0].Summary, string(callDiagnosticDispatch)) {
		t.Fatalf("handoff failure disappeared or changed acceptance: %+v %v", result, err)
	}
	if strings.Contains(result.Evidence[0].Summary, "businessRequestStarted") {
		t.Fatal("missing handoff observations were falsely asserted as HTTP facts")
	}
}

// Read-only failures may emit safe coordinates, but never become provisioning
// evidence or cause an extra supplier request.
func TestReadonlyHTTPFailureDoesNotBecomeProvisioningEvidence(t *testing.T) {
	client := &diagnosticFailedHTTPClient{}
	observations := 0
	h := &confidentialHTTPHandler{
		providerMethod:  resourcecap.DynamicProviderMethodQueryResource,
		login:           &resourcecap.ConfidentialHTTPRequest{Kind: resourcecap.ConfidentialHTTPKindAuthenticate, Method: http.MethodPost, Path: "/login"},
		configuration:   channelconfig.Configuration{Config: resourcecap.ProviderConfig{Endpoint: "https://example.test", Settings: diagnosticLoginSettings()}},
		environment:     resourcecap.ProviderEnv{HTTPClient: client, AuthenticationSessions: &confidentialHTTPSessions{token: []byte("private-session")}},
		observeResponse: func(resourcecap.Evidence) { observations++ },
	}
	_, err := h.Execute(t.Context(), &pluginhost.ConfidentialOutboundRequest{Sequence: 2, Payload: []byte(`{"kind":"request","method":"GET","path":"/clouds/41","responseFields":["data"],"useAuthentication":true}`)})
	if err == nil || !bizerr.Is(err, resourcecap.CodeProviderNetwork) || client.calls != 1 || observations != 0 {
		t.Fatal("read failure was repeated, lost its safe error or created operation evidence")
	}
}
