package productautomation

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/plugin/capability/capmodel"
	"lina-core/pkg/plugin/capability/plugincap"
	"lina-plugin-idc-core/backend/cap/resourceinterfaceprojectioncap"
	"lina-plugin-idc-core/backend/internal/model/do"
	"lina-plugin-idc-core/backend/internal/model/entity"
	"lina-plugin-idc-core/backend/internal/service/configurationcontract"
	"lina-plugin-idc-core/backend/internal/service/purchaseconfigurationfile"
)

const maximumFrozenConfigurationBatch = 100

type frozenValidationBatchRow struct {
	InterfaceOrigin                string   `orm:"interface_origin_code"`
	ResourceOwnerID                string   `orm:"resource_owner_id"`
	VersionNo                      string   `orm:"version_no"`
	BindingInterfaceInstanceNo     string   `orm:"binding_interface_instance_no"`
	InterfaceDisplayNameSnapshot   string   `orm:"interface_display_name_snapshot"`
	PluginID                       string   `orm:"plugin_id"`
	ExtensionPointCode             string   `orm:"extension_point_code"`
	PluginReleaseVersion           string   `orm:"plugin_release_version"`
	ContractVersion                string   `orm:"contract_version"`
	AuthorizedMethods              []string `orm:"authorized_methods"`
	ReleaseDigest                  string   `orm:"release_digest"`
	BindingDeclarationDigest       string   `orm:"binding_declaration_digest"`
	ConfigurationInterfaceNo       string   `orm:"configuration_interface_no"`
	InterfaceConfigurationRevision int64    `orm:"interface_configuration_revision"`
	ProductSchemaVersion           string   `orm:"product_schema_version"`
	ProductDeclarationSnapshot     string   `orm:"product_declaration_snapshot"`
	CustomerSchemaVersion          string   `orm:"customer_schema_version"`
	CustomerPresentationSnapshot   string   `orm:"customer_presentation_snapshot"`
	CustomerDeclarationSnapshot    string   `orm:"customer_declaration_snapshot"`
	VersionDeclarationDigest       string   `orm:"version_declaration_digest"`
	InterfaceSchemaVersion         string   `orm:"interface_schema_version"`
	InterfaceDeclarationSnapshot   string   `orm:"interface_declaration_snapshot"`
	InterfacePublicValueSnapshot   string   `orm:"interface_public_value_snapshot"`
	InterfaceDeclarationDigest     string   `orm:"interface_declaration_digest"`
	InterfaceValueDigest           string   `orm:"interface_value_digest"`
	ResourceOwnerRevision          string   `orm:"resource_owner_configuration_revision"`
	ImportedContract               *entity.CatalogImportContract
}

