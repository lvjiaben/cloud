package api

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestConfigurationWorkspaceAPIDocChineseCoverage(t *testing.T) {
	bundle := readFinancialAPIDocBundle(t, "manifest/i18n/zh-CN/apidoc")
	root := financialAPIDocPluginRoot(t)
	for _, file := range []struct{ path, scope string }{{"backend/api/admin/v1/catalog_automation_types.go", "admin"}, {"backend/api/admin/v1/catalog_configuration_price_dimension.go", "admin"}, {"backend/api/admin/v1/catalog_configuration_price_batch.go", "admin"}, {"backend/api/public/v1/purchase_configuration_types.go", "public"}} {
		for _, key := range collectFinancialAPIDocKeys(t, filepath.Join(root, file.path), file.scope) {
			if strings.TrimSpace(bundle[key]) == "" {
				t.Errorf("missing Chinese configuration workspace documentation: %s", key)
			}
		}
	}
}
