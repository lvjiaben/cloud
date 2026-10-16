// This file freezes provider-declared product and customer configuration on a
// draft product version and reuses it as a publication admission condition.

package productautomation

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
	"lina-core/pkg/plugin/capability/capmodel"
	"lina-core/pkg/plugin/capability/plugincap"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/internal/model/do"
	"lina-plugin-idc-core/backend/internal/model/entity"
	"lina-plugin-idc-core/backend/internal/service/catalog"
	"lina-plugin-idc-core/backend/internal/service/configurationcontract"
)

// SaveVersionConfiguration validates current provider evidence and stores one
// draft-only snapshot. Published versions are rejected by both service and SQL.
func (s *serviceImpl) SaveVersionConfiguration(ctx context.Context, input SaveVersionConfigurationInput) error {
	authorization, err := s.authorizer.AuthorizeVersionConfiguration(ctx, input.VersionNo)
	if err != nil {
		return err
	}
	if err = requireAuthorization(authorization); err != nil {
		return err
	}
	if validateBusinessNo(input.VersionNo, "ver") != nil || input.ExpectedVersion < 0 {
		return bizerr.NewCode(catalog.CodeManagementInvalid)
	}
	binding, err := s.loadBinding(ctx, input.VersionNo, false)
	if err != nil || binding == nil {
		return bizerr.NewCode(catalog.CodeManagementConflict)
	}
	exact, err := s.extensions.GetProviderExact(ctx, plugincap.BusinessExtensionProviderExactInput{
		PluginID: binding.PluginId, ReleaseVersion: binding.PluginReleaseVersion,
		Point: binding.ExtensionPointCode, ContractVersion: binding.ContractVersion,
		ReleaseDigest: binding.ReleaseDigest, DeclarationDigest: binding.DeclarationDigest,
	})
	if err != nil || !sameExactEvidence(bindingEvidence(binding), exact) {
		return bizerr.NewCode(CodeEvidenceUnavailable)
	}
	if exact.PluginID == portableImportProviderID {
		var imported *entity.CatalogImportContract
		if loadErr := s.database.Model(do.CatalogImportContract{}).Ctx(ctx).Where(do.CatalogImportContract{TenantId: 0, VersionNo: input.VersionNo}).Scan(&imported); loadErr != nil {
			return bizerr.WrapCode(loadErr, catalog.CodeManagementUnavailable)
		}
		if imported != nil {
			return bizerr.NewCode(catalog.CodeManagementConflict)
		}
	}
	currentInterfaceRevision, err := s.loadLatestInterfaceConfigurationRevision(ctx, binding.InterfaceInstanceNo, false)
	if err != nil || currentInterfaceRevision == nil {
		return bizerr.NewCode(catalog.CodeManagementConflict)
	}
	scope := dynamicCandidateScope{interfaceInstanceNo: binding.InterfaceInstanceNo, configurationRevision: currentInterfaceRevision.Revision}
	product, err := s.compileConfigurationResolved(ctx, exact, exact.ProductConfiguration, input.MerchantValues, configurationSurfaceProduct, scope)
	if err != nil {
		return bizerr.NewCode(catalog.CodeManagementInvalid)
	}
	customerSchema, err := selectCustomerSchema(exact, input.CustomerFieldSelections)
	if err != nil {
		return bizerr.NewCode(catalog.CodeManagementInvalid)
	}
	if customerSchema != nil {
		if err = s.validateConfigurationRuntime(ctx, exact, customerSchema, nil, configurationSurfaceCustomer, scope); err != nil {
			return bizerr.NewCode(catalog.CodeManagementInvalid)
		}
	}
	customerJSON, err := configurationDeclarationJSON(customerSchema)
	if err != nil {
		return bizerr.WrapCode(err, catalog.CodeManagementUnavailable)
	}
	return normalizeManagementWriteError(s.database.Transaction(ctx, func(txCtx context.Context, _ gdb.TX) error {
		if lockErr := s.lockDraftVersion(txCtx, input.VersionNo); lockErr != nil {
			return lockErr
		}
		lockedBinding, loadErr := s.loadBinding(txCtx, input.VersionNo, true)
		if loadErr != nil || lockedBinding == nil || !sameExactEvidence(bindingEvidence(lockedBinding), exact) {
			return bizerr.NewCode(catalog.CodeManagementConflict)
		}
		interfaceRevision, loadErr := s.loadLatestInterfaceConfigurationRevision(txCtx, lockedBinding.InterfaceInstanceNo, true)
		if loadErr != nil || interfaceRevision == nil || interfaceRevision.Revision != currentInterfaceRevision.Revision {
			return bizerr.NewCode(catalog.CodeManagementConflict)
		}
		interfaceRevisionNo := int64(0)
		if interfaceRevision != nil {
			interfaceRevisionNo = interfaceRevision.Revision
		}
		var current *entity.CatalogProductVersionConfiguration
		if loadErr = s.database.Model(do.CatalogProductVersionConfiguration{}).Ctx(txCtx).
			Where(do.CatalogProductVersionConfiguration{TenantId: idccap.DefaultTenantID, VersionNo: input.VersionNo}).LockUpdate().Scan(&current); loadErr != nil {
			return gerror.Wrap(loadErr, "load product version configuration")
		}
		productVersion := configurationSchemaVersion(exact.ProductConfiguration)
		customerVersion := configurationSchemaVersion(customerSchema)
		declarationDigest := configurationDigest(append(append([]byte{}, product.DeclarationJSON...), customerJSON...))
		merchantValueDigest := configurationDigest(product.ValuesJSON)
		data := do.CatalogProductVersionConfiguration{
			TenantId: idccap.DefaultTenantID, VersionNo: input.VersionNo,
			InterfaceInstanceNo: lockedBinding.InterfaceInstanceNo, InterfaceConfigurationRevision: interfaceRevisionNo,
			ProductSchemaVersion: productVersion, ProductDeclarationSnapshot: string(product.DeclarationJSON),
			MerchantValueSnapshot: string(product.ValuesJSON), CustomerSchemaVersion: customerVersion,
			CustomerDeclarationSnapshot: string(customerJSON), DeclarationDigest: declarationDigest,
			MerchantValueDigest: merchantValueDigest, ChangedBy: authorization.ActorUserID,
		}
		targetVersion := int64(1)
		if current == nil {
			if input.ExpectedVersion != 0 {
				return bizerr.NewCode(catalog.CodeManagementConflict)
			}
		} else {
			if input.ExpectedVersion != current.Version {
				return bizerr.NewCode(catalog.CodeManagementConflict)
			}
			targetVersion = current.Version + 1
		}
		data.Version = targetVersion
		if bindErr := s.bindManagementConfigurationFiles(txCtx, configurationSurfaceProduct, input.VersionNo, input.ExpectedVersion, targetVersion, authorization.ActorUserID, exact.ProductConfiguration, input.MerchantValues); bindErr != nil {
			return bindErr
		}
		if current == nil {
			_, loadErr = s.database.Model(do.CatalogProductVersionConfiguration{}).Ctx(txCtx).Data(data).Insert()
		} else {
			_, loadErr = s.database.Model(do.CatalogProductVersionConfiguration{}).Ctx(txCtx).
				Where(do.CatalogProductVersionConfiguration{TenantId: idccap.DefaultTenantID, VersionNo: input.VersionNo, Version: current.Version}).Data(data).Update()
		}
		if loadErr != nil {
			return gerror.Wrap(loadErr, "save product version configuration")
		}
		beforeSnapshot := map[string]any{}
		if current != nil {
			beforeSnapshot = map[string]any{
				"version":                          current.Version,
				"interface_configuration_revision": current.InterfaceConfigurationRevision,
				"declaration_digest":               current.DeclarationDigest,
				"merchant_value_digest":            current.MerchantValueDigest,
			}
		}
		return s.audit.AppendBusinessTransition(txCtx, idccap.BusinessAuditTransition{
			IdempotencyScope: "product-automation.version-configuration", IdempotencyKey: versionConfigurationAuditKey(input.VersionNo, targetVersion, interfaceRevisionNo, declarationDigest, merchantValueDigest),
			OperationCode: "product-automation.version-configuration.save", TargetType: "catalog_product_version", TargetNo: input.VersionNo,
			ResultCode: "succeeded", ReasonCode: "manual_save", RiskCode: "medium", RequiredPermission: authorization.RequiredPermission,
			BeforeSnapshot: beforeSnapshot, AfterSnapshot: map[string]any{"version": targetVersion, "interface_configuration_revision": interfaceRevisionNo, "declaration_digest": declarationDigest, "merchant_value_digest": merchantValueDigest},
		})
	}))
}

