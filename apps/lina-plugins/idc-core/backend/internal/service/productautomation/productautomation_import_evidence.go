// This file admits a portable customer declaration only from a trusted,
// version-bound catalog read. Provider identity and release evidence remain
// exact; imported schemas never rewrite the provider's own declaration.

package productautomation

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
	"lina-core/pkg/plugin/capability/capmodel"
	"lina-plugin-idc-core/backend/cap/channelcommercecap"
	"lina-plugin-idc-core/backend/cap/productfulfillmentcap"
	"lina-plugin-idc-core/backend/cap/resourceinterfaceprojectioncap"
	"lina-plugin-idc-core/backend/internal/model/do"
	"lina-plugin-idc-core/backend/internal/model/entity"
	"lina-plugin-idc-core/backend/internal/service/catalog"
	"lina-plugin-idc-core/backend/internal/service/configurationcontract"
)

const portableImportProviderID = "idc-peer"

func (s *serviceImpl) insertImportedFulfillmentPolicy(ctx context.Context, versionNo string, actor int) error {
	contract, err := s.loadFrozenFulfillmentContract(ctx, versionNo)
	if err != nil {
		return err
	}
	policy, err := catalog.BuildFulfillmentPolicyEvidence(catalog.FulfillmentStrategyPaymentConfirmedPlugin, contract)
	if err != nil {
		return err
	}
	result, err := s.database.Exec(ctx, `INSERT INTO plugin_idc_core_catalog_product_version_fulfillment_policy
(tenant_id,version_no,strategy_code,granularity_code,ongoing_management,renewal_mode_code,renewal_method,entitlement_owner_id,provision_method,query_method,maximum_attempts,retry_base_seconds,policy_digest,created_by)
VALUES(0,?,?,?,?,?,?,?,?,?,?,?,?,?)`, versionNo, policy.StrategyCode, policy.GranularityCode, policy.OngoingManagement, policy.RenewalMode, policy.RenewalMethod, policy.EntitlementOwnerID, policy.ProvisionMethod, policy.QueryMethod, policy.MaximumAttempts, policy.RetryBaseSeconds, policy.PolicyDigest, actor)
	if err != nil {
		return gerror.Wrap(err, "freeze imported fulfillment policy")
	}
	affected, err := result.RowsAffected()
	if err != nil || affected != 1 {
		return gerror.New("imported fulfillment policy was not inserted")
	}
	return nil
}

func (s *serviceImpl) applyImportedFulfillmentContract(ctx context.Context, versionNo string, exact *capmodel.BusinessExtensionProviderEvidence, result *catalog.FrozenFulfillmentContract) error {
	if exact == nil || exact.PluginID != portableImportProviderID {
		return nil
	}
	binding, err := s.loadBinding(ctx, versionNo, false)
	if err != nil {
		return err
	}
	configuration, err := s.loadVersionConfigurationRow(ctx, versionNo)
	if err != nil {
		return err
	}
	if _, err = s.importedCustomerSchema(ctx, binding, configuration, exact); err != nil {
		return bizerr.NewCode(catalog.CodeFulfillmentPolicyContractChanged)
	}
	var contract *entity.CatalogImportContract
	if err = s.database.Model(do.CatalogImportContract{}).Ctx(ctx).Where(do.CatalogImportContract{TenantId: 0, VersionNo: versionNo}).Scan(&contract); err != nil || contract == nil {
		return bizerr.NewCode(catalog.CodeFulfillmentPolicyContractChanged)
	}
	var snapshot channelcommercecap.ImportSnapshot
	if json.Unmarshal([]byte(contract.Snapshot), &snapshot) != nil {
		return bizerr.NewCode(catalog.CodeFulfillmentPolicyContractChanged)
	}
	result.Granularity = snapshot.Fulfillment.Granularity
	result.OngoingManagement = snapshot.Fulfillment.OngoingService
	result.RenewalMode = productfulfillmentcap.RenewalModeUnavailable
	result.RenewalMethod = ""
	if !result.OngoingManagement {
		result.EntitlementOwnerID = ""
	}
	if snapshot.Fulfillment.RenewalMode != productfulfillmentcap.RenewalModeUnavailable {
		declaration, declarationErr := productfulfillmentcap.ParseProviderDeclaration(exact.ProviderMetadata, exact.AuthorizedMethods)
		if declarationErr != nil || declaration.MethodRoles.Renewal == "" {
			return bizerr.NewCode(catalog.CodeFulfillmentPolicyContractChanged)
		}
		result.RenewalMode = productfulfillmentcap.RenewalModeProviderConfirmed
		result.RenewalMethod = declaration.MethodRoles.Renewal
	}
	return nil
}

