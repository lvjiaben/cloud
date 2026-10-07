package v1

import (
	"github.com/gogf/gf/v2/frame/g"

	publicv1 "lina-plugin-idc-core/backend/api/public/v1"
)

type CommerceCartOpenReq struct {
	g.Meta       `path:"/account/commerce/carts" method:"post" tags:"IDC Customer Commerce" summary:"Open account shopping cart" dc:"Create or reuse the current active customer shopping cart."`
	CurrencyCode string `json:"currencyCode" v:"required|regex:^[A-Z]{3}$" dc:"Exact configured cart currency" eg:"CNY"`
}
type CommerceCartOpenRes struct {
	Cart *CommerceCart `json:"cart" dc:"Current customer shopping cart"`
}

type CommerceCartGetReq struct {
	g.Meta `path:"/account/commerce/cart" method:"get" tags:"IDC Customer Commerce" summary:"Get account shopping cart" dc:"Return the current active customer shopping cart and server-side availability projection."`
}
type CommerceCartGetRes struct {
	Cart *CommerceCart `json:"cart" dc:"Current customer shopping cart and server-side item state"`
}

type CommerceGuestCartMergeReq struct {
	g.Meta `path:"/account/commerce/guest-cart-merge" method:"post" tags:"IDC Customer Commerce" summary:"Merge anonymous shopping cart" dc:"Consume the protected anonymous-cart browser credential and transactionally merge its purchase intents into the current active customer cart."`
}

type CommerceGuestCartMergeRes struct {
	Merged bool          `json:"merged" dc:"Whether an anonymous cart credential was consumed" eg:"true"`
	Cart   *CommerceCart `json:"cart,omitempty" dc:"Merged current customer shopping cart, when a credential was consumed"`
}

type CommerceCartItemAddReq struct {
	g.Meta                `path:"/account/commerce/cart-items" method:"post" tags:"IDC Customer Commerce" summary:"Add product to shopping cart" dc:"Resolve current signed public catalog facts and append one product configuration with optimistic concurrency."`
	ExpectedCartVersion   int64                                  `json:"expectedCartVersion" v:"required|min:1" dc:"Expected optimistic cart version"`
	Quantity              int                                    `json:"quantity" v:"required|min:1|max:1000" dc:"Number of product units"`
	PublicSlug            string                                 `json:"publicSlug" v:"required|length:1,128" dc:"Public product slug"`
	ConfigurationRevision string                                 `json:"configurationRevision" v:"required|length:43,43" dc:"Opaque published configuration revision"`
	BillingTermToken      string                                 `json:"billingTermToken" v:"required|length:1,128" dc:"Opaque selected billing term token"`
	CurrencyCode          string                                 `json:"currencyCode" v:"required|regex:^[A-Z]{3}$" dc:"Exact configured currency"`
	PurchaseCycles        int                                    `json:"purchaseCycles" v:"required|min:1|max:120" dc:"Requested billing cycles"`
	Values                []*publicv1.PurchaseConfigurationValue `json:"values" v:"required" dc:"Complete governed values bound to the selected quoted term"`
}
type CommerceCartItemAddRes struct {
	Cart *CommerceCart `json:"cart" dc:"Updated current customer shopping cart"`
}

type CommerceCartItemRemoveReq struct {
	g.Meta              `path:"/account/commerce/cart-items/{cartItemNo}" method:"delete" tags:"IDC Customer Commerce" summary:"Remove shopping cart item" dc:"Remove one current customer cart item with cart and item optimistic concurrency versions."`
	CartItemNo          string `json:"cartItemNo" in:"path" v:"required|max-length:40" dc:"Stable cart-item number"`
	ExpectedCartVersion int64  `json:"expectedCartVersion" v:"required|min:1" dc:"Expected optimistic cart version"`
	ExpectedItemVersion int64  `json:"expectedItemVersion" v:"required|min:1" dc:"Expected optimistic item version"`
}
type CommerceCartItemRemoveRes struct {
	Cart *CommerceCart `json:"cart" dc:"Updated current customer shopping cart"`
}

