// This file owns new-business admission and trusted owner impact checks. These
// checks are separate from plugin runtime enablement so existing services stay
// visible after new sales stop.

package lifecycle

import (
	"context"
	"sort"
	"strings"

	pluginv1 "lina-core/api/plugin/v1"
	"lina-core/internal/service/plugin/internal/plugintypes"
	"lina-core/internal/service/plugin/internal/store"
	"lina-core/pkg/bizerr"
	"lina-core/pkg/plugin/capability/capmodel"
	"lina-core/pkg/plugin/pluginhost"
	"lina-core/pkg/statusflag"
)

// BusinessImpactItem is one trusted business owner's aggregate lifecycle impact.
type BusinessImpactItem struct {
	OwnerPluginID string
	Category      pluginhost.BusinessImpactCategory
	TotalCount    int64
	BlockingCount int64
}

// BusinessImpactReport is the complete bounded impact projection for one target.
type BusinessImpactReport struct {
	PluginID      string
	Items         []BusinessImpactItem
	BlockingCount int64
}

// StopNewBusiness prevents new binding, purchase, and fulfillment candidates
// without disabling the plugin or hiding existing-service surfaces.
func (s *serviceImpl) StopNewBusiness(ctx context.Context, pluginID string) error {
	registry, err := s.requireInstalledRegistry(ctx, pluginID)
	if err != nil {
		return err
	}
	if plugintypes.NormalizeNewBusinessState(registry.NewBusinessState) == plugintypes.NewBusinessStateStopped {
		return nil
	}
	if err = s.storeSvc.SetPluginNewBusinessState(ctx, pluginID, plugintypes.NewBusinessStateStopped); err != nil {
		return err
	}
	return s.syncEnabledSnapshotAndPublishRuntimeChange(ctx, pluginID, "plugin_new_business_stopped")
}

// ResumeNewBusiness reopens admission only when the current installed release,
// enablement, runtime state, and hard dependencies are ready for new work.
func (s *serviceImpl) ResumeNewBusiness(ctx context.Context, pluginID string, frameworkVersion string) error {
	registry, err := s.requireInstalledRegistry(ctx, pluginID)
	if err != nil {
		return err
	}
	if registry.Status != statusflag.EnabledValue.Int() {
		return bizerr.NewCode(CodePluginNewBusinessResumeNotReady, bizerr.P("pluginId", pluginID))
	}
	manifest, err := s.catalogSvc.GetDesiredManifest(pluginID)
	if err != nil {
		return bizerr.WrapCode(err, CodePluginNewBusinessResumeNotReady, bizerr.P("pluginId", pluginID))
	}
	runtimeState, err := s.storeSvc.BuildRuntimeUpgradeState(ctx, registry, manifest)
	if err != nil {
		return bizerr.WrapCode(err, CodePluginNewBusinessResumeNotReady, bizerr.P("pluginId", pluginID))
	}
	if !store.RuntimeStateAllowsBusinessEntry(runtimeState.State) {
		return bizerr.NewCode(CodePluginNewBusinessResumeNotReady, bizerr.P("pluginId", pluginID))
	}
	if err = s.ensureEnableDependencies(ctx, pluginID, frameworkVersion); err != nil {
		return err
	}
	if plugintypes.NormalizeType(manifest.Type) == pluginv1.PluginTypeDynamic {
		if err = s.runtimeSvc.EnsureRuntimeArtifactAvailable(manifest, "resume-new-business"); err != nil {
			return bizerr.WrapCode(err, CodePluginNewBusinessResumeNotReady, bizerr.P("pluginId", pluginID))
		}
	}
	if plugintypes.NormalizeNewBusinessState(registry.NewBusinessState) == plugintypes.NewBusinessStateAccepting {
		return nil
	}
	if err = s.storeSvc.SetPluginNewBusinessState(ctx, pluginID, plugintypes.NewBusinessStateAccepting); err != nil {
		return err
	}
	return s.syncEnabledSnapshotAndPublishRuntimeChange(ctx, pluginID, "plugin_new_business_resumed")
}

