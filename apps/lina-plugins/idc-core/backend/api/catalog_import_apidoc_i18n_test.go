// This file keeps imported-draft API source metadata covered by the plugin's
// Chinese documentation bundle while English continues to use source text.

package api

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestCatalogImportAPIDocTranslations(t *testing.T) {
	t.Parallel()
	bundle := readFinancialAPIDocBundle(t, "manifest/i18n/zh-CN/apidoc")
	for _, name := range []string{"catalog_product_copy.go", "catalog_product_import.go", "catalog_product_description.go", "catalog_product_detail_types.go", "catalog_fulfillment_policy_update.go", "catalog_automation_types.go"} {
		path := filepath.Join(financialAPIDocPluginRoot(t), "backend", "api", "admin", "v1", name)
		for _, key := range collectFinancialAPIDocKeys(t, path, "admin") {
			if strings.TrimSpace(bundle[key]) == "" {
				t.Errorf("missing catalog import translation: %s", key)
			}
		}
	}
}
