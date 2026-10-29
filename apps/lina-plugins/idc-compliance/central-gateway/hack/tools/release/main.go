// Command release builds the local compliance gateway component bundle shipped
// with every complete LinaPro installation.
package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"time"
)

const (
	moduleName             = "lina-platform/idc-compliance-gateway"
	commandPackage         = "./cmd/idc-compliance-gateway"
	keyringPackage         = "./cmd/idc-compliance-local-gateway-keyring"
	supplierKeyringPackage = "./cmd/idc-compliance-local-gateway-supplier-keyring"
	evidenceKeyringPackage = "./cmd/idc-compliance-local-gateway-evidence-keyring"
	componentID            = "idc-compliance-local-gateway"
	binaryBaseName         = "idc-compliance-local-gateway"
	manifestFileName       = "manifest.json"
)

var safeMetadata = regexp.MustCompile(`^[A-Za-z0-9._:+-]{1,128}$`)

type releaseOptions struct {
	OutputPath string
	GOOS       string
	GOARCH     string
	Version    string
	Commit     string
	BuiltAt    string
}

type manifestFile struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}

type artifactManifest struct {
	SchemaVersion             string         `json:"schemaVersion"`
	ComponentID               string         `json:"componentId"`
	GOOS                      string         `json:"goos"`
	GOARCH                    string         `json:"goarch"`
	Version                   string         `json:"version"`
	Commit                    string         `json:"commit"`
	BuiltAt                   string         `json:"builtAt"`
	Files                     []manifestFile `json:"files"`
	SupplierClientsIncluded   bool           `json:"supplierClientsIncluded"`
	BusinessRoutesEnabled     bool           `json:"businessRoutesEnabled"`
	ProductPackageRequired    bool           `json:"productPackageRequired"`
	LocalInstallationOnly     bool           `json:"localInstallationOnly"`
	SameProgramForAllInstalls bool           `json:"sameProgramForAllInstallations"`
}

type deliverySource struct {
	Source      string
	Destination string
}

func main() {
	options := releaseOptions{}
	flag.StringVar(&options.OutputPath, "out", "", "required output component-bundle directory")
	flag.StringVar(&options.GOOS, "goos", runtime.GOOS, "target GOOS")
	flag.StringVar(&options.GOARCH, "goarch", runtime.GOARCH, "target GOARCH")
	flag.StringVar(&options.Version, "version", "dev", "release version")
	flag.StringVar(&options.Commit, "commit", "unknown", "source commit")
	flag.StringVar(&options.BuiltAt, "built-at", "unknown", "reproducible build time or unknown")
	flag.Parse()
	if err := buildRelease(options); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func buildRelease(options releaseOptions) error {
	if strings.TrimSpace(options.OutputPath) == "" {
		return fmt.Errorf("-out is required")
	}
	for name, value := range map[string]string{
		"goos": options.GOOS, "goarch": options.GOARCH, "version": options.Version,
		"commit": options.Commit, "built-at": options.BuiltAt,
	} {
		if !safeMetadata.MatchString(value) {
			return fmt.Errorf("%s contains unsupported characters or length", name)
		}
	}
	root, err := findModuleRoot()
	if err != nil {
		return err
	}
	outputPath, err := filepath.Abs(options.OutputPath)
	if err != nil {
		return fmt.Errorf("resolve output path: %w", err)
	}
	if _, err = os.Stat(outputPath); err == nil {
		return fmt.Errorf("refusing to overwrite existing release path %s", outputPath)
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("inspect release path %s: %w", outputPath, err)
	}
	parent := filepath.Dir(outputPath)
	if err = os.MkdirAll(parent, 0o755); err != nil {
		return fmt.Errorf("create release parent directory: %w", err)
	}
	temporaryPath, err := os.MkdirTemp(parent, ".local-gateway-bundle-*")
	if err != nil {
		return fmt.Errorf("create temporary component bundle: %w", err)
	}
	defer os.RemoveAll(temporaryPath)

	binaryName := binaryBaseName
	keyringBinaryName := binaryBaseName + "-keyring"
	supplierKeyringBinaryName := binaryBaseName + "-supplier-keyring"
	evidenceKeyringBinaryName := binaryBaseName + "-evidence-keyring"
	if options.GOOS == "windows" {
		binaryName += ".exe"
		keyringBinaryName += ".exe"
		supplierKeyringBinaryName += ".exe"
		evidenceKeyringBinaryName += ".exe"
	}
	binaryPath := filepath.Join(temporaryPath, "bin", binaryName)
	if err = os.MkdirAll(filepath.Dir(binaryPath), 0o755); err != nil {
		return fmt.Errorf("create binary directory: %w", err)
	}
	if err = buildBinary(root, binaryPath, commandPackage, options); err != nil {
		return err
	}
	if err = buildBinary(root, filepath.Join(temporaryPath, "bin", keyringBinaryName), keyringPackage, options); err != nil {
		return err
	}
	if err = buildBinary(root, filepath.Join(temporaryPath, "bin", supplierKeyringBinaryName), supplierKeyringPackage, options); err != nil {
		return err
	}
	if err = buildBinary(root, filepath.Join(temporaryPath, "bin", evidenceKeyringBinaryName), evidenceKeyringPackage, options); err != nil {
		return err
	}

	sources, err := deliverySources(root)
	if err != nil {
		return err
	}
	for _, source := range sources {
		if err = copyDeliveryFile(source.Source, filepath.Join(temporaryPath, source.Destination)); err != nil {
			return err
		}
	}
	files, err := collectManifestFiles(temporaryPath)
	if err != nil {
		return err
	}
	manifest := artifactManifest{
		SchemaVersion: "v1", ComponentID: componentID, GOOS: options.GOOS, GOARCH: options.GOARCH,
		Version: options.Version, Commit: options.Commit, BuiltAt: options.BuiltAt, Files: files,
		SupplierClientsIncluded: true, BusinessRoutesEnabled: true, ProductPackageRequired: true,
		LocalInstallationOnly: true, SameProgramForAllInstalls: true,
	}
	manifestBytes, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("encode release manifest: %w", err)
	}
	manifestBytes = append(manifestBytes, '\n')
	if err = os.WriteFile(filepath.Join(temporaryPath, manifestFileName), manifestBytes, 0o644); err != nil {
		return fmt.Errorf("write release manifest: %w", err)
	}
	if err = os.Rename(temporaryPath, outputPath); err != nil {
		return fmt.Errorf("publish component bundle: %w", err)
	}
	fmt.Printf("built local gateway component bundle %s\nmanifest %s\n", outputPath, filepath.Join(outputPath, manifestFileName))
	return nil
}

