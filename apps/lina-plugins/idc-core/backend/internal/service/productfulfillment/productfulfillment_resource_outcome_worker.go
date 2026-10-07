// This file delivers resource-owner outcome reports from a durable post-commit
// outbox so temporary owner failures cannot strand credential lifecycle state.

package productfulfillment

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-plugin-idc-core/backend/cap/provisionresourcecap"
)

const maximumResourceOutcomeBatch = 64

// ResourceOutcomeBatch reports one bounded owner-report delivery pass.
type ResourceOutcomeBatch struct{ Selected, Reported, Retried, ManualAttention int }

// ResourceOutcomeWorker converges durable post-commit owner reports.
type ResourceOutcomeWorker interface {
	Run(context.Context, int) (*ResourceOutcomeBatch, error)
}

type resourceOutcomeWorker struct {
	database gdb.DB
	source   provisionresourcecap.Source
}

type resourceOutcomeClaim struct {
	TaskNo, LeaseToken, ReportPayload, ReportDigest string
	AttemptCount                                    int
}

// NewResourceOutcomeWorker constructs the post-commit resource-owner reporter.
func NewResourceOutcomeWorker(database gdb.DB, source provisionresourcecap.Source) (ResourceOutcomeWorker, error) {
	if database == nil || source == nil {
		return nil, gerror.New("resource outcome worker dependencies are unavailable")
	}
	return &resourceOutcomeWorker{database: database, source: source}, nil
}

func (worker *resourceOutcomeWorker) Run(ctx context.Context, limit int) (*ResourceOutcomeBatch, error) {
	if worker == nil || worker.database == nil || worker.source == nil || ctx == nil || ctx.Err() != nil ||
		limit < 1 || limit > maximumResourceOutcomeBatch {
		return nil, gerror.New("resource outcome worker input is invalid")
	}
	report := &ResourceOutcomeBatch{}
	for report.Selected < limit {
		claim, err := worker.claim(ctx)
		if err != nil {
			return report, err
		}
		if claim == nil {
			break
		}
		report.Selected++
		var outcome provisionresourcecap.OutcomeReport
		deliveryErr := json.Unmarshal([]byte(claim.ReportPayload), &outcome)
		if deliveryErr == nil && sha256Hex([]byte(claim.ReportPayload)) != claim.ReportDigest {
			deliveryErr = gerror.New("resource outcome report digest is invalid")
		}
		if deliveryErr == nil && provisionresourcecap.ValidateOutcomeReport(outcome) != nil {
			deliveryErr = gerror.New("resource outcome report payload is invalid")
		}
		if deliveryErr == nil {
			deliveryErr = worker.source.ReportOutcome(ctx, outcome)
		}
		manual, err := worker.finish(ctx, claim, deliveryErr)
		if err != nil {
			return report, err
		}
		if deliveryErr == nil {
			report.Reported++
		} else if manual {
			report.ManualAttention++
		} else {
			report.Retried++
		}
	}
	return report, nil
}

func (worker *resourceOutcomeWorker) claim(ctx context.Context) (*resourceOutcomeClaim, error) {
	var claim *resourceOutcomeClaim
	err := worker.database.Transaction(ctx, func(txCtx context.Context, _ gdb.TX) error {
		if _, err := worker.database.Exec(txCtx, `UPDATE plugin_idc_core_product_fulfillment_resource_outcome_outbox SET status_code='manual_attention',lease_token='',lease_expires_at=NULL,last_error_code='resource_outcome_delivery_exhausted',updated_at=CURRENT_TIMESTAMP WHERE tenant_id=0 AND status_code='delivering' AND lease_expires_at<=CURRENT_TIMESTAMP AND attempt_count>=10`); err != nil {
			return err
		}
		var row *struct {
			TaskNo, ReportPayload, ReportDigest string
			AttemptCount                        int
		}
		if err := worker.database.GetScan(txCtx, &row, `SELECT task_no,report_payload,report_digest,attempt_count FROM plugin_idc_core_product_fulfillment_resource_outcome_outbox WHERE tenant_id=0 AND attempt_count<10 AND ((status_code IN ('pending','retry_wait') AND next_attempt_at<=CURRENT_TIMESTAMP) OR (status_code='delivering' AND lease_expires_at<=CURRENT_TIMESTAMP)) ORDER BY next_attempt_at,task_no LIMIT 1 FOR UPDATE SKIP LOCKED`); err != nil || row == nil {
			return err
		}
		lease, err := resourceOutcomeLease()
		if err != nil {
			return err
		}
		result, err := worker.database.Exec(txCtx, `UPDATE plugin_idc_core_product_fulfillment_resource_outcome_outbox SET status_code='delivering',attempt_count=attempt_count+1,lease_token=?,lease_expires_at=CURRENT_TIMESTAMP+INTERVAL '60 seconds',updated_at=CURRENT_TIMESTAMP WHERE tenant_id=0 AND task_no=? AND attempt_count=? AND status_code IN ('pending','retry_wait','delivering')`, lease, row.TaskNo, row.AttemptCount)
		if err != nil || affected(result) != 1 {
			return errorsNew("product_fulfillment_resource_outcome_claim_conflict")
		}
		claim = &resourceOutcomeClaim{TaskNo: row.TaskNo, LeaseToken: lease, ReportPayload: row.ReportPayload, ReportDigest: row.ReportDigest, AttemptCount: row.AttemptCount + 1}
		return nil
	})
	return claim, err
}

func (worker *resourceOutcomeWorker) finish(ctx context.Context, claim *resourceOutcomeClaim, deliveryErr error) (bool, error) {
	status, errorCode, manual := resourceOutcomeDeliveryState(claim.AttemptCount, deliveryErr)
	result, err := worker.database.Exec(ctx, `UPDATE plugin_idc_core_product_fulfillment_resource_outcome_outbox SET status_code=?,next_attempt_at=CURRENT_TIMESTAMP+INTERVAL '30 seconds',lease_token='',lease_expires_at=NULL,last_error_code=?,reported_at=CASE WHEN ?='reported' THEN CURRENT_TIMESTAMP ELSE NULL END,updated_at=CURRENT_TIMESTAMP WHERE tenant_id=0 AND task_no=? AND status_code='delivering' AND lease_token=?`, status, errorCode, status, claim.TaskNo, claim.LeaseToken)
	if err != nil || affected(result) != 1 {
		return false, errorsNew("product_fulfillment_resource_outcome_finish_conflict")
	}
	return manual, nil
}

func resourceOutcomeDeliveryState(attempt int, deliveryErr error) (string, string, bool) {
	if deliveryErr == nil {
		return "reported", "", false
	}
	if attempt >= 10 {
		return "manual_attention", "resource_outcome_delivery_exhausted", true
	}
	return "retry_wait", "resource_outcome_delivery_failed", false
}

func resourceOutcomeLease() (string, error) {
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return "pro_" + hex.EncodeToString(raw), nil
}
