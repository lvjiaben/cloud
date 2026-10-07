// This file persists accepted affiliate attributions as append-only records.
// Same-subject self-referrals append a rejected attribution and one redacted
// marketing review case in the same transaction.
package saleschannel

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-plugin-idc-core/backend/internal/dao"
	"lina-plugin-idc-core/backend/internal/model/do"
	"lina-plugin-idc-core/backend/internal/model/entity"
)

type affiliateIdentityRelation struct {
	SubjectKind         string `orm:"subject_kind"`
	SubjectDigest       string `orm:"subject_digest"`
	RelatedAccountCount int    `orm:"related_account_count"`
}

// PersistStoredAttribution loads the authoritative stored link, touch and
// verified identity projections. The HTTP caller can supply only opaque
// identifiers and the referred customer, never an identity-match decision.
func (s *serviceImpl) PersistStoredAttribution(ctx context.Context, now time.Time, linkNo, touchNo, referredCustomerNo string) (AttributionDecision, error) {
	if s.database == nil || ctx == nil || now.IsZero() || linkNo == "" || touchNo == "" || referredCustomerNo == "" {
		return AttributionDecision{}, gerror.New("affiliate attribution persistence request is invalid")
	}
	var decision AttributionDecision
	err := s.database.Transaction(ctx, func(txCtx context.Context, tx gdb.TX) error {
		var (
			link  *entity.SalesAffiliateLink
			touch *entity.SalesAffiliateTouch
		)
		if loadErr := tx.Model(dao.SalesAffiliateLink.Table()).Ctx(txCtx).
			Where(do.SalesAffiliateLink{TenantId: 0, LinkNo: linkNo}).Scan(&link); loadErr != nil {
			return gerror.Wrap(loadErr, "load affiliate link")
		}
		if link == nil {
			return gerror.New("affiliate link is unavailable")
		}
		if loadErr := tx.Model(dao.SalesAffiliateTouch.Table()).Ctx(txCtx).
			Where(do.SalesAffiliateTouch{TenantId: 0, TouchNo: touchNo, LinkNo: linkNo}).Scan(&touch); loadErr != nil {
			return gerror.Wrap(loadErr, "load affiliate touch")
		}
		if touch == nil {
			return gerror.New("affiliate touch is unavailable")
		}
		relation, relationErr := loadAffiliateIdentityRelation(tx, link.OwnerCustomerNo, referredCustomerNo)
		if relationErr != nil {
			return gerror.Wrap(relationErr, "load affiliate verified identity relation")
		}
		decision, relationErr = s.persistAttribution(txCtx, tx, AttributionRequest{
			Now: now, Link: link, Touch: touch, ReferredCustomerNo: referredCustomerNo,
			relatedIdentity: relation != nil,
		}, relation)
		return relationErr
	})
	if err != nil {
		return AttributionDecision{}, gerror.Wrap(err, "persist stored affiliate attribution")
	}
	return decision, nil
}

// PersistAttribution validates the referral link and touch, then appends one
// eligible or rejected self-referral attribution exactly once. Production HTTP
// paths use PersistStoredAttribution so identity facts remain server-derived.
func (s *serviceImpl) PersistAttribution(ctx context.Context, request AttributionRequest) (AttributionDecision, error) {
	if s.database == nil {
		return AttributionDecision{}, gerror.New("sales channel database is unavailable")
	}
	var decision AttributionDecision
	err := s.database.Transaction(ctx, func(txCtx context.Context, tx gdb.TX) error {
		var relation *affiliateIdentityRelation
		if request.relatedIdentity {
			relation = &affiliateIdentityRelation{RelatedAccountCount: 2}
		}
		var persistErr error
		decision, persistErr = s.persistAttribution(txCtx, tx, request, relation)
		return persistErr
	})
	if err != nil {
		return AttributionDecision{}, gerror.Wrap(err, "persist affiliate attribution")
	}
	return decision, nil
}

