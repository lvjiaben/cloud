// Package serveroperationsource implements the resource-owner side of normal
// customer and administrator server actions.
package serveroperationsource

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/cap/secretcap"
	"lina-plugin-idc-core/backend/cap/serveroperationresourcecap"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
	"lina-plugin-idc-resource/backend/internal/composition"
	"lina-plugin-idc-resource/backend/internal/service/resourcetask"
)

type Service struct {
	database gdb.DB
	tasks    *resourcetask.Store
	secrets  secretcap.Source
}

var _ serveroperationresourcecap.Source = (*Service)(nil)
var _ serveroperationresourcecap.ParameterValidator = (*Service)(nil)
var _ serveroperationresourcecap.OwnershipTransfer = (*Service)(nil)

func New(database gdb.DB) (*Service, error) {
	return NewWithSecrets(database, nil)
}

func NewWithSecrets(database gdb.DB, secrets secretcap.Source) (*Service, error) {
	if database == nil {
		return nil, gerror.New("server operation database is required")
	}
	tasks, err := resourcetask.NewStore(database)
	if err != nil {
		return nil, err
	}
	return &Service{database: database, tasks: tasks, secrets: secrets}, nil
}

// TransferOwnership updates only the resource-owner projection. The caller
// invokes this inside the core service transfer transaction, so a missing or
// stale projection rolls back both owners instead of exposing split ownership.
func (s *Service) TransferOwnership(ctx context.Context, request serveroperationresourcecap.OwnershipTransferRequest) error {
	if s == nil || s.database == nil || ctx == nil || ctx.Err() != nil ||
		serveroperationresourcecap.ValidateOwnershipTransferRequest(request) != nil {
		return serveroperationresourcecap.ErrSourceUnavailable
	}
	result, err := s.database.Exec(ctx, `UPDATE plugin_idc_resource_server_entitlement_projection
SET customer_no=?,version=version+1,updated_at=CURRENT_TIMESTAMP
WHERE tenant_id=0 AND entitlement_no=? AND customer_no=?`,
		request.TargetCustomerNo, request.ServiceNo, request.SourceCustomerNo)
	if err != nil {
		return gerror.Wrap(err, "transfer resource server ownership")
	}
	affected, err := result.RowsAffected()
	if err != nil || affected != 1 {
		return serveroperationresourcecap.ErrTaskConflict
	}
	return nil
}

var _ serveroperationresourcecap.InputSecretSource = (*Service)(nil)

func (s *Service) PrepareInputSecret(ctx context.Context, request serveroperationresourcecap.InputSecretRequest) (*serveroperationresourcecap.PreparedInputSecret, error) {
	if s == nil || s.secrets == nil || ctx == nil || ctx.Err() != nil ||
		serveroperationresourcecap.ValidateInputSecretRequest(request) != nil {
		return nil, serveroperationresourcecap.ErrSourceUnavailable
	}
	manager, err := composition.BindProviderSecretManager(s.secrets, request.ChannelNo)
	if err != nil {
		return nil, serveroperationresourcecap.ErrSourceUnavailable
	}
	created, err := manager.Create(ctx, "server-operation:"+request.IdempotencyKey, request.Produce)
	if err != nil || created.Handle.IsZero() {
		return nil, serveroperationresourcecap.ErrSourceUnavailable
	}
	return &serveroperationresourcecap.PreparedInputSecret{Handle: created.Handle, Reused: created.Reused}, nil
}

func (s *Service) RevokeInputSecret(ctx context.Context, channelNo string, handle secretcap.Handle) error {
	if s == nil || s.secrets == nil || ctx == nil || ctx.Err() != nil || handle.IsZero() {
		return serveroperationresourcecap.ErrSourceUnavailable
	}
	manager, err := composition.BindProviderSecretManager(s.secrets, channelNo)
	if err != nil {
		return serveroperationresourcecap.ErrSourceUnavailable
	}
	metadata, err := manager.Metadata(ctx, handle)
	if err != nil {
		return serveroperationresourcecap.ErrSourceUnavailable
	}
	if !metadata.Configured {
		return nil
	}
	if _, err = manager.Revoke(ctx, handle, metadata.UpdatedAt); err != nil {
		return serveroperationresourcecap.ErrSourceUnavailable
	}
	return nil
}

