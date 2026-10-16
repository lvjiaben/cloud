// This file validates immutable local renewal evidence before issuing one
// scoped confidential runtime context and invoking the exact provider release.
package peerrenewal

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"slices"
	"time"

	"lina-core/pkg/plugin/capability/capmodel"
	"lina-core/pkg/plugin/pluginhost"
	"lina-plugin-idc-core/backend/cap/renewalownercap"
	"lina-plugin-idc-core/backend/cap/resourceinterfaceprojectioncap"
)

type frozen struct {
	ReadEvidence, ResourceConfigurationRevision                                                                                                                    string
	EntitlementNo, OperationNo, PriceSnapshotNo, OwnerID, PluginID, ReleaseVersion, ContractVersion, DeclarationDigest, ReleaseDigest                              string
	RenewalMethod, ExternalReference, TaskExternalReference, UpstreamServiceNo, InterfaceInstanceNo, SourceIntentNo, OrderNo, OrderItemNo, ConfigurationSnapshotNo string
	InterfaceConfigurationRevision, ExpectedVersion                                                                                                                int64
	OriginalExpiresAt, ProjectedExpiresAt                                                                                                                          time.Time
	PurchaseCycles, PeriodCount                                                                                                                                    int
	PeriodUnitCode, CurrencyCode                                                                                                                                   string
}

func (s *source) invoke(ctx context.Context, input renewalownercap.TaskRequest, method string) (*renewalownercap.Task, error) {
	if ctx == nil || ctx.Err() != nil || renewalownercap.ValidateTaskRequest(input) != nil || input.OwnerID != renewalownercap.PeerOwnerID || input.PluginID != renewalownercap.PeerOwnerID || input.RenewalMethod != renewalownercap.PeerRenewMethod {
		return nil, renewalownercap.ErrTaskConflict
	}
	row, err := s.load(ctx, input)
	if err != nil {
		return nil, err
	}
	request := renewalownercap.PeerRequest{Request: input, ExternalReference: row.ExternalReference, UpstreamServiceNo: row.UpstreamServiceNo, PurchaseCycles: row.PurchaseCycles, CurrencyCode: row.CurrencyCode, PeriodUnitCode: row.PeriodUnitCode, PeriodCount: row.PeriodCount}
	if renewalownercap.ValidatePeerRequest(request) != nil {
		return nil, renewalownercap.ErrTaskConflict
	}
	raw, err := json.Marshal(request)
	if err != nil {
		return nil, renewalownercap.ErrTaskConflict
	}
	var result *capmodel.BusinessExtensionMethodResult
	if s.outbound == nil {
		return nil, renewalownercap.ErrOwnerUnavailable
	}
	var read resourceinterfaceprojectioncap.CatalogReadEvidence
	if json.Unmarshal([]byte(row.ReadEvidence), &read) != nil || read.Validate() != nil || read.InterfaceInstanceNo != row.InterfaceInstanceNo || read.InterfaceConfigurationRevision != row.ResourceConfigurationRevision || read.PluginID != row.PluginID || read.PluginReleaseVersion != row.ReleaseVersion || read.ContractVersion != row.ContractVersion || read.ReleaseDigest != row.ReleaseDigest || read.DeclarationDigest != row.DeclarationDigest || !slices.Contains(read.AuthorizedMethods, method) {
		return nil, renewalownercap.ErrTaskConflict
	}
	crossed := false
	err = s.outbound.WithOutbound(ctx, resourceinterfaceprojectioncap.ExistingBusinessOutboundRequest{InterfaceInstanceNo: row.InterfaceInstanceNo, InterfaceConfigurationRevision: read.InterfaceConfigurationRevision, ResourceChannelRef: read.ResourceChannelRef, PluginID: row.PluginID, PluginReleaseVersion: row.ReleaseVersion, ContractVersion: row.ContractVersion, ReleaseDigest: row.ReleaseDigest, DeclarationDigest: row.DeclarationDigest, Method: method}, func(handler pluginhost.ConfidentialOutboundHandler) error {
		return s.extensions.WithConfidentialOutboundContext(ctx, pluginhost.ConfidentialOutboundContextInput{ProviderPluginID: row.PluginID, Point: capmodel.BusinessExtensionProductFulfillment, ContractVersion: row.ContractVersion, ProviderMethod: method, OutboundMethod: "resource-provider." + method, ReleaseVersion: row.ReleaseVersion, ReleaseDigest: row.ReleaseDigest, DeclarationDigest: row.DeclarationDigest, InterfaceInstanceID: row.InterfaceInstanceNo, Purpose: capmodel.BusinessExtensionInvocationPurposeExistingBusiness, ExpiresAfter: 30 * time.Second, AllowedResultFields: []string{"StatusCode", "Data", "Authenticated"}, Handler: handler}, func(callCtx context.Context, outboundContext *capmodel.BusinessExtensionRuntimeContext) error {
			crossed = true
			var invokeErr error
			result, invokeErr = s.extensions.InvokeProviderMethod(callCtx, pluginhost.BusinessExtensionMethodInvokeInput{PluginID: row.PluginID, Point: capmodel.BusinessExtensionProductFulfillment, ContractVersion: row.ContractVersion, Method: method, ReleaseVersion: row.ReleaseVersion, ReleaseDigest: row.ReleaseDigest, DeclarationDigest: row.DeclarationDigest, Purpose: capmodel.BusinessExtensionInvocationPurposeExistingBusiness, RuntimeContext: outboundContext, Payload: raw})
			return invokeErr
		})
	})
	if err != nil && !crossed {
		return nil, renewalownercap.ErrOwnerUnavailable
	}

	// Crossing the provider boundary without an authenticated result must never
	// allow CreateTask to be called again as if the write had not happened.
	var task renewalownercap.Task
	if err != nil || result == nil || result.Version != capmodel.BusinessExtensionInvocationProtocolV1 || json.Unmarshal(result.Payload, &task) != nil || renewalownercap.ValidateTask(task) != nil || task.TaskNo != input.OperationNo {
		sum := sha256.Sum256(raw)
		return &renewalownercap.Task{TaskNo: input.OperationNo, Status: renewalownercap.TaskStatusResultUnknown, FailureCode: "peer_renewal_result_unknown", EvidenceDigest: hex.EncodeToString(sum[:]), ObservedAt: time.Now().UTC()}, nil
	}
	return &task, nil
}

