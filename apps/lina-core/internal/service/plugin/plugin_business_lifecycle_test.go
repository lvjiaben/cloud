package plugin

import (
	"context"
	"errors"
	"testing"
	"testing/fstest"
	"time"

	"gopkg.in/yaml.v3"

	"lina-core/internal/dao"
	"lina-core/internal/model/do"
	"lina-core/internal/service/plugin/internal/catalog"
	"lina-core/internal/service/plugin/internal/lifecycle"
	"lina-core/internal/service/plugin/internal/plugintypes"
	"lina-core/internal/service/plugin/internal/testutil"
	"lina-core/pkg/bizerr"
	"lina-core/pkg/plugin/capability/capmodel"
	"lina-core/pkg/plugin/capability/plugincap"
	"lina-core/pkg/plugin/pluginhost"
	"lina-core/pkg/statusflag"
)

func requireActiveBusinessImpactOwner(
	t *testing.T,
	ctx context.Context,
	service *serviceImpl,
	pluginID string,
	ownerPluginID string,
) {
	t.Helper()
	registry, err := service.storeSvc.GetRegistry(ctx, pluginID)
	if err != nil || registry == nil {
		t.Fatalf("load registry for %s: registry=%#v err=%v", pluginID, registry, err)
	}
	release, err := service.storeSvc.GetRegistryRelease(ctx, registry)
	if err != nil || release == nil {
		t.Fatalf("load active release for %s: release=%#v err=%v", pluginID, release, err)
	}
	snapshot, err := service.storeSvc.ParseManifestSnapshot(release.ManifestSnapshot)
	if err != nil || snapshot == nil {
		t.Fatalf("parse active release snapshot for %s: snapshot=%#v err=%v", pluginID, snapshot, err)
	}
	declaration := &capmodel.BusinessExtensionDeclaration{
		Point:   capmodel.BusinessExtensionProductFulfillment,
		Version: "v1",
		Role:    capmodel.BusinessExtensionRoleProvider,
		Owner:   ownerPluginID,
		Methods: []string{"provision"},
	}
	snapshot.Dependencies = &plugintypes.DependencySpec{Plugins: []*plugintypes.PluginDependencySpec{{
		ID: ownerPluginID, Version: ">=v0.5.0",
	}}}
	snapshot.BusinessExtensions = catalog.CloneBusinessExtensionDeclarations([]*capmodel.BusinessExtensionDeclaration{declaration})
	snapshot.AuthorizedBusinessExtensions = catalog.CloneBusinessExtensionDeclarations([]*capmodel.BusinessExtensionDeclaration{declaration})
	content, err := yaml.Marshal(snapshot)
	if err != nil {
		t.Fatalf("marshal active release snapshot for %s: %v", pluginID, err)
	}
	if _, err = dao.SysPluginRelease.Ctx(ctx).
		Where(do.SysPluginRelease{Id: release.Id}).
		Data(do.SysPluginRelease{ManifestSnapshot: string(content)}).
		Update(); err != nil {
		t.Fatalf("persist active release snapshot for %s: %v", pluginID, err)
	}
}

func setActiveBusinessExtensionMethods(t *testing.T, ctx context.Context, service *serviceImpl, pluginID string, methods []string) {
	t.Helper()
	registry, err := service.storeSvc.GetRegistry(ctx, pluginID)
	if err != nil || registry == nil {
		t.Fatalf("load registry for method update: registry=%#v err=%v", registry, err)
	}
	release, err := service.storeSvc.GetRegistryRelease(ctx, registry)
	if err != nil || release == nil {
		t.Fatalf("load release for method update: release=%#v err=%v", release, err)
	}
	snapshot, err := service.storeSvc.ParseManifestSnapshot(release.ManifestSnapshot)
	if err != nil || snapshot == nil || len(snapshot.AuthorizedBusinessExtensions) != 1 {
		t.Fatalf("load extension snapshot: snapshot=%#v err=%v", snapshot, err)
	}
	snapshot.AuthorizedBusinessExtensions[0].Methods = append([]string(nil), methods...)
	if len(snapshot.BusinessExtensions) == 1 {
		snapshot.BusinessExtensions[0].Methods = append([]string(nil), methods...)
	}
	content, err := yaml.Marshal(snapshot)
	if err != nil {
		t.Fatalf("marshal method snapshot: %v", err)
	}
	if _, err = dao.SysPluginRelease.Ctx(ctx).Where(do.SysPluginRelease{Id: release.Id}).Data(do.SysPluginRelease{ManifestSnapshot: string(content)}).Update(); err != nil {
		t.Fatalf("persist method snapshot: %v", err)
	}
}

