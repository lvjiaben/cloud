// This file declares display-safe customer management response DTOs.

package v1

type CustomerLevelItem struct {
	LevelNo  string `json:"levelNo" dc:"Stable customer level number" eg:"LVL_0123456789abcdef0123456789abcdef"`
	Code     string `json:"code" dc:"Merchant-defined customer level code" eg:"gold"`
	NameZhCn string `json:"nameZhCn" dc:"Simplified Chinese customer level name" eg:"Gold"`
	NameEnUs string `json:"nameEnUs" dc:"English customer level name" eg:"Gold"`
	Priority int    `json:"priority" dc:"Customer level display priority" eg:"100"`
}

type CustomerSummaryItem struct {
	CustomerNo                   string             `json:"customerNo" dc:"Stable external customer number" eg:"CUS_0123456789abcdef0123456789abcdef"`
	CustomerCode                 string             `json:"customerCode" dc:"Short monotonic customer number shown to staff and customers" eg:"10000"`
	PrimaryPhone                 string             `json:"primaryPhone" dc:"Primary customer phone with calling code, or empty when the primary login is email" eg:"+8613800138000"`
	Type                         string             `json:"type" dc:"Customer type: individual or enterprise" eg:"individual"`
	Status                       string             `json:"status" dc:"Account status: pending_activation, active, restricted, disabled, or anonymized" eg:"active"`
	Verification                 string             `json:"verification" dc:"Identity verification status: unverified, pending, verified, rejected, expired, or revoked" eg:"verified"`
	DisplayName                  string             `json:"displayName" dc:"Customer display name without legal identity fields" eg:"Example Customer"`
	CountryCode                  string             `json:"countryCode" dc:"ISO 3166-1 alpha-2 customer country code" eg:"CN"`
	PreferredLocale              string             `json:"preferredLocale" dc:"Preferred customer locale" eg:"zh-CN"`
	EnterpriseContractPolicyCode string             `json:"enterpriseContractPolicyCode" dc:"Enterprise contract rule: none or required; empty for individuals" eg:"none"`
	EnterpriseContractTemplateNo string             `json:"enterpriseContractTemplateNo" dc:"Enterprise contract template number, or empty"`
	Level                        *CustomerLevelItem `json:"level" dc:"Current customer level, or null when no level is assigned" eg:"null"`
	TagCount                     int                `json:"tagCount" dc:"Number of active customer tags" eg:"2"`
	Version                      int64              `json:"version" dc:"Optimistic concurrency version" eg:"1"`
	CreatedAt                    int64              `json:"createdAt" dc:"Customer creation time as a Unix timestamp in milliseconds" eg:"1786161600000"`
	UpdatedAt                    int64              `json:"updatedAt" dc:"Customer update time as a Unix timestamp in milliseconds" eg:"1786161660000"`
}

type CustomerSegmentItem struct {
	Number     string `json:"number" dc:"Stable group or tag number" eg:"TAG_0123456789abcdef0123456789abcdef"`
	Code       string `json:"code" dc:"Merchant-defined group or tag code" eg:"priority"`
	NameZhCn   string `json:"nameZhCn" dc:"Simplified Chinese segment name" eg:"Priority"`
	NameEnUs   string `json:"nameEnUs" dc:"English segment name" eg:"Priority"`
	ColorToken string `json:"colorToken" dc:"Semantic color token; empty for groups" eg:"warning"`
	Status     string `json:"status" dc:"Segment definition status: active or archived" eg:"active"`
}

type CustomerSegmentPreview struct {
	Items   []*CustomerSegmentItem `json:"items" dc:"At most twenty customer groups or tags" eg:"[]"`
	HasMore bool                   `json:"hasMore" dc:"Whether more segments require the dedicated paginated endpoint" eg:"false"`
}

type CustomerOverviewCounts struct {
	ActiveContacts       int64 `json:"activeContacts" dc:"Number of active notification contacts" eg:"2"`
	ActiveAddresses      int64 `json:"activeAddresses" dc:"Number of active customer addresses" eg:"1"`
	ActiveCustomFields   int64 `json:"activeCustomFields" dc:"Number of active custom field values" eg:"3"`
	ActiveNotes          int64 `json:"activeNotes" dc:"Number of active internal notes" eg:"4"`
	PlannedFollowUps     int64 `json:"plannedFollowUps" dc:"Number of planned follow-ups" eg:"1"`
	AvailableAttachments int64 `json:"availableAttachments" dc:"Number of scanned and available attachments" eg:"2"`
	ActiveBlacklist      int64 `json:"activeBlacklist" dc:"Number of currently effective blacklist entries" eg:"0"`
	OpenRisks            int64 `json:"openRisks" dc:"Number of open or acknowledged risk records" eg:"1"`
	ActiveGroups         int64 `json:"activeGroups" dc:"Number of active customer group assignments" eg:"1"`
	ActiveTags           int64 `json:"activeTags" dc:"Number of active customer tag assignments" eg:"2"`
}

type CustomerBusinessSummary struct {
	Orders        int64 `json:"orders" dc:"Number of customer orders" eg:"3"`
	Bills         int64 `json:"bills" dc:"Number of customer receivable bills" eg:"2"`
	Transactions  int64 `json:"transactions" dc:"Number of ledger transactions involving customer accounts" eg:"5"`
	Servers       int64 `json:"servers" dc:"Number of customer service instances" eg:"2"`
	Contracts     int64 `json:"contracts" dc:"Number of customer contracts" eg:"1"`
	TaxInvoices   int64 `json:"taxInvoices" dc:"Number of customer tax invoice requests" eg:"1"`
	Tickets       int64 `json:"tickets" dc:"Number of customer support tickets" eg:"4"`
	Notifications int64 `json:"notifications" dc:"Number of customer notification delivery tasks" eg:"8"`
	Audits        int64 `json:"audits" dc:"Number of customer-linked business audit events" eg:"6"`
}

type CustomerBalanceSummary struct {
	AccountTypeCode       string `json:"accountTypeCode" dc:"Display-safe balance account type" eg:"balance"`
	CurrencyCode          string `json:"currencyCode" dc:"ISO 4217 currency code" eg:"CNY"`
	StatusCode            string `json:"statusCode" dc:"Balance account status" eg:"active"`
	BalanceMinor          string `json:"balanceMinor" dc:"Book balance in minor units" eg:"10000"`
	HeldBalanceMinor      string `json:"heldBalanceMinor" dc:"Held balance in minor units" eg:"1000"`
	AvailableBalanceMinor string `json:"availableBalanceMinor" dc:"Available balance in minor units" eg:"9000"`
	UpdatedAt             int64  `json:"updatedAt" dc:"Balance update time in Unix milliseconds" eg:"1786161660000"`
}
