// This file commits native free management intents, confirmation consumption,
// secrets and owner tasks atomically before any asynchronous supplier write.
package customeroperations

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"lina-plugin-idc-core/backend/cap/secretcap"

	"lina-plugin-idc-core/backend/cap/customerauthcap"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/cap/serveroperationresourcecap"
)

type Command struct {
	ServerNo, Action, IdempotencyKey string
	Confirmed                        bool
	Parameters                       json.RawMessage
	Password                         []byte
	ProofToken                       string
}

func (s *Service) Create(ctx context.Context, command Command) (*Operation, error) {
	defer clear(command.Password)
	action := serveroperationresourcecap.Action(command.Action)
	if !keyPattern.MatchString(command.IdempotencyKey) || len(command.ProofToken) > 4096 || len(command.Password) > 1024 || (action != serveroperationresourcecap.ActionStart && action != serveroperationresourcecap.ActionConsole && !command.Confirmed) {
		return nil, ErrInvalid
	}
	parameters, err := normalizeParameters(action, command.Parameters)
	if err != nil {
		return nil, err
	}
	owned, err := s.ownedRoute(ctx, command.ServerNo, false)
	if err != nil {
		return nil, err
	}
	operationNo := operationNumber(owned.CustomerNo, owned.ServerNo, command.IdempotencyKey)
	digest := parametersDigest(parameters)
	// Replays return the exact existing intent even after runtime or entitlement
	// changes. They never perform another supplier read or submit another task.
	if existing, readErr := s.loadOperation(ctx, owned.LocalResourceNo, operationNo, command.Action, digest); readErr != nil || existing != nil {
		return existing, readErr
	}
	observationCtx, cancelObservation := context.WithTimeout(ctx, customerObservationBudget)
	observation, err := s.observe(observationCtx, owned)
	cancelObservation()
	if err != nil || !allowedObservation(observation, command.Action) {
		return nil, ErrConflict
	}
	if err = validatePassword(action, command.Password); err != nil {
		return nil, err
	}
	var result *Operation
	var prepared *serveroperationresourcecap.PreparedInputSecret
	err = s.entitlements.WithCurrentEntitlement(ctx, owned.EntitlementNo, func(ctx context.Context, entitlement *customerauthcap.Entitlement) error {
		locked, lockErr := s.ownedRoute(ctx, command.ServerNo, true)
		if lockErr != nil || !sameRoute(locked, owned) {
			return ErrUnavailable
		}
		if existing, readErr := s.loadOperation(ctx, owned.LocalResourceNo, operationNo, command.Action, digest); readErr != nil || existing != nil {
			result = existing
			return readErr
		}
		busy, checkErr := s.busy(ctx, owned.LocalResourceNo)
		if checkErr != nil || busy {
			return ErrConflict
		}
		if validateErr := s.validateInputs(ctx, owned, action, parameters); validateErr != nil {
			return ErrConflict
		}
		if proofErr := s.consumeConfirmation(ctx, command, action, parameters); proofErr != nil {
			return proofErr
		}
		var bindings []serveroperationresourcecap.SecretBinding
		prepared, bindings, checkErr = s.preparePassword(ctx, owned, operationNo, action, command.Password)
		if checkErr != nil {
			return checkErr
		}
		request := serveroperationresourcecap.TaskRequest{OperationNo: operationNo, ServiceNo: owned.LocalResourceNo, ChannelNo: owned.ChannelNo, PoolNo: owned.PoolNo, Action: serveroperationresourcecap.Action(command.Action), ExpectedServiceVersion: entitlement.Version, Parameters: parameters, SecretBindings: bindings}
		request.RequestDigest, checkErr = serveroperationresourcecap.TaskRequestDigest(request)
		if checkErr != nil {
			return ErrUnavailable
		}
		created, createErr := s.operations.CreateTask(ctx, request)
		if createErr != nil || created == nil {
			return errors.Join(ErrUnavailable, createErr)
		}
		if receiptErr := s.recordAccessoryDeletion(ctx, owned, action, parameters, operationNo, created.ProviderTaskNo); receiptErr != nil {
			return receiptErr
		}
		if auditErr := s.audit.AppendBusinessTransition(ctx, idccap.BusinessAuditTransition{IdempotencyScope: "resource.customer-operation.create", IdempotencyKey: operationNo, OperationCode: "resource.customer-operation.create", TargetType: "server_service", TargetNo: command.ServerNo, CustomerNo: owned.CustomerNo, ResultCode: "succeeded", ReasonCode: "customer_request", RiskCode: "medium", CorrelationNo: operationNo, BeforeSnapshot: map[string]any{}, AfterSnapshot: map[string]any{"action": command.Action, "entitlement_version": entitlement.Version, "runtime_status": string(observation.State)}}); auditErr != nil {
			return auditErr
		}
		result, createErr = s.loadOperation(ctx, owned.LocalResourceNo, operationNo, command.Action, digest)
		if createErr != nil || result == nil {
			return ErrUnavailable
		}
		return nil
	})
	if err != nil && prepared != nil && !prepared.Reused {
		if revokeErr := s.operations.RevokeInputSecret(ctx, owned.ChannelNo, prepared.Handle); revokeErr != nil {
			return nil, errors.Join(ErrUnavailable, err, revokeErr)
		}
	}
	return result, err
}

