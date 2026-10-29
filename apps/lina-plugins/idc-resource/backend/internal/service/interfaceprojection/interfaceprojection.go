// Package interfaceprojection persists and delivers secret-free resource
// interface projections to the generic product automation owner.
package interfaceprojection

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strconv"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/cap/resourceinterfaceprojectioncap"
)

const (
	maximumDeliveryDelaySeconds = 300
	deliveryLeaseSeconds        = 30
	maximumDeliveryAttempts     = 10
)

// Worker claims projection events from PostgreSQL and retries failed sink
// calls without relying on process-local state.
type Worker struct {
	database gdb.DB
	sink     resourceinterfaceprojectioncap.Sink
	random   func() (string, error)
}

// NewWorker constructs one stateless outbox worker using the shared database.
func NewWorker(database gdb.DB, sink resourceinterfaceprojectioncap.Sink) (*Worker, error) {
	if database == nil || sink == nil {
		return nil, gerror.New("resource interface projection worker dependencies are required")
	}
	return &Worker{database: database, sink: sink, random: deliveryToken}, nil
}

// NextSequence allocates the next sequence from this interface's shared outbox.
// The caller must hold its interface binding row lock until the enqueue transaction
// commits, so channel edits and secret revisions serialize on the same authority.
func NextSequence(ctx context.Context, tx gdb.TX, interfaceInstanceNo string) (int64, error) {
	if ctx == nil || ctx.Err() != nil || tx == nil || interfaceInstanceNo == "" {
		return 0, gerror.New("resource interface projection sequence input is invalid")
	}
	value, err := tx.GetValue(`SELECT COALESCE(MAX(projection_sequence),0)+1
FROM plugin_idc_resource_interface_projection_outbox WHERE tenant_id=? AND interface_instance_no=?`,
		idccap.DefaultTenantID, interfaceInstanceNo)
	if err != nil {
		return 0, gerror.Wrap(err, "read resource interface projection sequence")
	}
	return value.Int64(), nil
}

// Enqueue inserts one deterministic projection sequence in the caller's
// resource transaction. Exact retries are ignored; conflicting payloads fail.
func Enqueue(
	ctx context.Context,
	tx gdb.TX,
	projection resourceinterfaceprojectioncap.Projection,
	sequence int64,
) error {
	if ctx == nil || ctx.Err() != nil || tx == nil || sequence < 1 ||
		resourceinterfaceprojectioncap.ValidateProjection(projection) != nil {
		return gerror.New("resource interface projection event is invalid")
	}
	payload, err := json.Marshal(projection)
	if err != nil {
		return gerror.Wrap(err, "encode resource interface projection")
	}
	outboxNo := projectionOutboxNo(projection.InterfaceInstanceNo, sequence)
	result, err := tx.Exec(`
INSERT INTO plugin_idc_resource_interface_projection_outbox
    (tenant_id,outbox_no,interface_instance_no,projection_sequence,projection_payload)
VALUES (?,?,?,?,?::jsonb)
ON CONFLICT (tenant_id,interface_instance_no,projection_sequence) DO NOTHING`,
		idccap.DefaultTenantID, outboxNo, projection.InterfaceInstanceNo, sequence, payload)
	if err != nil {
		return gerror.Wrap(err, "enqueue resource interface projection")
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return gerror.Wrap(err, "read resource interface projection enqueue result")
	}
	if rows == 1 {
		return nil
	}
	existingValue, err := tx.GetValue(`
SELECT projection_payload::TEXT
FROM plugin_idc_resource_interface_projection_outbox
WHERE tenant_id=? AND interface_instance_no=? AND projection_sequence=?`,
		idccap.DefaultTenantID, projection.InterfaceInstanceNo, sequence)
	if err != nil {
		return gerror.Wrap(err, "read existing resource interface projection")
	}
	existing := existingValue.String()
	var current resourceinterfaceprojectioncap.Projection
	if json.Unmarshal([]byte(existing), &current) != nil || !sameProjection(current, projection) {
		return gerror.New("resource interface projection sequence conflicts")
	}
	return nil
}

type claimedProjection struct {
	ID           int64  `orm:"id"`
	LockedBy     string `orm:"locked_by"`
	AttemptCount int    `orm:"attempt_count"`
	Payload      string `orm:"projection_payload"`
}

