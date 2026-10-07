// Package audit owns append-only, redacted IDC business audit evidence.
package audit

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"io"
	"net/netip"
	"reflect"
	"time"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/plugin/capability/authcap/authz"
	"lina-core/pkg/plugin/capability/bizctxcap"
	"lina-plugin-idc-core/backend/internal/service/principal"
)

// ActorType is the trusted origin class persisted with an audit event.
type ActorType string

// Supported trusted actor types. Business inputs never select these values.
const (
	ActorUser     ActorType = "user"
	ActorSystem   ActorType = "system"
	ActorExternal ActorType = "external"
)

// ResultCode is the persisted outcome of an audited operation.
type ResultCode string

// Supported audit results.
const (
	ResultSucceeded     ResultCode = "succeeded"
	ResultDenied        ResultCode = "denied"
	ResultFailed        ResultCode = "failed"
	ResultResultUnknown ResultCode = "result_unknown"
)

// RiskCode is the persisted risk classification of an audited operation.
type RiskCode string

// Supported audit risk classifications.
const (
	RiskLow      RiskCode = "low"
	RiskMedium   RiskCode = "medium"
	RiskHigh     RiskCode = "high"
	RiskCritical RiskCode = "critical"
)

// IntegrityKey is one named HMAC key. Event-signing key IDs are persisted with
// each event, while the reservation-lookup key ID identifies deployment
// configuration only. Secret is copied into service-owned memory and is never
// persisted. Every cryptographic purpose requires independent key material.
type IntegrityKey struct {
	ID     string
	Secret []byte
}

// Config defines finite audit evidence, proxy, reader, and integrity bounds.
// ReservationLookupKey is an installation-stable, independently purposed HMAC
// key for locating idempotency reservations across replay-key rotation. It must
// remain available for at least the complete business-number retention period.
// ActiveReplayDigestKey signs new request/reference fingerprints while retained
// replay keys verify old events. The independent integrity keyring signs and
// verifies complete rows.
type Config struct {
	ReservationLookupKey     IntegrityKey
	ActiveReplayDigestKey    IntegrityKey
	RetainedReplayDigestKeys []IntegrityKey
	ActiveIntegrityKey       IntegrityKey
	RetainedIntegrityKeys    []IntegrityKey
	TrustedProxyCIDRs        []string
	MaximumSnapshotBytes     int
	MaximumSnapshotDepth     int
	MaximumSnapshotNodes     int
	MaximumPageSize          int
	MaximumResponseBytes     int
}

// RequestEvidence contains raw request metadata accepted only for immediate
// normalization and keyed hashing. Raw IP and user-agent values are never persisted.
type RequestEvidence struct {
	RemoteAddress string
	ForwardedFor  string
	UserAgent     string
	RequestID     string
	TraceID       string
}

// Fields contains caller-owned business semantics. Actor identity is deliberately
// absent and is always resolved from a trusted runtime context.
type Fields struct {
	IdempotencyScope   string
	IdempotencyKey     string
	OperationCode      string
	TargetType         string
	TargetNo           string
	CustomerNo         string
	Result             ResultCode
	ReasonCode         string
	Risk               RiskCode
	RequiredPermission string
	CorrelationNo      string
	ContextSnapshot    any
	Request            RequestEvidence
}

// TransitionInput describes one before/after state transition. Both snapshots
// must be JSON objects and are versioned, bounded, and recursively redacted.
type TransitionInput struct {
	Fields
	BeforeSnapshot any
	AfterSnapshot  any
}

// BatchTransitionInput records one bounded state transition with immutable,
// customer-searchable links. CustomerNo in Fields must remain empty.
type BatchTransitionInput struct {
	Fields
	AffectedCustomerNos []string
	BeforeSnapshot      any
	AfterSnapshot       any
}

// AppendResult identifies a newly appended or safely replayed audit event.
type AppendResult struct {
	AuditNo  string
	Replayed bool
}

// ObservationInput describes non-transition evidence such as a denied read or
// security observation. Snapshot is stored as the versioned after-state evidence.
type ObservationInput struct {
	Fields
	Snapshot any
}

// Event is the stable plugin-internal projection of one audit row.
type Event struct {
	AuditNo           string
	OccurredAt        time.Time
	ActorType         ActorType
	ActorUserID       int
	ActingUserID      int
	ActorAudienceCode principal.AudienceCode
	ActorCustomerNo   string
	CustomerNo        string
	OperationCode     string
	TargetType        string
	TargetNo          string
	Result            ResultCode
	ReasonCode        string
	Risk              RiskCode
	BeforeSnapshot    json.RawMessage
	AfterSnapshot     json.RawMessage
	ContextSnapshot   json.RawMessage
	RequestID         string
	TraceID           string
	CorrelationNo     string
	IPDigest          string
	UserAgentDigest   string
}

// Cursor is an internal keyset cursor. InternalID is never an external business identifier.
type Cursor struct {
	OccurredAt time.Time
	InternalID int64
}

