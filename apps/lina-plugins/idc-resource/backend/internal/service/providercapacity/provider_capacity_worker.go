// This file implements database-clock claiming and exactly-one-page providerCapacity work.
package providercapacity

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"io"
	"reflect"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
	"lina-plugin-idc-resource/backend/internal/dao"
	"lina-plugin-idc-resource/backend/internal/model/do"
	"lina-plugin-idc-resource/backend/internal/model/entity"
	"lina-plugin-idc-resource/backend/internal/service/providerhealth"
)

const (
	MaximumWorkerAttempts = 8
	DefaultLeaseDuration  = 2 * time.Minute
	providerPageTimeout   = 90 * time.Second
	minimumRetryDelay     = 30 * time.Second
	maximumRetryDelay     = 15 * time.Minute
)

type ProviderConstructor interface {
	Construct(ctx context.Context, channelRef string) (resourcecap.Provider, error)
}

type providerSessionConstructor interface {
	ConstructSession(ctx context.Context, channelRef string) (resourcecap.Provider, func(), error)
}

type Worker interface {
	// RunOne claims at most one due observation and requests exactly one Provider page.
	RunOne(ctx context.Context) (worked bool, err error)
}

type worker struct {
	database     gdb.DB
	providers    ProviderConstructor
	health       providerhealth.Service
	observations *serviceImpl
	random       io.Reader
	leasePeriod  time.Duration
}

type claimedObservation struct {
	ObservationNo string
	ChannelNo     string
	Kind          resourcecap.ProviderCapacityKind
	PageSize      int
	Cursor        string
	Attempt       int
	leaseDigest   string
}

func NewWorker(
	database gdb.DB,
	providers ProviderConstructor,
	health providerhealth.Service,
) (Worker, error) {
	if database == nil || isNilProviderConstructor(providers) || health == nil {
		return nil, gerror.New("resource providerCapacity worker dependencies are invalid")
	}
	return &worker{
		database: database, providers: providers, health: health,
		observations: &serviceImpl{database: database, random: rand.Reader},
		random:       rand.Reader, leasePeriod: DefaultLeaseDuration,
	}, nil
}

func (w *worker) RunOne(ctx context.Context) (bool, error) {
	if w == nil || ctx == nil || ctx.Err() != nil {
		return false, gerror.New("resource providerCapacity worker is unavailable")
	}
	claim, err := w.claimOne(ctx)
	if err != nil || claim == nil {
		return false, err
	}
	provider, cleanup, err := w.constructProvider(ctx, claim.ChannelNo)
	if err != nil {
		return true, w.persistCallFailure(ctx, claim, "provider_unavailable", true)
	}
	defer cleanup()
	request := resourcecap.ProviderCapacityRequest{
		Kind: claim.Kind, Cursor: claim.Cursor, Limit: claim.PageSize,
	}
	callCtx, cancel := context.WithTimeout(ctx, providerPageTimeout)
	startedAt := time.Now()
	page, callErr := provider.ListProviderCapacity(callCtx, request)
	latency := time.Since(startedAt)
	cancel()
	if callErr != nil {
		failure := resourcecap.NormalizeProviderError(callErr)
		if err = w.persistCallFailure(
			ctx, claim, "provider_"+string(failure.Code), retrySafeProviderCapacityRead(failure),
		); err != nil {
			return true, err
		}
		return true, w.recordProviderCapacityHealth(
			ctx, claim.ChannelNo, outcomeForProviderCapacityFailure(failure), string(failure.Code), latency,
		)
	}
	if page == nil {
		if err = w.persistCallFailure(ctx, claim, "provider_invalid_response", false); err != nil {
			return true, err
		}
		return true, w.recordProviderCapacityHealth(
			ctx, claim.ChannelNo, providerhealth.OutcomeResultUnknown,
			"invalid_response", latency,
		)
	}
	_, err = w.observations.Stage(ctx, StageInput{
		ObservationNo: claim.ObservationNo, Request: request, Page: *page,
		leaseDigest: claim.leaseDigest,
	})
	if err != nil {
		if errors.Is(err, ErrSynchronizationLeaseChanged) {
			return true, nil
		}
		if err = w.failLeased(ctx, claim, "provider_invalid_response"); err != nil {
			return true, err
		}
		return true, w.recordProviderCapacityHealth(
			ctx, claim.ChannelNo, providerhealth.OutcomeResultUnknown,
			"invalid_response", latency,
		)
	}
	return true, w.recordProviderCapacityHealth(
		ctx, claim.ChannelNo, providerhealth.OutcomeSucceeded, "succeeded", latency,
	)
}

