// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// RedLetterCandidate is the golang structure of table plugin_idc_compliance_red_letter_candidate for DAO operations like Where/Data.
type RedLetterCandidate struct {
	g.Meta                 `orm:"table:plugin_idc_compliance_red_letter_candidate, do:true"`
	Id                     any        //
	TenantId               any        //
	CandidateNo            any        //
	TaskNo                 any        //
	RefundAllocationNo     any        //
	TaxInvoiceNo           any        //
	TaxInvoiceAllocationNo any        //
	TaxInvoiceItemNo       any        //
	RequestedGrossMinor    any        //
	AvailableGrossMinor    any        //
	AvailableNetMinor      any        //
	AvailableTaxMinor      any        //
	CreatedAt              *time.Time //
}
