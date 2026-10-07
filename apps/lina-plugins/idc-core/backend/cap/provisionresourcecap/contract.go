// Package provisionresourcecap defines the versioned core-to-resource
// fulfillment provisioning port. Every request is bound to one frozen product
// interface instance and configuration revision before it crosses the owner
// boundary.
package provisionresourcecap

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"regexp"
	"strings"
	"time"

	"github.com/gogf/gf/v2/errors/gerror"
)

const (
	// ContractVersion is the exact resource-owner provisioning protocol version.
	ContractVersion = "v1"
	// OwnerID selects the built-in server resource owner without teaching the
	// generic fulfillment core any server field semantics.
	OwnerID = "idc-resource"

	TaskStatusPending       TaskStatus = "pending"
	TaskStatusRunning       TaskStatus = "running"
	TaskStatusSucceeded     TaskStatus = "succeeded"
	TaskStatusFailed        TaskStatus = "failed"
	TaskStatusResultUnknown TaskStatus = "result_unknown"
	TaskStatusManualReview  TaskStatus = "manual_review"

	OutcomeStatusSucceeded    OutcomeStatus = "succeeded"
	OutcomeStatusFailed       OutcomeStatus = "failed"
	OutcomeStatusManualReview OutcomeStatus = "manual_review"
)

