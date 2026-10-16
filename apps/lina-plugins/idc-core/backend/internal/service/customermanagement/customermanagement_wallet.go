// This file decorates a bounded visible customer page with wallets in one query.

package customermanagement

import (
	"context"
	"fmt"
	"time"

	"github.com/gogf/gf/v2/errors/gerror"
	"lina-plugin-idc-core/backend/internal/dao"
)

// walletHoldStatus identifies the two states whose unexpired remainder reduces spendable funds.
type walletHoldStatus string

const (
	walletHoldActive            walletHoldStatus = "active"
	walletHoldPartiallyConsumed walletHoldStatus = "partially_consumed"
)

// loadWalletProjection receives only customers already admitted by the staff query.
func loadWalletProjection(ctx context.Context, customers []string, destination map[string][]BalanceSummary) error {
	for _, customerNo := range customers {
		destination[customerNo] = []BalanceSummary{}
	}
	var rows []struct {
		CustomerNo       string
		AccountTypeCode  string
		CurrencyCode     string
		StatusCode       string
		BalanceMinor     int64
		HeldBalanceMinor int64
		UpdatedAt        time.Time
	}
	err := dao.BillingLedgerAccount.Ctx(ctx).As("a").
		Fields(fmt.Sprintf(`a.customer_no,a.account_type_code,a.currency_code,a.status_code,
COALESCE((SELECT s.closing_balance_minor FROM plugin_idc_core_billing_balance_snapshot s WHERE s.tenant_id=a.tenant_id AND s.account_no=a.account_no ORDER BY s.sequence_no DESC LIMIT 1),0) AS balance_minor,
COALESCE((SELECT SUM(h.amount_minor-h.consumed_amount_minor-h.released_amount_minor) FROM plugin_idc_core_billing_fund_hold h WHERE h.tenant_id=a.tenant_id AND h.account_no=a.account_no AND h.status_code IN ('%s','%s') AND h.expires_at>CURRENT_TIMESTAMP),0) AS held_balance_minor,
COALESCE((SELECT s.created_at FROM plugin_idc_core_billing_balance_snapshot s WHERE s.tenant_id=a.tenant_id AND s.account_no=a.account_no ORDER BY s.sequence_no DESC LIMIT 1),a.updated_at) AS updated_at`, walletHoldActive, walletHoldPartiallyConsumed)).
		Where("a.tenant_id", 0).WhereIn("a.customer_no", customers).
		OrderAsc("a.customer_no").OrderAsc("a.currency_code").OrderAsc("a.account_type_code").Limit(len(customers)*64 + 1).Scan(&rows)
	if err != nil {
		return gerror.Wrap(err, "load visible customer wallets")
	}
	if len(rows) > len(customers)*64 {
		return gerror.New("customer wallet projection exceeds limit")
	}
	for _, row := range rows {
		if _, visible := destination[row.CustomerNo]; !visible || row.BalanceMinor < 0 || row.HeldBalanceMinor < 0 || row.HeldBalanceMinor > row.BalanceMinor || row.UpdatedAt.IsZero() {
			return gerror.New("invalid customer wallet projection")
		}
		if len(destination[row.CustomerNo]) >= 64 {
			return gerror.New("customer wallet projection exceeds per-customer limit")
		}
		destination[row.CustomerNo] = append(destination[row.CustomerNo], BalanceSummary{AccountTypeCode: row.AccountTypeCode, CurrencyCode: row.CurrencyCode, StatusCode: row.StatusCode, BalanceMinor: row.BalanceMinor, HeldBalanceMinor: row.HeldBalanceMinor, AvailableBalanceMinor: row.BalanceMinor - row.HeldBalanceMinor, UpdatedAt: row.UpdatedAt})
	}
	return nil
}
