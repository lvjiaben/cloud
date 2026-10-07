// This file owns catalog-local manifest read-model caches. Cached manifests are
// kept immutable by returning detached copies to callers, and dynamic artifacts
// are guarded by filesystem identity, size, and mtime so steady-state scans
// avoid reading full WASM files.

package catalog

import (
	"crypto/sha256"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/gogf/gf/v2/errors/gerror"

	hostconfig "lina-core/internal/service/config"
	"lina-core/internal/service/plugin/internal/plugintypes"
	"lina-core/internal/service/plugin/internal/resourcefs"
	"lina-core/pkg/plugin/capability/capmodel"
	"lina-core/pkg/plugin/pluginbridge/protocol"
)

// sourceManifestCacheEntry stores one parsed source manifest with the manifest
// file identity used to detect development-time source-plugin changes.
type sourceManifestCacheEntry struct {
	sourceKey string
	size      int64
	modTime   time.Time
	manifest  *Manifest
}

// runtimeArtifactCacheEntry stores one parsed runtime artifact manifest and the
// filesystem identity used to decide whether it can be reused.
type runtimeArtifactCacheEntry struct {
	fileInfo    os.FileInfo
	size        int64
	modTime     time.Time
	fingerprint [sha256.Size]byte
	manifest    *Manifest
}

// InvalidateManifestCache removes cached manifest projections for one plugin
// ID. Empty pluginID clears all dynamic manifest cache entries.
func (s *serviceImpl) InvalidateManifestCache(pluginID string) {
	if s == nil {
		return
	}
	normalizedPluginID := strings.TrimSpace(pluginID)
	s.cacheMu.Lock()
	defer s.cacheMu.Unlock()
	if normalizedPluginID == "" {
		s.sourceManifestCache = make(map[string]*sourceManifestCacheEntry)
		s.runtimeArtifactCache = make(map[string]*runtimeArtifactCacheEntry)
		s.runtimePluginArtifactIndex = make(map[string]string)
		return
	}
	delete(s.sourceManifestCache, normalizedPluginID)
	if artifactPath, ok := s.runtimePluginArtifactIndex[normalizedPluginID]; ok {
		delete(s.runtimeArtifactCache, artifactPath)
		delete(s.runtimePluginArtifactIndex, normalizedPluginID)
		return
	}
	for artifactPath, entry := range s.runtimeArtifactCache {
		if entry == nil || entry.manifest == nil {
			continue
		}
		if strings.TrimSpace(entry.manifest.ID) == normalizedPluginID {
			delete(s.runtimeArtifactCache, artifactPath)
		}
	}
	delete(s.runtimePluginArtifactIndex, normalizedPluginID)
}

// getCachedSourceManifest returns a cached source manifest when the current
// embedded or filesystem manifest stat still matches the cached identity.
func (s *serviceImpl) getCachedSourceManifest(pluginID string, sourcePlugin any, manifestFS fs.FS) (*Manifest, bool) {
	if s == nil || manifestFS == nil || strings.TrimSpace(pluginID) == "" {
		return nil, false
	}
	info, err := fs.Stat(manifestFS, resourcefs.EmbeddedManifestPath)
	if err != nil {
		return nil, false
	}
	s.cacheMu.RLock()
	entry := s.sourceManifestCache[strings.TrimSpace(pluginID)]
	sourceKey := sourceManifestCacheSourceKey(sourcePlugin)
	if entry != nil && entry.sourceKey == sourceKey && entry.size == info.Size() && entry.modTime.Equal(info.ModTime()) {
		manifest := CloneManifest(entry.manifest)
		s.cacheMu.RUnlock()
		return manifest, true
	}
	s.cacheMu.RUnlock()
	return nil, false
}

