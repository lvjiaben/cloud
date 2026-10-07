package integration

import (
	"sort"
	"strings"

	"github.com/gogf/gf/v2/errors/gerror"

	pluginv1 "lina-core/api/plugin/v1"
	"lina-core/internal/service/plugin/internal/catalog"
	"lina-core/internal/service/plugin/internal/plugintypes"
)

// orderSourceManifestsDependencyFirst keeps source-plugin assembly stable while
// ensuring every in-process dependency has published its services first.
func orderSourceManifestsDependencyFirst(manifests []*catalog.Manifest) ([]*catalog.Manifest, error) {
	byID := make(map[string]*catalog.Manifest, len(manifests))
	for _, manifest := range manifests {
		if manifest == nil || strings.TrimSpace(manifest.ID) == "" ||
			plugintypes.NormalizeType(manifest.Type) != pluginv1.PluginTypeSource {
			continue
		}
		byID[strings.TrimSpace(manifest.ID)] = manifest
	}
	ids := make([]string, 0, len(byID))
	for pluginID := range byID {
		ids = append(ids, pluginID)
	}
	sort.Strings(ids)

	ordered := make([]*catalog.Manifest, 0, len(ids))
	visiting := make(map[string]bool, len(ids))
	visited := make(map[string]bool, len(ids))
	var visit func(string) error
	visit = func(pluginID string) error {
		if visited[pluginID] {
			return nil
		}
		if visiting[pluginID] {
			return gerror.Newf("source plugin dependency cycle blocks assembly: %s", pluginID)
		}
		manifest := byID[pluginID]
		if manifest == nil {
			return nil
		}
		visiting[pluginID] = true
		if manifest.Dependencies != nil {
			dependencies := make([]string, 0, len(manifest.Dependencies.Plugins))
			for _, dependency := range manifest.Dependencies.Plugins {
				if dependency == nil {
					continue
				}
				dependencyID := strings.TrimSpace(dependency.ID)
				if _, exists := byID[dependencyID]; exists {
					dependencies = append(dependencies, dependencyID)
				}
			}
			sort.Strings(dependencies)
			for _, dependencyID := range dependencies {
				if err := visit(dependencyID); err != nil {
					return err
				}
			}
		}
		visiting[pluginID] = false
		visited[pluginID] = true
		ordered = append(ordered, manifest)
		return nil
	}
	for _, pluginID := range ids {
		if err := visit(pluginID); err != nil {
			return nil, err
		}
	}
	return ordered, nil
}
