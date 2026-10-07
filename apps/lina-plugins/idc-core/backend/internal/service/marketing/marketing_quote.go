package marketing

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
	"strings"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
)

const maximumPromotionQuoteLines = 100

type quoteEligibleCandidate struct {
	candidate   eligibleCandidate
	lineIndexes []int
}

// ReserveQuote evaluates all lines with one bounded source projection and
// reserves each selected campaign exactly once for the complete quote.
func (s *serviceImpl) ReserveQuote(
	ctx context.Context,
	input QuoteReserveInput,
) (*QuoteReservation, error) {
	normalized, err := normalizeQuoteReserveInput(input)
	if err != nil {
		return nil, err
	}
	requestDigest, err := promotionQuoteRequestDigest(normalized)
	if err != nil {
		return nil, bizerr.WrapCode(err, CodeUnavailable)
	}
	var result *QuoteReservation
	err = s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		source, loadErr := s.loadPromotionEvaluationSource(
			ctx, normalized.CustomerNo, normalized.CurrencyCode, normalized.AssetNumbers,
		)
		if loadErr != nil {
			return loadErr
		}
		evaluated, evaluateErr := buildQuotePromotionEvaluation(normalized, source)
		if evaluateErr != nil {
			return evaluateErr
		}
		existing, existingErr := s.loadReservation(
			ctx, normalized.CustomerNo, normalized.QuoteReference, false,
		)
		if existingErr != nil {
			return existingErr
		}
		if len(existing) > 0 {
			result, existingErr = projectExistingQuoteReservation(
				normalized, evaluated, existing, requestDigest,
			)
			return existingErr
		}
		if len(evaluated) == 0 {
			result = &QuoteReservation{
				CustomerNo: normalized.CustomerNo, QuoteReference: normalized.QuoteReference,
				CurrencyCode: normalized.CurrencyCode, Applied: []QuoteAppliedBenefit{},
			}
			return nil
		}
		applied := quoteAppliedBenefits(evaluated)
		quotas, lockErr := s.lockSelectedBenefits(ctx, applied)
		if lockErr != nil {
			return lockErr
		}
		existing, existingErr = s.loadReservation(
			ctx, normalized.CustomerNo, normalized.QuoteReference, false,
		)
		if existingErr != nil {
			return existingErr
		}
		if len(existing) > 0 {
			result, existingErr = projectExistingQuoteReservation(
				normalized, evaluated, existing, requestDigest,
			)
			return existingErr
		}
		lockedSource, loadErr := s.loadPromotionEvaluationSource(
			ctx, normalized.CustomerNo, normalized.CurrencyCode, normalized.AssetNumbers,
		)
		if loadErr != nil {
			return loadErr
		}
		lockedEvaluation, evaluateErr := buildQuotePromotionEvaluation(normalized, lockedSource)
		if evaluateErr != nil {
			return evaluateErr
		}
		if !sameQuoteAppliedBenefits(evaluated, lockedEvaluation) {
			return bizerr.NewCode(CodeConflict)
		}
		expiresAt := lockedSource.now.Add(s.reservationTTL)
		for index := range lockedEvaluation {
			reservationNo, reserveErr := s.reserveApplied(
				ctx,
				EvaluateInput{
					CustomerNo: normalized.CustomerNo, QuoteReference: normalized.QuoteReference,
					CurrencyCode: normalized.CurrencyCode,
				},
				requestDigest, expiresAt, lockedEvaluation[index].AppliedBenefit,
				quotas[lockedEvaluation[index].CampaignNo],
			)
			if reserveErr != nil {
				return reserveErr
			}
			lockedEvaluation[index].ReservationReference = reservationNo
		}
		result = &QuoteReservation{
			CustomerNo: normalized.CustomerNo, QuoteReference: normalized.QuoteReference,
			CurrencyCode: normalized.CurrencyCode, ExpiresAt: &expiresAt,
			Applied: lockedEvaluation,
		}
		return nil
	})
	if err != nil {
		if _, ok := bizerr.As(err); ok {
			return nil, err
		}
		return nil, bizerr.WrapCode(err, CodeUnavailable)
	}
	return result, nil
}

