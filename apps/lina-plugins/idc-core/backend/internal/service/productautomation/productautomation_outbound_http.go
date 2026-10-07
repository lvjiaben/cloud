package productautomation

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/plugin/capability/capmodel"
	"lina-plugin-idc-core/backend/cap/supplierinterfacecap"
)

const supplierOutboundTimeout = 90 * time.Second

type supplierOutboundDoer interface {
	Do(*http.Request) (*http.Response, error)
}

type supplierOutboundClientFactory func(string) (supplierOutboundDoer, func(), error)

type exactOriginSupplierClient struct {
	host   string
	port   string
	client supplierOutboundDoer
}

func newSupplierOutboundClient(endpoint string) (supplierOutboundDoer, func(), error) {
	origin, err := parseSupplierOrigin(endpoint)
	if err != nil {
		return nil, nil, err
	}
	transport := &http.Transport{
		Proxy:               nil,
		ForceAttemptHTTP2:   true,
		MaxIdleConns:        16,
		MaxIdleConnsPerHost: 4,
		IdleConnTimeout:     30 * time.Second,
		TLSHandshakeTimeout: 10 * time.Second,
		TLSClientConfig: &tls.Config{
			MinVersion: tls.VersionTLS12,
			ServerName: origin.Hostname(),
		},
		DialContext: (&supplierPublicDialer{
			resolver: net.DefaultResolver,
			dialer:   &net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second},
		}).DialContext,
	}
	client := &http.Client{
		Transport: transport,
		Timeout:   supplierOutboundTimeout,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	return &exactOriginSupplierClient{host: origin.Hostname(), port: supplierOriginPort(origin), client: client}, transport.CloseIdleConnections, nil
}

func (c *exactOriginSupplierClient) Do(request *http.Request) (*http.Response, error) {
	if c == nil || c.client == nil || request == nil || request.URL == nil || request.URL.Scheme != "https" || request.URL.User != nil || request.URL.Opaque != "" || request.URL.Fragment != "" ||
		!strings.EqualFold(request.URL.Hostname(), c.host) || supplierOriginPort(request.URL) != c.port || (request.Host != "" && !strings.EqualFold(request.Host, request.URL.Host)) {
		return nil, gerror.New("supplier outbound request is not permitted")
	}
	return c.client.Do(request)
}

func (s *serviceImpl) runtimeOutboundHTTP(ctx context.Context, callerPluginID, contextID string, request supplierinterfacecap.OutboundHTTPRequest) (*supplierinterfacecap.OutboundHTTPResponse, error) {
	if s == nil || ctx == nil || ctx.Err() != nil || s.outboundClientFactory == nil || supplierinterfacecap.ValidateOutboundHTTPRequest(request) != nil {
		return nil, gerror.New("supplier runtime outbound request is unavailable")
	}
	runtimeContext, ok := s.runtimeContexts.get(contextID, callerPluginID)
	if !ok || !containsString(runtimeContext.authorizedMethods, runtimeContext.method) {
		return nil, gerror.New("supplier runtime outbound context is unavailable")
	}
	endpoint, ok := runtimeEndpoint(runtimeContext, request.EndpointField)
	if !ok {
		return nil, gerror.New("supplier runtime outbound endpoint is unavailable")
	}
	client, cleanup, err := s.outboundClientFactory(endpoint)
	if err != nil || client == nil || cleanup == nil {
		return nil, gerror.New("supplier runtime outbound client is unavailable")
	}
	defer cleanup()

	base, err := url.Parse(endpoint)
	if err != nil {
		return nil, gerror.New("supplier runtime outbound endpoint is unavailable")
	}
	target := *base
	target.Path = strings.TrimRight(base.Path, "/") + request.Path
	query := target.Query()
	for key, value := range request.Query {
		query.Set(key, value)
	}
	target.RawQuery = query.Encode()
	body := io.Reader(http.NoBody)
	if len(request.Body) > 0 {
		body = bytes.NewReader(request.Body)
	}
	httpRequest, err := http.NewRequestWithContext(ctx, request.Method, target.String(), body)
	if err != nil {
		return nil, gerror.New("supplier runtime outbound request is unavailable")
	}
	httpRequest.Header.Set("Accept", "application/json")
	if len(request.Body) > 0 {
		httpRequest.Header.Set("Content-Type", "application/json")
	}

	secretValues := make([][]byte, 0, len(request.SecretHeaders))
	defer func() {
		for _, value := range secretValues {
			clear(value)
		}
	}()
	headers := make([]string, 0, len(request.SecretHeaders))
	for header := range request.SecretHeaders {
		headers = append(headers, header)
	}
	slices.Sort(headers)
	for _, header := range headers {
		fieldKey := request.SecretHeaders[header]
		var copied []byte
		err = s.runtimeSecretAccess(ctx, callerPluginID, contextID, fieldKey, func(value []byte) error {
			copied = append([]byte(nil), value...)
			httpRequest.Header.Set(header, string(value))
			return nil
		})
		if err != nil {
			return nil, gerror.New("supplier runtime outbound credential is unavailable")
		}
		secretValues = append(secretValues, copied)
	}
	response, err := client.Do(httpRequest)
	if err != nil || response == nil || response.Body == nil {
		return nil, gerror.New("supplier runtime outbound request failed")
	}
	defer response.Body.Close()
	if response.StatusCode >= http.StatusMultipleChoices && response.StatusCode < http.StatusBadRequest {
		return nil, gerror.New("supplier runtime outbound redirect is forbidden")
	}
	content, err := io.ReadAll(io.LimitReader(response.Body, supplierinterfacecap.MaximumOutboundPayload+1))
	if err != nil || len(content) == 0 || len(content) > supplierinterfacecap.MaximumOutboundPayload || !json.Valid(content) {
		clear(content)
		return nil, gerror.New("supplier runtime outbound response is invalid")
	}
	defer clear(content)
	for _, secret := range secretValues {
		if supplierSecretAppears(content, secret) {
			return nil, gerror.New("supplier runtime outbound response contains credential material")
		}
	}
	projected, err := projectSupplierResponse(content, request.ResponseFields)
	if err != nil {
		return nil, err
	}
	result := &supplierinterfacecap.OutboundHTTPResponse{StatusCode: response.StatusCode, Data: projected}
	if supplierinterfacecap.ValidateOutboundHTTPResponse(result) != nil {
		return nil, gerror.New("supplier runtime outbound response is invalid")
	}
	return result, nil
}

