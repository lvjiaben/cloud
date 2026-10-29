// This file rechecks native queued authority immediately before any cloud write.

package customeroperations

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"lina-plugin-idc-core/backend/cap/customerauthcap"
	"lina-plugin-idc-core/backend/cap/servercommercialcap"
	"lina-plugin-idc-core/backend/cap/serveroperationresourcecap"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
	"lina-plugin-idc-resource/backend/internal/service/resourcetask"
)

// NewWriteAdmission constructs the source-only native task dispatch gate. Other
// owner task protocols retain their own existing admission and execution policy.
func NewWriteAdmission(database gdb.DB, entitlements customerauthcap.QueuedEntitlementAccess, parameters queuedParameters, objects servercommercialcap.Service) (resourcetask.WriteAdmission, error) {
	queuedObjects, ok := objects.(queuedRestoreAccess)
	queuedWrites, writesOK := objects.(servercommercialcap.QueuedWriteAccess)
	if database == nil || entitlements == nil || parameters == nil || !ok || !writesOK {
		return nil, ErrUnavailable
	}
	admission := &nativeWriteAdmission{database: database, entitlements: entitlements, parameters: parameters, queuedObjects: queuedObjects, queuedWrites: queuedWrites}
	return admission.admit, nil
}

type nativeWriteAdmission struct {
	database      gdb.DB
	entitlements  customerauthcap.QueuedEntitlementAccess
	parameters    queuedParameters
	queuedObjects queuedRestoreAccess
	queuedWrites  servercommercialcap.QueuedWriteAccess
}

func (a *nativeWriteAdmission) admit(ctx context.Context, task resourcetask.ClaimedTask, request resourcecap.OperationRequest, provider resourcecap.Provider, invoke func(context.Context) error) (resultErr error) {
	var identity struct {
		OperationNo string `json:"operationNo"`
	}
	if json.Unmarshal([]byte(task.OwnerMetadata), &identity) != nil {
		return &resourcecap.ProviderCallError{Code: resourcecap.ProviderCallErrorCodeInvalidResponse}
	}
	if !strings.HasPrefix(identity.OperationNo, "nso_") {
		return invoke(ctx)
	}

	invoked := false
	defer func() {
		if resultErr == nil || invoked {
			return
		}
		claimed, readErr := a.database.GetValue(ctx, `SELECT COALESCE((owner_metadata->>'nativeWriteClaim')::integer,0)>COALESCE((owner_metadata->>'nonAcceptanceAttempt')::integer,0) FROM plugin_idc_resource_task WHERE tenant_id=0 AND task_no=?`, task.TaskNo)
		if readErr != nil || claimed.Bool() {
			resultErr = resourcetask.ErrAdmissionOutcomeUncertain
		}
	}()

	metadata, err := validateNativeTask(task, request)
	if err != nil {
		return err
	}
	action := string(metadata.Action)
	denied := &resourcecap.ProviderCallError{Code: resourcecap.ProviderCallErrorCodeInvalidResponse}

	var entitlementIdentity *struct {
		EntitlementNo string `orm:"entitlement_no"`
		CustomerNo    string `orm:"customer_no"`
	}
	if err = a.database.GetScan(ctx, &entitlementIdentity, `SELECT entitlement_no,customer_no FROM plugin_idc_resource_server_entitlement_projection WHERE tenant_id=0 AND local_resource_no=?`, task.LocalResourceNo); err != nil || entitlementIdentity == nil {
		return denied
	}
	// The read is bounded and occurs before taking any owner row lock. Both
	// frozen identity and the task lease are rechecked in the short claim TX.
	if gdb.TXFromCtx(ctx, a.database.GetGroup()) != nil {
		return denied
	}
	if authorityErr := a.entitlements.WithQueuedEntitlement(ctx, entitlementIdentity.EntitlementNo, metadata.ExpectedServiceVersion, func(_ context.Context, current *customerauthcap.Entitlement) error {
		if current == nil || current.CustomerNo != entitlementIdentity.CustomerNo {
			return denied
		}
		return nil
	}); authorityErr != nil {
		return denied
	}
	if observeErr := observeNativeWrite(ctx, provider, request.ResourceRef, action); observeErr != nil {
		return denied
	}
	err = a.entitlements.WithQueuedEntitlement(ctx, entitlementIdentity.EntitlementNo, metadata.ExpectedServiceVersion, func(ctx context.Context, entitlement *customerauthcap.Entitlement) error {
		var owned *route
		if entitlement == nil {
			return denied
		}
		if readErr := a.database.GetScan(ctx, &owned, strings.Replace(ownedRouteQuery, "ownership.server_no=?", "ownership.entitlement_no=?", 1)+` FOR UPDATE OF ownership,mapping`, entitlement.CustomerNo, entitlementIdentity.EntitlementNo); readErr != nil {
			return denied
		}
		if owned == nil || owned.LocalResourceNo != task.LocalResourceNo || owned.EntitlementNo != entitlementIdentity.EntitlementNo || owned.ChannelNo != task.ChannelNo || owned.PoolNo != task.PoolNo || owned.ExternalResourceRef != request.ResourceRef {
			return denied
		}
		busy, readErr := a.database.GetValue(ctx, `SELECT EXISTS(SELECT 1 FROM plugin_idc_resource_task WHERE tenant_id=0 AND local_resource_no=? AND task_no<>? AND status_code NOT IN ('succeeded','failed','canceled'))`, task.LocalResourceNo, task.TaskNo)
		if readErr != nil || busy.Bool() {
			return denied
		}
		if validateErr := a.parameters.ValidateParameters(ctx, serveroperationresourcecap.CapabilityRoute{ServiceNo: task.LocalResourceNo, ChannelNo: task.ChannelNo, PoolNo: task.PoolNo}, metadata.Action, metadata.SourceParameters); validateErr != nil {
			return denied
		}
		if restoreAction(metadata.Action) {
			object, objectErr := accessoryObjectRequest(owned, metadata.Action, metadata.SourceParameters)
			if objectErr != nil || a.queuedObjects.CheckQueuedRestore(ctx, object) != nil {
				return denied
			}
		}
		if metadata.Action == serveroperationresourcecap.ActionReinstall {
			if imageErr := validateReinstallImage(ctx, a.parameters, serveroperationresourcecap.CapabilityRoute{ServiceNo: task.LocalResourceNo, ChannelNo: task.ChannelNo, PoolNo: task.PoolNo}, metadata.SourceParameters); imageErr != nil {
				return denied
			}
		}
		if claimErr := claimNativeWrite(ctx, a.database, task); claimErr != nil {
			return denied
		}
		return a.queuedWrites.HoldQueuedWrite(ctx, servercommercialcap.QueuedWrite{OperationNo: metadata.OperationNo, TaskNo: task.TaskNo, EntitlementNo: owned.EntitlementNo, CustomerNo: owned.CustomerNo, RequestDigest: hex.EncodeToString(metadata.RequestDigest[:]), ExpectedEntitlementVersion: metadata.ExpectedServiceVersion, Action: metadata.Action})
	})
	if err != nil {
		return denied
	}
	// Core's durable fence survives this commit and any uncertain response.
	// A reclaimed worker cannot invoke again because the task claim persists.
	invoked = true
	return invoke(ctx)
}

