package admin

import (
	"context"
	"time"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-plugin-idc-core/backend/api/admin/v1"
	"lina-plugin-idc-core/backend/internal/model/entity"
	"lina-plugin-idc-core/backend/internal/service/saleschannel"
)

func (c *ControllerV1) SalesChannelOfferEvaluate(ctx context.Context, req *v1.SalesChannelOfferEvaluateReq) (*v1.SalesChannelOfferEvaluateRes, error) {
	if c.salesChannel == nil {
		return nil, gerror.New("sales channel service is unavailable")
	}
	offer, err := c.salesChannel.EvaluateStored(ctx, req.CustomerNo, req.ProductNo, req.CurrencyCode, req.BasePriceMinor, req.CostMinor, req.ExistingCreditUsed, time.UnixMilli(req.NowUnixMilli))
	if err != nil {
		return nil, err
	}
	res := &v1.SalesChannelOfferEvaluateRes{Eligible: offer.Eligible, PriceMinor: offer.PriceMinor, RemainingCreditMinor: offer.RemainingCreditMinor, ReasonCode: offer.ReasonCode}
	if offer.Eligible {
		res.Snapshot = &v1.SalesChannelPriceSnapshot{
			QualificationNo: offer.Snapshot.QualificationNo, QualificationVersion: offer.Snapshot.QualificationVersion,
			ProductRuleNo: offer.Snapshot.ProductRuleNo, ProductRuleVersion: offer.Snapshot.ProductRuleVersion,
			PriceRuleNo: offer.Snapshot.PriceRuleNo, PriceRuleVersion: offer.Snapshot.PriceRuleVersion,
			CurrencyCode: offer.Snapshot.CurrencyCode, BasePriceMinor: offer.Snapshot.BasePriceMinor, PriceMinor: offer.Snapshot.PriceMinor,
		}
	}
	return res, nil
}

// SalesChannelQualificationTransitionValidate checks the qualification state
// machine without writing anything. The caller must append the qualification
// event and projection in its own transaction after this decision.
func (c *ControllerV1) SalesChannelQualificationTransitionValidate(ctx context.Context, req *v1.SalesChannelQualificationTransitionValidateReq) (*v1.SalesChannelQualificationTransitionValidateRes, error) {
	if c.salesChannel == nil {
		return nil, gerror.New("sales channel service is unavailable")
	}
	request := saleschannel.QualificationTransitionRequest{
		Now: time.UnixMilli(req.NowUnixMilli), CurrentStatus: saleschannel.QualificationStatus(req.CurrentStatus),
		NextStatus: saleschannel.QualificationStatus(req.NextStatus), CurrentLevelNo: req.CurrentLevelNo,
		NextLevelNo: req.NextLevelNo, EffectiveAt: time.UnixMilli(req.EffectiveAtUnixMilli),
		ReasonCode: req.ReasonCode, ActorReference: req.ActorReference, EvidenceDigest: req.EvidenceDigest,
	}
	if req.ExpiresAtUnixMilli != nil {
		expiresAt := time.UnixMilli(*req.ExpiresAtUnixMilli)
		request.ExpiresAt = &expiresAt
	}
	decision, err := c.salesChannel.ValidateQualificationTransition(ctx, request)
	if err != nil {
		return nil, err
	}
	return &v1.SalesChannelQualificationTransitionValidateRes{Allowed: decision.Allowed, ReasonCode: decision.ReasonCode}, nil
}

func (c *ControllerV1) SalesChannelAffiliateAttribute(ctx context.Context, req *v1.SalesChannelAffiliateAttributeReq) (*v1.SalesChannelAffiliateAttributeRes, error) {
	if c.salesChannel == nil {
		return nil, gerror.New("sales channel service is unavailable")
	}
	decision, err := c.salesChannel.PersistStoredAttribution(ctx, time.UnixMilli(req.NowUnixMilli), req.LinkNo, req.TouchNo, req.ReferredCustomerNo)
	if err != nil {
		return nil, err
	}
	return &v1.SalesChannelAffiliateAttributeRes{StatusCode: decision.StatusCode, RiskCode: decision.RiskReasonCode, PolicyCode: decision.PolicyCode, ReferrerCustomerNo: decision.ReferrerCustomerNo, ReferredCustomerNo: decision.ReferredCustomerNo}, nil
}