// storeCachedSourceManifest records a detached source manifest with the current
// plugin.yaml stat identity.
func (s *serviceImpl) storeCachedSourceManifest(pluginID string, sourcePlugin any, manifestFS fs.FS, manifest *Manifest) {
	if s == nil || manifestFS == nil || manifest == nil || strings.TrimSpace(pluginID) == "" {
		return
	}
	info, err := fs.Stat(manifestFS, resourcefs.EmbeddedManifestPath)
	if err != nil {
		return
	}
	s.cacheMu.Lock()
	if s.sourceManifestCache == nil {
		s.sourceManifestCache = make(map[string]*sourceManifestCacheEntry)
	}
	s.sourceManifestCache[strings.TrimSpace(pluginID)] = &sourceManifestCacheEntry{
		sourceKey: sourceManifestCacheSourceKey(sourcePlugin),
		size:      info.Size(),
		modTime:   info.ModTime(),
		manifest:  CloneManifest(manifest),
	}
	s.cacheMu.Unlock()
}

// sourceManifestCacheSourceKey distinguishes same-ID source-plugin registry
// replacements that keep the same plugin.yaml content but change callbacks or providers.
func sourceManifestCacheSourceKey(sourcePlugin any) string {
	if sourcePlugin == nil {
		return ""
	}
	value := reflect.ValueOf(sourcePlugin)
	if value.Kind() == reflect.Pointer && !value.IsNil() {
		return value.Type().String() + ":" + strconv.FormatUint(uint64(value.Pointer()), 10)
	}
	return value.Type().String()
}

// loadRuntimeManifestFromArtifactCache returns a cached dynamic manifest when
// the artifact path, size, and modification time are unchanged.
func (s *serviceImpl) loadRuntimeManifestFromArtifactCache(artifactPath string) (*Manifest, bool, error) {
	if s == nil {
		return nil, false, nil
	}
	key, info, err := runtimeArtifactCacheKey(artifactPath)
	if err != nil {
		return nil, false, err
	}
	fingerprint, err := runtimeArtifactFingerprint(key)
	if err != nil {
		return nil, false, err
	}
	s.cacheMu.RLock()
	entry := s.runtimeArtifactCache[key]
	if entry != nil && entry.size == info.Size() && entry.modTime.Equal(info.ModTime()) &&
		entry.fingerprint == fingerprint && sameRuntimeArtifactFile(entry.fileInfo, info) {
		manifest := CloneManifest(entry.manifest)
		s.cacheMu.RUnlock()
		return manifest, true, nil
	}
	s.cacheMu.RUnlock()
	return nil, false, nil
}

// storeRuntimeManifestArtifactCache records a parsed dynamic manifest using its
// filesystem identity. The cached value is detached from the caller-owned copy.
func (s *serviceImpl) storeRuntimeManifestArtifactCache(artifactPath string, manifest *Manifest) error {
	if s == nil || manifest == nil {
		return nil
	}
	key, info, err := runtimeArtifactCacheKey(artifactPath)
	if err != nil {
		return err
	}
	fingerprint, err := runtimeArtifactFingerprint(key)
	if err != nil {
		return err
	}
	cachedManifest := CloneManifest(manifest)
	s.cacheMu.Lock()
	s.runtimeArtifactCache[key] = &runtimeArtifactCacheEntry{
		fileInfo:    info,
		size:        info.Size(),
		modTime:     info.ModTime(),
		fingerprint: fingerprint,
		manifest:    cachedManifest,
	}
	if cachedManifest != nil && strings.TrimSpace(cachedManifest.ID) != "" {
		s.runtimePluginArtifactIndex[cachedManifest.ID] = key
	}
	s.cacheMu.Unlock()
	return nil
}

// runtimeArtifactFingerprint hashes the complete package so an in-place middle
// mutation cannot reuse a stale manifest when size and mtime are restored.
func runtimeArtifactFingerprint(artifactPath string) ([sha256.Size]byte, error) {
	var empty [sha256.Size]byte
	file, err := os.Open(artifactPath)
	if err != nil {
		return empty, gerror.Wrapf(err, "open dynamic plugin artifact failed: %s", artifactPath)
	}
	defer func() { _ = file.Close() }()

	hasher := sha256.New()
	if _, err = io.Copy(hasher, file); err != nil {
		return empty, gerror.Wrapf(err, "fingerprint complete dynamic plugin artifact failed: %s", artifactPath)
	}
	var fingerprint [sha256.Size]byte
	copy(fingerprint[:], hasher.Sum(nil))
	return fingerprint, nil
}

