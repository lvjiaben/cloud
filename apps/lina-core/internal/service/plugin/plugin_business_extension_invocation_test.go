// This file verifies bounded provider-call payloads and the lifecycle split
// between new-business interactions and existing-service operations.

package plugin

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"lina-core/internal/dao"
	"lina-core/internal/model/do"
	"lina-core/internal/service/plugin/internal/plugintypes"
	"lina-core/internal/service/plugin/internal/testutil"
	"lina-core/pkg/plugin/capability/capmodel"
	"lina-core/pkg/plugin/capability/plugincap"
	"lina-core/pkg/plugin/pluginhost"
	"lina-core/pkg/statusflag"
)

func TestBusinessExtensionInvocationLifecycleSeparatesNewAndExistingBusiness(t *testing.T) {
	service := newTestService()
	ctx := context.Background()
	pluginID := "business-invoke-lifecycle-test"
	ownerID := "business-invoke-owner-test"
	testutil.CleanupPluginGovernanceRowsHard(t, ctx, pluginID)
	t.Cleanup(func() { testutil.CleanupPluginGovernanceRowsHard(t, ctx, pluginID) })
	testutil.CreateTestRuntimeStorageArtifact(t, pluginID, "Business invoke lifecycle test", "v0.5.0", nil, nil)
	if _, err := service.Install(ctx, pluginID, InstallOptions{}); err != nil {
		t.Fatalf("install provider: %v", err)
	}
	if err := service.UpdateStatus(ctx, pluginID, UpdateStatusOptions{Status: statusflag.EnabledValue.Int()}); err != nil {
		t.Fatalf("enable provider: %v", err)
	}
	registerTestProductFulfillmentOwner(t, ownerID)
	requireActiveBusinessImpactOwner(t, ctx, service, pluginID, ownerID)
	setActiveBusinessExtensionMethods(t, ctx, service, pluginID, []string{"configuration.candidates.list"})
	registrar := service.ForBusinessExtensionOwner(ownerID)
	page, err := registrar.ListProviders(ctx, plugincap.BusinessExtensionProviderListInput{
		Point: capmodel.BusinessExtensionProductFulfillment, ContractVersion: "v1",
	})
	if err != nil || page == nil || page.Total != 1 {
		t.Fatalf("discover exact provider evidence: page=%#v err=%v", page, err)
	}
	evidence := page.Items[0]
	base := pluginhost.BusinessExtensionMethodInvokeInput{
		PluginID: evidence.PluginID, Point: evidence.Point, ContractVersion: evidence.ContractVersion,
		Method: "configuration.candidates.list", ReleaseVersion: evidence.ReleaseVersion,
		ReleaseDigest: evidence.ReleaseDigest, DeclarationDigest: evidence.DeclarationDigest,
		Payload: []byte(`{"field":"region"}`),
	}
	base.Purpose = capmodel.BusinessExtensionInvocationPurposeNewBusiness
	if _, err = service.recheckProviderInvocation(ctx, ownerID, base); err != nil {
		t.Fatalf("accepting provider should allow new-business invocation evidence: %v", err)
	}
	wrongMethod := base
	wrongMethod.Method = "configuration.unknown"
	if _, err = service.recheckProviderInvocation(ctx, ownerID, wrongMethod); err == nil || !strings.Contains(err.Error(), "not authorized") {
		t.Fatalf("undeclared provider method must fail closed, got %v", err)
	}
	wrongRelease := base
	wrongRelease.ReleaseDigest = strings.Repeat("0", 64)
	if _, err = service.recheckProviderInvocation(ctx, ownerID, wrongRelease); err == nil || !strings.Contains(err.Error(), "no longer active") {
		t.Fatalf("mismatched provider release evidence must fail closed, got %v", err)
	}
	if err = service.StopNewBusiness(ctx, pluginID); err != nil {
		t.Fatalf("stop new business: %v", err)
	}
	if _, err = service.recheckProviderInvocation(ctx, ownerID, base); err == nil || !strings.Contains(err.Error(), "not accepting") {
		t.Fatalf("stopped provider must reject new-business invocation, got %v", err)
	}
	base.Purpose = capmodel.BusinessExtensionInvocationPurposeExistingBusiness
	if _, err = service.recheckProviderInvocation(ctx, ownerID, base); err != nil {
		t.Fatalf("stopped provider must keep exact existing-business invocation available: %v", err)
	}
	if _, err = dao.SysPlugin.Ctx(ctx).Where(do.SysPlugin{PluginId: pluginID}).Data(do.SysPlugin{
		Status: statusflag.Disabled.Int(), CurrentState: plugintypes.HostStateInstalled.String(),
	}).Update(); err != nil {
		t.Fatalf("disable provider fixture: %v", err)
	}
	if _, err = service.recheckProviderInvocation(ctx, ownerID, base); err == nil || !strings.Contains(err.Error(), "not enabled") {
		t.Fatalf("disabled provider must reject existing-business invocation, got %v", err)
	}
}

