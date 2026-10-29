// This file reads durable operation results and consumes confidential outputs
// before delivery. No status poll performs a supplier request.
package customeroperations

import (
	"context"
	"encoding/json"
	"regexp"
	"time"

	"lina-plugin-idc-core/backend/cap/customerauthcap"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/cap/secretcap"
	"lina-plugin-idc-resource/backend/internal/composition"
)

var operationPattern = regexp.MustCompile(`^nso_[0-9a-f]{32}$`)

// GetOperation checks current ownership even when the original operation finished.
func (s *Service) GetOperation(ctx context.Context, serverNo, operationNo string) (*Operation, error) {
	if !operationPattern.MatchString(operationNo) {
		return nil, ErrInvalid
	}
	owned, err := s.ownedRoute(ctx, serverNo, false)
	if err != nil {
		return nil, err
	}
	if _, err = s.entitlements.ReadCurrentEntitlement(ctx, owned.EntitlementNo); err != nil {
		return nil, ErrUnavailable
	}
	var result *Operation
	err = s.database.GetScan(ctx, &result, `SELECT task.owner_metadata->>'operationNo' AS operation_no, task.owner_metadata->>'action' AS action, task.status_code AS status, task.failure_code, task.created_at,task.updated_at,
CASE WHEN task.status_code IN ('pending','retry_wait') AND health.circuit_open AND health.write_protected_until>CURRENT_TIMESTAMP THEN health.write_protected_until END AS waiting_until
FROM plugin_idc_resource_task task LEFT JOIN plugin_idc_resource_channel_health_state health ON health.tenant_id=task.tenant_id AND health.channel_no=task.channel_no
WHERE task.tenant_id=0 AND task.local_resource_no=? AND task.channel_no=? AND task.owner_metadata->>'operationNo'=?`, owned.LocalResourceNo, owned.ChannelNo, operationNo)
	if err != nil || result == nil {
		return nil, ErrUnavailable
	}
	result.AvailableOutputs = []Output{}
	if result.Status != "succeeded" {
		return result, nil
	}
	err = s.database.GetScan(ctx, &result.AvailableOutputs, `SELECT binding.binding_name AS name,binding.output_policy AS policy,binding.expires_at
FROM plugin_idc_resource_task_secret_binding binding JOIN plugin_idc_resource_task task ON task.tenant_id=binding.tenant_id AND task.task_no=binding.task_no
WHERE task.tenant_id=0 AND task.local_resource_no=? AND task.channel_no=? AND task.owner_metadata->>'operationNo'=? AND task.status_code='succeeded'
AND binding.binding_direction='output' AND binding.status_code='active' AND (binding.expires_at IS NULL OR binding.expires_at>CURRENT_TIMESTAMP)
AND ((task.owner_metadata->>'action'='reinstall' AND binding.binding_name='generatedCredential' AND binding.output_policy='one_time_credential')
OR (task.owner_metadata->>'action'='console' AND binding.binding_name='consoleTicket' AND binding.output_policy='short_lived_ticket' AND binding.expires_at IS NOT NULL))`, owned.LocalResourceNo, owned.ChannelNo, operationNo)
	if err != nil {
		return nil, ErrUnavailable
	}
	return result, nil
}

// RevealOutput commits the one-time claim and audit before calling a response
// consumer. A lost response cannot make the same output available again.
func (s *Service) RevealOutput(ctx context.Context, serverNo, operationNo, name string, confirmed bool, consume secretcap.Consumer) error {
	if !confirmed || consume == nil || !operationPattern.MatchString(operationNo) || (name != "generatedCredential" && name != "consoleTicket") {
		return ErrInvalid
	}
	owned, err := s.ownedRoute(ctx, serverNo, false)
	if err != nil {
		return err
	}
	if name == "consoleTicket" {
		callCtx, cancel := context.WithTimeout(ctx, customerObservationBudget)
		observed, observeErr := s.observe(callCtx, owned)
		cancel()
		if observeErr != nil || !allowedState(observed.State, "console") {
			return ErrConflict
		}
	}
	handle, expiresAt, err := s.claimOutput(ctx, owned, serverNo, operationNo, name)
	if err != nil {
		return err
	}
	accessor, err := composition.BindProviderSecretAccessor(s.secrets, owned.ChannelNo)
	if err != nil {
		return ErrUnavailable
	}
	purpose, err := secretcap.NewPurpose("resource.customer-operation-output")
	if err != nil {
		return ErrUnavailable
	}
	request, err := secretcap.NewAccessRequest(handle, purpose)
	if err != nil {
		return ErrUnavailable
	}
	if err = accessor.Access(ctx, request, func(value []byte) error {
		if len(value) == 0 || len(value) > 16384 || !json.Valid(value) || (expiresAt != nil && !expiresAt.After(time.Now())) {
			return ErrUnavailable
		}
		return consume(value)
	}); err != nil {
		return ErrUnavailable
	}
	return nil
}