func versionConfigurationAuditKey(versionNo string, version, interfaceRevision int64, declarationDigest, merchantValueDigest string) string {
	return versionNo + ":" + fmt.Sprintf("%d:%d", version, interfaceRevision) + ":" + declarationDigest + ":" + merchantValueDigest
}

func (s *serviceImpl) loadLatestInterfaceConfigurationRevision(ctx context.Context, interfaceNo string, lock bool) (*entity.ProductAutomationInterfaceConfigurationRevision, error) {
	model := s.database.Model(do.ProductAutomationInterfaceConfigurationRevision{}).Ctx(ctx).
		Where(do.ProductAutomationInterfaceConfigurationRevision{TenantId: idccap.DefaultTenantID, InterfaceInstanceNo: interfaceNo}).OrderDesc("revision").Limit(1)
	if lock {
		model = model.LockUpdate()
	}
	var row *entity.ProductAutomationInterfaceConfigurationRevision
	if err := model.Scan(&row); err != nil {
		return nil, gerror.Wrap(err, "load interface configuration revision")
	}
	return row, nil
}

func configurationDeclarationJSON(schema *capmodel.ConfigurationSchemaDeclaration) ([]byte, error) {
	if schema == nil {
		return []byte(`{}`), nil
	}
	value, err := json.Marshal(schema)
	return value, gerror.Wrap(err, "encode configuration declaration")
}

