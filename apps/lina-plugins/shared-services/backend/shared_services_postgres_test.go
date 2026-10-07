package backend

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"image/png"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	_ "github.com/gogf/gf/contrib/drivers/pgsql/v2"
	"github.com/gogf/gf/v2/database/gdb"

	"lina-core/pkg/plugin/capability/capregistry"
	"lina-plugin-shared-services/backend/cap/challengecap"
	"lina-plugin-shared-services/backend/cap/inboxcap"
	"lina-plugin-shared-services/backend/internal/service/challenge"
	"lina-plugin-shared-services/backend/internal/service/inbox"
)

var equationPattern = regexp.MustCompile(`([0-9]+) \+ ([0-9]+) = \?`)

func TestSharedServicesPostgreSQLProofAndInboxInvariants(t *testing.T) {
	database, cleanup := openSharedServicesPostgreSQL(t)
	defer cleanup()

	challengeService, err := challenge.New(database, challenge.Config{Provider: "local", RequestTimeout: time.Second})
	if err != nil {
		t.Fatalf("construct challenge service: %v", err)
	}
	challengeClient, err := challenge.Bind(challengeService, "test-consumer")
	if err != nil {
		t.Fatalf("bind challenge caller: %v", err)
	}
	binding := "browser-random-0123456789"
	presentation, err := challengeClient.Create(t.Context(), challengecap.CreateRequest{Purpose: "support.presales", Binding: binding, Kind: challengecap.PresentationImage})
	if err != nil || presentation.Kind != challengecap.PresentationImage || presentation.ImageMediaType != "image/png" || len(presentation.ImageBytes) == 0 || presentation.AccessiblePrompt != "" {
		t.Fatalf("create image challenge: presentation=%+v err=%v", presentation, err)
	}
	if _, decodeErr := png.Decode(bytes.NewReader(presentation.ImageBytes)); decodeErr != nil || bytes.Contains(presentation.ImageBytes, []byte(" + ")) {
		t.Fatalf("image challenge is not an opaque PNG: err=%v", decodeErr)
	}
	accessibleBinding := "accessible-browser-0123456789"
	accessible, err := challengeClient.Create(t.Context(), challengecap.CreateRequest{Purpose: "support.presales", Binding: accessibleBinding, Kind: challengecap.PresentationAccessible})
	if err != nil || accessible.Kind != challengecap.PresentationAccessible || len(accessible.ImageBytes) != 0 || accessible.AccessiblePrompt == "" {
		t.Fatalf("create accessible challenge: presentation=%+v err=%v", accessible, err)
	}
	match := equationPattern.FindStringSubmatch(accessible.AccessiblePrompt)
	if len(match) != 3 {
		t.Fatalf("accessible challenge does not contain the generated equation")
	}
	left, _ := strconv.Atoi(match[1])
	right, _ := strconv.Atoi(match[2])
	proof, err := challengeClient.Complete(t.Context(), challengecap.CompleteRequest{
		ChallengeNo: accessible.ChallengeNo, Purpose: "support.presales", Binding: accessibleBinding, Answer: strconv.Itoa(left + right),
	})
	if err != nil || proof.Token == "" || !proof.ExpiresAt.After(time.Now()) {
		t.Fatalf("complete challenge: equation=%q answer=%d proof=%+v err=%v", match[0], left+right, proof, err)
	}
	consumed, err := challengeClient.Consume(t.Context(), challengecap.ConsumeRequest{Token: proof.Token, Purpose: "support.presales", Binding: accessibleBinding})
	if err != nil || consumed.ChallengeNo != accessible.ChallengeNo {
		t.Fatalf("consume proof: result=%+v err=%v", consumed, err)
	}
	if _, err = challengeClient.Consume(t.Context(), challengecap.ConsumeRequest{Token: proof.Token, Purpose: "support.presales", Binding: accessibleBinding}); err == nil {
		t.Fatal("replayed proof was accepted")
	}
	if _, err = challengeClient.Consume(t.Context(), challengecap.ConsumeRequest{Token: proof.Token, Purpose: "account.register", Binding: accessibleBinding}); err == nil {
		t.Fatal("cross-purpose proof was accepted")
	}

	inboxService, err := inbox.New(database)
	if err != nil {
		t.Fatalf("construct inbox service: %v", err)
	}
	inboxClient, err := inbox.Bind(inboxService, "test-consumer")
	if err != nil {
		t.Fatalf("bind inbox caller: %v", err)
	}
	request := inboxcap.PublishRequest{RecipientUserID: 88, SourceType: "support_ticket", SourceNo: "ticket_1",
		IdempotencyKey: "event_12345678", Title: "Ticket updated", Body: "A reply is available.",
		Category: "support", ActionURL: "/support/tickets/ticket_1"}
	first, err := inboxClient.Publish(t.Context(), request)
	if err != nil || first.MessageNo == "" || first.EvidenceNo == "" || first.Reused {
		t.Fatalf("publish inbox message: result=%+v err=%v", first, err)
	}
	const concurrentPublishes = 12
	results := make(chan *inboxcap.PublishResult, concurrentPublishes)
	errors := make(chan error, concurrentPublishes)
	var wait sync.WaitGroup
	for range concurrentPublishes {
		wait.Add(1)
		go func() {
			defer wait.Done()
			result, publishErr := inboxClient.Publish(context.Background(), request)
			results <- result
			errors <- publishErr
		}()
	}
	wait.Wait()
	close(results)
	close(errors)
	for publishErr := range errors {
		if publishErr != nil {
			t.Fatalf("idempotent concurrent publish failed: %v", publishErr)
		}
	}
	for result := range results {
		if result == nil || result.MessageNo != first.MessageNo || result.EvidenceNo != first.EvidenceNo || !result.Reused {
			t.Fatalf("idempotent concurrent publish mismatch: %+v", result)
		}
	}
	changed := request
	changed.Body = "Different content."
	if _, err = inboxClient.Publish(t.Context(), changed); err == nil {
		t.Fatal("changed content reused the same idempotency key")
	}
	messageCount, err := database.GetValue(t.Context(), "SELECT count(*) FROM plugin_shared_services_inbox_message")
	if err != nil || messageCount.Int() != 1 {
		t.Fatalf("unexpected inbox message count: count=%d err=%v", messageCount.Int(), err)
	}
	evidenceCount, err := database.GetValue(t.Context(), "SELECT count(*) FROM plugin_shared_services_inbox_delivery_evidence")
	if err != nil || evidenceCount.Int() != 1 {
		t.Fatalf("unexpected inbox evidence count: count=%d err=%v", evidenceCount.Int(), err)
	}
	if _, err = database.Exec(t.Context(), `INSERT INTO plugin_shared_services_inbox_message
(tenant_id,message_no,recipient_user_id,caller_plugin_id,source_type,source_no,idempotency_key,content_digest,title,body,category)
VALUES(1,'forbidden',1,'test-consumer','source','one','idempotency','digest','title','body','category')`); err == nil {
		t.Fatal("cross-tenant inbox row was accepted")
	}

	challengeInvoker := challenge.NewInvoker()
	if err = challengeInvoker.Bind(challengeService); err != nil {
		t.Fatalf("bind dynamic challenge invoker: %v", err)
	}
	createPayload, err := json.Marshal(challengecap.CreateRequest{
		Purpose: "support.presales", Binding: "dynamic-browser-0123456789", Kind: challengecap.PresentationAccessible,
	})
	if err != nil {
		t.Fatalf("encode dynamic challenge request: %v", err)
	}
	dynamicChallenge, err := challengeInvoker.Invoke(t.Context(), capregistry.Invocation{
		CallerPluginID: "dynamic-consumer", OwnerPluginID: challengecap.OwnerPluginID,
		Service: challengecap.ServiceName, Version: challengecap.ContractVersion,
		Method: string(challengecap.MethodCreate), Payload: createPayload,
	})
	if err != nil || dynamicChallenge == nil || len(dynamicChallenge.Payload) == 0 {
		t.Fatalf("dynamic challenge invocation failed: result=%+v err=%v", dynamicChallenge, err)
	}

	inboxInvoker := inbox.NewInvoker()
	if err = inboxInvoker.Bind(inboxService); err != nil {
		t.Fatalf("bind dynamic inbox invoker: %v", err)
	}
	dynamicPublishPayload, err := json.Marshal(inboxcap.PublishRequest{
		RecipientUserID: 89, SourceType: "support_ticket", SourceNo: "ticket_2",
		IdempotencyKey: "event_dynamic_1234", Title: "Ticket assigned", Body: "A ticket is ready.",
		Category: "support", ActionURL: "/support/tickets/ticket_2",
	})
	if err != nil {
		t.Fatalf("encode dynamic inbox request: %v", err)
	}
	dynamicMessage, err := inboxInvoker.Invoke(t.Context(), capregistry.Invocation{
		CallerPluginID: "dynamic-consumer", OwnerPluginID: inboxcap.OwnerPluginID,
		Service: inboxcap.ServiceName, Version: inboxcap.ContractVersion,
		Method: inboxcap.MethodPublish, Payload: dynamicPublishPayload,
	})
	if err != nil || dynamicMessage == nil || len(dynamicMessage.Payload) == 0 {
		t.Fatalf("dynamic inbox invocation failed: result=%+v err=%v", dynamicMessage, err)
	}
}