type taskMetadata struct {
	ContractVersion            string                            `json:"contractVersion"`
	OperationNo                string                            `json:"operationNo"`
	Action                     serveroperationresourcecap.Action `json:"action"`
	ExpectedServiceVersion     int64                             `json:"expectedServiceVersion"`
	RequestDigest              [sha256.Size]byte                 `json:"requestDigest"`
	TranslatedParametersDigest [sha256.Size]byte                 `json:"translatedParametersDigest"`
	CapabilityEvidenceNo       string                            `json:"capabilityEvidenceNo"`
	CapabilityEvidenceDigest   [sha256.Size]byte                 `json:"capabilityEvidenceDigest"`
}

type taskRow struct {
	TaskNo              string     `orm:"task_no"`
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
	SecretOutputs       []serveroperationresourcecap.SecretOutput
}

type secretOutputRow struct {
	BindingName  string     `orm:"binding_name"`
	SecretRef    string     `orm:"secret_ref"`
	OutputPolicy string     `orm:"output_policy"`
	ExpiresAt    *time.Time `orm:"expires_at"`
}

type routeEvidence struct {
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

func (s *Service) CreateTask(ctx context.Context, request serveroperationresourcecap.TaskRequest) (*serveroperationresourcecap.Task, error) {
	if s == nil || s.database == nil || s.tasks == nil || ctx == nil || ctx.Err() != nil || serveroperationresourcecap.ValidateTaskRequest(request) != nil {
		return nil, serveroperationresourcecap.ErrSourceUnavailable
	}
	requestID := "core.server-operation." + request.OperationNo
	existing, err := s.loadTaskByRequest(ctx, request.ChannelNo, requestID)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		result, projectErr := projectTask(existing, request)
		if result != nil {
			result.Reused = true
		}
		return result, projectErr
	}
	capability, ok := capabilityFor(request.Action)
	if !ok {
		return nil, serveroperationresourcecap.ErrCapabilityUnavailable
	}
	if !validSecretBindings(request) {
		return nil, serveroperationresourcecap.ErrCapabilityUnavailable
	}
	translatedParameters, err := s.translateAccessoryParameters(ctx, request.ServiceNo, request.Action, request.Parameters)
	if err != nil {
		return nil, err
	}
	providerRequest := request
	providerRequest.Parameters = translatedParameters
	evidence, err := s.resolveEvidence(ctx, providerRequest, capability)
	if err != nil {
		return nil, err
	}
	requestDigest, err := decodeDigest(request.RequestDigest)
	if err != nil {
		return nil, serveroperationresourcecap.ErrTaskConflict
	}
	metadata := taskMetadata{ContractVersion: serveroperationresourcecap.ContractVersion, OperationNo: request.OperationNo, Action: request.Action, ExpectedServiceVersion: request.ExpectedServiceVersion, RequestDigest: requestDigest, CapabilityEvidenceNo: evidence.SnapshotNo}
	translatedDigest, err := digestJSON(translatedParameters)
	if err != nil {
		return nil, err
	}
	metadata.TranslatedParametersDigest, err = decodeDigest(translatedDigest)
	if err != nil {
		return nil, err
	}
	capabilityEvidenceDigest, err := digest(struct {
		Contract, ServiceNo, ChannelNo, PoolNo, MappingNo, SnapshotNo, AdapterVersion, Capability, Support, Constraints string
		ChannelVersion, PoolVersion, MappingVersion                                                                     int64
		ObservedUnixMicro                                                                                               int64
	}{"idc-resource/server-operation-capability/v1", request.ServiceNo, request.ChannelNo, request.PoolNo, evidence.MappingNo, evidence.SnapshotNo, evidence.AdapterVersion, string(capability), evidence.SupportCode, evidence.Constraints, evidence.ChannelVersion, evidence.PoolVersion, evidence.MappingVersion, evidence.ObservedAt.UTC().UnixMicro()})
	if err != nil {
		return nil, err
	}
	metadata.CapabilityEvidenceDigest, err = decodeDigest(capabilityEvidenceDigest)
	if err != nil {
		return nil, err
	}
	ownerMetadata, err := json.Marshal(metadata)
	if err != nil {
		return nil, gerror.Wrap(err, "encode server operation metadata")
	}
	bindings := make([]resourcecap.SecretBinding, len(request.SecretBindings))
	for index, binding := range request.SecretBindings {
		bindings[index] = resourcecap.SecretBinding{Name: binding.Name, Handle: binding.Handle}
	}
	created, err := s.tasks.Create(ctx, resourcetask.CreateInput{RequestID: requestID, Capability: capability, LocalResourceNo: request.ServiceNo, ChannelNo: request.ChannelNo, PoolNo: request.PoolNo, Parameters: translatedParameters, OwnerMetadata: ownerMetadata, Priority: 100, MaxAttempts: 8, SecretBindings: bindings})
	if err != nil {
		return nil, errors.Join(serveroperationresourcecap.ErrSourceUnavailable, gerror.Wrap(err, "create server operation Provider task"))
	}
	row, err := s.loadTask(ctx, created.TaskNo, request.ChannelNo)
	if err != nil {
		return nil, err
	}
	result, err := projectTask(row, request)
	if result != nil {
		result.Reused = !created.Created
	}
	return result, err
}