func normalizeQuoteReserveInput(input QuoteReserveInput) (QuoteReserveInput, error) {
	input.CustomerNo = strings.TrimSpace(input.CustomerNo)
	input.QuoteReference = strings.TrimSpace(input.QuoteReference)
	input.CurrencyCode = strings.TrimSpace(input.CurrencyCode)
	if !identifierPattern.MatchString(input.CustomerNo) ||
		!identifierPattern.MatchString(input.QuoteReference) || len(input.QuoteReference) < 8 ||
		!currencyPattern.MatchString(input.CurrencyCode) ||
		(input.PurchasePhase != PurchasePhaseNewPurchase && input.PurchasePhase != PurchasePhaseRenewal) ||
		input.CycleNumber < 1 || len(input.Lines) == 0 || len(input.Lines) > maximumPromotionQuoteLines ||
		len(input.AssetNumbers) > maximumPromotionAssets {
		return QuoteReserveInput{}, bizerr.NewCode(CodeInvalid)
	}
	seenLines := make(map[string]struct{}, len(input.Lines))
	for index := range input.Lines {
		line := &input.Lines[index]
		line.ReferenceNo = strings.TrimSpace(line.ReferenceNo)
		line.ProductNo = strings.TrimSpace(line.ProductNo)
		line.CategoryNo = strings.TrimSpace(line.CategoryNo)
		if !identifierPattern.MatchString(line.ReferenceNo) || !identifierPattern.MatchString(line.ProductNo) ||
			!identifierPattern.MatchString(line.CategoryNo) ||
			line.SubtotalMinor < 0 {
			return QuoteReserveInput{}, bizerr.NewCode(CodeInvalid)
		}
		if _, duplicate := seenLines[line.ReferenceNo]; duplicate {
			return QuoteReserveInput{}, bizerr.NewCode(CodeInvalid)
		}
		seenLines[line.ReferenceNo] = struct{}{}
	}
	seenAssets := make(map[string]struct{}, len(input.AssetNumbers))
	for index := range input.AssetNumbers {
		input.AssetNumbers[index] = strings.TrimSpace(input.AssetNumbers[index])
		if !identifierPattern.MatchString(input.AssetNumbers[index]) {
			return QuoteReserveInput{}, bizerr.NewCode(CodeInvalid)
		}
		if _, duplicate := seenAssets[input.AssetNumbers[index]]; duplicate {
			return QuoteReserveInput{}, bizerr.NewCode(CodeInvalid)
		}
		seenAssets[input.AssetNumbers[index]] = struct{}{}
	}
	sort.Strings(input.AssetNumbers)
	return input, nil
}

func buildQuotePromotionEvaluation(
	input QuoteReserveInput,
	source *promotionEvaluationSource,
) ([]QuoteAppliedBenefit, error) {
	eligible, err := eligibleQuotePromotionCandidates(input, source)
	if err != nil {
		return nil, err
	}
	return applyQuotePromotionCandidates(input, eligible)
}

