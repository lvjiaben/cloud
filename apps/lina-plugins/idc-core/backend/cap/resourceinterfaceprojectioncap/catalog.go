// This file defines the bounded, read-only sales projection used when a staff
// member selects an upstream product. It grants no catalog write capability and
// deliberately separates base prices from configurable-option surcharges.
package resourceinterfaceprojectioncap

import (
	"context"
	"lina-plugin-idc-core/backend/cap/channelcommercecap"
)

// CatalogPeriod is a connector-proven billing interval, never a display label.
type CatalogPeriod string

const (
	CatalogPeriodOnce  CatalogPeriod = "once"
	CatalogPeriodHour  CatalogPeriod = "hour"
	CatalogPeriodDay   CatalogPeriod = "day"
	CatalogPeriodMonth CatalogPeriod = "month"
	CatalogPeriodYear  CatalogPeriod = "year"
)

// CatalogQuery requests one bounded page of supplier-visible products.
type CatalogQuery struct {
	Page     int    `json:"page"`
	PageSize int    `json:"pageSize"`
	Keyword  string `json:"keyword"`
}

// CatalogProduct contains sale information, not supplier database structures.
type CatalogProduct struct {
	Reference   string `json:"reference" dc:"Opaque supplier product selection value" eg:"101"`
	Name        string `json:"name" dc:"Supplier-authored product name" eg:"Cloud server"`
	Description string `json:"description" dc:"Plain-text product description without HTML or credentials" eg:"General purpose server"`
}

// CatalogPage reports the exact visible, filtered count. Suppliers without a
// paged API must still bound response size and reject incomplete snapshots.
type CatalogPage struct {
	Items []CatalogProduct `json:"items"`
	Total int              `json:"total"`
}

// CatalogProductQuery identifies exactly one product selected by an operator.
type CatalogProductQuery struct {
	Reference string `json:"reference"`
}

// CatalogPrice is an explicit base/option component in major currency units.
// Decimal text preserves upstream precision; the catalog owner resolves currency
// precision and applies the confirmed rounding rule. Missing prices are never 0.
type CatalogPrice struct {
	CurrencyCode string        `json:"currencyCode" dc:"Explicit ISO currency code supplied by the upstream" eg:"CNY"`
	Period       CatalogPeriod `json:"period" dc:"Proven billing interval: once, hour, day, month or year" eg:"month"`
	PeriodCount  int           `json:"periodCount" dc:"Number of billing interval units" eg:"1"`
	Amount       string        `json:"amount" dc:"Exact decimal price component in major currency units, not a complete machine quote" eg:"100.00"`
	SetupFee     string        `json:"setupFee" dc:"Explicit setup fee component in major currency units; unknown is not zero" eg:"0.00"`
}

// CatalogOption describes one supplier-authored configurable choice. References
// are opaque selection values; names do not imply CPU, RAM, region or OS roles.
type CatalogOption struct {
	Reference string          `json:"reference" dc:"Opaque supplier configuration selection value" eg:"10"`
	Name      string          `json:"name" dc:"Supplier-authored configuration label, not an inferred hardware role" eg:"Configuration"`
	Choices   []CatalogChoice `json:"choices" dc:"Bounded supplier choices and their separate prices" eg:"[]"`
}

// CatalogChoice keeps its own price components separate from the product base.
type CatalogChoice struct {
	Reference string         `json:"reference" dc:"Opaque supplier choice selection value" eg:"20"`
	Name      string         `json:"name" dc:"Supplier-authored choice name" eg:"Standard"`
	Prices    []CatalogPrice `json:"prices" dc:"Choice price components, separate from product base prices" eg:"[]"`
}

