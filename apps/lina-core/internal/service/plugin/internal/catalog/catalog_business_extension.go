// This file validates and batches business extension declarations separately
// from technical hooks, plugin runtime type, and distribution governance.

package catalog

import (
	"encoding/json"
	"reflect"
	"regexp"
	"sort"
	"strings"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/internal/service/plugin/internal/plugintypes"
	"lina-core/pkg/plugin/capability/capmodel"
	bridgecontract "lina-core/pkg/plugin/pluginbridge/contract"
	"lina-core/pkg/plugin/pluginhost"
)

// ManifestBusinessDeclarationsMatch reports whether plugin.yaml and a dynamic
// artifact freeze the same normalized business extensions and frontend surfaces.
func ManifestBusinessDeclarationsMatch(manifest *Manifest, artifact *ArtifactManifest) bool {
	if manifest == nil || artifact == nil {
		return manifest == nil && artifact == nil
	}
	left := &Manifest{
		ID: manifest.ID, Dependencies: plugintypes.CloneDependencySpec(manifest.Dependencies),
		BusinessExtensions: CloneBusinessExtensionDeclarations(manifest.BusinessExtensions),
		AdminSurfaces:      CloneFrontendSurfaceDeclarations(manifest.AdminSurfaces),
		CustomerSurfaces:   CloneFrontendSurfaceDeclarations(manifest.CustomerSurfaces),
	}
	right := &Manifest{
		ID: artifact.ID, Dependencies: plugintypes.CloneDependencySpec(artifact.Dependencies),
		BusinessExtensions: CloneBusinessExtensionDeclarations(artifact.BusinessExtensions),
		AdminSurfaces:      CloneFrontendSurfaceDeclarations(artifact.AdminSurfaces),
		CustomerSurfaces:   CloneFrontendSurfaceDeclarations(artifact.CustomerSurfaces),
	}
	if ValidateBusinessExtensions(left) != nil || ValidateFrontendSurfaces(left) != nil ||
		ValidateBusinessExtensions(right) != nil || ValidateFrontendSurfaces(right) != nil {
		return false
	}
	return reflect.DeepEqual(left.BusinessExtensions, right.BusinessExtensions) &&
		reflect.DeepEqual(left.AdminSurfaces, right.AdminSurfaces) &&
		reflect.DeepEqual(left.CustomerSurfaces, right.CustomerSurfaces)
}

var (
	businessExtensionPointPattern = regexp.MustCompile(`^[a-z][a-z0-9]*(?:-[a-z0-9]+)*$`)
	contractVersionPattern        = regexp.MustCompile(`^v[1-9][0-9]*$`)
	extensionMethodPattern        = regexp.MustCompile(`^[a-z][a-z0-9]*(?:[._-][a-z0-9]+)*$`)
	surfaceKeyPattern             = regexp.MustCompile(`^[a-z][a-z0-9]*(?:-[a-z0-9]+)*$`)
)

// BusinessExtensionBinding is one effective owner-authorized provider
// declaration returned by batch discovery.
type BusinessExtensionBinding struct {
	PluginID     string
	PluginName   string
	Type         string
	Distribution string
	Declaration  *capmodel.BusinessExtensionDeclaration
}

// BusinessExtensionDiscovery is an immutable batch projection keyed by
// business extension point. Callers can select a matching business page
// without rescanning manifests or issuing per-plugin database queries.
type BusinessExtensionDiscovery struct {
	Owners    map[string]*BusinessExtensionBinding
	Providers map[string][]*BusinessExtensionBinding
}

// DiscoverBusinessExtensions scans the catalog once, then resolves all
// business extension publications and providers in memory.
func (s *serviceImpl) DiscoverBusinessExtensions() (*BusinessExtensionDiscovery, error) {
	manifests, err := s.ScanManifests()
	if err != nil {
		return nil, err
	}
	return BuildBusinessExtensionDiscovery(manifests)
}

// AuthorizeManifestBusinessExtensions resolves one install/enable candidate
// against one complete catalog snapshot. The candidate replaces its discovered
// copy so staged releases cannot inherit authorization from an older version.
func (s *serviceImpl) AuthorizeManifestBusinessExtensions(manifest *Manifest) ([]*capmodel.BusinessExtensionDeclaration, error) {
	if manifest == nil {
		return []*capmodel.BusinessExtensionDeclaration{}, nil
	}
	manifests, err := s.ScanManifests()
	if err != nil {
		return nil, err
	}
	return AuthorizeBusinessExtensionsFromManifests(manifest, manifests)
}

