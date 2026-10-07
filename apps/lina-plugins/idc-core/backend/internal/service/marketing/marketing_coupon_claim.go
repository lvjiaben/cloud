package marketing

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/netip"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gcode"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/internal/service/principal"
)

var promotionCodePattern = regexp.MustCompile(`^[A-Z0-9]{8,24}$`)

var CodeCouponInvalid = bizerr.MustDefine(
	"IDC_MARKETING_COUPON_INVALID", "The promotion code is invalid", gcode.CodeInvalidOperation,
)

type CouponClaimService interface {
	ListMine(context.Context) ([]CouponItem, error)
	Claim(context.Context, CouponClaimInput) (*CouponItem, error)
}

type CouponClaimInput struct {
	Code     string
	DeviceID string
	SourceIP string
}

type CouponItem struct {
	AssetNo         string
	CampaignNo      string
	NameZhCN        string
	NameEnUS        string
	AssetType       string
	CurrencyCode    string
	FaceAmountMinor *int64
	RateBasisPoints *int
	UsageLimit      int
	ConsumedUses    int
	Status          string
	ValidFrom       time.Time
	ValidUntil      time.Time
	ClaimedAt       *time.Time
}

type couponClaimServiceImpl struct {
	database  gdb.DB
	principal principal.Reader
	key       []byte
}

func NewCouponClaimService(database gdb.DB, principalReader principal.Reader, key []byte) (CouponClaimService, error) {
	if database == nil || principalReader == nil || len(key) != sha256.Size {
		return nil, gerror.New("marketing coupon claims require database, principal, and installation key")
	}
	return &couponClaimServiceImpl{database: database, principal: principalReader, key: append([]byte(nil), key...)}, nil
}

func (s *couponClaimServiceImpl) ListMine(ctx context.Context) ([]CouponItem, error) {
	customerNo, err := s.currentCustomer(ctx)
	if err != nil {
		return nil, err
	}
	items := make([]CouponItem, 0)
	if err = s.database.GetScan(ctx, &items, `SELECT asset.asset_no, asset.campaign_no,
 campaign.name_zh_cn, campaign.name_en_us, asset.asset_type_code AS asset_type,
 asset.currency_code, asset.face_amount_minor, asset.rate_basis_points, asset.usage_limit,
 asset.consumed_uses, asset.status_code AS status, asset.valid_from, asset.valid_until, asset.claimed_at
FROM plugin_idc_core_marketing_promotion_asset asset
JOIN plugin_idc_core_marketing_campaign campaign ON campaign.campaign_no=asset.campaign_no
WHERE asset.tenant_id=0 AND asset.customer_no=?
ORDER BY asset.claimed_at DESC, asset.id DESC LIMIT 100`, customerNo); err != nil {
		return nil, bizerr.WrapCode(err, CodeInsightsUnavailable)
	}
	return items, nil
}

