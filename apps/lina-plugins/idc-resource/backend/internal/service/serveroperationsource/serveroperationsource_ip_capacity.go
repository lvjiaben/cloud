package serveroperationsource

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"time"

	"github.com/gogf/gf/v2/database/gdb"

	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/cap/serveroperationresourcecap"
)

var (
	stableIPReasonPattern = regexp.MustCompile(`^[a-z][a-z0-9_.-]{0,63}$`)
	ipReferencePattern    = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.:-]{0,63}$`)
)

var _ serveroperationresourcecap.IPCapacity = (*Service)(nil)

type ipReservationRow struct {
	ReservationNo         string    `orm:"reservation_no"`
	CapacityObservationNo string    `orm:"capacity_observation_no"`
	Quantity              int       `orm:"quantity"`
	Status                string    `orm:"status_code"`
	RequestDigest         string    `orm:"request_digest"`
	ProviderTaskNo        string    `orm:"provider_task_no"`
	CommercialOperationNo string    `orm:"commercial_operation_no"`
	ReleaseReason         string    `orm:"release_reason_code"`
	ExpiresAt             time.Time `orm:"expires_at"`
}

func (s *Service) ReserveIP(ctx context.Context,
	request serveroperationresourcecap.IPReservationRequest,
) (*serveroperationresourcecap.IPReservation, error) {
	if s == nil || s.database == nil || ctx == nil || ctx.Err() != nil ||
		serveroperationresourcecap.ValidateIPReservationRequest(request) != nil {
		return nil, serveroperationresourcecap.ErrIPReservationConflict
	}
	idempotency := sha256.Sum256([]byte(request.IdempotencyKey))
	idempotencyDigest := hex.EncodeToString(idempotency[:])
	reservationNo := "IPR_" + idempotencyDigest[:32]
	var result *serveroperationresourcecap.IPReservation
	err := s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		var existing *ipReservationRow
		if err := s.database.GetScan(ctx, &existing, `SELECT reservation_no,capacity_observation_no,quantity,status_code,request_digest,expires_at
FROM plugin_idc_resource_ip_reservation
WHERE tenant_id=? AND idempotency_digest=?`, idccap.DefaultTenantID, idempotencyDigest); err != nil {
			return err
		}
		if existing != nil {
			if existing.RequestDigest != request.RequestDigest {
				return serveroperationresourcecap.ErrIPReservationConflict
			}
			result = projectIPReservation(existing, true)
			return nil
		}
		nowValue, readErr := s.database.GetValue(ctx, `SELECT CURRENT_TIMESTAMP`)
		if readErr != nil {
			return serveroperationresourcecap.ErrIPCapacityUnavailable
		}
		now := nowValue.Time().UTC()
		if now.IsZero() {
			return serveroperationresourcecap.ErrIPCapacityUnavailable
		}
		if !request.ExpiresAt.After(now) || request.ExpiresAt.After(now.Add(2*time.Hour)) {
			return serveroperationresourcecap.ErrIPReservationConflict
		}
		poolIDValue, readErr := s.database.GetValue(ctx, `SELECT pool.id
FROM plugin_idc_resource_pool pool
JOIN plugin_idc_resource_channel channel
 ON channel.tenant_id=pool.tenant_id AND channel.channel_no=pool.channel_no
WHERE pool.tenant_id=? AND pool.pool_no=? AND pool.channel_no=?
 AND pool.status_code='enabled' AND pool.deleted_at IS NULL
 AND channel.status_code='enabled' AND channel.deleted_at IS NULL
FOR UPDATE`, idccap.DefaultTenantID, request.Route.PoolNo, request.Route.ChannelNo)
		if readErr != nil || poolIDValue.Int64() <= 0 {
			return serveroperationresourcecap.ErrIPCapacityUnavailable
		}
		if _, err := s.database.Exec(ctx, `UPDATE plugin_idc_resource_ip_reservation
SET status_code='expired',release_reason_code='reservation_expired',released_at=?,updated_at=?,version=version+1
WHERE tenant_id=? AND channel_no=? AND pool_no=? AND status_code='reserved' AND expires_at<=?`,
			now, now, idccap.DefaultTenantID, request.Route.ChannelNo, request.Route.PoolNo, now); err != nil {
			return err
		}
		var observation *struct {
			ObservationNo string    `orm:"observation_no"`
			ObservedAt    time.Time `orm:"observed_at"`
		}
		if err := s.database.GetScan(ctx, &observation, `SELECT observation_no,observed_at
