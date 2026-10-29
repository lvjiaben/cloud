// This file verifies provider construction, authentication, transport, and capability contracts.

package zjmfcloud

import (
	"context"
	"crypto/x509"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"lina-plugin-idc-core/backend/cap/secretcap"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
)

type stripVerifiedChainsClient struct {
	base resourcecap.HTTPClient
}

type wrongPeerHostnameClient struct {
	base resourcecap.HTTPClient
}

func (c stripVerifiedChainsClient) Do(request *http.Request) (*http.Response, error) {
	response, err := c.base.Do(request)
	if response != nil && response.TLS != nil {
		state := *response.TLS
		state.VerifiedChains = nil
		response.TLS = &state
	}
	return response, err
}

func (c wrongPeerHostnameClient) Do(request *http.Request) (*http.Response, error) {
	response, err := c.base.Do(request)
	if response != nil && response.TLS != nil && len(response.TLS.PeerCertificates) > 0 {
		state := *response.TLS
		state.PeerCertificates = append([]*x509.Certificate(nil), response.TLS.PeerCertificates...)
		leaf := *state.PeerCertificates[0]
		leaf.DNSNames = []string{"wrong-host.invalid"}
		leaf.IPAddresses = nil
		state.PeerCertificates[0] = &leaf
		response.TLS = &state
	}
	return response, err
}

func TestParseEndpointRejectsUnsafeOrAmbiguousAddresses(t *testing.T) {
	t.Parallel()
	valid := []string{
		"https://cloud.example.com/v1",
		"https://cloud.example.com/administrator/v1/",
	}
	for _, endpoint := range valid {
		if _, err := parseEndpoint(endpoint); err != nil {
			t.Errorf("parseEndpoint(%q) rejected a valid endpoint: %v", endpoint, err)
		}
	}
	invalid := []string{
		"http://cloud.example.com/v1",
		"https://cloud.example.com",
		"https://cloud.example.com/v2",
		"https://user:pass@cloud.example.com/v1",
		"https://cloud.example.com/a/../v1",
		"https://cloud.example.com/%76%31",
		"https://cloud.example.com/v1?skip_verify=true",
		"https://cloud.example.com/v1#fragment",
	}
	for _, endpoint := range invalid {
		if _, err := parseEndpoint(endpoint); !errors.Is(err, errInvalidConfig) {
			t.Errorf("parseEndpoint(%q) error=%v, want invalid configuration", endpoint, err)
		}
	}
}

