package accessorydirectory

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"reflect"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
)

const (
	refreshLeaseDuration = 2 * time.Minute
	refreshCallTimeout   = 90 * time.Second
	refreshRetryMinimum  = 30 * time.Second
	refreshRetryMaximum  = 15 * time.Minute
	unsupportedRetry     = time.Hour
)

type ProviderConstructor interface {
	Construct(context.Context, string) (resourcecap.Provider, error)
}

type providerSessionConstructor interface {
	ConstructSession(context.Context, string) (resourcecap.Provider, func(), error)
}

type Worker interface {
	RunOne(context.Context) (bool, error)
}

type worker struct {
	database  gdb.DB
	providers ProviderConstructor
	directory *serviceImpl
	random    io.Reader
}

type refreshClaim struct {
	MappingNo           string `orm:"mapping_no"`
	LocalResourceNo     string `orm:"local_resource_no"`
	ChannelNo           string `orm:"channel_no"`
	ExternalResourceRef string `orm:"external_resource_ref"`
	AttemptCount        int    `orm:"attempt_count"`
	leaseDigest         string
}

func NewWorker(database gdb.DB, providers ProviderConstructor) (Worker, error) {
	if database == nil || nilProviderConstructor(providers) {
		return nil, gerror.New("resource accessory refresh worker dependencies are invalid")
	}
	return &worker{database: database, providers: providers,
		directory: &serviceImpl{database: database, random: rand.Reader}, random: rand.Reader}, nil
}

func (w *worker) RunOne(ctx context.Context) (bool, error) {
	if w == nil || ctx == nil || ctx.Err() != nil {
		return false, gerror.New("resource accessory refresh worker is unavailable")
	}
	claim, err := w.claimOne(ctx)
	if err != nil || claim == nil {
		return false, err
	}
	provider, cleanup, err := w.constructProvider(ctx, claim.ChannelNo)
	if err != nil {
		return true, w.retry(ctx, claim, "provider_unavailable", false)
	}
	defer cleanup()
	accessories, ok := provider.(resourcecap.AccessoryProvider)
	if !ok {
		return true, w.retry(ctx, claim, "capability_unsupported", true)
	}
	callCtx, cancel := context.WithTimeout(ctx, refreshCallTimeout)
	snapshot, callErr := accessories.QueryAccessories(callCtx, resourcecap.AccessoryQuery{ResourceRef: claim.ExternalResourceRef})
	cancel()
	if callErr != nil || snapshot == nil {
		return true, w.retry(ctx, claim, "provider_read_failed", false)
	}
	if _, err = w.directory.publish(ctx, PublishInput{
		MappingNo: claim.MappingNo, LocalResourceNo: claim.LocalResourceNo, ChannelNo: claim.ChannelNo,
		ExternalResourceRef: claim.ExternalResourceRef, Snapshot: *snapshot,
	}, claim.leaseDigest); err != nil {
		if errors.Is(err, ErrDirectoryUnavailable) {
			return true, nil
		}
		return true, err
	}
	return true, w.complete(ctx, claim, snapshot.ExpiresAt)
}

func (w *worker) constructProvider(ctx context.Context, channelNo string) (resourcecap.Provider, func(), error) {
	if sessions, ok := w.providers.(providerSessionConstructor); ok {
		provider, cleanup, err := sessions.ConstructSession(ctx, channelNo)
		if err != nil || provider == nil || cleanup == nil {
			return nil, nil, gerror.New("resource accessory Provider construction failed")
		}
		return provider, cleanup, nil
	}
	provider, err := w.providers.Construct(ctx, channelNo)
	if err != nil || provider == nil {
		return nil, nil, gerror.New("resource accessory Provider construction failed")
	}
	return provider, func() {}, nil
}

