// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// FinancialInternalNote is the golang structure of table plugin_idc_core_financial_internal_note for DAO operations like Where/Data.
type FinancialInternalNote struct {
	g.Meta            `orm:"table:plugin_idc_core_financial_internal_note, do:true"`
	Id                any        //
	TenantId          any        //
	NoteNo            any        //
	TargetTypeCode    any        //
	TargetNo          any        //
	CustomerNo        any        //
	Content           any        //
	CorrectsNoteNo    any        //
	CreatedBy         any        //
	IdempotencyDigest any        //
	RequestDigest     any        //
	CreatedAt         *time.Time //
}
