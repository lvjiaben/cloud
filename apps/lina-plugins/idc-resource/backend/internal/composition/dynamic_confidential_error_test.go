// These tests prove diagnostics survive confidential runtime sanitization while
// no endpoint, supplier text or credential is retained in the returned error.
package composition

import (
	"context"
	"crypto/x509"
	"encoding/json"
	"errors"
	"lina-core/pkg/bizerr"
	"lina-core/pkg/plugin/capability/capmodel"
	"lina-core/pkg/plugin/pluginhost"
	"lina-plugin-idc-core/backend/cap/secretcap"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
	"lina-plugin-idc-resource/backend/internal/service/channelconfig"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestConfidentialTransportDiagnostic(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
		code *bizerr.Code
	}{
		{"tls", x509.UnknownAuthorityError{}, resourcecap.CodeProviderTLSUntrusted},
		{"timeout", context.DeadlineExceeded, resourcecap.CodeProviderTimeout},
		{"dns", &net.DNSError{Err: "secret-address", Name: "private.internal"}, resourcecap.CodeProviderDNS},
		{"network", errors.New("private-secret"), resourcecap.CodeProviderNetwork},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := confidentialTransportError(&url.Error{Op: "POST", URL: "https://private.internal/password", Err: tc.err})
			if !bizerr.Is(result, tc.code) || strings.Contains(result.Error(), "private") || errors.Is(result, tc.err) {
				t.Fatalf("unsafe or incorrect diagnostic: %v", result)
			}
		})
	}
}

func TestConfidentialHTTPStatusDiagnostic(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status int
		body   string
		code   *bizerr.Code
	}{
		{"missing", 404, "<html>private</html>", resourcecap.CodeProviderEndpointNotFound},
		{"unauthorized", 401, "private", resourcecap.CodeProviderAuthRejected},
		{"forbidden", 403, "private", resourcecap.CodeProviderAuthRejected},
		{"html", 200, "<html>private</html>", resourcecap.CodeProviderInvalidResponse},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer server.Close()
			h := confidentialHTTPHandler{configuration: channelconfig.Configuration{Config: resourcecap.ProviderConfig{Endpoint: server.URL}}, environment: resourcecap.ProviderEnv{HTTPClient: confidentialHTTPClient{server.Client()}}}
			_, err := h.perform(context.Background(), resourcecap.ConfidentialHTTPRequest{Method: http.MethodGet, Path: "/api"}, nil)
			if !bizerr.Is(err, tc.code) || strings.Contains(err.Error(), "private") {
				t.Fatalf("unsafe status diagnostic: %v", err)
			}
		})
	}
}

type sanitizedDiagnosticRegistrar struct {
	dynamicRegistrarFake
	returnProof bool
}

func (f *sanitizedDiagnosticRegistrar) InvokeProviderMethod(ctx context.Context, input pluginhost.BusinessExtensionMethodInvokeInput) (*capmodel.BusinessExtensionMethodResult, error) {
	_, err := f.dynamicRegistrarFake.InvokeProviderMethod(ctx, input)
	if err != nil {
		if f.returnProof {
			return &capmodel.BusinessExtensionMethodResult{Payload: []byte("{\"Verified\":false}")}, nil
		}
		return nil, errors.New("host sanitized the callback error")
	}
	return nil, errors.New("unexpected successful callback")
}
func TestDynamicProviderPreservesOnlyOwnerDiagnostic(t *testing.T) {
	for _, code := range []*bizerr.Code{resourcecap.CodeProviderTLSUntrusted, resourcecap.CodeProviderTimeout, resourcecap.CodeProviderDNS, resourcecap.CodeProviderNetwork, resourcecap.CodeProviderEndpointNotFound, resourcecap.CodeProviderInvalidResponse, resourcecap.CodeProviderAuthRejected, resourcecap.CodeProviderLocalAuthPolicy} {
		registrar := &sanitizedDiagnosticRegistrar{}
		p := &dynamicProvider{extensions: registrar, configuration: channelconfig.Configuration{Config: resourcecap.ProviderConfig{Settings: []byte("{}")}}, outbound: func(context.Context, channelconfig.Configuration, string, []resourcecap.SecretBinding) (pluginhost.ConfidentialOutboundHandler, func(), error) {
			return func(context.Context, *pluginhost.ConfidentialOutboundRequest) (*pluginhost.ConfidentialOutboundResult, error) {
				return nil, bizerr.NewCode(code)
			}, func() {}, nil
		}}
		var target resourcecap.ConnectionVerification
		err := p.invoke(context.Background(), resourcecap.DynamicProviderMethodVerifyConnection, struct{}{}, nil, &target)
		if !bizerr.Is(err, code) || strings.Contains(err.Error(), "callback") {
			t.Fatalf("owner classification lost: %v", err)
		}
		registrar.returnProof = true
		err = p.invoke(context.Background(), resourcecap.DynamicProviderMethodVerifyConnection, struct{}{}, nil, &target)
		if !bizerr.Is(err, code) {
			t.Fatalf("false proof swallowed owner diagnostic: %v", err)
		}

	}
}