func TestValidateBusinessExtensionJSONFailsClosed(t *testing.T) {
	tests := []struct {
		name    string
		payload []byte
	}{
		{name: "non object", payload: []byte(`[]`)},
		{name: "multiple values", payload: []byte(`{} {}`)},
		{name: "secret handle", payload: []byte(`{"secretRef":"hidden"}`)},
		{name: "too many items", payload: []byte(`{"items":[` + strings.Repeat(`0,`, businessExtensionJSONMaxArrayItems) + `0]}`)},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := validateBusinessExtensionJSON(test.payload, businessExtensionResponseMaxBytes); err == nil {
				t.Fatal("expected bounded JSON validation to reject payload")
			}
		})
	}
	if err := validateBusinessExtensionJSON([]byte(`{"items":[{"value":"cn"}]}`), businessExtensionResponseMaxBytes); err != nil {
		t.Fatalf("expected safe bounded object to pass: %v", err)
	}
}

func TestValidateBusinessExtensionRuntimeContext(t *testing.T) {
	valid := &capmodel.BusinessExtensionRuntimeContext{
		ContextID: "ctx_7gSQ8cKYbF2wVh9pR4mX6nJ3dT5zQa1E",
	}
	if err := validateBusinessExtensionRuntimeContext(valid); err != nil {
		t.Fatalf("expected safe opaque runtime context to pass: %v", err)
	}
	if err := validateBusinessExtensionRuntimeContext(nil); err != nil {
		t.Fatalf("expected omitted runtime context to remain supported: %v", err)
	}
	validInput := pluginhost.BusinessExtensionMethodInvokeInput{
		PluginID: "phone-provider", Point: capmodel.BusinessExtensionProductFulfillment,
		ContractVersion: "v1", Method: "configuration.candidates.list",
		ReleaseVersion: "v1.0.0", ReleaseDigest: strings.Repeat("a", 64),
		DeclarationDigest: strings.Repeat("b", 64), Purpose: capmodel.BusinessExtensionInvocationPurposeNewBusiness,
		RuntimeContext: valid, Payload: []byte(`{"field":"model"}`),
	}
	if err := validateBusinessExtensionInvokeInput(validInput); err != nil {
		t.Fatalf("expected complete invocation with opaque runtime context to pass: %v", err)
	}
	validInput.RuntimeContext = nil
	if err := validateBusinessExtensionInvokeInput(validInput); err != nil {
		t.Fatalf("expected an existing non-instance invocation to remain valid: %v", err)
	}

	tests := []struct {
		name      string
		contextID string
	}{
		{name: "short", contextID: "ctx_short"},
		{name: "unsafe character", contextID: "ctx_7gSQ8cKYbF2wVh9pR4mX6nJ3dT5zQa/E"},
		{name: "too long", contextID: strings.Repeat("a", businessExtensionContextIDMaxBytes+1)},
		{name: "instance material", contextID: "ctx_instance_7gSQ8cKYbF2wVh9pR4mX6nJ3dT5zQa1E"},
		{name: "revision material", contextID: "ctx_revision_7gSQ8cKYbF2wVh9pR4mX6nJ3dT5zQa1E"},
		{name: "configuration material", contextID: "ctx_config_7gSQ8cKYbF2wVh9pR4mX6nJ3dT5zQa1E"},
		{name: "secret material", contextID: "ctx_secret_7gSQ8cKYbF2wVh9pR4mX6nJ3dT5zQa1E"},
		{name: "token material", contextID: "ctx_token_7gSQ8cKYbF2wVh9pR4mX6nJ3dT5zQa1E"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := validateBusinessExtensionRuntimeContext(&capmodel.BusinessExtensionRuntimeContext{ContextID: test.contextID}); err == nil {
				t.Fatal("expected unsafe runtime context to fail closed")
			}
		})
	}
}

