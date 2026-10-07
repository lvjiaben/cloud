// This file contains deterministic AFF attribution checks. Persistence remains
// append-only and is handled by the caller after the decision is accepted.
package saleschannel

import (
	"context"
	"sort"
	"time"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-plugin-idc-core/backend/internal/model/entity"
)

// AttributionTouch is the minimum privacy-safe projection needed to select a
// first-touch or last-touch winner. Raw device and IP values stay outside this
// package and are represented by their evidence digest in persistence.
type AttributionTouch struct {
	TouchNo    string
	OccurredAt time.Time
	ExpiresAt  *time.Time
}

type AttributionSelectionRequest struct {
	Now                time.Time
	PolicyCode         string
	OwnerCustomerNo    string
	ReferredCustomerNo string
	relatedIdentity    bool
	Touches            []AttributionTouch
	AlreadyAttributed  bool
}

type AttributionSelection struct {
	Allowed    bool
	TouchNo    string
	ReasonCode string
}

const (
	AffiliateReasonDuplicate       = "duplicate_attribution"
	AffiliateReasonNoValidTouch    = "no_valid_touch"
	AffiliateReasonTouchSetInvalid = "touch_set_invalid"
	maximumAttributionTouches      = 128
)

// SelectAttribution deterministically chooses one valid touch. The caller
// stores the selected result under a unique referred-customer constraint, so
// retries or concurrent registrations cannot silently replace an attribution.
func (s *serviceImpl) SelectAttribution(ctx context.Context, request AttributionSelectionRequest) (AttributionSelection, error) {
	if ctx == nil {
		return AttributionSelection{}, gerror.New("affiliate selection context is required")
	}
	if err := ctx.Err(); err != nil {
		return AttributionSelection{}, gerror.Wrap(err, "affiliate selection context is unavailable")
	}
	if request.Now.IsZero() || request.OwnerCustomerNo == "" || request.ReferredCustomerNo == "" ||
		request.OwnerCustomerNo == request.ReferredCustomerNo {
		return AttributionSelection{ReasonCode: AffiliateReasonSelfReferral}, nil
	}
	if request.relatedIdentity {
		return AttributionSelection{ReasonCode: AffiliateReasonRelatedIdentity}, nil
	}
	if request.AlreadyAttributed {
		return AttributionSelection{ReasonCode: AffiliateReasonDuplicate}, nil
	}
	if request.PolicyCode != AffiliateFirstTouch && request.PolicyCode != AffiliateLastTouch {
		return AttributionSelection{}, gerror.New("affiliate attribution policy is invalid")
	}
	if len(request.Touches) > maximumAttributionTouches {
		return AttributionSelection{ReasonCode: AffiliateReasonTouchSetInvalid}, nil
	}
	valid := make([]AttributionTouch, 0, len(request.Touches))
	seenTouches := make(map[string]struct{}, len(request.Touches))
	for _, touch := range request.Touches {
		if _, exists := seenTouches[touch.TouchNo]; exists {
			return AttributionSelection{ReasonCode: AffiliateReasonTouchSetInvalid}, nil
		}
		seenTouches[touch.TouchNo] = struct{}{}
		if touch.TouchNo == "" || touch.OccurredAt.IsZero() || touch.OccurredAt.After(request.Now) {
			continue
		}
		if touch.ExpiresAt != nil && !request.Now.Before(*touch.ExpiresAt) {
			continue
		}
		valid = append(valid, touch)
	}
	if len(valid) == 0 {
		return AttributionSelection{ReasonCode: AffiliateReasonNoValidTouch}, nil
	}
	sort.SliceStable(valid, func(i, j int) bool {
		if valid[i].OccurredAt.Equal(valid[j].OccurredAt) {
			return valid[i].TouchNo < valid[j].TouchNo
		}
		return valid[i].OccurredAt.Before(valid[j].OccurredAt)
	})
	selected := valid[0]
	if request.PolicyCode == AffiliateLastTouch {
		selected = valid[len(valid)-1]
	}
	return AttributionSelection{Allowed: true, TouchNo: selected.TouchNo}, nil
}

const (
	// AffiliateFirstTouch credits the earliest valid touch in the window.
	AffiliateFirstTouch = "first_touch"
	// AffiliateLastTouch credits the latest valid touch in the window.
	AffiliateLastTouch = "last_touch"
	// AffiliateEligible means the attribution can produce commission.
	AffiliateEligible = "eligible"
	// AffiliateLinkActive allows a referral link to receive touches.
	AffiliateLinkActive = "active"
	// AffiliateRejectedSelfReferral blocks an owner's self-referral.
	AffiliateRejectedSelfReferral = "rejected_self_referral"
	// AffiliateRejectedRisk blocks an attribution for a non-self-referral risk.
	AffiliateRejectedRisk = "rejected_risk"
	// AffiliateReasonSelfReferral is the auditable self-referral reason.
	AffiliateReasonSelfReferral = "self_referral"
	// AffiliateReasonRelatedIdentity is the auditable same verified-subject reason.
	AffiliateReasonRelatedIdentity = "same_identity"
	// AffiliateReasonTouchExpired is the auditable expired-touch reason.
	AffiliateReasonTouchExpired = "touch_expired"
	// AffiliateReasonLinkUnavailable is the auditable link-state reason.
	AffiliateReasonLinkUnavailable = "link_unavailable"
)

// AttributionRequest contains the authoritative link, touch and server-derived
// anti-fraud facts. Raw visitor, device and IP values are never stored.
type AttributionRequest struct {
	Now                time.Time
	Link               *entity.SalesAffiliateLink
	Touch              *entity.SalesAffiliateTouch
	ReferredCustomerNo string
	relatedIdentity    bool
}

