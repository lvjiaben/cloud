// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// CommerceAssistedQuoteRequest is the golang structure of table plugin_idc_core_commerce_assisted_quote_request for DAO operations like Where/Data.
type CommerceAssistedQuoteRequest struct {
	g.Meta                     `orm:"table:plugin_idc_core_commerce_assisted_quote_request, do:true"`
	Id                         any        //
	TenantId                   any        //
	RequestNo                  any        //
	CustomerNo                 any        //
	CurrencyCode               any        //
	StatusCode                 any        //
	ItemCount                  any        //
	ManualDiscountMinor        any        //
	PaymentTermCode            any        //
	PaymentDueSeconds          any        //
	Reason                     any        //
	InternalNote               any        //
	IdempotencyDigest          any        //
	RequestDigest              any        //
	ExpiresAt                  *time.Time //
	RequestedBy                any        //
	DecisionCode               *string    //
	DecisionBy                 *int       //
	DecisionAt                 *time.Time //
	DecisionReason             *string    //
	DecisionRequestDigest      *string    //
	DecisionEvidenceDigest     *string    //
	QuoteNo                    *string    //
	OrderNo                    *string    //
	Version                    any        //
	CreatedAt                  *time.Time //
	UpdatedAt                  *time.Time //
	ContractOverridePolicyCode any        // Administrator order choice captured before assisted checkout
	ContractOverrideTemplateNo any        //
}