func buildBinary(root, outputPath, packagePath string, options releaseOptions) error {
	ldflags := strings.Join([]string{
		"-s", "-w",
		"-X", moduleName + "/internal/buildinfo.version=" + options.Version,
		"-X", moduleName + "/internal/buildinfo.commit=" + options.Commit,
		"-X", moduleName + "/internal/buildinfo.builtAt=" + options.BuiltAt,
	}, " ")
	buildContext, cancelBuild := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancelBuild()
	command := exec.CommandContext(buildContext, "go", "build", "-mod=readonly", "-trimpath", "-buildvcs=false", "-ldflags", ldflags,
		"-o", outputPath, packagePath)
	command.Dir = root
	command.Env = withEnvironment(os.Environ(), map[string]string{
		"CGO_ENABLED": "0", "GOOS": options.GOOS, "GOARCH": options.GOARCH, "GOWORK": "off",
	})
	if output, err := command.CombinedOutput(); err != nil {
		return fmt.Errorf("build local gateway: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}

func deliverySources(root string) ([]deliverySource, error) {
	result := []deliverySource{
		{Source: filepath.Join(root, "deploy", "local-gateway.env.example"), Destination: filepath.Join("config", "local-gateway.env.example")},
		{Source: filepath.Join(root, "deploy", "README.md"), Destination: filepath.Join("docs", "README.md")},
		{Source: filepath.Join(root, "deploy", "README.zh-CN.md"), Destination: filepath.Join("docs", "README.zh-CN.md")},
		{Source: filepath.Join(root, "deploy", "DATA-LIFECYCLE.md"), Destination: filepath.Join("docs", "DATA-LIFECYCLE.md")},
		{Source: filepath.Join(root, "deploy", "DATA-LIFECYCLE.zh-CN.md"), Destination: filepath.Join("docs", "DATA-LIFECYCLE.zh-CN.md")},
	}
	migrations, err := filepath.Glob(filepath.Join(root, "internal", "migration", "sql", "*.sql"))
	if err != nil {
		return nil, fmt.Errorf("list local gateway migrations: %w", err)
	}
	if len(migrations) == 0 {
		return nil, fmt.Errorf("local gateway delivery has no database migrations")
	}
	sort.Strings(migrations)
	for _, migration := range migrations {
		result = append(result, deliverySource{
			Source: migration, Destination: filepath.Join("migrations", filepath.Base(migration)),
		})
	}
	return result, nil
}

func copyDeliveryFile(source, destination string) error {
	content, err := os.ReadFile(source)
	if err != nil {
		return fmt.Errorf("read delivery file %s: %w", source, err)
	}
	if len(content) == 0 {
		return fmt.Errorf("delivery file %s is empty", source)
	}
	if err = os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		return fmt.Errorf("create delivery directory for %s: %w", destination, err)
	}
	if err = os.WriteFile(destination, content, 0o644); err != nil {
		return fmt.Errorf("write delivery file %s: %w", destination, err)
	}
	return nil
}

func collectManifestFiles(root string) ([]manifestFile, error) {
	result := []manifestFile{}
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || entry.Name() == manifestFileName {
			return nil
		}
		digest, digestErr := fileSHA256(path)
		if digestErr != nil {
			return digestErr
		}
		relative, relativeErr := filepath.Rel(root, path)
		if relativeErr != nil {
			return relativeErr
		}
		result = append(result, manifestFile{Path: filepath.ToSlash(relative), SHA256: digest})
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("collect component bundle manifest: %w", err)
	}
	sort.Slice(result, func(left, right int) bool { return result[left].Path < result[right].Path })
	return result, nil
}

func findModuleRoot() (string, error) {
	directory, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("get working directory: %w", err)
	}
	for {
		content, readErr := os.ReadFile(filepath.Join(directory, "go.mod"))
		if readErr == nil && strings.Contains(string(content), "module "+moduleName) {
			return directory, nil
		}
		parent := filepath.Dir(directory)
		if parent == directory {
			return "", fmt.Errorf("module root %s was not found", moduleName)
		}
		directory = parent
	}
}

func withEnvironment(current []string, overrides map[string]string) []string {
	result := make([]string, 0, len(current)+len(overrides))
	for _, entry := range current {
		key, _, found := strings.Cut(entry, "=")
		if _, overridden := overrides[strings.ToUpper(key)]; found && overridden {
			continue
		}
		result = append(result, entry)
	}
	for key, value := range overrides {
		result = append(result, key+"="+value)
	}
	return result
}

func fileSHA256(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("open artifact for hashing: %w", err)
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", fmt.Errorf("hash artifact: %w", err)
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}
