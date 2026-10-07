// This file loads the dynamic plugin manifest and validates manifest-level
// metadata shared by the standalone wasm builder flow.

package wasmbuilder

import (
	"bytes"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"

	"lina-core/pkg/plugin/capability/capmodel"
	"lina-core/pkg/plugin/pluginbridge/protocol"
)

func validateRuntimeBuildManifest(manifest *pluginManifest, manifestPath string) error {
	if manifest == nil {
		return fmt.Errorf("dynamic plugin manifest cannot be nil")
	}
	if strings.TrimSpace(manifest.ID) == "" {
		return fmt.Errorf("dynamic plugin manifest missing id: %s", manifestPath)
	}
	if strings.TrimSpace(manifest.Name) == "" {
		return fmt.Errorf("dynamic plugin manifest missing name: %s", manifestPath)
	}
	if strings.TrimSpace(manifest.Version) == "" {
		return fmt.Errorf("dynamic plugin manifest missing version: %s", manifestPath)
	}
	manifest.Type = strings.ToLower(strings.TrimSpace(manifest.Type))
	if manifest.Type != pluginTypeDynamic {
		return fmt.Errorf("dynamic sample manifest type must be dynamic: %s", manifestPath)
	}
	manifest.Distribution = strings.ToLower(strings.TrimSpace(manifest.Distribution))
	if manifest.Distribution == "" {
		manifest.Distribution = "managed"
	}
	if manifest.Distribution != "managed" {
		return fmt.Errorf("dynamic plugin distribution must be managed: %s", manifestPath)
	}
	manifest.ScopeNature = strings.ToLower(strings.TrimSpace(manifest.ScopeNature))
	if manifest.ScopeNature == "" {
		manifest.ScopeNature = pluginScopeNatureTenantAware
	}
	if manifest.ScopeNature != pluginScopeNaturePlatformOnly &&
		manifest.ScopeNature != pluginScopeNatureTenantAware {
		return fmt.Errorf("dynamic plugin scope_nature only supports platform_only/tenant_aware: %s", manifest.ScopeNature)
	}
	if manifest.SupportsMultiTenant == nil {
		return fmt.Errorf("dynamic plugin manifest missing supports_multi_tenant: %s", manifestPath)
	}
	if manifest.ScopeNature == pluginScopeNaturePlatformOnly && *manifest.SupportsMultiTenant {
		return fmt.Errorf("dynamic plugin supports_multi_tenant cannot be true when scope_nature is platform_only")
	}
	manifest.DefaultInstallMode = strings.ToLower(strings.TrimSpace(manifest.DefaultInstallMode))
	if manifest.ScopeNature == pluginScopeNaturePlatformOnly || !*manifest.SupportsMultiTenant {
		manifest.DefaultInstallMode = pluginInstallModeGlobal
	} else if manifest.DefaultInstallMode == "" {
		manifest.DefaultInstallMode = pluginInstallModeTenantScoped
	} else if manifest.DefaultInstallMode != pluginInstallModeGlobal &&
		manifest.DefaultInstallMode != pluginInstallModeTenantScoped {
		return fmt.Errorf("dynamic plugin default_install_mode only supports global/tenant_scoped: %s", manifest.DefaultInstallMode)
	}
	if !pluginManifestIDPattern.MatchString(manifest.ID) {
		return fmt.Errorf("dynamic plugin id must use kebab-case: %s", manifest.ID)
	}
	if err := validateSemanticVersion(manifest.Version); err != nil {
		return fmt.Errorf("dynamic plugin version is invalid: %w", err)
	}
	if err := validateDependencySpec(manifest.ID, manifest.Dependencies); err != nil {
		return fmt.Errorf("dynamic plugin dependencies invalid: %w", err)
	}
	if err := validateBuildBusinessExtensions(manifest); err != nil {
		return fmt.Errorf("dynamic plugin business extensions invalid: %w", err)
	}
	if err := validateBuildFrontendSurfaces("admin", manifest.AdminSurfaces); err != nil {
		return fmt.Errorf("dynamic plugin admin surfaces invalid: %w", err)
	}
	if err := validateBuildFrontendSurfaces("customer", manifest.CustomerSurfaces); err != nil {
		return fmt.Errorf("dynamic plugin customer surfaces invalid: %w", err)
	}
	if err := protocol.ValidateHostServiceSpecsForPlugin(manifest.ID, manifest.HostServices); err != nil {
		return fmt.Errorf("dynamic plugin hostServices invalid: %w", err)
	}
	hostServices, err := protocol.NormalizeHostServiceSpecsForPlugin(manifest.ID, manifest.HostServices)
	if err != nil {
		return fmt.Errorf("dynamic plugin hostServices normalization failed: %w", err)
	}
	manifest.HostServices = hostServices
	return nil
}

