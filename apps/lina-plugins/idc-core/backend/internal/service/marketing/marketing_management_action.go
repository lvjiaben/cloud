// This file owns audited staff issuance, revocation, and lineage-preserving points corrections.

package marketing

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/internal/service/billing"
)

const maximumManagedPoints int64 = 1_000_000_000_000

// ManagementActionAuthorizer keeps each staff mutation behind its own exact permission.
type ManagementActionAuthorizer interface {
	AuthorizeAssetIssue(ctx context.Context) (actorUserID int, err error)
	AuthorizeCouponCodeGenerate(ctx context.Context) (actorUserID int, err error)
	AuthorizeAssetRevoke(ctx context.Context) (actorUserID int, err error)
	AuthorizePointsCredit(ctx context.Context) (actorUserID int, err error)
	AuthorizePointsReverse(ctx context.Context) (actorUserID int, err error)
	AuthorizeGrantPost(ctx context.Context) (actorUserID int, err error)
	AuthorizeGrantReverse(ctx context.Context) (actorUserID int, err error)
	AuthorizeGrantReview(ctx context.Context) (actorUserID int, err error)
}

// AssetIssueInput issues one single-use asset from an active campaign rule.
type AssetIssueInput struct {
	RequestNo  string
	CampaignNo string
	BenefitNo  string
	CustomerNo string
	ReasonCode string
}

// AssetRevokeInput revokes an unreserved asset with optimistic concurrency.
type AssetRevokeInput struct {
	RequestNo         string
	AssetNo           string
	ExpectedUpdatedAt time.Time
	ReasonCode        string
}

// PointsCreditInput adds non-monetary points through one idempotent manual lot.
type PointsCreditInput struct {
	RequestNo  string
	CustomerNo string
	Points     int64
	ExpiresAt  *time.Time
	ReasonCode string
}

// PointsReverseManagementInput reverses one exact source award without arbitrary balance subtraction.
type PointsReverseManagementInput struct {
	RequestNo            string
	CustomerNo           string
	AwardSourceType      string
	AwardSourceReference string
	Points               int64
	ReasonCode           string
}

// ManagedAssetMutation is the bounded result of one staff asset action.
type ManagedAssetMutation struct {
	AssetNo    string
	CustomerNo string
	Status     string
	UpdatedAt  time.Time
	Reused     bool
}

// ManagedPointsMutation is the bounded result of one staff points action.
type ManagedPointsMutation struct {
	LotNo           string
	CustomerNo      string
	AvailablePoints int64
	FrozenPoints    int64
	DebtPoints      int64
	Reused          bool
}

// ManagementActionService exposes audited local marketing actions and reviewed
// financial coordination. It never provides arbitrary point or cash subtraction.
type ManagementActionService interface {
	IssueAsset(ctx context.Context, input AssetIssueInput) (*ManagedAssetMutation, error)
	RevokeAsset(ctx context.Context, input AssetRevokeInput) (*ManagedAssetMutation, error)
	CreditPoints(ctx context.Context, input PointsCreditInput) (*ManagedPointsMutation, error)
	ReversePointsAward(ctx context.Context, input PointsReverseManagementInput) (*ManagedPointsMutation, error)
	RequestValueGrantPosting(ctx context.Context, input ValueGrantFinancialRequestInput) (*ManagedValueGrantFinancialMutation, error)
	RequestValueGrantReversal(ctx context.Context, input ValueGrantFinancialRequestInput) (*ManagedValueGrantFinancialMutation, error)
	ReviewValueGrantAdjustment(ctx context.Context, input ValueGrantFinancialReviewInput) (*ManagedValueGrantFinancialMutation, error)
	ListValueGrantAdjustments(ctx context.Context, input ValueGrantFinancialAdjustmentSearchInput) (*ManagedValueGrantFinancialAdjustmentPage, error)
}

type managementActionServiceImpl struct {
	database    gdb.DB
	audit       idccap.BusinessAuditWriter
	authorizer  ManagementActionAuthorizer
	points      PointsService
	valueGrants ValueGrantService
	financial   billing.FinancialAdjustmentService
}

var _ ManagementActionService = (*managementActionServiceImpl)(nil)