func useTestBusinessImpactOwnerAssets(t *testing.T, owner pluginhost.Declarations, ownerPluginID string) {
	t.Helper()
	registerTestProductFulfillmentValidator(t, owner)
	owner.Assets().UseEmbeddedFiles(fstest.MapFS{
		"plugin.yaml": &fstest.MapFile{Data: []byte(
			"id: " + ownerPluginID + "\n" +
				"name: Test business impact owner\n" +
				"version: v0.5.0\n" +
				"type: source\n" +
				"distribution: builtin\n" +
				"scope_nature: platform_only\n" +
				"supports_multi_tenant: false\n" +
				"default_install_mode: global\n",
		)},
	})
	definition, ok := owner.(pluginhost.SourcePluginDefinition)
	if !ok || definition.GetEmbeddedFiles() == nil {
		t.Fatalf("bind embedded business impact owner assets: %s", ownerPluginID)
	}
}

func registerTestProductFulfillmentValidator(t *testing.T, owner pluginhost.Declarations) {
	t.Helper()
	if err := owner.BusinessExtensions().RegisterProviderValidator(
		capmodel.BusinessExtensionProductFulfillment,
		"v1",
		func([]*capmodel.BusinessExtensionDeclaration) error { return nil },
	); err != nil {
		t.Fatalf("register test product fulfillment validator: %v", err)
	}
}

func registerTestProductFulfillmentOwner(t *testing.T, ownerPluginID string) {
	t.Helper()
	owner := pluginhost.NewDeclarations(ownerPluginID)
	registerTestProductFulfillmentValidator(t, owner)
	cleanup, err := pluginhost.RegisterSourcePluginForTest(owner)
	if err != nil {
		t.Fatalf("register test product fulfillment owner: %v", err)
	}
	t.Cleanup(cleanup)
}

func TestBusinessImpactProviderFailuresFailClosed(t *testing.T) {
	cases := []struct {
		name    string
		suffix  string
		handler pluginhost.SourcePluginBusinessImpactHandler
	}{
		{
			name:   "provider error",
			suffix: "provider-error",
			handler: func(context.Context, pluginhost.PluginBusinessImpactInput) ([]pluginhost.PluginBusinessImpactItem, error) {
				return nil, errors.New("database unavailable")
			},
		},
		{
			name:   "invalid duplicate category",
			suffix: "duplicate-category",
			handler: func(context.Context, pluginhost.PluginBusinessImpactInput) ([]pluginhost.PluginBusinessImpactItem, error) {
				return []pluginhost.PluginBusinessImpactItem{
					{Category: pluginhost.BusinessImpactInFlightTasks, TotalCount: 1, BlockingCount: 0},
					{Category: pluginhost.BusinessImpactInFlightTasks, TotalCount: 1, BlockingCount: 1},
				}, nil
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			service := newTestService()
			ctx := context.Background()
			pluginID := "business-impact-fail-closed-" + tc.suffix
			testutil.CleanupPluginGovernanceRowsHard(t, ctx, pluginID)
			t.Cleanup(func() { testutil.CleanupPluginGovernanceRowsHard(t, ctx, pluginID) })
			testutil.CreateTestRuntimeStorageArtifact(t, pluginID, "Business impact fail closed", "v0.5.0", nil, nil)
			if _, err := service.Install(ctx, pluginID, InstallOptions{}); err != nil {
				t.Fatalf("install target plugin: %v", err)
			}

			owner := pluginhost.NewDeclarations("business-impact-fail-owner-" + tc.suffix)
			useTestBusinessImpactOwnerAssets(t, owner, owner.ID())
			if err := owner.Lifecycle().RegisterBusinessImpactHandler(tc.handler); err != nil {
				t.Fatalf("register impact handler: %v", err)
			}
			cleanup, err := pluginhost.RegisterSourcePluginForTest(owner)
			if err != nil {
				t.Fatalf("register impact owner: %v", err)
			}
			t.Cleanup(cleanup)
			requireActiveBusinessImpactOwner(t, ctx, service, pluginID, owner.ID())

			_, err = service.lifecycleSvc.CheckBusinessImpact(ctx, pluginID)
			if !bizerr.Is(err, lifecycle.CodePluginBusinessImpactCheckFailed) {
				t.Fatalf("expected fail-closed impact error, got %v", err)
			}
		})
	}
}

