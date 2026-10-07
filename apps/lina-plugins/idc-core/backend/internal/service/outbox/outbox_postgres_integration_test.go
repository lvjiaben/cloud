// This file verifies outbox transaction, concurrency, and lease invariants in an isolated PostgreSQL schema.

package outbox

import (
	"bytes"
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

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/internal/dao"
	"lina-plugin-idc-core/backend/internal/model/do"
	"lina-plugin-idc-core/backend/internal/model/entity"
)

const outboxIntegrationDSNEnv = "LINAPRO_IDC_CORE_TEST_DSN"

type outboxIntegrationClock struct {
	mutex sync.RWMutex
	now   time.Time
}

// TestOutboxPostgreSQLIntegration exercises all durable semantics against real PostgreSQL.
func TestOutboxPostgreSQLIntegration(t *testing.T) {
	ctx := context.Background()
	configureOutboxIntegrationDB(t, ctx)
	t.Run("transaction_and_concurrent_deduplication", func(t *testing.T) {
		testOutboxTransactionAndConcurrentDeduplication(t, ctx)
	})
	t.Run("skip_locked_renewal_and_fencing", func(t *testing.T) {
		testOutboxSkipLockedRenewalAndFencing(t, ctx)
	})
	t.Run("retry_dead_letter_and_unknown_convergence", func(t *testing.T) {
		testOutboxRetryDeadLetterAndUnknownConvergence(t, ctx)
	})
	t.Run("event_type_claim_and_convergence_scope", func(t *testing.T) {
		testOutboxEventTypeClaimAndConvergenceScope(t, ctx)
	})
}

// testOutboxEventTypeClaimAndConvergenceScope proves a dedicated worker cannot
// lease or terminally converge rows owned by a different event consumer.
func testOutboxEventTypeClaimAndConvergenceScope(t *testing.T, ctx context.Context) {
	clock := newOutboxIntegrationClock(time.Date(2026, 8, 6, 6, 0, 0, 0, time.UTC))
	service, err := newService(
		Config{LeaseDuration: 10 * time.Second, DefaultMaxAttempts: 1},
		clock.Now,
		rand.Reader,
	)
	if err != nil {
		t.Fatalf("create filtered-claim integration service failed: %v", err)
	}
	targetType := newOutboxIntegrationEventType(t, "filtered-target")
	foreignType := newOutboxIntegrationEventType(t, "filtered-foreign")
	cleanupOutboxIntegrationType(t, ctx, targetType)
	cleanupOutboxIntegrationType(t, ctx, foreignType)
	target := enqueueOutboxIntegrationEvent(t, ctx, service, outboxIntegrationInput(targetType, "target-ready"))
	foreign := enqueueOutboxIntegrationEvent(t, ctx, service, outboxIntegrationInput(foreignType, "foreign-ready"))

	claimed, err := service.Claim(ctx, ClaimInput{
		WorkerID:   "target-worker",
		Limit:      5,
		EventTypes: []string{" " + targetType + " ", targetType},
	})
	if err != nil || len(claimed) != 1 || claimed[0].EventNo != target.EventNo {
		t.Fatalf("filtered claim crossed event-type ownership: claimed=%+v err=%v", claimed, err)
	}
	if _, err = service.MarkPublished(ctx, leaseRef(claimed[0])); err != nil {
		t.Fatalf("publish filtered target failed: %v", err)
	}
	foreignClaim, err := service.Claim(ctx, ClaimInput{WorkerID: "generic-worker", Limit: 1})
	if err != nil || len(foreignClaim) != 1 || foreignClaim[0].EventNo != foreign.EventNo {
		t.Fatalf("empty filter no longer preserved generic claiming: claimed=%+v err=%v", foreignClaim, err)
	}
	if _, err = service.MarkPublished(ctx, leaseRef(foreignClaim[0])); err != nil {
		t.Fatalf("publish generic foreign event failed: %v", err)
	}

	targetExhausted := enqueueOutboxIntegrationEvent(t, ctx, service, outboxIntegrationInput(targetType, "target-exhausted"))
	foreignExhausted := enqueueOutboxIntegrationEvent(t, ctx, service, outboxIntegrationInput(foreignType, "foreign-exhausted"))
	columns := dao.OutboxEvent.Columns()
	result, err := dao.OutboxEvent.Ctx(ctx).
		Where(do.OutboxEvent{TenantId: idccap.DefaultTenantID}).
		WhereIn(columns.EventNo, []string{targetExhausted.EventNo, foreignExhausted.EventNo}).
		Data(do.OutboxEvent{AttemptCount: 1}).Update()
	if err != nil {
		t.Fatalf("prepare filtered exhausted rows failed: %v", err)
	}
	if err = requireAffected(result, 2, "prepare filtered exhausted rows"); err != nil {
		t.Fatal(err)
	}
	if claimed, err = service.Claim(ctx, ClaimInput{
		WorkerID: "target-worker", Limit: 5, EventTypes: []string{targetType},
	}); err != nil || len(claimed) != 0 {
		t.Fatalf("filtered exhausted convergence returned rows: claimed=%+v err=%v", claimed, err)
	}
	if row := loadOutboxIntegrationRow(t, ctx, targetExhausted.EventNo); Status(row.StatusCode) != StatusDeadLetter {
		t.Fatalf("target exhausted row did not converge: %+v", row)
	}
	if row := loadOutboxIntegrationRow(t, ctx, foreignExhausted.EventNo); Status(row.StatusCode) != StatusPending {
		t.Fatalf("foreign exhausted row was converged by target worker: %+v", row)
	}
	if claimed, err = service.Claim(ctx, ClaimInput{
		WorkerID: "foreign-worker", Limit: 5, EventTypes: []string{foreignType},
	}); err != nil || len(claimed) != 0 {
		t.Fatalf("foreign exhausted convergence returned rows: claimed=%+v err=%v", claimed, err)
	}
	if row := loadOutboxIntegrationRow(t, ctx, foreignExhausted.EventNo); Status(row.StatusCode) != StatusDeadLetter {
		t.Fatalf("foreign exhausted row did not converge under its owner: %+v", row)
	}
}

