// Currency API metadata is verified against the same runtime key shape as other financial APIs.
package api

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestCurrencyAPIDocTranslations(t *testing.T) {
	t.Parallel()
	bundle := readFinancialAPIDocBundle(t, "manifest/i18n/zh-CN/apidoc")
	for area, names := range map[string][]string{"admin": {"currency_list.go", "currency_create.go", "currency_update.go", "currency_delete.go"}, "public": {"currency_options.go"}} {
		for _, name := range names {
			source := filepath.Join(financialAPIDocPluginRoot(t), "backend", "api", area, "v1", name)
			for _, key := range collectFinancialAPIDocKeys(t, source, area) {
				if strings.TrimSpace(bundle[key]) == "" {
					t.Errorf("missing currency API translation: %s", key)
				}
			}
		}
	}
}
