// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// ServerAccessoryPricePolicyVersion is the golang structure for table server_accessory_price_policy_version.
type ServerAccessoryPricePolicyVersion struct {
	Id                 int64      `json:"id"                 orm:"id"                    description:""`
	TenantId           int        `json:"tenantId"           orm:"tenant_id"             description:""`
	PolicyNo           string     `json:"policyNo"           orm:"policy_no"             description:""`
	PolicyVersion      int64      `json:"policyVersion"      orm:"policy_version"        description:""`
	AccessoryKind      string     `json:"accessoryKind"      orm:"accessory_kind"        description:""`
	CurrencyCode       string     `json:"currencyCode"       orm:"currency_code"         description:""`
	FreeCountPerServer int        `json:"freeCountPerServer" orm:"free_count_per_server" description:""`
	UnitAmountMinor    int64      `json:"unitAmountMinor"    orm:"unit_amount_minor"     description:""`
	PeriodUnitCode     string     `json:"periodUnitCode"     orm:"period_unit_code"      description:""`
	PeriodCount        int        `json:"periodCount"        orm:"period_count"          description:""`
	ContentDigest      string     `json:"contentDigest"      orm:"content_digest"        description:""`
	PublishedBy        int        `json:"publishedBy"        orm:"published_by"          description:""`
	PublishedAt        *time.Time `json:"publishedAt"        orm:"published_at"          description:""`
	TrafficGb          int        `json:"trafficGb"          orm:"traffic_gb"            description:""`
	NameZhCn           string     `json:"nameZhCn"           orm:"name_zh_cn"            description:""`
	NameEnUs           string     `json:"nameEnUs"           orm:"name_en_us"            description:""`
}
