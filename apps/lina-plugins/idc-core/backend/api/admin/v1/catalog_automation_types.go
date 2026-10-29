// This file defines the minimal safe projections shared by automation-interface and fulfillment-binding APIs.

package v1

import (
	"encoding/json"
	"lina-core/pkg/plugin/capability/capmodel"
)

type CatalogAutomationProviderItem struct {
	SelectionToken         string                                   `json:"selectionToken" dc:"Short-lived authenticated provider selection token" eg:"opaque-selection-token"`
	DisplayName            string                                   `json:"displayName" dc:"Provider display name" eg:"SSL Certificate Provider"`
	InterfaceConfiguration *capmodel.ConfigurationSchemaDeclaration `json:"interfaceConfiguration,omitempty" dc:"Host-governed structured interface configuration declaration"`
}

// CatalogStructuredConfigurationValue carries exactly one explicit typed arm.
// The service rejects unknown fields, duplicate fields, and multiple arms.
type CatalogStructuredConfigurationValue struct {
	FieldKey            string   `json:"fieldKey" v:"required|max-length:64" dc:"Stable field key declared by the selected provider"`
	TextValue           *string  `json:"textValue,omitempty" dc:"Single-line, multi-line, or interface-secret input"`
	IntegerValue        *string  `json:"integerValue,omitempty" dc:"Exact base-10 integer input without JavaScript precision loss" eg:"9223372036854775807"`
	DecimalValue        *string  `json:"decimalValue,omitempty" dc:"Exact decimal input without floating-point conversion"`
	BooleanValue        *bool    `json:"booleanValue,omitempty" dc:"Boolean input"`
	DateValue           *string  `json:"dateValue,omitempty" dc:"ISO date input"`
	DateTimeValue       *string  `json:"dateTimeValue,omitempty" dc:"RFC3339 datetime input"`
	OptionTokens        []string `json:"optionTokens,omitempty" dc:"Single-select or multi-select option tokens"`
	TreeOptionTokens    []string `json:"treeOptionTokens,omitempty" dc:"Host-governed tree option tokens"`
	AddressOptionTokens []string `json:"addressOptionTokens,omitempty" dc:"Host-governed address option tokens"`
	FileTokens          []string `json:"fileTokens,omitempty" dc:"Host-governed file references"`
}

type CatalogConfigurationSecretStatus struct {
	FieldKey   string `json:"fieldKey" dc:"Stable field key declared by the provider"`
	Configured bool   `json:"configured" dc:"Whether a secret is currently configured" eg:"true"`
	MaskedTail string `json:"maskedTail" dc:"Bounded display-only secret suffix" eg:"1234"`
	UpdatedAt  int64  `json:"updatedAt" dc:"Secret update time as Unix milliseconds" eg:"1787702400000"`
}

type CatalogAutomationInterfaceConfiguration struct {
	Declaration        *capmodel.ConfigurationSchemaDeclaration      `json:"declaration,omitempty" dc:"Presentation-only provider interface declaration with dynamic methods removed"`
	Values             []*CatalogStructuredConfigurationValue        `json:"values" dc:"Safe typed non-secret values"`
	Secrets            []*CatalogConfigurationSecretStatus           `json:"secrets" dc:"Redacted secret status without handles or plaintext"`
	ResolvedCandidates map[string][]*capmodel.ConfigurationCandidate `json:"resolvedCandidates" dc:"Current safe bilingual dynamic candidates keyed by field key"`
	Revision           int64                                         `json:"revision" dc:"Latest immutable configuration revision" eg:"2"`
}

