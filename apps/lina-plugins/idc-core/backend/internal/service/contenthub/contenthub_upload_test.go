package contenthub

import (
	"encoding/binary"
	"testing"
	"time"

	"lina-core/pkg/bizerr"
)

func TestInspectDownloadProbeRequiresScanningForDangerousAllowedTypes(t *testing.T) {
	zipHeader := make([]byte, 32)
	copy(zipHeader, []byte("PK\x03\x04"))
	exeHeader := []byte{'M', 'Z', 0, 0}
	sevenZipHeader := clamdOrdinarySevenZipSample()
	isoHeader := make([]byte, 0x8006)
	copy(isoHeader[0x8001:], []byte("CD001"))
	for _, test := range []struct {
		name, fileName, kind string
		header               []byte
		size                 int64
	}{
		{name: "zip", fileName: "bundle.zip", kind: "zip", header: zipHeader, size: 32},
		{name: "exe", fileName: "agent.exe", kind: "exe", header: exeHeader, size: 4},
		{name: "7z", fileName: "bundle.7z", kind: "7z", header: sevenZipHeader, size: 8},
		{name: "iso", fileName: "image.iso", kind: "iso", header: isoHeader, size: int64(len(isoHeader))},
	} {
		t.Run(test.name, func(t *testing.T) {
			inspection, err := inspectDownloadProbe(test.header, test.fileName, test.size)
			if err != nil || inspection.Kind != test.kind || !inspection.ScanRequired {
				t.Fatalf("inspection=%+v err=%v", inspection, err)
			}
		})
	}
}

func TestInspectDownloadProbeRejectsEncryptedZipAndExtensionSpoof(t *testing.T) {
	encrypted := make([]byte, 32)
	copy(encrypted, []byte("PK\x03\x04"))
	binary.LittleEndian.PutUint16(encrypted[6:8], 1)
	if _, err := inspectDownloadProbe(encrypted, "bundle.zip", int64(len(encrypted))); err == nil {
		t.Fatal("encrypted zip header was accepted")
	}
	if _, err := inspectDownloadProbe([]byte("not-an-executable"), "agent.exe", 17); err == nil {
		t.Fatal("extension-only executable was accepted")
	}
}

func TestContentWrapPreservesBusinessConflict(t *testing.T) {
	err := contentWrap(bizerr.NewCode(CodeConflict), "scanner transition")
	if !bizerr.Is(err, CodeConflict) {
		t.Fatalf("business conflict was replaced: %v", err)
	}
}

func TestValidateDownloadUploadUsesDeploymentTestedDangerousLimit(t *testing.T) {
	service := &serviceImpl{scanner: &scannerFake{configured: true, timeout: time.Minute, dangerousLimit: 1024}}
	if err := service.validateDownloadUpload("manual.pdf", maximumDownloadBytes); err != nil {
		t.Fatalf("ordinary maximum was rejected: %v", err)
	}
	if err := service.validateDownloadUpload("archive.7z", 1024); err != nil {
		t.Fatalf("dangerous file within tested limit was rejected: %v", err)
	}
	if err := service.validateDownloadUpload("archive.7z", 1025); !bizerr.Is(err, CodeUploadRejected) {
		t.Fatalf("dangerous file above tested limit returned %v", err)
	}
	service.scanner = &scannerFake{configured: true, notReady: true, timeout: time.Minute, dangerousLimit: 1024}
	if err := service.validateDownloadUpload("archive.zip", 100); !bizerr.Is(err, CodeScannerUnavailable) {
		t.Fatalf("dangerous file bypassed failed scanner self-test: %v", err)
	}
	if err := service.validateDownloadUpload("script.ps1", 100); !bizerr.Is(err, CodeUploadRejected) {
		t.Fatalf("unsupported script returned %v", err)
	}
}
