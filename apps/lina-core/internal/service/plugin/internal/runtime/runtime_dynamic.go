// This file exposes public runtime-state projections consumed by plugin-aware
// frontend shells that need minimal installation and enablement state.

package runtime

import (
	"context"
	pluginv1 "lina-core/api/plugin/v1"
	"strings"

	"lina-core/internal/service/plugin/internal/catalog"
	"lina-core/internal/service/plugin/internal/plugintypes"
	"lina-core/pkg/statusflag"
)

// RuntimeStateListOutput defines output for public runtime state queries.
type RuntimeStateListOutput struct {
	// List contains public plugin runtime states.
	List []*PluginDynamicStateItem
}

// PluginDynamicStateItem represents public runtime state of one plugin.
type PluginDynamicStateItem struct {
	// Id is the stable plugin identifier.
	Id string
	// Installed reports whether the plugin is installed or integrated.
	Installed int
	// Enabled reports whether the plugin is currently enabled.
	Enabled int
	// Version is the currently active plugin version.
	Version string
	// Generation is the current active plugin generation on the host.
	Generation int64
	// StatusKey is the host config key used by the public shell.
	StatusKey string
	// RuntimeState reports whether plugin business entries may run.
	RuntimeState RuntimeUpgradeState
	// Type distinguishes source pages from isolated dynamic-plugin pages.
	Type string
	// AdminSurfaces contains host-governed administration-console entries from
	// the effective release, never from an unpromoted upgrade candidate.
	AdminSurfaces []*catalog.FrontendSurfaceBinding
	// CustomerSurfaces contains host-governed signed-in customer-center entries
	// from the effective release.
	CustomerSurfaces []*catalog.FrontendSurfaceBinding
}

// ListRuntimeStates returns public plugin runtime states for shell slot rendering.
func (s *serviceImpl) ListRuntimeStates(ctx context.Context) (*RuntimeStateListOutput, error) {
	readCtx, err := s.storeSvc.WithStartupDataSnapshot(ctx)
	if err != nil {
		return nil, err
	}
	registries, err := s.storeSvc.ListAllRegistries(readCtx)
	if err != nil {
		return nil, err
	}
	manifestByID, err := s.catalogSvc.ScanManifestsByID()
	if err != nil {
		manifestByID = map[string]*catalog.Manifest{}
	}

	items := make([]*PluginDynamicStateItem, 0, len(registries))
	for _, registry := range registries {
		if registry == nil {
			continue
		}
		pluginID := strings.TrimSpace(registry.PluginId)
		if pluginID == "" {
			continue
		}

		manifest := manifestByID[pluginID]
		effectiveManifest := manifest
		release, releaseErr := s.storeSvc.GetRegistryRelease(readCtx, registry)
		if releaseErr != nil {
			// A registry release lookup failure must never expose declarations
			// scanned from a staged package that is not the effective release.
			effectiveManifest = nil
		} else if release != nil {
			effectiveManifest = nil
			if snapshot, snapshotErr := s.storeSvc.ParseManifestSnapshot(release.ManifestSnapshot); snapshotErr == nil && snapshot != nil {
				effectiveManifest = &catalog.Manifest{
					ID: snapshot.ID, Name: snapshot.Name, Version: snapshot.Version, Type: snapshot.Type,
					Distribution:     snapshot.Distribution,
					PublicAssets:     catalog.ClonePublicAssetSpecs(snapshot.PublicAssets),
					AdminSurfaces:    catalog.CloneFrontendSurfaceDeclarations(snapshot.AdminSurfaces),
					CustomerSurfaces: catalog.CloneFrontendSurfaceDeclarations(snapshot.CustomerSurfaces),
				}
			}
		} else if registry.ReleaseId > 0 || plugintypes.NormalizeType(registry.Type) == pluginv1.PluginTypeDynamic {
			// Dynamic plugins may only publish surfaces from an immutable active
			// release. A missing row or pointer is corrupt runtime state, so never
			// fall back to the staged artifact discovered on disk.
			effectiveManifest = nil
		}

		var (
			installed    = registry.Installed
			enabled      = registry.Status
			runtimeState = RuntimeUpgradeStateNormal
		)
		if plugintypes.NormalizeType(registry.Type) == pluginv1.PluginTypeDynamic {
			exists, _, err := s.hasArtifactStorageFile(ctx, pluginID)
			if err != nil {
				return nil, err
			}
			if !exists {
				installed = statusflag.Uninstalled.Int()
				enabled = statusflag.Disabled.Int()
			}
		}
		if projection, err := s.storeSvc.BuildRuntimeUpgradeState(readCtx, registry, manifest); err == nil {
			runtimeState = projection.State
		}

		generation := registry.Generation
		if generation <= 0 {
			generation = 1
		}

		var adminSurfaces, customerSurfaces []*catalog.FrontendSurfaceBinding
		if surfaceDiscovery, surfaceErr := catalog.BuildManifestFrontendSurfaceDiscovery(effectiveManifest); surfaceErr == nil {
			adminSurfaces = surfaceDiscovery.Admin
			customerSurfaces = surfaceDiscovery.Customer
		}

		items = append(items, &PluginDynamicStateItem{
			Id:               pluginID,
			Installed:        installed,
			Enabled:          enabled,
			Version:          registry.Version,
			Generation:       generation,
			StatusKey:        s.storeSvc.BuildPluginStatusKey(pluginID),
			RuntimeState:     runtimeState,
			Type:             registry.Type,
			AdminSurfaces:    adminSurfaces,
			CustomerSurfaces: customerSurfaces,
		})
	}
	return &RuntimeStateListOutput{List: items}, nil
}
