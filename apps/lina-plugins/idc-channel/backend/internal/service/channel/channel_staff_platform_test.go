// These tests verify real PostgreSQL ownership, rotation concurrency and exact call pagination.
package channel

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestStaffCustomerPlatformPostgreSQL(t *testing.T) {
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
	empty, err := svc.StaffPlatform(ctx, "platform-a")
	if err != nil || empty.Authorized {
		t.Fatalf("unbound: %+v %v", empty, err)
	}
	if _, err = svc.RotateStaffSecret(ctx, "platform-a", 1); err == nil {
		t.Fatal("unbound rotated")
	}
	expires := time.Now().Add(time.Hour).Truncate(time.Millisecond)
	created, err := svc.CreateToken(ctx, TokenInput{CustomerNo: "platform-a", DisplayName: "Platform", Scopes: []string{"catalog.read"}, SourceCIDRs: []string{"127.0.0.0/8"}, ExpiresAt: &expires})
	if err != nil {
		t.Fatal(err)
	}
	own, err := svc.StaffPlatform(ctx, "platform-a")
	if err != nil || !own.Authorized || own.AppID != created.AppID || len(own.Tokens) != 1 {
		t.Fatalf("own: %+v %v", own, err)
	}
	denied := New(db, applicationCore{}, applicationContext{scope: 5})
	if _, err = denied.StaffPlatform(ctx, "platform-a"); err == nil {
		t.Fatal("restricted employee read customer API metadata")
	}
	if _, err = denied.StaffCalls(ctx, "platform-a", PageQuery{}); err == nil {
		t.Fatal("restricted employee read customer API calls")
	}
	if _, err = denied.RotateStaffSecret(ctx, "platform-a", created.ID); err == nil {
		t.Fatal("restricted employee rotated secret")
	}
	other := New(db, platformCore{customer: "platform-b"}, applicationContext{scope: 1})
	empty, err = other.StaffPlatform(ctx, "platform-b")
	if err != nil || empty.Authorized {
		t.Fatalf("cross-customer metadata: %+v %v", empty, err)
	}
	if _, err = other.StaffCalls(ctx, "platform-b", PageQuery{PageSize: 101}); err == nil {
		t.Fatal("cross-customer logs")
	}
	if _, err = other.RotateStaffSecret(ctx, "platform-b", created.ID); err == nil {
		t.Fatal("cross-customer rotation")
	}
	var wg sync.WaitGroup
	results := make(chan TokenResult, 2)
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			result, err := svc.RotateStaffSecret(ctx, "platform-a", created.ID)
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
	own, err = svc.StaffPlatform(ctx, "platform-a")
	if err != nil || !own.Tokens[0].ExpiresAt.Equal(expires) {
		t.Fatalf("expiry changed: %+v %v", own, err)
	}
	otherToken, err := svc.CreateToken(ctx, TokenInput{CustomerNo: "platform-b", DisplayName: "Other", Scopes: []string{"catalog.read"}})
	if err != nil {
		t.Fatal(err)
	}
	if err = svc.RecordCall(ctx, APICall{ChannelID: otherToken.AppID, Method: "GET", Path: "catalog.read", Succeeded: true}); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 23; i++ {
		if err = svc.RecordCall(ctx, APICall{ChannelID: created.AppID, Method: "GET", Path: "catalog.read", Succeeded: i%2 == 0, DurationMillis: 1}); err != nil {
			t.Fatal(err)
		}
	}
	calls, err := svc.StaffCalls(ctx, "platform-a", PageQuery{Page: 3, PageSize: 10})
	if err != nil || calls.Total != 23 || len(calls.List) != 3 {
		t.Fatalf("pagination %+v %v", calls, err)
	}
	if _, err = svc.StaffCalls(ctx, "platform-a", PageQuery{PageSize: 101}); err == nil {
		t.Fatal("unbounded page accepted")
	}
	if _, err = svc.SetChannelStatus(ctx, created.AppID, "pause"); err != nil {
		t.Fatal(err)
	}
	own, err = svc.StaffPlatform(ctx, "platform-a")
	if err != nil || !own.Authorized {
		t.Fatalf("disabled history hidden: %+v %v", own, err)
	}
	if _, err = svc.RotateStaffSecret(ctx, "platform-a", rotated.ID); err == nil {
		t.Fatal("paused authorization rotated")
	}
	calls, err = svc.StaffCalls(ctx, "platform-a", PageQuery{})
	if err != nil || calls.Total != 23 {
		t.Fatalf("disabled logs: %+v %v", calls, err)
	}
}