// sameRuntimeArtifactFile reports whether two stat results still point to the
// same underlying artifact. Atomic file replacement can preserve path, size,
// and mtime on coarse filesystems, so inode/device identity keeps the stat guard
// from serving a stale desired manifest after a package overwrite.
func sameRuntimeArtifactFile(cached os.FileInfo, current os.FileInfo) bool {
	if cached == nil || current == nil {
		return false
	}
	if !os.SameFile(cached, current) {
		return false
	}
	cachedGeneration, cachedOK := runtimeArtifactGeneration(cached)
	currentGeneration, currentOK := runtimeArtifactGeneration(current)
	if cachedOK && currentOK {
		return reflect.DeepEqual(cachedGeneration, currentGeneration)
	}
	return true
}

// runtimeArtifactGeneration returns the platform stat field that changes when
// a path is atomically replaced even if its size and modification time are
// deliberately preserved. Windows exposes CreationTime while Unix platforms
// expose a change-time field.
func runtimeArtifactGeneration(info os.FileInfo) (any, bool) {
	if info == nil || info.Sys() == nil {
		return nil, false
	}
	value := reflect.ValueOf(info.Sys())
	for value.Kind() == reflect.Pointer {
		if value.IsNil() {
			return nil, false
		}
		value = value.Elem()
	}
	if value.Kind() != reflect.Struct {
		return nil, false
	}
	for _, fieldName := range []string{"Ctim", "Ctimespec", "Birthtimespec", "CreationTime"} {
		field := value.FieldByName(fieldName)
		if field.IsValid() && field.CanInterface() {
			return field.Interface(), true
		}
	}
	return nil, false
}

// recordRuntimeArtifactParse increments a package-private counter used by
// cache-boundary tests to prove full artifact parsing stays bounded.
func (s *serviceImpl) recordRuntimeArtifactParse(artifactPath string) {
	if s == nil {
		return
	}
	key := filepath.Clean(strings.TrimSpace(artifactPath))
	if key == "" {
		return
	}
	s.cacheMu.Lock()
	if s.parseCounts == nil {
		s.parseCounts = make(map[string]int)
	}
	s.parseCounts[key]++
	s.cacheMu.Unlock()
}

// runtimeArtifactCacheKey normalizes artifact path identity and returns its
// current filesystem metadata.
func runtimeArtifactCacheKey(artifactPath string) (string, os.FileInfo, error) {
	normalizedPath := strings.TrimSpace(artifactPath)
	if normalizedPath == "" {
		return "", nil, gerror.New("dynamic plugin artifact path cannot be empty")
	}
	absolutePath := normalizedPath
	if !filepath.IsAbs(absolutePath) {
		var err error
		absolutePath, err = filepath.Abs(absolutePath)
		if err != nil {
			return "", nil, err
		}
	}
	key := filepath.Clean(absolutePath)
	info, err := os.Stat(key)
	if err != nil {
		return "", nil, gerror.Wrapf(err, "stat dynamic plugin artifact failed: %s", key)
	}
	if info.IsDir() {
		return "", nil, gerror.Newf("dynamic plugin artifact path is a directory: %s", key)
	}
	return key, info, nil
}

