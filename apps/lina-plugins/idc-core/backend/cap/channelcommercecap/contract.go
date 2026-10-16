// Package channelcommercecap defines the source-only contract used by the
// downstream-channel plugin to reuse core-owned customer, commerce, finance,
// and service lifecycle authorities.
package channelcommercecap

import (
	"context"
	"encoding/json"
	"time"
)

const (
	OwnerPluginID         = "idc-core"
	AllowedCallerPluginID = "idc-channel"
)

type PurchaseValue struct {
	FieldToken          string   `json:"fieldToken"`
	Type                string   `json:"type"`
	TextValue           *string  `json:"textValue"`
	IntegerValue        *string  `json:"integerValue"`
	DecimalValue        *string  `json:"decimalValue"`
	BooleanValue        *bool    `json:"booleanValue"`
	DateValue           *string  `json:"dateValue"`
	DateTimeValue       *string  `json:"dateTimeValue"`
	OptionTokens        []string `json:"optionTokens"`
	TreeOptionTokens    []string `json:"treeOptionTokens"`
	AddressOptionTokens []string `json:"addressOptionTokens"`
	FileTokens          []string `json:"fileTokens"`
}

type Purchase struct {
	PublicSlug            string          `json:"publicSlug"`
	ConfigurationRevision string          `json:"configurationRevision"`
	BillingTermToken      string          `json:"billingTermToken"`
	CurrencyCode          string          `json:"currencyCode"`
	PurchaseCycles        int             `json:"purchaseCycles"`
	Quantity              int             `json:"quantity"`
	Values                []PurchaseValue `json:"values"`
}

type QuoteInput struct {
	ChannelID, IdempotencyKey string
	PayerCustomerNo           string
	Purchase                  Purchase
}

type Quote struct {
	QuoteNo          string
	CurrencyCode     string
	SubtotalMinor    int64
	DiscountMinor    int64
	TaxAmountMinor   int64
	TotalAmountMinor int64
	ExpiresAt        time.Time
}

type DelegatedCustomerInput struct {
	ChannelID        string
	ExternalID       string
	CustomerType     string
	DisplayName      string
	CountryCode      string
	PreferredLocale  string
	Email            string
	PhoneCountryCode string
	Phone            string
	IdempotencyKey   string
}

type DelegatedCustomer struct {
	CustomerNo       string
	Status           string
	IdentityStatus   string
	InvitationNo     string
	InvitationExpiry time.Time
	Reused           bool
}

type CreateOrderInput struct {
	QuoteNo                string
	ChannelID              string
	ExternalCustomerID     string
	OwnershipMode          string
	PayerCustomerNo        string
	ServiceOwnerCustomerNo string
	IdempotencyKey         string
	SettlementMethod       string
	Purchase               Purchase
}

type Order struct {
	OrderNo                string
	InvoiceNo              string
	PayerCustomerNo        string
	ServiceOwnerCustomerNo string
	CurrencyCode           string
	Status                 string
	TotalAmountMinor       int64
	FinanciallySettled     bool
	IdentityReady          bool
	Reused                 bool
}

type SettleOrderInput struct {
	ChannelID        string
	OrderNo          string
	IdempotencyKey   string
	SettlementMethod string
}

type CancelOrderInput struct {
	ChannelID      string
	OrderNo        string
	IdempotencyKey string
}

type LifecycleInput struct {
	ChannelID      string
	ServiceNo      string
	Operation      string
	IdempotencyKey string
	Parameters     json.RawMessage
	Confirmed      bool
}

type LifecycleResult struct {
	OperationNo     string
	QuoteNo         string
	Status          string
	CurrencyCode    string
	Direction       string
	AmountMinor     int64
	EffectiveAt     *time.Time
	ExpiresAt       *time.Time
	PaymentRequired bool
	OutputAvailable bool
	Reused          bool
}

type ConfidentialOutputConsumer func([]byte) error

type FinanceCurrency struct {
	CurrencyCode          string
	BalanceMinor          int64
	HeldBalanceMinor      int64
	AvailableBalanceMinor int64
	CreditLimitMinor      int64
	CreditAvailableMinor  int64
	ReceivableMinor       int64
	PaidMinor             int64
	RefundedMinor         int64
	ServiceCostMinor      int64
}

type FinanceOverview struct {
	Currencies       []FinanceCurrency
	Invoices         []FinanceInvoice
	CreditStatements []CreditStatement
	CreditRepayments []CreditRepayment
	InvoiceTotal     int
	StatementTotal   int
	RepaymentTotal   int
}