func (w *worker) claimOne(ctx context.Context) (*refreshClaim, error) {
	var claim *refreshClaim
	err := w.database.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		if _, err := tx.Exec(`INSERT INTO plugin_idc_resource_accessory_refresh_state (mapping_no)
SELECT mapping.mapping_no FROM plugin_idc_resource_mapping mapping
JOIN plugin_idc_resource_channel channel ON channel.channel_no=mapping.channel_no
WHERE mapping.tenant_id=? AND mapping.status_code='active' AND mapping.external_resource_ref<>''
 AND channel.status_code='enabled' AND channel.deleted_at IS NULL
ON CONFLICT (mapping_no) DO NOTHING`, idccap.DefaultTenantID); err != nil {
			return gerror.Wrap(err, "seed resource accessory refresh state")
		}
		var row *refreshClaim
		if err := tx.GetScan(&row, `SELECT refresh.mapping_no,mapping.local_resource_no,mapping.channel_no,
 mapping.external_resource_ref,refresh.attempt_count
FROM plugin_idc_resource_accessory_refresh_state refresh
JOIN plugin_idc_resource_mapping mapping ON mapping.mapping_no=refresh.mapping_no
JOIN plugin_idc_resource_channel channel ON channel.channel_no=mapping.channel_no
WHERE refresh.tenant_id=? AND refresh.next_attempt_at<=CURRENT_TIMESTAMP
 AND (refresh.lease_expires_at IS NULL OR refresh.lease_expires_at<=CURRENT_TIMESTAMP)
 AND mapping.status_code='active' AND mapping.external_resource_ref<>''
 AND channel.status_code='enabled' AND channel.deleted_at IS NULL
ORDER BY refresh.next_attempt_at,refresh.id LIMIT 1 FOR UPDATE OF refresh SKIP LOCKED`, idccap.DefaultTenantID); err != nil {
			return gerror.Wrap(err, "claim resource accessory refresh")
		}
		if row == nil {
			return nil
		}
		digest, err := newRefreshLeaseDigest(w.random)
		if err != nil {
			return err
		}
		if _, err = tx.Exec(`UPDATE plugin_idc_resource_accessory_refresh_state
SET attempt_count=LEAST(attempt_count+1,1000000),lease_token_digest=?,
 lease_expires_at=CURRENT_TIMESTAMP+INTERVAL '2 minutes',updated_at=CURRENT_TIMESTAMP
WHERE tenant_id=? AND mapping_no=?`, digest, idccap.DefaultTenantID, row.MappingNo); err != nil {
			return gerror.Wrap(err, "persist resource accessory refresh lease")
		}
		row.AttemptCount++
		row.leaseDigest = digest
		claim = row
		return nil
	})
	return claim, err
}

func (w *worker) retry(ctx context.Context, claim *refreshClaim, failureCode string, unsupported bool) error {
	delay := unsupportedRetry
	if !unsupported {
		delay = refreshRetryMinimum << min(claim.AttemptCount-1, 5)
		if delay > refreshRetryMaximum {
			delay = refreshRetryMaximum
		}
	}
	return w.release(ctx, claim, delay, failureCode, false)
}

func (w *worker) complete(ctx context.Context, claim *refreshClaim, expiresAt time.Time) error {
	nowValue, err := w.database.GetValue(ctx, "SELECT CURRENT_TIMESTAMP")
	if err != nil {
		return gerror.Wrap(err, "read resource accessory completion time")
	}
	now := nowValue.Time().UTC()
	next := expiresAt.UTC().Add(-time.Minute)
	if next.Before(now.Add(refreshRetryMinimum)) {
		next = now.Add(refreshRetryMinimum)
	}
	return w.release(ctx, claim, next.Sub(now), "", true)
}

func (w *worker) release(ctx context.Context, claim *refreshClaim, delay time.Duration, failureCode string, reset bool) error {
	attemptExpression := "attempt_count"
	if reset {
		attemptExpression = "0"
	}
	result, err := w.database.Exec(ctx, `UPDATE plugin_idc_resource_accessory_refresh_state
SET next_attempt_at=CURRENT_TIMESTAMP+(? * INTERVAL '1 microsecond'),attempt_count=`+attemptExpression+`,
 lease_token_digest=NULL,lease_expires_at=NULL,failure_code=?,updated_at=CURRENT_TIMESTAMP
WHERE tenant_id=? AND mapping_no=? AND lease_token_digest=?`, delay.Microseconds(), failureCode,
		idccap.DefaultTenantID, claim.MappingNo, claim.leaseDigest)
	if err != nil {
		return gerror.Wrap(err, "release resource accessory refresh lease")
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected > 1 {
		return gerror.New("resource accessory refresh lease transition is not singular")
	}
	return nil
}

func newRefreshLeaseDigest(source io.Reader) (string, error) {
	var value [32]byte
	if _, err := io.ReadFull(source, value[:]); err != nil {
		return "", err
	}
	digest := sha256.Sum256(value[:])
	return hex.EncodeToString(digest[:]), nil
}

func nilProviderConstructor(value ProviderConstructor) bool {
	if value == nil {
		return true
	}
	reflected := reflect.ValueOf(value)
	switch reflected.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return reflected.IsNil()
	default:
		return false
	}
}
