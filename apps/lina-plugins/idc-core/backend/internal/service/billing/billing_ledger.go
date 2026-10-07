// This file persists immutable double-entry ledger transactions and projections.

package billing

import (
	"context"
	"io"
	"math"
	"sort"
	"strings"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
)

const (
	sourceConfirmedRecharge     = "confirmed_recharge"
	sourceBalanceConsumption    = "balance_consumption"
	sourceInvoiceBalancePayment = "invoice_balance_payment"
	sourceHoldConsumption       = "balance_hold_consume"
	sourceLedgerReversal        = "ledger_reversal"
)

type accountSpec struct {
	OwnerType      string
	OwnerReference string
	CustomerNo     string
	AccountType    string
	NormalBalance  string
	Withdrawable   bool
}

type accountRow struct {
	AccountNo      string `orm:"account_no"`
	OwnerType      string `orm:"owner_type_code"`
	OwnerReference string `orm:"owner_reference"`
	CustomerNo     string `orm:"customer_no"`
	AccountType    string `orm:"account_type_code"`
	NormalBalance  string `orm:"normal_balance_code"`
	CurrencyCode   string `orm:"currency_code"`
	StatusCode     string `orm:"status_code"`
}

type transactionRow struct {
	ID                    int64  `orm:"id"`
	TransactionNo         string `orm:"transaction_no"`
	TransactionType       string `orm:"transaction_type_code"`
	CurrencyCode          string `orm:"currency_code"`
	SourceType            string `orm:"source_type_code"`
	SourceReference       string `orm:"source_reference"`
	SourceSnapshotDigest  string `orm:"source_snapshot_digest"`
	OriginalTransactionNo string `orm:"original_transaction_no"`
	EntryCount            int    `orm:"entry_count"`
	DebitTotalMinor       int64  `orm:"debit_total_minor"`
	CreditTotalMinor      int64  `orm:"credit_total_minor"`
	ContentDigest         string `orm:"content_digest"`
}

type entrySemantic struct {
	AccountNo     string `json:"account_no"`
	Direction     string `json:"direction"`
	Role          string `json:"role"`
	AmountMinor   int64  `json:"amount_minor"`
	InvoiceNo     string `json:"invoice_no,omitempty"`
	InvoiceItemNo string `json:"invoice_item_no,omitempty"`
	Reason        string `json:"reason"`
}

type transactionSemantic struct {
	TransactionType       string          `json:"transaction_type"`
	CurrencyCode          string          `json:"currency_code"`
	SourceType            string          `json:"source_type"`
	SourceReference       string          `json:"source_reference"`
	SourceSnapshotDigest  string          `json:"source_snapshot_digest"`
	OriginalTransactionNo string          `json:"original_transaction_no,omitempty"`
	Entries               []entrySemantic `json:"entries"`
}

type latestSnapshotRow struct {
	SnapshotNo          string `orm:"snapshot_no"`
	AccountNo           string `orm:"account_no"`
	SequenceNo          int64  `orm:"sequence_no"`
	ClosingBalanceMinor int64  `orm:"closing_balance_minor"`
}

type preparedEntry struct {
	EntryNo    string
	LineNumber int
	entrySemantic
	ContentDigest string
}

type preparedSnapshot struct {
	SnapshotNo          string
	AccountNo           string
	NormalBalance       string
	SequenceNo          int64
	PreviousSnapshotNo  any
	PreviousSequenceNo  any
	OpeningBalanceMinor int64
	DebitAmountMinor    int64
	CreditAmountMinor   int64
	ClosingBalanceMinor int64
	ContentDigest       string
}

func customerBalanceSpec(customerNo string) accountSpec {
	return accountSpec{
		OwnerType: "customer", OwnerReference: customerNo, CustomerNo: customerNo,
		AccountType: AccountTypeCustomerBalance, NormalBalance: "credit",
	}
}

func merchantCashSpec(merchantNo string) accountSpec {
	return accountSpec{
		OwnerType: "merchant", OwnerReference: merchantNo,
		AccountType: "merchant_cash", NormalBalance: "debit",
	}
}

