// This file defines normalized Provider providerCapacity, operation, and evidence values.

package resourcecap

import (
	"encoding/json"
	"strings"
	"time"
)

// ProviderCapacityKind identifies one upstream projection returned through ListProviderCapacity.
type ProviderCapacityKind string

// Normalized upstream providerCapacity kinds.
const (
	ProviderCapacityKindRegion  ProviderCapacityKind = "region"
	ProviderCapacityKindLine    ProviderCapacityKind = "line"
	ProviderCapacityKindPlan    ProviderCapacityKind = "plan"
	ProviderCapacityKindImage   ProviderCapacityKind = "image"
	ProviderCapacityKindStock   ProviderCapacityKind = "stock"
	ProviderCapacityKindCost    ProviderCapacityKind = "cost"
	ProviderCapacityKindBalance ProviderCapacityKind = "balance"
	ProviderCapacityKindExpiry  ProviderCapacityKind = "expiry"
)

// ProviderCapacityRequest requests one bounded cursor page from an upstream projection.
type ProviderCapacityRequest struct {
	// Kind selects the normalized projection.
	Kind ProviderCapacityKind
	// Cursor is an opaque Provider-issued continuation token.
	Cursor string
	// Limit bounds the requested page size.
	Limit int
	// Filters contains adapter-owned non-secret JSON filters.
	Filters json.RawMessage
}

// ProviderCapacityPage is one normalized upstream projection page.
type ProviderCapacityPage struct {
	// Items contains only entries from the requested projection kind.
	Items []ProviderCapacityItem
	// NextCursor resumes the next page when non-empty.
	NextCursor string
	// Complete reports whether this page completes a coherent upstream snapshot.
	Complete bool
	// Evidence contains redacted upstream references supporting the observation.
	Evidence []Evidence
}

// ProviderCapacityItem is one normalized upstream projection entry.
type ProviderCapacityItem struct {
	// Kind identifies the projection represented by Data.
	Kind ProviderCapacityKind
	// ExternalRef is the upstream entry identifier and must never contain credentials.
	ExternalRef string
	// Data contains adapter-normalized non-secret JSON fields.
	Data json.RawMessage
	// ObservedAt records when the adapter observed the entry.
	ObservedAt time.Time
}

// OperationRequest submits one normalized, idempotent upstream action.
type OperationRequest struct {
	// RequestID is the resource-owner idempotency key for this action.
	RequestID string
	// Capability identifies the action to execute.
	Capability CapabilityCode
	// ResourceRef is the upstream resource identifier when the action targets an existing resource.
	ResourceRef string
	// Parameters contains normalized non-secret JSON input.
	Parameters json.RawMessage
	// SecretBindings maps operation-specific inputs to authorized opaque handles.
	SecretBindings []SecretBinding
	// ProviderCorrelation is owner-created callback routing metadata. It never
	// contains the callback token.
	ProviderCorrelation *ProviderCorrelation
}

// TaskQuery requests the latest observation of one external task.
type TaskQuery struct {
	// ExternalTaskRef is the opaque task identifier returned by the Provider.
	ExternalTaskRef string
	// ResourceRef optionally supplies the associated upstream resource identifier.
	ResourceRef string
}

// ResourceQuery requests the latest observation of one upstream resource.
type ResourceQuery struct {
	// ExternalResourceRef is the opaque upstream resource identifier.
	ExternalResourceRef string
}

// TaskStatus is the normalized state of an upstream operation.
type TaskStatus string

// Normalized upstream task states.
const (
	TaskStatusAccepted   TaskStatus = "accepted"
	TaskStatusRunning    TaskStatus = "running"
	TaskStatusSucceeded  TaskStatus = "succeeded"
	TaskStatusFailed     TaskStatus = "failed"
	TaskStatusUnknown    TaskStatus = "unknown"
	TaskStatusCanceled   TaskStatus = "canceled"
	TaskStatusTerminated TaskStatus = "terminated"
)

// IsTerminal reports whether an observed task state has a final outcome.
// Unknown remains reconcilable rather than being treated as terminal.
func (s TaskStatus) IsTerminal() bool {
	switch s {
	case TaskStatusSucceeded, TaskStatusFailed, TaskStatusCanceled, TaskStatusTerminated:
		return true
	default:
		return false
	}
}

// ResourceState is the stable lifecycle projection returned by every
// Provider. Adapter-specific or unrecognized upstream values must be retained
// only as redacted evidence and mapped to ResourceStateUnknown.
type ResourceState string