func (s *couponClaimServiceImpl) Claim(ctx context.Context, input CouponClaimInput) (*CouponItem, error) {
	customerNo, err := s.currentCustomer(ctx)
	if err != nil {
		return nil, err
	}
	code := strings.ToUpper(strings.TrimSpace(input.Code))
	deviceID := strings.TrimSpace(input.DeviceID)
	address, parseErr := netip.ParseAddr(strings.TrimSpace(input.SourceIP))
	if !promotionCodePattern.MatchString(code) || len(deviceID) < 16 || len(deviceID) > 128 || parseErr != nil {
		return nil, bizerr.NewCode(CodeCouponInvalid)
	}
	sources := []couponSource{
		{Kind: "device", Digest: s.digest("coupon-device", deviceID)},
		{Kind: "ip", Digest: s.digest("coupon-ip", address.Unmap().String())},
	}
	sort.Slice(sources, func(i, j int) bool { return sources[i].Kind+sources[i].Digest < sources[j].Kind+sources[j].Digest })
	codeDigest := s.digestBytes("coupon-code", code)
	var result CouponItem
	rejected := false
	err = s.database.Transaction(ctx, func(txCtx context.Context, _ gdb.TX) error {
		for _, source := range sources {
			if _, lockErr := s.database.GetValue(txCtx, `SELECT pg_advisory_xact_lock(hashtextextended(?, 62100411))`, source.Kind+":"+source.Digest); lockErr != nil {
				return lockErr
			}
		}
		blocked, blockErr := s.anyCouponSourceBlocked(txCtx, sources)
		if blockErr != nil {
			return blockErr
		}
		if blocked {
			rejected = true
			return nil
		}
		var candidate *couponClaimCandidate
		if readErr := s.database.GetScan(txCtx, &candidate, `SELECT asset.asset_no, asset.campaign_no,
 campaign.name_zh_cn, campaign.name_en_us, asset.asset_type_code AS asset_type,
 asset.currency_code, asset.face_amount_minor, asset.rate_basis_points, asset.usage_limit,
 asset.consumed_uses, asset.status_code AS status, asset.valid_from, asset.valid_until,
 asset.claimed_at, campaign.is_newcomer_exclusive,
 (asset.status_code='issued' AND asset.customer_no IS NULL AND asset.valid_from<=CURRENT_TIMESTAMP
  AND asset.valid_until>CURRENT_TIMESTAMP AND campaign.status_code='active'
  AND campaign.starts_at<=CURRENT_TIMESTAMP AND campaign.ends_at>CURRENT_TIMESTAMP) AS claimable
FROM plugin_idc_core_marketing_promotion_asset asset
JOIN plugin_idc_core_marketing_campaign campaign ON campaign.campaign_no=asset.campaign_no
WHERE asset.tenant_id=0 AND asset.claim_code_digest=? FOR UPDATE OF asset`, codeDigest); readErr != nil {
			return readErr
		}
		if candidate == nil || candidate.AssetNo == "" || !candidate.Claimable {
			if recordErr := s.recordCouponFailures(txCtx, customerNo, sources); recordErr != nil {
				return recordErr
			}
			rejected = true
			return nil
		}
		if candidate.IsNewcomerExclusive {
			duplicate, duplicateErr := s.duplicateNewcomerClaim(txCtx, customerNo, candidate.CampaignNo)
			if duplicateErr != nil {
				return duplicateErr
			}
			if duplicate {
				rejected = true
				return nil
			}
		}
		rows, updateErr := s.database.Exec(txCtx, `UPDATE plugin_idc_core_marketing_promotion_asset
SET customer_no=?, status_code='active', claimed_at=CURRENT_TIMESTAMP, updated_at=CURRENT_TIMESTAMP
WHERE tenant_id=0 AND asset_no=? AND status_code='issued' AND customer_no IS NULL`, customerNo, candidate.AssetNo)
		if updateErr != nil {
			return updateErr
		}
		affected, affectedErr := rows.RowsAffected()
		if affectedErr != nil || affected != 1 {
			if affectedErr != nil {
				return affectedErr
			}
			rejected = true
			return nil
		}
		if _, resetErr := s.database.Exec(txCtx, `UPDATE plugin_idc_core_marketing_coupon_source_state
SET failure_count=0, window_started_at=CURRENT_TIMESTAMP, blocked_until=NULL, updated_at=CURRENT_TIMESTAMP
WHERE tenant_id=0 AND ((source_kind_code=? AND source_digest=?) OR (source_kind_code=? AND source_digest=?))`,
			sources[0].Kind, sources[0].Digest, sources[1].Kind, sources[1].Digest); resetErr != nil {
			return resetErr
		}
		return s.database.GetScan(txCtx, &result, `SELECT asset.asset_no, asset.campaign_no,
 campaign.name_zh_cn, campaign.name_en_us, asset.asset_type_code AS asset_type,
 asset.currency_code, asset.face_amount_minor, asset.rate_basis_points, asset.usage_limit,
 asset.consumed_uses, asset.status_code AS status, asset.valid_from, asset.valid_until, asset.claimed_at
FROM plugin_idc_core_marketing_promotion_asset asset
JOIN plugin_idc_core_marketing_campaign campaign ON campaign.campaign_no=asset.campaign_no
WHERE asset.asset_no=?`, candidate.AssetNo)
	})
	if err != nil {
		return nil, bizerr.WrapCode(err, CodeInsightsUnavailable)
	}
	if rejected || result.AssetNo == "" {
		return nil, bizerr.NewCode(CodeCouponInvalid)
	}
	return &result, nil
}