// CanAcceptNewBusiness is the authoritative gate used by future provider,
// interface-instance, product-version, purchase, and fulfillment candidates.
func (s *serviceImpl) CanAcceptNewBusiness(ctx context.Context, pluginID string) (bool, error) {
	registry, err := s.storeSvc.GetRegistry(ctx, strings.TrimSpace(pluginID))
	if err != nil {
		return false, err
	}
	if registry == nil || registry.Installed != statusflag.Installed.Int() ||
		registry.Status != statusflag.EnabledValue.Int() ||
		plugintypes.NormalizeNewBusinessState(registry.NewBusinessState) != plugintypes.NewBusinessStateAccepting {
		return false, nil
	}
	manifest, err := s.catalogSvc.GetDesiredManifest(pluginID)
	if err != nil {
		return false, nil
	}
	runtimeState, err := s.storeSvc.BuildRuntimeUpgradeState(ctx, registry, manifest)
	if err != nil {
		return false, err
	}
	return store.RuntimeStateAllowsBusinessEntry(runtimeState.State), nil
}

// CheckBusinessImpact asks each registered stable source-plugin owner for one
// aggregate query. Object-by-object reads and private details are forbidden.
func (s *serviceImpl) CheckBusinessImpact(ctx context.Context, pluginID string) (*BusinessImpactReport, error) {
	registry, err := s.requireInstalledRegistry(ctx, pluginID)
	if err != nil {
		return nil, err
	}
	requiredOwners, err := s.requiredBusinessImpactOwners(ctx, registry)
	if err != nil {
		return nil, err
	}
	providers := pluginhost.ListSourcePluginBusinessImpactProviders()
	providerByID := make(map[string]pluginhost.SourcePluginDefinition, len(providers))
	for _, provider := range providers {
		providerByID[provider.ID()] = provider
	}
	report := &BusinessImpactReport{PluginID: pluginID, Items: make([]BusinessImpactItem, 0, len(requiredOwners)*4)}
	for _, ownerPluginID := range requiredOwners {
		provider := providerByID[ownerPluginID]
		if provider == nil || provider.GetBusinessImpactHandler() == nil {
			return nil, invalidBusinessImpactProviderResult(pluginID, ownerPluginID)
		}
		handler := provider.GetBusinessImpactHandler()
		items, err := handler(ctx, pluginhost.PluginBusinessImpactInput{TargetPluginID: pluginID})
		if err != nil {
			return nil, bizerr.WrapCode(
				err,
				CodePluginBusinessImpactCheckFailed,
				bizerr.P("pluginId", pluginID),
				bizerr.P("ownerPluginId", provider.ID()),
			)
		}
		if len(items) > 4 {
			return nil, invalidBusinessImpactProviderResult(pluginID, provider.ID())
		}
		seen := make(map[pluginhost.BusinessImpactCategory]struct{}, len(items))
		for _, item := range items {
			if !pluginhost.IsSupportedBusinessImpactCategory(item.Category) ||
				item.TotalCount < 0 || item.BlockingCount < 0 || item.BlockingCount > item.TotalCount {
				return nil, invalidBusinessImpactProviderResult(pluginID, provider.ID())
			}
			if _, exists := seen[item.Category]; exists {
				return nil, invalidBusinessImpactProviderResult(pluginID, provider.ID())
			}
			seen[item.Category] = struct{}{}
			if item.TotalCount == 0 {
				continue
			}
			report.Items = append(report.Items, BusinessImpactItem{
				OwnerPluginID: provider.ID(), Category: item.Category,
				TotalCount: item.TotalCount, BlockingCount: item.BlockingCount,
			})
			report.BlockingCount += item.BlockingCount
		}
	}
	return report, nil
}

// requiredBusinessImpactOwners derives the complete trusted owner set from
// the installed active release snapshot. Staged or desired manifests are not
// allowed to weaken lifecycle protection for the release serving customers.
func (s *serviceImpl) requiredBusinessImpactOwners(
	ctx context.Context,
	registry *store.PluginRecord,
) ([]string, error) {
	snapshot, err := s.activeBusinessManifestSnapshot(ctx, registry)
	if err != nil {
		return nil, err
	}
	if len(snapshot.BusinessExtensions) > 0 && len(snapshot.AuthorizedBusinessExtensions) == 0 {
		return nil, invalidBusinessImpactProviderResult(registry.PluginId, "unknown")
	}
	owners := make(map[string]struct{}, len(snapshot.AuthorizedBusinessExtensions))
	for _, declaration := range snapshot.AuthorizedBusinessExtensions {
		if declaration == nil || declaration.Role != capmodel.BusinessExtensionRoleProvider {
			continue
		}
		ownerPluginID := strings.TrimSpace(declaration.Owner)
		if ownerPluginID == "" {
			return nil, invalidBusinessImpactProviderResult(registry.PluginId, "unknown")
		}
		owners[ownerPluginID] = struct{}{}
	}
	result := make([]string, 0, len(owners))
	for ownerPluginID := range owners {
		result = append(result, ownerPluginID)
	}
	sort.Strings(result)
	return result, nil
}

