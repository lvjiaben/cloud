// This file constructs the production resource-owner Provider runtime.
package composition

import (
	"context"
	"time"

	"lina-plugin-idc-core/backend/cap/secretcap"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
	providerspi "lina-plugin-idc-resource/backend/cap/resourcecap/spi"
	"lina-plugin-idc-resource/backend/internal/dao"
	"lina-plugin-idc-resource/backend/internal/service/channelconfig"
	"lina-plugin-idc-resource/backend/internal/service/providerauthsession"
)

func NewProductionProviderRuntime(
	ctx context.Context,
	declarations []providerspi.Declaration,
	options ...ProviderRuntimeOption,
) (*ProviderRuntime, error) {
	secretSource, err := LoadSecretSource(ctx)
	if err != nil {
		return nil, err
	}
	channels, err := channelconfig.NewReader(dao.Channel.DB())
	if err != nil {
		return nil, err
	}
	return NewProviderRuntime(declarations, secretSource, channels,
		func(ctx context.Context, configuration channelconfig.Configuration) (resourcecap.ProviderEnv, func(), error) {
			return productionProviderEnvironment(ctx, configuration, secretSource)
		}, options...,
	)
}

func productionProviderEnvironment(
	ctx context.Context,
	configuration channelconfig.Configuration,
	secretSource secretcap.Source,
) (resourcecap.ProviderEnv, func(), error) {
	client, closeClient, err := newOutboundHTTPClient(configuration.Config.Endpoint, configuration.Config.TLSCertificateSHA256)
	if err != nil {
		return resourcecap.ProviderEnv{}, nil, err
	}
	manager, err := BindProviderSecretManager(secretSource, configuration.Config.ChannelRef)
	if err != nil {
		closeClient()
		return resourcecap.ProviderEnv{}, nil, err
	}
	accessor, err := BindProviderSecretAccessor(secretSource, configuration.Config.ChannelRef)
	if err != nil {
		closeClient()
		return resourcecap.ProviderEnv{}, nil, err
	}
	sessions, err := providerauthsession.New(dao.Channel.DB(), configuration.Config.ChannelRef, manager, accessor)
	if err != nil {
		closeClient()
		return resourcecap.ProviderEnv{}, nil, err
	}
	if ctx == nil || ctx.Err() != nil {
		closeClient()
		return resourcecap.ProviderEnv{}, nil, context.Canceled
	}
	return resourcecap.ProviderEnv{
		HTTPClient: client, Clock: productionClock{},
		Logger: productionLogger{}, Metrics: productionMetrics{},
		AuthenticationSessions: sessions, Secrets: accessor,
	}, closeClient, nil
}

type productionClock struct{}

func (productionClock) Now() time.Time { return time.Now().UTC() }

// Diagnostics are accepted only after the owner validator proves they contain
// no credential material. The first production release deliberately discards
// the payload instead of forwarding adapter-controlled strings to host logs.
type productionLogger struct{}

func (productionLogger) Log(
	_ context.Context,
	level resourcecap.LogLevel,
	message string,
	fields ...resourcecap.LogField,
) {
	_ = resourcecap.ValidateLogEntry(level, message, fields...)
}

type productionMetrics struct{}

func (productionMetrics) AddCounter(
	_ context.Context,
	name string,
	_ int64,
	labels ...resourcecap.MetricLabel,
) {
	_ = resourcecap.ValidateMetric(name, labels...)
}

func (productionMetrics) Observe(
	_ context.Context,
	name string,
	value float64,
	labels ...resourcecap.MetricLabel,
) {
	_ = resourcecap.ValidateMetricObservation(name, value, labels...)
}
