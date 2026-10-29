package idccompliance

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAcceptanceMockUsesRuntimeConfigurationIdentifiers(t *testing.T) {
	content, err := os.ReadFile(filepath.Join(
		"manifest", "sql", "mock-data", "001-idc-compliance-acceptance-demo.sql",
	))
	if err != nil {
		t.Fatalf("read compliance acceptance mock: %v", err)
	}

	sql := string(content)
	for _, identifier := range []string{
		"compliance-provider-aliyun-face",
		"local-supplier-nuonuo_invoice",
	} {
		if !strings.Contains(sql, identifier) {
			t.Errorf("acceptance mock is missing runtime configuration identifier %q", identifier)
		}
	}
	for _, obsolete := range []string{
		"demo_compliance_aliyun_face",
		"demo_supplier_tencent_ess",
		"demo_supplier_nuonuo",
	} {
		if strings.Contains(sql, obsolete) {
			t.Errorf("acceptance mock still contains obsolete configuration identifier %q", obsolete)
		}
	}
}
