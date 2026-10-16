// This file preserves immutable configuration intent and freezes it after
// direct order creation without impersonating the target customer.
package commerce

import (
	"context"
	"encoding/json"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/internal/model/entity"
	"lina-plugin-idc-core/backend/internal/service/catalog"
)

// storeAssistedConfigurations records at most eight already validated inputs.
func (s *assistedQuoteServiceImpl) storeAssistedConfigurations(ctx context.Context, requestNo string, items []resolvedAssistedQuoteItem) error {
	for _, item := range items {
		encoded, err := json.Marshal(item.input.Purchase)
		if err != nil {
			return bizerr.WrapCode(err, CodeAssistedQuoteInvalid)
		}
		if _, err = s.database.Exec(ctx, `INSERT INTO plugin_idc_core_commerce_assisted_configuration
   (tenant_id,request_item_no,request_no,purchase_input) VALUES (0,?,?,CAST(? AS jsonb))`, item.requestItemNo, requestNo, string(encoded)); err != nil {
			return bizerr.WrapCode(err, CodeAssistedQuoteUnavailable)
		}
	}
	return nil
}

// freezeAssistedConfigurations reuses the governed freezer with explicit ownership.
// Each request has at most eight items; files require the real customer's session.
func (s *assistedQuoteServiceImpl) freezeAssistedConfigurations(ctx context.Context, request *entity.CommerceAssistedQuoteRequest, graph *quoteCartGraph, quote *Quote) error {
	var inputs []struct {
		RequestItemNo string
		PurchaseInput string
	}
	if err := s.database.GetScan(ctx, &inputs, `SELECT request_item_no,purchase_input FROM plugin_idc_core_commerce_assisted_configuration WHERE tenant_id=0 AND request_no=? LIMIT ?`, request.RequestNo, maximumAssistedQuoteItems+1); err != nil {
		return bizerr.WrapCode(err, CodeAssistedQuoteUnavailable)
	}
	if len(inputs) != len(graph.items) || len(quote.Items) != len(graph.items) {
		return bizerr.NewCode(CodeAssistedConfigurationMissing)
	}
	byItem := make(map[string]catalog.PurchaseConfigurationInput, len(inputs))
	for _, row := range inputs {
		var input catalog.PurchaseConfigurationInput
		if err := json.Unmarshal([]byte(row.PurchaseInput), &input); err != nil {
			return bizerr.WrapCode(err, CodeAssistedQuoteConflict)
		}
		byItem[row.RequestItemNo] = input
	}
	versions := make([]string, 0, len(graph.items))
	for _, item := range graph.items {
		versions = append(versions, item.VersionNo)
	}
	var configurations []struct{ VersionNo string }
	if err := s.database.GetScan(ctx, &configurations, `SELECT version_no FROM plugin_idc_core_catalog_product_version_configuration WHERE tenant_id=0 AND version_no IN(?)`, versions); err != nil {
		return bizerr.WrapCode(err, CodeAssistedQuoteUnavailable)
	}
	required := make(map[string]bool, len(configurations))
	for _, configuration := range configurations {
		required[configuration.VersionNo] = true
	}
	for index, item := range graph.items {
		input, ok := byItem[item.CartItemNo]
		if !ok {
			return bizerr.NewCode(CodeAssistedQuoteConflict)
		}
		resolved, err := s.catalog.ResolvePurchaseConfiguration(ctx, input)
		if err != nil {
			return err
		}
		if resolved == nil || resolved.ProductNo != item.ProductNo || resolved.VersionNo != item.VersionNo || resolved.BillingRuleNo != item.BillingRuleNo {
			return bizerr.NewCode(CodeAssistedQuoteConflict)
		}
		// Manual fixed products have no structured declaration or automation binding.
		if !required[item.VersionNo] {
			continue
		}
		if _, err := s.commerce.freezeQuoteItemConfiguration(ctx, request.CustomerNo, request.RequestedBy, FreezeQuoteItemConfigurationInput{
			QuoteItemNo: quote.Items[index].QuoteItemNo,
			Purchase:    catalog.PublicPurchaseConfigurationInput{PublicSlug: input.PublicSlug, ConfigurationRevision: input.ConfigurationRevision, Values: input.Values},
		}); err != nil {
			return err
		}
	}
	return nil
}