func TestStopNewBusinessKeepsExistingRuntimeEnabled(t *testing.T) {
	service := newTestService()
	ctx := context.Background()
	pluginID := "business-stop-runtime-test"
	testutil.CleanupPluginGovernanceRowsHard(t, ctx, pluginID)
	t.Cleanup(func() { testutil.CleanupPluginGovernanceRowsHard(t, ctx, pluginID) })
	testutil.CreateTestRuntimeStorageArtifact(t, pluginID, "Business stop runtime test", "v0.5.0", nil, nil)

	if _, err := service.Install(ctx, pluginID, InstallOptions{}); err != nil {
		t.Fatalf("install target plugin: %v", err)
	}
	if err := service.UpdateStatus(ctx, pluginID, UpdateStatusOptions{Status: statusflag.EnabledValue.Int()}); err != nil {
		t.Fatalf("enable target plugin: %v", err)
	}
	if err := service.StopNewBusiness(ctx, pluginID); err != nil {
		t.Fatalf("stop new business: %v", err)
	}
	if !service.IsEnabled(ctx, pluginID) {
		t.Fatal("stopping new business must not disable existing-service runtime entries")
	}
	accepting := service.CanAcceptNewBusiness(ctx, pluginID)
	if accepting {
		t.Fatal("stopped plugin must not accept new business")
	}
	stoppedRegistry, err := service.storeSvc.GetRegistry(ctx, pluginID)
	if err != nil || stoppedRegistry == nil || stoppedRegistry.NewBusinessStoppedAt == nil {
		t.Fatalf("expected persisted stop-new-business evidence, registry=%#v err=%v", stoppedRegistry, err)
	}
	stoppedAt := *stoppedRegistry.NewBusinessStoppedAt

	if err := service.ResumeNewBusiness(ctx, pluginID); err != nil {
		t.Fatalf("resume new business: %v", err)
	}
	accepting = service.CanAcceptNewBusiness(ctx, pluginID)
	if !accepting {
		t.Fatalf("expected ready plugin to accept new business after resume, accepting=%v", accepting)
	}
	resumedRegistry, err := service.storeSvc.GetRegistry(ctx, pluginID)
	if err != nil || resumedRegistry == nil || resumedRegistry.NewBusinessStoppedAt == nil ||
		!resumedRegistry.NewBusinessStoppedAt.Equal(stoppedAt) {
		t.Fatalf("expected resume to preserve the most recent stop evidence, registry=%#v err=%v", resumedRegistry, err)
	}
}