func merchantRevenueSpec(merchantNo string) accountSpec {
	return accountSpec{
		OwnerType: "merchant", OwnerReference: merchantNo,
		AccountType: "revenue", NormalBalance: "credit",
	}
}

func (s *serviceImpl) lockSource(ctx context.Context, sourceType string, sourceReference string) error {
	_, err := s.database.GetValue(ctx,
		`SELECT pg_advisory_xact_lock(hashtextextended(?, 64122062))`,
		sourceType+":"+sourceReference,
	)
	if err != nil {
		return gerror.Wrap(err, "lock billing source")
	}
	return nil
}

func (s *serviceImpl) loadMerchantNo(ctx context.Context, requireEnabled bool) (string, error) {
	rows := make([]struct {
		MerchantNo string `orm:"merchant_no"`
		Enabled    bool   `orm:"enabled"`
	}, 0, 2)
	if err := s.database.GetScan(ctx, &rows, `
SELECT merchant_no, enabled FROM plugin_idc_core_merchant_profile
WHERE tenant_id = 0 ORDER BY id ASC LIMIT 2`); err != nil {
		return "", gerror.Wrap(err, "load billing merchant identity")
	}
	if len(rows) != 1 || !validBusinessReference(rows[0].MerchantNo, 40) {
		return "", bizerr.NewCode(CodeUnavailable)
	}
	if requireEnabled && !rows[0].Enabled {
		return "", bizerr.NewCode(CodeConflict)
	}
	return rows[0].MerchantNo, nil
}

func (s *serviceImpl) requireActiveCustomer(ctx context.Context, customerNo string) error {
	count, err := s.database.GetValue(ctx, `
SELECT count(*) FROM plugin_idc_core_customer_subject
WHERE tenant_id = 0 AND customer_no = ? AND status_code = 'active'`, customerNo)
	if err != nil {
		return gerror.Wrap(err, "validate billing customer")
	}
	if count.Int() != 1 {
		return bizerr.NewCode(CodeConflict)
	}
	return nil
}

func (s *serviceImpl) ensureAndLockAccounts(
	ctx context.Context,
	currency string,
	requireActive bool,
	specs ...accountSpec,
) (map[string]*accountRow, error) {
	for _, spec := range specs {
		accountNo := deterministicAccountNo(spec.OwnerType, spec.OwnerReference, spec.AccountType, currency)
		var customerNo any
		if spec.CustomerNo != "" {
			customerNo = spec.CustomerNo
		}
		if _, err := s.database.Exec(ctx, `
INSERT INTO plugin_idc_core_billing_ledger_account (
    tenant_id, account_no, owner_type_code, owner_reference, customer_no,
    account_type_code, normal_balance_code, currency_code, withdrawable
) VALUES (0, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT (tenant_id, owner_type_code, owner_reference, account_type_code, currency_code)
DO NOTHING`, accountNo, spec.OwnerType, spec.OwnerReference, customerNo,
			spec.AccountType, spec.NormalBalance, currency, spec.Withdrawable); err != nil {
			return nil, gerror.Wrap(err, "initialize billing ledger account")
		}
	}
	accountNumbers := make([]string, 0, len(specs))
	for _, spec := range specs {
		accountNumbers = append(accountNumbers,
			deterministicAccountNo(spec.OwnerType, spec.OwnerReference, spec.AccountType, currency))
	}
	sort.Strings(accountNumbers)
	accounts := make(map[string]*accountRow, len(accountNumbers))
	for _, accountNo := range accountNumbers {
		var row *accountRow
		if err := s.database.GetScan(ctx, &row, `
SELECT account_no, owner_type_code, owner_reference, COALESCE(customer_no, '') AS customer_no,
       account_type_code, normal_balance_code, currency_code, status_code
FROM plugin_idc_core_billing_ledger_account
WHERE tenant_id = 0 AND account_no = ? FOR UPDATE`, accountNo); err != nil {
			return nil, gerror.Wrap(err, "lock billing ledger account")
		}
		if row == nil || row.CurrencyCode != currency {
			return nil, bizerr.NewCode(CodeIntegrity)
		}
		accounts[accountNo] = row
	}
	for _, spec := range specs {
		accountNo := deterministicAccountNo(spec.OwnerType, spec.OwnerReference, spec.AccountType, currency)
		row := accounts[accountNo]
		if row == nil || row.OwnerType != spec.OwnerType || row.OwnerReference != spec.OwnerReference ||
			row.CustomerNo != spec.CustomerNo || row.AccountType != spec.AccountType ||
			row.NormalBalance != spec.NormalBalance || (requireActive && row.StatusCode != "active") {
			return nil, bizerr.NewCode(CodeConflict)
		}
	}
	return accounts, nil
}

