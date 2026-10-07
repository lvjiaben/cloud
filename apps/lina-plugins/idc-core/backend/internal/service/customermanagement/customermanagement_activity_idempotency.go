package customermanagement

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strconv"

	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/internal/service/customer"
)

type activityReservation struct {
	scope        string
	callerDigest string
	keyDigest    string
	resourceNo   string
	replayed     bool
}

func reserveActivity(
	ctx context.Context,
	scope string,
	actorUserID int,
	customerNo string,
	idempotencyKey string,
	request any,
	resourceType string,
	candidateResourceNo string,
) (activityReservation, error) {
	fingerprint, err := activityFingerprint(request)
	if err != nil {
		return activityReservation{}, err
	}
	callerDigest := activityDigest("staff:" + strconv.Itoa(actorUserID) + ":customer:" + customerNo)
	keyDigest := activityDigest(idempotencyKey)
	insert, err := g.DB().Exec(ctx, `INSERT INTO plugin_idc_core_idempotency_record
(tenant_id,scope,caller_scope_digest,idempotency_key_digest,request_fingerprint,status_code,resource_type,resource_no,response_status_code,response_snapshot,expires_at)
VALUES (0,?,?,?,?,'in_progress',?,?,200,'{}'::jsonb,CURRENT_TIMESTAMP+INTERVAL '30 days')
ON CONFLICT (tenant_id,scope,caller_scope_digest,idempotency_key_digest) DO NOTHING`,
		scope, callerDigest, keyDigest, fingerprint, resourceType, candidateResourceNo)
	if err != nil {
		return activityReservation{}, gerror.Wrap(err, "reserve customer activity idempotency")
	}
	created, err := insert.RowsAffected()
	if err != nil {
		return activityReservation{}, gerror.Wrap(err, "read customer activity reservation result")
	}
	var row struct {
		Fingerprint string `orm:"request_fingerprint"`
		Status      string `orm:"status_code"`
		ResourceNo  string `orm:"resource_no"`
	}
	if err = g.DB().GetScan(ctx, &row, `SELECT request_fingerprint,status_code,resource_no
FROM plugin_idc_core_idempotency_record
WHERE tenant_id=0 AND scope=? AND caller_scope_digest=? AND idempotency_key_digest=? FOR UPDATE`,
		scope, callerDigest, keyDigest); err != nil {
		return activityReservation{}, gerror.Wrap(err, "load customer activity reservation")
	}
	if row.Fingerprint != fingerprint || row.ResourceNo == "" {
		return activityReservation{}, bizerr.NewCode(customer.CodeIdempotencyConflict)
	}
	reservation := activityReservation{scope: scope, callerDigest: callerDigest, keyDigest: keyDigest, resourceNo: row.ResourceNo}
	if created == 1 {
		if row.Status != "in_progress" {
			return activityReservation{}, gerror.New("new customer activity reservation has invalid status")
		}
		return reservation, nil
	}
	if row.Status == "succeeded" {
		reservation.replayed = true
		return reservation, nil
	}
	return activityReservation{}, gerror.New("customer activity idempotency reservation is not replayable")
}

func completeActivity(ctx context.Context, reservation activityReservation) error {
	result, err := g.DB().Exec(ctx, `UPDATE plugin_idc_core_idempotency_record
SET status_code='succeeded',completed_at=CURRENT_TIMESTAMP,updated_at=CURRENT_TIMESTAMP
WHERE tenant_id=0 AND scope=? AND caller_scope_digest=? AND idempotency_key_digest=? AND status_code='in_progress'`,
		reservation.scope, reservation.callerDigest, reservation.keyDigest)
	if err != nil {
		return gerror.Wrap(err, "complete customer activity idempotency")
	}
	affected, err := result.RowsAffected()
	if err != nil || affected != 1 {
		return gerror.New("customer activity idempotency completion was lost")
	}
	return nil
}

func activityFingerprint(request any) (string, error) {
	payload, err := json.Marshal(request)
	if err != nil {
		return "", gerror.Wrap(err, "encode customer activity fingerprint")
	}
	return activityDigest(string(payload)), nil
}

func activityDigest(value string) string {
	digest := sha256.Sum256([]byte(value))
	return hex.EncodeToString(digest[:])
}
