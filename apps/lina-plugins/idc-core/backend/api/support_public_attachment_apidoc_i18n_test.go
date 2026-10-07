package api

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestSupportPublicAttachmentAPIDocChineseBundleCoversSourceMetadata(t *testing.T) {
	t.Parallel()
	bundle := readFinancialAPIDocBundle(t, "manifest/i18n/zh-CN/apidoc")
	root := financialAPIDocPluginRoot(t)
	keys := collectFinancialAPIDocKeys(t, filepath.Join(root, filepath.FromSlash("backend/api/supportpublic/v1/support.go")), "public")
	for _, key := range keys {
		if !strings.Contains(key, ".PresalesAttachmentUpload") && key != "plugins.idc_core.api.public.v1.PresalesReplyReq.fields.uploadNos.dc" {
			continue
		}
		if strings.TrimSpace(bundle[key]) == "" {
			t.Errorf("missing Simplified Chinese public support attachment apidoc key %s", key)
		}
	}
}
