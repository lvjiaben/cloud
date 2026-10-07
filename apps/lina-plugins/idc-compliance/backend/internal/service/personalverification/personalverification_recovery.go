// This file performs bounded restart-safe recovery without replaying provider initialization.

package personalverification

import (
	"context"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-compliance/backend/internal/dao"
	"lina-plugin-idc-compliance/backend/internal/model/do"
	"lina-plugin-idc-compliance/backend/internal/model/entity"
	"lina-plugin-idc-core/backend/cap/idccap"
)

// Recover performs a bounded, restart-safe scan. Initialization-unknown calls
// are never replayed; only already-issued certify IDs are actively queried.
func (s *serviceImpl) Recover(ctx context.Context, limit int) (*RecoveryResult, error) {
	if s == nil || ctx == nil || ctx.Err() != nil || limit <= 0 || limit > maximumRecoveryBatch {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	now := s.now().UTC()
	var attempts []*entity.PersonalVerificationAttempt
	columns := dao.PersonalVerificationAttempt.Columns()
	err := s.database.Model(dao.PersonalVerificationAttempt.Table()).Ctx(ctx).
		Fields(columns.AttemptNo, columns.StatusCode).
		Where(do.PersonalVerificationAttempt{TenantId: idccap.DefaultTenantID}).
		Where(
			"status_code IN(?,?,?) OR (status_code = ? AND initialization_started_at <= ?)",
			attemptStatusUnknown, attemptStatusReady, attemptStatusPending,
			attemptStatusPreparing, now.Add(-preparingTimeout),
		).
		OrderAsc(columns.Id).Limit(limit).Scan(&attempts)
	if err != nil {
		return nil, bizerr.WrapCode(err, CodeUnavailable)
	}
	result := &RecoveryResult{Selected: len(attempts)}
	var firstError error
	for _, attempt := range attempts {
		if attempt == nil || attempt.AttemptNo == "" {
			continue
		}
		if attemptStatusCode(attempt.StatusCode) == attemptStatusReady ||
			attemptStatusCode(attempt.StatusCode) == attemptStatusPending {
			result.Queried++
		}
		verification, queryErr := s.query(ctx, attempt.AttemptNo, "")
		if queryErr != nil || verification == nil {
			result.Pending++
			if queryErr != nil && firstError == nil {
				firstError = queryErr
			}
			continue
		}
		switch verification.Status {
		case StatusVerified:
			result.Verified++
		case StatusExpired:
			result.Expired++
		case StatusReviewRequired:
			result.ReviewRequired++
		default:
			result.Pending++
		}
	}
	return result, firstError
}
