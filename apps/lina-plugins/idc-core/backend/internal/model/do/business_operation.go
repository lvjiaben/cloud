// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// BusinessOperation is the golang structure of table plugin_idc_core_business_operation for DAO operations like Where/Data.
type BusinessOperation struct {
	g.Meta                      `orm:"table:plugin_idc_core_business_operation, do:true"`
	Id                          any        //
	TenantId                    any        //
	OperationNo                 any        //
	OperationTypeCode           any        //
	CustomerNo                  any        //
	ServiceNo                   any        //
	CurrencyCode                any        //
	PriceSnapshotNo             *string    //
	QuoteNo                     *string    //
	OrderNo                     *string    //
	InvoiceNo                   *string    //
	StatusCode                  any        //
	FinancialSettlementRequired any        //
	ExecutionModeCode           any        //
	ResourceOwnerCode           any        //
	ChannelNo                   *string    //
	PoolNo                      any        //
	ProviderTaskNo              *string    //
	CapabilityEvidenceNo        *string    //
	CapabilityEvidenceDigest    *string    //
	IdempotencyDigest           any        //
	RequestDigest               any        //
	ContentDigest               any        //
	ExpectedServiceVersion      any        //
	LastErrorCode               any        //
	FinanciallySettledAt        *time.Time //
	ExecutionStartedAt          *time.Time //
	CompletedAt                 *time.Time //
	Version                     any        //
	CreatedAt                   *time.Time //
	UpdatedAt                   *time.Time //
	SourceFulfillmentNo         *string    // Immutable new-purchase server fulfillment identity
	SettlementNo                *string    // Unified financial settlement authority for a paid new-purchase operation
	SubjectTypeCode             any        //
	SubjectNo                   any        //
	ExpectedSubjectVersion      any        //
}
