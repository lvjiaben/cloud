// This file maps billing refund projections into customer API DTOs.

package account

import (
	"strconv"
	"time"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/api/account/v1"
	"lina-plugin-idc-core/backend/internal/service/billing"
)

func toRefundPreviewInput(orderNo string, values []*v1.RefundSelectionInput) (billing.RefundPreviewInput, error) {
	result := billing.RefundPreviewInput{
		OrderNo: orderNo, Selections: make([]billing.RefundSelectionInput, 0, len(values)),
	}
	for _, value := range values {
		if value == nil {
			return billing.RefundPreviewInput{}, bizerr.NewCode(billing.CodeRefundInvalid)
		}
		parsed, err := strconv.ParseInt(value.AmountMinor, 10, 64)
		if err != nil || parsed <= 0 || strconv.FormatInt(parsed, 10) != value.AmountMinor {
			return billing.RefundPreviewInput{}, bizerr.NewCode(billing.CodeRefundInvalid)
		}
		result.Selections = append(result.Selections, billing.RefundSelectionInput{
			InvoiceItemNo: value.InvoiceItemNo, EntitlementNos: append([]string(nil), value.EntitlementNos...),
			Quantity: value.Quantity, AmountMinor: parsed,
		})
	}
	return result, nil
}

func toAPIRefundPreview(value *billing.RefundPreview) *v1.RefundPreview {
	if value == nil {
		return nil
	}
	result := &v1.RefundPreview{
		OrderNo: value.OrderNo, InvoiceNo: value.InvoiceNo, CurrencyCode: value.CurrencyCode,
		AmountMinor: amount(value.AmountMinor), ApprovalModeCode: value.ApprovalMode,
		RequiresEntitlementTermination: value.RequiresEntitlementTermination,
		HasEntitlementImpact:           value.HasResourceImpact,
		HasDangerousOperation:          value.HasDangerousOperation, HighAmount: value.HighAmount,
		ImpactDigest: value.ImpactDigest,
		Items:        make([]*v1.RefundPreviewItem, 0, len(value.Items)),
	}
	for _, item := range value.Items {
		mapped := &v1.RefundPreviewItem{
			InvoiceItemNo: item.InvoiceItemNo, OrderItemNo: item.OrderItemNo,
			Quantity:              item.Quantity,
			AmountMinor:           amount(item.AmountMinor),
			RefundableAmountMinor: amount(item.RefundableAmountMinor),
			DescriptionZhCN:       item.DescriptionZhCN, DescriptionEnUS: item.DescriptionEnUS,
			Impacts: make([]*v1.RefundEntitlementImpact, 0, len(item.Impacts)),
		}
		for _, impact := range item.Impacts {
			mapped.Impacts = append(mapped.Impacts, &v1.RefundEntitlementImpact{
				ProductNameZhCN: impact.ProductNameZhCN, ProductNameEnUS: impact.ProductNameEnUS,
				EntitlementVersion: impact.EntitlementVersion,
				StatusCode:         impact.StatusCode, RequiresTermination: impact.RequiresTermination,
			})
		}
		result.Items = append(result.Items, mapped)
	}
	return result
}

func toAPIRefund(value *billing.OnlineRefund) *v1.Refund {
	if value == nil {
		return nil
	}
	return &v1.Refund{
		RefundNo: value.RefundNo, OrderNo: value.OrderNo, InvoiceNo: value.InvoiceNo,
		CurrencyCode: value.CurrencyCode, StatusCode: value.Status,
		ReasonCode: string(value.Reason), ReasonDetail: value.ReasonDetail,
		AmountMinor: amount(value.AmountMinor), ApprovalModeCode: value.ApprovalMode,
		DestinationCode:                value.Destination,
		RequiresEntitlementTermination: value.RequiresEntitlementTermination,
		HasEntitlementImpact:           value.HasResourceImpact,
		HasDangerousOperation:          value.HasDangerousOperation, HighAmount: value.HighAmount,
		Version: value.Version, ImpactDigest: value.ImpactDigest,
		SubmittedAt: value.SubmittedAt.UnixMilli(),
		ApprovedAt:  optionalUnixMillis(value.ApprovedAt), CompletedAt: optionalUnixMillis(value.CompletedAt),
		Reused: value.Reused,
	}
}

func optionalUnixMillis(value *time.Time) *int64 {
	if value == nil {
		return nil
	}
	milliseconds := value.UnixMilli()
	return &milliseconds
}
