// This file provides isolated TLS fixtures and deterministic provider dependencies.

package zjmfcloud

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"testing"
	"time"

	"lina-plugin-idc-core/backend/cap/secretcap"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
)

type fakeSecretAccessor struct {
	mu     sync.Mutex
	values map[string][]byte
	reads  map[string]int
}

type fakeSecretSink struct {
	mu       sync.Mutex
	values   map[string][]byte
	requests []resourcecap.SecretStoreRequest
	next     int
	fail     bool
}

func (s *fakeSecretSink) Store(
	_ context.Context,
	request resourcecap.SecretStoreRequest,
	produce resourcecap.SecretProducer,
) (secretcap.Handle, error) {
	if s.fail {
		return secretcap.Handle{}, errInvalidResponse
	}
	var plaintext []byte
	if err := produce(func(value []byte) error {
		plaintext = append([]byte(nil), value...)
		return nil
	}); err != nil {
		clear(plaintext)
		return secretcap.Handle{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.next++
	handle, err := secretcap.NewHandle("sec_zjmf_output_" + strconv.Itoa(s.next))
	if err != nil {
		clear(plaintext)
		return secretcap.Handle{}, err
	}
	if s.values == nil {
		s.values = make(map[string][]byte)
	}
	s.values[handle.OpaqueRef()] = plaintext
	s.requests = append(s.requests, request)
	return handle, nil
}

func (s *fakeSecretSink) load(handle secretcap.Handle) ([]byte, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	value, ok := s.values[handle.OpaqueRef()]
	return append([]byte(nil), value...), ok
}

func (s *fakeSecretSink) requestSnapshot() []resourcecap.SecretStoreRequest {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]resourcecap.SecretStoreRequest(nil), s.requests...)
}

func (a *fakeSecretAccessor) Access(
	_ context.Context,
	request secretcap.AccessRequest,
	consume secretcap.Consumer,
) error {
	if err := secretcap.ValidateAccess(request, consume); err != nil {
		return err
	}
	a.mu.Lock()
	value := append([]byte(nil), a.values[request.Handle.OpaqueRef()]...)
	a.reads[request.Handle.OpaqueRef()]++
	a.mu.Unlock()
	defer clear(value)
	return consume(value)
}

func (a *fakeSecretAccessor) put(handle secretcap.Handle, value []byte) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.values[handle.OpaqueRef()] = append([]byte(nil), value...)
}

func (a *fakeSecretAccessor) readCount(handle secretcap.Handle) int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.reads[handle.OpaqueRef()]
}

type fixedClock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *fixedClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *fixedClock) Advance(duration time.Duration) {
	c.mu.Lock()
	c.now = c.now.Add(duration)
	c.mu.Unlock()
}

type discardLogger struct{}

func (discardLogger) Log(context.Context, resourcecap.LogLevel, string, ...resourcecap.LogField) {}

type discardMetrics struct{}

func (discardMetrics) AddCounter(context.Context, string, int64, ...resourcecap.MetricLabel) {}
func (discardMetrics) Observe(context.Context, string, float64, ...resourcecap.MetricLabel)  {}

type fakeAuthenticationSessions struct {
	mu         sync.Mutex
	clock      *fixedClock
	token      []byte
	generation int64
	expiresAt  time.Time
}

func (s *fakeAuthenticationSessions) Access(
	_ context.Context,
	request resourcecap.AuthenticationSessionRequest,
	produce resourcecap.AuthenticationTokenProducer,
	consume resourcecap.AuthenticationTokenConsumer,
) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.token) == 0 || !s.clock.Now().Before(s.expiresAt) {
		var produced []byte
		if err := produce(func(value []byte) error {
			produced = append([]byte(nil), value...)
			return nil
		}); err != nil {
			return err
		}
		clear(s.token)
		s.token = produced
		s.generation++
		s.expiresAt = s.clock.Now().Add(request.TTL)
	}
	return consume(s.token, s.generation)
}

