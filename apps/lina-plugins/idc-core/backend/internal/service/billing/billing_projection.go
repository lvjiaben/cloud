package billing

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"hash"
	"math"
	"strings"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
)

const projectionTransactionPageSize = 1_000

type projectionEntryRow struct {
	TransactionID     int64  `orm:"transaction_id"`
	TransactionNo     string `orm:"transaction_no"`
	LineNumber        int    `orm:"line_number"`
	Direction         string `orm:"direction_code"`
	Role              string `orm:"entry_role_code"`
	AmountMinor       int64  `orm:"amount_minor"`
	Reason            string `orm:"reason_code"`
	EntryDigest       string `orm:"entry_digest"`
	TransactionDigest string `orm:"transaction_digest"`
}

type projectionSnapshotRow struct {
	SnapshotNo          string `orm:"snapshot_no"`
	TransactionNo       string `orm:"transaction_no"`
	SequenceNo          int64  `orm:"sequence_no"`
	PreviousSnapshotNo  string `orm:"previous_snapshot_no"`
	PreviousSequenceNo  int64  `orm:"previous_sequence_no"`
	OpeningBalanceMinor int64  `orm:"opening_balance_minor"`
	DebitAmountMinor    int64  `orm:"debit_amount_minor"`
	CreditAmountMinor   int64  `orm:"credit_amount_minor"`
	ClosingBalanceMinor int64  `orm:"closing_balance_minor"`
	ContentDigest       string `orm:"content_digest"`
}

type rebuiltStep struct {
	TransactionNo     string   `json:"transaction_no"`
	DebitMinor        int64    `json:"debit_minor"`
	CreditMinor       int64    `json:"credit_minor"`
	ClosingMinor      int64    `json:"closing_minor"`
	EntryDigests      []string `json:"entry_digests"`
	TransactionDigest string   `json:"transaction_digest"`
}

type projectionRebuildState struct {
	AfterTransactionID int64
	SequenceNo         int64
	RebuiltBalance     int64
	SnapshotBalance    int64
	PreviousSnapshotNo string
	TransactionCount   int
	EntryCount         int
	Consistent         bool
	Proof              hash.Hash
}