// NewManagementActionService creates the local audited marketing action service.
func NewManagementActionService(
	database gdb.DB,
	auditWriter idccap.BusinessAuditWriter,
	authorizer ManagementActionAuthorizer,
	points PointsService,
	valueGrants ValueGrantService,
	financial billing.FinancialAdjustmentService,
) (ManagementActionService, error) {
	if isNilMarketingManagementDependency(database) || isNilMarketingManagementDependency(auditWriter) ||
		isNilMarketingManagementDependency(authorizer) || isNilMarketingManagementDependency(points) ||
		isNilMarketingManagementDependency(valueGrants) || isNilMarketingManagementDependency(financial) {
		return nil, gerror.New("marketing management actions require database, audit, authorization, points, value-grant, and financial services")
	}
	return newManagementActionService(database, auditWriter, authorizer, points, valueGrants, financial)
}

func newManagementActionService(
	database gdb.DB,
	auditWriter idccap.BusinessAuditWriter,
	authorizer ManagementActionAuthorizer,
	points PointsService,
	valueGrants ValueGrantService,
	financial billing.FinancialAdjustmentService,
) (*managementActionServiceImpl, error) {
	if isNilMarketingManagementDependency(database) || isNilMarketingManagementDependency(auditWriter) ||
		isNilMarketingManagementDependency(authorizer) || isNilMarketingManagementDependency(points) ||
		isNilMarketingManagementDependency(valueGrants) || isNilMarketingManagementDependency(financial) {
		return nil, gerror.New("marketing management actions require database, audit, authorization, points, value-grant, and financial services")
	}
	return &managementActionServiceImpl{
		database: database, audit: auditWriter, authorizer: authorizer, points: points,
		valueGrants: valueGrants, financial: financial,
	}, nil
}

func (s *managementActionServiceImpl) IssueAsset(
	ctx context.Context,
	input AssetIssueInput,
) (*ManagedAssetMutation, error) {
	actorUserID, err := s.authorizer.AuthorizeAssetIssue(ctx)
	if err != nil {
		return nil, err
	}
	normalized, err := normalizeAssetIssue(input)
	if err != nil || actorUserID <= 0 {
		return nil, bizerr.NewCode(CodeManagementActionInvalid)
	}
	assetNo := deterministicManagedAssetNo(normalized.RequestNo)
	var result *ManagedAssetMutation
	blockedByNewcomerRule := false
	err = s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		existing, loadErr := loadManagedAsset(ctx, s.database, assetNo, true)
		if loadErr != nil {
			return loadErr
		}
		if existing != nil {
			if existing.CampaignNo != normalized.CampaignNo || existing.BenefitNo != normalized.BenefitNo ||
				existing.CustomerNo != normalized.CustomerNo {
				return bizerr.NewCode(CodeManagementActionConflict)
			}
			result = &ManagedAssetMutation{
				AssetNo: existing.AssetNo, CustomerNo: existing.CustomerNo, Status: existing.Status,
				UpdatedAt: existing.UpdatedAt, Reused: true,
			}
			return s.appendAssetIssueAudit(ctx, normalized, result)
		}
		rule, ruleErr := loadManagedAssetRule(ctx, s.database, normalized)
		if ruleErr != nil {
			return ruleErr
		}
		if rule == nil {
			return bizerr.NewCode(CodeManagementActionConflict)
		}
		if rule.IsNewcomerExclusive {
			var blockErr error
			blockedByNewcomerRule, blockErr = s.blockDuplicateNewcomerClaim(ctx, normalized, actorUserID, rule.DatabaseNow)
			if blockErr != nil {
				return blockErr
			}
			if blockedByNewcomerRule {
				return nil
			}
		}
		digest, digestErr := benefitDigest(rule.candidate())
		if digestErr != nil {
			return gerror.Wrap(digestErr, "digest managed promotion asset benefit")
		}
		assetType := "coupon"
		if rule.CampaignType == "voucher" {
			assetType = "voucher"
		}
		insert, insertErr := s.database.Exec(ctx, `
INSERT INTO plugin_idc_core_marketing_promotion_asset (
    tenant_id, asset_no, campaign_no, benefit_no, benefit_type_code,
    asset_type_code, customer_no, benefit_snapshot_digest, currency_code,
    face_amount_minor, rate_basis_points, usage_limit, reserved_uses,
    consumed_uses, valid_from, valid_until, status_code, claimed_at
) VALUES (0, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 1, 0, 0, ?, ?, 'active', ?)`,
			assetNo, normalized.CampaignNo, normalized.BenefitNo, rule.BenefitType,
			assetType, normalized.CustomerNo, digest, rule.CurrencyCode,
			rule.AmountMinor, rule.RateBasisPoints, rule.DatabaseNow, rule.EndsAt, rule.DatabaseNow,
		)
		if insertErr != nil {
			return gerror.Wrap(insertErr, "insert managed promotion asset")
		}
		if affectedErr := requireOneAffected(insert, "insert managed promotion asset"); affectedErr != nil {
			return affectedErr
		}
		result = &ManagedAssetMutation{
			AssetNo: assetNo, CustomerNo: normalized.CustomerNo, Status: "active", UpdatedAt: rule.DatabaseNow,
		}
		return s.appendAssetIssueAudit(ctx, normalized, result)
	})
	if err == nil && blockedByNewcomerRule {
		return nil, bizerr.NewCode(CodeManagementActionConflict)
	}
	return managementActionResult(result, err)
}