type CommerceQuoteCreateReq struct {
	g.Meta              `path:"/account/commerce/quotes" method:"post" tags:"IDC Customer Commerce" summary:"Create final shopping cart quote" dc:"Revalidate the complete customer cart and persist one immutable expiring final quote."`
	ExpectedCartVersion int64    `json:"expectedCartVersion" v:"required|min:1" dc:"Expected optimistic cart version"`
	AssetNumbers        []string `json:"assetNumbers" dc:"Optional selected customer marketing asset numbers; collection bounds are enforced by the quote service"`
}
type CommerceQuoteCreateRes struct {
	Quote *CommerceQuote `json:"quote" dc:"Immutable final quote"`
}

type CommerceOrderCheckoutReq struct {
	g.Meta         `path:"/account/commerce/orders" method:"post" tags:"IDC Customer Commerce" summary:"Checkout final quote" dc:"Atomically convert one current immutable quote into an order, invoice, inventory reservations and fulfillment units."`
	QuoteNo        string `json:"quoteNo" v:"required|max-length:40" dc:"Current immutable quote number"`
	IdempotencyKey string `json:"idempotencyKey" v:"required|min-length:8|max-length:128" dc:"Random action key stable across retries"`
}
type CommerceOrderCheckoutRes struct {
	Checkout *CommerceCheckout `json:"checkout" dc:"Committed or exactly replayed checkout result"`
}

type CommercePaymentPreparationCreateReq struct {
	g.Meta  `path:"/account/commerce/orders/{orderNo}/payment-preparations" method:"post" tags:"IDC Customer Commerce" summary:"Prepare order payment" dc:"Re-evaluate current order, invoice and identity facts before a payment attempt can be created."`
	OrderNo string `json:"orderNo" in:"path" v:"required|max-length:40" dc:"Current customer-owned order number"`
}
type CommercePaymentPreparationCreateRes struct {
	Preparation *CommercePaymentPreparation `json:"preparation" dc:"Current payment-preparation decision"`
}

type CommercePaymentPreparationResumeReq struct {
	g.Meta    `path:"/account/commerce/payment-preparations/{contextNo}" method:"put" tags:"IDC Customer Commerce" summary:"Resume payment preparation" dc:"Reload a customer-owned payment preparation after identity or order state changes."`
	ContextNo string `json:"contextNo" in:"path" v:"required|max-length:40" dc:"Current customer-owned preparation context"`
}
type CommercePaymentPreparationResumeRes struct {
	Preparation *CommercePaymentPreparation `json:"preparation" dc:"Re-evaluated payment-preparation decision"`
}

type CommerceOrderCancelReq struct {
	g.Meta         `path:"/account/commerce/orders/{orderNo}/cancellation" method:"put" tags:"IDC Customer Commerce" summary:"Cancel unpaid order" dc:"Request idempotent customer cancellation; payment-processing orders wait for payment-owner closure."`
	OrderNo        string `json:"orderNo" in:"path" v:"required|max-length:40" dc:"Current customer-owned order number"`
	IdempotencyKey string `json:"idempotencyKey" v:"required|min-length:8|max-length:128" dc:"Random action key stable across retries"`
}
type CommerceOrderCancelRes struct {
	Cancellation *CommerceOrderCancellation `json:"cancellation" dc:"Committed, pending, or exactly replayed cancellation result"`
}

type CommerceOrderListReq struct {
	g.Meta     `path:"/account/commerce/orders" method:"get" tags:"IDC Customer Commerce" summary:"List my orders" dc:"Return one bounded encrypted-keyset page owned by the current active customer."`
	StatusCode string `json:"statusCode" dc:"Optional exact order status"`
	Cursor     string `json:"cursor" v:"max-length:128" dc:"Encrypted cursor from the previous page"`
	Limit      int    `json:"limit" d:"20" v:"min:1|max:50" dc:"Maximum orders returned in this page"`
}
type CommerceOrderListRes struct {
	Orders     []*CommerceOrderSummary `json:"orders" dc:"Bounded current-customer order summaries"`
	NextCursor string                  `json:"nextCursor" dc:"Encrypted next-page cursor, or empty"`
}

type CommerceOrderDetailReq struct {
	g.Meta  `path:"/account/commerce/orders/{orderNo}" method:"get" tags:"IDC Customer Commerce" summary:"Get my order" dc:"Return one bounded aggregate of a current customer-owned order without frontend waterfall requests."`
	OrderNo string `json:"orderNo" in:"path" v:"required|max-length:40" dc:"Current customer-owned order number"`
}
type CommerceOrderDetailRes struct {
	Order *CommerceOrderDetail `json:"order" dc:"Bounded customer-safe order aggregate"`
}