// CloneManifest returns a detached copy of one plugin manifest so cached read
// models remain immutable across callers.
func CloneManifest(manifest *Manifest) *Manifest {
	if manifest == nil {
		return nil
	}
	out := *manifest
	out.SupportsMultiTenant = cloneBoolPtr(manifest.SupportsMultiTenant)
	out.I18N = cloneI18NConfig(manifest.I18N)
	out.Dependencies = plugintypes.CloneDependencySpec(manifest.Dependencies)
	out.BusinessExtensions = cloneBusinessExtensionDeclarations(manifest.BusinessExtensions)
	out.AdminSurfaces = cloneFrontendSurfaceDeclarations(manifest.AdminSurfaces)
	out.CustomerSurfaces = cloneFrontendSurfaceDeclarations(manifest.CustomerSurfaces)
	out.Menus = cloneMenuSpecs(manifest.Menus)
	out.PublicAssets = ClonePublicAssetSpecs(manifest.PublicAssets)
	out.Hooks = CloneHookSpecs(manifest.Hooks)
	out.LifecycleHandlers = CloneLifecycleContracts(manifest.LifecycleHandlers)
	out.BackendResources = cloneResourceSpecMap(manifest.BackendResources)
	out.Routes = cloneRouteContracts(manifest.Routes)
	out.BridgeSpec = cloneBridgeSpec(manifest.BridgeSpec)
	out.HostCapabilities = cloneStringSet(manifest.HostCapabilities)
	out.HostServices = cloneHostServiceSpecs(manifest.HostServices)
	out.RuntimeArtifact = cloneArtifactSpec(manifest.RuntimeArtifact)
	return &out
}

// cloneBoolPtr returns a detached copy of one bool pointer.
func cloneBoolPtr(value *bool) *bool {
	if value == nil {
		return nil
	}
	out := *value
	return &out
}

// cloneI18NConfig returns a detached copy of the host-shaped i18n config.
func cloneI18NConfig(config *hostconfig.I18nConfig) *hostconfig.I18nConfig {
	if config == nil {
		return nil
	}
	out := *config
	if len(config.Locales) > 0 {
		out.Locales = append([]hostconfig.I18nLocaleConfig(nil), config.Locales...)
	}
	return &out
}

// cloneMenuSpecs returns detached menu specs including query maps.
func cloneMenuSpecs(items []*MenuSpec) []*MenuSpec {
	if len(items) == 0 {
		return []*MenuSpec{}
	}
	out := make([]*MenuSpec, 0, len(items))
	for _, item := range items {
		if item == nil {
			continue
		}
		next := *item
		next.Visible = cloneIntPtr(item.Visible)
		next.Status = cloneIntPtr(item.Status)
		next.IsFrame = cloneIntPtr(item.IsFrame)
		next.IsCache = cloneIntPtr(item.IsCache)
		if len(item.Query) > 0 {
			next.Query = make(map[string]interface{}, len(item.Query))
			for key, value := range item.Query {
				next.Query[key] = value
			}
		}
		out = append(out, &next)
	}
	return out
}

// cloneIntPtr returns a detached copy of one int pointer.
func cloneIntPtr(value *int) *int {
	if value == nil {
		return nil
	}
	out := *value
	return &out
}

// cloneResourceSpecMap returns detached backend resource specs keyed by ID.
func cloneResourceSpecMap(items map[string]*ResourceSpec) map[string]*ResourceSpec {
	if len(items) == 0 {
		return map[string]*ResourceSpec{}
	}
	out := make(map[string]*ResourceSpec, len(items))
	for key, item := range items {
		out[key] = CloneResourceSpec(item)
	}
	return out
}

// cloneRouteContracts returns detached dynamic route contracts.
func cloneRouteContracts(items []*protocol.RouteContract) []*protocol.RouteContract {
	if len(items) == 0 {
		return []*protocol.RouteContract{}
	}
	out := make([]*protocol.RouteContract, 0, len(items))
	for _, item := range items {
		if item == nil {
			continue
		}
		next := *item
		next.Tags = append([]string(nil), item.Tags...)
		if len(item.Meta) > 0 {
			next.Meta = make(map[string]string, len(item.Meta))
			for key, value := range item.Meta {
				next.Meta[key] = value
			}
		}
		out = append(out, &next)
	}
	return out
}

// cloneBridgeSpec returns a detached bridge ABI specification.
func cloneBridgeSpec(spec *protocol.BridgeSpec) *protocol.BridgeSpec {
	if spec == nil {
		return nil
	}
	out := *spec
	return &out
}

// cloneStringSet returns a detached string set.
func cloneStringSet(items map[string]struct{}) map[string]struct{} {
	if len(items) == 0 {
		return map[string]struct{}{}
	}
	out := make(map[string]struct{}, len(items))
	for key, value := range items {
		out[key] = value
	}
	return out
}

