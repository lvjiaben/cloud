// This file verifies Basic credentials are injected by the trusted resource
// boundary and cannot be forged by or echoed to a managed adapter.
package composition

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"lina-core/pkg/plugin/pluginhost"
	"lina-plugin-idc-core/backend/cap/secretcap"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
	"lina-plugin-idc-resource/backend/internal/service/channelconfig"
)

func TestConfidentialHTTPBasicKeepsApplicationCredentialsOutsideGuest(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		username, password, ok := r.BasicAuth()
		if !ok || username != "app_test" || password != "test-secret" {
			t.Error("Basic application credentials were not injected")
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if r.URL.Path == "/echo" {
			if err := json.NewEncoder(w).Encode(map[string]string{"status": r.Header.Get("Authorization")}); err != nil {
				t.Error(err)
			}
			return
		}
		if r.URL.Path == "/quotes" {
			var body struct {
				RequestID string `json:"requestId"`
				Purchase  struct {
					Quantity int `json:"quantity"`
				} `json:"purchase"`
			}
			if r.Method != http.MethodPost || r.Header.Get("Content-Type") != "application/json" || json.NewDecoder(r.Body).Decode(&body) != nil || body.RequestID != "quote-test" || body.Purchase.Quantity != 2 {
				t.Error("structured purchase JSON was not forwarded intact")
				w.WriteHeader(http.StatusBadRequest)
				return
			}
		}
		if _, err := w.Write([]byte(`{"status":"ready"}`)); err != nil {
			t.Error(err)
		}
	}))
	defer server.Close()
	user, err := secretcap.NewHandle("secret_app_identity")
	if err != nil {
		t.Fatal(err)
	}
	secret, err := secretcap.NewHandle("secret_app_secret")
	if err != nil {
		t.Fatal(err)
	}
	handler := &confidentialHTTPHandler{configuration: channelconfig.Configuration{Config: resourcecap.ProviderConfig{ChannelRef: "channel_self", Endpoint: server.URL, Settings: json.RawMessage(`{"__host":{"policy":{"mode":"per_request_basic","mappings":[{"role":"username","slot":"app_id"},{"role":"password","slot":"app_secret"}]}}}`), SecretBindings: []resourcecap.SecretBinding{{Name: "app_id", Handle: user}, {Name: "app_secret", Handle: secret}}}}, environment: resourcecap.ProviderEnv{HTTPClient: confidentialHTTPClient{client: server.Client()}, Secrets: confidentialHTTPAccessor{"secret_app_identity": []byte("app_test"), "secret_app_secret": []byte("test-secret")}}}
	request := resourcecap.ConfidentialHTTPRequest{Kind: resourcecap.ConfidentialHTTPKindRequest, Method: http.MethodGet, Path: "/capabilities", UseAuthentication: true, ResponseFields: []string{"status"}}
	result := executeConfidentialHTTPStep(t, handler, 1, request)
	if !strings.Contains(string(result), "ready") || strings.Contains(string(result), "test-secret") {
		t.Fatal("unexpected public projection")
	}
	payload, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	payload = append(payload[:len(payload)-1], []byte(`,"headers":{"Authorization":"Basic forged"}}`)...)
	if _, err = handler.Execute(context.Background(), &pluginhost.ConfidentialOutboundRequest{Sequence: 2, Payload: payload}); err == nil {
		t.Fatal("adapter forged the authentication header")
	}
	request.Headers = nil
	request.Path = "/echo"
	payload, err = json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = handler.Execute(context.Background(), &pluginhost.ConfidentialOutboundRequest{Sequence: 3, Payload: payload}); err == nil {
		t.Fatal("upstream echoed the Basic secret into the guest")
	}
	request.Path = "/quotes"
	request.Method = http.MethodPost
	request.JSONBody = json.RawMessage(`{"requestId":"quote-test","purchase":{"quantity":2}}`)
	result = executeConfidentialHTTPStep(t, handler, 4, request)
	if !strings.Contains(string(result), "ready") {
		t.Fatal("structured purchase request failed")
	}
	for _, invalid := range []resourcecap.ConfidentialHTTPRequest{
		{Kind: resourcecap.ConfidentialHTTPKindRequest, Method: http.MethodPost, JSONBody: json.RawMessage(`[]`)},
		{Kind: resourcecap.ConfidentialHTTPKindRequest, Method: http.MethodGet, JSONBody: json.RawMessage(`{}`)},
		{Kind: resourcecap.ConfidentialHTTPKindRequest, Method: http.MethodPost, JSONBody: json.RawMessage(`{}`), Form: map[string]string{"quantity": "2"}},
		{Kind: resourcecap.ConfidentialHTTPKindRequest, Method: http.MethodPost, JSONBody: json.RawMessage(`{}`), SecretFormFields: map[string]string{"password": "password"}},
	} {
		invalid.Path = "/quotes"
		invalid.UseAuthentication = true
		invalid.ResponseFields = []string{"status"}
		if err := validateConfidentialHTTPRequest(invalid); err == nil {
			t.Fatal("ambiguous or invalid JSON request accepted")
		}
	}
}
