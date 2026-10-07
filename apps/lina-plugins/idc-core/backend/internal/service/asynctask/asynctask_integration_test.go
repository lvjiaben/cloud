// This file verifies asynchronous-task transactions and leases against an explicitly selected PostgreSQL database.

package asynctask

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	_ "github.com/gogf/gf/contrib/drivers/pgsql/v2"
	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/internal/dao"
	"lina-plugin-idc-core/backend/internal/model/do"
	"lina-plugin-idc-core/backend/internal/model/entity"
)

// asyncTaskIntegrationDSNEnv opts tests into a disposable PostgreSQL database.
const asyncTaskIntegrationDSNEnv = "LINAPRO_IDC_CORE_TEST_DSN"

// integrationClock provides deterministic, concurrency-safe task time.
type integrationClock struct {
	mutex sync.RWMutex
	value time.Time
}

// Now returns the current deterministic test time.
func (c *integrationClock) Now() time.Time {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	return c.value
}

// Set moves the deterministic test time to one exact instant.
func (c *integrationClock) Set(value time.Time) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.value = databaseTime(value)
}

// Advance moves the deterministic test time forward by one duration.
func (c *integrationClock) Advance(duration time.Duration) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.value = databaseTime(c.value.Add(duration))
}

// asyncTaskIntegrationHarness owns only random task rows created by one test.
type asyncTaskIntegrationHarness struct {
	ctx       context.Context
	service   *serviceImpl
	clock     *integrationClock
	mutex     sync.Mutex
	taskNos   []string
	uniqueTag string
}

// TestCreateIsTransactionalAndIdempotentPostgreSQL proves transaction gating,
// replay convergence, immutable conflict detection, and concurrent creation.
func TestCreateIsTransactionalAndIdempotentPostgreSQL(t *testing.T) {
	harness := newAsyncTaskIntegrationHarness(t)
	input := harness.createInput("sequential", 3, nil)
	first, err := harness.createInTransaction(input)
	if err != nil {
		t.Fatalf("create integration task failed: %v", err)
	}
	harness.track(first.Task.TaskNo)
	if first.Reused || !strings.HasPrefix(first.Task.TaskNo, taskNumberPrefix) {
		t.Fatalf("unexpected first create result: %+v", first)
	}
	second, err := harness.createInTransaction(input)
	if err != nil {
		t.Fatalf("replay integration task failed: %v", err)
	}
	if !second.Reused || second.Task.TaskNo != first.Task.TaskNo {
		t.Fatalf("replay did not converge: first=%+v second=%+v", first, second)
	}
	changed := input
	changed.Payload = json.RawMessage(`{"schema_version":1,"changed":true}`)
	if _, err = harness.createInTransaction(changed); !bizerr.Is(err, CodeIdempotencyConflict) {
		t.Fatalf("changed idempotent request should conflict, got %v", err)
	}

	var stored *entity.AsyncTask
	if err = dao.AsyncTask.Ctx(harness.ctx).
		Where(do.AsyncTask{TenantId: idccap.DefaultTenantID, TaskNo: first.Task.TaskNo}).
		Scan(&stored); err != nil {
		t.Fatalf("load stored asynchronous task failed: %v", err)
	}
	if stored == nil || stored.IdempotencyKeyDigest != digest(input.IdempotencyKey) ||
		stored.IdempotencyKeyDigest == input.IdempotencyKey {
		t.Fatalf("raw idempotency key was not safely digested: %+v", stored)
	}
	assertAsyncTaskIntegrationCount(t, harness.ctx, dao.AsyncTask.Table(), "task_no", first.Task.TaskNo, 1)
	assertAsyncTaskIntegrationCount(t, harness.ctx, dao.BusinessNumber.Table(), "business_no", first.Task.TaskNo, 1)

	implicitAttemptsInput := harness.createInput("implicit-attempt-default", 0, nil)
	implicitAttemptsCreated, err := harness.createInTransaction(implicitAttemptsInput)
	if err != nil {
		t.Fatalf("create implicit-attempt task failed: %v", err)
	}
	harness.track(implicitAttemptsCreated.Task.TaskNo)
	reconfiguredService, err := newService(
		Config{DefaultMaxAttempts: 5},
		harness.clock.Now,
		rand.Reader,
	)
	if err != nil {
		t.Fatalf("create reconfigured task service failed: %v", err)
	}
	var implicitAttemptsReplayed *CreateResult
	err = dao.AsyncTask.Transaction(harness.ctx, func(ctx context.Context, _ gdb.TX) error {
		implicitAttemptsReplayed, err = reconfiguredService.Create(ctx, implicitAttemptsInput)
		return err
	})
	if err != nil || !implicitAttemptsReplayed.Reused ||
		implicitAttemptsReplayed.Task.TaskNo != implicitAttemptsCreated.Task.TaskNo ||
		implicitAttemptsReplayed.Task.MaxAttempts != implicitAttemptsCreated.Task.MaxAttempts {
		t.Fatalf(
			"implicit attempt default changed an idempotent replay: created=%+v replayed=%+v err=%v",
			implicitAttemptsCreated,
			implicitAttemptsReplayed,
			err,
		)
	}
	explicitAttemptsInput := implicitAttemptsInput
	explicitAttemptsInput.MaxAttempts = 5
	err = dao.AsyncTask.Transaction(harness.ctx, func(ctx context.Context, _ gdb.TX) error {
		_, createErr := reconfiguredService.Create(ctx, explicitAttemptsInput)
		return createErr
	})
	if !bizerr.Is(err, CodeIdempotencyConflict) {
		t.Fatalf("explicitly changed task attempt budget should conflict, got %v", err)
	}

	deadline := harness.clock.Now().Add(time.Minute)
	deadlineReplayInput := harness.createInput("expired-deadline-replay", 3, &deadline)
	deadlineReplayInput.AvailableAt = time.Time{}
	deadlineCreated, err := harness.createInTransaction(deadlineReplayInput)
	if err != nil {
		t.Fatalf("create deadline replay fixture failed: %v", err)
	}
	harness.track(deadlineCreated.Task.TaskNo)
	harness.clock.Advance(2 * time.Minute)
	deadlineReplayed, err := harness.createInTransaction(deadlineReplayInput)
	if err != nil {
		t.Fatalf("replay committed task after its deadline failed: %v", err)
	}
	if !deadlineReplayed.Reused || deadlineReplayed.Task.TaskNo != deadlineCreated.Task.TaskNo {
		t.Fatalf(
			"expired deadline replay did not converge: created=%+v replayed=%+v",
			deadlineCreated,
			deadlineReplayed,
		)
	}
	expiredNewInput := harness.createInput("new-expired-deadline", 3, &deadline)
	if _, err = harness.createInTransaction(expiredNewInput); !bizerr.Is(err, CodeInvalid) {
		t.Fatalf("new task with expired deadline should be invalid, got %v", err)
	}
	rollbackInput := harness.createInput("rollback", 3, nil)
	rollbackSentinel := errors.New("force outer transaction rollback")
	var rolledBack *CreateResult
	err = dao.AsyncTask.Transaction(harness.ctx, func(ctx context.Context, _ gdb.TX) error {
		rolledBack, err = harness.service.Create(ctx, rollbackInput)
		if err != nil {
			return err
		}
		return rollbackSentinel
	})
	if !errors.Is(err, rollbackSentinel) || rolledBack == nil {
		t.Fatalf("outer transaction did not return rollback sentinel: task=%+v err=%v", rolledBack, err)
	}
	harness.track(rolledBack.Task.TaskNo)
	assertAsyncTaskIntegrationCount(t, harness.ctx, dao.AsyncTask.Table(), "task_no", rolledBack.Task.TaskNo, 0)
	assertAsyncTaskIntegrationCount(t, harness.ctx, dao.BusinessNumber.Table(), "business_no", rolledBack.Task.TaskNo, 0)

	concurrentInput := harness.createInput("concurrent", 3, nil)
	const callers = 6
	var (
		waitGroup    sync.WaitGroup
		start        = make(chan struct{})
		results      = make([]*CreateResult, callers)
		createErrors = make([]error, callers)
	)
	for index := range callers {
		index := index
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			<-start
			results[index], createErrors[index] = harness.createInTransaction(concurrentInput)
		}()
	}
	close(start)
	waitGroup.Wait()
	createdCount := 0
	convergedTaskNo := ""
	for index := range callers {
		if createErrors[index] != nil {
			t.Fatalf("concurrent create %d failed: %v", index, createErrors[index])
		}
		if convergedTaskNo == "" {
			convergedTaskNo = results[index].Task.TaskNo
		}
		if results[index].Task.TaskNo != convergedTaskNo {
			t.Fatalf("concurrent create diverged: %q and %q", convergedTaskNo, results[index].Task.TaskNo)
		}
		if !results[index].Reused {
			createdCount++
		}
	}
	if createdCount != 1 {
		t.Fatalf("expected one concurrent creator, got %d", createdCount)
	}
	harness.track(convergedTaskNo)
	assertAsyncTaskIntegrationCount(t, harness.ctx, dao.AsyncTask.Table(), "task_no", convergedTaskNo, 1)
	assertAsyncTaskIntegrationCount(t, harness.ctx, dao.BusinessNumber.Table(), "business_no", convergedTaskNo, 1)
}

