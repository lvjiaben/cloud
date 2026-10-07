// This file verifies plugin root-composition delegates fail fast when startup
// wiring is incomplete and remain observable after binding.

package plugin

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/gogf/gf/v2/net/goai"

	"lina-core/internal/service/cachecoord"
	"lina-core/internal/service/cachecoord/revisionctrl"
	"lina-core/internal/service/plugin/internal/catalog"
	"lina-core/pkg/bizerr"
	"lina-core/pkg/plugin/pluginhost"
)

type pageBridgeGrantFailingCacheCoord struct {
	err error
}

func (f *pageBridgeGrantFailingCacheCoord) ConfigureDomain(cachecoord.DomainSpec) error {
	return nil
}

func (f *pageBridgeGrantFailingCacheCoord) MarkChanged(
	context.Context,
	cachecoord.Domain,
	cachecoord.Scope,
	cachecoord.ChangeReason,
) (int64, error) {
	return 0, f.err
}

func (f *pageBridgeGrantFailingCacheCoord) MarkTenantChanged(
	context.Context,
	cachecoord.Domain,
	cachecoord.Scope,
	cachecoord.InvalidationScope,
	cachecoord.ChangeReason,
) (int64, error) {
	return 0, f.err
}

func (f *pageBridgeGrantFailingCacheCoord) EnsureFresh(
	context.Context,
	cachecoord.Domain,
	cachecoord.Scope,
	cachecoord.Refresher,
) (int64, error) {
	return 0, f.err
}

func (f *pageBridgeGrantFailingCacheCoord) CurrentRevision(
	context.Context,
	cachecoord.Domain,
	cachecoord.Scope,
) (int64, error) {
	return 0, f.err
}

func (f *pageBridgeGrantFailingCacheCoord) Snapshot(context.Context) ([]cachecoord.SnapshotItem, error) {
	return nil, f.err
}

// TestRuntimeDelegateReportsUnboundSideEffects verifies side-effecting
// delegate calls do not silently succeed before startup binding completes.
func TestRuntimeDelegateReportsUnboundSideEffects(t *testing.T) {
	delegate := NewRuntimeDelegate()
	if delegate.Bound() {
		t.Fatal("expected new runtime delegate to start unbound")
	}
	if err := delegate.BindService(nil); err == nil || !strings.Contains(err.Error(), "nil service") {
		t.Fatalf("expected nil bind error, got %v", err)
	}
	if err := delegate.DispatchHookEvent(context.Background(), pluginhost.ExtensionPointAuthLoginSucceeded, map[string]interface{}{}); err == nil ||
		!strings.Contains(err.Error(), "not bound") {
		t.Fatalf("expected unbound auth hook error, got %v", err)
	}
	if err := delegate.ProjectDynamicRoutesToOpenAPI(context.Background(), goai.Paths{}); err == nil ||
		!strings.Contains(err.Error(), "not bound") {
		t.Fatalf("expected unbound openapi projection error, got %v", err)
	}
	if err := delegate.EnsureTenantDeleteAllowed(context.Background(), 1001); err == nil ||
		!strings.Contains(err.Error(), "not bound") {
		t.Fatalf("expected unbound tenant delete guard error, got %v", err)
	}
}

// TestRuntimeDelegateReportsBoundAfterStartupWiring verifies test startup
// helpers connect the delegate to the completed plugin service.
func TestRuntimeDelegateReportsBoundAfterStartupWiring(t *testing.T) {
	delegate := NewRuntimeDelegate()
	service := newTestService()

	if err := delegate.BindService(service); err != nil {
		t.Fatalf("bind runtime delegate: %v", err)
	}
	if !delegate.Bound() {
		t.Fatal("expected runtime delegate to report bound")
	}
}

// TestIssuePageBridgeGrantHidesInternalFailures verifies both cache freshness
// and runtime validation failures keep their internal cause while exposing the
// same stable, translatable business error to HTTP callers.
func TestIssuePageBridgeGrantHidesInternalFailures(t *testing.T) {
	t.Run("cache freshness failure", func(t *testing.T) {
		cause := errors.New("cache coordination backend unavailable")
		service := &serviceImpl{
			runtimeCacheRevisionCtrl: revisionctrl.NewControllerWithCoordinator(
				true,
				&pageBridgeGrantFailingCacheCoord{err: cause},
				revisionctrl.NewObservedRevision(),
				nil,
			),
		}

		_, err := service.IssuePageBridgeGrant(context.Background(), PageBridgeGrantInput{})
		if !bizerr.Is(err, CodePluginPageBridgeGrantDenied) {
			t.Fatalf("expected stable page bridge grant error, got %v", err)
		}
		if !errors.Is(err, cause) {
			t.Fatalf("expected wrapped cache freshness cause, got %v", err)
		}
	})

	t.Run("runtime validation failure", func(t *testing.T) {
		service := newTestService()
		_, err := service.IssuePageBridgeGrant(context.Background(), PageBridgeGrantInput{})
		if !bizerr.Is(err, CodePluginPageBridgeGrantDenied) {
			t.Fatalf("expected stable page bridge grant error, got %v", err)
		}
		if err == nil || !strings.Contains(err.Error(), CodePluginPageBridgeGrantDenied.Fallback()) {
			t.Fatalf("expected stable fallback with preserved internal cause, got %v", err)
		}
	})
}