// activeBusinessManifestSnapshot reads only the release frozen by the registry
// pointer. Missing or malformed history fails closed instead of falling back to
// a mutable discovered manifest.
func (s *serviceImpl) activeBusinessManifestSnapshot(
	ctx context.Context,
	registry *store.PluginRecord,
) (*store.ManifestSnapshot, error) {
	if registry == nil {
		return nil, bizerr.NewCode(CodePluginBusinessImpactCheckFailed)
	}
	release, err := s.storeSvc.GetRegistryRelease(ctx, registry)
	if err != nil {
		return nil, bizerr.WrapCode(err, CodePluginBusinessImpactCheckFailed, bizerr.P("pluginId", registry.PluginId))
	}
	if release == nil {
		return nil, bizerr.NewCode(CodePluginBusinessImpactCheckFailed, bizerr.P("pluginId", registry.PluginId))
	}
	snapshot, err := s.storeSvc.ParseManifestSnapshot(release.ManifestSnapshot)
	if err != nil {
		return nil, bizerr.WrapCode(err, CodePluginBusinessImpactCheckFailed, bizerr.P("pluginId", registry.PluginId))
	}
	if snapshot == nil {
		return nil, bizerr.NewCode(CodePluginBusinessImpactCheckFailed, bizerr.P("pluginId", registry.PluginId))
	}
	return snapshot, nil
}

func (s *serviceImpl) activeReleaseHasBusinessEvidence(
	ctx context.Context,
	registry *store.PluginRecord,
) (bool, error) {
	snapshot, err := s.activeBusinessManifestSnapshot(ctx, registry)
	if err != nil {
		return false, err
	}
	return len(snapshot.BusinessExtensions) > 0 || len(snapshot.AuthorizedBusinessExtensions) > 0, nil
}

// ensurePluginBusinessDrained is a non-bypassable host safety gate. Force
// uninstall may recover broken artifacts, but it cannot waive live business.
func (s *serviceImpl) ensurePluginBusinessDrained(ctx context.Context, pluginID string) error {
	report, err := s.CheckBusinessImpact(ctx, pluginID)
	if err != nil {
		return err
	}
	if report.BlockingCount == 0 {
		return nil
	}
	return bizerr.NewCode(
		CodePluginBusinessReferencesBlocked,
		bizerr.P("pluginId", pluginID),
		bizerr.P("blockingCount", report.BlockingCount),
	)
}

func (s *serviceImpl) ensurePluginStoppedNewBusiness(ctx context.Context, pluginID string) error {
	registry, err := s.requireInstalledRegistry(ctx, pluginID)
	if err != nil {
		return err
	}
	if plugintypes.NormalizeNewBusinessState(registry.NewBusinessState) == plugintypes.NewBusinessStateStopped {
		return nil
	}
	return bizerr.NewCode(CodePluginNewBusinessMustStopFirst, bizerr.P("pluginId", pluginID))
}

func (s *serviceImpl) requireInstalledRegistry(ctx context.Context, pluginID string) (*store.PluginRecord, error) {
	registry, err := s.storeSvc.GetRegistry(ctx, strings.TrimSpace(pluginID))
	if err != nil {
		return nil, err
	}
	if registry == nil || registry.Installed != statusflag.Installed.Int() {
		return nil, bizerr.NewCode(CodePluginNotInstalled)
	}
	return registry, nil
}

func invalidBusinessImpactProviderResult(pluginID string, ownerPluginID string) error {
	return bizerr.NewCode(
		CodePluginBusinessImpactCheckFailed,
		bizerr.P("pluginId", pluginID),
		bizerr.P("ownerPluginId", ownerPluginID),
	)
}