func TestParseSettingsUsesBoundedFailClosedOptions(t *testing.T) {
	t.Parallel()
	defaults, err := parseSettings([]byte(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	if defaults.TokenCacheTTL != defaultTokenCacheTTL || defaults.MaximumConcurrent != defaultMaximumConcurrent {
		t.Fatalf("unexpected defaults: %+v", defaults)
	}
	valid, err := parseSettings([]byte(`{
		"login_cache_ttl":"1m",
        "request_timeout":"1s",
        "circuit_failure_threshold":2,
        "circuit_open_duration":"1s",
        "maximum_concurrent_requests":1,
        "console_host_allowlist":["console.example.com","2001:db8::1"]
    }`))
	if err != nil || valid.TokenCacheTTL != time.Minute || valid.RequestTimeout != time.Second ||
		len(valid.ConsoleHosts) != 2 {
		t.Fatalf("parse bounded settings: %+v, %v", valid, err)
	}
	for _, raw := range [][]byte{
		[]byte(`{} true`),
		[]byte(`{"skip_tls_verify":true}`),
		[]byte(`{"follow_redirects":true}`),
		[]byte(`{"login_cache_ttl":"24h"}`),
		[]byte(`{"login_cache_ttl":"60"}`),
		[]byte(`{"request_timeout":"0s"}`),
		[]byte(`{"request_timeout":1}`),
		[]byte(`{"circuit_open_duration":"601s"}`),
		[]byte(`{"maximum_concurrent_requests":65}`),
		[]byte(`{"console_host_allowlist":["*.example.com"]}`),
		[]byte(`{"console_host_allowlist":["console.example.com:443"]}`),
		[]byte(`{"console_host_allowlist":["console.example.com","CONSOLE.EXAMPLE.COM"]}`),
	} {
		if _, err := parseSettings(raw); !errors.Is(err, errInvalidConfig) {
			t.Errorf("parseSettings(%s) error=%v, want invalid configuration", raw, err)
		}
	}
}

func TestDocumentedSettingsPassProviderContractCredentialScan(t *testing.T) {
	t.Parallel()
	username, err := secretcap.NewHandle("sec_settings_username")
	if err != nil {
		t.Fatal(err)
	}
	password, err := secretcap.NewHandle("sec_settings_password")
	if err != nil {
		t.Fatal(err)
	}
	config := resourcecap.ProviderConfig{
		ChannelRef: "channel-settings",
		Endpoint:   "https://cloud.example.com/administrator/v1",
		Settings: []byte(`{
			"login_cache_ttl":"15m",
			"request_timeout":"30s",
			"circuit_failure_threshold":5,
			"circuit_open_duration":"30s",
			"maximum_concurrent_requests":8,
			"console_host_allowlist":["console.example.com"]
		}`),
		SecretBindings: []resourcecap.SecretBinding{
			{Name: usernameBinding, Handle: username},
			{Name: passwordBinding, Handle: password},
		},
	}
	if err := config.Validate(); err != nil {
		t.Fatalf("documented settings were rejected by the Provider contract: %v", err)
	}
}

func TestJSONDecodersRejectEveryTrailingValueShape(t *testing.T) {
	t.Parallel()
	for _, raw := range [][]byte{
		[]byte(`{"id":1} true`),
		[]byte(`{"id":1} 7`),
		[]byte(`{"id":1} "tail"`),
		[]byte(`{"id":1} {}`),
	} {
		if object, err := decodeObject(raw); err == nil {
			clearRawObject(object)
			t.Errorf("decodeObject accepted trailing JSON in %q", raw)
		}
	}
	for _, raw := range [][]byte{
		[]byte(`"token" true`),
		[]byte(`"token" 7`),
		[]byte(`"token" "tail"`),
		[]byte(`"token" {}`),
	} {
		if token, err := decodeToken(raw); err == nil {
			clear(token)
			t.Errorf("decodeToken accepted trailing JSON in %q", raw)
		}
	}
	if value, ok := decodeScalar([]byte(`1 false`)); ok {
		t.Fatalf("decodeScalar accepted trailing JSON value: %v", value)
	}
}

func TestCapabilityMatrixIsCompleteAndExplicit(t *testing.T) {
	t.Parallel()
	fixture := newProviderFixture(t, http.NotFoundHandler())
	snapshot, err := fixture.provider.DiscoverCapabilities(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Capabilities) != resourcecap.MaximumCapabilityCount {
		t.Fatalf("capability count=%d", len(snapshot.Capabilities))
	}
	if err := snapshot.ValidateAt(fixture.clock.Now()); err != nil {
		t.Fatalf("validate capability snapshot: %v", err)
	}
	wantSupport := map[resourcecap.CapabilityCode]resourcecap.SupportLevel{
		resourcecap.CapabilityChangeIP:       resourcecap.SupportLevelUnsupported,
		resourcecap.CapabilityCost:           resourcecap.SupportLevelUnsupported,
		resourcecap.CapabilityBalance:        resourcecap.SupportLevelUnsupported,
		resourcecap.CapabilityExpiry:         resourcecap.SupportLevelUnsupported,
		resourcecap.CapabilityRenew:          resourcecap.SupportLevelUnsupported,
		resourcecap.CapabilityResetPassword:  resourcecap.SupportLevelSupported,
		resourcecap.CapabilityConsole:        resourcecap.SupportLevelSupported,
		resourcecap.CapabilityReinstall:      resourcecap.SupportLevelConditional,
		resourcecap.CapabilityEnterRescue:    resourcecap.SupportLevelConditional,
		resourcecap.CapabilityExitRescue:     resourcecap.SupportLevelSupported,
		resourcecap.CapabilityManageSnapshot: resourcecap.SupportLevelConditional,
		resourcecap.CapabilityBackup:         resourcecap.SupportLevelConditional,
		resourcecap.CapabilitySecurityGroup:  resourcecap.SupportLevelConditional,
		resourcecap.CapabilityNAT:            resourcecap.SupportLevelConditional,
		resourcecap.CapabilityVirtualMedia:   resourcecap.SupportLevelConditional,
		resourcecap.CapabilityTrafficPackage: resourcecap.SupportLevelConditional,
		resourcecap.CapabilityUpstreamTicket: resourcecap.SupportLevelUnsupported,
	}
	for _, descriptor := range snapshot.Capabilities {
		if want, exists := wantSupport[descriptor.Code]; exists {
			if descriptor.Support != want {
				t.Errorf("capability %s support=%s, want %s", descriptor.Code, descriptor.Support, want)
			}
			delete(wantSupport, descriptor.Code)
		}
	}
	for code := range wantSupport {
		t.Errorf("capability %s was absent from the matrix", code)
	}
}

func TestUpstreamTicketIsExplicitlyUnsupportedWithoutNetworkAccess(t *testing.T) {
	t.Parallel()
	var requests atomic.Int64
	fixture := newProviderFixture(t, http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		requests.Add(1)
	}))
	snapshot, err := fixture.provider.DiscoverCapabilities(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, descriptor := range snapshot.Capabilities {
		if descriptor.Code != resourcecap.CapabilityUpstreamTicket {
			continue
		}
		if descriptor.Support != resourcecap.SupportLevelUnsupported ||
			descriptor.UnsupportedReason != upstreamTicketUnsupportedReason {
			t.Fatalf("unexpected upstream-ticket descriptor: %+v", descriptor)
		}
		request := resourcecap.OperationRequest{
			RequestID: "request-upstream-ticket-unsupported", Capability: descriptor.Code,
			ResourceRef: "41", Parameters: []byte(`{}`),
		}
		if _, executeErr := fixture.provider.Execute(context.Background(), request); !errors.Is(executeErr, errUnsupported) {
			t.Fatalf("Execute error=%v, want unsupported", executeErr)
		}
		if requests.Load() != 0 {
			t.Fatalf("unsupported ticket action made %d network requests", requests.Load())
		}
		return
	}
	t.Fatal("upstream-ticket capability was absent")
}

func TestNATCapabilityConstraintsMatchAcceptedParameterShape(t *testing.T) {
	t.Parallel()
	fixture := newProviderFixture(t, http.NotFoundHandler())
	snapshot, err := fixture.provider.DiscoverCapabilities(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, descriptor := range snapshot.Capabilities {
		if descriptor.Code != resourcecap.CapabilityNAT {
			continue
		}
		if descriptor.Support != resourcecap.SupportLevelConditional || len(descriptor.Constraints) != 6 {
			t.Fatalf("unexpected NAT descriptor: %+v", descriptor)
		}
		wantNames := []string{"action", "nat_ref", "name", "external_port", "internal_port", "protocol"}
		for index, want := range wantNames {
			if descriptor.Constraints[index].Name != want {
				t.Fatalf("constraint %d name=%s, want %s", index, descriptor.Constraints[index].Name, want)
			}
		}
		if !descriptor.Constraints[0].Required ||
			strings.Join(descriptor.Constraints[0].AllowedValues, ",") != "create,delete" ||
			strings.Join(descriptor.Constraints[5].AllowedValues, ",") != "tcp,udp,tcp_udp" {
			t.Fatalf("unexpected NAT action/protocol constraints: %+v", descriptor.Constraints)
		}
		return
	}
	t.Fatal("NAT capability was absent")
}

func TestVirtualMediaCapabilityConstraintsMatchAcceptedParameterShape(t *testing.T) {
	t.Parallel()
	fixture := newProviderFixture(t, http.NotFoundHandler())
	snapshot, err := fixture.provider.DiscoverCapabilities(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, descriptor := range snapshot.Capabilities {
		if descriptor.Code != resourcecap.CapabilityVirtualMedia {
			continue
		}
		if descriptor.Support != resourcecap.SupportLevelConditional || len(descriptor.Constraints) != 2 ||
			descriptor.Constraints[0].Name != "action" || !descriptor.Constraints[0].Required ||
			strings.Join(descriptor.Constraints[0].AllowedValues, ",") != "mount,unmount" ||
			descriptor.Constraints[1].Name != "image_ref" || descriptor.Constraints[1].Required ||
			descriptor.Constraints[1].Minimum == nil || *descriptor.Constraints[1].Minimum != 1 {
			t.Fatalf("unexpected virtual-media descriptor: %+v", descriptor)
		}
		return
	}
	t.Fatal("virtual-media capability was absent")
}

func TestTrafficPackageCapabilityConstraintsMatchAcceptedParameterShape(t *testing.T) {
	t.Parallel()
	fixture := newProviderFixture(t, http.NotFoundHandler())
	snapshot, err := fixture.provider.DiscoverCapabilities(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, descriptor := range snapshot.Capabilities {
		if descriptor.Code != resourcecap.CapabilityTrafficPackage {
			continue
		}
		if descriptor.Support != resourcecap.SupportLevelConditional || len(descriptor.Constraints) != 2 ||
			descriptor.Constraints[0].Name != "action" || !descriptor.Constraints[0].Required ||
			strings.Join(descriptor.Constraints[0].AllowedValues, ",") != "add" ||
			descriptor.Constraints[1].Name != "traffic_gb" || !descriptor.Constraints[1].Required ||
			descriptor.Constraints[1].Minimum == nil || *descriptor.Constraints[1].Minimum != 1 ||
			descriptor.Constraints[1].Maximum == nil || *descriptor.Constraints[1].Maximum != maximumTrafficPackageGB {
			t.Fatalf("unexpected traffic-package descriptor: %+v", descriptor)
		}
		return
	}
	t.Fatal("traffic-package capability was absent")
}

func TestSecurityGroupCapabilityConstraintsMatchAcceptedParameterShape(t *testing.T) {
	t.Parallel()
	fixture := newProviderFixture(t, http.NotFoundHandler())
	snapshot, err := fixture.provider.DiscoverCapabilities(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, descriptor := range snapshot.Capabilities {
		if descriptor.Code != resourcecap.CapabilitySecurityGroup {
			continue
		}
		if descriptor.Support != resourcecap.SupportLevelConditional || len(descriptor.Constraints) != 2 {
			t.Fatalf("unexpected security-group descriptor: %+v", descriptor)
		}
		if descriptor.Constraints[0].Name != "action" || !descriptor.Constraints[0].Required ||
			strings.Join(descriptor.Constraints[0].AllowedValues, ",") != "attach,detach" {
			t.Fatalf("unexpected action constraint: %+v", descriptor.Constraints[0])
		}
		if descriptor.Constraints[1].Name != "security_group_ref" || descriptor.Constraints[1].Required ||
			descriptor.Constraints[1].Minimum == nil || *descriptor.Constraints[1].Minimum != 1 {
			t.Fatalf("unexpected group constraint: %+v", descriptor.Constraints[1])
		}
		return
	}
	t.Fatal("security-group capability was absent")
}

func TestSnapshotCapabilityConstraintsMatchAcceptedParameterShape(t *testing.T) {
	t.Parallel()
	fixture := newProviderFixture(t, http.NotFoundHandler())
	snapshot, err := fixture.provider.DiscoverCapabilities(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, code := range []resourcecap.CapabilityCode{
		resourcecap.CapabilityManageSnapshot, resourcecap.CapabilityBackup,
	} {
		var descriptor *resourcecap.CapabilityDescriptor
		for index := range snapshot.Capabilities {
			if snapshot.Capabilities[index].Code == code {
				descriptor = &snapshot.Capabilities[index]
				break
			}
		}
		if descriptor == nil {
			t.Fatalf("capability %s was absent", code)
		}
		want := map[string]bool{"action": true, "disk_ref": false, "snapshot_ref": false, "name": false}
		for _, constraint := range descriptor.Constraints {
			required, exists := want[constraint.Name]
			if !exists || constraint.Required != required {
				t.Errorf("capability %s unexpected constraint %+v", code, constraint)
			}
			delete(want, constraint.Name)
			if constraint.Name == "action" && strings.Join(constraint.AllowedValues, ",") != "create,delete" {
				t.Errorf("capability %s actions=%v", code, constraint.AllowedValues)
			}
		}
		if len(want) != 0 {
			t.Errorf("capability %s omitted constraints %v", code, want)
		}
	}
}

func TestResizeCapabilityConstraintsMatchAcceptedParameterShape(t *testing.T) {
	t.Parallel()
	fixture := newProviderFixture(t, http.NotFoundHandler())
	snapshot, err := fixture.provider.DiscoverCapabilities(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]struct{}{
		"cpu": {}, "memory": {}, "in_bw": {}, "out_bw": {}, "system_disk_size": {},
	}
	for _, descriptor := range snapshot.Capabilities {
		if descriptor.Code != resourcecap.CapabilityResize {
			continue
		}
		for _, constraint := range descriptor.Constraints {
			if constraint.Required {
				t.Errorf("resize constraint %s unexpectedly claimed to be individually required", constraint.Name)
			}
			if _, exists := want[constraint.Name]; !exists {
				t.Errorf("resize advertised unsupported parameter %s", constraint.Name)
			} else {
				delete(want, constraint.Name)
			}
		}
	}
	for name := range want {
		t.Errorf("resize omitted accepted parameter %s", name)
	}
}

func TestUnverifiedTLSResponseRetainsEndpointPolicy(t *testing.T) {
	t.Parallel()
	var loginCount int
	var mu sync.Mutex
	login := loginHandler(t, &loginCount, &mu, "token-one")
	fixture := newProviderFixture(t, http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/administrator/v1/login" {
			login(writer, request)
			return
		}
		writeJSON(t, writer, http.StatusOK, `{"data":[],"meta":{"page":1,"total_page":1}}`)
	}))
	fixture.provider.env.HTTPClient = stripVerifiedChainsClient{base: fixture.client}
	_, err := fixture.provider.ListProviderCapacity(context.Background(), resourcecap.ProviderCapacityRequest{
		Kind: resourcecap.ProviderCapacityKindRegion, Limit: 10,
	})
	if err != nil {
		t.Fatalf("certificate verification unexpectedly blocked configured endpoint: %v", err)
	}
}

func TestCertificateHostnameDoesNotOverrideConfiguredEndpoint(t *testing.T) {
	t.Parallel()
	fixture := newProviderFixture(t, http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/administrator/v1/login" {
			writeJSON(t, writer, http.StatusCreated, `"token-one"`)
			return
		}
		writeJSON(t, writer, http.StatusOK, `{"data":[],"meta":{"page":1,"total_page":1}}`)
	}))
	fixture.provider.env.HTTPClient = wrongPeerHostnameClient{base: fixture.client}
	_, err := fixture.provider.ListProviderCapacity(context.Background(), resourcecap.ProviderCapacityRequest{
		Kind: resourcecap.ProviderCapacityKindRegion, Limit: 10,
	})
	if err != nil {
		t.Fatalf("certificate hostname unexpectedly blocked configured endpoint: %v", err)
	}
}

