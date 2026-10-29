// Explicit parameter mappings turn supplier directory relationships into
// product-owned choice restrictions. No cloud category or supplier field name
// is recognized here; the persisted rules reuse the common form compiler.
package productautomation

import (
	"context"
	"encoding/json"
	"maps"

	"github.com/gogf/gf/v2/errors/gerror"
	"lina-core/pkg/bizerr"
	"lina-core/pkg/plugin/capability/capmodel"
	"lina-plugin-idc-core/backend/cap/resourceinterfaceprojectioncap"
	"lina-plugin-idc-core/backend/internal/service/catalog"
	"lina-plugin-idc-core/backend/internal/service/configurationcontract"
)

// Freeze owner-derived choice restrictions with the same commercial snapshot
// and optimistic revision as controls and pricing. No remote call occurs here.
func (s *serviceImpl) compileCommercialPresentations(ctx context.Context, input SaveVersionConfigurationInput, exact *capmodel.BusinessExtensionProviderEvidence, schema *capmodel.ConfigurationSchemaDeclaration, merchant json.RawMessage, relationships []resourceinterfaceprojectioncap.ChoiceRelationship) (string, error) {
	selections, err := s.retainSavedChoiceRules(ctx, input.VersionNo, input.ExpectedVersion, schema, input.CustomerFieldSelections, relationships)
	if err != nil {
		return "", err
	}
	selections, err = commercialChoiceSelections(exact, schema, selections, merchant, relationships)
	if err != nil {
		return "", bizerr.WrapCode(err, catalog.CodeManagementInvalid)
	}
	presentation, err := compileCustomerPresentations(schema, selections)
	if err != nil {
		return "", bizerr.WrapCode(err, catalog.CodeManagementInvalid)
	}
	return presentation, nil
}

func commercialChoiceSelections(exact *capmodel.BusinessExtensionProviderEvidence, schema *capmodel.ConfigurationSchemaDeclaration, selections []CustomerFieldSelection, merchant json.RawMessage, relationships []resourceinterfaceprojectioncap.ChoiceRelationship) ([]CustomerFieldSelection, error) {
	result := append([]CustomerFieldSelection(nil), selections...)
	if len(relationships) == 0 || schema == nil {
		return result, nil
	}
	var (
		fields         = make(map[string]*capmodel.ConfigurationFieldDeclaration, len(schema.Fields))
		byParameter    = make(map[string]string)
		indices        = make(map[string]int, len(result))
		choiceBindings = make(map[string]map[string]string, len(result))
		base           map[string]json.RawMessage
	)
	if json.Unmarshal(merchant, &base) != nil || base == nil {
		return nil, gerror.New("commercial choice base parameters are unavailable")
	}
	for _, field := range schema.Fields {
		if field != nil {
			fields[field.Key] = field
		}
	}
	if exact.CustomerConfiguration != nil {
		for _, field := range exact.CustomerConfiguration.Fields {
			if field != nil && fields[field.Key] != nil {
				byParameter[field.Key] = field.Key
			}
		}
	}
	for key, parameter := range customerProductFields(exact) {
		if fields[key] != nil {
			byParameter[parameter] = key
		}
	}
	for index, selection := range result {
		choiceBindings[selection.FieldKey] = selection.ProductChoiceBindings
		indices[selection.FieldKey] = index
		if selection.ProductFieldKey != "" {
			byParameter[selection.ProductFieldKey] = selection.FieldKey
		}
	}
	// Replace only rules for relationships actually observed in this directory;
	// a retained saved definition remains available during a supplier outage.
	replaced := make(map[string]bool)
	for _, relationship := range relationships {
		child := fields[byParameter[relationship.ChildFieldKey]]
		if child == nil {
			continue
		}
		if child.Type != capmodel.ConfigurationFieldSingleSelect || len(child.StaticCandidates) == 0 {
			return nil, gerror.New("related commercial choices require a bounded select list")
		}
		parent := fields[byParameter[relationship.ParentFieldKey]]
		if parent == nil {
			if err := validateFixedParentChoices(base[relationship.ParentFieldKey], child, relationship, choiceBindings[child.Key]); err != nil {
				return nil, err
			}
			continue
		}
		if parent.Type != capmodel.ConfigurationFieldSingleSelect || len(parent.StaticCandidates) == 0 {
			return nil, gerror.New("related parent choices require a bounded select list")
		}
		index, exists := indices[child.Key]
		if !exists {
			index = len(result)
			indices[child.Key] = index
			result = append(result, CustomerFieldSelection{FieldKey: child.Key})
		}
		if !replaced[child.Key] {
			result[index].ChoiceRules = nil
			replaced[child.Key] = true
		}
		for _, choice := range parent.StaticCandidates {
			allowed := allowedRelatedChoices(configurationcontract.ProductChoiceValue(choiceBindings[parent.Key], choice.Value), child, relationship, choiceBindings[child.Key])
			result[index].ChoiceRules = append(result[index].ChoiceRules, &capmodel.ConfigurationRuleDeclaration{
				When:    &capmodel.ConfigurationConditionDeclaration{Field: parent.Key, Operator: capmodel.ConfigurationConditionEqual, Value: choice.Value},
				Actions: []*capmodel.ConfigurationActionDeclaration{{Action: capmodel.ConfigurationActionFilterCandidates, Field: child.Key, Values: allowed}},
			})
		}
	}
	return result, nil
}

