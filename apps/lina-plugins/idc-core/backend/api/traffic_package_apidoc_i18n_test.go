// This file verifies all fixed-package API metadata in the plugin language scope.

package api

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestTrafficPackageAPIDocChineseBundleCoversSourceMetadata(t *testing.T) {
	t.Parallel()
	bundle := readFinancialAPIDocBundle(t, "manifest/i18n/zh-CN/apidoc")
	for _, area := range []string{"admin", "account"} {
		names := []string{"server_traffic_package_list.go"}
		if area == "admin" { names = append(names, "traffic_package_create.go", "traffic_package_update.go", "traffic_package_list.go", "traffic_package_product_list.go", "server_accessory_operation.go") }
		for _, name := range names {
			path := filepath.Join(financialAPIDocPluginRoot(t), "backend", "api", area, "v1", name)
			for _, key := range collectFinancialAPIDocKeys(t, path, area) {
				if strings.TrimSpace(bundle[key]) == "" { t.Errorf("missing traffic package API translation %s", key) }
			}
		}
	}
}