func (s *fakeAuthenticationSessions) Invalidate(
	_ context.Context,
	request resourcecap.AuthenticationSessionInvalidation,
) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if request.Generation == s.generation {
		clear(s.token)
		s.token = nil
		s.expiresAt = time.Time{}
	}
	return nil
}

type providerFixture struct {
	server   *httptest.Server
	client   *http.Client
	clock    *fixedClock
	secrets  *fakeSecretAccessor
	sink     *fakeSecretSink
	provider *Provider
}

func newProviderFixture(t *testing.T, handler http.Handler) *providerFixture {
	t.Helper()
	server := httptest.NewUnstartedServer(handler)
	server.StartTLS()
	t.Cleanup(server.Close)

	pool := x509.NewCertPool()
	pool.AddCert(server.Certificate())
	client := &http.Client{
		Transport: &http.Transport{TLSClientConfig: &tls.Config{
			MinVersion: tls.VersionTLS12,
			RootCAs:    pool,
		}},
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	t.Cleanup(client.CloseIdleConnections)

	clock := &fixedClock{now: time.Date(2026, 8, 6, 8, 0, 0, 0, time.UTC)}
	username, err := secretcap.NewHandle("sec_zjmf_username")
	if err != nil {
		t.Fatal(err)
	}
	password, err := secretcap.NewHandle("sec_zjmf_password")
	if err != nil {
		t.Fatal(err)
	}
	secrets := &fakeSecretAccessor{
		values: map[string][]byte{
			username.OpaqueRef(): []byte("merchant-user"),
			password.OpaqueRef(): []byte("merchant-pass"),
		},
		reads: map[string]int{},
	}
	sink := &fakeSecretSink{values: map[string][]byte{}}
	authSessions := &fakeAuthenticationSessions{clock: clock}
	env := resourcecap.ProviderEnv{
		PluginID:               "idc-zjmfcloud",
		ProviderID:             ProviderID,
		Secrets:                secrets,
		SecretSink:             sink,
		AuthenticationSessions: authSessions,
		HTTPClient:             client,
		Clock:                  clock,
		Logger:                 discardLogger{},
		Metrics:                discardMetrics{},
	}
	config := resourcecap.ProviderConfig{
		ChannelRef: "channel-fixture",
		Endpoint:   server.URL + "/administrator/v1",
		Settings:   []byte(`{}`),
		SecretBindings: []resourcecap.SecretBinding{
			{Name: usernameBinding, Handle: username},
			{Name: passwordBinding, Handle: password},
		},
	}
	created, err := New(context.Background(), env, config)
	if err != nil {
		t.Fatalf("construct provider: %v", err)
	}
	return &providerFixture{
		server: server, client: client, clock: clock, secrets: secrets, sink: sink,
		provider: created.(*Provider),
	}
}

func writeJSON(t *testing.T, writer http.ResponseWriter, status int, body string) {
	t.Helper()
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(status)
	if _, err := writer.Write([]byte(body)); err != nil {
		t.Errorf("write response: %v", err)
	}
}

func loginHandler(t *testing.T, counter *int, mu *sync.Mutex, token string) http.HandlerFunc {
	t.Helper()
	return func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/administrator/v1/login" || request.URL.RawQuery != "" {
			t.Errorf("unexpected login URL %s", request.URL.RequestURI())
			writeJSON(t, writer, http.StatusNotFound, `{"error":"not found"}`)
			return
		}
		if err := request.ParseForm(); err != nil {
			t.Errorf("parse login form: %v", err)
		}
		if request.Form.Get("username") != "merchant-user" || request.Form.Get("password") != "merchant-pass" {
			t.Error("login did not receive the scoped fixture credentials")
		}
		mu.Lock()
		(*counter)++
		mu.Unlock()
		writeJSON(t, writer, http.StatusCreated, strconv.Quote(token))
	}
}