func allowedRelatedChoices(parent string, child *capmodel.ConfigurationFieldDeclaration, relationship resourceinterfaceprojectioncap.ChoiceRelationship, bindings ...map[string]string) []string {
	var aliases map[string]string
	if len(bindings) > 0 {
		aliases = bindings[0]
	}
	allowed := make(map[string]bool)
	for _, link := range relationship.Links {
		if link.ParentValue == parent {
			allowed[link.ChildValue] = true
		}
	}
	result := make([]string, 0)
	// Merchant order and labels remain authoritative; only eligibility narrows.
	for _, choice := range child.StaticCandidates {
		if allowed[configurationcontract.ProductChoiceValue(aliases, choice.Value)] {
			result = append(result, choice.Value)
		}
	}
	return result
}

func validateFixedParentChoices(raw json.RawMessage, child *capmodel.ConfigurationFieldDeclaration, relationship resourceinterfaceprojectioncap.ChoiceRelationship, bindings ...map[string]string) error {
	var value string
	if json.Unmarshal(raw, &value) != nil {
		return gerror.New("fixed relationship parent is unavailable")
	}
	if len(allowedRelatedChoices(value, child, relationship, bindings...)) != len(child.StaticCandidates) {
		return gerror.New("commercial choices conflict with the fixed interface parameter")
	}
	return nil
}

// A form DTO cannot erase owner-generated restrictions by omitting them.
// Preserve only the same commercial fields and exact parameter mappings;
// explicit field removal or remapping must not reuse the old supplier meaning.
func (s *serviceImpl) retainSavedChoiceRules(ctx context.Context, versionNo string, expected int64, schema *capmodel.ConfigurationSchemaDeclaration, selections []CustomerFieldSelection, observed ...[]resourceinterfaceprojectioncap.ChoiceRelationship) ([]CustomerFieldSelection, error) {
	if schema == nil {
		return selections, nil
	}
	row, err := s.loadVersionConfigurationRow(ctx, versionNo)
	if err != nil {
		return nil, bizerr.WrapCode(err, catalog.CodeManagementUnavailable)
	}
	if row == nil {
		return selections, nil
	}
	if row.Version != expected {
		return nil, bizerr.NewCode(catalog.CodeManagementConflict)
	}
	previous, err := configurationcontract.DecodePresentations(row.CustomerPresentationSnapshot)
	if err != nil {
		return nil, bizerr.WrapCode(err, catalog.CodeManagementUnavailable)
	}
	var (
		fields   = make(map[string]bool, len(schema.Fields))
		bindings = make(map[string]configurationcontract.Presentation, len(selections))
		result   = append([]CustomerFieldSelection(nil), selections...)
	)
	for _, field := range schema.Fields {
		fields[field.Key] = true
	}
	for _, selection := range result {
		bindings[selection.FieldKey] = configurationcontract.Presentation{ProductFieldKey: selection.ProductFieldKey, ProductChoiceBindings: selection.ProductChoiceBindings}
	}
	for index, selection := range result {
		result[index].ChoiceRules = nil
		before := previous[selection.FieldKey]
		if selection.LocalField != nil && selection.LocalField.Type != capmodel.ConfigurationFieldSingleSelect {
			continue
		}
		for _, rule := range before.ChoiceRules {
			if rule == nil {
				continue
			}
			same := sameProductChoiceMapping(before, bindings[selection.FieldKey]) && sameChoiceRuleBindings(rule.When, fields, bindings, previous)
			if !same && choiceRuleAliasesChanged(rule.When, selection.FieldKey, bindings, previous) && conditionFieldsRetained(rule.When, fields) && !observedChoiceRule(rule.When, selection.FieldKey, bindings, observed) {
				return nil, bizerr.NewCode(codeChoiceRelationsRequired)
			}
			if same {
				result[index].ChoiceRules = append(result[index].ChoiceRules, rule)
			}
		}
	}
	return result, nil
}