func configurationSchemaVersion(schema *capmodel.ConfigurationSchemaDeclaration) string {
	if schema == nil {
		return "none"
	}
	return schema.Version
}

type frozenConfigurationCandidates struct {
	interfaceValues configurationcontract.ResolvedCandidates
	productValues   configurationcontract.ResolvedCandidates
	customerValues  configurationcontract.ResolvedCandidates
}

func (s *serviceImpl) resolveFrozenConfigurationCandidates(
	ctx context.Context,
	binding *entity.CatalogProductVersionFulfillmentBinding,
	exact *capmodel.BusinessExtensionProviderEvidence,
) (*frozenConfigurationCandidates, error) {
	if binding == nil || exact == nil {
		return nil, bizerr.NewCode(catalog.CodeManagementConflict)
	}
	interfaceValues, err := s.resolveCurrentInterfaceCandidates(ctx, binding.InterfaceInstanceNo, exact)
	if err != nil {
		return nil, err
	}
	var row *entity.CatalogProductVersionConfiguration
	if err = s.database.Model(do.CatalogProductVersionConfiguration{}).Ctx(ctx).
		Where(do.CatalogProductVersionConfiguration{TenantId: idccap.DefaultTenantID, VersionNo: binding.VersionNo}).
		Scan(&row); err != nil || row == nil {
		return nil, bizerr.NewCode(catalog.CodeManagementConflict)
	}
	merchantValues, err := configurationValuesFromJSON(exact.ProductConfiguration, []byte(row.MerchantValueSnapshot))
	if err != nil {
		return nil, bizerr.NewCode(catalog.CodeManagementConflict)
	}
	customerSchema, schemaErr := s.importedCustomerSchema(ctx, binding, row, exact)
	if schemaErr != nil {
		return nil, bizerr.NewCode(catalog.CodeManagementConflict)
	}
	scope := dynamicCandidateScope{versionNo: binding.VersionNo, interfaceInstanceNo: binding.InterfaceInstanceNo, configurationRevision: row.InterfaceConfigurationRevision}
	productValues, err := s.resolveDynamicCandidates(ctx, exact, exact.ProductConfiguration, merchantValues, configurationSurfaceProduct, scope)
	if err != nil {
		return nil, err
	}
	customerValues, err := s.resolveDynamicCandidates(ctx, exact, customerSchema, nil, configurationSurfaceCustomer, scope)
	if err != nil {
		return nil, err
	}
	return &frozenConfigurationCandidates{
		interfaceValues: interfaceValues, productValues: productValues, customerValues: customerValues,
	}, nil
}

