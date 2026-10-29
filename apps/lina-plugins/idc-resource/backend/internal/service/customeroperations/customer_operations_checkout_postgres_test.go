package customeroperations

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"lina-plugin-idc-core/backend/cap/servercommercialcap"
	"lina-plugin-idc-core/backend/cap/serveroperationresourcecap"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
	"lina-plugin-idc-resource/backend/internal/service/accessorydirectory"
)

type checkoutOwner struct {
	servercommercialcap.Service
	db     gdb.DB
	amount int64
}

func (o *checkoutOwner) ReadPurchase(ctx context.Context, input servercommercialcap.PurchaseReadRequest) (*servercommercialcap.Purchase, error) {
	exists, err := o.db.GetValue(ctx, `SELECT EXISTS(SELECT 1 FROM test_native_checkout WHERE request_key=? AND quote_no=?)`, input.IdempotencyKey, input.QuoteNo)
	if err != nil || !exists.Bool() {
		return nil, err
	}
	return &servercommercialcap.Purchase{OperationNo: "commercial-checkout", QuoteNo: input.QuoteNo, Reused: true, TotalAmountMinor: o.amount, CurrencyCode: "CNY"}, nil
}

func (o *checkoutOwner) ReadCheckout(_ context.Context, input servercommercialcap.CheckoutRequest) (*servercommercialcap.CheckoutIntent, error) {
	return &servercommercialcap.CheckoutIntent{QuoteNo: input.QuoteNo, EntitlementNo: input.Target.EntitlementNo, LocalResourceNo: input.Target.LocalResourceNo, CurrencyCode: "CNY", Action: serveroperationresourcecap.ActionChangeIP, TotalAmountMinor: o.amount, ExpiresAt: time.Now().Add(time.Minute)}, nil
}

func (o *checkoutOwner) Checkout(ctx context.Context, input servercommercialcap.CheckoutRequest) (*servercommercialcap.Purchase, error) {
	if gdb.TXFromCtx(ctx, o.db.GetGroup()) == nil {
		return nil, ErrUnavailable
	}
	existing, err := o.db.GetValue(ctx, `SELECT EXISTS(SELECT 1 FROM test_native_checkout WHERE request_key=?)`, input.IdempotencyKey)
	if err != nil {
		return nil, err
	}
	if !existing.Bool() {
		if _, err = o.db.Exec(ctx, `INSERT INTO test_native_checkout(request_key,quote_no)VALUES(?,?)`, input.IdempotencyKey, input.QuoteNo); err != nil {
			return nil, err
		}
	}
	return &servercommercialcap.Purchase{OperationNo: "commercial-checkout", QuoteNo: input.QuoteNo, Reused: existing.Bool(), TotalAmountMinor: o.amount, CurrencyCode: "CNY"}, nil
}

func TestNativeCommercialCheckoutAndProofShareTransactionPostgreSQL(t *testing.T) {
	f := newManagementFixture(t)
	if _, err := f.db.Exec(f.ctx, `CREATE TABLE test_native_checkout(request_key TEXT PRIMARY KEY,quote_no TEXT NOT NULL)`); err != nil {
		t.Fatal(err)
	}
	directory, err := accessorydirectory.New(f.db)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	if _, err = directory.Publish(f.ctx, accessorydirectory.PublishInput{MappingNo: "mapping-1", LocalResourceNo: "SRV_" + strings.Repeat("1", 32), ChannelNo: "channel-main", ExternalResourceRef: "external-1", Snapshot: resourcecap.AccessorySnapshot{ObservedAt: now, ExpiresAt: now.Add(time.Minute), Items: []resourcecap.AccessoryItem{}}}); err != nil {
		t.Fatal(err)
	}
	owner := &checkoutOwner{db: f.db, amount: 100}
	service, err := NewCommercial(f.service, owner)
	if err != nil {
		t.Fatal(err)
	}
	command := CommercialCheckoutCommand{ServerNo: f.serverNo, QuoteNo: "owned-ip-quote", IdempotencyKey: "checkout-1", Confirmed: true}
	proof, err := service.PrepareCheckout(f.ctx, command)
	if err != nil || !proof.Required {
		t.Fatal("high-risk frozen action omitted proof", err)
	}
	if _, err = service.Checkout(f.ctx, command); err == nil {
		t.Fatal("missing proof committed checkout")
	}
	count, err := f.db.GetValue(f.ctx, `SELECT COUNT(*) FROM test_native_checkout`)
	if err != nil || count.Int() != 0 {
		t.Fatal("checkout did not roll back with proof", err)
	}
	command.ProofToken = proof.Token
	owner.amount = 200
	if _, err = service.Checkout(f.ctx, command); err == nil {
		t.Fatal("confirmation accepted changed frozen amount")
	}
	owner.amount = 100
	first, err := service.Checkout(f.ctx, command)
	if err != nil || first.Reused {
		t.Fatal("exact confirmed checkout failed", err)
	}
	command.ProofToken = ""
	before := f.provider.calls
	f.provider.delay = time.Minute
	replay, err := service.Checkout(f.ctx, command)
	if err != nil || !replay.Reused || replay.OperationNo != first.OperationNo || f.provider.calls != before {
		t.Fatal("checkout replay consumed proof twice", err)
	}
	count, err = f.db.GetValue(f.ctx, `SELECT COUNT(*) FROM test_native_checkout`)
	if err != nil || count.Int() != 1 {
		t.Fatal("duplicate checkout", err)
	}
}
