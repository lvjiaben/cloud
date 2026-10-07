// This file verifies supplier-identity key separation, rotation, parsing, and
// authenticated encryption behavior without external services.

package supplieridentity

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"testing"
)

type pointerCanonicalizer struct{}

func (*pointerCanonicalizer) Canonicalize(_ context.Context, input []byte) ([]byte, error) {
	return append([]byte(nil), input...), nil
}

func TestKeyringUsesIndependentVersionedFamilies(t *testing.T) {
	random := bytes.NewReader(bytes.Repeat([]byte{0x5a}, SecretBytes*3))
	if _, err := Generate(random); !errors.Is(err, ErrInvalidKeyring) {
		t.Fatalf("duplicate generated family secrets err=%v", err)
	}
	file, err := Generate(bytes.NewReader(append(
		append(bytes.Repeat([]byte{1}, SecretBytes), bytes.Repeat([]byte{2}, SecretBytes)...),
		bytes.Repeat([]byte{3}, SecretBytes)...,
	)))
	if err != nil {
		t.Fatalf("generate independent keyring: %v", err)
	}
	if file.Encryption.Keys[0].Secret == file.Lookup.Keys[0].Secret ||
		file.Lookup.Keys[0].Secret == file.Idempotency.Keys[0].Secret ||
		file.Encryption.Keys[0].Secret == file.Idempotency.Keys[0].Secret {
		t.Fatal("key families reused secret material")
	}
	if _, err = Rotate(
		file, PurposeSubjectLookup, bytes.NewReader(bytes.Repeat([]byte{1}, SecretBytes)),
	); !errors.Is(err, ErrInvalidKeyring) {
		t.Fatalf("rotation reused another family's secret err=%v", err)
	}
	rotated, err := Rotate(
		file, PurposeSubjectLookup, bytes.NewReader(bytes.Repeat([]byte{4}, SecretBytes)),
	)
	if err != nil {
		t.Fatalf("rotate lookup key: %v", err)
	}
	if rotated.Lookup.ActiveVersion != 2 || len(rotated.Lookup.Keys) != 2 ||
		rotated.Lookup.Keys[0].State != KeyStateRetained ||
		rotated.Encryption.ActiveVersion != 1 || rotated.Idempotency.ActiveVersion != 1 {
		t.Fatalf("unexpected rotated keyring: %+v", rotated)
	}
	content, err := Marshal(rotated)
	if err != nil {
		t.Fatalf("marshal keyring: %v", err)
	}
	parsed, err := Parse(content)
	clearBytes(content)
	if err != nil || parsed.Lookup.ActiveVersion != 2 {
		t.Fatalf("parsed=%+v err=%v", parsed, err)
	}
}

func TestSealBindsInstallationBindingProviderAndVersions(t *testing.T) {
	file, err := Generate(bytes.NewReader(append(
		append(bytes.Repeat([]byte{1}, SecretBytes), bytes.Repeat([]byte{2}, SecretBytes)...),
		bytes.Repeat([]byte{3}, SecretBytes)...,
	)))
	if err != nil {
		t.Fatalf("generate keyring: %v", err)
	}
	ring, err := New(file)
	if err != nil {
		t.Fatalf("construct keyring: %v", err)
	}
	defer ring.Clear()
	installation := "linst_" + string(bytes.Repeat([]byte{'1'}, 64))
	binding := "lsb_" + string(bytes.Repeat([]byte{'2'}, 32))
	plaintext := []byte("supplier-subject-001")
	sealed, err := ring.Seal(
		bytes.NewReader(bytes.Repeat([]byte{9}, 12)), installation, binding,
		ProviderTencentESSBasic, plaintext,
	)
	if err != nil {
		t.Fatalf("seal subject: %v", err)
	}
	opened, err := ring.Open(installation, binding, ProviderTencentESSBasic, sealed)
	if err != nil || !bytes.Equal(opened, plaintext) {
		t.Fatalf("opened=%q err=%v", opened, err)
	}
	clearBytes(opened)
	if _, err = ring.Open(installation, binding, ProviderNuonuoInvoice, sealed); !errors.Is(err, ErrKeyUnavailable) {
		t.Fatalf("provider AAD change err=%v", err)
	}
	sealed.Payload[0] ^= 1
	if _, err = ring.Open(installation, binding, ProviderTencentESSBasic, sealed); !errors.Is(err, ErrKeyUnavailable) {
		t.Fatalf("ciphertext tamper err=%v", err)
	}
	clearBytes(sealed.Nonce)
	clearBytes(sealed.Payload)
}