func (s *frozenCustomerConfigurationValidator) ValidateBatch(ctx context.Context, inputs []FrozenCustomerConfigurationInput) ([]FrozenCustomerConfigurationEvidence, error) {
	if len(inputs) < 1 || len(inputs) > maximumFrozenConfigurationBatch {
		return nil, gerror.New("frozen customer configuration batch is invalid")
	}
	type target struct {
		VersionNo   string `json:"version_no"`
		InterfaceNo string `json:"interface_no"`
		Revision    int64  `json:"revision"`
	}
	targets := make([]target, 0, len(inputs))
	targetSeen := make(map[string]bool, len(inputs))
	for _, input := range inputs {
		if !validFrozenConfigurationInput(input) {
			return nil, gerror.New("frozen customer configuration evidence is invalid")
		}
		key := frozenValidationTargetKey(input.VersionNo, input.InterfaceConfigurationRevision)
		if !targetSeen[key] {
			targetSeen[key] = true
			targets = append(targets, target{input.VersionNo, input.InterfaceInstanceNo, input.InterfaceConfigurationRevision})
		}
	}
	targetJSON, err := json.Marshal(targets)
	if err != nil {
		return nil, err
	}
	var rows []frozenValidationBatchRow
	if err := s.database.GetScan(ctx, &rows, `WITH requested AS (
SELECT target.version_no,target.interface_no,target.revision FROM jsonb_to_recordset(CAST(? AS jsonb)) AS target(version_no TEXT,interface_no TEXT,revision BIGINT)
)
SELECT binding.version_no,interface.interface_origin_code,interface.resource_owner_id,
binding.interface_instance_no AS binding_interface_instance_no,binding.plugin_id,binding.extension_point_code,
binding.interface_display_name_snapshot,
binding.plugin_release_version,binding.contract_version,binding.authorized_methods,binding.release_digest,
binding.declaration_digest AS binding_declaration_digest,
configuration.interface_instance_no AS configuration_interface_no,revision.revision AS interface_configuration_revision,
configuration.product_schema_version,configuration.product_declaration_snapshot,
configuration.customer_schema_version,configuration.customer_declaration_snapshot,configuration.customer_presentation_snapshot,
configuration.declaration_digest AS version_declaration_digest,
revision.schema_version AS interface_schema_version,revision.declaration_snapshot AS interface_declaration_snapshot,
revision.public_value_snapshot AS interface_public_value_snapshot,
revision.declaration_digest AS interface_declaration_digest,revision.value_digest AS interface_value_digest,
revision.resource_owner_configuration_revision
FROM plugin_idc_core_catalog_product_version_fulfillment_binding binding
JOIN requested ON requested.version_no=binding.version_no AND requested.interface_no=binding.interface_instance_no
JOIN plugin_idc_core_product_automation_interface_instance interface ON interface.tenant_id=binding.tenant_id AND interface.interface_instance_no=binding.interface_instance_no
JOIN plugin_idc_core_catalog_product_version_configuration configuration
  ON configuration.tenant_id=binding.tenant_id AND configuration.version_no=binding.version_no
JOIN plugin_idc_core_product_automation_interface_configuration_revision revision
  ON revision.tenant_id=configuration.tenant_id
 AND revision.interface_instance_no=configuration.interface_instance_no
 AND revision.revision=requested.revision
 AND (interface.interface_origin_code='resource' OR revision.revision=configuration.interface_configuration_revision)
WHERE binding.tenant_id=0`, string(targetJSON)); err != nil {
		return nil, gerror.Wrap(err, "load frozen configuration batch evidence")
	}
	if len(rows) != len(targets) {
		return nil, gerror.New("frozen configuration batch evidence is unavailable")
	}
	exactCache, err := s.loadFrozenProviderEvidenceBatch(ctx, rows)
	if err != nil {
		return nil, err
	}
	peerVersions := make([]string, 0)
	for _, row := range rows {
		// Exact declared import capability controls this bounded optional read.
		if supportsPortableImport(exactCache[frozenProviderEvidenceKey(row)]) {
			peerVersions = append(peerVersions, row.VersionNo)
		}
	}
	if len(peerVersions) > 0 {
		var contracts []*entity.CatalogImportContract
		if err := s.database.Model(do.CatalogImportContract{}).Ctx(ctx).Where(do.CatalogImportContract{TenantId: 0}).WhereIn("version_no", peerVersions).Scan(&contracts); err != nil {
			return nil, err
		}
		byVersion := make(map[string]*entity.CatalogImportContract, len(contracts))
		for _, contract := range contracts {
			byVersion[contract.VersionNo] = contract
		}
		for index := range rows {
			rows[index].ImportedContract = byVersion[rows[index].VersionNo]
		}
	}
	byVersion := make(map[string]frozenValidationBatchRow, len(rows))
	for _, row := range rows {
		byVersion[frozenValidationTargetKey(row.VersionNo, row.InterfaceConfigurationRevision)] = row
	}
	result := make([]FrozenCustomerConfigurationEvidence, len(inputs))
	fileBatches := make([]purchaseconfigurationfile.OrderEvidenceBatchInput, 0)
	fileResultIndex := make([]int, 0)
	for index, input := range inputs {
		row, ok := byVersion[frozenValidationTargetKey(input.VersionNo, input.InterfaceConfigurationRevision)]
		if !ok {
			return nil, gerror.New("frozen configuration binding evidence does not match")
		}
		evidence, fileBatch, err := s.validateFrozenBatchItem(input, row, exactCache)
		if err != nil {
			return nil, err
		}
		result[index] = evidence
		if fileBatch != nil {
			fileBatches = append(fileBatches, *fileBatch)
			fileResultIndex = append(fileResultIndex, index)
		}
	}
	if len(fileBatches) > 0 {
		verified, err := s.files.VerifyOrderEvidenceBatch(ctx, fileBatches)
		if err != nil || len(verified) != len(fileBatches) {
			return nil, gerror.New("frozen customer files are unavailable")
		}
		for index, fields := range verified {
			result[fileResultIndex[index]].VerifiedFiles = fields
		}
	}
	return result, nil
}

