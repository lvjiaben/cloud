// This file defines the minimal construction environment and runtime Provider SPI.

package resourcecap

import (
	"context"
	"encoding/json"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-plugin-idc-core/backend/cap/secretcap"
)

// Provider is the normalized runtime boundary implemented by an upstream
// resource adapter. Implementations return observations and external task
// references only; the resource owner remains responsible for persistence,
// local state transitions, compensation, and business notifications. Callers
// must validate requests before invocation and validate every returned value
// before persistence or state transition; adapters are not a trust boundary.
// Every non-nil Go error returned by these methods is untrusted and must pass
// through NormalizeProviderError before logging, persistence, retry policy, or
// propagation across the resource-owner boundary.
type Provider interface {
	// DiscoverCapabilities returns the adapter's current support matrix.
	DiscoverCapabilities(ctx context.Context) (*CapabilitySnapshot, error)
	// ListProviderCapacity returns one bounded page of normalized upstream providerCapacity.
	ListProviderCapacity(ctx context.Context, request ProviderCapacityRequest) (*ProviderCapacityPage, error)
	// Execute submits one idempotent resource operation.
	Execute(ctx context.Context, request OperationRequest) (*OperationResult, error)
	// QueryTask observes one previously submitted upstream task.
	QueryTask(ctx context.Context, request TaskQuery) (*OperationResult, error)
	// QueryResource observes one upstream resource without mutating local state.
	QueryResource(ctx context.Context, request ResourceQuery) (*ResourceSnapshot, error)
}

// PrincipalProvider is the optional upstream-account boundary implemented by
// adapters whose resource API requires a channel-local customer account.
// LookupPrincipal is read-only. CreatePrincipal is a single external write;
// its caller must durably fence an unknown result and may only reconcile it by
// calling LookupPrincipal. A CreatePrincipal call must never be replayed merely
// because its response was lost.
type PrincipalProvider interface {
	// LookupPrincipal performs one exact username lookup without creating data.
	LookupPrincipal(ctx context.Context, request PrincipalLookupRequest) (*PrincipalLookupResult, error)
	// CreatePrincipal submits one upstream account creation attempt.
	CreatePrincipal(ctx context.Context, request PrincipalCreateRequest) (*PrincipalCreateResult, error)
}

// ProviderFactory constructs a Provider lazily for one configured resource
// channel. The supplied context is the construction cancellation boundary.
// Owner composition must validate ProviderEnv and ProviderConfig before calling
// a factory and must reject a nil Provider returned without an error.
type ProviderFactory func(
	ctx context.Context,
	env ProviderEnv,
	config ProviderConfig,
) (Provider, error)

// ProviderConfig is the detached, in-process channel configuration supplied
// to one factory. SecretBindings contain opaque core-owned handles only,
// never secret material or independently implemented secret references. A
// ProviderConfig is not a persistence or bridge DTO because Handle rejects
// serialization by design.
type ProviderConfig struct {
	// ChannelRef is the opaque resource-owner channel identifier.
	ChannelRef string
	// Endpoint is the administrator-approved upstream API endpoint.
	Endpoint string
	// Settings contains adapter-owned non-secret JSON configuration.
	Settings json.RawMessage
	// SecretBindings maps adapter setting names to authorized secret references.
	SecretBindings []SecretBinding
}

// SecretBinding associates an adapter setting with one authorized core-owned
// secret handle. The resource owner constructs bindings explicitly and the
// adapter can use them only through ProviderEnv.Secrets.
type SecretBinding struct {
	// Name is the adapter-defined setting name.
	Name string
	// Handle is an opaque reference accepted by the injected core accessor.
	Handle secretcap.Handle
}

// SecretOutputPolicy identifies how the owner may reveal a confidential
// operation output after it has been encrypted into a secret handle.
type SecretOutputPolicy string

// Confidential output policies.
const (
	SecretOutputPolicyOneTimeCredential SecretOutputPolicy = "one_time_credential"
	SecretOutputPolicyShortLivedTicket  SecretOutputPolicy = "short_lived_ticket"
)