func TestBusinessExtensionAdmissionSerializesStopNewBusiness(t *testing.T) {
	service := newTestService()
	ctx := context.Background()
	pluginID := "business-admission-lock-test"
	ownerID := "business-admission-owner-test"
	testutil.CleanupPluginGovernanceRowsHard(t, ctx, pluginID)
	t.Cleanup(func() { testutil.CleanupPluginGovernanceRowsHard(t, ctx, pluginID) })
	testutil.CreateTestRuntimeStorageArtifact(t, pluginID, "Business admission lock test", "v0.5.0", nil, nil)
	if _, err := service.Install(ctx, pluginID, InstallOptions{}); err != nil {
		t.Fatalf("install provider: %v", err)
	}
	if err := service.UpdateStatus(ctx, pluginID, UpdateStatusOptions{Status: statusflag.EnabledValue.Int()}); err != nil {
		t.Fatalf("enable provider: %v", err)
	}
	registerTestProductFulfillmentOwner(t, ownerID)
	requireActiveBusinessImpactOwner(t, ctx, service, pluginID, ownerID)
	setActiveBusinessExtensionMethods(t, ctx, service, pluginID, nil)

	registrar := service.ForBusinessExtensionOwner(ownerID)
	page, err := registrar.ListProviders(ctx, plugincap.BusinessExtensionProviderListInput{
		Point: capmodel.BusinessExtensionProductFulfillment, ContractVersion: "v1",
	})
	if err != nil || page == nil || page.Total != 1 {
		t.Fatalf("discover provider: page=%#v err=%v", page, err)
	}
	discovered := page.Items[0]
	entered := make(chan struct{})
	release := make(chan struct{})
	guardDone := make(chan error, 1)
	go func() {
		guardDone <- registrar.WithNewBusinessAdmission(ctx, pluginhost.BusinessExtensionAdmissionInput{
			PluginID: discovered.PluginID, Point: discovered.Point, ContractVersion: discovered.ContractVersion,
			Method: "", ReleaseVersion: discovered.ReleaseVersion,
			ReleaseDigest: discovered.ReleaseDigest, DeclarationDigest: discovered.DeclarationDigest,
		}, func(_ context.Context, frozen *capmodel.BusinessExtensionProviderEvidence) error {
			if frozen.DeclarationDigest != discovered.DeclarationDigest {
				return errors.New("frozen evidence changed")
			}
			close(entered)
			<-release
			return nil
		})
	}()
	<-entered
	if err = service.StopNewBusiness(ctx, pluginID); !bizerr.Is(err, CodePluginLifecycleAlreadyRunning) {
		close(release)
		t.Fatalf("expected stop-new to share admission lock, got %v", err)
	}
	close(release)
	if err = <-guardDone; err != nil {
		t.Fatalf("complete guarded owner transaction: %v", err)
	}
	if err = service.StopNewBusiness(ctx, pluginID); err != nil {
		t.Fatalf("stop new business after callback: %v", err)
	}
	exact, err := registrar.GetProviderExact(ctx, plugincap.BusinessExtensionProviderExactInput{
		PluginID: discovered.PluginID, ReleaseVersion: discovered.ReleaseVersion, Point: discovered.Point,
		ContractVersion: discovered.ContractVersion, ReleaseDigest: discovered.ReleaseDigest,
		DeclarationDigest: discovered.DeclarationDigest,
	})
	if err != nil || exact == nil {
		t.Fatalf("stopped provider historical evidence must remain visible: evidence=%#v err=%v", exact, err)
	}
	if _, err = dao.SysPlugin.Ctx(ctx).Where(do.SysPlugin{PluginId: pluginID}).Data(do.SysPlugin{
		Installed:    statusflag.Uninstalled.Int(),
		Status:       statusflag.Disabled.Int(),
		DesiredState: plugintypes.HostStateUninstalled.String(),
		CurrentState: plugintypes.HostStateUninstalled.String(),
		ReleaseId:    0,
	}).Update(); err != nil {
		t.Fatalf("mark provider uninstalled while retaining release evidence: %v", err)
	}
	if _, err = dao.SysPluginRelease.Ctx(ctx).Where(do.SysPluginRelease{
		PluginId: pluginID, ReleaseVersion: discovered.ReleaseVersion,
	}).Data(do.SysPluginRelease{Status: plugintypes.ReleaseStatusUninstalled.String()}).Update(); err != nil {
		t.Fatalf("mark retained provider release uninstalled: %v", err)
	}
	exact, err = registrar.GetProviderExact(ctx, plugincap.BusinessExtensionProviderExactInput{
		PluginID: discovered.PluginID, ReleaseVersion: discovered.ReleaseVersion, Point: discovered.Point,
		ContractVersion: discovered.ContractVersion, ReleaseDigest: discovered.ReleaseDigest,
		DeclarationDigest: discovered.DeclarationDigest,
	})
	if err != nil || exact == nil {
		t.Fatalf("uninstalled provider retained historical evidence must remain visible: evidence=%#v err=%v", exact, err)
	}
}