// RunOne claims and delivers at most one event. Concurrent workers use
// SKIP LOCKED and a renewable database lease, so one failed node cannot pin
// the outbox indefinitely.
func (worker *Worker) RunOne(ctx context.Context) (bool, error) {
	return worker.runOne(ctx, "")
}

func (worker *Worker) runOne(ctx context.Context, interfaceNo string) (bool, error) {
	if worker == nil || worker.database == nil || worker.sink == nil ||
		ctx == nil || ctx.Err() != nil || worker.random == nil {
		return false, gerror.New("resource interface projection worker is unavailable")
	}
	lockToken, err := worker.random()
	if err != nil {
		return false, gerror.Wrap(err, "generate resource interface projection lease")
	}
	var claimed *claimedProjection
	err = worker.database.Transaction(ctx, func(txCtx context.Context, tx gdb.TX) error {
		if _, updateErr := tx.Exec(`
UPDATE plugin_idc_resource_interface_projection_outbox
SET status_code='manual_attention',locked_by='',lease_expires_at=NULL,
    last_failure_code='projection_delivery_exhausted',updated_at=CURRENT_TIMESTAMP
WHERE tenant_id=? AND attempt_count>=? AND (?='' OR interface_instance_no=?) AND status_code IN ('pending','delivering')
  AND (status_code='pending' OR lease_expires_at<CURRENT_TIMESTAMP)`,
			idccap.DefaultTenantID, maximumDeliveryAttempts, interfaceNo, interfaceNo); updateErr != nil {
			return updateErr
		}
		return tx.GetScan(&claimed, `
WITH candidate AS (
    SELECT id
    FROM plugin_idc_resource_interface_projection_outbox
    WHERE tenant_id=? AND attempt_count<? AND (?='' OR interface_instance_no=?) AND (
        (status_code='pending' AND available_at<=CURRENT_TIMESTAMP) OR
        (status_code='delivering' AND lease_expires_at<CURRENT_TIMESTAMP)
    )
      AND NOT EXISTS (
          SELECT 1
          FROM plugin_idc_resource_interface_projection_outbox earlier
          WHERE earlier.tenant_id=plugin_idc_resource_interface_projection_outbox.tenant_id
            AND earlier.interface_instance_no=plugin_idc_resource_interface_projection_outbox.interface_instance_no
            AND earlier.projection_sequence<plugin_idc_resource_interface_projection_outbox.projection_sequence
            AND earlier.status_code<>'delivered'
      )
    ORDER BY id
    LIMIT 1
    FOR UPDATE SKIP LOCKED
)
UPDATE plugin_idc_resource_interface_projection_outbox outbox
SET status_code='delivering',attempt_count=attempt_count+1,
    locked_by=?,lease_expires_at=CURRENT_TIMESTAMP + make_interval(secs => ?),updated_at=CURRENT_TIMESTAMP
FROM candidate
WHERE outbox.id=candidate.id
RETURNING outbox.id,outbox.locked_by,outbox.attempt_count,
          outbox.projection_payload::TEXT AS projection_payload`,
			idccap.DefaultTenantID, maximumDeliveryAttempts, interfaceNo, interfaceNo, lockToken, deliveryLeaseSeconds)
	})
	if err != nil {
		return false, gerror.Wrap(err, "claim resource interface projection")
	}
	if claimed == nil || claimed.ID == 0 {
		return false, nil
	}
	var projection resourceinterfaceprojectioncap.Projection
	if json.Unmarshal([]byte(claimed.Payload), &projection) != nil ||
		resourceinterfaceprojectioncap.ValidateProjection(projection) != nil {
		return true, worker.retry(ctx, claimed, "projection_invalid")
	}
	if err = worker.sink.Upsert(ctx, projection); err != nil {
		return true, worker.retry(ctx, claimed, "projection_delivery_failed")
	}
	result, err := worker.database.Exec(ctx, `
UPDATE plugin_idc_resource_interface_projection_outbox
SET status_code='delivered',locked_by='',lease_expires_at=NULL,
    delivered_at=CURRENT_TIMESTAMP,last_failure_code='',updated_at=CURRENT_TIMESTAMP
WHERE tenant_id=? AND id=? AND status_code='delivering' AND locked_by=?`,
		idccap.DefaultTenantID, claimed.ID, claimed.LockedBy)
	if err != nil {
		return true, gerror.Wrap(err, "complete resource interface projection")
	}
	rows, err := result.RowsAffected()
	if err != nil || rows != 1 {
		return true, gerror.New("resource interface projection lease was lost")
	}
	return true, nil
}