// cloneHostServiceSpecs returns detached host service declarations.
func cloneHostServiceSpecs(items []*protocol.HostServiceSpec) []*protocol.HostServiceSpec {
	if len(items) == 0 {
		return []*protocol.HostServiceSpec{}
	}
	out := make([]*protocol.HostServiceSpec, 0, len(items))
	for _, item := range items {
		if item == nil {
			continue
		}
		next := *item
		next.Methods = append([]string(nil), item.Methods...)
		next.Paths = append([]string(nil), item.Paths...)
		next.Tables = append([]string(nil), item.Tables...)
		next.Keys = append([]string(nil), item.Keys...)
		next.Resources = cloneHostServiceResourceSpecs(item.Resources)
		out = append(out, &next)
	}
	return out
}

// cloneHostServiceResourceSpecs returns detached host service resource specs.
func cloneHostServiceResourceSpecs(items []*protocol.HostServiceResourceSpec) []*protocol.HostServiceResourceSpec {
	if len(items) == 0 {
		return []*protocol.HostServiceResourceSpec{}
	}
	out := make([]*protocol.HostServiceResourceSpec, 0, len(items))
	for _, item := range items {
		if item == nil {
			continue
		}
		next := *item
		next.AllowMethods = append([]string(nil), item.AllowMethods...)
		next.HeaderAllowList = append([]string(nil), item.HeaderAllowList...)
		if len(item.Attributes) > 0 {
			next.Attributes = make(map[string]string, len(item.Attributes))
			for key, value := range item.Attributes {
				next.Attributes[key] = value
			}
		}
		out = append(out, &next)
	}
	return out
}

// cloneArtifactSpec returns a detached runtime artifact projection.
func cloneArtifactSpec(item *ArtifactSpec) *ArtifactSpec {
	if item == nil {
		return nil
	}
	out := *item
	out.Manifest = cloneArtifactManifest(item.Manifest)
	out.FrontendAssets = cloneArtifactFrontendAssets(item.FrontendAssets)
	out.InstallSQLAssets = cloneArtifactSQLAssets(item.InstallSQLAssets)
	out.UninstallSQLAssets = cloneArtifactSQLAssets(item.UninstallSQLAssets)
	out.MockSQLAssets = cloneArtifactSQLAssets(item.MockSQLAssets)
	out.ManifestResources = cloneArtifactManifestResources(item.ManifestResources)
	out.HookSpecs = CloneHookSpecs(item.HookSpecs)
	out.LifecycleContracts = CloneLifecycleContracts(item.LifecycleContracts)
	if len(item.ResourceSpecs) > 0 {
		out.ResourceSpecs = make([]*ResourceSpec, 0, len(item.ResourceSpecs))
		for _, resource := range item.ResourceSpecs {
			out.ResourceSpecs = append(out.ResourceSpecs, CloneResourceSpec(resource))
		}
	}
	out.RouteContracts = cloneRouteContracts(item.RouteContracts)
	out.BridgeSpec = cloneBridgeSpec(item.BridgeSpec)
	out.Capabilities = append([]string(nil), item.Capabilities...)
	out.HostServices = cloneHostServiceSpecs(item.HostServices)
	return &out
}

// cloneArtifactManifest returns a detached embedded manifest identity snapshot.
func cloneArtifactManifest(item *ArtifactManifest) *ArtifactManifest {
	if item == nil {
		return nil
	}
	out := *item
	out.SupportsMultiTenant = cloneBoolPtr(item.SupportsMultiTenant)
	out.Dependencies = plugintypes.CloneDependencySpec(item.Dependencies)
	out.BusinessExtensions = cloneBusinessExtensionDeclarations(item.BusinessExtensions)
	out.AdminSurfaces = cloneFrontendSurfaceDeclarations(item.AdminSurfaces)
	out.CustomerSurfaces = cloneFrontendSurfaceDeclarations(item.CustomerSurfaces)
	out.Menus = cloneMenuSpecs(item.Menus)
	out.PublicAssets = ClonePublicAssetSpecs(item.PublicAssets)
	return &out
}

