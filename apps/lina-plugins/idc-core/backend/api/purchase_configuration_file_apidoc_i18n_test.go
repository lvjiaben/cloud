package api

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestPurchaseConfigurationFileAPIDocChineseBundleCoversSourceMetadata(t *testing.T) {
	t.Parallel()
	bundle := readFinancialAPIDocBundle(t, "manifest/i18n/zh-CN/apidoc")
	root := financialAPIDocPluginRoot(t)
	keys := collectFinancialAPIDocKeys(t, filepath.Join(root, filepath.FromSlash("backend/api/account/v1/purchase_configuration_file.go")), "account")
	for _, key := range keys {
		if !strings.Contains(key, ".PurchaseConfigurationFileUpload") {
			continue
		}
		if strings.TrimSpace(bundle[key]) == "" {
			t.Errorf("missing Simplified Chinese purchase configuration file apidoc key %s", key)
		}
	}
}

func TestPublicPurchaseConfigurationFileRequirementsAPIDocChineseBundleCoversSourceMetadata(t *testing.T) {
	t.Parallel()
	bundle := readFinancialAPIDocBundle(t, "manifest/i18n/zh-CN/apidoc")
	root := financialAPIDocPluginRoot(t)
	keys := collectFinancialAPIDocKeys(t, filepath.Join(root, filepath.FromSlash("backend/api/public/v1/purchase_configuration_types.go")), "public")
	for _, key := range keys {
		if !strings.Contains(key, ".PurchaseConfigurationUploadPolicy.") &&
			!strings.Contains(key, ".PurchaseConfigurationField.fields.uploadPolicy.") &&
			!strings.Contains(key, ".PurchaseConfigurationValue.fields.fileTokens.") {
			continue
		}
		if strings.TrimSpace(bundle[key]) == "" {
			t.Errorf("missing Simplified Chinese public purchase-file requirement key %s", key)
		}
	}
}

func TestAccountPurchaseConfigurationAPIDocChineseBundleCoversSourceMetadata(t *testing.T) {
	t.Parallel()
	bundle := readFinancialAPIDocBundle(t, "manifest/i18n/zh-CN/apidoc")
	root := financialAPIDocPluginRoot(t)
	keys := collectFinancialAPIDocKeys(t, filepath.Join(root, filepath.FromSlash("backend/api/account/v1/purchase_configuration.go")), "account")
	for _, key := range keys {
		if strings.TrimSpace(bundle[key]) == "" {
			t.Errorf("missing Simplified Chinese account purchase configuration key %s", key)
		}
	}
}