// testOutboxTransactionAndConcurrentDeduplication verifies commit, rollback,
// immutable replay checks, raw-key hashing, and concurrent convergence.
func testOutboxTransactionAndConcurrentDeduplication(t *testing.T, ctx context.Context) {
	clock := newOutboxIntegrationClock(time.Date(2026, 8, 6, 2, 0, 0, 0, time.UTC))
	service, err := newService(Config{}, clock.Now, rand.Reader)
	if err != nil {
		t.Fatalf("create integration service failed: %v", err)
	}
	eventType := newOutboxIntegrationEventType(t, "transaction")
	cleanupOutboxIntegrationType(t, ctx, eventType)

	outside := outboxIntegrationInput(eventType, "outside")
	if _, err = service.Enqueue(ctx, outside); !bizerr.Is(err, CodeTransactionRequired) {
		t.Fatalf("enqueue outside transaction returned %v", err)
	}

	rollbackInput := outboxIntegrationInput(eventType, "rollback")
	rollbackMarker := errors.New("rollback integration event")
	err = dao.OutboxEvent.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		if _, enqueueErr := service.Enqueue(ctx, rollbackInput); enqueueErr != nil {
			return enqueueErr
		}
		return rollbackMarker
	})
	if !errors.Is(err, rollbackMarker) {
		t.Fatalf("rollback transaction returned %v", err)
	}
	assertOutboxIntegrationCount(t, ctx, eventType, 0)

	committedInput := outboxIntegrationInput(eventType, "committed")
	created := enqueueOutboxIntegrationEvent(t, ctx, service, committedInput)
	if created.EventNo == "" || created.AvailableAt.IsZero() || created.AvailableAt.Equal(time.Unix(0, 0)) {
		t.Fatalf("created event has invalid operational timestamps: %+v", created)
	}
	clock.Add(2 * time.Minute)
	replayed := enqueueOutboxIntegrationEvent(t, ctx, service, committedInput)
	if replayed.EventNo != created.EventNo || !replayed.AvailableAt.Equal(created.AvailableAt) {
		t.Fatalf("implicit-schedule replay diverged: created=%+v replayed=%+v", created, replayed)
	}
	reconfiguredService, err := newService(
		Config{DefaultMaxAttempts: 5},
		clock.Now,
		rand.Reader,
	)
	if err != nil {
		t.Fatalf("create reconfigured integration service failed: %v", err)
	}
	reconfiguredReplay := enqueueOutboxIntegrationEvent(t, ctx, reconfiguredService, committedInput)
	if reconfiguredReplay.EventNo != created.EventNo ||
		reconfiguredReplay.MaxAttempts != created.MaxAttempts {
		t.Fatalf(
			"implicit attempt default changed an idempotent replay: created=%+v replayed=%+v",
			created,
			reconfiguredReplay,
		)
	}
	explicitAttempts := committedInput
	explicitAttempts.MaxAttempts = 5
	err = dao.OutboxEvent.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		_, enqueueErr := reconfiguredService.Enqueue(ctx, explicitAttempts)
		return enqueueErr
	})
	if !bizerr.Is(err, CodeIdempotencyConflict) {
		t.Fatalf("explicitly changed attempt budget should conflict, got %v", err)
	}
	mismatch := committedInput
	mismatch.Payload = json.RawMessage(`{"orderNo":"changed"}`)
	err = dao.OutboxEvent.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		_, enqueueErr := service.Enqueue(ctx, mismatch)
		return enqueueErr
	})
	if !bizerr.Is(err, CodeIdempotencyConflict) {
		t.Fatalf("changed replay returned %v", err)
	}
	stored := loadOutboxIntegrationRow(t, ctx, created.EventNo)
	if stored.DeduplicationDigest == committedInput.DeduplicationKey ||
		stored.DeduplicationDigest != digest(committedInput.DeduplicationKey) ||
		strings.Contains(stored.Payload, "changed") {
		t.Fatalf("stored event did not preserve hashed immutable intent: %+v", stored)
	}

	concurrentType := newOutboxIntegrationEventType(t, "concurrent")
	cleanupOutboxIntegrationType(t, ctx, concurrentType)
	concurrentInput := outboxIntegrationInput(concurrentType, "same-effect")
	const callers = 6
	var waitGroup sync.WaitGroup
	start := make(chan struct{})
	results := make([]*Event, callers)
	errorsByCaller := make([]error, callers)
	for index := range callers {
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			<-start
			errorsByCaller[index] = dao.OutboxEvent.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
				var enqueueErr error
				results[index], enqueueErr = service.Enqueue(ctx, concurrentInput)
				return enqueueErr
			})
		}()
	}
	close(start)
	waitGroup.Wait()
	winner := ""
	for index := range callers {
		if errorsByCaller[index] != nil || results[index] == nil {
			t.Fatalf("concurrent enqueue %d failed: result=%+v err=%v", index, results[index], errorsByCaller[index])
		}
		if winner == "" {
			winner = results[index].EventNo
		}
		if results[index].EventNo != winner {
			t.Fatalf("concurrent enqueue diverged: %q and %q", winner, results[index].EventNo)
		}
	}
	assertOutboxIntegrationCount(t, ctx, concurrentType, 1)
}

