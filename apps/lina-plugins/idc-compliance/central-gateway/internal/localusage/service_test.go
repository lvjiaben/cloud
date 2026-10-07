package localusage

import (
	"context"
	"errors"
	"testing"
	"time"
)

type storeStub struct {
	plan  *Plan
	usage []Usage
	calls int
}

func (s *storeStub) LatestPlan(context.Context, string) (*Plan, error) {
	s.calls++
	if s.plan != nil {
		copyOfPlan := *s.plan
		return &copyOfPlan, nil
	}
	return &Plan{ProviderCode: ProviderTencentESSBasic, StatusCode: "active"}, nil
}

func (s *storeStub) ListUsage(
	context.Context,
	string,
	time.Time,
	time.Time,
	int,
) ([]Usage, error) {
	s.calls++
	return s.usage, nil
}

func (s *storeStub) LatestReconciliation(
	context.Context,
	string,
	time.Time,
	time.Time,
) (*Reconciliation, error) {
	s.calls++
	return &Reconciliation{ProviderCode: ProviderTencentESSBasic, CurrencyCode: "CNY"}, nil
}

func (s *storeStub) LatestSLA(
	context.Context,
	string,
	time.Time,
	time.Time,
) (*SLA, error) {
	s.calls++
	return &SLA{ProviderCode: ProviderTencentESSBasic}, nil
}

func TestGetUsesFourBoundedLocalReads(t *testing.T) {
	store := &storeStub{usage: []Usage{{
		ProviderCode: ProviderTencentESSBasic, MetricCode: "contract.completed",
		QuantityNumerator: 1, QuantityDenominator: 1, CurrencyCode: "CNY",
	}}}
	service, err := New(store)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}
	from := time.Date(2026, time.August, 1, 0, 0, 0, 0, time.UTC)
	projection, err := service.Get(context.Background(), Query{
		ProviderCode: ProviderTencentESSBasic, From: from, To: from.Add(24 * time.Hour), Limit: 50,
	})
	if err != nil || store.calls != 4 || len(projection.Usage) != 1 {
		t.Fatalf("projection=%+v calls=%d err=%v", projection, store.calls, err)
	}
}

func TestGetRejectsUnboundedOrCentralProviderQueries(t *testing.T) {
	service, err := New(&storeStub{})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}
	from := time.Date(2026, time.August, 1, 0, 0, 0, 0, time.UTC)
	for _, query := range []Query{
		{ProviderCode: "vendor_central_proxy", From: from, To: from.Add(time.Hour), Limit: 1},
		{ProviderCode: ProviderNuonuoInvoice, From: from, To: from.Add(time.Hour), Limit: MaximumUsageRows + 1},
		{ProviderCode: ProviderNuonuoInvoice, From: from, To: from.Add(401 * 24 * time.Hour), Limit: 1},
	} {
		if _, err = service.Get(context.Background(), query); !errors.Is(err, ErrInvalid) {
			t.Fatalf("query=%+v err=%v", query, err)
		}
	}
}

func TestAdmitNewChargeStopsInsufficientPlanAndKeepsUnmeteredHistoryAvailable(t *testing.T) {
	for _, status := range []string{
		"quota_exhausted", "authorization_missing", "suspended", "revoked", "expired",
	} {
		service, err := New(&storeStub{plan: &Plan{
			ProviderCode: ProviderTencentESSBasic, StatusCode: status,
		}})
		if err != nil {
			t.Fatalf("new service for %s: %v", status, err)
		}
		if err = service.AdmitNewCharge(context.Background(), ProviderTencentESSBasic); !errors.Is(err, ErrChargeUnavailable) {
			t.Fatalf("status=%s admission error=%v", status, err)
		}
	}
	service, err := New(&storeStub{plan: &Plan{
		ProviderCode: ProviderTencentESSBasic, StatusCode: "active",
	}})
	if err != nil || service.AdmitNewCharge(context.Background(), ProviderTencentESSBasic) != nil {
		t.Fatalf("active buyer plan was blocked: %v", err)
	}
}
