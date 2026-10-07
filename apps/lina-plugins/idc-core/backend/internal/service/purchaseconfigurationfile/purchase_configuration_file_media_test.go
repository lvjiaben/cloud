package purchaseconfigurationfile

import (
	"net/http"
	"strings"
	"testing"

	"lina-core/pkg/plugin/capability/capmodel"
)

func TestDeclaredExtensionAndSniffedMIMEAreValidatedWithoutOSMappings(t *testing.T) {
	textPolicy := &capmodel.ConfigurationUploadPolicyDeclaration{
		AllowedExtensions: []string{".pem", ".csr", ".key"},
		AllowedMIMETypes:  []string{"text/plain"},
	}
	actualText := strings.Split(http.DetectContentType([]byte("-----BEGIN CERTIFICATE-----\n")), ";")[0]
	for _, name := range []string{"certificate.pem", "request.csr", "private.key"} {
		if !extensionAllowed(textPolicy, name) || !mimeAllowed(textPolicy, actualText) {
			t.Fatalf("expected declared %s + sniffed %s to be accepted without OS MIME mapping", name, actualText)
		}
	}

	pdfPolicy := &capmodel.ConfigurationUploadPolicyDeclaration{
		AllowedExtensions: []string{".pdf"}, AllowedMIMETypes: []string{"application/pdf"},
	}
	actualFakePDF := strings.Split(http.DetectContentType([]byte("plain text pretending to be a PDF")), ";")[0]
	if extensionAllowed(pdfPolicy, "proof.pdf") && mimeAllowed(pdfPolicy, actualFakePDF) {
		t.Fatal("expected fake PDF content to be rejected by the sniffed MIME allowlist")
	}
}
