// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// SalesCommissionSettlementReview is the golang structure of table plugin_idc_core_sales_commission_settlement_review for DAO operations like Where/Data.
type SalesCommissionSettlementReview struct {
	g.Meta         `orm:"table:plugin_idc_core_sales_commission_settlement_review, do:true"`
	Id             any        //
	TenantId       any        //
	ReviewNo       any        //
	SettlementNo   any        //
	DecisionCode   any        //
	ReasonCode     any        //
	ReviewerUserId any        //
	EvidenceDigest any        //
	ReviewedAt     *time.Time //
}
