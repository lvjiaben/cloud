package contenthub

import (
	"context"
	"reflect"
	"testing"
	"time"
)

func TestUploadPolicyUsesTheUploadValidationAllowlist(t *testing.T) {
	service := &serviceImpl{scanner: &scannerFake{configured: true, timeout: time.Minute, dangerousLimit: 1024}}
	policy := service.uploadPolicy()
	if policy.OrdinaryMaxSizeBytes != maximumDownloadBytes || policy.InspectionRequiredMaxSizeBytes != 1024 || !policy.InspectionRequiredAvailable {
		t.Fatalf("unexpected upload policy limits: %+v", policy)
	}
	for _, extension := range policy.OrdinaryExtensions {
		if inspectionRequired, allowed := classifyDownloadExtension("file" + extension); !allowed || inspectionRequired {
			t.Fatalf("ordinary extension %q is not accepted as ordinary", extension)
		}
	}
	for _, extension := range policy.InspectionRequiredExtensions {
		if inspectionRequired, allowed := classifyDownloadExtension("file" + extension); !allowed || !inspectionRequired {
			t.Fatalf("inspection-required extension %q is not accepted as inspection-required", extension)
		}
	}
	if _, allowed := classifyDownloadExtension("file.ps1"); allowed {
		t.Fatal("an extension outside the projected allowlist was accepted")
	}
	if !reflect.DeepEqual(policy.OrdinaryExtensions, ordinaryDownloadExtensions[:]) || !reflect.DeepEqual(policy.InspectionRequiredExtensions, inspectionRequiredDownloadExtensions[:]) {
		t.Fatalf("projected extensions do not match validation sources: %+v", policy)
	}
}

func TestUploadPolicyFailsClosedWhenScannerIsNotReady(t *testing.T) {
	service := &serviceImpl{scanner: &scannerFake{configured: true, notReady: true, timeout: time.Minute, dangerousLimit: 1024}}
	policy := service.uploadPolicy()
	if policy.InspectionRequiredAvailable || policy.InspectionRequiredMaxSizeBytes != 0 {
		t.Fatalf("unready scanner leaked an available dangerous upload policy: %+v", policy)
	}

	policy.OrdinaryExtensions[0] = ".changed"
	next := service.uploadPolicy()
	if next.OrdinaryExtensions[0] == ".changed" {
		t.Fatal("caller mutation changed the authoritative allowlist")
	}
}

func TestGetUploadPolicyRequiresAuthorizedStaff(t *testing.T) {
	service := newAuthorizedContentHubTestService(nil)
	service.scanner = &scannerFake{configured: true, timeout: time.Minute, dangerousLimit: 2048}
	policy, err := service.GetUploadPolicy(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !policy.InspectionRequiredAvailable || policy.InspectionRequiredMaxSizeBytes != 2048 {
		t.Fatalf("authorized staff received unexpected policy: %+v", policy)
	}
}
