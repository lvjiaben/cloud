// This file verifies the plugin-owned Chinese API documentation for customer server operations.

package api

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestServerOperationAPIDocChineseBundleCoversSourceMetadata(t *testing.T) {
	t.Parallel()
	bundle := readFinancialAPIDocBundle(t, "manifest/i18n/zh-CN/apidoc")
	for _, name := range []string{"server_operation.go", "server_accessory_quote.go", "server_transfer.go", "ssh_public_key.go"} {
		path := filepath.Join(financialAPIDocPluginRoot(t),
			filepath.FromSlash("backend/api/account/v1/"+name))
		for _, key := range collectFinancialAPIDocKeys(t, path, "account") {
			if strings.TrimSpace(bundle[key]) == "" {
				t.Errorf("missing Simplified Chinese server-operation apidoc key %s", key)
			}
		}
	}
}
