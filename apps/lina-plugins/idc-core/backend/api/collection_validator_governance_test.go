// This file prevents GoFrame string-length validators from being applied to IDC API collections.

package api

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// TestIDCAPICollectionsDoNotUseStringLengthRules scans every IDC plugin API contract.
func TestIDCAPICollectionsDoNotUseStringLengthRules(t *testing.T) {
	t.Parallel()
	pluginsRoot := filepath.Clean(filepath.Join("..", "..", ".."))
	entries, err := os.ReadDir(pluginsRoot)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if !entry.IsDir() || !strings.HasPrefix(entry.Name(), "idc-") {
			continue
		}
		apiRoot := filepath.Join(pluginsRoot, entry.Name(), "backend", "api")
		if _, statErr := os.Stat(apiRoot); statErr != nil {
			if os.IsNotExist(statErr) {
				continue
			}
			t.Fatal(statErr)
		}
		walkErr := filepath.WalkDir(apiRoot, func(path string, item os.DirEntry, walkErr error) error {
			if walkErr != nil || item.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return walkErr
			}
			parsed, parseErr := parser.ParseFile(token.NewFileSet(), path, nil, 0)
			if parseErr != nil {
				return parseErr
			}
			ast.Inspect(parsed, func(node ast.Node) bool {
				field, ok := node.(*ast.Field)
				if !ok || field.Tag == nil {
					return true
				}
				array, ok := field.Type.(*ast.ArrayType)
				if !ok || array.Len != nil {
					return true
				}
				validation := reflect.StructTag(strings.Trim(field.Tag.Value, "`")).Get("v")
				for _, rule := range []string{"length:", "min-length:", "max-length:", "size:"} {
					if strings.Contains(validation, rule) {
						t.Errorf("%s uses string rule %q on a collection", path, rule)
					}
				}
				return true
			})
			return nil
		})
		if walkErr != nil {
			t.Fatal(walkErr)
		}
	}
}
