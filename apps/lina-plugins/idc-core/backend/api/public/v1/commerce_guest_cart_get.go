// This file exposes only cookie-owned anonymous cart display facts.
package v1

import "github.com/gogf/gf/v2/frame/g"

// GuestCartView is a bounded anonymous purchase-intent projection, never a final quote.
type GuestCartView struct {
	Reference    string               `json:"reference" dc:"Opaque cart reference; never an ownership credential" eg:"CRT_01J2ABCDEF"`
	CurrencyCode string               `json:"currencyCode" dc:"Cart currency" eg:"CNY"`
	Version      int64                `json:"version" dc:"Optimistic cart version" eg:"1"`
	Items        []*GuestCartViewItem `json:"items" dc:"At most 100 current cart purchase intents" eg:"[]"`
}

// GuestCartViewItem omits configuration values, private files and customer facts.
type GuestCartViewItem struct {
	ConfigurationReference string `json:"configurationReference" dc:"Opaque selected-configuration reference retained through cart merge; never configuration values or an ownership credential" eg:"CFS_01J2ABCDEF"`
	CartItemNo             string `json:"cartItemNo" dc:"Opaque purchase item reference" eg:"CIT_01J2ABCDEF"`
	ProductNameZhCN        string `json:"productNameZhCN" dc:"Public Chinese product name" eg:"Cloud server"`
	ProductNameEnUS        string `json:"productNameEnUS" dc:"Public English product name" eg:"Cloud server"`
	PublicSlug             string `json:"publicSlug" dc:"Public product slug" eg:"cloud-server"`
	Quantity               int    `json:"quantity" dc:"Number of units" eg:"1"`
	PurchaseCycles         int    `json:"purchaseCycles" dc:"Number of billing cycles" eg:"1"`
	Version                int64  `json:"version" dc:"Optimistic item version" eg:"1"`
}

// GuestCartGetReq reads only the cart owned by the protected browser cookie.
type GuestCartGetReq struct {
	g.Meta `path:"/public/commerce/cart" method:"get" tags:"IDC Public Commerce" summary:"Read anonymous shopping cart" dc:"Return bounded public purchase-intent facts for the protected cookie owner, or an empty cart when no cookie exists. Never return configuration values or ownership tokens."`
}

// GuestCartGetRes returns an optional anonymous cart.
type GuestCartGetRes struct {
	Cart *GuestCartView `json:"cart" dc:"Cookie-owned cart, or null when absent" eg:"{}"`
}
