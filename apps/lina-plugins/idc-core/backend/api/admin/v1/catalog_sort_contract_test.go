// This file protects the additive product display-order contract and translation.
package v1

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

func TestCatalogProductSortDocumentation(t *testing.T) {
	field, ok := reflect.TypeFor[CatalogProductDescriptionUpdateReq]().FieldByName("SortOrder")
	if !ok || field.Type != reflect.TypeFor[*int]() || field.Tag.Get("json") != "sortOrder" || field.Tag.Get("dc") == "" || field.Tag.Get("eg") == "" {
		t.Fatal("sort order contract is incomplete")
	}
	body, err := os.ReadFile("../../../../manifest/i18n/zh-CN/apidoc/catalog-description.json")
	if err != nil {
		t.Fatal(err)
	}
	var translated map[string]string
	if err = json.Unmarshal(body, &translated); err != nil {
		t.Fatal(err)
	}
	if translated["plugins.idc_core.api.admin.v1.CatalogProductDescriptionUpdateReq.fields.sortOrder.dc"] == "" {
		t.Fatal("sort order translation missing")
	}
}

// TestCatalogListStatisticsDocumentation locks the summary and deletion contracts.
func TestCatalogListStatisticsDocumentation(t *testing.T) {
	body, err := os.ReadFile("../../../../manifest/i18n/zh-CN/apidoc/catalog-list-statistics.json")
	if err != nil {
		t.Fatal(err)
	}
	var translated map[string]string
	if err = json.Unmarshal(body, &translated); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"SortOrder", "AvailableQuantity", "ProvisionedQuantity", "TotalQuantity"} {
		field, ok := reflect.TypeFor[CatalogProductSummaryItem]().FieldByName(name)
		if !ok || field.Tag.Get("dc") == "" || field.Tag.Get("eg") == "" {
			t.Fatalf("incomplete field: %s", name)
		}
		if translated["plugins.idc_core.api.admin.v1.CatalogProductSummaryItem.fields."+field.Tag.Get("json")+".dc"] == "" {
			t.Fatalf("missing translation: %s", name)
		}
	}
	meta, _ := reflect.TypeFor[CatalogProductDeleteReq]().FieldByName("Meta")
	if meta.Tag.Get("method") != "delete" || meta.Tag.Get("permission") != "idc:admin:catalog:product:delete" {
		t.Fatal("unsafe deletion route")
	}
}
