// This file explicitly projects bounded opaque configuration data. JSON never
// serializes an internal core object, supplier key, price or credential.
package accountnative

import (
	"lina-plugin-idc-core/backend/cap/servercommercialcap"
	"lina-plugin-idc-resource/backend/api/accountnative/v1"
	"lina-plugin-idc-resource/backend/internal/service/customeroperations"
)

func configurationValues(values []*v1.NativeConfigurationValue) ([]servercommercialcap.TypedPurchaseValue, error) {
	if values == nil {
		return nil, nil
	}
	if len(values) > 100 {
		return nil, customeroperations.ErrInvalid
	}
	result := make([]servercommercialcap.TypedPurchaseValue, 0, len(values))
	for _, value := range values {
		if value == nil {
			return nil, customeroperations.ErrInvalid
		}
		result = append(result, servercommercialcap.TypedPurchaseValue{FieldToken: value.FieldToken, Type: value.Type, TextValue: value.TextValue, IntegerValue: value.IntegerValue, DecimalValue: value.DecimalValue, BooleanValue: value.BooleanValue, DateValue: value.DateValue, DateTimeValue: value.DateTimeValue, OptionTokens: value.OptionTokens, TreeOptionTokens: value.TreeOptionTokens, AddressOptionTokens: value.AddressOptionTokens, FileTokens: value.FileTokens})
	}
	return result, nil
}
func projectConfiguration(item *servercommercialcap.ConfigurationContext) *v1.NativeServerConfigurationContextRes {
	result := &v1.NativeServerConfigurationContextRes{EntitlementNo: item.EntitlementNo, EntitlementVersion: item.EntitlementVersion, StructuredConfigurationNo: item.StructuredConfigurationNo, ConfigurationRevision: item.ConfigurationRevision, CurrencyCode: item.CurrencyCode, PurchaseCycles: item.PurchaseCycles, Ready: item.Ready, BillingTermToken: item.BillingTermToken, Fields: make([]*v1.NativeConfigurationField, 0, len(item.Fields)), Values: make([]*v1.NativeConfigurationValue, 0, len(item.Values)), Issues: make([]*v1.NativeConfigurationIssue, 0, len(item.Issues))}
	for _, field := range item.Fields {
		result.Fields = append(result.Fields, projectConfigurationField(field))
	}
	for _, value := range item.Values {
		result.Values = append(result.Values, &v1.NativeConfigurationValue{FieldToken: value.FieldToken, Type: value.Type, TextValue: value.TextValue, IntegerValue: value.IntegerValue, DecimalValue: value.DecimalValue, BooleanValue: value.BooleanValue, DateValue: value.DateValue, DateTimeValue: value.DateTimeValue, OptionTokens: value.OptionTokens, TreeOptionTokens: value.TreeOptionTokens, AddressOptionTokens: value.AddressOptionTokens, FileTokens: value.FileTokens})
	}
	for _, issue := range item.Issues {
		result.Issues = append(result.Issues, &v1.NativeConfigurationIssue{FieldToken: issue.FieldToken, IssueCode: issue.IssueCode})
	}
	return result
}
func projectConfigurationField(field servercommercialcap.ConfigurationField) *v1.NativeConfigurationField {
	result := &v1.NativeConfigurationField{GroupZhCN: field.GroupZhCN, GroupEnUS: field.GroupEnUS, Unit: field.Unit, Advanced: field.Advanced, DisplayControl: field.DisplayControl, QuantityStep: field.QuantityStep, NumericChoices: field.NumericChoices, FieldToken: field.FieldToken, Type: field.Type, Visible: field.Visible, Required: field.Required, Disabled: field.Disabled, LabelZhCN: field.Label.ZhCN, LabelEnUS: field.Label.EnUS, DescriptionZhCN: field.Description.ZhCN, DescriptionEnUS: field.Description.EnUS, Format: field.Constraints.Format, MinimumLength: field.Constraints.MinimumLength, MaximumLength: field.Constraints.MaximumLength, Options: projectConfigurationOptions(field.Options)}
	// Compiler types determine which exact-string numeric constraints apply.
	switch field.Type {
	case "integer":
		result.MinimumInteger = field.Constraints.Minimum
		result.MaximumInteger = field.Constraints.Maximum
	case "decimal":
		result.MinimumDecimal = field.Constraints.Minimum
		result.MaximumDecimal = field.Constraints.Maximum
	}
	if field.NumericLabels != nil {
		result.NumericLabels = make(map[string]*v1.NativeConfigurationLocalizedText, len(field.NumericLabels))
		for key, value := range field.NumericLabels {
			if value != nil {
				result.NumericLabels[key] = &v1.NativeConfigurationLocalizedText{ZhCN: value.ZhCN, EnUS: value.EnUS}
			}
		}
	}
	if policy := field.UploadPolicy; policy != nil {
		result.UploadPolicy = &v1.NativeConfigurationUploadPolicy{MaximumFiles: policy.MaximumFiles, MaximumFileBytes: policy.MaximumFileBytes, MaximumTotalBytes: policy.MaximumTotalBytes, AllowedMIMETypes: policy.AllowedMIMETypes, AllowedExtensions: policy.AllowedExtensions, SafetyScanRequired: policy.SafetyScanRequired}
	}
	return result
}
func projectConfigurationOptions(options []servercommercialcap.ConfigurationOption) []*v1.NativeConfigurationOption {
	result := make([]*v1.NativeConfigurationOption, 0, len(options))
	for _, option := range options {
		result = append(result, &v1.NativeConfigurationOption{OptionToken: option.OptionToken, LabelZhCN: option.Label.ZhCN, LabelEnUS: option.Label.EnUS, Children: projectConfigurationOptions(option.Children)})
	}
	return result
}