// testOutboxSkipLockedRenewalAndFencing verifies non-overlapping claims,
// explicit row-lock skipping, SQL NULL cleanup, and attempt-number fencing.
func testOutboxSkipLockedRenewalAndFencing(t *testing.T, ctx context.Context) {
	clock := newOutboxIntegrationClock(time.Date(2026, 8, 6, 3, 0, 0, 0, time.UTC))
	service, err := newService(Config{LeaseDuration: 30 * time.Second}, clock.Now, rand.Reader)
	if err != nil {
		t.Fatalf("create claim integration service failed: %v", err)
	}
	eventType := newOutboxIntegrationEventType(t, "claim")
	cleanupOutboxIntegrationType(t, ctx, eventType)
	events := make([]*Event, 3)
	for index := range events {
		events[index] = enqueueOutboxIntegrationEvent(
			t,
			ctx,
			service,
			outboxIntegrationInput(eventType, fmt.Sprintf("claim-%d", index)),
		)
	}

	lockReady := make(chan error, 1)
	releaseLock := make(chan struct{})
	lockDone := make(chan error, 1)
	go func() {
		lockDone <- dao.OutboxEvent.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
			var row *entity.OutboxEvent
			lockErr := dao.OutboxEvent.Ctx(ctx).
				Where(do.OutboxEvent{TenantId: idccap.DefaultTenantID, EventNo: events[0].EventNo}).
				LockUpdate().Scan(&row)
			lockReady <- lockErr
			if lockErr != nil {
				return lockErr
			}
			<-releaseLock
			return nil
		})
	}()
	if err = <-lockReady; err != nil {
		t.Fatalf("lock first integration event failed: %v", err)
	}
	firstBatch, err := service.Claim(ctx, ClaimInput{WorkerID: "worker-a", Limit: 1})
	close(releaseLock)
	if lockErr := <-lockDone; lockErr != nil {
		t.Fatalf("locked integration transaction failed: %v", lockErr)
	}
	if err != nil || len(firstBatch) != 1 || firstBatch[0].EventNo == events[0].EventNo {
		t.Fatalf("skip-locked claim mismatch: claimed=%+v err=%v", firstBatch, err)
	}
	remaining, err := service.Claim(ctx, ClaimInput{WorkerID: "worker-b", Limit: 3})
	if err != nil || len(remaining) != 2 {
		t.Fatalf("claim remaining non-overlapping events failed: claimed=%+v err=%v", remaining, err)
	}
	seen := map[string]struct{}{firstBatch[0].EventNo: {}}
	for index := range remaining {
		if _, duplicate := seen[remaining[index].EventNo]; duplicate {
			t.Fatalf("event %s was claimed twice", remaining[index].EventNo)
		}
		seen[remaining[index].EventNo] = struct{}{}
	}
	if len(seen) != len(events) {
		t.Fatalf("claims did not cover all events: %+v", seen)
	}

	clock.Add(10 * time.Second)
	renewed, err := service.Renew(ctx, leaseRef(firstBatch[0]))
	if err != nil || !renewed.LeaseExpiresAt.After(firstBatch[0].LeaseExpiresAt) {
		t.Fatalf("renew live lease failed: renewed=%+v err=%v", renewed, err)
	}
	published, err := service.MarkPublished(ctx, leaseRef(*renewed))
	if err != nil || published.Status != StatusPublished || published.PublishedAt == nil {
		t.Fatalf("publish renewed event failed: event=%+v err=%v", published, err)
	}
	assertOutboxIntegrationLeaseCleared(t, ctx, published.EventNo)
	if _, err = service.MarkPublished(ctx, leaseRef(*renewed)); !bizerr.Is(err, CodeLeaseLost) {
		t.Fatalf("closed publication lease replay returned %v", err)
	}
	if _, err = service.MarkResultUnknown(ctx, UnknownInput{
		Lease:        leaseRef(remaining[0]),
		ErrorCode:    "remote_result_unknown",
		ErrorSummary: "Remote result could not be proven",
	}); err != nil {
		t.Fatalf("mark claimed event unknown failed: %v", err)
	}
	assertOutboxIntegrationLeaseCleared(t, ctx, remaining[0].EventNo)
	if _, err = service.MarkPublished(ctx, leaseRef(remaining[1])); err != nil {
		t.Fatalf("publish remaining event failed: %v", err)
	}
	assertOutboxIntegrationLeaseCleared(t, ctx, remaining[1].EventNo)

	staleType := newOutboxIntegrationEventType(t, "stale")
	cleanupOutboxIntegrationType(t, ctx, staleType)
	staleClock := newOutboxIntegrationClock(time.Date(2026, 8, 6, 4, 0, 0, 0, time.UTC))
	// Zero-filled deterministic entropy intentionally reuses the same lease token
	// so this test proves attempt_count is part of the persisted fence.
	staleService, err := newService(
		Config{LeaseDuration: 5 * time.Second, DefaultMaxAttempts: 3},
		staleClock.Now,
		bytes.NewReader(make([]byte, 96)),
	)
	if err != nil {
		t.Fatalf("create stale-lease integration service failed: %v", err)
	}
	enqueueOutboxIntegrationEvent(t, ctx, staleService, outboxIntegrationInput(staleType, "same-token"))
	oldBatch, err := staleService.Claim(ctx, ClaimInput{WorkerID: "same-worker", Limit: 1})
	if err != nil || len(oldBatch) != 1 {
		t.Fatalf("claim first deterministic lease failed: claimed=%+v err=%v", oldBatch, err)
	}
	staleClock.Set(oldBatch[0].LeaseExpiresAt)
	if _, err = staleService.Renew(ctx, leaseRef(oldBatch[0])); !bizerr.Is(err, CodeLeaseLost) {
		t.Fatalf("exactly expired renewal returned %v", err)
	}
	newBatch, err := staleService.Claim(ctx, ClaimInput{WorkerID: "same-worker", Limit: 1})
	if err != nil || len(newBatch) != 1 {
		t.Fatalf("reclaim expired deterministic lease failed: claimed=%+v err=%v", newBatch, err)
	}
	if newBatch[0].LeaseToken != oldBatch[0].LeaseToken ||
		newBatch[0].AttemptNo != oldBatch[0].AttemptNo+1 {
		t.Fatalf("reclaim did not preserve token-reuse fencing scenario: old=%+v new=%+v", oldBatch[0], newBatch[0])
	}
	if _, err = staleService.MarkPublished(ctx, leaseRef(oldBatch[0])); !bizerr.Is(err, CodeLeaseLost) {
		t.Fatalf("stale attempt with reused token returned %v", err)
	}
	if _, err = staleService.MarkResultUnknown(ctx, UnknownInput{
		Lease:        leaseRef(newBatch[0]),
		ErrorCode:    "query_required",
		ErrorSummary: "Query external state before any manual action",
	}); err != nil {
		t.Fatalf("close current deterministic lease failed: %v", err)
	}
	assertOutboxIntegrationLeaseCleared(t, ctx, newBatch[0].EventNo)
}

