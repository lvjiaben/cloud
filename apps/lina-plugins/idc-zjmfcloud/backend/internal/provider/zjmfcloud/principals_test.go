package zjmfcloud

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"

	"lina-plugin-idc-core/backend/cap/secretcap"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
)

func TestLookupPrincipalDistinguishesExactAbsenceFromFailure(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name       string
		body       string
		wantExists bool
		wantRef    string
		wantError  bool
	}{
		{name: "found", body: `{"data":{"id":42}}`, wantExists: true, wantRef: "42"},
		{name: "absent", body: `{"id":0}`},
		{name: "missing id", body: `{"data":{}}`, wantError: true},
		{name: "negative id", body: `{"id":-1}`, wantError: true},
	} {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			var loginCount int
			var mu sync.Mutex
			login := loginHandler(t, &loginCount, &mu, "token-principal")
			fixture := newProviderFixture(t, http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				if request.URL.Path == "/administrator/v1/login" {
					login(writer, request)
					return
				}
				if request.URL.Path != "/administrator/v1/user/check" || request.Method != http.MethodPost {
					t.Errorf("unexpected principal lookup %s %s", request.Method, request.URL.Path)
				}
				if err := request.ParseForm(); err != nil {
					t.Errorf("parse lookup form: %v", err)
				}
				if request.Form.Get("username") != "principal-user" || len(request.Form) != 1 {
					t.Errorf("lookup form=%v", request.Form)
				}
				writeJSON(t, writer, http.StatusOK, test.body)
			}))
			username := putPrincipalSecret(t, fixture, "username", "principal-user")
			result, err := fixture.provider.LookupPrincipal(context.Background(), resourcecap.PrincipalLookupRequest{
				RequestID: "principal-lookup-1", Username: username,
			})
			if test.wantError {
				var callError *resourcecap.ProviderCallError
				if result != nil || !errors.As(err, &callError) || callError.ResultUnknown {
					t.Fatalf("malformed lookup result=%+v err=%v", result, err)
				}
				return
			}
			if err != nil || result == nil || result.Exists != test.wantExists ||
				result.ExternalPrincipalRef != test.wantRef || result.Validate() != nil {
				t.Fatalf("lookup result=%+v err=%v", result, err)
			}
		})
	}
}

func TestCreatePrincipalUsesOnlySecretBindingsAndReturnsOpaqueID(t *testing.T) {
	t.Parallel()
	var loginCount int
	var mu sync.Mutex
	login := loginHandler(t, &loginCount, &mu, "token-principal")
	var createCalls atomic.Int64
	fixture := newProviderFixture(t, http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/administrator/v1/login" {
			login(writer, request)
			return
		}
		createCalls.Add(1)
		if request.URL.Path != "/administrator/v1/user" || request.Method != http.MethodPost {
			t.Errorf("unexpected principal create %s %s", request.Method, request.URL.Path)
		}
		if err := request.ParseForm(); err != nil {
			t.Errorf("parse create form: %v", err)
		}
		want := map[string]string{
			"state": "1", "username": "principal-user", "password": "principal-pass",
			"phone": "13800138000", "email": "principal@example.invalid", "real_name": "Test User",
		}
		if len(request.Form) != len(want) {
			t.Errorf("create form=%v", request.Form)
		}
		for name, value := range want {
			if request.Form.Get(name) != value {
				t.Errorf("create field %s=%q, want %q", name, request.Form.Get(name), value)
			}
		}
		writeJSON(t, writer, http.StatusCreated, `{"data":{"id":"43","password":"discard-me"}}`)
	}))
	bindings := []resourcecap.SecretBinding{
		{Name: resourcecap.PrincipalBindingUsername, Handle: putPrincipalSecret(t, fixture, "username", "principal-user")},
		{Name: resourcecap.PrincipalBindingPassword, Handle: putPrincipalSecret(t, fixture, "password", "principal-pass")},
		{Name: resourcecap.PrincipalBindingPhone, Handle: putPrincipalSecret(t, fixture, "phone", "13800138000")},
		{Name: resourcecap.PrincipalBindingEmail, Handle: putPrincipalSecret(t, fixture, "email", "principal@example.invalid")},
		{Name: resourcecap.PrincipalBindingRealName, Handle: putPrincipalSecret(t, fixture, "real-name", "Test User")},
	}
	result, err := fixture.provider.CreatePrincipal(context.Background(), resourcecap.PrincipalCreateRequest{
		RequestID: "principal-create-1", SecretBindings: bindings,
	})
	if err != nil || result == nil || result.ExternalPrincipalRef != "43" || result.Validate() != nil {
		t.Fatalf("principal creation result=%+v err=%v", result, err)
	}
	if createCalls.Load() != 1 {
		t.Fatalf("principal create calls=%d, want 1", createCalls.Load())
	}
	for _, binding := range bindings {
		if fixture.secrets.readCount(binding.Handle) != 1 {
			t.Fatalf("binding %s reads=%d, want 1", binding.Name, fixture.secrets.readCount(binding.Handle))
		}
	}
}

