// Package lifecycle materializes immutable service-expiry work without
// mutating service state or calling a resource Provider from the scan path.
package lifecycle

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"
)

// MaximumScheduleBatch bounds one scheduler or processor invocation.
const MaximumScheduleBatch = 64

// actionCode is the persisted lifecycle work kind.
type actionCode string

// taskStatus is the persisted lifecycle task state.
type taskStatus string

// entitlementStatus is the customer service entitlement projection.
type entitlementStatus string

// runtimeStatus is the local resource runtime projection.
type runtimeStatus string

// providerTaskStatus is the service-level Provider task projection.
type providerTaskStatus string

// operationStatus is the unified business operation state used by lifecycle work.
type operationStatus string

// Internal lifecycle enum values mirror the constrained dictionary and SQL values.
const (
	actionReminder    actionCode = "reminder"
	actionEnterGrace  actionCode = "enter_grace"
	actionMarkExpired actionCode = "mark_expired"
	actionSuspend     actionCode = "suspend"
	actionResume      actionCode = "resume"
	actionTerminate   actionCode = "terminate"

	taskStatusPending      taskStatus = "pending"
	taskStatusClaimed      taskStatus = "claimed"
	taskStatusDispatched   taskStatus = "dispatched"
	taskStatusSucceeded    taskStatus = "succeeded"
	taskStatusFailed       taskStatus = "failed"
	taskStatusCancelled    taskStatus = "cancelled"
	taskStatusManualReview taskStatus = "manual_review"

	entitlementStatusActive     entitlementStatus = "active"
	entitlementStatusGrace      entitlementStatus = "grace"
	entitlementStatusExpired    entitlementStatus = "expired"
	entitlementStatusTerminated entitlementStatus = "terminated"

	runtimeStatusRunning     runtimeStatus = "running"
	runtimeStatusStopped     runtimeStatus = "stopped"
	runtimeStatusSuspended   runtimeStatus = "suspended"
	runtimeStatusTerminating runtimeStatus = "terminating"
	runtimeStatusTerminated  runtimeStatus = "terminated"

	providerTaskStatusPending   providerTaskStatus = "pending"
	providerTaskStatusExecuting providerTaskStatus = "executing"
	providerTaskStatusSucceeded providerTaskStatus = "succeeded"
	providerTaskStatusFailed    providerTaskStatus = "failed"
	providerTaskStatusUnknown   providerTaskStatus = "result_unknown"
	providerTaskStatusManual    providerTaskStatus = "manual_review"

	operationStatusExecuting    operationStatus = "executing"
	operationStatusSucceeded    operationStatus = "succeeded"
	operationStatusFailed       operationStatus = "failed"
	operationStatusManualReview operationStatus = "manual_review"
)

// Scheduler is the narrow contract used by the primary-node lifecycle job.
type Scheduler interface {
	// Run materializes at most limit due lifecycle facts without changing service state.
	Run(ctx context.Context, limit int) (*Batch, error)
}

// Batch summarizes one bounded, set-based scheduling pass.
type Batch struct {
	Selected int
	Created  int64
}

type service struct {
	database gdb.DB
}

type candidate struct {
	ServiceNo                    string     `orm:"service_no"`
	CustomerNo                   string     `orm:"customer_no"`
	CurrencyCode                 string     `orm:"currency_code"`
	PolicyNo                     string     `orm:"policy_no"`
	PolicyVersion                int64      `orm:"policy_version"`
	ActionCode                   actionCode `orm:"action_code"`
	ReminderSequence             *int       `orm:"reminder_sequence"`
	ExpectedServiceVersion       int64      `orm:"expected_service_version"`
	ExpectedEntitlementExpiresAt time.Time  `orm:"expected_entitlement_expires_at"`
	DueAt                        time.Time  `orm:"due_at"`
}

// New constructs a lifecycle scheduler over the plugin database.
func New(database gdb.DB) (Scheduler, error) {
	if database == nil {
		return nil, gerror.New("service lifecycle database is unavailable")
	}
	return &service{database: database}, nil
}

// Run selects at most limit due facts using database time, then inserts their
// immutable task projections in one version-fenced statement. It deliberately
// does not update service state and does not dispatch Provider operations.
func (s *service) Run(ctx context.Context, limit int) (*Batch, error) {
	if s == nil || s.database == nil || ctx == nil || ctx.Err() != nil ||
		limit <= 0 || limit > MaximumScheduleBatch {
		return nil, gerror.New("invalid service lifecycle schedule request")
	}
	candidates := make([]candidate, 0, limit)
	if err := s.database.GetScan(ctx, &candidates, lifecycleCandidatesSQL, limit); err != nil {
		return nil, gerror.Wrap(err, "list due service lifecycle facts")
	}
	report := &Batch{Selected: len(candidates)}
	if len(candidates) == 0 {
		return report, nil
	}
	query, arguments, err := lifecycleInsert(candidates)
	if err != nil {
		return nil, err
	}
	result, err := s.database.Exec(ctx, query, arguments...)
	if err != nil {
		return nil, gerror.Wrap(err, "materialize service lifecycle tasks")
	}
	report.Created, err = result.RowsAffected()
	if err != nil {
		return nil, gerror.Wrap(err, "count materialized service lifecycle tasks")
	}
	return report, nil
}

func lifecycleInsert(candidates []candidate) (string, []any, error) {
	const fieldsPerCandidate = 13
	rows := make([]string, 0, len(candidates))
	arguments := make([]any, 0, len(candidates)*fieldsPerCandidate)
	for _, item := range candidates {
		identityDigest, contentDigest, err := candidateDigests(item)
		if err != nil {
			return "", nil, err
		}
		rows = append(rows, "(?,?,?,?,?,?,?,?,?,?,?,?,?)")
		reminderSequence := 0
		if item.ReminderSequence != nil {
			reminderSequence = *item.ReminderSequence
		}
		arguments = append(arguments,
			"LCT_"+identityDigest[:32], item.ServiceNo, item.CustomerNo,
			item.CurrencyCode, item.PolicyNo, item.PolicyVersion, item.ActionCode,
			reminderSequence, item.ExpectedServiceVersion,
			item.ExpectedEntitlementExpiresAt.UTC(), item.DueAt.UTC(),
			identityDigest, contentDigest,
		)
	}
	return lifecycleInsertPrefix + strings.Join(rows, ",") + lifecycleInsertSuffix,
		arguments, nil
}

func candidateDigests(item candidate) (string, string, error) {
	identityPayload := struct {
		ServiceNo, PolicyNo            string
		ActionCode                     actionCode
		PolicyVersion, ExpiryUnixMicro int64
		ReminderSequence               *int
	}{item.ServiceNo, item.PolicyNo, item.ActionCode, item.PolicyVersion,
		item.ExpectedEntitlementExpiresAt.UTC().UnixMicro(), item.ReminderSequence}
	identity, err := digest(identityPayload)
	if err != nil {
		return "", "", err
	}
	content, err := digest(struct {
		IdentityDigest                         string
		CustomerNo, CurrencyCode               string
		ExpectedServiceVersion, DueAtUnixMicro int64
	}{identity, item.CustomerNo, item.CurrencyCode,
		item.ExpectedServiceVersion, item.DueAt.UTC().UnixMicro()})
	if err != nil {
		return "", "", err
	}
	return identity, content, nil
}

func digest(value any) (string, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return "", gerror.Wrap(err, "encode service lifecycle digest")
	}
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:]), nil
}
