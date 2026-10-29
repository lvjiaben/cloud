// Commercial fields can exist before any supplier is chosen. Manual offers
// use the same schema compiler, presentation and price identities; no provider
// evidence, credentials, dynamic methods or synthetic interface is fabricated.
package productautomation

import (
	"bytes"
	"context"
	"sort"

	"github.com/gogf/gf/v2/database/gdb"
	"lina-core/pkg/bizerr"
	"lina-core/pkg/plugin/capability/capmodel"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/internal/model/do"
	"lina-plugin-idc-core/backend/internal/model/entity"
	"lina-plugin-idc-core/backend/internal/service/catalog"
	"lina-plugin-idc-core/backend/internal/service/configurationcontract"
)

func emptyOfferSchema() *capmodel.ConfigurationSchemaDeclaration {
	return &capmodel.ConfigurationSchemaDeclaration{Version: "v1", Fields: []*capmodel.ConfigurationFieldDeclaration{}}
}

func manualCustomerSchema(selections []CustomerFieldSelection) (*capmodel.ConfigurationSchemaDeclaration, error) {
	if len(selections) > 128 {
		return nil, bizerr.NewCode(catalog.CodeManagementInvalid)
	}
	schema := emptyOfferSchema()
	seen := make(map[string]bool, len(selections))
	ordered := append([]CustomerFieldSelection(nil), selections...)
	sort.SliceStable(ordered, func(i, j int) bool { return ordered[i].SortOrder < ordered[j].SortOrder })
	for _, selection := range ordered {
		if selection.LocalField == nil || selection.ProductFieldKey != "" || len(selection.ProductChoiceBindings) > 0 || seen[selection.FieldKey] {
			return nil, bizerr.NewCode(catalog.CodeManagementInvalid)
		}
		field, err := localCustomerField(selection)
		if err != nil {
			return nil, bizerr.WrapCode(err, catalog.CodeManagementInvalid)
		}
		seen[selection.FieldKey] = true
		schema.Fields = append(schema.Fields, field)
	}
	if _, err := configurationcontract.ValidateSchema(schema, configurationSurfaceCustomer, nil); err != nil {
		return nil, bizerr.WrapCode(err, catalog.CodeManagementInvalid)
	}
	return schema, nil
}

// readManualConfiguration is private: callers authorize the exact version or
// hold the publication lock before using it. The existence check also prevents
// returning an editable empty definition for an invented version number.
func (s *serviceImpl) readManualConfiguration(ctx context.Context, versionNo string) (*VersionConfiguration, error) {
	exists, err := s.database.Model(do.CatalogProductVersion{}).Ctx(ctx).Where(do.CatalogProductVersion{TenantId: 0, VersionNo: versionNo}).Exist()
	if err != nil {
		return nil, bizerr.WrapCode(err, catalog.CodeManagementUnavailable)
	}
	if !exists {
		return nil, bizerr.NewCode(catalog.CodeManagementNotAccessible)
	}
	row, err := s.loadVersionConfigurationRow(ctx, versionNo)
	if err != nil {
		return nil, bizerr.WrapCode(err, catalog.CodeManagementUnavailable)
	}
	result := &VersionConfiguration{
		ProductDeclaration: emptyOfferSchema(), CustomerDeclaration: emptyOfferSchema(),
		MerchantValues: []ConfigurationValue{}, CustomerFieldSelections: []CustomerFieldSelection{},
		CustomerConfigurableFields: []*capmodel.ConfigurationFieldDeclaration{},
		ProductResolvedCandidates:  configurationcontract.ResolvedCandidates{}, CustomerResolvedCandidates: configurationcontract.ResolvedCandidates{},
	}
	if row == nil {
		return result, nil
	}
	if row.InterfaceInstanceNo != "" || row.InterfaceConfigurationRevision != 0 {
		return nil, bizerr.NewCode(catalog.CodeManagementConflict)
	}
	customer, err := decodeConfigurationDeclaration(row.CustomerDeclarationSnapshot)
	if err != nil || customer == nil {
		return nil, bizerr.WrapCode(err, catalog.CodeManagementUnavailable)
	}
	selections := make([]CustomerFieldSelection, 0, len(customer.Fields))
	for _, field := range customer.Fields {
		if field == nil {
			return nil, bizerr.NewCode(catalog.CodeManagementUnavailable)
		}
		selections = append(selections, CustomerFieldSelection{FieldKey: field.Key, LocalField: field, Minimum: field.Minimum, Maximum: field.Maximum})
	}
	selections, err = withCustomerPresentations(selections, row.CustomerPresentationSnapshot)
	if err != nil {
		return nil, bizerr.WrapCode(err, catalog.CodeManagementUnavailable)
	}
	canonical, err := manualCustomerSchema(selections)
	if err != nil {
		return nil, bizerr.WrapCode(err, catalog.CodeManagementUnavailable)
	}
	canonical.Conditions = customer.Conditions
	if _, err = configurationcontract.ValidateSchema(canonical, configurationSurfaceCustomer, nil); err != nil {
		return nil, bizerr.WrapCode(err, catalog.CodeManagementUnavailable)
	}
	productJSON, err := configurationDeclarationJSON(result.ProductDeclaration)
	if err != nil {
		return nil, bizerr.WrapCode(err, catalog.CodeManagementUnavailable)
	}
	storedProduct, err := decodeConfigurationDeclaration(row.ProductDeclarationSnapshot)
	if err != nil {
		return nil, bizerr.WrapCode(err, catalog.CodeManagementUnavailable)
	}
	storedProductJSON, err := configurationDeclarationJSON(storedProduct)
	if err != nil || !bytes.Equal(storedProductJSON, productJSON) || row.ProductSchemaVersion != "v1" || row.CustomerSchemaVersion != "v1" || row.Version < 1 || row.MerchantValueDigest != configurationDigest([]byte(`{}`)) {
		return nil, bizerr.NewCode(catalog.CodeManagementConflict)
	}
	customerJSON, err := configurationDeclarationJSON(canonical)
	if err != nil || row.DeclarationDigest != configurationDigest(append(append([]byte(nil), productJSON...), customerJSON...)) || row.MerchantValueSnapshot != "{}" {
		return nil, bizerr.NewCode(catalog.CodeManagementConflict)
	}
	presentations, err := configurationcontract.DecodePresentations(row.CustomerPresentationSnapshot)
	if err != nil {
		return nil, bizerr.WrapCode(err, catalog.CodeManagementUnavailable)
	}
	presentation, err := configurationcontract.WithPresentationChoiceRules(canonical, presentations)
	if err != nil {
		return nil, bizerr.WrapCode(err, catalog.CodeManagementUnavailable)
	}
	result.CustomerDeclaration, result.CustomerFieldSelections, result.Version = presentation, selections, row.Version
	return result, nil
}

