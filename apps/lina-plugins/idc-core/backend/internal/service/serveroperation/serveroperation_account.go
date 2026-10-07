package serveroperation

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-plugin-idc-core/backend/cap/identitycap"
	"lina-plugin-idc-core/backend/cap/secretcap"
	"lina-plugin-idc-core/backend/cap/serveroperationresourcecap"
	"lina-plugin-idc-core/backend/internal/service/reauth"
)

type accountOperationContext struct {
	ChannelNo             string
	PoolNo                string
	EntitlementStatusCode string
	RuntimeStatusCode     string
	TrafficSuspended      bool
	RequireReauth         bool `orm:"require_high_risk_reauth"`
}

func (s *accountServiceImpl) PrepareCurrentCustomer(ctx context.Context,
	command AccountCommand) (*Authorization, error) {
	command.Password = nil
	subject, normalized, operationContext, err := s.authorizeContext(ctx, command)
	if err != nil {
		return nil, err
	}
	if err = s.parameters.ValidateParameters(ctx, serveroperationresourcecap.CapabilityRoute{
		ServiceNo: command.ServiceNo, ChannelNo: operationContext.ChannelNo, PoolNo: operationContext.PoolNo,
	}, command.Action, normalized); err != nil {
		return nil, err
	}
	if !operationContext.RequireReauth || !highRiskAction(command.Action, normalized) {
		return &Authorization{}, nil
	}
	binding, err := accountOperationBinding(subject.CustomerNo, command, normalized)
	if err != nil {
		return nil, err
	}
	proof, err := s.recentAuth.IssueSessionConfirmation(ctx, binding)
	if err != nil {
		return nil, err
	}
	return &Authorization{Required: true, Token: proof.Token,
		ExpiresAt: proof.ExpiresAt.UnixMilli()}, nil
}

func (s *accountServiceImpl) CreateCurrentCustomer(ctx context.Context,
	command AccountCommand) (*Operation, error) {
	if s == nil || s.database == nil || s.customers == nil || s.operations == nil ||
		s.inputSecrets == nil || s.recentAuth == nil || ctx == nil || ctx.Err() != nil ||
		len(command.Password) > 1024 || len(command.ProofToken) > 4096 {
		clear(command.Password)
		return nil, ErrInvalidInput
	}
	defer clear(command.Password)
	subject, normalized, operationContext, err := s.authorizeContext(ctx, command)
	if err != nil {
		return nil, err
	}
	if err = s.parameters.ValidateParameters(ctx, serveroperationresourcecap.CapabilityRoute{
		ServiceNo: command.ServiceNo, ChannelNo: operationContext.ChannelNo, PoolNo: operationContext.PoolNo,
	}, command.Action, normalized); err != nil {
		return nil, err
	}
	requiresProof := operationContext.RequireReauth && highRiskAction(command.Action, normalized)
	if requiresProof {
		binding, bindingErr := accountOperationBinding(subject.CustomerNo, command, normalized)
		if bindingErr != nil {
			return nil, bindingErr
		}
		if _, err = s.recentAuth.Consume(ctx, command.ProofToken, binding); err != nil {
			return nil, err
		}
	} else if command.ProofToken != "" {
		return nil, ErrInvalidInput
	}

	bindings := []serveroperationresourcecap.SecretBinding(nil)
	var prepared *serveroperationresourcecap.PreparedInputSecret
	if command.Action == serveroperationresourcecap.ActionResetPassword {
		if len(command.Password) < 8 {
			return nil, ErrInvalidInput
		}
		prepared, err = s.inputSecrets.PrepareInputSecret(ctx,
			serveroperationresourcecap.InputSecretRequest{
				ChannelNo: operationContext.ChannelNo, IdempotencyKey: command.ServiceNo + ":" + command.IdempotencyKey,
				BindingName: "password", Produce: func(consume secretcap.Consumer) error {
					return consume(command.Password)
				},
			})
		if err != nil || prepared == nil || prepared.Handle.IsZero() {
			return nil, serveroperationresourcecap.ErrSourceUnavailable
		}
		bindings = []serveroperationresourcecap.SecretBinding{{Name: "password", Handle: prepared.Handle}}
	} else if len(command.Password) != 0 {
		return nil, ErrInvalidInput
	}
	result, createErr := s.operations.Create(ctx, Command{
		CustomerNo: subject.CustomerNo, ServiceNo: command.ServiceNo, Action: command.Action,
		Parameters: normalized, SecretBindings: bindings, IdempotencyKey: command.IdempotencyKey,
		ActorType: "customer", ActorReference: subject.CustomerNo,
	})
	if createErr != nil && prepared != nil && !prepared.Reused {
		if revokeErr := s.inputSecrets.RevokeInputSecret(ctx,
			operationContext.ChannelNo, prepared.Handle); revokeErr != nil {
			return nil, errors.Join(createErr, gerror.Wrap(revokeErr,
				"revoke orphan server operation password"))
		}
	}
	return result, createErr
}