// AuthorizeBusinessExtensionsFromManifests is the pure batch authorization
// projection used by lifecycle code and deterministic tests.
func AuthorizeBusinessExtensionsFromManifests(manifest *Manifest, manifests []*Manifest) ([]*capmodel.BusinessExtensionDeclaration, error) {
	if manifest == nil {
		return []*capmodel.BusinessExtensionDeclaration{}, nil
	}
	replaced := false
	for index, item := range manifests {
		if item != nil && strings.TrimSpace(item.ID) == strings.TrimSpace(manifest.ID) {
			manifests[index] = CloneManifest(manifest)
			replaced = true
		}
	}
	if !replaced {
		manifests = append(manifests, CloneManifest(manifest))
	}
	authorizationManifests := make([]*Manifest, 0, len(manifests))
	for _, item := range manifests {
		if item == nil {
			continue
		}
		if item.ID == manifest.ID || manifestPublishesBusinessExtension(item) {
			authorizationManifests = append(authorizationManifests, item)
		}
	}
	discovery, err := BuildBusinessExtensionDiscovery(authorizationManifests)
	if err != nil {
		return nil, err
	}
	authorized := make([]*capmodel.BusinessExtensionDeclaration, 0, len(manifest.BusinessExtensions))
	for _, declaration := range manifest.BusinessExtensions {
		if declaration == nil {
			continue
		}
		if declaration.Role == capmodel.BusinessExtensionRoleOwner {
			owner := discovery.Owners[businessExtensionContractKey(declaration)]
			if owner != nil && owner.PluginID == manifest.ID {
				authorized = append(authorized, cloneBusinessExtensionDeclaration(declaration))
			}
			continue
		}
		for _, provider := range discovery.Providers[declaration.Point] {
			if provider.PluginID == manifest.ID && reflect.DeepEqual(provider.Declaration, declaration) {
				authorized = append(authorized, cloneBusinessExtensionDeclaration(declaration))
				break
			}
		}
	}
	if len(authorized) != len(manifest.BusinessExtensions) {
		return nil, gerror.Newf("plugin %s has business extension declarations that were not authorized", manifest.ID)
	}
	return authorized, nil
}

func manifestPublishesBusinessExtension(manifest *Manifest) bool {
	if manifest == nil {
		return false
	}
	for _, declaration := range manifest.BusinessExtensions {
		if declaration != nil && declaration.Role == capmodel.BusinessExtensionRoleOwner {
			return true
		}
	}
	return false
}

// ValidateBusinessExtensions validates and normalizes every business extension
// independently. Providers must explicitly depend on the plugin that owns the
// extension contract; plugin names are never used for capability inference.
func ValidateBusinessExtensions(manifest *Manifest) error {
	if manifest == nil {
		return nil
	}
	dependencies := pluginDependencyVersionMap(manifest.Dependencies)
	seen := make(map[string]struct{}, len(manifest.BusinessExtensions))
	providerGroups := make(map[string][]*capmodel.BusinessExtensionDeclaration)
	for index, declaration := range manifest.BusinessExtensions {
		if declaration == nil {
			return gerror.Newf("business extension declaration %d cannot be nil", index+1)
		}
		declaration.Point = strings.TrimSpace(declaration.Point)
		declaration.Version = strings.TrimSpace(declaration.Version)
		declaration.Role = strings.ToLower(strings.TrimSpace(declaration.Role))
		declaration.Owner = strings.TrimSpace(declaration.Owner)
		if !businessExtensionPointPattern.MatchString(declaration.Point) {
			return gerror.Newf("business extension point must use safe kebab-case: %s", declaration.Point)
		}
		if !contractVersionPattern.MatchString(declaration.Version) {
			return gerror.Newf("business extension %s contract version must use v1-style format", declaration.Point)
		}
		if err := plugintypes.ValidatePluginID(declaration.Owner); err != nil {
			return gerror.Wrapf(err, "business extension %s owner is invalid", declaration.Point)
		}
		if declaration.Role != capmodel.BusinessExtensionRoleOwner && declaration.Role != capmodel.BusinessExtensionRoleProvider {
			return gerror.Newf("business extension %s role only supports owner/provider", declaration.Point)
		}
		if declaration.Role == capmodel.BusinessExtensionRoleOwner && declaration.Owner != manifest.ID {
			return gerror.Newf("business extension owner declaration %s must be owned by current plugin %s", declaration.Point, manifest.ID)
		}
		if declaration.Role == capmodel.BusinessExtensionRoleProvider {
			if declaration.Owner == manifest.ID {
				return gerror.Newf("business extension provider %s cannot name itself as owner", declaration.Point)
			}
			if strings.TrimSpace(dependencies[declaration.Owner]) == "" {
				return gerror.Newf("business extension provider %s requires a versioned dependency on owner plugin %s", declaration.Point, declaration.Owner)
			}
		}
		methods, err := normalizeBusinessExtensionMethods(declaration.Methods)
		if err != nil {
			return gerror.Wrapf(err, "business extension %s methods are invalid", declaration.Point)
		}
		declaration.Methods = methods
		if declaration.Role != capmodel.BusinessExtensionRoleProvider && declaration.ProviderMetadata != nil {
			return gerror.Newf("business extension %s provider metadata is only valid for providers", declaration.Point)
		}
		if err = normalizeBusinessExtensionProviderMetadata(declaration.ProviderMetadata); err != nil {
			return gerror.Wrapf(err, "business extension %s provider metadata is invalid", declaration.Point)
		}
		if err = ValidateConfigurationDeclarations(declaration); err != nil {
			return gerror.Wrapf(err, "business extension %s configuration is invalid", declaration.Point)
		}
		key := businessExtensionDeclarationKey(declaration)
		if _, ok := seen[key]; ok {
			return gerror.Newf("business extension declaration is duplicated: %s", key)
		}
		seen[key] = struct{}{}
		if declaration.Role == capmodel.BusinessExtensionRoleProvider {
			groupKey := declaration.Owner + "\x00" + declaration.Point
			providerGroups[groupKey] = append(providerGroups[groupKey], declaration)
		}
	}
	return validateOwnerDefinedProviderMetadata(providerGroups)
}

