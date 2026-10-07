package billing

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
)

func (s *serviceImpl) PostConfirmedRecharge(
	ctx context.Context,
	input ConfirmedRechargeInput,
) (*Posting, error) {
	normalized, err := normalizeConfirmedRecharge(input)
	if err != nil {
		return nil, err
	}
	var result *Posting
	err = s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		if lockErr := s.lockSource(ctx, sourceConfirmedRecharge, normalized.SourceReference); lockErr != nil {
			return lockErr
		}
		existing, loadErr := loadTransactionBySource(
			ctx, s.database, sourceConfirmedRecharge, normalized.SourceReference,
		)
		if loadErr != nil {
			return loadErr
		}
		if existing == nil {
			if customerErr := s.requireActiveCustomer(ctx, normalized.CustomerNo); customerErr != nil {
				return customerErr
			}
		}
		merchantNo, merchantErr := s.loadMerchantNo(ctx, existing == nil)
		if merchantErr != nil {
			return merchantErr
		}
		balanceSpec := customerBalanceSpec(normalized.CustomerNo)
		cashSpec := merchantCashSpec(merchantNo)
		accounts, accountErr := s.ensureAndLockAccounts(
			ctx, normalized.CurrencyCode, existing == nil, balanceSpec, cashSpec,
		)
		if accountErr != nil {
			return accountErr
		}
		balanceAccountNo := deterministicAccountNo(
			balanceSpec.OwnerType, balanceSpec.OwnerReference,
			balanceSpec.AccountType, normalized.CurrencyCode,
		)
		cashAccountNo := deterministicAccountNo(
			cashSpec.OwnerType, cashSpec.OwnerReference,
			cashSpec.AccountType, normalized.CurrencyCode,
		)
		transaction, _, postErr := s.postLocked(
			ctx, "recharge", normalized.CurrencyCode,
			sourceConfirmedRecharge, normalized.SourceReference,
			normalized.SourceSnapshotDigest, "",
			[]entrySemantic{
				{AccountNo: cashAccountNo, Direction: "debit", Role: "cash",
					AmountMinor: normalized.AmountMinor, Reason: "confirmed_recharge"},
				{AccountNo: balanceAccountNo, Direction: "credit", Role: "balance",
					AmountMinor: normalized.AmountMinor, Reason: "confirmed_recharge"},
			}, accounts,
		)
		if postErr != nil {
			return postErr
		}
		balance, held, balanceErr := loadCurrentBalanceAndHeld(ctx, s.database, balanceAccountNo)
		if balanceErr != nil {
			return balanceErr
		}
		result = &Posting{
			TransactionNo: transaction.TransactionNo, TransactionType: "recharge",
			CurrencyCode: normalized.CurrencyCode, AmountMinor: normalized.AmountMinor,
			CustomerNo: normalized.CustomerNo, BalanceMinor: balance,
			HeldBalanceMinor: held, AvailableBalanceMinor: balance - held,
			Reused: existing != nil,
		}
		return nil
	})
	return result, err
}

