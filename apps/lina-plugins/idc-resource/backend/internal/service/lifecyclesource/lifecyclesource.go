// Package lifecyclesource implements the resource-owner side of expiry actions.
package lifecyclesource

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/cap/lifecycleresourcecap"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
	"lina-plugin-idc-resource/backend/internal/service/resourcetask"
)

// Service implements the resource-owner side of lifecycle Provider tasks.
type Service struct {
	database gdb.DB
	tasks    *resourcetask.Store
}

var _ lifecycleresourcecap.Source = (*Service)(nil)

// New constructs a lifecycle source using the resource plugin's authoritative database.
func New(database gdb.DB) (*Service, error) {
	if database == nil {
		return nil, gerror.New("resource lifecycle database is required")
	}
	tasks, err := resourcetask.NewStore(database)
	if err != nil {
		return nil, err
	}
	return &Service{database: database, tasks: tasks}, nil
}

type taskParameters struct {
	ContractVersion          string                      `json:"contractVersion"`
	CustomerNo               string                      `json:"customerNo,omitempty"`
	LifecycleTaskNo          string                      `json:"lifecycleTaskNo"`
	Action                   lifecycleresourcecap.Action `json:"action"`
	ExpectedServiceVersion   int64                       `json:"expectedServiceVersion"`
	ExpectedExpiresUnixNano  int64                       `json:"expectedExpiresUnixNano"`
	IdempotencyDigest        [sha256.Size]byte           `json:"idempotencyDigest"`
	CapabilityEvidenceNo     string                      `json:"capabilityEvidenceNo"`
	CapabilityEvidenceDigest [sha256.Size]byte           `json:"capabilityEvidenceDigest"`
}

type taskRow struct {
	TaskNo              string     `orm:"task_no"`
	RequestDigest       string     `orm:"request_id_digest"`
	CapabilityCode      string     `orm:"capability_code"`
	LocalResourceNo     string     `orm:"local_resource_no"`
	ChannelNo           string     `orm:"channel_no"`
	PoolNo              string     `orm:"pool_no"`
	StatusCode          string     `orm:"status_code"`
	SanitizedParameters string     `orm:"sanitized_parameters"`
	OwnerMetadata       string     `orm:"owner_metadata"`
	FailureCode         string     `orm:"failure_code"`
	ResultUnknown       bool       `orm:"result_unknown"`
	Version             int64      `orm:"version"`
	FinishedAt          *time.Time `orm:"finished_at"`
	UpdatedAt           time.Time  `orm:"updated_at"`
}

type routeEvidence struct {
	ChannelNo      string    `orm:"channel_no"`
	PoolNo         string    `orm:"pool_no"`
	MappingNo      string    `orm:"mapping_no"`
	SnapshotNo     string    `orm:"snapshot_no"`
	AdapterVersion string    `orm:"adapter_version"`
	SupportCode    string    `orm:"support_code"`
	Constraints    string    `orm:"constraints"`
	ChannelVersion int64     `orm:"channel_version"`
	PoolVersion    int64     `orm:"pool_version"`
	MappingVersion int64     `orm:"mapping_version"`
	ItemCount      int       `orm:"item_count"`
	ActualCount    int       `orm:"actual_count"`
	ObservedAt     time.Time `orm:"observed_at"`
}

