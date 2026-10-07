package composition

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"lina-core/pkg/plugin/capability/capmodel"
	"lina-core/pkg/plugin/capability/plugincap"
	"lina-core/pkg/plugin/pluginhost"
	"lina-plugin-idc-core/backend/cap/secretcap"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
	"lina-plugin-idc-resource/backend/cap/resourcecap/spi"
	"lina-plugin-idc-resource/backend/internal/service/channelconfig"
)

type dynamicRegistrarFake struct {
	registration pluginhost.ConfidentialOutboundContextInput
	invocation   pluginhost.BusinessExtensionMethodInvokeInput
}

func (*dynamicRegistrarFake) ListProviders(context.Context, plugincap.BusinessExtensionProviderListInput) (*capmodel.PageResult[*capmodel.BusinessExtensionProviderEvidence], error) {
	return nil, errors.New("unused")
}
func (*dynamicRegistrarFake) GetProviderExact(context.Context, plugincap.BusinessExtensionProviderExactInput) (*capmodel.BusinessExtensionProviderEvidence, error) {
	return nil, errors.New("unused")
}
func (*dynamicRegistrarFake) GetProvidersExact(context.Context, []plugincap.BusinessExtensionProviderExactInput) ([]*capmodel.BusinessExtensionProviderEvidence, error) {
	return nil, errors.New("unused")
}
func (*dynamicRegistrarFake) WithNewBusinessAdmission(context.Context, pluginhost.BusinessExtensionAdmissionInput, capmodel.BusinessExtensionAdmissionAction) error {
	return errors.New("unused")
}
func (f *dynamicRegistrarFake) WithConfidentialOutboundContext(
	ctx context.Context,
	input pluginhost.ConfidentialOutboundContextInput,
	action pluginhost.ConfidentialOutboundAction,
) error {
	f.registration = input
	return action(ctx, &capmodel.BusinessExtensionRuntimeContext{ContextID: "opaque_context_1"})
}
func (f *dynamicRegistrarFake) InvokeProviderMethod(
	ctx context.Context,
	input pluginhost.BusinessExtensionMethodInvokeInput,
) (*capmodel.BusinessExtensionMethodResult, error) {
	f.invocation = input
	result, err := f.registration.Handler(ctx, &pluginhost.ConfidentialOutboundRequest{Payload: input.Payload})
	if err != nil {
		return nil, err
	}
	return &capmodel.BusinessExtensionMethodResult{Payload: result.Payload}, nil
}

type runtimeChannelReader struct {
	configuration channelconfig.Configuration
	err           error
	calls         int
}

func (r *runtimeChannelReader) ReadEnabled(
	_ context.Context,
	_ string,
) (channelconfig.Configuration, error) {
	r.calls++
	return r.configuration, r.err
}

type runtimeProvider struct{}

func (runtimeProvider) DiscoverCapabilities(context.Context) (*resourcecap.CapabilitySnapshot, error) {
	return nil, nil
}
func (runtimeProvider) ListProviderCapacity(context.Context, resourcecap.ProviderCapacityRequest) (*resourcecap.ProviderCapacityPage, error) {
	return nil, nil
}
func (runtimeProvider) Execute(context.Context, resourcecap.OperationRequest) (*resourcecap.OperationResult, error) {
	return nil, nil
}
func (runtimeProvider) QueryTask(context.Context, resourcecap.TaskQuery) (*resourcecap.OperationResult, error) {
	return nil, nil
}
func (runtimeProvider) QueryResource(context.Context, resourcecap.ResourceQuery) (*resourcecap.ResourceSnapshot, error) {
	return nil, nil
}

type runtimeHTTPClient struct{}

func (runtimeHTTPClient) Do(*http.Request) (*http.Response, error) { return nil, errors.New("unused") }

type runtimeClock struct{}

func (runtimeClock) Now() time.Time { return time.Unix(1, 0).UTC() }

type runtimeLogger struct{}

func (runtimeLogger) Log(context.Context, resourcecap.LogLevel, string, ...resourcecap.LogField) {}

type runtimeMetrics struct{}

func (runtimeMetrics) AddCounter(context.Context, string, int64, ...resourcecap.MetricLabel) {}
func (runtimeMetrics) Observe(context.Context, string, float64, ...resourcecap.MetricLabel)  {}

type runtimeAuthenticationSessions struct{}

func (runtimeAuthenticationSessions) Access(
	context.Context,
	resourcecap.AuthenticationSessionRequest,
	resourcecap.AuthenticationTokenProducer,
	resourcecap.AuthenticationTokenConsumer,
) error {
	return nil
}

func (runtimeAuthenticationSessions) Invalidate(
	context.Context,
	resourcecap.AuthenticationSessionInvalidation,
) error {
	return nil
}

