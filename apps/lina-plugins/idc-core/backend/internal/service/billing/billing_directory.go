package billing

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"reflect"
	"regexp"
	"strings"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/internal/service/principal"
)

const (
	defaultAccountInvoiceLimit     = 20
	maximumAccountInvoiceLimit     = 50
	defaultAccountTransactionLimit = 20
	maximumAccountTransactionLimit = 50
	defaultAccountServiceLimit     = 20
	maximumAccountServiceLimit     = 50
	defaultInvoiceItemLimit        = 50
	maximumInvoiceItemLimit        = 100
	maximumAccountBalanceCount     = 64
)

var directoryStatusPattern = regexp.MustCompile(`^[a-z][a-z0-9_]{0,31}$`)

// InvoiceDirectoryFilter describes one bounded invoice page.
type InvoiceDirectoryFilter struct {
	CustomerNo   string
	StatusCode   string
	CurrencyCode string
	Cursor       string
	Limit        int
}

// TransactionDirectoryFilter describes one customer-account ledger page.
type TransactionDirectoryFilter struct {
	CustomerNo   string
	CurrencyCode string
	AccountType  string
	Cursor       string
	Limit        int
}

// RenewalServiceDirectoryFilter describes one bounded renewable-service page.
type RenewalServiceDirectoryFilter struct {
	CustomerNo string
	StatusCode string
	Cursor     string
	Limit      int
}

// InvoiceItemDirectoryFilter describes one bounded invoice-item page.
type InvoiceItemDirectoryFilter struct {
	Cursor string
	Limit  int
}

// BalanceDirectoryItem is one current customer-owned account projection.
type BalanceDirectoryItem struct {
	AccountNo             string
	AccountTypeCode       string
	CurrencyCode          string
	StatusCode            string
	Withdrawable          bool
	BalanceMinor          int64
	HeldBalanceMinor      int64
	AvailableBalanceMinor int64
	UpdatedAt             time.Time
}

// InvoiceDirectoryItem is one receivable list row without internal digests.
type InvoiceDirectoryItem struct {
	InvoiceNo              string
	CustomerNo             string
	CustomerCode           string
	CustomerDisplayName    string
	CustomerPrimaryPhone   string
	OrderNo                string
	SourceCode             string
	CurrencyCode           string
	StatusCode             string
	ItemCount              int
	TotalAmountMinor       int64
	PaidAmountMinor        int64
	WaivedAmountMinor      int64
	CreditAmountMinor      int64
	OutstandingAmountMinor int64
	RefundableAmountMinor  int64
	RefundedAmountMinor    int64
	PaymentDueAt           time.Time
	IssuedAt               *time.Time
	SettledAt              *time.Time
	CreatedAt              time.Time
	CanPayFromBalance      bool
	CanMerge               bool
}

// InvoiceDirectoryPage is an encrypted-keyset page without an expensive total.
type InvoiceDirectoryPage struct {
	Total      int
	Invoices   []InvoiceDirectoryItem
	NextCursor string
}

// InvoiceItemDirectoryItem is one immutable customer-safe receivable line.
type InvoiceItemDirectoryItem struct {
	InvoiceItemNo    string
	LineNumber       int
	ItemTypeCode     string
	SourceReference  string
	DescriptionZhCN  string
	DescriptionEnUS  string
	SubtotalMinor    int64
	DiscountMinor    int64
	NetAmountMinor   int64
	TaxAmountMinor   int64
	TotalAmountMinor int64
}

// InvoiceDirectoryDetail combines one invoice with one bounded item page.
type InvoiceDirectoryDetail struct {
	Invoice    InvoiceDirectoryItem
	Items      []InvoiceItemDirectoryItem
	NextCursor string
}

// TransactionDirectoryItem is the effect of one immutable transaction on the
// selected customer account type, not an arbitrary journal-entry projection.
type TransactionDirectoryItem struct {
	TransactionNo       string
	TransactionTypeCode string
	CurrencyCode        string
	AccountTypeCode     string
	AmountMinor         int64
	ChangeMinor         int64
	RelatedInvoiceNo    string
	PostedAt            time.Time
}

// TransactionDirectoryPage is an encrypted-keyset ledger page.
type TransactionDirectoryPage struct {
	Transactions []TransactionDirectoryItem
	NextCursor   string
}

