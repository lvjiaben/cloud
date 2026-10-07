package customeraccount

import (
	"context"

	"lina-plugin-idc-core/backend/api/customeraccount/v1"
)

func (c *ControllerV1) CommissionOverviewGet(ctx context.Context, req *v1.CommissionOverviewGetReq) (res *v1.CommissionOverviewGetRes, err error) {
	profile, err := c.customers.CurrentProfile(ctx)
	if err != nil {
		return nil, err
	}
	item, err := c.salesChannel.LoadCustomerCommissionOverview(ctx, profile.Subject.CustomerNo, req.CurrencyCode, req.Limit)
	if err != nil {
		return nil, err
	}
	res = &v1.CommissionOverviewGetRes{Accounts: make([]*v1.CustomerCommissionAccount, 0, len(item.Accounts)), Entries: make([]*v1.CustomerCommissionEntry, 0, len(item.Entries))}
	for _, account := range item.Accounts {
		res.Accounts = append(res.Accounts, &v1.CustomerCommissionAccount{CurrencyCode: account.CurrencyCode, AvailableAmountMinor: account.AvailableAmountMinor, PendingAmountMinor: account.PendingAmountMinor, MaturedAmountMinor: account.MaturedAmountMinor, SettledAmountMinor: account.SettledAmountMinor, FrozenAmountMinor: account.FrozenAmountMinor, StatusCode: account.StatusCode})
	}
	for _, entry := range item.Entries {
		res.Entries = append(res.Entries, &v1.CustomerCommissionEntry{EntryNo: entry.EntryNo, CurrencyCode: entry.CurrencyCode, AmountMinor: entry.AmountMinor, StatusCode: entry.StatusCode, SourceOrderNo: entry.SourceOrderNo, MaturityAt: entry.MaturityAt, ReversedAmountMinor: entry.ReversedAmountMinor, CreatedAt: entry.CreatedAt})
	}
	return res, nil
}
