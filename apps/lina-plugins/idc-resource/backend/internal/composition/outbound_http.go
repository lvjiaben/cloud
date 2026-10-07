// This file implements the resource-owner outbound HTTPS and DNS rebinding policy.
package composition

import (
	"context"
	"crypto/tls"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-plugin-idc-resource/backend/cap/resourcecap"
)

const (
	providerDialTimeout           = 10 * time.Second
	providerTLSHandshakeTimeout   = 10 * time.Second
	providerResponseHeaderTimeout = 20 * time.Second
	providerIdleConnectionTimeout = 30 * time.Second
	providerRequestTimeout        = 90 * time.Second
)

type networkResolver interface {
	LookupNetIP(ctx context.Context, network string, host string) ([]netip.Addr, error)
}

type networkDialer interface {
	DialContext(ctx context.Context, network string, address string) (net.Conn, error)
}

type requestDoer interface {
	Do(request *http.Request) (*http.Response, error)
}

// outboundHTTPClient restricts every request to the configured channel origin.
type outboundHTTPClient struct {
	host   string
	port   string
	client requestDoer
}

var _ resourcecap.HTTPClient = (*outboundHTTPClient)(nil)

// NewOutboundHTTPClient constructs a Provider HTTP client with exact-origin,
// TLS, redirect, DNS, and destination-address enforcement.
func NewOutboundHTTPClient(endpoint string) (resourcecap.HTTPClient, func(), error) {
	origin, err := parseOutboundOrigin(endpoint)
	if err != nil {
		return nil, nil, err
	}
	dialPolicy := &publicNetworkDialer{
		resolver: net.DefaultResolver,
		dialer:   &net.Dialer{Timeout: providerDialTimeout, KeepAlive: 30 * time.Second},
	}
	transport := &http.Transport{
		Proxy:                 nil,
		DialContext:           dialPolicy.DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          32,
		MaxIdleConnsPerHost:   8,
		IdleConnTimeout:       providerIdleConnectionTimeout,
		TLSHandshakeTimeout:   providerTLSHandshakeTimeout,
		ResponseHeaderTimeout: providerResponseHeaderTimeout,
		ExpectContinueTimeout: time.Second,
		TLSClientConfig: &tls.Config{
			MinVersion: tls.VersionTLS12,
			ServerName: origin.Hostname(),
		},
	}
	client := &http.Client{
		Transport: transport,
		Timeout:   providerRequestTimeout,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	closeClient := func() {
		transport.CloseIdleConnections()
	}
	return &outboundHTTPClient{
		host: origin.Hostname(), port: canonicalHTTPSPort(origin), client: client,
	}, closeClient, nil
}

func (c *outboundHTTPClient) Do(request *http.Request) (*http.Response, error) {
	if c == nil || c.client == nil || request == nil || request.URL == nil ||
		request.Context() == nil || request.Context().Err() != nil ||
		request.URL.Scheme != "https" || request.URL.User != nil || request.URL.Opaque != "" ||
		request.URL.Fragment != "" || !strings.EqualFold(request.URL.Hostname(), c.host) ||
		canonicalHTTPSPort(request.URL) != c.port ||
		(request.Host != "" && !strings.EqualFold(request.Host, request.URL.Host)) {
		return nil, gerror.New("resource Provider outbound request is not permitted")
	}
	return c.client.Do(request)
}

func parseOutboundOrigin(endpoint string) (*url.URL, error) {
	parsed, err := url.Parse(endpoint)
	if err != nil || parsed.Scheme != "https" || parsed.Hostname() == "" || parsed.User != nil ||
		parsed.Opaque != "" || parsed.RawQuery != "" || parsed.ForceQuery || parsed.Fragment != "" {
		return nil, gerror.New("resource Provider outbound endpoint is invalid")
	}
	if port := parsed.Port(); port != "" {
		parsedPort, parseErr := strconv.ParseUint(port, 10, 16)
		if parseErr != nil || parsedPort == 0 {
			return nil, gerror.New("resource Provider outbound endpoint is invalid")
		}
	}
	return parsed, nil
}

func canonicalHTTPSPort(value *url.URL) string {
	if value == nil {
		return ""
	}
	if port := value.Port(); port != "" {
		return port
	}
	return "443"
}

type publicNetworkDialer struct {
	resolver networkResolver
	dialer   networkDialer
}

func (d *publicNetworkDialer) DialContext(
	ctx context.Context,
	network string,
	address string,
) (net.Conn, error) {
	if d == nil || d.resolver == nil || d.dialer == nil || ctx == nil || ctx.Err() != nil ||
		(network != "tcp" && network != "tcp4" && network != "tcp6") {
		return nil, gerror.New("resource Provider network destination is not permitted")
	}
	host, port, err := net.SplitHostPort(address)
	if err != nil || host == "" || port == "" {
		return nil, gerror.New("resource Provider network destination is not permitted")
	}
	parsedPort, err := strconv.ParseUint(port, 10, 16)
	if err != nil || parsedPort == 0 {
		return nil, gerror.New("resource Provider network destination is not permitted")
	}
	addresses, err := d.resolve(ctx, host)
	if err != nil || len(addresses) == 0 {
		return nil, gerror.New("resource Provider network destination is not permitted")
	}
	for _, candidate := range addresses {
		if !isPublicProviderAddress(candidate) {
			return nil, gerror.New("resource Provider network destination is not permitted")
		}
	}
	for _, candidate := range addresses {
		connection, dialErr := d.dialer.DialContext(
			ctx,
			networkForAddress(network, candidate),
			net.JoinHostPort(candidate.Unmap().String(), port),
		)
		if dialErr == nil {
			return connection, nil
		}
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
	}
	return nil, gerror.New("resource Provider network connection failed")
}

func (d *publicNetworkDialer) resolve(ctx context.Context, host string) ([]netip.Addr, error) {
	if literal, err := netip.ParseAddr(host); err == nil {
		return []netip.Addr{literal.Unmap()}, nil
	}
	addresses, err := d.resolver.LookupNetIP(ctx, "ip", host)
	if err != nil {
		return nil, err
	}
	result := make([]netip.Addr, 0, len(addresses))
	seen := make(map[netip.Addr]struct{}, len(addresses))
	for _, address := range addresses {
		address = address.Unmap()
		if !address.IsValid() {
			return nil, gerror.New("resource Provider DNS response is invalid")
		}
		if _, exists := seen[address]; exists {
			continue
		}
		seen[address] = struct{}{}
		result = append(result, address)
	}
	return result, nil
}

func networkForAddress(requested string, address netip.Addr) string {
	if requested == "tcp" {
		if address.Unmap().Is4() {
			return "tcp4"
		}
		return "tcp6"
	}
	return requested
}

func isPublicProviderAddress(address netip.Addr) bool {
	address = address.Unmap()
	if !address.IsValid() || !address.IsGlobalUnicast() || address.IsPrivate() ||
		address.IsLoopback() || address.IsLinkLocalUnicast() || address.IsMulticast() || address.IsUnspecified() {
		return false
	}
	if address.Is4() {
		value := address.As4()
		return !(value[0] == 0 ||
			(value[0] == 100 && value[1]&0xc0 == 64) ||
			(value[0] == 192 && value[1] == 0 && (value[2] == 0 || value[2] == 2)) ||
			(value[0] == 198 && (value[1] == 18 || value[1] == 19 || value[1] == 51 && value[2] == 100)) ||
			(value[0] == 203 && value[1] == 0 && value[2] == 113) ||
			value[0] >= 224)
	}
	value := address.As16()
	if value[0] == 0x20 && value[1] == 0x01 && value[2] == 0x0d && value[3] == 0xb8 {
		return false
	}
	if value[0] == 0x01 && value[1] == 0x00 {
		allZero := true
		for _, character := range value[2:8] {
			allZero = allZero && character == 0
		}
		if allZero {
			return false
		}
	}
	return true
}
