// These tests verify real PostgreSQL ownership, rotation concurrency and exact call pagination.
package channel

import (
	"context"
	"lina-plugin-idc-core/backend/cap/channelcommercecap"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// platformCore supplies only the current-customer capability while reusing payer validation.
type platformCore struct {
	applicationCore
	customer string
}

func (c platformCore) CurrentCustomer(context.Context) (*channelcommercecap.CustomerSubject, error) {
	return &channelcommercecap.CustomerSubject{CustomerNo: c.customer, Status: "active", IdentityStatus: "unverified", ActorUserID: 1}, nil
}

func TestCustomerPlatformPostgreSQL(t *testing.T) {
	db := channelPageDatabase(t)
	ctx := context.Background()
	for _, name := range []string{"001-channel-core.sql", "002-channel-webhook-delivery.sql", "003-channel-token-security.sql", "004-channel-commerce-management.sql", "005-channel-list-pagination.sql", "006-customer-open-platform.sql"} {
		content, err := os.ReadFile(filepath.Join("../../../../manifest/sql", name))
		if err != nil {
			t.Fatal(err)
		}
		for range 2 {
			if _, err = db.Exec(ctx, string(content)); err != nil {
				t.Fatal(err)
			}
		}
	}
	svc := New(db, platformCore{customer: "platform-a"}, applicationContext{scope: 1})
	empty, err := svc.OwnPlatform(ctx)
	if err != nil || empty.Authorized {
		t.Fatalf("unbound: %+v %v", empty, err)
	}
	if _, err = svc.RotateOwnSecret(ctx, 1); err == nil {
		t.Fatal("unbound rotated")
	}
	expires := time.Now().Add(time.Hour).Truncate(time.Millisecond)
	created, err := svc.CreateToken(ctx, TokenInput{CustomerNo: "platform-a", DisplayName: "Platform", Scopes: []string{"catalog.read"}, SourceCIDRs: []string{"127.0.0.0/8"}, ExpiresAt: &expires})
	if err != nil {
		t.Fatal(err)
	}
	own, err := svc.OwnPlatform(ctx)
	if err != nil || !own.Authorized || own.AppID != created.AppID || len(own.Tokens) != 1 {
		t.Fatalf("own: %+v %v", own, err)
	}
	other := New(db, platformCore{customer: "platform-b"}, applicationContext{scope: 1})
	empty, err = other.OwnPlatform(ctx)
	if err != nil || empty.Authorized {
		t.Fatalf("cross-customer metadata: %+v %v", empty, err)
	}
	if _, err = other.OwnCalls(ctx, PageQuery{}); err == nil {
		t.Fatal("cross-customer logs")
	}
	if _, err = other.RotateOwnSecret(ctx, created.ID); err == nil {
		t.Fatal("cross-customer rotation")
	}
	var wg sync.WaitGroup
	results := make(chan TokenResult, 2)
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			result, err := svc.RotateOwnSecret(ctx, created.ID)
			if err == nil {
				results <- result
			}
		}()
	}
	wg.Wait()
	close(results)
	var rotated TokenResult
	count := 0
	for result := range results {
		count++
		rotated = result
	}
	if count != 1 {
		t.Fatalf("expected one concurrent reset winner, got %d", count)
	}
	if _, err = svc.AuthenticateToken(ctx, created.Token, "127.0.0.1", "catalog.read"); err == nil {
		t.Fatal("old secret accepted")
	}
	if _, err = svc.AuthenticateToken(ctx, rotated.Token, "127.0.0.1", "catalog.read"); err != nil {
		t.Fatal(err)
	}
	if _, err = svc.AuthenticateToken(ctx, rotated.Token, "127.0.0.1", "order.create"); err == nil {
		t.Fatal("rotation expanded scope")
	}
	if _, err = svc.AuthenticateToken(ctx, rotated.Token, "192.0.2.1", "catalog.read"); err == nil {
		t.Fatal("rotation removed source limit")
	}
	own, err = svc.OwnPlatform(ctx)
	if err != nil || !own.Tokens[0].ExpiresAt.Equal(expires) {
		t.Fatalf("expiry changed: %+v %v", own, err)
	}
	for i := 0; i < 23; i++ {
		if err = svc.RecordCall(ctx, APICall{ChannelID: created.AppID, Method: "GET", Path: "catalog.read", Succeeded: i%2 == 0, DurationMillis: 1}); err != nil {
			t.Fatal(err)
		}
	}
	calls, err := svc.OwnCalls(ctx, PageQuery{Page: 3, PageSize: 10})
	if err != nil || calls.Total != 23 || len(calls.List) != 3 {
		t.Fatalf("pagination %+v %v", calls, err)
	}
	if _, err = svc.OwnCalls(ctx, PageQuery{PageSize: 101}); err == nil {
		t.Fatal("unbounded page accepted")
	}
	if _, err = svc.SetChannelStatus(ctx, created.AppID, "pause"); err != nil {
		t.Fatal(err)
	}
	own, err = svc.OwnPlatform(ctx)
	if err != nil || !own.Authorized {
		t.Fatalf("disabled history hidden: %+v %v", own, err)
	}
	if _, err = svc.RotateOwnSecret(ctx, rotated.ID); err == nil {
		t.Fatal("paused authorization rotated")
	}
	calls, err = svc.OwnCalls(ctx, PageQuery{})
	if err != nil || calls.Total != 23 {
		t.Fatalf("disabled logs: %+v %v", calls, err)
	}
}
