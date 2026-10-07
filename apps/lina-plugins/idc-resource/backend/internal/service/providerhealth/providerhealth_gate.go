// This file implements bounded database-authoritative Provider write gates.

package providerhealth

import (
	"context"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
	"lina-plugin-idc-resource/backend/internal/dao"
	"lina-plugin-idc-resource/backend/internal/model/do"
	"lina-plugin-idc-resource/backend/internal/model/entity"
)

// CheckWrites reads one bounded set of PostgreSQL-clock protection decisions.
// Expired circuits are reset together, so an execution batch never creates a
// per-task or per-channel database query path.
func (s *serviceImpl) CheckWrites(
	ctx context.Context,
	channelNos []string,
) (map[string]Gate, error) {
	unique, err := validateGateChannels(channelNos)
	if s == nil || ctx == nil || ctx.Err() != nil || err != nil {
		return nil, gerror.New("resource Provider health gates are unavailable")
	}
	gates := make(map[string]Gate, len(unique))
	if len(unique) == 0 {
		return gates, nil
	}
	err = dao.ChannelHealthState.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		now, timeErr := databaseTime(ctx)
		if timeErr != nil {
			return timeErr
		}
		if resetErr := resetExpiredCircuits(ctx, unique, now); resetErr != nil {
			return resetErr
		}
		columns := dao.ChannelHealthState.Columns()
		var rows []entity.ChannelHealthState
		if queryErr := dao.ChannelHealthState.Ctx(ctx).
			Fields(columns.ChannelNo, columns.WriteProtectedUntil).
			Where(do.ChannelHealthState{
				TenantId: idccap.DefaultTenantID, CircuitOpen: true,
			}).
			WhereIn(columns.ChannelNo, unique).
			WhereGT(columns.WriteProtectedUntil, now).
			Limit(len(unique)).
			Scan(&rows); queryErr != nil {
			return gerror.Wrap(queryErr, "load resource Provider write protections")
		}
		for index := range rows {
			if rows[index].WriteProtectedUntil != nil {
				gates[rows[index].ChannelNo] = Gate{
					ProtectedUntil: rows[index].WriteProtectedUntil.UTC(),
				}
			}
		}
		return nil
	})
	if err != nil {
		return nil, gerror.Wrap(err, "check resource Provider write protections")
	}
	return gates, nil
}

// CheckWrite is the single-channel convenience boundary used by diagnostics.
func (s *serviceImpl) CheckWrite(ctx context.Context, channelNo string) (Gate, error) {
	gates, err := s.CheckWrites(ctx, []string{channelNo})
	if err != nil {
		return Gate{}, err
	}
	return gates[channelNo], nil
}

func validateGateChannels(channelNos []string) ([]string, error) {
	if len(channelNos) > maximumGateChannelCount {
		return nil, gerror.New("resource Provider health gate batch is too large")
	}
	seen := make(map[string]struct{}, len(channelNos))
	unique := make([]string, 0, len(channelNos))
	for _, channelNo := range channelNos {
		if resourcecap.ValidateChannelRef(channelNo) != nil {
			return nil, gerror.New("resource Provider health channel is invalid")
		}
		if _, exists := seen[channelNo]; exists {
			continue
		}
		seen[channelNo] = struct{}{}
		unique = append(unique, channelNo)
	}
	return unique, nil
}

func resetExpiredCircuits(
	ctx context.Context,
	channelNos []string,
	now time.Time,
) error {
	columns := dao.ChannelHealthState.Columns()
	if _, err := dao.ChannelHealthState.Ctx(ctx).
		Where(do.ChannelHealthState{TenantId: idccap.DefaultTenantID, CircuitOpen: true}).
		WhereIn(columns.ChannelNo, channelNos).
		WhereNotNull(columns.WriteProtectedUntil).
		WhereLTE(columns.WriteProtectedUntil, now).
		Data(do.ChannelHealthState{
			SampleCount: 0, FailureCount: 0, UnknownCount: 0, ConsecutiveFailures: 0,
			CircuitOpen: false, WindowStartedAt: &now, LastSummaryCode: "",
			Version: gdb.Raw(columns.Version + " + 1"),
		}).Update(); err != nil {
		return gerror.Wrap(err, "reset expired resource Provider protections")
	}
	// Generated nullable pointer fields omit nil, so use the existing fixed
	// column/value update form for this narrow SQL NULL transition.
	_, err := dao.ChannelHealthState.Ctx(ctx).
		Where(do.ChannelHealthState{TenantId: idccap.DefaultTenantID, CircuitOpen: false}).
		WhereIn(columns.ChannelNo, channelNos).
		Data(columns.WriteProtectedUntil, nil).
		Update()
	if err != nil {
		return gerror.Wrap(err, "clear expired resource Provider protection deadlines")
	}
	return nil
}

func databaseTime(ctx context.Context) (time.Time, error) {
	value, err := dao.ChannelHealthState.DB().GetValue(ctx, "SELECT CURRENT_TIMESTAMP")
	if err != nil {
		return time.Time{}, gerror.Wrap(err, "read resource Provider health database time")
	}
	now := value.Time()
	if now.IsZero() {
		return time.Time{}, gerror.New("resource Provider health database time is unavailable")
	}
	return now.UTC(), nil
}
