// Package lifecycleresourcecap defines the versioned core-to-resource port for
// non-financial expiry actions.
package lifecycleresourcecap

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"time"

	"github.com/gogf/gf/v2/errors/gerror"
)

// ContractVersion identifies the immutable core-to-resource wire contract.
const ContractVersion = "v2"

// MaximumResidualResourceCount keeps termination reconciliation bounded.
const MaximumResidualResourceCount = 16

// Action is one resource lifecycle operation owned by the Provider task layer.
type Action string

// TaskStatus is the bounded resource-owner task state projected to core.
type TaskStatus string

// Lifecycle action and task status constants are the only accepted wire values.
const (
	ActionSuspend   Action = "suspend"
	ActionResume    Action = "resume"
	ActionTerminate Action = "terminate"

	TaskStatusPending       TaskStatus = "pending"
	TaskStatusRunning       TaskStatus = "running"
	TaskStatusSucceeded     TaskStatus = "succeeded"
	TaskStatusFailed        TaskStatus = "failed"
	TaskStatusResultUnknown TaskStatus = "result_unknown"
	TaskStatusManualReview  TaskStatus = "manual_review"
)

var (
	// ErrCapabilityUnavailable means no eligible Provider capability could be selected.
	ErrCapabilityUnavailable = errors.New("resource lifecycle capability is unavailable")
	// ErrSourceUnavailable means the resource owner cannot safely accept the request.
	ErrSourceUnavailable = errors.New("resource lifecycle source is unavailable")
	// ErrTaskUnavailable means the requested resource-owned task does not exist.
	ErrTaskUnavailable = errors.New("resource lifecycle task is unavailable")
	// ErrTaskConflict means a task reference conflicts with its immutable request.
	ErrTaskConflict = errors.New("resource lifecycle task conflicts with its frozen request")

	referencePattern   = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.:-]{0,127}$`)
	digestPattern      = regexp.MustCompile(`^[0-9a-f]{64}$`)
	entitlementPattern = regexp.MustCompile(`^ent_[0-9a-f]{32}$`)
	errorCodePattern   = regexp.MustCompile(`^[a-z][a-z0-9_.-]{0,127}$`)
)

// TaskRequest freezes the exact local service fact and Provider route. It
// contains no customer amount, credential, or caller-supplied Provider ID.
type TaskRequest struct {
	LifecycleTaskNo string
	ServiceNo       string
	// CustomerNo selects entitlement-owned suspension/recovery. ServiceNo is
	// then the entitlement number and ChannelNo/PoolNo must be empty: the
	// resource owner resolves them from its authoritative ownership projection.
	// Empty preserves the existing legacy service route contract.
	CustomerNo             string `json:",omitempty"`
	ChannelNo              string
	PoolNo                 string
	Action                 Action
	ExpectedServiceVersion int64
	ExpectedExpiresAt      time.Time
	IdempotencyDigest      string
}

// TaskReadRequest binds a Provider task to its immutable core request.
type TaskReadRequest struct {
	ProviderTaskNo string
	Request        TaskRequest
}

// Task is the bounded resource-owner projection consumed by core.
type Task struct {
	ProviderTaskNo           string
	Status                   TaskStatus
	FailureCode              string
	CapabilityEvidenceNo     string
	CapabilityEvidenceDigest string
	EvidenceDigest           string
	ObservedAt               time.Time
	Reused                   bool
	ResidualResources        []ResidualResource
}

// ResidualResource is one non-secret resource that the Provider could not
// prove reclaimed during termination.
type ResidualResource struct {
	ResourceKind string `json:"resourceKind"`
	ResourceRef  string `json:"resourceRef"`
	Status       string `json:"status"`
	ReasonCode   string `json:"reasonCode"`
}

// Source is implemented by idc-resource and injected at startup.
type Source interface {
	// CreateTask creates or reuses one resource-owned Provider task.
	CreateTask(ctx context.Context, request TaskRequest) (*Task, error)
	// ReadTask returns one bounded task projection bound to the original request.
	ReadTask(ctx context.Context, request TaskReadRequest) (*Task, error)
}

// ValidateTaskRequest rejects malformed or unfenced lifecycle requests.
func ValidateTaskRequest(request TaskRequest) error {
	if !validReference(request.LifecycleTaskNo, 40) || !validReference(request.ServiceNo, 64) ||
		!validAction(request.Action) || request.ExpectedServiceVersion <= 0 ||
		request.ExpectedExpiresAt.IsZero() || !digestPattern.MatchString(request.IdempotencyDigest) {
		return gerror.New("resource lifecycle task request is invalid")
	}
	if request.CustomerNo != "" {
		if !validReference(request.CustomerNo, 40) || !entitlementPattern.MatchString(request.ServiceNo) ||
			request.ChannelNo != "" || request.PoolNo != "" || request.Action == ActionTerminate {
			return gerror.New("resource entitlement lifecycle task request is invalid")
		}
	} else if entitlementPattern.MatchString(request.ServiceNo) || !validReference(request.ChannelNo, 64) || !validReference(request.PoolNo, 64) {
		return gerror.New("resource legacy lifecycle task route is invalid")
	}
	return nil
}

// ValidateTaskReadRequest binds a task read to its original lifecycle request.
func ValidateTaskReadRequest(request TaskReadRequest) error {
	if !validReference(request.ProviderTaskNo, 64) || ValidateTaskRequest(request.Request) != nil {
		return gerror.New("resource lifecycle task read request is invalid")
	}
	return nil
}

// ValidateTask rejects incomplete, contradictory, or unbounded task projections.
func ValidateTask(task Task) error {
	requiresFailure := task.Status == TaskStatusFailed || task.Status == TaskStatusResultUnknown ||
		task.Status == TaskStatusManualReview
	forbidsFailure := task.Status == TaskStatusPending || task.Status == TaskStatusRunning ||
		task.Status == TaskStatusSucceeded
	if !validReference(task.ProviderTaskNo, 64) || !validReference(task.CapabilityEvidenceNo, 64) ||
		!digestPattern.MatchString(task.CapabilityEvidenceDigest) ||
		!digestPattern.MatchString(task.EvidenceDigest) || task.ObservedAt.IsZero() ||
		!validTaskStatus(task.Status) ||
		(task.FailureCode != "" && !errorCodePattern.MatchString(task.FailureCode)) ||
		(requiresFailure && task.FailureCode == "") || (forbidsFailure && task.FailureCode != "") {
		return gerror.New("resource lifecycle task projection is invalid")
	}
	if len(task.ResidualResources) > MaximumResidualResourceCount ||
		(len(task.ResidualResources) != 0 && task.Status != TaskStatusResultUnknown &&
			task.Status != TaskStatusManualReview) {
		return gerror.New("resource lifecycle residual resources are invalid")
	}
	seen := make(map[string]struct{}, len(task.ResidualResources))
	for _, item := range task.ResidualResources {
		if !validMachineValue(item.ResourceKind) || !validReference(item.ResourceRef, 128) ||
			!validResidualStatus(item.Status) || !validMachineValue(item.ReasonCode) {
			return gerror.New("resource lifecycle residual resource is invalid")
		}
		key := item.ResourceKind + "\x00" + item.ResourceRef
		if _, exists := seen[key]; exists {
			return gerror.New("resource lifecycle residual resource is duplicated")
		}
		seen[key] = struct{}{}
	}
	return nil
}

func validReference(value string, maximum int) bool {
	return len(value) <= maximum && value == strings.TrimSpace(value) && referencePattern.MatchString(value)
}

func validAction(action Action) bool {
	return action == ActionSuspend || action == ActionResume || action == ActionTerminate
}

func validTaskStatus(status TaskStatus) bool {
	switch status {
	case TaskStatusPending, TaskStatusRunning, TaskStatusSucceeded, TaskStatusFailed,
		TaskStatusResultUnknown, TaskStatusManualReview:
		return true
	default:
		return false
	}
}

func validMachineValue(value string) bool {
	return len(value) >= 1 && len(value) <= 64 && value == strings.TrimSpace(value) &&
		errorCodePattern.MatchString(value)
}

func validResidualStatus(value string) bool {
	return value == "present" || value == "release_pending" ||
		value == "release_failed" || value == "unknown"
}
