// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// CreditAccountMovement is the golang structure of table plugin_idc_core_credit_account_movement for DAO operations like Where/Data.
type CreditAccountMovement struct {
	g.Meta                `orm:"table:plugin_idc_core_credit_account_movement, do:true"`
	Id                    any        //
	TenantId              any        //
	MovementNo            any        //
	CreditAccountNo       any        //
	CustomerNo            any        //
	CurrencyCode          any        //
	ExposureNo            *string    //
	StatementNo           *string    //
	RepaymentNo           *string    //
	MovementTypeCode      any        //
	AmountMinor           any        //
	AccountVersionBefore  any        //
	AccountVersionAfter   any        //
	StatusBeforeCode      *string    //
	StatusAfterCode       any        //
	TotalLimitBeforeMinor any        //
	TotalLimitAfterMinor  any        //
	AvailableBeforeMinor  any        //
	AvailableAfterMinor   any        //
	UnbilledBeforeMinor   any        //
	UnbilledAfterMinor    any        //
	BilledBeforeMinor     any        //
	BilledAfterMinor      any        //
	DelinquentBeforeMinor any        //
	DelinquentAfterMinor  any        //
	WrittenOffBeforeMinor any        //
	WrittenOffAfterMinor  any        //
	SourceTypeCode        any        //
	SourceReference       any        //
	IdempotencyDigest     any        //
	EvidenceDigest        any        //
	ContentDigest         any        //
	ActorTypeCode         any        //
	ActorId               any        //
	OccurredAt            *time.Time //
	CreatedAt             *time.Time //
}
