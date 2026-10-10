// This file protects generated HTTP contracts and their translated metadata.
package productdirectory

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

func TestDirectoryContractMetadataHasChineseCoverage(t *testing.T) {
	_, current, _, _ := runtime.Caller(0)
	root := filepath.Clean(filepath.Join(filepath.Dir(current), "..", "..", "..", ".."))
	content, err := os.ReadFile(filepath.Join(root, "manifest/i18n/zh-CN/apidoc/product-directories.json"))
	if err != nil {
		t.Fatal(err)
	}
	translations := map[string]string{}
	if err = json.Unmarshal(content, &translations); err != nil {
		t.Fatal(err)
	}
	for _, namespace := range []string{"directoryadmin", "directoryaccount"} {
		file, err := parser.ParseFile(token.NewFileSet(), filepath.Join(root, "backend/api", namespace, "v1/directory.go"), nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		ast.Inspect(file, func(node ast.Node) bool {
			typ, ok := node.(*ast.TypeSpec)
			if !ok {
				return true
			}
			structure, ok := typ.Type.(*ast.StructType)
			if !ok {
				return true
			}
			if namespace == "directoryaccount" && strings.HasPrefix(typ.Name.Name, "Visibility") {
				t.Errorf("obsolete customer preference contract %s", typ.Name.Name)
			}
			prefix := "plugins.idc_core.api." + namespace + ".v1." + typ.Name.Name
			for _, field := range structure.Fields.List {
				if field.Tag == nil {
					continue
				}
				raw, err := strconv.Unquote(field.Tag.Value)
				if err != nil {
					t.Fatal(err)
				}
				tag := reflect.StructTag(raw)
				if namespace == "directoryaccount" && (tag.Get("json") == "hidden" || strings.Contains(tag.Get("path"), "/visibility")) {
					t.Error("customer preference contract remains exposed")
				}
				if len(field.Names) == 0 {
					for _, kind := range []string{"tags", "summary", "dc"} {
						if tag.Get(kind) != "" && translations[prefix+".meta."+kind] == "" {
							t.Errorf("missing metadata translation %s.%s", prefix, kind)
						}
					}
					if namespace == "directoryadmin" && (!strings.HasPrefix(tag.Get("path"), "/admin/") || !strings.HasPrefix(tag.Get("permission"), "idc:admin:product-directory:")) {
						t.Errorf("unprotected merchant API %s", prefix)
					}
				} else if tag.Get("dc") != "" && translations[prefix+".fields."+strings.Split(tag.Get("json"), ",")[0]+".dc"] == "" {
					t.Errorf("missing field translation %s.%s", prefix, field.Names[0].Name)
				}
			}
			return false
		})
	}
	content, err = os.ReadFile(filepath.Join(root, "manifest/i18n/en-US/apidoc/product-directories.json"))
	if err != nil {
		t.Fatal(err)
	}
	var english map[string]string
	if err = json.Unmarshal(content, &english); err != nil || len(english) != 0 {
		t.Fatalf("English uses source metadata: %v", err)
	}
}