func (w *worker) recordProviderCapacityHealth(
	ctx context.Context,
	channelNo string,
	outcome providerhealth.Outcome,
	summaryCode string,
	latency time.Duration,
) error {
	return w.health.Record(ctx, providerhealth.Observation{
		ChannelNo: channelNo, Operation: providerhealth.OperationProviderCapacity,
		Outcome: outcome, SummaryCode: summaryCode, Latency: latency,
		AuthenticationFailure: summaryCode ==
			string(resourcecap.ProviderCallErrorCodeAuthentication),
	})
}

func outcomeForProviderCapacityFailure(failure *resourcecap.ProviderCallError) providerhealth.Outcome {
	if failure == nil || failure.ResultUnknown {
		return providerhealth.OutcomeResultUnknown
	}
	return providerhealth.OutcomeFailed
}

func (w *worker) constructProvider(
	ctx context.Context,
	channelNo string,
) (resourcecap.Provider, func(), error) {
	if sessions, ok := w.providers.(providerSessionConstructor); ok {
		provider, cleanup, err := sessions.ConstructSession(ctx, channelNo)
		if err != nil || cleanup == nil {
			return nil, nil, gerror.New("resource providerCapacity Provider construction failed")
		}
		return provider, cleanup, nil
	}
	provider, err := w.providers.Construct(ctx, channelNo)
	return provider, func() {}, err
}

func (w *worker) claimOne(ctx context.Context) (*claimedObservation, error) {
	var claimed *claimedObservation
	err := w.database.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		now, err := databaseTime(tx)
		if err != nil {
			return err
		}
		columns := dao.ProviderCapacityObservation.Columns()
		var row entity.ProviderCapacityObservation
		err = tx.Model(dao.ProviderCapacityObservation.Table()).Safe().Ctx(ctx).
			Fields(
				columns.Id, columns.ObservationNo, columns.ChannelNo, columns.CapacityKind,
				columns.PageSize, columns.NextCursor, columns.AttemptCount,
			).
			Where(do.ProviderCapacityObservation{TenantId: idccap.DefaultTenantID, StatusCode: StatusCollecting}).
			WhereNotNull(columns.NextAttemptAt).
			WhereLTE(columns.NextAttemptAt, now).
			WhereLT(columns.AttemptCount, MaximumWorkerAttempts).
			Where("("+columns.LeaseExpiresAt+" IS NULL OR "+columns.LeaseExpiresAt+" <= ?)", now).
			OrderAsc(columns.NextAttemptAt).OrderAsc(columns.Id).
			Limit(1).LockUpdateSkipLocked().Scan(&row)
		if errors.Is(err, sql.ErrNoRows) || row.Id == 0 {
			return nil
		}
		if err != nil {
			return gerror.Wrap(err, "load due resource providerCapacity synchronization")
		}
		digest, err := newProviderCapacityLeaseDigest(w.random)
		if err != nil {
			return err
		}
		expiresAt := now.Add(w.leasePeriod)
		result, err := tx.Model(dao.ProviderCapacityObservation.Table()).Safe().Ctx(ctx).
			Where(do.ProviderCapacityObservation{
				TenantId: idccap.DefaultTenantID, Id: row.Id,
				ObservationNo: row.ObservationNo, StatusCode: StatusCollecting,
				AttemptCount: row.AttemptCount,
			}).Data(gdb.Map{
			"attempt_count":      gdb.Raw(columns.AttemptCount + " + 1"),
			"lease_token_digest": digest, "lease_expires_at": expiresAt,
			"next_attempt_at": nil, "failure_code": "",
		}).Update()
		if err != nil {
			return gerror.Wrap(err, "claim resource providerCapacity synchronization")
		}
		affected, err := result.RowsAffected()
		if err != nil || affected != 1 {
			return gerror.New("resource providerCapacity synchronization changed concurrently")
		}
		claimed = &claimedObservation{
			ObservationNo: row.ObservationNo, ChannelNo: row.ChannelNo,
			Kind: resourcecap.ProviderCapacityKind(row.CapacityKind), PageSize: row.PageSize,
			Cursor: row.NextCursor, Attempt: row.AttemptCount + 1,
			leaseDigest: digest,
		}
		return nil
	})
	if err != nil {
		return nil, gerror.Wrap(err, "claim resource providerCapacity synchronization")
	}
	return claimed, nil
}

