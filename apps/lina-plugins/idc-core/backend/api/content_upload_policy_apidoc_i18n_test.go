package api

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestContentUploadPolicyAPIDocChineseBundleCoversSourceMetadata(t *testing.T) {
	t.Parallel()
	bundle := readFinancialAPIDocBundle(t, "manifest/i18n/zh-CN/apidoc")
	root := financialAPIDocPluginRoot(t)
	keys := collectFinancialAPIDocKeys(t, filepath.Join(root, filepath.FromSlash("backend/api/admin/v1/content.go")), "admin")
	for _, key := range keys {
		if !strings.Contains(key, ".ContentUploadPolicyReq.") && !strings.Contains(key, ".ContentUploadPolicyRes.") {
			continue
		}
		if strings.TrimSpace(bundle[key]) == "" {
			t.Errorf("missing Simplified Chinese content upload policy apidoc key %s", key)
		}
	}
}
