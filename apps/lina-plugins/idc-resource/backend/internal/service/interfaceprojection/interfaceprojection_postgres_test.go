package interfaceprojection

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"

	_ "github.com/gogf/gf/contrib/drivers/pgsql/v2"
	"github.com/gogf/gf/v2/database/gdb"

	"lina-plugin-idc-core/backend/cap/resourceinterfaceprojectioncap"
)

const interfaceProjectionDSNEnv = "LINAPRO_IDC_RESOURCE_TEST_DSN"

type recordingSink struct {
	mu       sync.Mutex
	calls    int
	failures int
	values   []resourceinterfaceprojectioncap.Projection
}

func (sink *recordingSink) Upsert(_ context.Context, value resourceinterfaceprojectioncap.Projection) error {
	sink.mu.Lock()
	defer sink.mu.Unlock()
	sink.calls++
	if sink.failures > 0 {
		sink.failures--
		return errors.New("injected sink failure")
	}
	sink.values = append(sink.values, value)
	return nil
}

func TestInterfaceProjectionPostgreSQLMultiWorkerRetryAndIdempotency(t *testing.T) {
	ctx, database := newInterfaceProjectionDatabase(t)
	projection := testProjection()
	seedProjectionBinding(t, ctx, database, projection)
	if err := database.Transaction(ctx, func(txCtx context.Context, tx gdb.TX) error {
		if err := Enqueue(txCtx, tx, projection, 1); err != nil {
			return err
		}
		return Enqueue(txCtx, tx, projection, 1)
	}); err != nil {
		t.Fatalf("enqueue exact replay: %v", err)
	}
	conflict := projection
	conflict.DisplayName = "Changed"
	if err := database.Transaction(ctx, func(txCtx context.Context, tx gdb.TX) error {
		return Enqueue(txCtx, tx, conflict, 1)
	}); err == nil {
		t.Fatal("conflicting projection replay was accepted")
	}

	sink := &recordingSink{}
	first, err := NewWorker(database, sink)
	if err != nil {
		t.Fatalf("construct first worker: %v", err)
	}
	second, err := NewWorker(database, sink)
	if err != nil {
		t.Fatalf("construct second worker: %v", err)
	}
	results := make(chan bool, 2)
	errorsSeen := make(chan error, 2)
	for _, worker := range []*Worker{first, second} {
		go func(current *Worker) {
			worked, runErr := current.RunOne(ctx)
			results <- worked
			errorsSeen <- runErr
		}(worker)
	}
	workedCount := 0
	for range 2 {
		if <-results {
			workedCount++
		}
		if runErr := <-errorsSeen; runErr != nil {
			t.Fatalf("concurrent projection delivery: %v", runErr)
		}
	}
	if workedCount != 1 || sink.calls != 1 || len(sink.values) != 1 {
		t.Fatalf("projection delivered more than once: worked=%d calls=%d values=%d", workedCount, sink.calls, len(sink.values))
	}

	secondProjection := projection
	secondProjection.DisplayName = "Supplier B"
	if err = database.Transaction(ctx, func(txCtx context.Context, tx gdb.TX) error {
		return Enqueue(txCtx, tx, secondProjection, 2)
	}); err != nil {
		t.Fatalf("enqueue retry projection: %v", err)
	}
	sink.failures = 1
	if worked, runErr := first.RunOne(ctx); !worked || runErr != nil {
		t.Fatalf("first retry delivery worked=%t err=%v", worked, runErr)
	}
	if _, err = database.Exec(ctx, `UPDATE plugin_idc_resource_interface_projection_outbox SET available_at=CURRENT_TIMESTAMP WHERE projection_sequence=2`); err != nil {
		t.Fatalf("release retry projection: %v", err)
	}
	if worked, runErr := second.RunOne(ctx); !worked || runErr != nil {
		t.Fatalf("second retry delivery worked=%t err=%v", worked, runErr)
	}
	var row struct {
		StatusCode      string `orm:"status_code"`
		AttemptCount    int    `orm:"attempt_count"`
		LastFailureCode string `orm:"last_failure_code"`
		Payload         string `orm:"projection_payload"`
	}
	if err = database.GetScan(ctx, &row, `SELECT status_code,attempt_count,last_failure_code,projection_payload::TEXT AS projection_payload FROM plugin_idc_resource_interface_projection_outbox WHERE projection_sequence=2`); err != nil {
		t.Fatalf("read retry projection: %v", err)
	}
	if row.StatusCode != "delivered" || row.AttemptCount != 2 || row.LastFailureCode != "" {
		t.Fatalf("retry projection state=%+v", row)
	}
	for _, forbidden := range []string{"endpoint", "password", "secret", "account", "token"} {
		if strings.Contains(strings.ToLower(row.Payload), forbidden) {
			t.Fatalf("projection leaked forbidden field %q: %s", forbidden, row.Payload)
		}
	}

	exhausted := projection
	exhausted.DisplayName = "Needs attention"
	if err = database.Transaction(ctx, func(txCtx context.Context, tx gdb.TX) error {
		return Enqueue(txCtx, tx, exhausted, 3)
	}); err != nil {
		t.Fatalf("enqueue exhausted projection: %v", err)
	}
	if _, err = database.Exec(ctx, `UPDATE plugin_idc_resource_interface_projection_outbox
SET status_code='delivering',attempt_count=9,locked_by='dead-node',
    lease_expires_at=CURRENT_TIMESTAMP - INTERVAL '1 second'
WHERE projection_sequence=3`); err != nil {
		t.Fatalf("seed expired projection lease: %v", err)
	}
	sink.failures = 1
	if worked, runErr := first.RunOne(ctx); !worked || runErr != nil {
		t.Fatalf("reclaim expired projection lease worked=%t err=%v", worked, runErr)
	}
	if err = database.GetScan(ctx, &row, `SELECT status_code,attempt_count,last_failure_code,projection_payload::TEXT AS projection_payload FROM plugin_idc_resource_interface_projection_outbox WHERE projection_sequence=3`); err != nil {
		t.Fatalf("read exhausted projection: %v", err)
	}
	if row.StatusCode != "manual_attention" || row.AttemptCount != maximumDeliveryAttempts ||
		row.LastFailureCode != "projection_delivery_failed" {
		t.Fatalf("exhausted projection did not stop safely: %+v", row)
	}
	if worked, runErr := second.RunOne(ctx); worked || runErr != nil {
		t.Fatalf("manual projection was redelivered worked=%t err=%v", worked, runErr)
	}

	fourth := projection
	fourth.InterfaceInstanceNo = "pfi_22222222222222222222222222222222"
	fourth.ResourceChannelRef = "CHN_projection00000002"
	fourth.DisplayName = "Fourth"
	fifth := projection
	fifth.InterfaceInstanceNo = fourth.InterfaceInstanceNo
	fifth.ResourceChannelRef = fourth.ResourceChannelRef
	fifth.DisplayName = "Fifth"
	seedProjectionBinding(t, ctx, database, fourth)
	if err = database.Transaction(ctx, func(txCtx context.Context, tx gdb.TX) error {
		if enqueueErr := Enqueue(txCtx, tx, fourth, 1); enqueueErr != nil {
			return enqueueErr
		}
		return Enqueue(txCtx, tx, fifth, 2)
	}); err != nil {
		t.Fatalf("enqueue ordered projections: %v", err)
	}
	if _, err = database.Exec(ctx, `UPDATE plugin_idc_resource_interface_projection_outbox SET available_at=CURRENT_TIMESTAMP + INTERVAL '1 hour' WHERE interface_instance_no=? AND projection_sequence=1`, fourth.InterfaceInstanceNo); err != nil {
		t.Fatalf("delay preceding projection: %v", err)
	}
	if worked, runErr := first.RunOne(ctx); worked || runErr != nil {
		t.Fatalf("later projection bypassed its predecessor worked=%t err=%v", worked, runErr)
	}
	if _, err = database.Exec(ctx, `UPDATE plugin_idc_resource_interface_projection_outbox SET available_at=CURRENT_TIMESTAMP WHERE interface_instance_no=? AND projection_sequence=1`, fourth.InterfaceInstanceNo); err != nil {
		t.Fatalf("release preceding projection: %v", err)
	}
	for _, worker := range []*Worker{first, second} {
		if worked, runErr := worker.RunOne(ctx); !worked || runErr != nil {
			t.Fatalf("deliver ordered projection worked=%t err=%v", worked, runErr)
		}
	}
	if len(sink.values) < 4 || sink.values[len(sink.values)-2].DisplayName != "Fourth" ||
		sink.values[len(sink.values)-1].DisplayName != "Fifth" {
		t.Fatalf("same-interface projections were delivered out of order: %+v", sink.values)
	}
}

