// This file defines the minimal safe projections shared by automation-interface and fulfillment-binding APIs.

package v1

import "lina-core/pkg/plugin/capability/capmodel"

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
	FieldKey        string   `json:"fieldKey" dc:"Declared customer field identity" eg:"cpu"`
	Minimum         string   `json:"minimum,omitempty" dc:"Enabled integer minimum within provider technical bounds" eg:"1"`
	Maximum         string   `json:"maximum,omitempty" dc:"Enabled integer maximum within provider technical bounds" eg:"16"`
	CandidateValues []string `json:"candidateValues,omitempty" dc:"Explicit subset of declared static candidate values" eg:"unlimited"`
}

type CatalogProductVersionConfiguration struct {
	CustomerConfigurableFields []*capmodel.ConfigurationFieldDeclaration     `json:"customerConfigurableFields" dc:"Optional customer fields; technical limits are not upstream sale packages"`
	CustomerFieldSelections    []*CatalogCustomerFieldSelection              `json:"customerFieldSelections" dc:"Merchant-enabled customer fields and sale bounds"`
	ImportedConfiguration      bool                                          `json:"importedConfiguration" dc:"Whether an immutable imported configuration exists; reimport to change upstream configuration while local sale prices remain editable" eg:"true"`
	ProductDeclaration         *capmodel.ConfigurationSchemaDeclaration      `json:"productDeclaration,omitempty" dc:"Presentation-only frozen merchant declaration with dynamic methods removed"`
	CustomerDeclaration        *capmodel.ConfigurationSchemaDeclaration      `json:"customerDeclaration,omitempty" dc:"Presentation-only frozen customer declaration with dynamic methods removed"`
	MerchantValues             []*CatalogStructuredConfigurationValue        `json:"merchantValues" dc:"Safe typed merchant values"`
	ProductResolvedCandidates  map[string][]*capmodel.ConfigurationCandidate `json:"productResolvedCandidates" dc:"Current safe bilingual merchant-field candidates keyed by field key"`
	CustomerResolvedCandidates map[string][]*capmodel.ConfigurationCandidate `json:"customerResolvedCandidates" dc:"Current safe bilingual initial customer-field candidates keyed by field key"`
	Version                    int64                                         `json:"version" dc:"Optimistic configuration version" eg:"2"`
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
	InterfaceInstanceNo string `json:"interfaceInstanceNo" dc:"Stable automation interface number" eg:"pfi_0123456789abcdef0123456789abcdef"`
	DisplayName         string `json:"displayName" dc:"Merchant-facing interface name" eg:"SSL Provider A"`
	NewBusinessState    string `json:"newBusinessState" dc:"New-business state: accepting or stopped" eg:"accepting"`
	Version             int64  `json:"version" dc:"Optimistic concurrency version" eg:"2"`
	CreatedAt           int64  `json:"createdAt" dc:"Creation time as Unix milliseconds" eg:"1787702400000"`
	ChangedAt           int64  `json:"changedAt" dc:"Last change time as Unix milliseconds" eg:"1787702460000"`
	CanDelete           bool   `json:"canDelete" dc:"Whether this mistaken interface has never been referenced and can be deleted" eg:"false"`
}

type CatalogFulfillmentBindingSummary struct {
	InterfaceInstanceNo       string `json:"interfaceInstanceNo" dc:"Bound automation interface number" eg:"pfi_0123456789abcdef0123456789abcdef"`
	InterfaceDisplayName      string `json:"interfaceDisplayName" dc:"Name frozen when the version was bound" eg:"SSL Provider A"`
	BoundAt                   int64  `json:"boundAt" dc:"Binding time as Unix milliseconds" eg:"1787702400000"`
	EvidenceVerified          bool   `json:"evidenceVerified" dc:"Whether frozen evidence is currently verified" eg:"true"`
	InterfaceNewBusinessState string `json:"interfaceNewBusinessState" dc:"Current interface new-business state" eg:"accepting"`
}
