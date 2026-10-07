package suppliercallback

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"
)

const MaximumDeliveryBatchSize = 50

var deliveryNoPattern = regexp.MustCompile(`^ldv_[0-9a-f]{32}$`)

type Delivery struct {
	DeliveryNo           string
	RouteNo              string
	RouteVersion         int64
	ProjectedStatus      Status
	ProviderStatus       string
	ReconciliationStatus string
	SourceKind           string
	DecisionCode         string
	SupplierObservedAt   time.Time
	ObservedAt           time.Time
}

func (s *QueryService) PendingDeliveries(
	ctx context.Context,
	provider string,
	operationScope string,
	limit int,
) ([]Delivery, error) {
	if s == nil || s.database == nil || ctx == nil || ctx.Err() != nil ||
		!validProvider(provider) || !operationScopePattern.MatchString(operationScope) ||
		limit < 1 || limit > MaximumDeliveryBatchSize {
		return nil, ErrInvalid
	}
	databaseContext, cancel := context.WithTimeout(ctx, callbackDatabaseTimeout)
	defer cancel()
	rows, err := s.database.QueryContext(databaseContext, `
SELECT d.delivery_no, d.route_no, d.route_version, d.projected_status,
       e.provider_status, d.reconciliation_status, e.source_kind,
       e.decision_code, e.supplier_observed_at, e.observed_at
FROM idc_gateway_local_route_delivery d
JOIN idc_gateway_local_callback_evidence e ON e.evidence_no = d.evidence_no
WHERE d.installation_id = $1 AND d.provider_code = $2 AND d.operation_scope = $3
  AND d.status_code = 'pending' AND e.provider_status IS NOT NULL
ORDER BY d.created_at, d.delivery_no
LIMIT $4`, s.installationID, provider, operationScope, limit)
	if err != nil {
		return nil, ErrRejected
	}
	defer rows.Close()
	result := make([]Delivery, 0, limit)
	for rows.Next() {
		var item Delivery
		var supplierObserved sql.NullTime
		if err = rows.Scan(
			&item.DeliveryNo, &item.RouteNo, &item.RouteVersion, &item.ProjectedStatus,
			&item.ProviderStatus, &item.ReconciliationStatus, &item.SourceKind,
			&item.DecisionCode, &supplierObserved, &item.ObservedAt,
		); err != nil || !validDelivery(provider, operationScope, item) {
			return nil, ErrRejected
		}
		if supplierObserved.Valid {
			item.SupplierObservedAt = supplierObserved.Time.UTC()
		}
		item.ObservedAt = item.ObservedAt.UTC()
		result = append(result, item)
	}
	if rows.Err() != nil {
		return nil, ErrRejected
	}
	return result, nil
}

func (s *QueryService) AcknowledgeDeliveries(
	ctx context.Context,
	provider string,
	operationScope string,
	deliveryNos []string,
) (returnError error) {
	if s == nil || s.database == nil || ctx == nil || ctx.Err() != nil ||
		!validProvider(provider) || !operationScopePattern.MatchString(operationScope) ||
		len(deliveryNos) < 1 || len(deliveryNos) > MaximumDeliveryBatchSize {
		return ErrInvalid
	}
	numbers := append([]string(nil), deliveryNos...)
	sort.Strings(numbers)
	for index, number := range numbers {
		if !deliveryNoPattern.MatchString(number) || index > 0 && number == numbers[index-1] {
			return ErrInvalid
		}
	}
	databaseContext, cancel := context.WithTimeout(ctx, callbackDatabaseTimeout)
	defer cancel()
	transaction, err := s.database.BeginTx(databaseContext, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return ErrRejected
	}
	defer finishTransaction(transaction, &returnError)
	arguments := []any{s.installationID, provider, operationScope}
	placeholders := make([]string, 0, len(numbers))
	for _, number := range numbers {
		arguments = append(arguments, number)
		placeholders = append(placeholders, fmt.Sprintf("$%d", len(arguments)))
	}
	query := `
SELECT delivery_no
FROM idc_gateway_local_route_delivery
WHERE installation_id = $1 AND provider_code = $2 AND operation_scope = $3
  AND delivery_no IN (` + strings.Join(placeholders, ",") + `)
ORDER BY delivery_no
FOR UPDATE`
	rows, err := transaction.QueryContext(databaseContext, query, arguments...)
	if err != nil {
		return ErrRejected
	}
	defer rows.Close()
	locked := make([]string, 0, len(numbers))
	for rows.Next() {
		var number string
		if rows.Scan(&number) != nil {
			return ErrRejected
		}
		locked = append(locked, number)
	}
	if closeErr := rows.Close(); closeErr != nil || rows.Err() != nil || len(locked) != len(numbers) {
		return ErrConflict
	}
	for index := range locked {
		if locked[index] != numbers[index] {
			return ErrConflict
		}
	}
	databaseNow, err := callbackDatabaseTime(databaseContext, transaction)
	if err != nil {
		return ErrRejected
	}
	update := `
UPDATE idc_gateway_local_route_delivery
SET status_code = 'acknowledged', acknowledged_at = $` + fmt.Sprintf("%d", len(arguments)+1) + `
WHERE installation_id = $1 AND provider_code = $2 AND operation_scope = $3
  AND status_code = 'pending' AND delivery_no IN (` + strings.Join(placeholders, ",") + `)`
	arguments = append(arguments, databaseNow)
	if _, err = transaction.ExecContext(databaseContext, update, arguments...); err != nil {
		return ErrRejected
	}
	if err = transaction.Commit(); err != nil {
		return ErrRejected
	}
	return nil
}

func validDelivery(provider string, operationScope string, item Delivery) bool {
	return deliveryNoPattern.MatchString(item.DeliveryNo) && routeNoPattern.MatchString(item.RouteNo) &&
		item.RouteVersion > 1 && validStatus(item.ProjectedStatus) &&
		validProviderStatus(provider, item.ProviderStatus) &&
		operationScopePattern.MatchString(operationScope) && !item.ObservedAt.IsZero() &&
		(item.SourceKind == "callback" || item.SourceKind == "active_query") &&
		(item.DecisionCode == "applied" || item.DecisionCode == "stale" || item.DecisionCode == "conflict") &&
		(item.ReconciliationStatus == "none" || item.ReconciliationStatus == "query_required" ||
			item.ReconciliationStatus == "reconciliation_required")
}