// testOutboxRetryDeadLetterAndUnknownConvergence verifies diagnostic retention,
// jitter scheduling, retry exhaustion, and both exhausted-row convergence paths.
func testOutboxRetryDeadLetterAndUnknownConvergence(t *testing.T, ctx context.Context) {
	clock := newOutboxIntegrationClock(time.Date(2026, 8, 6, 5, 0, 0, 0, time.UTC))
	service, err := newService(
		Config{LeaseDuration: 10 * time.Second, BaseBackoff: 8 * time.Second, MaximumBackoff: 20 * time.Second},
		clock.Now,
		rand.Reader,
	)
	if err != nil {
		t.Fatalf("create retry integration service failed: %v", err)
	}
	eventType := newOutboxIntegrationEventType(t, "retry")
	cleanupOutboxIntegrationType(t, ctx, eventType)
	input := outboxIntegrationInput(eventType, "retry-twice")
	input.MaxAttempts = 2
	enqueueOutboxIntegrationEvent(t, ctx, service, input)
	first, err := service.Claim(ctx, ClaimInput{WorkerID: "retry-worker", Limit: 1})
	if err != nil || len(first) != 1 || first[0].AttemptNo != 1 {
		t.Fatalf("claim retry event failed: claimed=%+v err=%v", first, err)
	}
	delay := service.retryBackoff(first[0].EventNo, first[0].AttemptNo)
	retried, err := service.RecordFailure(ctx, FailureInput{
		Lease:        leaseRef(first[0]),
		Retryable:    true,
		ErrorCode:    "remote_busy",
		ErrorSummary: "Remote publisher is temporarily busy",
	})
	if err != nil || retried.Status != StatusRetryWait ||
		!retried.AvailableAt.Equal(databaseTime(clock.Now().Add(delay))) {
		t.Fatalf("schedule retry failed: event=%+v delay=%s err=%v", retried, delay, err)
	}
	assertOutboxIntegrationLeaseCleared(t, ctx, retried.EventNo)
	clock.Set(retried.AvailableAt.Add(-time.Microsecond))
	if early, claimErr := service.Claim(ctx, ClaimInput{WorkerID: "retry-worker", Limit: 1}); claimErr != nil || len(early) != 0 {
		t.Fatalf("retry became claimable too early: claimed=%+v err=%v", early, claimErr)
	}
	clock.Set(retried.AvailableAt)
	second, err := service.Claim(ctx, ClaimInput{WorkerID: "retry-worker", Limit: 1})
	if err != nil || len(second) != 1 || second[0].AttemptNo != 2 ||
		second[0].LastErrorCode != "remote_busy" || second[0].LastErrorSummary == "" {
		t.Fatalf("reclaim did not retain prior diagnostics: claimed=%+v err=%v", second, err)
	}
	dead, err := service.RecordFailure(ctx, FailureInput{
		Lease:        leaseRef(second[0]),
		Retryable:    true,
		ErrorCode:    "remote_busy",
		ErrorSummary: "Remote publisher remained busy",
	})
	if err != nil || dead.Status != StatusDeadLetter {
		t.Fatalf("retry exhaustion did not dead-letter event: event=%+v err=%v", dead, err)
	}
	assertOutboxIntegrationLeaseCleared(t, ctx, dead.EventNo)

	expiredType := newOutboxIntegrationEventType(t, "expired")
	cleanupOutboxIntegrationType(t, ctx, expiredType)
	expiredInput := outboxIntegrationInput(expiredType, "expire-at-max")
	expiredInput.MaxAttempts = 1
	enqueueOutboxIntegrationEvent(t, ctx, service, expiredInput)
	expiredClaim, err := service.Claim(ctx, ClaimInput{WorkerID: "expiry-worker", Limit: 1})
	if err != nil || len(expiredClaim) != 1 {
		t.Fatalf("claim expiring max-attempt event failed: claimed=%+v err=%v", expiredClaim, err)
	}
	clock.Set(expiredClaim[0].LeaseExpiresAt)
	converged, err := service.Claim(ctx, ClaimInput{WorkerID: "expiry-worker", Limit: 5})
	if err != nil || len(converged) != 0 {
		t.Fatalf("expired exhausted event was unexpectedly reclaimed: claimed=%+v err=%v", converged, err)
	}
	expiredRow := loadOutboxIntegrationRow(t, ctx, expiredClaim[0].EventNo)
	if Status(expiredRow.StatusCode) != StatusResultUnknown || expiredRow.LastErrorCode != expiredUnknownErrorCode {
		t.Fatalf("expired exhausted event did not freeze unknown: %+v", expiredRow)
	}
	assertOutboxIntegrationLeaseCleared(t, ctx, expiredRow.EventNo)

	anomalousType := newOutboxIntegrationEventType(t, "anomalous")
	cleanupOutboxIntegrationType(t, ctx, anomalousType)
	anomalousInput := outboxIntegrationInput(anomalousType, "already-exhausted")
	anomalousInput.MaxAttempts = 1
	anomalous := enqueueOutboxIntegrationEvent(t, ctx, service, anomalousInput)
	result, err := dao.OutboxEvent.Ctx(ctx).
		Where(do.OutboxEvent{TenantId: idccap.DefaultTenantID, EventNo: anomalous.EventNo}).
		Data(do.OutboxEvent{AttemptCount: 1}).Update()
	if err != nil {
		t.Fatalf("prepare anomalous exhausted event failed: %v", err)
	}
	if err = requireAffected(result, 1, "prepare anomalous exhausted event"); err != nil {
		t.Fatal(err)
	}
	converged, err = service.Claim(ctx, ClaimInput{WorkerID: "expiry-worker", Limit: 5})
	if err != nil || len(converged) != 0 {
		t.Fatalf("unclaimed exhausted event was unexpectedly claimed: claimed=%+v err=%v", converged, err)
	}
	anomalousRow := loadOutboxIntegrationRow(t, ctx, anomalous.EventNo)
	if Status(anomalousRow.StatusCode) != StatusDeadLetter || anomalousRow.LastErrorCode != attemptsExhaustedErrorCode {
		t.Fatalf("unclaimed exhausted event did not dead-letter: %+v", anomalousRow)
	}
	assertOutboxIntegrationLeaseCleared(t, ctx, anomalousRow.EventNo)
}