func (s *managementActionServiceImpl) RevokeAsset(
	ctx context.Context,
	input AssetRevokeInput,
) (*ManagedAssetMutation, error) {
	actorUserID, err := s.authorizer.AuthorizeAssetRevoke(ctx)
	if err != nil {
		return nil, err
	}
	normalized, err := normalizeAssetRevoke(input)
	if err != nil || actorUserID <= 0 {
		return nil, bizerr.NewCode(CodeManagementActionInvalid)
	}
	var result *ManagedAssetMutation
	err = s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		asset, loadErr := loadManagedAsset(ctx, s.database, normalized.AssetNo, true)
		if loadErr != nil {
			return loadErr
		}
		if asset == nil {
			return bizerr.NewCode(CodeManagementActionConflict)
		}
		originalStatus := asset.Status
		if originalStatus == "revoked" {
			originalStatus = "active"
			if asset.ClaimedAt == nil {
				originalStatus = "issued"
			}
		}
		before := map[string]any{
			"state": originalStatus, "reserved_uses": 0, "consumed_uses": asset.ConsumedUses,
			"expected_updated_at_ms": normalized.ExpectedUpdatedAt.UnixMilli(),
		}
		if asset.Status == "revoked" {
			result = &ManagedAssetMutation{
				AssetNo: asset.AssetNo, CustomerNo: asset.CustomerNo, Status: asset.Status,
				UpdatedAt: asset.UpdatedAt, Reused: true,
			}
			return s.appendAssetRevokeAudit(ctx, normalized, result, before, asset.ConsumedUses)
		}
		if (asset.Status != "issued" && asset.Status != "active") || asset.ReservedUses != 0 ||
			asset.UpdatedAt.UnixMilli() != normalized.ExpectedUpdatedAt.UnixMilli() {
			return bizerr.NewCode(CodeManagementActionConflict)
		}
		var updatedRows []struct {
			UpdatedAt time.Time `orm:"updated_at"`
		}
		updateErr := s.database.GetScan(ctx, &updatedRows, `
UPDATE plugin_idc_core_marketing_promotion_asset
SET status_code = 'revoked', updated_at = CURRENT_TIMESTAMP
WHERE tenant_id = 0 AND asset_no = ? AND status_code = ? AND reserved_uses = 0 AND updated_at = ?
RETURNING updated_at`, asset.AssetNo, asset.Status, asset.UpdatedAt)
		if updateErr != nil {
			return gerror.Wrap(updateErr, "revoke managed promotion asset")
		}
		if len(updatedRows) != 1 {
			return bizerr.NewCode(CodeManagementActionConflict)
		}
		result = &ManagedAssetMutation{
			AssetNo: asset.AssetNo, CustomerNo: asset.CustomerNo, Status: "revoked",
			UpdatedAt: updatedRows[0].UpdatedAt,
		}
		return s.appendAssetRevokeAudit(ctx, normalized, result, before, asset.ConsumedUses)
	})
	return managementActionResult(result, err)
}