func (s *serviceImpl) lockExistingAccounts(
	ctx context.Context,
	accountNumbers []string,
	requireActive bool,
) (map[string]*accountRow, error) {
	ordered := append([]string(nil), accountNumbers...)
	sort.Strings(ordered)
	deduplicated := ordered[:0]
	for _, accountNo := range ordered {
		if len(deduplicated) == 0 || deduplicated[len(deduplicated)-1] != accountNo {
			deduplicated = append(deduplicated, accountNo)
		}
	}
	if len(deduplicated) == 0 || len(deduplicated) > 64 {
		return nil, bizerr.NewCode(CodeIntegrity)
	}
	arguments := make([]any, 0, len(deduplicated))
	for _, accountNo := range deduplicated {
		arguments = append(arguments, accountNo)
	}
	rows := make([]accountRow, 0, len(deduplicated))
	query := `
SELECT account_no, owner_type_code, owner_reference, COALESCE(customer_no, '') AS customer_no,
       account_type_code, normal_balance_code, currency_code, status_code
FROM plugin_idc_core_billing_ledger_account
WHERE tenant_id = 0 AND account_no IN (` + sqlPlaceholders(len(deduplicated)) + `)
ORDER BY account_no FOR UPDATE`
	if err := s.database.GetScan(ctx, &rows, query, arguments...); err != nil {
		return nil, gerror.Wrap(err, "lock existing billing accounts")
	}
	if len(rows) != len(deduplicated) {
		return nil, bizerr.NewCode(CodeConflict)
	}
	accounts := make(map[string]*accountRow, len(rows))
	for index := range rows {
		row := &rows[index]
		if requireActive && row.StatusCode != "active" {
			return nil, bizerr.NewCode(CodeConflict)
		}
		accounts[row.AccountNo] = row
	}
	return accounts, nil
}

func loadTransactionBySource(
	ctx context.Context,
	database gdb.DB,
	sourceType string,
	sourceReference string,
) (*transactionRow, error) {
	var row *transactionRow
	if err := database.GetScan(ctx, &row, `
SELECT id, transaction_no, transaction_type_code, currency_code,
       source_type_code, source_reference, source_snapshot_digest,
       COALESCE(original_transaction_no, '') AS original_transaction_no,
       entry_count, debit_total_minor, credit_total_minor, content_digest
FROM plugin_idc_core_billing_ledger_transaction
WHERE tenant_id = 0 AND source_type_code = ? AND source_reference = ?`,
		sourceType, sourceReference); err != nil {
		return nil, gerror.Wrap(err, "load billing transaction by source")
	}
	return row, nil
}

