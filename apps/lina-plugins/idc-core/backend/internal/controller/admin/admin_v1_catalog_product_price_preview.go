// This file maps a bounded saved-product selection preview to the existing
// authorized configuration projection and the authoritative checkout calculator.

package admin

import (
	"bytes"
	"context"
	"encoding/json"
	"io"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/api/admin/v1"
	"lina-plugin-idc-core/backend/internal/service/catalog"
)

func (c *ControllerV1) CatalogProductPricePreview(ctx context.Context, req *v1.CatalogProductPricePreviewReq) (*v1.CatalogProductPricePreviewRes, error) {
	values, err := decodePricePreviewValues(req.Values)
	if err != nil {
		return nil, err
	}
	detail, err := c.catalogManagement.GetProductDetail(ctx, req.ProductNo)
	if err != nil {
		return nil, err
	}
	if detail.Version.VersionNo != req.VersionNo {
		return nil, bizerr.NewCode(catalog.CodePublicQuoteStale)
	}
	configuration, err := c.productAutomation.GetVersionConfiguration(ctx, req.VersionNo)
	if err != nil {
		return nil, err
	}
	input := catalog.ProductEditorPricePreviewInput{ProductNo: req.ProductNo, VersionNo: req.VersionNo, CurrencyCode: req.CurrencyCode, Values: mapConfigurationInput(values)}
	if configuration != nil {
		input.ConfigurationVersion = configuration.Version
		input.ResolvedCandidates = configuration.CustomerResolvedCandidates
	}
	result, err := c.catalogManagement.PreviewProductPrice(ctx, input)
	if err != nil {
		return nil, err
	}
	preview := &v1.CatalogProductPricePreview{VersionNo: result.VersionNo, CurrencyCode: result.CurrencyCode, PriceIncludesTax: result.PriceIncludesTax, Terms: make([]*v1.CatalogProductPricePreviewTerm, 0, len(result.Terms))}
	for _, term := range result.Terms {
		preview.Terms = append(preview.Terms, &v1.CatalogProductPricePreviewTerm{BillingRuleNo: term.BillingRuleNo, Status: term.Status, ChargeType: string(term.ChargeType), PeriodUnit: string(term.PeriodUnit), PeriodCount: term.PeriodCount, Cycles: term.Cycles, BaseAmountMinor: previewAmountPointer(term.BaseAmountMinor), ConfigurationAmountMinor: previewAmountPointer(term.ConfigurationAmountMinor), SetupAmountMinor: previewAmountPointer(term.SetupAmountMinor), TaxAmountMinor: term.TaxAmountMinor, TotalAmountMinor: term.TotalAmountMinor})
		item := preview.Terms[len(preview.Terms)-1]
		item.Issues = make([]*v1.CatalogProductPricePreviewIssue, 0, len(term.Issues))
		for _, issue := range term.Issues {
			item.Issues = append(item.Issues, &v1.CatalogProductPricePreviewIssue{Kind: string(issue.Kind), Reason: string(issue.Reason), FieldKey: issue.FieldKey, NameZhCN: issue.NameZhCN, NameEnUS: issue.NameEnUS})
		}
	}
	return &v1.CatalogProductPricePreviewRes{Preview: preview}, nil
}

func decodePricePreviewValues(content string) ([]*v1.CatalogStructuredConfigurationValue, error) {
	if len(content) == 0 || len(content) > 32768 {
		return nil, bizerr.NewCode(catalog.CodePublicQuoteInvalid)
	}
	var values []*v1.CatalogStructuredConfigurationValue
	decoder := json.NewDecoder(bytes.NewBufferString(content))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&values); err != nil {
		return nil, bizerr.NewCode(catalog.CodePublicQuoteInvalid)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF || values == nil || len(values) > 128 {
		return nil, bizerr.NewCode(catalog.CodePublicQuoteInvalid)
	}
	for _, value := range values {
		if value == nil || len(value.FileTokens) > 0 {
			return nil, bizerr.NewCode(catalog.CodePublicQuoteInvalid)
		}
	}
	return values, nil
}

// Empty internal amounts represent unavailable schedules, never a zero price.
func previewAmountPointer(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}