// configureOutboxIntegrationDB clones only the outbox table into a random
// schema and points the DAO search path there, isolating tests from every existing row.
func configureOutboxIntegrationDB(t *testing.T, ctx context.Context) {
	t.Helper()
	dsn := strings.TrimSpace(os.Getenv(outboxIntegrationDSNEnv))
	if dsn == "" {
		t.Skipf("set %s to run PostgreSQL outbox integration tests", outboxIntegrationDSNEnv)
	}
	adminDB, err := gdb.New(gdb.ConfigNode{Link: dsn})
	if err != nil {
		t.Fatalf("create outbox integration admin database failed: %v", err)
	}
	schema := newOutboxIntegrationSchema(t)
	table := dao.OutboxEvent.Table()
	if _, err = adminDB.Exec(ctx, "CREATE SCHEMA "+quotePostgreSQLIdentifier(schema)); err != nil {
		_ = adminDB.Close(ctx)
		t.Fatalf("create isolated outbox schema failed: %v", err)
	}
	originalConfig := gdb.GetAllConfig()
	var scopedDB gdb.DB
	t.Cleanup(func() {
		if scopedDB != nil {
			if closeErr := scopedDB.Close(ctx); closeErr != nil {
				t.Errorf("close scoped outbox integration database failed: %v", closeErr)
			}
		}
		if restoreErr := gdb.SetConfig(originalConfig); restoreErr != nil {
			t.Errorf("restore outbox integration database config failed: %v", restoreErr)
		}
		if !strings.HasPrefix(schema, "linapro_outbox_test_") {
			t.Errorf("refusing to drop unexpected integration schema %q", schema)
		} else if _, dropErr := adminDB.Exec(
			ctx,
			"DROP SCHEMA "+quotePostgreSQLIdentifier(schema)+" CASCADE",
		); dropErr != nil {
			t.Errorf("drop isolated outbox integration schema failed: %v", dropErr)
		}
		if closeErr := adminDB.Close(ctx); closeErr != nil {
			t.Errorf("close outbox integration admin database failed: %v", closeErr)
		}
	})
	if _, err = adminDB.Exec(ctx, outboxIntegrationTableDDL(schema, table)); err != nil {
		t.Fatalf("create isolated outbox table failed: %v", err)
	}
	scopedDSN := appendOutboxDSNParameter(dsn, "search_path="+schema)
	if err = gdb.SetConfig(gdb.Config{
		gdb.DefaultGroupName: gdb.ConfigGroup{{Link: scopedDSN}},
	}); err != nil {
		t.Fatalf("configure scoped outbox integration database failed: %v", err)
	}
	scopedDB = dao.OutboxEvent.DB()
	currentSchema, err := scopedDB.GetValue(ctx, "SELECT current_schema()")
	if err != nil || currentSchema.String() != schema {
		t.Fatalf("scoped outbox search path mismatch: got=%q want=%q err=%v", currentSchema.String(), schema, err)
	}
	if _, err = scopedDB.Exec(ctx, "SELECT 1 FROM "+quotePostgreSQLIdentifier(table)+" LIMIT 0"); err != nil {
		t.Fatalf("query isolated outbox table failed: %v", err)
	}
}