func (s *Service) ReadTask(ctx context.Context, request serveroperationresourcecap.TaskReadRequest) (*serveroperationresourcecap.Task, error) {
	if s == nil || s.database == nil || ctx == nil || ctx.Err() != nil || serveroperationresourcecap.ValidateTaskReadRequest(request) != nil {
		return nil, serveroperationresourcecap.ErrSourceUnavailable
	}
	row, err := s.loadTask(ctx, request.ProviderTaskNo, request.Request.ChannelNo)
	if err != nil {
		return nil, err
	}
	return projectTask(row, request.Request)
}

func (s *Service) resolveEvidence(ctx context.Context, request serveroperationresourcecap.TaskRequest, capability resourcecap.CapabilityCode) (*routeEvidence, error) {
	var row *routeEvidence
	err := s.database.GetScan(ctx, &row, `
WITH latest AS (
 SELECT snapshot_no, channel_no, adapter_version, item_count, observed_at
 FROM plugin_idc_resource_capability_snapshot
 WHERE tenant_id = ? AND channel_no = ? AND status_code = 'complete'
 ORDER BY observed_at DESC, id DESC LIMIT 1
)
SELECT mapping.mapping_no, latest.snapshot_no, latest.adapter_version,
 item.support_code, item.constraints::TEXT AS constraints,
 channel.version AS channel_version, pool.version AS pool_version,
 mapping.version AS mapping_version, latest.item_count, counts.actual_count,
 latest.observed_at
FROM latest
JOIN plugin_idc_resource_channel channel
 ON channel.tenant_id = ? AND channel.channel_no = latest.channel_no
 AND channel.status_code = 'enabled' AND channel.deleted_at IS NULL
JOIN plugin_idc_resource_pool pool
 ON pool.tenant_id = channel.tenant_id AND pool.channel_no = channel.channel_no
 AND pool.pool_no = ? AND pool.status_code = 'enabled' AND pool.deleted_at IS NULL
JOIN plugin_idc_resource_mapping mapping
 ON mapping.tenant_id = channel.tenant_id AND mapping.channel_no = channel.channel_no
 AND mapping.pool_no = pool.pool_no AND mapping.local_resource_no = ?
 AND mapping.status_code = 'active' AND mapping.external_resource_ref <> ''
JOIN LATERAL (
 SELECT COUNT(*)::INT AS actual_count
 FROM plugin_idc_resource_capability_snapshot_item candidate
 WHERE candidate.tenant_id = channel.tenant_id AND candidate.snapshot_no = latest.snapshot_no
) counts ON TRUE
JOIN plugin_idc_resource_capability_snapshot_item item
 ON item.tenant_id = channel.tenant_id AND item.snapshot_no = latest.snapshot_no
 AND item.capability_code = ?`, idccap.DefaultTenantID, request.ChannelNo, idccap.DefaultTenantID, request.PoolNo, request.ServiceNo, capability)
	if err != nil {
		return nil, gerror.Wrap(err, "resolve server operation capability")
	}
	if row == nil || row.ItemCount != resourcecap.MaximumCapabilityCount || row.ActualCount != resourcecap.MaximumCapabilityCount || row.ObservedAt.IsZero() || validateCapabilityParameters(row.SupportCode, row.Constraints, request.Parameters) != nil {
		return nil, serveroperationresourcecap.ErrCapabilityUnavailable
	}
	nowValue, err := s.database.GetValue(ctx, "SELECT CURRENT_TIMESTAMP")
	if err != nil {
		return nil, gerror.Wrap(err, "read server operation database time")
	}
	now := nowValue.Time().UTC()
	if row.ObservedAt.After(now) || !row.ObservedAt.Add(24*time.Hour).After(now) {
		return nil, serveroperationresourcecap.ErrCapabilityUnavailable
	}
	return row, nil
}

