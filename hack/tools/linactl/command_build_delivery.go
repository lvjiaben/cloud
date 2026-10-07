// This file assembles target-specific customer and website runtime artifacts.

package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"linactl/internal/fileutil"
	"linactl/internal/toolutil"
)

const deliveryManifestName = "delivery.json"

type deliveryPorts struct {
	API      int `json:"api"`
	Admin    int `json:"admin"`
	Customer int `json:"customer"`
	Website  int `json:"website"`
}

type deliveryWebsite struct {
	Entry      string `json:"entry"`
	HealthPath string `json:"healthPath"`
	Public     string `json:"public"`
	Runtime    string `json:"runtime"`
}

type deliveryArtifact struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}

type deliveryManifest struct {
	SchemaVersion    int              `json:"schemaVersion"`
	Target           string           `json:"target"`
	HostBinary       string           `json:"hostBinary"`
	AdminFrontend    string           `json:"adminFrontend"`
	CustomerFrontend deliveryArtifact `json:"customerFrontend"`
	Website          deliveryWebsite  `json:"website"`
	WebsiteArtifact  deliveryArtifact `json:"websiteArtifact"`
	Runtime          deliveryArtifact `json:"runtime"`
	RuntimeVersion   string           `json:"runtimeVersion"`
	Ports            deliveryPorts    `json:"ports"`
}

func deliveryPackageRoot(outputDir string, target targetPlatform, multiPlatform bool) string {
	if !multiPlatform {
		return outputDir
	}
	return filepath.Join(outputDir, target.OS+"_"+target.Arch)
}

func targetExecutableName(name string, targetOS string) string {
	if targetOS == "windows" && filepath.Ext(name) == "" {
		return name + ".exe"
	}
	return name
}

func targetNodeExecutableName(targetOS string) string {
	if targetOS == "windows" {
		return "node.exe"
	}
	return "node"
}

func targetRuntimeEnvironmentSuffix(target targetPlatform) string {
	replacer := strings.NewReplacer("-", "_", ".", "_", "/", "_")
	return strings.ToUpper(replacer.Replace(target.OS + "_" + target.Arch))
}

func targetRuntimeEnvironmentValue(env []string, base string, target targetPlatform) string {
	targetKey := base + "_" + targetRuntimeEnvironmentSuffix(target)
	return strings.TrimSpace(toolutil.FirstNonEmpty(
		toolutil.EnvValue(env, targetKey),
		toolutil.EnvValue(env, base),
	))
}

func resolveSiteNodeRuntime(ctx context.Context, a *app, env []string, target targetPlatform) (string, string, error) {
	runtimePath := targetRuntimeEnvironmentValue(env, "LINAPRO_SITE_NODE_RUNTIME", target)
	runtimeVersion := targetRuntimeEnvironmentValue(env, "LINAPRO_SITE_NODE_RUNTIME_VERSION", target)
	nativeTarget := target.OS == runtime.GOOS && target.Arch == runtime.GOARCH

	if runtimePath == "" && nativeTarget {
		var err error
		runtimePath, err = a.lookPath("node")
		if err != nil {
			return "", "", fmt.Errorf("locate Node.js runtime for website delivery: %w", err)
		}
	}
	if runtimePath == "" {
		suffix := targetRuntimeEnvironmentSuffix(target)
		return "", "", fmt.Errorf("website delivery for %s/%s requires LINAPRO_SITE_NODE_RUNTIME_%s and LINAPRO_SITE_NODE_RUNTIME_VERSION_%s", target.OS, target.Arch, suffix, suffix)
	}
	absoluteRuntime, err := filepath.Abs(filepath.Clean(runtimePath))
	if err != nil {
		return "", "", fmt.Errorf("resolve website Node.js runtime %q: %w", runtimePath, err)
	}
	info, err := os.Stat(absoluteRuntime)
	if err != nil {
		return "", "", fmt.Errorf("stat website Node.js runtime %s: %w", absoluteRuntime, err)
	}
	if !info.Mode().IsRegular() {
		return "", "", fmt.Errorf("website Node.js runtime is not a regular file: %s", absoluteRuntime)
	}

	if runtimeVersion == "" && nativeTarget {
		runtimeVersion, err = probeNodeRuntimeVersion(ctx, a, env, absoluteRuntime)
		if err != nil {
			return "", "", err
		}
	}
	if runtimeVersion == "" {
		return "", "", errors.New("cross-target website Node.js runtime version is empty")
	}
	if !strings.HasPrefix(runtimeVersion, "v") {
		return "", "", fmt.Errorf("website Node.js runtime version must use the exact vMAJOR.MINOR.PATCH form: %q", runtimeVersion)
	}
	return absoluteRuntime, runtimeVersion, nil
}