// AttributionDecision is the append-only result to persist as one attribution
// row. Rejected decisions retain only a reason code and evidence digest.
type AttributionDecision struct {
	StatusCode         string
	RiskReasonCode     string
	PolicyCode         string
	ReferrerCustomerNo string
	ReferredCustomerNo string
}

// Attribute validates one AFF touch and preflights duplicate attribution when
// the service has a database. Persistence always repeats that check under lock.
func (s *serviceImpl) Attribute(ctx context.Context, request AttributionRequest) (AttributionDecision, error) {
	return s.attribute(ctx, request, true)
}

func (s *serviceImpl) attribute(ctx context.Context, request AttributionRequest, preflightDuplicateCheck bool) (AttributionDecision, error) {
	if ctx == nil {
		return AttributionDecision{}, gerror.New("affiliate attribution context is required")
	}
	if err := ctx.Err(); err != nil {
		return AttributionDecision{}, gerror.Wrap(err, "affiliate attribution context is unavailable")
	}
	if request.Now.IsZero() || request.Link == nil || request.Touch == nil || request.ReferredCustomerNo == "" {
		return AttributionDecision{}, gerror.New("affiliate attribution request is invalid")
	}
	if request.Link.LinkNo != request.Touch.LinkNo || request.Link.StatusCode != AffiliateLinkActive ||
		(request.Link.ExpiresAt != nil && !request.Now.Before(*request.Link.ExpiresAt)) {
		return AttributionDecision{StatusCode: AffiliateRejectedRisk, RiskReasonCode: AffiliateReasonLinkUnavailable, PolicyCode: request.Link.AttributionPolicyCode, ReferredCustomerNo: request.ReferredCustomerNo}, nil
	}
	if request.Touch.ExpiresAt == nil || !request.Now.Before(*request.Touch.ExpiresAt) {
		return AttributionDecision{StatusCode: AffiliateRejectedRisk, RiskReasonCode: AffiliateReasonTouchExpired, PolicyCode: request.Link.AttributionPolicyCode, ReferrerCustomerNo: request.Link.OwnerCustomerNo, ReferredCustomerNo: request.ReferredCustomerNo}, nil
	}
	if request.Touch.OccurredAt == nil || request.Touch.OccurredAt.IsZero() || request.Touch.OccurredAt.After(request.Now) {
		return AttributionDecision{StatusCode: AffiliateRejectedRisk, RiskReasonCode: AffiliateReasonTouchExpired, PolicyCode: request.Link.AttributionPolicyCode, ReferrerCustomerNo: request.Link.OwnerCustomerNo, ReferredCustomerNo: request.ReferredCustomerNo}, nil
	}
	if request.Link.AttributionWindowSeconds <= 0 || request.Touch.OccurredAt.Add(time.Duration(request.Link.AttributionWindowSeconds)*time.Second).Before(request.Now) {
		return AttributionDecision{StatusCode: AffiliateRejectedRisk, RiskReasonCode: AffiliateReasonTouchExpired, PolicyCode: request.Link.AttributionPolicyCode, ReferrerCustomerNo: request.Link.OwnerCustomerNo, ReferredCustomerNo: request.ReferredCustomerNo}, nil
	}
	if request.Link.OwnerCustomerNo == request.ReferredCustomerNo {
		return AttributionDecision{StatusCode: AffiliateRejectedSelfReferral, RiskReasonCode: AffiliateReasonSelfReferral, PolicyCode: request.Link.AttributionPolicyCode, ReferrerCustomerNo: request.Link.OwnerCustomerNo, ReferredCustomerNo: request.ReferredCustomerNo}, nil
	}
	if request.relatedIdentity {
		return AttributionDecision{StatusCode: AffiliateRejectedSelfReferral, RiskReasonCode: AffiliateReasonRelatedIdentity, PolicyCode: request.Link.AttributionPolicyCode, ReferrerCustomerNo: request.Link.OwnerCustomerNo, ReferredCustomerNo: request.ReferredCustomerNo}, nil
	}
	// A referred customer may have only one eligible attribution.  The partial
	// unique index protects concurrent writers; this read gives callers a clear
	// deterministic decision before they attempt the append-only insert.
	if preflightDuplicateCheck && s.database != nil {
		rows, err := s.database.GetAll(ctx, `SELECT 1 FROM plugin_idc_core_sales_affiliate_attribution WHERE tenant_id=0 AND referred_customer_no=? AND status_code='eligible' LIMIT 1`, request.ReferredCustomerNo)
		if err != nil {
			return AttributionDecision{}, gerror.Wrap(err, "check existing affiliate attribution")
		}
		if len(rows) > 0 {
			return AttributionDecision{StatusCode: AffiliateRejectedRisk, RiskReasonCode: AffiliateReasonDuplicate, PolicyCode: request.Link.AttributionPolicyCode, ReferrerCustomerNo: request.Link.OwnerCustomerNo, ReferredCustomerNo: request.ReferredCustomerNo}, nil
		}
	}
	if request.Link.AttributionPolicyCode != AffiliateFirstTouch && request.Link.AttributionPolicyCode != AffiliateLastTouch {
		return AttributionDecision{}, gerror.New("affiliate attribution policy is invalid")
	}
	return AttributionDecision{StatusCode: AffiliateEligible, PolicyCode: request.Link.AttributionPolicyCode, ReferrerCustomerNo: request.Link.OwnerCustomerNo, ReferredCustomerNo: request.ReferredCustomerNo}, nil
}
