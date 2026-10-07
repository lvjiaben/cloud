// This file defines current-customer refund preview, submission, consent, and response DTOs.

package v1

import "github.com/gogf/gf/v2/frame/g"

type RefundSelectionInput struct {
	InvoiceItemNo  string   `json:"invoiceItemNo" v:"required|max-length:40" dc:"Selected invoice item owned by the current customer" eg:"BII_01J2ABCDEF"`
	EntitlementNos []string `json:"entitlementNos" dc:"Optional exact long-lived entitlements affected by this order-item refund; the refund quantity remains an independent order-item fact" eg:"[]"`
	Quantity       int      `json:"quantity" v:"required|min:1|max:1000" dc:"Refund quantity from this order item" eg:"1"`
	AmountMinor    string   `json:"amountMinor" v:"required|regex:^[1-9][0-9]{0,18}$" dc:"Exact positive refund amount in the invoice currency smallest unit" eg:"1299"`
}

type RefundPreviewCreateReq struct {
	g.Meta     `path:"/account/refunds/preview" method:"post" tags:"IDC Customer Refund" summary:"Preview my refund" dc:"Re-read the current customer's invoice, funding, order-item quantity, and optional entitlement facts and return one immutable refund impact preview. Customer identity is never accepted from the request."`
	OrderNo    string                  `json:"orderNo" v:"required|max-length:40" dc:"Current-customer order number" eg:"ORD_01J2ABCDEF"`
	Selections []*RefundSelectionInput `json:"selections" v:"required" dc:"One to one hundred non-overlapping order-item refund selections with optional exact long-lived entitlements; collection bounds are enforced by the refund service" eg:"[]"`
}

type RefundPreviewCreateRes struct {
	Preview *RefundPreview `json:"preview" dc:"Immutable amount, review, and entitlement-impact preview" eg:"{}"`
}

type RefundCreateReq struct {
	g.Meta         `path:"/account/refunds" method:"post" tags:"IDC Customer Refund" summary:"Submit my refund" dc:"Submit the exact preview after re-reading current financial, order-item, and optional entitlement facts. Other reason requires a detail."`
	OrderNo        string                  `json:"orderNo" v:"required|max-length:40" dc:"Current-customer order number" eg:"ORD_01J2ABCDEF"`
	Selections     []*RefundSelectionInput `json:"selections" v:"required" dc:"Selections identical to the accepted preview; collection bounds are enforced by the refund service" eg:"[]"`
	PreviewDigest  string                  `json:"previewDigest" v:"required|regex:^[0-9a-f]{64}$" dc:"Digest returned by the latest accepted preview" eg:"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"`
	ReasonCode     string                  `json:"reasonCode" v:"required|in:duplicate_purchase,wrong_configuration,service_issue,no_longer_needed,other" dc:"Fixed customer refund reason" eg:"wrong_configuration"`
	ReasonDetail   string                  `json:"reasonDetail" v:"max-length:1000" dc:"Required when reasonCode is other; optional otherwise" eg:"This item is no longer needed"`
	IdempotencyKey string                  `json:"idempotencyKey" v:"required|min-length:8|max-length:128" dc:"Random retry key; the raw value is never persisted" eg:"refund-01J2ABCDEF"`
}

type RefundCreateRes struct {
	Refund *Refund `json:"refund" dc:"Created or safely replayed customer refund" eg:"{}"`
}

type RefundBalanceConsentCreateReq struct {
	g.Meta          `path:"/account/refunds/{refundNo}/balance-consent" method:"post" tags:"IDC Customer Refund" summary:"Respond to my balance refund choice" dc:"Keep the entire refund pending when declined, or atomically release the whole mixed-source refund after accepting same-currency balance only for original sources proved unsupported."`
	RefundNo        string `json:"refundNo" in:"path" v:"required|max-length:40" dc:"Current-customer refund number waiting for this choice" eg:"rfd_01J2ABCDEF"`
	ExpectedVersion int64  `json:"expectedVersion" v:"required|min:1" dc:"Refund version shown with the choice" eg:"2"`
	Consent         bool   `json:"consent" dc:"Accept same-currency balance only for the unsupported original-source portion; false records refusal and keeps the whole refund pending" eg:"true"`
	IdempotencyKey  string `json:"idempotencyKey" v:"required|min-length:8|max-length:128" dc:"Random retry key; the raw value is never persisted" eg:"refund-consent-01J2ABCDEF"`
}

type RefundBalanceConsentCreateRes struct {
	Refund *Refund `json:"refund" dc:"Pending refusal or atomically released refund result" eg:"{}"`
}

