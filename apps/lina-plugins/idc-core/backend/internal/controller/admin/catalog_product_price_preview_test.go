// This file verifies strict preview input shapes, bounds, and exclusion of file content.
package admin

import (
	"strings"
	"testing"
)

func TestDecodePricePreviewValuesRejectsUnsafeOrUnboundedContent(t *testing.T) {
	for _, content := range []string{"", `null`, `[{"fieldKey":"cpu","unknown":1}]`, `[] {}`, `[null]`, `[{"fieldKey":"file","fileTokens":["secret"]}]`, strings.Repeat(" ", 32769)} {
		if _, err := decodePricePreviewValues(content); err == nil {
			t.Errorf("accepted invalid preview input of %d bytes", len(content))
		}
	}
	values, err := decodePricePreviewValues(`[{"fieldKey":"cpu","integerValue":"4"}]`)
	if err != nil || len(values) != 1 || values[0].IntegerValue == nil || *values[0].IntegerValue != "4" {
		t.Fatalf("valid values=%+v err=%v", values, err)
	}
}

// Explicit zero is a known price, while an unavailable amount must serialize null.
func TestProductPricePreviewAmountAvailability(t *testing.T) {
	if previewAmountPointer("") != nil {
		t.Fatal("unavailable amount is not null")
	}
	zero := previewAmountPointer("0")
	if zero == nil || *zero != "0" {
		t.Fatal("explicit zero price was lost")
	}
}
