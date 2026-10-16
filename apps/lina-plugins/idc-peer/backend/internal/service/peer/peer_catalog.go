// This file implements bounded same-system protocol projections and rejects
// missing ownership or configuration evidence rather than guessing it.

package peer

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"

	"lina-core/pkg/plugin/capability/confidentialoutboundcap"
	"lina-plugin-idc-core/backend/cap/channelcommercecap"
	"lina-plugin-idc-core/backend/cap/resourceinterfaceprojectioncap"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
)

var configurationFields = []string{"configurationRevision", "fields", "fieldToken", "type", "label", "zhCn", "enUs", "description", "visible", "required", "disabled", "constraints", "format", "minimum", "maximum", "minimumLength", "maximumLength", "options", "optionToken", "children", "uploadPolicy", "maximumFiles", "maximumFileBytes", "maximumTotalBytes", "allowedMimeTypes", "allowedExtensions", "safetyScanRequired", "values", "textValue", "integerValue", "decimalValue", "booleanValue", "dateValue", "dateTimeValue", "optionTokens", "treeOptionTokens", "addressOptionTokens", "fileTokens"}
var billingFields = []string{"configurationRevision", "currencyCode", "terms", "billingTermToken", "label", "zhCn", "enUs", "chargeType", "periodUnit", "periodCount", "minimumPurchaseCycles", "status", "setupAmount", "subtotalAmount", "taxAmount", "totalAmount"}
var importSnapshotFields = []string{
	"protocolVersion",
	"product",
	"publicSlug",
	"nameZhCn",
	"nameEnUs",
	"summaryZhCn",
	"summaryEnUs",
	"currencies",
	"availability",
	"configurationRevision",
	"schema",
	"version",
	"fields",
	"key",
	"type",
	"label",
	"enUS",
	"zhCN",
	"description",
	"required",
	"textFormat",
	"minimum",
	"maximum",
	"minimumLength",
	"maximumLength",
	"staticCandidates",
	"value",
	"parentValue",
	"dynamicCandidates",
	"method",
	"conditions",
	"when",
	"operator",
	"field",
	"children",
	"actions",
	"action",
	"values",
	"uploadPolicies",
	"id",
	"maximumFiles",
	"maximumFileBytes",
	"maximumTotalBytes",
	"allowedMIMETypes",
	"allowedExtensions",
	"safetyScanRequired",
	"mappings",
	"fieldKey",
	"fieldToken",
	"options",
	"optionToken",
	"prices",
	"currencyCode",
	"termReference",
	"chargeType",
	"periodUnit",
	"periodCount",
	"pricePhase",
	"quantityMode",
	"trialQuantity",
	"minimumPurchaseCycles",
	"maximumRenewalCycles",
	"usage",
	"meterUnit",
	"tierMode",
	"collectionUnit",
	"collectionCount",
	"roundingMode",
	"roundingQuantity",
	"minimumQuantity",
	"maximumQuantity",
	"missingPolicy",
	"base",
	"amountMinor",
	"setupAmountMinor",
	"renewalAmountMinor",
	"tax",
	"priceIncludesTax",
	"treatment",
	"rateBasisPoints",
	"categoryCode",
	"roundingScope",
	"roundingQuantumMinor",
	"wholeItemRule",
	"prior",
	"kind",
	"fixedAmountMinor",
	"discountBasisPoints",
	"granularity",
	"ongoingService",
	"renewalMode", "pricingComplete", "precision", "fulfillment", "bands", "uploadPolicy",
}