func validatePortableImportEvidence(snapshot *channelcommercecap.ImportSnapshot, read *resourceinterfaceprojectioncap.CatalogReadEvidence, exact *capmodel.BusinessExtensionProviderEvidence, supplied []resourceinterfaceprojectioncap.CatalogConfigurationValue) error {
	if snapshot == nil || snapshot.Validate() != nil || read == nil || read.Validate() != nil || exact == nil || exact.PluginID != portableImportProviderID || read.PluginID != exact.PluginID || read.ReleaseDigest != exact.ReleaseDigest || read.DeclarationDigest != exact.DeclarationDigest {
		return bizerr.NewCode(catalog.CodeManagementConflict)
	}
	values := make(map[string]string, len(supplied))
	for _, value := range supplied {
		if value.TextValue != nil {
			values[value.FieldKey] = *value.TextValue
		}
	}
	if values["upstream_public_slug"] != snapshot.Product.PublicSlug || values["upstream_configuration_revision"] != snapshot.ConfigurationRevision {
		return bizerr.NewCode(catalog.CodeManagementConflict)
	}
	if _, err := configurationcontract.ValidateSchema(snapshot.Schema, configurationSurfaceCustomer, exact.AuthorizedMethods); err != nil {
		return bizerr.NewCode(catalog.CodeManagementInvalid)
	}
	return nil
}

// importedCustomerSchema verifies the immutable portable contract against the
// existing exact binding and interface revision before accepting its schema.
func (s *serviceImpl) importedCustomerSchema(ctx context.Context, binding *entity.CatalogProductVersionFulfillmentBinding, configuration *entity.CatalogProductVersionConfiguration, exact *capmodel.BusinessExtensionProviderEvidence) (*capmodel.ConfigurationSchemaDeclaration, error) {
	if binding == nil || configuration == nil || exact == nil {
		return nil, gerror.New("import configuration evidence is unavailable")
	}
	if exact.PluginID != portableImportProviderID {
		return verifiedCustomerSchema(exact, configuration.CustomerDeclarationSnapshot)
	}
	var contract *entity.CatalogImportContract
	if err := s.database.Model(do.CatalogImportContract{}).Ctx(ctx).Where(do.CatalogImportContract{TenantId: 0, VersionNo: binding.VersionNo, InterfaceInstanceNo: binding.InterfaceInstanceNo}).Scan(&contract); err != nil {
		return nil, err
	}
	if contract == nil {
		return exact.CustomerConfiguration, nil
	}
	var revision *entity.ProductAutomationInterfaceConfigurationRevision
	if err := s.database.Model(do.ProductAutomationInterfaceConfigurationRevision{}).Ctx(ctx).Where(do.ProductAutomationInterfaceConfigurationRevision{TenantId: 0, InterfaceInstanceNo: binding.InterfaceInstanceNo, Revision: configuration.InterfaceConfigurationRevision}).Scan(&revision); err != nil {
		return nil, err
	}
	return verifiedPortableCustomerSchema(contract, binding, revision, exact)
}

func verifiedPortableCustomerSchema(contract *entity.CatalogImportContract, binding *entity.CatalogProductVersionFulfillmentBinding, revision *entity.ProductAutomationInterfaceConfigurationRevision, exact *capmodel.BusinessExtensionProviderEvidence) (*capmodel.ConfigurationSchemaDeclaration, error) {
	if contract == nil || binding == nil || revision == nil || exact == nil || exact.PluginID != portableImportProviderID || !sameExactEvidence(bindingEvidence(binding), exact) || contract.VersionNo != binding.VersionNo || contract.InterfaceInstanceNo != binding.InterfaceInstanceNo || revision.InterfaceInstanceNo != binding.InterfaceInstanceNo {
		return nil, gerror.New("portable import binding evidence does not match")
	}
	var snapshot channelcommercecap.ImportSnapshot
	if err := json.Unmarshal([]byte(contract.Snapshot), &snapshot); err != nil || snapshot.Validate() != nil {
		return nil, gerror.New("portable import snapshot is invalid")
	}
	encoded, err := json.Marshal(snapshot)
	if err != nil {
		return nil, err
	}
	digest := sha256.Sum256(encoded)
	if hex.EncodeToString(digest[:]) != contract.SnapshotDigest {
		return nil, gerror.New("portable import snapshot digest mismatch")
	}
	var read resourceinterfaceprojectioncap.CatalogReadEvidence
	if err = json.Unmarshal([]byte(contract.ReadEvidence), &read); err != nil || read.Validate() != nil || read.InterfaceInstanceNo != binding.InterfaceInstanceNo || read.InterfaceConfigurationRevision != revision.ResourceOwnerConfigurationRevision || read.PluginID != exact.PluginID || read.PluginReleaseVersion != exact.ReleaseVersion || read.ContractVersion != exact.ContractVersion || read.ReleaseDigest != exact.ReleaseDigest || read.DeclarationDigest != exact.DeclarationDigest || !sameImportMethods(read.AuthorizedMethods, exact.AuthorizedMethods) {
		return nil, gerror.New("portable import read evidence mismatch")
	}
	if _, err = configurationcontract.ValidateSchema(snapshot.Schema, configurationSurfaceCustomer, exact.AuthorizedMethods); err != nil {
		return nil, err
	}
	return snapshot.Schema, nil
}