func TestProviderRuntimeConstructsFromExactFrozenDeclaration(t *testing.T) {
	reader := &runtimeChannelReader{configuration: channelconfig.Configuration{
		PluginID:   "provider-plugin",
		ProviderID: "provider-one",
		Config: resourcecap.ProviderConfig{
			ChannelRef: "channel_a1", Endpoint: "https://provider.invalid", Settings: []byte(`{}`),
		},
	}}
	source := &captureSource{value: inertAccessor{}, manager: inertManager{}}
	factoryCalls := 0
	runtime, err := NewProviderRuntime([]spi.Declaration{{
		Key: resourcecap.ProviderKey{
			PluginID: "provider-plugin", ProviderID: "provider-one", ContractVersion: resourcecap.ContractVersion,
		},
		Factory: func(
			_ context.Context,
			environment resourcecap.ProviderEnv,
			configuration resourcecap.ProviderConfig,
		) (resourcecap.Provider, error) {
			factoryCalls++
			if environment.PluginID != "provider-plugin" || environment.ProviderID != "provider-one" ||
				environment.Secrets == nil || environment.SecretSink == nil ||
				configuration.ChannelRef != "channel_a1" {
				t.Fatal("factory received incomplete scoped construction inputs")
			}
			return runtimeProvider{}, nil
		},
	}}, source, reader, func(
		_ context.Context,
		configuration channelconfig.Configuration,
	) (resourcecap.ProviderEnv, func(), error) {
		if configuration.Config.ChannelRef != "channel_a1" {
			t.Fatal("environment factory received wrong channel")
		}
		return resourcecap.ProviderEnv{
			HTTPClient: runtimeHTTPClient{}, Clock: runtimeClock{},
			Logger: runtimeLogger{}, Metrics: runtimeMetrics{},
			AuthenticationSessions: runtimeAuthenticationSessions{},
		}, func() {}, nil
	})
	if err != nil {
		t.Fatalf("construct Provider runtime failed: %v", err)
	}
	for attempt := 0; attempt < 2; attempt++ {
		provider, constructErr := runtime.Construct(context.Background(), "channel_a1")
		if constructErr != nil || provider == nil {
			t.Fatalf("construct enabled Provider attempt %d failed: %v", attempt+1, constructErr)
		}
	}
	if reader.calls != 2 || factoryCalls != 2 || len(source.accessorScopes) != 2 || len(source.managerScopes) != 2 {
		t.Fatalf(
			"runtime cached mutable construction state: reads=%d factories=%d accessors=%d managers=%d",
			reader.calls,
			factoryCalls,
			len(source.accessorScopes),
			len(source.managerScopes),
		)
	}
}

func TestProviderRuntimeFailsClosedForMissingOrDuplicateFactory(t *testing.T) {
	source := &captureSource{value: inertAccessor{}, manager: inertManager{}}
	reader := &runtimeChannelReader{configuration: channelconfig.Configuration{
		PluginID: "provider-plugin", ProviderID: "provider-missing",
		Config: resourcecap.ProviderConfig{
			ChannelRef: "channel_a1", Endpoint: "https://provider.invalid", Settings: []byte(`{}`),
		},
	}}
	environments := func(context.Context, channelconfig.Configuration) (resourcecap.ProviderEnv, func(), error) {
		return resourcecap.ProviderEnv{}, func() {}, nil
	}
	runtime, err := NewProviderRuntime(nil, source, reader, environments)
	if err != nil {
		t.Fatalf("empty runtime declarations should remain valid: %v", err)
	}
	if _, err = runtime.Construct(context.Background(), "channel_a1"); err == nil {
		t.Fatal("missing Provider factory was accepted")
	}
	declaration := spi.Declaration{
		Key: resourcecap.ProviderKey{
			PluginID: "provider-plugin", ProviderID: "provider-one", ContractVersion: resourcecap.ContractVersion,
		},
		Factory: func(context.Context, resourcecap.ProviderEnv, resourcecap.ProviderConfig) (resourcecap.Provider, error) {
			return runtimeProvider{}, nil
		},
	}
	if _, err = NewProviderRuntime([]spi.Declaration{declaration, declaration}, source, reader, environments); err == nil {
		t.Fatal("duplicate Provider factory was accepted")
	}
}

// TestProviderRuntimeDoesNotConstructForDisabledChannel proves the channel
// owner gate runs before the immutable factory is selected. A disabled channel
// therefore preserves its declaration and history without creating a client.
func TestProviderRuntimeDoesNotConstructForDisabledChannel(t *testing.T) {
	disabled := errors.New("channel is disabled")
	reader := &runtimeChannelReader{err: disabled}
	source := &captureSource{value: inertAccessor{}, manager: inertManager{}}
	factoryCalls := 0
	runtime, err := NewProviderRuntime([]spi.Declaration{{
		Key: resourcecap.ProviderKey{
			PluginID: "provider-plugin", ProviderID: "provider-one", ContractVersion: resourcecap.ContractVersion,
		},
		Factory: func(context.Context, resourcecap.ProviderEnv, resourcecap.ProviderConfig) (resourcecap.Provider, error) {
			factoryCalls++
			return runtimeProvider{}, nil
		},
	}}, source, reader, func(context.Context, channelconfig.Configuration) (resourcecap.ProviderEnv, func(), error) {
		t.Fatal("disabled channel reached environment construction")
		return resourcecap.ProviderEnv{}, nil, nil
	})
	if err != nil {
		t.Fatalf("create runtime: %v", err)
	}
	if _, err = runtime.Construct(context.Background(), "channel_disabled"); err == nil {
		t.Fatal("disabled channel constructed a Provider client")
	}
	if reader.calls != 1 || factoryCalls != 0 || len(source.accessorScopes) != 0 || len(source.managerScopes) != 0 {
		t.Fatalf(
			"disabled channel crossed construction boundary: reads=%d factories=%d accessors=%d managers=%d",
			reader.calls, factoryCalls, len(source.accessorScopes), len(source.managerScopes),
		)
	}
}

