// This file calculates one immutable commission snapshot from a published rule.
package saleschannel

import (
	"context"
	"math/big"
	"sort"
	"time"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-plugin-idc-core/backend/internal/model/entity"
)

const (
	// CommissionFixed pays a fixed amount for the matching operation.
	CommissionFixed = "fixed"
	// CommissionRate pays a percentage expressed in basis points.
	CommissionRate = "rate"
	// CommissionTiered selects a rate from ordered amount tiers.
	CommissionTiered = "tiered"
)

// CommissionRequest contains the frozen business amount and the candidate
// rules loaded for one beneficiary, product and currency.
type CommissionRequest struct {
	Now                 time.Time
	OperationTypeCode   string
	BeneficiaryTypeCode string
	ProductNo           string
	CurrencyCode        string
	BaseAmountMinor     int64
	Rule                *entity.SalesCommissionRule
	Tiers               []entity.SalesCommissionTier
}

// CommissionSnapshot is safe to persist in the immutable accrual record.
type CommissionSnapshot struct {
	AmountMinor       int64
	MaturesAt         time.Time
	RuleNo            string
	RuleVersion       int64
	TierNo            string
	CurrencyCode      string
	OperationTypeCode string
}

// CalculateCommission returns one deterministic commission amount and its
// maturity date. It never writes the ledger itself.
func (s *serviceImpl) CalculateCommission(ctx context.Context, request CommissionRequest) (CommissionSnapshot, error) {
	if ctx == nil {
		return CommissionSnapshot{}, gerror.New("commission calculation context is required")
	}
	if err := ctx.Err(); err != nil {
		return CommissionSnapshot{}, gerror.Wrap(err, "commission calculation context is unavailable")
	}
	if request.Now.IsZero() || request.Rule == nil || request.BaseAmountMinor < 0 ||
		request.Rule.MaturityDays < 0 || request.Rule.MaturityDays > 3650 ||
		!validCurrencyCode(request.CurrencyCode) || request.Rule.CurrencyCode != request.CurrencyCode ||
		request.Rule.OperationTypeCode != request.OperationTypeCode ||
		request.Rule.BeneficiaryTypeCode != request.BeneficiaryTypeCode ||
		(request.Rule.ProductNo != nil && *request.Rule.ProductNo != request.ProductNo) ||
		request.Rule.StatusCode != string(RulePublished) ||
		!validWindow(request.Now, request.Rule.EffectiveAt, request.Rule.ExpiresAt) {
		return CommissionSnapshot{}, gerror.New("commission_rule_unavailable")
	}
	if request.Rule.CapAmountMinor != nil && *request.Rule.CapAmountMinor < 0 {
		return CommissionSnapshot{}, gerror.New("commission_cap_invalid")
	}
	amount, tierNo, err := commissionAmount(request.BaseAmountMinor, request.Rule, request.Tiers)
	if err != nil {
		return CommissionSnapshot{}, err
	}
	if request.Rule.CapAmountMinor != nil && amount > *request.Rule.CapAmountMinor {
		amount = *request.Rule.CapAmountMinor
	}
	if amount < 0 {
		return CommissionSnapshot{}, gerror.New("commission_amount_invalid")
	}
	return CommissionSnapshot{
		AmountMinor:       amount,
		MaturesAt:         request.Now.Add(time.Duration(request.Rule.MaturityDays) * 24 * time.Hour),
		RuleNo:            request.Rule.RuleNo,
		RuleVersion:       request.Rule.Version,
		TierNo:            tierNo,
		CurrencyCode:      request.CurrencyCode,
		OperationTypeCode: request.OperationTypeCode,
	}, nil
}

func commissionAmount(base int64, rule *entity.SalesCommissionRule, tiers []entity.SalesCommissionTier) (int64, string, error) {
	switch rule.CalculationTypeCode {
	case CommissionFixed:
		if rule.FixedAmountMinor == nil || *rule.FixedAmountMinor < 0 {
			return 0, "", gerror.New("commission_fixed_amount_missing")
		}
		return *rule.FixedAmountMinor, "", nil
	case CommissionRate:
		if rule.RateBasisPoints == nil || *rule.RateBasisPoints < 0 || *rule.RateBasisPoints > 10000 {
			return 0, "", gerror.New("commission_rate_missing")
		}
		return multiplyDivide(base, int64(*rule.RateBasisPoints), 10000)
	case CommissionTiered:
		ordered := append([]entity.SalesCommissionTier(nil), tiers...)
		sort.SliceStable(ordered, func(i, j int) bool {
			if ordered[i].LowerBoundMinor == ordered[j].LowerBoundMinor {
				return ordered[i].TierNo < ordered[j].TierNo
			}
			return ordered[i].LowerBoundMinor < ordered[j].LowerBoundMinor
		})
		var previousUpper int64
		hasPrevious := false
		previousUnbounded := false
		var selected *entity.SalesCommissionTier
		for _, tier := range ordered {
			if tier.RuleNo != rule.RuleNo {
				continue
			}
			if tier.LowerBoundMinor < 0 || tier.RateBasisPoints < 0 || tier.RateBasisPoints > 10000 ||
				(tier.UpperBoundMinor != nil && *tier.UpperBoundMinor <= tier.LowerBoundMinor) {
				return 0, "", gerror.New("commission_tier_invalid")
			}
			if previousUnbounded || (hasPrevious && tier.LowerBoundMinor < previousUpper) {
				return 0, "", gerror.New("commission_tier_overlap")
			}
			if base >= tier.LowerBoundMinor && (tier.UpperBoundMinor == nil || base < *tier.UpperBoundMinor) {
				clonedTier := tier
				selected = &clonedTier
			}
			hasPrevious = true
			if tier.UpperBoundMinor == nil {
				previousUnbounded = true
			} else {
				previousUpper = *tier.UpperBoundMinor
			}
		}
		if selected != nil {
			amount, _, err := multiplyDivide(base, int64(selected.RateBasisPoints), 10000)
			return amount, selected.TierNo, err
		}
		return 0, "", gerror.New("commission_tier_unavailable")
	default:
		return 0, "", gerror.New("commission_calculation_invalid")
	}
}

func multiplyDivide(value, multiplier, divisor int64) (int64, string, error) {
	if value < 0 || multiplier < 0 || divisor <= 0 {
		return 0, "", gerror.New("commission_amount_invalid")
	}
	product := new(big.Int).Mul(big.NewInt(value), big.NewInt(multiplier))
	product.Quo(product, big.NewInt(divisor))
	if !product.IsInt64() {
		return 0, "", gerror.New("commission_amount_overflow")
	}
	return product.Int64(), "", nil
}
