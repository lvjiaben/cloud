// This file converts governed administration-console surfaces into the
// existing permission-aware sys_menu projection. Customer surfaces are kept
// separate and are assembled by the signed-in customer shell.

package integration

import (
	"strings"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/internal/service/plugin/internal/catalog"
	"lina-core/pkg/plugin/capability/capmodel"
	"lina-core/pkg/plugin/pluginhost"
)

var adminSurfaceParentMenuKeys = map[string]string{
	capmodel.FrontendSlotAdminCatalogProducts:       "plugin:idc-core:catalog-products",
	capmodel.FrontendSlotAdminResourceOperations:    "plugin:idc-core:resource-operations",
	capmodel.FrontendSlotAdminOrdersFinance:         "plugin:idc-core:orders-finance",
	capmodel.FrontendSlotAdminNotificationMarketing: "plugin:idc-core:notification-marketing",
	capmodel.FrontendSlotAdminComplianceGovernance:  "plugin:idc-core:compliance-governance",
	capmodel.FrontendSlotAdminSupportContent:        "plugin:idc-core:support-content",
	capmodel.FrontendSlotAdminExtensionManagement:   "extension",
}

func manifestWithGovernedAdminSurfaceMenus(manifest *catalog.Manifest) (*catalog.Manifest, error) {
	if manifest == nil || len(manifest.AdminSurfaces) == 0 {
		return manifest, nil
	}
	discovery, err := catalog.BuildManifestFrontendSurfaceDiscovery(manifest)
	if err != nil {
		return nil, err
	}
	projected := catalog.CloneManifest(manifest)
	if strings.EqualFold(strings.TrimSpace(manifest.Type), "dynamic") {
		projected.Menus = withoutShadowedDynamicSurfaceMenus(
			projected.Menus,
			discovery.Admin,
		)
	}
	for _, surface := range discovery.Admin {
		parentKey := adminSurfaceParentMenuKeys[surface.Slot]
		if strings.TrimSpace(parentKey) == "" {
			return nil, gerror.Newf("admin surface slot has no host menu mapping: %s", surface.Slot)
		}
		menu := &catalog.MenuSpec{
			Key:       "plugin:" + manifest.ID + ":surface-admin:" + surface.Key,
			ParentKey: parentKey,
			Name:      surface.Title,
			Path:      surface.RoutePath,
			Perms:     surface.Permission,
			Icon:      surface.Icon,
			Type:      catalog.MenuTypePage.String(),
			Remark:    "governed admin surface for plugin " + manifest.ID,
		}
		if surface.EntryURL != "" {
			menu.Query = map[string]interface{}{
				pluginhost.GovernedSurfaceSourceQueryKey:       surface.EntryURL,
				pluginhost.GovernedSurfacePluginQueryKey:       manifest.ID,
				pluginhost.GovernedSurfaceKeyQueryKey:          surface.Key,
				pluginhost.GovernedSurfaceVersionQueryKey:      surface.Version,
				pluginhost.GovernedSurfaceCapabilitiesQueryKey: strings.Join(surface.BridgeCapabilities, ","),
				pluginhost.GovernedSurfaceHostQueryKey:         catalog.FrontendSurfaceHostAdmin,
			}
		} else {
			menu.Component = pluginhost.DynamicPageComponentPath
			menu.Query = map[string]interface{}{
				pluginhost.GovernedSurfacePluginQueryKey: manifest.ID,
				pluginhost.GovernedSurfaceEntryQueryKey:  surface.Entry,
			}
		}
		projected.Menus = append(projected.Menus, menu)
	}
	if err = catalog.ValidateManifestMenus(projected); err != nil {
		return nil, err
	}
	return projected, nil
}

// withoutShadowedDynamicSurfaceMenus drops legacy source-page menu trees that
// point at the same route as a governed dynamic surface. Keeping both creates
// two route records with one missing the immutable asset and bridge metadata;
// which record wins then depends on menu order and can render a blank page.
// Dynamic route permissions are materialized separately from route contracts,
// so removing the legacy button descendants does not weaken authorization.
func withoutShadowedDynamicSurfaceMenus(
	menus []*catalog.MenuSpec,
	surfaces []*catalog.FrontendSurfaceBinding,
) []*catalog.MenuSpec {
	shadowedKeys := make(map[string]struct{})
	for _, menu := range menus {
		if menu == nil {
			continue
		}
		for _, surface := range surfaces {
			if surface != nil && strings.TrimSpace(menu.Path) == strings.TrimSpace(surface.RoutePath) {
				shadowedKeys[strings.TrimSpace(menu.Key)] = struct{}{}
				break
			}
		}
	}
	for changed := true; changed; {
		changed = false
		for _, menu := range menus {
			if menu == nil {
				continue
			}
			key := strings.TrimSpace(menu.Key)
			if _, alreadyShadowed := shadowedKeys[key]; alreadyShadowed {
				continue
			}
			if _, parentShadowed := shadowedKeys[strings.TrimSpace(menu.ParentKey)]; parentShadowed {
				shadowedKeys[key] = struct{}{}
				changed = true
			}
		}
	}
	result := make([]*catalog.MenuSpec, 0, len(menus))
	for _, menu := range menus {
		if menu == nil {
			continue
		}
		if _, shadowed := shadowedKeys[strings.TrimSpace(menu.Key)]; !shadowed {
			result = append(result, menu)
		}
	}
	return result
}
