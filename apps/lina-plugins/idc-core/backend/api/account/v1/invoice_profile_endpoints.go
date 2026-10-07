package v1

import "github.com/gogf/gf/v2/frame/g"

type InvoiceProfileListReq struct {
	g.Meta `path:"/account/billing/invoice-profiles" method:"get" tags:"IDC Customer Billing" summary:"List my invoice titles" dc:"Return one bounded encrypted-keyset page containing only the current customer's active and archived invoice titles."`
	Cursor string `json:"cursor" v:"max-length:128" dc:"Encrypted cursor from the previous page" eg:"AQIDBA"`
	Limit  int    `json:"limit" d:"20" v:"min:1|max:50" dc:"Maximum profiles returned in this page" eg:"20"`
}
type InvoiceProfileListRes struct {
	Items      []*InvoiceProfile `json:"items" dc:"Current-customer invoice titles without bank-account plaintext" eg:"[]"`
	NextCursor string            `json:"nextCursor" dc:"Encrypted next-page cursor, or empty" eg:""`
}

type InvoiceProfileCreateReq struct {
	g.Meta            `path:"/account/billing/invoice-profiles" method:"post" tags:"IDC Customer Billing" summary:"Add my invoice title" dc:"Create one current-customer invoice title. An optional bank account is consumed by secretcap and never returned."`
	TitleTypeCode     string `json:"titleTypeCode" v:"required|max-length:16" dc:"individual or enterprise" eg:"enterprise"`
	InvoiceTitle      string `json:"invoiceTitle" v:"required|max-length:512" dc:"Personal name or enterprise legal name" eg:"Example Technology Co., Ltd."`
	TaxRegistrationNo string `json:"taxRegistrationNo" v:"max-length:128" dc:"Required for enterprise and forbidden for individual" eg:"91310000EXAMPLE"`
	RegisteredAddress string `json:"registeredAddress" v:"max-length:512" dc:"Optional enterprise registered address" eg:"Shanghai"`
	RegisteredPhone   string `json:"registeredPhone" v:"max-length:32" dc:"Optional enterprise registered phone" eg:"021-12345678"`
	BankName          string `json:"bankName" v:"max-length:256" dc:"Optional enterprise bank name" eg:"Example Bank"`
	BankAccount       string `json:"bankAccount" v:"max-length:128" dc:"Optional enterprise bank account consumed and never returned" eg:"6222000012345678"`
	IsDefault         bool   `json:"isDefault" dc:"Whether to make this profile the explicit default" eg:"true"`
	IdempotencyKey    string `json:"idempotencyKey" v:"required|min-length:16|max-length:128" dc:"Stable operation key reused only for the same create submission" eg:"invoice-profile-20260817-0001"`
}
type InvoiceProfileCreateRes struct {
	Item *InvoiceProfile `json:"item" dc:"Committed secret-safe invoice profile" eg:"{}"`
}

type InvoiceProfileUpdateReq struct {
	g.Meta            `path:"/account/billing/invoice-profiles/{profileNo}" method:"put" tags:"IDC Customer Billing" summary:"Update my invoice title" dc:"Update one active current-customer profile. Bank-account action must explicitly keep, replace, or clear the encrypted value."`
	ProfileNo         string `json:"profileNo" in:"path" v:"required|max-length:40" dc:"Stable owned profile number" eg:"ivp_0123456789abcdef0123456789abcdef"`
	ExpectedVersion   int64  `json:"expectedVersion" v:"required|min:1" dc:"Profile version observed before editing" eg:"2"`
	TitleTypeCode     string `json:"titleTypeCode" v:"required|max-length:16" dc:"individual or enterprise" eg:"enterprise"`
	InvoiceTitle      string `json:"invoiceTitle" v:"required|max-length:512" dc:"Personal name or enterprise legal name" eg:"Example Technology Co., Ltd."`
	TaxRegistrationNo string `json:"taxRegistrationNo" v:"max-length:128" dc:"Required for enterprise and forbidden for individual" eg:"91310000EXAMPLE"`
	RegisteredAddress string `json:"registeredAddress" v:"max-length:512" dc:"Optional enterprise registered address" eg:"Shanghai"`
	RegisteredPhone   string `json:"registeredPhone" v:"max-length:32" dc:"Optional enterprise registered phone" eg:"021-12345678"`
	BankName          string `json:"bankName" v:"max-length:256" dc:"Optional enterprise bank name" eg:"Example Bank"`
	BankAccountAction string `json:"bankAccountAction" v:"required|max-length:16" dc:"Explicit bank-account action: keep, replace, or clear" eg:"keep"`
	BankAccount       string `json:"bankAccount" v:"max-length:128" dc:"Required only for replace; consumed and never returned" eg:"6222000012345678"`
	IdempotencyKey    string `json:"idempotencyKey" v:"required|min-length:16|max-length:128" dc:"Stable operation key reused only for the same update submission" eg:"invoice-profile-update-20260817-0001"`
}
type InvoiceProfileUpdateRes struct {
	Item *InvoiceProfile `json:"item" dc:"Updated secret-safe invoice profile" eg:"{}"`
}

type InvoiceProfileDefaultSetReq struct {
	g.Meta          `path:"/account/billing/invoice-profiles/{profileNo}/default" method:"post" tags:"IDC Customer Billing" summary:"Set my default invoice title" dc:"Make one active owned profile the explicit default without changing any title fields."`
	ProfileNo       string `json:"profileNo" in:"path" v:"required|max-length:40" dc:"Stable owned profile number" eg:"ivp_0123456789abcdef0123456789abcdef"`
	ExpectedVersion int64  `json:"expectedVersion" v:"required|min:1" dc:"Profile version observed before setting default" eg:"2"`
}
type InvoiceProfileDefaultSetRes struct {
	Item *InvoiceProfile `json:"item" dc:"Current default invoice profile" eg:"{}"`
}

type InvoiceProfileArchiveReq struct {
	g.Meta          `path:"/account/billing/invoice-profiles/{profileNo}/deactivation" method:"post" tags:"IDC Customer Billing" summary:"Deactivate my invoice title" dc:"Archive one active owned profile without deleting history or automatically selecting another default."`
	ProfileNo       string `json:"profileNo" in:"path" v:"required|max-length:40" dc:"Stable owned profile number" eg:"ivp_0123456789abcdef0123456789abcdef"`
	ExpectedVersion int64  `json:"expectedVersion" v:"required|min:1" dc:"Profile version observed before deactivation" eg:"2"`
}
type InvoiceProfileArchiveRes struct {
	Item *InvoiceProfile `json:"item" dc:"Archived invoice profile retained for history" eg:"{}"`
}