func normalizeBusinessExtensionProviderMetadata(metadata *capmodel.BusinessExtensionProviderMetadata) error {
	if metadata == nil {
		return nil
	}
	metadata.Type = strings.ToLower(strings.TrimSpace(metadata.Type))
	metadata.Version = strings.TrimSpace(metadata.Version)
	if !businessExtensionPointPattern.MatchString(metadata.Type) {
		return gerror.New("provider metadata type must use safe kebab-case")
	}
	if !contractVersionPattern.MatchString(metadata.Version) {
		return gerror.New("provider metadata version must use v1-style format")
	}
	if metadata.Values == nil {
		return gerror.New("provider metadata values must be an object")
	}
	encoded, err := json.Marshal(metadata.Values)
	if err != nil || len(encoded) > 16*1024 {
		return gerror.New("provider metadata values must be JSON-compatible and no larger than 16 KiB")
	}
	normalized := make(map[string]any)
	if err = json.Unmarshal(encoded, &normalized); err != nil {
		return gerror.Wrap(err, "normalize provider metadata values")
	}
	metadata.Values = normalized
	return nil
}

func validateOwnerDefinedProviderMetadata(groups map[string][]*capmodel.BusinessExtensionDeclaration) error {
	for _, declarations := range groups {
		if len(declarations) == 0 {
			continue
		}
		if len(declarations) != 1 {
			return gerror.Newf(
				"business extension provider release must declare exactly one contract version: %s",
				declarations[0].Point,
			)
		}
		owner, ownerRegistered := pluginhost.GetSourcePlugin(declarations[0].Owner)
		for _, declaration := range declarations {
			validator, validatorRegistered := pluginhost.BusinessExtensionProviderValidator(nil), false
			if ownerRegistered {
				validator, validatorRegistered = owner.GetBusinessExtensionProviderValidator(declaration.Point, declaration.Version)
			}
			if !validatorRegistered {
				return gerror.Newf("business extension owner validator is unavailable: %s@%s", declaration.Point, declaration.Version)
			}
			if err := validator(CloneBusinessExtensionDeclarations(declarations)); err != nil {
				return gerror.Wrapf(err, "business extension owner rejected provider declaration %s@%s", declaration.Point, declaration.Version)
			}
		}
	}
	return nil
}

// ValidateFrontendSurfaces validates the two independently governed surface
// sets. Detailed slot and route ownership enforcement is performed by the
// frontend-surface assembler, while this boundary rejects ambiguous manifests.
func ValidateFrontendSurfaces(manifest *Manifest) error {
	if manifest == nil {
		return nil
	}
	if err := validateFrontendSurfaceSet("admin", manifest.AdminSurfaces); err != nil {
		return err
	}
	return validateFrontendSurfaceSet("customer", manifest.CustomerSurfaces)
}

