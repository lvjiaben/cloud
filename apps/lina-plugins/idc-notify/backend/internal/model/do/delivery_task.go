// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// DeliveryTask is the golang structure of table plugin_idc_notify_delivery_task for DAO operations like Where/Data.
type DeliveryTask struct {
	g.Meta                      `orm:"table:plugin_idc_notify_delivery_task, do:true"`
	TenantId                    any        //
	TaskNo                      any        //
	ChainNo                     any        //
	PreviousTaskNo              any        //
	EventCode                   any        //
	TopicCode                   any        //
	ChannelCode                 any        //
	TemplateNo                  any        //
	TemplateVersion             any        //
	LocaleCode                  any        //
	OwnerUserId                 any        //
	CustomerNo                  any        //
	ContactNo                   any        //
	RecipientMask               any        //
	BusinessObjectType          any        //
	BusinessObjectNo            any        //
	BusinessObjectVersion       any        //
	MarketingSnapshotNo         any        //
	MarketingDecisionDigest     any        //
	MarketingConsentEventNo     any        //
	MarketingConsentVersion     any        //
	MarketingPreferenceVersion  any        //
	MarketingQuietHoursVersion  any        //
	MarketingEvaluatedAt        any        //
	SubjectContent              any        //
	BodyContent                 any        //
	ProviderVariables           any        //
	ContentDigest               any        //
	StatusCode                  any        //
	ResultClass                 any        //
	ExternalBoundaryCrossed     any        //
	ProviderMessageRef          any        //
	ProviderQueryDeadline       any        //
	ProviderQueryDeadlineSource any        //
	RetrySequence               any        //
	NextAttemptAt               any        //
	AttemptLease                any        //
	AttemptLeaseUntil           any        //
	Version                     any        //
	QueuedAt                    *time.Time //
	CompletedAt                 any        //
	UpdatedAt                   *time.Time //
}