// TestClaimSkipLockedAndLeaseFencePostgreSQL proves concurrent claims do not
// duplicate work and every transition uses the opaque live-lease fence.
func TestClaimSkipLockedAndLeaseFencePostgreSQL(t *testing.T) {
	harness := newAsyncTaskIntegrationHarness(t)
	const taskCount = 8
	for index := range taskCount {
		result := harness.create(t, "claim-"+string(rune('a'+index)), 3, nil)
		if result.Task.Status != StatusPending {
			t.Fatalf("created task has unexpected status: %+v", result.Task)
		}
	}

	type workerClaim struct {
		worker string
		tasks  []ClaimedTask
		err    error
	}
	claims := []workerClaim{{worker: "worker-alpha"}, {worker: "worker-beta"}}
	start := make(chan struct{})
	var waitGroup sync.WaitGroup
	for index := range claims {
		index := index
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			<-start
			claims[index].tasks, claims[index].err = harness.service.Claim(
				harness.ctx,
				ClaimInput{WorkerID: claims[index].worker, Limit: taskCount / 2},
			)
		}()
	}
	close(start)
	waitGroup.Wait()

	claimedByTask := make(map[string]string, taskCount)
	claimedTask := ClaimedTask{}
	for _, claim := range claims {
		if claim.err != nil {
			t.Fatalf("worker %s claim failed: %v", claim.worker, claim.err)
		}
		if len(claim.tasks) != taskCount/2 {
			t.Fatalf("worker %s claimed %d tasks, want %d", claim.worker, len(claim.tasks), taskCount/2)
		}
		for _, task := range claim.tasks {
			if prior := claimedByTask[task.TaskNo]; prior != "" {
				t.Fatalf("task %s was claimed by both %s and %s", task.TaskNo, prior, claim.worker)
			}
			claimedByTask[task.TaskNo] = claim.worker
			if task.Status != StatusRunning || task.Lease.AttemptNo != 1 ||
				!strings.HasPrefix(task.Lease.LeaseToken, leaseTokenPrefix) ||
				task.Lease.LeaseToken == claim.worker {
				t.Fatalf("worker %s received invalid lease: %+v", claim.worker, task)
			}
			if claimedTask.TaskNo == "" {
				claimedTask = task
			}
		}
	}
	if len(claimedByTask) != taskCount {
		t.Fatalf("claimed %d distinct tasks, want %d", len(claimedByTask), taskCount)
	}
	for taskNo, workerID := range claimedByTask {
		attempt := loadAsyncTaskIntegrationAttempt(t, harness.ctx, taskNo, 1)
		if attempt.WorkerId != workerID || attempt.StatusCode != string(AttemptStatusRunning) {
			t.Fatalf("attempt worker evidence mismatch for %s: %+v want worker %s", taskNo, attempt, workerID)
		}
	}

	foreignLease := claimedTask.Lease
	foreignLease.LeaseToken = "LSE_ffffffffffffffffffffffffffffffff"
	if _, err := harness.service.Complete(harness.ctx, CompleteInput{
		Lease:          foreignLease,
		ResultSnapshot: json.RawMessage(`{"provider_status":"done"}`),
	}); !bizerr.Is(err, CodeLeaseLost) {
		t.Fatalf("foreign lease should be fenced, got %v", err)
	}
	staleAttempt := claimedTask.Lease
	staleAttempt.AttemptNo++
	if _, err := harness.service.Complete(harness.ctx, CompleteInput{
		Lease:          staleAttempt,
		ResultSnapshot: json.RawMessage(`{"provider_status":"done"}`),
	}); !bizerr.Is(err, CodeLeaseLost) {
		t.Fatalf("stale attempt should be fenced, got %v", err)
	}
	if _, err := harness.service.Complete(harness.ctx, CompleteInput{
		Lease:          claimedTask.Lease,
		ResultSnapshot: json.RawMessage(`{"accessToken":"plaintext"}`),
	}); !bizerr.Is(err, CodeInvalid) {
		t.Fatalf("secret-bearing completion should be invalid, got %v", err)
	}
	if _, err := harness.service.Complete(harness.ctx, CompleteInput{
		Lease:          claimedTask.Lease,
		ResultSnapshot: json.RawMessage(`{}`),
	}); !bizerr.Is(err, CodeInvalid) {
		t.Fatalf("empty completion evidence should be invalid, got %v", err)
	}

	initialExpiry := claimedTask.LeaseExpiresAt
	harness.clock.Advance(5 * time.Second)
	renewed, err := harness.service.Renew(harness.ctx, claimedTask.Lease)
	if err != nil {
		t.Fatalf("renew live lease failed: %v", err)
	}
	if !renewed.LeaseExpiresAt.After(initialExpiry) || renewed.Lease != claimedTask.Lease {
		t.Fatalf("renew did not extend the same fence: before=%s after=%+v", initialExpiry, renewed)
	}
	completed, err := harness.service.Complete(harness.ctx, CompleteInput{
		Lease:               renewed.Lease,
		ExternalReferenceNo: "provider-operation-1",
		ResultSnapshot:      json.RawMessage(`{"provider_status":"done"}`),
	})
	if err != nil {
		t.Fatalf("complete live lease failed: %v", err)
	}
	if completed.Status != StatusSucceeded || completed.CompletedAt == nil ||
		string(completed.ResultSnapshot) != `{"provider_status":"done"}` {
		t.Fatalf("unexpected completed task: %+v", completed)
	}
	stored := loadAsyncTaskIntegrationTask(t, harness.ctx, completed.TaskNo)
	if stored.LeaseExpiresAt != nil || stored.LockedBy != "" || stored.StatusCode != string(StatusSucceeded) {
		t.Fatalf("completion did not clear lease: %+v", stored)
	}
	attempt := loadAsyncTaskIntegrationAttempt(t, harness.ctx, completed.TaskNo, 1)
	if attempt.StatusCode != string(AttemptStatusSucceeded) || attempt.EndedAt == nil ||
		attempt.ExternalReferenceNo != "provider-operation-1" {
		t.Fatalf("completion did not close attempt evidence: %+v", attempt)
	}
	if _, err = harness.service.Complete(harness.ctx, CompleteInput{
		Lease:          renewed.Lease,
		ResultSnapshot: json.RawMessage(`{"provider_status":"done"}`),
	}); !bizerr.Is(err, CodeLeaseLost) {
		t.Fatalf("completed lease should be stale, got %v", err)
	}
}