func TestConfidentialFrozenLoginPathNormalization(t *testing.T) {
	for _, tc := range []struct{ endpoint, declared, want string }{
		{"https://example.test/custom/v1", "/administrator/v1/login", "/login"},
		{"https://example.test/administrator/v1", "/login", "/login"},
		{"https://example.test/custom/v2", "/administrator/v1/login", "/administrator/v1/login"},
		{"https://example.test/custom", "/administrator/v1/login", "/administrator/v1/login"},
	} {
		if got := confidentialCanonicalLoginPath(tc.endpoint, tc.declared); got != tc.want {
			t.Fatalf("canonical %q, want %q", got, tc.want)
		}
	}
}

func TestConfidentialFrozenLoginDispatchAndPolicy(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.URL.Path != "/custom/v1/login" {
			t.Errorf("duplicated API prefix: %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`"session-token"`))
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
	h := confidentialHTTPHandler{configuration: channelconfig.Configuration{Config: resourcecap.ProviderConfig{
		Endpoint:       server.URL + "/custom/v1",
		Settings:       json.RawMessage(`{"__host":{"policy":{"mode":"login_session","mappings":[{"role":"password","slot":"password"},{"role":"username","slot":"username"}],"path":"/administrator/v1/login","readPath":["$"],"header":"Access-Token","ttlSeconds":3600}}}`),
		SecretBindings: []resourcecap.SecretBinding{{Name: "username", Handle: username}, {Name: "password", Handle: password}},
	}}, environment: resourcecap.ProviderEnv{HTTPClient: confidentialHTTPClient{server.Client()}, Secrets: confidentialHTTPAccessor{"secret_username": []byte("user-value"), "secret_password": []byte("pass-value")}}}
	for _, path := range []string{"/login", "/administrator/v1/login", "/login"} {
		request := resourcecap.ConfidentialHTTPRequest{Kind: resourcecap.ConfidentialHTTPKindAuthenticate, Method: http.MethodPost, Path: path}
		if path != "/login" {
			request.CredentialFields = map[string]string{"username": "username", "password": "password"}
		}
		if err := validateConfidentialHTTPRequest(request); err != nil {
			t.Fatal(err)
		}
		token, err := h.performLogin(context.Background(), request)
		if err != nil || string(token) != "session-token" {
			t.Fatalf("frozen path rejected: %v", err)
		}
	}
	if calls != 3 {
		t.Fatal("expected exactly three authenticated calls", calls)
	}
	_, err = h.performLogin(context.Background(), resourcecap.ConfidentialHTTPRequest{Method: http.MethodPost, Path: "/unrelated/v1/login", CredentialFields: map[string]string{"username": "username", "password": "password"}})
	if !bizerr.Is(err, resourcecap.CodeProviderLocalAuthPolicy) || calls != 3 {
		t.Fatalf("unrelated path accepted or misclassified: %v calls=%d", err, calls)
	}
	_, err = h.performLogin(context.Background(), resourcecap.ConfidentialHTTPRequest{Method: http.MethodPost, Path: "/login", CredentialFields: map[string]string{"password": "username"}})
	if !bizerr.Is(err, resourcecap.CodeProviderLocalAuthPolicy) || calls != 3 {
		t.Fatal("forged explicit credential mapping accepted")
	}
}
