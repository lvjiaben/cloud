// This file verifies deterministic, complete local gateway component bundles
// and their installation-independent release metadata.

package main

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestReleaseMetadataValidation(t *testing.T) {
	if !safeMetadata.MatchString("v1.0.0-rc.1") || safeMetadata.MatchString("value with spaces") ||
		safeMetadata.MatchString("") {
		t.Fatal("release metadata validator accepted an unsafe value")
	}
}

func TestWithEnvironmentReplacesTargetKeys(t *testing.T) {
	got := withEnvironment([]string{"PATH=x", "GOOS=old", "goarch=old"}, map[string]string{
		"GOOS": "linux", "GOARCH": "amd64",
	})
	joined := strings.Join(got, "|")
	if strings.Contains(joined, "GOOS=old") || strings.Contains(joined, "goarch=old") ||
		!strings.Contains(joined, "GOOS=linux") || !strings.Contains(joined, "GOARCH=amd64") {
		t.Fatalf("unexpected environment: %s", joined)
	}
}

func TestGatewayDependencyGraphUsesOnlyReviewedLocalSupplierSDK(t *testing.T) {
	root, err := findModuleRoot()
	if err != nil {
		t.Fatalf("find module root: %v", err)
	}
	command := exec.CommandContext(context.Background(), "go", "list", "-deps", commandPackage)
	command.Dir = root
	command.Env = withEnvironment(os.Environ(), map[string]string{"GOWORK": "off"})
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("list dependency graph: %v: %s", err, output)
	}
	dependencies := strings.ToLower(string(output))
	for _, forbidden := range []string{"lina-core", "lina-plugin-", "nuonuo", "tencentcloud"} {
		if strings.Contains(dependencies, forbidden) {
			t.Fatalf("dependency graph unexpectedly contains %q", forbidden)
		}
	}
}

func TestBuildReleaseRefusesExistingArtifact(t *testing.T) {
	path := filepath.Join(t.TempDir(), "gateway-bundle")
	if err := os.Mkdir(path, 0o700); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	err := buildRelease(releaseOptions{
		OutputPath: path, GOOS: "linux", GOARCH: "amd64", Version: "test", Commit: "unknown", BuiltAt: "unknown",
	})
	if err == nil || !strings.Contains(err.Error(), "refusing to overwrite") {
		t.Fatalf("expected overwrite refusal, got %v", err)
	}
}

func TestDeliverySourcesContainConfigurationDocsAndMigrations(t *testing.T) {
	root, err := findModuleRoot()
	if err != nil {
		t.Fatalf("find module root: %v", err)
	}
	sources, err := deliverySources(root)
	if err != nil {
		t.Fatalf("collect delivery sources: %v", err)
	}
	joined := ""
	for _, source := range sources {
		joined += filepath.ToSlash(source.Destination) + "\n"
	}
	for _, required := range []string{
		"config/local-gateway.env.example",
		"docs/README.md",
		"docs/README.zh-CN.md",
		"docs/DATA-LIFECYCLE.md",
		"docs/DATA-LIFECYCLE.zh-CN.md",
		"migrations/001-identity-foundation.sql",
		"migrations/010-retire-central-settlement.sql",
		"migrations/012-local-data-lifecycle.sql",
		"migrations/013-local-supplier-subject.sql",
		"migrations/014-local-supplier-write-guard.sql",
		"migrations/015-local-callback-reconciliation.sql",
		"migrations/016-local-active-query-takeover.sql",
		"migrations/017-local-supplier-usage-runtime.sql",
		"migrations/018-local-data-lifecycle-runtime.sql",
	} {
		if !strings.Contains(joined, required) {
			t.Fatalf("delivery sources are missing %s:\n%s", required, joined)
		}
	}
}

func TestManifestDeclaresMandatoryLocalDelivery(t *testing.T) {
	manifest := artifactManifest{
		SchemaVersion: "v1", ComponentID: componentID, ProductPackageRequired: true,
		SupplierClientsIncluded: true, BusinessRoutesEnabled: true,
		LocalInstallationOnly: true, SameProgramForAllInstalls: true,
	}
	payload, err := json.Marshal(manifest)
	if err != nil {
		t.Fatalf("marshal manifest: %v", err)
	}
	encoded := string(payload)
	for _, expected := range []string{
		`"componentId":"idc-compliance-local-gateway"`,
		`"supplierClientsIncluded":true`,
		`"businessRoutesEnabled":true`,
		`"productPackageRequired":true`,
		`"localInstallationOnly":true`,
		`"sameProgramForAllInstallations":true`,
	} {
		if !strings.Contains(encoded, expected) {
			t.Fatalf("manifest is missing %s: %s", expected, encoded)
		}
	}
}
