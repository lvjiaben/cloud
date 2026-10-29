// This file defines the governed anonymous purchase-configuration DTOs.

package v1

import "lina-core/pkg/plugin/capability/capmodel"

// PurchaseConfigurationOption is one opaque bilingual static choice.
type PurchaseConfigurationOption struct {
	OptionToken string                         `json:"optionToken" dc:"Opaque option token scoped to this configuration revision"`
	LabelZhCN   string                         `json:"labelZhCN" dc:"Simplified Chinese option label"`
	LabelEnUS   string                         `json:"labelEnUS" dc:"English option label"`
	Children    []*PurchaseConfigurationOption `json:"children,omitempty" dc:"Bounded governed child options"`
}

// PurchaseConfigurationUploadPolicy exposes requirements only. Anonymous
// callers never receive an upload address, object path, credential, or token.
type PurchaseConfigurationUploadPolicy struct {
	MaximumFiles       int      `json:"maximumFiles" dc:"Maximum number of files for this field"`
	MaximumFileBytes   int64    `json:"maximumFileBytes" dc:"Maximum bytes for one file"`
	MaximumTotalBytes  int64    `json:"maximumTotalBytes" dc:"Maximum total bytes across the field"`
	AllowedMIMETypes   []string `json:"allowedMimeTypes" dc:"Allowed inspected MIME types"`
	AllowedExtensions  []string `json:"allowedExtensions" dc:"Allowed lowercase file extensions"`
	SafetyScanRequired bool     `json:"safetyScanRequired" dc:"Whether host safety scanning is required before use"`
}

// PurchaseConfigurationField contains only host-renderable public facts.
type PurchaseConfigurationField struct {
	Presentation    string                             `json:"presentation,omitempty" dc:"Safe display role from the exact candidate source: operating_system or region; absent for ordinary fields" eg:"operating_system"`
	GroupZhCN       string                             `json:"groupZhCN,omitempty" dc:"Merchant configuration group Chinese label" eg:"Specifications"`
	GroupEnUS       string                             `json:"groupEnUS,omitempty" dc:"Merchant configuration group English label" eg:"Specifications"`
	Unit            string                             `json:"unit,omitempty" dc:"Display-only unit suffix without value conversion" eg:"GiB"`
	Advanced        bool                               `json:"advanced,omitempty" dc:"Whether this field appears under advanced purchase settings" eg:"false"`
	NumericLabels   map[string]*capmodel.LocalizedText `json:"numericLabels,omitempty" dc:"Bilingual display labels for numeric specification choices" eg:"{}"`
	DisplayControl  string                             `json:"displayControl,omitempty" dc:"Merchant-owned local purchase control: auto, input, slider, select or radio" eg:"slider"`
	QuantityStep    string                             `json:"quantityStep,omitempty" dc:"Exact positive integer increment from the minimum; defaults to one" eg:"1"`
	NumericChoices  []string                           `json:"numericChoices,omitempty" dc:"Merchant-defined numeric choices; submissions retain the integer value type" eg:"[]"`
	FieldToken      string                             `json:"fieldToken" dc:"Opaque field token scoped to this configuration revision"`
	Type            string                             `json:"type" dc:"Governed field type"`
	LabelZhCN       string                             `json:"labelZhCN" dc:"Simplified Chinese field label"`
	LabelEnUS       string                             `json:"labelEnUS" dc:"English field label"`
	DescriptionZhCN string                             `json:"descriptionZhCN" dc:"Simplified Chinese field description"`
	DescriptionEnUS string                             `json:"descriptionEnUS" dc:"English field description"`
	Visible         bool                               `json:"visible" dc:"Authoritative current visibility"`
	Required        bool                               `json:"required" dc:"Authoritative current required state"`
	Disabled        bool                               `json:"disabled" dc:"Authoritative current disabled state"`
	Format          string                             `json:"format,omitempty" dc:"Optional host-governed text format"`
	MinimumDecimal  string                             `json:"minimumDecimal,omitempty" dc:"Exact minimum decimal value"`
	MaximumDecimal  string                             `json:"maximumDecimal,omitempty" dc:"Exact maximum decimal value"`
	MinimumInteger  string                             `json:"minimumInteger,omitempty" dc:"Exact minimum integer value"`
	MaximumInteger  string                             `json:"maximumInteger,omitempty" dc:"Exact maximum integer value"`
	MinimumLength   int                                `json:"minimumLength,omitempty" dc:"Minimum text length"`
	MaximumLength   int                                `json:"maximumLength,omitempty" dc:"Maximum text length"`
	Options         []*PurchaseConfigurationOption     `json:"options,omitempty" dc:"Opaque bounded static options"`
	UploadPolicy    *PurchaseConfigurationUploadPolicy `json:"uploadPolicy,omitempty" dc:"Host-enforced file requirements without an upload address or token"`
}