type CatalogCustomerFieldSelection struct {
	CustomerVisible        *bool           `json:"customerVisible,omitempty" dc:"Whether customers can edit this field; false uses the validated default value" eg:"false"`
	DefaultValue           json.RawMessage `json:"defaultValue,omitempty" dc:"Typed default used for a hidden purchase field; must satisfy its sellable bounds" eg:"2"`
	UpgradeEnabled         *bool           `json:"upgradeEnabled,omitempty" dc:"Whether this configuration field may change during a paid upgrade; omitted preserves existing behavior" eg:"true"`
	UpgradeMinimum         string          `json:"upgradeMinimum,omitempty" dc:"Optional minimum integer target for a configuration upgrade, within the sellable range" eg:"2"`
	UpgradeMaximum         string          `json:"upgradeMaximum,omitempty" dc:"Optional maximum integer target for a configuration upgrade, within the sellable range" eg:"16"`
	UpgradeCandidateValues []string        `json:"upgradeCandidateValues,omitempty" dc:"Optional allowed target choices for configuration upgrades; empty permits all sellable choices; at most 512 values" eg:"[]"`

	ProductChoiceBindings map[string]string                       `json:"productChoiceBindings,omitempty" dc:"Optional explicit upstream values keyed by local single-choice identifiers; at most 512 bindings of 256 bytes each; requires an explicit provisioning parameter binding and does not change commercial prices" eg:"{}"`
	InformationOnly       bool                                    `json:"informationOnly,omitempty" dc:"Explicitly resolve a retained configuration as customer information only; true cannot accompany a product parameter binding" eg:"true"`
	ProductFieldKey       string                                  `json:"productFieldKey,omitempty" dc:"Explicit provisioning API parameter name; declared fields retain their type checks, custom native names and contents are frozen for direct provisioning; empty records customer information only" eg:"region"`
	NumericPricing        string                                  `json:"numericPricing,omitempty" dc:"Numeric pricing policy: quantity keeps unit or band prices, per_choice uses independent fixed specification prices; omitted preserves existing behavior" eg:"quantity"`
	GroupZhCN             string                                  `json:"groupZhCN,omitempty" dc:"Configuration group Chinese label" eg:"Specifications"`
	GroupEnUS             string                                  `json:"groupEnUS,omitempty" dc:"Configuration group English label" eg:"Specifications"`
	Unit                  string                                  `json:"unit,omitempty" dc:"Merchant display suffix without parameter conversion" eg:"GiB"`
	DescriptionZhCN       string                                  `json:"descriptionZhCN,omitempty" dc:"Merchant Chinese field description" eg:"Choose a specification"`
	DescriptionEnUS       string                                  `json:"descriptionEnUS,omitempty" dc:"Merchant English field description" eg:"Choose a specification"`
	Advanced              bool                                    `json:"advanced,omitempty" dc:"Whether the purchase field is in advanced settings" eg:"false"`
	NumericLabels         map[string]*capmodel.LocalizedText      `json:"numericLabels,omitempty" dc:"Optional bilingual display labels keyed by numeric choices" eg:"{}"`
	LocalField            *capmodel.ConfigurationFieldDeclaration `json:"localField,omitempty" dc:"Merchant-owned typed purchase field with an automatically generated local_ key; only explicitly bound contents are sent to the provisioning API" eg:"{}"`

	DisplayControl  string                             `json:"displayControl,omitempty" v:"in:auto,input,slider,select,radio" dc:"Optional local purchase control: auto, input, slider, select or radio" eg:"slider"`
	QuantityStep    string                             `json:"quantityStep,omitempty" v:"max-length:16" dc:"Optional exact positive integer step, anchored at the configured minimum; defaults to one" eg:"1"`
	NumericChoices  []string                           `json:"numericChoices,omitempty"  dc:"Optional local integer choices for select or radio controls, within the sale range and step" eg:"[]"`
	FieldKey        string                             `json:"fieldKey" dc:"Declared customer field identity" eg:"cpu"`
	Minimum         string                             `json:"minimum,omitempty" dc:"Enabled integer minimum within provider technical bounds" eg:"1"`
	Maximum         string                             `json:"maximum,omitempty" dc:"Enabled integer maximum within provider technical bounds" eg:"16"`
	CandidateValues []string                           `json:"candidateValues,omitempty" dc:"Ordered sellable subset of declared or synchronized candidate values" eg:"unlimited"`
	CandidateLabels map[string]*capmodel.LocalizedText `json:"candidateLabels,omitempty" dc:"Optional bilingual labels for the ordered merchant choice values" eg:"{}"`
	DisplayNameZhCN string                             `json:"displayNameZhCN,omitempty" dc:"Merchant Chinese display name; empty preserves the connector label" eg:"CPU cores"`
	DisplayNameEnUS string                             `json:"displayNameEnUS,omitempty" dc:"Merchant English display name; empty preserves the connector label" eg:"CPU cores"`
	SortOrder       int                                `json:"sortOrder,omitempty" dc:"Optional display order for enabled merchant fields, from zero to 1024" eg:"1"`
}

