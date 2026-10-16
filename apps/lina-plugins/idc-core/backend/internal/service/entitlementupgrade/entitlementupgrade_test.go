// This file verifies the agreed paid-period upgrade basis and exact rounding.

package entitlementupgrade

import (
	"math"
	"testing"
	"time"
)

// TestSameParametersPreservesJSONIntegerPrecision guards JSONB normalization
// without treating distinct large supplier identifiers as the same value.
func TestSameParametersPreservesJSONIntegerPrecision(t *testing.T) {
	for _, tc := range []struct {
		name, left, right string
		want              bool
	}{
		{"jsonb_formatting", `{"cpu":4,"memory":8}`, ` {"memory": 8, "cpu": 4} `, true},
		{"large_integer_equal", `{"id":9007199254740993}`, `{"id": 9007199254740993}`, true},
		{"large_integer_different", `{"id":9007199254740993}`, `{"id":9007199254740992}`, false},
		{"changed_configuration", `{"cpu":4}`, `{"cpu":8}`, false},
		{"invalid_json", `{"cpu":`, `{"cpu":4}`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := sameParameters([]byte(tc.left), []byte(tc.right)); got != tc.want {
				t.Fatalf("sameParameters=%v, want %v", got, tc.want)
			}
		})
	}
}

func TestProrateDifferenceUsesActualPaidPeriod(t *testing.T) {
	start := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	for _, test := range []struct {
		name              string
		actual, target    int64
		elapsed, duration time.Duration
		want              int64
		reject            bool
	}{
		{"agreed_half_period", 8000, 12000, 15 * 24 * time.Hour, 30 * 24 * time.Hour, 2000, false},
		{"discounted_actual_not_current_catalog", 5000, 12000, 15 * 24 * time.Hour, 30 * 24 * time.Hour, 3500, false},
		{"zero_amount_allowed", 8000, 8000, time.Hour, 24 * time.Hour, 0, false},
		{"half_minor_rounds_up", 0, 1, time.Hour, 2 * time.Hour, 1, false},
		{"below_half_minor_rounds_down", 0, 1, 2 * time.Hour, 3 * time.Hour, 0, false},
		{"overflow_safe", 0, math.MaxInt64, time.Hour, 2 * time.Hour, 4611686018427387904, false},
		{"negative_difference", 12000, 8000, time.Hour, 24 * time.Hour, 0, true},
		{"expired", 8000, 12000, 24 * time.Hour, 24 * time.Hour, 0, true},
		{"before_period", 8000, 12000, -time.Hour, 24 * time.Hour, 0, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, err := ProrateDifference(PeriodPrice{ActualMinor: test.actual, TargetMinor: test.target, StartedAt: start, ExpiresAt: start.Add(test.duration), Now: start.Add(test.elapsed)})
			if test.reject {
				if err == nil {
					t.Fatal("invalid paid-period quote accepted")
				}
				return
			}
			if err != nil || got != test.want {
				t.Fatalf("got %d, %v; want %d", got, err, test.want)
			}
		})
	}
}