func (s *serviceImpl) postLocked(
	ctx context.Context,
	transactionType string,
	currency string,
	sourceType string,
	sourceReference string,
	sourceSnapshotDigest string,
	originalTransactionNo string,
	entries []entrySemantic,
	accounts map[string]*accountRow,
) (*transactionRow, map[string]int64, error) {
	semantic := transactionSemantic{
		TransactionType: transactionType, CurrencyCode: currency,
		SourceType: sourceType, SourceReference: sourceReference,
		SourceSnapshotDigest:  sourceSnapshotDigest,
		OriginalTransactionNo: originalTransactionNo, Entries: entries,
	}
	transactionDigest, err := contentDigest(semantic)
	if err != nil {
		return nil, nil, err
	}
	if existing, loadErr := loadTransactionBySource(ctx, s.database, sourceType, sourceReference); loadErr != nil {
		return nil, nil, loadErr
	} else if existing != nil {
		if existing.TransactionType != transactionType || existing.CurrencyCode != currency ||
			existing.SourceSnapshotDigest != sourceSnapshotDigest ||
			existing.OriginalTransactionNo != originalTransactionNo ||
			existing.ContentDigest != transactionDigest || existing.EntryCount != len(entries) {
			return nil, nil, bizerr.NewCode(CodeConflict)
		}
		balances, balanceErr := loadTransactionClosingBalances(ctx, s.database, existing.TransactionNo)
		return existing, balances, balanceErr
	}
	latest, err := loadLatestSnapshots(ctx, s.database, accounts)
	if err != nil {
		return nil, nil, err
	}
	debits, credits, debitTotal, creditTotal, err := aggregateLedgerEntries(
		entries, accounts, currency,
	)
	if err != nil {
		return nil, nil, err
	}
	transactionNo, err := newBillingNumber(s.random, "btx_")
	if err != nil {
		return nil, nil, err
	}
	preparedEntries, err := prepareLedgerEntries(s.random, transactionNo, entries)
	if err != nil {
		return nil, nil, err
	}
	preparedSnapshots, closingBalances, err := prepareBalanceSnapshots(
		s.random, transactionNo, accounts, latest, debits, credits,
	)
	if err != nil {
		return nil, nil, err
	}
	var original any
	if originalTransactionNo != "" {
		original = originalTransactionNo
	}
	if _, err = s.database.Exec(ctx, `
INSERT INTO plugin_idc_core_billing_ledger_transaction (
    tenant_id, transaction_no, transaction_type_code, currency_code,
    source_type_code, source_reference, source_snapshot_digest,
    original_transaction_no, entry_count, debit_total_minor, credit_total_minor,
    content_digest, posted_by
) VALUES (0, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 0)`,
		transactionNo, transactionType, currency, sourceType, sourceReference,
		sourceSnapshotDigest, original, len(entries), debitTotal, creditTotal,
		transactionDigest); err != nil {
		return nil, nil, gerror.Wrap(err, "insert billing ledger transaction")
	}
	if err = insertEntries(ctx, s.database, transactionNo, currency, preparedEntries); err != nil {
		return nil, nil, err
	}
	if err = insertSnapshots(ctx, s.database, transactionNo, currency, preparedSnapshots); err != nil {
		return nil, nil, err
	}
	return &transactionRow{
		TransactionNo: transactionNo, TransactionType: transactionType,
		CurrencyCode: currency, SourceType: sourceType, SourceReference: sourceReference,
		SourceSnapshotDigest: sourceSnapshotDigest, OriginalTransactionNo: originalTransactionNo,
		EntryCount: len(entries), DebitTotalMinor: debitTotal, CreditTotalMinor: creditTotal,
		ContentDigest: transactionDigest,
	}, closingBalances, nil
}

func aggregateLedgerEntries(
	entries []entrySemantic,
	accounts map[string]*accountRow,
	currency string,
) (map[string]int64, map[string]int64, int64, int64, error) {
	debits := make(map[string]int64, len(accounts))
	credits := make(map[string]int64, len(accounts))
	var debitTotal int64
	var creditTotal int64
	for _, entry := range entries {
		account := accounts[entry.AccountNo]
		if account == nil || account.CurrencyCode != currency || !validAmount(entry.AmountMinor) {
			return nil, nil, 0, 0, bizerr.NewCode(CodeIntegrity)
		}
		switch entry.Direction {
		case "debit":
			if debitTotal > math.MaxInt64-entry.AmountMinor || debits[entry.AccountNo] > math.MaxInt64-entry.AmountMinor {
				return nil, nil, 0, 0, bizerr.NewCode(CodeInvalid)
			}
			debitTotal += entry.AmountMinor
			debits[entry.AccountNo] += entry.AmountMinor
		case "credit":
			if creditTotal > math.MaxInt64-entry.AmountMinor || credits[entry.AccountNo] > math.MaxInt64-entry.AmountMinor {
				return nil, nil, 0, 0, bizerr.NewCode(CodeInvalid)
			}
			creditTotal += entry.AmountMinor
			credits[entry.AccountNo] += entry.AmountMinor
		default:
			return nil, nil, 0, 0, bizerr.NewCode(CodeIntegrity)
		}
	}
	if len(entries) < 2 || len(entries) > 64 || debitTotal <= 0 || debitTotal != creditTotal {
		return nil, nil, 0, 0, bizerr.NewCode(CodeIntegrity)
	}
	return debits, credits, debitTotal, creditTotal, nil
}