FROM plugin_idc_resource_provider_capacity_observation
WHERE tenant_id=? AND channel_no=? AND capacity_kind='stock' AND status_code='complete'
 AND observed_at>=? ORDER BY observed_at DESC,id DESC LIMIT 1 FOR SHARE`,
			idccap.DefaultTenantID, request.Route.ChannelNo,
			now.Add(-time.Duration(request.MaximumCapacityAgeSeconds)*time.Second)); err != nil || observation == nil {
			return serveroperationresourcecap.ErrIPCapacityUnavailable
		}
		var stock struct {
			ItemCount  int   `orm:"item_count"`
			ValidCount int   `orm:"valid_count"`
			FreeCount  int64 `orm:"free_count"`
		}
		if err := s.database.GetScan(ctx, &stock, `SELECT count(*) item_count,
count(*) FILTER (WHERE jsonb_exists(projection,'ip_free') AND projection->>'ip_free' ~ '^[0-9]+$') valid_count,
COALESCE(sum(CASE WHEN jsonb_exists(projection,'ip_free') AND projection->>'ip_free' ~ '^[0-9]+$'
 THEN (projection->>'ip_free')::BIGINT ELSE 0 END),0) free_count
FROM plugin_idc_resource_provider_capacity_observation_item
WHERE tenant_id=? AND observation_no=? AND capacity_kind='stock'`,
			idccap.DefaultTenantID, observation.ObservationNo); err != nil || stock.ItemCount <= 0 ||
			stock.ValidCount != stock.ItemCount || stock.FreeCount < 0 {
			return serveroperationresourcecap.ErrIPCapacityUnavailable
		}
		reservedValue, readErr := s.database.GetValue(ctx, `SELECT COALESCE(sum(quantity),0)
FROM plugin_idc_resource_ip_reservation
WHERE tenant_id=? AND channel_no=? AND pool_no=? AND (status_code IN ('commercial_hold','in_flight') OR (status_code='reserved' AND expires_at>?))`,
			idccap.DefaultTenantID, request.Route.ChannelNo, request.Route.PoolNo, now)
		if readErr != nil {
			return readErr
		}
		reserved := reservedValue.Int64()
		if stock.FreeCount-reserved < int64(request.Quantity) {
			return serveroperationresourcecap.ErrIPCapacityUnavailable
		}
		if _, err := s.database.Exec(ctx, `INSERT INTO plugin_idc_resource_ip_reservation
(tenant_id,reservation_no,service_no,channel_no,pool_no,capacity_observation_no,capacity_kind,quantity,status_code,
 idempotency_digest,request_digest,expires_at,created_at,updated_at)
VALUES (?,?,?,?,?,?,'stock',?,'reserved',?,?,?,?,?)`, idccap.DefaultTenantID, reservationNo,
			request.Route.ServiceNo, request.Route.ChannelNo, request.Route.PoolNo, observation.ObservationNo,
			request.Quantity, idempotencyDigest, request.RequestDigest, request.ExpiresAt.UTC(), now, now); err != nil {
			return err
		}
		result = &serveroperationresourcecap.IPReservation{ReservationNo: reservationNo,
			CapacityObservationNo: observation.ObservationNo, Quantity: request.Quantity,
			Status: serveroperationresourcecap.IPReservationReserved, ExpiresAt: request.ExpiresAt.UTC()}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if result == nil || serveroperationresourcecap.ValidateIPReservation(*result) != nil {
		return nil, serveroperationresourcecap.ErrIPReservationConflict
	}
	return result, nil
}

func (s *Service) ReleaseIP(ctx context.Context,
	request serveroperationresourcecap.IPReservationReleaseRequest,
) (*serveroperationresourcecap.IPReservation, error) {
	if s == nil || s.database == nil || ctx == nil || ctx.Err() != nil ||
		!stableIPReasonPattern.MatchString(request.ReasonCode) {
		return nil, serveroperationresourcecap.ErrIPReservationConflict
	}
	return s.transitionIPReservation(ctx, request.ReservationNo, "", request.ReasonCode, false)
}

func (s *Service) ConsumeIP(ctx context.Context,
	request serveroperationresourcecap.IPReservationConsumeRequest,
) (*serveroperationresourcecap.IPReservation, error) {
	if s == nil || s.database == nil || ctx == nil || ctx.Err() != nil {
		return nil, serveroperationresourcecap.ErrIPReservationConflict
	}
	return s.transitionIPReservation(ctx, request.ReservationNo, request.ProviderTaskNo, "", true)
}

func (s *Service) transitionIPReservation(ctx context.Context, reservationNo, taskNo, reason string,
	consume bool,
) (*serveroperationresourcecap.IPReservation, error) {
	if !validIPReference(reservationNo) || (consume && !validIPReference(taskNo)) {
		return nil, serveroperationresourcecap.ErrIPReservationConflict
	}
	var result *serveroperationresourcecap.IPReservation
	err := s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		var row *ipReservationRow
		if err := s.database.GetScan(ctx, &row, `SELECT reservation_no,capacity_observation_no,quantity,status_code,request_digest,
