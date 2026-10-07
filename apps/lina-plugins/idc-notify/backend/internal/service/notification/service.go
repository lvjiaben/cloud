// Package notification owns notification channels, templates, preferences, and delivery evidence.
package notification

import (
	"context"
	"reflect"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/plugin/capability"
	"lina-core/pkg/plugin/capability/bizctxcap"
	"lina-core/pkg/plugin/capability/sessioncap"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/cap/managementcap"
	"lina-plugin-idc-core/backend/cap/notificationcap"
	"lina-plugin-idc-core/backend/cap/onboardingcap"
	"lina-plugin-idc-core/backend/cap/secretcap"
	"lina-plugin-idc-core/backend/cap/supportcap"
	"lina-plugin-idc-notify/backend/cap/channelcap"
	"lina-plugin-shared-services/backend/cap/challengecap"
	"lina-plugin-shared-services/backend/cap/inboxcap"
)

const (
	notificationTenantID = idccap.DefaultTenantID
	tenantIDColumn       = "tenant_id"
)

type Service interface {
	// ListChannels returns the bounded installed notification-channel registry.
	ListChannels(context.Context) ([]Channel, error)
	// ConfigureChannel validates and stores one external channel configuration.
	ConfigureChannel(context.Context, ChannelConfigurationInput) (*Channel, error)
	// ConfigureSMTP stores one installation-owned SMTP account.
	ConfigureSMTP(context.Context, SMTPConfigurationInput) (*Channel, error)
	// ConfigureAliyunSMS selects one installation-owned Aliyun SMS account.
	ConfigureAliyunSMS(context.Context, AliyunSMSConfigurationInput) (*Channel, error)
	// ConfigureSMSBao selects one installation-owned SMSBao account.
	ConfigureSMSBao(context.Context, SMSBaoConfigurationInput) (*Channel, error)
	// ConfigureTencentSMS stores one installation-owned Tencent Cloud SMS account.
	ConfigureTencentSMS(context.Context, TencentSMSConfigurationInput) (*Channel, error)
	// ConfigureWeChatOfficial stores one installation-owned verified service account.
	ConfigureWeChatOfficial(context.Context, WeChatOfficialConfigurationInput) (*Channel, error)
	// ConfigureWeCom stores one installation-owned WeCom custom application.
	ConfigureWeCom(context.Context, WeComConfigurationInput) (*Channel, error)
	// StartOwnWeComBinding issues one short-lived current-staff OAuth authorization.
	StartOwnWeComBinding(context.Context, WeComBindingAuthorizationInput) (*WeComBindingAuthorization, error)
	// CompleteOwnWeComBinding consumes the official code for the same current staff session.
	CompleteOwnWeComBinding(context.Context, WeComBindingCompletionInput) (*WeComBindingStatus, error)
	// GetOwnWeComBinding returns only the current staff member's binding state.
	GetOwnWeComBinding(context.Context) (*WeComBindingStatus, error)
	// RemoveOwnWeComBinding starts recoverable secret revocation and queue cancellation.
	RemoveOwnWeComBinding(context.Context, WeComBindingRemovalInput) (*WeComBindingStatus, error)
	// ProcessWeComBindings recovers bounded binding secret lifecycle work.
	ProcessWeComBindings(context.Context, int) error
	VerifyWeChatCallback(context.Context, EncryptedCallbackInput) (string, error)
	AcceptWeChatCallback(context.Context, EncryptedCallbackInput) error
	// TestChannel sends once to an administrator-entered target without retaining the raw target.
	TestChannel(context.Context, ChannelTestInput) (*ChannelTestResult, error)
	// TestOwnWeCom sends once to the authorized operator's current active WeCom binding.
	TestOwnWeCom(context.Context, ChannelTestInput) (*ChannelTestResult, error)
	// DisableChannel stops new sends and cancels not-yet-sent queued work.
	DisableChannel(context.Context, ChannelLifecycleInput) (*Channel, error)
	// EnableChannel enables one safely configured notification channel.
	EnableChannel(context.Context, ChannelLifecycleInput) (*Channel, error)
	// ClearChannel revokes a disabled channel credential after pending-result checks.
	ClearChannel(context.Context, ChannelLifecycleInput) (*Channel, error)
	// ProcessChannelIntents recovers a bounded batch of interrupted channel credential changes.
	ProcessChannelIntents(context.Context, int) error
	// ListEvents returns the bounded registered notification-event catalog.
	ListEvents(context.Context) ([]Event, error)
	// ListTemplates returns a bounded set of immutable template versions.
	ListTemplates(context.Context, string, string, int) ([]Template, error)
	// GetTemplate returns one authorized template version.
	GetTemplate(context.Context, string, int64) (*Template, error)
	// SaveTemplate creates one immutable template version.
	SaveTemplate(context.Context, TemplateWriteInput) (*Template, error)
	// PreviewTemplate renders one template version without sending it.
	PreviewTemplate(context.Context, TemplatePreviewInput) (*TemplatePreview, error)
	// GetOwnPreferences returns the core-owned current-customer preference projection.
	GetOwnPreferences(context.Context) (*PreferenceSnapshot, error)
	// SaveOwnPreference applies one current-customer preference command through core.
	SaveOwnPreference(context.Context, PreferenceWriteInput) (*PreferenceWriteResult, error)
	// ListDeliveries returns a bounded administrator delivery page.
	ListDeliveries(context.Context, DeliveryListInput) (*DeliveryPage, error)
	// GetDelivery returns one authorized delivery projection.
	GetDelivery(context.Context, string) (*Delivery, error)
	// GetResendEligibility re-evaluates every manual-resend safety gate.
	GetResendEligibility(context.Context, string) (*ResendEligibility, error)
	// ManualResend creates one linear resend successor after every gate passes.
	ManualResend(context.Context, ManualResendInput) (*Delivery, error)
	// ManualCloseUnknown closes one manual-review task without inventing a result.
	ManualCloseUnknown(context.Context, ManualClosureInput) (*Delivery, error)
	// ListOwnDeliveries returns deliveries owned by the authoritative current customer.
	ListOwnDeliveries(context.Context, string, int) (*DeliveryPage, error)
	// Enqueue freezes one validated notification into the durable delivery queue.
	Enqueue(context.Context, EnqueueInput) (*Delivery, error)
	// ProcessEvents materializes a bounded batch from the core-owned event feed.
	ProcessEvents(context.Context, int) error
	// ProcessDue claims and processes a bounded batch of due delivery attempts.
	ProcessDue(context.Context, int) error
	// ProcessQueries converges a bounded batch of pending provider results.
	ProcessQueries(context.Context, int) error
	CreatePresalesHumanChallenge(context.Context, PresalesHumanChallengeInput) (*PresalesHumanChallenge, error)
	CompletePresalesHumanChallenge(context.Context, PresalesHumanCompletionInput) (*PresalesHumanProof, error)
	SendPresalesVerificationCode(context.Context, PresalesVerificationInput) (*PresalesVerification, error)
	CreateAnonymousPresalesTicket(context.Context, PresalesTicketInput) (*PresalesAccess, error)
	RenewAnonymousPresalesAccess(context.Context, PresalesRenewalInput) (*PresalesAccess, error)
	CreateOnboardingHumanChallenge(context.Context, OnboardingHumanChallengeInput) (*PresalesHumanChallenge, error)
	CompleteOnboardingHumanChallenge(context.Context, OnboardingHumanCompletionInput) (*PresalesHumanProof, error)
	SendOnboardingVerificationCode(context.Context, OnboardingVerificationInput) (*PresalesVerification, error)
	CreateVerifiedOnboardingIntent(context.Context, OnboardingIntentInput) (*onboardingcap.VerifiedIntent, error)
}

