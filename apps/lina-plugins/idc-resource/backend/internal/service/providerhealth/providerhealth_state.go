// This file validates and advances the bounded per-channel health window.

package providerhealth

import (
	"context"
	"database/sql"
	"errors"
	"regexp"
	"time"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
	"lina-plugin-idc-resource/backend/internal/dao"
	"lina-plugin-idc-resource/backend/internal/model/do"
	"lina-plugin-idc-resource/backend/internal/model/entity"
)

// summaryCodePattern permits only bounded machine codes in health storage.
var summaryCodePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$`)

type healthState = entity.ChannelHealthState

func loadHealthState(
	ctx context.Context,
	channelNo string,
) (*healthState, error) {
	var row healthState
	columns := dao.ChannelHealthState.Columns()
	err := dao.ChannelHealthState.Ctx(ctx).
		Fields(
			columns.SampleCount, columns.FailureCount, columns.UnknownCount,
			columns.ConsecutiveFailures, columns.CircuitOpen, columns.WriteProtectedUntil,
			columns.WindowStartedAt, columns.WindowSampleLimit,
			columns.MinimumSamplesToProtect, columns.FailureLimit,
			columns.UnknownResultLimit, columns.ConsecutiveFailureLimit,
			columns.ProtectionDurationSeconds, columns.PolicyVersion,
			columns.PolicyUpdatedAt,
		).
		Where(do.ChannelHealthState{TenantId: idccap.DefaultTenantID, ChannelNo: channelNo}).
		LockUpdate().
		Scan(&row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, gerror.Wrap(err, "load resource Provider health state")
	}
	if row.WindowStartedAt == nil || row.WindowStartedAt.IsZero() {
		return nil, nil
	}
	return &row, nil
}

func resetExpiredState(row *healthState, now time.Time) {
	if row.CircuitOpen && (row.WriteProtectedUntil == nil || !row.WriteProtectedUntil.After(now)) {
		row.SampleCount = 0
		row.FailureCount = 0
		row.UnknownCount = 0
		row.ConsecutiveFailures = 0
		row.CircuitOpen = false
		row.WriteProtectedUntil = nil
		row.WindowStartedAt = &now
	}
}

func applyObservation(row *healthState, input Observation, now time.Time) {
	if row.WindowSampleLimit == 0 {
		applyDefaultPolicy(row)
	}
	if row.SampleCount >= row.WindowSampleLimit {
		row.SampleCount = 0
		row.FailureCount = 0
		row.UnknownCount = 0
		row.WindowStartedAt = &now
	}
	row.SampleCount++
	if input.Outcome == OutcomeSucceeded {
		row.ConsecutiveFailures = 0
	} else {
		row.FailureCount++
		row.ConsecutiveFailures++
		if input.Outcome == OutcomeResultUnknown {
			row.UnknownCount++
		}
	}
	if input.Operation == OperationWrite && policyProtects(row) {
		until := now.Add(time.Duration(row.ProtectionDurationSeconds) * time.Second)
		row.CircuitOpen = true
		row.WriteProtectedUntil = &until
	}
}

func applyDefaultPolicy(row *healthState) {
	row.WindowSampleLimit = defaultWindowSampleLimit
	row.MinimumSamplesToProtect = defaultMinimumSamplesToProtect
	row.FailureLimit = defaultFailureLimit
	row.UnknownResultLimit = defaultUnknownResultLimit
	row.ConsecutiveFailureLimit = defaultConsecutiveFailureLimit
	row.ProtectionDurationSeconds = defaultProtectionDurationSeconds
}

func policyProtects(row *healthState) bool {
	return row.SampleCount >= row.MinimumSamplesToProtect &&
		(row.ConsecutiveFailures >= row.ConsecutiveFailureLimit ||
			row.UnknownCount >= row.UnknownResultLimit || row.FailureCount >= row.FailureLimit)
}

func validateObservation(input Observation) error {
	if resourcecap.ValidateChannelRef(input.ChannelNo) != nil ||
		!summaryCodePattern.MatchString(input.SummaryCode) || input.Latency < 0 ||
		input.Latency > time.Hour {
		return gerror.New("resource Provider health observation fields are invalid")
	}
	switch input.Operation {
	case OperationAuthentication, OperationQuery, OperationWrite, OperationProviderCapacity:
	default:
		return gerror.New("resource Provider health operation is invalid")
	}
	switch input.Outcome {
	case OutcomeSucceeded:
	case OutcomeFailed, OutcomeResultUnknown:
	default:
		return gerror.New("resource Provider health outcome is invalid")
	}
	if input.AuthenticationFailure &&
		(input.Operation == OperationAuthentication || input.Outcome == OutcomeSucceeded ||
			input.SummaryCode != string(resourcecap.ProviderCallErrorCodeAuthentication)) {
		return gerror.New("resource Provider authentication health marker is invalid")
	}
	return nil
}
