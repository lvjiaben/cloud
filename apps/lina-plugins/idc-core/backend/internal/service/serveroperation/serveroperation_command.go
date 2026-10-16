package serveroperation

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"reflect"
	"regexp"
	"strings"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/cap/serveroperationresourcecap"
	"lina-plugin-idc-core/backend/internal/service/serviceguard"
)

var referencePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.:-]{0,127}$`)

type serviceRow struct {
	ServiceNo              string
	CustomerNo             string
	CurrencyCode           string
	PoolNo                 string
	ChannelNo              string
	EntitlementStatusCode  string
	RuntimeStatusCode      string
	TrafficSuspended       bool
	ProviderTaskStatus     string `orm:"provider_task_status_code"`
	Version                int64
	CurrentPriceSnapshotNo string `orm:"current_price_snapshot_no"`
	CustomerSubjectVersion int64  `orm:"customer_subject_version"`
}

type replayRow struct {
	OperationNo    string
	ServiceNo      string
	CustomerNo     string
	ActionCode     string
	StatusCode     string
	Parameters     string
	SecretBindings string
	CreatedAt      time.Time `orm:"created_at"`
}

type persistedSecretBinding struct {
	Name string `json:"name"`
	Ref  string `json:"ref"`
}

func (s *serviceImpl) Create(ctx context.Context, command Command) (*Operation, error) {
	normalized, bindings, err := normalizeCommand(command)
	if invalidCreateInvocation(s, ctx, err) {
		return nil, ErrInvalidInput
	}
	// Resize is a priced lifecycle change. It must enter through the dedicated
	// quote and settlement workflow; the generic action endpoint must never
	// bypass proration, downgrade scheduling, or disk-shrink protection.
	if command.Action == serveroperationresourcecap.ActionResize {
		return nil, ErrActionUnavailable
	}
	operationNo, err := deterministicReference("sop_", struct {
		ActorType, ActorReference, IdempotencyKey string
	}{command.ActorType, command.ActorReference, command.IdempotencyKey})
	if err != nil {
		return nil, ErrInvalidInput
	}
	var result *Operation
	err = s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		operation, createErr := s.createOperationTransaction(ctx, operationNo, command, normalized, bindings)
		result = operation
		return createErr
	})
	if err != nil {
		if isCreateDomainError(err) {
			return nil, err
		}
		return nil, gerror.Wrap(err, "create server operation")
	}
	return result, nil
}

func (s *serviceImpl) createOperationTransaction(ctx context.Context, operationNo string,
	command Command, normalized, bindings json.RawMessage) (*Operation, error) {
	replayed, err := loadReplay(ctx, s.database, operationNo)
	if err != nil {
		return nil, err
	}
	if replayed != nil {
		if replayed.ServiceNo != command.ServiceNo || replayed.CustomerNo != command.CustomerNo ||
			replayed.ActionCode != string(command.Action) || !sameJSON(replayed.Parameters, normalized) ||
			!sameJSON(replayed.SecretBindings, bindings) {
			return nil, ErrOperationConflict
		}
		return &Operation{OperationNo: replayed.OperationNo, ServiceNo: replayed.ServiceNo,
			Action: command.Action, Status: replayed.StatusCode, CreatedAt: replayed.CreatedAt.UTC(), Reused: true}, nil
	}
	service, err := loadServiceForUpdate(ctx, s.database, command.ServiceNo, command.CustomerNo)
	if err != nil {
		return nil, err
	}
	if service == nil {
		return nil, ErrServiceUnavailable
	}
	if !allowedAction(service, command.Action) {
		return nil, ErrActionUnavailable
	}
	if err = requireRestorableAccessoryEntitlement(ctx, s.database, service.ServiceNo,
		command.Action, normalized); err != nil {
		return nil, err
	}
	refundLocked, err := serviceguard.HasActiveRefundLock(ctx, s.database, service.ServiceNo)
	if err != nil {
		return nil, err
	}
	if refundLocked {
		return nil, ErrOperationConflict
	}
	active, err := s.database.GetValue(ctx, `SELECT EXISTS (
SELECT 1 FROM plugin_idc_core_server_operation_execution
WHERE tenant_id = 0 AND service_no = ?
  AND status_code IN ('ready', 'pending', 'running', 'result_unknown')
UNION ALL
SELECT 1 FROM plugin_idc_core_business_operation
WHERE tenant_id = 0 AND service_no = ?
  AND operation_type_code = 'addon'
  AND status_code IN ('awaiting_financial_settlement','financially_settled','manual_review'))`,
		service.ServiceNo, service.ServiceNo)
	if err != nil {
		return nil, gerror.Wrap(err, "check active server operation")
	}
	if active.Bool() || service.ChannelNo == "" || service.PoolNo == "" ||
		(service.ProviderTaskStatus != "none" && service.ProviderTaskStatus != "succeeded" &&
			service.ProviderTaskStatus != "failed") {
		return nil, ErrOperationConflict
	}
	nextVersion := service.Version + 1
	request := serveroperationresourcecap.TaskRequest{OperationNo: operationNo,
		ServiceNo: service.ServiceNo, ChannelNo: service.ChannelNo, PoolNo: service.PoolNo,
		Action: command.Action, ExpectedServiceVersion: nextVersion, Parameters: normalized,
		SecretBindings: command.SecretBindings}
	request.RequestDigest, err = serveroperationresourcecap.TaskRequestDigest(request)
	if err != nil {
		return nil, ErrInvalidInput
	}
	idempotencyDigest, err := digestValue(struct {
		ActorType, ActorReference, Key string
	}{command.ActorType, command.ActorReference, command.IdempotencyKey})
	if err != nil {
		return nil, err
	}
	contentDigest, err := digestValue(struct {
		OperationNo, CustomerNo, ServiceNo string
		Action                             serveroperationresourcecap.Action
		Parameters                         json.RawMessage
		Bindings                           json.RawMessage
		ExpectedVersion                    int64
	}{operationNo, command.CustomerNo, command.ServiceNo, command.Action,
		normalized, bindings, nextVersion})
	if err != nil {
		return nil, err
	}
	now, err := databaseNow(ctx, s.database)
	if err != nil {
		return nil, err
	}
	if _, err = s.database.Exec(ctx, `INSERT INTO plugin_idc_core_business_operation (
tenant_id, operation_no, operation_type_code, customer_no, service_no, currency_code,
status_code, financial_settlement_required, execution_mode_code, resource_owner_code,
pool_no, idempotency_digest, request_digest, content_digest, expected_service_version,
last_error_code, version, created_at, updated_at
) VALUES (0, ?, ?, ?, ?, ?, 'ready', FALSE, 'unresolved', 'idc-resource',
?, ?, ?, ?, ?, '', 1, ?, ?)`, operationNo, operationTypeForAction(command.Action), command.CustomerNo,
		command.ServiceNo, service.CurrencyCode, service.PoolNo, idempotencyDigest,
		request.RequestDigest, contentDigest, nextVersion, now, now); err != nil {
		return nil, gerror.Wrap(err, "insert server business operation")
	}
	executionNo, err := deterministicReference("sox_", operationNo)
	if err != nil {
		return nil, err
	}
	if _, err = s.database.Exec(ctx, `INSERT INTO plugin_idc_core_server_operation_execution (
tenant_id, execution_no, operation_no, service_no, action_code, resource_owner_code,
channel_no, pool_no, expected_service_version, original_runtime_status_code,
parameters, secret_bindings,
status_code, request_digest, version, created_at, updated_at
) VALUES (0, ?, ?, ?, ?, 'idc-resource', ?, ?, ?, ?, ?::JSONB, ?::JSONB,
'ready', ?, 1, ?, ?)`, executionNo, operationNo, command.ServiceNo, command.Action,
		service.ChannelNo, service.PoolNo, nextVersion, service.RuntimeStatusCode,
		string(normalized), string(bindings), request.RequestDigest, now, now); err != nil {
		return nil, gerror.Wrap(err, "insert server operation execution")
	}
	if err = execOne(ctx, s.database, `UPDATE plugin_idc_core_service_instance
