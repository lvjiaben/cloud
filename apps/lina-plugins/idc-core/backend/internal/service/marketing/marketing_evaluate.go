// This file evaluates promotion scope, cycle, currency, assets, and stacking locally.

package marketing

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"math/big"
	"sort"
	"strings"
	"time"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
)

type candidateRow struct {
	CampaignNo           string  `orm:"campaign_no"`
	CampaignType         string  `orm:"campaign_type_code"`
	TargetSegmentNo      *string `orm:"target_segment_no"`
	TargetSnapshotNo     *string `orm:"target_snapshot_no"`
	Priority             int     `orm:"priority"`
	StackPolicy          string  `orm:"stack_policy_code"`
	StackGroup           string  `orm:"stack_group_code"`
	BenefitNo            string  `orm:"benefit_no"`
	BenefitType          string  `orm:"benefit_type_code"`
	CurrencyCode         *string `orm:"currency_code"`
	AmountMinor          *int64  `orm:"amount_minor"`
	RateBasisPoints      *int    `orm:"rate_basis_points"`
	MaximumDiscountMinor *int64  `orm:"maximum_discount_minor"`
	MinimumSpendMinor    *int64  `orm:"minimum_spend_minor"`
	PurchasePhase        string  `orm:"purchase_phase_code"`
	CycleScope           string  `orm:"cycle_scope_code"`
	CycleCount           *int    `orm:"cycle_count"`
	CampaignVersion      int64   `orm:"campaign_version"`
	BenefitVersion       int64   `orm:"benefit_version"`
}

type scopeRow struct {
	CampaignNo    string `orm:"campaign_no"`
	ScopeKind     string `orm:"scope_kind_code"`
	MatchMode     string `orm:"match_mode_code"`
	ReferenceCode string `orm:"reference_code"`
}

type snapshotMemberRow struct {
	SnapshotNo string `orm:"snapshot_no"`
}

type assetRow struct {
	AssetNo               string     `orm:"asset_no"`
	CampaignNo            string     `orm:"campaign_no"`
	BenefitNo             string     `orm:"benefit_no"`
	BenefitType           string     `orm:"benefit_type_code"`
	BenefitSnapshotDigest string     `orm:"benefit_snapshot_digest"`
	CurrencyCode          string     `orm:"currency_code"`
	FaceAmountMinor       *int64     `orm:"face_amount_minor"`
	RateBasisPoints       *int       `orm:"rate_basis_points"`
	ValidFrom             *time.Time `orm:"valid_from"`
	ValidUntil            *time.Time `orm:"valid_until"`
}

type eligibleCandidate struct {
	row     candidateRow
	assetNo string
	amount  *int64
	rate    *int
	digest  string
}

type promotionEvaluationSource struct {
	now         time.Time
	candidates  []candidateRow
	scopes      []scopeRow
	memberships []snapshotMemberRow
	assets      []assetRow
}

func (s *serviceImpl) Evaluate(ctx context.Context, input EvaluateInput) (*Evaluation, error) {
	normalized, err := normalizeEvaluateInput(input)
	if err != nil {
		return nil, err
	}
	result, err := s.evaluate(ctx, normalized)
	if err != nil {
		if _, ok := bizerr.As(err); ok {
			return nil, err
		}
		return nil, bizerr.WrapCode(err, CodeUnavailable)
	}
	return result, nil
}

func (s *serviceImpl) evaluate(ctx context.Context, input EvaluateInput) (*Evaluation, error) {
	source, err := s.loadPromotionEvaluationSource(
		ctx, input.CustomerNo, input.CurrencyCode, input.AssetNumbers,
	)
	if err != nil {
		return nil, err
	}
	return buildEvaluation(input, source.candidates, source.scopes, source.memberships, source.assets)
}

