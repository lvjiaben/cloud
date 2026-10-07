package localgatewayclient

import "testing"

func TestValidTencentTemplateListRejectsNegativeSigningCounts(t *testing.T) {
	t.Parallel()
	result := TencentTemplateList{
		SyncedAt: 1,
		Items: []TencentTemplate{{
			TemplateRef: "AbCdEfGhIjKlMnOpQrStUvWxYz012345",
			Name:        "provider template", ProviderState: "active",
			CustomerSignatureCount: -1,
		}},
	}
	if validTencentTemplateList(result) {
		t.Fatal("negative provider signing count was accepted")
	}
}

func TestSafeTencentPreviewURLRequiresHTTPS(t *testing.T) {
	t.Parallel()
	if !safeTencentPreviewURL("https://ess.tencent.example/template-preview") {
		t.Fatal("safe HTTPS preview URL was rejected")
	}
	for _, value := range []string{
		"http://ess.tencent.example/template-preview",
		"https://user@example.com/template-preview",
		"https://ess.tencent.example/template-preview#fragment",
	} {
		if safeTencentPreviewURL(value) {
			t.Fatalf("unsafe preview URL was accepted: %q", value)
		}
	}
}
