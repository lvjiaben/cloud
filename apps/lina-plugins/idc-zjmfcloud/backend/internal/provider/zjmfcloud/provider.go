// Package zjmfcloud implements the ZJMF public-cloud administrator v1 adapter.
package zjmfcloud

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-plugin-idc-core/backend/cap/secretcap"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
)

const (
	// ProviderID is the stable declaration identity for the administrator v1 adapter.
	ProviderID = "zjmfcloud-v1"

	adapterVersion  = "zjmfcloud-v1.7.0"
	usernameBinding = "username"
	passwordBinding = "password"

	defaultTokenCacheTTL       = 15 * time.Minute
	minimumTokenCacheTTL       = time.Minute
	maximumTokenCacheTTL       = time.Hour
	defaultRequestTimeout      = 30 * time.Second
	minimumRequestTimeout      = time.Second
	maximumRequestTimeout      = 2 * time.Minute
	defaultCircuitFailures     = 5
	maximumCircuitFailures     = 50
	defaultCircuitOpenDuration = 30 * time.Second
	maximumCircuitOpenDuration = 10 * time.Minute
	defaultMaximumConcurrent   = 8
	maximumConcurrentLimit     = 64
	maximumLoginResponseBytes  = 32 << 10
	maximumAPIResponseBytes    = 4 << 20
	maximumTokenBytes          = 8 << 10
	maximumCursorPage          = 1_000_000
)

var (
	errUnsupported      = gerror.New("zjmfcloud: capability is unsupported")
	errInvalidConfig    = gerror.New("zjmfcloud: provider configuration is invalid")
	errInvalidResponse  = gerror.New("zjmfcloud: upstream response is invalid")
	errTransportFailure = gerror.New("zjmfcloud: upstream transport failed")
	errTransportPolicy  = gerror.New("zjmfcloud: upstream transport policy rejected the response")
	errRequestTimeout   = gerror.New("zjmfcloud: upstream request timed out")
	errAuthentication   = gerror.New("zjmfcloud: upstream authentication failed")
	errCircuitOpen      = gerror.New("zjmfcloud: upstream circuit is open")
	errResponseTooLarge = gerror.New("zjmfcloud: upstream response exceeds the configured boundary")
	errUnexpectedStatus = gerror.New("zjmfcloud: upstream returned an unexpected status")
)

type settings struct {
	TokenCacheTTL       time.Duration
	RequestTimeout      time.Duration
	CircuitFailures     int
	CircuitOpenDuration time.Duration
	MaximumConcurrent   int
	ConsoleHosts        map[string]struct{}
}

type settingsDocument struct {
	LoginCacheTTL       *string  `json:"login_cache_ttl"`
	RequestTimeout      *string  `json:"request_timeout"`
	CircuitFailures     *int     `json:"circuit_failure_threshold"`
	CircuitOpenDuration *string  `json:"circuit_open_duration"`
	MaximumConcurrent   *int     `json:"maximum_concurrent_requests"`
	ConsoleHosts        []string `json:"console_host_allowlist"`
}

// Provider is one channel-scoped adapter. It stores no resource mapping or task truth.
type Provider struct {
	env        resourcecap.ProviderEnv
	endpoint   *url.URL
	settings   settings
	username   secretcap.Handle
	password   secretcap.Handle
	channelRef string

	breakerMu          sync.Mutex
	consecutiveFailure int
	circuitOpenUntil   time.Time

	concurrency chan struct{}
}

var _ resourcecap.Provider = (*Provider)(nil)

// New validates a detached channel configuration and constructs a channel-scoped Provider.
func New(
	ctx context.Context,
	env resourcecap.ProviderEnv,
	config resourcecap.ProviderConfig,
) (resourcecap.Provider, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := env.Validate(); err != nil {
		return nil, errors.Join(errInvalidConfig, err)
	}
	if err := config.Validate(); err != nil {
		return nil, errors.Join(errInvalidConfig, err)
	}
	if env.PluginID != "idc-zjmfcloud" || env.ProviderID != ProviderID {
		return nil, errInvalidConfig
	}
	standardClient, ok := env.HTTPClient.(*http.Client)
	if !ok {
		return nil, errInvalidConfig
	}
	ownedClient := *standardClient
	ownedClient.CheckRedirect = func(_ *http.Request, _ []*http.Request) error {
		return http.ErrUseLastResponse
	}
	env.HTTPClient = &ownedClient

	endpoint, err := parseEndpoint(config.Endpoint)
	if err != nil {
		return nil, err
	}
	parsedSettings, err := parseSettings(config.Settings)
	if err != nil {
		return nil, err
	}
	bindings, err := parseBindings(config.SecretBindings)
	if err != nil {
		return nil, err
	}

	return &Provider{
		env:         env,
		endpoint:    endpoint,
		settings:    parsedSettings,
		username:    bindings[usernameBinding],
		password:    bindings[passwordBinding],
		channelRef:  config.ChannelRef,
		concurrency: make(chan struct{}, parsedSettings.MaximumConcurrent),
	}, nil
}

