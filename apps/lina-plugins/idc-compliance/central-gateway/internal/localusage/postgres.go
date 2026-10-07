package localusage

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// PostgreSQLStore reads only final installation-local supplier projections.
type PostgreSQLStore struct{ database *sql.DB }

func NewPostgreSQLStore(database *sql.DB) (*PostgreSQLStore, error) {
	if database == nil {
		return nil, ErrUnavailable
	}
	return &PostgreSQLStore{database: database}, nil
}

func (s *PostgreSQLStore) LatestPlan(ctx context.Context, provider string) (*Plan, error) {
	row := s.database.QueryRowContext(ctx, `
SELECT provider_code, status_code, COALESCE(currency_code, ''), valid_from, valid_until, observed_at
FROM idc_gateway_supplier_plan_snapshot
WHERE provider_code = $1
ORDER BY observed_at DESC, plan_snapshot_no DESC
LIMIT 1`, provider)
	var result Plan
	var validFrom, validUntil sql.NullTime
	err := row.Scan(
		&result.ProviderCode, &result.StatusCode, &result.CurrencyCode,
		&validFrom, &validUntil, &result.ObservedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("load local supplier plan: %w", err)
	}
	result.ValidFrom = nullTimePointer(validFrom)
	result.ValidUntil = nullTimePointer(validUntil)
	return &result, nil
}

func (s *PostgreSQLStore) ListUsage(
	ctx context.Context,
	provider string,
	from time.Time,
	to time.Time,
	limit int,
) ([]Usage, error) {
	rows, err := s.database.QueryContext(ctx, `
SELECT provider_code, metric_code, period_started_at, period_ended_at,
       quantity_numerator, quantity_denominator, supplier_cost_minor,
       COALESCE(currency_code, ''), observed_at
FROM idc_gateway_supplier_usage_snapshot
WHERE provider_code = $1 AND period_started_at >= $2 AND period_ended_at <= $3
ORDER BY period_started_at DESC, period_ended_at DESC, usage_snapshot_no DESC
LIMIT $4`, provider, from, to, limit)
	if err != nil {
		return nil, fmt.Errorf("list local supplier usage: %w", err)
	}
	defer rows.Close()
	result := make([]Usage, 0, limit)
	for rows.Next() {
		var item Usage
		var cost sql.NullInt64
		if err = rows.Scan(
			&item.ProviderCode, &item.MetricCode, &item.PeriodStartedAt, &item.PeriodEndedAt,
			&item.QuantityNumerator, &item.QuantityDenominator, &cost,
			&item.CurrencyCode, &item.ObservedAt,
		); err != nil {
			return nil, fmt.Errorf("scan local supplier usage: %w", err)
		}
		if cost.Valid {
			value := cost.Int64
			item.SupplierCostMinor = &value
		}
		result = append(result, item)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate local supplier usage: %w", err)
	}
	return result, nil
}

func (s *PostgreSQLStore) LatestReconciliation(
	ctx context.Context,
	provider string,
	from time.Time,
	to time.Time,
) (*Reconciliation, error) {
	row := s.database.QueryRowContext(ctx, `
SELECT provider_code, period_started_at, period_ended_at, currency_code,
       supplier_cost_minor, local_observed_cost_minor, difference_minor,
       status_code, observed_at
FROM idc_gateway_supplier_reconciliation
WHERE provider_code = $1 AND period_started_at >= $2 AND period_ended_at <= $3
ORDER BY observed_at DESC, reconciliation_no DESC
LIMIT 1`, provider, from, to)
	var result Reconciliation
	err := row.Scan(
		&result.ProviderCode, &result.PeriodStartedAt, &result.PeriodEndedAt,
		&result.CurrencyCode, &result.SupplierCostMinor, &result.LocalObservedCostMinor,
		&result.DifferenceMinor, &result.StatusCode, &result.ObservedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("load local supplier reconciliation: %w", err)
	}
	return &result, nil
}

func (s *PostgreSQLStore) LatestSLA(
	ctx context.Context,
	provider string,
	from time.Time,
	to time.Time,
) (*SLA, error) {
	row := s.database.QueryRowContext(ctx, `
SELECT provider_code, period_started_at, period_ended_at, request_count,
       success_count, availability_ppm, latency_p95_millis, latency_p99_millis
FROM idc_gateway_supplier_sla_snapshot
WHERE provider_code = $1 AND period_started_at >= $2 AND period_ended_at <= $3
ORDER BY period_ended_at DESC, sla_snapshot_no DESC
LIMIT 1`, provider, from, to)
	var result SLA
	err := row.Scan(
		&result.ProviderCode, &result.PeriodStartedAt, &result.PeriodEndedAt,
		&result.RequestCount, &result.SuccessCount, &result.AvailabilityPPM,
		&result.LatencyP95Millis, &result.LatencyP99Millis,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("load local supplier SLA: %w", err)
	}
	return &result, nil
}

func nullTimePointer(value sql.NullTime) *time.Time {
	if !value.Valid {
		return nil
	}
	result := value.Time.UTC()
	return &result
}
