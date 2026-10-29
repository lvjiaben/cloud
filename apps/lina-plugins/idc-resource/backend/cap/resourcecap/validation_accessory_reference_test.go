package resourcecap

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strconv"
	"strings"
	"testing"
)

func TestAccessoryReferenceIsOpaqueWithoutExemptingSecretFields(t *testing.T) {
	for _, prefix := range []string{"ACO_", "ACD_", "SVR-"} {
		for index := 0; index < 512; index++ {
			digest := sha256.Sum256([]byte("accessory-reference-regression-" + strconv.Itoa(index)))
			suffix := hex.EncodeToString(digest[:16])
			if prefix == "SVR-" {
				suffix = strings.ToUpper(suffix)
			}
			identifier := prefix + suffix
			payload, err := json.Marshal(map[string]any{"sourceParameters": map[string]string{"objectNo": identifier}})
			if err != nil {
				t.Fatal(err)
			}
			if err = ValidateOwnerMetadata(payload); err != nil {
				t.Fatalf("generated accessory reference rejected: %s: %v", identifier, err)
			}
			payload, err = json.Marshal(map[string]string{"password": identifier})
			if err != nil {
				t.Fatal(err)
			}
			if ValidateOwnerMetadata(payload) == nil {
				t.Fatal("secret-bearing key bypassed scan")
			}
		}
		payload, err := json.Marshal(map[string]string{"objectNo": prefix + "password=must-not-pass"})
		if err != nil {
			t.Fatal(err)
		}
		if ValidateOwnerMetadata(payload) == nil {
			t.Fatal("prefix disguised credential material")
		}
	}
}
