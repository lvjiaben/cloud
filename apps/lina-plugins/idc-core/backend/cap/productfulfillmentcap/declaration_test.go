package productfulfillmentcap

import (
	"testing"

	"lina-core/pkg/plugin/capability/capmodel"
)

func TestValidateProviderDeclarationsRequiresOneOwnerTypedContract(t *testing.T) {
	valid := testProviderBusinessDeclaration(t)
	if err := ValidateProviderDeclarations([]*capmodel.BusinessExtensionDeclaration{valid}); err != nil {
		t.Fatalf("valid provider declaration was rejected: %v", err)
	}
	if err := ValidateProviderDeclarations(nil); err == nil {
		t.Fatal("missing provider declaration was accepted")
	}
	second := *valid
	second.Version = "v2"
	if err := ValidateProviderDeclarations([]*capmodel.BusinessExtensionDeclaration{valid, &second}); err == nil {
		t.Fatal("multiple contract versions in one provider release were accepted")
	}
}

func TestParseProviderDeclarationRequiresExplicitOngoingManagement(t *testing.T) {
	metadata, err := NewProviderMetadata(testProviderDeclaration())
	if err != nil {
		t.Fatal(err)
	}
	delete(metadata.Values, "ongoingManagement")
	if _, err = ParseProviderDeclaration(metadata, []string{"provision", "query"}); err == nil {
		t.Fatal("provider declaration without ongoing management was accepted")
	}
	metadata, err = NewProviderMetadata(testProviderDeclaration())
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := ParseProviderDeclaration(metadata, []string{"provision", "query"})
	if err != nil || parsed.OngoingManagement {
		t.Fatalf("one-time delivery declaration was not preserved: declaration=%+v err=%v", parsed, err)
	}
	managed := testProviderDeclaration()
	managed.OngoingManagement = true
	managed.EntitlementOwnerID = "idc-resource"
	metadata, err = NewProviderMetadata(managed)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err = ParseProviderDeclaration(metadata, []string{"provision", "query"})
	if err != nil || !parsed.OngoingManagement {
		t.Fatalf("ongoing management declaration was not preserved: declaration=%+v err=%v", parsed, err)
	}
}

func TestParseProviderDeclarationRequiresExplicitRenewalFacts(t *testing.T) {
	metadata, err := NewProviderMetadata(testProviderDeclaration())
	if err != nil {
		t.Fatal(err)
	}
	delete(metadata.Values, "renewalSupport")
	if _, err = ParseProviderDeclaration(metadata, []string{"provision", "query"}); err == nil {
		t.Fatal("provider declaration without renewal support was accepted")
	}
	metadata, err = NewProviderMetadata(testProviderDeclaration())
	if err != nil {
		t.Fatal(err)
	}
	delete(metadata.Values, "expiryManagement")
	if _, err = ParseProviderDeclaration(metadata, []string{"provision", "query"}); err == nil {
		t.Fatal("provider declaration without expiry management was accepted")
	}
}

func TestRenewalModeRequiresExplicitConsistentFacts(t *testing.T) {
	oneTime := testProviderDeclaration()
	if mode, err := RenewalMode(oneTime); err != nil || mode != RenewalModeUnavailable {
		t.Fatalf("one-time renewal mode=%q err=%v", mode, err)
	}
	local := testProviderDeclaration()
	local.OngoingManagement = true
	local.EntitlementOwnerID = "ssl-owner"
	if mode, err := RenewalMode(local); err != nil || mode != RenewalModeLocalEntitlement {
		t.Fatalf("local renewal mode=%q err=%v", mode, err)
	}
	provider := testProviderDeclaration()
	provider.OngoingManagement = true
	provider.EntitlementOwnerID = "idc-resource"
	provider.RenewalSupport = RenewalSupportSupported
	provider.ExpiryManagement = ExpiryManagementProvider
	provider.MethodRoles.Renewal = "renew"
	if mode, err := RenewalMode(provider); err != nil || mode != RenewalModeProviderConfirmed {
		t.Fatalf("provider renewal mode=%q err=%v", mode, err)
	}
	mixed := testProviderDeclaration()
	mixed.OngoingManagement = true
	mixed.EntitlementOwnerID = "idc-resource"
	mixed.ExpiryManagement = ExpiryManagementProvider
	if mode, err := RenewalMode(mixed); err != nil || mode != RenewalModeUnavailable {
		t.Fatalf("mixed renewal mode=%q err=%v", mode, err)
	}
}

func TestValidateProviderDeclarationsRejectsInvalidGranularityAndRoles(t *testing.T) {
	for _, testCase := range []struct {
		name   string
		mutate func(*ProviderDeclaration)
	}{
		{name: "invalid granularity", mutate: func(value *ProviderDeclaration) { value.Granularity = "per_request" }},
		{name: "missing query", mutate: func(value *ProviderDeclaration) { value.MethodRoles.Query = "" }},
		{name: "duplicate roles", mutate: func(value *ProviderDeclaration) { value.MethodRoles.Query = value.MethodRoles.Provision }},
		{name: "unauthorized provision", mutate: func(value *ProviderDeclaration) { value.MethodRoles.Provision = "submit" }},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			typed := testProviderDeclaration()
			testCase.mutate(typed)
			metadata, err := NewProviderMetadata(typed)
			if err != nil {
				t.Fatal(err)
			}
			declaration := testProviderBusinessDeclaration(t)
			declaration.ProviderMetadata = metadata
			if err = ValidateProviderDeclarations([]*capmodel.BusinessExtensionDeclaration{declaration}); err == nil {
				t.Fatal("invalid owner-typed provider declaration was accepted")
			}
		})
	}
}