func (worker *Worker) retry(
	ctx context.Context,
	claimed *claimedProjection,
	failureCode string,
) error {
	delaySeconds := claimed.AttemptCount * 5
	if delaySeconds > maximumDeliveryDelaySeconds {
		delaySeconds = maximumDeliveryDelaySeconds
	}
	status := "pending"
	if claimed.AttemptCount >= maximumDeliveryAttempts {
		status = "manual_attention"
	}
	result, err := worker.database.Exec(ctx, `
UPDATE plugin_idc_resource_interface_projection_outbox
SET status_code=?,locked_by='',lease_expires_at=NULL,
    available_at=CURRENT_TIMESTAMP + make_interval(secs => ?),
    last_failure_code=?,updated_at=CURRENT_TIMESTAMP
WHERE tenant_id=? AND id=? AND status_code='delivering' AND locked_by=?`,
		status, delaySeconds, failureCode, idccap.DefaultTenantID, claimed.ID, claimed.LockedBy)
	if err != nil {
		return gerror.Wrap(err, "retry resource interface projection")
	}
	rows, err := result.RowsAffected()
	if err != nil || rows != 1 {
		return gerror.New("resource interface projection retry lease was lost")
	}
	return nil
}

func projectionOutboxNo(interfaceInstanceNo string, sequence int64) string {
	payload := interfaceInstanceNo + ":" + strconv.FormatInt(sequence, 10)
	digest := sha256.Sum256([]byte(payload))
	return "RIO_" + hex.EncodeToString(digest[:16])
}

func deliveryToken() (string, error) {
	value := make([]byte, 16)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return hex.EncodeToString(value), nil
}

func sameProjection(left, right resourceinterfaceprojectioncap.Projection) bool {
	leftPayload, leftErr := json.Marshal(left)
	rightPayload, rightErr := json.Marshal(right)
	return leftErr == nil && rightErr == nil && string(leftPayload) == string(rightPayload)
}

// Ensure delivers only the requested interface's ordered outbox prefix before
// import admission. It never waits for leases or backoff and fails closed after
// twenty events; a retry continues from the durable delivered prefix.
func (worker *Worker) Ensure(ctx context.Context, interfaceNo, revision string) error {
	request := resourceinterfaceprojectioncap.CandidateRequest{InterfaceInstanceNo: interfaceNo, ExpectedConfigurationRevision: revision, Method: "projection.ensure", Values: json.RawMessage(`{}`)}
	if worker == nil || worker.database == nil || revision == "" || resourceinterfaceprojectioncap.ValidateCandidateRequest(request) != nil {
		return gerror.New("resource interface projection is unavailable")
	}
	for delivered := 0; delivered <= 20; delivered++ {
		value, err := worker.database.GetValue(ctx, `SELECT CASE WHEN status_code='delivered' THEN projection_payload->>'InterfaceConfigurationRevision' ELSE '' END FROM plugin_idc_resource_interface_projection_outbox WHERE tenant_id=? AND interface_instance_no=? ORDER BY projection_sequence DESC LIMIT 1`, idccap.DefaultTenantID, interfaceNo)
		if err != nil {
			return gerror.Wrap(err, "read delivered interface projection")
		}
		if value.String() == revision {
			return nil
		}
		if delivered == 20 {
			break
		}
		progress, err := worker.runOne(ctx, interfaceNo)
		if err != nil {
			return err
		}
		if !progress {
			break
		}
	}
	return gerror.New("resource interface projection has not reached the requested revision")
}

// RefreshUnused drains the current durable projection before entering the
// core-owned unused-interface fence. Failed historical events are never skipped.
func (worker *Worker) RefreshUnused(ctx context.Context, currentRevision string, projection resourceinterfaceprojectioncap.Projection, apply func(context.Context) error) error {
	if err := worker.Ensure(ctx, projection.InterfaceInstanceNo, currentRevision); err != nil {
		return err
	}
	return worker.sink.RefreshUnused(ctx, projection, apply)
}