SET last_business_operation_no = ?, version = version + 1, updated_at = ?
WHERE tenant_id = 0 AND service_no = ? AND customer_no = ? AND version = ?`,
		operationNo, now, command.ServiceNo, command.CustomerNo, service.Version); err != nil {
		return nil, gerror.Wrap(err, "fence server operation service")
	}
	if err = insertEvent(ctx, s.database, operationNo, command.ServiceNo, 1, nil,
		"ready", "server_operation.created", command.ActorType, command.ActorReference,
		contentDigest, now); err != nil {
		return nil, err
	}
	if err = s.audit.AppendBusinessTransition(ctx, idccap.BusinessAuditTransition{
		IdempotencyScope:   "server.operation.create",
		IdempotencyKey:     command.IdempotencyKey,
		OperationCode:      "server.operation.create",
		TargetType:         "server_service",
		TargetNo:           command.ServiceNo,
		CustomerNo:         command.CustomerNo,
		ResultCode:         idccap.BusinessAuditResultSucceeded,
		ReasonCode:         operationReasonCode(command.ActorType),
		RiskCode:           operationRiskCode(command.Action, normalized),
		RequiredPermission: command.RequiredPermission,
		CorrelationNo:      operationNo,
		BeforeSnapshot: map[string]any{
			"runtime_status":  service.RuntimeStatusCode,
			"service_version": service.Version,
		},
		AfterSnapshot: map[string]any{
			"action": command.Action, "status": "ready", "service_version": nextVersion,
		},
	}); err != nil {
		return nil, gerror.Wrap(err, "append server operation audit")
	}
	return &Operation{OperationNo: operationNo, ServiceNo: command.ServiceNo,
		Action: command.Action, Status: "ready", CreatedAt: now.UTC()}, nil
}

func operationReasonCode(actorType string) string {
	if actorType == "staff" {
		return "staff_server_accessory_operation"
	}
	return "customer_server_operation"
}

func invalidCreateInvocation(service *serviceImpl, ctx context.Context, normalizeErr error) bool {
	return normalizeErr != nil || service == nil || service.database == nil ||
		service.audit == nil || ctx == nil || ctx.Err() != nil
}

func operationRiskCode(action serveroperationresourcecap.Action, parameters json.RawMessage) string {
	if action == serveroperationresourcecap.ActionResetPassword ||
		action == serveroperationresourcecap.ActionReinstall ||
		action == serveroperationresourcecap.ActionResize ||
		action == serveroperationresourcecap.ActionChangeIP ||
		action == serveroperationresourcecap.ActionSnapshotRestore ||
		action == serveroperationresourcecap.ActionBackupRestore {
		return "high"
	}
	if action == serveroperationresourcecap.ActionStop || action == serveroperationresourcecap.ActionReboot {
		var input struct {
			Mode string `json:"mode"`
		}
		if json.Unmarshal(parameters, &input) != nil || (input.Mode != "" && input.Mode != "graceful") {
			return "high"
		}
	}
	return "medium"
}

func isCreateDomainError(err error) bool {
	return errors.Is(err, ErrServiceUnavailable) || errors.Is(err, ErrActionUnavailable) ||
		errors.Is(err, ErrOperationConflict) || errors.Is(err, ErrInvalidInput)
}

func normalizeCommand(command Command) (json.RawMessage, []byte, error) {
	if !validReference(command.CustomerNo, 40) || !validReference(command.ServiceNo, 40) ||
		!validReference(command.ActorReference, 64) ||
		(command.ActorType != "customer" && command.ActorType != "staff") ||
		(command.ActorType == "customer" && command.RequiredPermission != "") ||
		(command.ActorType == "staff" && !idccap.IsCatalogedAdminPermission(command.RequiredPermission)) ||
		len(command.IdempotencyKey) < 8 || len(command.IdempotencyKey) > 256 ||
		command.IdempotencyKey != strings.TrimSpace(command.IdempotencyKey) {
		return nil, nil, ErrInvalidInput
	}
	var object map[string]any
	if len(command.Parameters) == 0 || len(command.Parameters) > 16*1024 ||
		json.Unmarshal(command.Parameters, &object) != nil || object == nil {
		return nil, nil, ErrInvalidInput
	}
	normalized, err := json.Marshal(object)
	if err != nil {
		return nil, nil, ErrInvalidInput
	}
	persisted := make([]persistedSecretBinding, len(command.SecretBindings))
	for index, binding := range command.SecretBindings {
		persisted[index] = persistedSecretBinding{Name: binding.Name, Ref: binding.Handle.OpaqueRef()}
	}
	bindings, err := json.Marshal(persisted)
	if err != nil {
		return nil, nil, ErrInvalidInput
	}
	probe := serveroperationresourcecap.TaskRequest{OperationNo: "operation_probe", ServiceNo: command.ServiceNo,
		ChannelNo: "channel_probe", PoolNo: "pool_probe", Action: command.Action,
		ExpectedServiceVersion: 1, Parameters: normalized, SecretBindings: command.SecretBindings}
	if _, err = serveroperationresourcecap.TaskRequestDigest(probe); err != nil {
		return nil, nil, ErrInvalidInput
	}
	return normalized, bindings, nil
}

func allowedAction(service *serviceRow, action serveroperationresourcecap.Action) bool {
	if service == nil || (service.EntitlementStatusCode != "active" && service.EntitlementStatusCode != "grace") {
		return false
	}
	switch service.RuntimeStatusCode {
	case "running":
		return action == serveroperationresourcecap.ActionStop || action == serveroperationresourcecap.ActionReboot ||
			action == serveroperationresourcecap.ActionResetPassword || action == serveroperationresourcecap.ActionConsole ||
			action == serveroperationresourcecap.ActionReinstall || accessoryRunningOrStoppedAction(action) ||
			action == serveroperationresourcecap.ActionTrafficAdd
	case "stopped":
		return action == serveroperationresourcecap.ActionStart || action == serveroperationresourcecap.ActionResetPassword ||
			action == serveroperationresourcecap.ActionReinstall || accessoryRunningOrStoppedAction(action) ||
			action == serveroperationresourcecap.ActionSnapshotRestore ||
			action == serveroperationresourcecap.ActionBackupRestore ||
			action == serveroperationresourcecap.ActionTrafficAdd
	case "suspended":
		return service.TrafficSuspended && action == serveroperationresourcecap.ActionTrafficAdd
	default:
		return false
	}
}

func accessoryRunningOrStoppedAction(action serveroperationresourcecap.Action) bool {
	switch action {
	case serveroperationresourcecap.ActionSnapshotCreate, serveroperationresourcecap.ActionSnapshotDelete,
		serveroperationresourcecap.ActionBackupCreate, serveroperationresourcecap.ActionBackupDelete,
		serveroperationresourcecap.ActionChangeIP,
		serveroperationresourcecap.ActionSecurityGroupAttach, serveroperationresourcecap.ActionSecurityGroupDetach,
		serveroperationresourcecap.ActionNATCreate, serveroperationresourcecap.ActionNATDelete,
		serveroperationresourcecap.ActionVirtualMediaMount, serveroperationresourcecap.ActionVirtualMediaUnmount:
		return true
	default:
		return false
	}
}

func operationTypeForAction(action serveroperationresourcecap.Action) serveroperationresourcecap.Action {
	if accessoryAction(action) {
		return serveroperationresourcecap.Action("addon")
	}
	return action
}

func accessoryAction(action serveroperationresourcecap.Action) bool {
	switch action {
	case serveroperationresourcecap.ActionSnapshotCreate, serveroperationresourcecap.ActionSnapshotDelete,
		serveroperationresourcecap.ActionSnapshotRestore, serveroperationresourcecap.ActionBackupCreate,
		serveroperationresourcecap.ActionBackupDelete, serveroperationresourcecap.ActionBackupRestore,
		serveroperationresourcecap.ActionSecurityGroupAttach, serveroperationresourcecap.ActionSecurityGroupDetach,
		serveroperationresourcecap.ActionNATCreate, serveroperationresourcecap.ActionNATDelete,
		serveroperationresourcecap.ActionVirtualMediaMount, serveroperationresourcecap.ActionVirtualMediaUnmount,
		serveroperationresourcecap.ActionTrafficAdd:
		return true
	default:
		return false
	}
}

func loadServiceForUpdate(ctx context.Context, database gdb.DB, serviceNo, customerNo string) (*serviceRow, error) {
	var row *serviceRow
	if err := database.GetScan(ctx, &row, `SELECT service.service_no, service.customer_no, service.currency_code,
