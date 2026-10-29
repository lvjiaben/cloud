// This file authorizes operational correction inspection and reconstructs the
// original paid resource request without changing order or configuration facts.
package productfulfillment

import (
	"context"
	"encoding/json"
	"github.com/gogf/gf/v2/database/gdb"
	"time"

	"lina-plugin-idc-core/backend/cap/provisionresourcecap"
)

// InspectRecoveryConfiguration requires the independent recovery permission and
// current customer scope before any configuration or upstream choice is read.
func (s *serviceImpl) InspectRecoveryConfiguration(ctx context.Context, intentNo string, selection *provisionresourcecap.RecoveryConfiguration) (*provisionresourcecap.RecoveryEditor, error) {
	if s.authorizer == nil || s.resourceSource == nil {
		return nil, errorsNew("product_fulfillment_management_unavailable")
	}
	authorization, err := s.authorizer.AuthorizeRecovery(ctx, intentNo)
	if err != nil {
		return nil, err
	}
	if authorization.ActorUserID <= 0 || authorization.RequiredPermission != PermissionPreparationRecover || !validAuthorizationScope(authorization) {
		return nil, errorsNew("product_fulfillment_not_accessible")
	}
	current, err := s.loadDetailAuthorized(ctx, intentNo, false, authorization)
	if err != nil {
		return nil, err
	}
	if !current.CanRecoverPreparation {
		return nil, errorsNew("product_fulfillment_action_conflict")
	}
	request, err := s.loadPaidResourceRequest(ctx, intentNo)
	if err != nil {
		return nil, err
	}
	if selection == nil {
		selection, err = loadRecoveryConfiguration(ctx, s.database, intentNo)
		if err != nil {
			return nil, err
		}
	}
	editor, err := s.resourceSource.InspectRecovery(ctx, provisionresourcecap.RecoverySelectionRequest{Request: request, Selection: selection})
	if err != nil {
		return nil, errorsNew("product_fulfillment_configuration_invalid")
	}
	return editor, nil
}

// loadPaidResourceRequest is bounded to the previously authorized intent. Values
// are reconstructed from original immutable snapshots, never current products.
func (s *serviceImpl) loadPaidResourceRequest(ctx context.Context, intentNo string) (provisionresourcecap.TaskRequest, error) {
	var row *claimedFulfillment
	err := s.database.GetScan(ctx, &row, `SELECT intent.intent_no,intent.order_no,intent.order_item_no,intent.product_no,intent.version_no,
 intent.customer_no,intent.configuration_snapshot_no,intent.interface_instance_no,intent.unit_sequence,intent.quantity,
 snapshot.configuration_revision,snapshot.typed_value_snapshot AS customer_configuration,
 configuration.customer_declaration_snapshot::text AS customer_declaration,
 configuration.customer_presentation_snapshot::text AS customer_presentation,
 configuration.product_declaration_snapshot::text AS product_declaration,
 configuration.merchant_value_snapshot AS product_configuration,configuration.merchant_value_digest AS product_configuration_digest,
 interface_configuration.resource_owner_configuration_revision AS resource_interface_configuration_revision
 FROM plugin_idc_core_product_fulfillment_intent intent
 JOIN plugin_idc_core_commerce_structured_configuration_snapshot snapshot ON snapshot.tenant_id=intent.tenant_id AND snapshot.snapshot_no=intent.configuration_snapshot_no AND snapshot.order_item_no=intent.order_item_no AND snapshot.order_no=intent.order_no
 JOIN plugin_idc_core_catalog_product_version_configuration configuration ON configuration.tenant_id=intent.tenant_id AND configuration.version_no=intent.version_no AND configuration.interface_instance_no=intent.interface_instance_no
 JOIN plugin_idc_core_product_automation_interface_configuration_revision interface_configuration ON interface_configuration.tenant_id=intent.tenant_id AND interface_configuration.interface_instance_no=intent.interface_instance_no AND interface_configuration.revision=intent.interface_configuration_revision
 WHERE intent.tenant_id=0 AND intent.intent_no=? AND intent.entitlement_owner_id='idc-resource' LIMIT 1`, intentNo)
	if err != nil || row == nil {
		return provisionresourcecap.TaskRequest{}, errorsNew("product_fulfillment_configuration_invalid")
	}
	customer, err := safeProviderCustomerConfiguration(row.CustomerDeclaration, row.CustomerConfiguration)
	if err != nil {
		return provisionresourcecap.TaskRequest{}, errorsNew("product_fulfillment_configuration_invalid")
	}
	productConfiguration, productDigest, err := row.boundProductConfiguration()
	if err != nil {
		return provisionresourcecap.TaskRequest{}, errorsNew("product_fulfillment_configuration_invalid")
	}
	request := provisionresourcecap.TaskRequest{OperationNo: row.IntentNo, CustomerNo: row.CustomerNo, ConfigurationSnapshotNo: row.ConfigurationSnapshotNo,
		InterfaceInstanceNo: row.InterfaceInstanceNo, InterfaceConfigurationRevision: row.ResourceInterfaceConfigurationRevision,
		ProductConfiguration: productConfiguration, ProductConfigurationDigest: productDigest, CustomerConfiguration: customer,
		Fulfillment: provisionresourcecap.FrozenFulfillmentIdentity{IntentNo: row.IntentNo, OrderNo: row.OrderNo, OrderItemNo: row.OrderItemNo, ProductNo: row.ProductNo, ProductVersionNo: row.VersionNo, ConfigurationRevision: row.ConfigurationRevision, UnitSequence: row.UnitSequence, Quantity: row.Quantity}}
	request.RequestDigest, err = provisionresourcecap.TaskRequestDigest(request)
	if err != nil {
		return request, errorsNew("product_fulfillment_configuration_invalid")
	}
	return request, nil
}