func parseEndpoint(value string) (*url.URL, error) {
	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme != "https" || parsed.Hostname() == "" || parsed.User != nil ||
		parsed.RawQuery != "" || parsed.Fragment != "" || parsed.Opaque != "" || parsed.RawPath != "" {
		return nil, errInvalidConfig
	}
	cleanedPath := strings.TrimSuffix(parsed.Path, "/")
	if cleanedPath == "" || path.Clean(cleanedPath) != cleanedPath || !strings.HasSuffix(cleanedPath, "/v1") {
		return nil, errInvalidConfig
	}
	parsed.Path = cleanedPath
	return parsed, nil
}

func parseSettings(raw json.RawMessage) (settings, error) {
	result := settings{
		TokenCacheTTL:       defaultTokenCacheTTL,
		RequestTimeout:      defaultRequestTimeout,
		CircuitFailures:     defaultCircuitFailures,
		CircuitOpenDuration: defaultCircuitOpenDuration,
		MaximumConcurrent:   defaultMaximumConcurrent,
	}
	if len(bytes.TrimSpace(raw)) == 0 {
		return result, nil
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var document settingsDocument
	if err := decoder.Decode(&document); err != nil || decoder.Decode(&struct{}{}) != io.EOF {
		return settings{}, errInvalidConfig
	}
	if document.LoginCacheTTL != nil {
		duration, parseErr := time.ParseDuration(*document.LoginCacheTTL)
		if parseErr != nil {
			return settings{}, errInvalidConfig
		}
		result.TokenCacheTTL = duration
	}
	if document.RequestTimeout != nil {
		duration, parseErr := time.ParseDuration(*document.RequestTimeout)
		if parseErr != nil {
			return settings{}, errInvalidConfig
		}
		result.RequestTimeout = duration
	}
	if document.CircuitFailures != nil {
		result.CircuitFailures = *document.CircuitFailures
	}
	if document.CircuitOpenDuration != nil {
		duration, parseErr := time.ParseDuration(*document.CircuitOpenDuration)
		if parseErr != nil {
			return settings{}, errInvalidConfig
		}
		result.CircuitOpenDuration = duration
	}
	if document.MaximumConcurrent != nil {
		result.MaximumConcurrent = *document.MaximumConcurrent
	}
	consoleHosts, err := parseConsoleHosts(document.ConsoleHosts)
	if err != nil {
		return settings{}, err
	}
	result.ConsoleHosts = consoleHosts
	if result.TokenCacheTTL < minimumTokenCacheTTL || result.TokenCacheTTL > maximumTokenCacheTTL ||
		result.RequestTimeout < minimumRequestTimeout || result.RequestTimeout > maximumRequestTimeout ||
		result.CircuitFailures < 1 || result.CircuitFailures > maximumCircuitFailures ||
		result.CircuitOpenDuration < time.Second || result.CircuitOpenDuration > maximumCircuitOpenDuration ||
		result.MaximumConcurrent < 1 || result.MaximumConcurrent > maximumConcurrentLimit {
		return settings{}, errInvalidConfig
	}
	return result, nil
}

func parseConsoleHosts(values []string) (map[string]struct{}, error) {
	if len(values) > 32 {
		return nil, errInvalidConfig
	}
	result := make(map[string]struct{}, len(values))
	for _, value := range values {
		canonical, ok := canonicalConsoleHost(value)
		if !ok || canonical != value && canonical != strings.ToLower(value) {
			return nil, errInvalidConfig
		}
		if _, exists := result[canonical]; exists {
			return nil, errInvalidConfig
		}
		result[canonical] = struct{}{}
	}
	return result, nil
}

func canonicalConsoleHost(value string) (string, bool) {
	if value == "" || value != strings.TrimSpace(value) || len(value) > 253 {
		return "", false
	}
	if parsed := net.ParseIP(value); parsed != nil {
		return parsed.String(), true
	}
	if !validHostname(value) {
		return "", false
	}
	return strings.ToLower(value), true
}

func (p *Provider) consoleHostAllowed(value string) bool {
	canonical, ok := canonicalConsoleHost(value)
	if !ok {
		return false
	}
	endpointHost, ok := canonicalConsoleHost(p.endpoint.Hostname())
	if ok && canonical == endpointHost {
		return true
	}
	_, allowed := p.settings.ConsoleHosts[canonical]
	return allowed
}

func parseBindings(bindings []resourcecap.SecretBinding) (map[string]secretcap.Handle, error) {
	if len(bindings) != 2 {
		return nil, errInvalidConfig
	}
	result := make(map[string]secretcap.Handle, len(bindings))
	for _, binding := range bindings {
		if binding.Name != usernameBinding && binding.Name != passwordBinding {
			return nil, errInvalidConfig
		}
		result[binding.Name] = binding.Handle
	}
	if result[usernameBinding].IsZero() || result[passwordBinding].IsZero() {
		return nil, errInvalidConfig
	}
	return result, nil
}

func (p *Provider) now() time.Time {
	return p.env.Clock.Now()
}

func (p *Provider) invalidateToken(ctx context.Context, generation int64) error {
	return p.env.AuthenticationSessions.Invalidate(ctx, resourcecap.AuthenticationSessionInvalidation{
		ChannelRef: p.channelRef,
		Generation: generation,
	})
}
