// Package asynctask owns transaction-bound task creation and lease-fenced asynchronous execution state.
package asynctask

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"io"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"
)

// Status is the persisted lifecycle state of an asynchronous task.
type Status string

// Supported asynchronous task states.
const (
	StatusPending        Status = "pending"
	StatusRunning        Status = "running"
	StatusRetryWait      Status = "retry_wait"
	StatusSucceeded      Status = "succeeded"
	StatusFailed         Status = "failed"
	StatusDeadLetter     Status = "dead_letter"
	StatusResultUnknown  Status = "result_unknown"
	StatusManualTakeover Status = "manual_takeover"
)

// AttemptStatus is the persisted result of one execution attempt.
type AttemptStatus string

// Supported asynchronous task attempt states.
const (
	AttemptStatusRunning       AttemptStatus = "running"
	AttemptStatusSucceeded     AttemptStatus = "succeeded"
	AttemptStatusFailed        AttemptStatus = "failed"
	AttemptStatusResultUnknown AttemptStatus = "result_unknown"
)

// Config defines bounded lease, retry, and payload behavior. Zero values use
// safe defaults; negative values and values above safety ceilings are rejected.
type Config struct {
	LeaseDuration      time.Duration
	BaseBackoff        time.Duration
	MaximumBackoff     time.Duration
	MaximumClaimBatch  int
	DefaultMaxAttempts int
	MaximumAttempts    int
	MaximumPayloadSize int
	MaximumResultSize  int
}

// CreateInput describes one redacted asynchronous business task. The raw
// idempotency key is digested before persistence and is never returned.
type CreateInput struct {
	TaskType       string
	SubjectType    string
	SubjectNo      string
	CustomerNo     string
	IdempotencyKey string
	Priority       int
	Payload        json.RawMessage
	AvailableAt    time.Time
	DeadlineAt     *time.Time
	MaxAttempts    int
}

// CreateResult reports whether task creation reused an identical committed intent.
type CreateResult struct {
	Task   Task
	Reused bool
}

// Task is the stable service projection of one asynchronous task.
type Task struct {
	TenantID             int
	TaskNo               string
	TaskType             string
	SubjectType          string
	SubjectNo            string
	CustomerNo           string
	ExternalReferenceNo  string
	Status               Status
	Priority             int
	Payload              json.RawMessage
	ResultSnapshot       json.RawMessage
	AvailableAt          time.Time
	DeadlineAt           *time.Time
	AttemptCount         int
	MaxAttempts          int
	CompletedAt          *time.Time
	ManualTakeoverAt     *time.Time
	ManualAssigneeUserID int
	ManualReason         string
	LastErrorCode        string
	LastErrorSummary     string
}

// ClaimInput requests an ordered, bounded batch for one worker identity.
type ClaimInput struct {
	WorkerID string
	Limit    int
	// TaskTypes restricts a domain worker to its owned task families. Empty
	// retains the generic administrative claim behavior.
	TaskTypes []string
}

// LeaseRef is the complete fencing tuple for one running attempt. Every worker
// transition validates tenant, task, running state, opaque token, attempt, and lease expiry.
type LeaseRef struct {
	TenantID   int
	TaskNo     string
	LeaseToken string
	AttemptNo  int
}

// ClaimedTask carries one task and its current fencing tuple.
type ClaimedTask struct {
	Task
	Lease          LeaseRef
	LeaseExpiresAt time.Time
}

// CompleteInput records a confirmed, redacted successful result.
type CompleteInput struct {
	Lease               LeaseRef
	ExternalReferenceNo string
	ResultSnapshot      json.RawMessage
}

// FailureInput records a confirmed failure. Retryable failures use bounded
// exponential backoff, exhaust into dead letter, and non-retryable failures fail.
type FailureInput struct {
	Lease               LeaseRef
	Retryable           bool
	ExternalReferenceNo string
	ResponseSnapshot    json.RawMessage
	ErrorCode           string
	ErrorSummary        string
}

// UnknownInput freezes a task whose external effect cannot be proven.
type UnknownInput struct {
	Lease               LeaseRef
	ExternalReferenceNo string
	ResponseSnapshot    json.RawMessage
	ErrorCode           string
	ErrorSummary        string
}