func prepareLedgerEntries(
	random io.Reader,
	transactionNo string,
	entries []entrySemantic,
) ([]preparedEntry, error) {
	prepared := make([]preparedEntry, 0, len(entries))
	for index, entry := range entries {
		entryNo, err := newBillingNumber(random, "ben_")
		if err != nil {
			return nil, err
		}
		digest, err := contentDigest(struct {
			TransactionNo string        `json:"transaction_no"`
			LineNumber    int           `json:"line_number"`
			Entry         entrySemantic `json:"entry"`
		}{transactionNo, index + 1, entry})
		if err != nil {
			return nil, err
		}
		prepared = append(prepared, preparedEntry{
			EntryNo: entryNo, LineNumber: index + 1,
			entrySemantic: entry, ContentDigest: digest,
		})
	}
	return prepared, nil
}

func prepareBalanceSnapshots(
	random io.Reader,
	transactionNo string,
	accounts map[string]*accountRow,
	latest map[string]*latestSnapshotRow,
	debits map[string]int64,
	credits map[string]int64,
) ([]preparedSnapshot, map[string]int64, error) {
	prepared := make([]preparedSnapshot, 0, len(accounts))
	closingBalances := make(map[string]int64, len(accounts))
	for accountNo, account := range accounts {
		previous := latest[accountNo]
		opening := int64(0)
		sequence := int64(1)
		var previousNo any
		var previousSequence any
		if previous != nil {
			opening = previous.ClosingBalanceMinor
			sequence = previous.SequenceNo + 1
			previousNo = previous.SnapshotNo
			previousSequence = previous.SequenceNo
		}
		closing, valid := calculateNormalBalance(
			opening, debits[accountNo], credits[accountNo], account.NormalBalance,
		)
		if !valid {
			return nil, nil, bizerr.NewCode(CodeInvalid)
		}
		if closing < 0 && protectedCustomerAccount(account.AccountType) {
			return nil, nil, bizerr.NewCode(CodeInsufficientBalance)
		}
		snapshotNo, err := newBillingNumber(random, "bsn_")
		if err != nil {
			return nil, nil, err
		}
		digest, err := contentDigest(struct {
			SnapshotNo    string `json:"snapshot_no"`
			AccountNo     string `json:"account_no"`
			TransactionNo string `json:"transaction_no"`
			SequenceNo    int64  `json:"sequence_no"`
			Opening       int64  `json:"opening"`
			Debit         int64  `json:"debit"`
			Credit        int64  `json:"credit"`
			Closing       int64  `json:"closing"`
		}{snapshotNo, accountNo, transactionNo, sequence, opening,
			debits[accountNo], credits[accountNo], closing})
		if err != nil {
			return nil, nil, err
		}
		prepared = append(prepared, preparedSnapshot{
			SnapshotNo: snapshotNo, AccountNo: accountNo, NormalBalance: account.NormalBalance,
			SequenceNo: sequence, PreviousSnapshotNo: previousNo,
			PreviousSequenceNo: previousSequence, OpeningBalanceMinor: opening,
			DebitAmountMinor: debits[accountNo], CreditAmountMinor: credits[accountNo],
			ClosingBalanceMinor: closing, ContentDigest: digest,
		})
		closingBalances[accountNo] = closing
	}
	sort.Slice(prepared, func(left int, right int) bool {
		return prepared[left].AccountNo < prepared[right].AccountNo
	})
	return prepared, closingBalances, nil
}

