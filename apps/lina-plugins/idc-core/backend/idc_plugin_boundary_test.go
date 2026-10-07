// This file enforces source-level ownership boundaries across the IDC plugin suite.

package backend

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"testing"
)

const idcModulePrefix = "lina-plugin-idc-"

var (
	goModulePattern      = regexp.MustCompile(`(?m)^\s*module\s+(\S+)\s*$`)
	sqlReferencesPattern = regexp.MustCompile(`(?i)\breferences\s+(?:[\"` + "`" + `]?[a-z0-9_]+[\"` + "`" + `]?\s*\.\s*)?[\"` + "`" + `]?([a-z0-9_]+)`)
)

type idcPluginBoundary struct {
	directory   string
	module      string
	tablePrefix string
}

// TestIDCPluginsUseOnlyPublicCrossPluginCapabilities guards the architectural
// invariant that an IDC plugin may call another plugin only through its
// backend/cap contract. Direct internal imports and implementation-package
// imports would bypass owner validation, authorization, and lifecycle gates.
func TestIDCPluginsUseOnlyPublicCrossPluginCapabilities(t *testing.T) {
	pluginsRoot := idcPluginsRoot(t)
	plugins := discoverIDCPluginBoundaries(t, pluginsRoot)
	for _, owner := range plugins {
		err := filepath.WalkDir(owner.directory, func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() {
				if path != owner.directory && ignoredBoundaryDirectory(entry.Name()) {
					return filepath.SkipDir
				}
				return nil
			}
			if filepath.Ext(path) != ".go" {
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
				if allowedCrossPluginCompositionImport(path, imported, importPath) {
					continue
				}
				if violation := crossPluginImportViolation(owner.module, importPath); violation != "" {
					t.Errorf("%s imports %q: %s", path, importPath, violation)
				}
			}
			return nil
		})
		if err != nil {
			t.Fatalf("scan Go sources in %s: %v", owner.directory, err)
		}
	}
}

// allowedCrossPluginCompositionImport permits the one black-box PostgreSQL
// composition test to trigger idc-resource's public source-plugin registration.
// The exact blank import exposes no implementation symbol and production code
// remains governed by crossPluginImportViolation.
func allowedCrossPluginCompositionImport(path string, imported *ast.ImportSpec, importPath string) bool {
	return filepath.Base(path) == "product_fulfillment_resource_chain_postgres_test.go" &&
		imported != nil && imported.Name != nil && imported.Name.Name == "_" &&
		importPath == "lina-plugin-idc-resource/backend"
}

// TestIDCPluginsDoNotAccessForeignTables prevents reads, writes, joins, and
// foreign keys across plugin-owned schemas. Cross-plugin data must flow through
// a public capability so the owner keeps authorization and invariants intact.
func TestIDCPluginsDoNotAccessForeignTables(t *testing.T) {
	pluginsRoot := idcPluginsRoot(t)
	plugins := discoverIDCPluginBoundaries(t, pluginsRoot)
	for _, owner := range plugins {
		err := filepath.WalkDir(owner.directory, func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() {
				if path != owner.directory && ignoredBoundaryDirectory(entry.Name()) {
					return filepath.SkipDir
				}
				return nil
			}
			var searchable []string
			switch {
			case filepath.Ext(path) == ".sql":
				content, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				searchable = []string{stripSQLComments(string(content))}
			case filepath.Ext(path) == ".go" && !strings.HasSuffix(path, "_test.go"):
				literals, err := goStringLiterals(path)
				if err != nil {
					t.Errorf("parse string literals from %s: %v", path, err)
					return nil
				}
				searchable = literals
			default:
				return nil
			}

			for _, content := range searchable {
				for _, foreign := range plugins {
					if foreign.module == owner.module || !containsTablePrefix(content, foreign.tablePrefix) {
						continue
					}
					t.Errorf(
						"%s owned by %s directly references table prefix %q owned by %s; use a public capability",
						path,
						owner.module,
						foreign.tablePrefix,
						foreign.module,
					)
					if hasForeignKeyReference(content, foreign.tablePrefix) {
						t.Errorf(
							"%s declares a cross-plugin foreign key to %s; persist an opaque business reference instead",
							path,
							foreign.module,
						)
					}
				}
			}
			return nil
		})
		if err != nil {
			t.Fatalf("scan table ownership in %s: %v", owner.directory, err)
		}
	}
}

