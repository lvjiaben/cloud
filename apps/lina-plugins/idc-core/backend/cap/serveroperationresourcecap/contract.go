// Package serveroperationresourcecap defines the source-plugin port used by
// core to ask the resource owner to execute one customer-visible server action.
package serveroperationresourcecap

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

	"lina-plugin-idc-core/backend/cap/secretcap"
)

const ContractVersion = "v1"

type Action string
type TaskStatus string
type SecretOutputPolicy string
type SecretBinding struct {
	Name   string
	Handle secretcap.Handle
}

// SecretOutput is an opaque reference to a confidential result owned by the
// installation secret service. It never contains the credential or ticket.
type SecretOutput struct {
	Name      string
	Policy    SecretOutputPolicy
	Handle    secretcap.Handle
	ExpiresAt time.Time
}

const (
	ActionStart               Action = "start"
	ActionStop                Action = "stop"
	ActionReboot              Action = "reboot"
	ActionResetPassword       Action = "reset_password"
	ActionConsole             Action = "console"
	ActionReinstall           Action = "reinstall"
	ActionResize              Action = "resize"
	ActionChangeIP            Action = "ip_change"
	ActionSnapshotCreate      Action = "snapshot_create"
	ActionSnapshotDelete      Action = "snapshot_delete"
	ActionSnapshotRestore     Action = "snapshot_restore"
	ActionBackupCreate        Action = "backup_create"
	ActionBackupDelete        Action = "backup_delete"
	ActionBackupRestore       Action = "backup_restore"
	ActionSecurityGroupAttach Action = "security_group_attach"
	ActionSecurityGroupDetach Action = "security_group_detach"
	ActionNATCreate           Action = "nat_create"
	ActionNATDelete           Action = "nat_delete"
	ActionVirtualMediaMount   Action = "virtual_media_mount"
	ActionVirtualMediaUnmount Action = "virtual_media_unmount"
	ActionTrafficAdd          Action = "traffic_add"

	TaskStatusPending       TaskStatus = "pending"
	TaskStatusRunning       TaskStatus = "running"
	TaskStatusSucceeded     TaskStatus = "succeeded"
	TaskStatusFailed        TaskStatus = "failed"
	TaskStatusResultUnknown TaskStatus = "result_unknown"
	TaskStatusManualReview  TaskStatus = "manual_review"

	SecretOutputPolicyOneTimeCredential SecretOutputPolicy = "one_time_credential"
	SecretOutputPolicyShortLivedTicket  SecretOutputPolicy = "short_lived_ticket"

	maximumParametersBytes = 16 * 1024
	maximumSecretBindings  = 8
)

var (
	ErrCapabilityUnavailable = errors.New("server operation capability is unavailable")
	ErrSourceUnavailable     = errors.New("server operation resource source is unavailable")
	ErrTaskUnavailable       = errors.New("server operation resource task is unavailable")
	ErrTaskConflict          = errors.New("server operation resource task conflicts with its frozen request")
	ErrOutputUnavailable     = errors.New("server operation confidential output is unavailable")
	ErrOutputConsumed        = errors.New("server operation confidential output was already consumed")
	referencePattern         = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.:-]{0,127}$`)
	digestPattern            = regexp.MustCompile(`^[0-9a-f]{64}$`)
	errorCodePattern         = regexp.MustCompile(`^[a-z][a-z0-9_.-]{0,127}$`)
	namePattern              = regexp.MustCompile(`^[a-z][a-z0-9_]{0,63}$`)
	outputNamePattern        = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_]{0,63}$`)
)

type ConfidentialOutputRequest struct {
	ProviderTaskNo string
	ChannelNo      string
	Name           string
	Policy         SecretOutputPolicy
	Handle         secretcap.Handle
	ExpiresAt      time.Time
}

// ConfidentialOutputConsumer receives a plaintext JSON object only while one
// customer-authorized no-store response is being written. It may write the
// bytes directly to that response, but must not retain, log, cache, audit, or
// include them in errors.
type ConfidentialOutputConsumer func([]byte) error

// ConfidentialOutputRedeemer performs the resource-owner side of one
// customer-authorized reveal without returning plaintext through an ordinary
// service result.
type ConfidentialOutputRedeemer interface {
	RedeemOutput(context.Context, ConfidentialOutputRequest, ConfidentialOutputConsumer) error
}

type TaskRequest struct {
	OperationNo            string
	ServiceNo              string
	ChannelNo              string
	PoolNo                 string
	Action                 Action
	ExpectedServiceVersion int64
	Parameters             json.RawMessage
	SecretBindings         []SecretBinding
	RequestDigest          string
}