func cloneBusinessExtensionDeclaration(item *capmodel.BusinessExtensionDeclaration) *capmodel.BusinessExtensionDeclaration {
	if item == nil {
		return nil
	}
	out := *item
	out.Methods = append([]string(nil), item.Methods...)
	out.InterfaceConfiguration = cloneConfigurationSchemaDeclaration(item.InterfaceConfiguration)
	out.ProductConfiguration = cloneConfigurationSchemaDeclaration(item.ProductConfiguration)
	out.CustomerConfiguration = cloneConfigurationSchemaDeclaration(item.CustomerConfiguration)
	out.ProviderMetadata = cloneBusinessExtensionProviderMetadata(item.ProviderMetadata)
	return &out
}

func cloneBusinessExtensionProviderMetadata(item *capmodel.BusinessExtensionProviderMetadata) *capmodel.BusinessExtensionProviderMetadata {
	if item == nil {
		return nil
	}
	out := *item
	out.Values = cloneOpaqueValues(item.Values)
	return &out
}

func cloneOpaqueValues(values map[string]any) map[string]any {
	if values == nil {
		return nil
	}
	out := make(map[string]any, len(values))
	for key, value := range values {
		out[key] = cloneOpaqueValue(value)
	}
	return out
}

func cloneOpaqueValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		return cloneOpaqueValues(typed)
	case []any:
		out := make([]any, len(typed))
		for index := range typed {
			out[index] = cloneOpaqueValue(typed[index])
		}
		return out
	default:
		return typed
	}
}

func cloneConfigurationSchemaDeclaration(item *capmodel.ConfigurationSchemaDeclaration) *capmodel.ConfigurationSchemaDeclaration {
	if item == nil {
		return nil
	}
	out := *item
	out.Fields = make([]*capmodel.ConfigurationFieldDeclaration, 0, len(item.Fields))
	for _, field := range item.Fields {
		if field == nil {
			out.Fields = append(out.Fields, nil)
			continue
		}
		next := *field
		next.Label = cloneLocalizedText(field.Label)
		next.Description = cloneLocalizedText(field.Description)
		next.StaticCandidates = make([]*capmodel.ConfigurationCandidate, 0, len(field.StaticCandidates))
		for _, candidate := range field.StaticCandidates {
			if candidate == nil {
				next.StaticCandidates = append(next.StaticCandidates, nil)
				continue
			}
			clonedCandidate := *candidate
			clonedCandidate.Label = cloneLocalizedText(candidate.Label)
			next.StaticCandidates = append(next.StaticCandidates, &clonedCandidate)
		}
		if field.DynamicCandidates != nil {
			clonedDynamic := *field.DynamicCandidates
			next.DynamicCandidates = &clonedDynamic
		}
		out.Fields = append(out.Fields, &next)
	}
	out.Conditions = make([]*capmodel.ConfigurationRuleDeclaration, 0, len(item.Conditions))
	for _, rule := range item.Conditions {
		if rule == nil {
			out.Conditions = append(out.Conditions, nil)
			continue
		}
		next := *rule
		next.When = cloneConfigurationConditionDeclaration(rule.When)
		next.Actions = make([]*capmodel.ConfigurationActionDeclaration, 0, len(rule.Actions))
		for _, action := range rule.Actions {
			if action == nil {
				next.Actions = append(next.Actions, nil)
				continue
			}
			clonedAction := *action
			clonedAction.Values = append([]string(nil), action.Values...)
			next.Actions = append(next.Actions, &clonedAction)
		}
		out.Conditions = append(out.Conditions, &next)
	}
	out.UploadPolicies = make([]*capmodel.ConfigurationUploadPolicyDeclaration, 0, len(item.UploadPolicies))
	for _, policy := range item.UploadPolicies {
		if policy == nil {
			out.UploadPolicies = append(out.UploadPolicies, nil)
			continue
		}
		next := *policy
		next.AllowedMIMETypes = append([]string(nil), policy.AllowedMIMETypes...)
		next.AllowedExtensions = append([]string(nil), policy.AllowedExtensions...)
		out.UploadPolicies = append(out.UploadPolicies, &next)
	}
	return &out
}

