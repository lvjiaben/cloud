// These regressions execute the real managed cloud bridge through the real
// resource confidential handler and a local TLS supplier. No remote account or
// supplier write is used; even action-shaped requests target only this fixture.
package composition

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/gogf/gf/v2/errors/gerror"
	"lina-core/pkg/plugin/capability/capmodel"
	"lina-core/pkg/plugin/pluginbridge/protocol"
	"lina-core/pkg/plugin/pluginhost"
	"lina-plugin-idc-core/backend/cap/secretcap"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
	"lina-plugin-idc-resource/backend/internal/service/channelconfig"
	cloudguest "lina-plugin-idc-zjmfcloud/backend"
)

type cloudReadWireExecutor struct {
	ctx     context.Context
	handler pluginhost.ConfidentialOutboundHandler
}

func (e cloudReadWireExecutor) Execute(_ string, sequence uint8, payload json.RawMessage) (json.RawMessage, error) {
	result, err := e.handler(e.ctx, &pluginhost.ConfidentialOutboundRequest{Sequence: sequence, Payload: payload})
	if err != nil {
		return nil, err
	}
	return result.Payload, nil
}

type cloudReadWireRegistrar struct{ dynamicRegistrarFake }

func (r *cloudReadWireRegistrar) InvokeProviderMethod(ctx context.Context, input pluginhost.BusinessExtensionMethodInvokeInput) (*capmodel.BusinessExtensionMethodResult, error) {
	invocation := capmodel.BusinessExtensionBridgeRequest{Version: capmodel.BusinessExtensionInvocationProtocolV1, OwnerPluginID: resourcecap.OwnerPluginID, ProviderPluginID: "idc-zjmfcloud", Point: resourcecap.DynamicProviderPoint, ContractVersion: resourcecap.DynamicProviderContractVersion, Method: input.Method, ReleaseVersion: "v0.3.19", ReleaseDigest: strings.Repeat("a", 64), DeclarationDigest: strings.Repeat("b", 64), Purpose: capmodel.BusinessExtensionInvocationPurposeExistingBusiness, RuntimeContext: input.RuntimeContext, Payload: input.Payload}
	body, err := json.Marshal(invocation)
	if err != nil {
		return nil, err
	}
	response, err := cloudguest.NewBusinessExtensionHandler(cloudReadWireExecutor{ctx, r.registration.Handler})(&protocol.BridgeRequestEnvelopeV1{PluginID: "idc-zjmfcloud", Route: &protocol.RouteMatchSnapshotV1{RequestType: capmodel.BusinessExtensionBridgeRequestType}, Request: &protocol.HTTPRequestSnapshotV1{Body: body}})
	if err != nil {
		return nil, err
	}
	if response == nil || response.StatusCode != http.StatusOK {
		return nil, gerror.New("managed cloud fixture failed")
	}
	var result capmodel.BusinessExtensionBridgeResponse
	if err = json.Unmarshal(response.Body, &result); err != nil {
		return nil, err
	}
	return &capmodel.BusinessExtensionMethodResult{Payload: result.Payload}, nil
}

func cloudReadHandler(t *testing.T, endpoint string, client *http.Client) *confidentialHTTPHandler {
	t.Helper()
	username, err := secretcap.NewHandle("secret_cloud_read_user")
	if err != nil {
		t.Fatal(err)
	}
	password, err := secretcap.NewHandle("secret_cloud_read_password")
	if err != nil {
		t.Fatal(err)
	}
	return &confidentialHTTPHandler{configuration: channelconfig.Configuration{PluginID: "idc-zjmfcloud", Config: resourcecap.ProviderConfig{Endpoint: endpoint, Settings: diagnosticLoginSettings(), SecretBindings: []resourcecap.SecretBinding{{Name: "username", Handle: username}, {Name: "password", Handle: password}}}}, providerMethod: resourcecap.DynamicProviderMethodQueryResource, environment: resourcecap.ProviderEnv{HTTPClient: confidentialHTTPClient{client}, Secrets: confidentialHTTPAccessor{"secret_cloud_read_user": []byte("merchant-login"), "secret_cloud_read_password": []byte("merchant-password")}, AuthenticationSessions: &confidentialHTTPSessions{}}}
}

