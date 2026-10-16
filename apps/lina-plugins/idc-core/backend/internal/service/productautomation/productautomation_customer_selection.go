// This file narrows provider customer configuration to explicitly enabled merchant
// fields. Frozen declarations keep the original provider version; historical
// releases without this opt-in contract remain exact and unchanged.

package productautomation

import (
	"context"
	"encoding/json"
	"strconv"

	"github.com/gogf/gf/v2/errors/gerror"
	"lina-core/pkg/plugin/capability/capmodel"
	"lina-plugin-idc-core/backend/cap/productfulfillmentcap"
)

func merchantCustomerFields(exact *capmodel.BusinessExtensionProviderEvidence) (map[string]bool, error) {
	declaration, err := productfulfillmentcap.ParseProviderDeclaration(exact.ProviderMetadata, exact.AuthorizedMethods)
	if err != nil {
		return nil, err
	}
	result := make(map[string]bool, len(declaration.MerchantCustomerFields))
	for _, key := range declaration.MerchantCustomerFields {
		result[key] = true
	}
	if len(result) > 0 {
		if exact.CustomerConfiguration == nil {
			return nil, gerror.New("merchant customer fields need a schema")
		}
		found := 0
		for _, field := range exact.CustomerConfiguration.Fields {
			if field != nil && result[field.Key] {
				if field.Required || (field.Type != capmodel.ConfigurationFieldInteger && (field.Type != capmodel.ConfigurationFieldSingleSelect || field.DynamicCandidates != nil)) {
					return nil, gerror.New("merchant customer field must be an optional integer or static single choice")
				}
				found++
			}
		}
		if found != len(result) {
			return nil, gerror.New("merchant customer field is not declared")
		}
	}
	return result, nil
}

func selectCustomerSchema(exact *capmodel.BusinessExtensionProviderEvidence, selections []CustomerFieldSelection) (*capmodel.ConfigurationSchemaDeclaration, error) {
	controlled, err := merchantCustomerFields(exact)
	if err != nil {
		return nil, err
	}
	if len(controlled) == 0 {
		if len(selections) > 0 {
			return nil, gerror.New("customer fields cannot be customized")
		}
		return exact.CustomerConfiguration, nil
	}
	if exact.CustomerConfiguration == nil || len(selections) > len(controlled) {
		return nil, gerror.New("customer configuration is unavailable")
	}
	encoded, err := json.Marshal(exact.CustomerConfiguration)
	if err != nil {
		return nil, err
	}
	var result capmodel.ConfigurationSchemaDeclaration
	if err = json.Unmarshal(encoded, &result); err != nil {
		return nil, err
	}
	selected := make(map[string]CustomerFieldSelection, len(selections))
	for _, selection := range selections {
		if !controlled[selection.FieldKey] {
			return nil, gerror.New("unknown customer field selection")
		}
		if _, exists := selected[selection.FieldKey]; exists {
			return nil, gerror.New("duplicate customer field selection")
		}
		selected[selection.FieldKey] = selection
	}
	fields := make([]*capmodel.ConfigurationFieldDeclaration, 0, len(result.Fields))
	enabled := make(map[string]bool, len(result.Fields))
	for _, field := range result.Fields {
		if field == nil {
			return nil, gerror.New("invalid customer field")
		}
		if controlled[field.Key] {
			selection, exists := selected[field.Key]
			if !exists {
				continue
			}
			if err = narrowCustomerField(field, selection); err != nil {
				return nil, err
			}
			delete(selected, field.Key)
		}
		fields = append(fields, field)
		enabled[field.Key] = true
	}
	if len(selected) > 0 {
		return nil, gerror.New("declared customer field is missing")
	}
	result.Fields = fields
	rules := make([]*capmodel.ConfigurationRuleDeclaration, 0, len(result.Conditions))
	for _, rule := range result.Conditions {
		if rule == nil || !customerConditionEnabled(rule.When, enabled) {
			continue
		}
		actions := make([]*capmodel.ConfigurationActionDeclaration, 0, len(rule.Actions))
		for _, action := range rule.Actions {
			if action != nil && enabled[action.Field] {
				actions = append(actions, action)
			}
		}
		if len(actions) > 0 {
			rule.Actions = actions
			rules = append(rules, rule)
		}
	}
	result.Conditions = rules
	return &result, nil
}