COALESCE(provider_task_no,'') provider_task_no,COALESCE(release_reason_code,'') release_reason_code,expires_at
FROM plugin_idc_resource_ip_reservation WHERE tenant_id=? AND reservation_no=? FOR UPDATE`,
			idccap.DefaultTenantID, reservationNo); err != nil || row == nil {
			return serveroperationresourcecap.ErrIPReservationConflict
		}
		if consume && row.Status == string(serveroperationresourcecap.IPReservationConsumed) {
			if row.ProviderTaskNo != taskNo {
				return serveroperationresourcecap.ErrIPReservationConflict
			}
			result = projectIPReservation(row, true)
			return nil
		}
		if !consume && (row.Status == string(serveroperationresourcecap.IPReservationReleased) ||
			row.Status == string(serveroperationresourcecap.IPReservationExpired)) {
			if row.ReleaseReason != reason {
				return serveroperationresourcecap.ErrIPReservationConflict
			}
			result = projectIPReservation(row, true)
			return nil
		}
		if row.Status != string(serveroperationresourcecap.IPReservationReserved) && row.Status != string(serveroperationresourcecap.IPReservationInFlight) {
			return serveroperationresourcecap.ErrIPReservationConflict
		}
		if row.Status == string(serveroperationresourcecap.IPReservationInFlight) {
			if consume && row.ProviderTaskNo != taskNo {
				return serveroperationresourcecap.ErrIPReservationConflict
			}
			if err := s.validateIPTaskOutcome(ctx, row.ProviderTaskNo, consume); err != nil {
				return err
			}
		}
		nowValue, readErr := s.database.GetValue(ctx, `SELECT CURRENT_TIMESTAMP`)
		if readErr != nil {
			return serveroperationresourcecap.ErrIPCapacityUnavailable
		}
		now := nowValue.Time().UTC()
		if now.IsZero() {
			return serveroperationresourcecap.ErrIPCapacityUnavailable
		}
		status := string(serveroperationresourcecap.IPReservationReleased)
		if consume {
			status = string(serveroperationresourcecap.IPReservationConsumed)
			if _, err := s.database.Exec(ctx, `UPDATE plugin_idc_resource_ip_reservation
SET status_code=?,provider_task_no=?,consumed_at=?,updated_at=?,version=version+1
WHERE tenant_id=? AND reservation_no=? AND status_code IN ('reserved','in_flight')`, status, taskNo, now, now,
				idccap.DefaultTenantID, reservationNo); err != nil {
				return err
			}
		} else if _, err := s.database.Exec(ctx, `UPDATE plugin_idc_resource_ip_reservation
SET status_code=?,release_reason_code=?,released_at=?,updated_at=?,version=version+1
WHERE tenant_id=? AND reservation_no=? AND status_code IN ('reserved','in_flight')`, status, reason, now, now,
			idccap.DefaultTenantID, reservationNo); err != nil {
			return err
		}
		row.Status = status
		result = projectIPReservation(row, false)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func projectIPReservation(row *ipReservationRow, reused bool) *serveroperationresourcecap.IPReservation {
	if row == nil {
		return nil
	}
	return &serveroperationresourcecap.IPReservation{ReservationNo: row.ReservationNo,
		CapacityObservationNo: row.CapacityObservationNo, Quantity: row.Quantity,
		Status:    serveroperationresourcecap.IPReservationStatus(row.Status),
		ExpiresAt: row.ExpiresAt.UTC(), Reused: reused}
}

func validIPReference(value string) bool {
	return len(value) >= 1 && len(value) <= 64 && ipReferencePattern.MatchString(value)
}
