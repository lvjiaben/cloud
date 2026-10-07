// This file verifies local identity validation rejects unsupported inputs before provider use.

package personalverification

import (
	"bytes"
	"testing"
	"time"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-compliance/backend/internal/provider/aliyunface"
)

func TestValidateStartInputRejectsUnsupportedDocumentAgeAndChecksum(t *testing.T) {
	now := time.Date(2026, 8, 12, 8, 0, 0, 0, time.UTC)
	adult := syntheticIdentityNumber(t, time.Date(2000, 2, 29, 0, 0, 0, 0, time.UTC))
	input := StartInput{
		CertType: aliyunface.CertTypeIdentityCard, CertName: []byte("测试用户"),
		CertNo: adult, ReturnURL: "https://portal.example.test/identity/return",
	}
	validated, err := validateStartInput(input, now)
	if err != nil || string(validated.number) != string(adult) || validated.maskedNumber == string(adult) {
		t.Fatalf("validated=%+v err=%v", validated, err)
	}
	clearBytes(validated.name)
	clearBytes(validated.number)

	unsupported := input
	unsupported.CertType = "PASSPORT"
	if _, err = validateStartInput(unsupported, now); !bizerr.Is(err, CodeUnsupportedDocument) {
		t.Fatalf("unsupported document error=%v", err)
	}
	minor := input
	minor.CertNo = syntheticIdentityNumber(t, time.Date(2014, 8, 13, 0, 0, 0, 0, time.UTC))
	if _, err = validateStartInput(minor, now); !bizerr.Is(err, CodeAgeUnsupported) {
		t.Fatalf("minor error=%v", err)
	}
	invalid := input
	invalid.CertNo = append([]byte(nil), adult...)
	invalid.CertNo[17] = '0'
	if bytes.Equal(invalid.CertNo, adult) {
		invalid.CertNo[17] = '1'
	}
	if _, err = validateStartInput(invalid, now); !bizerr.Is(err, CodeUnsupportedDocument) {
		t.Fatalf("invalid checksum error=%v", err)
	}
	controlName := input
	controlName.CertName = []byte("测试\n用户")
	if _, err = validateStartInput(controlName, now); !bizerr.Is(err, CodeInvalid) {
		t.Fatalf("control-character name error=%v", err)
	}
}

func TestSnapshotProducerIsBoundedAndClearsItsCallbackView(t *testing.T) {
	challenge := bytes.Repeat([]byte{0x5a}, snapshotChallengeBytes)
	producer, err := newSnapshotProducer(challenge, []byte("测试用户"), []byte("110101200002290019"))
	if err != nil {
		t.Fatal(err)
	}
	var callbackView []byte
	err = producer(func(payload []byte) error {
		callbackView = payload
		decodedChallenge, name, identity, decodeErr := decodeSnapshot(payload)
		if decodeErr != nil || string(name) != "测试用户" || string(identity) != "110101200002290019" {
			t.Fatalf("name=%q identity=%q err=%v", name, identity, decodeErr)
		}
		if !bytes.Equal(decodedChallenge, challenge) {
			t.Fatal("snapshot challenge changed")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, value := range callbackView {
		if value != 0 {
			t.Fatal("snapshot producer retained plaintext after callback")
		}
	}
}

func syntheticIdentityNumber(t *testing.T, birth time.Time) []byte {
	t.Helper()
	prefix := "110101" + birth.UTC().Format("20060102") + "001"
	if len(prefix) != 17 {
		t.Fatal("synthetic identity prefix is invalid")
	}
	sum := 0
	for index := 0; index < 17; index++ {
		sum += int(prefix[index]-'0') * identityCardWeights[index]
	}
	return []byte(prefix + string(identityCardChecks[sum%11]))
}
