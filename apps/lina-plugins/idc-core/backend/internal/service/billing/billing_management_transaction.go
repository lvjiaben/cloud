package billing

import (
	"context"
	"strings"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/internal/service/customerprojection"
)

type managementTransactionRow struct {
	ID                  int64     `orm:"id"`
	TransactionNo       string    `orm:"transaction_no"`
	TransactionTypeCode string    `orm:"transaction_type_code"`
	CustomerNo          string    `orm:"customer_no"`
	CustomerCount       int       `orm:"customer_count"`
	CurrencyCode        string    `orm:"currency_code"`
	AmountMinor         int64     `orm:"amount_minor"`
	RelatedInvoiceNo    string    `orm:"related_invoice_no"`
	EntryCount          int       `orm:"entry_count"`
	PostedAt            time.Time `orm:"posted_at"`
}

type ManagementTransactionEntry struct {
	EntryNo         string
	LineNumber      int
	AccountNo       string
	OwnerTypeCode   string
	OwnerReference  string
	CustomerNo      string
	AccountTypeCode string
	DirectionCode   string
	EntryRoleCode   string
	AmountMinor     int64
	InvoiceNo       string
	InvoiceItemNo   string
	ReasonCode      string
}

type ManagementTransactionDetail struct {
	TransactionNo         string
	TransactionTypeCode   string
	CurrencyCode          string
	SourceTypeCode        string
	SourceReference       string
	OriginalTransactionNo string
	EntryCount            int
	AmountMinor           int64
	PostedBy              int
	PostedAt              time.Time
	Entries               []ManagementTransactionEntry
	NextCursor            string
}

type managementTransactionEntryRow struct {
	ID              int64   `orm:"id"`
	EntryNo         string  `orm:"entry_no"`
	LineNumber      int     `orm:"line_number"`
	AccountNo       string  `orm:"account_no"`
	OwnerTypeCode   string  `orm:"owner_type_code"`
	OwnerReference  string  `orm:"owner_reference"`
	CustomerNo      *string `orm:"customer_no"`
	AccountTypeCode string  `orm:"account_type_code"`
	DirectionCode   string  `orm:"direction_code"`
	EntryRoleCode   string  `orm:"entry_role_code"`
	AmountMinor     int64   `orm:"amount_minor"`
	InvoiceNo       *string `orm:"invoice_no"`
	InvoiceItemNo   *string `orm:"invoice_item_no"`
	ReasonCode      string  `orm:"reason_code"`
}