func (s *source) load(ctx context.Context, input renewalownercap.TaskRequest) (*frozen, error) {
	var rows []frozen
	const query = `SELECT e.entitlement_no,o.operation_no,o.price_snapshot_no,o.resource_owner_code AS owner_id,
 e.plugin_id,e.plugin_release_version AS release_version,e.contract_version,e.declaration_digest,i.release_digest,
 c.read_evidence,r.resource_owner_configuration_revision AS resource_configuration_revision,i.renewal_method,e.upstream_order_reference AS external_reference,t.external_reference AS task_external_reference,e.upstream_service_no,e.interface_instance_no,i.interface_configuration_revision,
 e.source_intent_no,e.order_no,e.order_item_no,e.configuration_snapshot_no,o.expected_subject_version AS expected_version,
 p.original_expires_at,p.projected_expires_at,p.purchase_cycles,p.period_count,p.period_unit_code,p.currency_code
 FROM plugin_idc_core_business_operation o
 JOIN plugin_idc_core_service_entitlement e ON e.tenant_id=o.tenant_id AND e.entitlement_no=o.subject_no AND e.customer_no=o.customer_no
	JOIN plugin_idc_core_service_price_snapshot p ON p.tenant_id=o.tenant_id AND p.price_snapshot_no=o.price_snapshot_no AND p.subject_no=e.entitlement_no AND p.customer_no=e.customer_no
 JOIN plugin_idc_core_product_fulfillment_intent i ON i.tenant_id=e.tenant_id AND i.intent_no=e.source_intent_no AND i.order_no=e.order_no AND i.order_item_no=e.order_item_no AND i.unit_sequence=e.unit_sequence
 JOIN plugin_idc_core_catalog_import_contract c ON c.tenant_id=e.tenant_id AND c.version_no=e.version_no AND c.interface_instance_no=e.interface_instance_no
 JOIN plugin_idc_core_product_automation_interface_configuration_revision r ON r.tenant_id=e.tenant_id AND r.interface_instance_no=e.interface_instance_no AND r.revision=i.interface_configuration_revision
 JOIN plugin_idc_core_product_fulfillment_task t ON t.tenant_id=i.tenant_id AND t.intent_no=i.intent_no
 WHERE o.tenant_id=0 AND o.operation_no=? AND o.operation_type_code='renewal' AND o.execution_mode_code='provider_confirmed'
 AND o.status_code IN('ready','executing') AND e.status_code IN('active','expired') AND e.renewal_mode_code='provider_confirmed'
 AND t.status_code='succeeded' AND e.upstream_service_no<>'' AND e.version=o.expected_subject_version LIMIT 2`
	if err := s.database.GetScan(ctx, &rows, query, input.OperationNo); err != nil {
		return nil, renewalownercap.ErrOwnerUnavailable
	}
	if len(rows) != 1 {
		return nil, renewalownercap.ErrTaskConflict
	}
	row := &rows[0]
	if row.EntitlementNo != input.EntitlementNo || row.OperationNo != input.OperationNo || row.PriceSnapshotNo != input.PriceSnapshotNo || row.OwnerID != input.OwnerID || row.PluginID != input.PluginID || row.ReleaseVersion != input.PluginReleaseVersion || row.ContractVersion != input.ContractVersion || row.DeclarationDigest != input.DeclarationDigest || row.RenewalMethod != input.RenewalMethod || row.ExpectedVersion != input.ExpectedEntitlementVersion || !row.OriginalExpiresAt.Equal(input.OriginalExpiresAt) || !row.ProjectedExpiresAt.Equal(input.ProjectedExpiresAt) || row.ExternalReference == "" || row.ExternalReference != row.TaskExternalReference || row.InterfaceConfigurationRevision < 1 {
		return nil, renewalownercap.ErrTaskConflict
	}
	return row, nil
}
