// This file defines optimistic removal from the protected anonymous cart.
package v1

import "github.com/gogf/gf/v2/frame/g"

// GuestCartItemRemoveReq removes one item belonging to the protected cookie owner.
type GuestCartItemRemoveReq struct {
	g.Meta              `path:"/public/commerce/cart-items/{cartItemNo}" method:"delete" tags:"IDC Public Commerce" summary:"Remove anonymous shopping cart item" dc:"Remove exactly one cookie-owned purchase intent using current cart and item versions; refuse stale or foreign references."`
	CartItemNo          string `json:"cartItemNo" v:"required|length:1,128" dc:"Opaque owned cart item reference" eg:"CIT_01J2ABCDEF"`
	ExpectedCartVersion int64  `json:"expectedCartVersion" v:"required|min:1" dc:"Last read cart version" eg:"1"`
	ExpectedItemVersion int64  `json:"expectedItemVersion" v:"required|min:1" dc:"Last read item version" eg:"1"`
}

// GuestCartItemRemoveRes returns the remaining public purchase intents.
type GuestCartItemRemoveRes struct {
	Cart *GuestCartView `json:"cart" dc:"Updated cookie-owned cart" eg:"{}"`
}