func protectedCustomerAccount(accountType string) bool {
	return accountType == AccountTypeCustomerBalance ||
		accountType == AccountTypeCustomerGift ||
		accountType == AccountTypeCustomerCommission
}

func loadLatestSnapshots(
	ctx context.Context,
	database gdb.DB,
	accounts map[string]*accountRow,
) (map[string]*latestSnapshotRow, error) {
	result := make(map[string]*latestSnapshotRow, len(accounts))
	if len(accounts) == 0 || len(accounts) > 64 {
		return nil, bizerr.NewCode(CodeIntegrity)
	}
	accountNumbers := make([]string, 0, len(accounts))
	arguments := make([]any, 0, len(accounts))
	for accountNo := range accounts {
		accountNumbers = append(accountNumbers, accountNo)
	}
	sort.Strings(accountNumbers)
	for _, accountNo := range accountNumbers {
		arguments = append(arguments, accountNo)
		result[accountNo] = nil
	}
	rows := make([]latestSnapshotRow, 0, len(accounts))
	query := `
SELECT DISTINCT ON (account_no)
       snapshot_no, account_no, sequence_no, closing_balance_minor
FROM plugin_idc_core_billing_balance_snapshot
WHERE tenant_id = 0 AND account_no IN (` + sqlPlaceholders(len(accountNumbers)) + `)
ORDER BY account_no ASC, sequence_no DESC, id DESC`
	if err := database.GetScan(ctx, &rows, query, arguments...); err != nil {
		return nil, gerror.Wrap(err, "load latest billing balance snapshots")
	}
	for index := range rows {
		row := &rows[index]
		result[row.AccountNo] = row
	}
	return result, nil
}

func loadTransactionClosingBalances(
	ctx context.Context,
	database gdb.DB,
	transactionNo string,
) (map[string]int64, error) {
	rows := make([]struct {
		AccountNo           string `orm:"account_no"`
		ClosingBalanceMinor int64  `orm:"closing_balance_minor"`
	}, 0, 4)
	if err := database.GetScan(ctx, &rows, `
SELECT account_no, closing_balance_minor
FROM plugin_idc_core_billing_balance_snapshot
WHERE tenant_id = 0 AND transaction_no = ?`, transactionNo); err != nil {
		return nil, gerror.Wrap(err, "load billing transaction snapshots")
	}
	result := make(map[string]int64, len(rows))
	for _, row := range rows {
		result[row.AccountNo] = row.ClosingBalanceMinor
	}
	return result, nil
}

func insertEntries(
	ctx context.Context,
	database gdb.DB,
	transactionNo string,
	currency string,
	entries []preparedEntry,
) error {
	var query strings.Builder
	query.WriteString(`INSERT INTO plugin_idc_core_billing_ledger_entry (
    tenant_id, entry_no, transaction_no, currency_code, line_number,
    account_no, direction_code, entry_role_code, amount_minor,
    invoice_no, invoice_item_no, reason_code, content_digest
) VALUES `)
	arguments := make([]any, 0, len(entries)*12)
	for index, entry := range entries {
		if index > 0 {
			query.WriteString(",")
		}
		query.WriteString("(0, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)")
		var invoiceNo any
		if entry.InvoiceNo != "" {
			invoiceNo = entry.InvoiceNo
		}
		var invoiceItemNo any
		if entry.InvoiceItemNo != "" {
			invoiceItemNo = entry.InvoiceItemNo
		}
		arguments = append(arguments, entry.EntryNo, transactionNo, currency, entry.LineNumber,
			entry.AccountNo, entry.Direction, entry.Role, entry.AmountMinor,
			invoiceNo, invoiceItemNo, entry.Reason, entry.ContentDigest)
	}
	if _, err := database.Exec(ctx, query.String(), arguments...); err != nil {
		return gerror.Wrap(err, "insert billing ledger entries")
	}
	return nil
}

