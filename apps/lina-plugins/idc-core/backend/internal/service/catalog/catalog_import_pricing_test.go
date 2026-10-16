// This file verifies exact imported draft pricing, final-only rounding and
// invalid-input rejection. Cases are pure and independent of database state.

package catalog

import (
	"math"
	"testing"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/channelcommercecap"
)

// TestImportMarkupPriceMinor verifies that a markup increases the upstream
// amount and that precision and rounding apply only to the final result.
func TestImportMarkupPriceMinor(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name      string
		upstream  string
		markup    string
		minorUnit int
		want      int64
	}{
		{name: "ten percent is 110 percent", upstream: "100", markup: "10", minorUnit: 2, want: 11000},
		{name: "zero percent", upstream: "100.01", markup: "0", minorUnit: 2, want: 10001},
		{name: "half cent rounds up", upstream: "0.05", markup: "10", minorUnit: 2, want: 6},
		{name: "below half cent", upstream: "0.05", markup: "9.999999999999999999", minorUnit: 2, want: 5},
		{name: "no intermediate rounding", upstream: "0.004", markup: "100", minorUnit: 2, want: 1},
		{name: "zero precision", upstream: "5", markup: "10", minorUnit: 0, want: 6},
		{name: "three digit precision", upstream: "1.005", markup: "10", minorUnit: 3, want: 1106},
		{name: "decimal markup", upstream: "10", markup: "12.345", minorUnit: 2, want: 1123},
		{name: "decimal leading zero", upstream: "010.00", markup: "010", minorUnit: 2, want: 1100},
		{name: "maximum int64 unchanged", upstream: "92233720368547758.07", markup: "0", minorUnit: 2, want: math.MaxInt64},
		{name: "markup reaches maximum int64 exactly", upstream: "46116860184273879.035", markup: "100", minorUnit: 2, want: math.MaxInt64},
		{name: "large intermediate multiplication", upstream: "8384883669867978000", markup: "10", minorUnit: 0, want: 9223372036854775800},
		{name: "no arbitrary markup maximum", upstream: "0.00000000000000000001", markup: "10000000000000000000000", minorUnit: 2, want: 100},
		{name: "zero amount", upstream: "0", markup: "999999999999999999999999999999999999999", minorUnit: 3, want: 0},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			got, err := importMarkupPriceMinor(test.upstream, test.markup, test.minorUnit)
			if err != nil || got != test.want {
				t.Fatalf("price=%d err=%v, want %d", got, err, test.want)
			}
		})
	}
}

// TestImportFinalPriceMinor verifies explicit sale prices without any upstream
// dependency, including exact int64 boundaries and supported currency precision.
func TestImportFinalPriceMinor(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		amount    string
		minorUnit int
		want      int64
	}{
		{amount: "42.42", minorUnit: 2, want: 4242},
		{amount: "0", minorUnit: 2, want: 0},
		{amount: "1.004999999999999999999", minorUnit: 2, want: 100},
		{amount: "1.005", minorUnit: 2, want: 101},
		{amount: "1.5", minorUnit: 0, want: 2},
		{amount: "1.2345", minorUnit: 3, want: 1235},
		{amount: "9223372036854775807", minorUnit: 0, want: math.MaxInt64},
		{amount: "92233720368547758.074999999999999999999", minorUnit: 2, want: math.MaxInt64},
		{amount: "0.0000000000000000001", minorUnit: 19, want: 1},
	} {
		t.Run(test.amount, func(t *testing.T) {
			t.Parallel()
			got, err := importFinalPriceMinor(test.amount, test.minorUnit)
			if err != nil || got != test.want {
				t.Fatalf("price=%d err=%v, want %d", got, err, test.want)
			}
		})
	}
}

// TestImportPricingRejectsInvalidDecimals verifies the same strict grammar for
// upstream amounts, markup percentages and manual final prices.
func TestImportPricingRejectsInvalidDecimals(t *testing.T) {
	t.Parallel()
	for _, raw := range []string{
		"", "-1", "-0", "+1", "NaN", "Inf", "Infinity", "1e3", "1E3", "1p2",
		"0x10", "0b10", "1/2", "1_000", "1,000", ".5", "1.", "1..2", ".", " 1", "1 ", "1\n", "１", "1%",
	} {
		t.Run(raw, func(t *testing.T) {
			t.Parallel()
			if got, err := importFinalPriceMinor(raw, 2); got != 0 || !bizerr.Is(err, CodeManagementInvalid) {
				t.Fatalf("final invalid input returned %d, %v", got, err)
			}
			if got, err := importMarkupPriceMinor(raw, "10", 2); got != 0 || !bizerr.Is(err, CodeManagementInvalid) {
				t.Fatalf("upstream invalid input returned %d, %v", got, err)
			}
			if got, err := importMarkupPriceMinor("100", raw, 2); got != 0 || !bizerr.Is(err, CodeManagementInvalid) {
				t.Fatalf("markup invalid input returned %d, %v", got, err)
			}
		})
	}
}

// TestImportPricingRejectsOverflow verifies failure before narrowing, including
// rounding across MaxInt64 and excessive scaling that must not allocate a huge power.
func TestImportPricingRejectsOverflow(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		amount    string
		markup    string
		minorUnit int
	}{
		{amount: "9223372036854775808", markup: "0", minorUnit: 0},
		{amount: "92233720368547758.075", markup: "0", minorUnit: 2},
		{amount: "99999999999999999999999999999999999", markup: "0", minorUnit: 2},
		{amount: "8384883669867978007", markup: "10", minorUnit: 0},
		{amount: "46116860184273879.0375", markup: "100", minorUnit: 2},
		{amount: "1", markup: "10000000000000000000000", minorUnit: 2},
		{amount: "0", markup: "0", minorUnit: -1},
		{amount: "1", markup: "0", minorUnit: math.MaxInt},
	} {
		t.Run(test.amount+"/"+test.markup, func(t *testing.T) {
			t.Parallel()
			if got, err := importMarkupPriceMinor(test.amount, test.markup, test.minorUnit); got != 0 || !bizerr.Is(err, CodeManagementInvalid) {
				t.Fatalf("invalid marked up price returned %d, %v", got, err)
			}
			if test.markup == "0" {
				if got, err := importFinalPriceMinor(test.amount, test.minorUnit); got != 0 || !bizerr.Is(err, CodeManagementInvalid) {
					t.Fatalf("invalid final price returned %d, %v", got, err)
				}
			}
		})
	}
}

// TestPortableImportResaleMarkup applies markup to every fixed source while
// retaining downstream discounts and leaving the received snapshot untouched.
func TestPortableImportResaleMarkup(t *testing.T) {
	snapshot := portableImportTestSource().ImportSnapshot
	bps := 2000
	snapshot.Prices[0].WholeItemRule = &channelcommercecap.ImportWholeItemRule{Kind: "discount", DiscountBasisPoints: &bps, Prior: snapshot.Prices[0].WholeItemRule}
	prices, err := calculatePortableImportPrices(snapshot, "10")
	if err != nil {
		t.Fatal(err)
	}
	rule := prices[0].portable.WholeItemRule
	if *rule.Prior.FixedAmountMinor != 220 || *rule.DiscountBasisPoints != 2000 || *snapshot.Prices[0].WholeItemRule.Prior.FixedAmountMinor != 200 {
		t.Fatalf("resale markup or immutability: %+v", rule)
	}
}