func (s *Service) ValidateParameters(ctx context.Context, route serveroperationresourcecap.CapabilityRoute,
	action serveroperationresourcecap.Action, parameters json.RawMessage) error {
	if s == nil || s.database == nil || ctx == nil || ctx.Err() != nil ||
		!validDirectoryReference(route.ServiceNo, 40) || !validDirectoryReference(route.ChannelNo, 64) ||
		!validDirectoryReference(route.PoolNo, 64) || len(parameters) == 0 || len(parameters) > 16*1024 {
		return serveroperationresourcecap.ErrSourceUnavailable
	}
	capability, ok := capabilityFor(action)
	if !ok {
		return serveroperationresourcecap.ErrCapabilityUnavailable
	}
	translated, err := s.translateAccessoryParameters(ctx, route.ServiceNo, action, parameters)
	if err != nil {
		return err
	}
	_, err = s.resolveEvidence(ctx, serveroperationresourcecap.TaskRequest{
		ServiceNo: route.ServiceNo, ChannelNo: route.ChannelNo, PoolNo: route.PoolNo,
		Action: action, Parameters: translated,
	}, capability)
	return err
}

type parameterConstraintWire struct {
	Name          string   `json:"name"`
	Required      bool     `json:"required"`
	AllowedValues []string `json:"allowedValues"`
	Minimum       *int64   `json:"minimum"`
	Maximum       *int64   `json:"maximum"`
	Pattern       string   `json:"pattern"`
}

func validateCapabilityParameters(support, encodedConstraints string, parameters json.RawMessage) error {
	if support != string(resourcecap.SupportLevelSupported) && support != string(resourcecap.SupportLevelConditional) {
		return serveroperationresourcecap.ErrCapabilityUnavailable
	}
	decoder := json.NewDecoder(bytes.NewBufferString(encodedConstraints))
	decoder.DisallowUnknownFields()
	var wires []parameterConstraintWire
	if err := decoder.Decode(&wires); err != nil || len(wires) > 64 {
		return serveroperationresourcecap.ErrCapabilityUnavailable
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return serveroperationresourcecap.ErrCapabilityUnavailable
	}
	if support == string(resourcecap.SupportLevelSupported) && len(wires) != 0 {
		return serveroperationresourcecap.ErrCapabilityUnavailable
	}
	var values map[string]json.RawMessage
	if json.Unmarshal(parameters, &values) != nil {
		return serveroperationresourcecap.ErrTaskConflict
	}
	if len(values) > len(wires) {
		return serveroperationresourcecap.ErrTaskConflict
	}
	seen := make(map[string]struct{}, len(wires))
	for _, wire := range wires {
		constraint := resourcecap.ParameterConstraint{Name: wire.Name, Required: wire.Required, AllowedValues: wire.AllowedValues, Minimum: wire.Minimum, Maximum: wire.Maximum, Pattern: wire.Pattern}
		if constraint.Validate() != nil {
			return serveroperationresourcecap.ErrCapabilityUnavailable
		}
		if _, exists := seen[constraint.Name]; exists {
			return serveroperationresourcecap.ErrCapabilityUnavailable
		}
		seen[constraint.Name] = struct{}{}
		raw, exists := values[constraint.Name]
		if !exists {
			if constraint.Required {
				return serveroperationresourcecap.ErrTaskConflict
			}
			continue
		}
		value, numeric, ok := scalarConstraintValue(raw)
		if !ok || (len(constraint.AllowedValues) > 0 && !contains(constraint.AllowedValues, value)) {
			return serveroperationresourcecap.ErrTaskConflict
		}
		if constraint.Minimum != nil && (!numeric || numericValue(value) < *constraint.Minimum) {
			return serveroperationresourcecap.ErrTaskConflict
		}
		if constraint.Maximum != nil && (!numeric || numericValue(value) > *constraint.Maximum) {
			return serveroperationresourcecap.ErrTaskConflict
		}
		if constraint.Pattern != "" {
			pattern, err := regexp.Compile(constraint.Pattern)
			if err != nil {
				return serveroperationresourcecap.ErrCapabilityUnavailable
			}
			if !pattern.MatchString(value) {
				return serveroperationresourcecap.ErrTaskConflict
			}
		}
	}
	for name := range values {
		if _, exists := seen[name]; !exists {
			return serveroperationresourcecap.ErrTaskConflict
		}
	}
	return nil
}