func TestBusinessExtensionSourceAndBridgeEnvelopesKeepRuntimeContextEquivalent(t *testing.T) {
	invocation := &capmodel.BusinessExtensionMethodInvocation{
		Version: capmodel.BusinessExtensionInvocationProtocolV1, OwnerPluginID: "idc-core",
		ProviderPluginID: "phone-provider", Point: capmodel.BusinessExtensionProductFulfillment,
		ContractVersion: "v1", Method: "configuration.candidates.list",
		ReleaseVersion: "v1.0.0", ReleaseDigest: strings.Repeat("a", 64),
		DeclarationDigest: strings.Repeat("b", 64), Purpose: capmodel.BusinessExtensionInvocationPurposeNewBusiness,
		RuntimeContext: &capmodel.BusinessExtensionRuntimeContext{ContextID: "ctx_7gSQ8cKYbF2wVh9pR4mX6nJ3dT5zQa1E"},
		Payload:        []byte(`{"field":"model"}`),
	}
	bridgeRequest := businessExtensionBridgeRequestFromInvocation(invocation)
	sourceJSON, err := json.Marshal(invocation)
	if err != nil {
		t.Fatalf("encode source invocation: %v", err)
	}
	bridgeJSON, err := json.Marshal(bridgeRequest)
	if err != nil {
		t.Fatalf("encode bridge invocation: %v", err)
	}
	if !bytes.Equal(sourceJSON, bridgeJSON) {
		t.Fatalf("source and WASM envelopes differ:\nsource=%s\nbridge=%s", sourceJSON, bridgeJSON)
	}

	invocation.RuntimeContext.ContextID = strings.Repeat("z", 40)
	if bridgeRequest.RuntimeContext.ContextID != "ctx_7gSQ8cKYbF2wVh9pR4mX6nJ3dT5zQa1E" {
		t.Fatal("bridge envelope must own a detached runtime-context snapshot")
	}
}

func TestValidateBusinessExtensionJSONSupportsGovernedCandidateBoundaries(t *testing.T) {
	items := make([]string, businessExtensionJSONMaxArrayItems)
	for index := range items {
		items[index] = `{"value":"candidate"}`
	}
	bounded := []byte(`{"items":[` + strings.Join(items, ",") + `]}`)
	if err := validateBusinessExtensionJSON(bounded, businessExtensionResponsePayloadMaxBytes); err != nil {
		t.Fatalf("expected 512 governed candidates to pass: %v", err)
	}
	tooMany := []byte(`{"items":[` + strings.Join(append(items, `{"value":"overflow"}`), ",") + `]}`)
	if err := validateBusinessExtensionJSON(tooMany, businessExtensionResponsePayloadMaxBytes); err == nil {
		t.Fatal("expected 513 governed candidates to fail closed")
	}

	chunks := make([]string, 120)
	for index := range chunks {
		chunks[index] = `"` + strings.Repeat("a", businessExtensionJSONMaxStringBytes) + `"`
	}
	nearLimit := []byte(`{"items":[` + strings.Join(chunks, ",") + `]}`)
	if err := validateBusinessExtensionJSON(nearLimit, businessExtensionResponsePayloadMaxBytes); err != nil {
		t.Fatalf("expected a safe response near 1 MiB to pass: %v", err)
	}
	chunks = append(chunks, chunks[:9]...)
	overLimit := []byte(`{"items":[` + strings.Join(chunks, ",") + `]}`)
	if len(overLimit) <= businessExtensionResponsePayloadMaxBytes {
		t.Fatalf("invalid test fixture: response size=%d", len(overLimit))
	}
	if err := validateBusinessExtensionJSON(overLimit, businessExtensionResponsePayloadMaxBytes); err == nil {
		t.Fatal("expected a response above 1 MiB to fail closed")
	}
}