func TestProviderRuntimeInvokesFrozenDynamicProviderWithoutSerializingSecrets(t *testing.T) {
	handle, err := secretcap.NewHandle("sec_dynamic_runtime_test")
	if err != nil {
		t.Fatalf("create opaque handle: %v", err)
	}
	reader := &runtimeChannelReader{configuration: channelconfig.Configuration{
		PluginID: "idc-zjmfcloud", ProviderID: "zjmfcloud-v1", RuntimeType: "dynamic",
		ContractVersion: resourcecap.DynamicProviderContractVersion,
		ReleaseVersion:  "v0.2.0", ReleaseDigest: strings.Repeat("a", 64),
		DeclarationDigest: strings.Repeat("b", 64),
		Config: resourcecap.ProviderConfig{
			ChannelRef: "channel_dynamic_1", Endpoint: "https://provider.invalid/v1",
			Settings: []byte(`{}`), SecretBindings: []resourcecap.SecretBinding{{Name: "password", Handle: handle}},
		},
	}}
	registrar := &dynamicRegistrarFake{}
	var outboundPayload []byte
	runtime, err := NewProviderRuntime(nil,
		&captureSource{value: inertAccessor{}, manager: inertManager{}}, reader,
		func(context.Context, channelconfig.Configuration) (resourcecap.ProviderEnv, func(), error) {
			t.Fatal("dynamic Provider must not construct a source environment")
			return resourcecap.ProviderEnv{}, nil, nil
		},
		WithDynamicBusinessExtensions(registrar, func(
			_ context.Context,
			configuration channelconfig.Configuration,
			method string,
			_ []resourcecap.SecretBinding,
		) (pluginhost.ConfidentialOutboundHandler, func(), error) {
			if configuration.ReleaseVersion != "v0.2.0" || method != resourcecap.DynamicProviderMethodDiscoverCapabilities {
				t.Fatal("dynamic outbound did not receive exact frozen evidence")
			}
			return func(_ context.Context, request *pluginhost.ConfidentialOutboundRequest) (*pluginhost.ConfidentialOutboundResult, error) {
				outboundPayload = append([]byte(nil), request.Payload...)
				capabilities := make([]resourcecap.CapabilityDescriptor, 0, resourcecap.MaximumCapabilityCount)
				for _, code := range resourcecap.SupportedCapabilityCodes() {
					capabilities = append(capabilities, resourcecap.CapabilityDescriptor{Code: code, Support: resourcecap.SupportLevelSupported})
				}
				payload, marshalErr := json.Marshal(resourcecap.CapabilitySnapshot{
					AdapterVersion: "dynamic-v1", Capabilities: capabilities, ObservedAt: time.Now().UTC(),
				})
				return &pluginhost.ConfidentialOutboundResult{Payload: payload}, marshalErr
			}, func() {}, nil
		}),
	)
	if err != nil {
		t.Fatalf("construct dynamic runtime: %v", err)
	}
	provider, cleanup, err := runtime.ConstructSession(context.Background(), "channel_dynamic_1")
	if err != nil {
		t.Fatalf("construct dynamic Provider: %v", err)
	}
	defer cleanup()
	if _, err = provider.DiscoverCapabilities(context.Background()); err != nil {
		t.Fatalf("invoke dynamic Provider: %v", err)
	}
	if registrar.invocation.ReleaseVersion != "v0.2.0" ||
		registrar.invocation.ReleaseDigest != strings.Repeat("a", 64) ||
		registrar.invocation.DeclarationDigest != strings.Repeat("b", 64) {
		t.Fatal("dynamic invocation did not preserve exact release evidence")
	}
	if registrar.registration.InterfaceInstanceID != "channel_dynamic_1" {
		t.Fatalf("confidential context was not bound to the exact interface instance: %#v", registrar.registration)
	}
	if bytes.Contains(outboundPayload, []byte("sec_dynamic_runtime_test")) ||
		bytes.Contains(registrar.invocation.Payload, []byte("sec_dynamic_runtime_test")) ||
		bytes.Contains(registrar.invocation.Payload, []byte("provider.invalid")) ||
		bytes.Contains(registrar.invocation.Payload, []byte("channel_dynamic_1")) {
		t.Fatal("dynamic invocation exposed host-only secret, endpoint, or interface evidence")
	}
}
