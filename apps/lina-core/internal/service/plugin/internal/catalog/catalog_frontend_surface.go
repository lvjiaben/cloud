// This file validates and assembles governed administration-console and
// customer-center frontend surfaces without performing database or filesystem
// reads. Callers can reuse one already-loaded manifest batch.

package catalog

import (
	"path"
	"regexp"
	"sort"
	"strings"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/plugin/capability/capmodel"
	"lina-core/pkg/plugin/pluginhost"
)

// Frontend surface host identifiers.
const (
	FrontendSurfaceHostAdmin    = "admin"
	FrontendSurfaceHostCustomer = "customer"
)

var (
	frontendRouteSegmentPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)
	frontendPermissionPattern   = regexp.MustCompile(`^[a-z0-9][a-z0-9:._/-]*$`)
	frontendIconPattern         = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9:._-]*$`)
)

var publishedFrontendSurfaceSlots = map[string]map[string]struct{}{
	FrontendSurfaceHostAdmin: {
		capmodel.FrontendSlotAdminCatalogProducts:       {},
		capmodel.FrontendSlotAdminResourceOperations:    {},
		capmodel.FrontendSlotAdminOrdersFinance:         {},
		capmodel.FrontendSlotAdminNotificationMarketing: {},
		capmodel.FrontendSlotAdminComplianceGovernance:  {},
		capmodel.FrontendSlotAdminSupportContent:        {},
		capmodel.FrontendSlotAdminExtensionManagement:   {},
	},
	FrontendSurfaceHostCustomer: {
		capmodel.FrontendSlotCustomerProducts:   {},
		capmodel.FrontendSlotCustomerFinance:    {},
		capmodel.FrontendSlotCustomerAccount:    {},
		capmodel.FrontendSlotCustomerSecurity:   {},
		capmodel.FrontendSlotCustomerCompliance: {},
		capmodel.FrontendSlotCustomerSupport:    {},
	},
}

// FrontendSurfaceBinding is one immutable host-ready surface projection.
type FrontendSurfaceBinding struct {
	PluginID           string
	PluginType         string
	PluginVersion      string
	Host               string
	Key                string
	Version            string
	Title              string
	Icon               string
	Permission         string
	Slot               string
	RoutePath          string
	Entry              string
	EntryURL           string
	Status             string
	BridgeCapabilities []string
}

// FrontendSurfaceDiscovery separates administration-console and customer
// surfaces so callers cannot accidentally expose one host's entries in the
// other host.
type FrontendSurfaceDiscovery struct {
	Admin    []*FrontendSurfaceBinding
	Customer []*FrontendSurfaceBinding
}

// BuildFrontendSurfaceDiscovery validates and assembles one already-loaded
// manifest batch in O(n). It performs no database or filesystem reads.
func BuildFrontendSurfaceDiscovery(manifests []*Manifest) (*FrontendSurfaceDiscovery, error) {
	result := &FrontendSurfaceDiscovery{
		Admin:    make([]*FrontendSurfaceBinding, 0),
		Customer: make([]*FrontendSurfaceBinding, 0),
	}
	for _, manifest := range manifests {
		if manifest == nil {
			continue
		}
		admin, err := buildFrontendSurfaceBindings(manifest, FrontendSurfaceHostAdmin, manifest.AdminSurfaces)
		if err != nil {
			return nil, err
		}
		customer, err := buildFrontendSurfaceBindings(manifest, FrontendSurfaceHostCustomer, manifest.CustomerSurfaces)
		if err != nil {
			return nil, err
		}
		result.Admin = append(result.Admin, admin...)
		result.Customer = append(result.Customer, customer...)
	}
	sortFrontendSurfaceBindings(result.Admin)
	sortFrontendSurfaceBindings(result.Customer)
	return result, nil
}

// BuildManifestFrontendSurfaceDiscovery assembles one manifest while retaining
// the same validation rules used by batch discovery.
func BuildManifestFrontendSurfaceDiscovery(manifest *Manifest) (*FrontendSurfaceDiscovery, error) {
	return BuildFrontendSurfaceDiscovery([]*Manifest{manifest})
}

func buildFrontendSurfaceBindings(
	manifest *Manifest,
	host string,
	surfaces []*capmodel.FrontendSurfaceDeclaration,
) ([]*FrontendSurfaceBinding, error) {
	if err := ValidateFrontendSurfaces(manifest); err != nil {
		return nil, gerror.Wrapf(err, "plugin %s frontend surfaces are invalid", manifest.ID)
	}
	result := make([]*FrontendSurfaceBinding, 0, len(surfaces))
	for _, surface := range surfaces {
		if _, ok := publishedFrontendSurfaceSlots[host][surface.Slot]; !ok {
			return nil, gerror.Newf("plugin %s %s surface %s uses unpublished host slot: %s", manifest.ID, host, surface.Key, surface.Slot)
		}
		route, err := normalizeFrontendSurfaceRoute(surface.Route)
		if err != nil {
			return nil, gerror.Wrapf(err, "plugin %s %s surface %s route is invalid", manifest.ID, host, surface.Key)
		}
		entry, err := normalizeFrontendSurfaceEntry(surface.Entry)
		if err != nil {
			return nil, gerror.Wrapf(err, "plugin %s %s surface %s entry is invalid", manifest.ID, host, surface.Key)
		}
		if surface.Permission != "" && !frontendPermissionPattern.MatchString(surface.Permission) {
			return nil, gerror.Newf("plugin %s %s surface %s permission is invalid", manifest.ID, host, surface.Key)
		}
		if host == FrontendSurfaceHostAdmin && surface.Permission == "" {
			return nil, gerror.Newf("plugin %s admin surface %s requires a host permission", manifest.ID, surface.Key)
		}
		if surface.Icon != "" && !frontendIconPattern.MatchString(surface.Icon) {
			return nil, gerror.Newf("plugin %s %s surface %s icon is invalid", manifest.ID, host, surface.Key)
		}
		binding := &FrontendSurfaceBinding{
			PluginID:           manifest.ID,
			PluginType:         manifest.Type,
			PluginVersion:      manifest.Version,
			Host:               host,
			Key:                surface.Key,
			Version:            surface.Version,
			Title:              surface.Title,
			Icon:               surface.Icon,
			Permission:         surface.Permission,
			Slot:               surface.Slot,
			RoutePath:          buildGovernedFrontendRoutePath(host, manifest.ID, route),
			Entry:              entry,
			Status:             surface.Status,
			BridgeCapabilities: append([]string(nil), surface.BridgeCapabilities...),
		}
		if strings.EqualFold(strings.TrimSpace(manifest.Type), "dynamic") {
			if !frontendSurfaceEntryIsPublished(manifest.PublicAssets, entry) {
				return nil, gerror.Newf("plugin %s %s surface %s entry is not covered by public_assets: %s", manifest.ID, host, surface.Key, entry)
			}
			binding.EntryURL = pluginhost.HostedAssetURLPrefix + manifest.ID + "/" + manifest.Version + "/" + entry
		}
		result = append(result, binding)
	}
	return result, nil
}

func frontendSurfaceEntryIsPublished(assets []*PublicAssetSpec, entry string) bool {
	for _, asset := range assets {
		if asset == nil || strings.TrimSpace(asset.Source) == "" {
			continue
		}
		mount := strings.Trim(strings.ReplaceAll(strings.TrimSpace(asset.Mount), "\\", "/"), "/")
		if mount == "" || entry == mount || strings.HasPrefix(entry, mount+"/") {
			return true
		}
	}
	return false
}

func normalizeFrontendSurfaceRoute(value string) (string, error) {
	raw := strings.ReplaceAll(strings.TrimSpace(value), "\\", "/")
	if strings.HasPrefix(raw, "/") {
		return "", gerror.New("route must be plugin-relative")
	}
	normalized := strings.Trim(raw, "/")
	if normalized == "" || normalized == "." || path.Clean(normalized) != normalized {
		return "", gerror.New("route must be a clean relative path")
	}
	for _, segment := range strings.Split(normalized, "/") {
		if !frontendRouteSegmentPattern.MatchString(segment) {
			return "", gerror.Newf("route segment must use safe kebab-case: %s", segment)
		}
	}
	return normalized, nil
}

func normalizeFrontendSurfaceEntry(value string) (string, error) {
	raw := strings.ReplaceAll(strings.TrimSpace(value), "\\", "/")
	if strings.HasPrefix(raw, "/") || strings.Contains(raw, "%") {
		return "", gerror.New("entry must be plugin-relative")
	}
	normalized := strings.Trim(raw, "/")
	if normalized == "" || normalized == "." || path.Clean(normalized) != normalized ||
		strings.ContainsAny(normalized, ":?#") {
		return "", gerror.New("entry must be a clean plugin-relative path")
	}
	for _, segment := range strings.Split(normalized, "/") {
		if segment == "" || segment == "." || segment == ".." {
			return "", gerror.New("entry cannot contain empty or traversal segments")
		}
	}
	return normalized, nil
}

func buildGovernedFrontendRoutePath(host string, pluginID string, route string) string {
	if host == FrontendSurfaceHostCustomer {
		return "/account/plugins/" + pluginID + "/" + route
	}
	return "/plugins/" + pluginID + "/" + route
}

func sortFrontendSurfaceBindings(items []*FrontendSurfaceBinding) {
	sort.Slice(items, func(i, j int) bool {
		if items[i].Slot != items[j].Slot {
			return items[i].Slot < items[j].Slot
		}
		if items[i].PluginID != items[j].PluginID {
			return items[i].PluginID < items[j].PluginID
		}
		return items[i].Key < items[j].Key
	})
}