func scalarConstraintValue(raw json.RawMessage) (string, bool, bool) {
	var text string
	if json.Unmarshal(raw, &text) == nil {
		_, err := strconv.ParseInt(text, 10, 64)
		return text, err == nil, text != ""
	}
	var number json.Number
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if decoder.Decode(&number) != nil {
		return "", false, false
	}
	if _, err := number.Int64(); err != nil {
		return "", false, false
	}
	return number.String(), true, true
}

func numericValue(value string) int64 { parsed, _ := strconv.ParseInt(value, 10, 64); return parsed }

func validSecretBindings(request serveroperationresourcecap.TaskRequest) bool {
	if request.Action == serveroperationresourcecap.ActionResetPassword {
		return len(request.SecretBindings) == 1 && request.SecretBindings[0].Name == "password" && !request.SecretBindings[0].Handle.IsZero()
	}
	return len(request.SecretBindings) == 0
}

func (s *Service) loadTask(ctx context.Context, taskNo, channelNo string) (*taskRow, error) {
	var row *taskRow
	err := s.database.GetScan(ctx, &row, `SELECT task_no, capability_code, local_resource_no, channel_no,
 COALESCE(pool_no, '') AS pool_no, status_code, sanitized_parameters::TEXT AS sanitized_parameters,
 owner_metadata::TEXT AS owner_metadata, failure_code, result_unknown, version, finished_at, updated_at
 FROM plugin_idc_resource_task WHERE tenant_id = ? AND task_no = ? AND channel_no = ?`, idccap.DefaultTenantID, taskNo, channelNo)
	if err != nil {
		return nil, gerror.Wrap(err, "read server operation Provider task")
	}
	if row != nil {
		outputs, outputErr := s.loadSecretOutputs(ctx, taskNo)
		if outputErr != nil {
			return nil, outputErr
		}
		row.SecretOutputs = outputs
	}
	return row, nil
}

func (s *Service) loadTaskByRequest(ctx context.Context, channelNo, requestID string) (*taskRow, error) {
	requestDigest := sha256.Sum256([]byte(requestID))
	var row *taskRow
	err := s.database.GetScan(ctx, &row, `SELECT task_no, capability_code, local_resource_no, channel_no,
 COALESCE(pool_no, '') AS pool_no, status_code, sanitized_parameters::TEXT AS sanitized_parameters,
 owner_metadata::TEXT AS owner_metadata, failure_code, result_unknown, version, finished_at, updated_at
 FROM plugin_idc_resource_task
 WHERE tenant_id = ? AND channel_no = ? AND request_id_digest = ?`,
		idccap.DefaultTenantID, channelNo, hex.EncodeToString(requestDigest[:]))
	if err != nil {
		return nil, gerror.Wrap(err, "read idempotent server operation Provider task")
	}
	if row != nil {
		outputs, outputErr := s.loadSecretOutputs(ctx, row.TaskNo)
		if outputErr != nil {
			return nil, outputErr
		}
		row.SecretOutputs = outputs
	}
	return row, nil
}

