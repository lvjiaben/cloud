// This file verifies enterprise legal, material, and cryptographic validation.

package enterpriseverification

import (
	"bytes"
	"testing"
	"time"

	"lina-plugin-idc-compliance/backend/internal/model/entity"
)

func TestUnifiedSocialCreditCodeChecksum(t *testing.T) {
	if !validUnifiedSocialCreditCode([]byte("91350211M000100Y46")) {
		t.Fatal("known valid unified social credit code was rejected")
	}
	if validUnifiedSocialCreditCode([]byte("91350211M000100Y44")) ||
		validUnifiedSocialCreditCode([]byte("91350211I000100Y46")) {
		t.Fatal("invalid unified social credit code was accepted")
	}
}

func TestMaterialContentSniffingRejectsDeclaredTypeMismatch(t *testing.T) {
	pdf := []byte("%PDF-1.4\n1 0 obj\n<<>>\nendobj\n%%EOF")
	mediaType, err := detectMaterialMediaType(pdf, "application/pdf")
	if err != nil || mediaType != "application/pdf" {
		t.Fatalf("mediaType=%q err=%v", mediaType, err)
	}
	mediaType, err = detectMaterialMediaType(pdf, "application/octet-stream")
	if err != nil || mediaType != "application/pdf" {
		t.Fatalf("octet stream mediaType=%q err=%v", mediaType, err)
	}
	if _, err = detectMaterialMediaType(pdf, "image/png"); err == nil {
		t.Fatal("declared media type mismatch was accepted")
	}
}

func TestEnterpriseEnvelopeIsDeterministicPerUniqueLabel(t *testing.T) {
	key := bytes.Repeat([]byte{0x7c}, 32)
	plaintext := []byte("highly-sensitive-enterprise-original")
	first, err := encryptEnvelope(key, plaintext, "material:mat_one")
	if err != nil {
		t.Fatal(err)
	}
	second, err := encryptEnvelope(key, plaintext, "material:mat_one")
	if err != nil {
		t.Fatal(err)
	}
	other, err := encryptEnvelope(key, plaintext, "material:mat_two")
	if err != nil {
		t.Fatal(err)
	}
	changed, err := encryptEnvelope(key, []byte("different-sensitive-original"), "material:mat_one")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first, second) || bytes.Equal(first, other) || bytes.Equal(first, changed) ||
		bytes.Equal(first[4:16], changed[4:16]) || bytes.Contains(first, plaintext) {
		t.Fatal("enterprise envelope determinism or plaintext exclusion failed")
	}
}

func TestEnterpriseEnvelopeDecryptRejectsTamperLabelAndOversize(t *testing.T) {
	key := bytes.Repeat([]byte{0x4d}, 32)
	plaintext := []byte("bounded-sensitive-original")
	envelope, err := encryptEnvelope(key, plaintext, "material:mat_one")
	if err != nil {
		t.Fatal(err)
	}
	decrypted, err := decryptEnvelope(key, envelope, "material:mat_one", len(plaintext))
	if err != nil || !bytes.Equal(decrypted, plaintext) {
		t.Fatalf("decrypted=%q err=%v", decrypted, err)
	}
	clearBytes(decrypted)
	if changed, changedErr := decryptEnvelope(key, envelope, "material:mat_two", len(plaintext)); changedErr == nil || changed != nil {
		t.Fatalf("changed-label plaintext=%q err=%v", changed, changedErr)
	}
	tampered := append([]byte(nil), envelope...)
	tampered[len(tampered)-1] ^= 0x01
	if changed, changedErr := decryptEnvelope(key, tampered, "material:mat_one", len(plaintext)); changedErr == nil || changed != nil {
		t.Fatalf("tampered plaintext=%q err=%v", changed, changedErr)
	}
	if changed, changedErr := decryptEnvelope(key, envelope, "material:mat_one", len(plaintext)-1); changedErr == nil || changed != nil {
		t.Fatalf("oversize plaintext=%q err=%v", changed, changedErr)
	}
}

func TestMaskLegalNameAcceptsAbsentOptionalAgent(t *testing.T) {
	if value := maskLegalName(nil); value != "" {
		t.Fatalf("maskLegalName(nil)=%q want empty", value)
	}
}

func TestMaterialAccessHonorsRetentionAndLegalHold(t *testing.T) {
	now := time.Date(2026, time.August, 13, 8, 0, 0, 0, time.UTC)
	future := now.Add(time.Hour)
	past := now.Add(-time.Nanosecond)

	tests := []struct {
		name string
		row  *entity.EnterpriseMaterial
		want bool
	}{
		{name: "available without deadline", row: &entity.EnterpriseMaterial{StatusCode: materialStatusAvailable}, want: true},
		{name: "available before deadline", row: &entity.EnterpriseMaterial{StatusCode: materialStatusAvailable, RetentionUntil: &future}, want: true},
		{name: "expired", row: &entity.EnterpriseMaterial{StatusCode: materialStatusAvailable, RetentionUntil: &past}},
		{name: "available flag cannot bypass hold", row: &entity.EnterpriseMaterial{StatusCode: materialStatusAvailable, LegalHold: true}},
		{name: "explicit legal hold remains accessible", row: &entity.EnterpriseMaterial{StatusCode: materialStatusLegalHold, LegalHold: true, RetentionUntil: &past}, want: true},
		{name: "inconsistent legal hold state", row: &entity.EnterpriseMaterial{StatusCode: materialStatusLegalHold}},
		{name: "missing row", row: nil},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := materialAccessibleAt(test.row, now); got != test.want {
				t.Fatalf("materialAccessibleAt()=%v want %v", got, test.want)
			}
		})
	}
}

func TestManagementTargetIsBoundedAndUnambiguous(t *testing.T) {
	caseNo := "ent_0123456789abcdef0123456789abcdef"
	materialNo := "mat_0123456789abcdef0123456789abcdef"
	target := managementTarget(caseNo, materialNo)
	if len(target) != len("enterprise-material:")+64 {
		t.Fatalf("managementTarget length=%d target=%q", len(target), target)
	}
	if target == managementTarget(caseNo+":"+materialNo, "") ||
		target == managementTarget(caseNo, materialNo+":changed") {
		t.Fatal("management target did not preserve exact case and material binding")
	}
}

func TestApplicantRoleRejectsUnnecessaryOrMissingAgentIdentity(t *testing.T) {
	base := SubmitInput{
		CaseNo:    "ent_0123456789abcdef0123456789abcdef",
		LegalName: []byte("示例科技有限公司"), RegistrationCountryCode: "CN",
		RegistrationIdentifier:            []byte("91350211M000100Y46"),
		LegalRepresentativeName:           []byte("测试法人"),
		LegalRepresentativeIdentityNumber: []byte("110101199001010015"),
		IdempotencyKey:                    "enterprise-submission-idempotency-0001",
	}
	base.ApplicantRole = ApplicantLegalRepresentative
	base.AgentName = []byte("不应收集")
	base.AgentIdentityNumber = []byte("110101199202020027")
	if _, err := validateLegalProfile(base); err == nil {
		t.Fatal("legal representative submission accepted unnecessary agent identity")
	}
	base.AgentName = nil
	base.AgentIdentityNumber = nil
	base.ApplicantRole = ApplicantAuthorizedAgent
	if _, err := validateLegalProfile(base); err == nil {
		t.Fatal("authorized agent submission accepted missing agent identity")
	}
}