func TestCloudManagedResourceQueryPreservesReviewedFactsOverLocalTLS(t *testing.T) {
	var reads, logins int
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/login":
			logins++
			if _, err := io.WriteString(w, `"private-auth-token"`); err != nil {
				t.Error(err)
			}
		case "/clouds/674":
			reads++
			if r.Method != http.MethodGet || r.Header.Get("Access-Token") != "private-auth-token" {
				t.Error("resource read lost method/authentication")
			}
			if _, err := io.WriteString(w, `{"data":{"id":674,"status":"on","hostname":"guest-674","os_name":"Debian 12","osuser":"root","port":22,"cpu":2,"memory":2048,"ip":[{"id":1,"ipaddress":"203.0.113.14"}],"username":"private-merchant","password":"private-server-password","token":"private-auth-token","path":"/private/supplier/config","private_key":"private-key"}}`); err != nil {
				t.Error(err)
			}
		default:
			t.Errorf("unexpected fixture path %s", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)
	handler := cloudReadHandler(t, server.URL, server.Client())
	registrar := &cloudReadWireRegistrar{}
	provider := &dynamicProvider{extensions: registrar, configuration: handler.configuration, outbound: func(_ context.Context, _ channelconfig.Configuration, method string, _ []resourcecap.SecretBinding, _ ...func(resourcecap.Evidence)) (pluginhost.ConfidentialOutboundHandler, func(), error) {
		handler.providerMethod = method
		return handler.Execute, func() {}, nil
	}}
	started := time.Now().UTC()
	result, err := provider.QueryResource(t.Context(), resourcecap.ResourceQuery{ExternalResourceRef: "674"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Validate() != nil || result.State != resourcecap.ResourceStateActive || result.ObservedAt.Before(started) || reads != 1 || logins != 1 {
		t.Fatalf("original confirmed observation did not recover: %+v reads=%d logins=%d", result, reads, logins)
	}
	var facts struct {
		Hostname      string   `json:"hostname"`
		LoginUsername string   `json:"login_username"`
		Addresses     []string `json:"addresses"`
	}
	if json.Unmarshal(result.Data, &facts) != nil || facts.Hostname != "guest-674" || facts.LoginUsername != "root" || len(facts.Addresses) != 1 || facts.Addresses[0] != "203.0.113.14" {
		t.Fatalf("normalized facts lost: %s", result.Data)
	}
	if !slices.Contains(registrar.registration.AllowedResultFields, "Data") {
		t.Fatal("real result policy rejected Data")
	}
	for _, private := range []string{"private-merchant", "private-server-password", "private-auth-token", "/private/supplier/config", "private-key"} {
		if strings.Contains(string(result.Data), private) {
			t.Fatalf("private source material escaped: %s", private)
		}
	}
}

func TestCloudResourceGuestLabelsRemainScopedBeforeAnyHTTP(t *testing.T) {
	for _, scenario := range []string{"other plugin", "other method", "write method", "action path", "collection", "query", "headers", "form", "password", "token", "private path"} {
		t.Run(scenario, func(t *testing.T) {
			var calls int
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls++; t.Error("rejected read reached supplier") }))
			t.Cleanup(server.Close)
			handler := cloudReadHandler(t, server.URL, server.Client())
			request := resourcecap.ConfidentialHTTPRequest{Kind: resourcecap.ConfidentialHTTPKindRequest, Method: http.MethodGet, Path: "/clouds/674", UseAuthentication: true, ResponseFields: []string{"data", "id", "status", "hostname", "osuser"}}
			switch scenario {
			case "other plugin":
				handler.configuration.PluginID = "idc-v10"
			case "other method":
				handler.providerMethod = resourcecap.DynamicProviderMethodExecute
			case "write method":
				request.Method = http.MethodPut
			case "action path":
				request.Path += "/password"
			case "collection":
				request.Path = "/clouds"
			case "query":
				request.Query = map[string][]string{"id": {"675"}}
			case "headers":
				request.Headers = map[string]string{"X-Resource": "675"}
			case "form":
				request.Form = map[string]string{"id": "675"}
			case "password":
				request.ResponseFields = append(request.ResponseFields, "password")
			case "token":
				request.ResponseFields = append(request.ResponseFields, "token")
			case "private path":
				request.ResponseFields = append(request.ResponseFields, "path")
			}
			raw, err := json.Marshal(request)
			if err != nil {
				t.Fatal(err)
			}
			if _, err = handler.Execute(t.Context(), &pluginhost.ConfidentialOutboundRequest{Sequence: 2, Payload: raw}); err == nil || calls != 0 {
				t.Fatalf("out-of-scope request admitted %s calls=%d", scenario, calls)
			}
		})
	}
}

// Action preconditions need identity/status only. They must not borrow the
// resource.query-only guest label exception when checking the same instance.
func TestCloudManagedPowerPrecheckUsesOrdinaryFieldsOverLocalTLS(t *testing.T) {
	var reads, writes int
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		var body string
		switch r.URL.Path {
		case "/login":
			body = `"private-auth-token"`
		case "/clouds/674":
			reads++
			if r.Method != http.MethodGet {
				t.Error("precheck was not read-only")
			}
			body = `{"data":{"id":674,"status":"on"}}`
		case "/clouds/674/off":
			writes++
			if r.Method != http.MethodPost {
				t.Error("wrong local fixture power verb")
			}
			body = `{"data":{"taskid":71}}`
		default:
			t.Errorf("unexpected local fixture route %s", r.URL.Path)
			http.NotFound(w, r)
			return
		}
		if _, err := io.WriteString(w, body); err != nil {
			t.Error(err)
		}
	}))
	t.Cleanup(server.Close)
	handler := cloudReadHandler(t, server.URL, server.Client())
	registrar := &cloudReadWireRegistrar{}
	provider := &dynamicProvider{extensions: registrar, configuration: handler.configuration, outbound: func(_ context.Context, _ channelconfig.Configuration, method string, _ []resourcecap.SecretBinding, _ ...func(resourcecap.Evidence)) (pluginhost.ConfidentialOutboundHandler, func(), error) {
		handler.providerMethod = method
		return handler.Execute, func() {}, nil
	}}
	result, err := provider.Execute(t.Context(), resourcecap.OperationRequest{RequestID: "local-power-regression", Capability: resourcecap.CapabilityStop, ResourceRef: "674", Parameters: json.RawMessage(`{}`)})
	if err != nil || result == nil || result.Status != resourcecap.TaskStatusAccepted || result.ExternalTaskRef != "71" || reads != 1 || writes != 1 {
		t.Fatalf("ordinary power path was blocked: %+v %v reads=%d writes=%d", result, err, reads, writes)
	}
}
