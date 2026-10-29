// This file verifies the source confirmation seam with real signed proofs,
// durable PostgreSQL replay claims and policy locks in disposable schemas.
package entitlementaccess

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
	"lina-core/pkg/plugin/capability/bizctxcap"
	"lina-core/pkg/plugin/capability/sessioncap"
	"lina-plugin-idc-core/backend/cap/customerauthcap"
	"lina-plugin-idc-core/backend/internal/service/reauth"
)

type confirmationActor struct{ customer string }

func (r *confirmationActor) CurrentCustomer(context.Context) (*customerauthcap.Subject, error) {
	return &customerauthcap.Subject{CustomerNo: r.customer, ActorUserID: 42}, nil
}

type confirmationContext struct{ current bizctxcap.CurrentContext }

func (c *confirmationContext) Current(context.Context) bizctxcap.CurrentContext { return c.current }

type confirmationSessions struct {
	sessioncap.Service
	current *sessioncap.SessionInfo
}

func (s *confirmationSessions) Current(context.Context) (*sessioncap.SessionInfo, error) {
	return s.current, nil
}

type confirmationKeys struct{}

func (confirmationKeys) Active(context.Context) (reauth.ActiveSigningKey, error) {
	return reauth.ActiveSigningKey{KeyID: "test", Key: []byte(strings.Repeat("k", 32))}, nil
}
func (confirmationKeys) Resolve(context.Context, string) ([]byte, error) {
	return []byte(strings.Repeat("k", 32)), nil
}