// TestCrossPluginBoundaryClassifiers covers attempts that previously could be
// hidden behind a non-internal implementation package, compile-time Go string
// assembly, SQL comment markers inside literals, or quoted foreign-key syntax.
func TestCrossPluginBoundaryClassifiers(t *testing.T) {
	const owner = "lina-plugin-idc-payment"
	for _, test := range []struct {
		name       string
		importPath string
		violation  bool
	}{
		{name: "same plugin internal", importPath: owner + "/backend/internal/service", violation: false},
		{name: "foreign public capability", importPath: idcModulePrefix + "core/backend/cap/idccap", violation: false},
		{name: "foreign internal", importPath: idcModulePrefix + "core/backend/internal/service", violation: true},
		{name: "foreign capability internal", importPath: idcModulePrefix + "core/backend/cap/idccap/internal/unsafe", violation: true},
		{name: "foreign implementation", importPath: idcModulePrefix + "core/backend/service", violation: true},
		{name: "foreign module root", importPath: idcModulePrefix + "core", violation: true},
		{name: "unrelated module", importPath: "github.com/gogf/gf/v2/frame/g", violation: false},
	} {
		t.Run(test.name, func(t *testing.T) {
			got := crossPluginImportViolation(owner, test.importPath) != ""
			if got != test.violation {
				t.Fatalf("crossPluginImportViolation(%q, %q) violation=%v, want %v", owner, test.importPath, got, test.violation)
			}
		})
	}

	compositionPath := filepath.Join("commerce", "product_fulfillment_resource_chain_postgres_test.go")
	blankImport := &ast.ImportSpec{Name: ast.NewIdent("_")}
	if !allowedCrossPluginCompositionImport(compositionPath, blankImport, "lina-plugin-idc-resource/backend") {
		t.Fatal("exact black-box composition registration import was rejected")
	}
	if allowedCrossPluginCompositionImport(compositionPath, nil, "lina-plugin-idc-resource/backend") ||
		allowedCrossPluginCompositionImport("other_test.go", blankImport, "lina-plugin-idc-resource/backend") ||
		allowedCrossPluginCompositionImport(compositionPath, blankImport, "lina-plugin-idc-resource/backend/internal/composition") {
		t.Fatal("cross-plugin composition import exception is broader than the exact registration seam")
	}

	cleaned := stripSQLComments("-- REFERENCES plugin_idc_core_secret(id)\n" +
		"/* plugin_idc_core_secret */ SELECT 1")
	if containsTablePrefix(cleaned, "plugin_idc_core_") {
		t.Fatal("SQL comments must not create table ownership findings")
	}
	if !hasForeignKeyReference(
		`FOREIGN KEY (secret_id) REFERENCES public."plugin_idc_core_secret" (id)`,
		"plugin_idc_core_",
	) {
		t.Fatal("quoted schema-qualified cross-plugin foreign key was not detected")
	}
	quotedMarkers := stripSQLComments(
		"SELECT '-- not a comment', '/* not a comment */';\n" +
			"REFERENCES plugin_idc_core_secret(id)",
	)
	if !hasForeignKeyReference(quotedMarkers, "plugin_idc_core_") {
		t.Fatal("comment markers inside SQL literals hid a later foreign key")
	}

	goSource := `package boundary

	const ownerPrefix = "plugin_"
	const ownerTable = ownerPrefix + "idc_core_secret"

	func table() string { return ("plugin_" + "idc_core_") + "audit" }
`
	path := filepath.Join(t.TempDir(), "boundary.go")
	if err := os.WriteFile(path, []byte(goSource), 0o600); err != nil {
		t.Fatalf("write temporary Go source: %v", err)
	}
	literals, err := goStringLiterals(path)
	if err != nil {
		t.Fatalf("scan assembled Go strings: %v", err)
	}
	assembled := 0
	for _, literal := range literals {
		if containsTablePrefix(literal, "plugin_idc_core_") {
			assembled++
		}
	}
	if assembled < 2 {
		t.Fatalf("compile-time string assembly findings=%d, want at least 2", assembled)
	}
}