type TaskReadRequest struct {
	ProviderTaskNo string
	Request        TaskRequest
}

type Task struct {
	ProviderTaskNo           string
	Status                   TaskStatus
	FailureCode              string
	CapabilityEvidenceNo     string
	CapabilityEvidenceDigest string
	EvidenceDigest           string
	ObservedAt               time.Time
	CompletedAt              time.Time
	SecretOutputs            []SecretOutput
	Reused                   bool
}

type Source interface {
	CreateTask(context.Context, TaskRequest) (*Task, error)
	ReadTask(context.Context, TaskReadRequest) (*Task, error)
}

// OwnershipTransferRequest asks the server business owner to move its exact
// entitlement projection in the same database transaction as the core-owned
// service. It contains no billing, payment, refund, or historical document.
type OwnershipTransferRequest struct {
	TransferNo       string
	ServiceNo        string
	SourceCustomerNo string
	TargetCustomerNo string
}

// OwnershipTransfer is implemented by the server business owner. Core must
// fail the transfer when the owner projection is missing or no longer belongs
// to SourceCustomerNo.
type OwnershipTransfer interface {
	TransferOwnership(context.Context, OwnershipTransferRequest) error
}

func ValidateOwnershipTransferRequest(request OwnershipTransferRequest) error {
	if !validReference(request.TransferNo, 40) || !validReference(request.ServiceNo, 64) ||
		!validReference(request.SourceCustomerNo, 40) || !validReference(request.TargetCustomerNo, 40) ||
		request.SourceCustomerNo == request.TargetCustomerNo {
		return gerror.New("server ownership transfer request is invalid")
	}
	return nil
}

type InputSecretRequest struct {
	ChannelNo      string
	IdempotencyKey string
	BindingName    string
	Produce        secretcap.PlaintextProducer
}

type PreparedInputSecret struct {
	Handle secretcap.Handle
	Reused bool
}

// InputSecretSource is the optional resource-owner boundary used before a
// reset-password task is created. Plaintext exists only inside Produce.
type InputSecretSource interface {
	PrepareInputSecret(context.Context, InputSecretRequest) (*PreparedInputSecret, error)
	RevokeInputSecret(context.Context, string, secretcap.Handle) error
}

type CapabilityRoute struct {
	ServiceNo string
	ChannelNo string
	PoolNo    string
}

// EntitlementRouteRequest asks the resource owner to resolve one exact,
// customer-owned generic entitlement to its active Provider route. Core first
// proves channel and entitlement ownership; the resource owner independently
// rechecks its own projection so neither side reads the other's private tables.
type EntitlementRouteRequest struct {
	ServiceNo  string
	CustomerNo string
}

type EntitlementRoute struct {
	ServiceNo string
	ChannelNo string
	PoolNo    string
}

type EntitlementRouteDirectory interface {
	ResolveEntitlementRoute(context.Context, EntitlementRouteRequest) (*EntitlementRoute, error)
}

type CapabilityProjection struct {
	ServiceNo  string
	Actions    []Action
	ObservedAt time.Time
}

// CapabilityDirectory lets core obtain one bounded, resource-owned action
// projection without reading resource tables or querying one Provider per row.
type CapabilityDirectory interface {
	ListCapabilities(context.Context, []CapabilityRoute) ([]CapabilityProjection, error)
}

// ParameterValidator lets core fail an invalid customer request before it
// creates an operation intent. The resource owner remains authoritative for
// the frozen Provider capability snapshot and parameter constraints.
type ParameterValidator interface {
	ValidateParameters(context.Context, CapabilityRoute, Action, json.RawMessage) error
}

// SSHPublicKeyInjectionGate is an optional resource-owner capability boundary.
// Core must call it before resolving any customer public-key material. A missing
// implementation or a rejected request means the product/Provider combination
// does not support SSH injection, so the UI and command path must fail closed.
type SSHPublicKeyInjectionGate interface {
	AllowSSHPublicKeyInjection(context.Context, CapabilityRoute, Action, string) error
}

type ReinstallImage struct {
	Reference  string
	Name       string
	Group      string
	OSType     string
	Arch       string
	BootMode   string
	Selectable bool
}

type ReinstallImageProjection struct {
	ServiceNo  string
	Images     []ReinstallImage
	ObservedAt time.Time
}

type AccessoryKind string
type AccessoryState string

const (
	AccessoryKindIP            AccessoryKind = "ip"
	AccessoryKindDisk          AccessoryKind = "disk"
	AccessoryKindSnapshot      AccessoryKind = "snapshot"
	AccessoryKindBackup        AccessoryKind = "backup"
	AccessoryKindSecurityGroup AccessoryKind = "security_group"
	AccessoryKindNAT           AccessoryKind = "nat"
	AccessoryKindVirtualMedia  AccessoryKind = "virtual_media"
	AccessoryKindTraffic       AccessoryKind = "traffic"
)

