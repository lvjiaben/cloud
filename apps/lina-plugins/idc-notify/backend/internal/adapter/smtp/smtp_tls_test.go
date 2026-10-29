// This file verifies the confirmed certificate-independent merchant policy
// while requiring encrypted TLS 1.2+ for implicit TLS and STARTTLS.
package smtp

import (
	"bufio"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type tlsDeliveryProbe struct{ auth, message bool }

func tlsTestCertificate(t *testing.T, hostname string) (tls.Certificate, []byte) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: hostname}, DNSNames: []string{hostname}, NotBefore: time.Now().Add(-time.Minute), NotAfter: time.Now().Add(time.Hour), KeyUsage: x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}, BasicConstraintsValid: true, IsCA: true}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	cert := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	rawKey, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	pair, err := tls.X509KeyPair(cert, pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: rawKey}))
	if err != nil {
		t.Fatal(err)
	}
	return pair, cert
}
func TestSMTPEncryptedMerchantPolicy(t *testing.T) {
	for _, security := range []string{"implicit_tls", "starttls"} {
		for _, mode := range []string{"trusted", "untrusted", "wrong_hostname", "obsolete_tls"} {
			t.Run(security+"/"+mode, func(t *testing.T) {
				hostname := "localhost"
				if mode == "wrong_hostname" {
					hostname = "another.example.invalid"
				}
				pair, root := tlsTestCertificate(t, hostname)
				if mode == "untrusted" {
					_, root = tlsTestCertificate(t, "untrusted-root.example.invalid")
				}
				ca := filepath.Join(t.TempDir(), "root.pem")
				if err := os.WriteFile(ca, root, 0600); err != nil {
					t.Fatal(err)
				}
				listener, err := net.Listen("tcp", "127.0.0.1:0")
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { listener.Close() })
				done := make(chan tlsDeliveryProbe, 1)
				go func() {
					var probe tlsDeliveryProbe
					defer func() { done <- probe }()
					raw, err := listener.Accept()
					if err != nil {
						return
					}
					defer raw.Close()
					raw.SetDeadline(time.Now().Add(5 * time.Second))
					if security == "starttls" {
						writer := bufio.NewWriter(raw)
						writer.WriteString("220 localhost\r\n")
						writer.Flush()
						reader := bufio.NewReader(raw)
						if _, err = reader.ReadString('\n'); err != nil {
							return
						}
						writer.WriteString("250-localhost\r\n250 STARTTLS\r\n")
						writer.Flush()
						if _, err = reader.ReadString('\n'); err != nil {
							return
						}
						writer.WriteString("220 begin TLS\r\n")
						writer.Flush()
					}
					secured := tls.Server(raw, &tls.Config{Certificates: []tls.Certificate{pair}, MinVersion: tls.VersionTLS12})
					if mode == "obsolete_tls" {
						secured = tls.Server(raw, &tls.Config{Certificates: []tls.Certificate{pair}, MinVersion: tls.VersionTLS10, MaxVersion: tls.VersionTLS11})
					}
					if err = secured.Handshake(); err != nil {
						return
					}
					writer := bufio.NewWriter(secured)
					reader := bufio.NewReader(secured)
					if security == "implicit_tls" {
						writer.WriteString("220 localhost\r\n")
						writer.Flush()
					}
					for {
						line, err := reader.ReadString('\n')
						if err != nil {
							return
						}
						upper := strings.ToUpper(line)
						switch {
						case strings.HasPrefix(upper, "EHLO"):
							writer.WriteString("250-localhost\r\n250 AUTH PLAIN\r\n")
						case strings.HasPrefix(upper, "AUTH"):
							probe.auth = true
							writer.WriteString("235 authenticated\r\n")
						case strings.HasPrefix(upper, "DATA"):
							writer.WriteString("354 end with dot\r\n")
							writer.Flush()
							for {
								line, err = reader.ReadString('\n')
								if err != nil {
									return
								}
								if line == ".\r\n" {
									break
								}
								probe.message = true
							}
							writer.WriteString("250 queued\r\n")
						case strings.HasPrefix(upper, "QUIT"):
							writer.WriteString("221 bye\r\n")
							writer.Flush()
							return
						default:
							writer.WriteString("250 OK\r\n")
						}
						if err = writer.Flush(); err != nil {
							return
						}
					}
				}()
				_, port, err := net.SplitHostPort(listener.Addr().String())
				if err != nil {
					t.Fatal(err)
				}
				cmd := exec.Command(os.Args[0], "-test.run=^TestSMTPEncryptedMerchantPolicyHelper$")
				cmd.Env = append(os.Environ(), "IDC_SMTP_TLS_HELPER=1", "IDC_SMTP_TLS_PORT="+port, "IDC_SMTP_TLS_SECURITY="+security, "IDC_SMTP_TLS_MODE="+mode, "SSL_CERT_FILE="+ca, "SSL_CERT_DIR="+t.TempDir())
				if output, err := cmd.CombinedOutput(); err != nil {
					t.Fatalf("TLS helper failed: %v %s", err, output)
				}
				select {
				case probe := <-done:
					if mode != "obsolete_tls" {
						if !probe.auth || !probe.message {
							t.Fatal("encrypted merchant-policy TLS did not deliver")
						}
					} else if probe.auth || probe.message {
						t.Fatal("credentials or message crossed failed TLS handshake")
					}
				case <-time.After(6 * time.Second):
					t.Fatal("SMTP listener did not finish")
				}
			})
		}
	}
}

// TestSMTPEncryptedMerchantPolicyHelper starts a fresh root-certificate process for each case;
// the production sender and TLS configuration are used without source overlays.
func TestSMTPEncryptedMerchantPolicyHelper(t *testing.T) {
	if os.Getenv("IDC_SMTP_TLS_HELPER") != "1" {
		t.Skip("subprocess-only TLS fixture")
	}
	adapter, err := newAdapter(accessorStub{}, map[string]struct{}{}, &net.Dialer{Timeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	err = adapter.send(context.Background(), configuration{host: "localhost", port: os.Getenv("IDC_SMTP_TLS_PORT"), security: os.Getenv("IDC_SMTP_TLS_SECURITY"), username: "local-test", fromAddress: "sender@example.invalid"}, []byte("synthetic-test-password"), "receiver@example.invalid", "TLS test", "synthetic test message", "tls-test")
	if os.Getenv("IDC_SMTP_TLS_MODE") != "obsolete_tls" {
		if err != nil {
			t.Fatal(err)
		}
	} else if err == nil {
		t.Fatalf("expected obsolete TLS rejection, got %v", err)
	}
}