func TestBusinessImpactCannotBeBypassedByForceUninstall(t *testing.T) {
	service := newTestService()
	ctx := context.Background()
	pluginID := "business-impact-force-test"
	testutil.CleanupPluginGovernanceRowsHard(t, ctx, pluginID)
	t.Cleanup(func() { testutil.CleanupPluginGovernanceRowsHard(t, ctx, pluginID) })
	testutil.CreateTestRuntimeStorageArtifact(t, pluginID, "Business impact force test", "v0.5.0", nil, nil)
	if _, err := service.Install(ctx, pluginID, InstallOptions{}); err != nil {
		t.Fatalf("install target plugin: %v", err)
	}
	if err := service.StopNewBusiness(ctx, pluginID); err != nil {
		t.Fatalf("stop target new business: %v", err)
	}

	owner := pluginhost.NewDeclarations("business-impact-owner-force-test")
	useTestBusinessImpactOwnerAssets(t, owner, owner.ID())
	if err := owner.Lifecycle().RegisterBusinessImpactHandler(func(
		_ context.Context,
		input pluginhost.PluginBusinessImpactInput,
	) ([]pluginhost.PluginBusinessImpactItem, error) {
		if input.TargetPluginID != pluginID {
			return nil, nil
		}
		return []pluginhost.PluginBusinessImpactItem{{
			Category: pluginhost.BusinessImpactActiveServices, TotalCount: 1, BlockingCount: 1,
		}}, nil
	}); err != nil {
		t.Fatalf("register impact handler: %v", err)
	}
	cleanupOwner, err := pluginhost.RegisterSourcePluginForTest(owner)
	if err != nil {
		t.Fatalf("register impact owner: %v", err)
	}
	t.Cleanup(cleanupOwner)
	requireActiveBusinessImpactOwner(t, ctx, service, pluginID, owner.ID())

	err = service.lifecycleSvc.Uninstall(ctx, pluginID, lifecycle.UninstallOptions{
		Force: true, PurgeStorageData: false, AllowForceUninstall: true,
	})
	if !bizerr.Is(err, lifecycle.CodePluginBusinessReferencesBlocked) {
		t.Fatalf("expected non-bypassable business impact blocker, got %v", err)
	}
	if !service.IsInstalled(ctx, pluginID) {
		t.Fatal("blocked force uninstall must leave the plugin installed")
	}
}

func TestMissingRequiredBusinessImpactOwnerFailsClosed(t *testing.T) {
	service := newTestService()
	ctx := context.Background()
	pluginID := "business-impact-missing-owner-test"
	ownerPluginID := "business-impact-owner-not-registered"
	testutil.CleanupPluginGovernanceRowsHard(t, ctx, pluginID)
	t.Cleanup(func() { testutil.CleanupPluginGovernanceRowsHard(t, ctx, pluginID) })
	testutil.CreateTestRuntimeStorageArtifact(t, pluginID, "Missing impact owner", "v0.5.0", nil, nil)
	if _, err := service.Install(ctx, pluginID, InstallOptions{}); err != nil {
		t.Fatalf("install target plugin: %v", err)
	}
	requireActiveBusinessImpactOwner(t, ctx, service, pluginID, ownerPluginID)

	_, err := service.CheckBusinessImpact(ctx, pluginID)
	if !bizerr.Is(err, lifecycle.CodePluginBusinessImpactCheckFailed) {
		t.Fatalf("expected missing required owner to fail closed, got %v", err)
	}
}

func TestPurgeDecisionUsesActiveReleaseSnapshot(t *testing.T) {
	service := newTestService()
	ctx := context.Background()
	pluginID := "business-purge-active-snapshot-test"
	testutil.CleanupPluginGovernanceRowsHard(t, ctx, pluginID)
	t.Cleanup(func() { testutil.CleanupPluginGovernanceRowsHard(t, ctx, pluginID) })
	testutil.CreateTestRuntimeStorageArtifact(t, pluginID, "Active snapshot purge guard", "v0.5.0", nil, nil)
	if _, err := service.Install(ctx, pluginID, InstallOptions{}); err != nil {
		t.Fatalf("install target plugin: %v", err)
	}
	owner := pluginhost.NewDeclarations("active-snapshot-owner")
	useTestBusinessImpactOwnerAssets(t, owner, owner.ID())
	if err := owner.Lifecycle().RegisterBusinessImpactHandler(func(
		context.Context,
		pluginhost.PluginBusinessImpactInput,
	) ([]pluginhost.PluginBusinessImpactItem, error) {
		return nil, nil
	}); err != nil {
		t.Fatalf("register active snapshot impact owner: %v", err)
	}
	cleanupOwner, err := pluginhost.RegisterSourcePluginForTest(owner)
	if err != nil {
		t.Fatalf("publish active snapshot impact owner: %v", err)
	}
	t.Cleanup(cleanupOwner)
	requireActiveBusinessImpactOwner(t, ctx, service, pluginID, owner.ID())
	if err := service.StopNewBusiness(ctx, pluginID); err != nil {
		t.Fatalf("stop target new business: %v", err)
	}

	err = service.Uninstall(ctx, pluginID, UninstallOptions{PurgeStorageData: true})
	if !bizerr.Is(err, lifecycle.CodePluginBusinessDataPurgeDenied) {
		t.Fatalf("expected active release business snapshot to deny purge, got %v", err)
	}
	if !service.IsInstalled(ctx, pluginID) {
		t.Fatal("purge denial must leave the plugin installed")
	}
}

