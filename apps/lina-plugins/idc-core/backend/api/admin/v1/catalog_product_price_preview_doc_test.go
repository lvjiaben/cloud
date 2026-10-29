// This file keeps the read-only price preview contract documented and localized.
package v1

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestProductPricePreviewDocumentation(t *testing.T) {
	content, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "manifest", "i18n", "zh-CN", "apidoc", "catalog-product-price-preview.json"))
	if err != nil {
		t.Fatal(err)
	}
	var translations map[string]string
	if err = json.Unmarshal(content, &translations); err != nil {
		t.Fatal(err)
	}
	for _, typeOf := range []reflect.Type{reflect.TypeOf(CatalogProductPricePreviewReq{}), reflect.TypeOf(CatalogProductPricePreviewRes{}), reflect.TypeOf(CatalogProductPricePreview{}), reflect.TypeOf(CatalogProductPricePreviewTerm{}), reflect.TypeOf(CatalogProductPricePreviewIssue{})} {
		for index := 0; index < typeOf.NumField(); index++ {
			field := typeOf.Field(index)
			source := field.Tag.Get("dc")
			if source == "" || strings.TrimSpace(translations[source]) == "" {
				t.Fatal("missing preview documentation translation", typeOf.Name(), field.Name)
			}
		}
	}
	meta := reflect.TypeOf(CatalogProductPricePreviewReq{}).Field(0)
	if meta.Tag.Get("method") != "get" || meta.Tag.Get("permission") != "idc:admin:catalog:product:query" {
		t.Fatal("preview must remain a read-only authorized GET")
	}
}
