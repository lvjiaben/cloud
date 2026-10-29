// This file projects generic product configuration into owned server changes.
// Only opaque public field choices cross HTTP; supplier keys and prices do not.
package v1

import "github.com/gogf/gf/v2/frame/g"

// NativeConfigurationLocalizedText is a bilingual numeric choice label.
type NativeConfigurationLocalizedText struct {
	ZhCN string `json:"zhCN" dc:"Simplified Chinese label" eg:"Standard"`
	EnUS string `json:"enUS" dc:"English label" eg:"Standard"`
}

// NativeConfigurationOption is one opaque bilingual static choice.
type NativeConfigurationOption struct {
	OptionToken string                       `json:"optionToken" dc:"Opaque option token scoped to this configuration revision" eg:""`
	LabelZhCN   string                       `json:"labelZhCN" dc:"Simplified Chinese option label" eg:""`
	LabelEnUS   string                       `json:"labelEnUS" dc:"English option label" eg:""`
	Children    []*NativeConfigurationOption `json:"children,omitempty" dc:"Bounded governed child options" eg:"[]"`
}

// NativeConfigurationUploadPolicy exposes requirements only. Anonymous
// callers never receive an upload address, object path, credential, or token.
type NativeConfigurationUploadPolicy struct {
	MaximumFiles       int      `json:"maximumFiles" dc:"Maximum number of files for this field" eg:"1"`
	MaximumFileBytes   int64    `json:"maximumFileBytes" dc:"Maximum bytes for one file" eg:"1"`
	MaximumTotalBytes  int64    `json:"maximumTotalBytes" dc:"Maximum total bytes across the field" eg:"1"`
	AllowedMIMETypes   []string `json:"allowedMimeTypes" dc:"Allowed inspected MIME types" eg:"[]"`
	AllowedExtensions  []string `json:"allowedExtensions" dc:"Allowed lowercase file extensions" eg:"[]"`
	SafetyScanRequired bool     `json:"safetyScanRequired" dc:"Whether host safety scanning is required before use" eg:"false"`
}

// NativeConfigurationField contains only host-renderable public facts.
type NativeConfigurationField struct {
	GroupZhCN       string                                       `json:"groupZhCN,omitempty" dc:"Merchant configuration group Chinese label" eg:"Specifications"`
	GroupEnUS       string                                       `json:"groupEnUS,omitempty" dc:"Merchant configuration group English label" eg:"Specifications"`
	Unit            string                                       `json:"unit,omitempty" dc:"Display-only unit suffix without value conversion" eg:"GiB"`
	Advanced        bool                                         `json:"advanced,omitempty" dc:"Whether this field appears under advanced purchase settings" eg:"false"`
	NumericLabels   map[string]*NativeConfigurationLocalizedText `json:"numericLabels,omitempty" dc:"Bilingual display labels for numeric specification choices" eg:"{}"`
	DisplayControl  string                                       `json:"displayControl,omitempty" dc:"Merchant-owned local purchase control: auto, input, slider, select or radio" eg:"slider"`
	QuantityStep    string                                       `json:"quantityStep,omitempty" dc:"Exact positive integer increment from the minimum; defaults to one" eg:"1"`
	NumericChoices  []string                                     `json:"numericChoices,omitempty" dc:"Merchant-defined numeric choices; submissions retain the integer value type" eg:"[]"`
	FieldToken      string                                       `json:"fieldToken" dc:"Opaque field token scoped to this configuration revision" eg:""`
	Type            string                                       `json:"type" dc:"Governed field type declared by the product compiler" eg:""`
	LabelZhCN       string                                       `json:"labelZhCN" dc:"Simplified Chinese field label" eg:""`
	LabelEnUS       string                                       `json:"labelEnUS" dc:"English field label" eg:""`
	DescriptionZhCN string                                       `json:"descriptionZhCN" dc:"Simplified Chinese field description" eg:""`
	DescriptionEnUS string                                       `json:"descriptionEnUS" dc:"English field description" eg:""`
	Visible         bool                                         `json:"visible" dc:"Authoritative current visibility" eg:"false"`
	Required        bool                                         `json:"required" dc:"Authoritative current required state" eg:"false"`
	Disabled        bool                                         `json:"disabled" dc:"Authoritative current disabled state" eg:"false"`
	Format          string                                       `json:"format,omitempty" dc:"Optional host-governed text format" eg:""`
	MinimumDecimal  string                                       `json:"minimumDecimal,omitempty" dc:"Exact minimum decimal value" eg:""`
	MaximumDecimal  string                                       `json:"maximumDecimal,omitempty" dc:"Exact maximum decimal value" eg:""`
	MinimumInteger  string                                       `json:"minimumInteger,omitempty" dc:"Exact minimum integer value" eg:""`
	MaximumInteger  string                                       `json:"maximumInteger,omitempty" dc:"Exact maximum integer value" eg:""`
	MinimumLength   int                                          `json:"minimumLength,omitempty" dc:"Minimum text length" eg:"1"`
	MaximumLength   int                                          `json:"maximumLength,omitempty" dc:"Maximum text length" eg:"1"`
	Options         []*NativeConfigurationOption                 `json:"options,omitempty" dc:"Opaque bounded static options" eg:"[]"`
	UploadPolicy    *NativeConfigurationUploadPolicy             `json:"uploadPolicy,omitempty" dc:"Host-enforced file requirements without an upload address or token" eg:"{}"`
}

