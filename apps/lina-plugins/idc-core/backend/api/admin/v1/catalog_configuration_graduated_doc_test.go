// These tests bind all extended algorithm and band-boundary DTO docs to the
// existing Chinese documentation catalog, with no runtime-language fallback.
package v1

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestGraduatedConfigurationDocumentation(t *testing.T) {
	translations := map[string]string{}
	files, err := filepath.Glob("../../../../manifest/i18n/zh-CN/apidoc/*.json")
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range files {
		raw, e := os.ReadFile(file)
		if e != nil {
			t.Fatal(e)
		}
		if e = json.Unmarshal(raw, &translations); e != nil {
			t.Fatal(e)
		}
	}
	for _, item := range []struct {
		model any
		field string
	}{
		{CatalogMerchantPriceTarget{}, "PricingMode"}, {CatalogMerchantConfigurationPriceRow{}, "PricingMode"},
		{CatalogConfigurationPriceCandidate{}, "PricingMode"}, {CatalogConfigurationPriceDimensionCreateReq{}, "PricingMode"}, {CatalogConfigurationPriceDimensionCreateRes{}, "PricingMode"},
		{CatalogProductDetailPrice{}, "ConfigurationPricingMode"},
		{CatalogPriceTierInput{}, "MinimumQuantity"}, {CatalogPriceTierInput{}, "MaximumQuantity"},
		{CatalogPriceScheduleTierRes{}, "MinimumQuantity"}, {CatalogPriceScheduleTierRes{}, "MaximumQuantity"},
		{CatalogProductDetailPrice{}, "MinimumQuantity"}, {CatalogProductDetailPrice{}, "MaximumQuantity"},
	} {
		typ := reflect.TypeOf(item.model)
		field, ok := typ.FieldByName(item.field)
		if !ok {
			t.Fatal("DTO field missing")
		}
		key := "plugins.idc_core.api.admin.v1." + typ.Name() + ".fields." + strings.Split(field.Tag.Get("json"), ",")[0] + ".dc"
		if field.Tag.Get("dc") == "" || field.Tag.Get("eg") == "" || translations[key] == "" {
			t.Errorf("missing graduated documentation %s", key)
		}
	}
}
