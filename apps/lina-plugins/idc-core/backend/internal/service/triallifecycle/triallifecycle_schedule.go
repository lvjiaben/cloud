// This file discovers bounded expired local trials and freezes one task per
// entitlement. Existing terminal work is retained for investigation, never reset.

package triallifecycle

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-plugin-idc-core/backend/cap/lifecycleresourcecap"
	"lina-plugin-idc-core/backend/internal/service/asynctask"
	"lina-plugin-idc-core/backend/internal/service/catalog"
)

// Trial task values are private machine identities; no human-facing enum is added.
const (
	taskType             = "trial.entitlement.suspend"
	resumeTaskType       = "trial.entitlement.resume"
	notificationTaskType = "trial.entitlement.notify"
	subjectType          = "service_entitlement"
	resourceOwner        = "idc-resource"
	statusActive         = "active"
	statusSuspended      = "suspended"
	maximumBatch         = 32
	maximumObservations  = 100
)

// expiryRow is the minimum authoritative projection needed to freeze a request.
type expiryRow struct {
	EntitlementNo string
	CustomerNo    string
	Version       int64
	ExpiresAt     time.Time
}

func (s *serviceImpl) Run(ctx context.Context, limit int) error {
	if limit < 1 || limit > maximumBatch {
		return gerror.New("trial lifecycle batch must be between 1 and 32")
	}
	if err := s.schedule(ctx, limit); err != nil {
		return err
	}
	if _, err := s.tasks.RecoverExpired(ctx, asynctask.RecoverExpiredInput{Limit: limit, TaskTypes: []string{taskType, resumeTaskType, notificationTaskType}}); err != nil {
		return err
	}
	tasks, err := s.tasks.Claim(ctx, asynctask.ClaimInput{WorkerID: "trial-entitlement-lifecycle", Limit: limit, TaskTypes: []string{taskType, resumeTaskType, notificationTaskType}})
	if err != nil {
		return err
	}
	var firstError error
	for _, task := range tasks {
		if err = s.process(ctx, task); err != nil && firstError == nil {
			firstError = err
		}
	}
	return firstError
}

func (s *serviceImpl) schedule(ctx context.Context, limit int) error {
	return s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		var rows []expiryRow
		if err := s.database.GetScan(ctx, &rows, `SELECT e.entitlement_no,e.customer_no,e.version,e.expires_at
FROM plugin_idc_core_service_entitlement e
WHERE e.tenant_id=0 AND e.charge_type_code=? AND e.status_code=?
 AND e.entitlement_owner_id=? AND e.upstream_service_no='' AND e.expires_at<=CURRENT_TIMESTAMP
 AND NOT EXISTS (SELECT 1 FROM plugin_idc_core_async_task task
 WHERE task.tenant_id=e.tenant_id AND task.task_type=? AND task.subject_no=e.entitlement_no
 AND (task.status_code<>'succeeded' OR task.result_snapshot->>'outcome' IS DISTINCT FROM 'no_longer_due'
      OR task.payload#>>'{Request,ExpectedServiceVersion}'=e.version::text))
ORDER BY e.expires_at,e.id LIMIT ? FOR UPDATE OF e SKIP LOCKED`, catalog.ChargeTypeTrial, statusActive, resourceOwner, taskType, limit); err != nil {
			return gerror.Wrap(err, "discover expired trial entitlements")
		}
		for _, row := range rows {
			// A cancelled, never-dispatched intent must not permanently exempt a
			// trial after an unrelated entitlement revision. Failed or dispatched
			// intents still block redispatch and require reconciliation.
			seed := sha256.Sum256([]byte(fmt.Sprintf("%s:%s:%d:%s", taskType, row.EntitlementNo, row.Version, row.ExpiresAt.UTC().Format(time.RFC3339Nano))))
			digest := hex.EncodeToString(seed[:])
			request := lifecycleresourcecap.TaskRequest{LifecycleTaskNo: "TLT_" + digest[:32], ServiceNo: row.EntitlementNo,
				CustomerNo: row.CustomerNo, Action: lifecycleresourcecap.ActionSuspend, ExpectedServiceVersion: row.Version,
				ExpectedExpiresAt: row.ExpiresAt, IdempotencyDigest: digest}
			if err := lifecycleresourcecap.ValidateTaskRequest(request); err != nil {
				return err
			}
			payload, err := json.Marshal(taskPayload{Request: request})
			if err != nil {
				return gerror.Wrap(err, "freeze trial lifecycle request")
			}
			if _, err = s.tasks.CreateInDatabase(ctx, s.database, asynctask.CreateInput{
				TaskType: taskType, SubjectType: subjectType, SubjectNo: row.EntitlementNo,
				CustomerNo: row.CustomerNo, IdempotencyKey: digest, Payload: payload, MaxAttempts: 12,
			}); err != nil {
				return err
			}
		}
		return nil
	})
}
