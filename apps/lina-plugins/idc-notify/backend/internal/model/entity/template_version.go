// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// TemplateVersion is the golang structure for table template_version.
type TemplateVersion struct {
	TenantId                  int        `json:"tenantId"                  orm:"tenant_id"                    description:""`
	TemplateNo                string     `json:"templateNo"                orm:"template_no"                  description:""`
	Version                   int64      `json:"version"                   orm:"version"                      description:""`
	EventCode                 string     `json:"eventCode"                 orm:"event_code"                   description:""`
	ChannelCode               string     `json:"channelCode"               orm:"channel_code"                 description:""`
	LocaleCode                string     `json:"localeCode"                orm:"locale_code"                  description:""`
	SubjectTemplate           string     `json:"subjectTemplate"           orm:"subject_template"             description:""`
	BodyTemplate              string     `json:"bodyTemplate"              orm:"body_template"                description:""`
	ProviderTemplateId        string     `json:"providerTemplateId"        orm:"provider_template_id"         description:""`
	ProviderSmsVariableOrder  string     `json:"providerSmsVariableOrder"  orm:"provider_sms_variable_order"  description:""`
	ProviderWechatDataMapping string     `json:"providerWechatDataMapping" orm:"provider_wechat_data_mapping" description:""`
	StatusCode                string     `json:"statusCode"                orm:"status_code"                  description:""`
	ContentDigest             string     `json:"contentDigest"             orm:"content_digest"               description:""`
	CreatedBy                 int64      `json:"createdBy"                 orm:"created_by"                   description:""`
	CreatedAt                 *time.Time `json:"createdAt"                 orm:"created_at"                   description:""`
	ProviderCode              string     `json:"providerCode"              orm:"provider_code"                description:""`
	ProviderSmsDataMapping    string     `json:"providerSmsDataMapping"    orm:"provider_sms_data_mapping"    description:""`
}
