// This file verifies immutable login ownership, one-item binding, edit retries,
// policy limits and that plaintext never enters purchase persistence.
package purchaseserverlogin

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	_ "github.com/gogf/gf/contrib/drivers/pgsql/v2"
	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"
	"lina-plugin-idc-core/backend/cap/secretcap"
	"lina-plugin-idc-core/backend/internal/model/do"
	"lina-plugin-idc-core/backend/internal/service/principal"
)

type audienceKey struct{}
type testIdentity struct{}

func (testIdentity) Current(ctx context.Context) (*principal.Audience, error) {
	value, _ := ctx.Value(audienceKey{}).(*principal.Audience)
	return value, nil
}

type testResolver struct{ target *Target }

func (r *testResolver) ResolvePurchaseServerLoginTarget(context.Context, string, string) (*Target, error) {
	return r.target, nil
}

type testSecrets struct {
	secretcap.Source
	scopes   []secretcap.Scope
	creates  int
	revoked  bool
	produced []byte
	value    string
}

func (s *testSecrets) Manager(scope secretcap.Scope) (secretcap.Manager, error) {
	s.scopes = append(s.scopes, scope)
	return &testManager{secrets: s}, nil
}

type testManager struct {
	secretcap.Manager
	secrets *testSecrets
}

func (s *testManager) Create(_ context.Context, key string, produce secretcap.PlaintextProducer) (secretcap.CreateResult, error) {
	s.secrets.creates++
	if err := produce(func(value []byte) error { s.secrets.produced = value; s.secrets.value = string(value); return nil }); err != nil {
		return secretcap.CreateResult{}, err
	}
	handle, err := secretcap.NewHandle("secret://test/" + key)
	return secretcap.CreateResult{Handle: handle, Metadata: secretcap.Metadata{Configured: true, UpdatedAt: time.Now()}}, err
}
func (s *testManager) Revoke(context.Context, secretcap.Handle, time.Time) (secretcap.Metadata, error) {
	s.secrets.revoked = true
	return secretcap.Metadata{}, nil
}

func testContext(customer string, userID int) context.Context {
	return context.WithValue(context.Background(), audienceKey{}, &principal.Audience{Code: principal.AudienceCustomer, Status: principal.AudienceStatusActive, CustomerNo: customer, UserID: userID})
}
func testTarget() *Target {
	return &Target{ProductVersionNo: "version-1", ConfigurationRevision: "revision-1", ChannelNo: "resource-channel-1", HostnameEnabled: true, PasswordEnabled: true}
}

func TestManualChoicesRespectSupplierLimitsWithoutGeneratorRestrictions(t *testing.T) {
	target := testTarget()
	target.HostnameRules = HostnameRules{Prefix: "merchant", Length: 12, Characters: []string{"digits"}}
	target.PasswordRules = PasswordRules{Length: 16, Characters: []string{"lowercase", "digits"}}
	for _, test := range []struct {
		name, hostname, password string
		want                     bool
	}{
		{"short manual host", "a", "abc123", true}, {"manual length not generator length", "my-server.example", "abcdef", true},
		{"automatic", "", "", true}, {"invalid hostname label", "a.-b", "", false}, {"empty label", "a..b", "", false},
		{"hostname uppercase", "Server", "", false}, {"hostname too long", strings.Repeat("x", 64), "", false},
		{"password too short", "", "a1", false}, {"password max", "", strings.Repeat("a", 20), true}, {"password long", "", strings.Repeat("a", 21), false},
		{"password class union", "", "abcdef", true}, {"password disallowed class", "", "ABC123", false}, {"whitespace", "", "abc 123", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := validChoices(Input{Hostname: test.hostname, Password: test.password}, target); got != test.want {
				t.Fatalf("valid=%v want=%v", got, test.want)
			}
		})
	}
	target.HostnameEnabled = false
	if validChoices(Input{Hostname: "my-server"}, target) {
		t.Fatal("disabled hostname input allowed")
	}
	target.PasswordEnabled = false
	if validChoices(Input{Password: "abc123"}, target) {
		t.Fatal("disabled password input allowed")
	}
}

