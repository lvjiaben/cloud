// This file maps internal automation models to minimal administrator-facing API projections.

package admin

import (
	"strconv"

	"lina-core/pkg/plugin/capability/capmodel"
	"lina-plugin-idc-core/backend/api/admin/v1"
	"lina-plugin-idc-core/backend/cap/resourceinterfaceprojectioncap"
	"lina-plugin-idc-core/backend/internal/service/productautomation"
)

func mapAutomationInterface(item *productautomation.InterfaceInstance) *v1.CatalogAutomationInterfaceItem {
	if item == nil {
		return nil
	}
	return &v1.CatalogAutomationInterfaceItem{CatalogAvailable: item.CatalogAvailable, InterfaceInstanceNo: item.InterfaceInstanceNo, DisplayName: item.DisplayName, NewBusinessState: item.NewBusinessState, Version: item.Version, CreatedAt: item.CreatedAt.UnixMilli(), ChangedAt: item.ChangedAt.UnixMilli(), CanDelete: item.CanDelete}
}

func mapAutomationBinding(item *productautomation.Binding) *v1.CatalogFulfillmentBindingSummary {
	if item == nil {
		return nil
	}
	return &v1.CatalogFulfillmentBindingSummary{CatalogAvailable: item.CatalogAvailable, InterfaceInstanceNo: item.InterfaceInstanceNo, InterfaceDisplayName: item.InterfaceDisplayName, BoundAt: item.BoundAt.UnixMilli(), EvidenceVerified: item.EvidenceVerified, InterfaceNewBusinessState: item.InterfaceNewBusinessState}
}

func mapConfigurationValues(values []productautomation.ConfigurationValue) []*v1.CatalogStructuredConfigurationValue {
	result := make([]*v1.CatalogStructuredConfigurationValue, 0, len(values))
	for _, value := range values {
		mapped := &v1.CatalogStructuredConfigurationValue{
			FieldKey: value.FieldKey, TextValue: value.TextValue,
			DecimalValue: value.DecimalValue, BooleanValue: value.BooleanValue, DateValue: value.DateValue,
			DateTimeValue: value.DateTimeValue, OptionTokens: append([]string(nil), value.OptionTokens...),
			TreeOptionTokens: append([]string(nil), value.TreeOptionTokens...), AddressOptionTokens: append([]string(nil), value.AddressOptionTokens...),
			FileTokens: append([]string(nil), value.FileTokens...),
		}
		if value.IntegerValue != nil {
			formatted := strconv.FormatInt(*value.IntegerValue, 10)
			mapped.IntegerValue = &formatted
		}
		result = append(result, mapped)
	}
	return result
}

func mapConfigurationInput(values []*v1.CatalogStructuredConfigurationValue) []productautomation.ConfigurationValue {
	result := make([]productautomation.ConfigurationValue, 0, len(values))
	for _, value := range values {
		if value == nil {
			result = append(result, productautomation.ConfigurationValue{})
			continue
		}
		mapped := productautomation.ConfigurationValue{
			FieldKey: value.FieldKey, TextValue: value.TextValue,
			DecimalValue: value.DecimalValue, BooleanValue: value.BooleanValue, DateValue: value.DateValue,
			DateTimeValue: value.DateTimeValue, OptionTokens: append([]string(nil), value.OptionTokens...),
			TreeOptionTokens: append([]string(nil), value.TreeOptionTokens...), AddressOptionTokens: append([]string(nil), value.AddressOptionTokens...),
			FileTokens: append([]string(nil), value.FileTokens...),
		}
		mapped.IntegerTextValue = value.IntegerValue
		result = append(result, mapped)
	}
	return result
}

func mapInterfaceConfiguration(value *productautomation.InterfaceConfiguration) *v1.CatalogAutomationInterfaceConfiguration {
	if value == nil {
		return nil
	}
	secrets := make([]*v1.CatalogConfigurationSecretStatus, 0, len(value.Secrets))
	for _, secret := range value.Secrets {
		secrets = append(secrets, &v1.CatalogConfigurationSecretStatus{FieldKey: secret.FieldKey, Configured: secret.Configured, MaskedTail: secret.MaskedTail, UpdatedAt: secret.UpdatedAt.UnixMilli()})
	}
	return &v1.CatalogAutomationInterfaceConfiguration{
		Declaration: value.Declaration, Values: mapConfigurationValues(value.Values), Secrets: secrets,
		ResolvedCandidates: mapResolvedCandidates(value.ResolvedCandidates), Revision: value.Revision,
	}
}

