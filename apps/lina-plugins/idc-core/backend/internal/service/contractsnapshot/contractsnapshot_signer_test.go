package contractsnapshot

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"testing"

	"lina-plugin-idc-core/backend/cap/contractcap"
)

func TestSubjectDigestUsesPurposeSeparatedIdentityKey(t *testing.T) {
	key := []byte("0123456789abcdef0123456789abcdef")
	service := &serviceImpl{lookupKey: append([]byte(nil), key...)}
	identity := []byte("110101199001010000")
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte("idc/personal-identity/v1\x00"))
	_, _ = mac.Write(identity)
	want := hex.EncodeToString(mac.Sum(nil))
	if got := service.subjectDigest("idc/personal-identity/v1\x00", identity); got != want {
		t.Fatalf("identity digest = %q, want %q", got, want)
	}
	if service.subjectDigest("idc/enterprise-registration/v1\x00", identity) == want {
		t.Fatal("personal and enterprise signer digests must remain purpose separated")
	}
}

func TestMatchCustomerSignerRejectsInvalidInputAndClearsProviderBuffers(t *testing.T) {
	name := []byte("Example Customer")
	identity := []byte("110101199001010000")
	observed := &contractcap.ObservedCustomerSigner{
		CustomerType:           contractcap.CustomerTypeIndividual,
		LegalName:              name,
		PersonalIdentityNumber: identity,
	}
	service := &serviceImpl{lookupKey: []byte("0123456789abcdef0123456789abcdef")}
	if err := service.MatchCustomerSigner(
		context.Background(), contractcap.VerifiedIdentity{}, observed,
	); err == nil {
		t.Fatal("invalid expected identity was accepted")
	}
	if observed.LegalName != nil || observed.PersonalIdentityNumber != nil {
		t.Fatal("provider signer buffers were retained")
	}
	assertCleared(t, name)
	assertCleared(t, identity)
}

func assertCleared(t *testing.T, value []byte) {
	t.Helper()
	for _, current := range value {
		if current != 0 {
			t.Fatal("provider signer buffer was not cleared")
		}
	}
}
