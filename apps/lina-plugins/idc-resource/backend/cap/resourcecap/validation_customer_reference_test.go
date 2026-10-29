// This file verifies generated customer identities cannot randomly block paid provisioning.

package resourcecap

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"strconv"
	"testing"
)

func TestCustomerReferenceNeverDecodesRandomIdentifierMaterial(t *testing.T) {
	identifiers := []string{"CUS_f30b8beaf12b73ba575722097f1a22d4"}
	for i := 0; i < 256; i++ {
		digest := sha256.Sum256([]byte("resource-customer-regression-" + strconv.Itoa(i)))
		identifiers = append(identifiers, "CUS_"+hex.EncodeToString(digest[:16]))
	}
	for _, identifier := range identifiers {
		if containsCredentialMaterial(identifier) {
			t.Fatalf("generated customer identity was mistaken for a credential: %s", identifier)
		}
		payload, err := json.Marshal(map[string]string{"customerNo": identifier})
		if err != nil {
			t.Fatal(err)
		}
		if err = ValidateOwnerMetadata(payload); err != nil {
			t.Fatalf("legitimate customer metadata was rejected: %v", err)
		}
	}
}

func TestCustomerReferenceExceptionPreservesCredentialRejection(t *testing.T) {
	const identifier = "CUS_f30b8beaf12b73ba575722097f1a22d4"
	for _, value := range []string{"password=actual-test-secret", base64.StdEncoding.EncodeToString([]byte("password=actual-test-secret")), "CUS_password=actual-test-secret"} {
		payload, err := json.Marshal(map[string]string{"customerNo": value})
		if err != nil {
			t.Fatal(err)
		}
		if ValidateOwnerMetadata(payload) == nil {
			t.Fatal("credential-like customer metadata was accepted")
		}
	}
	for _, key := range []string{"password", "accessToken", "clientSecret"} {
		payload, err := json.Marshal(map[string]string{key: identifier})
		if err != nil {
			t.Fatal(err)
		}
		if ValidateOwnerMetadata(payload) == nil {
			t.Fatal("a secret-bearing field was exempted by its value's identifier format")
		}
	}
}
