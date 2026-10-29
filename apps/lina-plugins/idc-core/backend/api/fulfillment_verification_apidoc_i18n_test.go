// This file guards the new verification route metadata and its plugin-owned translations.
package api

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestFulfillmentVerificationAPIDocTranslations(t *testing.T) {
	t.Parallel()
	bundle := readFinancialAPIDocBundle(t, "manifest/i18n/zh-CN/apidoc")
	source := filepath.Join(financialAPIDocPluginRoot(t), "backend", "api", "admin", "v1", "product_fulfillment_non_acceptance_verification.go")
	for _, key := range collectFinancialAPIDocKeys(t, source, "admin") {
		if strings.TrimSpace(bundle[key]) == "" {
			t.Errorf("missing verification API translation: %s", key)
		}
	}
}
