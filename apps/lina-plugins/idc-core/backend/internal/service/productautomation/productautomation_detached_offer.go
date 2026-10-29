// Replacing a supplier preserves the merchant's ordinary commercial fields.
// Supplier field names become local identities, while price identities and
// amounts remain attached to the same draft. No old mapping is auto-reused.
package productautomation

import (
	"context"
	"encoding/json"
	"sort"
	"strings"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"
	"lina-core/pkg/bizerr"
	"lina-core/pkg/plugin/capability/capmodel"
	"lina-plugin-idc-core/backend/internal/model/do"
	"lina-plugin-idc-core/backend/internal/model/entity"
	"lina-plugin-idc-core/backend/internal/service/catalog"
	"lina-plugin-idc-core/backend/internal/service/configurationcontract"
)

type detachedOfferDefinition struct {
	configurationVersion int64
	interfaceInstanceNo  string
	schema               *capmodel.ConfigurationSchemaDeclaration
	presentations        map[string]configurationcontract.Presentation
	identities           map[string]string
}

// detachedCommercialDefinition combines persisted merchant facts with one
// complete authorized directory projection. The retained choices become local
// facts, and no old supplier method remains executable after detachment.
func detachedCommercialDefinition(row *entity.CatalogProductVersionConfiguration, candidates configurationcontract.ResolvedCandidates) (*detachedOfferDefinition, error) {
	schema, err := decodeConfigurationDeclaration(row.CustomerDeclarationSnapshot)
	if err != nil || (schema != nil && len(schema.UploadPolicies) != 0) {
		return nil, bizerr.NewCode(codeOfferDetachUnsupported)
	}
	product, err := decodeConfigurationDeclaration(row.ProductDeclarationSnapshot)
	if err != nil {
		return nil, bizerr.NewCode(catalog.CodeManagementConflict)
	}
	if product != nil {
		for _, field := range product.Fields {
			if field == nil || field.Type == capmodel.ConfigurationFieldFile || field.Type == capmodel.ConfigurationFieldSecret {
				return nil, bizerr.NewCode(codeOfferDetachUnsupported)
			}
		}
	}
	productJSON, err := configurationDeclarationJSON(product)
	if err != nil {
		return nil, err
	}
	customerJSON, err := configurationDeclarationJSON(schema)
	if err != nil || row.Version < 1 || row.DeclarationDigest != configurationDigest(append(append([]byte(nil), productJSON...), customerJSON...)) {
		return nil, bizerr.NewCode(catalog.CodeManagementConflict)
	}
	_, merchantJSON, err := decodeStoredMerchantValues(row.MerchantValueSnapshot)
	if err != nil || row.MerchantValueDigest != configurationDigest(merchantJSON) {
		return nil, bizerr.NewCode(catalog.CodeManagementConflict)
	}
	if schema == nil {
		schema = emptyOfferSchema()
	}
	candidates = cloneResolvedCandidates(candidates)
	presentations, err := configurationcontract.DecodePresentations(row.CustomerPresentationSnapshot)
	if err != nil {
		return nil, bizerr.WrapCode(err, catalog.CodeManagementUnavailable)
	}
	var (
		identities = make(map[string]string, len(schema.Fields))
		used       = make(map[string]bool, len(schema.Fields))
		result     = &detachedOfferDefinition{schema: schema, presentations: make(map[string]configurationcontract.Presentation)}
	)
	for _, field := range schema.Fields {
		if field == nil {
			return nil, bizerr.NewCode(codeOfferDetachUnsupported)
		}
		key := field.Key
		if !strings.HasPrefix(key, localCustomerFieldPrefix) {
			key = "local_retained_" + configurationDigest([]byte(field.Key))[:24]
		}
		if used[key] {
			return nil, bizerr.NewCode(codeOfferDetachUnsupported)
		}
		used[key], identities[field.Key] = true, key
	}
	for _, field := range schema.Fields {
		previous := field.Key
		presentation := presentations[previous]
		// A former fixed package parameter already belonged to the base
		// price. Preserve that exemption only while it remains one value.
		presentation.FixedPriceIncluded = presentation.FixedPriceIncluded || presentation.ProductFieldKey != ""
		if field.DynamicCandidates != nil {
			if len(candidates[previous]) == 0 {
				return nil, bizerr.NewCode(codeOfferOptionsRequired)
			}
			field.StaticCandidates = candidates[previous]
		}
		presentation.BasePriceIncluded = presentation.BasePriceIncluded || (!strings.HasPrefix(previous, localCustomerFieldPrefix) && field.Required)
		presentation.AutomationMappingPending = presentation.AutomationMappingPending || presentation.ProductFieldKey != "" || !strings.HasPrefix(previous, localCustomerFieldPrefix)
		presentation.ProductFieldKey = ""
		presentation.ProductChoiceBindings = nil
		field.Key, field.DynamicCandidates = identities[previous], nil
		if _, fieldErr := localCustomerField(CustomerFieldSelection{FieldKey: field.Key, LocalField: field, Minimum: field.Minimum, Maximum: field.Maximum}); fieldErr != nil {
			return nil, bizerr.WrapCode(fieldErr, codeOfferDetachUnsupported)
		}
		for _, rule := range presentation.ChoiceRules {
			renameDetachedChoiceRule(rule, identities)
		}
		result.presentations[field.Key] = presentation
	}
	for _, rule := range schema.Conditions {
		renameDetachedChoiceRule(rule, identities)
	}
	// Independent forms canonically follow merchant order, including a
	// previously provider-owned core field moved among local fields.
	sort.SliceStable(schema.Fields, func(i, j int) bool {
		return result.presentations[schema.Fields[i].Key].SortOrder < result.presentations[schema.Fields[j].Key].SortOrder
	})
	schema.Version = "v1"
	if _, err = configurationcontract.ValidateSchema(schema, configurationSurfaceCustomer, nil); err != nil {
		return nil, bizerr.WrapCode(err, codeOfferDetachUnsupported)
	}
	if err = configurationcontract.ValidatePresentations(schema, result.presentations); err != nil {
		return nil, bizerr.WrapCode(err, codeOfferDetachUnsupported)
	}
	result.identities = identities
	result.configurationVersion, result.interfaceInstanceNo = row.Version, row.InterfaceInstanceNo
	return result, nil
}