func TestAmbiguousPrincipalCreateIsUnknownAndNeverRetried(t *testing.T) {
	t.Parallel()
	var loginCount int
	var mu sync.Mutex
	login := loginHandler(t, &loginCount, &mu, "token-principal")
	fixture := newProviderFixture(t, http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/administrator/v1/login" {
			login(writer, request)
			return
		}
		writeJSON(t, writer, http.StatusInternalServerError, `{}`)
	}))
	client := &interceptClient{
		base: fixture.client, path: "/administrator/v1/user", err: errors.New("simulated disconnect"),
	}
	fixture.provider.env.HTTPClient = client
	result, err := fixture.provider.CreatePrincipal(context.Background(), resourcecap.PrincipalCreateRequest{
		RequestID: "principal-create-unknown",
		SecretBindings: []resourcecap.SecretBinding{
			{Name: resourcecap.PrincipalBindingUsername, Handle: putPrincipalSecret(t, fixture, "username", "principal-user")},
			{Name: resourcecap.PrincipalBindingPassword, Handle: putPrincipalSecret(t, fixture, "password", "principal-pass")},
		},
	})
	var callError *resourcecap.ProviderCallError
	if result != nil || !errors.As(err, &callError) || !callError.ResultUnknown || callError.Retryable {
		t.Fatalf("ambiguous principal creation result=%+v err=%v", result, err)
	}
	if client.attempts.Load() != 1 {
		t.Fatalf("ambiguous create attempts=%d, want exactly 1", client.attempts.Load())
	}
}

func TestUnauthorizedPrincipalCreateIsUnknownAndNeverRetried(t *testing.T) {
	t.Parallel()
	var loginCount atomic.Int64
	var createCalls atomic.Int64
	fixture := newProviderFixture(t, http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/administrator/v1/login" {
			loginCount.Add(1)
			writeJSON(t, writer, http.StatusCreated, `"token-principal"`)
			return
		}
		createCalls.Add(1)
		writeJSON(t, writer, http.StatusUnauthorized, `{"error":"expired"}`)
	}))
	result, err := fixture.provider.CreatePrincipal(context.Background(), resourcecap.PrincipalCreateRequest{
		RequestID: "principal-create-unauthorized",
		SecretBindings: []resourcecap.SecretBinding{
			{Name: resourcecap.PrincipalBindingUsername, Handle: putPrincipalSecret(t, fixture, "username", "principal-user")},
			{Name: resourcecap.PrincipalBindingPassword, Handle: putPrincipalSecret(t, fixture, "password", "principal-pass")},
		},
	})
	var callError *resourcecap.ProviderCallError
	if result != nil || !errors.As(err, &callError) || !callError.ResultUnknown || callError.Retryable {
		t.Fatalf("unauthorized principal creation result=%+v err=%v", result, err)
	}
	if loginCount.Load() != 1 || createCalls.Load() != 1 {
		t.Fatalf("unauthorized principal creation login=%d create=%d", loginCount.Load(), createCalls.Load())
	}
}

func TestInvalidPrincipalBindingsFailBeforeNetwork(t *testing.T) {
	t.Parallel()
	var calls atomic.Int64
	fixture := newProviderFixture(t, http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		calls.Add(1)
		writeJSON(t, writer, http.StatusInternalServerError, `{}`)
	}))
	username := putPrincipalSecret(t, fixture, "username", "principal-user")
	for _, bindings := range [][]resourcecap.SecretBinding{
		{{Name: resourcecap.PrincipalBindingUsername, Handle: username}},
		{{Name: "unexpected", Handle: username}, {Name: resourcecap.PrincipalBindingPassword, Handle: username}},
		{{Name: resourcecap.PrincipalBindingUsername, Handle: username}, {Name: resourcecap.PrincipalBindingUsername, Handle: username}},
	} {
		result, err := fixture.provider.CreatePrincipal(context.Background(), resourcecap.PrincipalCreateRequest{
			RequestID: "principal-create-invalid", SecretBindings: bindings,
		})
		if result != nil || err == nil {
			t.Fatalf("invalid bindings result=%+v err=%v", result, err)
		}
	}
	if calls.Load() != 0 {
		t.Fatalf("invalid bindings reached network %d times", calls.Load())
	}
}

func putPrincipalSecret(t *testing.T, fixture *providerFixture, suffix string, value string) secretcap.Handle {
	t.Helper()
	handle, err := secretcap.NewHandle("sec_principal_" + suffix)
	if err != nil {
		t.Fatal(err)
	}
	fixture.secrets.put(handle, []byte(value))
	return handle
}
