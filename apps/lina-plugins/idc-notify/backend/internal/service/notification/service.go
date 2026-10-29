// Package notification owns notification channels, templates, preferences, and delivery evidence.
package notification

import (
	"context"
	"reflect"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/plugin/capability"
	"lina-core/pkg/plugin/capability/authcap/extlogin"
	"lina-core/pkg/plugin/capability/bizctxcap"
	"lina-core/pkg/plugin/capability/sessioncap"
	"lina-plugin-idc-core/backend/cap/customerlogincap"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/cap/managementcap"
	"lina-plugin-idc-core/backend/cap/notificationcap"
	"lina-plugin-idc-core/backend/cap/onboardingcap"
	"lina-plugin-idc-core/backend/cap/profilechangecap"
	"lina-plugin-idc-core/backend/cap/secretcap"
	"lina-plugin-idc-core/backend/cap/supportcap"
	idcnotify "lina-plugin-idc-notify"
	"lina-plugin-idc-notify/backend/cap/channelcap"
	"lina-plugin-idc-notify/backend/internal/service/loginprofile"
	"lina-plugin-shared-services/backend/cap/challengecap"
	"lina-plugin-shared-services/backend/cap/inboxcap"
)

const (
	notificationTenantID = idccap.DefaultTenantID
	tenantIDColumn       = "tenant_id"
)

type Service interface {
	profilechangecap.Provider
	profilechangecap.RecoveryProvider
	// CustomerPasswordLogin authenticates an explicit username or contact method.
	CustomerPasswordLogin(context.Context, CustomerLoginInput) (*loginprofile.Session, error)
	// CreateRecoveryHumanChallenge binds a challenge to the current contact.
	CreateRecoveryHumanChallenge(context.Context, RecoveryChallengeInput) (*PresalesHumanChallenge, error)
	// CompleteRecoveryHumanChallenge returns single-use authorization to send.
	CompleteRecoveryHumanChallenge(context.Context, RecoveryCompletionInput) (*PresalesHumanProof, error)
	// SendRecoveryCode sends a bounded, purpose-bound password recovery OTP.
	SendRecoveryCode(context.Context, RecoveryCodeInput) (*PresalesVerification, error)
	// ResetCustomerPassword changes the verified account password automatically.
	ResetCustomerPassword(context.Context, RecoveryResetInput) (*RecoveryResult, error)
	// CheckRecoverySession rejects exact sessions captured by a completed reset.
	CheckRecoverySession(context.Context) error
	// RetryRecoveryCleanup retries revocation using a separate post-reset grant.
	RetryRecoveryCleanup(context.Context, RecoveryCleanupInput) (bool, error)
	// CleanupRecoverySessions retries bounded revocation for the actual current user.
	CleanupRecoverySessions(context.Context, string) (bool, error)
	// SendLoginContactCode sends a rate-limited code to the authoritative old or verified new contact.
	SendLoginContactCode(context.Context, LoginContactInput) (*PresalesVerification, error)
	// VerifyLoginContactCode verifies one session-bound code without changing the host profile.
	VerifyLoginContactCode(context.Context, LoginContactVerifyInput) error
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
	// TestChannel sends once to an administrator-entered target without retaining the raw target.
	TestChannel(context.Context, ChannelTestInput) (*ChannelTestResult, error)
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
	renewalPeriods       renewalPeriodLabels
	loginDirectory       customerlogincap.Directory
	loginAuth            loginprofile.AuthClient
	externalLogin        extlogin.Service
	database             gdb.DB
	loginProfiles        loginprofile.Reader
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
	loginProfiles loginprofile.Reader,
	loginDirectory customerlogincap.Directory,
	loginAuth loginprofile.AuthClient,
	externalLogin extlogin.Service,
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
	if database == nil || nilDependency(loginDirectory) || nilDependency(loginAuth) || nilDependency(externalLogin) || nilDependency(loginProfiles) || managementcap.ValidateRuntime(management) != nil || services == nil ||
		nilDependency(services.Sessions()) || nilDependency(services.BizCtx()) ||
		nilDependency(inbox) || notificationcap.ValidateRuntime(sources) != nil || nilDependency(challenges) ||
		nilDependency(support) || nilDependency(onboarding) || nilDependency(verificationAccessor) || verificationHandle.IsZero() ||
		runtimeConfig.SMSVerificationDailyLimit < 1 || runtimeConfig.SMSVerificationDailyLimit > 100000 || len(secrets) == 0 {
		return nil, gerror.New("notification service dependencies are invalid")
	}
	for _, channel := range []string{"email", "sms"} {
		binding, exists := secrets[channel]
		if !exists || nilDependency(binding.Manager) || nilDependency(binding.Accessor) {
			return nil, gerror.New("notification channel secret binding is invalid")
		}
	}
	renewalPeriods, err := loadRenewalPeriodLabels(idcnotify.EmbeddedFiles)
	if err != nil {
		return nil, err
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
		database: database, loginProfiles: loginProfiles, loginDirectory: loginDirectory, loginAuth: loginAuth, externalLogin: externalLogin, management: management, sessions: services.Sessions(),
		bizctx: services.BizCtx(), inbox: inbox, sources: sources, challenges: challenges, support: support, onboarding: onboarding,
		verificationAccessor: verificationAccessor, verificationHandle: verificationHandle, runtimeConfig: runtimeConfig,
		secrets: secrets, adapters: adapters, now: time.Now, renewalPeriods: renewalPeriods,
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
