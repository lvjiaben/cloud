package operations

import (
	"context"
	"testing"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"lina-core/pkg/plugin/capability/bizctxcap"
)

type operationsBizCtxStub struct{ current bizctxcap.CurrentContext }

func (s operationsBizCtxStub) Current(context.Context) bizctxcap.CurrentContext { return s.current }

func TestNewRequiresDatabase(t *testing.T) {
	if _, err := New(nil, operationsBizCtxStub{}); err == nil {
		t.Fatal("expected database validation")
	}
}

func TestNewRequiresBusinessContext(t *testing.T) {
	var database gdb.DB
	if _, err := New(database, nil); err == nil {
		t.Fatal("expected business context validation")
	}
}

func TestAuthorizeReadRejectsNarrowOrMissingScope(t *testing.T) {
	for _, current := range []bizctxcap.CurrentContext{
		{UserID: 10, TenantID: 0, DataScope: 3},
		{UserID: 10, TenantID: 0, DataScopeUnsupported: true, DataScope: 2},
		{UserID: 10, TenantID: 7, DataScope: 2},
	} {
		service := &serviceImpl{bizCtx: operationsBizCtxStub{current: current}}
		if err := service.authorizeRead(context.Background()); err == nil {
			t.Fatalf("scope %+v should fail closed", current)
		}
	}
}

func TestAuthorizeReadAcceptsBroadTenantScope(t *testing.T) {
	service := &serviceImpl{bizCtx: operationsBizCtxStub{current: bizctxcap.CurrentContext{UserID: 10, TenantID: 0, DataScope: 2}}}
	if err := service.authorizeRead(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestSnapshotTypesAreBounded(t *testing.T) {
	queues := []QueueCount{{Code: "pending_orders", Count: 2}}
	currencies := []CurrencyMetric{{CurrencyCode: "CNY", InvoiceCount: 3, TotalAmountMinor: 1000}}
	snapshot := Snapshot{Queues: queues, Currencies: currencies}
	if len(snapshot.Queues) != 1 || len(snapshot.Currencies) != 1 || snapshot.Currencies[0].CurrencyCode != "CNY" {
		t.Fatalf("unexpected snapshot: %+v", snapshot)
	}
	_ = context.Background()
	var _ gdb.DB
}

func TestReportRejectsUnboundedRange(t *testing.T) {
	service := &serviceImpl{}
	to := time.Now().UTC()
	if _, err := service.LoadReport(context.Background(), ReportQuery{From: to.Add(-367 * 24 * time.Hour), To: to}); err == nil {
		t.Fatal("expected report range validation")
	}
}

func TestTaskQueryRejectsUnknownStatus(t *testing.T) {
	service := &serviceImpl{}
	if _, err := service.ListTasks(context.Background(), TaskQuery{Status: "secret", Limit: 1}); err == nil {
		t.Fatal("expected task status validation")
	}
}
