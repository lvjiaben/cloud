// This file verifies staff authorization precedes supplier access and rejects
// undeclared/raw fields at the sales projection trust boundary.
package interfacecandidate

import (
	"context"
	"github.com/gogf/gf/v2/database/gdb"
	"lina-core/pkg/plugin/capability/capmodel"
	"lina-plugin-idc-core/backend/cap/channelcommercecap"
	"lina-plugin-idc-core/backend/cap/productfulfillmentcap"
	"lina-plugin-idc-resource/backend/internal/service/interfaceprojection"
	"strings"
	"testing"

	"github.com/gogf/gf/v2/errors/gerror"
	"lina-plugin-idc-core/backend/cap/managementcap"
	"lina-plugin-idc-core/backend/cap/resourceinterfaceprojectioncap"
)

type catalogAuthorizerStub struct {
	calls   int
	actor   int
	deny    bool
	request managementcap.AuthorizationRequest
}

func (a *catalogAuthorizerStub) Authorize(_ context.Context, request managementcap.AuthorizationRequest) (int, error) {
	a.calls++
	a.request = request
	if a.deny {
		return 0, gerror.New("denied")
	}
	return a.actor, nil
}

func TestCatalogSourceChecksStaffAndTargetBeforeUnboundRuntime(t *testing.T) {
	t.Parallel()
	for _, actor := range []int{0, 1} {
		authorizer := &catalogAuthorizerStub{actor: actor}
		database, err := gdb.New(gdb.ConfigNode{Type: "pgsql", Host: "127.0.0.1", Name: "linapro_test_unbound", User: "unused"})
		if err != nil {
			t.Fatal(err)
		}
		projection, err := interfaceprojection.NewWorker(database, &catalogProjectionSink{})
		if err != nil {
			t.Fatal(err)
		}
		source, err := NewCatalogSource(NewLazy(), authorizer, projection)
		if err != nil {
			t.Fatal(err)
		}
		interfaceNo := "pfi_11111111111111111111111111111111"
		if _, err = source.ListCatalog(t.Context(), interfaceNo, resourceinterfaceprojectioncap.CatalogQuery{Page: 1, PageSize: 10}); err == nil {
			t.Fatal("unbound or unauthenticated source accepted")
		}
		if authorizer.calls != 1 || authorizer.request.Permission != catalogReadPermission || authorizer.request.TargetNo != interfaceNo {
			t.Fatalf("incorrect permission boundary: %+v", authorizer)
		}
		if _, err = source.ReadCatalogProduct(t.Context(), "malformed", resourceinterfaceprojectioncap.CatalogProductQuery{Reference: "101"}); err == nil {
			t.Fatal("invalid interface accepted")
		}
		if authorizer.calls != 1 {
			t.Fatal("malformed interface reached authorization or storage")
		}
	}
}

func TestDecodeCatalogResponseRejectsSecretsAndTrailingDocuments(t *testing.T) {
	t.Parallel()
	for _, payload := range []string{
		`{"items":[],"total":0,"password":"secret"}`,
		`{"items":[],"total":0} {"token":"secret"}`,
		`{"items":[{"reference":"1","name":"server","credential":"secret"}],"total":1}`,
	} {
		var page resourceinterfaceprojectioncap.CatalogPage
		if decodeCatalogResponse([]byte(payload), &page) == nil {
			t.Fatal("unsafe supplier result accepted")
		}
	}
	var page resourceinterfaceprojectioncap.CatalogPage
	if err := decodeCatalogResponse([]byte(`{"items":[],"total":0}`), &page); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"ReadEvidence", "readEvidence"} {
		var detail resourceinterfaceprojectioncap.CatalogDetail
		if decodeCatalogResponse([]byte(`{"product":{"reference":"101","name":"server"},"`+key+`":{"InterfaceConfigurationRevision":"rci_forged"}}`), &detail) == nil {
			t.Fatal("guest-supplied owner read evidence was accepted")
		}
	}
}

// catalogProjectionSink records only secret-free admitted owner projections.
type catalogProjectionSink struct {
	values []resourceinterfaceprojectioncap.Projection
}

func (s *catalogProjectionSink) Upsert(_ context.Context, value resourceinterfaceprojectioncap.Projection) error {
	s.values = append(s.values, value)
	return nil
}

func (sink *catalogProjectionSink) RefreshUnused(ctx context.Context, projection resourceinterfaceprojectioncap.Projection, apply func(context.Context) error) error {
	if err := apply(ctx); err != nil {
		return err
	}
	return sink.Upsert(ctx, projection)
}

