// This file verifies exact page bridge grants, active-release ownership and
// authoritative customer proof without sharing identities between requests.

package runtime

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	configsvc "lina-core/internal/service/config"
	"lina-core/internal/service/datascope"
	"lina-core/internal/service/plugin/internal/catalog"
	"lina-core/internal/service/plugin/internal/store"
	"lina-core/pkg/plugin/capability/capmodel"
	bridgecontract "lina-core/pkg/plugin/pluginbridge/contract"
	"lina-core/pkg/plugin/pluginhost"
)

func TestAuthorizePageBridgeRouteRequiresUntamperedSignedExactGrant(t *testing.T) {
	const secret = "page-bridge-test-secret"
	service := &serviceImpl{configSvc: routeTestJwtConfig{Service: configsvc.New(), secret: secret}}
	state := pageBridgeTestState()
	claims := pageBridgeGrantClaims{
		Version: 1, ExpiresAt: time.Now().Add(time.Minute).Unix(), PluginID: "phone-provider",
		OwnerPluginID: "phone-provider", ProviderReleaseVersion: "1.0.0", OwnerReleaseVersion: "1.0.0",
		Host: "customer", SurfaceKey: "phone-orders", SurfaceVersion: "v1",
		Capability: "phone-orders.query-own", Method: http.MethodGet, Path: "/api/v1/phone-orders",
		TokenID: "session-1", UserID: 42, TenantID: 7,
	}
	identity := &bridgecontract.IdentitySnapshotV1{TokenID: "session-1", UserID: 42, TenantId: 7}
	grant, err := signPageBridgeGrant(secret, claims)
	if err != nil {
		t.Fatal(err)
	}
	validHeaders := http.Header{pluginhost.PageBridgeGrantHeaderName: []string{grant}}
	if failure := service.authorizePageBridgeRoute(context.Background(), state, identity, validHeaders); failure != nil {
		t.Fatalf("expected signed exact grant to pass, got %#v", failure)
	}

	for name, mutate := range map[string]func(*pageBridgeGrantClaims){
		"wrong capability": func(value *pageBridgeGrantClaims) { value.Capability = "phone-orders.update" },
		"wrong plugin":     func(value *pageBridgeGrantClaims) { value.PluginID = "ssl-provider" },
		"wrong surface":    func(value *pageBridgeGrantClaims) { value.SurfaceKey = "other" },
		"wrong version":    func(value *pageBridgeGrantClaims) { value.SurfaceVersion = "v2" },
		"wrong host":       func(value *pageBridgeGrantClaims) { value.Host = "admin" },
		"wrong method":     func(value *pageBridgeGrantClaims) { value.Method = http.MethodPost },
		"wrong path":       func(value *pageBridgeGrantClaims) { value.Path = "/api/v1/phone-orders/delete" },
		"wrong session":    func(value *pageBridgeGrantClaims) { value.TokenID = "session-2" },
		"wrong user":       func(value *pageBridgeGrantClaims) { value.UserID = 99 },
		"wrong tenant":     func(value *pageBridgeGrantClaims) { value.TenantID = 8 },
		"expired":          func(value *pageBridgeGrantClaims) { value.ExpiresAt = time.Now().Add(-time.Minute).Unix() },
	} {
		t.Run(name, func(t *testing.T) {
			changed := claims
			mutate(&changed)
			token, signErr := signPageBridgeGrant(secret, changed)
			if signErr != nil {
				t.Fatal(signErr)
			}
			headers := http.Header{pluginhost.PageBridgeGrantHeaderName: []string{token}}
			if failure := service.authorizePageBridgeRoute(context.Background(), state, identity, headers); failure == nil || failure.StatusCode != http.StatusForbidden {
				t.Fatalf("expected forbidden page bridge request, got %#v", failure)
			}
		})
	}

	replacement := "A"
	if strings.HasSuffix(grant, replacement) {
		replacement = "B"
	}
	tampered := grant[:len(grant)-1] + replacement
	if failure := service.authorizePageBridgeRoute(context.Background(), state, identity, http.Header{pluginhost.PageBridgeGrantHeaderName: []string{tampered}}); failure == nil {
		t.Fatal("expected a tampered grant to be rejected")
	}
}