func (s *Service) loadSecretOutputs(ctx context.Context, taskNo string) ([]serveroperationresourcecap.SecretOutput, error) {
	var rows []secretOutputRow
	if err := s.database.GetScan(ctx, &rows, `SELECT binding_name, secret_ref, output_policy, expires_at
FROM plugin_idc_resource_task_secret_binding
WHERE tenant_id = ? AND task_no = ? AND binding_direction = 'output'
ORDER BY binding_name, id`, idccap.DefaultTenantID, taskNo); err != nil {
		return nil, gerror.Wrap(err, "read server operation confidential outputs")
	}
	outputs := make([]serveroperationresourcecap.SecretOutput, 0, len(rows))
	for _, row := range rows {
		handle, err := secretcap.NewHandle(row.SecretRef)
		if err != nil {
			return nil, serveroperationresourcecap.ErrTaskConflict
		}
		output := serveroperationresourcecap.SecretOutput{Name: row.BindingName, Policy: serveroperationresourcecap.SecretOutputPolicy(row.OutputPolicy), Handle: handle}
		if row.ExpiresAt != nil {
			output.ExpiresAt = row.ExpiresAt.UTC()
		}
		outputs = append(outputs, output)
	}
	return outputs, nil
}

func projectTask(row *taskRow, request serveroperationresourcecap.TaskRequest) (*serveroperationresourcecap.Task, error) {
	if row == nil || row.TaskNo == "" {
		return nil, serveroperationresourcecap.ErrTaskUnavailable
	}
	var metadata taskMetadata
	capability, ok := capabilityFor(request.Action)
	requestDigest, digestErr := decodeDigest(request.RequestDigest)
	translatedDigest, translatedDigestErr := digestJSON(json.RawMessage(row.SanitizedParameters))
	decodedTranslatedDigest, decodedTranslatedDigestErr := decodeDigest(translatedDigest)
	if json.Unmarshal([]byte(row.OwnerMetadata), &metadata) != nil || digestErr != nil || translatedDigestErr != nil || decodedTranslatedDigestErr != nil || !ok || metadata.ContractVersion != serveroperationresourcecap.ContractVersion || metadata.OperationNo != request.OperationNo || metadata.Action != request.Action || metadata.ExpectedServiceVersion != request.ExpectedServiceVersion || metadata.RequestDigest != requestDigest || metadata.TranslatedParametersDigest != decodedTranslatedDigest || row.CapabilityCode != string(capability) || row.LocalResourceNo != request.ServiceNo || row.ChannelNo != request.ChannelNo || row.PoolNo != request.PoolNo {
		return nil, serveroperationresourcecap.ErrTaskConflict
	}
	status, failure := projectStatus(row)
	evidence, err := digest(struct {
		TaskNo, Status, Failure             string
		Version                             int64
		ResultUnknown                       bool
		FinishedUnixMicro, UpdatedUnixMicro int64
	}{row.TaskNo, row.StatusCode, row.FailureCode, row.Version, row.ResultUnknown, unixMicro(row.FinishedAt), row.UpdatedAt.UTC().UnixMicro()})
	if err != nil {
		return nil, err
	}
	result := &serveroperationresourcecap.Task{ProviderTaskNo: row.TaskNo, Status: status, FailureCode: failure, CapabilityEvidenceNo: metadata.CapabilityEvidenceNo, CapabilityEvidenceDigest: hex.EncodeToString(metadata.CapabilityEvidenceDigest[:]), EvidenceDigest: evidence, ObservedAt: row.UpdatedAt.UTC(), SecretOutputs: append([]serveroperationresourcecap.SecretOutput(nil), row.SecretOutputs...)}
	if row.FinishedAt != nil {
		result.CompletedAt = row.FinishedAt.UTC()
	}
	if serveroperationresourcecap.ValidateTaskForRequest(*result, request) != nil {
		return nil, serveroperationresourcecap.ErrTaskConflict
	}
	return result, nil
}

