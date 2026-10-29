// This file admits native commercial tasks under their durable core claim,
// preserving the existing task lease and keeping network I/O outside that claim.
package nativecommercial

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"lina-plugin-idc-core/backend/cap/servercommercialcap"
	"lina-plugin-idc-core/backend/cap/serveroperationresourcecap"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
	"lina-plugin-idc-resource/backend/internal/service/resourcetask"
	"time"
)

// Admission wraps the already-created ordinary native/legacy admission gate.
// Only task numbers in this owner's immutable commercial map use the new port.
func (s *Service) Admission(other resourcetask.WriteAdmission) resourcetask.WriteAdmission {
	return func(ctx context.Context, task resourcetask.ClaimedTask, request resourcecap.OperationRequest, provider resourcecap.Provider, invoke func(context.Context) error) (resultErr error) {
		denied := &resourcecap.ProviderCallError{Code: resourcecap.ProviderCallErrorCodeInvalidResponse}
		var record *binding
		if err := s.database.GetScan(ctx, &record, `SELECT operation_no,entitlement_no,customer_no,task_no,local_resource_no,channel_no,pool_no,external_resource_ref,action_code,parameters,request_digest,intent_digest,entitlement_version FROM plugin_idc_resource_native_commercial_task WHERE tenant_id=0 AND task_no=?`, task.TaskNo); err != nil {
			return denied
		}
		if record == nil {
			return other(ctx, task, request, provider, invoke)
		}
		invoked := false
		defer func() {
			if resultErr == nil || invoked {
				return
			}
			claimed, readErr := s.database.GetValue(ctx, `SELECT binding.admitted_at IS NOT NULL AND binding.admitted_attempt>COALESCE((task.owner_metadata->>'nonAcceptanceAttempt')::integer,0) FROM plugin_idc_resource_native_commercial_task binding JOIN plugin_idc_resource_task task ON task.tenant_id=binding.tenant_id AND task.task_no=binding.task_no WHERE binding.tenant_id=0 AND binding.task_no=?`, task.TaskNo)
			if readErr != nil || claimed.Bool() {
				resultErr = resourcetask.ErrAdmissionOutcomeUncertain
			}
		}()
		if !validTask(record, task, request) {
			return denied
		}
		readCtx, cancel := context.WithTimeout(ctx, 1500*time.Millisecond)
		observed, err := provider.QueryResource(readCtx, resourcecap.ResourceQuery{ExternalResourceRef: request.ResourceRef})
		cancel()
		if err != nil || observed == nil || observed.Validate() != nil || observed.ExternalResourceRef != request.ResourceRef || observed.ObservedAt.IsZero() || time.Since(observed.ObservedAt) > time.Minute || observed.ObservedAt.After(time.Now().Add(time.Second)) || !allowedState(record.ActionCode, observed.State) {
			return denied
		}
		err = s.commercial.WithQueuedOperation(ctx, record.OperationNo, task.TaskNo, func(txctx context.Context, operation servercommercialcap.Operation) error {
			if !sameIntent(record, operation) {
				return denied
			}
			route, routeErr := s.lockRoute(txctx, operation)
			if routeErr != nil || route.ExternalResourceRef != request.ResourceRef {
				return denied
			}
			conflict, readErr := s.database.GetValue(txctx, `SELECT EXISTS(SELECT 1 FROM plugin_idc_resource_task WHERE tenant_id=0 AND local_resource_no=? AND task_no<>? AND status_code NOT IN ('succeeded','failed','canceled'))`, record.LocalResourceNo, task.TaskNo)
			if readErr != nil || conflict.Bool() {
				return denied
			}
			if validateErr := s.resources.ValidateParameters(txctx, serveroperationresourcecap.CapabilityRoute{ServiceNo: record.LocalResourceNo, ChannelNo: record.ChannelNo, PoolNo: record.PoolNo}, record.ActionCode, record.Parameters); validateErr != nil {
				return denied
			}
			result, updateErr := s.database.Exec(txctx, `UPDATE plugin_idc_resource_native_commercial_task binding SET admitted_at=CURRENT_TIMESTAMP,admitted_attempt=?,updated_at=CURRENT_TIMESTAMP
FROM plugin_idc_resource_task task WHERE binding.tenant_id=0 AND binding.task_no=? AND task.tenant_id=binding.tenant_id AND task.task_no=binding.task_no
AND task.status_code='submitting' AND task.locked_by=? AND task.attempt_count=? AND task.lease_expires_at>CURRENT_TIMESTAMP AND task.result_unknown=false
AND (binding.admitted_attempt IS NULL OR (binding.admitted_attempt<? AND COALESCE((task.owner_metadata->>'nonAcceptanceAttempt')::integer,0)>=binding.admitted_attempt))`, task.AttemptNo, task.TaskNo, task.LeaseToken, task.AttemptNo, task.AttemptNo)
			if updateErr != nil {
				return updateErr
			}
			count, countErr := result.RowsAffected()
			if countErr != nil || count != 1 {
				return denied
			}
			return nil
		})
		if err != nil {
			return denied
		}
		// Core keeps this executing task as an authoritative refund/cancel fence.
		// A crash after claim is recovered by the resource lease as unknown, never resent.
		invoked = true
		return invoke(ctx)
	}
}