func (s *managementActionServiceImpl) CreditPoints(
	ctx context.Context,
	input PointsCreditInput,
) (*ManagedPointsMutation, error) {
	actorUserID, err := s.authorizer.AuthorizePointsCredit(ctx)
	if err != nil {
		return nil, err
	}
	normalized, err := normalizePointsCredit(input)
	if err != nil || actorUserID <= 0 {
		return nil, bizerr.NewCode(CodeManagementActionInvalid)
	}
	var result *ManagedPointsMutation
	err = s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		if activeErr := requireManagedCustomer(ctx, s.database, normalized.CustomerNo, true); activeErr != nil {
			return activeErr
		}
		before, loadErr := lockPointsAccount(ctx, s.database, normalized.CustomerNo, true)
		if loadErr != nil {
			return loadErr
		}
		mutation, mutationErr := s.points.Award(ctx, PointsAwardInput{
			CustomerNo: normalized.CustomerNo, SourceType: "manual", SourceReference: normalized.RequestNo,
			Points: normalized.Points, ExpiresAt: normalized.ExpiresAt,
		})
		if mutationErr != nil {
			return mutationErr
		}
		result = managedPointsMutation(mutation)
		if result.Reused {
			return nil
		}
		return s.appendPointsAudit(
			ctx, "credit", normalized.RequestNo, normalized.CustomerNo,
			normalized.ReasonCode, normalized.Points, result, map[string]any{"available_points": before.AvailablePoints, "frozen_points": before.FrozenPoints, "debt_points": before.DebtPoints},
		)
	})
	return managementActionResult(result, err)
}

func (s *managementActionServiceImpl) ReversePointsAward(
	ctx context.Context,
	input PointsReverseManagementInput,
) (*ManagedPointsMutation, error) {
	actorUserID, err := s.authorizer.AuthorizePointsReverse(ctx)
	if err != nil {
		return nil, err
	}
	normalized, err := normalizePointsReverseManagement(input)
	if err != nil || actorUserID <= 0 {
		return nil, bizerr.NewCode(CodeManagementActionInvalid)
	}
	var result *ManagedPointsMutation
	err = s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		if activeErr := requireManagedCustomer(ctx, s.database, normalized.CustomerNo, false); activeErr != nil {
			return activeErr
		}
		before, loadErr := lockPointsAccount(ctx, s.database, normalized.CustomerNo, true)
		if loadErr != nil {
			return loadErr
		}
		mutation, mutationErr := s.points.ReverseAward(ctx, PointsReverseInput{
			CustomerNo: normalized.CustomerNo, AwardSourceType: normalized.AwardSourceType,
			AwardSourceReference: normalized.AwardSourceReference, ReversalSourceType: "manual",
			ReversalSourceReference: normalized.RequestNo, Points: normalized.Points,
		})
		if mutationErr != nil {
			return mutationErr
		}
		result = managedPointsMutation(mutation)
		if result.Reused {
			return nil
		}
		return s.appendPointsAudit(
			ctx, "reverse", normalized.RequestNo, normalized.CustomerNo,
			normalized.ReasonCode, normalized.Points, result, map[string]any{"available_points": before.AvailablePoints, "frozen_points": before.FrozenPoints, "debt_points": before.DebtPoints},
		)
	})
	return managementActionResult(result, err)
}

type managedAssetRow struct {
	AssetNo      string     `orm:"asset_no"`
	CampaignNo   string     `orm:"campaign_no"`
	BenefitNo    string     `orm:"benefit_no"`
	CustomerNo   string     `orm:"customer_no"`
	Status       string     `orm:"status_code"`
	ReservedUses int        `orm:"reserved_uses"`
	ConsumedUses int        `orm:"consumed_uses"`
	ClaimedAt    *time.Time `orm:"claimed_at"`
	UpdatedAt    time.Time  `orm:"updated_at"`
}

type managedAssetRuleRow struct {
	CampaignNo            string    `orm:"campaign_no"`
	CampaignType          string    `orm:"campaign_type_code"`
	TargetSegmentNo       *string   `orm:"target_segment_no"`
	TargetSnapshotNo      *string   `orm:"target_snapshot_no"`
	Priority              int       `orm:"priority"`
	StackPolicy           string    `orm:"stack_policy_code"`
	StackGroup            string    `orm:"stack_group_code"`
	BenefitNo             string    `orm:"benefit_no"`
	BenefitType           string    `orm:"benefit_type_code"`
	CurrencyCode          *string   `orm:"currency_code"`
	AmountMinor           *int64    `orm:"amount_minor"`
	RateBasisPoints       *int      `orm:"rate_basis_points"`
	MaximumDiscountMinor  *int64    `orm:"maximum_discount_minor"`
	MinimumSpendMinor     *int64    `orm:"minimum_spend_minor"`
	PurchasePhase         string    `orm:"purchase_phase_code"`
	CycleScope            string    `orm:"cycle_scope_code"`
	CycleCount            *int      `orm:"cycle_count"`
	CampaignVersion       int64     `orm:"campaign_version"`
	BenefitVersion        int64     `orm:"benefit_version"`
	GlobalQuotaLimit      *int64    `orm:"global_quota_limit"`
	PerCustomerQuotaLimit *int      `orm:"per_customer_quota_limit"`
	DatabaseNow           time.Time `orm:"database_now"`
	EndsAt                time.Time `orm:"ends_at"`
	IsNewcomerExclusive   bool      `orm:"is_newcomer_exclusive"`
}