func TestDigestCandidatesCoverRetainedVersions(t *testing.T) {
	file, err := Generate(bytes.NewReader(append(
		append(bytes.Repeat([]byte{1}, SecretBytes), bytes.Repeat([]byte{2}, SecretBytes)...),
		bytes.Repeat([]byte{3}, SecretBytes)...,
	)))
	if err != nil {
		t.Fatalf("generate keyring: %v", err)
	}
	file, err = Rotate(
		file, PurposeSubjectLookup, bytes.NewReader(bytes.Repeat([]byte{4}, SecretBytes)),
	)
	if err != nil {
		t.Fatalf("rotate lookup key: %v", err)
	}
	ring, err := New(file)
	if err != nil {
		t.Fatalf("construct keyring: %v", err)
	}
	defer ring.Clear()
	digests, err := ring.LookupDigests(ProviderTencentESSBasic, []byte("canonical"))
	if err != nil || len(digests) != 2 || digests[0].KeyVersion != 1 || digests[1].KeyVersion != 2 {
		t.Fatalf("digests=%+v err=%v", digests, err)
	}
	if digests[0].Sum == digests[1].Sum {
		t.Fatal("retained lookup versions produced the same digest")
	}
}

func TestParseRejectsUnknownOrSecretBearingRevokedKeys(t *testing.T) {
	if _, err := Parse([]byte(`{"schemaVersion":"v1","unknown":true}`)); !errors.Is(err, ErrInvalidKeyring) {
		t.Fatalf("unknown field err=%v", err)
	}
	file, err := Generate(bytes.NewReader(append(
		append(bytes.Repeat([]byte{1}, SecretBytes), bytes.Repeat([]byte{2}, SecretBytes)...),
		bytes.Repeat([]byte{3}, SecretBytes)...,
	)))
	if err != nil {
		t.Fatalf("generate keyring: %v", err)
	}
	file.Lookup.Keys = append(file.Lookup.Keys, FileKey{
		Version: 2, State: KeyStateRevoked, Secret: file.Lookup.Keys[0].Secret,
	})
	if _, err = New(file); !errors.Is(err, ErrInvalidKeyring) {
		t.Fatalf("secret-bearing revoked key err=%v", err)
	}
	file, err = Generate(bytes.NewReader(append(
		append(bytes.Repeat([]byte{1}, SecretBytes), bytes.Repeat([]byte{2}, SecretBytes)...),
		bytes.Repeat([]byte{3}, SecretBytes)...,
	)))
	if err != nil {
		t.Fatalf("regenerate keyring: %v", err)
	}
	file.Lookup.Keys = append(file.Lookup.Keys, FileKey{
		Version: 2, State: KeyStateRetained, Secret: file.Encryption.Keys[0].Secret,
	})
	if _, err = Rotate(
		file, PurposeSubjectLookup, bytes.NewReader(bytes.Repeat([]byte{4}, SecretBytes)),
	); !errors.Is(err, ErrInvalidKeyring) {
		t.Fatalf("non-latest active or duplicate retained key err=%v", err)
	}
}

func TestNewServiceRejectsTypedNilCanonicalizer(t *testing.T) {
	file, err := Generate(bytes.NewReader(append(
		append(bytes.Repeat([]byte{1}, SecretBytes), bytes.Repeat([]byte{2}, SecretBytes)...),
		bytes.Repeat([]byte{3}, SecretBytes)...,
	)))
	if err != nil {
		t.Fatalf("generate keyring: %v", err)
	}
	ring, err := New(file)
	if err != nil {
		t.Fatalf("construct keyring: %v", err)
	}
	defer ring.Clear()
	var canonicalizer *pointerCanonicalizer
	if _, err = NewService(
		&sql.DB{}, "linst_"+string(bytes.Repeat([]byte{'1'}, 64)), ring,
		map[string]Canonicalizer{ProviderTencentESSBasic: canonicalizer},
	); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("typed-nil canonicalizer err=%v", err)
	}
}
