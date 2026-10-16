// Customer management API metadata must remain covered by the Chinese bundle.

package api

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestCustomerManagementAPIDocTranslations(t *testing.T) {
	t.Parallel()
	bundle := readFinancialAPIDocBundle(t, "manifest/i18n/zh-CN/apidoc")
	for _, name := range []string{"customer_verification.go", "customer_segments_manage.go", "customer_audit_list.go", "customer_activation.go", "customer_types.go", "customer_products.go", "customer_product_edit.go", "customer_overview_get.go"} {
		path := filepath.Join(financialAPIDocPluginRoot(t), "backend", "api", "admin", "v1", name)
		for _, key := range collectFinancialAPIDocKeys(t, path, "admin") {
			if name == "customer_types.go" && !strings.Contains(key, "CustomerSummaryItem.fields.primaryEmail.dc") && !strings.Contains(key, "CustomerSummaryItem.fields.primaryPhone.dc") && !strings.Contains(key, "CustomerSummaryItem.fields.balances.dc") && !strings.Contains(key, "CustomerOperationsStatistics.") && !strings.Contains(key, "CustomerCurrencyStatistics.") && !strings.Contains(key, "CustomerTicketStatistics.") && !strings.Contains(key, "CustomerProductCategoryStatistics.") {
				continue
			}
			if strings.TrimSpace(bundle[key]) == "" {
				t.Errorf("missing customer management translation: %s", key)
			}
		}
	}
	key := "plugins.idc_core.api.admin.v1.CustomerSummaryItem.fields.primaryEmail.dc"
	if strings.TrimSpace(bundle[key]) == "" {
		t.Errorf("missing account projection translation: %s", key)
	}
}