func narrowCustomerField(field *capmodel.ConfigurationFieldDeclaration, selection CustomerFieldSelection) error {
	switch field.Type {
	case capmodel.ConfigurationFieldInteger:
		minimum, err := strconv.ParseInt(selection.Minimum, 10, 64)
		if err != nil {
			return gerror.New("minimum quantity is required")
		}
		maximum, err := strconv.ParseInt(selection.Maximum, 10, 64)
		if err != nil || minimum > maximum {
			return gerror.New("maximum quantity is invalid")
		}
		lower, err := strconv.ParseInt(field.Minimum, 10, 64)
		if err != nil || minimum < lower {
			return gerror.New("minimum exceeds provider technical bounds")
		}
		upper, err := strconv.ParseInt(field.Maximum, 10, 64)
		if err != nil || maximum > upper {
			return gerror.New("maximum exceeds provider technical bounds")
		}
		if len(selection.CandidateValues) > 0 {
			return gerror.New("quantity fields do not accept candidates")
		}
		field.Minimum, field.Maximum = selection.Minimum, selection.Maximum
	case capmodel.ConfigurationFieldSingleSelect:
		if field.DynamicCandidates != nil || len(selection.CandidateValues) == 0 || selection.Minimum != "" || selection.Maximum != "" {
			return gerror.New("explicit declared candidates are required")
		}
		candidates := make([]*capmodel.ConfigurationCandidate, 0, len(selection.CandidateValues))
		values := make(map[string]bool, len(selection.CandidateValues))
		for _, value := range selection.CandidateValues {
			if values[value] {
				return gerror.New("duplicate candidate")
			}
			values[value] = true
		}
		for _, candidate := range field.StaticCandidates {
			if values[candidate.Value] {
				candidates = append(candidates, candidate)
				delete(values, candidate.Value)
			}
		}
		if len(values) > 0 {
			return gerror.New("unknown customer candidate")
		}
		field.StaticCandidates = candidates
	default:
		return gerror.New("unsupported merchant customer field type")
	}
	return nil
}

func customerConditionEnabled(condition *capmodel.ConfigurationConditionDeclaration, enabled map[string]bool) bool {
	if condition == nil {
		return false
	}
	if condition.Field != "" && !enabled[condition.Field] {
		return false
	}
	for _, child := range condition.Children {
		if !customerConditionEnabled(child, enabled) {
			return false
		}
	}
	return true
}

func frozenCustomerSelections(exact *capmodel.BusinessExtensionProviderEvidence, frozen *capmodel.ConfigurationSchemaDeclaration) ([]CustomerFieldSelection, error) {
	controlled, err := merchantCustomerFields(exact)
	if err != nil {
		return nil, err
	}
	selections := make([]CustomerFieldSelection, 0)
	if frozen != nil {
		for _, field := range frozen.Fields {
			if controlled[field.Key] {
				selection := CustomerFieldSelection{FieldKey: field.Key, Minimum: field.Minimum, Maximum: field.Maximum}
				for _, candidate := range field.StaticCandidates {
					selection.CandidateValues = append(selection.CandidateValues, candidate.Value)
				}
				selections = append(selections, selection)
			}
		}
	}
	expected, err := selectCustomerSchema(exact, selections)
	if err != nil || !sameExactConfiguration(expected, frozen) {
		return nil, gerror.New("frozen merchant customer schema does not match provider bounds")
	}
	return selections, nil
}

func verifiedCustomerSchema(exact *capmodel.BusinessExtensionProviderEvidence, snapshot string) (*capmodel.ConfigurationSchemaDeclaration, error) {
	frozen, err := decodeConfigurationDeclaration(snapshot)
	if err != nil {
		return nil, err
	}
	if _, err = frozenCustomerSelections(exact, frozen); err != nil {
		return nil, err
	}
	return frozen, nil
}

