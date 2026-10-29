// This file composes frozen source Provider factories into a channel runtime.
package composition

import (
	"context"
	"regexp"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/logger"
	"lina-core/pkg/plugin/capability/capmodel"
	"lina-core/pkg/plugin/capability/plugincap"
	"lina-core/pkg/plugin/pluginhost"
	"lina-plugin-idc-core/backend/cap/secretcap"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
	"lina-plugin-idc-resource/backend/cap/resourcecap/spi"
	"lina-plugin-idc-resource/backend/internal/service/channelconfig"
)

var taskConfigurationRevisionPattern = regexp.MustCompile(`^rci_[0-9a-f]{32}$`)

// ChannelReader loads one exact enabled channel without returning serializable secret material.
type ChannelReader interface {
	ReadEnabled(ctx context.Context, channelRef string) (channelconfig.Configuration, error)
}

type revisionChannelReader interface {
	ReadRevision(ctx context.Context, channelRef, revisionNo string) (channelconfig.Configuration, error)
}

type configuredChannelReader interface {
	ReadConfigured(ctx context.Context, channelRef string) (channelconfig.Configuration, error)
}

// EnvironmentFactory supplies resource-owner controlled non-secret Provider dependencies.
type EnvironmentFactory func(
	ctx context.Context,
	configuration channelconfig.Configuration,
) (resourcecap.ProviderEnv, func(), error)

// ProviderRuntime explicitly owns immutable factories, source-secret access,
// channel configuration, and environment construction. It is not global.
type ProviderRuntime struct {
	secretSource       secretcap.Source
	channels           ChannelReader
	environments       EnvironmentFactory
	factories          map[string]resourcecap.ProviderFactory
	businessExtensions pluginhost.BusinessExtensionRegistrar
	dynamicOutbound    DynamicOutboundFactory
}

type ProviderRuntimeOption func(*ProviderRuntime)

func WithDynamicBusinessExtensions(
	extensions pluginhost.BusinessExtensionRegistrar,
	outbound DynamicOutboundFactory,
) ProviderRuntimeOption {
	return func(runtime *ProviderRuntime) {
		runtime.businessExtensions = extensions
		runtime.dynamicOutbound = outbound
	}
}

// NewProviderRuntime validates and detaches the exact startup dependencies.
func NewProviderRuntime(
	declarations []spi.Declaration,
	secretSource secretcap.Source,
	channels ChannelReader,
	environments EnvironmentFactory,
	options ...ProviderRuntimeOption,
) (*ProviderRuntime, error) {
	if isNilInterface(secretSource) || isNilInterface(channels) || environments == nil {
		return nil, gerror.New("resource Provider runtime dependencies are invalid")
	}
	factories := make(map[string]resourcecap.ProviderFactory, len(declarations))
	for _, declaration := range declarations {
		if err := declaration.Key.Validate(); err != nil ||
			declaration.Key.ContractVersion != resourcecap.ContractVersion ||
			declaration.Factory == nil {
			return nil, gerror.New("resource Provider declaration is invalid")
		}
		key := providerFactoryKey(declaration.Key.PluginID, declaration.Key.ProviderID)
		if _, exists := factories[key]; exists {
			return nil, gerror.New("resource Provider declaration is duplicated")
		}
		factories[key] = declaration.Factory
	}
	runtime := &ProviderRuntime{
		secretSource: secretSource,
		channels:     channels,
		environments: environments,
		factories:    factories,
	}
	for _, option := range options {
		if option != nil {
			option(runtime)
		}
	}
	return runtime, nil
}

// Construct builds a fresh Provider only for one currently enabled channel.
// It performs no runtime registry lookup and never caches secret handles.
func (r *ProviderRuntime) Construct(ctx context.Context, channelRef string) (resourcecap.Provider, error) {
	provider, _, err := r.ConstructSession(ctx, channelRef)
	return provider, err
}

// ConstructSession returns a fresh Provider and the exact owner transport cleanup.
// Callers performing production work must invoke cleanup after the bounded call.
func (r *ProviderRuntime) ConstructSession(
	ctx context.Context,
	channelRef string,
) (resourcecap.Provider, func(), error) {
	if r == nil || ctx == nil || ctx.Err() != nil {
		return nil, nil, gerror.New("resource Provider construction failed")
	}
	configuration, err := r.channels.ReadEnabled(ctx, channelRef)
	if err != nil {
		logger.Warningf(ctx, "resource provider channel read failed: %s", gerror.Current(err).Error())
		return nil, nil, gerror.New("resource Provider construction failed")
	}
	return r.constructConfiguration(ctx, channelRef, configuration)
}