// CreateTask creates or reuses one Provider task after freezing route capability evidence.
func (s *Service) CreateTask(
	ctx context.Context,
	request lifecycleresourcecap.TaskRequest,
) (*lifecycleresourcecap.Task, error) {
	if s == nil || s.database == nil || s.tasks == nil || ctx == nil || ctx.Err() != nil ||
		lifecycleresourcecap.ValidateTaskRequest(request) != nil {
		return nil, lifecycleresourcecap.ErrSourceUnavailable
	}
	request, err := s.resolveEntitlementRoute(ctx, request)
	if err != nil {
		return nil, err
	}
	requestID := "core.lifecycle." + request.LifecycleTaskNo
	existing, err := s.loadTaskByRequest(ctx, request.ChannelNo, requestID)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		result, readErr := s.projectTask(ctx, existing, request)
		if result != nil {
			result.Reused = true
		}
		return result, readErr
	}
	capability, ok := lifecycleCapability(request.Action)
	if !ok {
		return nil, lifecycleresourcecap.ErrCapabilityUnavailable
	}
	evidence, err := s.resolveEvidence(ctx, request, capability)
	if err != nil {
		return nil, err
	}
	parameters, err := encodeParameters(request, evidence)
	if err != nil {
		return nil, err
	}
	created, err := s.tasks.Create(ctx, resourcetask.CreateInput{
		RequestID: requestID, Capability: capability,
		LocalResourceNo: request.ServiceNo, ChannelNo: request.ChannelNo,
		PoolNo: request.PoolNo, Parameters: providerParameters(request.Action),
		OwnerMetadata: parameters, Priority: 100, MaxAttempts: 8,
	})
	if err != nil {
		return nil, errors.Join(lifecycleresourcecap.ErrSourceUnavailable,
			gerror.Wrap(err, "create Provider lifecycle task"))
	}
	row, err := s.loadTask(ctx, created.TaskNo, request.ChannelNo)
	if err != nil {
		return nil, err
	}
	result, err := s.projectTask(ctx, row, request)
	if result != nil {
		result.Reused = !created.Created
	}
	return result, err
}

// ReadTask projects a resource-owned task only when its immutable core request still matches.
func (s *Service) ReadTask(
	ctx context.Context,
	request lifecycleresourcecap.TaskReadRequest,
) (*lifecycleresourcecap.Task, error) {
	if s == nil || s.database == nil || ctx == nil || ctx.Err() != nil ||
		lifecycleresourcecap.ValidateTaskReadRequest(request) != nil {
		return nil, lifecycleresourcecap.ErrSourceUnavailable
	}
	normalized, err := s.resolveEntitlementRoute(ctx, request.Request)
	if err != nil {
		return nil, err
	}
	request.Request = normalized
	row, err := s.loadTask(ctx, request.ProviderTaskNo, request.Request.ChannelNo)
	if err != nil {
		return nil, err
	}
	return s.projectTask(ctx, row, request.Request)
}

func (s *Service) resolveEvidence(
	ctx context.Context,
	request lifecycleresourcecap.TaskRequest,
	capability resourcecap.CapabilityCode,
) (*taskParameters, error) {
	var row *routeEvidence
	if err := s.database.GetScan(ctx, &row, `
WITH latest AS (
    SELECT snapshot_no, channel_no, adapter_version, item_count, observed_at
    FROM plugin_idc_resource_capability_snapshot
    WHERE tenant_id = ? AND channel_no = ? AND status_code = 'complete'
    ORDER BY observed_at DESC, id DESC
    LIMIT 1
)
SELECT channel.channel_no, pool.pool_no, mapping.mapping_no,
       latest.snapshot_no, latest.adapter_version, item.support_code,
       item.constraints::TEXT AS constraints,
       channel.version AS channel_version, pool.version AS pool_version,
       mapping.version AS mapping_version, latest.item_count,
       counts.actual_count, latest.observed_at
FROM latest
INNER JOIN plugin_idc_resource_channel channel
    ON channel.tenant_id = ? AND channel.channel_no = latest.channel_no
   AND channel.status_code = 'enabled' AND channel.deleted_at IS NULL
INNER JOIN plugin_idc_resource_pool pool
    ON pool.tenant_id = channel.tenant_id AND pool.channel_no = channel.channel_no
   AND pool.pool_no = ? AND pool.status_code = 'enabled' AND pool.deleted_at IS NULL
INNER JOIN plugin_idc_resource_mapping mapping
    ON mapping.tenant_id = channel.tenant_id AND mapping.channel_no = channel.channel_no
   AND mapping.pool_no = pool.pool_no AND mapping.local_resource_no = ?
   AND mapping.status_code = 'active' AND mapping.external_resource_ref <> ''
INNER JOIN LATERAL (
    SELECT COUNT(*)::INT AS actual_count
    FROM plugin_idc_resource_capability_snapshot_item candidate
    WHERE candidate.tenant_id = channel.tenant_id
      AND candidate.snapshot_no = latest.snapshot_no
) counts ON TRUE
INNER JOIN plugin_idc_resource_capability_snapshot_item item
    ON item.tenant_id = channel.tenant_id AND item.snapshot_no = latest.snapshot_no
   AND item.capability_code = ?`, idccap.DefaultTenantID, request.ChannelNo,
		idccap.DefaultTenantID, request.PoolNo, request.ServiceNo, capability); err != nil {
		return nil, gerror.Wrap(err, "resolve resource lifecycle capability")
	}
	if row == nil || row.ChannelNo != request.ChannelNo || row.PoolNo != request.PoolNo ||
		row.ItemCount != resourcecap.MaximumCapabilityCount ||
		row.ActualCount != resourcecap.MaximumCapabilityCount ||
		!usableLifecycleCapability(request.Action, row.SupportCode, row.Constraints) ||
		row.ObservedAt.IsZero() {
		return nil, lifecycleresourcecap.ErrCapabilityUnavailable
	}
	nowValue, err := s.database.GetValue(ctx, "SELECT CURRENT_TIMESTAMP")
	if err != nil {
		return nil, gerror.Wrap(err, "read resource lifecycle database time")
	}
	now := nowValue.Time().UTC()
	if now.IsZero() || row.ObservedAt.After(now) ||
		!row.ObservedAt.Add(24*time.Hour).After(now) {
		return nil, lifecycleresourcecap.ErrCapabilityUnavailable
	}
	digest, err := evidenceDigest(request, capability, row)
	if err != nil {
		return nil, err
	}
	idempotencyBytes, decodeErr := decodeDigest(request.IdempotencyDigest)
	if decodeErr != nil {
		return nil, decodeErr
	}
	capabilityBytes, decodeErr := decodeDigest(digest)
	if decodeErr != nil {
		return nil, decodeErr
	}
	return &taskParameters{
		ContractVersion: lifecycleresourcecap.ContractVersion,
		CustomerNo:      request.CustomerNo,
		LifecycleTaskNo: request.LifecycleTaskNo, Action: request.Action,
		ExpectedServiceVersion:  request.ExpectedServiceVersion,
		ExpectedExpiresUnixNano: request.ExpectedExpiresAt.UTC().UnixNano(),
		IdempotencyDigest:       idempotencyBytes,
		CapabilityEvidenceNo:    row.SnapshotNo, CapabilityEvidenceDigest: capabilityBytes,
	}, nil
}

