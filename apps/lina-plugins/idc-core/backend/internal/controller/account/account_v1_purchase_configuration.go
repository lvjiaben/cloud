// This file maps authenticated purchase configuration evaluation and quoting.
package account

import (
	"context"

	v1 "lina-plugin-idc-core/backend/api/account/v1"
	publicv1 "lina-plugin-idc-core/backend/api/public/v1"
	publiccontroller "lina-plugin-idc-core/backend/internal/controller/public"
	"lina-plugin-idc-core/backend/internal/service/catalog"
	"lina-plugin-idc-core/backend/internal/service/commerce"
)

func (c *ControllerV1) PurchaseConfigurationEvaluate(ctx context.Context, req *v1.PurchaseConfigurationEvaluateReq) (*v1.PurchaseConfigurationEvaluateRes, error) {
	values, err := publiccontroller.MapPurchaseConfigurationInput(req.Values)
	if err != nil {
		return nil, err
	}
	evaluation, err := c.purchaseConfiguration.EvaluatePurchaseConfiguration(ctx, catalog.PublicPurchaseConfigurationInput{
		PublicSlug: req.PublicSlug, ConfigurationRevision: req.ConfigurationRevision, Values: values,
	})
	if err != nil {
		return nil, err
	}
	issues := make([]*publicv1.PurchaseConfigurationIssue, 0, len(evaluation.Issues))
	for _, issue := range evaluation.Issues {
		issues = append(issues, &publicv1.PurchaseConfigurationIssue{FieldToken: issue.FieldToken, IssueCode: issue.IssueCode})
	}
	return &v1.PurchaseConfigurationEvaluateRes{PurchaseConfigurationEvaluation: publicv1.PurchaseConfigurationEvaluation{
		PurchaseConfigurationView: publiccontroller.MapPurchaseConfigurationView(&evaluation.PublicPurchaseConfigurationView),
		Valid:                     evaluation.Valid, Issues: issues,
	}}, nil
}

func (c *ControllerV1) PurchaseConfigurationFreeze(ctx context.Context, req *v1.PurchaseConfigurationFreezeReq) (*v1.PurchaseConfigurationFreezeRes, error) {
	values, err := publiccontroller.MapPurchaseConfigurationInput(req.Values)
	if err != nil {
		return nil, err
	}
	frozen, err := c.commerce.FreezeAccountQuoteItemConfiguration(ctx, commerce.FreezeQuoteItemConfigurationInput{
		QuoteItemNo: req.QuoteItemNo,
		Purchase: catalog.PublicPurchaseConfigurationInput{
			PublicSlug: req.PublicSlug, ConfigurationRevision: req.ConfigurationRevision, Values: values,
		},
	})
	if err != nil {
		return nil, err
	}
	return &v1.PurchaseConfigurationFreezeRes{
		SnapshotNo: frozen.SnapshotNo, ConfigurationFrozen: frozen.ConfigurationFrozen,
		ConfigurationRevision: frozen.ConfigurationRevision,
	}, nil
}

func (c *ControllerV1) PurchaseConfigurationQuote(ctx context.Context, req *v1.PurchaseConfigurationQuoteReq) (*v1.PurchaseConfigurationQuoteRes, error) {
	values, err := publiccontroller.MapPurchaseConfigurationInput(req.Values)
	if err != nil {
		return nil, err
	}
	quote, err := c.purchaseConfiguration.QuotePurchaseConfiguration(ctx, catalog.PublicPurchaseConfigurationQuoteInput{
		PublicPurchaseConfigurationInput: catalog.PublicPurchaseConfigurationInput{
			PublicSlug: req.PublicSlug, ConfigurationRevision: req.ConfigurationRevision, Values: values,
		},
		CurrencyCode: req.CurrencyCode,
	})
	if err != nil {
		return nil, err
	}
	terms := make([]*publicv1.PurchaseConfigurationQuoteTerm, 0, len(quote.Terms))
	for _, term := range quote.Terms {
		terms = append(terms, &publicv1.PurchaseConfigurationQuoteTerm{
			BillingTermToken: term.BillingTermToken, LabelZhCN: term.Label.ZhCN, LabelEnUS: term.Label.EnUS,
			MinimumPurchaseCycles: term.MinimumPurchaseCycles, Status: term.Status,
			SetupAmount: term.SetupAmount, SubtotalAmount: term.SubtotalAmount,
			TaxAmount: term.TaxAmount, TotalAmount: term.TotalAmount,
		})
	}
	return &v1.PurchaseConfigurationQuoteRes{
		ConfigurationRevision: quote.ConfigurationRevision, CurrencyCode: quote.CurrencyCode, Terms: terms,
	}, nil
}
