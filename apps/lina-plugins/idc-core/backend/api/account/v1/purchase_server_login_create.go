// This file defines private, authenticated server-login choice creation.
package v1

import "github.com/gogf/gf/v2/frame/g"

// PurchaseServerLoginCreateReq accepts optional customer server login choices.
// Password validation belongs to the secret-free service error boundary: generic
// GoFrame length messages interpolate the rejected value and must not see it.
type PurchaseServerLoginCreateReq struct {
	g.Meta                   `path:"/account/catalog/products/{publicSlug}/purchase-configuration/server-login" method:"post" tags:"IDC Customer Catalog" summary:"Save my server login choices" dc:"Save optional hostname and password only when the published merchant policy enables customer input. Password is stored through the secret owner and never returned."`
	PublicSlug               string `json:"publicSlug" in:"path" v:"required|length:1,128" dc:"Public product slug" eg:"cloud-server"`
	ConfigurationRevision    string `json:"configurationRevision" v:"required|max-length:128" dc:"Opaque published configuration revision" eg:"Q1JfYWJj"`
	PreviousServerLoginToken string `json:"previousServerLoginToken" v:"max-length:128" dc:"Optional owned server login token; an empty password preserves its saved password when editing" eg:""`
	Hostname                 string `json:"hostname" v:"max-length:63" dc:"Optional server hostname; empty keeps automatic generation" eg:"my-server"`
	Password                 string `json:"password" dc:"Optional server password of 6 to 20 allowed characters; empty keeps automatic generation" eg:""`
}

// PurchaseServerLoginCreateRes contains only a safe opaque token and status.
type PurchaseServerLoginCreateRes struct {
	ServerLoginToken   string `json:"serverLoginToken" dc:"Opaque token bound to the current customer login and published product version" eg:"PSL_0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"`
	Hostname           string `json:"hostname" dc:"Saved customer hostname, or empty for automatic generation" eg:"my-server"`
	PasswordConfigured bool   `json:"passwordConfigured" dc:"Whether a private customer password was saved; password bytes are never returned" eg:"true"`
}
