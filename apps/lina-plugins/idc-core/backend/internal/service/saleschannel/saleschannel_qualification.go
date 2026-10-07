package saleschannel

import (
	"context"
	"strings"
	"time"

	"github.com/gogf/gf/v2/errors/gerror"
)

// QualificationTransitionRequest describes one append-only qualification
// change. The caller persists the event and projection in one transaction.
type QualificationTransitionRequest struct {
	Now            time.Time
	CurrentStatus  QualificationStatus
	NextStatus     QualificationStatus
	CurrentLevelNo string
	NextLevelNo    string
	EffectiveAt    time.Time
	ExpiresAt      *time.Time
	ReasonCode     string
	ActorReference string
	EvidenceDigest string
}

type QualificationTransitionDecision struct {
	Allowed    bool
	ReasonCode string
}

const (
	QualificationReasonInvalid             = "qualification_transition_invalid"
	QualificationReasonTerminal            = "qualification_terminal"
	QualificationReasonLevelMissing        = "qualification_level_missing"
	QualificationReasonExpiryInvalid       = "qualification_expiry_invalid"
	QualificationReasonEvidenceMissing     = "qualification_evidence_missing"
	QualificationReasonEffectiveDateFuture = "qualification_effective_date_future"
)

// ValidateQualificationTransition enforces a small, explicit state machine.
// Expired/revoked records are terminal; changing level or status never edits
// an old order snapshot and always requires a new evidence event.
func (s *serviceImpl) ValidateQualificationTransition(ctx context.Context, request QualificationTransitionRequest) (QualificationTransitionDecision, error) {
	if ctx == nil {
		return QualificationTransitionDecision{}, gerror.New("qualification transition context is required")
	}
	if err := ctx.Err(); err != nil {
		return QualificationTransitionDecision{}, gerror.Wrap(err, "qualification transition context is unavailable")
	}
	if request.Now.IsZero() || request.EffectiveAt.IsZero() || request.NextLevelNo == "" ||
		request.ReasonCode == "" || request.ActorReference == "" || !isLowerHexDigest(request.EvidenceDigest) {
		return QualificationTransitionDecision{ReasonCode: QualificationReasonInvalid}, nil
	}
	if request.EffectiveAt.After(request.Now) {
		return QualificationTransitionDecision{ReasonCode: QualificationReasonEffectiveDateFuture}, nil
	}
	if request.ExpiresAt != nil && !request.ExpiresAt.After(request.EffectiveAt) {
		return QualificationTransitionDecision{ReasonCode: QualificationReasonExpiryInvalid}, nil
	}
	if request.CurrentStatus == "" {
		if request.NextStatus != QualificationActive || request.CurrentLevelNo != "" {
			return QualificationTransitionDecision{ReasonCode: QualificationReasonInvalid}, nil
		}
		return QualificationTransitionDecision{Allowed: true}, nil
	}
	if request.CurrentLevelNo == "" {
		return QualificationTransitionDecision{ReasonCode: QualificationReasonLevelMissing}, nil
	}
	if request.CurrentStatus == QualificationExpired || request.CurrentStatus == QualificationRevoked {
		return QualificationTransitionDecision{ReasonCode: QualificationReasonTerminal}, nil
	}
	valid := map[QualificationStatus]bool{
		QualificationActive: true, QualificationSuspended: true,
		QualificationExpired: true, QualificationRevoked: true,
	}
	if !valid[request.CurrentStatus] || !valid[request.NextStatus] {
		return QualificationTransitionDecision{ReasonCode: QualificationReasonInvalid}, nil
	}
	if request.NextStatus == QualificationActive && request.ExpiresAt != nil && !request.Now.Before(*request.ExpiresAt) {
		return QualificationTransitionDecision{ReasonCode: QualificationReasonExpiryInvalid}, nil
	}
	if request.NextStatus == request.CurrentStatus && request.NextLevelNo == request.CurrentLevelNo {
		return QualificationTransitionDecision{ReasonCode: QualificationReasonInvalid}, nil
	}
	return QualificationTransitionDecision{Allowed: true}, nil
}

func isLowerHexDigest(value string) bool {
	if len(value) != 64 || strings.ToLower(value) != value {
		return false
	}
	for _, character := range value {
		if (character < '0' || character > '9') && (character < 'a' || character > 'f') {
			return false
		}
	}
	return true
}