func (s *serviceImpl) RebuildCustomerAccountProjection(
	ctx context.Context,
	customerNo string,
	currency string,
	accountType string,
) (*AccountProjection, error) {
	customerNo = strings.TrimSpace(customerNo)
	currency = strings.TrimSpace(currency)
	accountType = strings.TrimSpace(accountType)
	if !validBusinessReference(customerNo, 40) || !currencyPattern.MatchString(currency) ||
		!validCustomerAccountType(accountType) {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	accountNo := deterministicAccountNo("customer", customerNo, accountType, currency)
	account, err := s.loadProjectionAccount(ctx, accountNo)
	if err != nil {
		return nil, err
	}
	if account.OwnerType != "customer" || account.CustomerNo != customerNo ||
		account.AccountType != accountType || account.CurrencyCode != currency {
		return nil, bizerr.NewCode(CodeIntegrity)
	}
	state := projectionRebuildState{Consistent: true, Proof: sha256.New()}
	if err = writeProjectionProof(state.Proof, struct {
		AccountNo     string `json:"account_no"`
		NormalBalance string `json:"normal_balance"`
	}{accountNo, account.NormalBalance}); err != nil {
		return nil, err
	}
	for {
		entries, loadErr := s.loadProjectionEntryPage(ctx, accountNo, state.AfterTransactionID)
		if loadErr != nil {
			return nil, loadErr
		}
		if len(entries) == 0 {
			break
		}
		steps, rebuiltBalance, rebuildErr := rebuildSteps(
			account.NormalBalance, state.RebuiltBalance, entries,
		)
		if rebuildErr != nil {
			return nil, rebuildErr
		}
		snapshots, snapshotErr := s.loadProjectionSnapshotPage(
			ctx, accountNo, state.SequenceNo, len(steps),
		)
		if snapshotErr != nil {
			return nil, snapshotErr
		}
		consistent, snapshotBalance, previousSnapshotNo := projectionPageMatchesSnapshots(
			steps, snapshots, account.NormalBalance, state.SequenceNo,
			state.SnapshotBalance, state.PreviousSnapshotNo,
		)
		state.Consistent = state.Consistent && consistent
		for _, step := range steps {
			if proofErr := writeProjectionProof(state.Proof, step); proofErr != nil {
				return nil, proofErr
			}
		}
		state.AfterTransactionID = entries[len(entries)-1].TransactionID
		state.SequenceNo += int64(len(steps))
		state.RebuiltBalance = rebuiltBalance
		state.SnapshotBalance = snapshotBalance
		state.PreviousSnapshotNo = previousSnapshotNo
		state.TransactionCount += len(steps)
		state.EntryCount += len(entries)
	}
	snapshotCount, err := s.database.GetValue(ctx, `
SELECT count(*) FROM plugin_idc_core_billing_balance_snapshot
WHERE tenant_id = 0 AND account_no = ?`, accountNo)
	if err != nil {
		return nil, gerror.Wrap(err, "count billing snapshots after rebuild")
	}
	if snapshotCount.Int64() != state.SequenceNo {
		state.Consistent = false
	}
	held := int64(0)
	if accountType == AccountTypeCustomerBalance {
		_, currentHeld, heldErr := loadCurrentBalanceAndHeld(ctx, s.database, accountNo)
		if heldErr != nil {
			return nil, heldErr
		}
		held = currentHeld
	}
	projection := &AccountProjection{
		AccountNo: accountNo, CustomerNo: customerNo, AccountType: accountType,
		CurrencyCode: currency, NormalBalance: account.NormalBalance,
		TransactionCount: state.TransactionCount, EntryCount: state.EntryCount,
		RebuiltBalanceMinor:  state.RebuiltBalance,
		SnapshotBalanceMinor: state.SnapshotBalance,
		HeldBalanceMinor:     held, AvailableBalanceMinor: state.RebuiltBalance - held,
		ContentDigest: hex.EncodeToString(state.Proof.Sum(nil)), Consistent: state.Consistent,
	}
	if !projection.Consistent || projection.AvailableBalanceMinor < 0 {
		return projection, bizerr.NewCode(CodeIntegrity)
	}
	return projection, nil
}

func (s *serviceImpl) loadProjectionAccount(ctx context.Context, accountNo string) (*accountRow, error) {
	var account *accountRow
	if err := s.database.GetScan(ctx, &account, `
SELECT account_no, owner_type_code, owner_reference, COALESCE(customer_no, '') AS customer_no,
       account_type_code, normal_balance_code, currency_code, status_code
FROM plugin_idc_core_billing_ledger_account
WHERE tenant_id = 0 AND account_no = ?`, accountNo); err != nil {
		return nil, gerror.Wrap(err, "load billing account for projection rebuild")
	}
	if account == nil {
		return nil, bizerr.NewCode(CodeNotFound)
	}
	return account, nil
}

func (s *serviceImpl) loadProjectionEntryPage(
	ctx context.Context,
	accountNo string,
	afterTransactionID int64,
) ([]projectionEntryRow, error) {
	rows := make([]projectionEntryRow, 0, projectionTransactionPageSize)
	if err := s.database.GetScan(ctx, &rows, `
WITH transaction_page AS (
    SELECT DISTINCT transaction.id
    FROM plugin_idc_core_billing_ledger_entry entry
    JOIN plugin_idc_core_billing_ledger_transaction transaction
      ON transaction.transaction_no = entry.transaction_no
    WHERE entry.tenant_id = 0 AND entry.account_no = ? AND transaction.id > ?
    ORDER BY transaction.id ASC
    LIMIT ?
)
SELECT transaction.id AS transaction_id, transaction.transaction_no,
       entry.line_number, entry.direction_code, entry.entry_role_code,
       entry.amount_minor, entry.reason_code, entry.content_digest AS entry_digest,
       transaction.content_digest AS transaction_digest
FROM transaction_page page
JOIN plugin_idc_core_billing_ledger_transaction transaction
  ON transaction.id = page.id
JOIN plugin_idc_core_billing_ledger_entry entry
  ON entry.transaction_no = transaction.transaction_no AND entry.account_no = ?
ORDER BY transaction.id ASC, entry.line_number ASC, entry.id ASC`,
		accountNo, afterTransactionID, projectionTransactionPageSize, accountNo); err != nil {
		return nil, gerror.Wrap(err, "load billing projection entry page")
	}
	return rows, nil
}

func (s *serviceImpl) loadProjectionSnapshotPage(
	ctx context.Context,
	accountNo string,
	afterSequence int64,
	pageSize int,
) ([]projectionSnapshotRow, error) {
	rows := make([]projectionSnapshotRow, 0, pageSize)
	if err := s.database.GetScan(ctx, &rows, `
SELECT snapshot_no, transaction_no, sequence_no,
       COALESCE(previous_snapshot_no, '') AS previous_snapshot_no,
       COALESCE(previous_sequence_no, 0) AS previous_sequence_no,
       opening_balance_minor, debit_amount_minor, credit_amount_minor,
       closing_balance_minor, content_digest
FROM plugin_idc_core_billing_balance_snapshot
WHERE tenant_id = 0 AND account_no = ? AND sequence_no > ?
ORDER BY sequence_no ASC
LIMIT ?`, accountNo, afterSequence, pageSize); err != nil {
		return nil, gerror.Wrap(err, "load billing projection snapshot page")
	}
	return rows, nil
}

func rebuildSteps(
	normalBalance string,
	openingBalance int64,
	entries []projectionEntryRow,
) ([]rebuiltStep, int64, error) {
	steps := make([]rebuiltStep, 0, len(entries))
	balance := openingBalance
	for index := 0; index < len(entries); {
		transactionID := entries[index].TransactionID
		transactionNo := entries[index].TransactionNo
		transactionDigest := entries[index].TransactionDigest
		step := rebuiltStep{TransactionNo: transactionNo, TransactionDigest: transactionDigest}
		for index < len(entries) && entries[index].TransactionID == transactionID {
			entry := entries[index]
			if entry.TransactionNo != transactionNo || entry.TransactionDigest != transactionDigest ||
				entry.AmountMinor <= 0 || !canonicalDigestPattern.MatchString(entry.EntryDigest) ||
				!canonicalDigestPattern.MatchString(entry.TransactionDigest) {
				return nil, 0, bizerr.NewCode(CodeIntegrity)
			}
			switch entry.Direction {
			case "debit":
				if step.DebitMinor > math.MaxInt64-entry.AmountMinor {
					return nil, 0, bizerr.NewCode(CodeIntegrity)
				}
				step.DebitMinor += entry.AmountMinor
			case "credit":
				if step.CreditMinor > math.MaxInt64-entry.AmountMinor {
					return nil, 0, bizerr.NewCode(CodeIntegrity)
				}
				step.CreditMinor += entry.AmountMinor
			default:
				return nil, 0, bizerr.NewCode(CodeIntegrity)
			}
			step.EntryDigests = append(step.EntryDigests, entry.EntryDigest)
			index++
		}
		var valid bool
		balance, valid = calculateNormalBalance(balance, step.DebitMinor, step.CreditMinor, normalBalance)
		if !valid {
			return nil, 0, bizerr.NewCode(CodeIntegrity)
		}
		step.ClosingMinor = balance
		steps = append(steps, step)
	}
	return steps, balance, nil
}

func projectionPageMatchesSnapshots(
	steps []rebuiltStep,
	snapshots []projectionSnapshotRow,
	normalBalance string,
	afterSequence int64,
	openingBalance int64,
	previousSnapshotNo string,
) (bool, int64, string) {
	consistent := len(steps) == len(snapshots)
	balance := openingBalance
	previousNo := previousSnapshotNo
	for index, step := range steps {
		if index >= len(snapshots) {
			return false, balance, previousNo
		}
		snapshot := snapshots[index]
		expectedSequence := afterSequence + int64(index) + 1
		if snapshot.SequenceNo != expectedSequence || snapshot.TransactionNo != step.TransactionNo ||
			snapshot.OpeningBalanceMinor != balance || snapshot.DebitAmountMinor != step.DebitMinor ||
			snapshot.CreditAmountMinor != step.CreditMinor || snapshot.ClosingBalanceMinor != step.ClosingMinor ||
			!canonicalDigestPattern.MatchString(snapshot.ContentDigest) {
			consistent = false
		}
		if expectedSequence == 1 {
			if snapshot.PreviousSnapshotNo != "" || snapshot.PreviousSequenceNo != 0 {
				consistent = false
			}
		} else if snapshot.PreviousSnapshotNo != previousNo ||
			snapshot.PreviousSequenceNo != expectedSequence-1 {
			consistent = false
		}
		calculated, valid := calculateNormalBalance(
			balance, step.DebitMinor, step.CreditMinor, normalBalance,
		)
		if !valid || calculated != snapshot.ClosingBalanceMinor {
			consistent = false
		} else {
			balance = calculated
		}
		previousNo = snapshot.SnapshotNo
	}
	return consistent, balance, previousNo
}

func writeProjectionProof(proof hash.Hash, value any) error {
	encoded, err := json.Marshal(value)
	if err != nil {
		return gerror.Wrap(err, "encode billing projection proof")
	}
	if _, err = proof.Write(encoded); err != nil {
		return gerror.Wrap(err, "write billing projection proof")
	}
	if _, err = proof.Write([]byte{'\n'}); err != nil {
		return gerror.Wrap(err, "delimit billing projection proof")
	}
	return nil
}

func validCustomerAccountType(value string) bool {
	return value == AccountTypeCustomerBalance || value == AccountTypeCustomerCredit ||
		value == AccountTypeCustomerGift || value == AccountTypeCustomerCommission
}