// ConstructConfiguredSession is reserved for an explicit administrator
// connection check. It builds a configured disabled channel without changing
// its business state or allowing normal jobs to use it.
func (r *ProviderRuntime) ConstructConfiguredSession(
	ctx context.Context,
	channelRef string,
) (resourcecap.Provider, func(), error) {
	if r == nil || ctx == nil || ctx.Err() != nil {
		return nil, nil, gerror.New("resource Provider construction failed")
	}
	reader, ok := r.channels.(configuredChannelReader)
	if !ok {
		return nil, nil, gerror.New("resource Provider construction failed")
	}
	configuration, err := reader.ReadConfigured(ctx, channelRef)
	if err != nil {
		return nil, nil, gerror.New("resource Provider construction failed")
	}
	if configuration.RuntimeType == "dynamic" {
		if r.businessExtensions == nil {
			return nil, nil, gerror.New("resource Provider connector is unavailable")
		}
		page, listErr := r.businessExtensions.ListProviders(ctx, plugincap.BusinessExtensionProviderListInput{
			Point: resourcecap.DynamicProviderPoint, ContractVersion: configuration.ContractVersion,
			Page: capmodel.PageRequest{PageNum: 1, PageSize: 100, Limit: 100},
		})
		if listErr != nil || page == nil || len(page.Items) > 100 {
			return nil, nil, gerror.New("resource Provider connector is unavailable")
		}
		var selected *capmodel.BusinessExtensionProviderEvidence
		for _, item := range page.Items {
			if item != nil && item.PluginID == configuration.PluginID && item.ContractVersion == configuration.ContractVersion {
				if selected != nil {
					return nil, nil, gerror.New("resource Provider connector is ambiguous")
				}
				selected = item
			}
		}
		if selected == nil || resourcecap.ValidateProviderMetadata(selected.ProviderMetadata, selected.InterfaceConfiguration) != nil ||
			selected.ProviderMetadata.Values["providerId"] != configuration.ProviderID {
			return nil, nil, gerror.New("resource Provider connector is incompatible")
		}
		settings, settingsErr := resourcecap.ProjectPublicInterfaceSettings(configuration.Config.Settings, false)
		if settingsErr != nil {
			return nil, nil, settingsErr
		}
		configuration.Config.Settings, settingsErr = resourcecap.CompileInterfaceSettings(settings, selected.ProviderMetadata)
		if settingsErr != nil {
			return nil, nil, settingsErr
		}
		configuration.ReleaseVersion = selected.ReleaseVersion
		configuration.ReleaseDigest = selected.ReleaseDigest
		configuration.DeclarationDigest = selected.DeclarationDigest
	}
	return r.constructConfiguration(ctx, channelRef, configuration)
}

// ConstructRevisionSession builds from the immutable interface configuration
// revision frozen into one durable task. It never falls back to current channel settings.
func (r *ProviderRuntime) ConstructRevisionSession(
	ctx context.Context,
	channelRef string,
	revisionNo string,
) (resourcecap.Provider, func(), error) {
	if r == nil || ctx == nil || ctx.Err() != nil ||
		!taskConfigurationRevisionPattern.MatchString(revisionNo) {
		return nil, nil, gerror.New("resource Provider construction failed")
	}
	reader, ok := r.channels.(revisionChannelReader)
	if !ok {
		return nil, nil, gerror.New("resource Provider construction failed")
	}
	configuration, err := reader.ReadRevision(ctx, channelRef, revisionNo)
	if err != nil {
		return nil, nil, gerror.New("resource Provider construction failed")
	}
	return r.constructConfiguration(ctx, channelRef, configuration)
}

func (r *ProviderRuntime) constructConfiguration(
	ctx context.Context,
	channelRef string,
	configuration channelconfig.Configuration,
) (resourcecap.Provider, func(), error) {
	if configuration.RuntimeType == "dynamic" {
		if r.businessExtensions == nil || r.dynamicOutbound == nil {
			return nil, nil, gerror.New("resource Provider construction failed")
		}
		return &dynamicProvider{
			extensions: r.businessExtensions, configuration: configuration, outbound: r.dynamicOutbound,
		}, func() {}, nil
	}
	factory := r.factories[providerFactoryKey(configuration.PluginID, configuration.ProviderID)]
	if factory == nil {
		return nil, nil, gerror.New("resource Provider construction failed")
	}
	environment, cleanup, err := r.environments(ctx, configuration)
	if err != nil || cleanup == nil {
		return nil, nil, gerror.New("resource Provider construction failed")
	}
	failed := true
	defer func() {
		if failed {
			cleanup()
		}
	}()
	environment.PluginID = configuration.PluginID
	environment.ProviderID = configuration.ProviderID
	environment, err = InjectProviderSecrets(r.secretSource, channelRef, environment)
	if err != nil || environment.Validate() != nil || configuration.Config.Validate() != nil {
		return nil, nil, gerror.New("resource Provider construction failed")
	}
	provider, err := factory(ctx, environment, configuration.Config)
	if err != nil || isNilInterface(provider) {
		return nil, nil, gerror.New("resource Provider construction failed")
	}
	failed = false
	return provider, cleanup, nil
}

func providerFactoryKey(pluginID string, providerID string) string {
	return pluginID + "\x00" + providerID
}
