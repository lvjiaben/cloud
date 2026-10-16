// This file verifies that public same-system API metadata remains translated
// when configuration, import snapshot, or recovery contracts change.
package v1

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

func TestPublicCatalogChineseDocumentation(t *testing.T) {
	raw, err := os.ReadFile("../../../../manifest/i18n/zh-CN/apidoc/index.json")
	if err != nil {
		t.Fatal(err)
	}
	var bundle map[string]string
	if json.Unmarshal(raw, &bundle) != nil {
		t.Fatal("invalid translation bundle")
	}
	for _, name := range []string{"catalog_configuration.go", "peer_catalog.go", "lifecycle_read.go", "purchase_file.go"} {
		file, parseErr := parser.ParseFile(token.NewFileSet(), name, nil, 0)
		if parseErr != nil {
			t.Fatal(parseErr)
		}
		ast.Inspect(file, func(node ast.Node) bool {
			declaration, ok := node.(*ast.TypeSpec)
			if !ok {
				return true
			}
			fields, ok := declaration.Type.(*ast.StructType)
			if !ok {
				return true
			}
			for _, field := range fields.Fields.List {
				if field.Tag == nil {
					continue
				}
				value, unquoteErr := strconv.Unquote(field.Tag.Value)
				if unquoteErr != nil {
					t.Fatal(unquoteErr)
				}
				tag := reflect.StructTag(value)
				for _, kind := range []string{"summary", "tags", "dc"} {
					if tag.Get(kind) == "" {
						continue
					}
					suffix := "meta." + kind
					if len(field.Names) > 0 {
						suffix = "fields." + strings.Split(tag.Get("json"), ",")[0] + "." + kind
					}
					key := "plugins.idc_channel.api.channel.v1." + declaration.Name.Name + "." + suffix
					if strings.TrimSpace(bundle[key]) == "" {
						t.Errorf("missing translation %s", key)
					}
				}
			}
			return false
		})
	}
}