func TestVerifyPageBridgeGrantRejectsNonCanonicalBase64URLSegments(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	token, err := signPageBridgeGrant("canonical-secret", pageBridgeGrantClaims{Version: 1, ExpiresAt: now.Add(time.Minute).Unix()})
	if err != nil {
		t.Fatal(err)
	}
	parts := strings.Split(token, ".")
	for index, name := range []string{"payload", "signature"} {
		t.Run(name, func(t *testing.T) {
			mutated := append([]string(nil), parts...)
			mutated[index] = equivalentNonCanonicalBase64URL(t, parts[index])
			if _, ok := verifyPageBridgeGrant("canonical-secret", strings.Join(mutated, "."), now); ok {
				t.Fatalf("non-canonical %s segment was accepted", name)
			}
		})
	}
}

func equivalentNonCanonicalBase64URL(t *testing.T, canonical string) string {
	t.Helper()
	expected, err := base64.RawURLEncoding.DecodeString(canonical)
	if err != nil {
		t.Fatal(err)
	}
	alphabet := "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_"
	for _, candidate := range alphabet {
		mutated := canonical[:len(canonical)-1] + string(candidate)
		if mutated == canonical {
			continue
		}
		decoded, decodeErr := base64.RawURLEncoding.DecodeString(mutated)
		if decodeErr == nil && bytes.Equal(decoded, expected) {
			return mutated
		}
	}
	t.Fatal("test segment has no equivalent non-canonical encoding")
	return ""
}

