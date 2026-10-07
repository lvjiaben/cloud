// This file implements database claims, encrypted token publication, access, and invalidation.
package providerauthsession

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/cap/secretcap"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
)

const (
	tableName       = "plugin_idc_resource_provider_auth_session"
	refreshLease    = 150 * time.Second
	waitInterval    = 25 * time.Millisecond
	maximumWait     = 3 * time.Minute
	leaseTokenBytes = 32
)

type sessionStatus string

const (
	sessionStatusIdle       sessionStatus = "idle"
	sessionStatusRefreshing sessionStatus = "refreshing"
	sessionStatusReady      sessionStatus = "ready"
)

type sessionRow struct {
	StatusCode       sessionStatus `orm:"status_code"`
	Generation       int64         `orm:"generation"`
	SecretRef        string        `orm:"secret_ref"`
	ExpiresAt        *time.Time    `orm:"expires_at"`
	LeaseExpiresAt   *time.Time    `orm:"lease_expires_at"`
	LeaseTokenDigest string        `orm:"lease_token_digest"`
}

type claim struct {
	ready       bool
	generation  int64
	secretRef   string
	leaseDigest string
}

type databaseClock struct {
	Now time.Time `orm:"now"`
}

// Access loads a live encrypted token or claims the single database refresh
// lease, performs login outside the transaction, and publishes the new token.
func (s *serviceImpl) Access(
	ctx context.Context,
	request resourcecap.AuthenticationSessionRequest,
	produce resourcecap.AuthenticationTokenProducer,
	consume resourcecap.AuthenticationTokenConsumer,
) error {
	if s == nil || ctx == nil || ctx.Err() != nil || request.Validate() != nil ||
		request.ChannelRef != s.channelRef || produce == nil || consume == nil {
		return gerror.New("resource Provider authentication session access failed")
	}
	waitCtx, cancel := context.WithTimeout(ctx, maximumWait)
	defer cancel()
	for {
		selected, err := s.selectOrClaim(waitCtx)
		if err != nil {
			return gerror.Wrap(err, "resource Provider authentication session access failed")
		}
		if selected.ready {
			if err = s.consumeReady(waitCtx, selected, consume); err == nil {
				return nil
			}
			if invalidateErr := s.Invalidate(waitCtx, resourcecap.AuthenticationSessionInvalidation{
				ChannelRef: s.channelRef, Generation: selected.generation,
			}); invalidateErr != nil {
				return gerror.New("resource Provider authentication session access failed")
			}
			continue
		}
		if selected.leaseDigest != "" {
			return s.refresh(waitCtx, request, selected, produce, consume)
		}
		timer := time.NewTimer(waitInterval)
		select {
		case <-waitCtx.Done():
			timer.Stop()
			return gerror.New("resource Provider authentication session access failed")
		case <-timer.C:
		}
	}
}

// Invalidate clears only the exact generation rejected by the upstream.
func (s *serviceImpl) Invalidate(
	ctx context.Context,
	request resourcecap.AuthenticationSessionInvalidation,
) error {
	if s == nil || ctx == nil || ctx.Err() != nil || request.Validate() != nil ||
		request.ChannelRef != s.channelRef {
		return gerror.New("resource Provider authentication session invalidation failed")
	}
	_, err := s.database.Exec(ctx, `UPDATE `+tableName+`
SET status_code=?,expires_at=NULL,lease_token_digest=NULL,
    lease_expires_at=NULL,invalidated_at=CURRENT_TIMESTAMP,version=version+1
WHERE tenant_id=? AND channel_no=? AND status_code=? AND generation=?`,
		sessionStatusIdle, idccap.DefaultTenantID, s.channelRef, sessionStatusReady, request.Generation)
	if err != nil {
		return gerror.New("resource Provider authentication session invalidation failed")
	}
	return nil
}