// NativeConfigurationValue uses exactly one typed value arm.
type NativeConfigurationValue struct {
	FieldToken          string   `json:"fieldToken" dc:"Opaque field token from the current revision" eg:""`
	Type                string   `json:"type" dc:"Governed field type matching the field declaration" eg:""`
	TextValue           *string  `json:"textValue,omitempty" dc:"Single-line or multi-line text value" eg:""`
	IntegerValue        *string  `json:"integerValue,omitempty" dc:"Exact canonical base-10 integer string" eg:""`
	DecimalValue        *string  `json:"decimalValue,omitempty" dc:"Exact canonical decimal string" eg:""`
	BooleanValue        *bool    `json:"booleanValue,omitempty" dc:"Boolean value" eg:""`
	DateValue           *string  `json:"dateValue,omitempty" dc:"Calendar date in YYYY-MM-DD form" eg:""`
	DateTimeValue       *string  `json:"dateTimeValue,omitempty" dc:"Configuration date-time input in RFC3339 form, not an event timestamp" eg:""`
	OptionTokens        []string `json:"optionTokens,omitempty" dc:"Opaque single-select or multi-select option tokens" eg:"[]"`
	TreeOptionTokens    []string `json:"treeOptionTokens,omitempty" dc:"Opaque governed tree selection" eg:"[]"`
	AddressOptionTokens []string `json:"addressOptionTokens,omitempty" dc:"Opaque ordered address path" eg:"[]"`
	FileTokens          []string `json:"fileTokens,omitempty" dc:"Governed file tokens revalidated against the exact login and frozen field policy" eg:"[]"`
}

// NativeConfigurationIssue is one stable localizable validation result.
type NativeConfigurationIssue struct {
	FieldToken string `json:"fieldToken,omitempty" dc:"Opaque affected field token" eg:""`
	IssueCode  string `json:"issueCode" dc:"Stable client-localizable issue code" eg:""`
}

