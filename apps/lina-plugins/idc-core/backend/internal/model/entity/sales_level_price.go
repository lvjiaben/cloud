// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// SalesLevelPrice is the golang structure for table sales_level_price.
type SalesLevelPrice struct {
	Id                    int64      `json:"id"                    orm:"id"                      description:""`
	TenantId              int        `json:"tenantId"              orm:"tenant_id"               description:""`
	PriceNo               string     `json:"priceNo"               orm:"price_no"                description:""`
	LevelNo               string     `json:"levelNo"               orm:"level_no"                description:""`
	PercentageBasisPoints int        `json:"percentageBasisPoints" orm:"percentage_basis_points" description:""`
	StatusCode            string     `json:"statusCode"            orm:"status_code"             description:""`
	Version               int64      `json:"version"               orm:"version"                 description:""`
	ContentDigest         string     `json:"contentDigest"         orm:"content_digest"          description:""`
	CreatedAt             *time.Time `json:"createdAt"             orm:"created_at"              description:""`
}