func (w *worker) persistCallFailure(
	ctx context.Context,
	claim *claimedObservation,
	failureCode string,
	retryable bool,
) error {
	if retryable && claim.Attempt < MaximumWorkerAttempts {
		return w.retryLeased(ctx, claim, failureCode)
	}
	if claim.Attempt >= MaximumWorkerAttempts {
		failureCode = "attempt_limit_exceeded"
	}
	return w.failLeased(ctx, claim, failureCode)
}

func (w *worker) retryLeased(ctx context.Context, claim *claimedObservation, failureCode string) error {
	return w.database.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		now, err := databaseTime(tx)
		if err != nil {
			return err
		}
		delay := minimumRetryDelay << min(claim.Attempt-1, 5)
		if delay > maximumRetryDelay {
			delay = maximumRetryDelay
		}
		return updateLeasedObservation(ctx, tx, claim, gdb.Map{
			"next_attempt_at": now.Add(delay), "lease_token_digest": "",
			"lease_expires_at": nil, "failure_code": "",
		})
	})
}

func (w *worker) failLeased(ctx context.Context, claim *claimedObservation, failureCode string) error {
	return w.database.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		now, err := databaseTime(tx)
		if err != nil {
			return err
		}
		return updateLeasedObservation(ctx, tx, claim, gdb.Map{
			"status_code": StatusFailed, "completed_at": now, "next_attempt_at": nil,
			"lease_token_digest": "", "lease_expires_at": nil, "failure_code": failureCode,
		})
	})
}

func updateLeasedObservation(
	ctx context.Context,
	tx gdb.TX,
	claim *claimedObservation,
	data gdb.Map,
) error {
	result, err := tx.Model(dao.ProviderCapacityObservation.Table()).Safe().Ctx(ctx).
		Where(do.ProviderCapacityObservation{
			TenantId: idccap.DefaultTenantID, ObservationNo: claim.ObservationNo,
			StatusCode: StatusCollecting, AttemptCount: claim.Attempt,
			LeaseTokenDigest: claim.leaseDigest,
		}).Data(data).Update()
	if err != nil {
		return gerror.Wrap(err, "transition leased resource providerCapacity synchronization")
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return gerror.Wrap(err, "inspect resource providerCapacity synchronization transition")
	}
	if affected == 0 {
		return nil
	}
	if affected != 1 {
		return gerror.New("resource providerCapacity synchronization transition is not singular")
	}
	return nil
}

func retrySafeProviderCapacityRead(failure *resourcecap.ProviderCallError) bool {
	if failure == nil {
		return false
	}
	switch failure.Code {
	case resourcecap.ProviderCallErrorCodeTransport,
		resourcecap.ProviderCallErrorCodeTimeout,
		resourcecap.ProviderCallErrorCodeInternal,
		resourcecap.ProviderCallErrorCodeUnknown:
		return true
	default:
		return failure.Retryable
	}
}

func newProviderCapacityLeaseDigest(random io.Reader) (string, error) {
	var content [32]byte
	if _, err := io.ReadFull(random, content[:]); err != nil {
		return "", gerror.Wrap(err, "generate resource providerCapacity synchronization lease")
	}
	digest := sha256.Sum256(content[:])
	return hex.EncodeToString(digest[:]), nil
}

func isNilProviderConstructor(value ProviderConstructor) bool {
	if value == nil {
		return true
	}
	reflected := reflect.ValueOf(value)
	switch reflected.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map,
		reflect.Pointer, reflect.Slice:
		return reflected.IsNil()
	default:
		return false
	}
}
