package admin

import (
	"context"

	v1 "lina-plugin-idc-core/backend/api/admin/v1"
	"lina-plugin-idc-core/backend/cap/serveroperationresourcecap"
	"lina-plugin-idc-core/backend/internal/service/serveroperation"
)

func (c *ControllerV1) ServerAccessoryPriceList(ctx context.Context,
	_ *v1.ServerAccessoryPriceListReq) (*v1.ServerAccessoryPriceListRes, error) {
	policies, err := c.accessoryPricing.ListPolicies(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]*v1.ServerAccessoryPricePolicy, len(policies))
	for index := range policies {
		result[index] = toAPIAccessoryPrice(&policies[index])
	}
	return &v1.ServerAccessoryPriceListRes{Items: result}, nil
}

func (c *ControllerV1) ServerAccessoryPricePublish(ctx context.Context,
	req *v1.ServerAccessoryPricePublishReq) (*v1.ServerAccessoryPricePublishRes, error) {
	policy, err := c.accessoryPricing.PublishPolicy(ctx, serveroperation.AccessoryPricePolicyInput{
		Kind: serveroperationresourcecap.AccessoryKind(req.Kind), CurrencyCode: req.CurrencyCode,
		FreeCountPerServer: req.FreeCountPerServer, UnitAmountMinor: req.UnitAmountMinor,
		PeriodUnitCode: req.PeriodUnitCode, PeriodCount: req.PeriodCount,
		ExpectedVersion: req.ExpectedVersion,
	})
	if err != nil {
		return nil, err
	}
	return &v1.ServerAccessoryPricePublishRes{Policy: toAPIAccessoryPrice(policy)}, nil
}

func toAPIAccessoryPrice(value *serveroperation.AccessoryPricePolicy) *v1.ServerAccessoryPricePolicy {
	if value == nil {
		return nil
	}
	return &v1.ServerAccessoryPricePolicy{PolicyNo: value.PolicyNo, Kind: string(value.Kind),
		CurrencyCode: value.CurrencyCode, FreeCountPerServer: value.FreeCountPerServer,
		UnitAmountMinor: value.UnitAmountMinor, PeriodUnitCode: value.PeriodUnitCode,
		PeriodCount: value.PeriodCount, Version: value.Version, PublishedAt: value.PublishedAt.UnixMilli()}
}