// Resolve the old directory before replacing its binding. One complete
// authorized projection serves all dynamic fields. The enclosing settings
// transaction may already exist; all reads remain bounded and revision checked.
func (s *serviceImpl) prepareDetachedOffer(ctx context.Context, versionNo string, actor int) (*detachedOfferDefinition, error) {
	binding, err := s.loadBinding(ctx, versionNo, false)
	if err != nil || binding == nil {
		return nil, err
	}
	authorization, err := s.authorizer.AuthorizeVersionConfiguration(ctx, versionNo)
	if err != nil || requireAuthorization(authorization) != nil || authorization.ActorUserID != actor {
		return nil, bizerr.NewCode(catalog.CodeManagementNotAccessible)
	}
	row, err := s.loadVersionConfigurationRow(ctx, versionNo)
	if err != nil || row == nil || row.InterfaceInstanceNo == "" {
		return nil, err
	}
	imported, err := s.database.Model(do.CatalogImportContract{}).Ctx(ctx).Where(do.CatalogImportContract{TenantId: 0, VersionNo: versionNo}).Exist()
	if err != nil {
		return nil, err
	}
	if imported {
		return nil, bizerr.NewCode(codeOfferDetachUnsupported)
	}
	schema, err := decodeConfigurationDeclaration(row.CustomerDeclarationSnapshot)
	if err != nil {
		return nil, bizerr.NewCode(catalog.CodeManagementConflict)
	}
	if schema == nil {
		return detachedCommercialDefinition(row, nil)
	}
	var candidates configurationcontract.ResolvedCandidates
	for _, field := range schema.Fields {
		if field == nil || field.DynamicCandidates == nil {
			continue
		}
		view, readErr := s.GetVersionConfiguration(ctx, versionNo)
		if readErr != nil || view == nil || view.Version != row.Version {
			return nil, bizerr.NewCode(codeOfferOptionsRequired)
		}
		candidates = view.CustomerResolvedCandidates
		break
	}
	return detachedCommercialDefinition(row, candidates)
}

