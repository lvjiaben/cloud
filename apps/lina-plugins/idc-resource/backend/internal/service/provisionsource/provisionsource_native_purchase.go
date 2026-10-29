// This file procures immutable imported units through the resource-owned task
// executor. Native purchases have no server pool, principal, hostname or login
// password; callback correlation and supplier credentials remain owner-held.
package provisionsource

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strconv"

	"github.com/gogf/gf/v2/errors/gerror"
	"lina-plugin-idc-core/backend/cap/provisionresourcecap"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
	"lina-plugin-idc-resource/backend/internal/composition"
	"lina-plugin-idc-resource/backend/internal/service/resourcetask"
)

// NativeProvisionParameterBuilder is the independent, secret-free imported
// purchase mapper. It grants no access to server fields or supplier execution.
type NativeProvisionParameterBuilder interface {
	BuildNative(context.Context, string, string, resourcecap.NativePurchasePlan) (json.RawMessage, error)
}

// createTask selects an explicit owner-owned plan arm before any server work.
func (s *Service) createTask(ctx context.Context, request provisionresourcecap.TaskRequest, requestID, credentialKey string, corrections ...*provisionresourcecap.RecoveryConfiguration) (*provisionresourcecap.Task, error) {
	if request.NativePurchase != nil {
		return s.createNativeTask(ctx, request, requestID, corrections...)
	}
	return s.createServerTask(ctx, request, requestID, credentialKey, corrections...)
}

// frozenNativeIdentity keeps native mapping separate from server projections.
func frozenNativeIdentity(request provisionresourcecap.TaskRequest) string {
	digest := sha256.Sum256([]byte("idc-resource/native-purchase/v1\x00" + request.Fulfillment.IntentNo + "\x00" + request.Fulfillment.OrderItemNo + "\x00" + strconv.Itoa(request.Fulfillment.UnitSequence)))
	return "NAT_" + hex.EncodeToString(digest[:16])
}

// createNativeTask reuses the durable owner idempotency fence and correlation.
func (s *Service) createNativeTask(ctx context.Context, request provisionresourcecap.TaskRequest, requestID string, corrections ...*provisionresourcecap.RecoveryConfiguration) (*provisionresourcecap.Task, error) {
	if !s.canCreateTask(ctx, request) || request.NativePurchase == nil {
		return nil, provisionresourcecap.ErrSourceUnavailable
	}
	for _, correction := range corrections {
		if correction != nil {
			return nil, provisionresourcecap.ErrTaskConflict
		}
	}
	existing, err := s.loadTaskByRequest(ctx, requestID)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		if !sameTaskRequest(existing, request) {
			return nil, provisionresourcecap.ErrTaskConflict
		}
		result, readErr := s.ReadTask(ctx, provisionresourcecap.TaskReadRequest{ProviderTaskNo: existing.TaskNo, Request: request})
		if result != nil {
			result.Reused = true
		}
		return result, readErr
	}
	route, parameters, err := s.prepareNativePurchase(ctx, request)
	if err != nil {
		return nil, provisionresourcecap.WrapPreparationFailure(provisionresourcecap.PreparationFailureParameters, err)
	}
	capabilityDigest, err := capabilityEvidenceDigest(route, parameters)
	if err != nil {
		return nil, err
	}
	metadata, err := taskOwnerMetadata(request, route.SnapshotNo, capabilityDigest)
	if err != nil {
		return nil, err
	}
	manager, err := composition.BindProviderSecretManager(s.secrets, route.ChannelNo)
	if err != nil {
		return nil, provisionresourcecap.ErrSourceUnavailable
	}
	correlation, err := s.providerCorrelationInput(ctx, manager, route, request, requestID)
	if err != nil || correlation == nil {
		return nil, provisionresourcecap.WrapPreparationFailure(provisionresourcecap.PreparationFailureCallback, provisionresourcecap.ErrSourceUnavailable)
	}
	created, err := s.tasks.Create(ctx, resourcetask.CreateInput{RequestID: requestID, Capability: resourcecap.CapabilityProvision, LocalResourceNo: frozenNativeIdentity(request), ChannelNo: route.ChannelNo, Parameters: parameters, OwnerMetadata: metadata, Priority: 100, MaxAttempts: 8, ProviderCorrelation: correlation})
	if err != nil {
		return nil, provisionresourcecap.WrapPreparationFailure(provisionresourcecap.PreparationFailureTaskStorage, err)
	}
	result, err := s.ReadTask(ctx, provisionresourcecap.TaskReadRequest{ProviderTaskNo: created.TaskNo, Request: request})
	if result != nil {
		result.Reused = !created.Created
	}
	return result, err
}