func (s *serviceImpl) validateFrozenVersionConfiguration(
	ctx context.Context,
	binding *entity.CatalogProductVersionFulfillmentBinding,
	exact *capmodel.BusinessExtensionProviderEvidence,
	lock bool,
	resolved *frozenConfigurationCandidates,
) error {
	if binding == nil || exact == nil || resolved == nil ||
		!configurationRuntimeAvailable(exact.ProductConfiguration, configurationSurfaceProduct) ||
		!configurationRuntimeAvailable(exact.CustomerConfiguration, configurationSurfaceCustomer) {
		return bizerr.NewCode(catalog.CodeManagementConflict)
	}
	latestInterfaceRevision, err := s.loadLatestInterfaceConfigurationRevision(
		ctx, binding.InterfaceInstanceNo, lock,
	)
	if err != nil || latestInterfaceRevision == nil {
		return bizerr.NewCode(catalog.CodeManagementConflict)
	}
	if err = s.validateInterfaceConfigurationRevision(
		ctx, binding.InterfaceInstanceNo, exact, latestInterfaceRevision, resolved.interfaceValues,
	); err != nil {
		if bizerr.Is(err, CodeEvidenceUnavailable) || bizerr.Is(err, catalog.CodeManagementConflict) {
			return bizerr.NewCode(catalog.CodeManagementConflict)
		}
		return err
	}
	model := s.database.Model(do.CatalogProductVersionConfiguration{}).Ctx(ctx).
		Where(do.CatalogProductVersionConfiguration{
			TenantId: idccap.DefaultTenantID, VersionNo: binding.VersionNo,
		})
	if lock {
		model = model.LockUpdate()
	}
	var row *entity.CatalogProductVersionConfiguration
	if err = model.Scan(&row); err != nil {
		return gerror.Wrap(err, "load frozen product version configuration")
	}
	if row == nil || row.InterfaceInstanceNo != binding.InterfaceInstanceNo ||
		row.InterfaceConfigurationRevision != latestInterfaceRevision.Revision {
		return bizerr.NewCode(catalog.CodeManagementConflict)
	}
	customerSchema, schemaErr := s.importedCustomerSchema(ctx, binding, row, exact)
	if schemaErr != nil {
		return bizerr.NewCode(catalog.CodeManagementConflict)
	}
	productDeclaration, err := configurationDeclarationJSON(exact.ProductConfiguration)
	if err != nil {
		return gerror.Wrap(err, "encode exact product configuration declaration")
	}
	customerDeclaration, err := configurationDeclarationJSON(customerSchema)
	if err != nil {
		return gerror.Wrap(err, "encode exact customer configuration declaration")
	}
	if row.ProductSchemaVersion != configurationSchemaVersion(exact.ProductConfiguration) ||
		row.CustomerSchemaVersion != configurationSchemaVersion(customerSchema) ||
		row.DeclarationDigest != configurationDigest(append(append([]byte{}, productDeclaration...), customerDeclaration...)) {
		return bizerr.NewCode(catalog.CodeManagementConflict)
	}
	storedProduct, err := decodeConfigurationDeclaration(row.ProductDeclarationSnapshot)
	if err != nil || !sameExactConfiguration(storedProduct, exact.ProductConfiguration) {
		return bizerr.NewCode(catalog.CodeManagementConflict)
	}
	storedCustomer, err := decodeConfigurationDeclaration(row.CustomerDeclarationSnapshot)
	if err != nil || !sameExactConfiguration(storedCustomer, customerSchema) {
		return bizerr.NewCode(catalog.CodeManagementConflict)
	}
	merchantValues, err := configurationValuesFromJSON(
		exact.ProductConfiguration, []byte(row.MerchantValueSnapshot),
	)
	if err != nil {
		return bizerr.NewCode(catalog.CodeManagementConflict)
	}
	compiled, err := compileConfigurationWithResolved(
		exact, exact.ProductConfiguration, merchantValues,
		configurationSurfaceProduct, resolved.productValues,
	)
	if err != nil || configurationDigest(compiled.DeclarationJSON) != configurationDigest(productDeclaration) ||
		configurationDigest(compiled.ValuesJSON) != row.MerchantValueDigest {
		return bizerr.NewCode(catalog.CodeManagementConflict)
	}
	if err = s.validateProviderProductConfiguration(ctx, exact, compiled.ValuesJSON); err != nil {
		return err
	}
	if _, err = configurationcontract.EvaluateResolved(
		customerSchema, nil, configurationSurfaceCustomer,
		exact.AuthorizedMethods, resolved.customerValues,
	); err != nil {
		return bizerr.NewCode(catalog.CodeManagementConflict)
	}
	if err = s.validateCustomerSelectionPrices(ctx, binding.VersionNo, exact, customerSchema); err != nil {
		return bizerr.WrapCode(err, codeCustomerOptionPricesRequired)
	}
	return nil
}