func (s *serviceImpl) persistAttribution(ctx context.Context, tx gdb.TX, request AttributionRequest, relation *affiliateIdentityRelation) (AttributionDecision, error) {
	// The transaction performs its own locked duplicate check. Skipping the
	// outside preflight here avoids borrowing a second database connection.
	decision, err := s.attribute(ctx, request, false)
	if err != nil || (decision.StatusCode != AffiliateEligible && decision.StatusCode != AffiliateRejectedSelfReferral) {
		return decision, err
	}
	if request.Link == nil || request.Touch == nil || request.Touch.OccurredAt == nil {
		return AttributionDecision{}, gerror.New("affiliate attribution request is invalid")
	}
	attributedAt := request.Now
	expiresAt := request.Touch.ExpiresAt
	if expiresAt == nil {
		expiresAt = request.Link.ExpiresAt
	}
	if expiresAt == nil {
		value := attributedAt.Add(time.Duration(request.Link.AttributionWindowSeconds) * time.Second)
		expiresAt = &value
	}
	if !expiresAt.After(attributedAt) {
		return AttributionDecision{}, gerror.New("affiliate attribution expiry is invalid")
	}
	evidence := request.Touch.EvidenceDigest
	if !isLowerHexDigest(evidence) {
		hash := sha256.Sum256([]byte(fmt.Sprintf("%s:%s:%s:%d", request.Link.LinkNo, request.Touch.TouchNo, request.ReferredCustomerNo, request.Touch.OccurredAt.UnixNano())))
		evidence = hex.EncodeToString(hash[:])
	}
	attributionHash := sha256.Sum256([]byte(fmt.Sprintf("%s:%s:%s", request.Link.LinkNo, request.Touch.TouchNo, request.ReferredCustomerNo)))
	attributionNo := "ATTR_" + hex.EncodeToString(attributionHash[:])[:32]

	if decision.StatusCode == AffiliateRejectedSelfReferral {
		if persistErr := persistRejectedSelfReferral(tx, request, decision, relation, attributionNo, evidence, attributedAt, expiresAt); persistErr != nil {
			return AttributionDecision{}, persistErr
		}
		return decision, nil
	}

	// The account-scoped lock and in-transaction recheck prevent concurrent
	// requests from creating more than one eligible attribution.
	if _, lockErr := tx.Exec(`SELECT pg_advisory_xact_lock(hashtextextended(?,0))`, request.ReferredCustomerNo); lockErr != nil {
		return AttributionDecision{}, lockErr
	}
	rows, queryErr := tx.GetAll(`SELECT attribution_no FROM plugin_idc_core_sales_affiliate_attribution WHERE tenant_id=0 AND referred_customer_no=? AND status_code='eligible' LIMIT 1 FOR UPDATE`, request.ReferredCustomerNo)
	if queryErr != nil {
		return AttributionDecision{}, queryErr
	}
	if len(rows) > 0 {
		decision.StatusCode = AffiliateRejectedRisk
		decision.RiskReasonCode = AffiliateReasonDuplicate
		return decision, nil
	}
	_, insertErr := tx.Model(dao.SalesAffiliateAttribution.Table()).Data(do.SalesAffiliateAttribution{
		TenantId: 0, AttributionNo: attributionNo, LinkNo: request.Link.LinkNo,
		TouchNo: request.Touch.TouchNo, ReferrerCustomerNo: request.Link.OwnerCustomerNo,
		ReferredCustomerNo: request.ReferredCustomerNo, PolicyCode: request.Link.AttributionPolicyCode,
		StatusCode: AffiliateEligible, EvidenceDigest: evidence,
		AttributedAt: &attributedAt, ExpiresAt: expiresAt,
	}).Insert()
	return decision, insertErr
}