// SearchInput applies bounded indexed filters to the management audit timeline.
type SearchInput struct {
	ActorUserID   int
	ActingUserID  int
	CustomerNo    string
	OperationCode string
	TargetType    string
	TargetNo      string
	Result        ResultCode
	Risk          RiskCode
	CorrelationNo string
	OccurredFrom  *time.Time
	OccurredUntil *time.Time
	Cursor        *Cursor
	Limit         int
}

// Page is one bounded keyset page. A nil Next cursor means the page is complete.
type Page struct {
	Events []Event
	Next   *Cursor
}

// Writer defines append-only audit persistence. It intentionally exposes no
// update, delete, replace, or mutable status operation.
type Writer interface {
	// AppendTransition writes or safely replays one event inside the caller's
	// already-active authoritative database transaction.
	AppendTransition(ctx context.Context, input TransitionInput) (*Event, error)
	// AppendBatchTransition writes one event plus immutable customer lookup
	// links without a per-customer audit loop.
	AppendBatchTransition(ctx context.Context, input BatchTransitionInput) (*AppendResult, error)
	// RecordObservation writes or safely replays one observation in a short
	// transaction owned by the audit service.
	RecordObservation(ctx context.Context, input ObservationInput) (*Event, error)
}

// Reader defines fail-closed management audit search.
type Reader interface {
	// Search requires an active staff audience, the exact audit read permission,
	// and a data scope representable by the audit table before querying.
	Search(ctx context.Context, input SearchInput) (*Page, error)
}

// Service combines the append-only writer and bounded reader contracts.
type Service interface {
	Writer
	Reader
}

// TrustedActorResolver is an internal trust seam for runtime-owned system or
// external identities. The default constructor installs a resolver that only
// accepts authenticated user identities from bizctx and principal bindings.
type TrustedActorResolver interface {
	ResolveTrustedActor(ctx context.Context) (TrustedActor, error)
}

// TrustedActor is produced only by TrustedActorResolver, never by business input.
type TrustedActor struct {
	Type                 ActorType
	Reference            string
	UserID               int
	ActingUserID         int
	AudienceCode         principal.AudienceCode
	CustomerNo           string
	DataScope            int
	DataScopeUnsupported bool
	PlatformBypass       bool
	Delegated            bool
}

// Ensure serviceImpl implements Service.
var _ Service = (*serviceImpl)(nil)

// serviceImpl implements trusted actor resolution, append-only persistence, and scoped reads.
type serviceImpl struct {
	config         Config
	actorResolver  TrustedActorResolver
	authorization  authz.Service
	trustedProxies []netip.Prefix
	replayKeys     map[string][]byte
	integrityKeys  map[string][]byte
	now            func() time.Time
	random         io.Reader
}

// New creates the production audit service. Its default resolver deliberately
// cannot synthesize system or external actors from headers or caller input.
func New(
	bizCtxSvc bizctxcap.Service,
	principalSvc principal.Reader,
	authorization authz.Service,
	config Config,
) (Service, error) {
	resolver, err := newUserActorResolver(bizCtxSvc, principalSvc)
	if err != nil {
		return nil, err
	}
	return newService(resolver, authorization, config, time.Now, rand.Reader)
}

// NewWithTrustedActorResolver is the explicit internal runtime composition seam
// for host-verified system or external actors. Request handlers and business
// inputs must never implement this trust boundary from headers or payload fields.
func NewWithTrustedActorResolver(
	actorResolver TrustedActorResolver,
	authorization authz.Service,
	config Config,
) (Service, error) {
	return newService(actorResolver, authorization, config, time.Now, rand.Reader)
}

// newService creates an implementation with explicit internal trust, clock,
// and randomness seams for runtime composition and deterministic tests.
func newService(
	actorResolver TrustedActorResolver,
	authorization authz.Service,
	config Config,
	now func() time.Time,
	random io.Reader,
) (*serviceImpl, error) {
	if isNilInterface(actorResolver) {
		return nil, gerror.New("audit service requires a trusted actor resolver")
	}
	if isNilInterface(authorization) {
		return nil, gerror.New("audit service requires host authorization service")
	}
	normalized, proxies, replayKeys, integrityKeys, err := normalizeConfig(config)
	if err != nil {
		return nil, err
	}
	if isNilInterface(now) {
		return nil, gerror.New("audit service requires a clock")
	}
	if isNilInterface(random) {
		return nil, gerror.New("audit service requires a random source")
	}
	return &serviceImpl{
		config:         normalized,
		actorResolver:  actorResolver,
		authorization:  authorization,
		trustedProxies: proxies,
		replayKeys:     replayKeys,
		integrityKeys:  integrityKeys,
		now:            now,
		random:         random,
	}, nil
}

// isNilInterface rejects typed-nil runtime dependencies before they can panic
// inside a request path.
func isNilInterface(value any) bool {
	if value == nil {
		return true
	}
	reflected := reflect.ValueOf(value)
	switch reflected.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return reflected.IsNil()
	default:
		return false
	}
}
