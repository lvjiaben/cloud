// Package renewalownercap defines the versioned, provider-neutral renewal owner port.
package renewalownercap

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"time"
)

const (
	ContractVersion         = "v1"
	ModeProviderConfirmed   = "provider_confirmed"
	TaskStatusPending       = "pending"
	TaskStatusRunning       = "running"
	TaskStatusSucceeded     = "succeeded"
	TaskStatusFailed        = "failed"
	TaskStatusResultUnknown = "result_unknown"
	TaskStatusManualReview  = "manual_review"
)

var (
	ErrOwnerUnavailable = errors.New("renewal owner is unavailable")
	ErrTaskUnavailable  = errors.New("renewal task is unavailable")
	ErrTaskConflict     = errors.New("renewal task conflicts with frozen facts")
	referencePattern    = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.:-]{0,127}$`)
	digestPattern       = regexp.MustCompile(`^[0-9a-f]{64}$`)
)

type TaskRequest struct {
	Version                    string
	OwnerID                    string
	OperationNo                string
	EntitlementNo              string
	PriceSnapshotNo            string
	PluginID                   string
	PluginReleaseVersion       string
	ContractVersion            string
	DeclarationDigest          string
	RenewalMethod              string
	ExpectedEntitlementVersion int64
	OriginalExpiresAt          time.Time
	ProjectedExpiresAt         time.Time
}

type TaskReadRequest struct {
	TaskNo  string
	Request TaskRequest
}

type Task struct {
	TaskNo         string
	Status         string
	FailureCode    string
	EvidenceDigest string
	ObservedAt     time.Time
	Reused         bool
}

type Source interface {
	CreateTask(context.Context, TaskRequest) (*Task, error)
	ReadTask(context.Context, TaskReadRequest) (*Task, error)
}

type Router interface {
	Resolve(string) (Source, bool)
}

func ValidateTaskRequest(request TaskRequest) error {
	if request.Version != ContractVersion || !validReference(request.OwnerID, 64) || !validReference(request.OperationNo, 40) ||
		!validReference(request.EntitlementNo, 40) || !validReference(request.PriceSnapshotNo, 40) ||
		!validReference(request.PluginID, 64) || !validReference(request.PluginReleaseVersion, 32) ||
		!validReference(request.ContractVersion, 32) || !digestPattern.MatchString(request.DeclarationDigest) ||
		!validReference(request.RenewalMethod, 128) || request.ExpectedEntitlementVersion < 1 ||
		request.OriginalExpiresAt.IsZero() || !request.ProjectedExpiresAt.After(request.OriginalExpiresAt) {
		return ErrTaskConflict
	}
	return nil
}

func ValidateTaskReadRequest(request TaskReadRequest) error {
	if !validReference(request.TaskNo, 64) {
		return ErrTaskConflict
	}
	return ValidateTaskRequest(request.Request)
}

func ValidateTask(task Task) error {
	requiresFailure := task.Status == TaskStatusFailed || task.Status == TaskStatusResultUnknown
	forbidsFailure := task.Status == TaskStatusPending || task.Status == TaskStatusRunning || task.Status == TaskStatusSucceeded
	if !validReference(task.TaskNo, 64) || !digestPattern.MatchString(task.EvidenceDigest) || task.ObservedAt.IsZero() ||
		!validTaskStatus(task.Status) || (task.FailureCode != "" && !validReference(task.FailureCode, 128)) ||
		(requiresFailure && task.FailureCode == "") || (forbidsFailure && task.FailureCode != "") {
		return ErrTaskConflict
	}
	return nil
}

func validReference(value string, maximum int) bool {
	return len(value) <= maximum && value == strings.TrimSpace(value) && referencePattern.MatchString(value)
}

func validTaskStatus(status string) bool {
	switch status {
	case TaskStatusPending, TaskStatusRunning, TaskStatusSucceeded, TaskStatusFailed, TaskStatusResultUnknown, TaskStatusManualReview:
		return true
	default:
		return false
	}
}
