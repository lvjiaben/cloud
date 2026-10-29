// This file maps governed public purchase-configuration DTOs without exposing
// plugin keys, raw candidate values, internal versions, or conditions.

package public

import (
	"context"

	"lina-core/pkg/bizerr"
	v1 "lina-plugin-idc-core/backend/api/public/v1"
	"lina-plugin-idc-core/backend/internal/service/catalog"
)

func (c *ControllerV1) PurchaseConfigurationGet(
	ctx context.Context,
	req *v1.PurchaseConfigurationGetReq,
) (*v1.PurchaseConfigurationGetRes, error) {
	service, err := catalog.StructuredConfigurationReaderFromPublicReader(c.catalog)
	if err != nil {
		return nil, err
	}
	view, err := service.GetPurchaseConfiguration(ctx, req.PublicSlug)
	if err != nil {
		return nil, err
	}
	return &v1.PurchaseConfigurationGetRes{PurchaseConfigurationView: MapPurchaseConfigurationView(view)}, nil
}

func (c *ControllerV1) PurchaseConfigurationEvaluate(
	ctx context.Context,
	req *v1.PurchaseConfigurationEvaluateReq,
) (*v1.PurchaseConfigurationEvaluateRes, error) {
	service, err := catalog.StructuredConfigurationReaderFromPublicReader(c.catalog)
	if err != nil {
		return nil, err
	}
	values, err := MapPurchaseConfigurationInput(req.Values)
	if err != nil {
		return nil, err
	}
	evaluation, err := service.EvaluatePurchaseConfiguration(ctx, catalog.PublicPurchaseConfigurationInput{
		PublicSlug: req.PublicSlug, ConfigurationRevision: req.ConfigurationRevision, Values: values,
	})
	if err != nil {
		return nil, err
	}
	issues := make([]*v1.PurchaseConfigurationIssue, 0, len(evaluation.Issues))
	for _, issue := range evaluation.Issues {
		issues = append(issues, &v1.PurchaseConfigurationIssue{FieldToken: issue.FieldToken, IssueCode: issue.IssueCode})
	}
	return &v1.PurchaseConfigurationEvaluateRes{PurchaseConfigurationEvaluation: v1.PurchaseConfigurationEvaluation{
		PurchaseConfigurationView: MapPurchaseConfigurationView(&evaluation.PublicPurchaseConfigurationView),
		Valid:                     evaluation.Valid, Issues: issues,
	}}, nil
}

func (c *ControllerV1) PurchaseConfigurationQuote(
	ctx context.Context,
	req *v1.PurchaseConfigurationQuoteReq,
) (*v1.PurchaseConfigurationQuoteRes, error) {
	service, err := catalog.StructuredConfigurationReaderFromPublicReader(c.catalog)
	if err != nil {
		return nil, err
	}
	values, err := MapPurchaseConfigurationInput(req.Values)
	if err != nil {
		return nil, err
	}
	quote, err := service.QuotePurchaseConfiguration(ctx, catalog.PublicPurchaseConfigurationQuoteInput{
		PublicPurchaseConfigurationInput: catalog.PublicPurchaseConfigurationInput{
			PublicSlug: req.PublicSlug, ConfigurationRevision: req.ConfigurationRevision, Values: values,
		},
		CurrencyCode: req.CurrencyCode,
	})
	if err != nil {
		return nil, err
	}
	terms := make([]*v1.PurchaseConfigurationQuoteTerm, 0, len(quote.Terms))
	for _, term := range quote.Terms {
		terms = append(terms, &v1.PurchaseConfigurationQuoteTerm{
			BillingTermToken: term.BillingTermToken,
			LabelZhCN:        term.Label.ZhCN, LabelEnUS: term.Label.EnUS,
			MinimumPurchaseCycles: term.MinimumPurchaseCycles, Status: term.Status,
			SetupAmount: term.SetupAmount, SubtotalAmount: term.SubtotalAmount,
			TaxAmount: term.TaxAmount, TotalAmount: term.TotalAmount,
		})
	}
	return &v1.PurchaseConfigurationQuoteRes{
		ConfigurationRevision: quote.ConfigurationRevision,
		CurrencyCode:          quote.CurrencyCode, Terms: terms,
	}, nil
}

func MapPurchaseConfigurationInput(input []*v1.PurchaseConfigurationValue) ([]catalog.PublicPurchaseConfigurationValue, error) {
	values := make([]catalog.PublicPurchaseConfigurationValue, 0, len(input))
	for _, row := range input {
		if row == nil {
			return nil, bizerr.NewCode(catalog.CodePublicConfigurationInvalid)
		}
		values = append(values, catalog.PublicPurchaseConfigurationValue{
			FieldToken: row.FieldToken, Type: row.Type,
			TextValue: row.TextValue, IntegerValue: row.IntegerValue,
			DecimalValue: row.DecimalValue, BooleanValue: row.BooleanValue,
			DateValue: row.DateValue, DateTimeValue: row.DateTimeValue,
			OptionTokens:        append([]string(nil), row.OptionTokens...),
			TreeOptionTokens:    append([]string(nil), row.TreeOptionTokens...),
			AddressOptionTokens: append([]string(nil), row.AddressOptionTokens...),
			FileTokens:          append([]string(nil), row.FileTokens...),
		})
	}
	return values, nil
}

