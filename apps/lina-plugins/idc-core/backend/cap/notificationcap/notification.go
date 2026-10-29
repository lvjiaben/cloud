// Package notificationcap defines the source-only facts consumed by the IDC
// notification plugin. It deliberately exposes no contact directory, secret,
// database handle, or dynamic-plugin bridge.
package notificationcap

import (
	"context"
	"reflect"
	"time"

	"github.com/gogf/gf/v2/errors/gerror"
)

const (
	OwnerPluginID          = "idc-core"
	AllowedCallerPluginID  = "idc-notify"
	MaximumContactBatch    = 500
	MaximumPreferenceItems = 500
)

type Channel string

const (
	ChannelInApp          Channel = "in_app"
	ChannelEmail          Channel = "email"
	ChannelSMS            Channel = "sms"
	ChannelWeChatOfficial Channel = "wechat_official"
)

type Topic string

const (
	TopicAccount   Topic = "account"
	TopicBilling   Topic = "billing"
	TopicService   Topic = "service"
	TopicSecurity  Topic = "security"
	TopicSupport   Topic = "support"
	TopicMarketing Topic = "marketing"
)

type AddressOwner string

const (
	AddressOwnerCoreContact   AddressOwner = "core_contact"
	AddressOwnerHostIdentity  AddressOwner = "host_identity"
	AddressOwnerNotifyBinding AddressOwner = "notify_binding"
)

// ContactRequest names one already-known customer/contact pair. Callers cannot
// omit ContactNo to discover or enumerate a customer's contacts.
type ContactRequest struct {
	CustomerNo string
	ContactNo  string
	Channel    Channel
	Topic      Topic
}

// ContactProjection contains only the current non-secret facts required to
// decide whether a previously selected target remains usable. Raw email and
// phone values are delivered only through ContactConsumer.
type ContactProjection struct {
	CustomerNo        string
	ContactNo         string
	Channel           Channel
	Topic             Topic
	PreferredLocale   string
	AddressOwner      AddressOwner
	HostUserID        int
	ContactVersion    int64
	PreferencePresent bool
	PreferenceEnabled bool
	PreferenceVersion int64
}

// ContactConsumer synchronously receives one exact projection and a temporary
// destination view. destination is populated only for core-owned email/SMS
// addresses and is cleared immediately after the callback returns.
type ContactConsumer func(index int, projection ContactProjection, destination []byte) error

// ContactResolver resolves only the exact bounded request set supplied by the
// caller. It offers no list, search, count, or destination return value.
type ContactResolver interface {
	ResolveBatch(context.Context, []ContactRequest, ContactConsumer) error
}

type BusinessObjectType string

const (
	BusinessObjectSupportTicket     BusinessObjectType = "support_ticket"
	BusinessObjectContentItem       BusinessObjectType = "content_item"
	BusinessObjectMarketingCampaign BusinessObjectType = "marketing_campaign"
	BusinessObjectAccount           BusinessObjectType = "account"
	BusinessObjectService           BusinessObjectType = "service"
	BusinessObjectBilling           BusinessObjectType = "billing"
)

type BusinessObjectRequest struct {
	Type     BusinessObjectType
	ObjectNo string
}

// BusinessObjectProjection is returned only after the current requester has
// been authorized against the object's owning service. Hidden and nonexistent
// objects share the same failure path.
type BusinessObjectProjection struct {
	Type           BusinessObjectType
	ObjectNo       string
	CurrentVersion int64
}

type BusinessObjectAccess interface {
	AuthorizeCurrent(context.Context, BusinessObjectRequest) (BusinessObjectProjection, error)
}

// CurrentAudienceProjection is the smallest source-owned customer identity
// needed to scope high-frequency account notification queries. It exposes no
// host user identifier, staff projection, contact, or preference collection.
type CurrentAudienceProjection struct {
	CustomerNo      string
	CustomerVersion int64
}

// CurrentAudience resolves only the authenticated current customer. Staff,
// impersonation, acting-user, cross-tenant, revoked, and inconsistent contexts
// are rejected through the same inaccessible result.
type CurrentAudience interface {
	Current(context.Context) (CurrentAudienceProjection, error)
}