func probeNodeRuntimeVersion(ctx context.Context, a *app, env []string, runtimePath string) (string, error) {
	runner := a.execCommand
	if runner == nil {
		runner = exec.CommandContext
	}
	command := runner(ctx, runtimePath, "--version")
	command.Env = env
	output, err := command.Output()
	if err != nil {
		return "", fmt.Errorf("read bundled website Node.js runtime version: %w", err)
	}
	version := strings.TrimSpace(string(output))
	parts := strings.Split(strings.TrimPrefix(version, "v"), ".")
	if len(parts) != 3 || parts[0] == "" || parts[1] == "" || parts[2] == "" {
		return "", fmt.Errorf("website Node.js runtime returned an invalid exact version: %q", version)
	}
	return version, nil
}

func assembleWebDelivery(ctx context.Context, a *app, env []string, options buildOptions, target targetPlatform, packageRoot string) error {
	customerSource := filepath.Join(a.root, "apps", "lina-vben", "apps", "web-customer", "dist")
	websiteSource := filepath.Join(a.root, "apps", "lina-websites", "sites", "default", ".output")
	if !fileutil.FileExists(filepath.Join(customerSource, "index.html")) {
		return fmt.Errorf("customer frontend build output is missing: %s", customerSource)
	}
	if !fileutil.FileExists(filepath.Join(websiteSource, "server", "index.mjs")) {
		return fmt.Errorf("default website server build output is missing: %s", websiteSource)
	}
	if !fileutil.DirExists(filepath.Join(websiteSource, "public")) {
		return fmt.Errorf("default website public build output is missing: %s", websiteSource)
	}

	runtimeSource, runtimeVersion, err := resolveSiteNodeRuntime(ctx, a, env, target)
	if err != nil {
		return err
	}

	customerDestination := filepath.Join(packageRoot, "web-customer")
	websiteDestination := filepath.Join(packageRoot, "website", "default")
	runtimeRelative := filepath.Join("runtime", targetNodeExecutableName(target.OS))
	runtimeDestination := filepath.Join(packageRoot, runtimeRelative)
	if err = fileutil.CopyDirContents(customerSource, customerDestination); err != nil {
		return fmt.Errorf("copy customer frontend delivery: %w", err)
	}
	if err = fileutil.CopyDirContents(websiteSource, websiteDestination); err != nil {
		return fmt.Errorf("copy default website delivery: %w", err)
	}
	if err = fileutil.CopyFile(runtimeSource, runtimeDestination); err != nil {
		return fmt.Errorf("copy website Node.js runtime: %w", err)
	}
	if target.OS != "windows" {
		if err = os.Chmod(runtimeDestination, 0o755); err != nil {
			return fmt.Errorf("make website Node.js runtime executable: %w", err)
		}
	}

	customerDigest, err := fileutil.HashDirectory(customerDestination)
	if err != nil {
		return err
	}
	websiteDigest, err := fileutil.HashDirectory(websiteDestination)
	if err != nil {
		return err
	}
	runtimeDigest, err := hashRegularFile(runtimeDestination)
	if err != nil {
		return err
	}

	manifest := deliveryManifest{
		SchemaVersion: 1,
		Target:        target.OS + "/" + target.Arch,
		HostBinary:    targetExecutableName(options.BinaryName, target.OS),
		AdminFrontend: "embedded",
		CustomerFrontend: deliveryArtifact{
			Path:   "web-customer",
			SHA256: customerDigest,
		},
		Website: deliveryWebsite{
			Entry:      "website/default/server/index.mjs",
			HealthPath: "/__site/health",
			Public:     "website/default/public",
			Runtime:    filepath.ToSlash(runtimeRelative),
		},
		WebsiteArtifact: deliveryArtifact{
			Path:   "website/default",
			SHA256: websiteDigest,
		},
		Runtime: deliveryArtifact{
			Path:   filepath.ToSlash(runtimeRelative),
			SHA256: runtimeDigest,
		},
		RuntimeVersion: runtimeVersion,
		Ports: deliveryPorts{
			API:      6030,
			Admin:    6031,
			Customer: 6032,
			Website:  6680,
		},
	}
	content, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("encode delivery manifest: %w", err)
	}
	content = append(content, '\n')
	if err = os.WriteFile(filepath.Join(packageRoot, deliveryManifestName), content, 0o644); err != nil {
		return fmt.Errorf("write delivery manifest: %w", err)
	}
	fmt.Fprintf(a.stdout, "Web delivery assembled for %s/%s: customer static assets, default website, and Node.js %s\n", target.OS, target.Arch, runtimeVersion)
	return nil
}

func hashRegularFile(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("open %s for hashing: %w", path, err)
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			fmt.Fprintf(os.Stderr, "warning: close %s: %v\n", path, closeErr)
		}
	}()
	hasher := sha256.New()
	if _, err = io.Copy(hasher, file); err != nil {
		return "", fmt.Errorf("hash %s: %w", path, err)
	}
	return "sha256:" + hex.EncodeToString(hasher.Sum(nil)), nil
}