func (s *serviceImpl) saveManualConfiguration(ctx context.Context, input SaveVersionConfigurationInput, authorization Authorization) error {
	if len(input.MerchantValues) != 0 {
		return bizerr.NewCode(catalog.CodeManagementInvalid)
	}
	selections, err := s.restoreDetachedMappingState(ctx, input, false)
	if err != nil {
		return err
	}
	input.CustomerFieldSelections = selections
	schema, err := manualCustomerSchema(input.CustomerFieldSelections)
	if err != nil {
		return err
	}
	if err = s.retainManualConditions(ctx, input.VersionNo, schema); err != nil {
		return err
	}
	input.CustomerFieldSelections, err = s.retainSavedChoiceRules(ctx, input.VersionNo, input.ExpectedVersion, schema, input.CustomerFieldSelections)
	if err != nil {
		return err
	}
	presentation, err := compileCustomerPresentations(schema, input.CustomerFieldSelections)
	if err != nil {
		return bizerr.WrapCode(err, catalog.CodeManagementInvalid)
	}
	productJSON, err := configurationDeclarationJSON(emptyOfferSchema())
	if err != nil {
		return bizerr.WrapCode(err, catalog.CodeManagementUnavailable)
	}
	customerJSON, err := configurationDeclarationJSON(schema)
	if err != nil {
		return bizerr.WrapCode(err, catalog.CodeManagementUnavailable)
	}
	declarationDigest := configurationDigest(append(append([]byte(nil), productJSON...), customerJSON...))
	valueDigest := configurationDigest([]byte(`{}`))
	return normalizeManagementWriteError(s.database.Transaction(ctx, func(txCtx context.Context, _ gdb.TX) error {
		if err := s.lockDraftVersion(txCtx, input.VersionNo); err != nil {
			return err
		}
		binding, err := s.loadBinding(txCtx, input.VersionNo, true)
		if err != nil {
			return err
		}
		if binding != nil {
			return bizerr.NewCode(catalog.CodeManagementConflict)
		}
		var current *entity.CatalogProductVersionConfiguration
		if err = s.database.Model(do.CatalogProductVersionConfiguration{}).Ctx(txCtx).
			Where(do.CatalogProductVersionConfiguration{TenantId: 0, VersionNo: input.VersionNo}).LockUpdate().Scan(&current); err != nil {
			return bizerr.WrapCode(err, catalog.CodeManagementUnavailable)
		}
		if (current == nil && input.ExpectedVersion != 0) || (current != nil && (current.Version != input.ExpectedVersion || current.InterfaceInstanceNo != "" || current.InterfaceConfigurationRevision != 0)) {
			return bizerr.NewCode(catalog.CodeManagementConflict)
		}
		next := input.ExpectedVersion + 1
		data := do.CatalogProductVersionConfiguration{TenantId: 0, VersionNo: input.VersionNo,
			InterfaceInstanceNo: gdb.Raw("NULL"), InterfaceConfigurationRevision: gdb.Raw("NULL"),
			ProductSchemaVersion: "v1", ProductDeclarationSnapshot: string(productJSON), MerchantValueSnapshot: "{}",
			CustomerSchemaVersion: "v1", CustomerDeclarationSnapshot: string(customerJSON), CustomerPresentationSnapshot: presentation,
			DeclarationDigest: declarationDigest, MerchantValueDigest: valueDigest, Version: next, ChangedBy: authorization.ActorUserID}
		if current == nil {
			_, err = s.database.Model(do.CatalogProductVersionConfiguration{}).Ctx(txCtx).Data(data).Insert()
		} else {
			_, err = s.database.Model(do.CatalogProductVersionConfiguration{}).Ctx(txCtx).
				Where(do.CatalogProductVersionConfiguration{TenantId: 0, VersionNo: input.VersionNo, Version: current.Version}).Data(data).Update()
		}
		if err != nil {
			return bizerr.WrapCode(err, catalog.CodeManagementUnavailable)
		}
		return s.audit.AppendBusinessTransition(txCtx, idccap.BusinessAuditTransition{
			IdempotencyScope: "product-automation.version-configuration", IdempotencyKey: versionConfigurationAuditKey(input.VersionNo, next, 0, declarationDigest, valueDigest),
			OperationCode: "product-automation.version-configuration.save", TargetType: "catalog_product_version", TargetNo: input.VersionNo,
			ResultCode: "succeeded", ReasonCode: "manual_save", RiskCode: "medium", RequiredPermission: authorization.RequiredPermission,
			BeforeSnapshot: map[string]any{"version": input.ExpectedVersion}, AfterSnapshot: map[string]any{"version": next, "declaration_digest": declarationDigest, "merchant_value_digest": valueDigest}})
	}))
}
