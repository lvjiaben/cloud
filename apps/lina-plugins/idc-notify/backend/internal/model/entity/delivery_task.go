// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// DeliveryTask is the golang structure for table delivery_task.
type DeliveryTask struct {
	TenantId                    int        `json:"tenantId"                    orm:"tenant_id"                      description:""`
	TaskNo                      string     `json:"taskNo"                      orm:"task_no"                        description:""`
	ChainNo                     string     `json:"chainNo"                     orm:"chain_no"                       description:""`
	PreviousTaskNo              any        `json:"previousTaskNo"              orm:"previous_task_no"               description:""`
	EventCode                   string     `json:"eventCode"                   orm:"event_code"                     description:""`
	TopicCode                   string     `json:"topicCode"                   orm:"topic_code"                     description:""`
	ChannelCode                 string     `json:"channelCode"                 orm:"channel_code"                   description:""`
	TemplateNo                  any        `json:"templateNo"                  orm:"template_no"                    description:""`
	TemplateVersion             any        `json:"templateVersion"             orm:"template_version"               description:""`
	LocaleCode                  string     `json:"localeCode"                  orm:"locale_code"                    description:""`
	OwnerUserId                 any        `json:"ownerUserId"                 orm:"owner_user_id"                  description:""`
	CustomerNo                  string     `json:"customerNo"                  orm:"customer_no"                    description:""`
	ContactNo                   string     `json:"contactNo"                   orm:"contact_no"                     description:""`
	RecipientMask               string     `json:"recipientMask"               orm:"recipient_mask"                 description:""`
	BusinessObjectType          string     `json:"businessObjectType"          orm:"business_object_type"           description:""`
	BusinessObjectNo            string     `json:"businessObjectNo"            orm:"business_object_no"             description:""`
	BusinessObjectVersion       int64      `json:"businessObjectVersion"       orm:"business_object_version"        description:""`
	MarketingSnapshotNo         string     `json:"marketingSnapshotNo"         orm:"marketing_snapshot_no"          description:""`
	MarketingDecisionDigest     any        `json:"marketingDecisionDigest"     orm:"marketing_decision_digest"      description:""`
	MarketingConsentEventNo     any        `json:"marketingConsentEventNo"     orm:"marketing_consent_event_no"     description:""`
	MarketingConsentVersion     any        `json:"marketingConsentVersion"     orm:"marketing_consent_version"      description:""`
	MarketingPreferenceVersion  any        `json:"marketingPreferenceVersion"  orm:"marketing_preference_version"   description:""`
	MarketingQuietHoursVersion  any        `json:"marketingQuietHoursVersion"  orm:"marketing_quiet_hours_version"  description:""`
	MarketingEvaluatedAt        any        `json:"marketingEvaluatedAt"        orm:"marketing_evaluated_at"         description:""`
	SubjectContent              string     `json:"subjectContent"              orm:"subject_content"                description:""`
	BodyContent                 string     `json:"bodyContent"                 orm:"body_content"                   description:""`
	ProviderVariables           string     `json:"providerVariables"           orm:"provider_variables"             description:""`
	ContentDigest               string     `json:"contentDigest"               orm:"content_digest"                 description:""`
	StatusCode                  string     `json:"statusCode"                  orm:"status_code"                    description:""`
	ResultClass                 string     `json:"resultClass"                 orm:"result_class"                   description:""`
	ExternalBoundaryCrossed     bool       `json:"externalBoundaryCrossed"     orm:"external_boundary_crossed"      description:""`
	ProviderMessageRef          any        `json:"providerMessageRef"          orm:"provider_message_ref"           description:""`
	ProviderQueryDeadline       any        `json:"providerQueryDeadline"       orm:"provider_query_deadline"        description:""`
	ProviderQueryDeadlineSource string     `json:"providerQueryDeadlineSource" orm:"provider_query_deadline_source" description:""`
	RetrySequence               int        `json:"retrySequence"               orm:"retry_sequence"                 description:""`
	NextAttemptAt               any        `json:"nextAttemptAt"               orm:"next_attempt_at"                description:""`
	AttemptLease                any        `json:"attemptLease"                orm:"attempt_lease"                  description:""`
	AttemptLeaseUntil           any        `json:"attemptLeaseUntil"           orm:"attempt_lease_until"            description:""`
	Version                     int64      `json:"version"                     orm:"version"                        description:""`
	QueuedAt                    *time.Time `json:"queuedAt"                    orm:"queued_at"                      description:""`
	CompletedAt                 any        `json:"completedAt"                 orm:"completed_at"                   description:""`
	UpdatedAt                   *time.Time `json:"updatedAt"                   orm:"updated_at"                     description:""`
}