func (s *managementFinanceDirectoryImpl) ListManagementTransactions(
	ctx context.Context,
	filter ManagementTransactionDirectoryFilter,
) (*ManagementTransactionDirectoryPage, error) {
	if _, err := s.authorizer.AuthorizeTransactionList(ctx); err != nil {
		return nil, err
	}
	var err error
	filter.CustomerNo, err = normalizeManagementReference(filter.CustomerNo)
	if err != nil {
		return nil, err
	}
	filter.TransactionTypeCode, err = normalizeManagementType(filter.TransactionTypeCode)
	if err != nil {
		return nil, err
	}
	filter.CurrencyCode, err = normalizeDirectoryCurrency(filter.CurrencyCode)
	if err != nil {
		return nil, err
	}
	filter.Cursor = strings.TrimSpace(filter.Cursor)
	filter.Limit, err = normalizeDirectoryLimit(filter.Limit, defaultAccountTransactionLimit, maximumManagementDirectoryLimit)
	if err != nil || len(filter.Cursor) > maximumDirectoryCursorLength || !validManagementPage(filter.Page, filter.Limit, filter.Cursor) {
		return nil, bizerr.NewCode(CodeDirectoryInvalid)
	}
	digest, err := directoryFilterDigest(struct {
		Kind, CustomerNo, TransactionTypeCode, CurrencyCode string
		Limit                                               int
	}{"management_transaction", filter.CustomerNo, filter.TransactionTypeCode, filter.CurrencyCode, filter.Limit})
	if err != nil {
		return nil, bizerr.WrapCode(err, CodeDirectoryUnavailable)
	}
	var boundary *directoryCursorBoundary
	if filter.Cursor != "" {
		boundary, err = s.cursor.decode(filter.Cursor, digest)
		if err != nil {
			return nil, bizerr.NewCode(CodeDirectoryInvalid)
		}
	}
	query := `
WITH candidates AS (
    SELECT transaction.id, transaction.transaction_no,
           transaction.transaction_type_code, transaction.currency_code,
           transaction.debit_total_minor AS amount_minor,
           transaction.entry_count, transaction.posted_at
    FROM plugin_idc_core_billing_ledger_transaction transaction
    WHERE transaction.tenant_id = 0`
	arguments := make([]any, 0, 12)
	if filter.TransactionTypeCode != "" {
		query += ` AND transaction.transaction_type_code = ?`
		arguments = append(arguments, filter.TransactionTypeCode)
	}
	if filter.CurrencyCode != "" {
		query += ` AND transaction.currency_code = ?`
		arguments = append(arguments, filter.CurrencyCode)
	}
	if filter.CustomerNo != "" {
		query += ` AND EXISTS (
        SELECT 1
        FROM plugin_idc_core_billing_ledger_entry entry
        LEFT JOIN plugin_idc_core_billing_ledger_account account
          ON account.tenant_id = entry.tenant_id AND account.account_no = entry.account_no
        LEFT JOIN plugin_idc_core_billing_invoice invoice
          ON invoice.tenant_id = entry.tenant_id AND invoice.invoice_no = entry.invoice_no
        WHERE entry.tenant_id = transaction.tenant_id
          AND entry.transaction_no = transaction.transaction_no
          AND COALESCE(account.customer_no, invoice.customer_no) = ?
    )`
		arguments = append(arguments, filter.CustomerNo)
	}
	countQuery, countArgs := query+") SELECT COUNT(*) FROM candidates", append([]any(nil), arguments...)
	if boundary != nil {
		boundaryTime := time.UnixMicro(boundary.SortTimeMicroseconds)
		query += ` AND ((transaction.posted_at < ?) OR
        (transaction.posted_at = ? AND transaction.id < ?))`
		arguments = append(arguments, boundaryTime, boundaryTime, boundary.ID)
	}
	query += ` ORDER BY transaction.posted_at DESC, transaction.id DESC LIMIT ? OFFSET ?
)
SELECT candidate.id, candidate.transaction_no, candidate.transaction_type_code,
       candidate.currency_code, candidate.amount_minor, candidate.entry_count,
       candidate.posted_at,
       COALESCE(facts.customer_no, '') AS customer_no,
       COALESCE(facts.customer_count, 0) AS customer_count,
       COALESCE(facts.related_invoice_no, '') AS related_invoice_no
FROM candidates candidate
LEFT JOIN LATERAL (
    SELECT MIN(COALESCE(account.customer_no, invoice.customer_no)) AS customer_no,
           COUNT(DISTINCT COALESCE(account.customer_no, invoice.customer_no)) AS customer_count,
           MAX(entry.invoice_no) AS related_invoice_no
    FROM plugin_idc_core_billing_ledger_entry entry
    LEFT JOIN plugin_idc_core_billing_ledger_account account
      ON account.tenant_id = entry.tenant_id AND account.account_no = entry.account_no
    LEFT JOIN plugin_idc_core_billing_invoice invoice
      ON invoice.tenant_id = entry.tenant_id AND invoice.invoice_no = entry.invoice_no
    WHERE entry.tenant_id = 0 AND entry.transaction_no = candidate.transaction_no
) facts ON TRUE
ORDER BY candidate.posted_at DESC, candidate.id DESC`
	arguments = append(arguments, filter.Limit+1, max(filter.Page-1, 0)*filter.Limit)
	rows := make([]managementTransactionRow, 0, filter.Limit+1)
	total, err := s.readManagementDirectoryPage(ctx, &rows, countQuery, countArgs, query, arguments)
	if err != nil {
		return nil, bizerr.WrapCode(err, CodeDirectoryUnavailable)
	}
	hasMore := len(rows) > filter.Limit
	if hasMore {
		rows = rows[:filter.Limit]
	}
	page := &ManagementTransactionDirectoryPage{Transactions: make([]ManagementTransactionDirectoryItem, 0, len(rows)), Total: total}
	for _, row := range rows {
		if row.ID < 1 || row.PostedAt.IsZero() || row.AmountMinor <= 0 || row.EntryCount < 2 || row.CustomerCount > 1 {
			return nil, bizerr.NewCode(CodeDirectoryIntegrity)
		}
		page.Transactions = append(page.Transactions, ManagementTransactionDirectoryItem{
			TransactionNo: row.TransactionNo, TransactionTypeCode: row.TransactionTypeCode,
			CustomerNo: row.CustomerNo, CurrencyCode: row.CurrencyCode,
			AmountMinor: row.AmountMinor, RelatedInvoiceNo: row.RelatedInvoiceNo,
			EntryCount: row.EntryCount, PostedAt: row.PostedAt.UTC(),
		})
	}
	customerNos := managementTransactionCustomerNos(page.Transactions)
	customers, projectionErr := customerprojection.Load(ctx, s.database, customerNos)
	if projectionErr != nil {
		return nil, bizerr.WrapCode(projectionErr, CodeDirectoryUnavailable)
	}
	for index := range page.Transactions {
		if page.Transactions[index].CustomerNo == "" {
			continue
		}
		customer := customers[page.Transactions[index].CustomerNo]
		page.Transactions[index].CustomerCode = customer.CustomerCode
		page.Transactions[index].CustomerDisplayName = customer.DisplayName
		page.Transactions[index].CustomerPrimaryPhone = customer.PrimaryPhone
	}
	if hasMore && len(rows) > 0 {
		last := rows[len(rows)-1]
		page.NextCursor, err = s.cursor.encode(directoryCursorBoundary{
			SortTimeMicroseconds: last.PostedAt.UnixMicro(), ID: last.ID,
		}, digest)
		if err != nil {
			return nil, bizerr.WrapCode(err, CodeDirectoryUnavailable)
		}
	}
	return page, nil
}

