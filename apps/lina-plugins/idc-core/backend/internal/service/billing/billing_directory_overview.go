package billing

import (
	"context"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
)

type balanceDirectoryRow struct {
	AccountNo        string    `orm:"account_no"`
	AccountTypeCode  string    `orm:"account_type_code"`
	CurrencyCode     string    `orm:"currency_code"`
	StatusCode       string    `orm:"status_code"`
	Withdrawable     bool      `orm:"withdrawable"`
	BalanceMinor     int64     `orm:"balance_minor"`
	HeldBalanceMinor int64     `orm:"held_balance_minor"`
	UpdatedAt        time.Time `orm:"updated_at"`
}

type receivableCurrencyRow struct {
	CurrencyCode     string `orm:"currency_code"`
	InvoiceCount     int    `orm:"invoice_count"`
	OutstandingMinor int64  `orm:"outstanding_minor"`
}

func (s *financeDirectoryImpl) GetAccountFinancialOverview(
	ctx context.Context,
) (*AccountFinancialOverview, error) {
	customerNo, err := s.currentCustomer(ctx)
	if err != nil {
		return nil, err
	}
	result := &AccountFinancialOverview{
		Balances: []BalanceDirectoryItem{}, OpenReceivableByCurrency: map[string]int64{},
		RechargeAvailable: false, RechargeReasonCode: "payment_capability_unavailable",
	}
	err = s.database.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		if _, setErr := tx.Exec("SET TRANSACTION ISOLATION LEVEL REPEATABLE READ, READ ONLY"); setErr != nil {
			return setErr
		}
		rows := make([]balanceDirectoryRow, 0, maximumAccountBalanceCount+1)
		if loadErr := tx.GetScan(&rows, `
SELECT account.account_no, account.account_type_code, account.currency_code,
       account.status_code, account.withdrawable,
       COALESCE(snapshot.closing_balance_minor, 0) AS balance_minor,
       COALESCE(hold.held_balance_minor, 0) AS held_balance_minor,
       COALESCE(snapshot.created_at, account.updated_at) AS updated_at
FROM plugin_idc_core_billing_ledger_account account
LEFT JOIN LATERAL (
    SELECT closing_balance_minor, created_at
    FROM plugin_idc_core_billing_balance_snapshot
    WHERE tenant_id = 0 AND account_no = account.account_no
    ORDER BY sequence_no DESC
    LIMIT 1
) snapshot ON TRUE
LEFT JOIN LATERAL (
    SELECT COALESCE(SUM(amount_minor - consumed_amount_minor - released_amount_minor), 0)
           AS held_balance_minor
    FROM plugin_idc_core_billing_fund_hold
    WHERE tenant_id = 0 AND account_no = account.account_no
      AND status_code IN ('active', 'partially_consumed')
      AND expires_at > CURRENT_TIMESTAMP
) hold ON TRUE
WHERE account.tenant_id = 0 AND account.customer_no = ?
ORDER BY account.currency_code ASC, account.account_type_code ASC, account.id ASC
LIMIT ?`, customerNo, maximumAccountBalanceCount+1); loadErr != nil {
			return gerror.Wrap(loadErr, "load customer financial balances")
		}
		if len(rows) > maximumAccountBalanceCount {
			return bizerr.NewCode(CodeDirectoryIntegrity)
		}
		for _, row := range rows {
			if row.BalanceMinor < 0 || row.HeldBalanceMinor < 0 || row.HeldBalanceMinor > row.BalanceMinor ||
				row.UpdatedAt.IsZero() {
				return bizerr.NewCode(CodeDirectoryIntegrity)
			}
			result.Balances = append(result.Balances, BalanceDirectoryItem{
				AccountNo: row.AccountNo, AccountTypeCode: row.AccountTypeCode,
				CurrencyCode: row.CurrencyCode, StatusCode: row.StatusCode,
				Withdrawable: row.Withdrawable, BalanceMinor: row.BalanceMinor,
				HeldBalanceMinor:      row.HeldBalanceMinor,
				AvailableBalanceMinor: row.BalanceMinor - row.HeldBalanceMinor,
				UpdatedAt:             row.UpdatedAt.UTC(),
			})
		}
		receivables := make([]receivableCurrencyRow, 0, maximumAccountBalanceCount+1)
		if loadErr := tx.GetScan(&receivables, `
SELECT currency_code, COUNT(*) AS invoice_count,
       COALESCE(SUM(total_amount_minor - paid_amount_minor - waived_amount_minor
           - credit_amount_minor), 0)
           AS outstanding_minor
FROM plugin_idc_core_billing_invoice
WHERE tenant_id = 0 AND customer_no = ?
  AND status_code IN ('pending_payment', 'partially_paid', 'overdue')
GROUP BY currency_code
ORDER BY currency_code ASC
LIMIT ?`, customerNo, maximumAccountBalanceCount+1); loadErr != nil {
			return gerror.Wrap(loadErr, "load customer receivable summary")
		}
		if len(receivables) > maximumAccountBalanceCount {
			return bizerr.NewCode(CodeDirectoryIntegrity)
		}
		for _, row := range receivables {
			if !currencyPattern.MatchString(row.CurrencyCode) || row.InvoiceCount < 1 || row.OutstandingMinor < 0 {
				return bizerr.NewCode(CodeDirectoryIntegrity)
			}
			result.OpenInvoiceCount += row.InvoiceCount
			result.OpenReceivableByCurrency[row.CurrencyCode] = row.OutstandingMinor
		}
		value, loadErr := tx.GetValue(`
SELECT COUNT(*)
FROM plugin_idc_core_service_entitlement entitlement
JOIN plugin_idc_core_service_renewal_policy policy
  ON policy.tenant_id = entitlement.tenant_id
 AND policy.policy_no = entitlement.current_renewal_policy_no
 AND policy.policy_version = entitlement.current_renewal_policy_version
 AND policy.entitlement_no = entitlement.entitlement_no
WHERE entitlement.tenant_id = 0 AND entitlement.customer_no = ?
  AND entitlement.renewal_mode_code IN ('provider_confirmed', 'local_entitlement')
  AND entitlement.charge_type_code <> 'one_time'
  AND entitlement.period_unit_code <> 'once'
  AND entitlement.expires_at IS NOT NULL
  AND entitlement.status_code IN ('active', 'expired')
  AND policy.auto_renew_enabled = TRUE`, customerNo)
		if loadErr != nil {
			return gerror.Wrap(loadErr, "count customer automatic renewal policies")
		}
		result.AutomaticRenewalEnabled = value.Int()
		return nil
	})
	if err != nil {
		if bizerr.Is(err, CodeDirectoryIntegrity) {
			return nil, err
		}
		return nil, bizerr.WrapCode(err, CodeDirectoryUnavailable)
	}
	return result, nil
}