func mapProductVersionConfiguration(value *productautomation.VersionConfiguration) *v1.CatalogProductVersionConfiguration {
	if value == nil {
		return nil
	}
	var (
		choiceRules   = make(map[string][]*capmodel.ConfigurationRuleDeclaration)
		pendingFields = make([]string, 0)
	)
	for _, selection := range value.CustomerFieldSelections {
		if selection.AutomationMappingPending {
			pendingFields = append(pendingFields, selection.FieldKey)
		}
		if len(selection.ChoiceRules) > 0 {
			choiceRules[selection.FieldKey] = selection.ChoiceRules
		}
	}
	return &v1.CatalogProductVersionConfiguration{PriceFieldTokens: value.PriceFieldTokens,
		PendingAutomationFields: pendingFields, DirectoryRelationships: mapDirectoryRelationships(value.DirectoryRelationships), CustomerChoiceRules: choiceRules,
		MerchantPriceTargets:  mapMerchantPriceTargets(value.MerchantPriceTargets),
		UpstreamOptionsStatus: string(value.UpstreamOptionsStatus),
		ProductChoiceFields:   value.ProductChoiceFields, CustomerProductFields: value.CustomerProductFields, CustomerConfigurableFields: value.CustomerConfigurableFields, CustomerFieldSelections: mapCustomerFieldSelectionOutput(value.CustomerFieldSelections),
		ImportedPricingMode: value.ImportedPricingMode, ImportedMarkupPercent: value.ImportedMarkupPercent, ImportedManualPrices: mapImportedManualPriceOutput(value.ImportedManualPrices),
		ImportedProductReference: value.ImportedProductReference, ImportedProductName: value.ImportedProductName, ImportedConfiguration: value.ImportedConfiguration,
		ProductDeclaration: value.ProductDeclaration, CustomerDeclaration: value.CustomerDeclaration,
		MerchantValues:             mapConfigurationValues(value.MerchantValues),
		ProductResolvedCandidates:  mapResolvedCandidates(value.ProductResolvedCandidates),
		CustomerResolvedCandidates: mapResolvedCandidates(value.CustomerResolvedCandidates), Version: value.Version,
	}
}

func mapDirectoryRelationships(values []resourceinterfaceprojectioncap.ChoiceRelationship) []*v1.CatalogDirectoryRelationship {
	result := make([]*v1.CatalogDirectoryRelationship, 0, len(values))
	for _, value := range values {
		item := &v1.CatalogDirectoryRelationship{ParentFieldKey: value.ParentFieldKey, ChildFieldKey: value.ChildFieldKey, Links: make([]*v1.CatalogDirectoryChoiceLink, 0, len(value.Links))}
		for _, link := range value.Links {
			item.Links = append(item.Links, &v1.CatalogDirectoryChoiceLink{ParentValue: link.ParentValue, ChildValue: link.ChildValue})
		}
		result = append(result, item)
	}
	return result
}

func mapConfigurationEvaluation(value *productautomation.ConfigurationEvaluation) *v1.CatalogConfigurationEvaluation {
	if value == nil {
		return nil
	}
	states := make(map[string]*v1.CatalogConfigurationFieldState, len(value.States))
	for fieldKey, state := range value.States {
		states[fieldKey] = &v1.CatalogConfigurationFieldState{
			Visible: state.Visible, Required: state.Required, Disabled: state.Disabled,
		}
	}
	issues := make([]*v1.CatalogConfigurationIssue, 0, len(value.Issues))
	for _, issue := range value.Issues {
		issues = append(issues, &v1.CatalogConfigurationIssue{FieldKey: issue.FieldKey, IssueCode: issue.Code})
	}
	return &v1.CatalogConfigurationEvaluation{
		Values: mapConfigurationValues(value.Values), FieldStates: states,
		Candidates: mapResolvedCandidates(value.Candidates), Issues: issues,
		Valid: value.Valid,
	}
}

