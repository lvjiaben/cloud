// Package signedwebhook provides the built-in HMAC-SHA256 machine notification channel.
package signedwebhook

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-plugin-idc-core/backend/cap/secretcap"
	"lina-plugin-idc-notify/backend/cap/channelcap"
)

const (
	providerCode     = "signed_webhook_hmac_sha256_v1"
	endpointField    = "endpoint_url"
	signingKeyField  = "signing_secret"
	signatureVersion = "v1"
	maximumResponse  = 4096
)

var deniedPrefixes = mustPrefixes(
	"0.0.0.0/8", "10.0.0.0/8", "100.64.0.0/10", "127.0.0.0/8", "169.254.0.0/16",
	"172.16.0.0/12", "192.0.0.0/24", "192.0.2.0/24", "192.31.196.0/24", "192.52.193.0/24",
	"192.88.99.0/24", "192.168.0.0/16", "192.175.48.0/24", "198.18.0.0/15",
	"198.51.100.0/24", "203.0.113.0/24", "224.0.0.0/4", "240.0.0.0/4",
	"::/128", "::1/128", "::ffff:0:0/96", "64:ff9b::/96", "64:ff9b:1::/48", "100::/64", "100:0:0:1::/64",
	"2001::/23", "2001:db8::/32", "2002::/16", "2620:4f:8000::/48", "3fff::/20", "5f00::/16",
	"fc00::/7", "fe80::/10", "ff00::/8",
)

var declaration struct {
	sync.Once
	err error
}

// Declare registers the built-in adapter exactly once before the channel registry freezes.
func Declare() error {
	declaration.Do(func() {
		declaration.err = channelcap.Declare(channelcap.Descriptor{
			ProviderCode: providerCode,
			Kind:         channelcap.KindSignedWebhook,
			Fields: []channelcap.Field{
				{Key: endpointField, LabelKey: "notification.signed_webhook.endpoint_url", Required: true, MaxLength: 2048},
				{Key: signingKeyField, LabelKey: "notification.signed_webhook.signing_secret", Secret: true, Required: true, MaxLength: 512},
			},
			Factory: func(accessor secretcap.Accessor) (channelcap.Adapter, error) {
				return newProductionAdapter(accessor, net.DefaultResolver, &net.Dialer{Timeout: 10 * time.Second, KeepAlive: -1})
			},
		})
	})
	return declaration.err
}

type adapter struct {
	accessor         secretcap.Accessor
	client           *http.Client
	validateEndpoint func(context.Context, *url.URL) error
	purpose          secretcap.Purpose
}

type addressResolver interface {
	LookupIPAddr(context.Context, string) ([]net.IPAddr, error)
}

type networkDialer interface {
	DialContext(context.Context, string, string) (net.Conn, error)
}

type validatedDialer struct {
	resolver addressResolver
	dialer   networkDialer
}

type eventEnvelope struct {
	EventNo    string `json:"eventNo"`
	Type       string `json:"type"`
	Version    int64  `json:"version"`
	OccurredAt int64  `json:"occurredAt"`
}

