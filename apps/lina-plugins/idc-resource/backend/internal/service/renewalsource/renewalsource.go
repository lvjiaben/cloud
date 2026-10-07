// Package renewalsource implements the server-resource owner's provider-confirmed renewal tasks.
package renewalsource

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"reflect"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/cap/renewalownercap"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
	"lina-plugin-idc-resource/backend/internal/service/resourcetask"
)

const OwnerID = "idc-resource"

type Service struct {
	database gdb.DB
	tasks    *resourcetask.Store
}

var _ renewalownercap.Source = (*Service)(nil)

func New(database gdb.DB) (*Service, error) {
	if database == nil {
		return nil, gerror.New("resource renewal database is required")
	}
	tasks, err := resourcetask.NewStore(database)
	if err != nil {
		return nil, err
	}
	return &Service{database: database, tasks: tasks}, nil
}

type renewalRoute struct {
	LocalResourceNo   string `orm:"local_resource_no"`
	ChannelNo         string `orm:"channel_no"`
	PoolNo            string `orm:"pool_no"`
	PluginID          string `orm:"provider_plugin_id"`
	PluginRelease     string `orm:"provider_release_version"`
	ContractVersion   string `orm:"provider_contract_version"`
	DeclarationDigest string `orm:"provider_declaration_digest"`
}

func (s *Service) resolveRoute(ctx context.Context, request renewalownercap.TaskRequest) (*renewalRoute, error) {
	var route *renewalRoute
	err := s.database.GetScan(ctx, &route, `
SELECT mapping.local_resource_no,mapping.channel_no,mapping.pool_no,
       channel.provider_plugin_id,channel.provider_release_version,
       channel.provider_contract_version,channel.provider_declaration_digest
FROM plugin_idc_resource_server_entitlement_projection ownership
JOIN plugin_idc_resource_mapping mapping
  ON mapping.tenant_id=ownership.tenant_id
 AND mapping.mapping_no=ownership.mapping_no
 AND mapping.local_resource_no=ownership.local_resource_no
JOIN plugin_idc_resource_channel channel
  ON channel.tenant_id=mapping.tenant_id AND channel.channel_no=mapping.channel_no
WHERE ownership.tenant_id=? AND ownership.entitlement_no=?
  AND ownership.provider_plugin_id=? AND ownership.plugin_release_version=?
  AND ownership.declaration_digest=?
  AND mapping.status_code='active' AND mapping.external_resource_ref<>''
  AND channel.status_code IN ('enabled','degraded','maintenance') AND channel.deleted_at IS NULL
LIMIT 1`, idccap.DefaultTenantID, request.EntitlementNo, request.PluginID,
		request.PluginReleaseVersion, request.DeclarationDigest)
	if err != nil {
		return nil, gerror.Wrap(err, "resolve resource renewal route")
	}
	if route == nil || route.LocalResourceNo != request.EntitlementNo ||
		route.PluginID != request.PluginID || route.PluginRelease != request.PluginReleaseVersion ||
		route.ContractVersion != request.ContractVersion || route.DeclarationDigest != request.DeclarationDigest {
		return nil, renewalownercap.ErrOwnerUnavailable
	}
	return route, nil
}