func (c *ControllerV1) SalesChannelCommissionCalculate(ctx context.Context, req *v1.SalesChannelCommissionCalculateReq) (*v1.SalesChannelCommissionCalculateRes, error) {
	if c.salesChannel == nil {
		return nil, gerror.New("sales channel service is unavailable")
	}
	if len(req.Tiers) > 128 {
		return nil, gerror.New("commission tiers exceed 128 items")
	}
	rule := &entity.SalesCommissionRule{RuleNo: req.RuleNo, OperationTypeCode: req.OperationTypeCode, BeneficiaryTypeCode: req.BeneficiaryTypeCode, CurrencyCode: req.CurrencyCode, CalculationTypeCode: req.CalculationTypeCode, FixedAmountMinor: req.FixedAmountMinor, RateBasisPoints: req.RateBasisPoints, CapAmountMinor: req.CapAmountMinor, MaturityDays: req.MaturityDays, StatusCode: string(saleschannel.RulePublished), Version: req.RuleVersion}
	if req.ProductNo != "" {
		rule.ProductNo = &req.ProductNo
	}
	effective := time.UnixMilli(req.EffectiveAtUnixMilli)
	rule.EffectiveAt = &effective
	if req.ExpiresAtUnixMilli != nil {
		value := time.UnixMilli(*req.ExpiresAtUnixMilli)
		rule.ExpiresAt = &value
	}
	tiers := make([]entity.SalesCommissionTier, 0, len(req.Tiers))
	for _, input := range req.Tiers {
		if input == nil {
			return nil, gerror.New("commission tier cannot be empty")
		}
		if input.RateBasisPoints < 0 || input.RateBasisPoints > 10000 || input.LowerBoundMinor < 0 || (input.UpperBoundMinor != nil && *input.UpperBoundMinor < input.LowerBoundMinor) {
			return nil, gerror.New("commission tier bounds are invalid")
		}
		tiers = append(tiers, entity.SalesCommissionTier{RuleNo: req.RuleNo, TierNo: input.TierNo, LowerBoundMinor: input.LowerBoundMinor, UpperBoundMinor: input.UpperBoundMinor, RateBasisPoints: input.RateBasisPoints})
	}
	snapshot, err := c.salesChannel.CalculateCommission(ctx, saleschannel.CommissionRequest{Now: time.UnixMilli(req.NowUnixMilli), OperationTypeCode: req.OperationTypeCode, BeneficiaryTypeCode: req.BeneficiaryTypeCode, ProductNo: req.ProductNo, CurrencyCode: req.CurrencyCode, BaseAmountMinor: req.BaseAmountMinor, Rule: rule, Tiers: tiers})
	if err != nil {
		return nil, err
	}
	return &v1.SalesChannelCommissionCalculateRes{AmountMinor: snapshot.AmountMinor, MaturesAtUnixMilli: snapshot.MaturesAt.UnixMilli(), RuleNo: snapshot.RuleNo, RuleVersion: snapshot.RuleVersion, TierNo: snapshot.TierNo, CurrencyCode: snapshot.CurrencyCode, OperationTypeCode: snapshot.OperationTypeCode}, nil
}

