// Native supplier pricing is an owner-to-owner read admitted by the same exact
// published binding as purchase. Browser input never chooses methods or accounts.
package productautomation

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"

	"github.com/gogf/gf/v2/errors/gerror"
	"lina-core/pkg/plugin/capability/plugincap"
	"lina-plugin-idc-core/backend/cap/channelcommercecap"
	"lina-plugin-idc-core/backend/cap/resourceinterfaceprojectioncap"
	"lina-plugin-idc-core/backend/internal/service/catalog"
	"lina-plugin-idc-core/backend/internal/service/configurationcontract"
)

// ReadImportedPurchaseBillingTerms authorizes a bounded batch with one owner
// projection and one provider lookup, independent of cart row count.
func (s *serviceImpl) ReadImportedPurchaseBillingTerms(ctx context.Context, inputs []catalog.ImportedBillingInput) ([]*channelcommercecap.BillingTerms, error) {
	if len(inputs) == 0 || len(inputs) > 100 {
		return nil, gerror.New("imported billing batch invalid")
	}
	source, ok := s.resourceCandidates.(resourceinterfaceprojectioncap.ImportedBillingSource)
	if !ok {
		return nil, gerror.New("imported billing source unavailable")
	}
	versions := make([]string, 0, len(inputs))
	seen := map[string]bool{}
	for _, input := range inputs {
		if validateBusinessNo(input.VersionNo, "ver") != nil {
			return nil, gerror.New("imported billing version invalid")
		}
		if !seen[input.VersionNo] {
			versions = append(versions, input.VersionNo)
			seen[input.VersionNo] = true
		}
	}
	versionJSON, err := json.Marshal(versions)
	if err != nil {
		return nil, err
	}
	var rows []importedBillingRow
	if err := s.database.GetScan(ctx, &rows, importedBillingEvidenceSQL, string(versionJSON), len(versions)+1); err != nil {
		return nil, err
	}
	if len(rows) != len(versions) {
		return nil, gerror.New("imported billing binding unavailable")
	}
	providers := make([]plugincap.BusinessExtensionProviderExactInput, 0, len(rows))
	decoded := map[string]importedBillingEvidence{}
	for _, row := range rows {
		evidence, err := decodeImportedBillingEvidence(row)
		if err != nil {
			return nil, err
		}
		decoded[row.VersionNo] = evidence
		providers = append(providers, plugincap.BusinessExtensionProviderExactInput{PluginID: row.PluginID, ReleaseVersion: row.ReleaseVersion, Point: row.Point, ContractVersion: row.ContractVersion, ReleaseDigest: row.ReleaseDigest, DeclarationDigest: row.DeclarationDigest})
	}
	exact, err := s.extensions.GetProvidersExact(ctx, providers)
	if err != nil || len(exact) != len(rows) {
		return nil, gerror.New("imported billing provider unavailable")
	}
	for index, provider := range exact {
		frozen := decoded[rows[index].VersionNo]
		if !sameImportReadProvider(&frozen.read, provider) {
			return nil, gerror.New("imported billing provider changed")
		}
	}
	results := make([]*channelcommercecap.BillingTerms, len(inputs))
	cache := map[string]*channelcommercecap.BillingTerms{}
	for index, input := range inputs {
		frozen := decoded[input.VersionNo]
		mapped, err := catalog.MapImportedBillingValues(&frozen.snapshot, input.Values)
		if err != nil {
			return nil, err
		}
		request := resourceinterfaceprojectioncap.ImportedBillingRequest{Evidence: frozen.read, Input: channelcommercecap.ConfigurationInput{PublicSlug: frozen.snapshot.Product.PublicSlug, ConfigurationRevision: frozen.snapshot.ConfigurationRevision, CurrencyCode: input.CurrencyCode, Values: mapped}}
		raw, err := json.Marshal(request)
		if err != nil {
			return nil, err
		}
		key := string(raw)
		result := cache[key]
		if result == nil {
			result, err = source.ReadImportedBillingTerms(ctx, request)
			if err != nil {
				return nil, err
			}
		}
		if result == nil || result.Validate() != nil || result.ConfigurationRevision != frozen.snapshot.ConfigurationRevision || result.CurrencyCode != input.CurrencyCode {
			return nil, gerror.New("imported billing result invalid")
		}
		cache[key] = result
		results[index] = result
	}
	return results, nil
}