func cloneConfigurationConditionDeclaration(item *capmodel.ConfigurationConditionDeclaration) *capmodel.ConfigurationConditionDeclaration {
	if item == nil {
		return nil
	}
	out := *item
	out.Children = make([]*capmodel.ConfigurationConditionDeclaration, 0, len(item.Children))
	for _, child := range item.Children {
		out.Children = append(out.Children, cloneConfigurationConditionDeclaration(child))
	}
	return &out
}

func cloneLocalizedText(item *capmodel.LocalizedText) *capmodel.LocalizedText {
	if item == nil {
		return nil
	}
	out := *item
	return &out
}

func cloneBusinessExtensionDeclarations(items []*capmodel.BusinessExtensionDeclaration) []*capmodel.BusinessExtensionDeclaration {
	if len(items) == 0 {
		return []*capmodel.BusinessExtensionDeclaration{}
	}
	out := make([]*capmodel.BusinessExtensionDeclaration, 0, len(items))
	for _, item := range items {
		if cloned := cloneBusinessExtensionDeclaration(item); cloned != nil {
			out = append(out, cloned)
		}
	}
	return out
}

// CloneBusinessExtensionDeclarations returns detached business extension declarations.
func CloneBusinessExtensionDeclarations(items []*capmodel.BusinessExtensionDeclaration) []*capmodel.BusinessExtensionDeclaration {
	return cloneBusinessExtensionDeclarations(items)
}

func cloneFrontendSurfaceDeclarations(items []*capmodel.FrontendSurfaceDeclaration) []*capmodel.FrontendSurfaceDeclaration {
	if len(items) == 0 {
		return []*capmodel.FrontendSurfaceDeclaration{}
	}
	out := make([]*capmodel.FrontendSurfaceDeclaration, 0, len(items))
	for _, item := range items {
		if item == nil {
			continue
		}
		next := *item
		next.BridgeCapabilities = append([]string(nil), item.BridgeCapabilities...)
		out = append(out, &next)
	}
	return out
}

// CloneFrontendSurfaceDeclarations returns detached governed frontend surfaces.
func CloneFrontendSurfaceDeclarations(items []*capmodel.FrontendSurfaceDeclaration) []*capmodel.FrontendSurfaceDeclaration {
	return cloneFrontendSurfaceDeclarations(items)
}

// cloneArtifactFrontendAssets returns detached frontend asset payloads.
func cloneArtifactFrontendAssets(items []*ArtifactFrontendAsset) []*ArtifactFrontendAsset {
	if len(items) == 0 {
		return []*ArtifactFrontendAsset{}
	}
	out := make([]*ArtifactFrontendAsset, 0, len(items))
	for _, item := range items {
		if item == nil {
			continue
		}
		next := *item
		next.Content = append([]byte(nil), item.Content...)
		out = append(out, &next)
	}
	return out
}

// cloneArtifactSQLAssets returns detached SQL asset payloads.
func cloneArtifactSQLAssets(items []*ArtifactSQLAsset) []*ArtifactSQLAsset {
	if len(items) == 0 {
		return []*ArtifactSQLAsset{}
	}
	out := make([]*ArtifactSQLAsset, 0, len(items))
	for _, item := range items {
		if item == nil {
			continue
		}
		next := *item
		out = append(out, &next)
	}
	return out
}

// cloneArtifactManifestResources returns detached manifest resource payloads.
func cloneArtifactManifestResources(items []*ArtifactManifestResource) []*ArtifactManifestResource {
	if len(items) == 0 {
		return []*ArtifactManifestResource{}
	}
	out := make([]*ArtifactManifestResource, 0, len(items))
	for _, item := range items {
		if item == nil {
			continue
		}
		next := *item
		next.Content = append([]byte(nil), item.Content...)
		out = append(out, &next)
	}
	return out
}