service.pool_no, COALESCE(service.channel_no, '') AS channel_no, service.entitlement_status_code,
service.runtime_status_code, service.provider_task_status_code, service.version,
EXISTS (SELECT 1 FROM plugin_idc_core_service_traffic_suspension suspension
 WHERE suspension.tenant_id = service.tenant_id AND suspension.service_no = service.service_no
 AND suspension.active) AS traffic_suspended
FROM plugin_idc_core_service_instance service
WHERE service.tenant_id = 0 AND service.service_no = ? AND service.customer_no = ? FOR UPDATE`, serviceNo, customerNo); err != nil {
		return nil, gerror.Wrap(err, "lock server operation service")
	}
	return row, nil
}

func loadReplay(ctx context.Context, database gdb.DB, operationNo string) (*replayRow, error) {
	var row *replayRow
	if err := database.GetScan(ctx, &row, `SELECT operation.operation_no, operation.service_no,
operation.customer_no, execution.action_code, operation.status_code,
execution.parameters::TEXT AS parameters, execution.secret_bindings::TEXT AS secret_bindings,
operation.created_at
FROM plugin_idc_core_business_operation operation
JOIN plugin_idc_core_server_operation_execution execution
 ON execution.tenant_id = operation.tenant_id AND execution.operation_no = operation.operation_no