type AccessoryItem struct {
	ObjectNo   string
	Kind       AccessoryKind
	Name       string
	State      AccessoryState
	Projection json.RawMessage
}

type AccessoryProjection struct {
	ServiceNo  string
	SnapshotNo string
	ObservedAt time.Time
	ExpiresAt  time.Time
	Actions    []Action
	Items      []AccessoryItem
}

type AccessoryDirectory interface {
	GetAccessories(context.Context, CapabilityRoute) (*AccessoryProjection, error)
}

// ReinstallDirectory publishes only bounded, field-whitelisted image choices
// from the resource owner's last complete channel snapshot.
type ReinstallDirectory interface {
	GetReinstallImages(context.Context, CapabilityRoute) (*ReinstallImageProjection, error)
}

func ValidateInputSecretRequest(request InputSecretRequest) error {
	if !validReference(request.ChannelNo, 64) || len(request.IdempotencyKey) < 8 ||
		len(request.IdempotencyKey) > 256 || request.IdempotencyKey != strings.TrimSpace(request.IdempotencyKey) ||
		request.BindingName != "password" || request.Produce == nil {
		return gerror.New("server operation input secret request is invalid")
	}
	return nil
}

func TaskRequestDigest(request TaskRequest) (string, error) {
	if err := validateTaskRequestFields(request); err != nil {
		return "", err
	}
	bindings := make([]struct{ Name, Ref string }, len(request.SecretBindings))
	for index, binding := range request.SecretBindings {
		bindings[index].Name = binding.Name
		bindings[index].Ref = binding.Handle.OpaqueRef()
	}
	payload, err := json.Marshal(struct {
		ContractVersion                           string
		OperationNo, ServiceNo, ChannelNo, PoolNo string
		Action                                    Action
		ExpectedServiceVersion                    int64
		Parameters                                json.RawMessage
		SecretBindings                            []struct{ Name, Ref string }
	}{ContractVersion, request.OperationNo, request.ServiceNo, request.ChannelNo,
		request.PoolNo, request.Action, request.ExpectedServiceVersion,
		request.Parameters, bindings})
	if err != nil {
		return "", gerror.Wrap(err, "encode server operation request")
	}
	digest := sha256.Sum256(payload)
	return hex.EncodeToString(digest[:]), nil
}

func ValidateTaskRequest(request TaskRequest) error {
	if !digestPattern.MatchString(request.RequestDigest) {
		return gerror.New("server operation request digest is invalid")
	}
	digest, err := TaskRequestDigest(request)
	if err != nil || subtle.ConstantTimeCompare([]byte(digest), []byte(request.RequestDigest)) != 1 {
		return gerror.New("server operation request digest is invalid")
	}
	return nil
}

func validateTaskRequestFields(request TaskRequest) error {
	if !validReference(request.OperationNo, 40) || !validReference(request.ServiceNo, 64) ||
		!validReference(request.ChannelNo, 64) || !validReference(request.PoolNo, 64) ||
		!validAction(request.Action) || request.ExpectedServiceVersion <= 0 ||
		len(request.Parameters) == 0 || len(request.Parameters) > maximumParametersBytes ||
		!validJSONObject(request.Parameters) || len(request.SecretBindings) > maximumSecretBindings {
		return gerror.New("server operation request is invalid")
	}
	seen := make(map[string]struct{}, len(request.SecretBindings))
	for _, binding := range request.SecretBindings {
		if !namePattern.MatchString(binding.Name) || binding.Handle.IsZero() {
			return gerror.New("server operation secret binding is invalid")
		}
		if _, exists := seen[binding.Name]; exists {
			return gerror.New("server operation secret binding is duplicated")
		}
		seen[binding.Name] = struct{}{}
	}
	return nil
}

func ValidateTaskReadRequest(request TaskReadRequest) error {
	if !validReference(request.ProviderTaskNo, 64) || ValidateTaskRequest(request.Request) != nil {
		return gerror.New("server operation task read request is invalid")
	}
	return nil
}

