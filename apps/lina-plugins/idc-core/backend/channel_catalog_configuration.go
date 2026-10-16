// This file implements bounded same-system protocol projections and rejects
// missing ownership or configuration evidence rather than guessing it.

package backend

import (
	"context"
	"golang.org/x/text/currency"
	"lina-core/pkg/bizerr"

	"lina-plugin-idc-core/backend/cap/channelcommercecap"
	"lina-plugin-idc-core/backend/internal/service/catalog"
)

func (runtime *channelCommerceRuntime) BrowseCatalogNumbered(ctx context.Context, input channelcommercecap.CatalogNumberedQuery) (*channelcommercecap.CatalogNumberedPage, error) {
	page, err := runtime.catalog.SearchNumbered(ctx, catalog.PublicCatalogNumberedInput{NamePrefix: input.NamePrefix, Currency: input.Currency, Page: input.Page, PageSize: input.PageSize})
	if err != nil {
		return nil, err
	}
	result := &channelcommercecap.CatalogNumberedPage{Products: make([]channelcommercecap.CatalogProduct, len(page.Products)), Total: page.Total}
	for index, row := range page.Products {
		result.Products[index] = channelcommercecap.CatalogProduct{PublicSlug: row.PublicSlug, NameZhCN: row.NameZhCN, NameEnUS: row.NameEnUS, SummaryZhCN: row.SummaryZhCN, SummaryEnUS: row.SummaryEnUS, Currencies: row.Currencies, Availability: row.Availability}
	}
	return result, nil
}

// ReadProductSnapshot uses a fixed number of reads. Billing prices are requested
// explicitly for a selected currency and valid configuration, without guessing
// default values or performing a query per currency during product import.
func (runtime *channelCommerceRuntime) ReadProductSnapshot(ctx context.Context, slug string) (*channelcommercecap.ProductSnapshot, error) {
	product, err := runtime.catalog.GetBySlug(ctx, slug)
	if err != nil {
		return nil, err
	}
	configuration, err := runtime.ReadCatalogConfiguration(ctx, slug)
	if err != nil {
		return nil, err
	}
	result := &channelcommercecap.ProductSnapshot{
		Product:       channelcommercecap.CatalogProduct{PublicSlug: product.PublicSlug, NameZhCN: product.NameZhCN, NameEnUS: product.NameEnUS, SummaryZhCN: product.SummaryZhCN, SummaryEnUS: product.SummaryEnUS, Currencies: product.Currencies, Availability: product.Availability},
		Configuration: *configuration, ConfigurationMode: "remote_evaluation", Currencies: make([]channelcommercecap.CatalogCurrency, 0, len(product.Currencies)), BillingTerms: []channelcommercecap.CurrencyBillingTerms{}, PricingReady: false,
	}
	for _, code := range product.Currencies {
		unit, parseErr := currency.ParseISO(code)
		if parseErr != nil || unit == currency.XXX {
			return nil, bizerr.NewCode(catalog.CodePublicCatalogUnavailable)
		}
		precision, _ := currency.Standard.Rounding(unit)
		result.Currencies = append(result.Currencies, channelcommercecap.CatalogCurrency{CurrencyCode: code, Precision: precision})
	}
	return result, nil
}

func (runtime *channelCommerceRuntime) ReadCatalogConfiguration(ctx context.Context, slug string) (*channelcommercecap.Configuration, error) {
	reader, err := catalog.StructuredConfigurationReaderFromPublicReader(runtime.catalog)
	if err != nil {
		return nil, err
	}
	view, err := reader.GetPurchaseConfiguration(ctx, slug)
	if err != nil {
		return nil, err
	}
	result := projectChannelConfiguration(*view)
	return &result, nil
}

func channelConfigurationInput(input channelcommercecap.ConfigurationInput) catalog.PublicPurchaseConfigurationInput {
	values := make([]catalog.PublicPurchaseConfigurationValue, len(input.Values))
	for index, value := range input.Values {
		values[index] = catalog.PublicPurchaseConfigurationValue{FieldToken: value.FieldToken, Type: value.Type, TextValue: value.TextValue, IntegerValue: value.IntegerValue, DecimalValue: value.DecimalValue, BooleanValue: value.BooleanValue, DateValue: value.DateValue, DateTimeValue: value.DateTimeValue, OptionTokens: value.OptionTokens, TreeOptionTokens: value.TreeOptionTokens, AddressOptionTokens: value.AddressOptionTokens, FileTokens: value.FileTokens}
	}
	return catalog.PublicPurchaseConfigurationInput{PublicSlug: input.PublicSlug, ConfigurationRevision: input.ConfigurationRevision, Values: values}
}