var buildContractVersionPattern = regexp.MustCompile(`^v[1-9][0-9]*$`)
var buildBusinessMethodPattern = regexp.MustCompile(`^[a-z][a-z0-9]*(?:[._-][a-z0-9]+)*$`)

func validateBuildBusinessExtensions(manifest *pluginManifest) error {
	dependencies := make(map[string]string)
	if manifest.Dependencies != nil {
		for _, dependency := range manifest.Dependencies.Plugins {
			if dependency != nil {
				dependencies[strings.TrimSpace(dependency.ID)] = strings.TrimSpace(dependency.Version)
			}
		}
	}
	seen := make(map[string]struct{}, len(manifest.BusinessExtensions))
	for index, declaration := range manifest.BusinessExtensions {
		if declaration == nil {
			return fmt.Errorf("declaration %d cannot be nil", index+1)
		}
		declaration.Point = strings.TrimSpace(declaration.Point)
		declaration.Version = strings.TrimSpace(declaration.Version)
		declaration.Role = strings.ToLower(strings.TrimSpace(declaration.Role))
		declaration.Owner = strings.TrimSpace(declaration.Owner)
		if !pluginManifestIDPattern.MatchString(declaration.Point) || !buildContractVersionPattern.MatchString(declaration.Version) {
			return fmt.Errorf("point/version is invalid: %s@%s", declaration.Point, declaration.Version)
		}
		if !pluginManifestIDPattern.MatchString(declaration.Owner) {
			return fmt.Errorf("owner is invalid: %s", declaration.Owner)
		}
		if declaration.Role != capmodel.BusinessExtensionRoleOwner && declaration.Role != capmodel.BusinessExtensionRoleProvider {
			return fmt.Errorf("role only supports owner/provider: %s", declaration.Point)
		}
		if declaration.Role == capmodel.BusinessExtensionRoleOwner && declaration.Owner != manifest.ID {
			return fmt.Errorf("owner declaration must use current plugin id: %s", declaration.Point)
		}
		if declaration.Role == capmodel.BusinessExtensionRoleProvider && strings.TrimSpace(dependencies[declaration.Owner]) == "" {
			return fmt.Errorf("provider %s requires versioned owner dependency %s", declaration.Point, declaration.Owner)
		}
		methodSeen := make(map[string]struct{}, len(declaration.Methods))
		for _, method := range declaration.Methods {
			method = strings.ToLower(strings.TrimSpace(method))
			if !buildBusinessMethodPattern.MatchString(method) {
				return fmt.Errorf("business extension method is invalid: %s", method)
			}
			if _, ok := methodSeen[method]; ok {
				return fmt.Errorf("duplicate business extension method: %s", method)
			}
			methodSeen[method] = struct{}{}
		}
		key := declaration.Owner + ":" + declaration.Point + ":" + declaration.Version + ":" + declaration.Role
		if _, ok := seen[key]; ok {
			return fmt.Errorf("duplicate declaration: %s", key)
		}
		seen[key] = struct{}{}
	}
	return nil
}