// RecoverExpiredInput bounds one expired-lease recovery batch.
type RecoverExpiredInput struct {
	Limit int
	// TaskTypes restricts recovery to the caller's owned families. Empty keeps
	// the administrative all-family behavior; values follow ClaimInput rules.
	TaskTypes []string
}

// TakeOverInput assigns a terminal automated task to a staff operator. The
// trusted caller must first prove AssigneeUserID is an active staff identity
// visible in the current administrative scope. Reason records why ownership
// changed; domain evidence remains in its owning records.
type TakeOverInput struct {
	TenantID       int
	TaskNo         string
	AssigneeUserID int
	Reason         string
}

// Service defines the plugin-internal durable task boundary.
type Service interface {
	// Create appends or idempotently reuses a task inside an already active
	// database transaction. Missing transaction context, invalid input, or a
	// changed request under the same idempotency scope is rejected.
	Create(ctx context.Context, input CreateInput) (*CreateResult, error)
	// CreateInDatabase is the explicit-database variant for a domain service
	// that already owns a transaction. The task is committed or rolled back
	// with that exact database transaction, never through an implicit group.
	CreateInDatabase(ctx context.Context, database gdb.DB, input CreateInput) (*CreateResult, error)
	// Claim atomically leases ready, non-expired tasks using PostgreSQL skip-locked
	// semantics and inserts all new attempt rows in one batch. No work returns an empty slice.
	Claim(ctx context.Context, input ClaimInput) ([]ClaimedTask, error)
	// Renew extends a still-live lease. Expired, stale, foreign, or otherwise
	// unfenced attempts are rejected even before recovery has scanned them.
	Renew(ctx context.Context, lease LeaseRef) (*ClaimedTask, error)
	// Complete closes a still-live attempt as confirmed success and clears its lease.
	Complete(ctx context.Context, input CompleteInput) (*Task, error)
	// CompleteInDatabase is the explicit-database variant used when the successful
	// business state change and task evidence must commit in the same transaction.
	CompleteInDatabase(ctx context.Context, database gdb.DB, input CompleteInput) (*Task, error)
	// Fail closes a still-live attempt as confirmed failure and explicitly chooses
	// retry backoff, dead letter on exhaustion, or terminal failure.
	Fail(ctx context.Context, input FailureInput) (*Task, error)
	// MarkResultUnknown closes a still-live attempt and freezes automatic execution.
	// It never schedules another write whose external effect cannot be proved.
	MarkResultUnknown(ctx context.Context, input UnknownInput) (*Task, error)
	// RecoverExpired closes expired running attempts and freezes their tasks as
	// result unknown. Recovery never interprets an expired write lease as safe retry.
	RecoverExpired(ctx context.Context, input RecoverExpiredInput) ([]Task, error)
	// TakeOver moves failed, dead-letter, or result-unknown work into an internal
	// manual queue with a caller-validated active staff assignee and reason.
	// This internal foundation deliberately does not query host staff state;
	// domain services remain
	// responsible for evidence-based query, retry, success, or termination decisions.
	TakeOver(ctx context.Context, input TakeOverInput) (*Task, error)
}

// Ensure serviceImpl implements Service.
var _ Service = (*serviceImpl)(nil)

// serviceImpl implements the task state machine with generated plugin DAOs.
type serviceImpl struct {
	config Config
	now    func() time.Time
	random io.Reader
}

// New validates configuration and creates a stateless task service. It owns no
// worker goroutine; polling is later registered through pluginhost.Jobs.
func New(config Config) (Service, error) {
	return newService(config, time.Now, rand.Reader)
}

// newService creates an implementation with deterministic test dependencies.
func newService(config Config, now func() time.Time, random io.Reader) (*serviceImpl, error) {
	normalized, err := normalizeConfig(config)
	if err != nil {
		return nil, err
	}
	if now == nil {
		return nil, gerror.New("async task service requires a clock")
	}
	if random == nil {
		return nil, gerror.New("async task service requires a random source")
	}
	return &serviceImpl{config: normalized, now: now, random: random}, nil
}
