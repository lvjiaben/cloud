// This file verifies that missing or skipped database coverage fails the gate.
package main

import (
	"io"
	"os"
	"path/filepath"
	"testing"
)

// TestVerifyResults proves missing, skipped, failed and malformed results fail.
func TestVerifyResults(t *testing.T) {
	for _, test := range []struct {
		name, events string
		wantError    bool
	}{
		{"pass", `{"Action":"pass","Test":"TestMoneyPostgreSQL"}`, false},
		{"skip", `{"Action":"skip","Test":"TestMoneyPostgreSQL"}`, true},
		{"missing", `{"Action":"pass"}`, true},
		{"fail", `{"Action":"fail","Test":"TestMoneyPostgreSQL"}`, true},
		{"invalid", "invalid", true},
		{"skipped subtest", "{\"Action\":\"skip\",\"Test\":\"TestMoneyPostgreSQL/critical_scenario\"}\n{\"Action\":\"pass\",\"Test\":\"TestMoneyPostgreSQL\"}", true},
	} {
		t.Run(test.name, func(t *testing.T) {
			err := verifyResults([]byte(test.events), []string{"TestMoneyPostgreSQL"}, io.Discard)
			if (err != nil) != test.wantError {
				t.Fatalf("unexpected result %v", err)
			}
		})
	}
}

// TestRequiredTestsExcludesOnlyCodegenFixtures verifies discovery preserves regressions.
func TestRequiredTestsExcludesOnlyCodegenFixtures(t *testing.T) {
	dir := t.TempDir()
	content := "package example\nfunc TestMoneyPostgreSQL(t *testing.T) {}\nfunc TestMoneyCodegenPostgreSQL(t *testing.T) {}\nfunc TestCheckoutBenchmarkManifestPostgreSQL(t *testing.T) {}\nfunc TestBenchmarkResultPersistencePostgreSQL(t *testing.T) {}\nfunc TestUnit(t *testing.T) {}\n"
	if err := os.WriteFile(filepath.Join(dir, "money_test.go"), []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	names, err := requiredTests(dir)
	if err != nil || len(names) != 2 || names[0] != "TestMoneyPostgreSQL" || names[1] != "TestBenchmarkResultPersistencePostgreSQL" {
		t.Fatalf("tests=%v error=%v", names, err)
	}
}

// TestRequiredSuitesIncludesNestedPackages prevents new service tests from escaping CI.
func TestRequiredSuitesIncludesNestedPackages(t *testing.T) {
	root := t.TempDir()
	for _, directory := range []string{".", "internal/service/directory", "internal/service/unit", "testdata"} {
		dir := filepath.Join(root, directory)
		if err := os.MkdirAll(dir, 0700); err != nil {
			t.Fatal(err)
		}
		name := "TestMoneyPostgreSQL"
		if directory == "internal/service/unit" {
			name = "TestUnit"
		}
		content := "package example\nfunc " + name + "(t *testing.T) {}\n"
		if err := os.WriteFile(filepath.Join(dir, "money_test.go"), []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}
	suites, err := requiredSuites(root)
	if err != nil || len(suites) != 2 {
		t.Fatalf("suites=%v error=%v", suites, err)
	}
	if suites[0].dir != root || suites[1].dir != filepath.Join(root, "internal/service/directory") {
		t.Fatalf("unexpected package discovery: %v", suites)
	}
	if suites[0].names[0] != suites[1].names[0] {
		t.Fatal("fixture must test independently bound equal names")
	}
}

// TestRequiredTestsRejectsUnlabeledPrincipalDatabaseCaller prevents future
// identity concurrency coverage from escaping the mandatory database gate.
func TestRequiredTestsRejectsUnlabeledPrincipalDatabaseCaller(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "principal_test.go"), []byte("package example\nfunc TestNewIdentityRace(t *testing.T) { configurePrincipalIntegrationDB(t, ctx) }\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := requiredTests(dir); err == nil {
		t.Fatal("unlabeled database regression silently omitted")
	}
}
