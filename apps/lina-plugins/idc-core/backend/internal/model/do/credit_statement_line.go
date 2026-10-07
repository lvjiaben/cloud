// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// CreditStatementLine is the golang structure of table plugin_idc_core_credit_statement_line for DAO operations like Where/Data.
type CreditStatementLine struct {
	g.Meta          `orm:"table:plugin_idc_core_credit_statement_line, do:true"`
	Id              any        //
	TenantId        any        //
	StatementLineNo any        //
	StatementNo     any        //
	CreditAccountNo any        //
	CustomerNo      any        //
	CurrencyCode    any        //
	LineNumber      any        //
	LineTypeCode    any        //
	ExposureNo      any        //
	SourceTypeCode  any        //
	SourceReference any        //
	AmountMinor     any        //
	ContentDigest   any        //
	CreatedAt       *time.Time //
}