func testProjection() resourceinterfaceprojectioncap.Projection {
	return resourceinterfaceprojectioncap.Projection{
		InterfaceInstanceNo: "pfi_11111111111111111111111111111111",
		DisplayName:         "Supplier A", ActorUserID: 101,
		OwnerID:            resourceinterfaceprojectioncap.OwnerID,
		ResourceChannelRef: "CHN_projection00000001",
		PluginID:           "server-connector", PluginReleaseVersion: "v1.0.0",
		ContractVersion: "v1", AuthorizedMethods: []string{"configuration.candidates"},
		ReleaseDigest: strings.Repeat("a", 64), DeclarationDigest: strings.Repeat("b", 64),
		NewBusinessState:               resourceinterfaceprojectioncap.StateStopped,
		InterfaceConfigurationRevision: "rci_11111111111111111111111111111111",
	}
}

func seedProjectionBinding(t *testing.T, ctx context.Context, database gdb.DB, projection resourceinterfaceprojectioncap.Projection) {
	t.Helper()
	if _, err := database.Exec(ctx, `INSERT INTO plugin_idc_resource_channel (channel_no,provider_id,provider_plugin_id,display_name,endpoint,status_code) VALUES (?, 'server-api', ?, ?, 'https://supplier.example', 'disabled')`, projection.ResourceChannelRef, projection.PluginID, projection.DisplayName); err != nil {
		t.Fatalf("seed projection channel: %v", err)
	}
	if _, err := database.Exec(ctx, `INSERT INTO plugin_idc_resource_interface_channel_binding (interface_instance_no,channel_no,binding_version) VALUES (?,?,1)`, projection.InterfaceInstanceNo, projection.ResourceChannelRef); err != nil {
		t.Fatalf("seed projection binding: %v", err)
	}
}