// outboxIntegrationTableDDL mirrors the generated outbox entity in a disposable schema.
func outboxIntegrationTableDDL(schema string, table string) string {
	qualifiedTable := quotePostgreSQLIdentifier(schema) + "." + quotePostgreSQLIdentifier(table)
	return fmt.Sprintf(`CREATE TABLE %s (
        "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
        "tenant_id" INT NOT NULL DEFAULT 0 CHECK ("tenant_id" = 0),
        "event_no" VARCHAR(40) NOT NULL UNIQUE,
        "event_type" VARCHAR(128) NOT NULL,
        "event_version" INT NOT NULL DEFAULT 1 CHECK ("event_version" > 0),
        "topic" VARCHAR(128) NOT NULL DEFAULT '',
        "aggregate_type" VARCHAR(64) NOT NULL,
        "aggregate_no" VARCHAR(64) NOT NULL,
        "correlation_no" VARCHAR(64) NOT NULL DEFAULT '',
        "deduplication_digest" CHAR(64) NOT NULL,
        "status_code" VARCHAR(24) NOT NULL DEFAULT 'pending'
            CHECK ("status_code" IN ('pending','processing','retry_wait','published','dead_letter','result_unknown')),
        "payload" JSONB NOT NULL DEFAULT '{}'::JSONB,
        "metadata" JSONB NOT NULL DEFAULT '{}'::JSONB,
        "occurred_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
        "available_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
        "locked_by" VARCHAR(128) NOT NULL DEFAULT '',
        "lease_expires_at" TIMESTAMPTZ,
        "attempt_count" INT NOT NULL DEFAULT 0,
        "max_attempts" INT NOT NULL DEFAULT 12,
        "published_at" TIMESTAMPTZ,
        "last_error_code" VARCHAR(128) NOT NULL DEFAULT '',
        "last_error_summary" VARCHAR(1024) NOT NULL DEFAULT '',
        "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
        "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
        CHECK ("attempt_count" >= 0 AND "max_attempts" > 0),
        UNIQUE ("tenant_id", "event_type", "deduplication_digest")
    )`, qualifiedTable)
}

