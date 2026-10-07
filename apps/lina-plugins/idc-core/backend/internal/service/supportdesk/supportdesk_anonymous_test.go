package supportdesk

import (
	"testing"
	"time"
)

func TestAnonymousAccessExpiryIsFixedTwoHours(t *testing.T) {
	signedAt := time.Date(2026, time.August, 15, 1, 2, 3, 456789000, time.UTC)
	got := anonymousAccessExpiry(signedAt)
	want := signedAt.Add(2 * time.Hour)
	if !got.Equal(want) {
		t.Fatalf("expiry=%s want=%s", got, want)
	}
	if anonymousAccessLifetime != 2*time.Hour {
		t.Fatalf("anonymous access lifetime=%s", anonymousAccessLifetime)
	}
}

func TestAnonymousTokenDigestIsDomainSeparatedAndStable(t *testing.T) {
	first := anonymousTokenDigest("fixed-token")
	second := anonymousTokenDigest("fixed-token")
	if first != second || len(first) != 64 {
		t.Fatalf("unexpected digest %q %q", first, second)
	}
	if first == anonymousTokenDigest("fixed-token-2") {
		t.Fatal("different token reused digest")
	}
}
