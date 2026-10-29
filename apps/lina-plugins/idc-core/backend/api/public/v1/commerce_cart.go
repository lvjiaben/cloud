package v1

import "github.com/gogf/gf/v2/frame/g"

// GuestCartReference is the only cart fact exposed to an anonymous website.
// The ownership token is carried in an HTTP-only cookie and never enters the
// response body.
type GuestCartReference struct {
	Reference string `json:"reference" dc:"Opaque server-owned guest cart reference" eg:"CRT_01J2ABCDEF"`
}

type GuestCartOpenReq struct {
	g.Meta       `path:"/public/commerce/carts" method:"post" tags:"IDC Public Commerce" summary:"Open anonymous shopping cart" dc:"Create or reuse one server-owned anonymous cart. Ownership is carried only by a protected cookie."`
	CurrencyCode string `json:"currencyCode" v:"required|regex:^[A-Z]{3}$" dc:"Exact configured cart currency" eg:"CNY"`
}

type GuestCartOpenRes struct {
	Cart *GuestCartReference `json:"cart" dc:"Opaque anonymous cart reference"`
}

type GuestCartItemAddReq struct {
	g.Meta                `path:"/public/commerce/cart-items" method:"post" tags:"IDC Public Commerce" summary:"Add product to anonymous shopping cart" dc:"Re-resolve the current public product, signed billing term and complete governed configuration before appending one anonymous purchase intent."`
	Quantity              int                           `json:"quantity" v:"required|min:1|max:1000" dc:"Number of product units"`
	PublicSlug            string                        `json:"publicSlug" v:"required|length:1,128" dc:"Public product slug"`
	ConfigurationRevision string                        `json:"configurationRevision" v:"required|length:43,43" dc:"Opaque published configuration revision"`
	BillingTermToken      string                        `json:"billingTermToken" v:"required|length:1,128" dc:"Opaque selected billing term token"`
	CurrencyCode          string                        `json:"currencyCode" v:"required|regex:^[A-Z]{3}$" dc:"Exact configured currency" eg:"CNY"`
	PurchaseCycles        int                           `json:"purchaseCycles" v:"required|min:1|max:120" dc:"Requested billing cycles"`
	Values                []*PurchaseConfigurationValue `json:"values" dc:"Complete governed values bound to the selected quoted term" eg:"[]"`
}

type GuestCartItemAddRes struct {
	Cart *GuestCartReference `json:"cart" dc:"Opaque anonymous cart reference"`
}
