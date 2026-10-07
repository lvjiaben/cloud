package serveroperation

import (
	"testing"
	"time"
)

func TestProratedUpgradeAmountRoundsUpAndUsesRemainingCycle(t *testing.T) {
	start := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	end := start.Add(30 * 24 * time.Hour)
	now := start.Add(20 * 24 * time.Hour)
	amount, err := proratedUpgradeAmount(1_001, start, end, now)
	if err != nil || amount != 334 {
		t.Fatalf("amount=%d err=%v", amount, err)
	}
}

func TestResizeDirectionMakesDowngradeDeferred(t *testing.T) {
	direction, err := classifyResize(10_000, 8_000)
	if err != nil || direction != resizeDirectionDowngrade {
		t.Fatalf("direction=%q err=%v", direction, err)
	}
	direction, err = classifyResize(10_000, 10_000)
	if err != nil || direction != resizeDirectionUpgrade {
		t.Fatalf("equal-price direction=%q err=%v", direction, err)
	}
}

func TestDiskShrinkIncludesRemovalQuantityAndCapacity(t *testing.T) {
	current := map[string]diskSelection{
		"system-disk": {Quantity: 1, SizeGiB: 80},
		"data-disk":   {Quantity: 2, SizeGiB: 200},
	}
	for name, target := range map[string]map[string]diskSelection{
		"removed":  {"system-disk": {Quantity: 1, SizeGiB: 80}},
		"quantity": {"system-disk": {Quantity: 1, SizeGiB: 80}, "data-disk": {Quantity: 1, SizeGiB: 200}},
		"capacity": {"system-disk": {Quantity: 1, SizeGiB: 79}, "data-disk": {Quantity: 2, SizeGiB: 200}},
	} {
		if !diskShrinkRequested(current, target) {
			t.Fatalf("%s was not recognized as a disk shrink", name)
		}
	}
	if diskShrinkRequested(current, map[string]diskSelection{
		"system-disk": {Quantity: 1, SizeGiB: 120},
		"data-disk":   {Quantity: 3, SizeGiB: 200},
	}) {
		t.Fatal("disk growth was incorrectly rejected")
	}
}
