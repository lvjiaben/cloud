// This file verifies signed payloads and public-network-only endpoint enforcement.

package signedwebhook

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"lina-plugin-idc-core/backend/cap/secretcap"
	"lina-plugin-idc-notify/backend/cap/channelcap"
)

type accessorStub struct{ payload []byte }

func (stub accessorStub) Access(_ context.Context, request secretcap.AccessRequest, consume secretcap.Consumer) error {
	if err := secretcap.ValidateAccess(request, consume); err != nil {
		return err
	}
	value := append([]byte(nil), stub.payload...)
	defer clear(value)
	return consume(value)
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (function roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return function(request)
}

type resolverStub struct {
	addresses []net.IPAddr
	err       error
}

func (stub resolverStub) LookupIPAddr(context.Context, string) ([]net.IPAddr, error) {
	return append([]net.IPAddr(nil), stub.addresses...), stub.err
}

type dialerStub struct {
	addresses []string
}

func (stub *dialerStub) DialContext(_ context.Context, _, address string) (net.Conn, error) {
	stub.addresses = append(stub.addresses, address)
	client, server := net.Pipe()
	_ = server.Close()
	return client, nil
}

func allowTestEndpoint(context.Context, *url.URL) error { return nil }

func TestDeliverSignsMinimalStableEventEnvelope(t *testing.T) {
	secret := []byte("0123456789abcdef0123456789abcdef")
	credentialPayload, err := json.Marshal(map[string][]byte{signingKeyField: secret})
	if err != nil {
		t.Fatalf("encode test credential: %v", err)
	}
	var bodies [][]byte
	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		body, readErr := io.ReadAll(request.Body)
		if readErr != nil {
			t.Fatalf("read signed webhook body: %v", readErr)
		}
		bodies = append(bodies, body)
		var envelope map[string]any
		if err := json.Unmarshal(body, &envelope); err != nil {
			t.Fatalf("decode signed webhook body: %v", err)
		}
		if len(envelope) != 4 || envelope["eventNo"] != "nchain_1" || envelope["type"] != "support.ticket_updated" {
			t.Fatalf("webhook payload is not the frozen minimal envelope: %s", body)
		}
		hash := hmac.New(sha256.New, secret)
		_, _ = hash.Write(body)
		expected := "sha256=" + hex.EncodeToString(hash.Sum(nil))
		if request.Header.Get("X-Lina-Signature") != expected ||
			request.Header.Get("X-Lina-Event-No") != "nchain_1" ||
			request.Header.Get("X-Lina-Signature-Version") != signatureVersion {
			t.Fatalf("webhook signature headers are invalid: %+v", request.Header)
		}
		return &http.Response{StatusCode: http.StatusNoContent, Body: io.NopCloser(strings.NewReader("")), Header: make(http.Header)}, nil
	})}
	adapter, err := newAdapter(accessorStub{payload: credentialPayload}, client, allowTestEndpoint)
	if err != nil {
		t.Fatalf("construct signed webhook adapter: %v", err)
	}
	handle, err := secretcap.NewHandle("opaque-webhook-test-handle")
	if err != nil {
		t.Fatalf("construct test secret handle: %v", err)
	}
	input := channelcap.Delivery{
		TaskNo: "ntask_1", EventNo: "nchain_1", EventType: "support.ticket_updated",
		EventVersion: 3, OccurredAt: time.UnixMilli(1_700_000_000_123).UTC(),
		Credential: handle, NonSecretConfig: map[string]string{endpointField: "https://example.test/hooks/idc"},
	}
	for attempt := 0; attempt < 2; attempt++ {
		input.IdempotencyKey = "ntask_1:" + string(rune('0'+attempt))
		result, deliverErr := adapter.Deliver(t.Context(), input)
		if deliverErr != nil || result.Class != channelcap.ResultDelivered || !result.CrossedExternalBorder {
			t.Fatalf("deliver signed webhook attempt %d: result=%+v err=%v", attempt, result, deliverErr)
		}
	}
	if len(bodies) != 2 || string(bodies[0]) != string(bodies[1]) {
		t.Fatalf("repeated delivery changed the event envelope: %q != %q", bodies[0], bodies[1])
	}
}