func loadAffiliateIdentityRelation(tx gdb.TX, ownerCustomerNo, referredCustomerNo string) (*affiliateIdentityRelation, error) {
	if ownerCustomerNo == referredCustomerNo {
		return &affiliateIdentityRelation{SubjectKind: "customer", SubjectDigest: ownerCustomerNo, RelatedAccountCount: 1}, nil
	}
	var relation *affiliateIdentityRelation
	err := tx.GetScan(&relation, `WITH pair_profiles AS (
 SELECT customer_no, 'individual' AS subject_kind, identity_lookup_digest AS subject_digest
 FROM plugin_idc_core_customer_individual_profile
 WHERE tenant_id=0 AND customer_no IN (?,?) AND identity_status_code='verified' AND identity_lookup_digest<>''
 UNION ALL
 SELECT customer_no, 'enterprise', registration_lookup_digest
 FROM plugin_idc_core_customer_enterprise_profile
 WHERE tenant_id=0 AND customer_no IN (?,?) AND verification_status_code='verified' AND registration_lookup_digest<>''
), shared_subject AS (
 SELECT subject_kind, subject_digest FROM pair_profiles
 GROUP BY subject_kind, subject_digest HAVING count(DISTINCT customer_no)=2
), related_accounts AS (
 SELECT profile.customer_no, shared.subject_kind, shared.subject_digest
 FROM shared_subject shared JOIN plugin_idc_core_customer_individual_profile profile
   ON shared.subject_kind='individual' AND profile.tenant_id=0
  AND profile.identity_status_code='verified' AND profile.identity_lookup_digest=shared.subject_digest
 UNION ALL
 SELECT profile.customer_no, shared.subject_kind, shared.subject_digest
 FROM shared_subject shared JOIN plugin_idc_core_customer_enterprise_profile profile
   ON shared.subject_kind='enterprise' AND profile.tenant_id=0
  AND profile.verification_status_code='verified' AND profile.registration_lookup_digest=shared.subject_digest
)
SELECT subject_kind, subject_digest, CAST(count(DISTINCT customer_no) AS INTEGER) AS related_account_count
FROM related_accounts GROUP BY subject_kind, subject_digest ORDER BY subject_kind LIMIT 1`,
		ownerCustomerNo, referredCustomerNo, ownerCustomerNo, referredCustomerNo)
	if err != nil || relation == nil || relation.SubjectDigest == "" {
		return nil, err
	}
	if relation.RelatedAccountCount < 2 {
		relation.RelatedAccountCount = 2
	}
	return relation, nil
}

func persistRejectedSelfReferral(
	tx gdb.TX,
	request AttributionRequest,
	decision AttributionDecision,
	relation *affiliateIdentityRelation,
	attributionNo string,
	evidence string,
	attributedAt time.Time,
	expiresAt *time.Time,
) error {
	if _, lockErr := tx.Exec(`SELECT pg_advisory_xact_lock(hashtextextended(?,0))`, attributionNo); lockErr != nil {
		return lockErr
	}
	existing, queryErr := tx.GetAll(`SELECT attribution_no FROM plugin_idc_core_sales_affiliate_attribution WHERE tenant_id=0 AND attribution_no=? LIMIT 1 FOR UPDATE`, attributionNo)
	if queryErr != nil {
		return queryErr
	}
	if len(existing) == 0 {
		riskReason := decision.RiskReasonCode
		if _, insertErr := tx.Model(dao.SalesAffiliateAttribution.Table()).Data(do.SalesAffiliateAttribution{
			TenantId: 0, AttributionNo: attributionNo, LinkNo: request.Link.LinkNo,
			TouchNo: request.Touch.TouchNo, ReferrerCustomerNo: request.Link.OwnerCustomerNo,
			ReferredCustomerNo: request.ReferredCustomerNo, PolicyCode: request.Link.AttributionPolicyCode,
			StatusCode: AffiliateRejectedSelfReferral, RiskReasonCode: &riskReason,
			EvidenceDigest: evidence, AttributedAt: &attributedAt, ExpiresAt: expiresAt,
		}).Insert(); insertErr != nil {
			return insertErr
		}
	}

	var (
		subjectKind         = "customer"
		subjectDigest       = request.ReferredCustomerNo
		relatedAccountCount = 1
		evidenceSummary     = "same_customer_self_referral"
	)
	if relation != nil && relation.RelatedAccountCount > 1 {
		subjectKind = relation.SubjectKind
		subjectDigest = relation.SubjectDigest
		relatedAccountCount = relation.RelatedAccountCount
		evidenceSummary = "verified_subject_self_referral"
	}
	caseHash := sha256.Sum256([]byte(fmt.Sprintf("affiliate-self-referral:v1:%s:%s:%s:%s:%s:%s", request.Link.LinkNo, request.Touch.TouchNo, request.Link.OwnerCustomerNo, request.ReferredCustomerNo, subjectKind, subjectDigest)))
	caseEvidence := hex.EncodeToString(caseHash[:])
	caseNo := "mab_" + caseEvidence[:32]
	_, caseErr := tx.Exec(`INSERT INTO plugin_idc_core_marketing_abuse_case
 (tenant_id, case_no, campaign_no, customer_no, signal_code, related_account_count,
  evidence_summary_code, evidence_digest, window_started_at, window_ended_at, observed_count)
VALUES (0, ?, NULL, ?, 'self_referral', ?, ?, ?, ?, ?, 1)
ON CONFLICT DO NOTHING`, caseNo, request.ReferredCustomerNo, relatedAccountCount,
		evidenceSummary, caseEvidence, request.Touch.OccurredAt.UTC(), request.Now.UTC())
	return caseErr
}