func TestIssuePageBridgeGrantUsesActiveReleaseAndAuthorizesExactSurfaceAudience(t *testing.T) {
	activeManifest := &catalog.Manifest{
		ID: "phone-provider", Version: "1.0.0", Type: "dynamic",
		AdminSurfaces: []*capmodel.FrontendSurfaceDeclaration{{
			Key: "phone-admin", Version: "v1", Permission: "phone-provider:orders:view",
			BridgeCapabilities: []string{"phone-orders.admin-query"},
		}},
		CustomerSurfaces: []*capmodel.FrontendSurfaceDeclaration{{
			Key: "phone-orders", Version: "v1",
			BridgeCapabilities: []string{"phone-orders.query-own"},
		}},
		Routes: []*bridgecontract.RouteContract{
			{Path: "/api/v1/admin/orders", Method: http.MethodGet, Access: bridgecontract.AccessLogin, Permission: "phone-provider:orders:view", Meta: map[string]string{pluginhost.RouteMetaPageBridgeCapability: "phone-orders.admin-query"}},
			{Path: "/api/v1/phone-orders", Method: http.MethodGet, Access: bridgecontract.AccessLogin, Meta: map[string]string{pluginhost.RouteMetaPageBridgeCapability: "phone-orders.query-own"}},
		},
	}
	activeRelease := &store.ReleaseRecord{Id: 7, PluginId: "phone-provider", ReleaseVersion: "1.0.0", Type: "dynamic"}
	storeFake := &pageBridgeGrantStore{
		registry:       &store.PluginRecord{PluginId: "phone-provider", Type: "dynamic", Installed: 1, Status: 1, ReleaseId: activeRelease.Id},
		activeRelease:  activeRelease,
		activeManifest: activeManifest,
		stagedManifest: &catalog.Manifest{ID: "phone-provider", Version: "2.0.0", Type: "dynamic"},
	}
	service := &serviceImpl{
		storeSvc:  storeFake,
		configSvc: routeTestJwtConfig{Service: configsvc.New(), secret: "page-bridge-issue-secret"},
	}
	base := PageBridgeGrantInput{
		TokenID: "session-1", UserID: 42, TenantID: 0, PluginID: "phone-provider",
		Host: "customer", SurfaceKey: "phone-orders", SurfaceVersion: "v1",
		Capability: "phone-orders.query-own", Method: http.MethodGet, Path: "phone-orders",
		DataScope: datascope.ScopeSelf,
	}
	grant, err := service.IssuePageBridgeGrant(context.Background(), base)
	if err != nil || grant == nil || grant.Grant == "" {
		t.Fatalf("expected current-customer grant from active release, grant=%#v err=%v", grant, err)
	}
	if grant.ExpiresAt <= time.Now().UnixMilli() || grant.ExpiresAt > time.Now().Add(pageBridgeGrantLifetime+time.Second).UnixMilli() {
		t.Fatalf("page bridge grant expiry is not a bounded Unix millisecond timestamp: %d", grant.ExpiresAt)
	}
	if storeFake.loadedReleaseID != activeRelease.Id || storeFake.loadedStaged {
		t.Fatalf("expected only active release %d to be loaded, active=%d staged=%v", activeRelease.Id, storeFake.loadedReleaseID, storeFake.loadedStaged)
	}

	admin := base
	admin.Host = "admin"
	admin.SurfaceKey = "phone-admin"
	admin.Capability = "phone-orders.admin-query"
	admin.Path = "admin/orders"
	admin.DataScope = datascope.ScopeAll
	admin.Permissions = []string{"phone-provider:orders:view"}
	if _, err = service.IssuePageBridgeGrant(context.Background(), admin); err != nil {
		t.Fatalf("expected exact administration surface permission to pass: %v", err)
	}
	representedCustomer := base
	representedCustomer.IsImpersonation = true
	if _, err = service.IssuePageBridgeGrant(context.Background(), representedCustomer); err != nil {
		t.Fatalf("expected represented-customer page bridge grant to pass: %v", err)
	}

	for name, mutate := range map[string]func(*PageBridgeGrantInput){
		"admin permission missing": func(value *PageBridgeGrantInput) { value.Permissions = nil },
		"wrong surface":            func(value *PageBridgeGrantInput) { value.SurfaceKey = "staged-phone-admin" },
		"wrong route metadata":     func(value *PageBridgeGrantInput) { value.Capability = "phone-orders.delete" },
		"customer uses staff scope": func(value *PageBridgeGrantInput) {
			value.Host = "customer"
			value.SurfaceKey = "phone-orders"
			value.Capability = "phone-orders.query-own"
			value.Path = "phone-orders"
			value.DataScope = datascope.ScopeAll
		},
		"dynamic self route still rejects no scope": func(value *PageBridgeGrantInput) {
			value.Host = "customer"
			value.SurfaceKey = "phone-orders"
			value.Capability = "phone-orders.query-own"
			value.Path = "phone-orders"
			value.DataScope = datascope.ScopeNone
		},
		"tenant impersonation": func(value *PageBridgeGrantInput) {
			value.Host = "customer"
			value.SurfaceKey = "phone-orders"
			value.Capability = "phone-orders.query-own"
			value.Path = "phone-orders"
			value.DataScope = datascope.ScopeSelf
			value.IsImpersonation = true
			value.ActingAsTenant = true
		},
		"non-default tenant": func(value *PageBridgeGrantInput) { value.TenantID = 9 },
	} {
		t.Run(name, func(t *testing.T) {
			input := admin
			mutate(&input)
			if _, issueErr := service.IssuePageBridgeGrant(context.Background(), input); issueErr == nil {
				t.Fatal("expected page bridge grant issuance to fail closed")
			}
		})
	}
}