func renameDetachedChoiceRule(rule *capmodel.ConfigurationRuleDeclaration, identities map[string]string) {
	if rule == nil {
		return
	}
	var renameCondition func(*capmodel.ConfigurationConditionDeclaration)
	renameCondition = func(condition *capmodel.ConfigurationConditionDeclaration) {
		if condition == nil {
			return
		}
		if condition.Field != "" {
			condition.Field = identities[condition.Field]
		}
		for _, child := range condition.Children {
			renameCondition(child)
		}
	}
	renameCondition(rule.When)
	for _, action := range rule.Actions {
		if action != nil {
			action.Field = identities[action.Field]
		}
	}
}

// detachDraftCommercialConfiguration runs inside Bind/Unbind's existing draft
// lock and transaction, before deleting the referenced binding. A later failure
// (including audit) rolls back the complete definition and all price aliases.
func (s *serviceImpl) detachDraftCommercialConfiguration(ctx context.Context, versionNo string, actor int, definition *detachedOfferDefinition) error {
	authorization, err := s.authorizer.AuthorizeVersionConfiguration(ctx, versionNo)
	if err != nil || requireAuthorization(authorization) != nil || authorization.ActorUserID != actor {
		return bizerr.NewCode(catalog.CodeManagementNotAccessible)
	}
	var row *entity.CatalogProductVersionConfiguration
	if err = s.database.Model(do.CatalogProductVersionConfiguration{}).Ctx(ctx).
		Where(do.CatalogProductVersionConfiguration{TenantId: 0, VersionNo: versionNo}).LockUpdate().Scan(&row); err != nil {
		return err
	}
	if row == nil || row.InterfaceInstanceNo == "" {
		return nil
	}
	if definition == nil || definition.configurationVersion != row.Version || definition.interfaceInstanceNo != row.InterfaceInstanceNo {
		return bizerr.NewCode(catalog.CodeManagementConflict)
	}
	var productJSON, customerJSON, presentationJSON []byte
	productJSON, err = configurationDeclarationJSON(emptyOfferSchema())
	if err != nil {
		return gerror.Wrap(err, "encode detached product declaration")
	}
	customerJSON, err = configurationDeclarationJSON(definition.schema)
	if err != nil {
		return gerror.Wrap(err, "encode detached customer declaration")
	}
	presentationJSON, err = json.Marshal(definition.presentations)
	if err != nil {
		return gerror.Wrap(err, "encode detached commercial presentation")
	}
	result, err := s.database.Model(do.CatalogProductVersionConfiguration{}).Ctx(ctx).
		Where(do.CatalogProductVersionConfiguration{TenantId: 0, VersionNo: versionNo, Version: row.Version}).
		Data(do.CatalogProductVersionConfiguration{
			InterfaceInstanceNo: gdb.Raw("NULL"), InterfaceConfigurationRevision: gdb.Raw("NULL"),
			ProductSchemaVersion: "v1", ProductDeclarationSnapshot: string(productJSON), MerchantValueSnapshot: "{}",
			CustomerSchemaVersion: "v1", CustomerDeclarationSnapshot: string(customerJSON), CustomerPresentationSnapshot: string(presentationJSON),
			DeclarationDigest: configurationDigest(append(append([]byte(nil), productJSON...), customerJSON...)), MerchantValueDigest: configurationDigest([]byte("{}")),
			Version: row.Version + 1, ChangedBy: actor,
		}).Update()
	if err != nil {
		return err
	}
	if count, countErr := result.RowsAffected(); countErr != nil || count != 1 {
		return bizerr.NewCode(catalog.CodeManagementConflict)
	}
	return s.renameDetachedPriceFields(ctx, versionNo, definition.identities)
}

func (s *serviceImpl) prepareEditableParameterBindings(ctx context.Context, input SaveVersionConfigurationInput, exact *capmodel.BusinessExtensionProviderEvidence) (SaveVersionConfigurationInput, error) {
	selections, err := s.restoreDetachedMappingState(ctx, input, true)
	if err != nil {
		return input, err
	}
	input.CustomerFieldSelections = selections
	input.MerchantValues, err = boundMerchantValues(exact, input.MerchantValues, selections)
	if err != nil {
		return input, bizerr.WrapCode(err, catalog.CodeManagementInvalid)
	}
	return input, nil
}

