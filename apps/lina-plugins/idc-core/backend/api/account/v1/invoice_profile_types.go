package v1

// InvoiceProfile is one current-customer-owned reusable invoice title.
type InvoiceProfile struct {
	ProfileNo             string `json:"profileNo" dc:"Stable customer invoice-profile number" eg:"ivp_0123456789abcdef0123456789abcdef"`
	TitleTypeCode         string `json:"titleTypeCode" dc:"Invoice-title type: individual or enterprise" eg:"enterprise"`
	InvoiceTitle          string `json:"invoiceTitle" dc:"Customer-visible invoice title" eg:"Example Technology Co., Ltd."`
	TaxRegistrationNo     string `json:"taxRegistrationNo,omitempty" dc:"Enterprise tax registration number; empty for an individual title" eg:"91310000EXAMPLE"`
	RegisteredAddress     string `json:"registeredAddress,omitempty" dc:"Optional registered address" eg:"Shanghai"`
	RegisteredPhone       string `json:"registeredPhone,omitempty" dc:"Optional registered phone" eg:"021-12345678"`
	BankName              string `json:"bankName,omitempty" dc:"Optional bank name" eg:"Example Bank"`
	BankAccountMasked     string `json:"bankAccountMasked,omitempty" dc:"Display-only masked bank account; never the original value" eg:"****6789"`
	BankAccountConfigured bool   `json:"bankAccountConfigured" dc:"Whether an encrypted bank account is retained for this profile" eg:"true"`
	IsDefault             bool   `json:"isDefault" dc:"Whether this is the customer's current default invoice title" eg:"true"`
	StatusCode            string `json:"statusCode" dc:"Profile status: active or archived" eg:"active"`
	Version               int64  `json:"version" dc:"Optimistic profile version" eg:"2"`
	CreatedAt             int64  `json:"createdAt" dc:"Creation time as Unix milliseconds" eg:"1720000000000"`
	UpdatedAt             int64  `json:"updatedAt" dc:"Last change time as Unix milliseconds" eg:"1720000000000"`
}