func TestRegistryOwnershipPrivatePersistenceAndEditPostgreSQL(t *testing.T) {
	db := testDatabase(t)
	target := testTarget()
	secrets := &testSecrets{}
	service, err := New(db, testIdentity{}, secrets, &testResolver{target: target})
	if err != nil {
		t.Fatal(err)
	}
	ctx := testContext("customer-1", 41)
	if _, err = service.Create(context.Background(), Input{ConfigurationRevision: "revision-1", Hostname: "server"}); err == nil {
		t.Fatal("anonymous accepted")
	}
	created, err := service.Create(ctx, Input{ConfigurationRevision: "revision-1", Hostname: "my-server", Password: "Safe123!"})
	if err != nil {
		t.Fatal(err)
	}
	if len(created.ServerLoginToken) != 68 || created.Hostname != "my-server" || !created.PasswordConfigured {
		t.Fatalf("invalid safe create projection")
	}
	for _, value := range secrets.produced {
		if value != 0 {
			t.Fatal("temporary plaintext view was retained")
		}
	}
	digest := sha256.Sum256([]byte(target.ChannelNo))
	scope := secrets.scopes[0]
	if scope.OwnerPluginID != "idc-resource" || scope.CallerScope != "resource.channel.c"+hex.EncodeToString(digest[:16]) {
		t.Fatalf("resource channel secret scope mismatch")
	}
	encoded, err := json.Marshal(created)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "Safe123!") || strings.Contains(string(encoded), "secret://") {
		t.Fatal("create projection leaked password or secret handle")
	}
	persisted, err := db.GetValue(ctx, `SELECT row_to_json(registry)::text FROM plugin_idc_core_purchase_server_login registry WHERE server_login_token=?`, created.ServerLoginToken)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(persisted.String(), "Safe123!") {
		t.Fatal("password entered registry database")
	}
	resolved, err := service.Resolve(ctx, created.ServerLoginToken, "customer-1", "version-1")
	if err != nil || resolved.PasswordSecretRef == "" {
		t.Fatalf("owned resolve: %v", err)
	}
	for _, test := range []struct {
		ctx               context.Context
		customer, version string
	}{
		{testContext("customer-1", 42), "customer-1", "version-1"}, {testContext("customer-2", 41), "customer-2", "version-1"}, {ctx, "customer-1", "version-2"},
	} {
		if _, err = service.Resolve(test.ctx, created.ServerLoginToken, test.customer, test.version); err == nil {
			t.Fatal("foreign login/customer/version accepted")
		}
		if test.ctx != ctx {
			foreignPassword := "PrivateTyped9!"
			if _, err = service.Create(test.ctx, Input{ConfigurationRevision: "revision-1", Password: foreignPassword, PreviousServerLoginToken: created.ServerLoginToken}); err == nil || strings.Contains(err.Error(), foreignPassword) || strings.Contains(gerror.Stack(err), foreignPassword) {
				t.Fatal("foreign edit accepted or leaked submitted password")
			}
		}
	}
	if err = service.Bind(ctx, created.ServerLoginToken, "customer-1", "version-1", "cart-item-1"); err != nil {
		t.Fatal(err)
	}
	if err = service.Bind(ctx, created.ServerLoginToken, "customer-1", "version-1", "cart-item-1"); err != nil {
		t.Fatal("idempotent bind failed")
	}
	if err = service.Bind(ctx, created.ServerLoginToken, "customer-1", "version-1", "cart-item-2"); err == nil {
		t.Fatal("token reused for second cart item")
	}
	edited, err := service.Create(ctx, Input{ConfigurationRevision: "revision-1", Hostname: "new-host", PreviousServerLoginToken: created.ServerLoginToken})
	if err != nil {
		t.Fatal(err)
	}
	inherited, err := service.Resolve(ctx, edited.ServerLoginToken, "customer-1", "version-1")
	if err != nil {
		t.Fatal(err)
	}
	if inherited.PasswordSecretRef != resolved.PasswordSecretRef || inherited.CartItemNo != "cart-item-1" || inherited.Hostname != "new-host" || secrets.creates != 1 {
		t.Fatal("editing failed to preserve original private password and cart binding")
	}
	if err = service.Bind(ctx, edited.ServerLoginToken, "customer-1", "version-1", "cart-item-2"); err == nil {
		t.Fatal("inherited password copied to unrelated item")
	}
	if _, err = service.Create(testContext("customer-2", 42), Input{ConfigurationRevision: "revision-1", PreviousServerLoginToken: created.ServerLoginToken, Hostname: "other"}); err == nil {
		t.Fatal("foreign previous token accepted")
	}
}

func TestRegistryAutomaticGenerationAndRollbackPostgreSQL(t *testing.T) {
	db := testDatabase(t)
	secrets := &testSecrets{}
	target := testTarget()
	resolver := &testResolver{target: target}
	service, err := New(db, testIdentity{}, secrets, resolver)
	if err != nil {
		t.Fatal(err)
	}
	ctx := testContext("customer-1", 41)
	automatic, err := service.Create(ctx, Input{ConfigurationRevision: "revision-1"})
	if err != nil || automatic.ServerLoginToken != "" || secrets.creates != 0 {
		t.Fatalf("empty choices must preserve automatic generation: %v", err)
	}
	target.HostnameEnabled = false
	if _, err = service.Create(ctx, Input{ConfigurationRevision: "revision-1", Hostname: "server"}); err == nil {
		t.Fatal("disabled hostname accepted")
	}
	target.HostnameEnabled = true
	if _, err = service.Create(ctx, Input{ConfigurationRevision: "stale", Hostname: "server"}); err == nil {
		t.Fatal("stale revision accepted")
	}
	created, err := service.Create(ctx, Input{ConfigurationRevision: "revision-1", Hostname: "server"})
	if err != nil {
		t.Fatal(err)
	}
	rollback := gerror.New("rollback cart insertion")
	err = db.Transaction(ctx, func(txCtx context.Context, _ gdb.TX) error {
		if bindErr := service.Bind(txCtx, created.ServerLoginToken, "customer-1", "version-1", "cart-rollback"); bindErr != nil {
			return bindErr
		}
		return rollback
	})
	if err == nil {
		t.Fatal("expected rollback")
	}
	resolved, err := service.Resolve(ctx, created.ServerLoginToken, "customer-1", "version-1")
	if err != nil || resolved.CartItemNo != "" {
		t.Fatal("rolled back cart left token bound")
	}
	var wait sync.WaitGroup
	success := make(chan bool, 2)
	for _, cart := range []string{"cart-a", "cart-b"} {
		wait.Add(1)
		go func(cart string) {
			defer wait.Done()
			success <- service.Bind(ctx, created.ServerLoginToken, "customer-1", "version-1", cart) == nil
		}(cart)
	}
	wait.Wait()
	close(success)
	wins := 0
	for ok := range success {
		if ok {
			wins++
		}
	}
	if wins != 1 {
		t.Fatalf("concurrent cart token must have one winner, got %d", wins)
	}
	if _, err = db.Exec(ctx, `DROP TABLE plugin_idc_core_purchase_server_login`); err != nil {
		t.Fatal(err)
	}
	if _, err = service.Create(ctx, Input{ConfigurationRevision: "revision-1", Password: "Safe123!"}); err == nil || !secrets.revoked {
		t.Fatal("failed registry insertion did not revoke orphan secret")
	}
}