// configurableCustomerFields returns only opt-in fields for merchant editing.
func configurableCustomerFields(exact *capmodel.BusinessExtensionProviderEvidence) []*capmodel.ConfigurationFieldDeclaration {
	controlled, err := merchantCustomerFields(exact)
	if err != nil || exact.CustomerConfiguration == nil {
		return nil
	}
	result := make([]*capmodel.ConfigurationFieldDeclaration, 0, len(controlled))
	for _, field := range exact.CustomerConfiguration.Fields {
		if controlled[field.Key] {
			result = append(result, field)
		}
	}
	return result
}

// validateCustomerSelectionPrices prevents an enabled option from silently
// falling back to the base product amount. Zero prices must be explicit rows.
func (s *serviceImpl) validateCustomerSelectionPrices(ctx context.Context, versionNo string, exact *capmodel.BusinessExtensionProviderEvidence, schema *capmodel.ConfigurationSchemaDeclaration) error {
	controlled, err := merchantCustomerFields(exact)
	if err != nil {
		return err
	}
	if len(controlled) == 0 || schema == nil {
		return nil
	}
	candidates, err := configurationPriceCandidates(versionNo, &VersionConfiguration{Version: 1, CustomerDeclaration: schema})
	if err != nil {
		return err
	}
	required := make([]configurationPriceCandidatePrivate, 0)
	for _, candidate := range candidates {
		if controlled[candidate.fieldKey] {
			required = append(required, candidate)
		}
	}
	if len(required) == 0 {
		return nil
	}
	type priceCoverage struct {
		FieldKey       string `orm:"field_key"`
		CandidateValue string `orm:"candidate_value"`
		CurrencyCode   string `orm:"currency_code"`
		BillingRuleNo  string `orm:"billing_rule_no"`
		PriceNo        string `orm:"price_no"`
	}
	var rows []priceCoverage
	err = s.database.GetScan(ctx, &rows, `SELECT d.field_key,d.candidate_value,b.currency_code,b.billing_rule_no,p.price_no
 FROM plugin_idc_core_catalog_price b
 JOIN plugin_idc_core_catalog_billing_rule r ON r.billing_rule_no=b.billing_rule_no AND r.version_no=b.version_no AND r.tenant_id=0 AND r.status_code='active'
 LEFT JOIN plugin_idc_core_catalog_configuration_price_dimension d ON d.version_no=b.version_no AND d.tenant_id=0 AND d.status_code='active'
 LEFT JOIN plugin_idc_core_catalog_price p ON p.version_no=b.version_no AND p.tenant_id=0 AND p.configuration_price_dimension_no=d.dimension_no AND p.currency_code=b.currency_code AND p.billing_rule_no=b.billing_rule_no AND p.status_code='active'
 WHERE b.tenant_id=0 AND b.version_no=? AND b.status_code='active' AND b.option_no IS NULL AND b.configuration_price_dimension_no IS NULL LIMIT ?`, versionNo, maximumConfigurationPriceCandidates*16+1)
	if err != nil {
		return err
	}
	if len(rows) > maximumConfigurationPriceCandidates*16 {
		return gerror.New("configuration price coverage bound exceeded")
	}
	bases := make(map[string]bool)
	coverage := make(map[string]bool)
	for _, row := range rows {
		base := row.CurrencyCode + "\x00" + row.BillingRuleNo
		bases[base] = true
		if row.PriceNo != "" {
			coverage[base+"\x00"+row.FieldKey+"\x00"+row.CandidateValue] = true
		}
	}
	if len(bases) == 0 {
		return gerror.New("enabled customer fields require explicit prices")
	}
	for base := range bases {
		for _, candidate := range required {
			if !coverage[base+"\x00"+candidate.fieldKey+"\x00"+candidate.candidateValue] {
				return gerror.New("enabled customer fields require explicit prices for every sale currency and billing period")
			}
		}
	}
	return nil
}
