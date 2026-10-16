// This file verifies that confidential HTTP execution exposes only screened projections.

package composition

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"lina-core/pkg/bizerr"
	"lina-core/pkg/plugin/pluginhost"
	"lina-plugin-idc-core/backend/cap/secretcap"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
	"lina-plugin-idc-resource/backend/internal/service/channelconfig"
)

type confidentialHTTPAccessor map[string][]byte

func (a confidentialHTTPAccessor) Access(_ context.Context, request secretcap.AccessRequest, consume secretcap.Consumer) error {
	return consume(append([]byte(nil), a[request.Handle.OpaqueRef()]...))
}

type confidentialHTTPSessions struct {
	token      []byte
	generation int64
}

func (s *confidentialHTTPSessions) Access(
	_ context.Context,
	_ resourcecap.AuthenticationSessionRequest,
	produce resourcecap.AuthenticationTokenProducer,
	consume resourcecap.AuthenticationTokenConsumer,
) error {
	if len(s.token) == 0 {
		if err := produce(func(token []byte) error {
			s.token = append([]byte(nil), token...)
			return nil
		}); err != nil {
			return err
		}
		s.generation++
	}
	if s.generation == 0 {
		s.generation = 1
	}
	return consume(append([]byte(nil), s.token...), s.generation)
}

func (s *confidentialHTTPSessions) Invalidate(_ context.Context, request resourcecap.AuthenticationSessionInvalidation) error {
	if request.Generation == s.generation {
		clear(s.token)
		s.token = nil
	}
	return nil
}

type confidentialHTTPClient struct{ client *http.Client }

func (c confidentialHTTPClient) Do(request *http.Request) (*http.Response, error) {
	return c.client.Do(request)
}

func TestConfidentialHTTPHandlerKeepsLoginTokenAndCredentialsOutOfGuest(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/administrator/v1/login":
			content, _ := io.ReadAll(request.Body)
			if string(content) != "password=pass-value&username=user-value" {
				t.Fatalf("unexpected login body %q", content)
			}
			_, _ = response.Write([]byte(`"host-token"`))
		case "/administrator/v1/clouds/41":
			if request.Header.Get("Access-Token") != "host-token" {
				t.Fatal("host did not inject the session token")
			}
			if request.Method == http.MethodPut {
				content, _ := io.ReadAll(request.Body)
				if string(content) != "password=operation-value" {
					t.Fatalf("unexpected operation body %q", content)
				}
				_, _ = response.Write([]byte(`{"data":{"taskid":71,"password":"supplier-leak"}}`))
				return
			}
			_, _ = response.Write([]byte(`{"data":{"id":41,"status":"on","password":"supplier-leak"}}`))
		default:
			http.NotFound(response, request)
		}
	}))
	defer server.Close()

	username, _ := secretcap.NewHandle("secret_username")
	password, _ := secretcap.NewHandle("secret_password")
	operationPassword, _ := secretcap.NewHandle("secret_operation_password")
	handler := &confidentialHTTPHandler{
		configuration: channelconfig.Configuration{Config: resourcecap.ProviderConfig{
			ChannelRef: "channel_dynamic", Endpoint: server.URL + "/administrator/v1",
			Settings:       json.RawMessage(`{"__host":{"policy":{"mode":"login_session","mappings":[{"role":"password","slot":"password"},{"role":"username","slot":"username"}],"path":"/login","readPath":["$"],"header":"Access-Token","ttlSeconds":3600}}}`),
			SecretBindings: []resourcecap.SecretBinding{{Name: "username", Handle: username}, {Name: "password", Handle: password}},
		}},
		environment: resourcecap.ProviderEnv{
			HTTPClient:             confidentialHTTPClient{client: server.Client()},
			Secrets:                confidentialHTTPAccessor{"secret_username": []byte("user-value"), "secret_password": []byte("pass-value"), "secret_operation_password": []byte("operation-value")},
			AuthenticationSessions: &confidentialHTTPSessions{},
		},
		operationSecrets: []resourcecap.SecretBinding{{Name: "password", Handle: operationPassword}},
	}
	authResult := executeConfidentialHTTPStep(t, handler, 1, resourcecap.ConfidentialHTTPRequest{
		Kind: resourcecap.ConfidentialHTTPKindAuthenticate, Method: http.MethodPost, Path: "/login",
		CredentialFields: map[string]string{"username": "username", "password": "password"},
	})
	if strings.Contains(string(authResult), "host-token") || strings.Contains(string(authResult), "pass-value") {
		t.Fatalf("authentication result leaked confidential material: %s", authResult)
	}
	businessResult := executeConfidentialHTTPStep(t, handler, 2, resourcecap.ConfidentialHTTPRequest{
		Kind: resourcecap.ConfidentialHTTPKindRequest, Method: http.MethodGet, Path: "/clouds/41", ResponseFields: []string{"data", "id", "status"}, UseAuthentication: true,
	})
	if strings.Contains(string(businessResult), "host-token") || strings.Contains(string(businessResult), "pass-value") || strings.Contains(string(businessResult), "supplier-leak") ||
		!strings.Contains(string(businessResult), `"status":"on"`) {
		t.Fatalf("business result was unsafe or incomplete: %s", businessResult)
	}
	operationResult := executeConfidentialHTTPStep(t, handler, 3, resourcecap.ConfidentialHTTPRequest{
		Kind: resourcecap.ConfidentialHTTPKindRequest, Method: http.MethodPut, Path: "/clouds/41", ResponseFields: []string{"data", "taskid"}, UseAuthentication: true,
		SecretFormBindings: []resourcecap.ConfidentialSecretFormBinding{{Field: "password", Slot: "password"}},
	})
	if strings.Contains(string(operationResult), "operation-value") || strings.Contains(string(operationResult), "supplier-leak") || !strings.Contains(string(operationResult), `"taskid":71`) {
		t.Fatalf("operation result leaked the host-only input or lost its acknowledgement: %s", operationResult)
	}
}