func frozenValidationTargetKey(versionNo string, revision int64) string {
	return fmt.Sprintf("%s:%d", versionNo, revision)
}

func validFrozenConfigurationInput(input FrozenCustomerConfigurationInput) bool {
	return input.OrderNo != "" && len(input.OrderNo) <= 64 && input.CustomerNo != "" && len(input.CustomerNo) <= 40 && input.UploaderUserID > 0 &&
		validateBusinessNo(input.VersionNo, "ver") == nil && validateBusinessNo(input.InterfaceInstanceNo, "pfi") == nil &&
		input.InterfaceConfigurationRevision > 0 && validConfigurationDigest(input.CustomerDeclarationDigest) &&
		validConfigurationDigest(input.CandidateSnapshotDigest) && validConfigurationDigest(input.CustomerValueDigest)
}

func (s *frozenCustomerConfigurationValidator) validateFrozenBatchItem(
	input FrozenCustomerConfigurationInput,
	row frozenValidationBatchRow,
	exactCache map[string]*capmodel.BusinessExtensionProviderEvidence,
) (FrozenCustomerConfigurationEvidence, *purchaseconfigurationfile.OrderEvidenceBatchInput, error) {
	if row.BindingInterfaceInstanceNo != input.InterfaceInstanceNo || row.ConfigurationInterfaceNo != input.InterfaceInstanceNo || row.InterfaceConfigurationRevision != input.InterfaceConfigurationRevision {
		return FrozenCustomerConfigurationEvidence{}, nil, gerror.New("frozen configuration binding evidence does not match")
	}
	exact := exactCache[frozenProviderEvidenceKey(row)]
	if exact == nil {
		return FrozenCustomerConfigurationEvidence{}, nil, gerror.New("frozen configuration provider evidence is unavailable")
	}
	binding := &entity.CatalogProductVersionFulfillmentBinding{VersionNo: row.VersionNo, InterfaceInstanceNo: row.BindingInterfaceInstanceNo, InterfaceDisplayNameSnapshot: row.InterfaceDisplayNameSnapshot, PluginId: row.PluginID, ExtensionPointCode: row.ExtensionPointCode, PluginReleaseVersion: row.PluginReleaseVersion, ContractVersion: row.ContractVersion, AuthorizedMethods: row.AuthorizedMethods, ReleaseDigest: row.ReleaseDigest, DeclarationDigest: row.BindingDeclarationDigest}
	if !sameProviderContract(bindingEvidence(binding), exact) {
		return FrozenCustomerConfigurationEvidence{}, nil, gerror.New("frozen configuration provider evidence does not match")
	}
	customer, err := validateFrozenBatchDeclarations(row, input, exact)
	if err != nil {
		return FrozenCustomerConfigurationEvidence{}, nil, err
	}
	presentations, presentationErr := configurationcontract.DecodePresentations(row.CustomerPresentationSnapshot)
	if presentationErr != nil || configurationcontract.ValidatePresentations(customer, presentations) != nil || configurationcontract.ValidateHiddenValues(customer, presentations, input.Values) != nil || len(configurationcontract.PresentationIssues(customer, presentations, input.Values)) > 0 {
		return FrozenCustomerConfigurationEvidence{}, nil, gerror.New("frozen customer controls are invalid")
	}
	if row.ImportedContract == nil {
		selections, selectionErr := frozenCustomerSelections(exact, customer)
		if selectionErr != nil {
			return FrozenCustomerConfigurationEvidence{}, nil, selectionErr
		}
		selections, selectionErr = withCustomerPresentations(selections, row.CustomerPresentationSnapshot)
		if selectionErr != nil {
			return FrozenCustomerConfigurationEvidence{}, nil, selectionErr
		}
		if selectionErr = validateProductFieldBindings(exact, selections); selectionErr != nil {
			return FrozenCustomerConfigurationEvidence{}, nil, selectionErr
		}
	} else {
		// The exact import contract was verified with channel revision and all
		// declaration digests above. Its remote fields are not the connector's
		// optional local field list; no extra local parameter mapping is allowed.
		for _, presentation := range presentations {
			if presentation.ProductFieldKey != "" {
				return FrozenCustomerConfigurationEvidence{}, nil, gerror.New("imported configuration cannot add local supplier parameter bindings")
			}
		}
	}
	candidateJSON, err := json.Marshal(input.Candidates)
	if err != nil || configurationDigest(candidateJSON) != input.CandidateSnapshotDigest {
		return FrozenCustomerConfigurationEvidence{}, nil, gerror.New("frozen customer candidate snapshot does not match")
	}
	effectiveCustomer, err := configurationcontract.WithPresentationChoiceRules(customer, presentations)
	if err != nil {
		return FrozenCustomerConfigurationEvidence{}, nil, err
	}
	compiled, err := configurationcontract.CompileResolved(effectiveCustomer, input.Values, configurationSurfaceCustomer, exact.AuthorizedMethods, input.Candidates)
	if err != nil || configurationDigest(compiled.ValuesJSON) != input.CustomerValueDigest {
		return FrozenCustomerConfigurationEvidence{}, nil, gerror.New("frozen customer values are invalid")
	}
	evidence := FrozenCustomerConfigurationEvidence{CustomerValueDigest: input.CustomerValueDigest, CandidateSnapshotDigest: input.CandidateSnapshotDigest}
	fileInputs, err := frozenCustomerFileInputs(input, customer)
	if err != nil || len(fileInputs) == 0 {
		return evidence, nil, err
	}
	batch := &purchaseconfigurationfile.OrderEvidenceBatchInput{
		Owner:  purchaseconfigurationfile.OrderEvidenceOwner{OrderNo: input.OrderNo, CustomerNo: input.CustomerNo, UploaderUserID: input.UploaderUserID},
		Fields: fileInputs,
	}
	return evidence, batch, nil
}

