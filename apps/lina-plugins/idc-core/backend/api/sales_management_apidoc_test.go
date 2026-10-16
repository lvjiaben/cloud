// This test requires every new management API description to have a Chinese
// translation, including response money/time fields and command metadata.
package api

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

func TestSalesManagementAPIDocChineseTranslation(t *testing.T) {
	bundle := readFinancialAPIDocBundle(t, "manifest/i18n/zh-CN/apidoc")
	files, err := filepath.Glob("admin/v1/sales_management*.go")
	if err != nil || len(files) == 0 {
		t.Fatalf("management DTO files: %v", err)
	}
	for _, path := range files {
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		ast.Inspect(file, func(node ast.Node) bool {
			spec, ok := node.(*ast.TypeSpec)
			if !ok {
				return true
			}
			body, ok := spec.Type.(*ast.StructType)
			if !ok {
				return true
			}
			prefix := "plugins.idc_core.api.admin.v1." + spec.Name.Name
			for _, field := range body.Fields.List {
				if field.Tag == nil {
					continue
				}
				raw, err := strconv.Unquote(field.Tag.Value)
				if err != nil {
					t.Fatal(err)
				}
				tags := reflect.StructTag(raw)
				if len(field.Names) == 0 {
					for _, key := range []string{"tags", "summary", "dc"} {
						if tags.Get(key) != "" && bundle[prefix+".meta."+key] == "" {
							t.Errorf("missing translation %s.meta.%s", prefix, key)
						}
					}
					continue
				}
				name := strings.Split(tags.Get("json"), ",")[0]
				if name != "" && tags.Get("dc") != "" && bundle[prefix+".fields."+name+".dc"] == "" {
					t.Errorf("missing translation %s.fields.%s.dc", prefix, name)
				}
			}
			return true
		})
	}
}
