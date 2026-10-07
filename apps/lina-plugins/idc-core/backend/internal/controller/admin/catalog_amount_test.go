package admin

import "testing"

func TestCatalogAmountConversionPreservesExactBigIntRange(t *testing.T) {
	t.Parallel()
	for _, raw := range []string{"0", "1000000000000000000", "9223372036854775807"} {
		value, err := parseCatalogAmount(raw)
		if err != nil || formatCatalogAmount(value) != raw {
			t.Fatalf("amount %q round trip value=%d err=%v", raw, value, err)
		}
	}
	if _, err := parseCatalogAmount("9223372036854775808"); err == nil {
		t.Fatal("amount above MaxInt64 was accepted")
	}
	if _, err := parseCatalogAmount("not-a-number"); err == nil {
		t.Fatal("non-decimal amount was accepted")
	}
	if optional, err := parseOptionalCatalogAmount(nil); err != nil || optional != nil {
		t.Fatalf("nil optional amount=%v err=%v", optional, err)
	}
	maximum := int64(9223372036854775807)
	if formatted := formatOptionalCatalogAmount(&maximum); formatted == nil || *formatted != "9223372036854775807" {
		t.Fatalf("optional maximum amount=%v", formatted)
	}
}