WHERE operation.tenant_id = 0 AND operation.operation_no = ?`, operationNo); err != nil {
		return nil, gerror.Wrap(err, "load idempotent server operation")
	}
	return row, nil
}

func insertEvent(ctx context.Context, database gdb.DB, operationNo, serviceNo string,
	sequence int, from *string, to, reason, actorType, actorReference, evidence string, now time.Time) error {
	content, err := digestValue(struct {
		OperationNo, ServiceNo, To, Reason, Evidence string
		Sequence                                     int
		At                                           int64
	}{operationNo, serviceNo, to, reason, evidence, sequence, now.UTC().UnixMicro()})
	if err != nil {
		return err
	}
	eventNo, err := deterministicReference("soe_", struct {
		OperationNo string
		Sequence    int
	}{operationNo, sequence})
	if err != nil {
		return err
	}
	_, err = database.Exec(ctx, `INSERT INTO plugin_idc_core_business_operation_event (
tenant_id, event_no, operation_no, service_no, event_sequence, from_status_code,
to_status_code, reason_code, actor_type_code, actor_reference, evidence_digest,
content_digest, created_at
) VALUES (0, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, eventNo, operationNo, serviceNo,
		sequence, from, to, reason, actorType, actorReference, evidence, content, now)
	return gerror.Wrap(err, "insert server operation event")
}