func TestConfidentialHTTPHandlerSignsBTPanelRequestInsideTrustedHost(t *testing.T) {
	const apiSecret = "bt-panel-api-secret"
	var observedBody string
	server := httptest.NewTLSServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost || request.URL.Path != "/system" {
			http.NotFound(response, request)
			return
		}
		_ = request.ParseForm()
		observedBody = request.Form.Encode()
		unixTime, parseErr := strconv.ParseInt(request.Form.Get("request_time"), 10, 64)
		if parseErr != nil || time.Now().Unix()-unixTime > 2 || unixTime-time.Now().Unix() > 2 {
			t.Fatal("host did not inject a current Unix timestamp")
		}
		inner := md5.Sum([]byte(apiSecret))
		innerHex := make([]byte, hex.EncodedLen(len(inner)))
		hex.Encode(innerHex, inner[:])
		outer := md5.Sum(append([]byte(request.Form.Get("request_time")), innerHex...))
		want := make([]byte, hex.EncodedLen(len(outer)))
		hex.Encode(want, outer[:])
		if request.Form.Get("request_token") != string(want) || request.Form.Get("action") != "GetSystemTotal" {
			if request.Form.Get("action") != "EchoToken" {
				t.Fatalf("invalid BT Panel signature form: %s", observedBody)
			}
		}
		if request.Form.Get("action") == "EchoToken" {
			_, _ = response.Write([]byte(`{"status":"` + request.Form.Get("request_token") + `"}`))
			return
		}
		_, _ = response.Write([]byte(`{"status":true,"data":{"name":"panel"}}`))
	}))
	defer server.Close()
	handle, _ := secretcap.NewHandle("secret_bt_panel")
	handler := &confidentialHTTPHandler{
		configuration: channelconfig.Configuration{PluginID: "idc-baota", Config: resourcecap.ProviderConfig{
			ChannelRef: "channel_bt_panel", Endpoint: server.URL,
			Settings:       json.RawMessage(`{"__host":{"policy":{"mode":"bt_panel_double_md5","mappings":[{"role":"material","slot":"api_key"}]}}}`),
			SecretBindings: []resourcecap.SecretBinding{{Name: "api_key", Handle: handle}},
		}},
		environment: resourcecap.ProviderEnv{HTTPClient: confidentialHTTPClient{client: server.Client()}, Secrets: confidentialHTTPAccessor{"secret_bt_panel": []byte(apiSecret)}},
	}
	request := resourcecap.ConfidentialHTTPRequest{Kind: resourcecap.ConfidentialHTTPKindRequest, Method: http.MethodPost, Path: "/system",
		Form: map[string]string{"action": "GetSystemTotal"}, ResponseFields: []string{"status", "data", "name"}, UseAuthentication: true}
	encoded, _ := json.Marshal(request)
	if strings.Contains(string(encoded), apiSecret) || strings.Contains(string(encoded), "request_token") || strings.Contains(string(encoded), "request_time") {
		t.Fatalf("guest payload contained host authentication material: %s", encoded)
	}
	result := executeConfidentialHTTPStep(t, handler, 1, request)
	if !strings.Contains(string(result), `"name":"panel"`) || strings.Contains(string(result), apiSecret) {
		t.Fatalf("unsafe BT Panel result: %s", result)
	}
	for _, field := range []string{"request_time", "request_token"} {
		hostile := request
		hostile.Form = map[string]string{"action": "GetSystemTotal", field: "stale-or-forged"}
		payload, _ := json.Marshal(hostile)
		if _, err := handler.Execute(context.Background(), &pluginhost.ConfidentialOutboundRequest{Sequence: 2, Payload: payload}); err == nil {
			t.Fatalf("guest-controlled %s was accepted", field)
		}
	}
	echo := request
	echo.Form = map[string]string{"action": "EchoToken"}
	payload, _ := json.Marshal(echo)
	if _, err := handler.Execute(context.Background(), &pluginhost.ConfidentialOutboundRequest{Sequence: 3, Payload: payload}); err == nil {
		t.Fatal("supplier reflected a derived BT Panel token into guest-visible data")
	}
	if strings.Contains(observedBody, apiSecret) {
		t.Fatalf("raw API secret was sent to supplier: %s", observedBody)
	}
}