func (s *accountServiceImpl) authorizeContext(ctx context.Context,
	command AccountCommand) (*identitycap.CustomerSubject, json.RawMessage, *accountOperationContext, error) {
	if s == nil || s.database == nil || s.customers == nil || s.recentAuth == nil ||
		ctx == nil || ctx.Err() != nil || !validReference(command.ServiceNo, 40) ||
		len(command.IdempotencyKey) < 8 || len(command.IdempotencyKey) > 256 ||
		command.IdempotencyKey != strings.TrimSpace(command.IdempotencyKey) {
		return nil, nil, nil, ErrInvalidInput
	}
	parameters := command.Parameters
	if len(parameters) == 0 {
		parameters = json.RawMessage(`{}`)
	}
	var object map[string]any
	if len(parameters) > 16*1024 || json.Unmarshal(parameters, &object) != nil || object == nil {
		return nil, nil, nil, ErrInvalidInput
	}
	normalized, err := json.Marshal(object)
	if err != nil {
		return nil, nil, nil, ErrInvalidInput
	}
	subject, err := s.customers.CurrentCustomer(ctx)
	if err != nil || subject == nil || !validReference(subject.CustomerNo, 40) {
		return nil, nil, nil, ErrServiceUnavailable
	}
	var operationContext *accountOperationContext
	if err = s.database.GetScan(ctx, &operationContext, `SELECT COALESCE(service.channel_no, '') AS channel_no,
service.pool_no,
service.entitlement_status_code, service.runtime_status_code,
EXISTS (SELECT 1 FROM plugin_idc_core_service_traffic_suspension suspension
 WHERE suspension.tenant_id = service.tenant_id AND suspension.service_no = service.service_no
 AND suspension.active) AS traffic_suspended,
policy.require_high_risk_reauth
FROM plugin_idc_core_service_instance service
JOIN plugin_idc_core_server_operation_policy policy ON policy.tenant_id = service.tenant_id
WHERE service.tenant_id = 0 AND service.service_no = ? AND service.customer_no = ?`,
		command.ServiceNo, subject.CustomerNo); err != nil {
		return nil, nil, nil, gerror.Wrap(err, "read customer server operation context")
	}
	if operationContext == nil || operationContext.ChannelNo == "" || !allowedAction(&serviceRow{
		EntitlementStatusCode: operationContext.EntitlementStatusCode,
		RuntimeStatusCode:     operationContext.RuntimeStatusCode,
		TrafficSuspended:      operationContext.TrafficSuspended,
	}, command.Action) {
		return nil, nil, nil, ErrActionUnavailable
	}
	return subject, normalized, operationContext, nil
}

func highRiskAction(action serveroperationresourcecap.Action, parameters json.RawMessage) bool {
	switch action {
	case serveroperationresourcecap.ActionResetPassword, serveroperationresourcecap.ActionReinstall,
		serveroperationresourcecap.ActionResize, serveroperationresourcecap.ActionChangeIP,
		serveroperationresourcecap.ActionSnapshotRestore, serveroperationresourcecap.ActionBackupRestore:
		return true
	case serveroperationresourcecap.ActionStop, serveroperationresourcecap.ActionReboot:
		var value struct {
			Mode string `json:"mode"`
		}
		if json.Unmarshal(parameters, &value) != nil {
			return true
		}
		return value.Mode != "" && value.Mode != "graceful"
	default:
		return false
	}
}

func accountOperationBinding(customerNo string, command AccountCommand,
	parameters json.RawMessage) (reauth.Binding, error) {
	canonical, err := json.Marshal(struct {
		Contract, CustomerNo, ServiceNo, Action, IdempotencyKey string
		Parameters                                              json.RawMessage
	}{"idc-core/customer-server-operation/v1", customerNo, command.ServiceNo,
		string(command.Action), command.IdempotencyKey, parameters})
	if err != nil {
		return reauth.Binding{}, gerror.Wrap(err, "encode server operation confirmation binding")
	}
	return reauth.Binding{Purpose: reauth.PurposeServerOperation,
		TargetDigest: reauth.DigestTarget(canonical)}, nil
}