func MapPurchaseConfigurationView(view *catalog.PublicPurchaseConfigurationView) v1.PurchaseConfigurationView {
	result := v1.PurchaseConfigurationView{
		ServerLoginPolicy:     mapPurchaseServerLoginPolicy(view.ServerLoginPolicy),
		Ready:                 view.Ready,
		ConfigurationRevision: view.ConfigurationRevision,
		Fields:                make([]*v1.PurchaseConfigurationField, 0, len(view.Fields)),
		Values:                make([]*v1.PurchaseConfigurationValue, 0, len(view.Values)),
	}
	for _, field := range view.Fields {
		mapped := &v1.PurchaseConfigurationField{
			Presentation: field.Presentation,
			GroupZhCN:    field.GroupZhCN, GroupEnUS: field.GroupEnUS, Unit: field.Unit, Advanced: field.Advanced, NumericLabels: field.NumericLabels,
			FieldToken: field.FieldToken, Type: field.Type, DisplayControl: field.DisplayControl, QuantityStep: field.QuantityStep, NumericChoices: field.NumericChoices,
			LabelZhCN: field.Label.ZhCN, LabelEnUS: field.Label.EnUS,
			DescriptionZhCN: field.Description.ZhCN, DescriptionEnUS: field.Description.EnUS,
			Visible: field.Visible, Required: field.Required, Disabled: field.Disabled,
			Format:        field.Constraints.Format,
			MinimumLength: field.Constraints.MinimumLength, MaximumLength: field.Constraints.MaximumLength,
			Options: mapPurchaseConfigurationOptions(field.Options),
		}
		if field.UploadPolicy != nil {
			mapped.UploadPolicy = &v1.PurchaseConfigurationUploadPolicy{
				MaximumFiles:       field.UploadPolicy.MaximumFiles,
				MaximumFileBytes:   field.UploadPolicy.MaximumFileBytes,
				MaximumTotalBytes:  field.UploadPolicy.MaximumTotalBytes,
				AllowedMIMETypes:   append([]string(nil), field.UploadPolicy.AllowedMIMETypes...),
				AllowedExtensions:  append([]string(nil), field.UploadPolicy.AllowedExtensions...),
				SafetyScanRequired: field.UploadPolicy.SafetyScanRequired,
			}
		}
		switch field.Type {
		case "integer":
			mapped.MinimumInteger, mapped.MaximumInteger = field.Constraints.Minimum, field.Constraints.Maximum
		case "decimal":
			mapped.MinimumDecimal, mapped.MaximumDecimal = field.Constraints.Minimum, field.Constraints.Maximum
		}
		result.Fields = append(result.Fields, mapped)
	}
	for _, value := range view.Values {
		result.Values = append(result.Values, &v1.PurchaseConfigurationValue{
			FieldToken: value.FieldToken, Type: value.Type,
			TextValue: value.TextValue, IntegerValue: value.IntegerValue,
			DecimalValue: value.DecimalValue, BooleanValue: value.BooleanValue,
			DateValue: value.DateValue, DateTimeValue: value.DateTimeValue,
			OptionTokens:        append([]string(nil), value.OptionTokens...),
			TreeOptionTokens:    append([]string(nil), value.TreeOptionTokens...),
			AddressOptionTokens: append([]string(nil), value.AddressOptionTokens...),
			FileTokens:          append([]string(nil), value.FileTokens...),
		})
	}
	return result
}

func mapPurchaseConfigurationOptions(input []catalog.PublicPurchaseConfigurationOption) []*v1.PurchaseConfigurationOption {
	result := make([]*v1.PurchaseConfigurationOption, 0, len(input))
	for _, option := range input {
		result = append(result, &v1.PurchaseConfigurationOption{
			OptionToken: option.OptionToken,
			LabelZhCN:   option.Label.ZhCN, LabelEnUS: option.Label.EnUS,
			Children: mapPurchaseConfigurationOptions(option.Children),
		})
	}
	return result
}

func mapPurchaseServerLoginPolicy(policy *catalog.PublicServerLoginPolicy) *v1.PurchaseServerLoginPolicy {
	if policy == nil {
		return nil
	}
	return &v1.PurchaseServerLoginPolicy{HostnameEnabled: policy.HostnameEnabled, PasswordEnabled: policy.PasswordEnabled,
		Hostname: v1.PurchaseServerLoginRules{Prefix: policy.Hostname.Prefix, Length: policy.Hostname.Length, Characters: policy.Hostname.Characters},
		Password: v1.PurchaseServerLoginRules{Length: policy.Password.Length, Characters: policy.Password.Characters}}
}