func managementTransactionCustomerNos(items []ManagementTransactionDirectoryItem) []string {
	customerNos := make([]string, 0, len(items))
	for _, item := range items {
		if item.CustomerNo != "" {
			customerNos = append(customerNos, item.CustomerNo)
		}
	}
	return customerNos
}

func (s *managementFinanceDirectoryImpl) GetManagementTransaction(
	ctx context.Context,
	transactionNo string,
	filter InvoiceItemDirectoryFilter,
) (*ManagementTransactionDetail, error) {
	transactionNo = strings.TrimSpace(transactionNo)
	if _, err := s.authorizer.AuthorizeTransactionDetail(ctx, transactionNo); err != nil {
		return nil, err
	}
	filter.Cursor = strings.TrimSpace(filter.Cursor)
	var err error
	filter.Limit, err = normalizeDirectoryLimit(filter.Limit, defaultInvoiceItemLimit, maximumInvoiceItemLimit)
	if err != nil || !validBusinessReference(transactionNo, 40) || len(filter.Cursor) > maximumDirectoryCursorLength {
		return nil, bizerr.NewCode(CodeDirectoryInvalid)
	}
	digest, err := directoryFilterDigest(struct {
		Kind, TransactionNo string
		Limit               int
	}{"management_transaction_entries", transactionNo, filter.Limit})
	if err != nil {
		return nil, bizerr.WrapCode(err, CodeDirectoryUnavailable)
	}
	var boundary *directoryCursorBoundary
	if filter.Cursor != "" {
		boundary, err = s.cursor.decode(filter.Cursor, digest)
		if err != nil {
			return nil, bizerr.NewCode(CodeDirectoryInvalid)
		}
	}
	result := &ManagementTransactionDetail{Entries: []ManagementTransactionEntry{}}
	err = s.database.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		if _, setErr := tx.Exec("SET TRANSACTION ISOLATION LEVEL REPEATABLE READ, READ ONLY"); setErr != nil {
			return setErr
		}
		var header *struct {
			TransactionNo         string     `orm:"transaction_no"`
			TransactionTypeCode   string     `orm:"transaction_type_code"`
			CurrencyCode          string     `orm:"currency_code"`
			SourceTypeCode        string     `orm:"source_type_code"`
			SourceReference       string     `orm:"source_reference"`
			OriginalTransactionNo *string    `orm:"original_transaction_no"`
			EntryCount            int        `orm:"entry_count"`
			AmountMinor           int64      `orm:"debit_total_minor"`
			PostedBy              int        `orm:"posted_by"`
			PostedAt              *time.Time `orm:"posted_at"`
		}
		if loadErr := tx.GetScan(&header, `
SELECT transaction_no, transaction_type_code, currency_code, source_type_code,
       source_reference, original_transaction_no, entry_count,
       debit_total_minor, posted_by, posted_at
FROM plugin_idc_core_billing_ledger_transaction
WHERE tenant_id = 0 AND transaction_no = ?`, transactionNo); loadErr != nil {
			return gerror.Wrap(loadErr, "load management transaction detail")
		}
		if header == nil || header.PostedAt == nil || header.EntryCount < 2 || header.AmountMinor <= 0 {
			return bizerr.NewCode(CodeDirectoryNotAccessible)
		}
		result.TransactionNo = header.TransactionNo
		result.TransactionTypeCode = header.TransactionTypeCode
		result.CurrencyCode = header.CurrencyCode
		result.SourceTypeCode = header.SourceTypeCode
		result.SourceReference = header.SourceReference
		result.OriginalTransactionNo = optionalDirectoryString(header.OriginalTransactionNo)
		result.EntryCount = header.EntryCount
		result.AmountMinor = header.AmountMinor
		result.PostedBy = header.PostedBy
		result.PostedAt = header.PostedAt.UTC()
		query := `
SELECT entry.id, entry.entry_no, entry.line_number, entry.account_no,
       account.owner_type_code, account.owner_reference, account.customer_no,
       account.account_type_code, entry.direction_code, entry.entry_role_code,
       entry.amount_minor, entry.invoice_no, entry.invoice_item_no, entry.reason_code
FROM plugin_idc_core_billing_ledger_entry entry
JOIN plugin_idc_core_billing_ledger_account account
  ON account.tenant_id = entry.tenant_id AND account.account_no = entry.account_no
WHERE entry.tenant_id = 0 AND entry.transaction_no = ?`
		arguments := []any{transactionNo}
		if boundary != nil {
			query += ` AND ((entry.line_number > ?) OR (entry.line_number = ? AND entry.id > ?))`
			arguments = append(arguments, boundary.SortTimeMicroseconds, boundary.SortTimeMicroseconds, boundary.ID)
		}
		query += ` ORDER BY entry.line_number ASC, entry.id ASC LIMIT ?`
		arguments = append(arguments, filter.Limit+1)
		rows := make([]managementTransactionEntryRow, 0, filter.Limit+1)
		if loadErr := tx.GetScan(&rows, query, arguments...); loadErr != nil {
			return gerror.Wrap(loadErr, "load management transaction entries")
		}
		hasMore := len(rows) > filter.Limit
		if hasMore {
			rows = rows[:filter.Limit]
		}
		for _, row := range rows {
			if row.ID < 1 || row.LineNumber < 1 || row.AmountMinor <= 0 {
				return bizerr.NewCode(CodeDirectoryIntegrity)
			}
			result.Entries = append(result.Entries, ManagementTransactionEntry{
				EntryNo: row.EntryNo, LineNumber: row.LineNumber, AccountNo: row.AccountNo,
				OwnerTypeCode: row.OwnerTypeCode, OwnerReference: row.OwnerReference,
				CustomerNo: optionalDirectoryString(row.CustomerNo), AccountTypeCode: row.AccountTypeCode,
				DirectionCode: row.DirectionCode, EntryRoleCode: row.EntryRoleCode,
				AmountMinor: row.AmountMinor, InvoiceNo: optionalDirectoryString(row.InvoiceNo),
				InvoiceItemNo: optionalDirectoryString(row.InvoiceItemNo), ReasonCode: row.ReasonCode,
			})
		}
		if hasMore && len(rows) > 0 {
			last := rows[len(rows)-1]
			result.NextCursor, err = s.cursor.encode(directoryCursorBoundary{
				SortTimeMicroseconds: int64(last.LineNumber), ID: last.ID,
			}, digest)
			return err
		}
		return nil
	})
	if err != nil {
		if _, ok := bizerr.As(err); ok {
			return nil, err
		}
		return nil, bizerr.WrapCode(err, CodeDirectoryUnavailable)
	}
	return result, nil
}