func mapResolvedCandidates(value map[string][]*capmodel.ConfigurationCandidate) map[string][]*capmodel.ConfigurationCandidate {
	result := make(map[string][]*capmodel.ConfigurationCandidate, len(value))
	for fieldKey, candidates := range value {
		result[fieldKey] = candidates
	}
	return result
}

func mapCustomerFieldSelections(values []*v1.CatalogCustomerFieldSelection) []productautomation.CustomerFieldSelection {
	result := make([]productautomation.CustomerFieldSelection, 0, len(values))
	for _, value := range values {
		if value != nil {
			result = append(result, productautomation.CustomerFieldSelection{CustomerVisible: value.CustomerVisible, DefaultValue: value.DefaultValue, UpgradeEnabled: value.UpgradeEnabled, UpgradeMinimum: value.UpgradeMinimum, UpgradeMaximum: value.UpgradeMaximum, UpgradeCandidateValues: value.UpgradeCandidateValues, InformationOnly: value.InformationOnly, GroupZhCN: value.GroupZhCN, GroupEnUS: value.GroupEnUS, Unit: value.Unit, DescriptionZhCN: value.DescriptionZhCN, DescriptionEnUS: value.DescriptionEnUS, Advanced: value.Advanced, NumericLabels: value.NumericLabels, NumericPricing: value.NumericPricing, ProductFieldKey: value.ProductFieldKey, ProductChoiceBindings: value.ProductChoiceBindings, LocalField: value.LocalField, FieldKey: value.FieldKey, Minimum: value.Minimum, Maximum: value.Maximum, CandidateValues: value.CandidateValues, CandidateLabels: value.CandidateLabels, DisplayNameZhCN: value.DisplayNameZhCN, DisplayNameEnUS: value.DisplayNameEnUS, SortOrder: value.SortOrder, DisplayControl: value.DisplayControl, QuantityStep: value.QuantityStep, NumericChoices: value.NumericChoices})
		}
	}
	return result
}
func mapCustomerFieldSelectionOutput(values []productautomation.CustomerFieldSelection) []*v1.CatalogCustomerFieldSelection {
	result := make([]*v1.CatalogCustomerFieldSelection, 0, len(values))
	for _, value := range values {
		result = append(result, &v1.CatalogCustomerFieldSelection{CustomerVisible: value.CustomerVisible, DefaultValue: value.DefaultValue, UpgradeEnabled: value.UpgradeEnabled, UpgradeMinimum: value.UpgradeMinimum, UpgradeMaximum: value.UpgradeMaximum, UpgradeCandidateValues: value.UpgradeCandidateValues, GroupZhCN: value.GroupZhCN, GroupEnUS: value.GroupEnUS, Unit: value.Unit, DescriptionZhCN: value.DescriptionZhCN, DescriptionEnUS: value.DescriptionEnUS, Advanced: value.Advanced, NumericLabels: value.NumericLabels, NumericPricing: value.NumericPricing, ProductFieldKey: value.ProductFieldKey, ProductChoiceBindings: value.ProductChoiceBindings, LocalField: value.LocalField, FieldKey: value.FieldKey, Minimum: value.Minimum, Maximum: value.Maximum, CandidateValues: value.CandidateValues, CandidateLabels: value.CandidateLabels, DisplayNameZhCN: value.DisplayNameZhCN, DisplayNameEnUS: value.DisplayNameEnUS, SortOrder: value.SortOrder, DisplayControl: value.DisplayControl, QuantityStep: value.QuantityStep, NumericChoices: value.NumericChoices})
	}
	return result
}

func mapImportedManualPriceOutput(rows []resourceinterfaceprojectioncap.CatalogPrice) []v1.CatalogImportPrice {
	result := make([]v1.CatalogImportPrice, len(rows))
	for index, row := range rows {
		result[index] = v1.CatalogImportPrice{CurrencyCode: row.CurrencyCode, Period: string(row.Period), PeriodCount: row.PeriodCount, Amount: row.Amount, SetupFee: row.SetupFee}
	}
	return result
}