func TestIssuePageBridgeGrantResolvesExactSourceOwnerWithoutBrowserOwnerInput(t *testing.T) {
	provider := &catalog.Manifest{ID: "yunsu-ssl", Version: "1.2.0", Type: "dynamic", CustomerSurfaces: []*capmodel.FrontendSurfaceDeclaration{{Key: "certificates", Version: "v1", BridgeCapabilities: []string{"service-entitlements.customer.v1"}}}}
	storeFake := &pageBridgeGrantStore{registry: &store.PluginRecord{PluginId: provider.ID, Type: "dynamic", Installed: 1, Status: 1, ReleaseId: 9}, activeRelease: &store.ReleaseRecord{Id: 9, PluginId: provider.ID, ReleaseVersion: provider.Version, Type: "dynamic"}, activeManifest: provider}
	owner := &catalog.Manifest{ID: "idc-core", Version: "0.14.0", Type: "source", SourcePlugin: pluginhost.NewDeclarations("idc-core").(pluginhost.SourcePluginDefinition)}
	var (
		proofCalls                int
		proofUserID               = 51
		proofErr                  error
		proofContext, cancelProof = context.WithCancel(context.Background())
		proof                     = func(ctx context.Context) (int, error) {
			proofCalls++
			if ctx != proofContext {
				t.Fatal("customer proof did not receive the original context")
			}
			return proofUserID, proofErr
		}
	)
	defer cancelProof()
	integration := &pageBridgeSourceIntegration{bindings: []pluginhost.SourceRouteBinding{{PluginID: "idc-core", Method: http.MethodGet, Path: "/x/idc-core/api/v1/account/service-entitlements/{entitlementNo}", PageBridgeCapability: "service-entitlements.customer.v1", PageBridgeHost: "customer", AuthorizePageBridgeCustomer: proof}}, exposed: true}
	service := &serviceImpl{storeSvc: storeFake, catalogSvc: &pageBridgeCatalog{manifest: owner}, integrationSvc: integration, configSvc: routeTestJwtConfig{Service: configsvc.New(), secret: "source-owner-secret"}}
	input := PageBridgeGrantInput{TokenID: "session-ssl", UserID: 51, PluginID: provider.ID, Host: "customer", SurfaceKey: "certificates", SurfaceVersion: "v1", Capability: "service-entitlements.customer.v1", Method: http.MethodGet, Path: "account/service-entitlements/ent_001", DataScope: datascope.ScopeNone}
	out, err := service.IssuePageBridgeGrant(proofContext, input)
	if err != nil || out == nil || out.OwnerPluginID != "idc-core" {
		t.Fatalf("expected server-resolved source owner, out=%#v err=%v", out, err)
	}
	claims, ok := verifyPageBridgeGrant("source-owner-secret", out.Grant, time.Now())
	if !ok || claims.PluginID != "yunsu-ssl" || claims.OwnerPluginID != "idc-core" || claims.ProviderReleaseVersion != "1.2.0" || claims.OwnerReleaseVersion != "0.14.0" {
		t.Fatalf("unexpected cross-owner claims: %#v", claims)
	}
	if proofCalls != 1 {
		t.Fatalf("expected one owner proof, got %d", proofCalls)
	}
	assistance := input
	assistance.IsImpersonation = true
	if _, err = service.IssuePageBridgeGrant(proofContext, assistance); err != nil || proofCalls != 2 {
		t.Fatalf("valid represented customer should be proved afresh: %v", err)
	}
	for _, testCase := range []struct {
		name   string
		userID int
		err    error
	}{
		{name: "unbound or staff identity", userID: 0},
		{name: "different customer", userID: 52},
		{name: "disabled customer", userID: 51, err: errors.New("customer is inactive")},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			proofUserID, proofErr = testCase.userID, testCase.err
			before := proofCalls
			if _, issueErr := service.IssuePageBridgeGrant(proofContext, input); issueErr == nil || proofCalls != before+1 {
				t.Fatal("grant reused an earlier customer proof or accepted a rejected identity")
			}
		})
	}
	proofUserID, proofErr = 51, nil
	integration.bindings[0].AuthorizePageBridgeCustomer = nil
	if _, issueErr := service.IssuePageBridgeGrant(proofContext, input); issueErr == nil {
		t.Fatal("source owner without an identity proof was accepted")
	}
	integration.bindings[0].AuthorizePageBridgeCustomer = proof
	for name, mutate := range map[string]func(*PageBridgeGrantInput){
		"wrong host":        func(value *PageBridgeGrantInput) { value.Host = "admin" },
		"wrong capability":  func(value *PageBridgeGrantInput) { value.Capability = "service-entitlements.admin.v1" },
		"wrong method":      func(value *PageBridgeGrantInput) { value.Method = http.MethodPost },
		"wrong path":        func(value *PageBridgeGrantInput) { value.Path = "account/other" },
		"anonymous":         func(value *PageBridgeGrantInput) { value.UserID = 0 },
		"super admin":       func(value *PageBridgeGrantInput) { value.IsSuperAdmin = true },
		"tenant":            func(value *PageBridgeGrantInput) { value.TenantID = 1 },
		"acting tenant":     func(value *PageBridgeGrantInput) { value.ActingAsTenant = true },
		"unsupported scope": func(value *PageBridgeGrantInput) { value.DataScopeUnsupported = true },
	} {
		t.Run(name, func(t *testing.T) {
			changed := input
			mutate(&changed)
			before := proofCalls
			if _, issueErr := service.IssuePageBridgeGrant(context.Background(), changed); issueErr == nil {
				t.Fatal("cross-owner grant unexpectedly issued")
			}
			if proofCalls != before {
				t.Fatal("an invalid binding reached the customer authorizer")
			}
		})
	}
	integration.exposed = false
	if _, err = service.IssuePageBridgeGrant(context.Background(), input); err == nil {
		t.Fatal("disabled provider/owner business entries unexpectedly issued a grant")
	}
}