func (row managedAssetRuleRow) candidate() candidateRow {
	return candidateRow{
		CampaignNo: row.CampaignNo, CampaignType: row.CampaignType,
		TargetSegmentNo: row.TargetSegmentNo, TargetSnapshotNo: row.TargetSnapshotNo,
		Priority: row.Priority, StackPolicy: row.StackPolicy, StackGroup: row.StackGroup,
		BenefitNo: row.BenefitNo, BenefitType: row.BenefitType, CurrencyCode: row.CurrencyCode,
		AmountMinor: row.AmountMinor, RateBasisPoints: row.RateBasisPoints,
		MaximumDiscountMinor: row.MaximumDiscountMinor, MinimumSpendMinor: row.MinimumSpendMinor,
		PurchasePhase: row.PurchasePhase, CycleScope: row.CycleScope, CycleCount: row.CycleCount,
		CampaignVersion: row.CampaignVersion, BenefitVersion: row.BenefitVersion,
	}
}

func loadManagedAsset(
	ctx context.Context,
	database gdb.DB,
	assetNo string,
	forUpdate bool,
) (*managedAssetRow, error) {
	query := `SELECT asset_no, campaign_no, benefit_no, coalesce(customer_no, '') AS customer_no,
status_code, reserved_uses, consumed_uses, claimed_at, updated_at
FROM plugin_idc_core_marketing_promotion_asset WHERE tenant_id = 0 AND asset_no = ?`
	if forUpdate {
		query += " FOR UPDATE"
	}
	var rows []managedAssetRow
	if err := database.GetScan(ctx, &rows, query, assetNo); err != nil {
		return nil, gerror.Wrap(err, "load managed promotion asset")
	}
	if len(rows) == 0 {
		return nil, nil
	}
	if len(rows) != 1 {
		return nil, gerror.New("managed promotion asset identity is ambiguous")
	}
	return &rows[0], nil
}

func loadManagedAssetRule(
	ctx context.Context,
	database gdb.DB,
	input AssetIssueInput,
) (*managedAssetRuleRow, error) {
	var rows []managedAssetRuleRow
	err := database.GetScan(ctx, &rows, `
SELECT c.campaign_no, c.campaign_type_code, c.target_segment_no, c.target_snapshot_no,
       c.priority, c.stack_policy_code, c.stack_group_code, c.version AS campaign_version,
       c.global_quota_limit, c.per_customer_quota_limit, c.ends_at, c.is_newcomer_exclusive,
       b.benefit_no, b.benefit_type_code, b.currency_code, b.amount_minor,
       b.rate_basis_points, b.maximum_discount_minor, b.minimum_spend_minor,
       b.purchase_phase_code, b.cycle_scope_code, b.cycle_count,
       b.version AS benefit_version, CURRENT_TIMESTAMP AS database_now
FROM plugin_idc_core_marketing_campaign c
JOIN plugin_idc_core_marketing_benefit_rule b
  ON b.tenant_id = c.tenant_id AND b.campaign_no = c.campaign_no
JOIN plugin_idc_core_customer_subject customer
  ON customer.tenant_id = c.tenant_id AND customer.customer_no = ? AND customer.status_code = 'active'
WHERE c.tenant_id = 0 AND c.campaign_no = ? AND b.benefit_no = ?
  AND c.status_code = 'active' AND CURRENT_TIMESTAMP >= c.starts_at AND CURRENT_TIMESTAMP < c.ends_at
  AND c.campaign_type_code IN ('coupon', 'voucher')
  AND b.status_code = 'active' AND b.benefit_type_code IN ('fixed_discount', 'percentage_discount', 'voucher')
  AND (c.target_snapshot_no IS NULL OR EXISTS (
      SELECT 1 FROM plugin_idc_core_marketing_snapshot_member member
      WHERE member.tenant_id = c.tenant_id AND member.snapshot_no = c.target_snapshot_no
        AND member.customer_no = customer.customer_no
  ))`, input.CustomerNo, input.CampaignNo, input.BenefitNo)
	if err != nil {
		return nil, gerror.Wrap(err, "load managed promotion asset rule")
	}
	if len(rows) == 0 {
		return nil, nil
	}
	if len(rows) != 1 {
		return nil, gerror.New("managed promotion asset rule is ambiguous")
	}
	return &rows[0], nil
}

