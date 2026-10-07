// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// CustomerSubject is the golang structure for table customer_subject.
type CustomerSubject struct {
	Id                           int64      `json:"id"                           orm:"id"                              description:""`
	TenantId                     int        `json:"tenantId"                     orm:"tenant_id"                       description:""`
	CustomerNo                   string     `json:"customerNo"                   orm:"customer_no"                     description:"Opaque public customer business number"`
	CustomerTypeCode             string     `json:"customerTypeCode"             orm:"customer_type_code"              description:"Customer type code: individual or enterprise"`
	StatusCode                   string     `json:"statusCode"                   orm:"status_code"                     description:"Customer lifecycle status code"`
	DisplayName                  string     `json:"displayName"                  orm:"display_name"                    description:"Customer display name without verified legal identity evidence"`
	CountryCode                  string     `json:"countryCode"                  orm:"country_code"                    description:""`
	PreferredLocale              string     `json:"preferredLocale"              orm:"preferred_locale"                description:"Preferred IETF locale tag"`
	Version                      int64      `json:"version"                      orm:"version"                         description:""`
	AnonymizedAt                 *time.Time `json:"anonymizedAt"                 orm:"anonymized_at"                   description:"Time when approved account closure anonymization completed"`
	CreatedBy                    int        `json:"createdBy"                    orm:"created_by"                      description:""`
	UpdatedBy                    int        `json:"updatedBy"                    orm:"updated_by"                      description:""`
	CreatedAt                    *time.Time `json:"createdAt"                    orm:"created_at"                      description:""`
	UpdatedAt                    *time.Time `json:"updatedAt"                    orm:"updated_at"                      description:""`
	SearchName                   string     `json:"searchName"                   orm:"search_name"                     description:"Service-normalized display-name projection for indexed prefix search"`
	StatusReasonCode             string     `json:"statusReasonCode"             orm:"status_reason_code"              description:"Stable reason code for the current account restriction or lifecycle state"`
	StatusReason                 string     `json:"statusReason"                 orm:"status_reason"                   description:"Bounded operator explanation presented through localized business copy"`
	StatusChangedBy              int        `json:"statusChangedBy"              orm:"status_changed_by"               description:""`
	StatusChangedAt              *time.Time `json:"statusChangedAt"              orm:"status_changed_at"               description:""`
	EnterpriseContractPolicyCode string     `json:"enterpriseContractPolicyCode" orm:"enterprise_contract_policy_code" description:"Enterprise default: none or required; it affects only orders created after the setting changes"`
	EnterpriseContractTemplateNo string     `json:"enterpriseContractTemplateNo" orm:"enterprise_contract_template_no" description:""`
	CustomerCode                 int64      `json:"customerCode"                 orm:"customer_code"                   description:"Short monotonic customer number shown to staff and customers; starts at 10000 and is never reused"`
}
