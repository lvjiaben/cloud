package purchaseconfigurationfile

import (
	"strings"
	"testing"

	"lina-core/pkg/plugin/capability/capmodel"
)

func TestStrictPolicySnapshotRejectsUnknownAndTrailingJSON(t *testing.T) {
	t.Parallel()
	for _, payload := range []string{
		`{"id":"safe.v1","maximumFiles":1,"maximumFileBytes":10,"maximumTotalBytes":10,"allowedMimeTypes":["application/pdf"],"allowedExtensions":[".pdf"],"safetyScanRequired":false,"script":"unsafe"}`,
		`{"id":"safe.v1","maximumFiles":1,"maximumFileBytes":10,"maximumTotalBytes":10,"allowedMimeTypes":["application/pdf"],"allowedExtensions":[".pdf"],"safetyScanRequired":false} {}`,
	} {
		var policy capmodel.ConfigurationUploadPolicyDeclaration
		if err := strictJSON([]byte(payload), &policy); err == nil {
			t.Fatalf("unsafe policy snapshot was accepted: %s", payload)
		}
	}
}

func TestValidPolicyRejectsUnboundedOrNonCanonicalValues(t *testing.T) {
	t.Parallel()
	valid := func() *capmodel.ConfigurationUploadPolicyDeclaration {
		return &capmodel.ConfigurationUploadPolicyDeclaration{
			ID: "customer.identity.v1", MaximumFiles: 2,
			MaximumFileBytes: 1024, MaximumTotalBytes: 2048,
			AllowedMIMETypes: []string{"application/pdf"}, AllowedExtensions: []string{".pdf"},
		}
	}
	if !validPolicy(valid()) {
		t.Fatal("valid bounded policy was rejected")
	}
	tests := []func(*capmodel.ConfigurationUploadPolicyDeclaration){
		func(policy *capmodel.ConfigurationUploadPolicyDeclaration) {
			policy.MaximumFiles = maximumUploadFiles + 1
		},
		func(policy *capmodel.ConfigurationUploadPolicyDeclaration) {
			policy.MaximumFileBytes = maximumUploadFileBytes + 1
		},
		func(policy *capmodel.ConfigurationUploadPolicyDeclaration) {
			policy.MaximumTotalBytes = maximumUploadTotalBytes + 1
		},
		func(policy *capmodel.ConfigurationUploadPolicyDeclaration) {
			policy.AllowedMIMETypes[0] = "text/html;unsafe=1"
		},
		func(policy *capmodel.ConfigurationUploadPolicyDeclaration) { policy.AllowedExtensions[0] = "../pdf" },
		func(policy *capmodel.ConfigurationUploadPolicyDeclaration) { policy.ID = "Customer.Identity" },
	}
	for index, mutate := range tests {
		policy := valid()
		mutate(policy)
		if validPolicy(policy) {
			t.Errorf("invalid policy %d was accepted: %+v", index, policy)
		}
	}
}

func TestUploadTokenIsOpaqueBoundedAndCaseInsensitiveHexIsNotAccepted(t *testing.T) {
	t.Parallel()
	token, err := newUploadToken(strings.NewReader("0123456789abcdef"))
	if err != nil || !validUploadToken(token) {
		t.Fatalf("token=%q err=%v", token, err)
	}
	if validUploadToken(strings.ToLower(token)) || validUploadToken(token+"0") || validUploadToken("../"+token) {
		t.Fatal("unsafe token form was accepted")
	}
}