func TestRegistryConstructionRequiresAllDependencies(t *testing.T) {
	if _, err := New(nil, nil, nil, nil); err == nil {
		t.Fatal("nil runtime dependencies accepted")
	}
}

func testDatabase(t *testing.T) gdb.DB {
	t.Helper()
	dsn := strings.TrimSpace(os.Getenv("LINAPRO_IDC_CORE_TEST_DSN"))
	if dsn == "" {
		t.Skip("set LINAPRO_IDC_CORE_TEST_DSN for PostgreSQL registry integration")
	}
	ctx := context.Background()
	admin, err := gdb.New(gdb.ConfigNode{Link: dsn})
	if err != nil {
		t.Fatal(err)
	}
	var random [8]byte
	if _, err = rand.Read(random[:]); err != nil {
		t.Fatal(err)
	}
	schema := "purchase_login_test_" + hex.EncodeToString(random[:])
	if _, err = admin.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	sep := "?"
	if strings.Contains(dsn, "?") {
		sep = "&"
	}
	db, err := gdb.New(gdb.ConfigNode{Link: dsn + sep + "search_path=" + schema})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := db.Close(ctx); err != nil {
			t.Error(err)
		}
		if _, err := admin.Exec(ctx, "DROP SCHEMA "+schema+" CASCADE"); err != nil {
			t.Error(err)
		}
		if err := admin.Close(ctx); err != nil {
			t.Error(err)
		}
	})
	if _, err = db.Exec(ctx, `CREATE TABLE plugin_idc_core_commerce_configuration_snapshot(id BIGINT);CREATE TABLE plugin_idc_core_commerce_structured_configuration_snapshot(id BIGINT)`); err != nil {
		t.Fatal(err)
	}
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("migration path unavailable")
	}
	migration, err := os.ReadFile(filepath.Join(filepath.Dir(file), "../../../../manifest/sql/157-purchase-server-login.sql"))
	if err != nil {
		t.Fatal(err)
	}
	for attempt := 0; attempt < 2; attempt++ {
		if _, err = db.Exec(ctx, string(migration)); err != nil {
			t.Fatal(err)
		}
	}
	count, err := db.Model(do.PurchaseServerLogin{}).Ctx(ctx).Count()
	if err != nil || count != 0 {
		t.Fatalf("registry initialization: %v", err)
	}
	return db
}

func TestRejectedLoginChoicesNeverEchoPassword(t *testing.T) {
	secret := "NeverEchoThisPasswordAcrossErrors123!"
	for _, scenario := range []struct {
		name     string
		ctx      context.Context
		target   *Target
		revision string
	}{
		{"invalid password", testContext("customer-1", 41), testTarget(), "revision-1"},
		{"disabled password", testContext("customer-1", 41), &Target{ProductVersionNo: "version-1", ConfigurationRevision: "revision-1", ChannelNo: "channel-1"}, "revision-1"},
		{"anonymous", context.Background(), testTarget(), "revision-1"},
		{"expired product revision", testContext("customer-1", 41), testTarget(), "old-revision"},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			service := &serviceImpl{principal: testIdentity{}, resolver: &testResolver{target: scenario.target}, secrets: &testSecrets{}}
			result, err := service.Create(scenario.ctx, Input{ConfigurationRevision: scenario.revision, Password: secret})
			if err == nil || result != nil {
				t.Fatal("invalid secret was accepted")
			}
			if strings.Contains(err.Error(), secret) || strings.Contains(gerror.Stack(err), secret) {
				t.Fatal("rejected password entered error or log stack")
			}
			response, marshalErr := json.Marshal(struct {
				Message string `json:"message"`
			}{Message: err.Error()})
			if marshalErr != nil {
				t.Fatal(marshalErr)
			}
			if strings.Contains(string(response), secret) {
				t.Fatal("rejected password entered error response")
			}
		})
	}
}
