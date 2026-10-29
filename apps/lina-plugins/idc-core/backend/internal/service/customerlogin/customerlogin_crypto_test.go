// This file verifies contact normalization, purpose separation and encrypted
// identity tamper rejection without storing passwords in the derived directory.
package customerlogin

import (
	"crypto/aes"
	"crypto/cipher"
	"lina-plugin-idc-core/backend/cap/customerlogincap"
	"strings"
	"testing"
)

func TestContactNormalizationAndEncryptedIdentity(t *testing.T) {
	block, err := aes.NewCipher([]byte(strings.Repeat("k", 32)))
	if err != nil {
		t.Fatal(err)
	}
	seal, err := cipher.NewGCM(block)
	if err != nil {
		t.Fatal(err)
	}
	s := &serviceImpl{indexKey: []byte(strings.Repeat("i", 32)), seal: seal}
	if s.digest(customerlogincap.Email, " Current@Example.com ") != s.digest(customerlogincap.Email, "current@example.com") {
		t.Fatal("email normalization differs")
	}
	if s.digest(customerlogincap.Phone, "13800138000") != s.digest(customerlogincap.Phone, "+8613800138000") {
		t.Fatal("phone normalization differs")
	}
	for _, bad := range []string{"user <a@example.com>", "bad", strings.Repeat("x", 321) + "@example.com"} {
		if s.digest(customerlogincap.Email, bad) != "" {
			t.Fatal("accepted invalid email")
		}
	}
	in := &customerlogincap.Identity{UserID: 42, CustomerNo: "CUS_test", Username: "independent", Email: "current@example.com", Phone: "13800138000", Revision: 3}
	encoded, err := s.encrypt(in)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(encoded, in.Email) || strings.Contains(encoded, in.Phone) {
		t.Fatal("plaintext contact in storage")
	}
	out, err := s.decrypt(encoded)
	if err != nil || *out != *in {
		t.Fatal("identity round trip failed", err)
	}
	replacement := "A"
	if encoded[0] == 'A' {
		replacement = "B"
	}
	if _, err = s.decrypt(replacement + encoded[1:]); err == nil {
		t.Fatal("accepted tampered identity")
	}
}
