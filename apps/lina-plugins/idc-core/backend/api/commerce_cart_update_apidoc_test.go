// This file guards the Chinese metadata coverage of the owned cart update API.
package api

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestCommerceCartUpdateAPIDocChineseCoverage(t *testing.T) {
	bundle := readFinancialAPIDocBundle(t, "manifest/i18n/zh-CN/apidoc")
	root := financialAPIDocPluginRoot(t)
	for _, file := range []string{"commerce_endpoints.go", "commerce_types.go"} {
		keys := collectFinancialAPIDocKeys(t, filepath.Join(root, "backend/api/account/v1", file), "account")
		for _, key := range keys {
			added := strings.Contains(key, ".CommerceCartItemUpdate") || strings.Contains(key, ".CommerceCartItemPurchase.") || strings.Contains(key, ".CommerceCartConfiguration.")
			for _, field := range []string{"configuration", "chargeTypeCode", "periodUnitCode", "periodCount"} {
				added = added || strings.Contains(key, ".CommerceCartItem.fields."+field+".")
			}
			if added && strings.TrimSpace(bundle[key]) == "" {
				t.Errorf("missing cart update Chinese metadata %s", key)
			}
		}
	}
}