func (s *Service) loadTaskByRequest(
	ctx context.Context,
	channelNo string,
	requestID string,
) (*taskRow, error) {
	digest := sha256.Sum256([]byte(requestID))
	var row *taskRow
	if err := s.database.GetScan(ctx, &row, lifecycleTaskSelect+`
WHERE tenant_id = ? AND channel_no = ? AND request_id_digest = ?`,
		idccap.DefaultTenantID, channelNo, hex.EncodeToString(digest[:])); err != nil {
		return nil, gerror.Wrap(err, "read idempotent Provider lifecycle task")
	}
	return row, nil
}

func (s *Service) loadTask(
	ctx context.Context,
	taskNo string,
	channelNo string,
) (*taskRow, error) {
	var row *taskRow
	if err := s.database.GetScan(ctx, &row, lifecycleTaskSelect+`
WHERE tenant_id = ? AND task_no = ? AND channel_no = ?`,
		idccap.DefaultTenantID, taskNo, channelNo); err != nil {
		return nil, gerror.Wrap(err, "read Provider lifecycle task")
	}
	return row, nil
}

func (s *Service) projectTask(
	ctx context.Context,
	row *taskRow,
	request lifecycleresourcecap.TaskRequest,
) (*lifecycleresourcecap.Task, error) {
	if row == nil || row.TaskNo == "" {
		return nil, lifecycleresourcecap.ErrTaskUnavailable
	}
	parameters, err := decodeParameters(row.OwnerMetadata)
	if err != nil || !sameIntent(row, request, parameters) {
		return nil, lifecycleresourcecap.ErrTaskConflict
	}
	status, failure := projectStatus(row)
	residuals, err := resourcetask.LoadResidualResources(ctx, s.database, row.TaskNo)
	if err != nil {
		return nil, err
	}
	projectedResiduals := make([]lifecycleresourcecap.ResidualResource, len(residuals))
	for index := range residuals {
		projectedResiduals[index] = lifecycleresourcecap.ResidualResource{
			ResourceKind: residuals[index].ResourceKind, ResourceRef: residuals[index].ResourceRef,
			Status: string(residuals[index].Status), ReasonCode: residuals[index].ReasonCode,
		}
	}
	evidence, err := taskEvidenceDigest(row, projectedResiduals)
	if err != nil {
		return nil, err
	}
	result := &lifecycleresourcecap.Task{
		ProviderTaskNo: row.TaskNo, Status: status, FailureCode: failure,
		CapabilityEvidenceNo:     parameters.CapabilityEvidenceNo,
		CapabilityEvidenceDigest: hex.EncodeToString(parameters.CapabilityEvidenceDigest[:]),
		EvidenceDigest:           evidence, ObservedAt: row.UpdatedAt.UTC(),
		ResidualResources: projectedResiduals,
	}
	if lifecycleresourcecap.ValidateTask(*result) != nil {
		return nil, lifecycleresourcecap.ErrTaskConflict
	}
	return result, nil
}