func sameChoiceRuleBindings(condition *capmodel.ConfigurationConditionDeclaration, fields map[string]bool, bindings map[string]configurationcontract.Presentation, previous map[string]configurationcontract.Presentation) bool {
	if condition == nil {
		return false
	}
	if condition.Field != "" && (!fields[condition.Field] || !sameProductChoiceMapping(bindings[condition.Field], previous[condition.Field])) {
		return false
	}
	for _, child := range condition.Children {
		if !sameChoiceRuleBindings(child, fields, bindings, previous) {
			return false
		}
	}
	return true
}

func sameProductChoiceMapping(before, after configurationcontract.Presentation) bool {
	return before.ProductFieldKey == after.ProductFieldKey && maps.Equal(before.ProductChoiceBindings, after.ProductChoiceBindings)
}

func choiceRuleAliasesChanged(condition *capmodel.ConfigurationConditionDeclaration, child string, current, previous map[string]configurationcontract.Presentation) bool {
	before, after := previous[child], current[child]
	// Removing a mapping or explicitly choosing a different parameter retains
	// its existing semantics; this guard concerns aliases within one binding.
	if after.ProductFieldKey == "" || before.ProductFieldKey != after.ProductFieldKey {
		return false
	}
	if !maps.Equal(before.ProductChoiceBindings, after.ProductChoiceBindings) {
		return true
	}
	if condition == nil {
		return false
	}
	if condition.Field != "" {
		before, after = previous[condition.Field], current[condition.Field]
		return after.ProductFieldKey != "" && before.ProductFieldKey == after.ProductFieldKey && !maps.Equal(before.ProductChoiceBindings, after.ProductChoiceBindings)
	}
	for _, parent := range condition.Children {
		if choiceRuleAliasesChanged(parent, child, current, previous) {
			return true
		}
	}
	return false
}

func conditionFieldsRetained(condition *capmodel.ConfigurationConditionDeclaration, fields map[string]bool) bool {
	if condition == nil || (condition.Field != "" && !fields[condition.Field]) {
		return false
	}
	for _, child := range condition.Children {
		if !conditionFieldsRetained(child, fields) {
			return false
		}
	}
	return true
}

// A changed mapping may rebuild an old restriction only from a currently
// available relationship. Missing evidence cannot silently widen the sale.
func observedChoiceRule(condition *capmodel.ConfigurationConditionDeclaration, child string, current map[string]configurationcontract.Presentation, observed [][]resourceinterfaceprojectioncap.ChoiceRelationship) bool {
	if condition == nil || len(observed) == 0 {
		return false
	}
	if condition.Field != "" {
		parentParameter, childParameter := current[condition.Field].ProductFieldKey, current[child].ProductFieldKey
		if parentParameter == "" {
			parentParameter = condition.Field
		}
		if childParameter == "" {
			childParameter = child
		}
		for _, relationship := range observed[0] {
			if relationship.ParentFieldKey == parentParameter && relationship.ChildFieldKey == childParameter {
				return true
			}
		}
		return false
	}
	for _, parent := range condition.Children {
		if !observedChoiceRule(parent, child, current, observed) {
			return false
		}
	}
	return len(condition.Children) > 0
}
