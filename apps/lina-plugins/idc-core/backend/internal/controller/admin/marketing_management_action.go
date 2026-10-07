// This file maps audited marketing action results to stable API DTOs.

package admin

import (
	"strconv"

	v1 "lina-plugin-idc-core/backend/api/admin/v1"
	"lina-plugin-idc-core/backend/internal/service/marketing"
)

func toAPIManagedAssetMutation(input *marketing.ManagedAssetMutation) *v1.MarketingManagedAssetMutation {
	return &v1.MarketingManagedAssetMutation{
		AssetNo: input.AssetNo, CustomerNo: input.CustomerNo, Status: input.Status,
		UpdatedAt: input.UpdatedAt.UnixMilli(), Reused: input.Reused,
	}
}

func toAPIManagedValueGrantFinancialMutation(
	input *marketing.ManagedValueGrantFinancialMutation,
) *v1.MarketingValueGrantFinancialMutation {
	if input == nil {
		return nil
	}
	return &v1.MarketingValueGrantFinancialMutation{
		AdjustmentNo: input.AdjustmentNo, GrantNo: input.GrantNo,
		AdjustmentType: input.AdjustmentType, Status: input.Status,
		CustomerNo: input.CustomerNo, CurrencyCode: input.CurrencyCode,
		AmountMinor:         strconv.FormatInt(input.AmountMinor, 10),
		LedgerTransactionNo: input.LedgerTransactionNo, Reused: input.Reused,
	}
}

func toAPIManagedPointsMutation(input *marketing.ManagedPointsMutation) *v1.MarketingManagedPointsMutation {
	return &v1.MarketingManagedPointsMutation{
		LotNo: input.LotNo, CustomerNo: input.CustomerNo,
		AvailablePoints: input.AvailablePoints, FrozenPoints: input.FrozenPoints,
		DebtPoints: input.DebtPoints, Reused: input.Reused,
	}
}
