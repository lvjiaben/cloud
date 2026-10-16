package serviceaction

import (
	"archive/zip"
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"io"
	"math/big"
	"strings"
	"testing"
	"time"
)

func TestSanitizeCertificateArchiveKeepsOnlyPublicCertificates(t *testing.T) {
	certificate := testCertificatePEM(t)
	source := certificateArchive(t, map[string]string{
		"nested/certificate.pem": certificate,
		"chain.crt":              certificate,
		"readme.txt":             "supplier note",
	})
	safe, err := sanitizeCertificateArchive(source)
	if err != nil {
		t.Fatal(err)
	}
	reader, err := zip.NewReader(bytes.NewReader(safe), int64(len(safe)))
	if err != nil || len(reader.File) != 2 {
		t.Fatalf("sanitized archive files=%d err=%v", len(reader.File), err)
	}
	for _, file := range reader.File {
		if strings.Contains(file.Name, "/") || file.Name == "readme.txt" {
			t.Fatalf("unsafe archive name %q", file.Name)
		}
	}
}

func TestSanitizeCertificateArchiveRejectsPrivateKey(t *testing.T) {
	for name, content := range map[string]string{
		"private.key":  "secret",
		"bundle.pem":   "-----BEGIN PRIVATE KEY-----\nsecret\n-----END PRIVATE KEY-----",
		"identity.pfx": "binary",
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := sanitizeCertificateArchive(certificateArchive(t, map[string]string{name: content, "certificate.crt": testCertificatePEM(t)})); err == nil {
				t.Fatal("private-key material was accepted")
			}
		})
	}
}

func TestSanitizeCertificateArchiveRejectsFakeOrTrailingMaterial(t *testing.T) {
	valid := testCertificatePEM(t)
	for name, content := range map[string]string{
		"fake.pem":     "-----BEGIN CERTIFICATE-----\nPUBLIC\n-----END CERTIFICATE-----",
		"trailing.pem": valid + "hidden binary",
		"mixed.pem":    valid + "-----BEGIN ENCRYPTED PRIVATE KEY-----\nAA==\n-----END ENCRYPTED PRIVATE KEY-----\n",
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := sanitizeCertificateArchive(certificateArchive(t, map[string]string{name: content})); err == nil {
				t.Fatal("invalid or trailing certificate material was accepted")
			}
		})
	}
}

func TestSanitizeCertificateArchiveAcceptsDERCertificate(t *testing.T) {
	der, _ := testCertificate(t)
	safe, err := sanitizeCertificateArchive(certificateArchiveBytes(t, map[string][]byte{"certificate.cer": der}))
	if err != nil {
		t.Fatal(err)
	}
	reader, err := zip.NewReader(bytes.NewReader(safe), int64(len(safe)))
	if err != nil || len(reader.File) != 1 {
		t.Fatalf("read normalized archive: files=%d err=%v", len(reader.File), err)
	}
	file, err := reader.File[0].Open()
	if err != nil {
		t.Fatal(err)
	}
	content, err := io.ReadAll(file)
	_ = file.Close()
	if err != nil || !bytes.Contains(content, []byte("CERTIFICATE")) {
		t.Fatal("DER certificate was not normalized to public PEM")
	}
}

// The archive sanitizer parses public certificates; it does not validate a CA
// signature. Altering its final signature byte gives deterministic DER fixtures
// for every whitespace byte without relying on randomized key signatures.
func TestPublicCertificatePEMPreservesBinarySignatureBytes(t *testing.T) {
	der, _ := testCertificate(t)
	for _, last := range []byte{' ', '\t', '\n', '\r', '\v', '\f'} {
		binary := bytes.Clone(der)
		binary[len(binary)-1] = last
		if _, err := x509.ParseCertificate(binary); err != nil {
			t.Fatalf("invalid DER fixture: %v", err)
		}
		encoded, err := publicCertificatePEM(binary)
		if err != nil {
			t.Fatalf("valid binary certificate ending in %x rejected: %v", last, err)
		}
		block, rest := pem.Decode(encoded)
		if block == nil || len(rest) != 0 || !bytes.Equal(block.Bytes, binary) {
			t.Fatal("binary certificate bytes were changed")
		}
		if _, err = publicCertificatePEM(append(bytes.Clone(binary), ' ')); err == nil {
			t.Fatal("trailing bytes outside the DER certificate were accepted")
		}
	}
}

func certificateArchive(t *testing.T, entries map[string]string) []byte {
	converted := make(map[string][]byte, len(entries))
	for name, content := range entries {
		converted[name] = []byte(content)
	}
	return certificateArchiveBytes(t, converted)
}

func certificateArchiveBytes(t *testing.T, entries map[string][]byte) []byte {
	t.Helper()
	var output bytes.Buffer
	writer := zip.NewWriter(&output)
	for name, content := range entries {
		entry, err := writer.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = entry.Write(content); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return output.Bytes()
}

func testCertificatePEM(t *testing.T) string {
	der, _ := testCertificate(t)
	return string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}))
}

func testCertificate(t *testing.T) ([]byte, *ecdsa.PrivateKey) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	template := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "example.test"}, DNSNames: []string{"example.test"}, NotBefore: now.Add(-time.Minute), NotAfter: now.Add(time.Hour), KeyUsage: x509.KeyUsageDigitalSignature}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	return der, key
}