// TestNativeCatalogSnapshotPinsProductFacingRelease verifies the source owner
// checks the product declaration as well as the resource-facing read release.
func TestNativeCatalogSnapshotPinsProductFacingRelease(t *testing.T) {
	evidence := nativeCatalogProductEvidence(t)
	snapshot := &channelcommercecap.ImportSnapshot{ProtocolVersion: channelcommercecap.ImportPurchaseCycleLimitProtocolVersion, Product: channelcommercecap.CatalogProduct{PublicSlug: "41"}, ConfigurationRevision: strings.Repeat("a", 64), Schema: &capmodel.ConfigurationSchemaDeclaration{Version: "v1", Fields: []*capmodel.ConfigurationFieldDeclaration{}}, Mappings: []channelcommercecap.ImportFieldMapping{}, Currencies: []channelcommercecap.CatalogCurrency{{CurrencyCode: "CNY", Precision: 2}}, Prices: []channelcommercecap.ImportPriceSchedule{{CurrencyCode: "CNY", TermReference: "monthly", ChargeType: "recurring", PeriodUnit: "month", PeriodCount: 1, MinimumPurchaseCycles: 1, Base: []channelcommercecap.ImportPriceBand{{AmountMinor: 1000}}}}, PricingComplete: true, Fulfillment: channelcommercecap.ImportFulfillment{Granularity: productfulfillmentcap.GranularityPerQuantityUnit, OngoingService: true, RenewalMode: productfulfillmentcap.RenewalModeUnavailable, MaximumPurchaseCycles: 1}}
	reader := NewLazy()
	reader.deps = &dependencies{extensions: &catalogReadExtensions{evidence: evidence}}
	read := &resourceinterfaceprojectioncap.CatalogReadEvidence{PluginID: evidence.PluginID, PluginReleaseVersion: evidence.ReleaseVersion, ContractVersion: evidence.ContractVersion, AuthorizedMethods: append([]string(nil), evidence.AuthorizedMethods...), ReleaseDigest: evidence.ReleaseDigest, DeclarationDigest: evidence.DeclarationDigest}
	detail := &resourceinterfaceprojectioncap.CatalogDetail{ImportSnapshot: snapshot}
	if err := reader.validateNativeCatalogSnapshot(t.Context(), read, detail); err != nil {
		t.Fatal(err)
	}
	evidence.DeclarationDigest = strings.Repeat("c", 64)
	if reader.validateNativeCatalogSnapshot(t.Context(), read, detail) == nil {
		t.Fatal("read mixed the resource release with another product declaration")
	}
	evidence.DeclarationDigest = read.DeclarationDigest
	snapshot.Mappings = []channelcommercecap.ImportFieldMapping{{FieldKey: "invented", FieldToken: "config:11"}}
	if reader.validateNativeCatalogSnapshot(t.Context(), read, detail) == nil {
		t.Fatal("source owner admitted an invented field mapping")
	}
}

func nativeCatalogProductEvidence(t *testing.T) *capmodel.BusinessExtensionProviderEvidence {
	t.Helper()
	metadata, err := productfulfillmentcap.NewProviderMetadata(&productfulfillmentcap.ProviderDeclaration{SaleImportProtocol: productfulfillmentcap.SaleImportProtocolNativeFinanceV1, Capability: productfulfillmentcap.ProviderCapability{EnUS: "Native sale", ZhCN: "原生转售"}, EntitlementOwnerID: "idc-resource", Granularity: productfulfillmentcap.GranularityPerQuantityUnit, OngoingManagement: true, RenewalSupport: productfulfillmentcap.RenewalSupportUnsupported, ExpiryManagement: productfulfillmentcap.ExpiryManagementUnsupported, MethodRoles: productfulfillmentcap.ProviderMethodRoles{Provision: "provision", Query: "query"}})
	if err != nil {
		t.Fatal(err)
	}
	return &capmodel.BusinessExtensionProviderEvidence{PluginID: "native-contract", Point: capmodel.BusinessExtensionProductFulfillment, ReleaseVersion: "v1.0.0", ContractVersion: "v1", ReleaseDigest: strings.Repeat("a", 64), DeclarationDigest: strings.Repeat("b", 64), AuthorizedMethods: []string{"provision", "query", "configuration.candidates"}, ProviderMetadata: metadata, ProductConfiguration: &capmodel.ConfigurationSchemaDeclaration{Version: "v1", Fields: []*capmodel.ConfigurationFieldDeclaration{{Key: "upstream_product_id", Type: capmodel.ConfigurationFieldSingleSelect, Required: true, DynamicCandidates: &capmodel.DynamicCandidateDeclaration{Version: "v1", Method: "configuration.candidates"}}, {Key: "upstream_configuration_revision", Type: capmodel.ConfigurationFieldSingleLine, Required: true, MinimumLength: 64, MaximumLength: 64}}}}
}