func validateBuildFrontendSurfaces(host string, surfaces []*capmodel.FrontendSurfaceDeclaration) error {
	seen := make(map[string]struct{}, len(surfaces))
	for index, surface := range surfaces {
		if surface == nil {
			return fmt.Errorf("%s declaration %d cannot be nil", host, index+1)
		}
		surface.Key = strings.TrimSpace(surface.Key)
		surface.Version = strings.TrimSpace(surface.Version)
		if !pluginManifestIDPattern.MatchString(surface.Key) || !buildContractVersionPattern.MatchString(surface.Version) {
			return fmt.Errorf("%s surface key/version is invalid: %s@%s", host, surface.Key, surface.Version)
		}
		if strings.TrimSpace(surface.Title) == "" || strings.TrimSpace(surface.Slot) == "" ||
			strings.TrimSpace(surface.Route) == "" || strings.TrimSpace(surface.Entry) == "" {
			return fmt.Errorf("%s surface %s requires title, slot, route, and entry", host, surface.Key)
		}
		if _, ok := seen[surface.Key]; ok {
			return fmt.Errorf("duplicate %s surface: %s", host, surface.Key)
		}
		seen[surface.Key] = struct{}{}
		capabilitySeen := make(map[string]struct{}, len(surface.BridgeCapabilities))
		for _, capability := range surface.BridgeCapabilities {
			capability = strings.ToLower(strings.TrimSpace(capability))
			if !buildBusinessMethodPattern.MatchString(capability) {
				return fmt.Errorf("%s surface bridge capability is invalid: %s", host, capability)
			}
			if _, ok := capabilitySeen[capability]; ok {
				return fmt.Errorf("duplicate %s surface bridge capability: %s", host, capability)
			}
			capabilitySeen[capability] = struct{}{}
		}
	}
	return nil
}

func cloneBuildBusinessExtensions(items []*capmodel.BusinessExtensionDeclaration) []*capmodel.BusinessExtensionDeclaration {
	result := make([]*capmodel.BusinessExtensionDeclaration, 0, len(items))
	for _, item := range items {
		if item == nil {
			continue
		}
		next := *item
		next.Methods = append([]string(nil), item.Methods...)
		result = append(result, &next)
	}
	return result
}

func cloneBuildFrontendSurfaces(items []*capmodel.FrontendSurfaceDeclaration) []*capmodel.FrontendSurfaceDeclaration {
	result := make([]*capmodel.FrontendSurfaceDeclaration, 0, len(items))
	for _, item := range items {
		if item == nil {
			continue
		}
		next := *item
		next.BridgeCapabilities = append([]string(nil), item.BridgeCapabilities...)
		result = append(result, &next)
	}
	return result
}

var buildDependencyVersionConstraintPattern = regexp.MustCompile(`^(>=|<=|>|<|=)?v?\d+\.\d+\.\d+(?:-[0-9A-Za-z.-]+)?$`)

func validateDependencySpec(pluginID string, spec *dependencySpec) error {
	if spec == nil {
		return nil
	}
	if spec.Framework != nil {
		spec.Framework.Version = strings.TrimSpace(spec.Framework.Version)
		if spec.Framework.Version != "" {
			if err := validateSemanticVersionRange(spec.Framework.Version); err != nil {
				return fmt.Errorf("framework version is invalid: %w", err)
			}
		}
	}
	seen := make(map[string]struct{}, len(spec.Plugins))
	for index, dependency := range spec.Plugins {
		if dependency == nil {
			return fmt.Errorf("dependency %d cannot be nil", index+1)
		}
		dependency.ID = strings.TrimSpace(dependency.ID)
		dependency.Version = strings.TrimSpace(dependency.Version)
		if dependency.ID == "" {
			return fmt.Errorf("dependency %d is missing id", index+1)
		}
		if !pluginManifestIDPattern.MatchString(dependency.ID) {
			return fmt.Errorf("dependency id must use kebab-case: %s", dependency.ID)
		}
		if dependency.ID == pluginID {
			return fmt.Errorf("plugin cannot depend on itself: %s", pluginID)
		}
		if _, ok := seen[dependency.ID]; ok {
			return fmt.Errorf("duplicate dependency: %s", dependency.ID)
		}
		seen[dependency.ID] = struct{}{}
		if dependency.Version != "" {
			if err := validateSemanticVersionRange(dependency.Version); err != nil {
				return fmt.Errorf("dependency %s version is invalid: %w", dependency.ID, err)
			}
		}
	}
	return nil
}