// claimOutput consumes the delivery in the owner transaction, before any HTTP bytes.
func (s *Service) claimOutput(ctx context.Context, owned *route, serverNo, operationNo, name string) (secretcap.Handle, *time.Time, error) {
	var handle secretcap.Handle
	var expiresAt *time.Time
	err := s.entitlements.WithCurrentEntitlement(ctx, owned.EntitlementNo, func(ctx context.Context, _ *customerauthcap.Entitlement) error {
		locked, lockErr := s.ownedRoute(ctx, serverNo, true)
		if lockErr != nil || !sameRoute(locked, owned) {
			return ErrUnavailable
		}
		if busy, busyErr := s.busy(ctx, owned.LocalResourceNo); busyErr != nil || busy {
			return ErrConflict
		}
		var binding *struct {
			BindingNo, SecretRef, Action, OutputPolicy string
			ExpiresAt                                  *time.Time
		}
		readErr := s.database.GetScan(ctx, &binding, `SELECT binding.binding_no,binding.secret_ref,binding.output_policy,binding.expires_at,task.owner_metadata->>'action' AS action
FROM plugin_idc_resource_task_secret_binding binding JOIN plugin_idc_resource_task task ON task.tenant_id=binding.tenant_id AND task.task_no=binding.task_no
WHERE task.tenant_id=0 AND task.local_resource_no=? AND task.channel_no=? AND task.owner_metadata->>'operationNo'=? AND task.status_code='succeeded'
AND binding.binding_name=? AND binding.binding_direction='output' AND binding.status_code='active' AND (binding.expires_at IS NULL OR binding.expires_at>CURRENT_TIMESTAMP)
FOR UPDATE OF binding,task`, owned.LocalResourceNo, owned.ChannelNo, operationNo, name)
		if readErr != nil || binding == nil {
			return ErrUnavailable
		}
		if (name == "generatedCredential" && (binding.Action != "reinstall" || binding.OutputPolicy != "one_time_credential")) || (name == "consoleTicket" && (binding.Action != "console" || binding.OutputPolicy != "short_lived_ticket" || binding.ExpiresAt == nil)) {
			return ErrUnavailable
		}
		var handleErr error
		handle, handleErr = secretcap.NewHandle(binding.SecretRef)
		if handleErr != nil {
			return ErrUnavailable
		}
		expiresAt = binding.ExpiresAt
		updated, updateErr := s.database.Exec(ctx, `UPDATE plugin_idc_resource_task_secret_binding SET status_code='revealed',revealed_at=CURRENT_TIMESTAMP,version=version+1,updated_at=CURRENT_TIMESTAMP WHERE tenant_id=0 AND binding_no=? AND status_code='active'`, binding.BindingNo)
		if updateErr != nil {
			return ErrUnavailable
		}
		count, countErr := updated.RowsAffected()
		if countErr != nil || count != 1 {
			return ErrConflict
		}
		return s.audit.AppendBusinessTransition(ctx, idccap.BusinessAuditTransition{IdempotencyScope: "resource.customer-operation.output", IdempotencyKey: binding.BindingNo, OperationCode: "resource.customer-operation.output", TargetType: "server_service", TargetNo: serverNo, CustomerNo: owned.CustomerNo, ResultCode: "succeeded", ReasonCode: "customer_request", RiskCode: "high", CorrelationNo: operationNo, BeforeSnapshot: map[string]any{"consumed": false}, AfterSnapshot: map[string]any{"consumed": true, "output_name": name}})
	})
	return handle, expiresAt, err
}