func (s *serviceImpl) ConsumeAvailableBalance(
	ctx context.Context,
	input BalanceConsumptionInput,
) (*Posting, error) {
	normalized, err := normalizeBalanceConsumption(input)
	if err != nil {
		return nil, err
	}
	var result *Posting
	err = s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		if lockErr := s.lockSource(ctx, sourceBalanceConsumption, normalized.SourceReference); lockErr != nil {
			return lockErr
		}
		existing, loadErr := loadTransactionBySource(
			ctx, s.database, sourceBalanceConsumption, normalized.SourceReference,
		)
		if loadErr != nil {
			return loadErr
		}
		if existing == nil {
			if customerErr := s.requireActiveCustomer(ctx, normalized.CustomerNo); customerErr != nil {
				return customerErr
			}
		}
		merchantNo, merchantErr := s.loadMerchantNo(ctx, existing == nil)
		if merchantErr != nil {
			return merchantErr
		}
		balanceSpec := customerBalanceSpec(normalized.CustomerNo)
		revenueSpec := merchantRevenueSpec(merchantNo)
		accounts, accountErr := s.ensureAndLockAccounts(
			ctx, normalized.CurrencyCode, existing == nil, balanceSpec, revenueSpec,
		)
		if accountErr != nil {
			return accountErr
		}
		balanceAccountNo := deterministicAccountNo(
			balanceSpec.OwnerType, balanceSpec.OwnerReference,
			balanceSpec.AccountType, normalized.CurrencyCode,
		)
		revenueAccountNo := deterministicAccountNo(
			revenueSpec.OwnerType, revenueSpec.OwnerReference,
			revenueSpec.AccountType, normalized.CurrencyCode,
		)
		if existing == nil {
			balance, held, balanceErr := loadCurrentBalanceAndHeld(ctx, s.database, balanceAccountNo)
			if balanceErr != nil {
				return balanceErr
			}
			if balance-held < normalized.AmountMinor {
				return bizerr.NewCode(CodeInsufficientBalance)
			}
		}
		transaction, _, postErr := s.postLocked(
			ctx, "balance_payment", normalized.CurrencyCode,
			sourceBalanceConsumption, normalized.SourceReference,
			normalized.SourceSnapshotDigest, "",
			[]entrySemantic{
				{AccountNo: balanceAccountNo, Direction: "debit", Role: "balance",
					AmountMinor: normalized.AmountMinor, Reason: "balance_consumption"},
				{AccountNo: revenueAccountNo, Direction: "credit", Role: "revenue",
					AmountMinor: normalized.AmountMinor, Reason: "balance_consumption"},
			}, accounts,
		)
		if postErr != nil {
			return postErr
		}
		balance, held, balanceErr := loadCurrentBalanceAndHeld(ctx, s.database, balanceAccountNo)
		if balanceErr != nil {
			return balanceErr
		}
		result = &Posting{
			TransactionNo: transaction.TransactionNo, TransactionType: "balance_payment",
			CurrencyCode: normalized.CurrencyCode, AmountMinor: normalized.AmountMinor,
			CustomerNo: normalized.CustomerNo, BalanceMinor: balance,
			HeldBalanceMinor: held, AvailableBalanceMinor: balance - held,
			Reused: existing != nil,
		}
		return nil
	})
	return result, err
}