func TestNonSuccessResponseRemainsUnknownAndNeverClaimsNotReceived(t *testing.T) {
	secret := []byte("0123456789abcdef0123456789abcdef")
	payload, _ := json.Marshal(map[string][]byte{signingKeyField: secret})
	client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusServiceUnavailable, Body: io.NopCloser(strings.NewReader("busy")), Header: make(http.Header)}, nil
	})}
	adapter, err := newAdapter(accessorStub{payload: payload}, client, allowTestEndpoint)
	if err != nil {
		t.Fatalf("construct signed webhook adapter: %v", err)
	}
	handle, _ := secretcap.NewHandle("opaque-webhook-test-handle")
	result, err := adapter.Deliver(t.Context(), channelcap.Delivery{
		TaskNo: "ntask_1", EventNo: "nchain_1", EventType: "support.ticket_updated",
		EventVersion: 1, OccurredAt: time.Now().UTC(), Credential: handle,
		NonSecretConfig: map[string]string{endpointField: "https://example.test/hook"},
	})
	if err != nil || result.Class != channelcap.ResultUnknown || !result.CrossedExternalBorder {
		t.Fatalf("ambiguous webhook response was not kept unknown: result=%+v err=%v", result, err)
	}
}

func TestEndpointPolicyRejectsHTTPAndNonPublicAddresses(t *testing.T) {
	if _, err := parseEndpoint("http://example.com/hook"); err == nil {
		t.Fatal("plain HTTP endpoint was accepted")
	}
	for _, host := range []string{
		"127.0.0.1", "10.0.0.1", "100.64.0.1", "169.254.169.254", "192.0.2.1", "192.31.196.1",
		"192.52.193.1", "192.88.99.1", "192.175.48.1", "198.18.0.1", "203.0.113.1", "::1",
		"64:ff9b::a00:1", "64:ff9b:1::1", "100:0:0:1::1", "2001::1", "2001:db8::1", "2002:a00:1::",
		"2620:4f:8000::1", "3fff::1", "5f00::1", "fc00::1",
	} {
		if _, err := resolvePublicAddresses(t.Context(), resolverStub{}, host); err == nil {
			t.Fatalf("non-public webhook address %s was accepted", host)
		}
	}
}

func TestMixedDNSAnswerRejectsWholeConnectionBeforeDial(t *testing.T) {
	resolver := resolverStub{addresses: []net.IPAddr{{IP: net.ParseIP("93.184.216.34")}, {IP: net.ParseIP("10.0.0.7")}}}
	dialer := &dialerStub{}
	validated := &validatedDialer{resolver: resolver, dialer: dialer}
	if _, err := validated.DialContext(t.Context(), "tcp", "hooks.example.com:443"); err == nil {
		t.Fatal("mixed public/private DNS result was accepted")
	}
	if len(dialer.addresses) != 0 {
		t.Fatalf("unsafe DNS result reached the network dialer: %v", dialer.addresses)
	}
}

func TestValidatedDialerPinsConnectionToResolvedPublicAddress(t *testing.T) {
	resolver := resolverStub{addresses: []net.IPAddr{{IP: net.ParseIP("93.184.216.34")}}}
	dialer := &dialerStub{}
	validated := &validatedDialer{resolver: resolver, dialer: dialer}
	connection, err := validated.DialContext(t.Context(), "tcp", "hooks.example.com:443")
	if err != nil {
		t.Fatalf("dial validated public address: %v", err)
	}
	_ = connection.Close()
	if len(dialer.addresses) != 1 || dialer.addresses[0] != "93.184.216.34:443" {
		t.Fatalf("connection was not pinned to the validated address: %v", dialer.addresses)
	}
}

func TestProductionClientDisablesRedirectsAndKeepsTLSVerification(t *testing.T) {
	payload, _ := json.Marshal(map[string][]byte{signingKeyField: []byte("0123456789abcdef0123456789abcdef")})
	adapter, err := newProductionAdapter(accessorStub{payload: payload}, resolverStub{addresses: []net.IPAddr{{IP: net.ParseIP("93.184.216.34")}}}, &dialerStub{})
	if err != nil {
		t.Fatalf("construct production adapter: %v", err)
	}
	transport, ok := adapter.client.Transport.(*http.Transport)
	if !ok || transport.TLSClientConfig == nil || transport.TLSClientConfig.InsecureSkipVerify || transport.TLSClientConfig.MinVersion < 0x0303 {
		t.Fatal("production webhook transport does not enforce verified TLS 1.2+")
	}
	request, _ := http.NewRequest(http.MethodGet, "https://other.example/hook", nil)
	if err = adapter.client.CheckRedirect(request, nil); err != http.ErrUseLastResponse {
		t.Fatalf("production webhook client did not block redirects: %v", err)
	}
}