func TestConfidentialHTTPHandlerRejectsRedirectAndAbsolutePath(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		http.Redirect(response, request, "/elsewhere", http.StatusFound)
	}))
	defer server.Close()
	client := server.Client()
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	handler := &confidentialHTTPHandler{
		configuration: channelconfig.Configuration{Config: resourcecap.ProviderConfig{ChannelRef: "channel_dynamic", Endpoint: server.URL,
			Settings: json.RawMessage(`{"__host":{"policy":{"mode":"login_session","mappings":[{"role":"username","slot":"username"}],"path":"/login","readPath":["$"],"header":"Access-Token","ttlSeconds":3600}}}`)}},
		environment: resourcecap.ProviderEnv{HTTPClient: confidentialHTTPClient{client: client}, AuthenticationSessions: &confidentialHTTPSessions{token: []byte("token")}},
		login:       &resourcecap.ConfidentialHTTPRequest{Kind: resourcecap.ConfidentialHTTPKindAuthenticate, Method: http.MethodPost, Path: "/login", CredentialFields: map[string]string{"username": "username"}},
	}
	request, _ := json.Marshal(resourcecap.ConfidentialHTTPRequest{Kind: resourcecap.ConfidentialHTTPKindRequest, Method: http.MethodGet, Path: "/redirect", ResponseFields: []string{"data"}, UseAuthentication: true})
	if _, err := handler.Execute(context.Background(), &pluginhost.ConfidentialOutboundRequest{Sequence: 1, Payload: request}); err == nil {
		t.Fatal("redirect was accepted")
	}
	absolute, _ := json.Marshal(resourcecap.ConfidentialHTTPRequest{Kind: resourcecap.ConfidentialHTTPKindRequest, Method: http.MethodGet, Path: "https://evil.invalid/", ResponseFields: []string{"data"}, UseAuthentication: true})
	if _, err := handler.Execute(context.Background(), &pluginhost.ConfidentialOutboundRequest{Sequence: 2, Payload: absolute}); err == nil {
		t.Fatal("absolute URL was accepted")
	}
}

