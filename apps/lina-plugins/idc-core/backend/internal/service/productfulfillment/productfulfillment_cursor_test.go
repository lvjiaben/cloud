package productfulfillment

import "testing"

func TestFulfillmentCursorRejectsTamperingAndCrossFilterReuse(t *testing.T) {
	codec, err := newFulfillmentCursorCodec([]byte("0123456789abcdef0123456789abcdef"))
	if err != nil {
		t.Fatal(err)
	}
	filter := ListInput{OrderNo: "order-1", SafeStatus: "processing", Limit: 20}
	digest := fulfillmentCursorDigest(filter, "scope-a")
	encoded, err := codec.encode("intent-1", digest)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := codec.decode(encoded, digest)
	if err != nil || decoded != "intent-1" {
		t.Fatalf("decode=%q err=%v", decoded, err)
	}

	tampered := encoded[:len(encoded)-1] + "A"
	if tampered == encoded {
		tampered = encoded[:len(encoded)-1] + "B"
	}
	if _, err = codec.decode(tampered, digest); err == nil {
		t.Fatal("tampered cursor must be rejected")
	}
	if _, err = codec.decode(encoded, fulfillmentCursorDigest(ListInput{OrderNo: "order-2", SafeStatus: "processing", Limit: 20}, "scope-a")); err == nil {
		t.Fatal("cursor reused with another filter must be rejected")
	}
	if _, err = codec.decode(encoded, fulfillmentCursorDigest(filter, "scope-b")); err == nil {
		t.Fatal("cursor reused with another data scope must be rejected")
	}
}

func TestFulfillmentCursorRejectsNonCanonicalEncoding(t *testing.T) {
	codec, err := newFulfillmentCursorCodec([]byte("0123456789abcdef0123456789abcdef"))
	if err != nil {
		t.Fatal(err)
	}
	digest := fulfillmentCursorDigest(ListInput{Limit: 20}, "scope-a")
	encoded, err := codec.encode("intent-1", digest)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = codec.decode(encoded+"=", digest); err == nil {
		t.Fatal("padded non-canonical cursor must be rejected")
	}
}