func newProductionAdapter(accessor secretcap.Accessor, resolver addressResolver, dialer networkDialer) (*adapter, error) {
	if resolver == nil || dialer == nil {
		return nil, gerror.New("signed webhook network policy is unavailable")
	}
	validated := &validatedDialer{resolver: resolver, dialer: dialer}
	transport := &http.Transport{
		Proxy:               nil,
		DialContext:         validated.DialContext,
		DisableKeepAlives:   true,
		TLSHandshakeTimeout: 10 * time.Second,
		TLSClientConfig:     &tls.Config{MinVersion: tls.VersionTLS12},
	}
	client := &http.Client{
		Transport: transport,
		Timeout:   10 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	return newAdapter(accessor, client, func(ctx context.Context, endpoint *url.URL) error {
		_, err := resolvePublicAddresses(ctx, resolver, endpoint.Hostname())
		return err
	})
}

func newAdapter(accessor secretcap.Accessor, client *http.Client, validate func(context.Context, *url.URL) error) (*adapter, error) {
	if accessor == nil || client == nil || validate == nil {
		return nil, gerror.New("signed webhook dependencies are unavailable")
	}
	purpose, err := secretcap.NewPurpose("notify.send")
	if err != nil {
		return nil, err
	}
	return &adapter{accessor: accessor, client: client, validateEndpoint: validate, purpose: purpose}, nil
}

func (a *adapter) ValidateConfiguration(ctx context.Context, nonSecret map[string]string, secrets map[string][]byte) error {
	endpoint, err := parseEndpoint(nonSecret[endpointField])
	if err != nil {
		return err
	}
	if err = a.validateEndpoint(ctx, endpoint); err != nil {
		return gerror.New("signed webhook endpoint is unavailable")
	}
	if !validSigningSecret(secrets[signingKeyField]) {
		return gerror.New("signed webhook signing secret is invalid")
	}
	return nil
}

func (a *adapter) ValidateDestination(_ []byte) error { return nil }

func (a *adapter) Deliver(ctx context.Context, input channelcap.Delivery) (channelcap.Result, error) {
	endpoint, err := parseEndpoint(input.NonSecretConfig[endpointField])
	if err != nil || input.EventNo == "" || input.EventType == "" || input.EventVersion < 0 || input.OccurredAt.IsZero() {
		return signedWebhookResult(input.EventNo, channelcap.ResultConfigurationError, false, 0), nil
	}
	if err = a.validateEndpoint(ctx, endpoint); err != nil {
		return signedWebhookResult(input.EventNo, channelcap.ResultConfigurationError, false, 0), nil
	}
	payload, err := json.Marshal(eventEnvelope{
		EventNo: input.EventNo, Type: input.EventType,
		Version: input.EventVersion, OccurredAt: input.OccurredAt.UTC().UnixMilli(),
	})
	if err != nil {
		return signedWebhookResult(input.EventNo, channelcap.ResultConfigurationError, false, 0), nil
	}
	request, err := secretcap.NewAccessRequest(input.Credential, a.purpose)
	if err != nil {
		return signedWebhookResult(input.EventNo, channelcap.ResultConfigurationError, false, 0), nil
	}
	result := signedWebhookResult(input.EventNo, channelcap.ResultUnknown, false, 0)
	accessErr := a.accessor.Access(ctx, request, func(plaintext []byte) error {
		var secrets map[string][]byte
		if err := json.Unmarshal(plaintext, &secrets); err != nil {
			return err
		}
		defer clearSecretMap(secrets)
		signingSecret := secrets[signingKeyField]
		if !validSigningSecret(signingSecret) {
			return errors.New("signed webhook credential is unavailable")
		}
		hash := hmac.New(sha256.New, signingSecret)
		_, _ = hash.Write(payload)
		signature := "sha256=" + hex.EncodeToString(hash.Sum(nil))

		httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint.String(), strings.NewReader(string(payload)))
		if err != nil {
			return err
		}
		httpRequest.Header.Set("Content-Type", "application/json")
		httpRequest.Header.Set("User-Agent", "idc-notify/1")
		httpRequest.Header.Set("X-Lina-Event-No", input.EventNo)
		httpRequest.Header.Set("X-Lina-Timestamp", strconv.FormatInt(input.OccurredAt.UTC().UnixMilli(), 10))
		httpRequest.Header.Set("X-Lina-Signature-Version", signatureVersion)
		httpRequest.Header.Set("X-Lina-Signature", signature)
		response, err := a.client.Do(httpRequest)
		if err != nil {
			result = signedWebhookResult(input.EventNo, channelcap.ResultUnknown, true, 0)
			return nil
		}
		defer response.Body.Close()
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, maximumResponse))
		if response.StatusCode >= http.StatusOK && response.StatusCode < http.StatusMultipleChoices {
			result = signedWebhookResult(input.EventNo, channelcap.ResultDelivered, true, response.StatusCode)
			return nil
		}
		// A non-2xx response proves the endpoint answered, but it cannot prove
		// whether the receiver committed a side effect. It therefore remains
		// unknown and is never automatically retried.
		result = signedWebhookResult(input.EventNo, channelcap.ResultUnknown, true, response.StatusCode)
		return nil
	})
	if accessErr != nil {
		return signedWebhookResult(input.EventNo, channelcap.ResultConfigurationError, false, 0), nil
	}
	return result, nil
}

func (a *adapter) Query(_ context.Context, input channelcap.Query) (channelcap.Result, error) {
	return signedWebhookResult(input.TaskNo, channelcap.ResultUnknown, true, 0), nil
}