const lifecycleTaskSelect = `
SELECT task_no, request_id_digest, capability_code, local_resource_no,
       channel_no, COALESCE(pool_no, '') AS pool_no, status_code,
       sanitized_parameters::TEXT AS sanitized_parameters,
       owner_metadata::TEXT AS owner_metadata,
       failure_code, result_unknown, version, finished_at, updated_at
FROM plugin_idc_resource_task`

func encodeParameters(
	request lifecycleresourcecap.TaskRequest,
	parameters *taskParameters,
) (json.RawMessage, error) {
	if parameters == nil || parameters.LifecycleTaskNo != request.LifecycleTaskNo {
		return nil, lifecycleresourcecap.ErrTaskConflict
	}
	encoded, err := json.Marshal(parameters)
	if err != nil {
		return nil, gerror.Wrap(err, "encode Provider lifecycle task")
	}
	return encoded, nil
}

func decodeParameters(encoded string) (*taskParameters, error) {
	var parameters taskParameters
	if err := json.Unmarshal([]byte(encoded), &parameters); err != nil {
		return nil, err
	}
	return &parameters, nil
}

func sameIntent(
	row *taskRow,
	request lifecycleresourcecap.TaskRequest,
	parameters *taskParameters,
) bool {
	capability, ok := lifecycleCapability(request.Action)
	idempotencyDigest, err := decodeDigest(request.IdempotencyDigest)
	return ok && err == nil && parameters != nil &&
		row.CapabilityCode == string(capability) && row.LocalResourceNo == request.ServiceNo &&
		row.ChannelNo == request.ChannelNo && row.PoolNo == request.PoolNo &&
		parameters.ContractVersion == lifecycleresourcecap.ContractVersion &&
		parameters.CustomerNo == request.CustomerNo &&
		parameters.LifecycleTaskNo == request.LifecycleTaskNo && parameters.Action == request.Action &&
		parameters.ExpectedServiceVersion == request.ExpectedServiceVersion &&
		parameters.ExpectedExpiresUnixNano == request.ExpectedExpiresAt.UTC().UnixNano() &&
		parameters.IdempotencyDigest == idempotencyDigest &&
		parameters.CapabilityEvidenceNo != "" &&
		parameters.CapabilityEvidenceDigest != [sha256.Size]byte{} &&
		bytes.Equal(canonicalProviderParameters(row.SanitizedParameters),
			providerParameters(request.Action))
}

func decodeDigest(value string) ([sha256.Size]byte, error) {
	var result [sha256.Size]byte
	decoded, err := hex.DecodeString(value)
	if err != nil || len(decoded) != sha256.Size {
		return result, lifecycleresourcecap.ErrTaskConflict
	}
	copy(result[:], decoded)
	return result, nil
}

func lifecycleCapability(action lifecycleresourcecap.Action) (resourcecap.CapabilityCode, bool) {
	switch action {
	case lifecycleresourcecap.ActionSuspend:
		return resourcecap.CapabilitySuspend, true
	case lifecycleresourcecap.ActionResume:
		return resourcecap.CapabilityResume, true
	case lifecycleresourcecap.ActionTerminate:
		return resourcecap.CapabilityTerminate, true
	default:
		return "", false
	}
}