// ValidatePageBridgeRouteBindings requires every business page capability to
// bind at least one exact dynamic route, and rejects route bindings that no
// governed surface declares. The route method and path are already part of the
// signed route contract, so the runtime can authorize the complete tuple.
func ValidatePageBridgeRouteBindings(manifest *Manifest, routes []*bridgecontract.RouteContract) error {
	if manifest == nil {
		return nil
	}
	declared := make(map[string]struct{})
	adminDeclared := make(map[string]struct{})
	for _, surfaces := range [][]*capmodel.FrontendSurfaceDeclaration{manifest.AdminSurfaces, manifest.CustomerSurfaces} {
		for _, surface := range surfaces {
			if surface == nil {
				continue
			}
			for _, capability := range surface.BridgeCapabilities {
				declared[strings.TrimSpace(capability)] = struct{}{}
			}
		}
	}
	for _, surface := range manifest.AdminSurfaces {
		if surface == nil {
			continue
		}
		for _, capability := range surface.BridgeCapabilities {
			adminDeclared[strings.TrimSpace(capability)] = struct{}{}
		}
	}
	bound := make(map[string]struct{})
	for _, route := range routes {
		if route == nil {
			continue
		}
		capability := strings.TrimSpace(route.Meta[pluginhost.RouteMetaPageBridgeCapability])
		if capability == "" {
			continue
		}
		if _, ok := declared[capability]; !ok {
			return gerror.Newf("dynamic route %s %s binds undeclared page bridge capability %s", route.Method, route.Path, capability)
		}
		if route.Access != bridgecontract.AccessLogin {
			return gerror.Newf("page bridge route must require login: %s %s", route.Method, route.Path)
		}
		if _, admin := adminDeclared[capability]; admin && strings.TrimSpace(route.Permission) == "" {
			return gerror.Newf("administration page bridge route requires a host permission: %s %s", route.Method, route.Path)
		}
		bound[capability] = struct{}{}
	}
	for capability := range declared {
		if _, ok := bound[capability]; !ok {
			return gerror.Newf("page bridge capability has no exact dynamic route binding: %s", capability)
		}
	}
	return nil
}

// DiscoverBusinessExtensions validates a complete already-loaded manifest
// batch and resolves providers against the matching owner publication. It is a
// pure in-memory O(n) projection and performs no database or filesystem reads.
func BuildBusinessExtensionDiscovery(manifests []*Manifest) (*BusinessExtensionDiscovery, error) {
	result := &BusinessExtensionDiscovery{
		Owners:    make(map[string]*BusinessExtensionBinding),
		Providers: make(map[string][]*BusinessExtensionBinding),
	}
	for _, manifest := range manifests {
		if manifest == nil {
			continue
		}
		if err := ValidateBusinessExtensions(manifest); err != nil {
			return nil, gerror.Wrapf(err, "plugin %s business extensions are invalid", manifest.ID)
		}
		for _, declaration := range manifest.BusinessExtensions {
			if declaration.Role != capmodel.BusinessExtensionRoleOwner {
				continue
			}
			key := businessExtensionContractKey(declaration)
			if existing := result.Owners[key]; existing != nil {
				return nil, gerror.Newf("business extension contract has multiple owners: %s (%s, %s)", key, existing.PluginID, manifest.ID)
			}
			result.Owners[key] = newBusinessExtensionBinding(manifest, declaration)
		}
	}
	for _, manifest := range manifests {
		if manifest == nil {
			continue
		}
		for _, declaration := range manifest.BusinessExtensions {
			if declaration.Role != capmodel.BusinessExtensionRoleProvider {
				continue
			}
			key := businessExtensionContractKey(declaration)
			owner := result.Owners[key]
			if owner == nil || owner.PluginID != declaration.Owner {
				return nil, gerror.Newf("plugin %s business extension %s has no matching owner publication", manifest.ID, key)
			}
			if !businessExtensionMethodsAllowed(owner.Declaration.Methods, declaration.Methods) {
				return nil, gerror.Newf("plugin %s business extension %s requests methods not published by owner %s", manifest.ID, key, declaration.Owner)
			}
			result.Providers[declaration.Point] = append(result.Providers[declaration.Point], newBusinessExtensionBinding(manifest, declaration))
		}
	}
	for point := range result.Providers {
		sort.Slice(result.Providers[point], func(i, j int) bool {
			return result.Providers[point][i].PluginID < result.Providers[point][j].PluginID
		})
	}
	return result, nil
}