// TestConfidentialHTTPHandlerRejectsSecretEchoesInAllowedFields proves that
// a hostile supplier cannot hide current secrets under otherwise safe fields.
func TestConfidentialHTTPHandlerRejectsSecretEchoesInAllowedFields(t *testing.T) {
	const (
		credentialCanary = "credential-canary-do-not-leak"
		operationCanary  = "operation-canary-do-not-leak"
		tokenCanary      = "token-canary-do-not-leak"
	)
	server := httptest.NewTLSServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		value := request.URL.Query().Get("echo")
		_, _ = response.Write([]byte(`{"data":{"status":"prefix-` + value + `-suffix","name":"safe"}}`))
	}))
	defer server.Close()

	credential, _ := secretcap.NewHandle("secret_response_credential")
	operation, _ := secretcap.NewHandle("secret_response_operation")
	for _, test := range []struct {
		name              string
		echo              string
		secretFormFields  map[string]string
		operationBindings []resourcecap.SecretBinding
	}{
		{name: "session token", echo: tokenCanary},
		{name: "configured credential", echo: credentialCanary},
		{name: "operation secret", echo: operationCanary, secretFormFields: map[string]string{"password": "password"}, operationBindings: []resourcecap.SecretBinding{{Name: "password", Handle: operation}}},
	} {
		t.Run(test.name, func(t *testing.T) {
			handler := &confidentialHTTPHandler{
				configuration: channelconfig.Configuration{Config: resourcecap.ProviderConfig{
					ChannelRef: "channel_dynamic", Endpoint: server.URL,
					Settings:       json.RawMessage(`{"__host":{"policy":{"mode":"login_session","mappings":[{"role":"username","slot":"username"}],"path":"/login","readPath":["$"],"header":"Access-Token","ttlSeconds":3600}}}`),
					SecretBindings: []resourcecap.SecretBinding{{Name: "username", Handle: credential}},
				}},
				environment: resourcecap.ProviderEnv{
					HTTPClient: confidentialHTTPClient{client: server.Client()},
					Secrets: confidentialHTTPAccessor{
						"secret_response_credential": []byte(credentialCanary),
						"secret_response_operation":  []byte(operationCanary),
					},
					AuthenticationSessions: &confidentialHTTPSessions{token: []byte(tokenCanary)},
				},
				operationSecrets: test.operationBindings,
				login: &resourcecap.ConfidentialHTTPRequest{
					Kind: resourcecap.ConfidentialHTTPKindAuthenticate, Method: http.MethodPost,
					Path: "/login", CredentialFields: map[string]string{"username": "username"},
				},
			}
			payload, err := json.Marshal(resourcecap.ConfidentialHTTPRequest{
				Kind: resourcecap.ConfidentialHTTPKindRequest, Method: http.MethodPost, Path: "/check",
				Query: map[string][]string{"echo": {test.echo}}, ResponseFields: []string{"data", "status", "name"},
				UseAuthentication: true, SecretFormFields: test.secretFormFields,
			})
			if err != nil {
				t.Fatal(err)
			}
			result, err := handler.Execute(context.Background(), &pluginhost.ConfidentialOutboundRequest{Sequence: 1, Payload: payload})
			if err == nil || result != nil {
				t.Fatalf("supplier secret echo was not rejected: result=%#v err=%v", result, err)
			}
			for _, forbidden := range []string{credentialCanary, operationCanary, tokenCanary} {
				if strings.Contains(err.Error(), forbidden) {
					t.Fatalf("screening error reflected confidential plaintext %q", forbidden)
				}
			}
		})
	}
}

func TestConfidentialHTTPHandlerUsesExplicitNestedBearerTokenPolicy(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/zjmf_api_login":
			_, _ = response.Write([]byte(`{"data":{"jwt":"jwt-value"},"jwt":"wrong-value"}`))
		case "/user_info":
			if request.Header.Get("Authorization") != "Bearer jwt-value" {
				t.Fatalf("unexpected authorization header %q", request.Header.Get("Authorization"))
			}
			_, _ = response.Write([]byte(`{"data":{"id":9}}`))
		default:
			http.NotFound(response, request)
		}
	}))
	defer server.Close()
	handle, _ := secretcap.NewHandle("secret_login_username")
	handler := &confidentialHTTPHandler{
		configuration: channelconfig.Configuration{Config: resourcecap.ProviderConfig{
			ChannelRef: "channel_finance", Endpoint: server.URL,
			Settings:       json.RawMessage(`{"__host":{"policy":{"mode":"login_session","mappings":[{"role":"username","slot":"username"}],"path":"/zjmf_api_login","readPath":["data","jwt"],"header":"Authorization","scheme":"Bearer","ttlSeconds":3600}}}`),
			SecretBindings: []resourcecap.SecretBinding{{Name: "username", Handle: handle}},
		}},
		environment: resourcecap.ProviderEnv{
			HTTPClient:             confidentialHTTPClient{client: server.Client()},
			Secrets:                confidentialHTTPAccessor{"secret_login_username": []byte("user")},
			AuthenticationSessions: &confidentialHTTPSessions{},
		},
	}
	executeConfidentialHTTPStep(t, handler, 1, resourcecap.ConfidentialHTTPRequest{
		Kind: resourcecap.ConfidentialHTTPKindAuthenticate, Method: http.MethodPost,
		Path: "/zjmf_api_login", CredentialFields: map[string]string{"username": "username"},
	})
	result := executeConfidentialHTTPStep(t, handler, 2, resourcecap.ConfidentialHTTPRequest{
		Kind: resourcecap.ConfidentialHTTPKindRequest, Method: http.MethodGet,
		Path: "/user_info", ResponseFields: []string{"data", "id"}, UseAuthentication: true,
	})
	if strings.Contains(string(result), "jwt-value") || !strings.Contains(string(result), `"id":9`) {
		t.Fatalf("unsafe bearer response: %s", result)
	}
}

