package onboarding

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	_ "github.com/gogf/gf/contrib/drivers/pgsql/v2"
	"github.com/gogf/gf/v2/database/gdb"

	"lina-plugin-idc-core/backend/cap/onboardingcap"
)

func TestCreateVerifiedIntentPostgreSQL(t *testing.T) {
	database := onboardingDatabase(t)
	service, err := New(database, []byte(strings.Repeat("k", 32)))
	if err != nil {
		t.Fatal(err)
	}
	verifiedAt := time.Now().UTC().Truncate(time.Millisecond)
	request := onboardingcap.CreateVerifiedIntentRequest{OperationDigest: strings.Repeat("1", 64), RequestDigest: strings.Repeat("2", 64), UsernameDigest: strings.Repeat("3", 64), ContactDigest: strings.Repeat("4", 64), BrowserDigest: strings.Repeat("5", 64), ContactChannel: "email", CustomerType: "individual", ReturnPath: "/account", VerifiedAt: verifiedAt}
	created, err := service.CreateVerifiedIntent(context.Background(), request)
	if err != nil || created.Reused || len(created.Token) != 43 || len(created.IntentNo) != 36 {
		t.Fatalf("created=%+v err=%v", created, err)
	}
	replayed, err := service.CreateVerifiedIntent(context.Background(), request)
	if err != nil || !replayed.Reused || replayed.IntentNo != created.IntentNo || replayed.Token != created.Token {
		t.Fatalf("replayed=%+v err=%v", replayed, err)
	}
	changed := request
	changed.RequestDigest = strings.Repeat("6", 64)
	if _, err = service.CreateVerifiedIntent(context.Background(), changed); err == nil {
		t.Fatal("changed idempotent request unexpectedly accepted")
	}
	second := request
	second.OperationDigest = strings.Repeat("7", 64)
	second.RequestDigest = strings.Repeat("8", 64)
	if _, err = service.CreateVerifiedIntent(context.Background(), second); err != nil {
		t.Fatal(err)
	}
	value, err := database.GetValue(context.Background(), `SELECT status_code FROM plugin_idc_core_customer_onboarding_intent WHERE intent_no=?`, created.IntentNo)
	if err != nil || value.String() != "cancelled" {
		t.Fatalf("old status=%q err=%v", value.String(), err)
	}
}

func onboardingDatabase(t *testing.T) gdb.DB {
	t.Helper()
	dsn := strings.TrimSpace(os.Getenv("LINAPRO_IDC_CORE_TEST_DSN"))
	if dsn == "" {
		t.Skip("set LINAPRO_IDC_CORE_TEST_DSN")
	}
	admin, err := gdb.New(gdb.ConfigNode{Link: dsn})
	if err != nil {
		t.Fatal(err)
	}
	var raw [8]byte
	if _, err = rand.Read(raw[:]); err != nil {
		t.Fatal(err)
	}
	schema := "linapro_onboarding_" + hex.EncodeToString(raw[:])
	quoted := `"` + schema + `"`
	if _, err = admin.Exec(context.Background(), `CREATE SCHEMA `+quoted); err != nil {
		t.Fatal(err)
	}
	separator := "?"
	if strings.Contains(dsn, "?") {
		separator = "&"
	}
	database, err := gdb.New(gdb.ConfigNode{Link: dsn + separator + "search_path=" + url.QueryEscape(schema)})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = database.Close(context.Background())
		_, _ = admin.Exec(context.Background(), `DROP SCHEMA `+quoted+` CASCADE`)
		_ = admin.Close(context.Background())
	})
	_, err = database.Exec(context.Background(), `CREATE TABLE plugin_idc_core_customer_onboarding_intent (id BIGSERIAL PRIMARY KEY,tenant_id INT NOT NULL DEFAULT 0,intent_no VARCHAR(40) NOT NULL UNIQUE,token_digest CHAR(64) NOT NULL UNIQUE,browser_nonce_digest CHAR(64) NOT NULL,normalized_username_hash CHAR(64) NOT NULL,contact_channel_code VARCHAR(16) NOT NULL,contact_hash CHAR(64) NOT NULL,customer_type_code VARCHAR(16) NOT NULL,status_code VARCHAR(16) NOT NULL,return_path VARCHAR(512) NOT NULL,host_registration_user_id INT,consumed_by_user_id INT,result_customer_no VARCHAR(40) NOT NULL DEFAULT '',verified_contact_at TIMESTAMPTZ,expires_at TIMESTAMPTZ NOT NULL,consumed_at TIMESTAMPTZ,attempt_count INT NOT NULL DEFAULT 0,version BIGINT NOT NULL DEFAULT 1,created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP)`)
	if err != nil {
		t.Fatal(err)
	}
	_, file, _, _ := runtime.Caller(0)
	content, err := os.ReadFile(filepath.Join(filepath.Dir(file), "..", "..", "..", "..", "manifest", "sql", "048-customer-onboarding-intent-creation.sql"))
	if err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if _, err = database.Exec(context.Background(), string(content)); err != nil {
			t.Fatal(err)
		}
	}
	return database
}