// enqueueOutboxIntegrationEvent commits one event through the required caller transaction.
func enqueueOutboxIntegrationEvent(
	t *testing.T,
	ctx context.Context,
	service Service,
	input EnqueueInput,
) *Event {
	t.Helper()
	var event *Event
	err := dao.OutboxEvent.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		var enqueueErr error
		event, enqueueErr = service.Enqueue(ctx, input)
		return enqueueErr
	})
	if err != nil || event == nil {
		t.Fatalf("commit integration outbox event failed: event=%+v err=%v", event, err)
	}
	return event
}

// outboxIntegrationInput returns one isolated, redacted event request.
func outboxIntegrationInput(eventType string, key string) EnqueueInput {
	return EnqueueInput{
		EventType:        eventType,
		EventVersion:     1,
		Topic:            "integration.lifecycle",
		AggregateType:    "integration_event",
		AggregateNo:      "AGG_" + digest(key)[:24],
		CorrelationNo:    "COR_" + digest(eventType + key)[:24],
		DeduplicationKey: key,
		Payload:          json.RawMessage(fmt.Sprintf(`{"effect":"%s"}`, key)),
		Metadata:         json.RawMessage(`{"source":"postgres-integration"}`),
	}
}

// leaseRef extracts the complete fencing tuple from a claimed event.
func leaseRef(claimed ClaimedEvent) LeaseRef {
	return LeaseRef{
		EventNo:    claimed.EventNo,
		LeaseToken: claimed.LeaseToken,
		AttemptNo:  claimed.AttemptNo,
	}
}