func (s *serviceImpl) loadPromotionEvaluationSource(
	ctx context.Context,
	customerNo string,
	currencyCode string,
	assetNumbers []string,
) (*promotionEvaluationSource, error) {
	nowValue, err := s.database.GetValue(ctx, `SELECT CURRENT_TIMESTAMP`)
	if err != nil {
		return nil, gerror.Wrap(err, "read marketing database time")
	}
	now := nowValue.Time()
	var candidates []candidateRow
	if err = s.database.GetScan(ctx, &candidates, promotionCandidatesSQL,
		now, now, currencyCode, customerNo, maximumPromotionCandidates+1,
	); err != nil {
		return nil, gerror.Wrap(err, "load promotion candidates")
	}
	if len(candidates) > maximumPromotionCandidates {
		return nil, gerror.New("promotion candidate bound exceeded")
	}
	var scopes []scopeRow
	if err = s.database.GetScan(ctx, &scopes, promotionScopesSQL,
		now, now, maximumPromotionScopes+1,
	); err != nil {
		return nil, gerror.Wrap(err, "load promotion scopes")
	}
	if len(scopes) > maximumPromotionScopes {
		return nil, gerror.New("promotion scope bound exceeded")
	}
	var memberships []snapshotMemberRow
	if err = s.database.GetScan(ctx, &memberships, promotionSnapshotMembershipSQL,
		customerNo, now, now, maximumPromotionCandidates+1,
	); err != nil {
		return nil, gerror.Wrap(err, "load promotion target membership")
	}
	var assets []assetRow
	if len(assetNumbers) > 0 {
		if err = s.database.GetScan(ctx, &assets, promotionAssetsSQL,
			customerNo, currencyCode, strings.Join(assetNumbers, ","),
			now, now, maximumPromotionAssets+1,
		); err != nil {
			return nil, gerror.Wrap(err, "load customer promotion assets")
		}
		if len(assets) > maximumPromotionAssets {
			return nil, gerror.New("customer promotion asset bound exceeded")
		}
	}
	return &promotionEvaluationSource{
		now: now, candidates: candidates, scopes: scopes,
		memberships: memberships, assets: assets,
	}, nil
}

func normalizeEvaluateInput(input EvaluateInput) (EvaluateInput, error) {
	input.CustomerNo = strings.TrimSpace(input.CustomerNo)
	input.QuoteReference = strings.TrimSpace(input.QuoteReference)
	input.CurrencyCode = strings.TrimSpace(input.CurrencyCode)
	input.ProductNo = strings.TrimSpace(input.ProductNo)
	input.CategoryNo = strings.TrimSpace(input.CategoryNo)
	if !identifierPattern.MatchString(input.CustomerNo) ||
		!identifierPattern.MatchString(input.QuoteReference) || len(input.QuoteReference) < 8 ||
		!currencyPattern.MatchString(input.CurrencyCode) || input.SubtotalMinor < 0 ||
		(input.PurchasePhase != PurchasePhaseNewPurchase && input.PurchasePhase != PurchasePhaseRenewal) ||
		input.CycleNumber < 1 || !identifierPattern.MatchString(input.ProductNo) ||
		!identifierPattern.MatchString(input.CategoryNo) ||
		len(input.AssetNumbers) > maximumPromotionAssets {
		return EvaluateInput{}, bizerr.NewCode(CodeInvalid)
	}
	seen := make(map[string]struct{}, len(input.AssetNumbers))
	for index := range input.AssetNumbers {
		input.AssetNumbers[index] = strings.TrimSpace(input.AssetNumbers[index])
		if !identifierPattern.MatchString(input.AssetNumbers[index]) {
			return EvaluateInput{}, bizerr.NewCode(CodeInvalid)
		}
		if _, found := seen[input.AssetNumbers[index]]; found {
			return EvaluateInput{}, bizerr.NewCode(CodeInvalid)
		}
		seen[input.AssetNumbers[index]] = struct{}{}
	}
	sort.Strings(input.AssetNumbers)
	return input, nil
}

