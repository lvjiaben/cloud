package serveroperation

import (
	"strings"
	"testing"
)

func TestTransferEmailNormalizationAndValidation(t *testing.T) {
	t.Parallel()
	if got := normalizeTransferEmail("  Alice@Example.COM "); got != "alice@example.com" {
		t.Fatalf("normalized email=%q", got)
	}
	for _, value := range []string{"alice@example.com", "a@b.test"} {
		if !validTransferEmail(value) {
			t.Fatalf("valid email rejected: %q", value)
		}
	}
	for _, value := range []string{"", "missing-at", "a@@b.test", "a @b.test", strings.Repeat("a", 321)} {
		if validTransferEmail(value) {
			t.Fatalf("invalid email accepted: %q", value)
		}
	}
}

func TestTransferIdentifiersAndDigestsAreStableAndOpaque(t *testing.T) {
	t.Parallel()
	first, err := newTransferNumber("TRF")
	if err != nil {
		t.Fatal(err)
	}
	second, err := newTransferNumber("TRF")
	if err != nil {
		t.Fatal(err)
	}
	if first == second || !validReference(first, 40) || !validReference(second, 40) {
		t.Fatalf("unsafe transfer numbers: %q %q", first, second)
	}
	if transferDigest("Alice@example.com") != transferDigest("Alice@example.com") ||
		transferDigest("Alice@example.com") == transferDigest("alice@example.com") {
		t.Fatal("transfer digest is not deterministic and input-sensitive")
	}
}
