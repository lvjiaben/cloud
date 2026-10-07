// This file implements the database-authoritative single-use proof fence.

package reauth

import (
	"context"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-plugin-idc-core/backend/internal/dao"
	"lina-plugin-idc-core/backend/internal/model/do"
)

// DAOReplayStore persists replay claims through the generated plugin DAO. It
// deliberately has no cache fallback: failure to prove a durable unique insert
// fails closed in the calling service.
type DAOReplayStore struct{}

// NewDAOReplayStore returns the production database replay authority.
func NewDAOReplayStore() ReplayStore {
	return &DAOReplayStore{}
}

// Claim atomically inserts one tenant/nonce digest. PostgreSQL ON CONFLICT DO
// NOTHING, emitted by InsertIgnore, converges concurrent consumers through the
// database unique constraint.
func (s *DAOReplayStore) Claim(ctx context.Context, claim ReplayClaim) (bool, error) {
	if s == nil || ctx == nil || !validReplayClaim(claim) {
		return false, gerror.New("recent authentication replay claim is invalid")
	}
	loginAt := databaseTime(claim.LoginAt)
	issuedAt := databaseTime(claim.IssuedAt)
	expiresAt := databaseTime(claim.ExpiresAt)
	result, err := dao.RecentAuthConsumption.Ctx(ctx).Data(do.RecentAuthConsumption{
		TenantId:      claim.TenantID,
		NonceDigest:   claim.NonceDigest,
		PurposeCode:   string(claim.Purpose),
		TargetDigest:  claim.TargetDigest,
		SessionDigest: claim.SessionDigest,
		UserId:        claim.UserID,
		LoginAt:       &loginAt,
		IssuedAt:      &issuedAt,
		ExpiresAt:     &expiresAt,
	}).InsertIgnore()
	if err != nil {
		return false, gerror.Wrap(err, "persist recent authentication replay claim")
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return false, gerror.Wrap(err, "read recent authentication replay claim result")
	}
	switch affected {
	case 0:
		return false, nil
	case 1:
		return true, nil
	default:
		return false, gerror.New("recent authentication replay claim affected an unexpected row count")
	}
}