// validateRecoveryCorrection runs only read-only preflight outside funding and
// task locks. An already committed immutable command is replayed before remote
// choices are queried, preserving retries even if those choices later change.
func (s *serviceImpl) validateRecoveryCorrection(ctx context.Context, input ActionInput, authorization Authorization, digest string) (*IntentDetail, error) {
	if input.RecoveryConfiguration == nil {
		return nil, nil
	}
	current, err := s.loadDetailAuthorized(ctx, input.IntentNo, false, authorization)
	if err != nil {
		return nil, err
	}
	replay, err := s.commandReplay(ctx, string(actionRecoverPreparation), input, digest)
	if err != nil {
		return nil, err
	}
	if replay {
		return current, nil
	}
	if current.Version != input.ExpectedVersion || !current.CanRecoverPreparation || s.resourceSource == nil {
		return nil, errorsNew("product_fulfillment_action_conflict")
	}
	request, err := s.loadPaidResourceRequest(ctx, input.IntentNo)
	if err != nil {
		return nil, err
	}
	validated, err := s.resourceSource.ValidateRecovery(ctx, provisionresourcecap.RecoverySelectionRequest{Request: request, Selection: input.RecoveryConfiguration})
	if err != nil || validated == nil || *validated != *input.RecoveryConfiguration {
		return nil, errorsNew("product_fulfillment_configuration_invalid")
	}
	return nil, nil
}

// loadRecoveryConfiguration selects the last immutable staff command, including
// an explicit null meaning retain original configuration in the next round.
func loadRecoveryConfiguration(ctx context.Context, database gdb.DB, intentNo string) (*provisionresourcecap.RecoveryConfiguration, error) {
	value, err := database.GetValue(ctx, `SELECT recovery_configuration::text FROM plugin_idc_core_product_fulfillment_command
 WHERE tenant_id=0 AND intent_no=? AND action_code='recover_preparation'
 ORDER BY resulting_version DESC LIMIT 1`, intentNo)
	if err != nil {
		return nil, err
	}
	if value.IsEmpty() || value.String() == "null" {
		return nil, nil
	}
	var correction provisionresourcecap.RecoveryConfiguration
	if json.Unmarshal(value.Bytes(), &correction) != nil {
		return nil, errorsNew("product_fulfillment_configuration_invalid")
	}
	return &correction, nil
}

// loadRecoveryConfigurationHistory reads at most 16 local immutable selections
// for a previously authorized detail, with no supplier calls or row waterfall.
func (s *serviceImpl) loadRecoveryConfigurationHistory(ctx context.Context, intentNo string) ([]RecoveryConfigurationRecord, error) {
	var rows []struct {
		Configuration string
		OccurredAt    time.Time
	}
	err := s.database.GetScan(ctx, &rows, `SELECT recovery_configuration::text AS configuration,created_at AS occurred_at
 FROM plugin_idc_core_product_fulfillment_command WHERE tenant_id=0 AND intent_no=?
 AND action_code='recover_preparation' AND recovery_configuration IS NOT NULL AND recovery_configuration<>'null'::jsonb
 ORDER BY resulting_version DESC LIMIT 16`, intentNo)
	if err != nil {
		return nil, errorsNew("product_fulfillment_management_unavailable")
	}
	records := make([]RecoveryConfigurationRecord, 0, len(rows))
	for _, row := range rows {
		var configuration provisionresourcecap.RecoveryConfiguration
		if json.Unmarshal([]byte(row.Configuration), &configuration) != nil {
			return nil, errorsNew("product_fulfillment_management_unavailable")
		}
		records = append(records, RecoveryConfigurationRecord{Configuration: configuration, OccurredAt: row.OccurredAt})
	}
	return records, nil
}

// validateRecoveryOwnerRelease retains original contract/inventory gates but
// applies new-business admission to the explicitly selected upstream interface.
func (s *serviceImpl) validateRecoveryOwnerRelease(ctx context.Context, current *IntentDetail, correction *provisionresourcecap.RecoveryConfiguration) error {
	if correction == nil {
		return s.validateOwnerRelease(ctx, current.OrderNo, current.OrderItemNo)
	}
	ready, err := evaluateOwnerGates(ctx, s.gates, current.OrderNo, []string{current.OrderItemNo})
	if err != nil || !ready[current.OrderItemNo] {
		return errorsNew("product_fulfillment_owner_gate_blocked")
	}
	state, err := s.database.GetValue(ctx, `SELECT new_business_state FROM plugin_idc_core_product_automation_interface_instance
 WHERE tenant_id=0 AND interface_instance_no=? AND resource_owner_id='idc-resource' LIMIT 1`, correction.InterfaceInstanceNo)
	if err != nil || state.String() != "accepting" {
		return errorsNew("product_fulfillment_new_business_stopped")
	}
	return nil
}
