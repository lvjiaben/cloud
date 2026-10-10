// This file rejects incomplete declared frontend resources before WASM publication.
package wasmbuilder

import (
	"fmt"
	"path"
	"strings"
)

// validatePackagedFrontendAssets rejects incomplete packages before they are
// written into the host's discovery directory.
func validatePackagedFrontendAssets(manifest *pluginManifest, assets []*frontendAsset) error {
	files := make(map[string]bool, len(assets))
	for _, asset := range assets {
		if asset != nil {
			files[asset.Path] = true
		}
	}
	for _, declaration := range manifest.PublicAssets {
		if declaration == nil {
			continue
		}
		prefix := strings.TrimSuffix(normalizeEmbeddedResourcePath(declaration.Source), "/") + "/"
		found := false
		for file := range files {
			if strings.HasPrefix(file, prefix) {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("public_assets source contains no packaged assets: %s; build plugin frontend before packaging", declaration.Source)
		}
	}
	surfaces := append(cloneBuildFrontendSurfaces(manifest.AdminSurfaces), manifest.CustomerSurfaces...)
	for _, surface := range surfaces {
		if surface == nil {
			continue
		}
		entry := strings.TrimPrefix(surface.Entry, "/")
		found := false
		for _, declaration := range manifest.PublicAssets {
			if declaration == nil {
				continue
			}
			mount := strings.Trim(declaration.Mount, "/")
			relative := entry
			if mount != "" {
				if !strings.HasPrefix(entry, mount+"/") {
					continue
				}
				relative = strings.TrimPrefix(entry, mount+"/")
			}
			if files[path.Join(normalizeEmbeddedResourcePath(declaration.Source), relative)] {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("surface %s references missing packaged entry: %s; build plugin frontend before packaging", surface.Key, surface.Entry)
		}
	}
	return nil
}
