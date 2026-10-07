// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// CreditApplicationReview is the golang structure of table plugin_idc_core_credit_application_review for DAO operations like Where/Data.
type CreditApplicationReview struct {
	g.Meta         `orm:"table:plugin_idc_core_credit_application_review, do:true"`
	Id             any        //
	TenantId       any        //
	ReviewNo       any        //
	ApplicationNo  any        //
	CustomerNo     any        //
	CurrencyCode   any        //
	ReviewSequence any        //
	DecisionCode   any        //
	ReviewerId     any        //
	CommentDigest  any        //
	EvidenceDigest any        //
	ContentDigest  any        //
	CreatedAt      *time.Time //
}