func validateSemanticVersionRange(value string) error {
	tokens := strings.Fields(strings.TrimSpace(value))
	if len(tokens) == 0 {
		return fmt.Errorf("version range cannot be empty")
	}
	for _, token := range tokens {
		if !buildDependencyVersionConstraintPattern.MatchString(token) {
			return fmt.Errorf("version range token must use semver comparison format: %s", token)
		}
		if err := validateSemanticVersion(trimVersionConstraintOperator(token)); err != nil {
			return err
		}
	}
	return nil
}

func trimVersionConstraintOperator(token string) string {
	token = strings.TrimSpace(token)
	for _, operator := range []string{">=", "<=", ">", "<", "="} {
		if strings.HasPrefix(token, operator) {
			return strings.TrimSpace(strings.TrimPrefix(token, operator))
		}
	}
	return token
}

func loadYAMLFile(filePath string, target interface{}) error {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}
	if len(content) == 0 {
		return fmt.Errorf("yaml file is empty: %s", filePath)
	}
	if err = yaml.Unmarshal(content, target); err != nil {
		return fmt.Errorf("failed to parse yaml file %s: %w", filePath, err)
	}
	return nil
}

func validateManifestDependencySchema(content []byte, fileLabel string) error {
	var root yaml.Node
	decoder := yaml.NewDecoder(bytes.NewReader(content))
	if err := decoder.Decode(&root); err != nil {
		return err
	}
	document := &root
	if root.Kind == yaml.DocumentNode && len(root.Content) > 0 {
		document = root.Content[0]
	}
	if document == nil || document.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(document.Content); i += 2 {
		key := strings.TrimSpace(document.Content[i].Value)
		switch key {
		case "dependencies":
			if err := validateDependencySchema(document.Content[i+1], fileLabel); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateDependencySchema(node *yaml.Node, fileLabel string) error {
	if node == nil || node.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(node.Content); i += 2 {
		key := strings.TrimSpace(node.Content[i].Value)
		switch key {
		case "plugins":
			if err := validatePluginDependencySchema(node.Content[i+1], fileLabel); err != nil {
				return err
			}
		}
	}
	return nil
}

func validatePluginDependencySchema(node *yaml.Node, fileLabel string) error {
	if node == nil || node.Kind != yaml.SequenceNode {
		return nil
	}
	for index, item := range node.Content {
		if item == nil || item.Kind != yaml.MappingNode {
			continue
		}
		for i := 0; i+1 < len(item.Content); i += 2 {
			key := strings.TrimSpace(item.Content[i].Value)
			switch key {
			case "id", "version":
			default:
				return fmt.Errorf("dynamic plugin manifest field dependencies.plugins[%d].%s is not part of the dependency schema: %s", index, key, fileLabel)
			}
		}
	}
	return nil
}

func validateSemanticVersion(value string) error {
	match := pluginManifestSemverPattern.FindStringSubmatch(strings.TrimSpace(value))
	if len(match) < 4 {
		return fmt.Errorf("version must use semver format: %s", value)
	}

	for _, raw := range match[1:4] {
		if _, err := strconv.Atoi(raw); err != nil {
			return err
		}
	}
	return nil
}