func newInterfaceProjectionDatabase(t *testing.T) (context.Context, gdb.DB) {
	t.Helper()
	ctx := context.Background()
	dsn := strings.TrimSpace(os.Getenv(interfaceProjectionDSNEnv))
	if dsn == "" {
		t.Skipf("set %s to run interface projection PostgreSQL tests", interfaceProjectionDSNEnv)
	}
	admin, err := gdb.New(gdb.ConfigNode{Link: dsn})
	if err != nil {
		t.Fatalf("create interface projection admin database: %v", err)
	}
	var suffix [8]byte
	if _, err = rand.Read(suffix[:]); err != nil {
		t.Fatalf("generate interface projection schema: %v", err)
	}
	schema := "linapro_interface_projection_test_" + hex.EncodeToString(suffix[:])
	if _, err = admin.Exec(ctx, `CREATE SCHEMA "`+schema+`"`); err != nil {
		t.Fatalf("create interface projection schema: %v", err)
	}
	database, err := gdb.New(gdb.ConfigNode{Link: appendProjectionDSNParameter(dsn, "search_path="+schema)})
	if err != nil {
		t.Fatalf("create scoped interface projection database: %v", err)
	}
	t.Cleanup(func() {
		if closeErr := database.Close(ctx); closeErr != nil {
			t.Errorf("close interface projection database: %v", closeErr)
		}
		if strings.HasPrefix(schema, "linapro_interface_projection_test_") {
			if _, dropErr := admin.Exec(ctx, `DROP SCHEMA "`+schema+`" CASCADE`); dropErr != nil {
				t.Errorf("drop interface projection schema: %v", dropErr)
			}
		}
		if closeErr := admin.Close(ctx); closeErr != nil {
			t.Errorf("close interface projection admin database: %v", closeErr)
		}
	})
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve interface projection migration path")
	}
	paths, err := filepath.Glob(filepath.Join(filepath.Dir(file), "..", "..", "..", "..", "manifest", "sql", "[0-9][0-9][0-9]-*.sql"))
	if err != nil || len(paths) == 0 {
		t.Fatalf("resolve interface projection migrations: %v", err)
	}
	for _, path := range paths {
		migration, readErr := os.ReadFile(path)
		if readErr != nil {
			t.Fatalf("read migration %s: %v", filepath.Base(path), readErr)
		}
		if _, executeErr := database.Exec(ctx, string(migration)); executeErr != nil {
			t.Fatalf("execute migration %s: %v", filepath.Base(path), executeErr)
		}
	}
	return ctx, database
}

func appendProjectionDSNParameter(dsn, parameter string) string {
	if parsed, err := url.Parse(dsn); err == nil && parsed.Scheme != "" {
		query := parsed.Query()
		pieces := strings.SplitN(parameter, "=", 2)
		query.Set(pieces[0], pieces[1])
		parsed.RawQuery = query.Encode()
		return parsed.String()
	}
	separator := "?"
	if strings.Contains(dsn, "?") {
		separator = "&"
	}
	return dsn + separator + parameter
}
