// Configuration context is a read-only source seam. It exposes only opaque
// public compiler projections and never supplier keys or saved file credentials.
package servercommercialcap

import "context"

// ConfigurationContextRequest omits Values to restore the current frozen values;
// a non-nil Values slice reevaluates a complete public edit under its revision.
type ConfigurationContextRequest struct {
	Target                TargetSnapshot
	ConfigurationRevision string
	Values                []TypedPurchaseValue
}

// ConfigurationLocalizedText is a public bilingual display value.
type ConfigurationLocalizedText struct{ ZhCN, EnUS string }

// ConfigurationOption is an allowed opaque choice, recursively projected.
type ConfigurationOption struct {
	OptionToken string
	Label       ConfigurationLocalizedText
	Children    []ConfigurationOption
}

// ConfigurationConstraints contains the existing public compiler limits.
type ConfigurationConstraints struct {
	Format, Minimum, Maximum     string
	MinimumLength, MaximumLength int
}

// ConfigurationUploadPolicy describes new uploads, never prior credentials.
type ConfigurationUploadPolicy struct {
	MaximumFiles                        int
	MaximumFileBytes, MaximumTotalBytes int64
	AllowedMIMETypes, AllowedExtensions []string
	SafetyScanRequired                  bool
}

// ConfigurationField mirrors the existing safe public presentation.
type ConfigurationField struct {
	GroupZhCN, GroupEnUS, Unit   string
	Advanced                     bool
	NumericLabels                map[string]*ConfigurationLocalizedText
	DisplayControl, QuantityStep string
	NumericChoices               []string
	FieldToken, Type             string
	Label, Description           ConfigurationLocalizedText
	Visible, Required, Disabled  bool
	Constraints                  ConfigurationConstraints
	Options                      []ConfigurationOption
	UploadPolicy                 *ConfigurationUploadPolicy
}

// ConfigurationIssue identifies a public field and stable validation issue.
type ConfigurationIssue struct{ FieldToken, IssueCode string }

// ConfigurationContext binds a public form to the exact current entitlement and
// frozen configuration. BillingTermToken is empty until the edit is valid; it is
// an explanatory current-period selector, never a final difference quote.
type ConfigurationContext struct {
	EntitlementNo, StructuredConfigurationNo, ConfigurationRevision, CurrencyCode, BillingTermToken string
	EntitlementVersion                                                                              int64
	PurchaseCycles                                                                                  int
	Ready                                                                                           bool
	Fields                                                                                          []ConfigurationField
	Values                                                                                          []TypedPurchaseValue
	Issues                                                                                          []ConfigurationIssue
}

// ConfigurationContextReader optionally extends the source service without
// forcing unrelated service substitutes to implement a configuration reader.
type ConfigurationContextReader interface {
	// ReadConfigurationContext authorizes the exact current owner or employee,
	// reevaluates allowed public choices and rechecks ownership/version after I/O.
	ReadConfigurationContext(context.Context, ConfigurationContextRequest) (*ConfigurationContext, error)
}