// CatalogConfigurationValue is one connector-proven scalar in its declared
// product schema. Exactly one value arm is allowed; optionValue represents one
// single-select choice. No names are interpreted as configuration field keys.
type CatalogConfigurationValue struct {
	FieldKey     string  `json:"fieldKey" dc:"Connector-declared product configuration field key" eg:"upstream_product_id"`
	TextValue    *string `json:"textValue,omitempty" dc:"Explicit plain-text field value" eg:"standard"`
	IntegerValue *int64  `json:"integerValue,omitempty" dc:"Explicit integer field value" eg:"2"`
	DecimalValue *string `json:"decimalValue,omitempty" dc:"Explicit decimal field value without floating-point conversion" eg:"1.5"`
	BooleanValue *bool   `json:"booleanValue,omitempty" dc:"Explicit boolean field value" eg:"true"`
	OptionValue  *string `json:"optionValue,omitempty" dc:"Explicit single-select choice from the declared field" eg:"101"`
}

// CatalogDetail is a supplier-authoritative projection for one selected item.
// A price preview or an arbitrary module's cycle label is not a complete base
// schedule and must not be placed in BasePrices.
type CatalogDetail struct {
	// ImportSnapshot is the complete authenticated sale contract used for
	// downstream drafts. It carries no provider credentials and remains absent
	// for connectors that cannot provide a complete portable declaration.
	ImportSnapshot *channelcommercecap.ImportSnapshot `json:"importSnapshot,omitempty"`
	// PeerPurchase retains an upstream evaluated configuration and opaque tokens.
	// It never implies that its initial field view is a complete static schema.
	PeerPurchase *channelcommercecap.ProductSnapshot `json:"peerPurchase,omitempty"`
	// ReadEvidence is attached by the resource owner after decoding the guest
	// response. It never crosses JSON, so neither a guest nor a browser can claim
	// the endpoint/account revision that produced the catalog projection.
	ReadEvidence *CatalogReadEvidence `json:"-"`
	Product      CatalogProduct       `json:"product" dc:"Selected supplier product" eg:"{}"`
	BasePrices   []CatalogPrice       `json:"basePrices" dc:"Explicit product base prices; excludes configuration surcharges" eg:"[]"`
	Options      []CatalogOption      `json:"options" dc:"Supplier configuration choices; no hardware roles are guessed" eg:"[]"`
	// False when any offered period or mandatory fee cannot be represented.
	PriceScheduleComplete bool                        `json:"priceScheduleComplete" dc:"All offered price components were represented; does not imply that configuration mappings are complete" eg:"false"`
	ProductConfiguration  []CatalogConfigurationValue `json:"productConfiguration" dc:"Connector-proven scalar configuration values; missing fields must not be inferred" eg:"[]"`
}

// CatalogReadEvidence binds an in-process sales read to its immutable resource
// configuration revision and the same connector's product-fulfillment evidence.
// The catalog owner persists this relationship through the imported version's
// frozen fulfillment binding and configuration revision, not in guest JSON.
type CatalogReadEvidence struct {
	InterfaceInstanceNo            string
	OwnerID                        string
	ResourceChannelRef             string
	InterfaceConfigurationRevision string
	PluginID                       string
	PluginReleaseVersion           string
	ContractVersion                string
	AuthorizedMethods              []string
	ReleaseDigest                  string
	DeclarationDigest              string
}

// CatalogSource is implemented by the resource owner. Every call must authorize
// a direct staff actor and the target interface before reading supplier data.
// It never receives a merchant price or executes import/publish operations.
type CatalogSource interface {
	ListCatalog(context.Context, string, CatalogQuery) (*CatalogPage, error)
	ReadCatalogProduct(context.Context, string, CatalogProductQuery) (*CatalogDetail, error)
}

// CatalogConfigurationSource evaluates one selected upstream configuration
// through its exact installed interface. Callers must authorize the product
// context; this source additionally applies direct-staff interface scope.
type CatalogConfigurationSource interface {
	EvaluateCatalogConfiguration(context.Context, string, channelcommercecap.ConfigurationInput) (*channelcommercecap.ConfigurationEvaluation, error)
	ReadCatalogBillingTerms(context.Context, string, channelcommercecap.ConfigurationInput) (*channelcommercecap.BillingTerms, error)
}
