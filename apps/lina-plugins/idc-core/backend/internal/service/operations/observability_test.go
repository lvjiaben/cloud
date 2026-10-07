package operations

import (
	"context"
	"testing"
	"time"
)

func TestSummarizeTimingGroupsSlowSegments(t *testing.T) {
	now := time.Date(2026, 8, 30, 0, 0, 0, 0, time.UTC)
	segments := []TimingSegment{
		{Name: "provider.call", Kind: "provider", StartedAt: now.Add(2 * time.Second), EndedAt: now.Add(2*time.Second + 1200*time.Millisecond)},
		{Name: "queue.wait", Kind: "queue", StartedAt: now, EndedAt: now.Add(700 * time.Millisecond)},
		{Name: "db.read", Kind: "database", StartedAt: now.Add(4 * time.Second), EndedAt: now.Add(4*time.Second + 100*time.Millisecond)},
	}
	summary, err := SummarizeTiming(context.Background(), segments, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if summary.TotalMs != 2000 || summary.QueueMs != 700 || summary.ProviderMs != 1200 || summary.DatabaseMs != 100 {
		t.Fatalf("unexpected summary: %+v", summary)
	}
	if len(summary.SlowSegments) != 1 || summary.SlowSegments[0].Name != "provider.call" {
		t.Fatalf("unexpected slow segments: %+v", summary.SlowSegments)
	}
}

func TestSummarizeTimingRejectsInvalidSegmentsAndCancelledContext(t *testing.T) {
	now := time.Now().UTC()
	if _, err := SummarizeTiming(context.Background(), []TimingSegment{{Name: "bad", Kind: "provider", StartedAt: now, EndedAt: now.Add(-time.Second)}}, time.Second); err == nil {
		t.Fatal("expected reversed timing segment to fail")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := SummarizeTiming(ctx, []TimingSegment{{Name: "ok", Kind: "local", StartedAt: now, EndedAt: now.Add(time.Millisecond)}}, time.Second); err == nil {
		t.Fatal("expected cancelled context to fail")
	}
}

func TestClassifySignalUsesKindSpecificMetricAndBoundaries(t *testing.T) {
	cases := []struct {
		kind, want      string
		count, age, val int64
	}{
		{"queue_backlog", "warning", 10, 0, 0},
		{"adapter_health", "critical", 100, 0, 0},
		{"inventory_freshness", "warning", 0, 65, 0},
		{"performance", "critical", 0, 0, 900},
		{"performance", "ok", 0, 0, 1},
	}
	for _, tc := range cases {
		got, err := ClassifySignal(tc.kind, tc.count, tc.age, tc.val, 10, 100)
		if err != nil || got != tc.want {
			t.Fatalf("%s: got %q err=%v, want %q", tc.kind, got, err, tc.want)
		}
	}
	if _, err := ClassifySignal("performance", 0, 0, 1, 100, 100); err == nil {
		t.Fatal("expected non-increasing thresholds to fail")
	}
}