func validateFrontendSurfaceSet(host string, surfaces []*capmodel.FrontendSurfaceDeclaration) error {
	seenKeys := make(map[string]struct{}, len(surfaces))
	seenRoutes := make(map[string]struct{}, len(surfaces))
	for index, surface := range surfaces {
		if surface == nil {
			return gerror.Newf("%s surface declaration %d cannot be nil", host, index+1)
		}
		surface.Key = strings.TrimSpace(surface.Key)
		surface.Version = strings.TrimSpace(surface.Version)
		surface.Title = strings.TrimSpace(surface.Title)
		surface.Icon = strings.TrimSpace(surface.Icon)
		surface.Permission = strings.ToLower(strings.TrimSpace(surface.Permission))
		surface.Slot = strings.TrimSpace(surface.Slot)
		surface.Route = strings.TrimSpace(surface.Route)
		surface.Entry = strings.TrimSpace(surface.Entry)
		surface.Status = strings.TrimSpace(surface.Status)
		if !surfaceKeyPattern.MatchString(surface.Key) {
			return gerror.Newf("%s surface key must use safe kebab-case: %s", host, surface.Key)
		}
		if !contractVersionPattern.MatchString(surface.Version) {
			return gerror.Newf("%s surface %s version must use v1-style format", host, surface.Key)
		}
		if surface.Title == "" || surface.Slot == "" || surface.Route == "" || surface.Entry == "" {
			return gerror.Newf("%s surface %s requires title, slot, route, and entry", host, surface.Key)
		}
		if _, ok := seenKeys[surface.Key]; ok {
			return gerror.Newf("%s surface key is duplicated: %s", host, surface.Key)
		}
		seenKeys[surface.Key] = struct{}{}
		normalizedRoute, err := normalizeFrontendSurfaceRoute(surface.Route)
		if err != nil {
			return gerror.Wrapf(err, "%s surface %s route is invalid", host, surface.Key)
		}
		if _, ok := seenRoutes[normalizedRoute]; ok {
			return gerror.Newf("%s surface route is duplicated: %s", host, normalizedRoute)
		}
		seenRoutes[normalizedRoute] = struct{}{}
		capabilities, err := normalizeBusinessExtensionMethods(surface.BridgeCapabilities)
		if err != nil {
			return gerror.Wrapf(err, "%s surface %s bridge capabilities are invalid", host, surface.Key)
		}
		surface.BridgeCapabilities = capabilities
	}
	return nil
}

func normalizeBusinessExtensionMethods(methods []string) ([]string, error) {
	seen := make(map[string]struct{}, len(methods))
	normalized := make([]string, 0, len(methods))
	for _, method := range methods {
		method = strings.ToLower(strings.TrimSpace(method))
		if !extensionMethodPattern.MatchString(method) {
			return nil, gerror.Newf("method must use safe lowercase identifier: %s", method)
		}
		if _, ok := seen[method]; ok {
			return nil, gerror.Newf("method is duplicated: %s", method)
		}
		seen[method] = struct{}{}
		normalized = append(normalized, method)
	}
	sort.Strings(normalized)
	return normalized, nil
}

func businessExtensionDeclarationKey(declaration *capmodel.BusinessExtensionDeclaration) string {
	return businessExtensionContractKey(declaration) + ":" + declaration.Role
}

func businessExtensionContractKey(declaration *capmodel.BusinessExtensionDeclaration) string {
	return declaration.Owner + ":" + declaration.Point + ":" + declaration.Version
}

func newBusinessExtensionBinding(manifest *Manifest, declaration *capmodel.BusinessExtensionDeclaration) *BusinessExtensionBinding {
	return &BusinessExtensionBinding{
		PluginID: manifest.ID, PluginName: manifest.Name, Type: manifest.Type,
		Distribution: manifest.Distribution, Declaration: cloneBusinessExtensionDeclaration(declaration),
	}
}

func businessExtensionMethodsAllowed(published, requested []string) bool {
	allowed := make(map[string]struct{}, len(published))
	for _, method := range published {
		allowed[method] = struct{}{}
	}
	for _, method := range requested {
		if _, ok := allowed[method]; !ok {
			return false
		}
	}
	return true
}
