// This file guards the source-only secret contract until a confidential
// dynamic transport has been designed and independently reviewed.

package secretcap

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

// TestSecretcapDoesNotPublishOrdinaryDynamicPlaintextBridge prevents an
// owner descriptor from routing plaintext through ordinary invocation payload
// bytes. Source plugins must receive a scoped Accessor by constructor injection.
func TestSecretcapDoesNotPublishOrdinaryDynamicPlaintextBridge(t *testing.T) {
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve secretcap test location")
	}
	root := filepath.Dir(currentFile)
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		parsed, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
		if err != nil {
			t.Errorf("parse imports from %s: %v", path, err)
			return nil
		}
		for _, imported := range parsed.Imports {
			importPath, unquoteErr := strconv.Unquote(imported.Path.Value)
			if unquoteErr != nil {
				t.Errorf("decode import in %s: %v", path, unquoteErr)
				continue
			}
			if strings.HasSuffix(importPath, "/capability/capregistry") {
				t.Errorf("source-only secretcap imports dynamic descriptor registry in %s", path)
			}
		}
		parsed, err = parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if err != nil {
			t.Errorf("parse declarations from %s: %v", path, err)
			return nil
		}
		for _, declaration := range parsed.Decls {
			function, isFunction := declaration.(*ast.FuncDecl)
			if isFunction && function.Recv == nil && function.Name.Name == "NewDescriptor" {
				t.Errorf("source-only secretcap publishes NewDescriptor in %s", path)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk secretcap: %v", err)
	}
}