// MarketingSendInput is the bounded public form of the core marketing gate.
type MarketingSendInput struct {
	CampaignNo    string
	SnapshotNo    string
	CustomerNo    string
	ContactNo     string
	Channel       Channel
	ContentDigest string
}

type MarketingSendDecision struct {
	CampaignNo        string
	SnapshotNo        string
	CustomerNo        string
	ContactNo         string
	Channel           Channel
	Allowed           bool
	ReasonCode        string
	ConsentEventNo    string
	ConsentVersion    int64
	PreferenceVersion int64
	QuietHoursVersion int64
	EvaluatedAt       time.Time
	DecisionDigest    string
}

type MarketingSendGate interface {
	CheckSendBatch(context.Context, []MarketingSendInput) ([]MarketingSendDecision, error)
}

// PreferenceChange replaces one exact contact/topic/channel choice. Security
// notifications cannot be disabled, even by an otherwise valid command.
type PreferenceChange struct {
	ContactNo string
	Topic     Topic
	Channel   Channel
	Enabled   bool
}

// MarketingConsentChange records the proof behind one marketing unsubscribe
// or restore action. Raw provider/browser evidence remains notify-owned.
type MarketingConsentChange struct {
	ContactNo       string
	Channel         Channel
	Action          string
	PolicyReference string
	PolicyVersion   string
	Locale          string
	EvidenceDigest  string
}

// QuietHoursChange replaces the customer-local marketing quiet-hours window.
type QuietHoursChange struct {
	TimezoneName string
	StartMinute  int
	EndMinute    int
	Enabled      bool
}

// PreferenceCommand is bound to the authenticated current customer by core.
// CustomerNo is deliberately absent so notify cannot mutate another customer.
type PreferenceCommand struct {
	IdempotencyKey   string
	ExpectedVersion  int64
	Changes          []PreferenceChange
	MarketingConsent *MarketingConsentChange
	QuietHours       *QuietHoursChange
}

// PreferenceResult is the aggregate optimistic version and canonical evidence.
type PreferenceResult struct {
	CustomerNo     string
	Version        int64
	EvidenceDigest string
	AppliedAt      time.Time
	Reused         bool
}

// PreferenceProjection is one explicit current contact/topic/channel choice.
// Missing combinations remain absent so callers cannot mistake a guessed
// default for a customer decision.
type PreferenceProjection struct {
	ContactNo string
	Topic     Topic
	Channel   Channel
	Enabled   bool
	Version   int64
}

// MarketingConsentProjection is the current explicit marketing decision for
// one contact and channel. Missing combinations are fail-closed.
type MarketingConsentProjection struct {
	ContactNo       string
	Channel         Channel
	Status          string
	PolicyReference string
	PolicyVersion   string
	Locale          string
	Version         int64
}

// QuietHoursProjection is the current customer-local marketing window.
type QuietHoursProjection struct {
	TimezoneName    string
	StartMinute     int
	EndMinute       int
	CrossesMidnight bool
	Enabled         bool
	Version         int64
}

// PreferenceSnapshot is bounded to the authenticated current customer. It
// contains no delivery address, contact name, or arbitrary-customer lookup.
type PreferenceSnapshot struct {
	CustomerNo        string
	PreferredLocale   string
	Version           int64
	Preferences       []PreferenceProjection
	MarketingConsents []MarketingConsentProjection
	QuietHours        *QuietHoursProjection
}

// PreferenceService exposes one current-customer read and command. CustomerNo
// is deliberately absent from both methods.
type PreferenceService interface {
	Current(context.Context) (*PreferenceSnapshot, error)
	UpdateCurrent(context.Context, PreferenceCommand) (*PreferenceResult, error)
}

// Runtime is the complete source-only boundary composed for notify.
type Runtime struct {
	Audience        CurrentAudience
	Events          EventFeed
	Contacts        ContactResolver
	BusinessObjects BusinessObjectAccess
	Marketing       MarketingSendGate
	Preferences     PreferenceService
}

func ValidateRuntime(runtime Runtime) error {
	if nilDependency(runtime.Audience) || nilDependency(runtime.Events) || nilDependency(runtime.Contacts) || nilDependency(runtime.BusinessObjects) ||
		nilDependency(runtime.Marketing) || nilDependency(runtime.Preferences) {
		return gerror.New("notification source runtime is incomplete")
	}
	return nil
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