// NativeServerConfigurationContextReq reads current values or evaluates an edit.
type NativeServerConfigurationContextReq struct {
	g.Meta                `path:"/account/native-servers/{serverNo}/configuration-context" method:"get" tags:"IDC Account Servers" summary:"Read my server change configuration" dc:"Read the current product-defined editable fields and values, or reevaluate a complete edit without changing the server or cart. No new-purchase price is returned."`
	ServerNo              string `json:"serverNo" in:"path" v:"required|regex:^SVR-[0-9A-F]{32}$" dc:"Owned public server number" eg:"SVR-0123456789ABCDEF0123456789ABCDEF"`
	ConfigurationRevision string `json:"configurationRevision" v:"max-length:128" dc:"Opaque current configuration revision; required when values are supplied" eg:""`
	Values                string `json:"values" v:"max-length:16384" dc:"URL-encoded JSON array of complete typed public values, at most one hundred fields and 16384 bytes; omit to read frozen values, use [] for an explicitly empty edit. Never include passwords." eg:"[]"`
}

// NativeServerConfigurationContextRes describes the existing period, not a quote.
type NativeServerConfigurationContextRes struct {
	EntitlementNo             string                      `json:"entitlementNo" dc:"Current owned entitlement number" eg:"ent_0123456789abcdef0123456789abcdef"`
	EntitlementVersion        int64                       `json:"entitlementVersion" dc:"Current entitlement revision checked by the owner" eg:"1"`
	StructuredConfigurationNo string                      `json:"structuredConfigurationNo" dc:"Current frozen structured configuration number" eg:"SCF_example"`
	ConfigurationRevision     string                      `json:"configurationRevision" dc:"Opaque current configuration revision" eg:"revision-example"`
	CurrencyCode              string                      `json:"currencyCode" dc:"Existing service currency code" eg:"CNY"`
	PurchaseCycles            int                         `json:"purchaseCycles" dc:"Existing service purchase cycles; not editable in this form" eg:"1"`
	Ready                     bool                        `json:"ready" dc:"The complete edited values are valid; a difference quote is still required" eg:"false"`
	BillingTermToken          string                      `json:"billingTermToken" dc:"Opaque current-period selector; empty until the complete edit is valid, never a final price" eg:""`
	Fields                    []*NativeConfigurationField `json:"fields" dc:"Bounded product-defined editable fields and authoritative options" eg:"[]"`
	Values                    []*NativeConfigurationValue `json:"values" dc:"Normalized complete typed public values without prior file credentials" eg:"[]"`
	Issues                    []*NativeConfigurationIssue `json:"issues" dc:"Bounded field validation issues" eg:"[]"`
}

// NativeServerTargetQuoteReq freezes an edited configuration before price calculation.
type NativeServerTargetQuoteReq struct {
	g.Meta                `path:"/account/native-servers/{serverNo}/target-quotes" method:"post" tags:"IDC Account Servers" summary:"Freeze my server change target" dc:"Freeze the complete valid product configuration for this owned server without modifying its current settings or shopping cart. Use the returned quoteNo as targetQuoteNo in the separate resize commercial quote."`
	ServerNo              string                      `json:"serverNo" in:"path" v:"required|regex:^SVR-[0-9A-F]{32}$" dc:"Owned public server number" eg:"SVR-0123456789ABCDEF0123456789ABCDEF"`
	ConfigurationRevision string                      `json:"configurationRevision" v:"required|max-length:128" dc:"Opaque current configuration revision" eg:"revision-example"`
	BillingTermToken      string                      `json:"billingTermToken" v:"required|max-length:4096" dc:"Opaque valid current-period selector returned by configuration context" eg:"term-example"`
	Values                []*NativeConfigurationValue `json:"values" v:"required|length:0,100" dc:"Complete typed public values from the valid edit; maximum one hundred fields" eg:"[]"`
}
type NativeServerTargetQuoteRes struct {
	QuoteNo                   string `json:"quoteNo" dc:"Owned target quote number to submit as targetQuoteNo for resize pricing" eg:"QTE_example"`
	QuoteItemNo               string `json:"quoteItemNo" dc:"Frozen target quote item number" eg:"QTI_example"`
	StructuredConfigurationNo string `json:"structuredConfigurationNo" dc:"Frozen target configuration number" eg:"SCF_example"`
}
