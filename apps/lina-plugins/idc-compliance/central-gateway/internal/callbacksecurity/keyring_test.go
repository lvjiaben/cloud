// This file verifies independent callback and reconciliation key families.

package callbacksecurity

import (
	"bytes"
	"crypto/rand"
	"testing"
)

func TestKeyringSealAndProviderScopedEventDigest(t *testing.T) {
	file, err := Generate(rand.Reader)
	if err != nil {
		t.Fatalf("generate keyring: %v", err)
	}
	encoded, err := Marshal(file)
	if err != nil {
		t.Fatalf("marshal keyring: %v", err)
	}
	parsed, err := Parse(encoded)
	clear(encoded)
	if err != nil {
		t.Fatalf("parse keyring: %v", err)
	}
	ring, err := New(parsed)
	if err != nil {
		t.Fatalf("construct keyring: %v", err)
	}
	defer ring.Clear()
	plaintext := []byte("raw callback evidence")
	sealed, err := ring.Seal(
		rand.Reader, "lev_00000000000000000000000000000001",
		"lwr_00000000000000000000000000000001", "tencent_essbasic", "callback", plaintext,
	)
	if err != nil || bytes.Contains(sealed.Ciphertext, plaintext) {
		t.Fatalf("seal evidence=%+v err=%v", sealed, err)
	}
	opened, err := ring.Open(
		"lev_00000000000000000000000000000001",
		"lwr_00000000000000000000000000000001", "tencent_essbasic", "callback", sealed,
	)
	if err != nil || !bytes.Equal(opened, plaintext) {
		t.Fatalf("open evidence=%q err=%v", opened, err)
	}
	clear(opened)
	tencent, err := ring.EventDigests("tencent_essbasic", []byte("event-1"))
	if err != nil {
		t.Fatalf("digest Tencent event: %v", err)
	}
	nuonuo, err := ring.EventDigests("nuonuo_invoice", []byte("event-1"))
	if err != nil {
		t.Fatalf("digest Nuonuo event: %v", err)
	}
	if tencent[0].Sum == nuonuo[0].Sum || tencent[0].KeyVersion != ring.ActiveEventVersion() {
		t.Fatal("event digest was not provider-scoped or active")
	}
	reference := []byte("provider-flow-1")
	sealedReference, err := ring.SealReference(
		rand.Reader, "lwr_00000000000000000000000000000001",
		"tencent_essbasic", reference,
	)
	if err != nil || bytes.Contains(sealedReference.Ciphertext, reference) {
		t.Fatalf("seal reference=%+v err=%v", sealedReference, err)
	}
	openedReference, err := ring.OpenReference(
		"lwr_00000000000000000000000000000001", "tencent_essbasic", sealedReference,
	)
	if err != nil || !bytes.Equal(openedReference, reference) {
		t.Fatalf("open reference=%q err=%v", openedReference, err)
	}
	clear(openedReference)
	referenceDigests, err := ring.ReferenceDigests("tencent_essbasic", reference)
	if err != nil || referenceDigests[0].KeyVersion != ring.ActiveReferenceLookupVersion() {
		t.Fatalf("reference digest=%+v err=%v", referenceDigests, err)
	}
	operationDigests, err := ring.TakeoverOperationDigests(
		"linst_"+string(bytes.Repeat([]byte{'1'}, 64)), bytes.Repeat([]byte{7}, SecretBytes),
	)
	if err != nil || operationDigests[0].KeyVersion != ring.ActiveTakeoverOperationVersion() {
		t.Fatalf("takeover operation digest=%+v err=%v", operationDigests, err)
	}
}

func TestKeyringRejectsReusedFamilySecret(t *testing.T) {
	file, err := Generate(rand.Reader)
	if err != nil {
		t.Fatalf("generate keyring: %v", err)
	}
	file.Event.Keys[0].Secret = file.Evidence.Keys[0].Secret
	if _, err = New(file); err == nil {
		t.Fatal("reused evidence/event secret was accepted")
	}
	file, err = Generate(rand.Reader)
	if err != nil {
		t.Fatalf("regenerate keyring: %v", err)
	}
	file.TakeoverOperation.Keys[0].Secret = file.Reference.Keys[0].Secret
	if _, err = New(file); err == nil {
		t.Fatal("reused reference/takeover secret was accepted")
	}
}
