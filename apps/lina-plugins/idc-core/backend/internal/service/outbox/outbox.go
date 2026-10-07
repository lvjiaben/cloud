// Package outbox owns transactional IDC event persistence and reliable, lease-fenced delivery state.
package outbox

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"io"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"
)

// Status is the persisted lifecycle state of an outbox event.
type Status string

// Supported outbox event states.
const (
	StatusPending       Status = "pending"
	StatusProcessing    Status = "processing"
	StatusRetryWait     Status = "retry_wait"
	StatusPublished     Status = "published"
	StatusDeadLetter    Status = "dead_letter"
	StatusResultUnknown Status = "result_unknown"
)

// Config defines bounded delivery behavior. Zero values select safe defaults;
// negative values are rejected so deployment mistakes cannot silently disable bounds.
type Config struct {
	LeaseDuration       time.Duration
	BaseBackoff         time.Duration
	MaximumBackoff      time.Duration
	MaximumClaimBatch   int
	DefaultMaxAttempts  int
	MaximumAttempts     int
	MaximumPayloadSize  int
	MaximumMetadataSize int
}

// EnqueueInput is a versioned, redacted event envelope. DeduplicationKey is
// hashed before persistence and must be stable for the originating business effect.
type EnqueueInput struct {
	EventType        string
	EventVersion     int
	Topic            string
	AggregateType    string
	AggregateNo      string
	CorrelationNo    string
	DeduplicationKey string
	Payload          json.RawMessage
	Metadata         json.RawMessage
	AvailableAt      time.Time
	MaxAttempts      int
}

// Event is the stable service projection of one persisted outbox event.
type Event struct {
	EventNo          string
	EventType        string
	EventVersion     int
	Topic            string
	AggregateType    string
	AggregateNo      string
	CorrelationNo    string
	Status           Status
	Payload          json.RawMessage
	Metadata         json.RawMessage
	OccurredAt       time.Time
	AvailableAt      time.Time
	AttemptCount     int
	MaxAttempts      int
	PublishedAt      *time.Time
	LastErrorCode    string
	LastErrorSummary string
}

// ClaimInput requests an ordered, bounded batch for one worker identity.
type ClaimInput struct {
	WorkerID   string
	Limit      int
	EventTypes []string
}

// ClaimedEvent carries the event plus an opaque claim token. The token must be
// returned for every transition and prevents a stale worker from committing a result.
type ClaimedEvent struct {
	Event
	LeaseToken     string
	AttemptNo      int
	LeaseExpiresAt time.Time
}

// LeaseRef is the complete fencing tuple for one delivery attempt. Every lease
// operation validates event, processing state, token, attempt, and live expiry.
type LeaseRef struct {
	EventNo    string
	LeaseToken string
	AttemptNo  int
}

// FailureInput records a redacted delivery failure. Retryable controls whether
// another attempt may be scheduled; retry exhaustion always produces a dead letter.
type FailureInput struct {
	Lease        LeaseRef
	Retryable    bool
	ErrorCode    string
	ErrorSummary string
}

// UnknownInput records an indeterminate delivery result and freezes automatic
// dispatch. A higher-level audited task workflow must reconcile it before retrying.
type UnknownInput struct {
	Lease        LeaseRef
	ErrorCode    string
	ErrorSummary string
}

// Service defines the plugin-internal transactional event delivery boundary.
type Service interface {
	// Enqueue appends or idempotently reuses an event inside an already active
	// database transaction. Payload and metadata must be redacted JSON objects;
	// invalid input, deduplication mismatch, or missing transaction is rejected.
	Enqueue(ctx context.Context, input EnqueueInput) (*Event, error)
	// Claim atomically leases up to Limit ready events using PostgreSQL skip-locked
	// semantics. A non-empty EventTypes set restricts claiming and exhausted-row
	// convergence to those exact types; an empty set preserves all-event behavior.
	// Claims are ordered, tenant-scoped, and fenced by opaque tokens; zero results
	// are returned as an empty slice.
	Claim(ctx context.Context, input ClaimInput) ([]ClaimedEvent, error)
	// Renew extends a still-live lease without incrementing its attempt. Expired,
	// stale, foreign, or otherwise unfenced attempts are rejected.
	Renew(ctx context.Context, lease LeaseRef) (*ClaimedEvent, error)
	// MarkPublished completes a still-live claim. A stale, foreign, expired, or
	// already-closed lease is rejected so cleared fencing evidence is never guessed.
	MarkPublished(ctx context.Context, lease LeaseRef) (*Event, error)
	// RecordFailure closes the current claim and either schedules bounded
	// exponential backoff or moves the event to dead letter.
	RecordFailure(ctx context.Context, input FailureInput) (*Event, error)
	// MarkResultUnknown closes the current claim in a non-retryable unknown state.
	// It never automatically resubmits an event whose effect cannot be proven.
	MarkResultUnknown(ctx context.Context, input UnknownInput) (*Event, error)
}

// Enqueuer is the transaction-local subset consumed by domain services that
// must atomically append an event to their own explicitly injected database.
type Enqueuer interface {
	Enqueue(ctx context.Context, input EnqueueInput) (*Event, error)
}

// Ensure serviceImpl implements Service.
var _ Service = (*serviceImpl)(nil)

// serviceImpl implements transactional event persistence with generated plugin DAOs.
type serviceImpl struct {
	config   Config
	now      func() time.Time
	random   io.Reader
	database gdb.DB
}

type databaseEnqueuer struct {
	service *serviceImpl
}

var _ Enqueuer = (*databaseEnqueuer)(nil)

func (w *databaseEnqueuer) Enqueue(ctx context.Context, input EnqueueInput) (*Event, error) {
	return w.service.Enqueue(ctx, input)
}

// New validates configuration and creates a stateless outbox service. The
// service owns no worker goroutine; task 2.11 wires polling through pluginhost.Jobs.
func New(config Config) (Service, error) {
	return newService(config, time.Now, rand.Reader)
}

// NewService binds the complete claim and transition lifecycle to an explicit
// database. It is the preferred constructor for plugin jobs and isolated tests.
func NewService(database gdb.DB, config Config) (Service, error) {
	if database == nil {
		return nil, gerror.New("outbox service requires a database")
	}
	service, err := newService(config, time.Now, rand.Reader)
	if err != nil {
		return nil, err
	}
	service.database = database
	return service, nil
}

// NewEnqueuer binds transactional event persistence to the caller's explicit
// database instead of resolving the process-global GoFrame database group.
func NewEnqueuer(database gdb.DB, config Config) (Enqueuer, error) {
	if database == nil {
		return nil, gerror.New("outbox enqueuer requires a database")
	}
	service, err := newService(config, time.Now, rand.Reader)
	if err != nil {
		return nil, err
	}
	service.database = database
	return &databaseEnqueuer{service: service}, nil
}

// newService creates an outbox implementation with explicit deterministic test dependencies.
func newService(config Config, now func() time.Time, random io.Reader) (*serviceImpl, error) {
	normalized, err := normalizeConfig(config)
	if err != nil {
		return nil, err
	}
	if now == nil {
		return nil, gerror.New("outbox service requires a clock")
	}
	if random == nil {
		return nil, gerror.New("outbox service requires a random source")
	}
	return &serviceImpl{config: normalized, now: now, random: random}, nil
}

func (s *serviceImpl) transaction(
	ctx context.Context,
	callback func(context.Context, gdb.TX) error,
) error {
	if s.database != nil {
		return s.database.Transaction(ctx, callback)
	}
	return s.eventModel(ctx).Transaction(ctx, callback)
}