func observeNativeWrite(ctx context.Context, provider resourcecap.Provider, resourceRef, action string) error {
	readCtx, cancel := context.WithTimeout(ctx, customerObservationBudget)
	defer cancel()
	observation, err := provider.QueryResource(readCtx, resourcecap.ResourceQuery{ExternalResourceRef: resourceRef})
	now := time.Now()
	if err != nil || observation == nil || observation.Validate() != nil || observation.ExternalResourceRef != resourceRef || now.Sub(observation.ObservedAt) > time.Minute || observation.ObservedAt.After(now.Add(time.Second)) || !allowedObservation(observation, action) {
		return ErrConflict
	}
	return nil
}

func requestIDHash(operationNo string) string {
	sum := sha256.Sum256([]byte("core.server-operation." + operationNo))
	return hex.EncodeToString(sum[:])
}

type queuedParameters interface {
	serveroperationresourcecap.ParameterValidator
	serveroperationresourcecap.ReinstallDirectory
}

type nativeTaskMetadata struct {
	ExternalResourceRef        string                            `json:"externalResourceRef"`
	SourceParameters           json.RawMessage                   `json:"sourceParameters"`
	ContractVersion            string                            `json:"contractVersion"`
	OperationNo                string                            `json:"operationNo"`
	Action                     serveroperationresourcecap.Action `json:"action"`
	ExpectedServiceVersion     int64                             `json:"expectedServiceVersion"`
	RequestDigest              [sha256.Size]byte                 `json:"requestDigest"`
	TranslatedParametersDigest [sha256.Size]byte                 `json:"translatedParametersDigest"`
}