// FinancePage selects a bounded numeric page within one financial collection.
type FinancePage struct {
	Page     int
	PageSize int
}

// FinancePages keeps each reconciliation table's pagination independent.
type FinancePages struct {
	Invoices   FinancePage
	Statements FinancePage
	Repayments FinancePage
}

type FinanceInvoice struct {
	InvoiceNo           string
	OrderNo             string
	CurrencyCode        string
	Status              string
	TotalAmountMinor    int64
	PaidAmountMinor     int64
	CreditAmountMinor   int64
	RefundedAmountMinor int64
	CreatedAt           time.Time
}

type CreditStatement struct {
	StatementNo      string
	CurrencyCode     string
	Status           string
	TotalMinor       int64
	PaidMinor        int64
	OutstandingMinor int64
	IssuedAt         time.Time
	PaymentDueAt     time.Time
}

type CreditRepayment struct {
	RepaymentNo    string
	StatementNo    string
	CurrencyCode   string
	Status         string
	AmountMinor    int64
	AllocatedMinor int64
	CreatedAt      time.Time
}

type RepayCreditInput struct {
	ChannelID       string
	PayerCustomerNo string
	StatementNo     string
	AmountMinor     int64
	IdempotencyKey  string
}

type RepayCreditResult struct {
	RepaymentNo           string
	StatementNo           string
	CurrencyCode          string
	Status                string
	AmountMinor           int64
	OutstandingMinor      int64
	AvailableBalanceMinor int64
	Reused                bool
}

type ServiceListInput struct {
	OrderNo         string
	AfterServiceNo  string
	ChannelID       string
	PayerCustomerNo string
	ServiceNo       string
	Limit           int
}

type ServiceItem struct {
	ServiceNo              string
	ServiceOwnerCustomerNo string
	OrderNo                string
	ProductNo              string
	VersionNo              string
	CurrencyCode           string
	Status                 string
	RenewalMode            string
	StartedAt              time.Time
	ExpiresAt              *time.Time
	UpdatedAt              time.Time
}

type CustomerSubject struct {
	CustomerNo       string
	CustomerTypeCode string
	Status           string
	IdentityStatus   string
	ActorUserID      int
}

type Runtime interface {
	BrowseCatalog(context.Context, CatalogQuery) (*CatalogPage, error)
	CurrentCustomer(context.Context) (*CustomerSubject, error)
	// ValidatePayer checks the active customer and login binding for credential
	// issuance; verification evidence is returned, not required here. Transaction
	// owners retain their independent identity and payment gates.
	ValidatePayer(context.Context, string) (*CustomerSubject, error)
	CreateDelegatedCustomer(context.Context, DelegatedCustomerInput) (*DelegatedCustomer, error)
	Quote(context.Context, QuoteInput) (*Quote, error)
	CreateOrder(context.Context, CreateOrderInput) (*Order, error)
	GetOrder(context.Context, string, string) (*Order, error)
	SettleOrder(context.Context, SettleOrderInput) (*Order, error)
	CancelOrder(context.Context, CancelOrderInput) (*Order, error)
	GetLifecycle(context.Context, string, string, string) (*LifecycleResult, error)
	Lifecycle(context.Context, LifecycleInput) (*LifecycleResult, error)
	RedeemLifecycleOutput(context.Context, string, string, string, ConfidentialOutputConsumer) error
	FinanceOverview(context.Context, string, string, ...FinancePages) (*FinanceOverview, error)
	RepayCredit(context.Context, RepayCreditInput) (*RepayCreditResult, error)
	ListServices(context.Context, ServiceListInput) ([]ServiceItem, error)
}

// CatalogQuery bounds public product discovery. Prices are obtained by quoting
// a configuration as the authenticated customer, never from stale publications.
type CatalogQuery struct {
	NamePrefix, Currency, Cursor string
	Limit                        int
}
type CatalogProduct struct {
	PublicSlug   string   `json:"publicSlug"`
	NameZhCN     string   `json:"nameZhCn"`
	NameEnUS     string   `json:"nameEnUs"`
	SummaryZhCN  string   `json:"summaryZhCn"`
	SummaryEnUS  string   `json:"summaryEnUs"`
	Currencies   []string `json:"currencies"`
	Availability string   `json:"availability"`
}
type CatalogPage struct {
	Products            []CatalogProduct `json:"products"`
	AvailableCurrencies []string         `json:"availableCurrencies"`
	NextCursor          string           `json:"nextCursor"`
}