func runtimeEndpoint(runtimeContext supplierRuntimeContext, fieldKey string) (string, bool) {
	declared := false
	if runtimeContext.interfaceDeclaration != nil {
		for _, field := range runtimeContext.interfaceDeclaration.Fields {
			if field != nil && field.Key == fieldKey && field.Type == capmodel.ConfigurationFieldSingleLine && field.TextFormat == capmodel.ConfigurationTextFormatURL {
				declared = true
				break
			}
		}
	}
	if !declared || runtimeContext.configurationView == nil {
		return "", false
	}
	for _, value := range runtimeContext.configurationView.Values {
		if value.FieldKey == fieldKey && value.TextValue != nil {
			return *value.TextValue, true
		}
	}
	return "", false
}

func projectSupplierResponse(content []byte, fields []string) (json.RawMessage, error) {
	allowed := make(map[string]struct{}, len(fields))
	for _, field := range fields {
		allowed[field] = struct{}{}
	}
	decoder := json.NewDecoder(bytes.NewReader(content))
	decoder.UseNumber()
	var value any
	if decoder.Decode(&value) != nil {
		return nil, gerror.New("supplier runtime outbound response is invalid")
	}
	projected, ok := projectSupplierValue(value, allowed, 0)
	if !ok {
		return nil, gerror.New("supplier runtime outbound response projection is empty")
	}
	result, err := json.Marshal(projected)
	if err != nil || len(result) > supplierinterfacecap.MaximumOutboundPayload {
		return nil, gerror.New("supplier runtime outbound response projection is invalid")
	}
	return result, nil
}

func projectSupplierValue(value any, allowed map[string]struct{}, depth int) (any, bool) {
	if depth > 16 {
		return nil, false
	}
	switch typed := value.(type) {
	case map[string]any:
		result := make(map[string]any)
		for key, child := range typed {
			if _, allowedField := allowed[key]; !allowedField {
				continue
			}
			if projected, ok := projectSupplierValue(child, allowed, depth+1); ok {
				result[key] = projected
			}
		}
		return result, len(result) > 0
	case []any:
		if len(typed) > 512 {
			return nil, false
		}
		result := make([]any, 0, len(typed))
		for _, child := range typed {
			if projected, ok := projectSupplierValue(child, allowed, depth+1); ok {
				result = append(result, projected)
			}
		}
		return result, true
	case nil, bool, json.Number, string:
		return typed, true
	default:
		return nil, false
	}
}

func supplierSecretAppears(content, secret []byte) bool {
	if len(secret) == 0 || len(content) == 0 {
		return false
	}
	return bytes.Contains(content, secret)
}

func parseSupplierOrigin(endpoint string) (*url.URL, error) {
	parsed, err := url.Parse(endpoint)
	if err != nil || parsed.Scheme != "https" || parsed.Hostname() == "" || parsed.User != nil || parsed.Opaque != "" || parsed.RawQuery != "" || parsed.ForceQuery || parsed.Fragment != "" {
		return nil, gerror.New("supplier outbound endpoint is invalid")
	}
	if port := parsed.Port(); port != "" {
		parsedPort, parseErr := strconv.ParseUint(port, 10, 16)
		if parseErr != nil || parsedPort == 0 {
			return nil, gerror.New("supplier outbound endpoint is invalid")
		}
	}
	return parsed, nil
}

func supplierOriginPort(value *url.URL) string {
	if value.Port() != "" {
		return value.Port()
	}
	return "443"
}

type supplierPublicDialer struct {
	resolver interface {
		LookupNetIP(context.Context, string, string) ([]netip.Addr, error)
	}
	dialer interface {
		DialContext(context.Context, string, string) (net.Conn, error)
	}
}

func (d *supplierPublicDialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if d == nil || d.resolver == nil || d.dialer == nil || err != nil || host == "" || port == "" || (network != "tcp" && network != "tcp4" && network != "tcp6") {
		return nil, gerror.New("supplier outbound destination is not permitted")
	}
	addresses, err := d.resolver.LookupNetIP(ctx, "ip", host)
	if err != nil || len(addresses) == 0 {
		return nil, gerror.New("supplier outbound destination is not permitted")
	}
	for _, candidate := range addresses {
		candidate = candidate.Unmap()
		if !isPublicSupplierAddress(candidate) {
			return nil, gerror.New("supplier outbound destination is not permitted")
		}
	}
	for _, candidate := range addresses {
		candidate = candidate.Unmap()
		requestedNetwork := network
		if network == "tcp" {
			if candidate.Is4() {
				requestedNetwork = "tcp4"
			} else {
				requestedNetwork = "tcp6"
			}
		}
		connection, dialErr := d.dialer.DialContext(ctx, requestedNetwork, net.JoinHostPort(candidate.String(), port))
		if dialErr == nil {
			return connection, nil
		}
	}
	return nil, gerror.New("supplier outbound connection failed")
}

func isPublicSupplierAddress(address netip.Addr) bool {
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
