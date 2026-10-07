package composition

import (
	"context"
	"crypto/tls"
	"errors"
	"net"
	"net/http"
	"net/netip"
	"strings"
	"testing"
)

type fixedNetworkResolver struct {
	addresses []netip.Addr
	err       error
	calls     int
}

func (r *fixedNetworkResolver) LookupNetIP(
	_ context.Context,
	_ string,
	_ string,
) ([]netip.Addr, error) {
	r.calls++
	return append([]netip.Addr(nil), r.addresses...), r.err
}

type captureNetworkDialer struct {
	addresses []string
	networks  []string
	err       error
}

func (d *captureNetworkDialer) DialContext(
	_ context.Context,
	network string,
	address string,
) (net.Conn, error) {
	d.networks = append(d.networks, network)
	d.addresses = append(d.addresses, address)
	if d.err != nil {
		return nil, d.err
	}
	client, server := net.Pipe()
	_ = server.Close()
	return client, nil
}

type captureRequestDoer struct {
	calls int
}

func (d *captureRequestDoer) Do(*http.Request) (*http.Response, error) {
	d.calls++
	return &http.Response{StatusCode: http.StatusNoContent, Body: http.NoBody}, nil
}

func TestOutboundHTTPClientEnforcesExactHTTPSOrigin(t *testing.T) {
	delegate := &captureRequestDoer{}
	client := &outboundHTTPClient{host: "provider.example", port: "8443", client: delegate}
	allowed, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodGet,
		"https://provider.example:8443/administrator/v1/clouds?page=1",
		nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	response, err := client.Do(allowed)
	if err != nil || response == nil || delegate.calls != 1 {
		t.Fatalf("exact origin request rejected: response=%v calls=%d err=%v", response, delegate.calls, err)
	}
	for _, rawURL := range []string{
		"http://provider.example:8443/administrator/v1/clouds",
		"https://provider.example/administrator/v1/clouds",
		"https://other.example:8443/administrator/v1/clouds",
		"https://user@provider.example:8443/administrator/v1/clouds",
		"https://provider.example:8443/administrator/v1/clouds#fragment",
	} {
		request, requestErr := http.NewRequestWithContext(context.Background(), http.MethodGet, rawURL, nil)
		if requestErr != nil {
			t.Fatal(requestErr)
		}
		if _, callErr := client.Do(request); callErr == nil {
			t.Fatalf("out-of-origin request accepted: %s", rawURL)
		}
	}
	overridden, err := http.NewRequestWithContext(
		context.Background(), http.MethodGet, "https://provider.example:8443/path", nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	overridden.Host = "other.example"
	if _, err = client.Do(overridden); err == nil {
		t.Fatal("overridden HTTP Host was accepted")
	}
	if delegate.calls != 1 {
		t.Fatalf("rejected requests reached transport: calls=%d", delegate.calls)
	}
}

func TestPublicNetworkDialerPinsValidatedAddressAndRejectsRebinding(t *testing.T) {
	resolver := &fixedNetworkResolver{addresses: []netip.Addr{netip.MustParseAddr("93.184.216.34")}}
	dialer := &captureNetworkDialer{}
	policy := &publicNetworkDialer{resolver: resolver, dialer: dialer}
	connection, err := policy.DialContext(context.Background(), "tcp", "provider.example:443")
	if err != nil || connection == nil {
		t.Fatalf("public destination rejected: %v", err)
	}
	_ = connection.Close()
	if resolver.calls != 1 || len(dialer.addresses) != 1 || dialer.addresses[0] != "93.184.216.34:443" ||
		dialer.networks[0] != "tcp4" {
		t.Fatalf("dial did not pin the validated address: resolver=%d networks=%v addresses=%v", resolver.calls, dialer.networks, dialer.addresses)
	}

	resolver.addresses = []netip.Addr{
		netip.MustParseAddr("93.184.216.34"),
		netip.MustParseAddr("127.0.0.1"),
	}
	dialer.addresses = nil
	if _, err = policy.DialContext(context.Background(), "tcp", "provider.example:443"); err == nil {
		t.Fatal("mixed public and private DNS response was accepted")
	}
	if len(dialer.addresses) != 0 {
		t.Fatal("mixed DNS response reached the network dialer")
	}
}

func TestProviderAddressPolicyRejectsNonPublicRanges(t *testing.T) {
	tests := map[string]bool{
		"93.184.216.34":        true,
		"2606:4700:4700::1111": true,
		"127.0.0.1":            false,
		"10.0.0.1":             false,
		"100.64.0.1":           false,
		"169.254.1.1":          false,
		"192.0.2.1":            false,
		"198.18.0.1":           false,
		"198.51.100.1":         false,
		"203.0.113.1":          false,
		"224.0.0.1":            false,
		"::1":                  false,
		"fc00::1":              false,
		"fe80::1":              false,
		"2001:db8::1":          false,
	}
	for raw, expected := range tests {
		if actual := isPublicProviderAddress(netip.MustParseAddr(raw)); actual != expected {
			t.Errorf("address %s public=%v want=%v", raw, actual, expected)
		}
	}
}

func TestNewOutboundHTTPClientUsesVerifiedTLSAndNoRedirects(t *testing.T) {
	client, closeClient, err := NewOutboundHTTPClient("https://provider.example:8443/administrator/v1")
	if err != nil || client == nil || closeClient == nil {
		t.Fatalf("construct outbound client failed: client=%T err=%v", client, err)
	}
	defer closeClient()
	ownerClient, ok := client.(*outboundHTTPClient)
	if !ok {
		t.Fatalf("unexpected outbound client type %T", client)
	}
	httpClient, ok := ownerClient.client.(*http.Client)
	if !ok || httpClient.CheckRedirect == nil || httpClient.Timeout != providerRequestTimeout {
		t.Fatal("outbound client redirect policy is unavailable")
	}
	transport, ok := httpClient.Transport.(*http.Transport)
	if !ok || transport.Proxy != nil || transport.DialContext == nil ||
		transport.TLSClientConfig == nil || transport.TLSClientConfig.InsecureSkipVerify ||
		transport.TLSClientConfig.MinVersion < tls.VersionTLS12 ||
		transport.TLSClientConfig.ServerName != "provider.example" {
		t.Fatal("outbound client TLS or network policy is incomplete")
	}
	redirectRequest, err := http.NewRequestWithContext(
		context.Background(), http.MethodGet, "https://provider.example:8443/other", nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	if redirectErr := httpClient.CheckRedirect(redirectRequest, nil); !errors.Is(redirectErr, http.ErrUseLastResponse) {
		t.Fatalf("redirect policy error=%v", redirectErr)
	}
	if _, _, err = NewOutboundHTTPClient("http://provider.example"); err == nil ||
		!strings.Contains(err.Error(), "endpoint") {
		t.Fatal("plaintext outbound endpoint was accepted")
	}
}
