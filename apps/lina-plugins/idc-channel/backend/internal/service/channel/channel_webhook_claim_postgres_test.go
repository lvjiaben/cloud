// These tests prove bounded webhook claim query counts, unique concurrent leases,
// expiry recovery, stale-worker fencing and atomic rollback in disposable schemas.
package channel

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/os/glog"
)

type webhookQuerySampleKey struct{}
type webhookQueryLogger struct {
	glog.ILogger
	mu     sync.Mutex
	counts map[int]int
}

func (l *webhookQueryLogger) Debug(ctx context.Context, _ ...any) { l.record(ctx) }
func (l *webhookQueryLogger) Error(ctx context.Context, _ ...any) { l.record(ctx) }
func (l *webhookQueryLogger) record(ctx context.Context) {
	if sample, ok := ctx.Value(webhookQuerySampleKey{}).(int); ok {
		l.mu.Lock()
		l.counts[sample]++
		l.mu.Unlock()
	}
}
func (l *webhookQueryLogger) count(sample int) int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.counts[sample]
}

func newWebhookClaimFixture(t *testing.T) (gdb.DB, *service) {
	t.Helper()
	database := channelPageDatabase(t)
	if _, err := database.Exec(context.Background(), `CREATE TABLE plugin_idc_channel_webhook(id BIGSERIAL PRIMARY KEY,channel_id TEXT,event_id TEXT,event_version TEXT DEFAULT 'v1',event_type TEXT DEFAULT 'test',subject_ref TEXT DEFAULT '',sequence_no BIGINT DEFAULT 0,payload JSONB DEFAULT '{}',payload_text TEXT DEFAULT '{}',payload_hash TEXT DEFAULT '',signature TEXT DEFAULT '',status TEXT DEFAULT 'pending',attempts INT DEFAULT 0,next_attempt_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,lease_token TEXT DEFAULT '',lease_expires_at TIMESTAMPTZ,delivered_at TIMESTAMPTZ,last_error TEXT DEFAULT '',updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,UNIQUE(channel_id,event_id))`); err != nil {
		t.Fatal(err)
	}
	return database, &service{db: database}
}
func seedWebhookClaims(t *testing.T, database gdb.DB, count int) {
	t.Helper()
	if _, err := database.Exec(context.Background(), `INSERT INTO plugin_idc_channel_webhook(channel_id,event_id) SELECT 'channel_'||(i%2),'event_'||i FROM generate_series(1,?) i`, count); err != nil {
		t.Fatal(err)
	}
}
func verifyWebhookClaims(t *testing.T, claims []WebhookDelivery, count int) {
	t.Helper()
	if len(claims) != count {
		t.Fatalf("claimed=%d want=%d", len(claims), count)
	}
	tokens := make(map[string]bool)
	ids := make(map[int64]bool)
	for _, row := range claims {
		if row.Status != WebhookStatusDelivering || row.Attempts < 1 || row.LeaseToken == "" || tokens[row.LeaseToken] || ids[row.ID] || row.LeaseExpires == nil || !row.LeaseExpires.After(time.Now().Add(-time.Second)) {
			t.Fatalf("invalid lease: %+v", row)
		}
		tokens[row.LeaseToken] = true
		ids[row.ID] = true
	}
}