// TestClaimTaskTypeFilterPostgreSQL proves a domain worker never consumes a
// different domain's ready task while the generic claim path remains intact.
func TestClaimTaskTypeFilterPostgreSQL(t *testing.T) {
	harness := newAsyncTaskIntegrationHarness(t)
	exportInput := harness.createInput("filtered-export", 3, nil)
	exportInput.TaskType = "customer.data_export"
	exportTask, err := harness.createInTransaction(exportInput)
	if err != nil {
		t.Fatalf("create customer export task failed: %v", err)
	}
	harness.track(exportTask.Task.TaskNo)
	provisionInput := harness.createInput("filtered-provision", 3, nil)
	provisionTask, err := harness.createInTransaction(provisionInput)
	if err != nil {
		t.Fatalf("create provision task failed: %v", err)
	}
	harness.track(provisionTask.Task.TaskNo)

	claimed, err := harness.service.Claim(harness.ctx, ClaimInput{
		WorkerID:  "customer-export-worker",
		Limit:     2,
		TaskTypes: []string{"customer.data_export"},
	})
	if err != nil {
		t.Fatalf("claim customer export task failed: %v", err)
	}
	if len(claimed) != 1 || claimed[0].TaskNo != exportTask.Task.TaskNo ||
		claimed[0].TaskType != "customer.data_export" {
		t.Fatalf("filtered worker claimed the wrong tasks: %+v", claimed)
	}

	generic, err := harness.service.Claim(harness.ctx, ClaimInput{
		WorkerID: "generic-worker",
		Limit:    2,
	})
	if err != nil {
		t.Fatalf("generic claim failed: %v", err)
	}
	if len(generic) != 1 || generic[0].TaskNo != provisionTask.Task.TaskNo ||
		generic[0].TaskType != provisionInput.TaskType {
		t.Fatalf("generic worker did not retain the unfiltered behavior: %+v", generic)
	}
}