var (
	ErrCapabilityUnavailable = errors.New("resource provisioning capability is unavailable")
	ErrPrincipalUnavailable  = errors.New("resource provisioning principal mapping is unavailable")
	ErrParametersUnavailable = errors.New("resource provisioning parameters are unavailable")
	ErrSourceUnavailable     = errors.New("resource provisioning source is unavailable")
	ErrTaskUnavailable       = errors.New("resource provisioning task is unavailable")
	ErrTaskConflict          = errors.New("resource provisioning task conflicts with its frozen request")

	referencePattern                     = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.:-]{0,127}$`)
	digestPattern                        = regexp.MustCompile(`^[0-9a-f]{64}$`)
	errorCodePattern                     = regexp.MustCompile(`^[a-z][a-z0-9_.-]{0,127}$`)
	interfacePattern                     = regexp.MustCompile(`^pfi_[0-9a-f]{32}$`)
	resourceConfigurationRevisionPattern = regexp.MustCompile(`^rci_[0-9a-f]{32}$`)
)

// TaskStatus is the bounded resource-owner state projected to core.
type TaskStatus string

// OutcomeStatus is the exact core-persisted terminal projection reported back
// to the resource owner after the core transaction commits.
type OutcomeStatus string

// TaskRequest freezes one paid fulfillment intent. It carries neither
// Provider identifiers nor credentials; the resource owner resolves both from
// the exact frozen interface instance and revision.
type TaskRequest struct {
	OperationNo                    string
	CustomerNo                     string
	ConfigurationSnapshotNo        string
	Fulfillment                    FrozenFulfillmentIdentity
	InterfaceInstanceNo            string
	InterfaceConfigurationRevision string
	ProductConfiguration           json.RawMessage
	ProductConfigurationDigest     string
	CustomerConfiguration          json.RawMessage
	RequestDigest                  string
}

// FrozenFulfillmentIdentity is the provider-neutral paid order identity. Core
// forwards it unchanged; the resource owner alone interprets server-specific
// product and customer configuration.
type FrozenFulfillmentIdentity struct {
	IntentNo              string
	OrderNo               string
	OrderItemNo           string
	ProductNo             string
	ProductVersionNo      string
	ConfigurationRevision string
	UnitSequence          int
	Quantity              int
}

// TaskReadRequest binds a resource-owned task to its immutable core request.
type TaskReadRequest struct {
	ProviderTaskNo string
	Request        TaskRequest
}

// Task is the bounded, secret-free resource-owner projection consumed by core.
type Task struct {
	ProviderTaskNo           string
	ChannelNo                string
	Status                   TaskStatus
	FailureCode              string
	CapabilityEvidenceNo     string
	CapabilityEvidenceDigest string
	EvidenceDigest           string
	ObservedAt               time.Time
	CompletedAt              time.Time
	Reused                   bool
}

// OutcomeReport is a secret-free, idempotent acknowledgement that core has
// durably applied one resource task outcome. Resource-owned credential
// lifecycle changes must not happen before this report.
type OutcomeReport struct {
	ProviderTaskNo     string
	Request            TaskRequest
	Status             OutcomeStatus
	TaskEvidenceDigest string
	CoreEvidenceDigest string
}

// Source is implemented by idc-resource and injected at startup.
type Source interface {
	CreateTask(context.Context, TaskRequest) (*Task, error)
	ReadTask(context.Context, TaskReadRequest) (*Task, error)
	ReportOutcome(context.Context, OutcomeReport) error
}

// ValidateTaskRequest rejects unbounded, ambiguous, or unfenced requests.
func ValidateTaskRequest(request TaskRequest) error {
	if !digestPattern.MatchString(request.RequestDigest) {
		return gerror.New("resource provisioning task request digest is invalid")
	}
	if err := validateTaskRequestFields(request); err != nil {
		return err
	}
	digest, err := taskRequestDigest(request)
	if err != nil || subtle.ConstantTimeCompare([]byte(digest), []byte(request.RequestDigest)) != 1 {
		return gerror.New("resource provisioning task request digest is invalid")
	}
	return nil
}

// TaskRequestDigest returns the canonical secret-free digest core must freeze
// before crossing the owner boundary.
func TaskRequestDigest(request TaskRequest) (string, error) {
	if err := validateTaskRequestFields(request); err != nil {
		return "", err
	}
	return taskRequestDigest(request)
}

func validateTaskRequestFields(request TaskRequest) error {
	return validateFrozenFulfillmentTaskRequest(request)
}

func validateFrozenFulfillmentTaskRequest(request TaskRequest) error {
	identity := request.Fulfillment
	if !validReference(request.OperationNo, 40) || request.OperationNo != identity.IntentNo ||
		!validReference(request.CustomerNo, 64) ||
		!validReference(request.ConfigurationSnapshotNo, 64) ||
		!interfacePattern.MatchString(request.InterfaceInstanceNo) ||
		!resourceConfigurationRevisionPattern.MatchString(request.InterfaceConfigurationRevision) ||
		!validReference(identity.IntentNo, 40) || !validReference(identity.OrderNo, 40) ||
		!validReference(identity.OrderItemNo, 40) || !validReference(identity.ProductNo, 40) ||
		!validReference(identity.ProductVersionNo, 40) ||
		strings.TrimSpace(identity.ConfigurationRevision) == "" ||
		identity.UnitSequence < 1 || identity.Quantity < 1 || identity.Quantity > 1000 ||
		identity.UnitSequence > identity.Quantity ||
		!digestPattern.MatchString(request.ProductConfigurationDigest) ||
		!validJSONObject(request.ProductConfiguration) ||
		!validJSONObject(request.CustomerConfiguration) {
		return gerror.New("resource provisioning frozen fulfillment request is invalid")
	}
	return nil
}

func taskRequestDigest(request TaskRequest) (string, error) {
	payload, err := json.Marshal(struct {
		ContractVersion                string
		OperationNo                    string
		CustomerNo                     string
		ConfigurationSnapshotNo        string
		Fulfillment                    FrozenFulfillmentIdentity
		InterfaceInstanceNo            string
		InterfaceConfigurationRevision string
		ProductConfiguration           json.RawMessage
		ProductConfigurationDigest     string
		CustomerConfiguration          json.RawMessage
	}{
		ContractVersion: ContractVersion, OperationNo: request.OperationNo,
		CustomerNo: request.CustomerNo, ConfigurationSnapshotNo: request.ConfigurationSnapshotNo,
		Fulfillment: request.Fulfillment, InterfaceInstanceNo: request.InterfaceInstanceNo,
		InterfaceConfigurationRevision: request.InterfaceConfigurationRevision,
		ProductConfiguration:           request.ProductConfiguration,
		ProductConfigurationDigest:     request.ProductConfigurationDigest,
		CustomerConfiguration:          request.CustomerConfiguration,
	})
	if err != nil {
		return "", gerror.Wrap(err, "encode resource provisioning task request")
	}
	digest := sha256.Sum256(payload)
	return hex.EncodeToString(digest[:]), nil
}

func validJSONObject(value json.RawMessage) bool {
	if len(value) == 0 || len(value) > 1<<20 || !json.Valid(value) {
		return false
	}
	var object map[string]json.RawMessage
	return json.Unmarshal(value, &object) == nil && object != nil
}

// ValidateTaskReadRequest proves one task read is bound to its frozen request.
func ValidateTaskReadRequest(request TaskReadRequest) error {
	if !validReference(request.ProviderTaskNo, 64) || ValidateTaskRequest(request.Request) != nil {
		return gerror.New("resource provisioning task read request is invalid")
	}
	return nil
}

// ValidateTask rejects incomplete or contradictory owner projections.
func ValidateTask(task Task) error {
	requiresFailure := task.Status == TaskStatusFailed || task.Status == TaskStatusResultUnknown ||
		task.Status == TaskStatusManualReview
	forbidsFailure := task.Status == TaskStatusPending || task.Status == TaskStatusRunning ||
		task.Status == TaskStatusSucceeded
	terminal := task.Status == TaskStatusSucceeded || task.Status == TaskStatusFailed ||
		task.Status == TaskStatusManualReview
	if !validReference(task.ProviderTaskNo, 64) || !validReference(task.ChannelNo, 64) ||
		!validReference(task.CapabilityEvidenceNo, 64) ||
		!digestPattern.MatchString(task.CapabilityEvidenceDigest) ||
		!digestPattern.MatchString(task.EvidenceDigest) || task.ObservedAt.IsZero() ||
		!validTaskStatus(task.Status) ||
		(task.FailureCode != "" && !errorCodePattern.MatchString(task.FailureCode)) ||
		(requiresFailure && task.FailureCode == "") || (forbidsFailure && task.FailureCode != "") ||
		(terminal && (task.CompletedAt.IsZero() || task.CompletedAt.Before(task.ObservedAt))) ||
		(!terminal && !task.CompletedAt.IsZero()) {
		return gerror.New("resource provisioning task projection is invalid")
	}
	return nil
}

// ValidateOutcomeReport rejects reports that are not bound to one frozen task
// request and two independently produced evidence digests.
func ValidateOutcomeReport(report OutcomeReport) error {
	if !validReference(report.ProviderTaskNo, 64) ||
		ValidateTaskRequest(report.Request) != nil ||
		!validOutcomeStatus(report.Status) ||
		!digestPattern.MatchString(report.TaskEvidenceDigest) ||
		!digestPattern.MatchString(report.CoreEvidenceDigest) {
		return gerror.New("resource provisioning outcome report is invalid")
	}
	return nil
}

func validOutcomeStatus(status OutcomeStatus) bool {
	switch status {
	case OutcomeStatusSucceeded, OutcomeStatusFailed, OutcomeStatusManualReview:
		return true
	default:
		return false
	}
}

func validReference(value string, maximum int) bool {
	return len(value) <= maximum && value == strings.TrimSpace(value) && referencePattern.MatchString(value)
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