type RefundPreview struct {
	OrderNo                        string               `json:"orderNo" dc:"Order covered by the refund" eg:"ORD_01J2ABCDEF"`
	InvoiceNo                      string               `json:"invoiceNo" dc:"Invoice covered by the refund" eg:"BIL_01J2ABCDEF"`
	CurrencyCode                   string               `json:"currencyCode" dc:"Exact refund currency" eg:"CNY"`
	AmountMinor                    string               `json:"amountMinor" dc:"Exact total in currency smallest units" eg:"1299"`
	ApprovalModeCode               string               `json:"approvalModeCode" dc:"Review mode: automatic or manual" eg:"manual"`
	RequiresEntitlementTermination bool                 `json:"requiresEntitlementTermination" dc:"Whether execution must terminate a selected long-lived entitlement" eg:"true"`
	HasEntitlementImpact           bool                 `json:"hasEntitlementImpact" dc:"Whether a selected long-lived entitlement is affected" eg:"true"`
	HasDangerousOperation          bool                 `json:"hasDangerousOperation" dc:"Whether a dangerous or unknown-result operation blocks safe execution" eg:"false"`
	HighAmount                     bool                 `json:"highAmount" dc:"Whether the amount reaches the current-currency high-amount threshold" eg:"false"`
	ImpactDigest                   string               `json:"impactDigest" dc:"Immutable digest binding the accepted preview" eg:"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"`
	Items                          []*RefundPreviewItem `json:"items" dc:"One to one hundred selected items with frozen impact evidence" eg:"[]"`
}

type RefundPreviewItem struct {
	InvoiceItemNo         string                     `json:"invoiceItemNo" dc:"Selected invoice item" eg:"BII_01J2ABCDEF"`
	OrderItemNo           string                     `json:"orderItemNo" dc:"Original order item" eg:"ORI_01J2ABCDEF"`
	Quantity              int                        `json:"quantity" dc:"Frozen refund quantity" eg:"1"`
	AmountMinor           string                     `json:"amountMinor" dc:"Requested amount in currency smallest units" eg:"1299"`
	RefundableAmountMinor string                     `json:"refundableAmountMinor" dc:"Refundable upper bound at preview time in currency smallest units" eg:"2599"`
	DescriptionZhCN       string                     `json:"descriptionZhCN" dc:"Frozen Simplified Chinese item description" eg:"Cloud server"`
	DescriptionEnUS       string                     `json:"descriptionEnUS" dc:"Frozen English item description" eg:"Cloud server"`
	Impacts               []*RefundEntitlementImpact `json:"impacts" dc:"Bounded frozen entitlement-impact snapshots" eg:"[]"`
}

type RefundEntitlementImpact struct {
	ProductNameZhCN     string `json:"productNameZhCN" dc:"Affected product name in Simplified Chinese" eg:"Cloud server"`
	ProductNameEnUS     string `json:"productNameEnUS" dc:"Affected product name in English" eg:"Cloud server"`
	EntitlementVersion  int64  `json:"entitlementVersion" dc:"Entitlement version frozen at preview time" eg:"3"`
	StatusCode          string `json:"statusCode" dc:"Entitlement status frozen at preview time" eg:"active"`
	RequiresTermination bool   `json:"requiresTermination" dc:"Whether refund execution must terminate this entitlement" eg:"true"`
}

type Refund struct {
	RefundNo                       string `json:"refundNo" dc:"Stable refund request number" eg:"rfd_01J2ABCDEF"`
	OrderNo                        string `json:"orderNo" dc:"Order covered by the refund" eg:"ORD_01J2ABCDEF"`
	InvoiceNo                      string `json:"invoiceNo" dc:"Invoice covered by the refund" eg:"BIL_01J2ABCDEF"`
	CurrencyCode                   string `json:"currencyCode" dc:"Exact refund currency" eg:"CNY"`
	StatusCode                     string `json:"statusCode" dc:"Authoritative refund status" eg:"pending_review"`
	ReasonCode                     string `json:"reasonCode" dc:"Customer-selected fixed refund reason" eg:"wrong_configuration"`
	ReasonDetail                   string `json:"reasonDetail" dc:"Optional customer refund explanation" eg:"This item is no longer needed"`
	AmountMinor                    string `json:"amountMinor" dc:"Exact refund amount in currency smallest units" eg:"1299"`
	ApprovalModeCode               string `json:"approvalModeCode" dc:"Review mode: automatic or manual" eg:"manual"`
	DestinationCode                string `json:"destinationCode" dc:"Destination: original_channel, same_currency_balance, or mixed" eg:"original_channel"`
	RequiresEntitlementTermination bool   `json:"requiresEntitlementTermination" dc:"Whether execution must terminate a selected long-lived entitlement" eg:"true"`
	HasEntitlementImpact           bool   `json:"hasEntitlementImpact" dc:"Whether a selected long-lived entitlement is affected" eg:"true"`
	HasDangerousOperation          bool   `json:"hasDangerousOperation" dc:"Whether a dangerous or unknown-result operation blocks safe execution" eg:"false"`
	HighAmount                     bool   `json:"highAmount" dc:"Whether approval reaches the current-currency high-amount threshold" eg:"false"`
	Version                        int64  `json:"version" dc:"Current optimistic refund version" eg:"1"`
	ImpactDigest                   string `json:"impactDigest" dc:"Immutable selected-item and entitlement-impact digest" eg:"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"`
	SubmittedAt                    int64  `json:"submittedAt" dc:"Submission time as Unix timestamp in milliseconds" eg:"1735689600000"`
	ApprovedAt                     *int64 `json:"approvedAt,omitempty" dc:"Approval time as Unix timestamp in milliseconds, or empty" eg:"1735689660000"`
	CompletedAt                    *int64 `json:"completedAt,omitempty" dc:"Completion time as Unix timestamp in milliseconds, or empty" eg:"1735689720000"`
	Reused                         bool   `json:"reused" dc:"Whether an exact safe replay returned the committed request" eg:"false"`
}
