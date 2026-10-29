// This file appends one idempotent successor for a confirmed rejected creation.
// It changes only local owner records and never calls the upstream provider.

package provisionsource

import (
	"context"
	"crypto/sha256"
	"encoding/hex"

	"github.com/gogf/gf/v2/database/gdb"

	"lina-plugin-idc-core/backend/cap/provisionresourcecap"
)

// RecoverRejected retains original parameters, resource identity and terminal
// history. A previous task can have only one successor, even after lost replies.
func (s *Service) RecoverRejected(ctx context.Context, input provisionresourcecap.TaskReadRequest) (*provisionresourcecap.Task, error) {
	if ctx == nil || ctx.Err() != nil || s == nil || s.database == nil || provisionresourcecap.ValidateTaskReadRequest(input) != nil {
		return nil, provisionresourcecap.ErrTaskConflict
	}
	// An account task must commit even when this recovery must wait for it.
	// Running that prerequisite inside the failed-task transaction would roll
	// back the very task its independent account worker needs to process.
	if err := s.prepareRecoveryPrincipal(ctx, input); err != nil {
		return nil, err
	}
	var result *provisionresourcecap.Task
	err := s.database.Transaction(ctx, func(txCtx context.Context, _ gdb.TX) error {
		// Serialize recovery against this exact immutable failed attempt.
		locked, err := s.database.GetValue(txCtx, `SELECT task_no FROM plugin_idc_resource_task WHERE tenant_id=0 AND task_no=? FOR UPDATE`, input.ProviderTaskNo)
		if err != nil {
			return err
		}
		if locked.String() != input.ProviderTaskNo {
			return provisionresourcecap.ErrTaskUnavailable
		}
		previous, err := s.loadTaskByNumber(txCtx, input.ProviderTaskNo)
		if err != nil {
			return err
		}
		projection, readErr := s.ReadTask(txCtx, input)
		if readErr != nil {
			return readErr
		}
		if previous == nil || !sameTaskRequest(previous, input.Request) || projection.Status != provisionresourcecap.TaskStatusFailed ||
			(projection.FailureCode != "upstream_request_rejected" && projection.FailureCode != provisionresourcecap.FailureNonAcceptanceVerified && projection.FailureCode != provisionresourcecap.FailureRequestNotSent) {
			return provisionresourcecap.ErrTaskConflict
		}
		requestID := "core.reprovision." + input.ProviderTaskNo
		if existing, loadErr := s.loadTaskByRequest(txCtx, requestID); loadErr != nil {
			return loadErr
		} else if existing != nil {
			if !sameTaskCorrection(existing, input.RecoveryConfiguration) {
				return provisionresourcecap.ErrTaskConflict
			}
			result, err = s.ReadTask(txCtx, provisionresourcecap.TaskReadRequest{ProviderTaskNo: existing.TaskNo, Request: input.Request})
			return err
		}
		if err = s.requireRejectedTaskReceipt(txCtx, input.ProviderTaskNo, previous.LocalResourceNo); err != nil {
			return err
		}
		digest := sha256.Sum256([]byte(requestID))
		result, err = s.createTask(txCtx, input.Request, requestID, "resource-provision-recovery-"+hex.EncodeToString(digest[:]), input.RecoveryConfiguration)
		return err
	})
	return result, err
}

// prepareRecoveryPrincipal validates the terminal predecessor and its released
// credentials before scheduling the selected channel's account outside the
// successor transaction. Existing successors keep their original replay path.
func (s *Service) prepareRecoveryPrincipal(ctx context.Context, input provisionresourcecap.TaskReadRequest) error {
	if input.RecoveryConfiguration == nil {
		return nil
	}
	existing, err := s.loadTaskByRequest(ctx, "core.reprovision."+input.ProviderTaskNo)
	if err != nil {
		return err
	}
	if existing != nil {
		return nil
	}
	previous, err := s.ReadTask(ctx, input)
	if err != nil {
		return err
	}
	if previous.Status != provisionresourcecap.TaskStatusFailed || (previous.FailureCode != "upstream_request_rejected" && previous.FailureCode != provisionresourcecap.FailureNonAcceptanceVerified && previous.FailureCode != provisionresourcecap.FailureRequestNotSent) {
		return provisionresourcecap.ErrTaskConflict
	}
	resourceNo, _ := frozenServerIdentity(input.Request)
	if err = s.requireRejectedTaskReceipt(ctx, input.ProviderTaskNo, resourceNo); err != nil {
		return err
	}
	effective, err := s.correctedRequest(ctx, input.Request, *input.RecoveryConfiguration)
	if err != nil {
		return err
	}
	pools, err := s.resolveInterfacePools(ctx, effective.InterfaceInstanceNo, effective.InterfaceConfigurationRevision)
	if err != nil {
		return err
	}
	ready, err := s.principal(ctx, effective.CustomerNo, pools[0].PoolNo)
	if err != nil {
		return err
	}
	if !ready {
		return provisionresourcecap.ErrSourceUnavailable
	}
	return nil
}

// requireRejectedTaskReceipt preserves the same credential, binding and active
// resource checks before preparation and again under the predecessor row lock.
func (s *Service) requireRejectedTaskReceipt(ctx context.Context, taskNo, resourceNo string) error {
	ready, err := s.database.GetValue(ctx, `SELECT EXISTS (
 SELECT 1 FROM plugin_idc_resource_provision_outcome_receipt receipt
 JOIN plugin_idc_resource_provision_credential credential ON credential.task_no=receipt.task_no
 WHERE receipt.tenant_id=0 AND receipt.task_no=? AND receipt.status_code='failed'
 AND credential.status_code='revoked'
 AND NOT EXISTS(SELECT 1 FROM plugin_idc_resource_upstream_task_binding WHERE task_no=receipt.task_no)
 AND NOT EXISTS(SELECT 1 FROM plugin_idc_resource_mapping WHERE tenant_id=0 AND local_resource_no=? AND status_code='active')
)`, taskNo, resourceNo)
	if err != nil {
		return err
	}
	if !ready.Bool() {
		return provisionresourcecap.ErrTaskConflict
	}
	return nil
}
