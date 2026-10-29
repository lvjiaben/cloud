package v1

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestCatalogFulfillmentPolicyRequestDoesNotExposeExecutionInternals(t *testing.T) {
	t.Parallel()
	typeOf := reflect.TypeOf(CatalogFulfillmentPolicyUpdateReq{})
	allowed := map[string]struct{}{"Meta": {}, "VersionNo": {}, "InterfaceInstanceNo": {}, "StrategyCode": {}, "UpstreamProduct": {}}
	for index := 0; index < typeOf.NumField(); index++ {
		field := typeOf.Field(index)
		if _, ok := allowed[field.Name]; !ok {
			t.Fatalf("merchant request exposes unexpected field %s", field.Name)
		}
	}
	for _, forbidden := range []string{
		"GranularityCode", "ProvisionMethod", "QueryMethod", "MaximumAttempts", "RetryBaseSeconds",
	} {
		if _, exists := typeOf.FieldByName(forbidden); exists {
			t.Fatalf("merchant request exposes technical field %s", forbidden)
		}
	}
}

func TestCatalogFulfillmentPolicySummaryDoesNotReturnMethodsOrRetrySettings(t *testing.T) {
	t.Parallel()
	typeOf := reflect.TypeOf(CatalogFulfillmentPolicySummary{})
	for _, forbidden := range []string{
		"GranularityCode", "ProvisionMethod", "QueryMethod", "MaximumAttempts", "RetryBaseSeconds", "PolicyDigest",
	} {
		if _, exists := typeOf.FieldByName(forbidden); exists {
			t.Fatalf("merchant summary exposes technical field %s", forbidden)
		}
	}
}

// Product option bindings have a readable English source and plugin-owned
// Chinese documentation, just like the surrounding catalog DTO contract.
func TestCatalogProductOptionBindingDocumentation(t *testing.T) {
	field, ok := reflect.TypeOf(CatalogCustomerFieldSelection{}).FieldByName("ProductFieldKey")
	if !ok || field.Tag.Get("json") != "productFieldKey,omitempty" || field.Tag.Get("eg") == "" || field.Tag.Get("dc") == "" {
		t.Fatal("binding API documentation is incomplete")
	}
	for _, r := range field.Tag.Get("dc") {
		if r > 127 {
			t.Fatal("binding source description is not English")
		}
	}
	data, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "manifest", "i18n", "zh-CN", "apidoc", "catalog-product-option-binding.json"))
	if err != nil {
		t.Fatal(err)
	}
	translations := map[string]string{}
	if err = json.Unmarshal(data, &translations); err != nil {
		t.Fatal(err)
	}
	priceField, _ := reflect.TypeOf(CatalogProductVersionConfiguration{}).FieldByName("PriceFieldTokens")
	if priceField.Tag.Get("dc") == "" || priceField.Tag.Get("eg") == "" || translations["plugins.idc_core.api.admin.v1.CatalogProductVersionConfiguration.fields.priceFieldTokens.dc"] == "" {
		t.Fatal("price field identity API documentation is incomplete")
	}
	if strings.TrimSpace(translations["plugins.idc_core.api.admin.v1.CatalogCustomerFieldSelection.fields.productFieldKey.dc"]) == "" {
		t.Fatal("binding Chinese API documentation is missing")
	}
}
