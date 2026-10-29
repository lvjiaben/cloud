// Slow login is prepared by the trusted owner while business calls retain the
// unchanged short guest deadline and confidential response projection.
package composition

import (
	"context"
	"io"
	"lina-core/pkg/plugin/pluginhost"
	"lina-plugin-idc-core/backend/cap/secretcap"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
	"lina-plugin-idc-resource/backend/internal/service/channelconfig"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestAuthenticationPreparationKeepsSlowLoginOutsideGuestBudget(t *testing.T) {
	var logins, business atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/login" {
			logins.Add(1)
			select {
			case <-r.Context().Done():
				return
			case <-time.After(6 * time.Second):
			}
			if _, err := io.WriteString(w, `"private-session-token"`); err != nil {
				t.Error("login fixture response failed")
			}
			return
		}
		business.Add(1)
		if _, err := io.WriteString(w, `{"data":{"id":41,"status":"on"}}`); err != nil {
			t.Error("business fixture response failed")
		}
	}))
	t.Cleanup(server.Close)
	username, err := secretcap.NewHandle("secret_prepare_username")
	if err != nil {
		t.Fatal(err)
	}
	password, err := secretcap.NewHandle("secret_prepare_password")
	if err != nil {
		t.Fatal(err)
	}
	h := &confidentialHTTPHandler{
		configuration: channelconfig.Configuration{PluginID: "idc-zjmfcloud", Config: resourcecap.ProviderConfig{Endpoint: server.URL, Settings: diagnosticLoginSettings(), SecretBindings: []resourcecap.SecretBinding{{Name: "username", Handle: username}, {Name: "password", Handle: password}}}},
		environment:   resourcecap.ProviderEnv{HTTPClient: confidentialHTTPClient{client: server.Client()}, Secrets: confidentialHTTPAccessor{"secret_prepare_username": []byte("fixture-user"), "secret_prepare_password": []byte("fixture-password")}, AuthenticationSessions: &confidentialHTTPSessions{}},
	}
	// This reproduces the original cold login failure under the real host budget.
	guestCtx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	err = h.authenticate(guestCtx, resourcecap.ConfidentialHTTPRequest{Kind: resourcecap.ConfidentialHTTPKindAuthenticate, Method: http.MethodPost, Path: "/login"})
	cancel()
	if err == nil || business.Load() != 0 {
		t.Fatal("cold login did not exhaust the guest budget safely")
	}
	if err = h.prepareAuthentication(t.Context()); err != nil {
		t.Fatal("owner could not prepare the slow login")
	}
	if logins.Load() != 2 || business.Load() != 0 {
		t.Fatal("preparation repeated login or sent a business request")
	}
	guestCtx, cancel = context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	if _, err = h.Execute(guestCtx, &pluginhost.ConfidentialOutboundRequest{Sequence: 1, Payload: []byte(`{"kind":"authenticate","method":"POST","path":"/login"}`)}); err != nil {
		t.Fatal("prepared guest login failed")
	}
	if _, err = h.Execute(guestCtx, &pluginhost.ConfidentialOutboundRequest{Sequence: 2, Payload: []byte(`{"kind":"request","method":"GET","path":"/clouds/41","responseFields":["data","id","status"],"useAuthentication":true}`)}); err != nil {
		t.Fatal("governed business read failed")
	}
	if logins.Load() != 2 || business.Load() != 1 {
		t.Fatal("prepared execution repeated authentication or business request")
	}
}

func TestAuthenticationPreparationDoesNotGuessOtherConnectors(t *testing.T) {
	for _, h := range []*confidentialHTTPHandler{
		{configuration: channelconfig.Configuration{PluginID: "another-connector"}},
		{configuration: channelconfig.Configuration{PluginID: "idc-zjmfcloud"}, providerMethod: resourcecap.DynamicProviderMethodDiscoverCapabilities},
	} {
		if err := h.prepareAuthentication(t.Context()); err != nil {
			t.Fatal("unrelated connector or static capability read entered preparation")
		}
	}
}