// SecretStoreRequest binds one confidential output to the idempotent
// operation and capability that produced it.
type SecretStoreRequest struct {
	// RequestID is the resource-owner idempotency key for the operation.
	RequestID string
	// Capability identifies the operation that produced the secret.
	Capability CapabilityCode
	// Name is the canonical output name.
	Name string
	// Policy controls subsequent owner-mediated reveal behavior.
	Policy SecretOutputPolicy
	// ExpiresAt is required only for short-lived tickets.
	ExpiresAt time.Time
}

// SecretOutput is one encrypted confidential operation result. Handle rejects
// serialization, so this value is an in-process contract rather than a DTO.
type SecretOutput struct {
	// Name is the canonical output name used in SecretStoreRequest.
	Name string
	// Policy controls subsequent owner-mediated reveal behavior.
	Policy SecretOutputPolicy
	// Handle identifies owner-managed encrypted material.
	Handle secretcap.Handle
	// ExpiresAt is required only for short-lived tickets.
	ExpiresAt time.Time
}

// SecretConsumer receives a plaintext byte view valid only for the callback.
// It must encrypt synchronously and must not retain, log, or serialize the view.
type SecretConsumer func(plaintext []byte) error

// SecretProducer supplies one complete confidential payload through consume.
// Producers must not retain an independent plaintext copy after returning.
type SecretProducer func(consume SecretConsumer) error

// SecretSink persists one confidential payload under the resource owner's
// encryption and authorization policy. Implementations invoke produce at most
// once, synchronously, and return a zero handle whenever storage fails.
type SecretSink interface {
	// Store encrypts and persists one produced payload without returning plaintext.
	Store(ctx context.Context, request SecretStoreRequest, produce SecretProducer) (secretcap.Handle, error)
}

// StoreSecret is the mandatory guarded path for adapters emitting confidential
// output. It validates metadata, enforces one synchronous producer/consumer
// call, clears the supplied plaintext view, discards handles on every failure,
// and never reflects an untrusted producer or sink error.
func StoreSecret(
	ctx context.Context,
	sink SecretSink,
	request SecretStoreRequest,
	produce SecretProducer,
) (secretcap.Handle, error) {
	if isNilInterface(ctx) || isNilInterface(sink) || produce == nil {
		return secretcap.Handle{}, gerror.New("resource Provider confidential output dependencies are invalid")
	}
	if err := request.Validate(); err != nil {
		return secretcap.Handle{}, err
	}

	var (
		producerCalls    atomic.Int32
		consumerCalls    atomic.Int32
		producerFailed   atomic.Bool
		producerReturned atomic.Bool
		storeReturned    atomic.Bool
	)
	guardedProducer := func(consume SecretConsumer) error {
		if storeReturned.Load() || producerCalls.Add(1) != 1 || consume == nil {
			producerFailed.Store(true)
			return gerror.New("resource Provider confidential output producer contract failed")
		}
		err := produce(func(plaintext []byte) error {
			if producerReturned.Load() || storeReturned.Load() || consumerCalls.Add(1) != 1 {
				clear(plaintext)
				producerFailed.Store(true)
				return gerror.New("resource Provider confidential output consumer contract failed")
			}
			defer clear(plaintext)
			if len(plaintext) == 0 || len(plaintext) > MaximumSecretPlaintextBytes {
				producerFailed.Store(true)
				return gerror.New("resource Provider confidential output plaintext is invalid")
			}
			if err := consume(plaintext); err != nil {
				producerFailed.Store(true)
				return gerror.New("resource Provider confidential output storage failed")
			}
			return nil
		})
		producerReturned.Store(true)
		if err != nil {
			producerFailed.Store(true)
			return gerror.New("resource Provider confidential output production failed")
		}
		return nil
	}

	handle, err := sink.Store(ctx, request, guardedProducer)
	storeReturned.Store(true)
	if err != nil || producerFailed.Load() || producerCalls.Load() != 1 || consumerCalls.Load() != 1 || handle.IsZero() {
		return secretcap.Handle{}, gerror.New("resource Provider confidential output storage failed")
	}
	return handle, nil
}

