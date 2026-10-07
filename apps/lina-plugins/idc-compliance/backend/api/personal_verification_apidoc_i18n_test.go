// This file verifies the plugin-owned Chinese API documentation for personal verification.

package api

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"testing"

	idccompliance "lina-plugin-idc-compliance"
)

func TestPersonalVerificationAPIDocChineseBundleCoversSourceMetadata(t *testing.T) {
	t.Parallel()
	bundle := readPersonalVerificationAPIDocBundle(t, "manifest/i18n/zh-CN/apidoc")
	root := personalVerificationAPIDocPluginRoot(t)
	for _, name := range []string{
		"customer_contract_types.go",
		"customer_contract_list.go",
		"customer_order_contract_get.go",
		"customer_contract_signing_link_create.go",
		"customer_contract_authoritative_state_update.go",
		"enterprise_legal_change_create.go",
		"enterprise_verification_types.go",
		"enterprise_verification_get.go",
		"enterprise_verification_consent_create.go",
		"enterprise_verification_create.go",
		"enterprise_verification_material_create.go",
		"enterprise_verification_material_content_update.go",
		"enterprise_verification_submit.go",
		"personal_verification_types.go",
		"personal_verification_get.go",
		"personal_verification_consent_create.go",
		"personal_verification_create.go",
		"personal_verification_authoritative_state_update.go",
		"customer_red_letter_types.go",
		"customer_red_letter_task_get.go",
		"customer_red_letter_authoritative_state_update.go",
	} {
		path := filepath.Join(root, "backend", "api", "account", "v1", name)
		for _, key := range collectComplianceAPIDocKeys(t, path, "account") {
			if strings.TrimSpace(bundle[key]) == "" {
				t.Errorf("missing Simplified Chinese personal verification apidoc key %s", key)
			}
		}
	}
}

func TestEnterpriseManagementAPIDocChineseBundleCoversSourceMetadata(t *testing.T) {
	t.Parallel()
	bundle := readPersonalVerificationAPIDocBundle(t, "manifest/i18n/zh-CN/apidoc")
	root := personalVerificationAPIDocPluginRoot(t)
	for _, name := range []string{
		"electronic_contract_onboarding_types.go",
		"electronic_contract_onboarding_get.go",
		"electronic_contract_onboarding_link_create.go",
		"electronic_contract_onboarding_recheck_create.go",
		"electronic_contract_replacement_preview_create.go",
		"electronic_contract_reissue_create.go",
		"electronic_contract_termination_create.go",
		"electronic_contract_template_types.go",
		"electronic_contract_template_list.go",
		"electronic_contract_template_get.go",
		"electronic_contract_template_create.go",
		"electronic_contract_template_copy_create.go",
		"electronic_contract_template_update.go",
		"electronic_contract_template_enablement_create.go",
		"electronic_contract_template_disablement_create.go",
		"electronic_contract_template_tencent_sync_create.go",
		"electronic_contract_template_editor_get.go",
		"electronic_contract_template_source_file_update.go",
		"electronic_contract_template_online_source_update.go",
		"electronic_contract_template_pdf_preview_create.go",
		"electronic_contract_template_preview_confirmation_create.go",
		"electronic_contract_template_field_set_create.go",
		"electronic_contract_template_preview_content_get.go",
		"electronic_contract_template_provider_preview_get.go",
		"electronic_contract_template_provider_mapping_create.go",
		"enterprise_verification_types.go",
		"enterprise_verification_case_list.go",
		"enterprise_verification_case_get.go",
		"enterprise_verification_material_access_create.go",
		"enterprise_verification_material_content_create.go",
		"enterprise_verification_review_start_create.go",
		"enterprise_verification_approval_create.go",
		"enterprise_verification_rejection_create.go",
		"supplier_configuration_get.go",
		"supplier_credential_update.go",
		"supplier_subject_binding_create.go",
		"supplier_subject_revocation_create.go",
		"supplier_subject_reopening_create.go",
		"red_letter_types.go",
		"red_letter_task_get.go",
		"red_letter_allocation_resolution_create.go",
		"red_letter_manual_completion_create.go",
	} {
		path := filepath.Join(root, "backend", "api", "admin", "v1", name)
		for _, key := range collectComplianceAPIDocKeys(t, path, "admin") {
			if strings.TrimSpace(bundle[key]) == "" {
				t.Errorf("missing Simplified Chinese enterprise management apidoc key %s", key)
			}
		}
	}
}

func TestPersonalVerificationAPIDocEnglishBundleStaysEmpty(t *testing.T) {
	t.Parallel()
	if bundle := readPersonalVerificationAPIDocBundle(t, "manifest/i18n/en-US/apidoc"); len(bundle) != 0 {
		t.Fatalf("English apidoc must use API source text, found %d duplicated keys", len(bundle))
	}
}

func personalVerificationAPIDocPluginRoot(t *testing.T) string {
	t.Helper()
	_, current, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve personal verification apidoc test path")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(current), "../.."))
}

func readPersonalVerificationAPIDocBundle(t *testing.T, directory string) map[string]string {
	t.Helper()
	bundle := make(map[string]string)
	err := fs.WalkDir(idccompliance.EmbeddedFiles, directory, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".json") {
			return nil
		}
		content, err := fs.ReadFile(idccompliance.EmbeddedFiles, path)
		if err != nil {
			return err
		}
		fileBundle := make(map[string]string)
		if err = json.Unmarshal(content, &fileBundle); err != nil {
			return err
		}
		for key, value := range fileBundle {
			bundle[key] = value
		}
		return nil
	})
	if err != nil {
		t.Fatalf("read personal verification apidoc bundle %s: %v", directory, err)
	}
	return bundle
}

func collectComplianceAPIDocKeys(t *testing.T, path string, audience string) []string {
	t.Helper()
	parsed, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
	if err != nil {
		t.Fatalf("parse personal verification API source %s: %v", path, err)
	}
	keys := make([]string, 0, 64)
	ast.Inspect(parsed, func(node ast.Node) bool {
		typeSpec, ok := node.(*ast.TypeSpec)
		if !ok {
			return true
		}
		structure, ok := typeSpec.Type.(*ast.StructType)
		if !ok {
			return false
		}
		prefix := "plugins.idc_compliance.api." + audience + ".v1." + typeSpec.Name.Name
		for _, field := range structure.Fields.List {
			keys = append(keys, personalVerificationAPIDocFieldKeys(t, prefix, field)...)
		}
		return false
	})
	return keys
}

func personalVerificationAPIDocFieldKeys(t *testing.T, prefix string, field *ast.Field) []string {
	t.Helper()
	if field == nil || field.Tag == nil {
		return nil
	}
	raw, err := strconv.Unquote(field.Tag.Value)
	if err != nil {
		t.Fatalf("decode personal verification API struct tag: %v", err)
	}
	tag := reflect.StructTag(raw)
	if strings.TrimSpace(tag.Get("path")) != "" {
		keys := make([]string, 0, 4)
		for _, name := range []string{"tags", "summary"} {
			if strings.TrimSpace(tag.Get(name)) != "" {
				keys = append(keys, prefix+".meta."+name)
			}
		}
		if strings.TrimSpace(tag.Get("dc")) != "" {
			keys = append(keys, prefix+".meta.dc", prefix+".schema.dc")
		}
		return keys
	}
	if strings.TrimSpace(tag.Get("dc")) == "" {
		return nil
	}
	jsonName := strings.TrimSpace(strings.Split(tag.Get("json"), ",")[0])
	if jsonName == "" || jsonName == "-" {
		return nil
	}
	return []string{prefix + ".fields." + jsonName + ".dc"}
}