func TestAuthenticationTokenPolicyRejectsMissingOrUnsafeConfiguration(t *testing.T) {
	for _, raw := range []string{
		`{"__host":{"policy":{"mode":"login_session","mappings":[{"role":"username","slot":"username"}],"path":"/login","header":"Authorization"}}}`,
		`{"__host":{"policy":{"mode":"login_session","mappings":[{"role":"username","slot":"username"}],"path":"/login","readPath":["jwt"],"header":"Cookie","ttlSeconds":3600}}}`,
		`{"__host":{"policy":{"mode":"login_session","mappings":[{"role":"username","slot":"username"}],"path":"/login","readPath":["jwt"],"header":"Authorization","scheme":"Basic","ttlSeconds":3600}}}`,
	} {
		handler := &confidentialHTTPHandler{configuration: channelconfig.Configuration{Config: resourcecap.ProviderConfig{Settings: json.RawMessage(raw)}}}
		if _, err := handler.authenticationSettings(); err == nil {
			t.Fatalf("unsafe token policy was accepted: %s", raw)
		}
	}
	if _, err := extractAuthenticationToken([]byte(`{"error":"denied"}`), []string{"jwt"}); err == nil {
		t.Fatal("missing login token was accepted")
	}
}

func TestConfidentialHTTPHandlerRefreshesGenerationOnceForConfiguredBusinessStatus(t *testing.T) {
	loginCount := 0
	businessCount := 0
	server := httptest.NewTLSServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/login":
			loginCount++
			_, _ = response.Write([]byte(`{"jwt":"token-` + string(rune('0'+loginCount)) + `"}`))
		case "/business":
			businessCount++
			if businessCount == 1 {
				_, _ = response.Write([]byte(`{"status":405,"data":{}}`))
				return
			}
			_, _ = response.Write([]byte(`{"status":200,"data":{"id":7}}`))
		}
	}))
	defer server.Close()
	handle, _ := secretcap.NewHandle("secret_refresh_username")
	handler := &confidentialHTTPHandler{
		configuration: channelconfig.Configuration{Config: resourcecap.ProviderConfig{
			ChannelRef: "channel_refresh", Endpoint: server.URL,
			Settings:       json.RawMessage(`{"__host":{"policy":{"mode":"login_session","mappings":[{"role":"username","slot":"username"}],"path":"/login","readPath":["jwt"],"header":"Authorization","scheme":"Bearer","refreshPath":["status"],"refreshValue":["405"],"ttlSeconds":3600}}}`),
			SecretBindings: []resourcecap.SecretBinding{{Name: "username", Handle: handle}},
		}},
		environment: resourcecap.ProviderEnv{HTTPClient: confidentialHTTPClient{client: server.Client()},
			Secrets: confidentialHTTPAccessor{"secret_refresh_username": []byte("user")}, AuthenticationSessions: &confidentialHTTPSessions{}},
	}
	executeConfidentialHTTPStep(t, handler, 1, resourcecap.ConfidentialHTTPRequest{Kind: resourcecap.ConfidentialHTTPKindAuthenticate, Method: http.MethodPost, Path: "/login", CredentialFields: map[string]string{"username": "username"}})
	result := executeConfidentialHTTPStep(t, handler, 2, resourcecap.ConfidentialHTTPRequest{Kind: resourcecap.ConfidentialHTTPKindRequest, Method: http.MethodGet, Path: "/business", ResponseFields: []string{"status", "data", "id"}, UseAuthentication: true})
	if loginCount != 2 || businessCount != 2 || !strings.Contains(string(result), `"id":7`) {
		t.Fatalf("refresh counts login=%d business=%d result=%s", loginCount, businessCount, result)
	}
}

