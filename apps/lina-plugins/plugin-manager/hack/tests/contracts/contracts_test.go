// Package contracts verifies every source manifest against the same public owner validators used at installation.
package contracts

import (
	"crypto/sha256"
	"gopkg.in/yaml.v3"
	"lina-core/pkg/plugin/capability/capmodel"
	"lina-plugin-idc-core/backend/cap/productfulfillmentcap"
	"lina-plugin-idc-payment/backend/cap/paymentcap"
	"lina-plugin-idc-portal/backend/cap/customertemplatecap"
	"lina-plugin-idc-portal/backend/cap/websitecap"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

type manifest struct {
	ID           string `yaml:"id"`
	Type         string `yaml:"type"`
	Distribution string `yaml:"distribution"`
	Dependencies struct {
		Plugins []struct {
			ID      string `yaml:"id"`
			Version string `yaml:"version"`
		}
	} `yaml:"dependencies"`
	Extensions []*capmodel.BusinessExtensionDeclaration `yaml:"business_extensions"`
}

// TestDocumentationLinks checks actual plugin-authored Markdown paths, excluding fenced examples.
func TestDocumentationLinks(t *testing.T) {
	root := pluginRoot(t)
	files, err := filepath.Glob(filepath.Join(root, "*", "README*.md"))
	if err != nil {
		t.Fatal(err)
	}
	files = append(files, filepath.Join(root, "README.md"), filepath.Join(root, "README.zh-CN.md"))
	for _, pattern := range []string{"*/docs/*.md", "*/manifest/docs/*/*.md"} {
		more, err := filepath.Glob(filepath.Join(root, pattern))
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, more...)
	}
	link := regexp.MustCompile(`\[[^\]]*\]\(([^)]+)\)`)
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		fenced := false
		for _, line := range strings.Split(string(data), "\n") {
			if strings.HasPrefix(strings.TrimSpace(line), "```") {
				fenced = !fenced
				continue
			}
			if fenced {
				continue
			}
			for _, match := range link.FindAllStringSubmatch(line, -1) {
				target := strings.Trim(match[1], "<>")
				target = strings.SplitN(target, "#", 2)[0]
				if target == "" || strings.Contains(target, "://") || strings.HasPrefix(target, "mailto:") {
					continue
				}
				if _, err = os.Stat(filepath.Join(filepath.Dir(file), filepath.FromSlash(target))); err != nil {
					t.Errorf("%s has missing relative link %s", file, target)
				}
			}
		}
	}
}

var validators = map[string]func([]*capmodel.BusinessExtensionDeclaration) error{
	"idc-core/product-fulfillment":   productfulfillmentcap.ValidateProviderDeclarations,
	"idc-resource/resource-provider": resourcecap.ValidateDynamicProviderDeclarations,
	"idc-payment/payment-method":     paymentcap.ValidateDynamicProviderDeclarations,
	"idc-portal/website-template":    websitecap.ValidateDeclarations,
	"idc-portal/customer-template":   customertemplatecap.ValidateDeclarations,
}

func pluginRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs("../../../..")
	if err != nil {
		t.Fatal(err)
	}
	return root
}

func TestEveryPluginHasDocumentationAndValidOwnerContract(t *testing.T) {
	files, err := filepath.Glob(filepath.Join(pluginRoot(t), "*", "plugin.yaml"))
	if err != nil || len(files) == 0 {
		t.Fatalf("find plugin manifests: %v", err)
	}
	t.Logf("Auditing %d plugin manifests", len(files))
	for _, file := range files {
		t.Run(filepath.Base(filepath.Dir(file)), func(t *testing.T) {
			body, err := os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
			t.Logf("plugin.yaml SHA256: %x", sha256.Sum256(body))
			var value manifest
			if err = yaml.Unmarshal(body, &value); err != nil {
				t.Fatal(err)
			}
			if value.ID != filepath.Base(filepath.Dir(file)) {
				t.Fatal("plugin ID differs from directory")
			}
			if value.Type != "source" && value.Type != "dynamic" {
				t.Fatal("invalid runtime type")
			}
			if value.Distribution != "builtin" && value.Distribution != "managed" {
				t.Fatal("invalid distribution")
			}
			for _, name := range []string{"README.md", "README.zh-CN.md"} {
				data, err := os.ReadFile(filepath.Join(filepath.Dir(file), name))
				if err != nil || len(strings.TrimSpace(string(data))) < 50 {
					t.Errorf("missing substantive %s", name)
				}
			}
			dependencies := map[string]string{}
			for _, item := range value.Dependencies.Plugins {
				dependencies[item.ID] = item.Version
			}
			for _, declaration := range value.Extensions {
				if declaration.Role == capmodel.BusinessExtensionRoleOwner {
					if declaration.Owner != value.ID {
						t.Error("owner declaration targets another plugin")
					}
					continue
				}
				key := declaration.Owner + "/" + declaration.Point
				validate := validators[key]
				if validate == nil {
					t.Fatalf("unpublished provider contract %s@%s: installation must reject", key, declaration.Version)
				}
				if dependencies[declaration.Owner] == "" {
					t.Error("missing versioned owner dependency")
				}
				if err = validate([]*capmodel.BusinessExtensionDeclaration{declaration}); err != nil {
					t.Fatalf("owner rejects actual manifest: %v", err)
				}
				copied := *declaration
				metadata := *declaration.ProviderMetadata
				copied.ProviderMetadata = &metadata
				copied.ProviderMetadata.Version = "v999"
				if err = validate([]*capmodel.BusinessExtensionDeclaration{&copied}); err == nil {
					t.Fatal("owner accepted unsupported metadata contract version")
				}
				copied = *declaration
				copied.ProviderMetadata = nil
				if err = validate([]*capmodel.BusinessExtensionDeclaration{&copied}); err == nil {
					t.Fatal("owner accepted missing required metadata")
				}
			}
		})
	}
}