func openSharedServicesPostgreSQL(t *testing.T) (gdb.DB, func()) {
	t.Helper()
	dsn := strings.TrimSpace(os.Getenv("LINAPRO_SHARED_SERVICES_TEST_DSN"))
	if dsn == "" {
		t.Skip("set LINAPRO_SHARED_SERVICES_TEST_DSN to run PostgreSQL tests")
	}
	admin, err := gdb.New(gdb.ConfigNode{Link: dsn})
	if err != nil {
		t.Fatalf("open PostgreSQL: %v", err)
	}
	var suffix [8]byte
	if _, err = rand.Read(suffix[:]); err != nil {
		t.Fatalf("generate schema suffix: %v", err)
	}
	schema := "linapro_shared_test_" + hex.EncodeToString(suffix[:])
	if _, err = admin.Exec(t.Context(), "CREATE SCHEMA "+quoteIdentifier(schema)); err != nil {
		t.Fatalf("create test schema: %v", err)
	}
	database, err := gdb.New(gdb.ConfigNode{Link: appendDSNOption(dsn, "search_path="+schema)})
	if err != nil {
		t.Fatalf("open scoped PostgreSQL: %v", err)
	}
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve shared plugin path")
	}
	pluginRoot := filepath.Clean(filepath.Join(filepath.Dir(file), ".."))
	content, err := os.ReadFile(filepath.Join(pluginRoot, "manifest", "sql", "001-shared-services.sql"))
	if err != nil {
		t.Fatalf("read shared migration: %v", err)
	}
	if _, err = database.Exec(t.Context(), string(content)); err != nil {
		t.Fatalf("apply shared migration: %v", err)
	}
	if _, err = database.Exec(t.Context(), string(content)); err != nil {
		t.Fatalf("reapply shared migration: %v", err)
	}
	cleanup := func() {
		_ = database.Close(context.Background())
		if strings.HasPrefix(schema, "linapro_shared_test_") {
			_, _ = admin.Exec(context.Background(), "DROP SCHEMA "+quoteIdentifier(schema)+" CASCADE")
		}
		_ = admin.Close(context.Background())
	}
	return database, cleanup
}

func quoteIdentifier(value string) string { return `"` + strings.ReplaceAll(value, `"`, `""`) + `"` }

func appendDSNOption(dsn string, option string) string {
	separator := "?"
	if strings.Contains(dsn, "?") {
		separator = "&"
	}
	return dsn + separator + option
}