// prepareNativePurchase resolves only the exact enabled source channel and its
// current capability evidence; it never creates pools or customer principals.
func (s *Service) prepareNativePurchase(ctx context.Context, request provisionresourcecap.TaskRequest) (*routeEvidence, json.RawMessage, error) {
	mapper, ok := s.managedParameters.(NativeProvisionParameterBuilder)
	if !ok || request.NativePurchase == nil {
		return nil, nil, provisionresourcecap.ErrParametersUnavailable
	}
	plan := resourcecap.NativePurchasePlan{Version: resourcecap.NativePurchasePlanVersion, Protocol: request.NativePurchaseProtocol, Purchase: *request.NativePurchase}
	if err := plan.Validate(); err != nil {
		return nil, nil, err
	}
	var route *routeEvidence
	err := s.database.GetScan(ctx, &route, `WITH selected AS (
 SELECT channel.channel_no,channel.provider_id,channel.provider_plugin_id,channel.provider_runtime_type,channel.version AS channel_version
 FROM plugin_idc_resource_interface_channel_binding binding
 JOIN plugin_idc_resource_channel channel ON channel.tenant_id=binding.tenant_id AND channel.channel_no=binding.channel_no
 JOIN plugin_idc_resource_interface_configuration_revision revision ON revision.tenant_id=binding.tenant_id AND revision.interface_instance_no=binding.interface_instance_no AND revision.revision_no=?
 WHERE binding.tenant_id=0 AND binding.interface_instance_no=? AND channel.status_code='enabled' AND channel.deleted_at IS NULL AND channel.provider_runtime_type='dynamic'
), latest AS (
 SELECT snapshot.snapshot_no,snapshot.channel_no,snapshot.adapter_version,snapshot.item_count,snapshot.observed_at
 FROM plugin_idc_resource_capability_snapshot snapshot JOIN selected ON selected.channel_no=snapshot.channel_no
 WHERE snapshot.tenant_id=0 AND snapshot.status_code='complete' ORDER BY snapshot.observed_at DESC,snapshot.id DESC LIMIT 1
)
SELECT selected.*,latest.snapshot_no,latest.adapter_version,latest.item_count,latest.observed_at,capability.support_code,capability.constraints::TEXT AS constraints,
 (SELECT COUNT(*)::INT FROM plugin_idc_resource_capability_snapshot_item item WHERE item.tenant_id=0 AND item.snapshot_no=latest.snapshot_no) AS actual_item_count
 FROM selected JOIN latest ON latest.channel_no=selected.channel_no
 JOIN plugin_idc_resource_capability_snapshot_item capability ON capability.tenant_id=0 AND capability.snapshot_no=latest.snapshot_no AND capability.capability_code=?`, request.InterfaceConfigurationRevision, request.InterfaceInstanceNo, resourcecap.CapabilityProvision)
	if err != nil {
		return nil, nil, gerror.Wrap(err, "resolve native purchase channel")
	}
	if route == nil || route.ChannelNo == "" || route.ItemCount != resourcecap.MaximumCapabilityCount || route.ActualItemCount != resourcecap.MaximumCapabilityCount || (route.SupportCode != string(resourcecap.SupportLevelSupported) && route.SupportCode != string(resourcecap.SupportLevelConditional)) {
		return nil, nil, provisionresourcecap.ErrCapabilityUnavailable
	}
	now, err := databaseNow(ctx, s.database)
	if err != nil {
		return nil, nil, err
	}
	if route.ObservedAt.IsZero() || route.ObservedAt.After(now) || !route.ObservedAt.UTC().Add(capabilityEvidenceLifetime).After(now) {
		return nil, nil, provisionresourcecap.ErrCapabilityUnavailable
	}
	parameters, err := mapper.BuildNative(ctx, route.ChannelNo, request.InterfaceConfigurationRevision, plan)
	if err != nil || validateCapabilityParameters(route.Constraints, parameters) != nil {
		return nil, nil, errors.Join(provisionresourcecap.ErrParametersUnavailable, err)
	}
	return route, parameters, nil
}
