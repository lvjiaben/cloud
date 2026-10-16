// This file models the real secret service's collapsed consumer errors and
// proves only reviewed owner codes survive nested secret callbacks.

package composition

import (
	"context"
	"encoding/json"
	"github.com/gogf/gf/v2/errors/gerror"
	"lina-core/pkg/bizerr"
	"lina-core/pkg/plugin/pluginhost"
	"lina-plugin-idc-core/backend/cap/secretcap"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
	"lina-plugin-idc-resource/backend/internal/service/channelconfig"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type maskingProviderSecretAccessor struct {
	denied error
	calls  int
}

func (accessor *maskingProviderSecretAccessor) Access(_ context.Context, _ secretcap.AccessRequest, consume secretcap.Consumer) (err error) {
	if accessor.denied != nil {
		return accessor.denied
	}
	accessor.calls++
	value := []byte("private-secret-canary")
	defer clear(value)
	defer func() {
		if recover() != nil {
			err = gerror.New("masked secret consumer failure")
		}
	}()
	if consume(value) != nil {
		return gerror.New("masked secret consumer failure")
	}
	return nil
}

func TestProviderSecretAccessPreservesOnlyReviewedClassification(t *testing.T) {
	for _, code := range []*bizerr.Code{resourcecap.CodeProviderTLSUntrusted, resourcecap.CodeProviderTimeout, resourcecap.CodeProviderDNS, resourcecap.CodeProviderNetwork, resourcecap.CodeProviderEndpointNotFound, resourcecap.CodeProviderInvalidResponse, resourcecap.CodeProviderAuthRejected, resourcecap.CodeProviderLocalAuthPolicy} {
		t.Run(code.RuntimeCode(), func(t *testing.T) {
			accessor := &maskingProviderSecretAccessor{}
			err := accessConfidentialProviderSecret(context.Background(), accessor, secretcap.AccessRequest{}, func([]byte) error {
				return accessConfidentialProviderSecret(context.Background(), accessor, secretcap.AccessRequest{}, func([]byte) error {
					return bizerr.WrapCode(gerror.New("private-secret-canary supplier-raw-body"), code)
				})
			})
			if !bizerr.Is(err, code) || accessor.calls != 2 {
				t.Fatalf("classification lost: err=%v calls=%d", err, accessor.calls)
			}
			if strings.Contains(gerror.Stack(err), "private-secret-canary") || strings.Contains(gerror.Stack(err), "supplier-raw-body") {
				t.Fatal("raw consumer cause escaped")
			}
		})
	}
	for _, panicConsumer := range []bool{false, true} {
		err := accessConfidentialProviderSecret(context.Background(), &maskingProviderSecretAccessor{}, secretcap.AccessRequest{}, func([]byte) error {
			if panicConsumer {
				panic("private-secret-canary")
			}
			return gerror.New("private-secret-canary")
		})
		if err == nil || err.Error() != "masked secret consumer failure" || resourcecap.SafeProviderConnectionError(err) != nil {
			t.Fatalf("unreviewed callback failure escaped: %v", err)
		}
	}
	denied := gerror.New("secret access denied")
	called := false
	err := accessConfidentialProviderSecret(context.Background(), &maskingProviderSecretAccessor{denied: denied}, secretcap.AccessRequest{}, func([]byte) error { called = true; return nil })
	if err != denied || called {
		t.Fatal("secret authorization failure was overridden")
	}
}

func TestProviderLoginRetainsTLSAndAuthFailureThroughMaskingSecretService(t *testing.T) {
	for _, test := range []struct {
		name      string
		status    int
		untrusted bool
		code      *bizerr.Code
	}{
		{name: "untrusted TLS", status: 200, untrusted: true, code: resourcecap.CodeProviderTLSUntrusted},
		{name: "authentication rejected", status: 401, code: resourcecap.CodeProviderAuthRejected},
		{name: "endpoint missing", status: 404, code: resourcecap.CodeProviderEndpointNotFound},
		{name: "supplier network status", status: 503, code: resourcecap.CodeProviderNetwork},
	} {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewTLSServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
				response.WriteHeader(test.status)
				if _, err := response.Write([]byte(`{"supplier_message":"private-secret-canary raw-supplier-reason"}`)); err != nil {
					t.Error(err)
				}
			}))
			defer server.Close()
			client := server.Client()
			if test.untrusted {
				client = &http.Client{}
			}
			username, err := secretcap.NewHandle("secret_masking_username")
			if err != nil {
				t.Fatal(err)
			}
			password, err := secretcap.NewHandle("secret_masking_password")
			if err != nil {
				t.Fatal(err)
			}
			accessor := &maskingProviderSecretAccessor{}
			handler := &confidentialHTTPHandler{forceFreshAuthentication: true, configuration: channelconfig.Configuration{Config: resourcecap.ProviderConfig{ChannelRef: "masked_channel", Endpoint: server.URL, Settings: json.RawMessage(`{"__host":{"policy":{"mode":"login_session","mappings":[{"role":"password","slot":"password"},{"role":"username","slot":"username"}],"path":"/login","readPath":["$"],"header":"Access-Token","ttlSeconds":3600}}}`), SecretBindings: []resourcecap.SecretBinding{{Name: "username", Handle: username}, {Name: "password", Handle: password}}}}, environment: resourcecap.ProviderEnv{HTTPClient: confidentialHTTPClient{client}, Secrets: accessor}}
			result, err := handler.Execute(context.Background(), &pluginhost.ConfidentialOutboundRequest{Sequence: 1, Payload: []byte(`{"kind":"authenticate","method":"POST","path":"/login"}`)})
			if result != nil || !bizerr.Is(err, test.code) || accessor.calls != 2 {
				t.Fatalf("safe login diagnostic lost: result=%#v err=%v calls=%d", result, err, accessor.calls)
			}
			for _, forbidden := range []string{"private-secret-canary", "raw-supplier-reason", server.URL} {
				if strings.Contains(gerror.Stack(err), forbidden) {
					t.Fatal("confidential login cause escaped")
				}
			}
		})
	}
}