type couponSource struct{ Kind, Digest string }

type couponClaimCandidate struct {
	CouponItem
	IsNewcomerExclusive bool `orm:"is_newcomer_exclusive"`
	Claimable           bool `orm:"claimable"`
}

func (s *couponClaimServiceImpl) anyCouponSourceBlocked(ctx context.Context, sources []couponSource) (bool, error) {
	value, err := s.database.GetValue(ctx, `SELECT EXISTS(SELECT 1 FROM plugin_idc_core_marketing_coupon_source_state
WHERE tenant_id=0 AND blocked_until>CURRENT_TIMESTAMP
  AND ((source_kind_code=? AND source_digest=?) OR (source_kind_code=? AND source_digest=?)))`,
		sources[0].Kind, sources[0].Digest, sources[1].Kind, sources[1].Digest)
	return value.Bool(), err
}

func (s *couponClaimServiceImpl) recordCouponFailures(ctx context.Context, customerNo string, sources []couponSource) error {
	for _, source := range sources {
		var state struct {
			WindowStartedAt time.Time  `orm:"window_started_at"`
			FailureCount    int        `orm:"failure_count"`
			BlockedUntil    *time.Time `orm:"blocked_until"`
		}
		if err := s.database.GetScan(ctx, &state, `INSERT INTO plugin_idc_core_marketing_coupon_source_state
 (tenant_id, source_kind_code, source_digest, window_started_at, failure_count, updated_at)
VALUES (0, ?, ?, CURRENT_TIMESTAMP, 1, CURRENT_TIMESTAMP)
ON CONFLICT (source_kind_code, source_digest) DO UPDATE SET
 window_started_at=CASE WHEN plugin_idc_core_marketing_coupon_source_state.window_started_at<CURRENT_TIMESTAMP-INTERVAL '10 minutes' THEN CURRENT_TIMESTAMP ELSE plugin_idc_core_marketing_coupon_source_state.window_started_at END,
 failure_count=CASE WHEN plugin_idc_core_marketing_coupon_source_state.window_started_at<CURRENT_TIMESTAMP-INTERVAL '10 minutes' THEN 1 ELSE LEAST(10,plugin_idc_core_marketing_coupon_source_state.failure_count+1) END,
 blocked_until=CASE WHEN (CASE WHEN plugin_idc_core_marketing_coupon_source_state.window_started_at<CURRENT_TIMESTAMP-INTERVAL '10 minutes' THEN 1 ELSE plugin_idc_core_marketing_coupon_source_state.failure_count+1 END)>=10 THEN CURRENT_TIMESTAMP+INTERVAL '30 minutes' ELSE plugin_idc_core_marketing_coupon_source_state.blocked_until END,
 updated_at=CURRENT_TIMESTAMP
RETURNING window_started_at, failure_count, blocked_until`, source.Kind, source.Digest); err != nil {
			return err
		}
		if state.FailureCount < 10 || state.BlockedUntil == nil {
			continue
		}
		evidenceDigest := s.digest("coupon-abuse-evidence", source.Kind+":"+source.Digest+":"+state.WindowStartedAt.UTC().Format(time.RFC3339Nano))
		caseNo := "mab_" + evidenceDigest[:32]
		if _, err := s.database.Exec(ctx, `INSERT INTO plugin_idc_core_marketing_abuse_case
 (tenant_id, case_no, campaign_no, customer_no, signal_code, related_account_count,
  evidence_summary_code, evidence_digest, window_started_at, window_ended_at, observed_count, blocked_until)
VALUES (0, ?, NULL, ?, 'coupon_bruteforce', 0, ?, ?, ?, CURRENT_TIMESTAMP, 10, ?)
ON CONFLICT DO NOTHING`, caseNo, customerNo, "coupon_source_"+source.Kind, evidenceDigest,
			state.WindowStartedAt, state.BlockedUntil.UTC()); err != nil {
			return err
		}
	}
	return nil
}

