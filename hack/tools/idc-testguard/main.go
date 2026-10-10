// Command idc-testguard runs every IDC PostgreSQL regression test except explicit
// code-generation fixtures and fails when a required test is skipped or missing.
// Each plugin test owns an isolated schema; the configured DSNs must point to a
// disposable test database, never an application database.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

// testEvent describes the go test -json result fields needed by the gate.
type testEvent struct{ Action, Test, Output string }

// requiredTests discovers real PostgreSQL tests without including codegen setup.
func requiredTests(dir string) ([]string, error) {
	files, err := filepath.Glob(filepath.Join(dir, "*_test.go"))
	if err != nil {
		return nil, err
	}
	var names []string
	for _, file := range files {
		parsed, parseErr := parser.ParseFile(token.NewFileSet(), file, nil, 0)
		if parseErr != nil {
			return nil, parseErr
		}
		for _, decl := range parsed.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Recv != nil {
				continue
			}
			name := fn.Name.Name
			// Principal identity regressions share an explicitly opted-in DB
			// fixture. Reject a new unlabeled caller instead of silently omitting it.
			if strings.HasPrefix(name, "Test") && !strings.Contains(name, "PostgreSQL") {
				usesPrincipalDatabase := false
				ast.Inspect(fn.Body, func(node ast.Node) bool {
					call, ok := node.(*ast.CallExpr)
					if ok {
						if callee, ok := call.Fun.(*ast.Ident); ok && callee.Name == "configurePrincipalIntegrationDB" {
							usesPrincipalDatabase = true
						}
					}
					return true
				})
				if usesPrincipalDatabase {
					return nil, fmt.Errorf("database regression %s in %s must include PostgreSQL in its test name", name, file)
				}
			}
			// This versioned performance manifest has its own explicit opt-in
			// environment and dataset contract; ordinary CI must not run it.
			if name == "TestCheckoutBenchmarkManifestPostgreSQL" {
				continue
			}
			if strings.HasPrefix(name, "Test") && strings.Contains(name, "PostgreSQL") && !strings.Contains(name, "Codegen") {
				names = append(names, name)
			}
		}
	}
	return names, nil
}

// verifyResults rejects skipped, failed, or absent required tests.
func verifyResults(output []byte, required []string, log io.Writer) error {
	results := make(map[string]string)
	decoder := json.NewDecoder(bytes.NewReader(output))
	for {
		var event testEvent
		err := decoder.Decode(&event)
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("invalid go test JSON: %w", err)
		}
		if event.Output != "" {
			if _, err = io.WriteString(log, event.Output); err != nil {
				return err
			}
		}
		if event.Test != "" && (event.Action == "pass" || event.Action == "skip" || event.Action == "fail") {
			results[event.Test] = event.Action
		}
	}
	for _, name := range required {
		if results[name] != "pass" {
			return fmt.Errorf("required integration test %s did not pass (result=%q)", name, results[name])
		}
		for child, result := range results {
			if strings.HasPrefix(child, name+"/") && result != "pass" {
				return fmt.Errorf("required integration subtest %s did not pass (result=%q)", child, result)
			}
		}
	}
	return nil
}

// requiredSuite binds each required test set to its own Go package.
type requiredSuite struct {
	dir   string
	names []string
}

// requiredSuites includes nested service packages without conflating equal test names.
func requiredSuites(root string) ([]requiredSuite, error) {
	var suites []requiredSuite
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() {
			return nil
		}
		if path != root && (entry.Name() == "testdata" || entry.Name() == "vendor" || strings.HasPrefix(entry.Name(), ".")) {
			return filepath.SkipDir
		}
		files, err := filepath.Glob(filepath.Join(path, "*_test.go"))
		if err != nil {
			return err
		}
		if len(files) == 0 {
			return nil
		}
		names, err := requiredTests(path)
		if err != nil {
			return err
		}
		if len(names) != 0 {
			suites = append(suites, requiredSuite{dir: path, names: names})
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if len(suites) == 0 {
		return nil, fmt.Errorf("no PostgreSQL tests found in %s", root)
	}
	return suites, nil
}

// run executes independent plugin database suites serially and verifies results.
func run(ctx context.Context) error {
	plugins := []string{"core", "payment", "resource", "compliance", "notify"}
	for _, plugin := range plugins {
		env := "LINAPRO_IDC_" + strings.ToUpper(plugin) + "_TEST_DSN"
		if strings.TrimSpace(os.Getenv(env)) == "" {
			return fmt.Errorf("%s must identify a disposable PostgreSQL test database", env)
		}
	}
	for _, plugin := range plugins {
		dir := filepath.Join("apps", "lina-plugins", "idc-"+plugin, "backend")
		suites, err := requiredSuites(dir)
		if err != nil {
			return err
		}
		for _, suite := range suites {
			names := suite.names
			escaped := make([]string, len(names))
			for i, name := range names {
				escaped[i] = regexp.QuoteMeta(name)
			}
			cmd := exec.CommandContext(ctx, "go", "test", "-json", "-count=1", "-p=1", "-parallel=1", "-run", "^("+strings.Join(escaped, "|")+")$", ".")
			// Plugin modules declare local replacements; avoid depending on the active
			// host-only or plugin-full development workspace.
			cmd.Env = append(os.Environ(), "GOWORK=off")
			cmd.Dir = suite.dir
			cmd.Stderr = os.Stderr
			output, commandErr := cmd.Output()
			if err = verifyResults(output, names, os.Stdout); err != nil {
				return fmt.Errorf("idc-%s: %w", plugin, err)
			}
			if commandErr != nil {
				return commandErr
			}
			fmt.Printf("IDC PostgreSQL gate: %s passed %d required tests\n", suite.dir, len(names))
		}
	}
	return nil
}

// main fails CI when the real database regression suites are not executed.
func main() {
	if err := run(context.Background()); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