func confirmationFixture(t *testing.T) (gdb.DB, *confirmationActor, *confirmationContext, *confirmationSessions, reauth.Signer) {
	t.Helper()
	dsn := os.Getenv("LINAPRO_IDC_CORE_TEST_DSN")
	if dsn == "" {
		t.Skip("set LINAPRO_IDC_CORE_TEST_DSN")
	}
	ctx := t.Context()
	admin, err := gdb.New(gdb.ConfigNode{Link: dsn})
	if err != nil {
		t.Fatal(err)
	}
	var nonce [8]byte
	if _, err = rand.Read(nonce[:]); err != nil {
		t.Fatal(err)
	}
	schema := "native_confirmation_" + hex.EncodeToString(nonce[:])
	if _, err = admin.Exec(ctx, `CREATE SCHEMA `+schema); err != nil {
		t.Fatal(err)
	}
	original := gdb.GetAllConfig()
	separator := "?"
	if strings.Contains(dsn, "?") {
		separator = "&"
	}
	if err = gdb.SetConfig(gdb.Config{gdb.DefaultGroupName: {{Link: dsn + separator + "search_path=" + url.QueryEscape(schema)}}}); err != nil {
		t.Fatal(err)
	}
	db := g.DB()
	t.Cleanup(func() {
		cleanup := context.Background()
		if err := db.Close(cleanup); err != nil {
			t.Error(err)
		}
		if err := gdb.SetConfig(original); err != nil {
			t.Error(err)
		}
		if _, err := admin.Exec(cleanup, `DROP SCHEMA `+schema+` CASCADE`); err != nil {
			t.Error(err)
		}
		if err := admin.Close(cleanup); err != nil {
			t.Error(err)
		}
	})
	for _, file := range []string{"004-security-context.sql", "128-interface-delete-session-operations.sql"} {
		data, e := os.ReadFile(filepath.Join("..", "..", "..", "..", "manifest", "sql", file))
		if e != nil {
			t.Fatal(e)
		}
		source := strings.Split(string(data), "-- Interface configuration cleanup.")[0]
		if _, e = db.Exec(ctx, source); e != nil {
			t.Fatal(e)
		}
	}
	if _, err = db.Exec(ctx, `CREATE TABLE plugin_idc_core_server_operation_policy(tenant_id INT PRIMARY KEY,require_high_risk_reauth BOOLEAN NOT NULL);INSERT INTO plugin_idc_core_server_operation_policy VALUES(0,TRUE);CREATE TABLE confirmation_owner_task(value TEXT);`); err != nil {
		t.Fatal(err)
	}
	login := time.Now().Add(-30 * 24 * time.Hour)
	subject := &confirmationActor{customer: "customer-a"}
	current := &confirmationContext{current: bizctxcap.CurrentContext{UserID: 42, TenantID: 0, TokenID: "native-session"}}
	sessions := &confirmationSessions{current: &sessioncap.SessionInfo{ID: "native-session", TenantID: "0", UserID: "42", LoginAt: &login}}
	signer, err := reauth.NewHMACSigner(confirmationKeys{})
	if err != nil {
		t.Fatal(err)
	}
	return db, subject, current, sessions, signer
}
func confirmationService(t *testing.T, db gdb.DB, reader *confirmationActor, current *confirmationContext, sessions *confirmationSessions, signer reauth.Signer, ttl time.Duration) *Service {
	t.Helper()
	proofs, err := reauth.New(reauth.Config{ProofTTL: ttl}, current, sessions, reauth.NewDAOReplayStore(), signer)
	if err != nil {
		t.Fatal(err)
	}
	sessionProofs, ok := proofs.(reauth.SessionConfirmationService)
	if !ok {
		t.Fatal("session proof service missing")
	}
	s, err := NewWithSessionConfirmation(db, reader, sessionProofs)
	if err != nil {
		t.Fatal(err)
	}
	return s
}
func TestNativeConfirmationBindingPolicyAndRollbackPostgreSQL(t *testing.T) {
	db, reader, current, sessions, signer := confirmationFixture(t)
	service := confirmationService(t, db, reader, current, sessions, signer, 0)
	ctx := t.Context()
	digest := strings.Repeat("a", 64)
	proof, err := service.PrepareCurrentConfirmation(ctx, digest)
	if err != nil || !proof.Required || proof.Token == "" {
		t.Fatal("old active session cannot prepare", proof, err)
	}
	if err = service.ConsumeCurrentConfirmation(ctx, digest, proof.Token); err == nil {
		t.Fatal("consumed outside owner transaction")
	}
	for _, wrong := range []string{strings.Repeat("b", 64), "invalid"} {
		if err = db.Transaction(ctx, func(tx context.Context, _ gdb.TX) error {
			return service.ConsumeCurrentConfirmation(tx, wrong, proof.Token)
		}); err == nil {
			t.Fatal("wrong operation accepted")
		}
	}
	reader.customer = "customer-b"
	if err = db.Transaction(ctx, func(tx context.Context, _ gdb.TX) error {
		return service.ConsumeCurrentConfirmation(tx, digest, proof.Token)
	}); err == nil {
		t.Fatal("cross customer accepted")
	}
	reader.customer = "customer-a"
	sessions.current.ID = "replacement"
	current.current.TokenID = "replacement"
	if err = db.Transaction(ctx, func(tx context.Context, _ gdb.TX) error {
		return service.ConsumeCurrentConfirmation(tx, digest, proof.Token)
	}); err == nil {
		t.Fatal("replaced session accepted")
	}
	sessions.current.ID = "native-session"
	current.current.TokenID = "native-session"
	forced := errors.New("owner task failure")
	err = db.Transaction(ctx, func(tx context.Context, _ gdb.TX) error {
		if e := service.ConsumeCurrentConfirmation(tx, digest, proof.Token); e != nil {
			return e
		}
		return forced
	})
	if !errors.Is(err, forced) {
		t.Fatal(err)
	}
	if err = db.Transaction(ctx, func(tx context.Context, _ gdb.TX) error {
		return service.ConsumeCurrentConfirmation(tx, digest, proof.Token)
	}); err != nil {
		t.Fatal("rollback consumed proof", err)
	}
	if err = db.Transaction(ctx, func(tx context.Context, _ gdb.TX) error {
		return service.ConsumeCurrentConfirmation(tx, digest, proof.Token)
	}); err == nil {
		t.Fatal("replay accepted")
	}
	if _, err = db.Exec(ctx, `UPDATE plugin_idc_core_server_operation_policy SET require_high_risk_reauth=FALSE`); err != nil {
		t.Fatal(err)
	}
	disabled, err := service.PrepareCurrentConfirmation(ctx, digest)
	if err != nil || disabled.Required || disabled.Token != "" {
		t.Fatal("disabled policy", disabled, err)
	}
	if err = db.Transaction(ctx, func(tx context.Context, _ gdb.TX) error { return service.ConsumeCurrentConfirmation(tx, digest, "") }); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(ctx, `UPDATE plugin_idc_core_server_operation_policy SET require_high_risk_reauth=TRUE`); err != nil {
		t.Fatal(err)
	}
	if err = db.Transaction(ctx, func(tx context.Context, _ gdb.TX) error {
		return service.ConsumeCurrentConfirmation(tx, digest, disabled.Token)
	}); err == nil {
		t.Fatal("enabled policy reused old no-proof result")
	}
	expiredService := confirmationService(t, db, reader, current, sessions, signer, time.Millisecond)
	expired, err := expiredService.PrepareCurrentConfirmation(ctx, digest)
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(3 * time.Millisecond)
	if err = db.Transaction(ctx, func(tx context.Context, _ gdb.TX) error {
		return expiredService.ConsumeCurrentConfirmation(tx, digest, expired.Token)
	}); err == nil {
		t.Fatal("expired proof accepted")
	}
}
func TestNativeConfirmationTwoInstancesSingleClaimPostgreSQL(t *testing.T) {
	db, reader, current, sessions, signer := confirmationFixture(t)
	first := confirmationService(t, db, reader, current, sessions, signer, 0)
	second := confirmationService(t, db, reader, current, sessions, signer, 0)
	digest := strings.Repeat("c", 64)
	proof, err := first.PrepareCurrentConfirmation(t.Context(), digest)
	if err != nil {
		t.Fatal(err)
	}
	var successes atomic.Int32
	var wait sync.WaitGroup
	for i := range 8 {
		wait.Add(1)
		go func() {
			defer wait.Done()
			s := first
			if i%2 == 1 {
				s = second
			}
			e := db.Transaction(t.Context(), func(tx context.Context, _ gdb.TX) error {
				if e := s.ConsumeCurrentConfirmation(tx, digest, proof.Token); e != nil {
					return e
				}
				_, e := db.Exec(tx, `INSERT INTO confirmation_owner_task VALUES('accepted')`)
				return e
			})
			if e == nil {
				successes.Add(1)
			}
		}()
	}
	wait.Wait()
	if successes.Load() != 1 {
		t.Fatalf("accepted %d operations", successes.Load())
	}
	count, err := db.GetValue(t.Context(), `SELECT count(*) FROM confirmation_owner_task`)
	if err != nil || count.Int() != 1 {
		t.Fatal("owner tasks not atomic", count, err)
	}
}