func TestValidateProviderDeclarationGovernsManagedServiceActions(t *testing.T) {
	declaration := testProviderDeclaration()
	declaration.OngoingManagement = true
	declaration.EntitlementOwnerID = "idc-core"
	declaration.ServiceActions = []ManagedServiceAction{
		{
			Key: "detail", Title: ProviderCapability{EnUS: "Certificate detail", ZhCN: "证书详情"},
			Method: "service.detail", Execution: ServiceActionExecutionProjection,
			Availability: ServiceActionAvailabilityAlways, Audiences: []string{ServiceActionAudienceCustomer, ServiceActionAudienceAdmin},
		},
		{
			Key: "cancel", Title: ProviderCapability{EnUS: "Cancel application", ZhCN: "取消申请"},
			Method: "service.cancel", QueryMethod: "service.sync", Execution: ServiceActionExecutionAsynchronous,
			Availability: ServiceActionAvailabilityBeforeFinal, Audiences: []string{ServiceActionAudienceCustomer, ServiceActionAudienceAdmin},
		},
	}
	authorized := []string{"provision", "query", "service.detail", "service.sync", "service.cancel"}
	if err := ValidateProviderDeclaration(declaration, authorized); err != nil {
		t.Fatalf("valid managed service actions were rejected: %v", err)
	}

	for _, testCase := range []struct {
		name   string
		mutate func(*ProviderDeclaration)
	}{
		{name: "undeclared method", mutate: func(value *ProviderDeclaration) { value.ServiceActions[0].Method = "service.unknown" }},
		{name: "async without query", mutate: func(value *ProviderDeclaration) { value.ServiceActions[1].QueryMethod = "" }},
		{name: "duplicate action", mutate: func(value *ProviderDeclaration) { value.ServiceActions[1].Key = value.ServiceActions[0].Key }},
		{name: "invalid audience", mutate: func(value *ProviderDeclaration) { value.ServiceActions[0].Audiences = []string{"operator"} }},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			copyValue := *declaration
			copyValue.ServiceActions = append([]ManagedServiceAction(nil), declaration.ServiceActions...)
			testCase.mutate(&copyValue)
			if err := ValidateProviderDeclaration(&copyValue, authorized); err == nil {
				t.Fatal("invalid managed service action was accepted")
			}
		})
	}
}

func TestValidateProviderDeclarationGovernsHostOwnedDownloads(t *testing.T) {
	declaration := testProviderDeclaration()
	declaration.OngoingManagement = true
	declaration.EntitlementOwnerID = "idc-core"
	declaration.ServiceDownloads = []ManagedServiceDownload{{
		Key: "certificate", Title: ProviderCapability{EnUS: "Download certificate", ZhCN: "下载证书"},
		Method: "service.download", Path: "/order/download-cert", EndpointField: "endpoint",
		ExternalReferenceField: "order_id", FixedBody: map[string]string{"format": "all"},
		SecretHeaders: map[string]string{"X-API-Key": "api_key", "X-API-Secret": "api_secret"},
		Audiences:     []string{ServiceActionAudienceCustomer, ServiceActionAudienceAdmin},
	}}
	methods := []string{"provision", "query", "service.download"}
	if err := ValidateProviderDeclaration(declaration, methods); err != nil {
		t.Fatal(err)
	}
	mutations := []func(*ProviderDeclaration){
		func(value *ProviderDeclaration) { value.ServiceDownloads[0].Path = "https://supplier.invalid/file" },
		func(value *ProviderDeclaration) {
			value.ServiceDownloads[0].SecretHeaders = map[string]string{"Authorization": "api_key"}
		},
		func(value *ProviderDeclaration) { value.ServiceDownloads[0].Method = "service.not-authorized" },
	}
	for index, mutate := range mutations {
		copyValue := *declaration
		copyValue.ServiceDownloads = append([]ManagedServiceDownload(nil), declaration.ServiceDownloads...)
		copyValue.ServiceDownloads[0].SecretHeaders = map[string]string{"X-API-Key": "api_key", "X-API-Secret": "api_secret"}
		mutate(&copyValue)
		if err := ValidateProviderDeclaration(&copyValue, methods); err == nil {
			t.Fatalf("mutation %d accepted", index)
		}
	}
}

func testProviderBusinessDeclaration(t *testing.T) *capmodel.BusinessExtensionDeclaration {
	t.Helper()
	metadata, err := NewProviderMetadata(testProviderDeclaration())
	if err != nil {
		t.Fatal(err)
	}
	return &capmodel.BusinessExtensionDeclaration{
		Point: capmodel.BusinessExtensionProductFulfillment, Version: "v1", Role: capmodel.BusinessExtensionRoleProvider,
		Owner: "idc-core", Methods: []string{"provision", "query"}, ProviderMetadata: metadata,
	}
}

func testProviderDeclaration() *ProviderDeclaration {
	return &ProviderDeclaration{
		Capability:       ProviderCapability{EnUS: "Automated fulfillment", ZhCN: "自动履约"},
		Granularity:      GranularityPerOrderItem,
		RenewalSupport:   RenewalSupportUnsupported,
		ExpiryManagement: ExpiryManagementUnsupported,
		MethodRoles:      ProviderMethodRoles{Provision: "provision", Query: "query"},
	}
}