// ProviderEnv contains only the explicitly authorized capabilities an adapter
// may use. It intentionally does not expose host service directories, database
// handles, resource-owner services, or notification services.
type ProviderEnv struct {
	// PluginID identifies the source plugin that owns the factory.
	PluginID string
	// ProviderID identifies the Provider implementation being constructed.
	ProviderID string
	// Secrets is the core secretcap accessor scoped to this adapter instance.
	Secrets secretcap.Accessor
	// SecretSink is the owner-managed encrypted output boundary.
	SecretSink SecretSink
	// AuthenticationSessions coordinates encrypted, channel-scoped upstream
	// login tokens through resource-owner storage. Adapters must use this
	// boundary instead of process-global or provider-instance token caches.
	AuthenticationSessions AuthenticationSessions
	// HTTPClient applies the resource owner's outbound transport policy.
	HTTPClient HTTPClient
	// Clock supplies testable wall-clock observations.
	Clock Clock
	// Logger receives redacted operational diagnostics.
	Logger Logger
	// Metrics receives bounded adapter measurements without business payloads.
	Metrics Metrics
}

// AuthenticationSessionRequest identifies one channel-scoped cached login.
// TTL is the maximum age accepted by the adapter; owner storage remains the
// authority for generation, expiry, refresh leases, and invalidation.
type AuthenticationSessionRequest struct {
	ChannelRef string
	TTL        time.Duration
}

// AuthenticationSessionInvalidation invalidates only the generation used by
// the failed request. A stale 401 therefore cannot discard a newer login.
type AuthenticationSessionInvalidation struct {
	ChannelRef string
	Generation int64
}

// AuthenticationTokenProducer performs one upstream login and supplies its
// plaintext token synchronously. The producer and owner must clear temporary
// plaintext views after the callback returns.
type AuthenticationTokenProducer func(consume SecretConsumer) error

// AuthenticationTokenConsumer receives one decrypted token and its database
// generation for the duration of the callback only.
type AuthenticationTokenConsumer func(token []byte, generation int64) error

// AuthenticationSessions is the resource-owner shared authentication seam.
// Access coordinates refresh through PostgreSQL and encrypted secretcap
// storage. Invalidate is generation-fenced and idempotent.
type AuthenticationSessions interface {
	Access(
		ctx context.Context,
		request AuthenticationSessionRequest,
		produce AuthenticationTokenProducer,
		consume AuthenticationTokenConsumer,
	) error
	Invalidate(ctx context.Context, request AuthenticationSessionInvalidation) error
}

// HTTPClient is the constrained outbound transport available to Providers.
type HTTPClient interface {
	// Do executes one request using the resource owner's security policy.
	Do(request *http.Request) (*http.Response, error)
}

// Clock supplies time without binding Providers to process-global time calls.
type Clock interface {
	// Now returns the current wall-clock time.
	Now() time.Time
}

// Logger records redacted adapter diagnostics.
type Logger interface {
	// Log records one structured diagnostic at the requested level. Owner
	// wrappers must enforce ValidateLogEntry before forwarding the event.
	Log(ctx context.Context, level LogLevel, message string, fields ...LogField)
}

// LogLevel classifies one Provider diagnostic.
type LogLevel string

// Provider diagnostic levels.
const (
	LogLevelDebug LogLevel = "debug"
	LogLevelInfo  LogLevel = "info"
	LogLevelWarn  LogLevel = "warn"
	LogLevelError LogLevel = "error"
)

// LogField is one non-secret structured diagnostic field.
type LogField struct {
	// Key names the diagnostic field.
	Key string
	// Value contains a redacted diagnostic value.
	Value string
}

// Metrics records bounded Provider counters and observations.
type Metrics interface {
	// AddCounter adds delta to one counter with non-secret labels. Owner wrappers
	// must enforce ValidateMetric before forwarding the measurement.
	AddCounter(ctx context.Context, name string, delta int64, labels ...MetricLabel)
	// Observe records one numeric observation with non-secret labels. Owner
	// wrappers must enforce ValidateMetricObservation before forwarding it.
	Observe(ctx context.Context, name string, value float64, labels ...MetricLabel)
}

// MetricLabel is one non-secret metric dimension.
type MetricLabel struct {
	// Name identifies the metric dimension.
	Name string
	// Value is the bounded dimension value.
	Value string
}