type newcomerClaimHistory struct {
	SubjectKind         string     `orm:"subject_kind"`
	SubjectDigest       string     `orm:"subject_digest"`
	PriorClaimCount     int        `orm:"prior_claim_count"`
	RelatedAccountCount int        `orm:"related_account_count"`
	FirstClaimedAt      *time.Time `orm:"first_claimed_at"`
}

func (s *managementActionServiceImpl) blockDuplicateNewcomerClaim(
	ctx context.Context,
	input AssetIssueInput,
	actorUserID int,
	now time.Time,
) (bool, error) {
	var history *newcomerClaimHistory
	if err := s.database.GetScan(ctx, &history, `
WITH subject AS (
  SELECT 'individual'::TEXT AS subject_kind, identity_lookup_digest AS subject_digest
  FROM plugin_idc_core_customer_individual_profile
  WHERE tenant_id=0 AND customer_no=? AND identity_status_code='verified' AND identity_lookup_digest<>''
  UNION ALL
  SELECT 'enterprise'::TEXT, registration_lookup_digest
  FROM plugin_idc_core_customer_enterprise_profile
  WHERE tenant_id=0 AND customer_no=? AND verification_status_code='verified' AND registration_lookup_digest<>''
), matching_customers AS (
  SELECT profile.customer_no, subject.subject_kind, subject.subject_digest
  FROM subject JOIN plugin_idc_core_customer_individual_profile profile
    ON subject.subject_kind='individual' AND profile.tenant_id=0
   AND profile.identity_status_code='verified' AND profile.identity_lookup_digest=subject.subject_digest
  UNION ALL
  SELECT profile.customer_no, subject.subject_kind, subject.subject_digest
  FROM subject JOIN plugin_idc_core_customer_enterprise_profile profile
    ON subject.subject_kind='enterprise' AND profile.tenant_id=0
   AND profile.verification_status_code='verified' AND profile.registration_lookup_digest=subject.subject_digest
)
SELECT matching.subject_kind, matching.subject_digest, count(asset.id)::INT AS prior_claim_count,
       count(DISTINCT asset.customer_no)::INT AS related_account_count,
       min(asset.claimed_at) AS first_claimed_at
FROM matching_customers matching
JOIN plugin_idc_core_marketing_promotion_asset asset
  ON asset.customer_no=matching.customer_no AND asset.claimed_at IS NOT NULL
JOIN plugin_idc_core_marketing_campaign campaign
  ON campaign.campaign_no=asset.campaign_no AND campaign.is_newcomer_exclusive=TRUE
GROUP BY matching.subject_kind, matching.subject_digest`, input.CustomerNo, input.CustomerNo); err != nil {
		return false, gerror.Wrap(err, "load newcomer claim history")
	}
	if history == nil || history.SubjectDigest == "" || history.PriorClaimCount < 1 || history.FirstClaimedAt == nil {
		return false, nil
	}
	evidenceDigest := marketingFactDigest("duplicate-newcomer-claim", history.SubjectKind, history.SubjectDigest, input.CampaignNo)
	caseNo := "mab_" + evidenceDigest[:32]
	if _, err := s.database.Exec(ctx, `INSERT INTO plugin_idc_core_marketing_abuse_case
 (tenant_id, case_no, campaign_no, customer_no, signal_code, related_account_count,
  evidence_summary_code, evidence_digest, window_started_at, window_ended_at, observed_count)
VALUES (0, ?, ?, ?, 'duplicate_newcomer_claim', ?, 'verified_subject_reused', ?, ?, ?, ?)
ON CONFLICT DO NOTHING`, caseNo, input.CampaignNo, input.CustomerNo,
		history.RelatedAccountCount, evidenceDigest, history.FirstClaimedAt.UTC(), now.UTC(), history.PriorClaimCount+1); err != nil {
		return false, gerror.Wrap(err, "create duplicate newcomer claim case")
	}
	if err := s.audit.AppendBusinessTransition(ctx, idccap.BusinessAuditTransition{
		IdempotencyScope: "marketing.asset.issue", IdempotencyKey: input.RequestNo,
		OperationCode: "marketing.asset.issue", TargetType: "marketing_campaign", TargetNo: input.CampaignNo,
		CustomerNo: input.CustomerNo, ResultCode: "denied", ReasonCode: "duplicate_newcomer_claim",
		RiskCode: "high", RequiredPermission: permissionMarketingAssetIssue, CorrelationNo: caseNo,
		BeforeSnapshot: map[string]any{"prior_newcomer_claims": history.PriorClaimCount},
		AfterSnapshot:  map[string]any{"blocked": true, "case_no": caseNo},
	}); err != nil {
		return false, gerror.Wrap(err, "audit duplicate newcomer claim rejection")
	}
	return true, nil
}