func eligibleQuotePromotionCandidates(
	input QuoteReserveInput,
	source *promotionEvaluationSource,
) ([]quoteEligibleCandidate, error) {
	if source == nil {
		return nil, gerror.New("promotion quote source is nil")
	}
	scopesByCampaign := make(map[string][]scopeRow)
	for _, scope := range source.scopes {
		scopesByCampaign[scope.CampaignNo] = append(scopesByCampaign[scope.CampaignNo], scope)
	}
	membershipSet := make(map[string]struct{}, len(source.memberships))
	for _, membership := range source.memberships {
		membershipSet[membership.SnapshotNo] = struct{}{}
	}
	requestedAssets := make(map[string]struct{}, len(input.AssetNumbers))
	for _, assetNo := range input.AssetNumbers {
		requestedAssets[assetNo] = struct{}{}
	}
	assetsByBenefit := make(map[string]assetRow)
	for _, asset := range source.assets {
		if _, requested := requestedAssets[asset.AssetNo]; !requested {
			continue
		}
		key := asset.CampaignNo + "\x00" + asset.BenefitNo
		if previous, found := assetsByBenefit[key]; !found || asset.AssetNo < previous.AssetNo {
			assetsByBenefit[key] = asset
		}
	}
	eligible := make([]quoteEligibleCandidate, 0, len(source.candidates))
	seenCampaign := make(map[string]struct{}, len(source.candidates))
	for _, row := range source.candidates {
		if _, seen := seenCampaign[row.CampaignNo]; seen || !campaignTargetMatches(row, membershipSet) ||
			!quoteBenefitType(row.BenefitType) {
			continue
		}
		lineIndexes := make([]int, 0, len(input.Lines))
		eligibleSubtotal := int64(0)
		for lineIndex, line := range input.Lines {
			lineInput := EvaluateInput{
				CustomerNo: input.CustomerNo, QuoteReference: input.QuoteReference,
				CurrencyCode: input.CurrencyCode, SubtotalMinor: line.SubtotalMinor,
				PurchasePhase: input.PurchasePhase, CycleNumber: input.CycleNumber,
				ProductNo: line.ProductNo, CategoryNo: line.CategoryNo,
			}
			if campaignScopeMatches(scopesByCampaign[row.CampaignNo], lineInput) {
				lineIndexes = append(lineIndexes, lineIndex)
				var err error
				eligibleSubtotal, err = safePromotionAmountAdd(eligibleSubtotal, line.SubtotalMinor)
				if err != nil {
					return nil, err
				}
			}
		}
		cycleInput := EvaluateInput{
			CurrencyCode: input.CurrencyCode, SubtotalMinor: eligibleSubtotal,
			PurchasePhase: input.PurchasePhase, CycleNumber: input.CycleNumber,
		}
		if len(lineIndexes) == 0 || !candidateCycleMatches(row, cycleInput) {
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
			candidate.assetNo, candidate.amount, candidate.rate = asset.AssetNo, asset.FaceAmountMinor, asset.RateBasisPoints
		}
		seenCampaign[row.CampaignNo] = struct{}{}
		eligible = append(eligible, quoteEligibleCandidate{candidate: candidate, lineIndexes: lineIndexes})
	}
	sort.Slice(eligible, func(left, right int) bool {
		if eligible[left].candidate.row.Priority != eligible[right].candidate.row.Priority {
			return eligible[left].candidate.row.Priority < eligible[right].candidate.row.Priority
		}
		if eligible[left].candidate.row.CampaignNo != eligible[right].candidate.row.CampaignNo {
			return eligible[left].candidate.row.CampaignNo < eligible[right].candidate.row.CampaignNo
		}
		return eligible[left].candidate.row.BenefitNo < eligible[right].candidate.row.BenefitNo
	})
	return eligible, nil
}

func applyQuotePromotionCandidates(
	input QuoteReserveInput,
	eligible []quoteEligibleCandidate,
) ([]QuoteAppliedBenefit, error) {
	remaining := make([]int64, len(input.Lines))
	for index, line := range input.Lines {
		remaining[index] = line.SubtotalMinor
	}
	selectedAny, exclusiveSelected := false, false
	selectedGroups := make(map[string]struct{})
	result := make([]QuoteAppliedBenefit, 0)
	for _, item := range eligible {
		candidate := item.candidate
		var skip bool
		switch candidate.row.StackPolicy {
		case "exclusive":
			skip = selectedAny
			if !skip {
				exclusiveSelected = true
			}
		case "same_group_exclusive":
			_, groupSelected := selectedGroups[candidate.row.StackGroup]
			skip = exclusiveSelected || groupSelected
		case "stackable":
			skip = exclusiveSelected
		default:
			return nil, gerror.New("persisted promotion stack policy is invalid")
		}
		if skip {
			continue
		}
		eligibleRemaining := int64(0)
		for _, lineIndex := range item.lineIndexes {
			var err error
			eligibleRemaining, err = safePromotionAmountAdd(eligibleRemaining, remaining[lineIndex])
			if err != nil {
				return nil, err
			}
		}
		discount, err := candidateDiscount(candidate, eligibleRemaining)
		if err != nil {
			return nil, err
		}
		if discount == 0 {
			continue
		}
		applied := QuoteAppliedBenefit{AppliedBenefit: AppliedBenefit{
			CampaignNo: candidate.row.CampaignNo, BenefitNo: candidate.row.BenefitNo,
			AssetNo: candidate.assetNo, BenefitType: candidate.row.BenefitType,
			StackPolicy: candidate.row.StackPolicy, StackGroup: candidate.row.StackGroup,
			Priority: candidate.row.Priority, DiscountAmountMinor: discount,
			BenefitSnapshotDigest: candidate.digest,
		}, Allocations: make([]QuoteBenefitAllocation, 0, len(item.lineIndexes))}
		unallocated := discount
		for _, lineIndex := range item.lineIndexes {
			allocated := remaining[lineIndex]
			if allocated > unallocated {
				allocated = unallocated
			}
			if allocated > 0 {
				remaining[lineIndex] -= allocated
				unallocated -= allocated
				applied.Allocations = append(applied.Allocations, QuoteBenefitAllocation{
					LineReference: input.Lines[lineIndex].ReferenceNo, AmountMinor: allocated,
				})
			}
			if unallocated == 0 {
				break
			}
		}
		if unallocated != 0 {
			return nil, gerror.New("promotion quote allocation is inconsistent")
		}
		selectedAny = true
		if candidate.row.StackPolicy == "same_group_exclusive" {
			selectedGroups[candidate.row.StackGroup] = struct{}{}
		}
		result = append(result, applied)
	}
	return result, nil
}