func idcPluginsRoot(t *testing.T) string {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve current test file")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(filename), "..", ".."))
}

func discoverIDCPluginBoundaries(t *testing.T, pluginsRoot string) []idcPluginBoundary {
	t.Helper()
	directories, err := filepath.Glob(filepath.Join(pluginsRoot, "idc-*"))
	if err != nil {
		t.Fatalf("enumerate IDC plugins: %v", err)
	}
	if len(directories) == 0 {
		t.Fatal("no IDC plugins found")
	}
	plugins := make([]idcPluginBoundary, 0, len(directories))
	for _, directory := range directories {
		moduleFile := filepath.Join(directory, "go.mod")
		content, err := os.ReadFile(moduleFile)
		if err != nil {
			t.Fatalf("read %s: %v", moduleFile, err)
		}
		match := goModulePattern.FindSubmatch(content)
		if len(match) != 2 {
			t.Fatalf("%s does not declare one Go module", moduleFile)
		}
		module := string(match[1])
		if !strings.HasPrefix(module, idcModulePrefix) {
			t.Fatalf("%s declares unexpected module %q", moduleFile, module)
		}
		base := filepath.Base(directory)
		plugins = append(plugins, idcPluginBoundary{
			directory:   directory,
			module:      module,
			tablePrefix: "plugin_" + strings.ReplaceAll(base, "-", "_") + "_",
		})
	}
	sort.Slice(plugins, func(left, right int) bool {
		return plugins[left].module < plugins[right].module
	})
	return plugins
}

func ignoredBoundaryDirectory(name string) bool {
	switch name {
	case ".git", "dist", "node_modules", "vendor":
		return true
	default:
		return false
	}
}

func crossPluginImportViolation(ownerModule string, importPath string) string {
	if !strings.HasPrefix(importPath, idcModulePrefix) {
		return ""
	}
	foreignModule := importPath
	if separator := strings.IndexByte(foreignModule, '/'); separator >= 0 {
		foreignModule = foreignModule[:separator]
	}
	if foreignModule == ownerModule {
		return ""
	}
	if strings.Contains(importPath, "/internal/") || strings.HasSuffix(importPath, "/internal") {
		return "cross-plugin internal imports are forbidden"
	}
	publicCapabilityPrefix := foreignModule + "/backend/cap/"
	if strings.HasPrefix(importPath, publicCapabilityPrefix) {
		return ""
	}
	if importPath == foreignModule+"/backend/cap" {
		return ""
	}
	return "cross-plugin implementation imports bypass the public capability boundary"
}

func goStringLiterals(path string) ([]string, error) {
	parsed, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
	if err != nil {
		return nil, err
	}
	constants := make(map[string]ast.Expr)
	for _, declaration := range parsed.Decls {
		general, ok := declaration.(*ast.GenDecl)
		if !ok || general.Tok != token.CONST {
			continue
		}
		for _, specification := range general.Specs {
			values, ok := specification.(*ast.ValueSpec)
			if !ok || len(values.Values) != len(values.Names) {
				continue
			}
			for index, name := range values.Names {
				constants[name.Name] = values.Values[index]
			}
		}
	}

	seen := make(map[string]struct{})
	literals := make([]string, 0)
	ast.Inspect(parsed, func(node ast.Node) bool {
		expression, ok := node.(ast.Expr)
		if !ok {
			return true
		}
		value, resolved := compileTimeString(expression, constants, make(map[string]bool))
		if resolved {
			if _, duplicate := seen[value]; duplicate {
				return true
			}
			seen[value] = struct{}{}
			literals = append(literals, value)
		}
		return true
	})
	return literals, nil
}

