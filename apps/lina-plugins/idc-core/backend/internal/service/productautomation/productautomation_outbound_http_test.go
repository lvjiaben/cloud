package productautomation

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"testing"

	"lina-core/pkg/plugin/capability/capmodel"
	"lina-plugin-idc-core/backend/cap/supplierinterfacecap"
)

type outboundDoerStub struct {
	request *http.Request
}

func (s *outboundDoerStub) Do(request *http.Request) (*http.Response, error) {
	s.request = request
	return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"success":true}`))}, nil
}

type outboundResolverStub struct{ addresses []netip.Addr }

func (s outboundResolverStub) LookupNetIP(context.Context, string, string) ([]netip.Addr, error) {
	return append([]netip.Addr(nil), s.addresses...), nil
}

type outboundDialerStub struct{ calls int }

func (s *outboundDialerStub) DialContext(context.Context, string, string) (net.Conn, error) {
	s.calls++
	return nil, context.DeadlineExceeded
}

func TestSupplierOriginRequiresHTTPSWithoutCredentialsOrQuery(t *testing.T) {
	for _, endpoint := range []string{
		"http://supplier.example/v1", "https://user:pass@supplier.example/v1",
		"https://supplier.example/v1?token=value", "https://supplier.example/v1#fragment",
	} {
		if _, err := parseSupplierOrigin(endpoint); err == nil {
			t.Fatalf("unsafe supplier origin was accepted: %s", endpoint)
		}
	}
	if _, err := parseSupplierOrigin("https://supplier.example/user/v1"); err != nil {
		t.Fatal(err)
	}
}

func TestExactOriginClientRejectsOriginSwitch(t *testing.T) {
	doer := &outboundDoerStub{}
	client := &exactOriginSupplierClient{host: "supplier.example", port: "443", client: doer}
	request, _ := http.NewRequest(http.MethodPost, "https://evil.example/payment/balance", strings.NewReader(`{}`))
	if _, err := client.Do(request); err == nil || doer.request != nil {
		t.Fatal("supplier origin switch reached the network client")
	}
	request.URL, _ = url.Parse("https://supplier.example/payment/balance")
	request.Host = ""
	if _, err := client.Do(request); err != nil || doer.request == nil {
		t.Fatalf("exact supplier origin was rejected: %v", err)
	}
}

func TestSupplierDialerRejectsPrivateOrMixedDNSBeforeDial(t *testing.T) {
	for name, addresses := range map[string][]netip.Addr{
		"private":        {netip.MustParseAddr("10.0.0.1")},
		"shared carrier": {netip.MustParseAddr("100.64.0.1")},
		"documentation":  {netip.MustParseAddr("203.0.113.10")},
		"ipv6 docs":      {netip.MustParseAddr("2001:db8::10")},
		"mixed":          {netip.MustParseAddr("1.1.1.1"), netip.MustParseAddr("192.168.1.2")},
	} {
		t.Run(name, func(t *testing.T) {
			dialer := &outboundDialerStub{}
			policy := &supplierPublicDialer{resolver: outboundResolverStub{addresses: addresses}, dialer: dialer}
			if _, err := policy.DialContext(context.Background(), "tcp", "supplier.example:443"); err == nil || dialer.calls != 0 {
				t.Fatalf("private DNS result reached dialer: calls=%d err=%v", dialer.calls, err)
			}
		})
	}
}

func TestSupplierResponseProjectionDropsUnlistedFields(t *testing.T) {
	projected, err := projectSupplierResponse([]byte(`{"success":true,"order_id":"YS-1","secret":"must-not-leak","data":{"status":"issued","token":"hidden"}}`), []string{"success", "order_id", "data", "status"})
	if err != nil {
		t.Fatal(err)
	}
	text := string(projected)
	if !strings.Contains(text, "YS-1") || !strings.Contains(text, "issued") || strings.Contains(text, "must-not-leak") || strings.Contains(text, "hidden") {
		t.Fatalf("unexpected supplier projection: %s", projected)
	}
}

func TestRuntimeEndpointRequiresDeclaredURLField(t *testing.T) {
	endpoint := "https://supplier.example/user/v1"
	runtimeContext := supplierRuntimeContext{
		interfaceDeclaration: &capmodel.ConfigurationSchemaDeclaration{Fields: []*capmodel.ConfigurationFieldDeclaration{{Key: "endpoint", Type: capmodel.ConfigurationFieldSingleLine, TextFormat: capmodel.ConfigurationTextFormatURL}}},
		configurationView:    &supplierinterfacecap.ConfigurationView{Values: []supplierinterfacecap.ConfigurationValue{{FieldKey: "endpoint", TextValue: &endpoint}}},
	}
	if actual, ok := runtimeEndpoint(runtimeContext, "endpoint"); !ok || actual != endpoint {
		t.Fatalf("endpoint=%q ok=%v", actual, ok)
	}
	if _, ok := runtimeEndpoint(runtimeContext, "another"); ok {
		t.Fatal("undeclared endpoint field was accepted")
	}
}
