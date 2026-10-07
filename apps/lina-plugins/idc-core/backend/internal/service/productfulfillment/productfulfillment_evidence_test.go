package productfulfillment

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"testing"
)

func TestProviderEvidenceIsPurposeBoundAndDoesNotPersistPlaintext(t *testing.T) {
	key := bytes.Repeat([]byte{0x41}, 32)
	sealerValue, err := NewProviderEvidenceSealer("key-v1", key)
	if err != nil {
		t.Fatal(err)
	}
	sealer := sealerValue.(*aesProviderEvidenceSealer)
	sealer.random = bytes.NewReader(bytes.Repeat([]byte{0x22}, 12))
	plain := []byte(`{"supplierReceipt":"private-receipt-1001"}`)
	sealed, err := sealer.Seal("pft_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", 2, plain)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(sealed.Ciphertext, []byte("private-receipt")) || bytes.Contains(sealed.Nonce, []byte("private-receipt")) {
		t.Fatal("provider evidence plaintext leaked into persisted material")
	}
	block, _ := aes.NewCipher(key)
	aead, _ := cipher.NewGCM(block)
	opened, err := aead.Open(nil, sealed.Nonce, sealed.Ciphertext, providerEvidenceAAD("pft_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", 2))
	if err != nil || !bytes.Equal(opened, plain) {
		t.Fatalf("open owner-private evidence failed: plaintext=%q err=%v", opened, err)
	}
	if _, err = aead.Open(nil, sealed.Nonce, sealed.Ciphertext, providerEvidenceAAD("pft_bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", 2)); err == nil {
		t.Fatal("provider evidence must be bound to its exact task and attempt")
	}
}