func requireManagedCustomer(
	ctx context.Context,
	database gdb.DB,
	customerNo string,
	activeOnly bool,
) error {
	var rows []struct {
		Status string `orm:"status_code"`
	}
	err := database.GetScan(ctx, &rows, `SELECT status_code
FROM plugin_idc_core_customer_subject
WHERE tenant_id = 0 AND customer_no = ?
FOR SHARE`, customerNo)
	if err != nil {
		return gerror.Wrap(err, "validate managed points customer")
	}
	if len(rows) != 1 || (activeOnly && rows[0].Status != "active") {
		return bizerr.NewCode(CodeManagementActionConflict)
	}
	return nil
}

func normalizeAssetIssue(input AssetIssueInput) (AssetIssueInput, error) {
	input.RequestNo = strings.TrimSpace(input.RequestNo)
	input.CampaignNo = strings.TrimSpace(input.CampaignNo)
	input.BenefitNo = strings.TrimSpace(input.BenefitNo)
	input.CustomerNo = strings.TrimSpace(input.CustomerNo)
	input.ReasonCode = strings.TrimSpace(input.ReasonCode)
	if !validBusinessReference(input.RequestNo) || !identifierPattern.MatchString(input.CampaignNo) ||
		!identifierPattern.MatchString(input.BenefitNo) || !identifierPattern.MatchString(input.CustomerNo) ||
		!validManagementReason(input.ReasonCode) {
		return AssetIssueInput{}, bizerr.NewCode(CodeManagementActionInvalid)
	}
	return input, nil
}

func normalizeAssetRevoke(input AssetRevokeInput) (AssetRevokeInput, error) {
	input.RequestNo = strings.TrimSpace(input.RequestNo)
	input.AssetNo = strings.TrimSpace(input.AssetNo)
	input.ReasonCode = strings.TrimSpace(input.ReasonCode)
	input.ExpectedUpdatedAt = input.ExpectedUpdatedAt.UTC()
	if !validBusinessReference(input.RequestNo) || !identifierPattern.MatchString(input.AssetNo) ||
		input.ExpectedUpdatedAt.IsZero() || !validManagementReason(input.ReasonCode) {
		return AssetRevokeInput{}, bizerr.NewCode(CodeManagementActionInvalid)
	}
	return input, nil
}

func normalizePointsCredit(input PointsCreditInput) (PointsCreditInput, error) {
	input.RequestNo = strings.TrimSpace(input.RequestNo)
	input.CustomerNo = strings.TrimSpace(input.CustomerNo)
	input.ReasonCode = strings.TrimSpace(input.ReasonCode)
	if input.ExpiresAt != nil {
		value := input.ExpiresAt.UTC()
		input.ExpiresAt = &value
	}
	if !validBusinessReference(input.RequestNo) || !identifierPattern.MatchString(input.CustomerNo) ||
		input.Points < 1 || input.Points > maximumManagedPoints || !validManagementReason(input.ReasonCode) {
		return PointsCreditInput{}, bizerr.NewCode(CodeManagementActionInvalid)
	}
	return input, nil
}

func normalizePointsReverseManagement(
	input PointsReverseManagementInput,
) (PointsReverseManagementInput, error) {
	input.RequestNo = strings.TrimSpace(input.RequestNo)
	input.CustomerNo = strings.TrimSpace(input.CustomerNo)
	input.AwardSourceType = strings.TrimSpace(input.AwardSourceType)
	input.AwardSourceReference = strings.TrimSpace(input.AwardSourceReference)
	input.ReasonCode = strings.TrimSpace(input.ReasonCode)
	if !validBusinessReference(input.RequestNo) || !identifierPattern.MatchString(input.CustomerNo) ||
		!validPointsAwardSource(input.AwardSourceType) || !validBusinessReference(input.AwardSourceReference) ||
		input.Points < 1 || input.Points > maximumManagedPoints || !validManagementReason(input.ReasonCode) {
		return PointsReverseManagementInput{}, bizerr.NewCode(CodeManagementActionInvalid)
	}
	return input, nil
}

