package composition

import (
	"context"

	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"

	"lina-core/pkg/plugin/capability"
	secretspi "lina-plugin-idc-core/backend/cap/secretcap/spi"
	"lina-plugin-idc-notify/backend/cap/channelcap"
	"lina-plugin-idc-notify/backend/cap/securitycodecap"
	"lina-plugin-idc-notify/backend/internal/service/notification"
	challengesource "lina-plugin-shared-services/backend/cap/challengecap/source"
)

// BuildSecurityCodeService creates the minimal notification-owned runtime used
// by another source plugin. It intentionally does not expose the ordinary
// notification contact directory or the full notification management service.
func BuildSecurityCodeService(ctx context.Context, services capability.Services, callerPluginID string) (securitycodecap.Service, error) {
	if ctx == nil || ctx.Err() != nil || services == nil || callerPluginID == "" {
		return nil, gerror.New("security-code service composition context is invalid")
	}
	challenges, err := challengesource.New(ctx, services, callerPluginID)
	if err != nil {
		return nil, gerror.Wrap(err, "construct security-code human challenge source")
	}
	secretSnapshot, err := secretspi.Freeze()
	if err != nil {
		return nil, gerror.Wrap(err, "freeze security-code secret source")
	}
	if secretSnapshot.Factory() == nil {
		return nil, gerror.New("security-code secret source factory is unavailable")
	}
	secretSource, err := secretSnapshot.Factory()(ctx)
	if err != nil {
		return nil, gerror.Wrap(err, "construct security-code secret source")
	}
	verificationAccessor, verificationHandle, err := bindVerificationSecret(ctx, secretSource)
	if err != nil {
		return nil, err
	}
	bindings, err := bindChannelSecrets(secretSource)
	if err != nil {
		return nil, err
	}
	if err = declareChannelAdapters(); err != nil {
		return nil, err
	}
	descriptors, err := channelcap.Freeze()
	if err != nil {
		return nil, gerror.Wrap(err, "freeze security-code channel adapters")
	}
	runtimeConfig, err := notification.LoadRuntimeConfig()
	if err != nil {
		return nil, err
	}
	return notification.NewSecurityCodeService(g.DB(), callerPluginID, challenges, verificationAccessor,
		verificationHandle, runtimeConfig, bindings, descriptors)
}