func insertSnapshots(
	ctx context.Context,
	database gdb.DB,
	transactionNo string,
	currency string,
	snapshots []preparedSnapshot,
) error {
	var query strings.Builder
	query.WriteString(`INSERT INTO plugin_idc_core_billing_balance_snapshot (
    tenant_id, snapshot_no, account_no, currency_code, normal_balance_code,
    transaction_no, sequence_no, previous_snapshot_no, previous_sequence_no,
    opening_balance_minor, debit_amount_minor, credit_amount_minor,
    closing_balance_minor, content_digest
) VALUES `)
	arguments := make([]any, 0, len(snapshots)*13)
	for index, snapshot := range snapshots {
		if index > 0 {
			query.WriteString(",")
		}
		query.WriteString("(0, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)")
		arguments = append(arguments, snapshot.SnapshotNo, snapshot.AccountNo, currency,
			snapshot.NormalBalance, transactionNo, snapshot.SequenceNo,
			snapshot.PreviousSnapshotNo, snapshot.PreviousSequenceNo,
			snapshot.OpeningBalanceMinor, snapshot.DebitAmountMinor,
			snapshot.CreditAmountMinor, snapshot.ClosingBalanceMinor,
			snapshot.ContentDigest)
	}
	if _, err := database.Exec(ctx, query.String(), arguments...); err != nil {
		return gerror.Wrap(err, "insert billing balance snapshots")
	}
	return nil
}

func transactionEntries(
	ctx context.Context,
	database gdb.DB,
	transactionNo string,
) ([]entrySemantic, error) {
	rows := make([]struct {
		AccountNo   string `orm:"account_no"`
		Direction   string `orm:"direction_code"`
		Role        string `orm:"entry_role_code"`
		AmountMinor int64  `orm:"amount_minor"`
		Reason      string `orm:"reason_code"`
	}, 0, 8)
	if err := database.GetScan(ctx, &rows, `
SELECT account_no, direction_code, entry_role_code, amount_minor, reason_code
FROM plugin_idc_core_billing_ledger_entry
WHERE tenant_id = 0 AND transaction_no = ? ORDER BY line_number ASC`, transactionNo); err != nil {
		return nil, gerror.Wrap(err, "load original billing entries")
	}
	entries := make([]entrySemantic, 0, len(rows))
	for _, row := range rows {
		entries = append(entries, entrySemantic{
			AccountNo: row.AccountNo, Direction: row.Direction, Role: row.Role,
			AmountMinor: row.AmountMinor, Reason: row.Reason,
		})
	}
	return entries, nil
}

func reverseEntries(entries []entrySemantic) []entrySemantic {
	result := make([]entrySemantic, 0, len(entries))
	for _, entry := range entries {
		direction := "debit"
		if entry.Direction == "debit" {
			direction = "credit"
		}
		result = append(result, entrySemantic{
			AccountNo: entry.AccountNo, Direction: direction, Role: entry.Role,
			AmountMinor: entry.AmountMinor, Reason: "transaction_reversal",
		})
	}
	return result
}

func postingAmount(entries []entrySemantic) (int64, error) {
	var total int64
	for _, entry := range entries {
		if entry.Direction != "debit" {
			continue
		}
		if total > math.MaxInt64-entry.AmountMinor {
			return 0, gerror.New("billing posting amount overflow")
		}
		total += entry.AmountMinor
	}
	return total, nil
}

func calculateNormalBalance(opening int64, debit int64, credit int64, normal string) (int64, bool) {
	first := debit
	second := credit
	if normal == "credit" {
		first = credit
		second = debit
	} else if normal != "debit" {
		return 0, false
	}
	if first > 0 && opening > math.MaxInt64-first {
		return 0, false
	}
	value := opening + first
	if second > 0 && value < math.MinInt64+second {
		return 0, false
	}
	return value - second, true
}

func sqlPlaceholders(count int) string {
	if count <= 0 {
		return ""
	}
	return strings.TrimSuffix(strings.Repeat("?,", count), ",")
}