type pageBridgeCatalog struct {
	catalog.Service
	manifest *catalog.Manifest
}

func (c *pageBridgeCatalog) GetDesiredManifest(pluginID string) (*catalog.Manifest, error) {
	if c.manifest != nil && c.manifest.ID == pluginID {
		return c.manifest, nil
	}
	return nil, nil
}

type pageBridgeSourceIntegration struct {
	IntegrationService
	bindings []pluginhost.SourceRouteBinding
	exposed  bool
}

func (i *pageBridgeSourceIntegration) ListSourceRouteBindings() []pluginhost.SourceRouteBinding {
	return pluginhost.CloneSourceRouteBindings(i.bindings)
}

func (i *pageBridgeSourceIntegration) CanExposeBusinessEntries(context.Context, string) bool {
	return i.exposed
}

type pageBridgeGrantStore struct {
	store.Service
	registry        *store.PluginRecord
	activeRelease   *store.ReleaseRecord
	activeManifest  *catalog.Manifest
	stagedManifest  *catalog.Manifest
	loadedReleaseID int
	loadedStaged    bool
}

func (s *pageBridgeGrantStore) GetRegistry(context.Context, string) (*store.PluginRecord, error) {
	return s.registry, nil
}

func (s *pageBridgeGrantStore) GetRegistryRelease(context.Context, *store.PluginRecord) (*store.ReleaseRecord, error) {
	return s.activeRelease, nil
}

func (s *pageBridgeGrantStore) LoadReleaseManifest(_ context.Context, release *store.ReleaseRecord) (*catalog.Manifest, error) {
	s.loadedReleaseID = release.Id
	if release.Id != s.activeRelease.Id {
		s.loadedStaged = true
		return s.stagedManifest, nil
	}
	return s.activeManifest, nil
}

func TestAuthorizePageBridgeRouteRequiresGrantForBridgeOnlyRoute(t *testing.T) {
	service := &serviceImpl{configSvc: routeTestJwtConfig{Service: configsvc.New(), secret: "test-secret"}}
	identity := &bridgecontract.IdentitySnapshotV1{TokenID: "session-1", UserID: 42, TenantId: 7}
	if failure := service.authorizePageBridgeRoute(context.Background(), pageBridgeTestState(), identity, http.Header{}); failure == nil || failure.StatusCode != http.StatusForbidden {
		t.Fatalf("expected missing signed grant to be forbidden, got %#v", failure)
	}
}

func pageBridgeTestState() *dynamicRouteRuntimeState {
	return &dynamicRouteRuntimeState{
		Manifest: &catalog.Manifest{ID: "phone-provider", Version: "1.0.0", CustomerSurfaces: []*capmodel.FrontendSurfaceDeclaration{{
			Key: "phone-orders", Version: "v1", BridgeCapabilities: []string{"phone-orders.query-own"},
		}}},
		Match: &dynamicRouteMatch{
			PluginID: "phone-provider", InternalPath: "/api/v1/phone-orders",
			Route: &bridgecontract.RouteContract{
				Path: "/api/v1/phone-orders", Method: http.MethodGet,
				Meta: map[string]string{pluginhost.RouteMetaPageBridgeCapability: "phone-orders.query-own"},
			},
		},
	}
}
