// This file defines bounded credit directory contracts, filters, and constructors.

package billing

import (
	"context"
	"strings"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/internal/service/principal"
)

const (
	defaultCreditDirectoryLimit       = 20
	maximumCreditDirectoryLimit       = 100
	defaultCreditStatementLineLimit   = 50
	maximumCreditStatementLineLimit   = 100
	maximumCreditOverviewAccountCount = 64
	maximumCreditReviewCount          = 2
	maximumCreditRepaymentPreview     = 100
)

// CreditApplicationDirectoryFilter selects one bounded application page.
type CreditApplicationDirectoryFilter struct {
	Page         int
	CustomerNo   string
	StatusCode   string
	CurrencyCode string
	Cursor       string
	Limit        int
}

// CreditAccountDirectoryFilter selects one bounded account page.
type CreditAccountDirectoryFilter struct {
	Page         int
	CustomerNo   string
	StatusCode   string
	CurrencyCode string
	Cursor       string
	Limit        int
}

// CreditStatementDirectoryFilter selects one bounded statement page.
type CreditStatementDirectoryFilter struct {
	Page         int
	CustomerNo   string
	StatusCode   string
	CurrencyCode string
	Cursor       string
	Limit        int
}

// CreditStatementLineDirectoryFilter selects one bounded immutable statement-line page.
type CreditStatementLineDirectoryFilter struct {
	Cursor string
	Limit  int
}

// CreditApplicationDirectoryItem is a safe application summary for the selected audience.
type CreditApplicationDirectoryItem struct {
	ApplicationNo         string
	CustomerNo            string
	CurrencyCode          string
	PolicyNo              string
	PolicyVersion         int64
	RequestedLimitMinor   int64
	RequestedValidFrom    time.Time
	RequestedValidUntil   time.Time
	RequiredApprovalCount int
	ReviewCount           int
	StatusCode            string
	CreditAccountNo       string
	CreatedAt             time.Time
	UpdatedAt             time.Time
}

// CreditApplicationDirectoryPage carries one bounded page and its authorized total.
type CreditApplicationDirectoryPage struct {
	Total        int
	Applications []CreditApplicationDirectoryItem
	NextCursor   string
}

// CreditApplicationReviewDirectoryItem is one immutable staff review summary.
type CreditApplicationReviewDirectoryItem struct {
	ReviewNo       string
	ReviewSequence int
	DecisionCode   string
	ReviewerID     int
	CreatedAt      time.Time
}

// CreditApplicationDirectoryDetail aggregates one application and its bounded reviews.
type CreditApplicationDirectoryDetail struct {
	Application CreditApplicationDirectoryItem
	Reviews     []CreditApplicationReviewDirectoryItem
}

// CreditAccountTermsDirectoryItem is the immutable current terms projection.
type CreditAccountTermsDirectoryItem struct {
	TermsNo                 string
	TermsVersion            int64
	TotalLimitMinor         int64
	BillingDay              int
	RepaymentPeriodDays     int
	GracePeriodDays         int
	DailyLateFeeBasisPoints int
	MaximumLateFeeMinor     int64
	EffectiveFrom           time.Time
	EffectiveUntil          time.Time
}

// CreditDelinquencyDirectoryItem is the latest delinquency summary for an account.
type CreditDelinquencyDirectoryItem struct {
	CaseNo                 string
	StatementNo            string
	StatusCode             string
	PrincipalMinor         int64
	LateFeeMinor           int64
	OutstandingAmountMinor int64
	OpenedAt               time.Time
	GraceUntil             time.Time
	ResolvedAt             *time.Time
}

// CreditAccountDirectoryItem is a validated account projection for the selected audience.
type CreditAccountDirectoryItem struct {
	CreditAccountNo          string
	CustomerNo               string
	CurrencyCode             string
	StatusCode               string
	TotalLimitMinor          int64
	AvailableLimitMinor      int64
	UnbilledPrincipalMinor   int64
	BilledPrincipalMinor     int64
	DelinquentPrincipalMinor int64
	WrittenOffPrincipalMinor int64
	ValidFrom                time.Time
	ValidUntil               time.Time
	FreezeReasonCode         string
	FrozenAt                 *time.Time
	Version                  int64
	UpdatedAt                time.Time
	Terms                    CreditAccountTermsDirectoryItem
	LatestDelinquency        *CreditDelinquencyDirectoryItem
}

// CreditAccountDirectoryPage carries one bounded page and its authorized total.
type CreditAccountDirectoryPage struct {
	Total      int
	Accounts   []CreditAccountDirectoryItem
	NextCursor string
}

// AccountCreditOverview aggregates bounded current-customer credit facts.
type AccountCreditOverview struct {
	Accounts                []CreditAccountDirectoryItem
	PendingApplicationCount int
	OpenStatementCount      int
	OverdueStatementCount   int
}