// PurchaseConfigurationValue uses exactly one typed value arm.
type PurchaseConfigurationValue struct {
	FieldToken          string   `json:"fieldToken" dc:"Opaque field token from the current revision"`
	Type                string   `json:"type" dc:"Governed field type matching the field declaration"`
	TextValue           *string  `json:"textValue,omitempty" dc:"Single-line or multi-line text value"`
	IntegerValue        *string  `json:"integerValue,omitempty" dc:"Exact canonical base-10 integer string"`
	DecimalValue        *string  `json:"decimalValue,omitempty" dc:"Exact canonical decimal string"`
	BooleanValue        *bool    `json:"booleanValue,omitempty" dc:"Boolean value"`
	DateValue           *string  `json:"dateValue,omitempty" dc:"Calendar date in YYYY-MM-DD form"`
	DateTimeValue       *string  `json:"dateTimeValue,omitempty" dc:"RFC3339 date-time value"`
	OptionTokens        []string `json:"optionTokens,omitempty" dc:"Opaque single-select or multi-select option tokens"`
	TreeOptionTokens    []string `json:"treeOptionTokens,omitempty" dc:"Opaque governed tree selection"`
	AddressOptionTokens []string `json:"addressOptionTokens,omitempty" dc:"Opaque ordered address path"`
	FileTokens          []string `json:"fileTokens,omitempty" dc:"Governed file tokens revalidated against the exact login and frozen field policy"`
}

// PurchaseConfigurationIssue is one stable localizable validation result.
type PurchaseConfigurationIssue struct {
	FieldToken string `json:"fieldToken,omitempty" dc:"Opaque affected field token"`
	IssueCode  string `json:"issueCode" dc:"Stable client-localizable issue code"`
}

// PurchaseConfigurationView is shared by initial and evaluated responses.
type PurchaseConfigurationView struct {
	ServerLoginPolicy     *PurchaseServerLoginPolicy    `json:"serverLoginPolicy,omitempty" dc:"Published optional customer server login input rules; omitted for products without server login settings" eg:"{}"`
	Ready                 bool                          `json:"ready" dc:"Whether the initial normalized configuration is already valid, without another evaluation request" eg:"false"`
	ConfigurationRevision string                        `json:"configurationRevision" dc:"Opaque current published configuration revision"`
	Fields                []*PurchaseConfigurationField `json:"fields" dc:"Authoritative bounded field states"`
	Values                []*PurchaseConfigurationValue `json:"values" dc:"Authoritative normalized typed values"`
}

// PurchaseConfigurationEvaluation adds server-owned validity and issues.
type PurchaseConfigurationEvaluation struct {
	PurchaseConfigurationView
	Valid  bool                          `json:"valid" dc:"Whether the current complete configuration is valid"`
	Issues []*PurchaseConfigurationIssue `json:"issues" dc:"Stable bounded validation issues"`
}

// PurchaseConfigurationQuoteTerm is one core-owned published price term.
type PurchaseConfigurationQuoteTerm struct {
	BillingTermToken      string `json:"billingTermToken" dc:"Opaque billing term token"`
	LabelZhCN             string `json:"labelZhCN" dc:"Simplified Chinese billing term label"`
	LabelEnUS             string `json:"labelEnUS" dc:"English billing term label"`
	MinimumPurchaseCycles int    `json:"minimumPurchaseCycles" dc:"Minimum purchase cycles"`
	Status                string `json:"status" dc:"Final or customer-context-required status"`
	SetupAmount           string `json:"setupAmount" dc:"Exact setup amount in minor currency units"`
	SubtotalAmount        string `json:"subtotalAmount" dc:"Exact pre-tax recurring or one-time subtotal in minor units"`
	TaxAmount             string `json:"taxAmount" dc:"Exact tax amount in minor currency units"`
	TotalAmount           string `json:"totalAmount" dc:"Exact total amount in minor currency units"`
}

// PurchaseServerLoginRules contains generation defaults, never credentials.
type PurchaseServerLoginRules struct {
	Prefix     string   `json:"prefix,omitempty" dc:"Optional generated hostname prefix; manual names do not require this prefix" eg:"srv"`
	Length     int      `json:"length,omitempty" dc:"Generated value length; manual passwords follow supplier limits instead" eg:"12"`
	Characters []string `json:"characters,omitempty" dc:"Generation character classes: lowercase, uppercase, digits or symbols" eg:"[]"`
}

// PurchaseServerLoginPolicy allows independent optional customer inputs.
type PurchaseServerLoginPolicy struct {
	HostnameEnabled bool                     `json:"hostnameEnabled" dc:"Whether customers may enter a hostname; blank generates automatically" eg:"true"`
	PasswordEnabled bool                     `json:"passwordEnabled" dc:"Whether customers may enter a server password; blank generates automatically" eg:"true"`
	Hostname        PurchaseServerLoginRules `json:"hostname" dc:"Nonsecret hostname generation defaults" eg:"{}"`
	Password        PurchaseServerLoginRules `json:"password" dc:"Nonsecret password generation defaults" eg:"{}"`
}