type importedBillingEvidence struct {
	snapshot channelcommercecap.ImportSnapshot
	read     resourceinterfaceprojectioncap.CatalogReadEvidence
}
type importedBillingRow struct{ VersionNo, Snapshot, SnapshotDigest, ReadEvidence, CustomerDeclaration, InterfaceNo, OwnerRevision, PluginID, ReleaseVersion, Point, ContractVersion, ReleaseDigest, DeclarationDigest string }

const importedBillingEvidenceSQL = `SELECT binding.version_no,contract.snapshot,contract.snapshot_digest,contract.read_evidence,
configuration.customer_declaration_snapshot AS customer_declaration,
binding.interface_instance_no AS interface_no,revision.resource_owner_configuration_revision AS owner_revision,
binding.plugin_id,binding.plugin_release_version AS release_version,binding.extension_point_code AS point,
binding.contract_version,binding.release_digest,binding.declaration_digest
FROM plugin_idc_core_catalog_product_version_fulfillment_binding binding
JOIN plugin_idc_core_catalog_import_contract contract ON contract.tenant_id=binding.tenant_id AND contract.version_no=binding.version_no AND contract.interface_instance_no=binding.interface_instance_no
JOIN plugin_idc_core_catalog_product_version version ON version.tenant_id=binding.tenant_id AND version.version_no=binding.version_no AND version.status_code='published'
JOIN plugin_idc_core_catalog_product product ON product.tenant_id=version.tenant_id AND product.current_version_no=version.version_no AND product.deleted_at IS NULL AND product.status_code='active'
JOIN plugin_idc_core_product_automation_interface_instance interface ON interface.tenant_id=binding.tenant_id AND interface.interface_instance_no=binding.interface_instance_no AND interface.new_business_state='accepting' AND interface.interface_origin_code='resource'
JOIN plugin_idc_core_catalog_product_version_configuration configuration ON configuration.tenant_id=binding.tenant_id AND configuration.version_no=binding.version_no AND configuration.interface_instance_no=binding.interface_instance_no
JOIN plugin_idc_core_product_automation_interface_configuration_revision revision ON revision.tenant_id=configuration.tenant_id AND revision.interface_instance_no=configuration.interface_instance_no AND revision.revision=configuration.interface_configuration_revision
WHERE binding.tenant_id=0 AND binding.version_no=ANY(ARRAY(SELECT jsonb_array_elements_text(CAST(? AS jsonb)))) ORDER BY binding.version_no LIMIT ?`

func (i *RuntimeCapabilityInvoker) ReadImportedPurchaseBillingTerms(ctx context.Context, inputs []catalog.ImportedBillingInput) ([]*channelcommercecap.BillingTerms, error) {
	if i == nil {
		return nil, gerror.New("supplier runtime capability unavailable")
	}
	i.mu.RLock()
	service := i.service
	i.mu.RUnlock()
	if service == nil {
		return nil, gerror.New("supplier runtime capability unavailable")
	}
	return service.ReadImportedPurchaseBillingTerms(ctx, inputs)
}

func decodeImportedBillingEvidence(row importedBillingRow) (importedBillingEvidence, error) {
	var snapshot channelcommercecap.ImportSnapshot
	var read resourceinterfaceprojectioncap.CatalogReadEvidence
	if json.Unmarshal([]byte(row.Snapshot), &snapshot) != nil || snapshot.Validate() != nil || json.Unmarshal([]byte(row.ReadEvidence), &read) != nil || read.Validate() != nil || !read.SaleImportProtocol.NativePurchase() || read.InterfaceInstanceNo != row.InterfaceNo || read.InterfaceConfigurationRevision != row.OwnerRevision {
		return importedBillingEvidence{}, gerror.New("imported billing evidence invalid")
	}
	raw, err := json.Marshal(snapshot)
	if err != nil {
		return importedBillingEvidence{}, err
	}
	digest := sha256.Sum256(raw)
	if hex.EncodeToString(digest[:]) != row.SnapshotDigest || row.ReleaseDigest != read.ReleaseDigest || row.DeclarationDigest != read.DeclarationDigest || row.PluginID != read.PluginID || row.ReleaseVersion != read.PluginReleaseVersion || row.ContractVersion != read.ContractVersion {
		return importedBillingEvidence{}, gerror.New("imported billing evidence changed")
	}
	schema, err := configurationcontract.DecodeDeclaration(row.CustomerDeclaration)
	if err != nil || !sameExactConfiguration(schema, snapshot.Schema) {
		return importedBillingEvidence{}, gerror.New("imported billing schema changed")
	}
	return importedBillingEvidence{snapshot: snapshot, read: read}, nil
}