// Only field aliases change. Dimension IDs, rates, tiers and currency facts
// stay put; the WHERE clause confines this one batch to the locked draft.
func (s *serviceImpl) renameDetachedPriceFields(ctx context.Context, versionNo string, identities map[string]string) error {
	keys := make([]string, 0, len(identities))
	for before, after := range identities {
		if before != after {
			keys = append(keys, before)
		}
	}
	if len(keys) == 0 {
		return nil
	}
	sort.Strings(keys)
	var (
		clauses      = make([]string, 0, len(keys))
		placeholders = make([]string, 0, len(keys))
		arguments    = make([]any, 0, len(keys)*3+1)
	)
	for _, key := range keys {
		clauses = append(clauses, "WHEN ? THEN ?")
		arguments = append(arguments, key, identities[key])
		placeholders = append(placeholders, "?")
	}
	arguments = append(arguments, versionNo)
	for _, key := range keys {
		arguments = append(arguments, key)
	}
	_, err := s.database.Exec(ctx, `UPDATE plugin_idc_core_catalog_configuration_price_dimension SET field_key=CASE field_key `+strings.Join(clauses, " ")+` ELSE field_key END WHERE tenant_id=0 AND version_no=? AND field_key IN (`+strings.Join(placeholders, ",")+`)`, arguments...)
	return err
}

// Pending state is read from the locked-revision identity, never accepted from
// a write DTO. A newly chosen parameter or explicit information-only choice
// resolves it; an unchanged blank selection does not silently resolve it.
func (s *serviceImpl) restoreDetachedMappingState(ctx context.Context, input SaveVersionConfigurationInput, bound bool) ([]CustomerFieldSelection, error) {
	row, err := s.loadVersionConfigurationRow(ctx, input.VersionNo)
	if err != nil {
		return nil, bizerr.WrapCode(err, catalog.CodeManagementUnavailable)
	}
	result := append([]CustomerFieldSelection(nil), input.CustomerFieldSelections...)
	for _, selection := range result {
		if selection.InformationOnly && (selection.ProductFieldKey != "" || selection.LocalField == nil || !strings.HasPrefix(selection.FieldKey, localCustomerFieldPrefix)) {
			return nil, bizerr.NewCode(catalog.CodeManagementInvalid)
		}
	}
	if row == nil {
		return result, nil
	}
	if row.Version != input.ExpectedVersion {
		return nil, bizerr.NewCode(catalog.CodeManagementConflict)
	}
	presentations, err := configurationcontract.DecodePresentations(row.CustomerPresentationSnapshot)
	if err != nil {
		return nil, bizerr.WrapCode(err, catalog.CodeManagementUnavailable)
	}
	for index, selection := range result {
		pending := presentations[selection.FieldKey].AutomationMappingPending
		resolved := selection.InformationOnly || selection.ProductFieldKey != ""
		if bound && pending && !resolved {
			return nil, bizerr.NewCode(codeOfferMappingRequired)
		}
		result[index].AutomationMappingPending = pending && !resolved
		result[index].BasePriceIncluded = presentations[selection.FieldKey].BasePriceIncluded
		result[index].FixedPriceIncluded = presentations[selection.FieldKey].FixedPriceIncluded
	}
	return result, nil
}

// Ordinary manual edits keep conditions for retained fields; removing a field
// also removes rules that reference it. No client-supplied rule is accepted.
func (s *serviceImpl) retainManualConditions(ctx context.Context, versionNo string, schema *capmodel.ConfigurationSchemaDeclaration) error {
	row, err := s.loadVersionConfigurationRow(ctx, versionNo)
	if err != nil || row == nil {
		return err
	}
	previous, err := decodeConfigurationDeclaration(row.CustomerDeclarationSnapshot)
	if err != nil || previous == nil {
		return bizerr.NewCode(catalog.CodeManagementConflict)
	}
	fields := make(map[string]bool, len(schema.Fields))
	for _, field := range schema.Fields {
		fields[field.Key] = true
	}
	for _, rule := range previous.Conditions {
		if rule == nil || !customerConditionEnabled(rule.When, fields) {
			continue
		}
		actions := make([]*capmodel.ConfigurationActionDeclaration, 0, len(rule.Actions))
		for _, action := range rule.Actions {
			if action != nil && fields[action.Field] {
				actions = append(actions, action)
			}
		}
		if len(actions) > 0 {
			rule.Actions = actions
			schema.Conditions = append(schema.Conditions, rule)
		}
	}
	if _, err = configurationcontract.ValidateSchema(schema, configurationSurfaceCustomer, nil); err != nil {
		return bizerr.WrapCode(err, catalog.CodeManagementInvalid)
	}
	return nil
}