// TestInternalDelegateProvidersReportUnboundSideEffects verifies internal
// cache and dependency delegates fail fast when root wiring is missing.
func TestInternalDelegateProvidersReportUnboundSideEffects(t *testing.T) {
	ctx := context.Background()
	manifest := &catalog.Manifest{ID: "plugin-a"}

	integrationDelegate := &integrationDelegateProvider{}
	if err := integrationDelegate.SyncPluginMenusAndPermissions(ctx, manifest); err == nil ||
		!strings.Contains(err.Error(), "not bound") {
		t.Fatalf("expected unbound integration menu and permission sync error, got %v", err)
	}
	if err := integrationDelegate.SyncPluginMenus(ctx, manifest); err == nil ||
		!strings.Contains(err.Error(), "not bound") {
		t.Fatalf("expected unbound integration menu sync error, got %v", err)
	}
	if err := integrationDelegate.DeletePluginMenusByManifest(ctx, manifest); err == nil ||
		!strings.Contains(err.Error(), "not bound") {
		t.Fatalf("expected unbound integration menu deletion error, got %v", err)
	}
	if err := integrationDelegate.SyncPluginResourceReferences(ctx, manifest); err == nil ||
		!strings.Contains(err.Error(), "not bound") {
		t.Fatalf("expected unbound integration resource reference sync error, got %v", err)
	}
	if err := integrationDelegate.DispatchPluginHookEvent(
		ctx,
		pluginhost.ExtensionPointPluginInstalled,
		map[string]interface{}{"pluginId": "plugin-a"},
	); err == nil || !strings.Contains(err.Error(), "not bound") {
		t.Fatalf("expected unbound integration hook dispatch error, got %v", err)
	}
	if filtered := integrationDelegate.FilterPermissionMenus(ctx, nil); filtered != nil {
		t.Fatalf("expected unbound integration permission filter to preserve nil input, got %v", filtered)
	}
	if !integrationDelegate.CanExposeBusinessEntries(ctx, "plugin-a") {
		t.Fatal("expected unbound integration business entries check to fail open")
	}

	cacheNotifier := &runtimeCacheChangeNotifierProvider{}
	if err := cacheNotifier.MarkRuntimeCacheChanged(ctx, "unit-test"); err == nil ||
		!strings.Contains(err.Error(), "not bound") {
		t.Fatalf("expected unbound cache notifier error, got %v", err)
	}
	if err := cacheNotifier.PublishPluginChange(ctx, "plugin-a", "dynamic", "unit-test"); err == nil ||
		!strings.Contains(err.Error(), "not bound") {
		t.Fatalf("expected unbound plugin change error, got %v", err)
	}

	dependencyValidator := &dependencyValidatorProvider{}
	if err := dependencyValidator.ValidateDynamicPluginCandidate(ctx, manifest); err == nil ||
		!strings.Contains(err.Error(), "not bound") {
		t.Fatalf("expected unbound dynamic dependency validation error, got %v", err)
	}
	if err := dependencyValidator.ValidateSourcePluginUpgradeCandidate(ctx, manifest); err == nil ||
		!strings.Contains(err.Error(), "not bound") {
		t.Fatalf("expected unbound source dependency validation error, got %v", err)
	}
}

// TestUpgradeCacheAdaptersReportMissingService verifies upgrade adapters never
// treat missing root service wiring as a successful cache refresh.
func TestUpgradeCacheAdaptersReportMissingService(t *testing.T) {
	ctx := context.Background()

	publisher := upgradeCachePublisher{}
	if err := publisher.PublishPluginChange(ctx, "plugin-a", "dynamic", "unit-test"); err == nil ||
		!strings.Contains(err.Error(), "not configured") {
		t.Fatalf("expected missing publisher service error, got %v", err)
	}
	if err := publisher.SyncEnabledSnapshotAndPublishRuntimeChange(ctx, "plugin-a", "unit-test"); err == nil ||
		!strings.Contains(err.Error(), "not configured") {
		t.Fatalf("expected missing publisher sync error, got %v", err)
	}

	freshener := upgradeCacheFreshener{}
	if err := freshener.EnsureRuntimeCacheFresh(ctx); err == nil ||
		!strings.Contains(err.Error(), "not configured") {
		t.Fatalf("expected missing freshener service error, got %v", err)
	}
}