func compileTimeString(
	expression ast.Expr,
	constants map[string]ast.Expr,
	resolving map[string]bool,
) (string, bool) {
	switch typed := expression.(type) {
	case *ast.BasicLit:
		if typed.Kind != token.STRING {
			return "", false
		}
		value, err := strconv.Unquote(typed.Value)
		return value, err == nil
	case *ast.BinaryExpr:
		if typed.Op != token.ADD {
			return "", false
		}
		left, leftOK := compileTimeString(typed.X, constants, resolving)
		right, rightOK := compileTimeString(typed.Y, constants, resolving)
		return left + right, leftOK && rightOK
	case *ast.ParenExpr:
		return compileTimeString(typed.X, constants, resolving)
	case *ast.Ident:
		if resolving[typed.Name] {
			return "", false
		}
		value, exists := constants[typed.Name]
		if !exists {
			return "", false
		}
		resolving[typed.Name] = true
		resolved, ok := compileTimeString(value, constants, resolving)
		delete(resolving, typed.Name)
		return resolved, ok
	default:
		return "", false
	}
}

func stripSQLComments(content string) string {
	var cleaned strings.Builder
	cleaned.Grow(len(content))
	for index := 0; index < len(content); {
		switch {
		case content[index] == '\'':
			index = copySQLQuoted(content, index, '\'', &cleaned)
		case content[index] == '"':
			index = copySQLQuoted(content, index, '"', &cleaned)
		case content[index] == '$':
			if delimiter, ok := sqlDollarQuoteDelimiter(content[index:]); ok {
				index = copySQLDollarQuoted(content, index, delimiter, &cleaned)
				continue
			}
			cleaned.WriteByte(content[index])
			index++
		case strings.HasPrefix(content[index:], "--"):
			index = skipSQLLineComment(content, index, &cleaned)
		case strings.HasPrefix(content[index:], "/*"):
			index = skipSQLBlockComment(content, index, &cleaned)
		default:
			cleaned.WriteByte(content[index])
			index++
		}
	}
	return cleaned.String()
}

func copySQLQuoted(content string, start int, quote byte, target *strings.Builder) int {
	target.WriteByte(content[start])
	for index := start + 1; index < len(content); index++ {
		target.WriteByte(content[index])
		if content[index] != quote {
			continue
		}
		if index+1 < len(content) && content[index+1] == quote {
			target.WriteByte(content[index+1])
			index++
			continue
		}
		return index + 1
	}
	return len(content)
}

func sqlDollarQuoteDelimiter(content string) (string, bool) {
	if len(content) < 2 || content[0] != '$' {
		return "", false
	}
	for index := 1; index < len(content); index++ {
		character := content[index]
		switch {
		case character == '$':
			return content[:index+1], true
		case character == '_' || character >= 'a' && character <= 'z' ||
			character >= 'A' && character <= 'Z' || index > 1 && character >= '0' && character <= '9':
			continue
		default:
			return "", false
		}
	}
	return "", false
}

func copySQLDollarQuoted(content string, start int, delimiter string, target *strings.Builder) int {
	remainder := content[start+len(delimiter):]
	closing := strings.Index(remainder, delimiter)
	if closing < 0 {
		target.WriteString(content[start:])
		return len(content)
	}
	end := start + len(delimiter) + closing + len(delimiter)
	target.WriteString(content[start:end])
	return end
}

func skipSQLLineComment(content string, start int, target *strings.Builder) int {
	index := start + 2
	for index < len(content) && content[index] != '\n' && content[index] != '\r' {
		index++
	}
	target.WriteByte(' ')
	return index
}

func skipSQLBlockComment(content string, start int, target *strings.Builder) int {
	depth := 1
	index := start + 2
	for index < len(content) && depth > 0 {
		switch {
		case strings.HasPrefix(content[index:], "/*"):
			depth++
			index += 2
		case strings.HasPrefix(content[index:], "*/"):
			depth--
			index += 2
		default:
			index++
		}
	}
	target.WriteByte(' ')
	return index
}

func containsTablePrefix(content string, tablePrefix string) bool {
	return strings.Contains(strings.ToLower(content), strings.ToLower(tablePrefix))
}

func hasForeignKeyReference(content string, tablePrefix string) bool {
	for _, match := range sqlReferencesPattern.FindAllStringSubmatch(content, -1) {
		if len(match) == 2 && strings.HasPrefix(strings.ToLower(match[1]), strings.ToLower(tablePrefix)) {
			return true
		}
	}
	return false
}
