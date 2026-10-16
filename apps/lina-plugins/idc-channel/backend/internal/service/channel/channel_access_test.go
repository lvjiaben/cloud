// These tests exercise customer credentials against PostgreSQL rather than
// accepting a mocked authentication result or testing only SQL text.
package channel

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"lina-core/pkg/plugin/capability/bizctxcap"
	"lina-plugin-idc-core/backend/cap/channelcommercecap"
)

type applicationCore struct{ channelcommercecap.Runtime }

func (applicationCore) ValidatePayer(_ context.Context, customer string) (*channelcommercecap.CustomerSubject, error) {
	return &channelcommercecap.CustomerSubject{CustomerNo: customer, Status: "active", IdentityStatus: "verified"}, nil
}

type applicationContext struct {
	bizctxcap.Service
	scope int
}

func (context applicationContext) Current(_ context.Context) bizctxcap.CurrentContext {
	return bizctxcap.CurrentContext{UserID: 1, DataScope: context.scope}
}

func TestApplicationCredentialsPostgreSQL(t *testing.T) {
	database := channelPageDatabase(t)
	ctx := context.Background()
	for _, name := range []string{"001-channel-core.sql", "002-channel-webhook-delivery.sql", "003-channel-token-security.sql", "004-channel-commerce-management.sql", "005-channel-list-pagination.sql"} {
		content, err := os.ReadFile(filepath.Join("../../../../manifest/sql", name))
		if err != nil {
			t.Fatal(err)
		}
		if _, err = database.Exec(ctx, string(content)); err != nil {
			t.Fatal(err)
		}
	}
	service := New(database, applicationCore{}, applicationContext{scope: 1})
	created, err := service.CreateToken(ctx, TokenInput{CustomerNo: "customer-a", DisplayName: "Test application", Scopes: []string{"catalog.read"}})
	if err != nil || created.AppID == "" || len(created.Token) != 64 {
		t.Fatalf("create application: %+v %v", created, err)
	}
	credential, err := service.AuthenticateToken(ctx, created.Token, "127.0.0.1", "catalog.read")
	if err != nil || credential.ChannelID != created.AppID {
		t.Fatalf("authenticate: %+v %v", credential, err)
	}
	if _, err = service.AuthenticateToken(ctx, created.Token, "127.0.0.1", "order.create"); err == nil {
		t.Fatal("ungranted operation accepted")
	}
	stored, err := database.GetValue(ctx, `SELECT token_hash FROM plugin_idc_channel_token WHERE id=?`, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.String() == created.Token {
		t.Fatal("secret was stored in plaintext")
	}
	rotated, err := service.CreateToken(ctx, TokenInput{CustomerNo: "customer-a", DisplayName: "Test application", Scopes: []string{"catalog.read", "order.create"}})
	if err != nil || rotated.AppID != created.AppID || rotated.Token == created.Token {
		t.Fatalf("rotate: %+v %v", rotated, err)
	}
	if _, err = service.AuthenticateToken(ctx, created.Token, "127.0.0.1", "catalog.read"); err == nil {
		t.Fatal("rotated secret accepted")
	}
	if _, err = service.SetChannelStatus(ctx, created.AppID, "pause"); err != nil {
		t.Fatal(err)
	}
	if _, err = service.AuthenticateToken(ctx, rotated.Token, "127.0.0.1", "catalog.read"); err == nil {
		t.Fatal("paused application accepted")
	}
	if _, err = service.SetChannelStatus(ctx, created.AppID, "resume"); err != nil {
		t.Fatal(err)
	}
	if err = service.RevokeToken(ctx, rotated.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = service.AuthenticateToken(ctx, rotated.Token, "127.0.0.1", "catalog.read"); err == nil {
		t.Fatal("revoked secret accepted")
	}
	expired := time.Now().Add(-time.Minute)
	if _, err = service.CreateToken(ctx, TokenInput{CustomerNo: "customer-b", DisplayName: "Rejected", Scopes: []string{"catalog.read"}, ExpiresAt: &expired}); err == nil {
		t.Fatal("expired credential accepted")
	}
	count, err := database.GetValue(ctx, `SELECT COUNT(*) FROM plugin_idc_channel_account WHERE customer_no='customer-b'`)
	if err != nil || count.Int() != 0 {
		t.Fatalf("failed create left application: %d %v", count.Int(), err)
	}
	narrow := New(database, applicationCore{}, applicationContext{scope: 5})
	if _, err = narrow.CreateToken(ctx, TokenInput{CustomerNo: "customer-c", DisplayName: "Forbidden", Scopes: []string{"catalog.read"}}); err == nil {
		t.Fatal("narrow data scope granted application")
	}
}

func TestDirectPeerIPIgnoresForwardingHeaders(t *testing.T) {
	if directPeerIP("127.0.0.1:6030") != "127.0.0.1" || directPeerIP("[::1]:443") != "::1" {
		t.Fatal("peer parsing failed")
	}
}