// loadOutboxIntegrationRow reads one generated row from the isolated schema.
func loadOutboxIntegrationRow(t *testing.T, ctx context.Context, eventNo string) *entity.OutboxEvent {
	t.Helper()
	var row *entity.OutboxEvent
	if err := dao.OutboxEvent.Ctx(ctx).Where(do.OutboxEvent{
		TenantId: idccap.DefaultTenantID,
		EventNo:  eventNo,
	}).Scan(&row); err != nil || row == nil {
		t.Fatalf("load integration outbox row failed: row=%+v err=%v", row, err)
	}
	return row
}

// assertOutboxIntegrationLeaseCleared proves both fencing columns are truly empty.
func assertOutboxIntegrationLeaseCleared(t *testing.T, ctx context.Context, eventNo string) {
	t.Helper()
	row := loadOutboxIntegrationRow(t, ctx, eventNo)
	if row.LockedBy != "" || row.LeaseExpiresAt != nil {
		t.Fatalf("outbox lease was not fully cleared: %+v", row)
	}
}

// cleanupOutboxIntegrationType deletes and verifies only one subtest-owned event type.
func cleanupOutboxIntegrationType(t *testing.T, ctx context.Context, eventType string) {
	t.Helper()
	t.Cleanup(func() {
		if _, err := dao.OutboxEvent.Ctx(ctx).Where(do.OutboxEvent{
			TenantId:  idccap.DefaultTenantID,
			EventType: eventType,
		}).Delete(); err != nil {
			t.Errorf("delete integration outbox type %q failed: %v", eventType, err)
		}
		assertOutboxIntegrationCount(t, ctx, eventType, 0)
	})
}

// assertOutboxIntegrationCount verifies one exact event-type scope.
func assertOutboxIntegrationCount(t *testing.T, ctx context.Context, eventType string, want int) {
	t.Helper()
	count, err := dao.OutboxEvent.Ctx(ctx).Where(do.OutboxEvent{
		TenantId:  idccap.DefaultTenantID,
		EventType: eventType,
	}).Count()
	if err != nil || count != want {
		t.Fatalf("event type %q count=%d want=%d err=%v", eventType, count, want, err)
	}
}

// newOutboxIntegrationEventType creates a bounded unique event namespace.
func newOutboxIntegrationEventType(t *testing.T, suffix string) string {
	t.Helper()
	var content [6]byte
	if _, err := rand.Read(content[:]); err != nil {
		t.Fatalf("generate integration event type failed: %v", err)
	}
	return "itest.outbox." + hex.EncodeToString(content[:]) + "." + suffix
}

// newOutboxIntegrationSchema creates a safe identifier for scoped destructive cleanup.
func newOutboxIntegrationSchema(t *testing.T) string {
	t.Helper()
	var content [8]byte
	if _, err := rand.Read(content[:]); err != nil {
		t.Fatalf("generate integration schema failed: %v", err)
	}
	return "linapro_outbox_test_" + hex.EncodeToString(content[:])
}

// appendOutboxDSNParameter appends one lib/pq query option to a GoFrame link.
func appendOutboxDSNParameter(dsn string, parameter string) string {
	separator := "?"
	if strings.Contains(dsn, "?") {
		separator = "&"
	}
	return dsn + separator + parameter
}

// quotePostgreSQLIdentifier quotes a trusted or database-resolved identifier.
func quotePostgreSQLIdentifier(value string) string {
	return `"` + strings.ReplaceAll(value, `"`, `""`) + `"`
}

// newOutboxIntegrationClock returns a concurrency-safe mutable clock.
func newOutboxIntegrationClock(now time.Time) *outboxIntegrationClock {
	return &outboxIntegrationClock{now: databaseTime(now)}
}

// Now returns the current integration database time.
func (c *outboxIntegrationClock) Now() time.Time {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	return c.now
}

// Set replaces the current integration database time.
func (c *outboxIntegrationClock) Set(value time.Time) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.now = databaseTime(value)
}

// Add advances the current integration database time.
func (c *outboxIntegrationClock) Add(delta time.Duration) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.now = databaseTime(c.now.Add(delta))
}