func TestBusinessExtensionInvocationContextHasStrictTimeout(t *testing.T) {
	ctx, cancel := newBusinessExtensionInvocationContext(context.Background())
	defer cancel()
	deadline, ok := ctx.Deadline()
	if !ok {
		t.Fatal("expected provider invocation context to have a deadline")
	}
	remaining := time.Until(deadline)
	if remaining <= 0 || remaining > businessExtensionInvocationTimeout {
		t.Fatalf("expected bounded provider timeout, got %s", remaining)
	}
}

func TestDispatchSourceBusinessExtensionInvocationExecutesRegisteredHandler(t *testing.T) {
	const providerID = "source-business-invoke-test"
	declarations := pluginhost.NewDeclarations(providerID)
	declarations.Assets().UseEmbeddedFiles(fstest.MapFS{
		"plugin.yaml": &fstest.MapFile{Data: []byte(
			"id: " + providerID + "\n" +
				"name: Source business invoke test\n" +
				"version: v1.0.0\n" +
				"type: source\n" +
				"distribution: builtin\n" +
				"scope_nature: platform_only\n" +
				"supports_multi_tenant: false\n" +
				"default_install_mode: global\n",
		)},
	})
	if err := declarations.Providers().ProvideBusinessExtensionMethods(func(
		_ context.Context,
		invocation *capmodel.BusinessExtensionMethodInvocation,
	) (*capmodel.BusinessExtensionMethodResult, error) {
		if invocation.OwnerPluginID != "owner-test" || invocation.Method != "configuration.candidates.list" {
			t.Fatalf("unexpected governed source invocation: %#v", invocation)
		}
		if invocation.RuntimeContext == nil || invocation.RuntimeContext.ContextID != "ctx_7gSQ8cKYbF2wVh9pR4mX6nJ3dT5zQa1E" {
			t.Fatalf("expected opaque runtime context to reach source provider: %#v", invocation.RuntimeContext)
		}
		return &capmodel.BusinessExtensionMethodResult{
			Version: capmodel.BusinessExtensionInvocationProtocolV1,
			Payload: []byte(`{"items":[{"value":"cn"}]}`),
		}, nil
	}); err != nil {
		t.Fatalf("register source provider handler: %v", err)
	}
	cleanup, err := pluginhost.RegisterSourcePluginForTest(declarations)
	if err != nil {
		t.Fatalf("register source provider fixture: %v", err)
	}
	t.Cleanup(cleanup)
	service := newTestService()
	manifest, err := service.catalogSvc.GetDesiredManifest(providerID)
	if err != nil {
		t.Fatalf("load registered source provider manifest: %v", err)
	}
	result, err := service.dispatchSourceProviderInvocation(context.Background(), &capmodel.BusinessExtensionMethodInvocation{
		Version: capmodel.BusinessExtensionInvocationProtocolV1, OwnerPluginID: "owner-test",
		ProviderPluginID: providerID, Point: capmodel.BusinessExtensionProductFulfillment,
		ContractVersion: "v1", Method: "configuration.candidates.list", ReleaseVersion: manifest.Version,
		ReleaseDigest:     service.catalogSvc.BuildRegistryChecksum(manifest),
		DeclarationDigest: "declaration", Purpose: capmodel.BusinessExtensionInvocationPurposeNewBusiness,
		RuntimeContext: &capmodel.BusinessExtensionRuntimeContext{ContextID: "ctx_7gSQ8cKYbF2wVh9pR4mX6nJ3dT5zQa1E"},
		Payload:        []byte(`{"field":"region"}`),
	})
	if err != nil || result == nil || string(result.Payload) != `{"items":[{"value":"cn"}]}` {
		t.Fatalf("execute source provider invocation: result=%#v err=%v", result, err)
	}
}