// TestConfidentialHTTPHandlerRejectsSensitiveResponseFieldAliases keeps the
// owner allowlist closed even if a managed guest asks for another spelling.
func TestConfidentialHTTPHandlerRejectsSensitiveResponseFieldAliases(t *testing.T) {
	for _, field := range []string{"rootpassword", "osuser", "management_url", "key", "auth"} {
		t.Run(field, func(t *testing.T) {
			err := validateConfidentialHTTPRequest(resourcecap.ConfidentialHTTPRequest{
				Kind: resourcecap.ConfidentialHTTPKindRequest, Method: http.MethodGet, Path: "/clouds/41",
				ResponseFields: []string{"data", field}, UseAuthentication: true,
			})
			if err == nil {
				t.Fatalf("sensitive response field %q was accepted", field)
			}
		})
	}
}

func executeConfidentialHTTPStep(t *testing.T, handler *confidentialHTTPHandler, sequence uint8, request resourcecap.ConfidentialHTTPRequest) json.RawMessage {
	t.Helper()
	payload, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	result, err := handler.Execute(context.Background(), &pluginhost.ConfidentialOutboundRequest{Sequence: sequence, Payload: payload})
	if err != nil {
		t.Fatalf("execute confidential HTTP step %d: %v", sequence, err)
	}
	return result.Payload
}

// A warm business session cannot make an explicit connection test succeed while
// the supplier is unreachable or rejects credentials; probes leave it intact.
func TestConfidentialConnectionProbeBypassesWarmSession(t *testing.T) {
	for _, tc := range []struct {
		name     string
		fresh    bool
		status   int
		calls    int
		accepted bool
	}{
		{"ordinary authentication reuses session", false, http.StatusUnauthorized, 0, true},
		{"connection probe performs fresh login", true, http.StatusOK, 1, true},
		{"cached session cannot hide rejection", true, http.StatusUnauthorized, 1, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if r.URL.Path != "/api/v1/login" || r.Method != http.MethodPost {
					t.Errorf("unexpected probe destination")
				}
				w.WriteHeader(tc.status)
				if _, err := w.Write([]byte(`"fresh-private-token"`)); err != nil {
					t.Error(err)
				}
			}))
			defer server.Close()
			username, err := secretcap.NewHandle("secret_username")
			if err != nil {
				t.Fatal(err)
			}
			password, err := secretcap.NewHandle("secret_password")
			if err != nil {
				t.Fatal(err)
			}
			sessions := &confidentialHTTPSessions{token: []byte("existing-business-token"), generation: 7}
			h := &confidentialHTTPHandler{
				forceFreshAuthentication: tc.fresh,
				configuration: channelconfig.Configuration{Config: resourcecap.ProviderConfig{
					ChannelRef: "probe_channel", Endpoint: server.URL + "/api/v1",
					Settings:       json.RawMessage(`{"__host":{"policy":{"mode":"login_session","mappings":[{"role":"password","slot":"password"},{"role":"username","slot":"username"}],"path":"/login","readPath":["$"],"header":"Access-Token","ttlSeconds":3600}}}`),
					SecretBindings: []resourcecap.SecretBinding{{Name: "username", Handle: username}, {Name: "password", Handle: password}},
				}},
				environment: resourcecap.ProviderEnv{HTTPClient: confidentialHTTPClient{server.Client()}, Secrets: confidentialHTTPAccessor{"secret_username": []byte("probe-user"), "secret_password": []byte("probe-password")}, AuthenticationSessions: sessions},
			}
			payload, err := json.Marshal(resourcecap.ConfidentialHTTPRequest{Kind: resourcecap.ConfidentialHTTPKindAuthenticate, Method: http.MethodPost, Path: "/login", CredentialFields: map[string]string{"username": "username", "password": "password"}})
			if err != nil {
				t.Fatal(err)
			}
			result, err := h.Execute(context.Background(), &pluginhost.ConfidentialOutboundRequest{Sequence: 1, Payload: payload})
			if calls != tc.calls || string(sessions.token) != "existing-business-token" || sessions.generation != 7 {
				t.Fatalf("probe changed session or skipped supplier: calls=%d generation=%d", calls, sessions.generation)
			}
			if !tc.accepted {
				if !bizerr.Is(err, resourcecap.CodeProviderAuthRejected) || result != nil {
					t.Fatalf("cached session hid rejection: %v", err)
				}
				return
			}
			if err != nil || result == nil {
				t.Fatalf("authentication failed: %v", err)
			}
			var response resourcecap.ConfidentialHTTPResponse
			if json.Unmarshal(result.Payload, &response) != nil || !response.Authenticated || strings.Contains(string(result.Payload), "private-token") || strings.Contains(string(result.Payload), "probe-password") {
				t.Fatalf("probe response invalid or confidential")
			}
		})
	}
}
