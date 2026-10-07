// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// MarketingSegment is the golang structure for table marketing_segment.
type MarketingSegment struct {
	Id                     int64      `json:"id"                     orm:"id"                       description:""`
	TenantId               int        `json:"tenantId"               orm:"tenant_id"                description:""`
	SegmentNo              string     `json:"segmentNo"              orm:"segment_no"               description:""`
	SegmentCode            string     `json:"segmentCode"            orm:"segment_code"             description:""`
	NameZhCn               string     `json:"nameZhCn"               orm:"name_zh_cn"               description:""`
	NameEnUs               string     `json:"nameEnUs"               orm:"name_en_us"               description:""`
	DescriptionZhCn        string     `json:"descriptionZhCn"        orm:"description_zh_cn"        description:""`
	DescriptionEnUs        string     `json:"descriptionEnUs"        orm:"description_en_us"        description:""`
	ModeCode               string     `json:"modeCode"               orm:"mode_code"                description:""`
	RefreshIntervalMinutes *int       `json:"refreshIntervalMinutes" orm:"refresh_interval_minutes" description:""`
	DefinitionDigest       string     `json:"definitionDigest"       orm:"definition_digest"        description:""`
	StatusCode             string     `json:"statusCode"             orm:"status_code"              description:""`
	Version                int64      `json:"version"                orm:"version"                  description:""`
	CreatedBy              int        `json:"createdBy"              orm:"created_by"               description:""`
	UpdatedBy              int        `json:"updatedBy"              orm:"updated_by"               description:""`
	CreatedAt              *time.Time `json:"createdAt"              orm:"created_at"               description:""`
	UpdatedAt              *time.Time `json:"updatedAt"              orm:"updated_at"               description:""`
}