func (s *serviceImpl) ReverseTransaction(
	ctx context.Context,
	input ReversalInput,
) (*Posting, error) {
	normalized, err := normalizeReversal(input)
	if err != nil {
		return nil, err
	}
	var result *Posting
	err = s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		if lockErr := s.lockSource(ctx, sourceLedgerReversal, normalized.SourceReference); lockErr != nil {
			return lockErr
		}
		existing, loadErr := loadTransactionBySource(
			ctx, s.database, sourceLedgerReversal, normalized.SourceReference,
		)
		if loadErr != nil {
			return loadErr
		}
		var original *transactionRow
		if scanErr := s.database.GetScan(ctx, &original, `
SELECT id, transaction_no, transaction_type_code, currency_code,
       source_type_code, source_reference, source_snapshot_digest,
       COALESCE(original_transaction_no, '') AS original_transaction_no,
       entry_count, debit_total_minor, credit_total_minor, content_digest
FROM plugin_idc_core_billing_ledger_transaction
WHERE tenant_id = 0 AND transaction_no = ? FOR UPDATE`,
			normalized.OriginalTransactionNo); scanErr != nil {
			return gerror.Wrap(scanErr, "lock original billing transaction")
		}
		if original == nil {
			return bizerr.NewCode(CodeNotFound)
		}
		if original.TransactionType == "reversal" || original.EntryCount < 2 || original.EntryCount > 64 {
			return bizerr.NewCode(CodeConflict)
		}
		entries, entryErr := transactionEntries(ctx, s.database, original.TransactionNo)
		if entryErr != nil {
			return entryErr
		}
		if len(entries) != original.EntryCount {
			return bizerr.NewCode(CodeIntegrity)
		}
		if existing == nil {
			reversalTotal, totalErr := s.database.GetValue(ctx, `
SELECT COALESCE(SUM(debit_total_minor), 0)
FROM plugin_idc_core_billing_ledger_transaction
WHERE tenant_id = 0 AND original_transaction_no = ?`, original.TransactionNo)
			if totalErr != nil {
				return gerror.Wrap(totalErr, "inspect original billing reversals")
			}
			if reversalTotal.Int64() != 0 {
				return bizerr.NewCode(CodeConflict)
			}
		}
		accountNumbers := make([]string, 0, len(entries))
		for _, entry := range entries {
			accountNumbers = append(accountNumbers, entry.AccountNo)
		}
		accounts, accountErr := s.lockExistingAccounts(ctx, accountNumbers, false)
		if accountErr != nil {
			return accountErr
		}
		reversedEntries := reverseEntries(entries)
		if existing == nil {
			for _, entry := range reversedEntries {
				account := accounts[entry.AccountNo]
				if account == nil || account.AccountType != AccountTypeCustomerBalance ||
					entry.Direction != "debit" {
					continue
				}
				balance, held, balanceErr := loadCurrentBalanceAndHeld(ctx, s.database, account.AccountNo)
				if balanceErr != nil {
					return balanceErr
				}
				if balance-held < entry.AmountMinor {
					return bizerr.NewCode(CodeInsufficientBalance)
				}
			}
		}
		transaction, _, postErr := s.postLocked(
			ctx, "reversal", original.CurrencyCode,
			sourceLedgerReversal, normalized.SourceReference,
			normalized.SourceSnapshotDigest, original.TransactionNo,
			reversedEntries, accounts,
		)
		if postErr != nil {
			return postErr
		}
		amount, amountErr := postingAmount(reversedEntries)
		if amountErr != nil {
			return bizerr.NewCode(CodeIntegrity)
		}
		result = &Posting{
			TransactionNo: transaction.TransactionNo, TransactionType: "reversal",
			CurrencyCode: original.CurrencyCode, AmountMinor: amount, Reused: existing != nil,
		}
		for _, account := range accounts {
			if account.AccountType != AccountTypeCustomerBalance || account.CustomerNo == "" {
				continue
			}
			balance, held, balanceErr := loadCurrentBalanceAndHeld(ctx, s.database, account.AccountNo)
			if balanceErr != nil {
				return balanceErr
			}
			result.CustomerNo = account.CustomerNo
			result.BalanceMinor = balance
			result.HeldBalanceMinor = held
			result.AvailableBalanceMinor = balance - held
			break
		}
		return nil
	})
	return result, err
}

func loadCurrentBalanceAndHeld(
	ctx context.Context,
	database gdb.DB,
	accountNo string,
) (int64, int64, error) {
	var state struct {
		Balance int64 `orm:"balance_minor"`
		Held    int64 `orm:"held_minor"`
	}
	if err := database.GetScan(ctx, &state, `
SELECT
    COALESCE((
        SELECT closing_balance_minor
        FROM plugin_idc_core_billing_balance_snapshot
        WHERE tenant_id = 0 AND account_no = ?
        ORDER BY sequence_no DESC LIMIT 1
    ), 0) AS balance_minor,
    COALESCE((
        SELECT SUM(amount_minor - consumed_amount_minor - released_amount_minor)
        FROM plugin_idc_core_billing_fund_hold
        WHERE tenant_id = 0 AND account_no = ?
          AND status_code IN ('active', 'partially_consumed')
          AND expires_at > CURRENT_TIMESTAMP
    ), 0) AS held_minor`, accountNo, accountNo); err != nil {
		return 0, 0, gerror.Wrap(err, "load current customer balance")
	}
	if state.Balance < 0 || state.Held < 0 || state.Held > state.Balance {
		return 0, 0, bizerr.NewCode(CodeIntegrity)
	}
	return state.Balance, state.Held, nil
}