type CatalogProductVersionConfiguration struct {
	ImportedProductName        string                                              `json:"importedProductName,omitempty" dc:"Read-only supplier product name from the immutable source evidence" eg:"Cloud server"`
	ImportedProductReference   string                                              `json:"importedProductReference,omitempty" dc:"Read-only selected upstream product reference, retained across internal edit versions" eg:"101"`
	PendingAutomationFields    []string                                            `json:"pendingAutomationFields,omitempty" dc:"Read-only retained fields needing explicit parameter mapping or information-only selection after an interface change; at most 128 field keys" eg:"[]"`
	DirectoryRelationships     []*CatalogDirectoryRelationship                     `json:"directoryRelationships,omitempty" dc:"Read-only connector choice relationships for the current local directory; at most 16 relationships and 8192 value pairs" eg:"[]"`
	CustomerChoiceRules        map[string][]*capmodel.ConfigurationRuleDeclaration `json:"customerChoiceRules,omitempty" dc:"Read-only frozen commercial candidate filters keyed by customer field; never accepted as update input" eg:"{}"`
	MerchantPriceTargets       []*CatalogMerchantPriceTarget                       `json:"merchantPriceTargets,omitempty" dc:"At most 4096 authorized editor choices joined to existing local price dimensions; no credentials or cost data" eg:"[]"`
	ProductChoiceFields        []string                                            `json:"productChoiceFields,omitempty" dc:"Declared product text parameters supporting validated merchant string choices; absent disables this binding" eg:"system_disk_io_limit"`
	CustomerProductFields      map[string]string                                   `json:"customerProductFields,omitempty" dc:"Product parameters supplied by declared customer fields; provider retains unit conversion" eg:"{}"`
	PriceFieldTokens           map[string]string                                   `json:"priceFieldTokens,omitempty" dc:"Read-only opaque price-field identities keyed by editable configuration field; distinguishes equal display labels" eg:"{}"`
	UpstreamOptionsStatus      string                                              `json:"upstreamOptionsStatus,omitempty" dc:"Supplier directory synchronization outcome: ready, pending, failed, retained" eg:"failed"`
	CustomerConfigurableFields []*capmodel.ConfigurationFieldDeclaration           `json:"customerConfigurableFields" dc:"Optional customer fields; technical limits are not upstream sale packages"`
	CustomerFieldSelections    []*CatalogCustomerFieldSelection                    `json:"customerFieldSelections" dc:"Merchant-enabled customer fields and sale bounds"`
	ImportedPricingMode        string                                              `json:"importedPricingMode,omitempty" dc:"Saved resale price policy: markup, manual, or mixed when periods use different policies" eg:"markup"`
	ImportedMarkupPercent      string                                              `json:"importedMarkupPercent,omitempty" dc:"Exact saved uplift percent when every imported period shares one percentage" eg:"10"`
	ImportedManualPrices       []CatalogImportPrice                                `json:"importedManualPrices" dc:"Saved whole-item prices for manually priced imported periods; excludes tax and lists setup separately" eg:"[]"`
	ImportedConfiguration      bool                                                `json:"importedConfiguration" dc:"Whether an immutable imported configuration exists; reimport to change upstream configuration while local sale prices remain editable" eg:"true"`
	ProductDeclaration         *capmodel.ConfigurationSchemaDeclaration            `json:"productDeclaration,omitempty" dc:"Presentation-only frozen merchant declaration with dynamic methods removed"`
	CustomerDeclaration        *capmodel.ConfigurationSchemaDeclaration            `json:"customerDeclaration,omitempty" dc:"Presentation-only frozen customer declaration with dynamic methods removed"`
	MerchantValues             []*CatalogStructuredConfigurationValue              `json:"merchantValues" dc:"Safe typed merchant values"`
	ProductResolvedCandidates  map[string][]*capmodel.ConfigurationCandidate       `json:"productResolvedCandidates" dc:"Current safe bilingual merchant-field candidates keyed by field key"`
	CustomerResolvedCandidates map[string][]*capmodel.ConfigurationCandidate       `json:"customerResolvedCandidates" dc:"Current safe bilingual initial customer-field candidates keyed by field key"`
	Version                    int64                                               `json:"version" dc:"Optimistic configuration version" eg:"2"`
}