func TestWebhookClaimsConstantQueriesPostgreSQL(t *testing.T) {
	database, service := newWebhookClaimFixture(t)
	ctx := context.Background()
	// Warm ORM table metadata before measuring the business SQL, never tokens.
	if _, err := service.ClaimWebhookDeliveries(ctx, 1, time.Minute); err != nil {
		t.Fatal(err)
	}
	previousLogger, previousDebug := database.GetLogger(), database.GetDebug()
	logger := &webhookQueryLogger{ILogger: previousLogger, counts: make(map[int]int)}
	database.SetLogger(logger)
	database.SetDebug(true)
	t.Cleanup(func() { database.SetDebug(previousDebug); database.SetLogger(previousLogger) })
	seedWebhookClaims(t, database, 33)
	one, err := service.ClaimWebhookDeliveries(context.WithValue(ctx, webhookQuerySampleKey{}, 1), 1, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	verifyWebhookClaims(t, one, 1)
	many, err := service.ClaimWebhookDeliveries(context.WithValue(ctx, webhookQuerySampleKey{}, 32), 32, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	verifyWebhookClaims(t, many, 32)
	if logger.count(1) != logger.count(32) || logger.count(32) > 7 || logger.count(32) < 5 {
		t.Fatalf("queries scale with rows: one=%d thirtyTwo=%d", logger.count(1), logger.count(32))
	}
	t.Logf("business SQL is constant: one=%d thirtyTwo=%d (including transaction logging when enabled)", logger.count(1), logger.count(32))
	empty, err := service.ClaimWebhookDeliveries(ctx, 32, time.Minute)
	if err != nil || len(empty) != 0 {
		t.Fatalf("live leases were reclaimed: %+v %v", empty, err)
	}
}

func TestWebhookClaimsConcurrentAndExpiredPostgreSQL(t *testing.T) {
	database, service := newWebhookClaimFixture(t)
	ctx := context.Background()
	seedWebhookClaims(t, database, 64)
	var wg sync.WaitGroup
	ready := make(chan struct{})
	results := make(chan []WebhookDelivery, 2)
	failures := make(chan error, 2)
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-ready
			claimed, err := service.ClaimWebhookDeliveries(ctx, 100, time.Minute)
			results <- claimed
			failures <- err
		}()
	}
	close(ready)
	wg.Wait()
	close(results)
	close(failures)
	for err := range failures {
		if err != nil {
			t.Fatal(err)
		}
	}
	var all []WebhookDelivery
	for claims := range results {
		verifyWebhookClaims(t, claims, 32)
		all = append(all, claims...)
	}
	verifyWebhookClaims(t, all, 64)
	if _, err := database.Exec(ctx, `TRUNCATE plugin_idc_channel_webhook`); err != nil {
		t.Fatal(err)
	}
	seedWebhookClaims(t, database, 5)
	if _, err := database.Exec(ctx, fmt.Sprintf(`UPDATE plugin_idc_channel_webhook SET status='delivering',attempts=2,lease_token='old',lease_expires_at=CURRENT_TIMESTAMP-INTERVAL '1 minute' WHERE event_id='event_1'; UPDATE plugin_idc_channel_webhook SET status='delivering',attempts=%d,lease_token='exhausted',lease_expires_at=CURRENT_TIMESTAMP-INTERVAL '1 minute' WHERE event_id='event_2'; UPDATE plugin_idc_channel_webhook SET next_attempt_at=CURRENT_TIMESTAMP+INTERVAL '1 hour' WHERE event_id='event_3'; UPDATE plugin_idc_channel_webhook SET status='delivered' WHERE event_id='event_4'; UPDATE plugin_idc_channel_webhook SET status='delivering',lease_token='live',lease_expires_at=CURRENT_TIMESTAMP+INTERVAL '1 hour' WHERE event_id='event_5'`, WebhookMaxAttempts)); err != nil {
		t.Fatal(err)
	}
	recovered, err := service.ClaimWebhookDeliveries(ctx, 32, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	verifyWebhookClaims(t, recovered, 1)
	if recovered[0].EventID != "event_1" || recovered[0].Attempts != 3 || recovered[0].LeaseToken == "old" {
		t.Fatalf("expired lease did not advance: %+v", recovered)
	}
	failed, err := database.GetOne(ctx, `SELECT status,lease_token FROM plugin_idc_channel_webhook WHERE event_id='event_2'`)
	if err != nil || failed["status"].String() != WebhookStatusFailed || failed["lease_token"].String() != "" {
		t.Fatalf("exhausted lease was not fenced: %+v %v", failed, err)
	}
	if err = service.MarkWebhookDelivered(ctx, recovered[0].ChannelID, recovered[0].EventID, "old"); err == nil {
		t.Fatal("stale worker closed a new lease")
	}
	if err = service.MarkWebhookDelivered(ctx, recovered[0].ChannelID, recovered[0].EventID, recovered[0].LeaseToken); err != nil {
		t.Fatal(err)
	}
}

func TestWebhookClaimsRollbackWholeBatchPostgreSQL(t *testing.T) {
	database, service := newWebhookClaimFixture(t)
	ctx := context.Background()
	seedWebhookClaims(t, database, 3)
	if _, err := database.Exec(ctx, `CREATE FUNCTION reject_one_lease() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF NEW.status='delivering' AND NEW.event_id='event_2' THEN RAISE EXCEPTION 'fixture rejection'; END IF; RETURN NEW; END $$; CREATE TRIGGER reject_one_lease BEFORE UPDATE ON plugin_idc_channel_webhook FOR EACH ROW EXECUTE FUNCTION reject_one_lease()`); err != nil {
		t.Fatal(err)
	}
	if _, err := service.ClaimWebhookDeliveries(ctx, 32, time.Minute); err == nil {
		t.Fatal("batch failure was hidden")
	}
	changed, err := database.GetValue(ctx, `SELECT COUNT(*) FROM plugin_idc_channel_webhook WHERE status<>'pending' OR attempts<>0 OR lease_token<>''`)
	if err != nil || changed.Int() != 0 {
		t.Fatalf("batch partially committed: %v %v", changed, err)
	}
}