func capabilityFor(action serveroperationresourcecap.Action) (resourcecap.CapabilityCode, bool) {
	switch action {
	case serveroperationresourcecap.ActionStart:
		return resourcecap.CapabilityStart, true
	case serveroperationresourcecap.ActionStop:
		return resourcecap.CapabilityStop, true
	case serveroperationresourcecap.ActionReboot:
		return resourcecap.CapabilityReboot, true
	case serveroperationresourcecap.ActionResetPassword:
		return resourcecap.CapabilityResetPassword, true
	case serveroperationresourcecap.ActionConsole:
		return resourcecap.CapabilityConsole, true
	case serveroperationresourcecap.ActionReinstall:
		return resourcecap.CapabilityReinstall, true
	case serveroperationresourcecap.ActionResize:
		return resourcecap.CapabilityResize, true
	case serveroperationresourcecap.ActionChangeIP:
		return resourcecap.CapabilityChangeIP, true
	case serveroperationresourcecap.ActionSnapshotCreate, serveroperationresourcecap.ActionSnapshotDelete,
		serveroperationresourcecap.ActionSnapshotRestore:
		return resourcecap.CapabilityManageSnapshot, true
	case serveroperationresourcecap.ActionBackupCreate, serveroperationresourcecap.ActionBackupDelete,
		serveroperationresourcecap.ActionBackupRestore:
		return resourcecap.CapabilityBackup, true
	case serveroperationresourcecap.ActionSecurityGroupAttach, serveroperationresourcecap.ActionSecurityGroupDetach:
		return resourcecap.CapabilitySecurityGroup, true
	case serveroperationresourcecap.ActionNATCreate, serveroperationresourcecap.ActionNATDelete:
		return resourcecap.CapabilityNAT, true
	case serveroperationresourcecap.ActionVirtualMediaMount, serveroperationresourcecap.ActionVirtualMediaUnmount:
		return resourcecap.CapabilityVirtualMedia, true
	case serveroperationresourcecap.ActionTrafficAdd:
		return resourcecap.CapabilityTrafficPackage, true
	default:
		return "", false
	}
}

func projectStatus(row *taskRow) (serveroperationresourcecap.TaskStatus, string) {
	switch resourcetask.Status(row.StatusCode) {
	case resourcetask.StatusPending, resourcetask.StatusRetryWait:
		return serveroperationresourcecap.TaskStatusPending, ""
	case resourcetask.StatusSubmitting, resourcetask.StatusAccepted, resourcetask.StatusRunning:
		return serveroperationresourcecap.TaskStatusRunning, ""
	case resourcetask.StatusSucceeded:
		return serveroperationresourcecap.TaskStatusSucceeded, ""
	case resourcetask.StatusFailed, resourcetask.StatusCanceled:
		return serveroperationresourcecap.TaskStatusFailed, failureCode(row, "resource.task_failed")
	case resourcetask.StatusResultUnknown:
		return serveroperationresourcecap.TaskStatusResultUnknown, failureCode(row, "resource.result_unknown")
	case resourcetask.StatusManualTakeover:
		return serveroperationresourcecap.TaskStatusManualReview, failureCode(row, "resource.manual_review")
	default:
		return serveroperationresourcecap.TaskStatusManualReview, "resource.task_invalid"
	}
}

func failureCode(row *taskRow, fallback string) string {
	if row != nil && row.FailureCode != "" {
		return strings.ToLower(row.FailureCode)
	}
	return fallback
}
func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
func digestJSON(encoded json.RawMessage) (string, error) {
	var value any
	if json.Unmarshal(encoded, &value) != nil {
		return "", serveroperationresourcecap.ErrTaskConflict
	}
	return digest(value)
}
func unixMicro(value *time.Time) int64 {
	if value == nil {
		return 0
	}
	return value.UTC().UnixMicro()
}

func decodeDigest(value string) ([sha256.Size]byte, error) {
	var result [sha256.Size]byte
	decoded, err := hex.DecodeString(value)
	if err != nil || len(decoded) != sha256.Size {
		return result, serveroperationresourcecap.ErrTaskConflict
	}
	copy(result[:], decoded)
	return result, nil
}
func digest(value any) (string, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return "", gerror.Wrap(err, "encode server operation evidence")
	}
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:]), nil
}
