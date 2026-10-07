package billing

import (
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"strings"
	"testing"
)

func TestDirectoryCursorRejectsTamperingAndFilterReuse(t *testing.T) {
	codec, err := newDirectoryCursorCodec([]byte(strings.Repeat("k", 32)))
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256([]byte("account:customer-a:invoice:CNY"))
	encoded, err := codec.encode(directoryCursorBoundary{SortTimeMicroseconds: 1_720_000_000_000_000, ID: 42}, digest)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := codec.decode(encoded, digest)
	if err != nil || decoded.ID != 42 || decoded.SortTimeMicroseconds != 1_720_000_000_000_000 {
		t.Fatalf("decoded=%+v err=%v", decoded, err)
	}
	wrongDigest := sha256.Sum256([]byte("account:customer-b:invoice:CNY"))
	if _, err = codec.decode(encoded, wrongDigest); !errors.Is(err, errInvalidDirectoryCursor) {
		t.Fatalf("cursor crossed an ownership/filter boundary: %v", err)
	}
	payload, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatal(err)
	}
	payload[len(payload)/2] ^= 1
	tampered := base64.RawURLEncoding.EncodeToString(payload)
	if _, err = codec.decode(tampered, digest); err == nil {
		t.Fatal("tampered cursor was accepted")
	}
}
