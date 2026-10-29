// This file narrows provider customer configuration to explicitly enabled merchant
// fields. Frozen declarations keep the original provider version; historical
// releases without this opt-in contract remain exact and unchanged.

package productautomation

import (
	"context"
	"encoding/json"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/gogf/gf/v2/errors/gerror"
	"lina-core/pkg/bizerr"
	"lina-core/pkg/plugin/capability/capmodel"
	"lina-plugin-idc-core/backend/cap/productfulfillmentcap"
	"lina-plugin-idc-core/backend/internal/service/catalog"
	"lina-plugin-idc-core/backend/internal/service/configurationcontract"
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
				if field.Required || (field.Type != capmodel.ConfigurationFieldInteger && field.Type != capmodel.ConfigurationFieldSingleLine && (field.Type != capmodel.ConfigurationFieldSingleSelect || field.DynamicCandidates != nil)) {
					return nil, gerror.New("merchant customer field must be an optional integer, bounded text or static single choice")
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

func selectProviderCustomerSchema(exact *capmodel.BusinessExtensionProviderEvidence, selections []CustomerFieldSelection) (*capmodel.ConfigurationSchemaDeclaration, error) {
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
	orders := make(map[string]int, len(selections))
	for _, selection := range selections {
		if !controlled[selection.FieldKey] {
			return nil, gerror.New("unknown customer field selection")
		}
		if _, exists := selected[selection.FieldKey]; exists {
			return nil, gerror.New("duplicate customer field selection")
		}
		if selection.SortOrder < 0 || selection.SortOrder > 1024 || utf8.RuneCountInString(selection.DisplayNameZhCN) > 128 || utf8.RuneCountInString(selection.DisplayNameEnUS) > 128 {
			return nil, gerror.New("invalid merchant field presentation")
		}
		if (strings.TrimSpace(selection.DisplayNameZhCN) == "") != (strings.TrimSpace(selection.DisplayNameEnUS) == "") {
			return nil, gerror.New("both merchant field display languages are required")
		}
		orders[selection.FieldKey] = selection.SortOrder
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
			if selection.DisplayNameZhCN != "" {
				field.Label = &capmodel.LocalizedText{ZhCN: strings.TrimSpace(selection.DisplayNameZhCN), EnUS: strings.TrimSpace(selection.DisplayNameEnUS)}
			}
			delete(selected, field.Key)
		}
		fields = append(fields, field)
		enabled[field.Key] = true
	}
	if len(selected) > 0 {
		return nil, gerror.New("declared customer field is missing")
	}
	sort.SliceStable(fields, func(i, j int) bool {
		left, leftCustom := orders[fields[i].Key]
		right, rightCustom := orders[fields[j].Key]
		if leftCustom != rightCustom {
			return !leftCustom
		}
		return left < right
	})
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
	case capmodel.ConfigurationFieldSingleLine:
		if selection.Minimum != "" || selection.Maximum != "" || len(selection.CandidateValues) > 0 || field.MaximumLength < 1 {
			return gerror.New("text choices retain their declared length and do not accept quantity bounds")
		}
	case capmodel.ConfigurationFieldSingleSelect:
		if field.DynamicCandidates != nil || len(selection.CandidateValues) == 0 {
			return gerror.New("explicit declared candidates are required")
		}
		return merchantChoiceSubset(field, selection)
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
	var providerFields []*capmodel.ConfigurationFieldDeclaration
	if exact.CustomerConfiguration != nil {
		providerFields = exact.CustomerConfiguration.Fields
	}
	if frozen != nil {
		for index, field := range frozen.Fields {
			renamed, customizedChoices := false, false
			for _, original := range providerFields {
				if field != nil && original != nil && field.Key == original.Key && field.Label != nil && original.Label != nil {
					renamed = *field.Label != *original.Label
					customizedChoices = original.Type == capmodel.ConfigurationFieldSingleSelect && !reflect.DeepEqual(field.StaticCandidates, original.StaticCandidates)
				}
			}
			if field != nil && (controlled[field.Key] || strings.HasPrefix(field.Key, localCustomerFieldPrefix) || renamed || customizedChoices) {
				selection := CustomerFieldSelection{FieldKey: field.Key, Minimum: field.Minimum, Maximum: field.Maximum, SortOrder: index}
				if strings.HasPrefix(field.Key, localCustomerFieldPrefix) {
					selection.LocalField = field
				}
				if field.Label != nil {
					selection.DisplayNameZhCN = field.Label.ZhCN
					selection.DisplayNameEnUS = field.Label.EnUS
				}
				if controlled[field.Key] || customizedChoices {
					selection.CandidateLabels = map[string]*capmodel.LocalizedText{}
					for _, candidate := range field.StaticCandidates {
						selection.CandidateValues = append(selection.CandidateValues, candidate.Value)
						selection.CandidateLabels[candidate.Value] = candidate.Label
					}
				}
				selections = append(selections, selection)
			}
		}
	}
	expected, err := selectCustomerSchema(exact, selections)
	if err != nil || !sameCompatibleProductConfiguration(frozen, expected) {
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

// configurableCustomerFields returns optional fields and retained core fields for local presentation.
func configurableCustomerFields(exact *capmodel.BusinessExtensionProviderEvidence) []*capmodel.ConfigurationFieldDeclaration {
	controlled, err := merchantCustomerFields(exact)
	if err != nil || exact.CustomerConfiguration == nil {
		return nil
	}
	result := make([]*capmodel.ConfigurationFieldDeclaration, 0, len(exact.CustomerConfiguration.Fields))
	for _, field := range exact.CustomerConfiguration.Fields {
		if field != nil && (controlled[field.Key] || field.Required) {
			result = append(result, field)
		}
	}
	return result
}

// validateCustomerSelectionPrices prevents an enabled option from silently
// falling back to the base product amount. Zero prices must be explicit rows.
func (s *serviceImpl) validateCustomerSelectionPrices(ctx context.Context, versionNo string, exact *capmodel.BusinessExtensionProviderEvidence, schema *capmodel.ConfigurationSchemaDeclaration, presentationJSON ...string) error {
	controlled := map[string]bool{}
	var err error
	if exact != nil {
		controlled, err = merchantCustomerFields(exact)
		if err != nil {
			return err
		}
	}
	if schema == nil {
		return nil
	}
	for _, field := range schema.Fields {
		if strings.HasPrefix(field.Key, localCustomerFieldPrefix) {
			controlled[field.Key] = true
		}
	}
	var selections []CustomerFieldSelection
	if len(presentationJSON) > 0 {
		selections, err = withCustomerPresentations(nil, presentationJSON[0])
		if err != nil {
			return err
		}
	}
	for _, selection := range selections {
		if selection.BasePriceIncluded {
			delete(controlled, selection.FieldKey)
		}
		if selection.ProductFieldKey == "" && !selection.FixedPriceIncluded {
			continue
		}
		for _, field := range schema.Fields {
			if field.Key == selection.FieldKey && ((field.Type == capmodel.ConfigurationFieldInteger && field.Minimum == field.Maximum) || (field.Type == capmodel.ConfigurationFieldSingleSelect && len(field.StaticCandidates) == 1)) {
				delete(controlled, field.Key)
			}
		}
	}
	candidates, err := configurationPriceCandidates(versionNo, &VersionConfiguration{Version: 1, CustomerDeclaration: schema, CustomerFieldSelections: selections}, s.booleanPriceLabels)
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
		return bizerr.NewCode(catalog.CodeManagementConflict)
	}
	for base := range bases {
		for _, candidate := range required {
			if !coverage[base+"\x00"+candidate.fieldKey+"\x00"+candidate.candidateValue] {
				return bizerr.NewCode(catalog.CodeManagementConflict)
			}
		}
	}
	return nil
}

// withCustomerPresentations restores local widgets alongside frozen provider values.
func withCustomerPresentations(selections []CustomerFieldSelection, raw string) ([]CustomerFieldSelection, error) {
	presentations, err := configurationcontract.DecodePresentations(raw)
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	for _, selection := range selections {
		seen[selection.FieldKey] = true
	}
	keys := make([]string, 0, len(presentations))
	for key := range presentations {
		if !seen[key] {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	for _, key := range keys {
		selections = append(selections, CustomerFieldSelection{FieldKey: key})
	}
	for i := range selections {
		p := presentations[selections[i].FieldKey]
		if _, exists := presentations[selections[i].FieldKey]; exists {
			selections[i].SortOrder = p.SortOrder
		}
		selections[i].CustomerVisible = p.CustomerVisible
		selections[i].DefaultValue = p.DefaultValue
		selections[i].UpgradeEnabled = p.UpgradeEnabled
		selections[i].UpgradeMinimum = p.UpgradeMinimum
		selections[i].UpgradeMaximum = p.UpgradeMaximum
		selections[i].UpgradeCandidateValues = p.UpgradeCandidateValues
		selections[i].ProductFieldKey = p.ProductFieldKey
		selections[i].ProductChoiceBindings = p.ProductChoiceBindings
		selections[i].AutomationMappingPending = p.AutomationMappingPending
		selections[i].BasePriceIncluded = p.BasePriceIncluded
		selections[i].FixedPriceIncluded = p.FixedPriceIncluded
		selections[i].ChoiceRules = p.ChoiceRules
		selections[i].NumericPricing = p.NumericPricing
		selections[i].DisplayControl = p.Control
		selections[i].QuantityStep = p.Step
		selections[i].NumericChoices = p.NumericChoices
		selections[i].GroupZhCN = p.GroupZhCN
		selections[i].GroupEnUS = p.GroupEnUS
		selections[i].Unit = p.Unit
		selections[i].DescriptionZhCN = p.DescriptionZhCN
		selections[i].DescriptionEnUS = p.DescriptionEnUS
		selections[i].Advanced = p.Advanced
		selections[i].NumericLabels = p.NumericLabels
	}
	return selections, nil
}

func compileCustomerPresentations(schema *capmodel.ConfigurationSchemaDeclaration, selections []CustomerFieldSelection) (string, error) {
	presentations := make(map[string]configurationcontract.Presentation)
	for _, selection := range selections {
		presentations[selection.FieldKey] = configurationcontract.Presentation{CustomerVisible: selection.CustomerVisible, DefaultValue: selection.DefaultValue, UpgradeEnabled: selection.UpgradeEnabled, UpgradeMinimum: selection.UpgradeMinimum, UpgradeMaximum: selection.UpgradeMaximum, UpgradeCandidateValues: selection.UpgradeCandidateValues, FixedPriceIncluded: selection.FixedPriceIncluded, BasePriceIncluded: selection.BasePriceIncluded, AutomationMappingPending: selection.AutomationMappingPending, ChoiceRules: selection.ChoiceRules, ProductFieldKey: selection.ProductFieldKey, ProductChoiceBindings: selection.ProductChoiceBindings, SortOrder: selection.SortOrder, NumericPricing: selection.NumericPricing, Control: selection.DisplayControl, Step: selection.QuantityStep, NumericChoices: selection.NumericChoices, GroupZhCN: selection.GroupZhCN, GroupEnUS: selection.GroupEnUS, Unit: selection.Unit, DescriptionZhCN: selection.DescriptionZhCN, DescriptionEnUS: selection.DescriptionEnUS, Advanced: selection.Advanced, NumericLabels: selection.NumericLabels}
	}
	if err := configurationcontract.ValidatePresentations(schema, presentations); err != nil {
		return "", err
	}
	encoded, err := json.Marshal(presentations)
	if err != nil {
		return "", err
	}
	if len(encoded) > 131072 {
		return "", gerror.New("purchase presentation exceeds its bound")
	}
	return string(encoded), nil
}
