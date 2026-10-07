// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// CommerceOrderReview is the golang structure of table plugin_idc_core_commerce_order_review for DAO operations like Where/Data.
type CommerceOrderReview struct {
	g.Meta         `orm:"table:plugin_idc_core_commerce_order_review, do:true"`
	Id             any        //
	TenantId       any        //
	ReviewNo       any        //
	OrderNo        any        //
	CustomerNo     any        //
	ReviewTypeCode any        //
	StatusCode     any        //
	ReasonCode     any        //
	RequestDigest  *string    //
	EvidenceDigest *string    //
	ReviewedBy     *int       //
	RequestedAt    *time.Time //
	ReviewedAt     *time.Time //
	CreatedAt      *time.Time //
	UpdatedAt      *time.Time //
}