func buildEvaluation(
	input EvaluateInput,
	candidates []candidateRow,
	scopes []scopeRow,
	memberships []snapshotMemberRow,
	assets []assetRow,
) (*Evaluation, error) {
	scopesByCampaign := make(map[string][]scopeRow)
	for _, scope := range scopes {
		scopesByCampaign[scope.CampaignNo] = append(scopesByCampaign[scope.CampaignNo], scope)
	}
	membershipSet := make(map[string]struct{}, len(memberships))
	for _, membership := range memberships {
		membershipSet[membership.SnapshotNo] = struct{}{}
	}
	requestedAssets := make(map[string]struct{}, len(input.AssetNumbers))
	for _, assetNo := range input.AssetNumbers {
		requestedAssets[assetNo] = struct{}{}
	}
	assetsByBenefit := make(map[string]assetRow)
	for _, asset := range assets {
		if _, requested := requestedAssets[asset.AssetNo]; !requested {
			continue
		}
		key := asset.CampaignNo + "\x00" + asset.BenefitNo
		if previous, found := assetsByBenefit[key]; !found || asset.AssetNo < previous.AssetNo {
			assetsByBenefit[key] = asset
		}
	}
	eligible := make([]eligibleCandidate, 0, len(candidates))
	seenCampaign := make(map[string]struct{}, len(candidates))
	for _, row := range candidates {
		if _, seen := seenCampaign[row.CampaignNo]; seen {
			continue
		}
		if !candidateCycleMatches(row, input) || !campaignScopeMatches(scopesByCampaign[row.CampaignNo], input) ||
			!campaignTargetMatches(row, membershipSet) {
			continue
		}
		digest, err := benefitDigest(row)
		if err != nil {
			return nil, err
		}
		candidate := eligibleCandidate{row: row, amount: row.AmountMinor, rate: row.RateBasisPoints, digest: digest}
		if row.CampaignType == "coupon" || row.CampaignType == "voucher" {
			asset, found := assetsByBenefit[row.CampaignNo+"\x00"+row.BenefitNo]
			if !found || asset.BenefitSnapshotDigest != digest || asset.BenefitType != row.BenefitType {
				continue
			}
			candidate.assetNo = asset.AssetNo
			candidate.amount = asset.FaceAmountMinor
			candidate.rate = asset.RateBasisPoints
		}
		if !quoteBenefitType(row.BenefitType) {
			continue
		}
		seenCampaign[row.CampaignNo] = struct{}{}
		eligible = append(eligible, candidate)
	}
	sort.Slice(eligible, func(left, right int) bool {
		if eligible[left].row.Priority != eligible[right].row.Priority {
			return eligible[left].row.Priority < eligible[right].row.Priority
		}
		if eligible[left].row.CampaignNo != eligible[right].row.CampaignNo {
			return eligible[left].row.CampaignNo < eligible[right].row.CampaignNo
		}
		return eligible[left].row.BenefitNo < eligible[right].row.BenefitNo
	})
	result := &Evaluation{CurrencyCode: input.CurrencyCode, SubtotalMinor: input.SubtotalMinor}
	remaining := input.SubtotalMinor
	selectedAny := false
	exclusiveSelected := false
	selectedGroups := make(map[string]struct{})
	for _, candidate := range eligible {
		reason := ""
		switch candidate.row.StackPolicy {
		case "exclusive":
			if selectedAny {
				reason = "higher_priority_conflict"
			} else {
				exclusiveSelected = true
			}
		case "same_group_exclusive":
			if exclusiveSelected {
				reason = "exclusive_conflict"
			} else if _, found := selectedGroups[candidate.row.StackGroup]; found {
				reason = "stack_group_conflict"
			}
		case "stackable":
			if exclusiveSelected {
				reason = "exclusive_conflict"
			}
		default:
			return nil, gerror.New("persisted promotion stack policy is invalid")
		}
		if reason != "" {
			result.Excluded = append(result.Excluded, ExcludedBenefit{
				CampaignNo: candidate.row.CampaignNo, BenefitNo: candidate.row.BenefitNo, ReasonCode: reason,
			})
			continue
		}
		discount, err := candidateDiscount(candidate, remaining)
		if err != nil {
			return nil, err
		}
		if discount == 0 {
			result.Excluded = append(result.Excluded, ExcludedBenefit{
				CampaignNo: candidate.row.CampaignNo, BenefitNo: candidate.row.BenefitNo, ReasonCode: "no_discount_value",
			})
			continue
		}
		result.Applied = append(result.Applied, AppliedBenefit{
			CampaignNo: candidate.row.CampaignNo, BenefitNo: candidate.row.BenefitNo,
			AssetNo: candidate.assetNo, BenefitType: candidate.row.BenefitType,
			StackPolicy: candidate.row.StackPolicy, StackGroup: candidate.row.StackGroup,
			Priority: candidate.row.Priority, DiscountAmountMinor: discount,
			BenefitSnapshotDigest: candidate.digest,
		})
		selectedAny = true
		if candidate.row.StackPolicy == "same_group_exclusive" {
			selectedGroups[candidate.row.StackGroup] = struct{}{}
		}
		remaining -= discount
	}
	result.PayableMinor = remaining
	result.DiscountTotalMinor = input.SubtotalMinor - remaining
	return result, nil
}