func ExecuteResource(executor confidentialoutboundcap.Service, contextID, method string, payload json.RawMessage) (json.RawMessage, error) {
	c := &client{executor: executor, contextID: contextID}
	switch method {
	case resourcecap.DynamicProviderMethodConfigurationCandidates:
		return ReadImportCandidates(executor, contextID, payload)
	case resourcecap.DynamicProviderMethodVerifyConnection:
		if err := c.verify(); err != nil {
			return nil, err
		}
		return json.Marshal(resourcecap.ConnectionVerification{Verified: true, ObservedAt: c.observedAt})
	case resourcecap.DynamicProviderMethodDiscoverCapabilities:
		if err := c.verify(); err != nil {
			return nil, err
		}
		return json.Marshal(&resourcecap.CapabilitySnapshot{AdapterVersion: "same-system-v1", ObservedAt: c.observedAt, Capabilities: []resourcecap.CapabilityDescriptor{{Code: resourcecap.CapabilityAuthentication, Support: resourcecap.SupportLevelSupported}, {Code: resourcecap.CapabilityDiscovery, Support: resourcecap.SupportLevelSupported}}})
	case resourcecap.DynamicProviderMethodListCatalog:
		var query resourceinterfaceprojectioncap.CatalogQuery
		if json.Unmarshal(payload, &query) != nil || query.Validate() != nil {
			return nil, errInvalid
		}
		var page channelcommercecap.CatalogNumberedPage
		if err := c.call(http.MethodGet, "/catalog/products", map[string][]string{"namePrefix": {query.Keyword}, "page": {strconv.Itoa(query.Page)}, "pageSize": {strconv.Itoa(query.PageSize)}}, nil, []string{"products", "total", "publicSlug", "nameZhCn", "nameEnUs", "summaryZhCn", "summaryEnUs", "currencies", "availability"}, &page); err != nil {
			return nil, err
		}
		result := resourceinterfaceprojectioncap.CatalogPage{Items: make([]resourceinterfaceprojectioncap.CatalogProduct, len(page.Products)), Total: page.Total}
		for index, product := range page.Products {
			result.Items[index] = projectProduct(product)
		}
		if result.Validate(query) != nil {
			return nil, errUnavailable
		}
		return json.Marshal(result)
	case resourcecap.DynamicProviderMethodReadCatalogProduct:
		var query resourceinterfaceprojectioncap.CatalogProductQuery
		if json.Unmarshal(payload, &query) != nil || query.Validate() != nil {
			return nil, errInvalid
		}
		var snapshot channelcommercecap.ImportSnapshot
		if err := c.call(http.MethodGet, "/catalog/products/"+url.PathEscape(query.Reference)+"/import-snapshot", nil, nil, uniqueFields(importSnapshotFields), &snapshot); err != nil {
			return nil, err
		}
		if snapshot.Validate() != nil || snapshot.Product.PublicSlug != query.Reference {
			return nil, errUnavailable
		}
		slug, revision := snapshot.Product.PublicSlug, snapshot.ConfigurationRevision
		result := resourceinterfaceprojectioncap.CatalogDetail{Product: projectProduct(snapshot.Product), ImportSnapshot: &snapshot, BasePrices: []resourceinterfaceprojectioncap.CatalogPrice{}, Options: []resourceinterfaceprojectioncap.CatalogOption{}, PriceScheduleComplete: snapshot.PricingComplete, ProductConfiguration: []resourceinterfaceprojectioncap.CatalogConfigurationValue{{FieldKey: "upstream_public_slug", TextValue: &slug}, {FieldKey: "upstream_configuration_revision", TextValue: &revision}}}
		if result.Validate(query) != nil {
			return nil, errUnavailable
		}
		return json.Marshal(result)
	case resourcecap.DynamicProviderMethodEvaluateCatalogConfiguration, resourcecap.DynamicProviderMethodCatalogBillingTerms:
		var input channelcommercecap.ConfigurationInput
		if json.Unmarshal(payload, &input) != nil || input.Validate() != nil {
			return nil, errInvalid
		}
		path := "/catalog/products/" + url.PathEscape(input.PublicSlug)
		query, err := configurationQuery(input)
		if err != nil {
			return nil, err
		}
		if method == resourcecap.DynamicProviderMethodEvaluateCatalogConfiguration {
			var result channelcommercecap.ConfigurationEvaluation
			fields := append(append([]string{}, configurationFields...), "valid", "issues", "issueCode")
			if err := c.call(http.MethodGet, path+"/configuration/evaluate", query, nil, fields, &result); err != nil {
				return nil, err
			}
			if result.Configuration.Validate() != nil || result.ConfigurationRevision != input.ConfigurationRevision {
				return nil, errUnavailable
			}
			return json.Marshal(result)
		}
		var result channelcommercecap.BillingTerms
		if err := c.call(http.MethodGet, path+"/billing-terms", query, nil, billingFields, &result); err != nil {
			return nil, err
		}
		if result.Validate() != nil || result.ConfigurationRevision != input.ConfigurationRevision || result.CurrencyCode != input.CurrencyCode {
			return nil, errUnavailable
		}
		return json.Marshal(result)
	default:
		return nil, errRejected
	}
}

func projectProduct(value channelcommercecap.CatalogProduct) resourceinterfaceprojectioncap.CatalogProduct {
	name, description := value.NameZhCN, value.SummaryZhCN
	if name == "" {
		name = value.NameEnUS
	}
	if description == "" {
		description = value.SummaryEnUS
	}
	runes := []rune(description)
	if len(runes) > 1024 {
		description = string(runes[:1024])
	}
	return resourceinterfaceprojectioncap.CatalogProduct{Reference: value.PublicSlug, Name: name, Description: description}
}

func uniqueFields(input []string) []string {
	result := make([]string, 0, len(input))
	seen := make(map[string]bool, len(input))
	for _, field := range input {
		if !seen[field] {
			seen[field] = true
			result = append(result, field)
		}
	}
	return result
}