func (s *frozenCustomerConfigurationValidator) loadFrozenProviderEvidenceBatch(ctx context.Context, rows []frozenValidationBatchRow) (map[string]*capmodel.BusinessExtensionProviderEvidence, error) {
	inputs := make([]plugincap.BusinessExtensionProviderExactInput, 0, len(rows))
	keys := make([]string, 0, len(rows))
	seen := make(map[string]struct{}, len(rows))
	for _, row := range rows {
		key := frozenProviderEvidenceKey(row)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		keys = append(keys, key)
		inputs = append(inputs, plugincap.BusinessExtensionProviderExactInput{PluginID: row.PluginID, ReleaseVersion: row.PluginReleaseVersion, Point: row.ExtensionPointCode, ContractVersion: row.ContractVersion, ReleaseDigest: row.ReleaseDigest, DeclarationDigest: row.BindingDeclarationDigest})
	}
	exactItems, err := s.extensions.GetProvidersExact(ctx, inputs)
	if err != nil || len(exactItems) != len(inputs) {
		return nil, gerror.New("frozen configuration provider evidence is unavailable")
	}
	result := make(map[string]*capmodel.BusinessExtensionProviderEvidence, len(exactItems))
	for index, exact := range exactItems {
		if exact == nil {
			return nil, gerror.New("frozen configuration provider evidence is unavailable")
		}
		result[keys[index]] = exact
	}
	return result, nil
}

func frozenProviderEvidenceKey(row frozenValidationBatchRow) string {
	return fmt.Sprintf("%s\x00%s\x00%s\x00%s\x00%s\x00%s", row.PluginID, row.PluginReleaseVersion, row.ExtensionPointCode, row.ContractVersion, row.ReleaseDigest, row.BindingDeclarationDigest)
}

