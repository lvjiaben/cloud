package composition

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"sync"

	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"

	"lina-core/pkg/plugin/capability"
	customerloginspi "lina-plugin-idc-core/backend/cap/customerlogincap/spi"
	"lina-plugin-idc-core/backend/cap/idccap"
	managementspi "lina-plugin-idc-core/backend/cap/managementcap/spi"
	notificationspi "lina-plugin-idc-core/backend/cap/notificationcap/spi"
	"lina-plugin-idc-core/backend/cap/onboardingcap"
	onboardingspi "lina-plugin-idc-core/backend/cap/onboardingcap/spi"
	"lina-plugin-idc-core/backend/cap/secretcap"
	secretspi "lina-plugin-idc-core/backend/cap/secretcap/spi"
	"lina-plugin-idc-core/backend/cap/supportcap"
	supportspi "lina-plugin-idc-core/backend/cap/supportcap/spi"
	"lina-plugin-idc-notify/backend/cap/channelcap"
	aliyunsmsadapter "lina-plugin-idc-notify/backend/internal/adapter/aliyunsms"
	smsbaoadapter "lina-plugin-idc-notify/backend/internal/adapter/smsbao"
	smtpadapter "lina-plugin-idc-notify/backend/internal/adapter/smtp"
	tencentsmsadapter "lina-plugin-idc-notify/backend/internal/adapter/tencentcloudsms"
	"lina-plugin-idc-notify/backend/internal/service/loginprofile"
	"lina-plugin-idc-notify/backend/internal/service/notification"
	challengesource "lina-plugin-shared-services/backend/cap/challengecap/source"
	inboxsource "lina-plugin-shared-services/backend/cap/inboxcap/source"
)

const notifyOwnerPluginID = "idc-notify"

var (
	declareChannelAdaptersOnce sync.Once
	errDeclareChannelAdapters  error
)

func BuildNotificationService(ctx context.Context, services capability.Services) (notification.Service, error) {
	if ctx == nil || ctx.Err() != nil || services == nil {
		return nil, gerror.New("notification service composition context is invalid")
	}
	managementSnapshot, err := managementspi.Freeze()
	if err != nil {
		return nil, gerror.Wrap(err, "freeze notification management runtime")
	}
	if managementSnapshot.Factory() == nil {
		return nil, gerror.New("notification management runtime factory is unavailable")
	}
	management, err := managementSnapshot.Factory()(ctx, services)
	if err != nil {
		return nil, gerror.Wrap(err, "construct notification management runtime")
	}
	sourceSnapshot, err := notificationspi.Freeze()
	if err != nil {
		return nil, gerror.Wrap(err, "freeze notification source runtime")
	}
	if sourceSnapshot.Factory() == nil {
		return nil, gerror.New("notification source runtime factory is unavailable")
	}
	sources, err := sourceSnapshot.Factory()(ctx, services, notifyOwnerPluginID)
	if err != nil {
		return nil, gerror.Wrap(err, "construct notification source runtime")
	}
	inbox, err := inboxsource.New(ctx, services, notifyOwnerPluginID)
	if err != nil {
		return nil, gerror.Wrap(err, "construct shared inbox source")
	}
	secretSnapshot, err := secretspi.Freeze()
	if err != nil {
		return nil, gerror.Wrap(err, "freeze notification secret source")
	}
	if secretSnapshot.Factory() == nil {
		return nil, gerror.New("notification secret source factory is unavailable")
	}
	secretSource, err := secretSnapshot.Factory()(ctx)
	if err != nil {
		return nil, gerror.Wrap(err, "construct notification secret source")
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
		return nil, gerror.Wrap(err, "freeze notification channel adapters")
	}
	challenges, err := challengesource.New(ctx, services, notifyOwnerPluginID)
	if err != nil {
		return nil, gerror.Wrap(err, "construct shared human challenge source")
	}
	support, err := bindPresalesSupport(ctx, services)
	if err != nil {
		return nil, err
	}
	onboarding, err := bindCustomerOnboarding(ctx, services)
	if err != nil {
		return nil, err
	}
	verificationAccessor, verificationHandle, err := bindVerificationSecret(ctx, secretSource)
	if err != nil {
		return nil, err
	}
	runtimeConfig, err := notification.LoadRuntimeConfig()
	if err != nil {
		return nil, err
	}
	factory := customerloginspi.Current()
	if factory == nil {
		return nil, gerror.New("customer login directory factory unavailable")
	}
	directory, err := factory(ctx, services, notifyOwnerPluginID)
	if err != nil {
		return nil, err
	}
	profileClient := loginprofile.New()
	return notification.New(g.DB(), profileClient, directory, profileClient, services.Auth().ExternalLogin(), management, services, inbox, sources, challenges, support, onboarding,
		verificationAccessor, verificationHandle, runtimeConfig, bindings, descriptors)
}

