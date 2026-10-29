// Public list filters must retain localized metadata when the anonymous batch contract changes.

package api

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestPublicCatalogListAPIDocChineseCoverage(t *testing.T) {
	bundle := readFinancialAPIDocBundle(t, "manifest/i18n/zh-CN/apidoc")
	source := filepath.Join(financialAPIDocPluginRoot(t), "backend/api/public/v1/catalog_product_list.go")
	for _, key := range collectFinancialAPIDocKeys(t, source, "public") {
		if strings.TrimSpace(bundle[key]) == "" {
			t.Errorf("missing public catalog list metadata %s", key)
		}
	}
}