func validateNativeTask(task resourcetask.ClaimedTask, request resourcecap.OperationRequest) (*nativeTaskMetadata, error) {
	var metadata nativeTaskMetadata
	denied := &resourcecap.ProviderCallError{Code: resourcecap.ProviderCallErrorCodeInvalidResponse}
	if json.Unmarshal([]byte(task.OwnerMetadata), &metadata) != nil || metadata.ContractVersion != serveroperationresourcecap.ContractVersion || !strings.HasPrefix(metadata.OperationNo, "nso_") || requestIDHash(metadata.OperationNo) != task.RequestIDDigest || request.ProviderCorrelation != nil || metadata.ExternalResourceRef == "" || metadata.ExternalResourceRef != request.ResourceRef {
		return nil, denied
	}
	capability, ok := nativeCapability(metadata.Action)
	if !ok || request.Capability != capability {
		return nil, denied
	}
	canonicalParameters, err := normalizeTranslated(request.Parameters)
	if err != nil || sha256.Sum256(canonicalParameters) != metadata.TranslatedParametersDigest {
		return nil, denied
	}
	if len(metadata.SourceParameters) == 0 {
		metadata.SourceParameters = canonicalParameters
	}
	normalized, err := normalizeParameters(metadata.Action, metadata.SourceParameters)
	if err != nil {
		return nil, denied
	}
	metadata.SourceParameters = normalized
	bindings := make([]serveroperationresourcecap.SecretBinding, len(request.SecretBindings))
	for i, binding := range request.SecretBindings {
		bindings[i] = serveroperationresourcecap.SecretBinding{Name: binding.Name, Handle: binding.Handle}
	}
	if metadata.Action == serveroperationresourcecap.ActionResetPassword || metadata.Action == serveroperationresourcecap.ActionEnterRescue {
		bindingName := "password"
		if metadata.Action == serveroperationresourcecap.ActionEnterRescue {
			bindingName = "temporary_password"
		}
		if len(bindings) != 1 || bindings[0].Name != bindingName || bindings[0].Handle.IsZero() {
			return nil, denied
		}
	} else if len(bindings) != 0 {
		return nil, denied
	}
	frozen := serveroperationresourcecap.TaskRequest{OperationNo: metadata.OperationNo, ServiceNo: task.LocalResourceNo, ChannelNo: task.ChannelNo, PoolNo: task.PoolNo, Action: metadata.Action, ExpectedServiceVersion: metadata.ExpectedServiceVersion, Parameters: metadata.SourceParameters, SecretBindings: bindings}
	digest, err := serveroperationresourcecap.TaskRequestDigest(frozen)
	if err != nil || digest != hex.EncodeToString(metadata.RequestDigest[:]) {
		return nil, denied
	}
	return &metadata, nil
}

func normalizeTranslated(raw json.RawMessage) (json.RawMessage, error) {
	values, err := scalarObject(raw)
	if err != nil {
		return nil, err
	}
	return json.Marshal(values)
}

func nativeCapability(action serveroperationresourcecap.Action) (resourcecap.CapabilityCode, bool) {
	switch action {
	case serveroperationresourcecap.ActionStart:
		return resourcecap.CapabilityStart, true
	case serveroperationresourcecap.ActionStop:
		return resourcecap.CapabilityStop, true
	case serveroperationresourcecap.ActionReboot:
		return resourcecap.CapabilityReboot, true
	case serveroperationresourcecap.ActionResetPassword:
		return resourcecap.CapabilityResetPassword, true
	case serveroperationresourcecap.ActionEnterRescue:
		return resourcecap.CapabilityEnterRescue, true
	case serveroperationresourcecap.ActionExitRescue:
		return resourcecap.CapabilityExitRescue, true
	case serveroperationresourcecap.ActionReinstall:
		return resourcecap.CapabilityReinstall, true
	case serveroperationresourcecap.ActionConsole:
		return resourcecap.CapabilityConsole, true
	case serveroperationresourcecap.ActionSecurityGroupAttach, serveroperationresourcecap.ActionSecurityGroupDetach:
		return resourcecap.CapabilitySecurityGroup, true
	case serveroperationresourcecap.ActionNATCreate, serveroperationresourcecap.ActionNATDelete:
		return resourcecap.CapabilityNAT, true
	case serveroperationresourcecap.ActionVirtualMediaMount, serveroperationresourcecap.ActionVirtualMediaUnmount:
		return resourcecap.CapabilityVirtualMedia, true
	case serveroperationresourcecap.ActionSnapshotDelete, serveroperationresourcecap.ActionSnapshotRestore:
		return resourcecap.CapabilityManageSnapshot, true
	case serveroperationresourcecap.ActionBackupDelete, serveroperationresourcecap.ActionBackupRestore:
		return resourcecap.CapabilityBackup, true
	default:
		return "", false
	}
}

// claimNativeWrite fences repeated dispatch of the same immutable task, even
// across workers. Failure of the core hold rolls this update back in the same TX.
func claimNativeWrite(ctx context.Context, database gdb.DB, task resourcetask.ClaimedTask) error {
	if gdb.TXFromCtx(ctx, database.GetGroup()) == nil {
		return ErrUnavailable
	}
	result, err := database.Exec(ctx, `UPDATE plugin_idc_resource_task SET owner_metadata=owner_metadata||jsonb_build_object('nativeWriteClaim',?::integer)
WHERE tenant_id=0 AND task_no=? AND status_code='submitting' AND locked_by=? AND attempt_count=? AND lease_expires_at>CURRENT_TIMESTAMP
AND NOT result_unknown AND (NOT jsonb_exists(owner_metadata,'nativeWriteClaim') OR ((owner_metadata->>'nativeWriteClaim')::integer<? AND COALESCE((owner_metadata->>'nonAcceptanceAttempt')::integer,0)>=(owner_metadata->>'nativeWriteClaim')::integer)) AND owner_metadata=?::jsonb`, task.AttemptNo, task.TaskNo, task.LeaseToken, task.AttemptNo, task.AttemptNo, task.OwnerMetadata)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil || count != 1 {
		return ErrConflict
	}
	return nil
}
