// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// SalesCommissionPayoutEvidence is the golang structure of table plugin_idc_core_sales_commission_payout_evidence for DAO operations like Where/Data.
type SalesCommissionPayoutEvidence struct {
	g.Meta                   `orm:"table:plugin_idc_core_sales_commission_payout_evidence, do:true"`
	Id                       any        //
	TenantId                 any        //
	EvidenceNo               any        //
	PayoutItemNo             any        //
	EvidenceTypeCode         any        //
	ProtectedObjectReference *string    //
	EvidenceDigest           any        //
	ProviderReferenceDigest  *string    //
	ObservedAt               *time.Time //
}