type CatalogConfigurationFieldState struct {
	Visible  bool `json:"visible" dc:"Whether the field is currently visible" eg:"true"`
	Required bool `json:"required" dc:"Whether the field is currently required" eg:"true"`
	Disabled bool `json:"disabled" dc:"Whether the field is currently disabled" eg:"false"`
}

type CatalogConfigurationIssue struct {
	FieldKey  string `json:"fieldKey,omitempty" dc:"Field key related to the stable issue, when applicable" eg:"certificateType"`
	IssueCode string `json:"issueCode" dc:"Stable client-localizable validation issue code" eg:"required"`
}

type CatalogConfigurationEvaluation struct {
	Values      []*CatalogStructuredConfigurationValue        `json:"values" dc:"Authoritative normalized non-secret typed values"`
	FieldStates map[string]*CatalogConfigurationFieldState    `json:"fieldStates" dc:"Authoritative field states keyed by field key"`
	Candidates  map[string][]*capmodel.ConfigurationCandidate `json:"candidates" dc:"Authoritative condition-filtered safe bilingual candidates keyed by field key"`
	Issues      []*CatalogConfigurationIssue                  `json:"issues" dc:"Stable validation issues without provider technical details"`
	Valid       bool                                          `json:"valid" dc:"Whether the current complete value set is valid" eg:"true"`
}

type CatalogAutomationInterfaceItem struct {
	CatalogAvailable    bool   `json:"catalogAvailable" dc:"Whether the selected interface declares both bounded upstream product directory methods" eg:"true"`
	InterfaceInstanceNo string `json:"interfaceInstanceNo" dc:"Stable automation interface number" eg:"pfi_0123456789abcdef0123456789abcdef"`
	DisplayName         string `json:"displayName" dc:"Merchant-facing interface name" eg:"SSL Provider A"`
	NewBusinessState    string `json:"newBusinessState" dc:"New-business state: accepting or stopped" eg:"accepting"`
	Version             int64  `json:"version" dc:"Optimistic concurrency version" eg:"2"`
	CreatedAt           int64  `json:"createdAt" dc:"Creation time as Unix milliseconds" eg:"1787702400000"`
	ChangedAt           int64  `json:"changedAt" dc:"Last change time as Unix milliseconds" eg:"1787702460000"`
	CanDelete           bool   `json:"canDelete" dc:"Whether this mistaken interface has never been referenced and can be deleted" eg:"false"`
}

type CatalogFulfillmentBindingSummary struct {
	CatalogAvailable          bool   `json:"catalogAvailable" dc:"Whether this bound interface supports upstream product selection" eg:"true"`
	InterfaceInstanceNo       string `json:"interfaceInstanceNo" dc:"Bound automation interface number" eg:"pfi_0123456789abcdef0123456789abcdef"`
	InterfaceDisplayName      string `json:"interfaceDisplayName" dc:"Name frozen when the version was bound" eg:"SSL Provider A"`
	BoundAt                   int64  `json:"boundAt" dc:"Binding time as Unix milliseconds" eg:"1787702400000"`
	EvidenceVerified          bool   `json:"evidenceVerified" dc:"Whether frozen evidence is currently verified" eg:"true"`
	InterfaceNewBusinessState string `json:"interfaceNewBusinessState" dc:"Current interface new-business state" eg:"accepting"`
}