func (s *Service) CreateTask(ctx context.Context, request renewalownercap.TaskRequest) (*renewalownercap.Task, error) {
	if s == nil || s.database == nil || s.tasks == nil || ctx == nil || ctx.Err() != nil ||
		request.OwnerID != OwnerID || renewalownercap.ValidateTaskRequest(request) != nil {
		return nil, renewalownercap.ErrOwnerUnavailable
	}
	route, err := s.resolveRoute(ctx, request)
	if err != nil {
		return nil, err
	}
	parameters, err := taskParameters(request)
	if err != nil {
		return nil, renewalownercap.ErrTaskConflict
	}
	requestID := taskRequestID(request.OperationNo)
	existing, err := s.loadTaskByRequest(ctx, route.ChannelNo, requestID)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		if !sameTaskIntent(existing, request, route, parameters) {
			return nil, renewalownercap.ErrTaskConflict
		}
		result, readErr := s.ReadTask(ctx, renewalownercap.TaskReadRequest{TaskNo: existing.TaskNo, Request: request})
		if result != nil {
			result.Reused = true
		}
		return result, readErr
	}
	created, err := s.tasks.Create(ctx, resourcetask.CreateInput{
		RequestID: requestID, Capability: resourcecap.CapabilityRenew,
		LocalResourceNo: route.LocalResourceNo, ChannelNo: route.ChannelNo,
		PoolNo: route.PoolNo, Parameters: parameters, Priority: 100, MaxAttempts: 8,
	})
	if err != nil {
		return nil, errors.Join(renewalownercap.ErrOwnerUnavailable, gerror.Wrap(err, "create Provider renewal task"))
	}
	result, err := s.ReadTask(ctx, renewalownercap.TaskReadRequest{TaskNo: created.TaskNo, Request: request})
	if result != nil {
		result.Reused = !created.Created
	}
	return result, err
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
	FailureCode         string     `orm:"failure_code"`
	ResultUnknown       bool       `orm:"result_unknown"`
	Version             int64      `orm:"version"`
	FinishedAt          *time.Time `orm:"finished_at"`
	UpdatedAt           time.Time  `orm:"updated_at"`
}

func (s *Service) ReadTask(ctx context.Context, request renewalownercap.TaskReadRequest) (*renewalownercap.Task, error) {
	if s == nil || s.database == nil || ctx == nil || ctx.Err() != nil ||
		request.Request.OwnerID != OwnerID || renewalownercap.ValidateTaskReadRequest(request) != nil {
		return nil, renewalownercap.ErrOwnerUnavailable
	}
	route, err := s.resolveRoute(ctx, request.Request)
	if err != nil {
		return nil, err
	}
	var row *taskRow
	if err = s.database.GetScan(ctx, &row, `
SELECT task_no,request_id_digest,capability_code,local_resource_no,
       channel_no,COALESCE(pool_no,'') AS pool_no,status_code,
       sanitized_parameters::TEXT AS sanitized_parameters,
       failure_code,result_unknown,version,finished_at,updated_at
FROM plugin_idc_resource_task
WHERE tenant_id=? AND task_no=? AND channel_no=?`, idccap.DefaultTenantID, request.TaskNo, route.ChannelNo); err != nil {
		return nil, gerror.Wrap(err, "read Provider renewal task")
	}
	if row == nil || row.TaskNo == "" {
		return nil, renewalownercap.ErrTaskUnavailable
	}
	parameters, err := taskParameters(request.Request)
	if err != nil || !sameTaskIntent(row, request.Request, route, parameters) {
		return nil, renewalownercap.ErrTaskConflict
	}
	status, failureCode := projectTaskStatus(row)
	evidence, err := taskEvidenceDigest(row)
	if err != nil {
		return nil, err
	}
	result := &renewalownercap.Task{TaskNo: row.TaskNo, Status: status, FailureCode: failureCode,
		EvidenceDigest: evidence, ObservedAt: row.UpdatedAt.UTC()}
	if renewalownercap.ValidateTask(*result) != nil {
		return nil, renewalownercap.ErrTaskConflict
	}
	return result, nil
}

func (s *Service) loadTaskByRequest(ctx context.Context, channelNo, requestID string) (*taskRow, error) {
	digest := sha256.Sum256([]byte(requestID))
	var row *taskRow
	if err := s.database.GetScan(ctx, &row, `
SELECT task_no,request_id_digest,capability_code,local_resource_no,
       channel_no,COALESCE(pool_no,'') AS pool_no,status_code,
       sanitized_parameters::TEXT AS sanitized_parameters,
       failure_code,result_unknown,version,finished_at,updated_at
FROM plugin_idc_resource_task
WHERE tenant_id=? AND channel_no=? AND request_id_digest=?`, idccap.DefaultTenantID, channelNo,
		hex.EncodeToString(digest[:])); err != nil {
		return nil, gerror.Wrap(err, "read idempotent Provider renewal task")
	}
	return row, nil
}