func TestFreshProviderInstancesShareOneAuthenticationSession(t *testing.T) {
	t.Parallel()
	var loginCount int
	var loginMu sync.Mutex
	login := loginHandler(t, &loginCount, &loginMu, "token-shared")
	fixture := newProviderFixture(t, http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/administrator/v1/login":
			login(writer, request)
		case "/administrator/v1/areas":
			if request.Header.Get("Access-Token") != "token-shared" {
				t.Error("providerCapacity request did not use the shared token")
			}
			writeJSON(t, writer, http.StatusOK, `{"data":[],"meta":{"page":1,"total_page":1}}`)
		default:
			writeJSON(t, writer, http.StatusNotFound, `{}`)
		}
	}))
	environment := fixture.provider.env
	configuration := resourcecap.ProviderConfig{
		ChannelRef: "channel-fixture",
		Endpoint:   fixture.server.URL + "/administrator/v1",
		Settings:   []byte(`{}`),
		SecretBindings: []resourcecap.SecretBinding{
			{Name: usernameBinding, Handle: fixture.provider.username},
			{Name: passwordBinding, Handle: fixture.provider.password},
		},
	}
	created, err := New(context.Background(), environment, configuration)
	if err != nil {
		t.Fatalf("construct second Provider instance: %v", err)
	}
	providers := []*Provider{fixture.provider, created.(*Provider)}
	var workers sync.WaitGroup
	errorsFound := make(chan error, len(providers))
	for _, provider := range providers {
		workers.Add(1)
		go func(current *Provider) {
			defer workers.Done()
			_, callErr := current.ListProviderCapacity(context.Background(), resourcecap.ProviderCapacityRequest{
				Kind:  resourcecap.ProviderCapacityKindRegion,
				Limit: 10,
			})
			errorsFound <- callErr
		}(provider)
	}
	workers.Wait()
	close(errorsFound)
	for callErr := range errorsFound {
		if callErr != nil {
			t.Fatalf("shared-session providerCapacity call failed: %v", callErr)
		}
	}
	loginMu.Lock()
	defer loginMu.Unlock()
	if loginCount != 1 {
		t.Fatalf("fresh Provider instances performed %d logins, want 1", loginCount)
	}
}