func validManagementReason(value string) bool {
	return value == "customer_service" || value == "campaign_fulfillment" ||
		value == "refund_correction" || value == "risk_control" ||
		value == "manual_correction" || value == "migration_reconciliation"
}

func deterministicManagedAssetNo(requestNo string) string {
	digest := sha256.Sum256([]byte("marketing.asset.issue\x00" + requestNo))
	return "mast_" + hex.EncodeToString(digest[:12])
}

func managedPointsMutation(input *PointsMutation) *ManagedPointsMutation {
	return &ManagedPointsMutation{
		LotNo: input.LotNo, CustomerNo: input.CustomerNo,
		AvailablePoints: input.AvailablePoints, FrozenPoints: input.FrozenPoints,
		DebtPoints: input.DebtPoints, Reused: input.Reused,
	}
}

func (s *managementActionServiceImpl) appendAssetIssueAudit(
	ctx context.Context,
	input AssetIssueInput,
	result *ManagedAssetMutation,
) error {
	return s.audit.AppendBusinessTransition(ctx, idccap.BusinessAuditTransition{BeforeSnapshot: map[string]any{"asset_no": ""},
		IdempotencyScope: "marketing.asset.issue", IdempotencyKey: input.RequestNo,
		OperationCode: "marketing.asset.issue", TargetType: "marketing_promotion_asset",
		TargetNo: result.AssetNo, CustomerNo: result.CustomerNo, ResultCode: "succeeded",
		ReasonCode: input.ReasonCode, RiskCode: "medium",
		RequiredPermission: permissionMarketingAssetIssue, CorrelationNo: input.CampaignNo,
		AfterSnapshot: map[string]any{
			"state": "active", "asset_no": result.AssetNo,
			"campaign_no": input.CampaignNo, "benefit_no": input.BenefitNo,
		},
	})
}

func (s *managementActionServiceImpl) appendAssetRevokeAudit(
	ctx context.Context,
	input AssetRevokeInput,
	result *ManagedAssetMutation,
	before map[string]any,
	consumedUses int,
) error {
	return s.audit.AppendBusinessTransition(ctx, idccap.BusinessAuditTransition{
		IdempotencyScope: "marketing.asset.revoke", IdempotencyKey: input.RequestNo,
		OperationCode: "marketing.asset.revoke", TargetType: "marketing_promotion_asset",
		TargetNo: result.AssetNo, CustomerNo: result.CustomerNo, ResultCode: "succeeded",
		ReasonCode: input.ReasonCode, RiskCode: "high", BeforeSnapshot: before,
		RequiredPermission: permissionMarketingAssetRevoke,
		AfterSnapshot:      managedAssetAuditSnapshot(result.Status, 0, consumedUses, result.UpdatedAt),
	})
}

func (s *managementActionServiceImpl) appendPointsAudit(
	ctx context.Context,
	operation string,
	requestNo string,
	customerNo string,
	reason string,
	points int64,
	result *ManagedPointsMutation,
	before map[string]any,
) error {
	requiredPermission := permissionMarketingPointsCredit
	if operation == "reverse" {
		requiredPermission = permissionMarketingPointsReverse
	}
	return s.audit.AppendBusinessTransition(ctx, idccap.BusinessAuditTransition{
		IdempotencyScope: "marketing.points." + operation, IdempotencyKey: requestNo,
		OperationCode: "marketing.points." + operation, TargetType: "marketing_points_account",
		TargetNo: customerNo, CustomerNo: customerNo, ResultCode: "succeeded",
		ReasonCode: reason, RiskCode: "high", CorrelationNo: result.LotNo,
		RequiredPermission: requiredPermission,
		BeforeSnapshot:     before,
		AfterSnapshot: map[string]any{
			"state": "recorded", "lot_no": result.LotNo, "points": points,
		},
	})
}

func managedAssetAuditSnapshot(
	status string,
	reservedUses int,
	consumedUses int,
	updatedAt time.Time,
) map[string]any {
	return map[string]any{
		"state": status, "reserved_uses": reservedUses, "consumed_uses": consumedUses,
		"updated_at_ms": updatedAt.UnixMilli(),
	}
}

func managementActionResult[T any](result *T, err error) (*T, error) {
	if err == nil {
		return result, nil
	}
	if _, ok := bizerr.As(err); ok {
		return nil, err
	}
	return nil, bizerr.WrapCode(err, CodeManagementActionUnavailable)
}
