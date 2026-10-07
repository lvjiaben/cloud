// This file verifies Simplified Chinese API-document coverage for support custom-field and entitlement-association contracts.
package api

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestSupportCustomFieldsAPIDocChineseBundleCoversSourceMetadata(t *testing.T) {
	t.Parallel()
	bundle := readFinancialAPIDocBundle(t, "manifest/i18n/zh-CN/apidoc")
	root := financialAPIDocPluginRoot(t)
	files := []struct {
		path  string
		scope string
	}{
		{path: "backend/api/admin/v1/support_custom_fields.go", scope: "admin"},
		{path: "backend/api/supportaccount/v1/support_custom_fields.go", scope: "supportaccount"},
		{path: "backend/api/supportpublic/v1/support_custom_fields.go", scope: "public"},
	}
	for _, file := range files {
		keys := collectFinancialAPIDocKeys(t, filepath.Join(root, filepath.FromSlash(file.path)), file.scope)
		for _, key := range keys {
			if strings.TrimSpace(bundle[key]) == "" {
				t.Errorf("missing Simplified Chinese support custom fields apidoc key %s", key)
			}
		}
	}
	for _, key := range []string{
		"plugins.idc_core.api.admin.v1.TicketItem.fields.relatedEntitlement.dc",
		"plugins.idc_core.api.admin.v1.TicketDetail.fields.customFields.dc",
		"plugins.idc_core.api.admin.v1.TicketCreateReq.fields.customFields.dc",
		"plugins.idc_core.api.supportaccount.v1.TicketItem.fields.relatedEntitlement.dc",
		"plugins.idc_core.api.supportaccount.v1.TicketDetail.fields.customFields.dc",
		"plugins.idc_core.api.supportaccount.v1.TicketCreateReq.fields.associationReference.dc",
		"plugins.idc_core.api.supportaccount.v1.TicketCreateReq.fields.customFields.dc",
	} {
		if strings.TrimSpace(bundle[key]) == "" {
			t.Errorf("missing Simplified Chinese support custom fields apidoc key %s", key)
		}
	}
}
