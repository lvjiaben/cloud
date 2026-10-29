// This file explicitly refreshes an unused supplier interface to the enabled
// connector release, preserving credentials, address, certificate trust and status.
package providerconfiguration

import (
	"context"
	"encoding/json"
	"github.com/gogf/gf/v2/database/gdb"
	"lina-core/pkg/bizerr"
	"lina-core/pkg/plugin/capability/capmodel"
	"lina-core/pkg/plugin/capability/plugincap"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/cap/resourceinterfaceprojectioncap"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
	"lina-plugin-idc-resource/backend/internal/service/interfaceprojection"
	"strings"
)

func (s *service) refreshChannelProvider(ctx context.Context, actor int, before *Channel, expectedVersion int64) (*Channel, error) {
	if before.Version != expectedVersion || s.extensions == nil || s.projection == nil || before.Status == string(deletedConfigurationStatus) {
		return nil, bizerr.NewCode(CodeConflict)
	}
	selected, err := s.currentProviderEvidence(ctx, before.ProviderPluginID)
	if err != nil {
		return nil, err
	}
	if selected.ReleaseVersion == before.ProviderReleaseVersion && selected.ReleaseDigest == before.ProviderReleaseDigest && selected.DeclarationDigest == before.ProviderDeclarationDigest {
		return nil, bizerr.NewCode(CodeConflict)
	}
	product, err := s.productFulfillmentEvidence(ctx, selected)
	if err != nil {
		return nil, err
	}
	if product == nil {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	settings, err := compileRefreshSettings(before, selected)
	if err != nil {
		return nil, bizerr.WrapCode(err, CodeInvalid)
	}
	revision, err := configurationRevisionNo()
	if err != nil {
		return nil, bizerr.WrapCode(err, CodeUnavailable)
	}
	projection := resourceinterfaceprojectioncap.Projection{
		InterfaceInstanceNo: before.InterfaceInstanceNo, DisplayName: before.DisplayName, ActorUserID: actor,
		OwnerID: resourceinterfaceprojectioncap.OwnerID, ResourceChannelRef: before.ChannelNo,
		PluginID: product.PluginID, PluginReleaseVersion: product.ReleaseVersion, ContractVersion: product.ContractVersion,
		AuthorizedMethods: append([]string(nil), product.AuthorizedMethods...), ReleaseDigest: product.ReleaseDigest, DeclarationDigest: product.DeclarationDigest,
		NewBusinessState: projectionState(before.Status), InterfaceConfigurationRevision: revision,
	}
	currentRevision, err := s.db.GetValue(ctx, `SELECT current_configuration_revision FROM plugin_idc_resource_interface_channel_binding WHERE tenant_id=? AND channel_no=?`, idccap.DefaultTenantID, before.ChannelNo)
	if err != nil {
		return nil, bizerr.WrapCode(err, CodeUnavailable)
	}
	err = s.withDynamicAdmissions(ctx, selected, product, func(admissionCtx context.Context) error {
		return s.projection.RefreshUnused(admissionCtx, currentRevision.String(), projection, func(coreTxCtx context.Context) error {
			return s.db.Transaction(coreTxCtx, func(txCtx context.Context, tx gdb.TX) error {
				var current struct{ Version int64 }
				if err := tx.GetScan(&current, `SELECT version FROM plugin_idc_resource_channel WHERE tenant_id=? AND channel_no=? AND deleted_at IS NULL FOR UPDATE`, idccap.DefaultTenantID, before.ChannelNo); err != nil {
					return err
				}
				if current.Version != expectedVersion {
					return bizerr.NewCode(CodeConflict)
				}
				pending, pendingErr := tx.GetValue(`SELECT EXISTS(SELECT 1 FROM plugin_idc_resource_interface_projection_outbox WHERE tenant_id=? AND interface_instance_no=? AND status_code<>'delivered')`, idccap.DefaultTenantID, before.InterfaceInstanceNo)
				if pendingErr != nil {
					return pendingErr
				}
				if pending.Bool() {
					return bizerr.NewCode(CodeConflict)
				}
				used, err := tx.GetValue(`SELECT
 EXISTS(SELECT 1 FROM plugin_idc_resource_task WHERE tenant_id=? AND channel_no=?) OR
 EXISTS(SELECT 1 FROM plugin_idc_resource_mapping WHERE tenant_id=? AND channel_no=?) OR
 EXISTS(SELECT 1 FROM plugin_idc_resource_principal_task WHERE tenant_id=? AND channel_no=?) OR
 EXISTS(SELECT 1 FROM plugin_idc_resource_upstream_task_binding WHERE tenant_id=? AND channel_no=?)`,
					idccap.DefaultTenantID, before.ChannelNo, idccap.DefaultTenantID, before.ChannelNo, idccap.DefaultTenantID, before.ChannelNo, idccap.DefaultTenantID, before.ChannelNo)
				if err != nil {
					return err
				}
				if used.Bool() {
					return bizerr.NewCode(CodeRefreshReferenced)
				}
				if err = ensureRequiredChannelSecrets(txCtx, tx, ChannelInput{ChannelNo: before.ChannelNo, Status: before.Status}, selected.InterfaceConfiguration); err != nil {
					return err
				}
				if _, err = tx.Exec(`UPDATE plugin_idc_resource_channel SET provider_contract_version=?,provider_release_version=?,provider_release_digest=?,provider_declaration_digest=?,non_secret_settings=CAST(? AS jsonb),version=version+1,updated_by=?,updated_at=CURRENT_TIMESTAMP WHERE tenant_id=? AND channel_no=? AND version=?`, selected.ContractVersion, selected.ReleaseVersion, selected.ReleaseDigest, selected.DeclarationDigest, string(settings), actor, idccap.DefaultTenantID, before.ChannelNo, expectedVersion); err != nil {
					return err
				}
				if _, err = tx.Exec(`UPDATE plugin_idc_resource_interface_channel_binding SET resource_plugin_release_version=?,resource_contract_version=?,resource_authorized_methods=string_to_array(?,','),resource_release_digest=?,resource_declaration_digest=?,product_plugin_id=?,product_plugin_release_version=?,product_contract_version=?,product_authorized_methods=string_to_array(?,','),product_release_digest=?,product_declaration_digest=?,current_configuration_revision=?,binding_version=binding_version+1,updated_at=CURRENT_TIMESTAMP WHERE tenant_id=? AND channel_no=?`, selected.ReleaseVersion, selected.ContractVersion, strings.Join(selected.AuthorizedMethods, ","), selected.ReleaseDigest, selected.DeclarationDigest, product.PluginID, product.ReleaseVersion, product.ContractVersion, strings.Join(product.AuthorizedMethods, ","), product.ReleaseDigest, product.DeclarationDigest, revision, idccap.DefaultTenantID, before.ChannelNo); err != nil {
					return err
				}
				if err = insertConfigurationRevision(txCtx, tx, revision, before.InterfaceInstanceNo, before.ChannelNo, before.Endpoint, settings, actor); err != nil {
					return err
				}
				sequence, sequenceErr := interfaceprojection.NextSequence(txCtx, tx, projection.InterfaceInstanceNo)
				if sequenceErr != nil {
					return sequenceErr
				}
				return interfaceprojection.Enqueue(txCtx, tx, projection, sequence)
			})
		})
	})
	if err != nil {
		if bizerr.Is(err, CodeConflict) || bizerr.Is(err, CodeRefreshReferenced) {
			return nil, err
		}
		if bizerr.Is(err, resourceinterfaceprojectioncap.CodeRefreshReferenced) {
			return nil, bizerr.NewCode(CodeRefreshReferenced)
		}
		return nil, bizerr.WrapCode(err, CodeUnavailable)
	}
	return s.channel(ctx, before.ChannelNo)
}

func (s *service) currentProviderEvidence(ctx context.Context, pluginID string) (*capmodel.BusinessExtensionProviderEvidence, error) {
	page, err := s.extensions.ListProviders(ctx, plugincap.BusinessExtensionProviderListInput{Point: resourcecap.DynamicProviderPoint, ContractVersion: resourcecap.DynamicProviderContractVersion, Page: capmodel.PageRequest{PageNum: 1, PageSize: 100, Limit: 100}})
	if err != nil || page == nil {
		return nil, bizerr.WrapCode(err, CodeUnavailable)
	}
	var selected *capmodel.BusinessExtensionProviderEvidence
	for _, candidate := range page.Items {
		if candidate != nil && candidate.PluginID == pluginID {
			if selected != nil {
				return nil, bizerr.NewCode(CodeInvalid)
			}
			selected = candidate
		}
	}
	if selected == nil || resourcecap.ValidateProviderMetadata(selected.ProviderMetadata, selected.InterfaceConfiguration) != nil {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	return selected, nil
}

func compileRefreshSettings(before *Channel, evidence *capmodel.BusinessExtensionProviderEvidence) (json.RawMessage, error) {
	values := map[string]json.RawMessage{}
	if json.Unmarshal(before.NonSecretSettings, &values) != nil || values == nil {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	if interfaceConfigurationDeclares(evidence.InterfaceConfiguration, "endpoint") {
		value, err := json.Marshal(before.Endpoint)
		if err != nil {
			return nil, err
		}
		values["endpoint"] = value
	}
	encoded, err := json.Marshal(values)
	if err != nil {
		return nil, err
	}
	normalized, err := normalizeNonSecretSettings(encoded, evidence.InterfaceConfiguration)
	if err != nil {
		return nil, err
	}
	return resourcecap.CompileInterfaceSettings(normalized, evidence.ProviderMetadata)
}
