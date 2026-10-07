package customeraccount

import (
	"context"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-plugin-idc-core/backend/api/customeraccount/v1"
)

func (c *ControllerV1) SalesChannelOverviewGet(ctx context.Context, _ *v1.SalesChannelOverviewGetReq) (*v1.SalesChannelOverviewGetRes, error) {
	if c.salesChannel == nil {
		return nil, gerror.New("customer sales channel service is unavailable")
	}
	profile, err := c.customers.CurrentProfile(ctx)
	if err != nil {
		return nil, err
	}
	snapshot, err := c.salesChannel.LoadCustomerSnapshot(ctx, profile.Subject.CustomerNo)
	if err != nil {
		return nil, err
	}
	res := &v1.SalesChannelOverviewGetRes{Links: make([]*v1.CustomerReferralLink, 0, len(snapshot.Links)), Currencies: make([]*v1.CustomerCommissionCurrency, 0, len(snapshot.Currencies))}
	for _, item := range snapshot.Links {
		res.Links = append(res.Links, &v1.CustomerReferralLink{LinkNo: item.LinkNo, Policy: item.Policy, Status: item.Status})
	}
	for _, item := range snapshot.Currencies {
		res.Currencies = append(res.Currencies, &v1.CustomerCommissionCurrency{CurrencyCode: item.CurrencyCode, PendingMinor: item.PendingMinor, AvailableMinor: item.AvailableMinor, FrozenMinor: item.FrozenMinor, PaidMinor: item.PaidMinor})
	}
	return res, nil
}