// RenewalServiceDirectoryItem is one customer-owned long-term entitlement with
// its current immutable automatic-renewal policy and latest attempt outcome.
type RenewalServiceDirectoryItem struct {
	EntitlementNo                string
	ProductNameZhCN              string
	ProductNameEnUS              string
	CurrencyCode                 string
	EntitlementStatusCode        string
	RenewalModeCode              string
	PeriodUnitCode               string
	PeriodCount                  int
	EntitlementExpiresAt         time.Time
	EntitlementVersion           int64
	PolicyNo                     string
	PolicyVersion                int64
	AutoRenewEnabled             bool
	RenewalCycles                int
	LeadSeconds                  int64
	MaximumAttempts              int
	RetryIntervalSeconds         int64
	FailureNotificationEnabled   bool
	LatestAttemptOutcomeCode     string
	LatestAttemptFailureCode     string
	LatestAttemptAt              *time.Time
	NextAttemptAt                *time.Time
	CanConfigureAutomaticRenewal bool
}

// RenewalServiceDirectoryPage is an encrypted-keyset renewable-service page.
type RenewalServiceDirectoryPage struct {
	Services   []RenewalServiceDirectoryItem
	NextCursor string
}

// AccountFinancialOverview is a bounded, single-request customer dashboard.
type AccountFinancialOverview struct {
	Balances                 []BalanceDirectoryItem
	OpenInvoiceCount         int
	OpenReceivableByCurrency map[string]int64
	AutomaticRenewalEnabled  int
	RechargeAvailable        bool
	RechargeReasonCode       string
}

// AccountFinanceReader owns authenticated customer financial read models.
type AccountFinanceReader interface {
	GetAccountFinancialOverview(context.Context) (*AccountFinancialOverview, error)
	ListAccountInvoices(context.Context, InvoiceDirectoryFilter) (*InvoiceDirectoryPage, error)
	GetAccountInvoice(context.Context, string, InvoiceItemDirectoryFilter) (*InvoiceDirectoryDetail, error)
	ListAccountTransactions(context.Context, TransactionDirectoryFilter) (*TransactionDirectoryPage, error)
	ListAccountRenewalServices(context.Context, RenewalServiceDirectoryFilter) (*RenewalServiceDirectoryPage, error)
}

type financeDirectoryImpl struct {
	database  gdb.DB
	principal principal.Reader
	cursor    *directoryCursorCodec
}

var _ AccountFinanceReader = (*financeDirectoryImpl)(nil)

// NewAccountFinanceReader constructs a customer-only local PostgreSQL read model.
func NewAccountFinanceReader(
	database gdb.DB,
	principalReader principal.Reader,
	cursorKey []byte,
) (AccountFinanceReader, error) {
	if isNilFinanceDirectoryDependency(database) || isNilFinanceDirectoryDependency(principalReader) {
		return nil, gerror.New("account financial directory requires database and principal services")
	}
	codec, err := newDirectoryCursorCodec(cursorKey)
	if err != nil {
		return nil, err
	}
	return &financeDirectoryImpl{database: database, principal: principalReader, cursor: codec}, nil
}

func isNilFinanceDirectoryDependency(value any) bool {
	if value == nil {
		return true
	}
	reflected := reflect.ValueOf(value)
	switch reflected.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return reflected.IsNil()
	default:
		return false
	}
}

func (s *financeDirectoryImpl) currentCustomer(ctx context.Context) (string, error) {
	if s == nil {
		return "", bizerr.NewCode(CodeDirectoryNotAccessible)
	}
	audience, err := currentFinancialCustomer(ctx, s.principal)
	if err != nil {
		return "", err
	}
	return audience.CustomerNo, nil
}

func currentFinancialCustomer(
	ctx context.Context,
	principalReader principal.Reader,
) (*principal.Audience, error) {
	if principalReader == nil {
		return nil, bizerr.NewCode(CodeDirectoryNotAccessible)
	}
	audience, err := principalReader.Current(ctx)
	if err != nil || audience == nil || audience.Code != principal.AudienceCustomer ||
		audience.Status != principal.AudienceStatusActive || audience.UserID < 1 ||
		!validBusinessReference(audience.CustomerNo, 40) {
		return nil, bizerr.NewCode(CodeDirectoryNotAccessible)
	}
	return audience, nil
}

func normalizeDirectoryLimit(limit, fallback, maximum int) (int, error) {
	if limit == 0 {
		return fallback, nil
	}
	if limit < 1 || limit > maximum {
		return 0, bizerr.NewCode(CodeDirectoryInvalid)
	}
	return limit, nil
}

func directoryFilterDigest(value any) ([sha256.Size]byte, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return [sha256.Size]byte{}, gerror.Wrap(err, "encode billing directory filter")
	}
	return sha256.Sum256(encoded), nil
}

func normalizeDirectoryCurrency(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value != "" && !currencyPattern.MatchString(value) {
		return "", bizerr.NewCode(CodeDirectoryInvalid)
	}
	return value, nil
}

func normalizeDirectoryStatus(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value != "" && !directoryStatusPattern.MatchString(value) {
		return "", bizerr.NewCode(CodeDirectoryInvalid)
	}
	return value, nil
}

func optionalDirectoryString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
