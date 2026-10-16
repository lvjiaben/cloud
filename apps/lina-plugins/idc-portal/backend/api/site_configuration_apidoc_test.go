// This file verifies website configuration documentation stays translated with the API source.
package api

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

// TestWebsiteConfigurationAPIDocCoverage checks every documented field and metadata translation.
func TestWebsiteConfigurationAPIDocCoverage(t *testing.T) {
	root := filepath.Join("..", "..")
	data, err := os.ReadFile(filepath.Join(root, "manifest/i18n/zh-CN/apidoc/index.json"))
	if err != nil {
		t.Fatal(err)
	}
	var bundle map[string]string
	if err = json.Unmarshal(data, &bundle); err != nil {
		t.Fatal(err)
	}
	source, err := parser.ParseFile(token.NewFileSet(), filepath.Join("admin", "v1", "site_configuration.go"), nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	ast.Inspect(source, func(node ast.Node) bool {
		typ, ok := node.(*ast.TypeSpec)
		if !ok {
			return true
		}
		structure, ok := typ.Type.(*ast.StructType)
		if !ok {
			return true
		}
		for _, field := range structure.Fields.List {
			if field.Tag == nil {
				continue
			}
			raw, tagErr := strconv.Unquote(field.Tag.Value)
			if tagErr != nil {
				t.Fatal(tagErr)
			}
			tag := reflect.StructTag(raw)
			name := "meta"
			if len(field.Names) > 0 {
				name = field.Names[0].Name
			}
			if tag.Get("dc") == "" {
				t.Errorf("missing dc: %s.%s", typ.Name.Name, name)
			}
			if name != "meta" && tag.Get("eg") == "" {
				t.Errorf("missing eg: %s.%s", typ.Name.Name, name)
			}
			for _, attribute := range []string{"summary", "tags", "dc"} {
				if tag.Get(attribute) == "" {
					continue
				}
				key := "plugins.idc_portal.api.admin.v1." + typ.Name.Name + "." + name + "." + attribute
				if strings.TrimSpace(bundle[key]) == "" {
					t.Errorf("missing Chinese translation: %s", key)
				}
			}
		}
		return false
	})
	data, err = os.ReadFile(filepath.Join(root, "manifest/i18n/en-US/apidoc/index.json"))
	if err != nil {
		t.Fatal(err)
	}
	var english map[string]string
	if err = json.Unmarshal(data, &english); err != nil {
		t.Fatal(err)
	}
	if len(english) != 0 {
		t.Fatal("English API documentation must use source text")
	}
}