func (s *couponClaimServiceImpl) duplicateNewcomerClaim(ctx context.Context, customerNo, campaignNo string) (bool, error) {
	var history *newcomerClaimHistory
	if err := s.database.GetScan(ctx, &history, `WITH subject AS (
 SELECT 'individual'::TEXT AS subject_kind, identity_lookup_digest AS subject_digest
 FROM plugin_idc_core_customer_individual_profile
 WHERE tenant_id=0 AND customer_no=? AND identity_status_code='verified' AND identity_lookup_digest<>''
 UNION ALL
 SELECT 'enterprise'::TEXT, registration_lookup_digest FROM plugin_idc_core_customer_enterprise_profile
 WHERE tenant_id=0 AND customer_no=? AND verification_status_code='verified' AND registration_lookup_digest<>''
), matching_customers AS (
 SELECT p.customer_no, s.subject_kind, s.subject_digest FROM subject s
 JOIN plugin_idc_core_customer_individual_profile p ON s.subject_kind='individual' AND p.tenant_id=0 AND p.identity_status_code='verified' AND p.identity_lookup_digest=s.subject_digest
 UNION ALL
 SELECT p.customer_no, s.subject_kind, s.subject_digest FROM subject s
 JOIN plugin_idc_core_customer_enterprise_profile p ON s.subject_kind='enterprise' AND p.tenant_id=0 AND p.verification_status_code='verified' AND p.registration_lookup_digest=s.subject_digest
)
SELECT matching.subject_kind, matching.subject_digest, count(asset.id)::INT AS prior_claim_count,
 count(DISTINCT asset.customer_no)::INT AS related_account_count, min(asset.claimed_at) AS first_claimed_at
FROM matching_customers matching JOIN plugin_idc_core_marketing_promotion_asset asset
 ON asset.customer_no=matching.customer_no AND asset.claimed_at IS NOT NULL
JOIN plugin_idc_core_marketing_campaign campaign
 ON campaign.campaign_no=asset.campaign_no AND campaign.is_newcomer_exclusive=TRUE
GROUP BY matching.subject_kind, matching.subject_digest`, customerNo, customerNo); err != nil {
		return false, err
	}
	if history == nil || history.SubjectDigest == "" || history.PriorClaimCount < 1 || history.FirstClaimedAt == nil {
		return false, nil
	}
	evidenceDigest := marketingFactDigest("duplicate-newcomer-claim", history.SubjectKind, history.SubjectDigest, campaignNo)
	caseNo := "mab_" + evidenceDigest[:32]
	if _, err := s.database.Exec(ctx, `INSERT INTO plugin_idc_core_marketing_abuse_case
 (tenant_id, case_no, campaign_no, customer_no, signal_code, related_account_count,
  evidence_summary_code, evidence_digest, window_started_at, window_ended_at, observed_count)
VALUES (0, ?, ?, ?, 'duplicate_newcomer_claim', ?, 'verified_subject_reused', ?, ?, CURRENT_TIMESTAMP, ?)
ON CONFLICT DO NOTHING`, caseNo, campaignNo, customerNo, history.RelatedAccountCount,
		evidenceDigest, history.FirstClaimedAt.UTC(), history.PriorClaimCount+1); err != nil {
		return false, err
	}
	return true, nil
}

func (s *couponClaimServiceImpl) currentCustomer(ctx context.Context) (string, error) {
	audience, err := s.principal.Current(ctx)
	if err != nil {
		return "", err
	}
	if audience == nil || audience.Code != principal.AudienceCustomer || audience.Status != principal.AudienceStatusActive || audience.CustomerNo == "" {
		return "", bizerr.NewCode(CodeCouponInvalid)
	}
	return audience.CustomerNo, nil
}

func (s *couponClaimServiceImpl) digest(purpose, value string) string {
	return hex.EncodeToString(s.digestBytes(purpose, value))
}

func (s *couponClaimServiceImpl) digestBytes(purpose, value string) []byte {
	mac := hmac.New(sha256.New, s.key)
	_, _ = mac.Write([]byte("idc/marketing/" + purpose + "/v1\x00"))
	_, _ = mac.Write([]byte(value))
	return mac.Sum(nil)
}
