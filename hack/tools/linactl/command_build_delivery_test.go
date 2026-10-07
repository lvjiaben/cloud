package main

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestAssembleWebDeliveryCopiesCustomerWebsiteAndLockedRuntime(t *testing.T) {
	root := t.TempDir()
	output := filepath.Join(root, "output")
	writeDeliveryTestFile(t, filepath.Join(root, "apps", "lina-vben", "apps", "web-customer", "dist", "index.html"), "customer")
	writeDeliveryTestFile(t, filepath.Join(root, "apps", "lina-websites", "sites", "default", ".output", "server", "index.mjs"), "website")
	writeDeliveryTestFile(t, filepath.Join(root, "apps", "lina-websites", "sites", "default", ".output", "public", "robots.txt"), "robots")
	runtimePath := filepath.Join(root, "fixture", targetNodeExecutableName(runtime.GOOS))
	writeDeliveryTestFile(t, runtimePath, "locked-runtime")

	target := targetPlatform{OS: runtime.GOOS, Arch: runtime.GOARCH}
	suffix := targetRuntimeEnvironmentSuffix(target)
	env := []string{
		"LINAPRO_SITE_NODE_RUNTIME_" + suffix + "=" + runtimePath,
		"LINAPRO_SITE_NODE_RUNTIME_VERSION_" + suffix + "=v24.20.0",
	}
	application := &app{root: root, env: env, stdout: io.Discard, stderr: io.Discard}
	options := buildOptions{BinaryName: "lina", OutputDir: output, Targets: []targetPlatform{target}}
	if err := assembleWebDelivery(context.Background(), application, env, options, target, output); err != nil {
		t.Fatalf("assemble delivery: %v", err)
	}

	for _, relative := range []string{
		"web-customer/index.html",
		"website/default/server/index.mjs",
		"website/default/public/robots.txt",
		filepath.ToSlash(filepath.Join("runtime", targetNodeExecutableName(target.OS))),
		deliveryManifestName,
	} {
		if _, err := os.Stat(filepath.Join(output, filepath.FromSlash(relative))); err != nil {
			t.Fatalf("expected delivery file %s: %v", relative, err)
		}
	}

	content, err := os.ReadFile(filepath.Join(output, deliveryManifestName))
	if err != nil {
		t.Fatal(err)
	}
	var manifest deliveryManifest
	if err = json.Unmarshal(content, &manifest); err != nil {
		t.Fatal(err)
	}
	if manifest.RuntimeVersion != "v24.20.0" || manifest.Ports.API != 6030 || manifest.Ports.Admin != 6031 || manifest.Ports.Customer != 6032 {
		t.Fatalf("unexpected delivery manifest: %#v", manifest)
	}
	if !strings.HasPrefix(manifest.Runtime.SHA256, "sha256:") || !strings.HasPrefix(manifest.WebsiteArtifact.SHA256, "sha256:") {
		t.Fatalf("expected locked runtime and website digests: %#v", manifest)
	}
}

func TestResolveSiteNodeRuntimeFailsClosedForMissingCrossTargetRuntime(t *testing.T) {
	target := targetPlatform{OS: "linux", Arch: "arm64"}
	if runtime.GOOS == target.OS && runtime.GOARCH == target.Arch {
		target = targetPlatform{OS: "windows", Arch: "amd64"}
	}
	application := &app{lookPath: func(string) (string, error) { return "", os.ErrNotExist }}
	_, _, err := resolveSiteNodeRuntime(context.Background(), application, nil, target)
	if err == nil || !strings.Contains(err.Error(), "LINAPRO_SITE_NODE_RUNTIME_") {
		t.Fatalf("expected actionable cross-target runtime error, got %v", err)
	}
}

func TestTargetExecutableNameUsesDeliveryTarget(t *testing.T) {
	if got := targetExecutableName("lina", "windows"); got != "lina.exe" {
		t.Fatalf("windows target name = %q", got)
	}
	if got := targetExecutableName("lina", "linux"); got != "lina" {
		t.Fatalf("linux target name = %q", got)
	}
}

func TestCleanBuildOutputDirPreservesDynamicPluginRuntimeStore(t *testing.T) {
	root := t.TempDir()
	output := filepath.Join(root, "temp", "output")
	writeDeliveryTestFile(t, filepath.Join(output, "lina.exe"), "old host binary")
	writeDeliveryTestFile(t, filepath.Join(output, "idc-local.wasm"), "staged plugin")
	writeDeliveryTestFile(t, filepath.Join(output, ".capability-storage", "idc-local", "private.bin"), "plugin data")
	writeDeliveryTestFile(t, filepath.Join(output, ".runtime", "idc-local", "runtime-plugin.wasm"), "guest runtime")
	writeDeliveryTestFile(t, filepath.Join(output, "releases", "idc-local", "v1.0.0", "checksum", "idc-local.wasm"), "active release")

	if err := cleanBuildOutputDir(root, output); err != nil {
		t.Fatalf("clean build output: %v", err)
	}
	for _, relative := range []string{
		"idc-local.wasm",
		filepath.Join(".capability-storage", "idc-local", "private.bin"),
		filepath.Join(".runtime", "idc-local", "runtime-plugin.wasm"),
		filepath.Join("releases", "idc-local", "v1.0.0", "checksum", "idc-local.wasm"),
	} {
		if _, err := os.Stat(filepath.Join(output, relative)); err != nil {
			t.Fatalf("persistent plugin artifact %s was removed: %v", relative, err)
		}
	}
	if _, err := os.Stat(filepath.Join(output, "lina.exe")); !os.IsNotExist(err) {
		t.Fatalf("expected generated host binary to be removed, got %v", err)
	}
}

func writeDeliveryTestFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
		t.Fatal(err)
	}
}