type adapterBinding struct {
	descriptor channelcap.Descriptor
	adapter    channelcap.Adapter
}

type SecretBinding struct {
	Manager  secretcap.Manager
	Accessor secretcap.Accessor
}

type serviceImpl struct {
	database             gdb.DB
	management           managementcap.Runtime
	sessions             sessioncap.Service
	bizctx               bizctxcap.Service
	inbox                inboxcap.Service
	sources              notificationcap.Runtime
	challenges           challengecap.Service
	support              supportcap.Service
	onboarding           onboardingcap.Service
	verificationAccessor secretcap.Accessor
	verificationHandle   secretcap.Handle
	runtimeConfig        RuntimeConfig
	secrets              map[string]SecretBinding
	adapters             map[string]adapterBinding
	now                  func() time.Time
}

var _ Service = (*serviceImpl)(nil)

func New(
	database gdb.DB,
	management managementcap.Runtime,
	services capability.Services,
	inbox inboxcap.Service,
	sources notificationcap.Runtime,
	challenges challengecap.Service,
	support supportcap.Service,
	onboarding onboardingcap.Service,
	verificationAccessor secretcap.Accessor,
	verificationHandle secretcap.Handle,
	runtimeConfig RuntimeConfig,
	secrets map[string]SecretBinding,
	descriptors []channelcap.Descriptor,
) (Service, error) {
	if database == nil || managementcap.ValidateRuntime(management) != nil || services == nil ||
		nilDependency(services.Sessions()) || nilDependency(services.BizCtx()) ||
		nilDependency(inbox) || notificationcap.ValidateRuntime(sources) != nil || nilDependency(challenges) ||
		nilDependency(support) || nilDependency(onboarding) || nilDependency(verificationAccessor) || verificationHandle.IsZero() ||
		runtimeConfig.SMSVerificationDailyLimit < 1 || runtimeConfig.SMSVerificationDailyLimit > 100000 || len(secrets) == 0 {
		return nil, gerror.New("notification service dependencies are invalid")
	}
	for _, channel := range []string{"email", "sms", "wechat_official", "wechat_work"} {
		binding, exists := secrets[channel]
		if !exists || nilDependency(binding.Manager) || nilDependency(binding.Accessor) {
			return nil, gerror.New("notification channel secret binding is invalid")
		}
	}
	adapters := make(map[string]adapterBinding, len(descriptors))
	for _, descriptor := range descriptors {
		if err := channelcap.ValidateDescriptor(descriptor); err != nil {
			return nil, err
		}
		secretBinding, exists := secrets[string(descriptor.Kind)]
		if !exists || nilDependency(secretBinding.Accessor) {
			return nil, gerror.New("notification adapter secret binding is unavailable")
		}
		adapter, err := descriptor.Factory(secretBinding.Accessor)
		if err != nil {
			return nil, gerror.Wrap(err, "construct notification channel adapter")
		}
		if err = channelcap.ValidateAdapter(adapter); err != nil {
			return nil, gerror.Wrap(err, "validate notification channel adapter")
		}
		adapters[descriptor.ProviderCode] = adapterBinding{descriptor: descriptor, adapter: adapter}
	}
	return &serviceImpl{
		database: database, management: management, sessions: services.Sessions(),
		bizctx: services.BizCtx(), inbox: inbox, sources: sources, challenges: challenges, support: support, onboarding: onboarding,
		verificationAccessor: verificationAccessor, verificationHandle: verificationHandle, runtimeConfig: runtimeConfig,
		secrets: secrets, adapters: adapters, now: time.Now,
	}, nil
}

func (s *serviceImpl) secretBinding(channel string) (SecretBinding, error) {
	if s == nil {
		return SecretBinding{}, gerror.New("notification service is unavailable")
	}
	binding, exists := s.secrets[channel]
	if !exists || nilDependency(binding.Manager) || nilDependency(binding.Accessor) {
		return SecretBinding{}, gerror.New("notification channel secret binding is unavailable")
	}
	return binding, nil
}

func nilDependency(value any) bool {
	if value == nil {
		return true
	}
	reflected := reflect.ValueOf(value)
	switch reflected.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return reflected.IsNil()
	default:
		return false
	}
}