// TestCompleteInDatabasePostgreSQL proves a caller-owned business transaction
// commits or rolls back task and attempt evidence as one unit.
func TestCompleteInDatabasePostgreSQL(t *testing.T) {
	harness := newAsyncTaskIntegrationHarness(t)
	created := harness.create(t, "explicit-complete", 3, nil)
	claimed := harness.claimOne(t, "explicit-complete-worker")
	if claimed.TaskNo != created.Task.TaskNo {
		t.Fatalf("claimed task %s, want %s", claimed.TaskNo, created.Task.TaskNo)
	}
	input := CompleteInput{
		Lease: claimed.Lease, ExternalReferenceNo: "entitlement-explicit-complete",
		ResultSnapshot: json.RawMessage(`{"result":"terminated"}`),
	}
	if _, err := harness.service.CompleteInDatabase(harness.ctx, g.DB(), input); !bizerr.Is(err, CodeTransactionRequired) {
		t.Fatalf("explicit completion without transaction error=%v", err)
	}
	rollback := errors.New("rollback explicit completion")
	err := g.DB().Transaction(harness.ctx, func(txCtx context.Context, _ gdb.TX) error {
		completed, completeErr := harness.service.CompleteInDatabase(txCtx, g.DB(), input)
		if completeErr != nil || completed == nil || completed.Status != StatusSucceeded {
			t.Fatalf("complete in rollback transaction result=%+v err=%v", completed, completeErr)
		}
		return rollback
	})
	if !errors.Is(err, rollback) {
		t.Fatalf("explicit completion rollback error=%v", err)
	}
	stored := loadAsyncTaskIntegrationTask(t, harness.ctx, claimed.TaskNo)
	attempt := loadAsyncTaskIntegrationAttempt(t, harness.ctx, claimed.TaskNo, claimed.Lease.AttemptNo)
	if stored.StatusCode != string(StatusRunning) || attempt.StatusCode != string(AttemptStatusRunning) {
		t.Fatalf("rolled-back explicit completion leaked evidence: task=%+v attempt=%+v", stored, attempt)
	}
	err = g.DB().Transaction(harness.ctx, func(txCtx context.Context, _ gdb.TX) error {
		completed, completeErr := harness.service.CompleteInDatabase(txCtx, g.DB(), input)
		if completeErr != nil || completed == nil || completed.Status != StatusSucceeded {
			return fmt.Errorf("commit explicit completion result=%+v: %w", completed, completeErr)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	stored = loadAsyncTaskIntegrationTask(t, harness.ctx, claimed.TaskNo)
	attempt = loadAsyncTaskIntegrationAttempt(t, harness.ctx, claimed.TaskNo, claimed.Lease.AttemptNo)
	if stored.StatusCode != string(StatusSucceeded) || attempt.StatusCode != string(AttemptStatusSucceeded) ||
		stored.LeaseExpiresAt != nil || stored.LockedBy != "" {
		t.Fatalf("committed explicit completion evidence is incomplete: task=%+v attempt=%+v", stored, attempt)
	}
}

// TestFailureUnknownAndManualTakeoverPostgreSQL proves retry scheduling,
// terminal classifications, unknown freezing, and evidence-preserving takeover.
func TestFailureUnknownAndManualTakeoverPostgreSQL(t *testing.T) {
	harness := newAsyncTaskIntegrationHarness(t)
	retryTask := harness.create(t, "retry", 2, nil)
	first := harness.claimOne(t, "worker-retry-1")
	if first.TaskNo != retryTask.Task.TaskNo {
		t.Fatalf("claimed %s, want retry task %s", first.TaskNo, retryTask.Task.TaskNo)
	}
	failureTime := harness.clock.Now()
	retry, err := harness.service.Fail(harness.ctx, FailureInput{
		Lease:               first.Lease,
		Retryable:           true,
		ExternalReferenceNo: "provider-attempt-1",
		ResponseSnapshot:    json.RawMessage(`{"provider_status":"capacity"}`),
		ErrorCode:           "provider_capacity",
		ErrorSummary:        "Provider temporarily has no capacity",
	})
	if err != nil {
		t.Fatalf("record retryable failure failed: %v", err)
	}
	expectedAvailable := databaseTime(failureTime.Add(harness.service.retryBackoff(retry.TaskNo, 1)))
	if retry.Status != StatusRetryWait || retry.CompletedAt != nil || !retry.AvailableAt.Equal(expectedAvailable) {
		t.Fatalf("unexpected retry state: %+v want available %s", retry, expectedAvailable)
	}
	storedRetry := loadAsyncTaskIntegrationTask(t, harness.ctx, retry.TaskNo)
	if storedRetry.LeaseExpiresAt != nil || storedRetry.LockedBy != "" {
		t.Fatalf("retry transition did not clear lease: %+v", storedRetry)
	}
	harness.clock.Set(retry.AvailableAt.Add(time.Microsecond))
	second := harness.claimOne(t, "worker-retry-2")
	if second.TaskNo != retry.TaskNo || second.Lease.AttemptNo != 2 {
		t.Fatalf("retry claim did not advance attempt: %+v", second)
	}
	deadLetter, err := harness.service.Fail(harness.ctx, FailureInput{
		Lease:            second.Lease,
		Retryable:        true,
		ResponseSnapshot: json.RawMessage(`{"provider_status":"capacity"}`),
		ErrorCode:        "provider_capacity",
		ErrorSummary:     "Provider still has no capacity",
	})
	if err != nil {
		t.Fatalf("record exhausted failure failed: %v", err)
	}
	if deadLetter.Status != StatusDeadLetter || deadLetter.CompletedAt == nil || deadLetter.AttemptCount != 2 {
		t.Fatalf("retry exhaustion did not dead-letter: %+v", deadLetter)
	}
	beforeTakeoverSnapshot := string(deadLetter.ResultSnapshot)
	taken, err := harness.service.TakeOver(harness.ctx, TakeOverInput{
		TenantID:       idccap.DefaultTenantID,
		TaskNo:         deadLetter.TaskNo,
		AssigneeUserID: 1001,
		Reason:         "Operator will query provider capacity evidence",
	})
	if err != nil {
		t.Fatalf("take over dead-letter task failed: %v", err)
	}
	if taken.Status != StatusManualTakeover || taken.ManualTakeoverAt == nil ||
		taken.ManualAssigneeUserID != 1001 || string(taken.ResultSnapshot) != beforeTakeoverSnapshot {
		t.Fatalf("manual takeover changed evidence or assignment: %+v", taken)
	}
	if _, err = harness.service.TakeOver(harness.ctx, TakeOverInput{
		TenantID:       idccap.DefaultTenantID,
		TaskNo:         deadLetter.TaskNo,
		AssigneeUserID: 1002,
		Reason:         "Second takeover must be rejected",
	}); !bizerr.Is(err, CodeStateConflict) {
		t.Fatalf("second takeover should conflict, got %v", err)
	}

	nonRetryable := harness.create(t, "nonretryable", 3, nil)
	nonRetryClaim := harness.claimOne(t, "worker-terminal")
	if nonRetryClaim.TaskNo != nonRetryable.Task.TaskNo {
		t.Fatalf("claimed %s, want non-retryable task %s", nonRetryClaim.TaskNo, nonRetryable.Task.TaskNo)
	}
	failed, err := harness.service.Fail(harness.ctx, FailureInput{
		Lease:            nonRetryClaim.Lease,
		ResponseSnapshot: json.RawMessage(`{}`),
		ErrorCode:        "request_rejected",
		ErrorSummary:     "Provider rejected the normalized request",
	})
	if err != nil || failed.Status != StatusFailed || failed.CompletedAt == nil {
		t.Fatalf("non-retryable failure was not terminal: task=%+v err=%v", failed, err)
	}

	unknownTask := harness.create(t, "unknown", 3, nil)
	unknownClaim := harness.claimOne(t, "worker-unknown")
	if unknownClaim.TaskNo != unknownTask.Task.TaskNo {
		t.Fatalf("claimed %s, want unknown task %s", unknownClaim.TaskNo, unknownTask.Task.TaskNo)
	}
	unknown, err := harness.service.MarkResultUnknown(harness.ctx, UnknownInput{
		Lease:            unknownClaim.Lease,
		ResponseSnapshot: json.RawMessage(`{}`),
		ErrorCode:        "provider_timeout",
		ErrorSummary:     "Provider timed out after accepting the request",
	})
	if err != nil || unknown.Status != StatusResultUnknown || unknown.CompletedAt == nil {
		t.Fatalf("unknown result did not freeze task: task=%+v err=%v", unknown, err)
	}
	remaining, err := harness.service.Claim(harness.ctx, ClaimInput{WorkerID: "worker-no-retry", Limit: 10})
	if err != nil || len(remaining) != 0 {
		t.Fatalf("result-unknown task was automatically retried: tasks=%+v err=%v", remaining, err)
	}
	unknownSnapshot := string(unknown.ResultSnapshot)
	unknownTaken, err := harness.service.TakeOver(harness.ctx, TakeOverInput{
		TenantID:       idccap.DefaultTenantID,
		TaskNo:         unknown.TaskNo,
		AssigneeUserID: 1003,
		Reason:         "Operator will query before any new write",
	})
	if err != nil || unknownTaken.Status != StatusManualTakeover ||
		string(unknownTaken.ResultSnapshot) != unknownSnapshot {
		t.Fatalf("unknown takeover changed evidence: task=%+v err=%v", unknownTaken, err)
	}
}

// TestRetryLeaseExpiryDoesNotReusePreviousAttemptEvidencePostgreSQL proves a
// new attempt starts with an empty current-evidence projection. If that new
// attempt expires, recovery cannot misattribute an earlier failed attempt's
// external reference or response to the result-unknown task.
func TestRetryLeaseExpiryDoesNotReusePreviousAttemptEvidencePostgreSQL(t *testing.T) {
	harness := newAsyncTaskIntegrationHarness(t)
	created := harness.create(t, "retry-expired-evidence", 3, nil)
	first := harness.claimOne(t, "worker-evidence-1")
	if first.TaskNo != created.Task.TaskNo {
		t.Fatalf("claimed %s, want evidence task %s", first.TaskNo, created.Task.TaskNo)
	}
	retry, err := harness.service.Fail(harness.ctx, FailureInput{
		Lease:               first.Lease,
		Retryable:           true,
		ExternalReferenceNo: "provider-old-attempt",
		ResponseSnapshot:    json.RawMessage(`{"provider_status":"old_failure"}`),
		ErrorCode:           "provider_capacity",
		ErrorSummary:        "Provider temporarily has no capacity",
	})
	if err != nil {
		t.Fatalf("record first retryable failure: %v", err)
	}
	harness.clock.Set(retry.AvailableAt.Add(time.Microsecond))
	second := harness.claimOne(t, "worker-evidence-2")
	if second.Lease.AttemptNo != 2 || second.ExternalReferenceNo != "" ||
		string(second.ResultSnapshot) != `{}` {
		t.Fatalf("new attempt retained stale current evidence: %+v", second)
	}

	harness.clock.Advance(harness.service.config.LeaseDuration + time.Microsecond)
	recovered, err := harness.service.RecoverExpired(harness.ctx, RecoverExpiredInput{Limit: 1})
	if err != nil || len(recovered) != 1 {
		t.Fatalf("recover second attempt: tasks=%+v err=%v", recovered, err)
	}
	if recovered[0].TaskNo != created.Task.TaskNo ||
		recovered[0].ExternalReferenceNo != "" ||
		string(recovered[0].ResultSnapshot) != `{}` {
		t.Fatalf("result-unknown task reused stale evidence: %+v", recovered[0])
	}
	stored := loadAsyncTaskIntegrationTask(t, harness.ctx, created.Task.TaskNo)
	if stored.ExternalReferenceNo != "" || stored.ResultSnapshot != `{}` {
		t.Fatalf("stored task retained stale current evidence: %+v", stored)
	}
	firstAttempt := loadAsyncTaskIntegrationAttempt(t, harness.ctx, created.Task.TaskNo, 1)
	if firstAttempt.ExternalReferenceNo != "provider-old-attempt" ||
		firstAttempt.ResponseSnapshot != `{"provider_status": "old_failure"}` {
		t.Fatalf("historical first-attempt evidence was not preserved: %+v", firstAttempt)
	}
	secondAttempt := loadAsyncTaskIntegrationAttempt(t, harness.ctx, created.Task.TaskNo, 2)
	if secondAttempt.StatusCode != string(AttemptStatusResultUnknown) ||
		secondAttempt.ExternalReferenceNo != "" || secondAttempt.ResponseSnapshot != `{}` {
		t.Fatalf("expired second attempt contains stale evidence: %+v", secondAttempt)
	}
}

// TestExpiredRecoveryRejectsAttemptFenceDriftPostgreSQL proves recovery fails
// closed when imported or manually repaired rows disagree about the current
// running attempt. It must not close a historical attempt by task number alone.
func TestExpiredRecoveryRejectsAttemptFenceDriftPostgreSQL(t *testing.T) {
	harness := newAsyncTaskIntegrationHarness(t)
	created := harness.create(t, "expired-attempt-fence-drift", 3, nil)
	claimed := harness.claimOne(t, "worker-fence-drift")
	if claimed.TaskNo != created.Task.TaskNo {
		t.Fatalf("claimed %s, want fence-drift task %s", claimed.TaskNo, created.Task.TaskNo)
	}
	harness.clock.Advance(harness.service.config.LeaseDuration + time.Microsecond)
	result, err := dao.AsyncTask.Ctx(harness.ctx).
		Where(do.AsyncTask{TenantId: idccap.DefaultTenantID, TaskNo: created.Task.TaskNo}).
		Data(do.AsyncTask{AttemptCount: claimed.Lease.AttemptNo + 1}).
		Update()
	if err != nil {
		t.Fatalf("inject task-attempt fence drift: %v", err)
	}
	if err = requireAffected(result, 1, "inject task-attempt fence drift"); err != nil {
		t.Fatal(err)
	}

	recovered, err := harness.service.RecoverExpired(harness.ctx, RecoverExpiredInput{Limit: 1})
	if recovered != nil || !bizerr.Is(err, CodeUnavailable) {
		t.Fatalf("attempt fence drift should fail closed: tasks=%+v err=%v", recovered, err)
	}
	storedTask := loadAsyncTaskIntegrationTask(t, harness.ctx, created.Task.TaskNo)
	if storedTask.StatusCode != string(StatusRunning) || storedTask.LockedBy == "" ||
		storedTask.LeaseExpiresAt == nil {
		t.Fatalf("failed recovery mutated task: %+v", storedTask)
	}
	storedAttempt := loadAsyncTaskIntegrationAttempt(t, harness.ctx, created.Task.TaskNo, 1)
	if storedAttempt.StatusCode != string(AttemptStatusRunning) || storedAttempt.EndedAt != nil {
		t.Fatalf("failed recovery closed the wrong attempt: %+v", storedAttempt)
	}
}

// TestExpiredDeadlineAndExhaustedRecoveryPostgreSQL proves expired leases never
// write, recovery never retries unknown effects, and malformed ready rows converge.
func TestExpiredDeadlineAndExhaustedRecoveryPostgreSQL(t *testing.T) {
	harness := newAsyncTaskIntegrationHarness(t)
	expiring := harness.create(t, "expired-lease", 3, nil)
	claimed := harness.claimOne(t, "worker-expiring")
	if claimed.TaskNo != expiring.Task.TaskNo {
		t.Fatalf("claimed %s, want expiring task %s", claimed.TaskNo, expiring.Task.TaskNo)
	}
	harness.clock.Advance(harness.service.config.LeaseDuration + time.Microsecond)
	assertLeaseLost := func(operation string, err error) {
		t.Helper()
		if !bizerr.Is(err, CodeLeaseLost) {
			t.Fatalf("%s accepted an expired lease: %v", operation, err)
		}
	}
	_, err := harness.service.Renew(harness.ctx, claimed.Lease)
	assertLeaseLost("renew", err)
	_, err = harness.service.Complete(harness.ctx, CompleteInput{
		Lease:          claimed.Lease,
		ResultSnapshot: json.RawMessage(`{"provider_status":"done"}`),
	})
	assertLeaseLost("complete", err)
	_, err = harness.service.Fail(harness.ctx, FailureInput{
		Lease:            claimed.Lease,
		Retryable:        true,
		ResponseSnapshot: json.RawMessage(`{}`),
		ErrorCode:        "expired_attempt",
		ErrorSummary:     "Expired attempt cannot report failure",
	})
	assertLeaseLost("fail", err)
	_, err = harness.service.MarkResultUnknown(harness.ctx, UnknownInput{
		Lease:            claimed.Lease,
		ResponseSnapshot: json.RawMessage(`{}`),
		ErrorCode:        "expired_attempt",
		ErrorSummary:     "Expired attempt cannot report an unknown result",
	})
	assertLeaseLost("mark unknown", err)

	recovered, err := harness.service.RecoverExpired(harness.ctx, RecoverExpiredInput{Limit: 1})
	if err != nil || len(recovered) != 1 || recovered[0].TaskNo != expiring.Task.TaskNo ||
		recovered[0].Status != StatusResultUnknown {
		t.Fatalf("expired recovery did not freeze task: tasks=%+v err=%v", recovered, err)
	}
	storedRecovered := loadAsyncTaskIntegrationTask(t, harness.ctx, expiring.Task.TaskNo)
	if storedRecovered.LeaseExpiresAt != nil || storedRecovered.LockedBy != "" ||
		storedRecovered.StatusCode != string(StatusResultUnknown) ||
		storedRecovered.LastErrorCode != leaseExpiredErrorCode {
		t.Fatalf("unexpected recovered task row: %+v", storedRecovered)
	}
	recoveredAttempt := loadAsyncTaskIntegrationAttempt(t, harness.ctx, expiring.Task.TaskNo, 1)
	if recoveredAttempt.StatusCode != string(AttemptStatusResultUnknown) ||
		recoveredAttempt.EndedAt == nil || recoveredAttempt.ErrorCode != leaseExpiredErrorCode {
		t.Fatalf("expired attempt was not closed as unknown: %+v", recoveredAttempt)
	}
	claimedAgain, err := harness.service.Claim(harness.ctx, ClaimInput{WorkerID: "worker-after-recovery", Limit: 1})
	if err != nil || len(claimedAgain) != 0 {
		t.Fatalf("recovered unknown task was retried: tasks=%+v err=%v", claimedAgain, err)
	}
	beforeTakeoverSnapshot := storedRecovered.ResultSnapshot
	taken, err := harness.service.TakeOver(harness.ctx, TakeOverInput{
		TenantID:       idccap.DefaultTenantID,
		TaskNo:         expiring.Task.TaskNo,
		AssigneeUserID: 2001,
		Reason:         "Investigate expired provider execution",
	})
	if err != nil || taken.Status != StatusManualTakeover ||
		string(taken.ResultSnapshot) != beforeTakeoverSnapshot {
		t.Fatalf("recovered task takeover changed evidence: task=%+v err=%v", taken, err)
	}

	deadline := harness.clock.Now().Add(10 * time.Second)
	deadlineTask := harness.create(t, "deadline", 3, &deadline)
	harness.clock.Advance(10*time.Second + time.Microsecond)
	claimedAfterDeadline, err := harness.service.Claim(
		harness.ctx,
		ClaimInput{WorkerID: "worker-deadline", Limit: 1},
	)
	if err != nil || len(claimedAfterDeadline) != 0 {
		t.Fatalf("deadline-expired task was claimed: tasks=%+v err=%v", claimedAfterDeadline, err)
	}
	storedDeadline := loadAsyncTaskIntegrationTask(t, harness.ctx, deadlineTask.Task.TaskNo)
	if storedDeadline.StatusCode != string(StatusFailed) || storedDeadline.CompletedAt == nil ||
		storedDeadline.LastErrorCode != deadlineExceededErrorCode || storedDeadline.LeaseExpiresAt != nil {
		t.Fatalf("deadline task did not converge to known failure: %+v", storedDeadline)
	}

	exhausted := harness.create(t, "migrated-exhausted", 1, nil)
	legacyExpiry := harness.clock.Now().Add(time.Minute)
	if _, err = dao.AsyncTask.Ctx(harness.ctx).
		Where(do.AsyncTask{TenantId: idccap.DefaultTenantID, TaskNo: exhausted.Task.TaskNo}).
		Data(do.AsyncTask{
			AttemptCount:   1,
			LockedBy:       "legacy-lease-token",
			LeaseExpiresAt: &legacyExpiry,
		}).Update(); err != nil {
		t.Fatalf("prepare migrated exhausted task failed: %v", err)
	}
	claimedExhausted, err := harness.service.Claim(
		harness.ctx,
		ClaimInput{WorkerID: "worker-exhausted", Limit: 1},
	)
	if err != nil || len(claimedExhausted) != 0 {
		t.Fatalf("attempt-exhausted task was claimed: tasks=%+v err=%v", claimedExhausted, err)
	}
	storedExhausted := loadAsyncTaskIntegrationTask(t, harness.ctx, exhausted.Task.TaskNo)
	if storedExhausted.StatusCode != string(StatusDeadLetter) || storedExhausted.CompletedAt == nil ||
		storedExhausted.LastErrorCode != attemptsExhaustedErrorCode ||
		storedExhausted.LockedBy != "" || storedExhausted.LeaseExpiresAt != nil {
		t.Fatalf("attempt-exhausted task did not converge: %+v", storedExhausted)
	}
	assertAsyncTaskIntegrationCount(
		t,
		harness.ctx,
		dao.AsyncTaskAttempt.Table(),
		"task_no",
		exhausted.Task.TaskNo,
		0,
	)
}

// newAsyncTaskIntegrationHarness connects to an explicit empty test database.
func newAsyncTaskIntegrationHarness(t *testing.T) *asyncTaskIntegrationHarness {
	t.Helper()
	ctx := context.Background()
	configureAsyncTaskIntegrationDB(t, ctx)
	clock := &integrationClock{value: databaseTime(time.Now().UTC())}
	service, err := newService(Config{
		LeaseDuration:     30 * time.Second,
		BaseBackoff:       time.Second,
		MaximumBackoff:    10 * time.Second,
		MaximumClaimBatch: 100,
	}, clock.Now, rand.Reader)
	if err != nil {
		t.Fatalf("create integration service failed: %v", err)
	}
	harness := &asyncTaskIntegrationHarness{
		ctx:       ctx,
		service:   service,
		clock:     clock,
		uniqueTag: newAsyncTaskIntegrationTag(t),
	}
	t.Cleanup(func() {
		harness.cleanup(t)
	})
	return harness
}

// configureAsyncTaskIntegrationDB creates three task tables in a random schema
// selected only through an opt-in database. No existing default-schema row is visible.
func configureAsyncTaskIntegrationDB(t *testing.T, ctx context.Context) {
	t.Helper()
	dsn := strings.TrimSpace(os.Getenv(asyncTaskIntegrationDSNEnv))
	if dsn == "" {
		t.Skipf("set %s to run PostgreSQL asynchronous-task integration tests", asyncTaskIntegrationDSNEnv)
	}
	adminDB, err := gdb.New(gdb.ConfigNode{Link: dsn})
	if err != nil {
		t.Fatalf("create asynchronous-task integration admin database failed: %v", err)
	}
	schema := newAsyncTaskIntegrationSchema(t)
	if _, err = adminDB.Exec(ctx, "CREATE SCHEMA "+quoteAsyncTaskPostgreSQLIdentifier(schema)); err != nil {
		_ = adminDB.Close(ctx)
		t.Fatalf("create isolated asynchronous-task schema failed: %v", err)
	}
	originalConfig := gdb.GetAllConfig()
	var scopedDB gdb.DB
	t.Cleanup(func() {
		if scopedDB != nil {
			if closeErr := scopedDB.Close(ctx); closeErr != nil {
				t.Errorf("close scoped asynchronous-task database failed: %v", closeErr)
			}
		}
		if restoreErr := gdb.SetConfig(originalConfig); restoreErr != nil {
			t.Errorf("restore asynchronous-task integration database config failed: %v", restoreErr)
		}
		if !strings.HasPrefix(schema, "linapro_async_task_test_") {
			t.Errorf("refusing to drop unexpected integration schema %q", schema)
		} else if _, dropErr := adminDB.Exec(
			ctx,
			"DROP SCHEMA "+quoteAsyncTaskPostgreSQLIdentifier(schema)+" CASCADE",
		); dropErr != nil {
			t.Errorf("drop isolated asynchronous-task schema failed: %v", dropErr)
		}
		if closeErr := adminDB.Close(ctx); closeErr != nil {
			t.Errorf("close asynchronous-task integration admin database failed: %v", closeErr)
		}
	})
	for _, statement := range asyncTaskIntegrationTableDDL(schema) {
		if _, err = adminDB.Exec(ctx, statement); err != nil {
			t.Fatalf("create isolated asynchronous-task table failed: %v", err)
		}
	}
	scopedDSN := appendAsyncTaskDSNParameter(dsn, "search_path="+schema)
	if err := gdb.SetConfig(gdb.Config{
		gdb.DefaultGroupName: gdb.ConfigGroup{{Link: scopedDSN}},
	}); err != nil {
		t.Fatalf("configure asynchronous-task integration database failed: %v", err)
	}
	scopedDB = g.DB()
	currentSchema, err := scopedDB.GetValue(ctx, "SELECT current_schema()")
	if err != nil || currentSchema.String() != schema {
		t.Fatalf("scoped asynchronous-task search path mismatch: got=%q want=%q err=%v", currentSchema.String(), schema, err)
	}
	for _, table := range []string{
		dao.BusinessNumber.Table(),
		dao.AsyncTask.Table(),
		dao.AsyncTaskAttempt.Table(),
	} {
		if _, err = scopedDB.Exec(
			ctx,
			"SELECT 1 FROM "+quoteAsyncTaskPostgreSQLIdentifier(table)+" LIMIT 0",
		); err != nil {
			t.Fatalf("query isolated asynchronous-task table %s failed: %v", table, err)
		}
	}
}

// asyncTaskIntegrationTableDDL mirrors the generated persistence shape needed
// by transactional creation, claiming, attempts, recovery, and takeover.
func asyncTaskIntegrationTableDDL(schema string) []string {
	qualify := func(table string) string {
		return quoteAsyncTaskPostgreSQLIdentifier(schema) + "." +
			quoteAsyncTaskPostgreSQLIdentifier(table)
	}
	return []string{
		fmt.Sprintf(`CREATE TABLE %s (
            "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
            "tenant_id" INT NOT NULL DEFAULT 0 CHECK ("tenant_id" = 0),
            "number_type" VARCHAR(64) NOT NULL,
            "business_no" VARCHAR(64) NOT NULL UNIQUE,
            "reservation_scope_digest" CHAR(64) NOT NULL,
            "idempotency_key_digest" CHAR(64) NOT NULL,
            "owner_type" VARCHAR(64) NOT NULL DEFAULT '',
            "owner_internal_id" BIGINT,
            "assigned_at" TIMESTAMPTZ,
            "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
            "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
            UNIQUE ("tenant_id", "number_type", "reservation_scope_digest", "idempotency_key_digest")
        )`, qualify(dao.BusinessNumber.Table())),
		fmt.Sprintf(`CREATE TABLE %s (
            "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
            "tenant_id" INT NOT NULL DEFAULT 0 CHECK ("tenant_id" = 0),
            "task_no" VARCHAR(40) NOT NULL UNIQUE,
            "task_type" VARCHAR(128) NOT NULL,
            "subject_type" VARCHAR(64) NOT NULL,
            "subject_no" VARCHAR(64) NOT NULL,
            "customer_no" VARCHAR(40) NOT NULL DEFAULT '',
            "external_reference_no" VARCHAR(128) NOT NULL DEFAULT '',
            "idempotency_key_digest" CHAR(64) NOT NULL,
            "status_code" VARCHAR(24) NOT NULL DEFAULT 'pending'
                CHECK ("status_code" IN ('pending','running','retry_wait','succeeded','failed','dead_letter','result_unknown','manual_takeover')),
            "priority" SMALLINT NOT NULL DEFAULT 0,
            "payload" JSONB NOT NULL DEFAULT '{}'::JSONB,
            "result_snapshot" JSONB NOT NULL DEFAULT '{}'::JSONB,
            "available_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
            "deadline_at" TIMESTAMPTZ,
            "locked_by" VARCHAR(128) NOT NULL DEFAULT '',
            "lease_expires_at" TIMESTAMPTZ,
            "attempt_count" INT NOT NULL DEFAULT 0,
            "max_attempts" INT NOT NULL DEFAULT 12,
            "completed_at" TIMESTAMPTZ,
            "manual_takeover_at" TIMESTAMPTZ,
            "manual_assignee_user_id" INT,
            "manual_reason" VARCHAR(1024) NOT NULL DEFAULT '',
            "last_error_code" VARCHAR(128) NOT NULL DEFAULT '',
            "last_error_summary" VARCHAR(1024) NOT NULL DEFAULT '',
            "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
            "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
            CHECK ("attempt_count" >= 0 AND "max_attempts" > 0),
            UNIQUE ("tenant_id", "task_type", "subject_type", "subject_no", "idempotency_key_digest")
        )`, qualify(dao.AsyncTask.Table())),
		fmt.Sprintf(`CREATE TABLE %s (
            "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
            "tenant_id" INT NOT NULL DEFAULT 0 CHECK ("tenant_id" = 0),
            "task_no" VARCHAR(40) NOT NULL,
            "attempt_no" INT NOT NULL CHECK ("attempt_no" > 0),
            "worker_id" VARCHAR(128) NOT NULL DEFAULT '',
            "external_reference_no" VARCHAR(128) NOT NULL DEFAULT '',
            "status_code" VARCHAR(24) NOT NULL DEFAULT 'running'
                CHECK ("status_code" IN ('running','succeeded','failed','result_unknown')),
            "request_snapshot" JSONB NOT NULL DEFAULT '{}'::JSONB,
            "response_snapshot" JSONB NOT NULL DEFAULT '{}'::JSONB,
            "started_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
            "ended_at" TIMESTAMPTZ,
            "error_code" VARCHAR(128) NOT NULL DEFAULT '',
            "error_summary" VARCHAR(1024) NOT NULL DEFAULT '',
            "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
            "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
            UNIQUE ("task_no", "attempt_no")
        )`, qualify(dao.AsyncTaskAttempt.Table())),
	}
}

// appendAsyncTaskDSNParameter appends one lib/pq query option to a GoFrame link.
func appendAsyncTaskDSNParameter(dsn string, parameter string) string {
	separator := "?"
	if strings.Contains(dsn, "?") {
		separator = "&"
	}
	return dsn + separator + parameter
}

// quoteAsyncTaskPostgreSQLIdentifier quotes a generated or database-owned identifier.
func quoteAsyncTaskPostgreSQLIdentifier(value string) string {
	return `"` + strings.ReplaceAll(value, `"`, `""`) + `"`
}

// newAsyncTaskIntegrationSchema returns a collision-resistant safe cleanup target.
func newAsyncTaskIntegrationSchema(t *testing.T) string {
	t.Helper()
	var content [8]byte
	if _, err := rand.Read(content[:]); err != nil {
		t.Fatalf("generate asynchronous-task integration schema failed: %v", err)
	}
	return "linapro_async_task_test_" + hex.EncodeToString(content[:])
}

// createInput returns one unique ready task request owned by this harness.
func (h *asyncTaskIntegrationHarness) createInput(
	suffix string,
	maxAttempts int,
	deadline *time.Time,
) CreateInput {
	return CreateInput{
		TaskType:       "integration.server.provision",
		SubjectType:    "integration_order",
		SubjectNo:      h.uniqueTag + "-" + suffix,
		CustomerNo:     "CUS_" + h.uniqueTag,
		IdempotencyKey: h.uniqueTag + ":" + suffix,
		Priority:       maximumSmallInteger,
		Payload:        json.RawMessage(`{"schema_version":1,"operation":"provision"}`),
		AvailableAt:    h.clock.Now(),
		DeadlineAt:     deadline,
		MaxAttempts:    maxAttempts,
	}
}

// create persists and tracks one unique task inside an explicit outer transaction.
func (h *asyncTaskIntegrationHarness) create(
	t *testing.T,
	suffix string,
	maxAttempts int,
	deadline *time.Time,
) *CreateResult {
	t.Helper()
	result, err := h.createInTransaction(h.createInput(suffix, maxAttempts, deadline))
	if err != nil {
		t.Fatalf("create integration task %s failed: %v", suffix, err)
	}
	h.track(result.Task.TaskNo)
	return result
}

// createInTransaction invokes Create with the required pre-existing GoFrame transaction.
func (h *asyncTaskIntegrationHarness) createInTransaction(input CreateInput) (*CreateResult, error) {
	var result *CreateResult
	err := dao.AsyncTask.Transaction(h.ctx, func(ctx context.Context, _ gdb.TX) error {
		var err error
		result, err = h.service.Create(ctx, input)
		return err
	})
	return result, err
}

// claimOne requires exactly one claimable task for deterministic transition tests.
func (h *asyncTaskIntegrationHarness) claimOne(t *testing.T, workerID string) ClaimedTask {
	t.Helper()
	claimed, err := h.service.Claim(h.ctx, ClaimInput{WorkerID: workerID, Limit: 1})
	if err != nil {
		t.Fatalf("claim one task for %s failed: %v", workerID, err)
	}
	if len(claimed) != 1 {
		t.Fatalf("claim one task for %s returned %d tasks", workerID, len(claimed))
	}
	return claimed[0]
}

// track records a created task number for precise cleanup.
func (h *asyncTaskIntegrationHarness) track(taskNo string) {
	h.mutex.Lock()
	defer h.mutex.Unlock()
	h.taskNos = append(h.taskNos, taskNo)
}

// trackedTaskNos returns distinct task numbers without exposing mutable state.
func (h *asyncTaskIntegrationHarness) trackedTaskNos() []string {
	h.mutex.Lock()
	defer h.mutex.Unlock()
	unique := make(map[string]struct{}, len(h.taskNos))
	for _, taskNo := range h.taskNos {
		unique[taskNo] = struct{}{}
	}
	taskNos := make([]string, 0, len(unique))
	for taskNo := range unique {
		taskNos = append(taskNos, taskNo)
	}
	return taskNos
}

// cleanup deletes only rows addressed by random task numbers created by this harness.
func (h *asyncTaskIntegrationHarness) cleanup(t *testing.T) {
	t.Helper()
	taskNos := h.trackedTaskNos()
	if len(taskNos) == 0 {
		return
	}
	if _, err := dao.AsyncTaskAttempt.Ctx(h.ctx).
		WhereIn(dao.AsyncTaskAttempt.Columns().TaskNo, taskNos).
		Delete(); err != nil {
		t.Errorf("delete integration task attempts failed: %v", err)
	}
	if _, err := dao.AsyncTask.Ctx(h.ctx).
		WhereIn(dao.AsyncTask.Columns().TaskNo, taskNos).
		Delete(); err != nil {
		t.Errorf("delete integration tasks failed: %v", err)
	}
	if _, err := dao.BusinessNumber.Ctx(h.ctx).
		Where(do.BusinessNumber{TenantId: idccap.DefaultTenantID, NumberType: businessNumberTypeTask}).
		WhereIn(dao.BusinessNumber.Columns().BusinessNo, taskNos).
		Delete(); err != nil {
		t.Errorf("delete integration task number reservations failed: %v", err)
	}
	for _, taskNo := range taskNos {
		assertAsyncTaskIntegrationCount(t, h.ctx, dao.AsyncTaskAttempt.Table(), "task_no", taskNo, 0)
		assertAsyncTaskIntegrationCount(t, h.ctx, dao.AsyncTask.Table(), "task_no", taskNo, 0)
		assertAsyncTaskIntegrationCount(t, h.ctx, dao.BusinessNumber.Table(), "business_no", taskNo, 0)
	}
}

// loadAsyncTaskIntegrationTask returns one task row or fails the current test.
func loadAsyncTaskIntegrationTask(t *testing.T, ctx context.Context, taskNo string) *entity.AsyncTask {
	t.Helper()
	var task *entity.AsyncTask
	if err := dao.AsyncTask.Ctx(ctx).
		Where(do.AsyncTask{TenantId: idccap.DefaultTenantID, TaskNo: taskNo}).
		Scan(&task); err != nil {
		t.Fatalf("load integration task %s failed: %v", taskNo, err)
	}
	if task == nil {
		t.Fatalf("integration task %s disappeared", taskNo)
	}
	return task
}

// loadAsyncTaskIntegrationAttempt returns one attempt row or fails the current test.
func loadAsyncTaskIntegrationAttempt(
	t *testing.T,
	ctx context.Context,
	taskNo string,
	attemptNo int,
) *entity.AsyncTaskAttempt {
	t.Helper()
	var attempt *entity.AsyncTaskAttempt
	if err := dao.AsyncTaskAttempt.Ctx(ctx).
		Where(do.AsyncTaskAttempt{
			TenantId:  idccap.DefaultTenantID,
			TaskNo:    taskNo,
			AttemptNo: attemptNo,
		}).Scan(&attempt); err != nil {
		t.Fatalf("load integration task attempt %s/%d failed: %v", taskNo, attemptNo, err)
	}
	if attempt == nil {
		t.Fatalf("integration task attempt %s/%d disappeared", taskNo, attemptNo)
	}
	return attempt
}

// assertAsyncTaskIntegrationCount verifies one exact table predicate.
func assertAsyncTaskIntegrationCount(
	t *testing.T,
	ctx context.Context,
	table string,
	column string,
	value any,
	want int,
) {
	t.Helper()
	count, err := g.DB().Model(table).Ctx(ctx).Where(column, value).Count()
	if err != nil {
		t.Fatalf("count %s by %s failed: %v", table, column, err)
	}
	if count != want {
		t.Fatalf("count %s by %s=%v is %d, want %d", table, column, value, count, want)
	}
}

// newAsyncTaskIntegrationTag returns a short random collision-resistant fixture tag.
func newAsyncTaskIntegrationTag(t *testing.T) string {
	t.Helper()
	var content [8]byte
	if _, err := rand.Read(content[:]); err != nil {
		t.Fatalf("generate asynchronous-task integration tag failed: %v", err)
	}
	return "it" + hex.EncodeToString(content[:])
}