// Normalized upstream resource states.
const (
	ResourceStatePending    ResourceState = "pending"
	ResourceStateRunning    ResourceState = "running"
	ResourceStateActive     ResourceState = "active"
	ResourceStateStopped    ResourceState = "stopped"
	ResourceStateSuspended  ResourceState = "suspended"
	ResourceStateFailed     ResourceState = "failed"
	ResourceStateTerminated ResourceState = "terminated"
	ResourceStateUnknown    ResourceState = "unknown"
)

// NormalizeResourceState maps a canonical, recognized normalized value to its
// typed state. Every upstream-specific or malformed value fails closed to
// ResourceStateUnknown instead of guessing success or failure.
func NormalizeResourceState(value string) ResourceState {
	if len(value) > maximumMachineValueBytes || containsCredentialMaterial(value) {
		return ResourceStateUnknown
	}
	candidate := ResourceState(strings.TrimSpace(strings.ToLower(value)))
	switch candidate {
	case ResourceStatePending,
		ResourceStateRunning,
		ResourceStateActive,
		ResourceStateStopped,
		ResourceStateSuspended,
		ResourceStateFailed,
		ResourceStateTerminated,
		ResourceStateUnknown:
		return candidate
	default:
		return ResourceStateUnknown
	}
}

// OperationResult is one normalized submission or task observation. A Go error
// reports failure to perform the call; upstream business failure belongs in
// Status and Failure so the resource owner can reconcile deterministically.
type OperationResult struct {
	// Status is the normalized upstream task state.
	Status TaskStatus
	// ExternalTaskRef identifies the upstream asynchronous task when one exists.
	ExternalTaskRef string
	// ExternalResourceRef identifies the affected upstream resource when known.
	ExternalResourceRef string
	// Resource contains a current normalized resource observation when available.
	Resource *ResourceSnapshot
	// Failure describes a redacted upstream business failure when Status is failed or unknown.
	Failure *ProviderFailure
	// Progress is an optional upstream completion percentage from zero through one hundred.
	Progress *int
	// SecretOutputs contains encrypted owner-managed handles, never plaintext.
	SecretOutputs []SecretOutput
	// Evidence contains redacted references supporting the returned result.
	Evidence []Evidence
	// ResidualResources contains only bounded, normalized resources that a
	// terminate operation could not prove reclaimed. Raw Provider payloads and
	// credentials are never allowed in this projection.
	ResidualResources []ResidualResource
}

// ResidualResourceStatus is the normalized reclamation state of one resource
// that remains after a terminate operation.
type ResidualResourceStatus string

// Residual resource statuses are intentionally small and fail closed.
const (
	ResidualResourcePresent        ResidualResourceStatus = "present"
	ResidualResourceReleasePending ResidualResourceStatus = "release_pending"
	ResidualResourceReleaseFailed  ResidualResourceStatus = "release_failed"
	ResidualResourceUnknown        ResidualResourceStatus = "unknown"
)

// ResidualResource is one safe, non-secret reclamation projection.
type ResidualResource struct {
	ResourceKind string
	ResourceRef  string
	Status       ResidualResourceStatus
	ReasonCode   string
}

// ResourceSnapshot is a normalized, non-secret upstream resource observation.
type ResourceSnapshot struct {
	// ExternalResourceRef identifies the observed upstream resource.
	ExternalResourceRef string
	// State is the adapter-normalized running state.
	State ResourceState
	// Data contains normalized non-secret JSON fields.
	Data json.RawMessage
	// ObservedAt records when the adapter observed the resource.
	ObservedAt time.Time
	// Evidence contains redacted references supporting the observation.
	Evidence []Evidence
}

// ProviderFailure describes a redacted upstream operation failure.
type ProviderFailure struct {
	// Code is the adapter-normalized failure code.
	Code string
	// Message is safe for operator diagnostics and must not include credentials.
	Message string
	// Retryable reports whether the same operation may be retried under owner policy.
	Retryable bool
	// ResultUnknown reports that the Provider could not prove success or failure.
	ResultUnknown bool
}

// EvidenceKindHTTPResponse identifies an owner-sanitized bounded response body.
const EvidenceKindHTTPResponse = "upstream_http_response"

// EvidenceKindCallDiagnostic identifies owner-recorded execution coordinates
// when no supplier response body is available. It never implies non-acceptance.
const EvidenceKindCallDiagnostic = "upstream_call_diagnostic"

// Evidence is a redacted reference to an upstream observation or response.
type Evidence struct {
	// Kind classifies the evidence without exposing raw payloads.
	Kind string
	// Reference is an upstream request, task, or resource identifier.
	Reference string
	// Summary contains a bounded redacted diagnostic.
	Summary string
	// ObservedAt records when the evidence was captured.
	ObservedAt time.Time
}