func candidateCycleMatches(row candidateRow, input EvaluateInput) bool {
	if row.CurrencyCode == nil || *row.CurrencyCode != input.CurrencyCode ||
		(row.MinimumSpendMinor != nil && input.SubtotalMinor < *row.MinimumSpendMinor) ||
		(row.PurchasePhase != "both" && row.PurchasePhase != input.PurchasePhase) {
		return false
	}
	switch row.CycleScope {
	case "first":
		return input.CycleNumber == 1
	case "all":
		return true
	case "limited":
		return row.CycleCount != nil && input.CycleNumber <= *row.CycleCount
	default:
		return false
	}
}

func campaignTargetMatches(row candidateRow, memberships map[string]struct{}) bool {
	if row.TargetSegmentNo == nil {
		return row.TargetSnapshotNo == nil
	}
	if row.TargetSnapshotNo == nil {
		return false
	}
	_, found := memberships[*row.TargetSnapshotNo]
	return found
}

func campaignScopeMatches(scopes []scopeRow, input EvaluateInput) bool {
	if len(scopes) == 0 {
		return false
	}
	included := false
	hasInclude := false
	for _, scope := range scopes {
		matched := scope.ScopeKind == "all_products" ||
			(scope.ScopeKind == "product" && scope.ReferenceCode == input.ProductNo) ||
			(scope.ScopeKind == "category" && scope.ReferenceCode == input.CategoryNo)
		if scope.MatchMode == "exclude" && matched {
			return false
		}
		if scope.MatchMode == "include" {
			hasInclude = true
			included = included || matched
		}
	}
	return hasInclude && included
}

func quoteBenefitType(value string) bool {
	return value == "fixed_discount" || value == "percentage_discount" ||
		value == "voucher" || value == "customer_price"
}

func candidateDiscount(candidate eligibleCandidate, remaining int64) (int64, error) {
	if remaining <= 0 {
		return 0, nil
	}
	var discount int64
	switch candidate.row.BenefitType {
	case "fixed_discount", "voucher":
		if candidate.amount == nil || *candidate.amount <= 0 {
			return 0, gerror.New("persisted fixed promotion amount is invalid")
		}
		discount = *candidate.amount
	case "customer_price":
		if candidate.amount == nil || *candidate.amount <= 0 || *candidate.amount >= remaining {
			return 0, nil
		}
		discount = remaining - *candidate.amount
	case "percentage_discount":
		if candidate.rate == nil || *candidate.rate <= 0 || *candidate.rate > 10000 {
			return 0, gerror.New("persisted percentage promotion rate is invalid")
		}
		numerator := new(big.Int).Mul(big.NewInt(remaining), big.NewInt(int64(*candidate.rate)))
		numerator.Add(numerator, big.NewInt(5000))
		numerator.Quo(numerator, big.NewInt(10000))
		if !numerator.IsInt64() {
			return 0, gerror.New("promotion amount overflow")
		}
		discount = numerator.Int64()
		if candidate.row.MaximumDiscountMinor != nil && discount > *candidate.row.MaximumDiscountMinor {
			discount = *candidate.row.MaximumDiscountMinor
		}
	default:
		return 0, gerror.New("persisted quote promotion type is invalid")
	}
	if discount > remaining {
		discount = remaining
	}
	return discount, nil
}