func deterministicReference(prefix string, value any) (string, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return "", gerror.Wrap(err, "encode server operation reference")
	}
	sum := sha256.Sum256(encoded)
	return prefix + hex.EncodeToString(sum[:16]), nil
}

func digestValue(value any) (string, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return "", gerror.Wrap(err, "encode server operation digest")
	}
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:]), nil
}

func databaseNow(ctx context.Context, database gdb.DB) (time.Time, error) {
	value, err := database.GetValue(ctx, "SELECT CURRENT_TIMESTAMP")
	if err != nil {
		return time.Time{}, gerror.Wrap(err, "read server operation database time")
	}
	return value.Time().UTC(), nil
}

func execOne(ctx context.Context, database gdb.DB, query string, arguments ...any) error {
	result, err := database.Exec(ctx, query, arguments...)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return gerror.Wrap(err, "read server operation transition result")
	}
	if affected != 1 {
		return ErrOperationConflict
	}
	return nil
}

func validReference(value string, maximum int) bool {
	return len(value) <= maximum && value == strings.TrimSpace(value) && referencePattern.MatchString(value)
}

func sameJSON(left string, right []byte) bool {
	var first, second any
	return json.Unmarshal([]byte(left), &first) == nil && json.Unmarshal(right, &second) == nil &&
		reflect.DeepEqual(first, second)
}
