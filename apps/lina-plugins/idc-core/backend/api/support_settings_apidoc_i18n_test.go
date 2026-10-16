package api

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestSupportSettingsAPIDocChineseBundleCoversSourceMetadata(t *testing.T) {
	t.Parallel()
	bundle := readFinancialAPIDocBundle(t, "manifest/i18n/zh-CN/apidoc")
	root := financialAPIDocPluginRoot(t)
	keys := collectFinancialAPIDocKeys(t, filepath.Join(root, filepath.FromSlash("backend/api/admin/v1/support_settings.go")), "admin")
	keys = append(keys, collectFinancialAPIDocKeys(t, filepath.Join(root, filepath.FromSlash("backend/api/admin/v1/support_organization_departments.go")), "admin")...)
	publicKeys := collectFinancialAPIDocKeys(t, filepath.Join(root, filepath.FromSlash("backend/api/supportpublic/v1/support.go")), "public")
	for _, key := range publicKeys {
		if supportAnonymousAPIDocKey(key) {
			keys = append(keys, key)
		}
	}
	for _, key := range keys {
		if strings.TrimSpace(bundle[key]) == "" {
			t.Errorf("missing Simplified Chinese support apidoc key %s", key)
		}
	}
}

func supportAnonymousAPIDocKey(key string) bool {
	for _, name := range []string{"PresalesCloseReq", "PresalesCloseRes", "PresalesReopenReq", "PresalesRatingReq"} {
		if strings.Contains(key, "."+name+".") {
			return true
		}
	}
	return false
}
