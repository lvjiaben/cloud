// This file assembles the final runtime artifact bytes and appends Lina custom
// sections for manifest, assets, routes, and bridge metadata.

package wasmbuilder

import (
	"encoding/json"
	"os"
	"strings"

	"lina-core/pkg/plugin/pluginbridge/protocol"
)

func buildRuntimeArtifactContent(
	manifest *pluginManifest,
	frontendAssets []*frontendAsset,
	i18nAssets []*i18nAsset,
	apiDocI18NAssets []*i18nAsset,
	installSQLAssets []*sqlAsset,
	uninstallSQLAssets []*sqlAsset,
	mockSQLAssets []*sqlAsset,
	manifestResources []*manifestResource,
	hookSpecs []*hookSpec,
	lifecycleSpecs []*protocol.LifecycleContract,
	resourceSpecs []*resourceSpec,
	routeContracts []*protocol.RouteContract,
	bridgeSpec *protocol.BridgeSpec,
	runtimePath string,
) ([]byte, error) {
	manifestPayload, err := json.Marshal(&dynamicArtifactManifest{
		ID:                  manifest.ID,
		Name:                manifest.Name,
		Version:             manifest.Version,
		Type:                pluginTypeDynamic,
		Distribution:        manifest.Distribution,
		ScopeNature:         manifest.ScopeNature,
		SupportsMultiTenant: manifest.SupportsMultiTenant,
		DefaultInstallMode:  manifest.DefaultInstallMode,
		Description:         manifest.Description,
		Dependencies:        cloneBuildDependencySpec(manifest.Dependencies),
		BusinessExtensions:  cloneBuildBusinessExtensions(manifest.BusinessExtensions),
		AdminSurfaces:       cloneBuildFrontendSurfaces(manifest.AdminSurfaces),
		CustomerSurfaces:    cloneBuildFrontendSurfaces(manifest.CustomerSurfaces),
		Menus:               manifest.Menus,
		PublicAssets:        cloneBuildPublicAssetSpecs(manifest.PublicAssets),
	})
	if err != nil {
		return nil, err
	}
	runtimePayload, err := json.Marshal(&protocol.RuntimeArtifactMetadata{
		RuntimeKind:           pluginDynamicKindWasm,
		ABIVersion:            pluginDynamicSupportedABIVersion,
		FrontendAssetCount:    len(frontendAssets),
		I18NAssetCount:        len(i18nAssets),
		APIDocI18NAssetCount:  len(apiDocI18NAssets),
		SQLAssetCount:         len(installSQLAssets) + len(uninstallSQLAssets) + len(mockSQLAssets),
		MockSQLAssetCount:     len(mockSQLAssets),
		ManifestResourceCount: len(manifestResources),
		RouteCount:            len(routeContracts),
	})
	if err != nil {
		return nil, err
	}

	content := []byte{0x00, 0x61, 0x73, 0x6d, 0x01, 0x00, 0x00, 0x00}
	if strings.TrimSpace(runtimePath) != "" {
		runtimeBytes, err := os.ReadFile(runtimePath)
		if err != nil {
			return nil, err
		}
		content = runtimeBytes
	}
	content = appendWasmCustomSection(content, pluginDynamicWasmSectionManifest, manifestPayload)
	content = appendWasmCustomSection(content, pluginDynamicWasmSectionDynamic, runtimePayload)

	if len(frontendAssets) > 0 {
		payload, err := json.Marshal(frontendAssets)
		if err != nil {
			return nil, err
		}
		content = appendWasmCustomSection(content, pluginDynamicWasmSectionFrontend, payload)
	}
	if len(i18nAssets) > 0 {
		payload, err := json.Marshal(i18nAssets)
		if err != nil {
			return nil, err
		}
		content = appendWasmCustomSection(content, pluginDynamicWasmSectionI18N, payload)
	}
	if len(apiDocI18NAssets) > 0 {
		payload, err := json.Marshal(apiDocI18NAssets)
		if err != nil {
			return nil, err
		}
		content = appendWasmCustomSection(content, pluginDynamicWasmSectionAPIDocI18N, payload)
	}
	if len(installSQLAssets) > 0 {
		payload, err := json.Marshal(installSQLAssets)
		if err != nil {
			return nil, err
		}
		content = appendWasmCustomSection(content, pluginDynamicWasmSectionInstallSQL, payload)
	}
	if len(uninstallSQLAssets) > 0 {
		payload, err := json.Marshal(uninstallSQLAssets)
		if err != nil {
			return nil, err
		}
		content = appendWasmCustomSection(content, pluginDynamicWasmSectionUninstallSQL, payload)
	}
	if len(mockSQLAssets) > 0 {
		payload, err := json.Marshal(mockSQLAssets)
		if err != nil {
			return nil, err
		}
		content = appendWasmCustomSection(content, pluginDynamicWasmSectionMockSQL, payload)
	}
	if len(manifestResources) > 0 {
		payload, err := json.Marshal(manifestResources)
		if err != nil {
			return nil, err
		}
		content = appendWasmCustomSection(content, pluginDynamicWasmSectionManifestResources, payload)
	}
	if len(hookSpecs) > 0 {
		payload, err := json.Marshal(hookSpecs)
		if err != nil {
			return nil, err
		}
		content = appendWasmCustomSection(content, pluginDynamicWasmSectionBackendHooks, payload)
	}
	if len(lifecycleSpecs) > 0 {
		payload, err := json.Marshal(lifecycleSpecs)
		if err != nil {
			return nil, err
		}
		content = appendWasmCustomSection(content, pluginDynamicWasmSectionBackendLifecycle, payload)
	}
	if len(resourceSpecs) > 0 {
		payload, err := json.Marshal(resourceSpecs)
		if err != nil {
			return nil, err
		}
		content = appendWasmCustomSection(content, pluginDynamicWasmSectionBackendRes, payload)
	}
	if len(routeContracts) > 0 {
		payload, err := json.Marshal(routeContracts)
		if err != nil {
			return nil, err
		}
		content = appendWasmCustomSection(content, pluginDynamicWasmSectionBackendRoutes, payload)
	}
	if bridgeSpec != nil {
		payload, err := json.Marshal(bridgeSpec)
		if err != nil {
			return nil, err
		}
		content = appendWasmCustomSection(content, pluginDynamicWasmSectionBackendBridge, payload)
	}
	if len(manifest.HostServices) > 0 {
		payload, err := json.Marshal(manifest.HostServices)
		if err != nil {
			return nil, err
		}
		content = appendWasmCustomSection(content, pluginDynamicWasmSectionBackendHostServices, payload)
	}
	return content, nil
}