func TestLifecycleLockPreventsAdmissionRaceWithDrainCommit(t *testing.T) {
	cases := []struct {
		name      string
		suffix    string
		operation func(context.Context, *serviceImpl, string) error
	}{
		{
			name:   "disable",
			suffix: "disable",
			operation: func(ctx context.Context, service *serviceImpl, pluginID string) error {
				return service.UpdateStatus(ctx, pluginID, UpdateStatusOptions{Status: 0})
			},
		},
		{
			name:   "uninstall",
			suffix: "uninstall",
			operation: func(ctx context.Context, service *serviceImpl, pluginID string) error {
				return service.Uninstall(ctx, pluginID, UninstallOptions{})
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			service := newTestService()
			ctx := context.Background()
			pluginID := "business-lifecycle-lock-" + tc.suffix
			ownerPluginID := "business-lifecycle-lock-owner-" + tc.suffix
			testutil.CleanupPluginGovernanceRowsHard(t, ctx, pluginID)
			t.Cleanup(func() { testutil.CleanupPluginGovernanceRowsHard(t, ctx, pluginID) })
			testutil.CreateTestRuntimeStorageArtifact(t, pluginID, "Lifecycle lock target", "v0.5.0", nil, nil)
			if _, err := service.Install(ctx, pluginID, InstallOptions{}); err != nil {
				t.Fatalf("install target plugin: %v", err)
			}
			if err := service.UpdateStatus(ctx, pluginID, UpdateStatusOptions{Status: statusflag.EnabledValue.Int()}); err != nil {
				t.Fatalf("enable target plugin: %v", err)
			}
			if err := service.StopNewBusiness(ctx, pluginID); err != nil {
				t.Fatalf("stop target new business: %v", err)
			}

			entered := make(chan struct{})
			release := make(chan struct{})
			owner := pluginhost.NewDeclarations(ownerPluginID)
			useTestBusinessImpactOwnerAssets(t, owner, ownerPluginID)
			if err := owner.Lifecycle().RegisterBusinessImpactHandler(func(
				context.Context,
				pluginhost.PluginBusinessImpactInput,
			) ([]pluginhost.PluginBusinessImpactItem, error) {
				close(entered)
				<-release
				return nil, nil
			}); err != nil {
				t.Fatalf("register blocking impact handler: %v", err)
			}
			cleanup, err := pluginhost.RegisterSourcePluginForTest(owner)
			if err != nil {
				t.Fatalf("register impact owner: %v", err)
			}
			t.Cleanup(cleanup)
			requireActiveBusinessImpactOwner(t, ctx, service, pluginID, ownerPluginID)

			operationResult := make(chan error, 1)
			go func() { operationResult <- tc.operation(ctx, service, pluginID) }()
			select {
			case <-entered:
			case err := <-operationResult:
				t.Fatalf("drain operation returned before impact handler: %v", err)
			case <-time.After(5 * time.Second):
				close(release)
				t.Fatal("timed out waiting for drain check")
			}
			resumeErr := service.ResumeNewBusiness(ctx, pluginID)
			if !bizerr.Is(resumeErr, CodePluginLifecycleAlreadyRunning) {
				t.Fatalf("expected concurrent resume to be rejected by lifecycle lock, got %v", resumeErr)
			}
			close(release)
			if err := <-operationResult; err != nil {
				t.Fatalf("drain operation failed after lock test: %v", err)
			}
		})
	}
}
