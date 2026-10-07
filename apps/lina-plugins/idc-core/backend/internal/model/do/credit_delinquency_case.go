// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// CreditDelinquencyCase is the golang structure of table plugin_idc_core_credit_delinquency_case for DAO operations like Where/Data.
type CreditDelinquencyCase struct {
	g.Meta                   `orm:"table:plugin_idc_core_credit_delinquency_case, do:true"`
	Id                       any        //
	TenantId                 any        //
	CaseNo                   any        //
	StatementNo              any        //
	CreditAccountNo          any        //
	CustomerNo               any        //
	CurrencyCode             any        //
	TermsNo                  any        //
	TermsVersion             any        //
	StatusCode               any        //
	PrincipalMinor           any        //
	LateFeeMinor             any        //
	OutstandingAmountMinor   any        //
	OpenedAt                 *time.Time //
	GraceUntil               *time.Time //
	FrozenAt                 *time.Time //
	ServiceActionRequestedAt *time.Time //
	WrittenOffAt             *time.Time //
	ResolvedAt               *time.Time //
	ReasonCode               any        //
	SourceSnapshotDigest     any        //
	Version                  any        //
	CreatedAt                *time.Time //
	UpdatedAt                *time.Time //
}
