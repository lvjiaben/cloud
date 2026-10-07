// This file statically guards the public resource contract and consumer import boundaries.

package resourcecap

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

// TestResourcecapHasNoRuntimeGlobalRegistrationAndRespectsInternalBoundaries
// ensures the Provider SPI remains an explicitly constructed declaration-time
// seam while allowing implementation-only children owned by resourcecap.
func TestResourcecapHasNoRuntimeGlobalRegistrationAndRespectsInternalBoundaries(t *testing.T) {
	root := resourcecapDirectory(t)
	const ownedInternalPrefix = "lina-plugin-idc-resource/backend/cap/resourcecap/internal/"
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
			if (strings.Contains(importPath, "/internal/") || strings.HasSuffix(importPath, "/internal")) &&
				!strings.HasPrefix(importPath, ownedInternalPrefix) {
				t.Errorf("resource contract crosses an unowned internal boundary %q in %s", importPath, path)
			}
		}

		parsed, err = parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if err != nil {
			t.Errorf("parse declarations from %s: %v", path, err)
			return nil
		}
		for _, declaration := range parsed.Decls {
			switch typed := declaration.(type) {
			case *ast.FuncDecl:
				if typed.Recv == nil && (typed.Name.Name == "init" ||
					typed.Name.Name == "Get" ||
					typed.Name.Name == "Resolve" ||
					typed.Name.Name == "Invoke" ||
					typed.Name.Name == "Lookup" ||
					typed.Name.Name == "Reset") {
					t.Errorf("declaration seam must not expose package-level %s in %s", typed.Name.Name, path)
				}
				if parsed.Name.Name == "spi" && typed.Recv == nil && ast.IsExported(typed.Name.Name) &&
					typed.Name.Name != "Declare" && typed.Name.Name != "Freeze" {
					t.Errorf("spi package exposes non-declaration startup function %s in %s", typed.Name.Name, path)
				}
				if parsed.Name.Name == "spi" && snapshotReceiverName(typed) == "Snapshot" &&
					typed.Name.Name != "Declarations" && typed.Name.Name != "Len" {
					t.Errorf("spi Snapshot exposes runtime locator method %s in %s", typed.Name.Name, path)
				}
			case *ast.GenDecl:
				if parsed.Name.Name == "spi" && typed.Tok == token.VAR {
					for _, specification := range typed.Specs {
						value, ok := specification.(*ast.ValueSpec)
						if !ok {
							t.Errorf("spi package has unsupported package-level declaration in %s", path)
							continue
						}
						for _, name := range value.Names {
							if name.Name != "startupRegistrar" && name.Name != "startupRegistrarErr" {
								t.Errorf("spi package must not hold runtime package state %s in %s", name.Name, path)
							}
						}
					}
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk resourcecap: %v", err)
	}
}

// TestProductionFreezeCallsStayInOwnerStartupComposition prevents the global
// declaration seam from becoming a runtime locator. The one permitted freeze
// belongs to the resource owner's startup job composition in backend/plugin.go.
func TestProductionFreezeCallsStayInOwnerStartupComposition(t *testing.T) {
	pluginsRoot := filepath.Clean(filepath.Join(resourcecapDirectory(t), "..", "..", "..", ".."))
	allowedPath := filepath.Clean(filepath.Join(pluginsRoot, "idc-resource", "backend", "plugin.go"))
	const spiImportPath = "lina-plugin-idc-resource/backend/cap/resourcecap/spi"
	freezeCalls := 0
	err := filepath.WalkDir(pluginsRoot, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		parsed, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if err != nil {
			t.Errorf("parse production source %s: %v", path, err)
			return nil
		}
		aliases := make(map[string]struct{})
		for _, imported := range parsed.Imports {
			importPath, unquoteErr := strconv.Unquote(imported.Path.Value)
			if unquoteErr != nil || importPath != spiImportPath {
				continue
			}
			alias := "spi"
			if imported.Name != nil {
				alias = imported.Name.Name
			}
			if alias == "." {
				t.Errorf("production source must not dot-import resource Provider SPI in %s", path)
				continue
			}
			if alias != "_" {
				aliases[alias] = struct{}{}
			}
		}
		ast.Inspect(parsed, func(node ast.Node) bool {
			selector, ok := node.(*ast.SelectorExpr)
			if !ok || selector.Sel.Name != "Freeze" {
				return true
			}
			qualifier, ok := selector.X.(*ast.Ident)
			if !ok {
				return true
			}
			if _, exists := aliases[qualifier.Name]; exists {
				freezeCalls++
				if filepath.Clean(path) != allowedPath {
					t.Errorf("production resource Provider Freeze is outside owner startup composition: %s", path)
				}
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatalf("walk production plugins: %v", err)
	}
	if freezeCalls != 3 {
		t.Errorf(
			"resource owner HTTP, Jobs, and provision-source startup callbacks must each read the frozen Provider declarations: found %d calls",
			freezeCalls,
		)
	}
}

// TestIDCPluginsDoNotImportResourceInternals ensures consumers stay on the
// resourcecap boundary rather than reaching into the owner's implementation.
func TestIDCPluginsDoNotImportResourceInternals(t *testing.T) {
	root := resourcecapDirectory(t)
	resourcePluginRoot := filepath.Clean(filepath.Join(root, "..", "..", ".."))
	pluginsRoot := filepath.Clean(filepath.Join(root, "..", "..", "..", ".."))
	const (
		forbiddenImport      = "lina-plugin-idc-resource/backend/internal"
		forbiddenCapInternal = "lina-plugin-idc-resource/backend/cap/resourcecap/internal"
	)
	err := filepath.WalkDir(pluginsRoot, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".go") {
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
			forbiddenPluginImplementation := importPath == forbiddenImport ||
				strings.HasPrefix(importPath, forbiddenImport+"/")
			resourceImplementation := importPath == forbiddenCapInternal ||
				strings.HasPrefix(importPath, forbiddenCapInternal+"/")
			insideResourceContract := path == root ||
				strings.HasPrefix(filepath.Clean(path), filepath.Clean(root)+string(filepath.Separator))
			insideResourceOwner := path == resourcePluginRoot ||
				strings.HasPrefix(filepath.Clean(path), resourcePluginRoot+string(filepath.Separator))
			if (forbiddenPluginImplementation && !insideResourceOwner) ||
				(resourceImplementation && !insideResourceContract) {
				t.Errorf("IDC plugin imports resource owner internals %q in %s", importPath, path)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk IDC plugins: %v", err)
	}
}

// resourcecapDirectory resolves the package root from this test source file.
func resourcecapDirectory(t *testing.T) string {
	t.Helper()
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve resourcecap test location")
	}
	return filepath.Dir(currentFile)
}

// snapshotReceiverName returns the direct named receiver for governance
// checks, accepting either value or pointer receivers.
func snapshotReceiverName(declaration *ast.FuncDecl) string {
	if declaration.Recv == nil || len(declaration.Recv.List) != 1 {
		return ""
	}
	receiver := declaration.Recv.List[0].Type
	if pointer, ok := receiver.(*ast.StarExpr); ok {
		receiver = pointer.X
	}
	identifier, _ := receiver.(*ast.Ident)
	if identifier == nil {
		return ""
	}
	return identifier.Name
}
