package billing

import (
	"context"
	"math"
	"strings"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
)

const maximumManagementCurrencyCount = 64
const maximumManagementDirectoryLimit = 100

type ManagementInvoiceDirectoryFilter struct {
	Page         int
	CustomerNo   string
	StatusCode   string
	CurrencyCode string
	Cursor       string
	Limit        int
}

type ManagementTransactionDirectoryFilter struct {
	Page                int
	CustomerNo          string
	TransactionTypeCode string
	CurrencyCode        string
	Cursor              string
	Limit               int
}

type ManagementAdjustmentDirectoryFilter struct {
	Page               int
	CustomerNo         string
	AdjustmentTypeCode string
	StatusCode         string
	CurrencyCode       string
	Cursor             string
	Limit              int
}

type ManagementReceivableSummary struct {
	CurrencyCode     string
	OutstandingMinor int64
}

type ManagementFinancialOverview struct {
	OpenInvoiceCount       int
	OverdueInvoiceCount    int
	PendingAdjustmentCount int
	Receivables            []ManagementReceivableSummary
}

type ManagementTransactionDirectoryItem struct {
	TransactionNo        string
	TransactionTypeCode  string
	CustomerNo           string
	CustomerCode         string
	CustomerDisplayName  string
	CustomerPrimaryPhone string
	CurrencyCode         string
	AmountMinor          int64
	RelatedInvoiceNo     string
	EntryCount           int
	PostedAt             time.Time
}

type ManagementTransactionDirectoryPage struct {
	Total        int
	Transactions []ManagementTransactionDirectoryItem
	NextCursor   string
}

type ManagementAdjustmentDirectoryItem struct {
	AdjustmentNo         string
	AdjustmentTypeCode   string
	StatusCode           string
	CustomerNo           string
	CustomerCode         string
	CustomerDisplayName  string
	CustomerPrimaryPhone string
	InvoiceNo            string
	CurrencyCode         string
	AmountMinor          int64
	ReasonCode           string
	ApprovalRequired     bool
	RequestedBy          int
	LedgerTransactionNo  string
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

type ManagementAdjustmentDirectoryPage struct {
	Total       int
	Adjustments []ManagementAdjustmentDirectoryItem
	NextCursor  string
}

// ManagementFinanceReader is the exact-permission staff financial read boundary.
type ManagementFinanceReader interface {
	GetManagementFinancialOverview(context.Context) (*ManagementFinancialOverview, error)
	ListManagementInvoices(context.Context, ManagementInvoiceDirectoryFilter) (*InvoiceDirectoryPage, error)
	GetManagementInvoice(context.Context, string, InvoiceItemDirectoryFilter) (*InvoiceDirectoryDetail, error)
	ListManagementTransactions(context.Context, ManagementTransactionDirectoryFilter) (*ManagementTransactionDirectoryPage, error)
	GetManagementTransaction(context.Context, string, InvoiceItemDirectoryFilter) (*ManagementTransactionDetail, error)
	ListManagementAdjustments(context.Context, ManagementAdjustmentDirectoryFilter) (*ManagementAdjustmentDirectoryPage, error)
	GetManagementAdjustment(context.Context, string) (*ManagementAdjustmentDetail, error)
}

type managementFinanceDirectoryImpl struct {
	database   gdb.DB
	authorizer ManagementFinanceAuthorizer
	cursor     *directoryCursorCodec
}

var _ ManagementFinanceReader = (*managementFinanceDirectoryImpl)(nil)

func NewManagementFinanceReader(
	database gdb.DB,
	authorizer ManagementFinanceAuthorizer,
	cursorKey []byte,
) (ManagementFinanceReader, error) {
	if isNilFinanceDirectoryDependency(database) || isNilFinanceDirectoryDependency(authorizer) {
		return nil, gerror.New("management financial directory requires database and authorizer")
	}
	codec, err := newDirectoryCursorCodec(cursorKey)
	if err != nil {
		return nil, err
	}
	return &managementFinanceDirectoryImpl{database: database, authorizer: authorizer, cursor: codec}, nil
}

func normalizeManagementReference(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value != "" && !validBusinessReference(value, 40) {
		return "", bizerr.NewCode(CodeDirectoryInvalid)
	}
	return value, nil
}

func normalizeManagementType(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value != "" && !sourceTypePattern.MatchString(value) {
		return "", bizerr.NewCode(CodeDirectoryInvalid)
	}
	return value, nil
}

func validManagementPage(page, limit int, cursor string) bool {
	return page >= 0 && limit > 0 && limit <= maximumManagementDirectoryLimit &&
		(page == 0 || (cursor == "" && page-1 <= math.MaxInt/limit))
}

// readManagementDirectoryPage reads the authorized filtered count and the bounded
// page from one snapshot. Callers authorize first and reuse their exact filters.
func (s *managementFinanceDirectoryImpl) readManagementDirectoryPage(
	ctx context.Context, rows any, countQuery string, countArgs []any, query string, args []any,
) (total int, err error) {
	err = s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		if _, readErr := s.database.Exec(ctx, "SET TRANSACTION ISOLATION LEVEL REPEATABLE READ, READ ONLY"); readErr != nil {
			return readErr
		}
		count, readErr := s.database.GetValue(ctx, countQuery, countArgs...)
		if readErr != nil {
			return readErr
		}
		total = count.Int()
		return s.database.GetScan(ctx, rows, query, args...)
	})
	return total, err
}