func cloneBuildDependencySpec(spec *dependencySpec) *dependencySpec {
	if spec == nil {
		return nil
	}
	clone := &dependencySpec{}
	if spec.Framework != nil {
		clone.Framework = &frameworkDependencySpec{Version: strings.TrimSpace(spec.Framework.Version)}
	}
	if len(spec.Plugins) > 0 {
		clone.Plugins = make([]*pluginDependencySpec, 0, len(spec.Plugins))
		for _, dependency := range spec.Plugins {
			if dependency == nil {
				clone.Plugins = append(clone.Plugins, nil)
				continue
			}
			clone.Plugins = append(clone.Plugins, &pluginDependencySpec{
				ID:      strings.TrimSpace(dependency.ID),
				Version: strings.TrimSpace(dependency.Version),
			})
		}
	}
	return clone
}

func cloneBuildPublicAssetSpecs(items []*publicAssetSpec) []*publicAssetSpec {
	if len(items) == 0 {
		return nil
	}
	cloned := make([]*publicAssetSpec, 0, len(items))
	for _, item := range items {
		if item == nil {
			continue
		}
		cloned = append(cloned, &publicAssetSpec{
			Source: strings.TrimSpace(item.Source),
			Mount:  strings.TrimSpace(item.Mount),
			Index:  strings.TrimSpace(item.Index),
		})
	}
	return cloned
}

func appendWasmCustomSection(content []byte, name string, payload []byte) []byte {
	section := make([]byte, 0, len(name)+len(payload)+8)
	section = appendULEB128(section, uint32(len(name)))
	section = append(section, []byte(name)...)
	section = append(section, payload...)

	content = append(content, 0x00)
	content = appendULEB128(content, uint32(len(section)))
	content = append(content, section...)
	return content
}

func appendULEB128(content []byte, value uint32) []byte {
	current := value
	for {
		part := byte(current & 0x7f)
		current >>= 7
		if current != 0 {
			part |= 0x80
		}
		content = append(content, part)
		if current == 0 {
			return content
		}
	}
}
