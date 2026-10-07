package billing

import (
	"context"
	"strings"
	"time"

	"lina-core/pkg/bizerr"
)

type transactionDirectoryRow struct {
	ID                  int64     `orm:"id"`
	TransactionNo       string    `orm:"transaction_no"`
	TransactionTypeCode string    `orm:"transaction_type_code"`
	CurrencyCode        string    `orm:"currency_code"`
	AccountTypeCode     string    `orm:"account_type_code"`
	AmountMinor         int64     `orm:"amount_minor"`
	ChangeMinor         int64     `orm:"change_minor"`
	RelatedInvoiceNo    string    `orm:"related_invoice_no"`
	PostedAt            time.Time `orm:"posted_at"`
}

func (s *financeDirectoryImpl) ListAccountTransactions(
	ctx context.Context,
	filter TransactionDirectoryFilter,
) (*TransactionDirectoryPage, error) {
	customerNo, err := s.currentCustomer(ctx)
	if err != nil {
		return nil, err
	}
	filter.CustomerNo = customerNo
	filter.CurrencyCode, err = normalizeDirectoryCurrency(filter.CurrencyCode)
	if err != nil {
		return nil, err
	}
	filter.AccountType = strings.TrimSpace(filter.AccountType)
	if filter.AccountType == "" {
		filter.AccountType = AccountTypeCustomerBalance
	}
	filter.Cursor = strings.TrimSpace(filter.Cursor)
	filter.Limit, err = normalizeDirectoryLimit(
		filter.Limit, defaultAccountTransactionLimit, maximumAccountTransactionLimit,
	)
	if err != nil || !validCustomerAccountType(filter.AccountType) ||
		len(filter.Cursor) > maximumDirectoryCursorLength {
		return nil, bizerr.NewCode(CodeDirectoryInvalid)
	}
	digest, err := directoryFilterDigest(struct {
		Kind, CustomerNo, CurrencyCode, AccountType string
		Limit                                       int
	}{"account_transaction", customerNo, filter.CurrencyCode, filter.AccountType, filter.Limit})
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
SELECT transaction.id, transaction.transaction_no,
       transaction.transaction_type_code, transaction.currency_code,
       account.account_type_code, transaction.debit_total_minor AS amount_minor,
       SUM(CASE
           WHEN account.normal_balance_code = 'credit' AND entry.direction_code = 'credit'
               THEN entry.amount_minor
           WHEN account.normal_balance_code = 'credit' AND entry.direction_code = 'debit'
               THEN -entry.amount_minor
           WHEN account.normal_balance_code = 'debit' AND entry.direction_code = 'debit'
               THEN entry.amount_minor
           ELSE -entry.amount_minor
       END) AS change_minor,
       COALESCE(MAX(entry.invoice_no), '') AS related_invoice_no,
       transaction.posted_at
FROM plugin_idc_core_billing_ledger_transaction transaction
JOIN plugin_idc_core_billing_ledger_entry entry
  ON entry.tenant_id = transaction.tenant_id
 AND entry.transaction_no = transaction.transaction_no
 AND entry.currency_code = transaction.currency_code
JOIN plugin_idc_core_billing_ledger_account account
  ON account.tenant_id = entry.tenant_id
 AND account.account_no = entry.account_no
 AND account.currency_code = entry.currency_code
WHERE transaction.tenant_id = 0 AND account.customer_no = ?
  AND account.account_type_code = ?`
	arguments := []any{customerNo, filter.AccountType}
	if filter.CurrencyCode != "" {
		query += ` AND transaction.currency_code = ?`
		arguments = append(arguments, filter.CurrencyCode)
	}
	if boundary != nil {
		boundaryTime := time.UnixMicro(boundary.SortTimeMicroseconds)
		query += ` AND ((transaction.posted_at < ?) OR
                    (transaction.posted_at = ? AND transaction.id < ?))`
		arguments = append(arguments, boundaryTime, boundaryTime, boundary.ID)
	}
	query += `
GROUP BY transaction.id, transaction.transaction_no,
         transaction.transaction_type_code, transaction.currency_code,
         account.account_type_code, transaction.debit_total_minor,
         transaction.posted_at
ORDER BY transaction.posted_at DESC, transaction.id DESC
LIMIT ?`
	arguments = append(arguments, filter.Limit+1)
	rows := make([]transactionDirectoryRow, 0, filter.Limit+1)
	if err = s.database.GetScan(ctx, &rows, query, arguments...); err != nil {
		return nil, bizerr.WrapCode(err, CodeDirectoryUnavailable)
	}
	hasMore := len(rows) > filter.Limit
	if hasMore {
		rows = rows[:filter.Limit]
	}
	page := &TransactionDirectoryPage{
		Transactions: make([]TransactionDirectoryItem, 0, len(rows)),
	}
	for _, row := range rows {
		if row.ID < 1 || row.PostedAt.IsZero() || row.AmountMinor <= 0 ||
			!validCustomerAccountType(row.AccountTypeCode) {
			return nil, bizerr.NewCode(CodeDirectoryIntegrity)
		}
		page.Transactions = append(page.Transactions, TransactionDirectoryItem{
			TransactionNo: row.TransactionNo, TransactionTypeCode: row.TransactionTypeCode,
			CurrencyCode: row.CurrencyCode, AccountTypeCode: row.AccountTypeCode,
			AmountMinor: row.AmountMinor, ChangeMinor: row.ChangeMinor,
			RelatedInvoiceNo: row.RelatedInvoiceNo, PostedAt: row.PostedAt.UTC(),
		})
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