func TestRedirectIsNotFollowedAndTokenIsNotForwarded(t *testing.T) {
	t.Parallel()
	var redirectedRequests atomic.Int64
	target := httptest.NewTLSServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		redirectedRequests.Add(1)
	}))
	t.Cleanup(target.Close)

	var loginCount int
	var mu sync.Mutex
	login := loginHandler(t, &loginCount, &mu, "token-one")
	fixture := newProviderFixture(t, http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/administrator/v1/login" {
			login(writer, request)
			return
		}
		writer.Header().Set("Location", target.URL+"/capture")
		writer.WriteHeader(http.StatusFound)
	}))
	_, err := fixture.provider.ListProviderCapacity(context.Background(), resourcecap.ProviderCapacityRequest{
		Kind: resourcecap.ProviderCapacityKindRegion, Limit: 10,
	})
	var callError *resourcecap.ProviderCallError
	if !errors.As(err, &callError) || callError.Code != resourcecap.ProviderCallErrorCodeInvalidResponse ||
		callError.Retryable || callError.ResultUnknown {
		t.Fatalf("ListProviderCapacity error=%v, want non-retryable invalid response", err)
	}
	if redirectedRequests.Load() != 0 {
		t.Fatalf("redirect target received %d requests", redirectedRequests.Load())
	}
}