func (s *Service) loadOperation(ctx context.Context, resourceNo, operationNo, action, digest string) (*Operation, error) {
	var row *struct {
		Operation
		OwnerMetadata string
	}
	if err := s.database.GetScan(ctx, &row, `SELECT owner_metadata->>'operationNo' AS operation_no,owner_metadata->>'action' AS action,status_code AS status,created_at,updated_at,owner_metadata::text AS owner_metadata FROM plugin_idc_resource_task WHERE tenant_id=0 AND local_resource_no=? AND owner_metadata->>'operationNo'=?`, resourceNo, operationNo); err != nil {
		return nil, ErrUnavailable
	}
	if row == nil {
		return nil, nil
	}
	var metadata struct {
		TranslatedParametersDigest [32]byte        `json:"translatedParametersDigest"`
		SourceParameters           json.RawMessage `json:"sourceParameters"`
	}
	if json.Unmarshal([]byte(row.OwnerMetadata), &metadata) != nil || row.OperationNo != operationNo || row.Action != action {
		return nil, ErrConflict
	}
	actualDigest := encodeDigest(metadata.TranslatedParametersDigest)
	if len(metadata.SourceParameters) > 0 {
		canonical, normalizeErr := normalizeParameters(serveroperationresourcecap.Action(action), metadata.SourceParameters)
		if normalizeErr != nil {
			return nil, ErrConflict
		}
		actualDigest = parametersDigest(canonical)
	}
	if actualDigest != digest {
		return nil, ErrConflict
	}
	return &row.Operation, nil
}

// Prepare proves current ownership and action readiness before asking core to
// bind a short-lived confirmation to the real customer login session.
func (s *Service) Prepare(ctx context.Context, command Command) (*customerauthcap.SessionConfirmation, error) {
	if !keyPattern.MatchString(command.IdempotencyKey) {
		return nil, ErrInvalid
	}
	action := serveroperationresourcecap.Action(command.Action)
	parameters, err := normalizeParameters(action, command.Parameters)
	if err != nil {
		return nil, err
	}
	owned, err := s.ownedRoute(ctx, command.ServerNo, false)
	if err != nil {
		return nil, err
	}
	entitlement, err := s.entitlements.ReadCurrentEntitlement(ctx, owned.EntitlementNo)
	if err != nil || entitlement == nil || !entitlement.CanOperate {
		return nil, ErrUnavailable
	}
	busy, err := s.busy(ctx, owned.LocalResourceNo)
	if err != nil || busy {
		return nil, ErrConflict
	}
	callCtx, cancel := context.WithTimeout(ctx, customerObservationBudget)
	defer cancel()
	observation, err := s.observe(callCtx, owned)
	if err != nil || !allowedObservation(observation, command.Action) {
		return nil, ErrConflict
	}
	if err = s.validateInputs(ctx, owned, action, parameters); err != nil {
		return nil, ErrConflict
	}
	if !highRisk(action, parameters) {
		return &customerauthcap.SessionConfirmation{}, nil
	}
	return s.confirmation.PrepareCurrentConfirmation(ctx, confirmationDigest(command, parameters))
}

