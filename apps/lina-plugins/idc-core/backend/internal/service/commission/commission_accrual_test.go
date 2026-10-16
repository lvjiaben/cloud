// This file checks fixed-per-invoice rounding and cumulative refund arithmetic.
package commission

import (
	"math"
	"reflect"
	"testing"
)

func TestCommissionAllocationAndCumulativeRefund(t *testing.T) {
	portions, err := allocateProportionally(1001, []int64{6000, 4000})
	if err != nil || !reflect.DeepEqual(portions, []int64{601, 400}) {
		t.Fatalf("fixed invoice commission: %v %v", portions, err)
	}
	amount, err := proportionalReversal(1000, 2500, 10000, 0)
	if err != nil || amount != 250 {
		t.Fatalf("first refund=%d %v", amount, err)
	}
	amount, err = proportionalReversal(1000, 5000, 10000, 250)
	if err != nil || amount != 250 {
		t.Fatalf("second refund=%d %v", amount, err)
	}
	amount, err = proportionalReversal(math.MaxInt64, math.MaxInt64, math.MaxInt64, math.MaxInt64-1)
	if err != nil || amount != 1 {
		t.Fatalf("overflow-safe full refund=%d %v", amount, err)
	}
}
