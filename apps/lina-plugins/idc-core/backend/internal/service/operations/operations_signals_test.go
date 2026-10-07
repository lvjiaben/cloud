package operations

import (
	"context"
	"testing"
	"time"
)

func TestValidateSignalCoversHealthKindsAndRejectsUnsafeValues(t *testing.T) {
	for _, kind := range []string{"queue_backlog", "adapter_health", "inventory_freshness", "performance"} {
		if err := validateSignal(Signal{Kind: kind, Code: "provider-a", Status: "warning"}); err != nil {
			t.Fatalf("kind %s rejected: %v", kind, err)
		}
	}
	cases := []Signal{
		{Kind: "unknown", Code: "x", Status: "warning"},
		{Kind: "performance", Code: "x", Status: "bad"},
		{Kind: "performance", Code: "", Status: "ok"},
		{Kind: "performance", Code: "x", Status: "ok", AgeSeconds: -1},
	}
	for _, signal := range cases {
		if err := validateSignal(signal); err == nil {
			t.Fatalf("invalid signal accepted: %+v", signal)
		}
	}
}

func TestValidateSignalDoesNotRequireLocalClock(t *testing.T) {
	signal := Signal{Kind: "inventory_freshness", Code: "catalog-default", Status: "unknown", ObservedAt: time.Time{}}
	if err := validateSignal(signal); err != nil {
		t.Fatal(err)
	}
	if err := (&serviceImpl{}).UpsertSignal(context.Background(), signal); err == nil {
		t.Fatal("nil database must fail closed")
	}
}