func taskRequestID(operationNo string) string { return "core.renewal." + operationNo }

func taskParameters(request renewalownercap.TaskRequest) (json.RawMessage, error) {
	return json.Marshal(struct {
		PriceSnapshotNo            string `json:"priceSnapshotNo"`
		RenewalMethod              string `json:"renewalMethod"`
		ExpectedEntitlementVersion int64  `json:"expectedEntitlementVersion"`
		OriginalExpiresUnixNano    int64  `json:"originalExpiresUnixNano"`
		ProjectedExpiresUnixNano   int64  `json:"projectedExpiresUnixNano"`
	}{request.PriceSnapshotNo, request.RenewalMethod, request.ExpectedEntitlementVersion,
		request.OriginalExpiresAt.UTC().UnixNano(),
		request.ProjectedExpiresAt.UTC().UnixNano()})
}

func sameTaskIntent(row *taskRow, request renewalownercap.TaskRequest, route *renewalRoute, parameters []byte) bool {
	if row == nil || route == nil || row.CapabilityCode != string(resourcecap.CapabilityRenew) ||
		row.LocalResourceNo != request.EntitlementNo || row.LocalResourceNo != route.LocalResourceNo ||
		row.ChannelNo != route.ChannelNo || row.PoolNo != route.PoolNo || row.RequestDigest == "" {
		return false
	}
	var persisted, expected any
	return json.Unmarshal([]byte(row.SanitizedParameters), &persisted) == nil &&
		json.Unmarshal(parameters, &expected) == nil && reflect.DeepEqual(persisted, expected)
}

func projectTaskStatus(row *taskRow) (string, string) {
	switch resourcetask.Status(row.StatusCode) {
	case resourcetask.StatusPending, resourcetask.StatusRetryWait:
		return renewalownercap.TaskStatusPending, ""
	case resourcetask.StatusSubmitting, resourcetask.StatusAccepted, resourcetask.StatusRunning:
		return renewalownercap.TaskStatusRunning, ""
	case resourcetask.StatusSucceeded:
		return renewalownercap.TaskStatusSucceeded, ""
	case resourcetask.StatusFailed, resourcetask.StatusCanceled:
		code := row.FailureCode
		if code == "" {
			code = "resource.task_failed"
		}
		return renewalownercap.TaskStatusFailed, code
	case resourcetask.StatusResultUnknown:
		return renewalownercap.TaskStatusResultUnknown, row.FailureCode
	case resourcetask.StatusManualTakeover:
		return renewalownercap.TaskStatusManualReview, row.FailureCode
	default:
		return renewalownercap.TaskStatusManualReview, "resource.task_invalid"
	}
}

func taskEvidenceDigest(row *taskRow) (string, error) {
	payload, err := json.Marshal(struct {
		Contract, TaskNo, RequestDigest, Status, FailureCode string
		ResultUnknown                                        bool
		Version, FinishedUnixMicro, UpdatedUnixMicro         int64
	}{"idc-resource/renewal-task/v1", row.TaskNo, row.RequestDigest, row.StatusCode, row.FailureCode,
		row.ResultUnknown, row.Version, timePointerUnixMicro(row.FinishedAt), row.UpdatedAt.UTC().UnixMicro()})
	if err != nil {
		return "", gerror.Wrap(err, "encode Provider renewal task evidence")
	}
	digest := sha256.Sum256(payload)
	return hex.EncodeToString(digest[:]), nil
}

func timePointerUnixMicro(value *time.Time) int64 {
	if value == nil {
		return 0
	}
	return value.UTC().UnixMicro()
}
