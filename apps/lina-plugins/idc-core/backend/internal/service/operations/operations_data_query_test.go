// This file verifies snapshot and merchant-calendar drill-down boundaries,
// rejecting mismatched filters before they can change a metric's meaning.

package operations

import (
	"context"
	"errors"
	"lina-core/pkg/plugin/capability/authcap/authz"
	"lina-core/pkg/plugin/capability/bizctxcap"
	"testing"
	"time"
)

// operationsAuthorizationFixture makes the host authority independent from the
// optional UI-oriented business-context permission projection.
type operationsAuthorizationFixture struct {
	authz.Service
	allowed bool
	err     error
	keys    *[]authz.PermissionKey
}

// HasPermission captures the exact requested key and returns the host verdict.
func (f operationsAuthorizationFixture) HasPermission(_ context.Context, key authz.PermissionKey) (bool, error) {
	if f.keys != nil {
		*f.keys = append(*f.keys, key)
	}
	return f.allowed, f.err
}

// TestDataRecordsUsesHostAuthorization rejects snapshot-based fake grants and
// accepts real host grants even when Current has no permission fields.
func TestDataRecordsUsesHostAuthorization(t *testing.T) {
	for _, tc := range []struct {
		name     string
		snapshot bizctxcap.CurrentContext
		allowed  bool
		err      error
	}{
		{"ordinary admin host grant", bizctxcap.CurrentContext{UserID: 1, DataScope: 2}, true, nil},
		{"snapshot cannot override deny", bizctxcap.CurrentContext{IsSuperAdmin: true, Permissions: []string{"*:*:*"}}, false, nil},
		{"host failure fails closed", bizctxcap.CurrentContext{IsSuperAdmin: true}, false, errors.New("authority unavailable")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var keys []authz.PermissionKey
			svc := &serviceImpl{bizCtx: operationsBizCtxStub{current: tc.snapshot}, authorization: operationsAuthorizationFixture{allowed: tc.allowed, err: tc.err, keys: &keys}}
			allowed, err := svc.canReadRecords(context.Background(), MetricTickets)
			if allowed != tc.allowed || !errors.Is(err, tc.err) || len(keys) != 1 || keys[0] != "idc:admin:support:ticket:view-all" {
				t.Fatalf("allowed=%v err=%v keys=%v", allowed, err, keys)
			}
		})
	}
}

// TestDataWindowUsesSnapshotAndMerchantCalendar covers date defaults and DST.
func TestDataWindowUsesSnapshotAndMerchantCalendar(t *testing.T) {
	location, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatal(err)
	}
	asOf := time.Date(2026, 3, 6, 12, 0, 0, 0, location)
	now := asOf.Add(time.Hour)
	for _, tc := range []struct {
		name       string
		query      DataQuery
		start, end int64
	}{
		{"today", DataQuery{Metric: MetricCustomers, Status: "today"}, time.Date(2026, 3, 6, 0, 0, 0, 0, location).UnixMilli(), asOf.UnixMilli()},
		{"online cumulative", DataQuery{Metric: MetricCustomers, Status: "online"}, 0, asOf.UnixMilli()},
		{"cumulative", DataQuery{Metric: MetricCustomers}, 0, asOf.UnixMilli()},
		{"servers calendar week", DataQuery{Metric: MetricServers}, asOf.UnixMilli(), asOf.AddDate(0, 0, 7).UnixMilli()},
		{"order snapshot", DataQuery{Metric: MetricOrders, CurrencyCode: "CNY"}, 0, asOf.UnixMilli()},
		{"clip chart today", DataQuery{Metric: MetricIncome, CurrencyCode: "CNY", StartAt: asOf.Add(-time.Hour).UnixMilli(), EndAt: now.Add(time.Hour).UnixMilli()}, asOf.Add(-time.Hour).UnixMilli(), asOf.UnixMilli()},
		{"future chart empty", DataQuery{Metric: MetricOrders, CurrencyCode: "CNY", StartAt: now.UnixMilli(), EndAt: now.Add(time.Hour).UnixMilli()}, asOf.UnixMilli(), asOf.UnixMilli()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			q := tc.query
			q.Page, q.PageSize, q.AsOf = 1, 20, asOf.UnixMilli()
			actual, err := normalizeDataWindow(q, now, location)
			if err != nil || actual.StartAt != tc.start || actual.EndAt != tc.end {
				t.Fatalf("query=%+v err=%v expected [%d,%d)", actual, err, tc.start, tc.end)
			}
		})
	}
}

// TestDataWindowRejectsUntrustedFilters ensures no cross-metric status can
// silently produce a broader record list than the requested dashboard value.
func TestDataWindowRejectsUntrustedFilters(t *testing.T) {
	now := time.Now()
	for _, q := range []DataQuery{
		{Metric: "arbitrary"},
		{Metric: MetricIncome, CurrencyCode: "CNY", Status: "online"},
		{Metric: MetricServers, Status: "today"},
		{Metric: MetricCustomers, Status: "pending"},
		{Metric: MetricOrders, CurrencyCode: "CNY", ProductNo: "P1"},
		{Metric: MetricCustomers, AsOf: now.Add(time.Hour).UnixMilli()},
		{Metric: MetricTickets, StartAt: 200, EndAt: 100},
		{Metric: MetricOrders, CurrencyCode: "invalid"},
	} {
		q.Page, q.PageSize = 1, 20
		if _, err := normalizeDataWindow(q, now, time.UTC); err == nil {
			t.Fatalf("accepted invalid query %+v", q)
		}
	}
}

// TestCalendarTrendDoesNotRollFebruaryIntoMarch checks both leap and ordinary
// years and retains absent comparisons as null, never fabricated zeros.
func TestCalendarTrendDoesNotRollFebruaryIntoMarch(t *testing.T) {
	for _, year := range []int{2024, 2026} {
		items := calendarTrend(time.Date(year, 3, 15, 12, 0, 0, 0, time.UTC))
		if len(items) != 31 {
			t.Fatalf("year %d has %d March days", year, len(items))
		}
		lastFebruaryDay := 28
		if year == 2024 {
			lastFebruaryDay = 29
		}
		for i, item := range items {
			if (item.PreviousCount != nil) != (i < lastFebruaryDay) {
				t.Fatalf("year %d day %d has invalid comparison %+v", year, i+1, item)
			}
		}
	}
}