func parseEndpoint(value string) (*url.URL, error) {
	if value == "" || value != strings.TrimSpace(value) || len(value) > 2048 {
		return nil, gerror.New("signed webhook endpoint is invalid")
	}
	parsed, err := url.ParseRequestURI(value)
	if err != nil || parsed.Host == "" || parsed.Hostname() == "" || parsed.User != nil || parsed.Fragment != "" ||
		parsed.Scheme != "https" {
		return nil, gerror.New("signed webhook endpoint is invalid")
	}
	if parsed.Port() != "" {
		port, parseErr := strconv.Atoi(parsed.Port())
		if parseErr != nil || port < 1 || port > 65535 {
			return nil, gerror.New("signed webhook endpoint is invalid")
		}
	}
	return parsed, nil
}

func (d *validatedDialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, gerror.New("signed webhook destination is invalid")
	}
	addresses, err := resolvePublicAddresses(ctx, d.resolver, host)
	if err != nil {
		return nil, err
	}
	var lastErr error
	for _, candidate := range addresses {
		if network == "tcp4" && !candidate.Is4() || network == "tcp6" && !candidate.Is6() {
			continue
		}
		connection, dialErr := d.dialer.DialContext(ctx, network, net.JoinHostPort(candidate.String(), port))
		if dialErr == nil {
			return connection, nil
		}
		lastErr = dialErr
	}
	if lastErr != nil {
		return nil, lastErr
	}
	return nil, gerror.New("signed webhook destination is unavailable")
}

func resolvePublicAddresses(ctx context.Context, resolver addressResolver, host string) ([]netip.Addr, error) {
	if host == "" || resolver == nil {
		return nil, gerror.New("signed webhook destination is invalid")
	}
	if literal, err := netip.ParseAddr(host); err == nil {
		literal = literal.Unmap()
		if !isPublicAddress(literal) {
			return nil, gerror.New("signed webhook destination is not public")
		}
		return []netip.Addr{literal}, nil
	}
	resolved, err := resolver.LookupIPAddr(ctx, host)
	if err != nil || len(resolved) == 0 {
		return nil, gerror.New("signed webhook destination cannot be resolved")
	}
	addresses := make([]netip.Addr, 0, len(resolved))
	seen := make(map[netip.Addr]struct{}, len(resolved))
	for _, item := range resolved {
		address, ok := netip.AddrFromSlice(item.IP)
		if !ok {
			return nil, gerror.New("signed webhook destination is invalid")
		}
		address = address.Unmap()
		if !isPublicAddress(address) {
			return nil, gerror.New("signed webhook destination is not public")
		}
		if _, exists := seen[address]; exists {
			continue
		}
		seen[address] = struct{}{}
		addresses = append(addresses, address)
	}
	if len(addresses) == 0 {
		return nil, gerror.New("signed webhook destination is unavailable")
	}
	return addresses, nil
}

func isPublicAddress(address netip.Addr) bool {
	if !address.IsValid() || !address.IsGlobalUnicast() || address.IsPrivate() || address.IsLoopback() ||
		address.IsLinkLocalUnicast() || address.IsLinkLocalMulticast() || address.IsMulticast() || address.IsUnspecified() {
		return false
	}
	for _, prefix := range deniedPrefixes {
		if prefix.Contains(address) {
			return false
		}
	}
	return true
}

func mustPrefixes(values ...string) []netip.Prefix {
	result := make([]netip.Prefix, len(values))
	for index, value := range values {
		result[index] = netip.MustParsePrefix(value)
	}
	return result
}

func validSigningSecret(value []byte) bool {
	return len(value) >= 32 && len(value) <= 512
}

func signedWebhookResult(eventNo string, class channelcap.ResultClass, crossed bool, status int) channelcap.Result {
	digest := sha256.Sum256([]byte("signed-webhook-v1\x00" + eventNo + "\x00" + string(class) + "\x00" + strconv.Itoa(status)))
	return channelcap.Result{
		Class: class, ProviderMessageRef: eventNo,
		EvidenceDigest: hex.EncodeToString(digest[:]), CrossedExternalBorder: crossed,
	}
}

func clearSecretMap(values map[string][]byte) {
	for key, value := range values {
		clear(value)
		delete(values, key)
	}
}