func declareChannelAdapters() error {
	declareChannelAdaptersOnce.Do(func() {
		for _, declare := range []func() error{smtpadapter.Declare, aliyunsmsadapter.Declare, smsbaoadapter.Declare,
			tencentsmsadapter.Declare} {
			if err := declare(); err != nil {
				errDeclareChannelAdapters = gerror.Wrap(err, "declare notification channel adapter")
				return
			}
		}
	})
	return errDeclareChannelAdapters
}

func bindPresalesSupport(ctx context.Context, services capability.Services) (supportcap.Service, error) {
	snapshot, err := supportspi.Freeze()
	if err != nil {
		return nil, gerror.Wrap(err, "freeze anonymous presales support source")
	}
	if snapshot.Factory() == nil {
		return nil, gerror.New("anonymous presales support source factory is unavailable")
	}
	support, err := snapshot.Factory()(ctx, services, notifyOwnerPluginID)
	if err != nil {
		return nil, gerror.Wrap(err, "construct anonymous presales support source")
	}
	return support, nil
}

func bindCustomerOnboarding(ctx context.Context, services capability.Services) (onboardingcap.Service, error) {
	snapshot, err := onboardingspi.Freeze()
	if err != nil {
		return nil, gerror.Wrap(err, "freeze customer onboarding source")
	}
	if snapshot.Factory() == nil {
		return nil, gerror.New("customer onboarding source factory is unavailable")
	}
	service, err := snapshot.Factory()(ctx, services, notifyOwnerPluginID)
	if err != nil {
		return nil, gerror.Wrap(err, "construct customer onboarding source")
	}
	return service, nil
}

func bindVerificationSecret(ctx context.Context, source secretcap.Source) (secretcap.Accessor, secretcap.Handle, error) {
	digest := sha256.Sum256([]byte("idc-notify\x00installation\x00verification"))
	scope := secretcap.Scope{TenantID: idccap.DefaultTenantID, OwnerPluginID: notifyOwnerPluginID,
		CallerScope: "notify.verification.c" + hex.EncodeToString(digest[:16])}
	manager, err := source.Manager(scope)
	if err != nil {
		return nil, secretcap.Handle{}, gerror.Wrap(err, "bind verification secret manager")
	}
	generated, ok := manager.(secretcap.GeneratedManager)
	if !ok {
		return nil, secretcap.Handle{}, gerror.New("verification generated-secret manager is unavailable")
	}
	const idempotencyKey = "notify-verification-hmac-v2"
	created, createErr := generated.CreateGenerated(ctx, idempotencyKey, secretcap.GeneratedSecretPolicyURLSafe256)
	if createErr != nil {
		created, err = manager.RecoverCreate(ctx, idempotencyKey)
		if err != nil {
			return nil, secretcap.Handle{}, gerror.Wrap(createErr, "create verification secret")
		}
	}
	if created.Handle.IsZero() || !created.Metadata.Configured {
		return nil, secretcap.Handle{}, gerror.New("verification secret is not configured")
	}
	accessor, err := source.Accessor(scope)
	if err != nil || accessor == nil {
		return nil, secretcap.Handle{}, gerror.Wrap(err, "bind verification secret accessor")
	}
	return accessor, created.Handle, nil
}

func bindChannelSecrets(source secretcap.Source) (map[string]notification.SecretBinding, error) {
	if source == nil {
		return nil, gerror.New("notification secret source is unavailable")
	}
	// Bind only the two supported external delivery channels.
	channels := []struct {
		code      string
		scopeCode string
	}{
		{code: "email", scopeCode: "email"},
		{code: "sms", scopeCode: "sms"},
	}
	result := make(map[string]notification.SecretBinding, len(channels))
	for _, channel := range channels {
		digest := sha256.Sum256([]byte("idc-notify\x00installation\x00" + channel.code))
		scope := secretcap.Scope{TenantID: idccap.DefaultTenantID, OwnerPluginID: notifyOwnerPluginID,
			CallerScope: "notify.channel." + channel.scopeCode + ".c" + hex.EncodeToString(digest[:16])}
		manager, err := source.Manager(scope)
		if err != nil {
			return nil, gerror.Wrap(err, "bind notification channel secret manager")
		}
		if manager == nil {
			return nil, gerror.New("notification channel secret manager is unavailable")
		}
		accessor, err := source.Accessor(scope)
		if err != nil {
			return nil, gerror.Wrap(err, "bind notification channel secret accessor")
		}
		if accessor == nil {
			return nil, gerror.New("notification channel secret accessor is unavailable")
		}
		result[channel.code] = notification.SecretBinding{Manager: manager, Accessor: accessor}
	}
	return result, nil
}
