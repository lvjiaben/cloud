// This file verifies Chinese API documentation for customer server login
// choices and their safe purchase and cart projections.
package api

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestPurchaseServerLoginAPIDocTranslations(t *testing.T) {
	t.Parallel()
	bundle := readFinancialAPIDocBundle(t, "manifest/i18n/zh-CN/apidoc")
	files := map[string][]string{"account": {"purchase_server_login_create.go", "commerce_endpoints.go", "commerce_types.go", "purchase_configuration.go"}, "public": {"purchase_configuration_types.go"}}
	for namespace, names := range files {
		for _, name := range names {
			path := filepath.Join(financialAPIDocPluginRoot(t), "backend", "api", namespace, "v1", name)
			for _, key := range collectFinancialAPIDocKeys(t, path, namespace) {
				// Other purchase models predate this change; inspect only the new safe
				// login/presentation fields while fully checking the new endpoint.
				if name != "purchase_server_login_create.go" && !strings.Contains(key, "ServerLogin") && !strings.Contains(key, ".fields.serverLogin") && !strings.Contains(key, ".fields.hasServerLogin") && !strings.Contains(key, ".fields.presentation.") {
					continue
				}
				if strings.TrimSpace(bundle[key]) == "" {
					t.Errorf("missing purchase server login translation: %s", key)
				}
			}
		}
	}
}