func quoteAppliedBenefits(values []QuoteAppliedBenefit) []AppliedBenefit {
	result := make([]AppliedBenefit, len(values))
	for index := range values {
		result[index] = values[index].AppliedBenefit
	}
	return result
}

func sameQuoteAppliedBenefits(left, right []QuoteAppliedBenefit) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index].AppliedBenefit != right[index].AppliedBenefit ||
			len(left[index].Allocations) != len(right[index].Allocations) {
			return false
		}
		for allocationIndex := range left[index].Allocations {
			if left[index].Allocations[allocationIndex] != right[index].Allocations[allocationIndex] {
				return false
			}
		}
	}
	return true
}

func projectExistingQuoteReservation(
	input QuoteReserveInput,
	evaluated []QuoteAppliedBenefit,
	rows []reservationRow,
	requestDigest string,
) (*QuoteReservation, error) {
	if len(rows) != len(evaluated) || len(rows) == 0 {
		return nil, bizerr.NewCode(CodeConflict)
	}
	rowIndex := make(map[string]reservationRow, len(rows))
	for _, row := range rows {
		if row.RequestSnapshotDigest != requestDigest || row.StatusCode != ReservationStatusActive || row.ExpiresAt == nil {
			return nil, bizerr.NewCode(CodeConflict)
		}
		rowIndex[row.CampaignNo+"\x00"+row.BenefitNo] = row
	}
	var expiresAt *time.Time
	result := append([]QuoteAppliedBenefit(nil), evaluated...)
	for index := range result {
		row, found := rowIndex[result[index].CampaignNo+"\x00"+result[index].BenefitNo]
		if !found || row.QuotedValueMinor != result[index].DiscountAmountMinor ||
			row.BenefitSnapshotDigest != result[index].BenefitSnapshotDigest {
			return nil, bizerr.NewCode(CodeConflict)
		}
		result[index].ReservationReference = row.ReservationNo
		if expiresAt == nil {
			value := *row.ExpiresAt
			expiresAt = &value
		} else if !expiresAt.Equal(*row.ExpiresAt) {
			return nil, bizerr.NewCode(CodeConflict)
		}
	}
	return &QuoteReservation{
		CustomerNo: input.CustomerNo, QuoteReference: input.QuoteReference,
		CurrencyCode: input.CurrencyCode, ExpiresAt: expiresAt, Applied: result,
	}, nil
}

func promotionQuoteRequestDigest(input QuoteReserveInput) (string, error) {
	encoded, err := json.Marshal(input)
	if err != nil {
		return "", gerror.Wrap(err, "encode promotion quote request")
	}
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:]), nil
}

func safePromotionAmountAdd(left, right int64) (int64, error) {
	if left < 0 || right < 0 || left > int64(^uint64(0)>>1)-right {
		return 0, gerror.New("promotion quote amount overflow")
	}
	return left + right, nil
}