func (c *ControllerV1) SalesChannelOverviewGet(ctx context.Context, _ *v1.SalesChannelOverviewGetReq) (*v1.SalesChannelOverviewGetRes, error) {
	if c.salesChannel == nil {
		return nil, gerror.New("sales channel service is unavailable")
	}
	overview, err := c.salesChannel.LoadOverview(ctx)
	if err != nil {
		return nil, err
	}
	res := &v1.SalesChannelOverviewGetRes{
		Qualifications: overview.Qualifications, AffiliateLinks: overview.AffiliateLinks,
		Attributions: overview.Attributions, PendingMaturity: overview.PendingMaturity,
		PendingSettlement: overview.PendingSettlement,
		Currencies:        make([]*v1.SalesChannelCurrencyOverview, 0, len(overview.Currencies)),
	}
	for _, item := range overview.Currencies {
		res.Currencies = append(res.Currencies, &v1.SalesChannelCurrencyOverview{CurrencyCode: item.CurrencyCode, AccruedMinor: item.AccruedMinor, MaturedMinor: item.MaturedMinor})
	}
	return res, nil
}

func (c *ControllerV1) SalesChannelCustomerReferrerGet(ctx context.Context, req *v1.SalesChannelCustomerReferrerGetReq) (*v1.SalesChannelCustomerReferrerGetRes, error) {
	if c.salesChannel == nil {
		return nil, gerror.New("sales channel service is unavailable")
	}
	if _, err := c.customerManagement.GetOverview(ctx, req.CustomerNo); err != nil {
		return nil, err
	}
	reader, ok := c.salesChannel.(saleschannel.ManagementReferrerReader)
	if !ok {
		return nil, gerror.New("customer referrer directory is unavailable")
	}
	result, err := reader.LoadManagementReferrer(ctx, req.CustomerNo)
	if err != nil {
		return nil, err
	}
	return &v1.SalesChannelCustomerReferrerGetRes{
		CustomerNo: result.CustomerNo, CustomerCode: result.CustomerCode, DisplayName: result.DisplayName,
	}, nil
}

func (c *ControllerV1) SalesChannelSettlementReview(ctx context.Context, req *v1.SalesChannelSettlementReviewReq) (*v1.SalesChannelSettlementReviewRes, error) {
	if c.salesChannel == nil {
		return nil, gerror.New("sales channel service is unavailable")
	}
	if err := c.salesChannel.ReviewSettlement(ctx, saleschannel.SettlementReviewRequest{SettlementNo: req.SettlementNo, DecisionCode: req.DecisionCode, ReasonCode: req.ReasonCode, EvidenceDigest: req.EvidenceDigest}); err != nil {
		return nil, err
	}
	return &v1.SalesChannelSettlementReviewRes{Success: true}, nil
}

func (c *ControllerV1) SalesChannelPayoutBatchCreate(ctx context.Context, req *v1.SalesChannelPayoutBatchCreateReq) (*v1.SalesChannelPayoutBatchCreateRes, error) {
	if c.salesChannel == nil {
		return nil, gerror.New("sales channel service is unavailable")
	}
	result, err := c.salesChannel.CreatePayoutBatch(ctx, saleschannel.PayoutBatchRequest{CurrencyCode: req.CurrencyCode, SettlementNos: req.SettlementNos})
	if err != nil {
		return nil, err
	}
	return &v1.SalesChannelPayoutBatchCreateRes{BatchNo: result.BatchNo, CurrencyCode: result.CurrencyCode, ItemCount: result.ItemCount, TotalAmountMinor: result.TotalAmountMinor}, nil
}

func (c *ControllerV1) SalesChannelPayoutResult(ctx context.Context, req *v1.SalesChannelPayoutResultReq) (*v1.SalesChannelPayoutResultRes, error) {
	if c.salesChannel == nil {
		return nil, gerror.New("sales channel service is unavailable")
	}
	if err := c.salesChannel.RecordPayoutResult(ctx, saleschannel.PayoutResultRequest{BatchNo: req.BatchNo, SettlementNo: req.SettlementNo, StatusCode: req.StatusCode, FailureReasonCode: req.FailureReasonCode}); err != nil {
		return nil, err
	}
	return &v1.SalesChannelPayoutResultRes{Success: true}, nil
}
