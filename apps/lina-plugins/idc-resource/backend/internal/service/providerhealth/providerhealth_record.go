// This file atomically records normalized Provider health observations.

package providerhealth

import (
	"context"
	"encoding/hex"
	"io"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-resource/backend/internal/dao"
	"lina-plugin-idc-resource/backend/internal/model/do"
)

// Record appends one immutable observation and advances the bounded write
// window in one transaction. Protection uses the PostgreSQL clock only.
func (s *serviceImpl) Record(ctx context.Context, input Observation) error {
	if s == nil || ctx == nil || ctx.Err() != nil ||
		validateObservation(input) != nil {
		return gerror.New("resource Provider health observation is invalid")
	}
	return dao.ChannelHealthState.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		now, err := databaseTime(ctx)
		if err != nil {
			return err
		}
		row, err := loadHealthState(ctx, input.ChannelNo)
		if err != nil {
			return err
		}
		if row == nil {
			if _, err = dao.ChannelHealthState.Ctx(ctx).
				Data(do.ChannelHealthState{
					TenantId: idccap.DefaultTenantID, ChannelNo: input.ChannelNo,
					WindowStartedAt: &now,
				}).InsertIgnore(); err != nil {
				return gerror.Wrap(err, "initialize resource Provider health state")
			}
			row, err = loadHealthState(ctx, input.ChannelNo)
			if err != nil || row == nil {
				return gerror.Wrap(err, "load initialized resource Provider health state")
			}
		}
		resetExpiredState(row, now)
		if input.Operation == OperationWrite {
			applyObservation(row, input, now)
		}
		columns := dao.ChannelHealthState.Columns()
		if _, err = dao.ChannelHealthState.Ctx(ctx).
			Where(do.ChannelHealthState{
				TenantId: idccap.DefaultTenantID, ChannelNo: input.ChannelNo,
			}).Data(do.ChannelHealthState{
			SampleCount: row.SampleCount, FailureCount: row.FailureCount,
			UnknownCount: row.UnknownCount, ConsecutiveFailures: row.ConsecutiveFailures,
			CircuitOpen: row.CircuitOpen, WriteProtectedUntil: row.WriteProtectedUntil,
			WindowStartedAt: row.WindowStartedAt, LastSummaryCode: input.SummaryCode,
			LastObservedAt: &now, Version: gdb.Raw(columns.Version + " + 1"),
		}).Update(); err != nil {
			return gerror.Wrap(err, "advance resource Provider health state")
		}
		if row.WriteProtectedUntil == nil {
			if _, err = dao.ChannelHealthState.Ctx(ctx).
				Where(do.ChannelHealthState{
					TenantId: idccap.DefaultTenantID, ChannelNo: input.ChannelNo,
				}).
				Data(columns.WriteProtectedUntil, nil).
				Update(); err != nil {
				return gerror.Wrap(err, "clear resource Provider protection deadline")
			}
		}
		healthCode, err := insertSnapshot(ctx, input, row, now, s.random)
		if err != nil {
			return err
		}
		if err = updateLatestProjection(ctx, input, healthCode, now); err != nil {
			return err
		}
		if input.AuthenticationFailure {
			authentication := input
			authentication.Operation = OperationAuthentication
			authentication.AuthenticationFailure = false
			authenticationCode, snapshotErr := insertSnapshot(ctx, authentication, row, now, s.random)
			if snapshotErr != nil {
				return snapshotErr
			}
			return updateLatestProjection(ctx, authentication, authenticationCode, now)
		}
		return nil
	})
}

func insertSnapshot(
	ctx context.Context,
	input Observation,
	row *healthState,
	now time.Time,
	random io.Reader,
) (string, error) {
	latencyMS := input.Latency.Milliseconds()
	healthCode := HealthHealthy
	if input.Outcome != OutcomeSucceeded {
		healthCode = HealthDegraded
	}
	if input.Operation == OperationWrite && row.CircuitOpen {
		healthCode = HealthUnavailable
	}
	var identity [28]byte
	if _, err := io.ReadFull(random, identity[:]); err != nil {
		return "", gerror.Wrap(err, "generate resource Provider health snapshot identity")
	}
	snapshotNo := "health_" + hex.EncodeToString(identity[:])
	_, err := dao.HealthSnapshot.Ctx(ctx).Data(do.HealthSnapshot{
		TenantId: idccap.DefaultTenantID, SnapshotNo: snapshotNo, ChannelNo: input.ChannelNo,
		OperationKind: input.Operation, HealthCode: healthCode, LatencyMs: latencyMS,
		ConsecutiveFailures: row.ConsecutiveFailures, CircuitOpen: row.CircuitOpen,
		SummaryCode: input.SummaryCode, ObservedAt: &now,
	}).Insert()
	if err != nil {
		return "", gerror.Wrap(err, "insert resource Provider health snapshot")
	}
	return string(healthCode), nil
}

// updateLatestProjection advances exactly one operation-kind projection.
func updateLatestProjection(
	ctx context.Context,
	input Observation,
	healthCode string,
	now time.Time,
) error {
	latencyMS := input.Latency.Milliseconds()
	var data do.ChannelHealthState
	switch input.Operation {
	case OperationAuthentication:
		data = do.ChannelHealthState{
			LastAuthenticationHealthCode:  healthCode,
			LastAuthenticationSummaryCode: input.SummaryCode,
			LastAuthenticationLatencyMs:   latencyMS,
			LastAuthenticationObservedAt:  &now,
		}
	case OperationQuery:
		data = do.ChannelHealthState{
			LastQueryHealthCode:  healthCode,
			LastQuerySummaryCode: input.SummaryCode,
			LastQueryLatencyMs:   latencyMS,
			LastQueryObservedAt:  &now,
		}
	case OperationWrite:
		data = do.ChannelHealthState{
			LastWriteHealthCode:  healthCode,
			LastWriteSummaryCode: input.SummaryCode,
			LastWriteLatencyMs:   latencyMS,
			LastWriteObservedAt:  &now,
		}
	case OperationProviderCapacity:
		data = do.ChannelHealthState{
			LastProviderCapacityHealthCode:  healthCode,
			LastProviderCapacitySummaryCode: input.SummaryCode,
			LastProviderCapacityLatencyMs:   latencyMS,
			LastProviderCapacityObservedAt:  &now,
		}
	default:
		return gerror.New("resource Provider health projection operation is invalid")
	}
	_, err := dao.ChannelHealthState.Ctx(ctx).
		Where(do.ChannelHealthState{TenantId: idccap.DefaultTenantID, ChannelNo: input.ChannelNo}).
		Data(data).Update()
	if err != nil {
		return gerror.Wrap(err, "update resource Provider health projection")
	}
	return nil
}