func (runtime *channelCommerceRuntime) EvaluateCatalogConfiguration(ctx context.Context, input channelcommercecap.ConfigurationInput) (*channelcommercecap.ConfigurationEvaluation, error) {
	ctx, contextErr := runtime.fileCustomerContext(ctx, input.CustomerNo)
	if contextErr != nil {
		return nil, contextErr
	}
	reader, err := catalog.StructuredConfigurationReaderFromPublicReader(runtime.catalog)
	if err != nil {
		return nil, err
	}
	view, err := reader.EvaluatePurchaseConfiguration(ctx, channelConfigurationInput(input))
	if err != nil {
		return nil, err
	}
	result := &channelcommercecap.ConfigurationEvaluation{Configuration: projectChannelConfiguration(view.PublicPurchaseConfigurationView), Valid: view.Valid, Issues: make([]channelcommercecap.ConfigurationIssue, len(view.Issues))}
	for index, issue := range view.Issues {
		result.Issues[index] = channelcommercecap.ConfigurationIssue{FieldToken: issue.FieldToken, IssueCode: issue.IssueCode}
	}
	return result, nil
}

func (runtime *channelCommerceRuntime) PreviewCatalogTerms(ctx context.Context, input channelcommercecap.ConfigurationInput) (*channelcommercecap.BillingTerms, error) {
	ctx, contextErr := runtime.fileCustomerContext(ctx, input.CustomerNo)
	if contextErr != nil {
		return nil, contextErr
	}
	reader, err := catalog.StructuredConfigurationReaderFromPublicReader(runtime.catalog)
	if err != nil {
		return nil, err
	}
	view, err := reader.QuotePurchaseConfiguration(ctx, catalog.PublicPurchaseConfigurationQuoteInput{PublicPurchaseConfigurationInput: channelConfigurationInput(input), CurrencyCode: input.CurrencyCode})
	if err != nil {
		return nil, err
	}
	result := &channelcommercecap.BillingTerms{ConfigurationRevision: view.ConfigurationRevision, CurrencyCode: view.CurrencyCode, Terms: make([]channelcommercecap.BillingTerm, len(view.Terms))}
	for index, term := range view.Terms {
		result.Terms[index] = channelcommercecap.BillingTerm{BillingTermToken: term.BillingTermToken, ChargeType: string(term.ChargeType), PeriodUnit: string(term.PeriodUnit), PeriodCount: term.PeriodCount, Label: channelcommercecap.LocalizedText{ZhCN: term.Label.ZhCN, EnUS: term.Label.EnUS}, MinimumPurchaseCycles: term.MinimumPurchaseCycles, Status: term.Status, SetupAmount: term.SetupAmount, SubtotalAmount: term.SubtotalAmount, TaxAmount: term.TaxAmount, TotalAmount: term.TotalAmount}
	}
	return result, nil
}

func projectChannelConfiguration(view catalog.PublicPurchaseConfigurationView) channelcommercecap.Configuration {
	result := channelcommercecap.Configuration{ConfigurationRevision: view.ConfigurationRevision, Fields: make([]channelcommercecap.ConfigurationField, len(view.Fields)), Values: make([]channelcommercecap.PurchaseValue, len(view.Values))}
	for index, field := range view.Fields {
		projected := channelcommercecap.ConfigurationField{FieldToken: field.FieldToken, Type: field.Type, Label: channelcommercecap.LocalizedText{ZhCN: field.Label.ZhCN, EnUS: field.Label.EnUS}, Description: channelcommercecap.LocalizedText{ZhCN: field.Description.ZhCN, EnUS: field.Description.EnUS}, Visible: field.Visible, Required: field.Required, Disabled: field.Disabled, Constraints: channelcommercecap.ConfigurationConstraints{Format: field.Constraints.Format, Minimum: field.Constraints.Minimum, Maximum: field.Constraints.Maximum, MinimumLength: field.Constraints.MinimumLength, MaximumLength: field.Constraints.MaximumLength}, Options: projectChannelOptions(field.Options)}
		if policy := field.UploadPolicy; policy != nil {
			projected.UploadPolicy = &channelcommercecap.ConfigurationUploadPolicy{MaximumFiles: policy.MaximumFiles, MaximumFileBytes: policy.MaximumFileBytes, MaximumTotalBytes: policy.MaximumTotalBytes, AllowedMIMETypes: policy.AllowedMIMETypes, AllowedExtensions: policy.AllowedExtensions, SafetyScanRequired: policy.SafetyScanRequired}
		}
		result.Fields[index] = projected
	}
	for index, value := range view.Values {
		result.Values[index] = channelcommercecap.PurchaseValue{FieldToken: value.FieldToken, Type: value.Type, TextValue: value.TextValue, IntegerValue: value.IntegerValue, DecimalValue: value.DecimalValue, BooleanValue: value.BooleanValue, DateValue: value.DateValue, DateTimeValue: value.DateTimeValue, OptionTokens: value.OptionTokens, TreeOptionTokens: value.TreeOptionTokens, AddressOptionTokens: value.AddressOptionTokens, FileTokens: value.FileTokens}
	}
	return result
}

func projectChannelOptions(options []catalog.PublicPurchaseConfigurationOption) []channelcommercecap.ConfigurationOption {
	result := make([]channelcommercecap.ConfigurationOption, len(options))
	for index, option := range options {
		result[index] = channelcommercecap.ConfigurationOption{OptionToken: option.OptionToken, Label: channelcommercecap.LocalizedText{ZhCN: option.Label.ZhCN, EnUS: option.Label.EnUS}, Children: projectChannelOptions(option.Children)}
	}
	return result
}

var _ channelcommercecap.CatalogConfigurationRuntime = (*channelCommerceRuntime)(nil)