func usableLifecycleCapability(
	action lifecycleresourcecap.Action,
	support string,
	value string,
) bool {
	var constraints []resourcecap.ParameterConstraint
	if json.Unmarshal([]byte(value), &constraints) != nil {
		return false
	}
	if support == string(resourcecap.SupportLevelSupported) {
		return len(constraints) == 0
	}
	if action != lifecycleresourcecap.ActionSuspend ||
		support != string(resourcecap.SupportLevelConditional) || len(constraints) != 1 {
		return false
	}
	constraint := constraints[0]
	if constraint.Name != "reason" || constraint.Required {
		return false
	}
	for _, allowed := range constraint.AllowedValues {
		if allowed == "due" {
			return true
		}
	}
	return len(constraint.AllowedValues) == 0
}

func providerParameters(action lifecycleresourcecap.Action) json.RawMessage {
	if action == lifecycleresourcecap.ActionSuspend {
		return json.RawMessage(`{"reason":"due"}`)
	}
	return json.RawMessage(`{}`)
}

func canonicalProviderParameters(encoded string) json.RawMessage {
	var value map[string]any
	if json.Unmarshal([]byte(encoded), &value) != nil || value == nil {
		return nil
	}
	canonical, err := json.Marshal(value)
	if err != nil {
		return nil
	}
	return canonical
}

func evidenceDigest(
	request lifecycleresourcecap.TaskRequest,
	capability resourcecap.CapabilityCode,
	row *routeEvidence,
) (string, error) {
	return digest(struct {
		Contract, ServiceNo, ChannelNo, PoolNo, MappingNo, SnapshotNo string
		AdapterVersion, Capability, Support, Constraints              string
		ChannelVersion, PoolVersion, MappingVersion                   int64
		ObservedUnixMicro                                             int64
	}{"idc-resource/lifecycle-capability/v1", request.ServiceNo,
		request.ChannelNo, request.PoolNo, row.MappingNo, row.SnapshotNo,
		row.AdapterVersion, string(capability), row.SupportCode, row.Constraints,
		row.ChannelVersion, row.PoolVersion, row.MappingVersion,
		row.ObservedAt.UTC().UnixMicro()})
}

func taskEvidenceDigest(row *taskRow,
	residuals []lifecycleresourcecap.ResidualResource,
) (string, error) {
	finished := int64(0)
	if row.FinishedAt != nil {
		finished = row.FinishedAt.UTC().UnixMicro()
	}
	return digest(struct {
		Contract, TaskNo, RequestDigest, Status, FailureCode string
		ResultUnknown                                        bool
		Version, FinishedUnixMicro, UpdatedUnixMicro         int64
		ResidualResources                                    []lifecycleresourcecap.ResidualResource
	}{"idc-resource/lifecycle-task/v2", row.TaskNo, row.RequestDigest,
		row.StatusCode, row.FailureCode, row.ResultUnknown, row.Version,
		finished, row.UpdatedAt.UTC().UnixMicro(), residuals})
}

func digest(value any) (string, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return "", gerror.Wrap(err, "encode resource lifecycle evidence")
	}
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:]), nil
}

func projectStatus(row *taskRow) (lifecycleresourcecap.TaskStatus, string) {
	switch resourcetask.Status(row.StatusCode) {
	case resourcetask.StatusPending, resourcetask.StatusRetryWait:
		return lifecycleresourcecap.TaskStatusPending, ""
	case resourcetask.StatusSubmitting, resourcetask.StatusAccepted, resourcetask.StatusRunning:
		return lifecycleresourcecap.TaskStatusRunning, ""
	case resourcetask.StatusSucceeded:
		return lifecycleresourcecap.TaskStatusSucceeded, ""
	case resourcetask.StatusFailed, resourcetask.StatusCanceled:
		return lifecycleresourcecap.TaskStatusFailed, failureCode(row, "resource.task_failed")
	case resourcetask.StatusResultUnknown:
		return lifecycleresourcecap.TaskStatusResultUnknown, failureCode(row, "resource.result_unknown")
	case resourcetask.StatusManualTakeover:
		return lifecycleresourcecap.TaskStatusManualReview, failureCode(row, "resource.manual_review")
	default:
		return lifecycleresourcecap.TaskStatusManualReview, "resource.task_invalid"
	}
}

func failureCode(row *taskRow, fallback string) string {
	if row != nil && row.FailureCode != "" {
		return strings.ToLower(row.FailureCode)
	}
	return fallback
}