// CreditStatementDirectoryItem is a validated statement projection for the selected audience.
type CreditStatementDirectoryItem struct {
	StatementNo            string
	CreditAccountNo        string
	CustomerNo             string
	CurrencyCode           string
	PeriodStartedAt        time.Time
	PeriodEndedAt          time.Time
	LineCount              int
	PrincipalMinor         int64
	FeeMinor               int64
	AdjustmentMinor        int64
	TotalAmountMinor       int64
	PaidAmountMinor        int64
	WrittenOffAmountMinor  int64
	OutstandingAmountMinor int64
	StatusCode             string
	IssuedAt               time.Time
	PaymentDueAt           time.Time
	OverdueAt              *time.Time
	SettledAt              *time.Time
	WrittenOffAt           *time.Time
	Version                int64
	AvailableBalanceMinor  int64
	CanRepayFromBalance    bool
	CreatedAt              time.Time
	UpdatedAt              time.Time
}

// CreditStatementDirectoryPage carries one bounded page and its authorized total.
type CreditStatementDirectoryPage struct {
	Total      int
	Statements []CreditStatementDirectoryItem
	NextCursor string
}

// CreditStatementLineDirectoryItem is one immutable statement line.
type CreditStatementLineDirectoryItem struct {
	StatementLineNo string
	LineNumber      int
	LineTypeCode    string
	SourceTypeCode  string
	SourceReference string
	AmountMinor     int64
	CreatedAt       time.Time
}

// CreditRepaymentDirectoryItem is a bounded, evidence-safe repayment summary.
type CreditRepaymentDirectoryItem struct {
	RepaymentNo            string
	RepaymentTypeCode      string
	FundingSourceTypeCode  string
	AmountMinor            int64
	AllocatedAmountMinor   int64
	UnallocatedAmountMinor int64
	StatusCode             string
	ConfirmedAt            *time.Time
	CreatedAt              time.Time
}

// CreditStatementDirectoryDetail aggregates one statement, a line page, and repayment summaries.
type CreditStatementDirectoryDetail struct {
	Statement           CreditStatementDirectoryItem
	Lines               []CreditStatementLineDirectoryItem
	NextLineCursor      string
	Repayments          []CreditRepaymentDirectoryItem
	RepaymentsTruncated bool
}

// AccountCreditReader exposes only the current active customer's credit facts.
type AccountCreditReader interface {
	// GetAccountCreditOverview returns a fixed-size account and status summary for the current active customer.
	GetAccountCreditOverview(context.Context) (*AccountCreditOverview, error)
	// ListAccountCreditApplications returns one owned encrypted-keyset page or an ownership-safe business error.
	ListAccountCreditApplications(context.Context, CreditApplicationDirectoryFilter) (*CreditApplicationDirectoryPage, error)
	// ListAccountCreditStatements returns one owned encrypted-keyset page with same-currency repayment eligibility.
	ListAccountCreditStatements(context.Context, CreditStatementDirectoryFilter) (*CreditStatementDirectoryPage, error)
	// GetAccountCreditStatement returns one owned statement with bounded lines and repayment summaries.
	GetAccountCreditStatement(context.Context, string, CreditStatementLineDirectoryFilter) (*CreditStatementDirectoryDetail, error)
}

// ManagementCreditReader is the exact-permission staff credit read boundary.
type ManagementCreditReader interface {
	ListManagementCreditEvidence(context.Context, string) ([]CreditEvidenceItem, error)
	OpenManagementCreditEvidence(context.Context, string, string) (*CreditEvidenceOpen, error)
	// ListManagementCreditApplications authorizes and returns one bounded staff application page.
	ListManagementCreditApplications(context.Context, CreditApplicationDirectoryFilter) (*CreditApplicationDirectoryPage, error)
	// GetManagementCreditApplication authorizes the target before returning its bounded review aggregate.
	GetManagementCreditApplication(context.Context, string) (*CreditApplicationDirectoryDetail, error)
	// ListManagementCreditAccounts authorizes and returns one bounded staff account page.
	ListManagementCreditAccounts(context.Context, CreditAccountDirectoryFilter) (*CreditAccountDirectoryPage, error)
	// GetManagementCreditAccount authorizes the target before returning its current account and terms projection.
	GetManagementCreditAccount(context.Context, string) (*CreditAccountDirectoryItem, error)
	// ListManagementCreditStatements authorizes and returns one bounded staff statement page.
	ListManagementCreditStatements(context.Context, CreditStatementDirectoryFilter) (*CreditStatementDirectoryPage, error)
	// GetManagementCreditStatement authorizes the target before returning bounded statement details.
	GetManagementCreditStatement(context.Context, string, CreditStatementLineDirectoryFilter) (*CreditStatementDirectoryDetail, error)
}

type creditDirectoryStore struct {
	database gdb.DB
	cursor   *directoryCursorCodec
}