func ValidateTask(task Task) error {
	requiresFailure := task.Status == TaskStatusFailed || task.Status == TaskStatusResultUnknown || task.Status == TaskStatusManualReview
	forbidsFailure := task.Status == TaskStatusPending || task.Status == TaskStatusRunning || task.Status == TaskStatusSucceeded
	terminal := task.Status == TaskStatusSucceeded || task.Status == TaskStatusFailed || task.Status == TaskStatusManualReview
	if !validReference(task.ProviderTaskNo, 64) || !validReference(task.CapabilityEvidenceNo, 64) ||
		!digestPattern.MatchString(task.CapabilityEvidenceDigest) || !digestPattern.MatchString(task.EvidenceDigest) ||
		task.ObservedAt.IsZero() || !validTaskStatus(task.Status) ||
		(task.FailureCode != "" && !errorCodePattern.MatchString(task.FailureCode)) ||
		(requiresFailure && task.FailureCode == "") || (forbidsFailure && task.FailureCode != "") ||
		(terminal && task.CompletedAt.IsZero()) || (!terminal && !task.CompletedAt.IsZero()) {
		return gerror.New("server operation task projection is invalid")
	}
	return validateTaskOutputs(task)
}

func validateTaskOutputs(task Task) error {
	if len(task.SecretOutputs) > maximumSecretBindings {
		return gerror.New("server operation task confidential outputs are invalid")
	}
	seen := make(map[string]struct{}, len(task.SecretOutputs))
	for _, output := range task.SecretOutputs {
		if !outputNamePattern.MatchString(output.Name) || output.Handle.IsZero() {
			return gerror.New("server operation task confidential output is invalid")
		}
		if _, exists := seen[output.Name]; exists {
			return gerror.New("server operation task confidential output is duplicated")
		}
		seen[output.Name] = struct{}{}
		switch output.Policy {
		case SecretOutputPolicyOneTimeCredential:
			if !output.ExpiresAt.IsZero() {
				return gerror.New("server operation one-time credential expiry is invalid")
			}
		case SecretOutputPolicyShortLivedTicket:
			if output.ExpiresAt.IsZero() || !output.ExpiresAt.After(task.ObservedAt) {
				return gerror.New("server operation short-lived ticket expiry is invalid")
			}
		default:
			return gerror.New("server operation task confidential output policy is invalid")
		}
	}
	if task.Status != TaskStatusSucceeded && len(task.SecretOutputs) != 0 {
		return gerror.New("unfinished server operation cannot publish confidential outputs")
	}
	return nil
}

// ValidateTaskForRequest binds confidential output policy to the action that
// produced it. Generic task validation alone cannot make that decision.
func ValidateTaskForRequest(task Task, request TaskRequest) error {
	if ValidateTaskRequest(request) != nil || ValidateTask(task) != nil {
		return gerror.New("server operation task and request are invalid")
	}
	if task.Status != TaskStatusSucceeded {
		if len(task.SecretOutputs) != 0 {
			return gerror.New("unfinished server operation published confidential outputs")
		}
		return nil
	}
	switch request.Action {
	case ActionConsole:
		if len(task.SecretOutputs) != 1 || task.SecretOutputs[0].Name != "consoleTicket" ||
			task.SecretOutputs[0].Policy != SecretOutputPolicyShortLivedTicket {
			return gerror.New("server console output is invalid")
		}
	case ActionReinstall:
		if len(task.SecretOutputs) != 1 || task.SecretOutputs[0].Name != "generatedCredential" ||
			task.SecretOutputs[0].Policy != SecretOutputPolicyOneTimeCredential {
			return gerror.New("server reinstall output is invalid")
		}
	default:
		if len(task.SecretOutputs) != 0 {
			return gerror.New("server operation published an unexpected confidential output")
		}
	}
	return nil
}

func validJSONObject(value json.RawMessage) bool {
	var object map[string]any
	return json.Unmarshal(value, &object) == nil && object != nil
}

func validReference(value string, maximum int) bool {
	return len(value) <= maximum && value == strings.TrimSpace(value) && referencePattern.MatchString(value)
}

func validAction(action Action) bool {
	switch action {
	case ActionStart, ActionStop, ActionReboot, ActionResetPassword, ActionConsole, ActionReinstall, ActionResize, ActionChangeIP,
		ActionSnapshotCreate, ActionSnapshotDelete, ActionSnapshotRestore,
		ActionBackupCreate, ActionBackupDelete, ActionBackupRestore,
		ActionSecurityGroupAttach, ActionSecurityGroupDetach, ActionNATCreate, ActionNATDelete,
		ActionVirtualMediaMount, ActionVirtualMediaUnmount, ActionTrafficAdd:
		return true
	default:
		return false
	}
}

func validTaskStatus(status TaskStatus) bool {
	switch status {
	case TaskStatusPending, TaskStatusRunning, TaskStatusSucceeded, TaskStatusFailed, TaskStatusResultUnknown, TaskStatusManualReview:
		return true
	default:
		return false
	}
}
