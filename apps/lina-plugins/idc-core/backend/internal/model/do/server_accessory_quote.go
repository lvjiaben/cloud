// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// ServerAccessoryQuote is the golang structure of table plugin_idc_core_server_accessory_quote for DAO operations like Where/Data.
type ServerAccessoryQuote struct {
	g.Meta                 `orm:"table:plugin_idc_core_server_accessory_quote, do:true"`
	Id                     any        //
	TenantId               any        //
	QuoteNo                any        //
	CustomerNo             any        //
	ServiceNo              any        //
	ActionCode             any        //
	AccessoryKind          any        //
	CurrencyCode           any        //
	RequestedQuantity      any        //
	ExistingObjectCount    any        //
	ReservedCreateCount    any        //
	FreeCountPerServer     any        //
	ChargedQuantity        any        //
	UnitAmountMinor        any        //
	TotalAmountMinor       any        //
	PeriodUnitCode         any        //
	PeriodCount            any        //
	PolicyNo               any        //
	PolicyVersion          any        //
	IdempotencyDigest      any        //
	RequestDigest          any        //
	ContentDigest          any        //
	IssuedAt               *time.Time //
	ExpiresAt              *time.Time //
	Parameters             any        // Canonical provider parameters frozen by the customer-visible quote
	CustomerSubjectVersion any        //
	ExpectedServiceVersion any        //
	SubtotalMinor          any        //
	NetAmountMinor         any        //
	TaxAmountMinor         any        //
	TaxPolicyNo            any        //
	TaxTreatmentCode       any        //
	TaxRateBasisPoints     any        //
	TaxCategoryCode        any        //
	RoundingScopeCode      any        //
	RoundingModeCode       any        //
	RoundingQuantumMinor   any        //
	BaselineObjectNos      any        //
	BaselineSnapshotNo     any        //
	BaselineObservedAt     *time.Time //
	SubjectTypeCode        any        //
	SubjectNo              any        //
	ExpectedSubjectVersion any        //
	NativeTargetSnapshot   any        // Source-validated frozen native resource route, directory and local server evidence; never accepted from HTTP
}