type accountCreditDirectoryImpl struct {
	store     creditDirectoryStore
	principal principal.Reader
}

type managementCreditDirectoryImpl struct {
	store      creditDirectoryStore
	authorizer ManagementCreditAuthorizer
	evidence   CreditEvidenceService
}

var _ AccountCreditReader = (*accountCreditDirectoryImpl)(nil)
var _ ManagementCreditReader = (*managementCreditDirectoryImpl)(nil)

// NewAccountCreditReader constructs the customer-owned credit directory with an encrypted cursor codec.
func NewAccountCreditReader(
	database gdb.DB,
	principalReader principal.Reader,
	cursorKey []byte,
) (AccountCreditReader, error) {
	if isNilFinanceDirectoryDependency(database) || isNilFinanceDirectoryDependency(principalReader) {
		return nil, gerror.New("account credit directory requires database and principal services")
	}
	codec, err := newDirectoryCursorCodec(cursorKey)
	if err != nil {
		return nil, err
	}
	return &accountCreditDirectoryImpl{
		store: creditDirectoryStore{database: database, cursor: codec}, principal: principalReader,
	}, nil
}

// NewManagementCreditReader constructs the exact-permission staff credit directory.
func NewManagementCreditReader(
	database gdb.DB,
	authorizer ManagementCreditAuthorizer,
	cursorKey []byte,
) (ManagementCreditReader, error) {
	if isNilFinanceDirectoryDependency(database) || isNilFinanceDirectoryDependency(authorizer) {
		return nil, gerror.New("management credit directory requires database and authorizer")
	}
	codec, err := newDirectoryCursorCodec(cursorKey)
	if err != nil {
		return nil, err
	}
	return &managementCreditDirectoryImpl{
		store: creditDirectoryStore{database: database, cursor: codec}, authorizer: authorizer,
	}, nil
}

func NewManagementCreditReaderWithEvidence(database gdb.DB, authorizer ManagementCreditAuthorizer, cursorKey []byte, evidence CreditEvidenceService) (ManagementCreditReader, error) {
	reader, err := NewManagementCreditReader(database, authorizer, cursorKey)
	if err != nil {
		return nil, err
	}
	if isNilFinanceDirectoryDependency(evidence) {
		return nil, gerror.New("management credit directory requires evidence service")
	}
	impl, ok := reader.(*managementCreditDirectoryImpl)
	if !ok {
		return nil, gerror.New("management credit directory implementation is unavailable")
	}
	impl.evidence = evidence
	return impl, nil
}

func (s *managementCreditDirectoryImpl) ListManagementCreditEvidence(ctx context.Context, applicationNo string) ([]CreditEvidenceItem, error) {
	if s.evidence == nil {
		return nil, bizerr.NewCode(CodeCreditUnavailable)
	}
	return s.evidence.ListManagementEvidence(ctx, applicationNo)
}

func (s *managementCreditDirectoryImpl) OpenManagementCreditEvidence(ctx context.Context, applicationNo, uploadNo string) (*CreditEvidenceOpen, error) {
	if s.evidence == nil {
		return nil, bizerr.NewCode(CodeCreditUnavailable)
	}
	return s.evidence.OpenManagementEvidence(ctx, applicationNo, uploadNo)
}

func (s *accountCreditDirectoryImpl) currentCustomer(ctx context.Context) (*principal.Audience, error) {
	if s == nil {
		return nil, bizerr.NewCode(CodeDirectoryNotAccessible)
	}
	return currentFinancialCustomer(ctx, s.principal)
}

func normalizeCreditDirectoryFilter(
	filterCustomerNo string,
	statusCode string,
	currencyCode string,
	cursor string,
	limit int,
) (string, string, string, string, int, error) {
	customerNo, err := normalizeManagementReference(filterCustomerNo)
	if err != nil {
		return "", "", "", "", 0, err
	}
	statusCode, err = normalizeDirectoryStatus(statusCode)
	if err != nil {
		return "", "", "", "", 0, err
	}
	currencyCode, err = normalizeDirectoryCurrency(currencyCode)
	if err != nil {
		return "", "", "", "", 0, err
	}
	cursor = strings.TrimSpace(cursor)
	limit, err = normalizeDirectoryLimit(limit, defaultCreditDirectoryLimit, maximumCreditDirectoryLimit)
	if err != nil || len(cursor) > maximumDirectoryCursorLength {
		return "", "", "", "", 0, bizerr.NewCode(CodeDirectoryInvalid)
	}
	return customerNo, statusCode, currencyCode, cursor, limit, nil
}

func creditDirectoryResultError(err error) error {
	if err == nil {
		return nil
	}
	if _, ok := bizerr.As(err); ok {
		return err
	}
	return bizerr.WrapCode(err, CodeDirectoryUnavailable)
}