func validateFrozenBatchDeclarations(
	row frozenValidationBatchRow,
	input FrozenCustomerConfigurationInput,
	exact *capmodel.BusinessExtensionProviderEvidence,
) (*capmodel.ConfigurationSchemaDeclaration, error) {
	customer, err := decodeConfigurationDeclaration(row.CustomerDeclarationSnapshot)
	if err != nil {
		return nil, err
	}
	var expectedCustomer *capmodel.ConfigurationSchemaDeclaration
	if row.ImportedContract != nil {
		binding := &entity.CatalogProductVersionFulfillmentBinding{VersionNo: row.VersionNo, InterfaceInstanceNo: row.BindingInterfaceInstanceNo, InterfaceDisplayNameSnapshot: row.InterfaceDisplayNameSnapshot, PluginId: row.PluginID, ExtensionPointCode: row.ExtensionPointCode, PluginReleaseVersion: row.PluginReleaseVersion, ContractVersion: row.ContractVersion, AuthorizedMethods: row.AuthorizedMethods, ReleaseDigest: row.ReleaseDigest, DeclarationDigest: row.BindingDeclarationDigest}
		revision := &entity.ProductAutomationInterfaceConfigurationRevision{InterfaceInstanceNo: row.BindingInterfaceInstanceNo, ResourceOwnerConfigurationRevision: row.ResourceOwnerRevision}
		expectedCustomer, err = verifiedPortableCustomerSchema(row.ImportedContract, binding, revision, exact, row.CustomerPresentationSnapshot)
	} else {
		expectedCustomer, err = verifiedCustomerSchema(exact, row.CustomerDeclarationSnapshot)
	}
	if err != nil || !sameCompatibleProductConfiguration(customer, expectedCustomer) {
		return nil, gerror.New("frozen customer declaration is unavailable")
	}
	product, err := decodeConfigurationDeclaration(row.ProductDeclarationSnapshot)
	if err != nil || !sameCompatibleProductConfiguration(product, exact.ProductConfiguration) {
		return nil, gerror.New("frozen product declaration is unavailable")
	}
	interfaceSchema, err := decodeConfigurationDeclaration(row.InterfaceDeclarationSnapshot)
	expectedInterface := exact.InterfaceConfiguration
	if row.InterfaceOrigin == "resource" {
		if row.ResourceOwnerID != resourceinterfaceprojectioncap.OwnerID || row.ResourceOwnerRevision == "" {
			return nil, gerror.New("frozen interface configuration evidence does not match")
		}
		// The immutable resource projection binds this opaque owner revision;
		// resource credentials and their schema remain in the resource owner.
		expectedInterface = &capmodel.ConfigurationSchemaDeclaration{Version: "resource-owner-v1"}
	} else if row.ResourceOwnerRevision != "" || row.ImportedContract != nil {
		return nil, gerror.New("frozen interface configuration evidence does not match")
	}
	if err != nil || !sameExactConfiguration(interfaceSchema, expectedInterface) || configurationSchemaVersion(interfaceSchema) != row.InterfaceSchemaVersion || configurationDeclarationDigest(interfaceSchema) != row.InterfaceDeclarationDigest || configurationStoredJSONDigest([]byte(row.InterfacePublicValueSnapshot)) != row.InterfaceValueDigest {
		return nil, gerror.New("frozen interface configuration evidence does not match")
	}
	customerJSON, err := configurationDeclarationJSON(customer)
	if err != nil || configurationDigest(customerJSON) != input.CustomerDeclarationDigest {
		return nil, gerror.New("frozen customer declaration digest does not match")
	}
	productJSON, err := configurationDeclarationJSON(product)
	if err != nil || configurationDigest(append(append([]byte{}, productJSON...), customerJSON...)) != row.VersionDeclarationDigest || configurationSchemaVersion(product) != row.ProductSchemaVersion || configurationSchemaVersion(customer) != row.CustomerSchemaVersion {
		return nil, gerror.New("frozen product version declaration evidence does not match")
	}
	return customer, nil
}