func (s *serviceImpl) selectOrClaim(ctx context.Context) (claim, error) {
	var selected claim
	err := s.database.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		if _, err := tx.Exec(`INSERT INTO `+tableName+`
    (tenant_id,channel_no,status_code,generation,version)
VALUES (?,?,?,0,1) ON CONFLICT (channel_no) DO NOTHING`,
			idccap.DefaultTenantID, s.channelRef, sessionStatusIdle); err != nil {
			return err
		}
		var row sessionRow
		if err := tx.GetScan(&row, `SELECT status_code,generation,COALESCE(secret_ref,'') AS secret_ref,
       expires_at,lease_expires_at,COALESCE(lease_token_digest,'') AS lease_token_digest
FROM `+tableName+` WHERE tenant_id=? AND channel_no=? FOR UPDATE`,
			idccap.DefaultTenantID, s.channelRef); err != nil {
			return err
		}
		var clock databaseClock
		if err := tx.GetScan(&clock, `SELECT CURRENT_TIMESTAMP AS now`); err != nil {
			return err
		}
		databaseNow := clock.Now
		if row.StatusCode == sessionStatusReady && row.SecretRef != "" && row.ExpiresAt != nil && databaseNow.Before(*row.ExpiresAt) {
			selected = claim{ready: true, generation: row.Generation, secretRef: row.SecretRef}
			return nil
		}
		if row.StatusCode == sessionStatusRefreshing && row.LeaseExpiresAt != nil && databaseNow.Before(*row.LeaseExpiresAt) {
			return nil
		}
		leaseDigest, err := s.newLeaseDigest()
		if err != nil {
			return err
		}
		generation := row.Generation + 1
		result, err := tx.Exec(`UPDATE `+tableName+`
SET status_code=?,generation=?,expires_at=NULL,
    lease_token_digest=?,lease_expires_at=?,version=version+1
WHERE tenant_id=? AND channel_no=? AND generation=?`, sessionStatusRefreshing, generation, leaseDigest,
			databaseNow.Add(refreshLease), idccap.DefaultTenantID, s.channelRef, row.Generation)
		if err != nil {
			return err
		}
		affected, err := result.RowsAffected()
		if err != nil || affected != 1 {
			return gerror.New("resource Provider authentication refresh claim failed")
		}
		selected = claim{generation: generation, secretRef: row.SecretRef, leaseDigest: leaseDigest}
		return nil
	})
	return selected, err
}

func (s *serviceImpl) refresh(
	ctx context.Context,
	request resourcecap.AuthenticationSessionRequest,
	selected claim,
	produce resourcecap.AuthenticationTokenProducer,
	consume resourcecap.AuthenticationTokenConsumer,
) error {
	var plaintext []byte
	err := produce(func(token []byte) error {
		if len(token) == 0 || len(token) > resourcecap.MaximumAuthenticationTokenBytes || plaintext != nil {
			return gerror.New("resource Provider authentication token is invalid")
		}
		plaintext = append([]byte(nil), token...)
		return nil
	})
	if err != nil || len(plaintext) == 0 {
		clear(plaintext)
		if releaseErr := s.releaseFailed(ctx, selected); releaseErr != nil {
			return gerror.New("resource Provider authentication refresh failed")
		}
		return gerror.New("resource Provider authentication refresh failed")
	}
	defer clear(plaintext)
	secretRef, err := s.storeToken(ctx, selected, plaintext)
	if err != nil {
		if releaseErr := s.releaseFailed(ctx, selected); releaseErr != nil {
			return gerror.New("resource Provider authentication refresh failed")
		}
		return gerror.New("resource Provider authentication refresh failed")
	}
	var published bool
	err = s.database.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		var clock databaseClock
		if err := tx.GetScan(&clock, `SELECT CURRENT_TIMESTAMP AS now`); err != nil {
			return err
		}
		databaseNow := clock.Now
		result, err := tx.Exec(`UPDATE `+tableName+`
SET status_code=?,secret_ref=?,expires_at=?,lease_token_digest=NULL,
    lease_expires_at=NULL,invalidated_at=NULL,version=version+1
WHERE tenant_id=? AND channel_no=? AND status_code=? AND generation=?
  AND lease_token_digest=? AND lease_expires_at>=CURRENT_TIMESTAMP`,
			sessionStatusReady, secretRef, databaseNow.Add(request.TTL), idccap.DefaultTenantID,
			s.channelRef, sessionStatusRefreshing, selected.generation, selected.leaseDigest)
		if err != nil {
			return err
		}
		affected, err := result.RowsAffected()
		published = err == nil && affected == 1
		return err
	})
	if err != nil || !published {
		return gerror.New("resource Provider authentication refresh lost its lease")
	}
	return consume(plaintext, selected.generation)
}