func TestProviderOverridesPermissiveRedirectPolicyBeforeCredentialUse(t *testing.T) {
	t.Parallel()
	var redirectedRequests atomic.Int64
	target := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		redirectedRequests.Add(1)
		if err := request.ParseForm(); err != nil {
			t.Errorf("parse redirected login form: %v", err)
		}
		if request.Form.Get("username") != "" || request.Form.Get("password") != "" {
			t.Error("provider credentials reached the redirect target")
		}
		writeJSON(t, writer, http.StatusCreated, `"stolen-token"`)
	}))
	t.Cleanup(target.Close)

	fixture := newProviderFixture(t, http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/administrator/v1/login" {
			writer.Header().Set("Location", target.URL+"/capture")
			writer.WriteHeader(http.StatusTemporaryRedirect)
			return
		}
		writeJSON(t, writer, http.StatusNotFound, `{}`)
	}))
	permissive := *fixture.client
	permissive.CheckRedirect = func(_ *http.Request, _ []*http.Request) error { return nil }
	env := fixture.provider.env
	env.HTTPClient = &permissive
	created, err := New(context.Background(), env, resourcecap.ProviderConfig{
		ChannelRef: "channel-permissive-redirect",
		Endpoint:   fixture.server.URL + "/administrator/v1",
		Settings:   []byte(`{}`),
		SecretBindings: []resourcecap.SecretBinding{
			{Name: usernameBinding, Handle: fixture.provider.username},
			{Name: passwordBinding, Handle: fixture.provider.password},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = created.ListProviderCapacity(context.Background(), resourcecap.ProviderCapacityRequest{
		Kind: resourcecap.ProviderCapacityKindRegion, Limit: 10,
	})
	if err == nil {
		t.Fatal("redirected authentication unexpectedly succeeded")
	}
	if redirectedRequests.Load() != 0 {
		t.Fatalf("permissive injected client followed %d redirect requests", redirectedRequests.Load())
	}
}

func TestCircuitBreakerStopsCallsUntilTheOpenWindowExpires(t *testing.T) {
	t.Parallel()
	var loginCount int
	var mu sync.Mutex
	login := loginHandler(t, &loginCount, &mu, "token-one")
	var upstreamCalls atomic.Int64
	fixture := newProviderFixture(t, http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/administrator/v1/login" {
			login(writer, request)
			return
		}
		upstreamCalls.Add(1)
		writeJSON(t, writer, http.StatusServiceUnavailable, `{"error":"temporary"}`)
	}))
	request := resourcecap.ProviderCapacityRequest{Kind: resourcecap.ProviderCapacityKindRegion, Limit: 10}
	for index := 0; index < defaultCircuitFailures; index++ {
		if _, err := fixture.provider.ListProviderCapacity(context.Background(), request); err == nil {
			t.Fatalf("failure %d unexpectedly succeeded", index+1)
		}
	}
	if _, err := fixture.provider.ListProviderCapacity(context.Background(), request); err == nil {
		t.Fatal("open circuit unexpectedly accepted a call")
	} else {
		var callError *resourcecap.ProviderCallError
		if !errors.As(err, &callError) || callError.Code != resourcecap.ProviderCallErrorCodeTransport ||
			!callError.Retryable || callError.ResultUnknown {
			t.Fatalf("open circuit error=%v, want retryable transport classification", err)
		}
	}
	if upstreamCalls.Load() != int64(defaultCircuitFailures) {
		t.Fatalf("upstream calls=%d, want %d", upstreamCalls.Load(), defaultCircuitFailures)
	}
	fixture.clock.Advance(defaultCircuitOpenDuration + time.Second)
	if _, err := fixture.provider.ListProviderCapacity(context.Background(), request); err == nil {
		t.Fatal("reopened probe unexpectedly succeeded")
	}
	if upstreamCalls.Load() != int64(defaultCircuitFailures+1) {
		t.Fatalf("upstream calls after reopen=%d", upstreamCalls.Load())
	}
}

