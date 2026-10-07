// This file loads the bounded set of current sales-channel records used by
// agent pricing. It deliberately keeps database access out of the pure rules.
package saleschannel

import (
	"context"
	"time"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-plugin-idc-core/backend/internal/dao"
	"lina-plugin-idc-core/backend/internal/model/do"
	"lina-plugin-idc-core/backend/internal/model/entity"
)

func (s *serviceImpl) EvaluateStored(
	ctx context.Context,
	customerNo, productNo, currencyCode string,
	basePriceMinor, costMinor, existingCreditUsed int64,
	now time.Time,
) (Offer, error) {
	if s.database == nil {
		return Offer{}, gerror.New("sales channel database is unavailable")
	}
	if customerNo == "" || productNo == "" || !validCurrencyCode(currencyCode) {
		return Offer{}, gerror.New("sales channel lookup request is invalid")
	}
	var (
		qualification *entity.SalesAgentQualification
		productRule   *entity.SalesAgentProductRule
		priceRule     *entity.SalesAgentPriceRule
		creditLimit   *entity.SalesAgentCreditLimit
	)
	qualificationQuery := s.database.Model(dao.SalesAgentQualification.Table()).Ctx(ctx).
		Where(do.SalesAgentQualification{TenantId: 0, CustomerNo: customerNo, StatusCode: string(QualificationActive)}).
		Where("effective_at <= ?", now).
		Where("expires_at IS NULL OR expires_at > ?", now).
		OrderDesc("effective_at").OrderDesc("id")
	if err := qualificationQuery.Scan(&qualification); err != nil {
		return Offer{}, gerror.Wrap(err, "load agent qualification")
	}
	if qualification == nil {
		return ineligible(ReasonQualificationUnavailable), nil
	}
	productQuery := s.database.Model(dao.SalesAgentProductRule.Table()).Ctx(ctx).
		Where(do.SalesAgentProductRule{TenantId: 0, LevelNo: qualification.LevelNo, ProductNo: productNo, StatusCode: string(RulePublished)}).
		Where("effective_at <= ?", now).
		Where("expires_at IS NULL OR expires_at > ?", now).
		OrderDesc("effective_at").OrderDesc("version")
	if err := productQuery.Scan(&productRule); err != nil {
		return Offer{}, gerror.Wrap(err, "load agent product rule")
	}
	if productRule == nil {
		return ineligible(ReasonProductNotEligible), nil
	}
	priceQuery := s.database.Model(dao.SalesAgentPriceRule.Table()).Ctx(ctx).
		Where(do.SalesAgentPriceRule{TenantId: 0, ProductRuleNo: productRule.ProductRuleNo, CurrencyCode: currencyCode, StatusCode: string(RulePublished)}).
		Where("effective_at <= ?", now).
		Where("expires_at IS NULL OR expires_at > ?", now).
		OrderDesc("effective_at").OrderDesc("version")
	if err := priceQuery.Scan(&priceRule); err != nil {
		return Offer{}, gerror.Wrap(err, "load agent price rule")
	}
	if priceRule == nil {
		return ineligible(ReasonPriceRuleUnavailable), nil
	}
	creditQuery := s.database.Model(dao.SalesAgentCreditLimit.Table()).Ctx(ctx).
		Where(do.SalesAgentCreditLimit{TenantId: 0, QualificationNo: qualification.QualificationNo, CurrencyCode: currencyCode, StatusCode: CreditLimitActive}).
		Where("effective_at <= ?", now).
		Where("expires_at IS NULL OR expires_at > ?", now).
		OrderDesc("effective_at").OrderDesc("id")
	if err := creditQuery.Scan(&creditLimit); err != nil {
		return Offer{}, gerror.Wrap(err, "load agent credit limit")
	}
	return s.Evaluate(ctx, OfferRequest{
		Now: now, ProductNo: productNo, CurrencyCode: currencyCode,
		BasePriceMinor: basePriceMinor, CostMinor: costMinor, ExistingCreditUsed: existingCreditUsed,
		Qualification: qualification, ProductRule: productRule, PriceRule: priceRule, CreditLimit: creditLimit,
	})
}
