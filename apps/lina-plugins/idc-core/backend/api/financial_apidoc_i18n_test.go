// This file verifies the plugin-owned Chinese API documentation for financial contracts.

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

	idccore "lina-plugin-idc-core"
)

func TestFinancialAPIDocChineseBundleCoversSourceMetadata(t *testing.T) {
	t.Parallel()
	bundle := readFinancialAPIDocBundle(t, "manifest/i18n/zh-CN/apidoc")
	root := financialAPIDocPluginRoot(t)
	files := []struct {
		path  string
		scope string
	}{
		{path: "backend/api/account/v1/financial_types.go", scope: "account"},
		{path: "backend/api/account/v1/trial_conversion.go", scope: "account"},
		{path: "backend/api/upgrades/v1/upgrades_read.go", scope: "upgrades"},
		{path: "backend/api/upgrades/v1/upgrades_write.go", scope: "upgrades"},
		{path: "backend/api/account/v1/financial_endpoints.go", scope: "account"},
		{path: "backend/api/account/v1/refund_endpoints.go", scope: "account"},
		{path: "backend/api/account/v1/refund_list.go", scope: "account"},
		{path: "backend/api/admin/v1/financial_types.go", scope: "admin"},
		{path: "backend/api/admin/v1/financial_endpoints.go", scope: "admin"},
		{path: "backend/api/admin/v1/financial_operations.go", scope: "admin"},
		{path: "backend/api/admin/v1/refund_endpoints.go", scope: "admin"},
		{path: "backend/api/admin/v1/refund_directory.go", scope: "admin"},
		{path: "backend/api/account/v1/credit_types.go", scope: "account"},
		{path: "backend/api/account/v1/credit_overview.go", scope: "account"},
		{path: "backend/api/account/v1/credit_application.go", scope: "account"},
		{path: "backend/api/account/v1/credit_statement.go", scope: "account"},
		{path: "backend/api/account/v1/credit_balance_repayment.go", scope: "account"},
		{path: "backend/api/admin/v1/credit_types.go", scope: "admin"},
		{path: "backend/api/admin/v1/credit_application.go", scope: "admin"},
		{path: "backend/api/admin/v1/credit_account.go", scope: "admin"},
		{path: "backend/api/admin/v1/credit_statement.go", scope: "admin"},
		{path: "backend/api/admin/v1/credit_policy.go", scope: "admin"},
	}
	for _, file := range files {
		keys := collectFinancialAPIDocKeys(t, filepath.Join(root, filepath.FromSlash(file.path)), file.scope)
		for _, key := range keys {
			if strings.TrimSpace(bundle[key]) == "" {
				t.Errorf("missing Simplified Chinese financial apidoc key %s", key)
			}
		}
	}
}

func TestFinancialAPIDocEnglishBundleStaysEmpty(t *testing.T) {
	t.Parallel()
	if bundle := readFinancialAPIDocBundle(t, "manifest/i18n/en-US/apidoc"); len(bundle) != 0 {
		t.Fatalf("English apidoc must use API source text, found %d duplicated keys", len(bundle))
	}
}

func financialAPIDocPluginRoot(t *testing.T) string {
	t.Helper()
	_, current, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve financial apidoc test path")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(current), "../.."))
}

func readFinancialAPIDocBundle(t *testing.T, directory string) map[string]string {
	t.Helper()
	bundle := make(map[string]string)
	err := fs.WalkDir(idccore.EmbeddedFiles, directory, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".json") {
			return nil
		}
		content, err := fs.ReadFile(idccore.EmbeddedFiles, path)
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
		t.Fatalf("read financial apidoc bundle %s: %v", directory, err)
	}
	return bundle
}

func collectFinancialAPIDocKeys(t *testing.T, path string, scope string) []string {
	t.Helper()
	parsed, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
	if err != nil {
		t.Fatalf("parse financial API source %s: %v", path, err)
	}
	keys := make([]string, 0, 256)
	ast.Inspect(parsed, func(node ast.Node) bool {
		typeSpec, ok := node.(*ast.TypeSpec)
		if !ok {
			return true
		}
		structure, ok := typeSpec.Type.(*ast.StructType)
		if !ok {
			return false
		}
		prefix := "plugins.idc_core.api." + scope + ".v1." + typeSpec.Name.Name
		for _, field := range structure.Fields.List {
			keys = append(keys, financialAPIDocFieldKeys(t, prefix, field)...)
		}
		return false
	})
	return keys
}

func financialAPIDocFieldKeys(t *testing.T, prefix string, field *ast.Field) []string {
	t.Helper()
	if field == nil || field.Tag == nil {
		return nil
	}
	raw, err := strconv.Unquote(field.Tag.Value)
	if err != nil {
		t.Fatalf("decode financial API struct tag: %v", err)
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