func TestOpenCircuitBlocksWritesButPreservesReadOnlyReconciliation(t *testing.T) {
	t.Parallel()
	var (
		loginCount  int
		loginMu     sync.Mutex
		failReads   atomic.Bool
		actionCalls atomic.Int64
	)
	failReads.Store(true)
	login := loginHandler(t, &loginCount, &loginMu, "token-one")
	fixture := newProviderFixture(t, http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/administrator/v1/login":
			login(writer, request)
		case "/administrator/v1/areas":
			if failReads.Load() {
				writeJSON(t, writer, http.StatusServiceUnavailable, `{"error":"temporary"}`)
				return
			}
			writeJSON(t, writer, http.StatusOK, `{"data":[],"meta":{"page":1,"total_page":1}}`)
		case "/administrator/v1/tasks/7001":
			writeJSON(t, writer, http.StatusOK,
				`{"data":{"id":7001,"hostid":41,"status":1,"progress":25}}`)
		case "/administrator/v1/clouds/41/on":
			actionCalls.Add(1)
			writeJSON(t, writer, http.StatusAccepted, `{"taskid":7002}`)
		default:
			t.Errorf("unexpected request %s %s", request.Method, request.URL.Path)
			writeJSON(t, writer, http.StatusNotFound, `{"error":"unexpected"}`)
		}
	}))
	providerCapacity := resourcecap.ProviderCapacityRequest{Kind: resourcecap.ProviderCapacityKindRegion, Limit: 10}
	for index := 0; index < defaultCircuitFailures; index++ {
		if _, err := fixture.provider.ListProviderCapacity(context.Background(), providerCapacity); err == nil {
			t.Fatalf("failure %d unexpectedly succeeded", index+1)
		}
	}
	failReads.Store(false)
	observed, err := fixture.provider.QueryTask(context.Background(), resourcecap.TaskQuery{
		ExternalTaskRef: "7001", ResourceRef: "41",
	})
	if err != nil || observed == nil || observed.Status != resourcecap.TaskStatusRunning {
		t.Fatalf("open circuit blocked read-only reconciliation: result=%+v err=%v", observed, err)
	}
	result, err := fixture.provider.Execute(context.Background(), resourcecap.OperationRequest{
		RequestID: "request-circuit-write", Capability: resourcecap.CapabilityStart,
		ResourceRef: "41", Parameters: []byte(`{}`),
	})
	if result != nil || err == nil || actionCalls.Load() != 0 {
		t.Fatalf("open circuit allowed write: result=%+v err=%v calls=%d", result, err, actionCalls.Load())
	}
	var callError *resourcecap.ProviderCallError
	if !errors.As(err, &callError) || callError.Code != resourcecap.ProviderCallErrorCodeTransport ||
		!callError.Retryable || callError.ResultUnknown {
		t.Fatalf("blocked write error=%v, want retryable known non-acceptance", err)
	}
	fixture.clock.Advance(defaultCircuitOpenDuration + time.Second)
	result, err = fixture.provider.Execute(context.Background(), resourcecap.OperationRequest{
		RequestID: "request-circuit-probe", Capability: resourcecap.CapabilityStart,
		ResourceRef: "41", Parameters: []byte(`{}`),
	})
	if err != nil || result == nil || result.Status != resourcecap.TaskStatusAccepted ||
		actionCalls.Load() != 1 {
		t.Fatalf("expired circuit did not admit one write probe: result=%+v err=%v calls=%d",
			result, err, actionCalls.Load())
	}
}