func benefitDigest(row candidateRow) (string, error) {
	snapshot := struct {
		CampaignNo, CampaignType, BenefitNo, BenefitType, PurchasePhase, CycleScope string
		CampaignVersion, BenefitVersion                                             int64
		CurrencyCode                                                                *string
		AmountMinor, MaximumDiscountMinor, MinimumSpendMinor                        *int64
		RateBasisPoints, CycleCount                                                 *int
	}{
		CampaignNo: row.CampaignNo, CampaignType: row.CampaignType,
		BenefitNo: row.BenefitNo, BenefitType: row.BenefitType,
		PurchasePhase: row.PurchasePhase, CycleScope: row.CycleScope,
		CampaignVersion: row.CampaignVersion, BenefitVersion: row.BenefitVersion,
		CurrencyCode: row.CurrencyCode, AmountMinor: row.AmountMinor,
		MaximumDiscountMinor: row.MaximumDiscountMinor, MinimumSpendMinor: row.MinimumSpendMinor,
		RateBasisPoints: row.RateBasisPoints, CycleCount: row.CycleCount,
	}
	encoded, err := json.Marshal(snapshot)
	if err != nil {
		return "", gerror.Wrap(err, "encode promotion benefit snapshot")
	}
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:]), nil
}

const promotionCandidatesSQL = `
SELECT c.campaign_no, c.campaign_type_code, c.target_segment_no, c.target_snapshot_no,
       c.priority, c.stack_policy_code, c.stack_group_code, c.version AS campaign_version,
       b.benefit_no, b.benefit_type_code, b.currency_code, b.amount_minor,
       b.rate_basis_points, b.maximum_discount_minor, b.minimum_spend_minor,
       b.purchase_phase_code, b.cycle_scope_code, b.cycle_count, b.version AS benefit_version
FROM plugin_idc_core_marketing_campaign c
JOIN plugin_idc_core_marketing_benefit_rule b
  ON b.tenant_id = c.tenant_id AND b.campaign_no = c.campaign_no
WHERE c.tenant_id = 0 AND c.status_code = 'active'
  AND c.starts_at <= ? AND c.ends_at > ?
  AND b.status_code = 'active' AND b.currency_code = ?
  AND NOT EXISTS (
    SELECT 1 FROM plugin_idc_core_marketing_campaign_restriction restriction
    WHERE restriction.tenant_id = c.tenant_id AND restriction.campaign_no = c.campaign_no
      AND restriction.customer_no = ? AND restriction.status_code = 'active'
  )
ORDER BY c.priority ASC, c.campaign_no ASC, b.benefit_no ASC
LIMIT ?`

const promotionScopesSQL = `
SELECT s.campaign_no, s.scope_kind_code, s.match_mode_code, s.reference_code
FROM plugin_idc_core_marketing_campaign_scope s
JOIN plugin_idc_core_marketing_campaign c
  ON c.tenant_id = s.tenant_id AND c.campaign_no = s.campaign_no
WHERE s.tenant_id = 0 AND c.status_code = 'active'
  AND c.starts_at <= ? AND c.ends_at > ?
ORDER BY s.campaign_no ASC, s.id ASC
LIMIT ?`

const promotionSnapshotMembershipSQL = `
SELECT DISTINCT sm.snapshot_no
FROM plugin_idc_core_marketing_snapshot_member sm
JOIN plugin_idc_core_marketing_segment_snapshot ss
  ON ss.snapshot_no = sm.snapshot_no
JOIN plugin_idc_core_marketing_campaign c
  ON c.target_snapshot_no = sm.snapshot_no
WHERE sm.tenant_id = 0 AND sm.customer_no = ? AND ss.status_code = 'active'
  AND c.status_code = 'active' AND c.starts_at <= ? AND c.ends_at > ?
ORDER BY sm.snapshot_no ASC
LIMIT ?`

const promotionAssetsSQL = `
SELECT asset_no, campaign_no, benefit_no, benefit_type_code, benefit_snapshot_digest,
       currency_code, face_amount_minor, rate_basis_points, valid_from, valid_until
FROM plugin_idc_core_marketing_promotion_asset
WHERE tenant_id = 0 AND customer_no = ? AND currency_code = ? AND status_code = 'active'
  AND asset_no = ANY(string_to_array(?, ','))
  AND valid_from <= ? AND valid_until > ?
ORDER BY asset_no ASC
LIMIT ?`
