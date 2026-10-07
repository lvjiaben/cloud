// This file maps staff refund directory, detail, review, proof, and policy APIs to billing services.

package admin

import (
	"context"
	"strconv"
	"time"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/api/admin/v1"
	"lina-plugin-idc-core/backend/internal/service/billing"
)

func (c *ControllerV1) RefundManagementList(
	ctx context.Context,
	req *v1.RefundManagementListReq,
) (*v1.RefundManagementListRes, error) {
	page, err := c.refunds.ListRefunds(ctx, billing.RefundDirectoryFilter{
		CustomerNo: req.CustomerNo, Status: req.StatusCode, Cursor: req.Cursor, Limit: req.Limit, Page: req.Page,
	})
	if err != nil {
		return nil, err
	}
	result := &v1.RefundManagementListRes{
		Total:      page.Total,
		Refunds:    make([]*v1.RefundManagementMutation, 0, len(page.Refunds)),
		NextCursor: page.NextCursor,
	}
	for _, refund := range page.Refunds {
		result.Refunds = append(result.Refunds, toAPIRefundManagementMutation(refund))
	}
	return result, nil
}

func (c *ControllerV1) RefundManagementDetail(
	ctx context.Context,
	req *v1.RefundManagementDetailReq,
) (*v1.RefundManagementDetailRes, error) {
	detail, err := c.refunds.GetRefundDetail(ctx, req.RefundNo)
	if err != nil {
		return nil, err
	}
	result := &v1.RefundManagementDetail{
		Refund: toAPIRefundManagementMutation(detail.Refund),
		Items:  make([]*v1.RefundManagementItem, 0, len(detail.Items)),
	}
	for _, item := range detail.Items {
		mapped := &v1.RefundManagementItem{
			RefundItemNo: item.RefundItemNo, InvoiceItemNo: item.InvoiceItemNo,
			OrderItemNo: item.OrderItemNo, Quantity: item.Quantity,
			AmountMinor:           strconv.FormatInt(item.AmountMinor, 10),
			RefundableAmountMinor: strconv.FormatInt(item.RefundableAmountMinor, 10),
			DescriptionZhCN:       item.DescriptionZhCN, DescriptionEnUS: item.DescriptionEnUS,
			Impacts: make([]*v1.RefundManagementImpact, 0, len(item.Impacts)),
		}
		for _, impact := range item.Impacts {
			mapped.Impacts = append(mapped.Impacts, &v1.RefundManagementImpact{
				EntitlementNo: impact.EntitlementNo, EntitlementVersion: impact.EntitlementVersion,
				StatusCode: impact.StatusCode, RequiresTermination: impact.RequiresTermination,
			})
		}
		result.Items = append(result.Items, mapped)
	}
	return &v1.RefundManagementDetailRes{Detail: result}, nil
}

func (c *ControllerV1) RefundManagementReviewProof(
	ctx context.Context,
	req *v1.RefundManagementReviewProofReq,
) (*v1.RefundManagementReviewProofRes, error) {
	proof, err := c.refunds.IssueRefundReviewProof(ctx, refundReviewIntent(
		req.RefundNo, req.RefundManagementReviewIntent,
	))
	if err != nil {
		return nil, err
	}
	return &v1.RefundManagementReviewProofRes{Proof: toAPIManagementActionProof(proof)}, nil
}

func (c *ControllerV1) RefundManagementReview(
	ctx context.Context,
	req *v1.RefundManagementReviewReq,
) (*v1.RefundManagementReviewRes, error) {
	result, err := c.refunds.ReviewRefund(ctx, refundReviewIntent(
		req.RefundNo, req.RefundManagementReviewIntent,
	), req.ProofToken)
	if err != nil {
		return nil, err
	}
	return &v1.RefundManagementReviewRes{Refund: toAPIRefundManagementMutation(result)}, nil
}

func (c *ControllerV1) RefundManagementPolicyConfigure(
	ctx context.Context,
	req *v1.RefundManagementPolicyConfigureReq,
) (*v1.RefundManagementPolicyConfigureRes, error) {
	threshold, err := strconv.ParseInt(req.HighAmountThresholdMinor, 10, 64)
	if err != nil || threshold <= 0 || strconv.FormatInt(threshold, 10) != req.HighAmountThresholdMinor {
		return nil, bizerr.NewCode(billing.CodeRefundInvalid)
	}
	policy, err := c.refunds.ConfigureRefundPolicy(ctx, billing.RefundPolicyInput{
		CurrencyCode: req.CurrencyCode, HighAmountThresholdMinor: threshold,
		AutomaticApprovalEnabled: req.AutomaticApprovalEnabled,
	}, req.ExpectedVersion)
	if err != nil {
		return nil, err
	}
	return &v1.RefundManagementPolicyConfigureRes{Policy: &v1.RefundManagementPolicy{
		PolicyNo: policy.PolicyNo, CurrencyCode: policy.CurrencyCode, Version: policy.Version,
		HighAmountThresholdMinor: strconv.FormatInt(policy.HighAmountThresholdMinor, 10),
		AutomaticApprovalEnabled: policy.AutomaticApprovalEnabled,
		ConfiguredBy:             policy.ConfiguredBy, EffectiveAt: policy.EffectiveAt.UnixMilli(),
	}}, nil
}

func refundReviewIntent(
	refundNo string,
	value v1.RefundManagementReviewIntent,
) billing.ManagementRefundReviewIntent {
	return billing.ManagementRefundReviewIntent{
		RequestNo: value.RequestNo, RefundNo: refundNo,
		ExpectedVersion: value.ExpectedVersion,
		Decision:        billing.RefundReviewDecision(value.DecisionCode),
		CommentDigest:   value.CommentDigest, EvidenceDigest: value.EvidenceDigest,
	}
}

func toAPIRefundManagementMutation(value *billing.OnlineRefund) *v1.RefundManagementMutation {
	if value == nil {
		return nil
	}
	return &v1.RefundManagementMutation{
		RefundNo: value.RefundNo, CustomerNo: value.CustomerNo,
		CustomerCode: value.CustomerCode, CustomerDisplayName: value.CustomerDisplayName,
		CustomerPrimaryPhone: value.CustomerPrimaryPhone,
		OrderNo:              value.OrderNo, InvoiceNo: value.InvoiceNo,
		CurrencyCode: value.CurrencyCode, StatusCode: value.Status,
		ReasonCode: string(value.Reason), ReasonDetail: value.ReasonDetail,
		AmountMinor: strconv.FormatInt(value.AmountMinor, 10), ApprovalModeCode: value.ApprovalMode,
		DestinationCode:                value.Destination,
		RequiresEntitlementTermination: value.RequiresEntitlementTermination,
		HasEntitlementImpact:           value.HasResourceImpact, HasDangerousOperation: value.HasDangerousOperation,
		HighAmount: value.HighAmount, Version: value.Version, ImpactDigest: value.ImpactDigest,
		SubmittedAt: value.SubmittedAt.UnixMilli(), ApprovedAt: refundOptionalUnixMillis(value.ApprovedAt),
		CompletedAt: refundOptionalUnixMillis(value.CompletedAt), Reused: value.Reused,
	}
}

func refundOptionalUnixMillis(value *time.Time) *int64 {
	if value == nil {
		return nil
	}
	result := value.UnixMilli()
	return &result
}
