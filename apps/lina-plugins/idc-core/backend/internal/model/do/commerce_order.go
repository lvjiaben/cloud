// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// CommerceOrder is the golang structure of table plugin_idc_core_commerce_order for DAO operations like Where/Data.
type CommerceOrder struct {
	g.Meta                       `orm:"table:plugin_idc_core_commerce_order, do:true"`
	Id                           any        //
	TenantId                     any        //
	OrderNo                      any        //
	CustomerNo                   any        //
	QuoteNo                      any        //
	CurrencyCode                 any        //
	SourceCode                   any        //
	StatusCode                   any        //
	RiskDecisionCode             any        //
	RequiresContract             any        //
	ItemCount                    any        //
	SubtotalMinor                any        //
	DiscountMinor                any        //
	NetAmountMinor               any        //
	TaxAmountMinor               any        //
	TotalAmountMinor             any        //
	CheckoutIdempotencyDigest    any        //
	CheckoutRequestDigest        any        //
	PaymentDueAt                 *time.Time //
	PaidAt                       *time.Time //
	CompletedAt                  *time.Time //
	CancelledAt                  *time.Time //
	Version                      any        //
	CreatedBy                    any        //
	CreatedAt                    *time.Time //
	UpdatedAt                    *time.Time //
	AssistedRequestNo            *string    //
	PaymentTermCode              any        //
	ContractOverridePolicyCode   any        // Order choice preserved separately: inherit product, explicit none, or required
	ContractOverrideTemplateNo   any        //
	EnterpriseContractPolicyCode any        //
	EnterpriseContractTemplateNo any        //
	ContractDecisionSourceCode   any        // Winning rule source after order > product > enterprise > default resolution
	ContractTemplateNo           any        //
	ContractTemplateVersionNo    any        //
	ContractTemplateName         any        //
	ContractLanguageCode         any        //
	ContractScopeCode            any        //
	ContractSigningOrderCode     any        //
}