func (s *serviceImpl) consumeReady(
	ctx context.Context,
	selected claim,
	consume resourcecap.AuthenticationTokenConsumer,
) error {
	handle, err := secretcap.NewHandle(selected.secretRef)
	if err != nil {
		return err
	}
	purpose, err := secretcap.NewPurpose("provider.authentication")
	if err != nil {
		return err
	}
	access, err := secretcap.NewAccessRequest(handle, purpose)
	if err != nil {
		return err
	}
	return s.accessor.Access(ctx, access, func(token []byte) error {
		if len(token) == 0 || len(token) > resourcecap.MaximumAuthenticationTokenBytes {
			return gerror.New("resource Provider authentication token is invalid")
		}
		return consume(token, selected.generation)
	})
}

func (s *serviceImpl) releaseFailed(ctx context.Context, selected claim) error {
	_, err := s.database.Exec(ctx, `UPDATE `+tableName+`
SET status_code=?,expires_at=NULL,lease_token_digest=NULL,
    lease_expires_at=NULL,version=version+1
WHERE tenant_id=? AND channel_no=? AND status_code=? AND generation=?
  AND lease_token_digest=?`, sessionStatusIdle, idccap.DefaultTenantID, s.channelRef,
		sessionStatusRefreshing, selected.generation, selected.leaseDigest)
	return err
}

func (s *serviceImpl) storeToken(ctx context.Context, selected claim, plaintext []byte) (string, error) {
	idempotencyKey := sessionIdempotencyKey(s.channelRef, selected.generation)
	producer := func(consume secretcap.Consumer) error { return consume(plaintext) }
	if selected.secretRef == "" {
		return s.createToken(ctx, idempotencyKey, producer)
	}
	handle, err := secretcap.NewHandle(selected.secretRef)
	if err != nil {
		return "", gerror.New("resource Provider authentication token storage failed")
	}
	metadata, err := s.manager.Metadata(ctx, handle)
	if err != nil {
		return "", gerror.New("resource Provider authentication token storage failed")
	}
	if !metadata.Configured {
		return s.createToken(ctx, idempotencyKey, producer)
	}
	if metadata.UpdatedAt.IsZero() {
		return "", gerror.New("resource Provider authentication token storage failed")
	}
	replaced, err := s.manager.Replace(ctx, handle, metadata.UpdatedAt, idempotencyKey, producer)
	if err != nil || !replaced.Metadata.Configured {
		return "", gerror.New("resource Provider authentication token storage failed")
	}
	return selected.secretRef, nil
}

func (s *serviceImpl) createToken(
	ctx context.Context,
	idempotencyKey string,
	producer secretcap.PlaintextProducer,
) (string, error) {
	created, err := s.manager.Create(ctx, idempotencyKey, producer)
	if err != nil || created.Handle.IsZero() {
		return "", gerror.New("resource Provider authentication token storage failed")
	}
	return created.Handle.OpaqueRef(), nil
}

func (s *serviceImpl) newLeaseDigest() (string, error) {
	raw := make([]byte, leaseTokenBytes)
	if _, err := io.ReadFull(s.random, raw); err != nil {
		clear(raw)
		return "", err
	}
	digest := sha256.Sum256(raw)
	clear(raw)
	return hex.EncodeToString(digest[:]), nil
}

func sessionIdempotencyKey(channelRef string, generation int64) string {
	digest := sha256.Sum256([]byte(channelRef))
	return fmt.Sprintf("provider-auth-%s-%d", hex.EncodeToString(digest[:16]), generation)
}
