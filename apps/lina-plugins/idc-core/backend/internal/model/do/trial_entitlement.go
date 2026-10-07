// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// TrialEntitlement is the golang structure of table plugin_idc_core_trial_entitlement for DAO operations like Where/Data.
type TrialEntitlement struct {
	g.Meta                 `orm:"table:plugin_idc_core_trial_entitlement, do:true"`
	Id                     any        //
	TenantId               any        //
	EntitlementNo          any        //
	ScopeNo                any        //
	CustomerNo             any        //
	ProductNo              any        //
	VersionNo              any        //
	BillingRuleNo          any        //
	OrderNo                any        //
	InvoiceNo              any        //
	OrderItemNo            any        //
	CurrencyCode           any        //
	TrialLimitQuantity     any        //
	StatusCode             any        //
	HoldExpiresAt          *time.Time //
	ConsumedAt             *time.Time //
	ReleasedAt             *time.Time //
	IdentityEvidenceDigest *string    //
	ContentDigest          any        //
	Version                any        //
	CreatedAt              *time.Time //
	UpdatedAt              *time.Time //
	UnitSequence           any        // Generic one-based quantity unit inside the immutable order item; it has no server meaning.
}