func (s *Service) validateInputs(ctx context.Context, owned *route, action serveroperationresourcecap.Action, parameters json.RawMessage) error {
	route := serveroperationresourcecap.CapabilityRoute{ServiceNo: owned.LocalResourceNo, ChannelNo: owned.ChannelNo, PoolNo: owned.PoolNo}
	if err := s.operations.ValidateParameters(ctx, route, action, parameters); err != nil {
		return err
	}
	if restoreAction(action) {
		object, err := accessoryObjectRequest(owned, action, parameters)
		if err != nil {
			return err
		}
		return s.objects.CheckRestore(ctx, object)
	}
	if action != serveroperationresourcecap.ActionReinstall {
		return nil
	}
	return validateReinstallImage(ctx, s.operations, route, parameters)
}

func validateReinstallImage(ctx context.Context, images serveroperationresourcecap.ReinstallDirectory, route serveroperationresourcecap.CapabilityRoute, parameters json.RawMessage) error {
	var input struct {
		OS string `json:"os"`
	}
	if json.Unmarshal(parameters, &input) != nil || input.OS == "" {
		return ErrInvalid
	}
	directory, err := images.GetReinstallImages(ctx, route)
	if err != nil || directory == nil {
		return ErrConflict
	}
	for _, image := range directory.Images {
		if image.Reference == input.OS && image.Selectable {
			return nil
		}
	}
	return ErrConflict
}

func validatePassword(action serveroperationresourcecap.Action, password []byte) error {
	if action == serveroperationresourcecap.ActionResetPassword || action == serveroperationresourcecap.ActionEnterRescue {
		if len(password) < 8 || bytes.ContainsAny(password, "\r\n\x00") {
			return ErrInvalid
		}
	} else if len(password) != 0 {
		return ErrInvalid
	}
	return nil
}

// preparePassword returns only opaque bindings and the cleanup ownership marker.
func (s *Service) preparePassword(ctx context.Context, owned *route, operationNo string, action serveroperationresourcecap.Action, password []byte) (*serveroperationresourcecap.PreparedInputSecret, []serveroperationresourcecap.SecretBinding, error) {
	if action != serveroperationresourcecap.ActionResetPassword && action != serveroperationresourcecap.ActionEnterRescue {
		return nil, nil, nil
	}
	bindingName := "password"
	if action == serveroperationresourcecap.ActionEnterRescue {
		bindingName = "temporary_password"
	}
	prepared, err := s.operations.PrepareInputSecret(ctx, serveroperationresourcecap.InputSecretRequest{ChannelNo: owned.ChannelNo, IdempotencyKey: operationNo, BindingName: bindingName, Produce: func(consume secretcap.Consumer) error { return consume(password) }})
	if err != nil || prepared == nil || prepared.Handle.IsZero() {
		return prepared, nil, ErrUnavailable
	}
	return prepared, []serveroperationresourcecap.SecretBinding{{Name: bindingName, Handle: prepared.Handle}}, nil
}

// consumeConfirmation binds the current session proof to the canonical intent.
func (s *Service) consumeConfirmation(ctx context.Context, command Command, action serveroperationresourcecap.Action, parameters json.RawMessage) error {
	if highRisk(action, parameters) {
		if err := s.confirmation.ConsumeCurrentConfirmation(ctx, confirmationDigest(command, parameters), command.ProofToken); err != nil {
			return ErrConflict
		}
		return nil
	}
	if command.ProofToken != "" {
		return ErrInvalid
	}
	return nil
}