// A connection probe must really log in even with a warm shared session; the
// fresh proof can then be consumed once within the ordinary guest execution.
func TestPreparedConnectionProbeUsesFreshLoginWithoutChangingSharedToken(t *testing.T) {
	for _, rejected := range []bool{false, true} {
		t.Run(map[bool]string{false: "accepted", true: "rejected"}[rejected], func(t *testing.T) {
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				if r.URL.Path != "/login" {
					t.Error("probe sent a resource request")
					w.WriteHeader(400)
					return
				}
				if rejected {
					w.WriteHeader(http.StatusUnauthorized)
					return
				}
				select {
				case <-r.Context().Done():
					return
				case <-time.After(6 * time.Second):
				}
				if _, err := io.WriteString(w, `"new-private-probe-token"`); err != nil {
					t.Error("probe fixture response failed")
				}
			}))
			t.Cleanup(server.Close)
			username, err := secretcap.NewHandle("secret_probe_prepare_user")
			if err != nil {
				t.Fatal(err)
			}
			password, err := secretcap.NewHandle("secret_probe_prepare_password")
			if err != nil {
				t.Fatal(err)
			}
			sessions := &confidentialHTTPSessions{token: []byte("old-private-business-session")}
			h := &confidentialHTTPHandler{
				configuration:            channelconfig.Configuration{PluginID: "idc-zjmfcloud", Config: resourcecap.ProviderConfig{Endpoint: server.URL, Settings: diagnosticLoginSettings(), SecretBindings: []resourcecap.SecretBinding{{Name: "username", Handle: username}, {Name: "password", Handle: password}}}},
				environment:              resourcecap.ProviderEnv{HTTPClient: confidentialHTTPClient{client: server.Client()}, Secrets: confidentialHTTPAccessor{"secret_probe_prepare_user": []byte("fixture-user"), "secret_probe_prepare_password": []byte("fixture-password")}, AuthenticationSessions: sessions},
				forceFreshAuthentication: true,
			}
			err = h.prepareAuthentication(t.Context())
			if (err != nil) != rejected || calls.Load() != 1 || string(sessions.token) != "old-private-business-session" {
				t.Fatal("probe reused or changed the shared business session")
			}
			if rejected {
				if h.preparedProbeAuthentication {
					t.Fatal("rejected login created an authentication proof")
				}
				return
			}
			guestCtx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()
			if _, err = h.Execute(guestCtx, &pluginhost.ConfidentialOutboundRequest{Sequence: 1, Payload: []byte(`{"kind":"authenticate","method":"POST","path":"/login"}`)}); err != nil || calls.Load() != 1 || h.preparedProbeAuthentication {
				t.Fatal("fresh probe was lost, repeated or reusable")
			}
		})
	}
}

// A warm session must not waive the configured login instruction validation.
func TestWarmAuthenticationStillValidatesConfiguredLoginInstruction(t *testing.T) {
	h := &confidentialHTTPHandler{
		configuration: channelconfig.Configuration{PluginID: "idc-zjmfcloud", Config: resourcecap.ProviderConfig{Endpoint: "https://example.test", Settings: diagnosticLoginSettings()}},
		environment:   resourcecap.ProviderEnv{AuthenticationSessions: &confidentialHTTPSessions{token: []byte("private-warm-session")}},
	}
	for _, request := range []resourcecap.ConfidentialHTTPRequest{
		{Kind: resourcecap.ConfidentialHTTPKindAuthenticate, Method: http.MethodPost, Path: "/unapproved-login"},
		{Kind: resourcecap.ConfidentialHTTPKindAuthenticate, Method: http.MethodGet, Path: "/login"},
		{Kind: resourcecap.ConfidentialHTTPKindAuthenticate, Method: http.MethodPost, Path: "/login", CredentialFields: map[string]string{"unexpected": "password"}},
	} {
		if err := h.authenticate(t.Context(), request); err == nil {
			t.Fatal("warm token accepted an unapproved authentication instruction")
		}
	}
}