func validTask(record *binding, task resourcetask.ClaimedTask, request resourcecap.OperationRequest) bool {
	if record.ExternalResourceRef != request.ResourceRef || record.LocalResourceNo != task.LocalResourceNo || record.ChannelNo != task.ChannelNo || record.PoolNo != task.PoolNo || len(request.SecretBindings) != 0 || request.ProviderCorrelation != nil {
		return false
	}
	capability := commercialCapability(record.ActionCode)
	if capability == "" || capability != request.Capability {
		return false
	}
	var metadata struct {
		OperationNo                               string
		ExternalResourceRef                       string
		RequestDigest, TranslatedParametersDigest [32]byte
	}
	if json.Unmarshal([]byte(task.OwnerMetadata), &metadata) != nil || metadata.OperationNo != record.OperationNo || metadata.ExternalResourceRef != record.ExternalResourceRef || hex.EncodeToString(metadata.RequestDigest[:]) != record.RequestDigest {
		return false
	}
	original, err := canonicalParameters(record.Parameters)
	if err != nil {
		return false
	}
	digest, err := serveroperationresourcecap.TaskRequestDigest(serveroperationresourcecap.TaskRequest{OperationNo: record.OperationNo, ServiceNo: record.LocalResourceNo, ChannelNo: record.ChannelNo, PoolNo: record.PoolNo, Action: record.ActionCode, ExpectedServiceVersion: record.EntitlementVersion, Parameters: original})
	if err != nil || digest != record.RequestDigest {
		return false
	}
	normalized, err := canonicalParameters(request.Parameters)
	if err != nil || sha256.Sum256(normalized) != metadata.TranslatedParametersDigest {
		return false
	}
	requestID := sha256.Sum256([]byte("core.server-operation." + record.OperationNo))
	return hex.EncodeToString(requestID[:]) == task.RequestIDDigest
}

func sameIntent(record *binding, operation servercommercialcap.Operation) bool {
	stored, storedErr := canonicalParameters(record.Parameters)
	current, currentErr := canonicalParameters(operation.Parameters)
	return storedErr == nil && currentErr == nil && string(stored) == string(current) && record.OperationNo == operation.OperationNo && record.EntitlementNo == operation.EntitlementNo && record.CustomerNo == operation.CustomerNo && record.LocalResourceNo == operation.LocalResourceNo && record.ChannelNo == operation.ChannelNo && record.PoolNo == operation.PoolNo && record.ActionCode == operation.Action && record.IntentDigest == operation.ContentDigest && record.EntitlementVersion == operation.ExpectedEntitlementVersion
}

func commercialCapability(action serveroperationresourcecap.Action) resourcecap.CapabilityCode {
	switch action {
	case serveroperationresourcecap.ActionSnapshotCreate:
		return resourcecap.CapabilityManageSnapshot
	case serveroperationresourcecap.ActionBackupCreate:
		return resourcecap.CapabilityBackup
	case serveroperationresourcecap.ActionTrafficAdd:
		return resourcecap.CapabilityTrafficPackage
	case serveroperationresourcecap.ActionChangeIP:
		return resourcecap.CapabilityChangeIP
	case serveroperationresourcecap.ActionResize:
		return resourcecap.CapabilityResize
	default:
		return ""
	}
}

func allowedState(action serveroperationresourcecap.Action, state resourcecap.ResourceState) bool {
	if commercialCapability(action) == "" {
		return false
	}
	if action == serveroperationresourcecap.ActionTrafficAdd && state == resourcecap.ResourceStateSuspended {
		return true
	}
	return state == resourcecap.ResourceStateActive || state == resourcecap.ResourceStateRunning || state == resourcecap.ResourceStateStopped
}
