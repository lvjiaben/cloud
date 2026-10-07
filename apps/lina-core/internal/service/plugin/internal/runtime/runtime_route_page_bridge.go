// This file signs exact isolated-page route grants and verifies them against
// active releases, authenticated actors and authoritative owner boundaries.

package runtime

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/net/ghttp"

	"lina-core/internal/service/datascope"
	"lina-core/internal/service/plugin/internal/catalog"
	"lina-core/pkg/plugin/capability/capmodel"
	bridgecontract "lina-core/pkg/plugin/pluginbridge/contract"
	bridgecodec "lina-core/pkg/plugin/pluginbridge/protocol"
	"lina-core/pkg/plugin/pluginhost"
	"lina-core/pkg/statusflag"
)

const pageBridgeGrantLifetime = 2 * time.Minute

var pageBridgeAPIPathPattern = regexp.MustCompile(`^[A-Za-z0-9_/-]+$`)

type PageBridgeGrantInput struct {
	TokenID              string
	UserID               int32
	TenantID             int32
	PluginID             string
	Host                 string
	SurfaceKey           string
	SurfaceVersion       string
	Capability           string
	Method               string
	Path                 string
	Permissions          []string
	DataScope            datascope.Scope
	DataScopeUnsupported bool
	IsSuperAdmin         bool
	ActingAsTenant       bool
	IsImpersonation      bool
}

type PageBridgeGrantOutput struct {
	Grant         string
	ExpiresAt     int64
	OwnerPluginID string
}

type pageBridgeGrantClaims struct {
	Version                int    `json:"v"`
	ExpiresAt              int64  `json:"exp"`
	PluginID               string `json:"providerPlugin"`
	OwnerPluginID          string `json:"ownerPlugin"`
	ProviderReleaseVersion string `json:"providerReleaseVersion"`
	OwnerReleaseVersion    string `json:"ownerReleaseVersion"`
	Host                   string `json:"host"`
	SurfaceKey             string `json:"surface"`
	SurfaceVersion         string `json:"surfaceVersion"`
	Capability             string `json:"capability"`
	Method                 string `json:"method"`
	Path                   string `json:"path"`
	TokenID                string `json:"tokenId"`
	UserID                 int32  `json:"userId"`
	TenantID               int32  `json:"tenantId"`
}

// IssuePageBridgeGrant validates the active release and signs one exact route
// authorization. Every node derives the HMAC key from the shared JWT secret.
func (s *serviceImpl) IssuePageBridgeGrant(ctx context.Context, input PageBridgeGrantInput) (*PageBridgeGrantOutput, error) {
	input.PluginID = strings.TrimSpace(input.PluginID)
	input.Host = strings.TrimSpace(input.Host)
	input.SurfaceKey = strings.TrimSpace(input.SurfaceKey)
	input.SurfaceVersion = strings.TrimSpace(input.SurfaceVersion)
	input.Capability = strings.TrimSpace(input.Capability)
	input.Method = strings.ToUpper(strings.TrimSpace(input.Method))
	input.Path = normalizePageBridgeAPIPath(input.Path)
	if input.TokenID == "" || input.UserID <= 0 || input.PluginID == "" || input.SurfaceKey == "" || input.SurfaceVersion == "" || input.Capability == "" || input.Path == "" {
		return nil, gerror.New("page bridge grant request is incomplete")
	}
	registry, err := s.storeSvc.GetRegistry(ctx, input.PluginID)
	if err != nil {
		return nil, err
	}
	if registry == nil || registry.Installed != statusflag.Installed.Int() || registry.Status != statusflag.EnabledValue.Int() || registry.ReleaseId <= 0 {
		return nil, gerror.New("dynamic plugin is not enabled")
	}
	manifest, err := s.LoadActiveDynamicPluginManifest(ctx, registry)
	if err != nil {
		return nil, err
	}
	if s.integrationSvc != nil && !s.integrationSvc.CanExposeBusinessEntries(ctx, input.PluginID) {
		return nil, gerror.New("dynamic plugin business entries are unavailable")
	}
	surface := findPageBridgeSurface(manifest, input.Host, input.SurfaceKey, input.SurfaceVersion, input.Capability)
	if surface == nil {
		return nil, gerror.New("page bridge surface capability is not available")
	}
	if err = authorizePageBridgeSurface(input, surface); err != nil {
		return nil, err
	}
	ownerPluginID, ownerVersion, ownerPermission, err := s.resolvePageBridgeOwner(ctx, manifest, input)
	if err != nil {
		return nil, err
	}
	if ownerPermission != "" && !input.IsSuperAdmin && !containsPageBridgePermission(input.Permissions, ownerPermission) {
		return nil, gerror.New("page bridge owner route permission is denied")
	}
	secret := strings.TrimSpace(s.configSvc.GetJwtSecret(ctx))
	if secret == "" {
		return nil, gerror.New("page bridge signing secret is unavailable")
	}
	now := time.Now()
	claims := pageBridgeGrantClaims{
		Version: 1, ExpiresAt: now.Add(pageBridgeGrantLifetime).Unix(),
		PluginID: input.PluginID, OwnerPluginID: ownerPluginID,
		ProviderReleaseVersion: manifest.Version, OwnerReleaseVersion: ownerVersion,
		Host: input.Host, SurfaceKey: input.SurfaceKey,
		SurfaceVersion: input.SurfaceVersion, Capability: input.Capability,
		Method: input.Method, Path: input.Path,
		TokenID: input.TokenID, UserID: input.UserID, TenantID: input.TenantID,
	}
	grant, err := signPageBridgeGrant(secret, claims)
	if err != nil {
		return nil, err
	}
	return &PageBridgeGrantOutput{Grant: grant, ExpiresAt: claims.ExpiresAt * 1000, OwnerPluginID: ownerPluginID}, nil
}

func (s *serviceImpl) resolvePageBridgeOwner(ctx context.Context, provider *catalog.Manifest, input PageBridgeGrantInput) (string, string, string, error) {
	if match := findPageBridgeRoute(provider, input.Method, input.Path); match != nil && strings.TrimSpace(match.Route.Meta[pluginhost.RouteMetaPageBridgeCapability]) == input.Capability {
		// Dynamic self-owned routes retain their existing audience policy. A
		// source owner's proof must never authorize a different dynamic route.
		if input.Host == catalog.FrontendSurfaceHostCustomer && input.DataScope != datascope.ScopeSelf {
			return "", "", "", gerror.New("customer page bridge surface audience is denied")
		}
		return provider.ID, provider.Version, strings.TrimSpace(match.Route.Permission), nil
	}
	if s.integrationSvc == nil {
		return "", "", "", gerror.New("page bridge owner route directory is unavailable")
	}
	var selected *pluginhost.SourceRouteBinding
	for _, binding := range s.integrationSvc.ListSourceRouteBindings() {
		if strings.TrimSpace(binding.PageBridgeCapability) != input.Capability || strings.TrimSpace(binding.PageBridgeHost) != input.Host ||
			strings.ToUpper(strings.TrimSpace(binding.Method)) != input.Method {
			continue
		}
		ownerID := strings.TrimSpace(binding.PluginID)
		prefix := pluginhost.PluginAPINamespacePrefix + "/" + ownerID
		if ownerID == "" || !strings.HasPrefix(binding.Path, prefix+"/") {
			continue
		}
		ownerPath := strings.TrimPrefix(binding.Path, prefix)
		if _, ok := matchDynamicRoutePath(ownerPath, input.Path); !ok {
			continue
		}
		if selected != nil {
			return "", "", "", gerror.New("page bridge owner route is ambiguous")
		}
		selectedBinding := binding
		selected = &selectedBinding
	}
	if selected == nil || !s.integrationSvc.CanExposeBusinessEntries(ctx, selected.PluginID) {
		return "", "", "", gerror.New("page bridge capability is not bound to an available owner route")
	}
	ownerManifest, err := s.catalogSvc.GetDesiredManifest(selected.PluginID)
	if err != nil || ownerManifest == nil || ownerManifest.SourcePlugin == nil || ownerManifest.Type != "source" {
		return "", "", "", gerror.New("page bridge owner release is unavailable")
	}
	if input.Host == catalog.FrontendSurfaceHostCustomer {
		if selected.AuthorizePageBridgeCustomer == nil {
			return "", "", "", gerror.New("customer page bridge owner identity proof is unavailable")
		}
		// Use the original authenticated request context. No synthetic data scope,
		// browser-provided identity or cross-request authorization cache is used.
		customerUserID, authErr := selected.AuthorizePageBridgeCustomer(ctx)
		if authErr != nil || customerUserID <= 0 || customerUserID != int(input.UserID) {
			return "", "", "", gerror.New("customer page bridge owner identity proof is denied")
		}
	}
	return selected.PluginID, ownerManifest.Version, strings.TrimSpace(selected.Permission), nil
}

func findPageBridgeRoute(manifest *catalog.Manifest, method string, actualPath string) *dynamicRouteMatch {
	if manifest == nil {
		return nil
	}
	for _, route := range manifest.Routes {
		if route == nil || strings.ToUpper(strings.TrimSpace(route.Method)) != method {
			continue
		}
		params, ok := matchDynamicRoutePath(route.Path, actualPath)
		if ok {
			return &dynamicRouteMatch{PluginID: manifest.ID, InternalPath: actualPath, Route: route, PathParams: params, Manifest: manifest}
		}
	}
	return nil
}

func normalizePageBridgeAPIPath(value string) string {
	trimmed := strings.Trim(strings.TrimSpace(value), "/")
	if trimmed == "" || strings.Contains(trimmed, "..") || strings.Contains(trimmed, "//") ||
		strings.Contains(trimmed, `\`) || !pageBridgeAPIPathPattern.MatchString(trimmed) {
		return ""
	}
	return "/api/v1/" + trimmed
}

func findPageBridgeSurface(manifest *catalog.Manifest, host, key, version, capability string) *capmodel.FrontendSurfaceDeclaration {
	if manifest == nil {
		return nil
	}
	var surfaces []*capmodel.FrontendSurfaceDeclaration
	if host == "admin" {
		surfaces = manifest.AdminSurfaces
	} else if host == "customer" {
		surfaces = manifest.CustomerSurfaces
	} else {
		return nil
	}
	for _, surface := range surfaces {
		if surface == nil || surface.Key != key || surface.Version != version {
			continue
		}
		for _, declared := range surface.BridgeCapabilities {
			if strings.TrimSpace(declared) == capability {
				return surface
			}
		}
		return nil
	}
	return nil
}

func surfaceDeclaresPageBridgeCapability(manifest *catalog.Manifest, host, key, version, capability string) bool {
	return findPageBridgeSurface(manifest, host, key, version, capability) != nil
}

// authorizePageBridgeSurface proves the current authenticated actor belongs to
// the selected host before signing any browser-selected binding. Administration
// surfaces require their exact declared permission. Customer audience proof is
// completed by resolvePageBridgeOwner before signing: the source owner checks
// its actual customer identity, while dynamic self-owned routes retain ScopeSelf.
func authorizePageBridgeSurface(input PageBridgeGrantInput, surface *capmodel.FrontendSurfaceDeclaration) error {
	representedCustomer := input.IsImpersonation && input.UserID > 0 && !input.ActingAsTenant
	if surface == nil || input.TenantID != 0 || input.ActingAsTenant || input.DataScopeUnsupported {
		return gerror.New("page bridge surface audience is not authorized")
	}
	switch input.Host {
	case catalog.FrontendSurfaceHostAdmin:
		if input.IsImpersonation {
			return gerror.New("administration page bridge surface does not accept impersonation")
		}
		required := strings.TrimSpace(surface.Permission)
		if required == "" || (!input.IsSuperAdmin && !containsPageBridgePermission(input.Permissions, required)) {
			return gerror.New("administration page bridge surface permission is denied")
		}
	case catalog.FrontendSurfaceHostCustomer:
		if input.IsSuperAdmin || (input.IsImpersonation && !representedCustomer) {
			return gerror.New("customer page bridge surface audience is denied")
		}
	default:
		return gerror.New("page bridge surface host is invalid")
	}
	return nil
}

func containsPageBridgePermission(permissions []string, required string) bool {
	for _, permission := range permissions {
		if strings.TrimSpace(permission) == required || strings.TrimSpace(permission) == "*:*:*" {
			return true
		}
	}
	return false
}

func signPageBridgeGrant(secret string, claims pageBridgeGrantClaims) (string, error) {
	payload, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}
	encoded := base64.RawURLEncoding.EncodeToString(payload)
	mac := hmac.New(sha256.New, []byte("linapro-page-bridge-v1:"+secret))
	_, _ = mac.Write([]byte(encoded))
	return encoded + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil)), nil
}

func verifyPageBridgeGrant(secret, token string, now time.Time) (*pageBridgeGrantClaims, bool) {
	parts := strings.Split(token, ".")
	if len(parts) != 2 || secret == "" {
		return nil, false
	}
	mac := hmac.New(sha256.New, []byte("linapro-page-bridge-v1:"+secret))
	_, _ = mac.Write([]byte(parts[0]))
	got, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil || base64.RawURLEncoding.EncodeToString(got) != parts[1] || !hmac.Equal(got, mac.Sum(nil)) {
		return nil, false
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil || base64.RawURLEncoding.EncodeToString(payload) != parts[0] {
		return nil, false
	}
	claims := &pageBridgeGrantClaims{}
	if json.Unmarshal(payload, claims) != nil || claims.Version != 1 || claims.ExpiresAt <= now.Unix() {
		return nil, false
	}
	return claims, true
}

func (s *serviceImpl) authorizePageBridgeRoute(ctx context.Context, state *dynamicRouteRuntimeState, identity *bridgecontract.IdentitySnapshotV1, headers http.Header) *bridgecontract.BridgeResponseEnvelopeV1 {
	if state == nil || state.Match == nil || state.Match.Route == nil || state.Manifest == nil || headers == nil {
		return bridgecodec.NewInternalErrorResponse("Dynamic page bridge state is incomplete")
	}
	routeCapability := strings.TrimSpace(state.Match.Route.Meta[pluginhost.RouteMetaPageBridgeCapability])
	grant := strings.TrimSpace(headers.Get(pluginhost.PageBridgeGrantHeaderName))
	if routeCapability == "" && grant == "" {
		return nil
	}
	claims, ok := verifyPageBridgeGrant(s.configSvc.GetJwtSecret(ctx), grant, time.Now())
	if !ok || identity == nil || routeCapability == "" || claims.PluginID != state.Match.PluginID || claims.OwnerPluginID != state.Match.PluginID ||
		claims.ProviderReleaseVersion != state.Manifest.Version || claims.OwnerReleaseVersion != state.Manifest.Version ||
		claims.TokenID != identity.TokenID || claims.UserID != identity.UserID || claims.TenantID != identity.TenantId ||
		claims.Capability != routeCapability || claims.Method != strings.ToUpper(strings.TrimSpace(state.Match.Route.Method)) ||
		claims.Path != state.Match.InternalPath ||
		!surfaceDeclaresPageBridgeCapability(state.Manifest, claims.Host, claims.SurfaceKey, claims.SurfaceVersion, claims.Capability) {
		return bridgecodec.NewFailureResponse(http.StatusForbidden, "PAGE_BRIDGE_FORBIDDEN", "Page bridge grant is invalid or expired")
	}
	return nil
}

// AuthorizeSourcePageBridgeRouteMiddleware enforces a signed provider-surface
// grant on source-owner routes that opt into the page bridge. Ordinary source
// routes remain governed by their existing authentication and permission chain.
func (s *serviceImpl) AuthorizeSourcePageBridgeRouteMiddleware(request *ghttp.Request) {
	if request == nil {
		return
	}
	binding, ambiguous := s.findSourcePageBridgeBinding(request.Method, request.URL.Path)
	if ambiguous {
		request.Response.WriteStatus(http.StatusForbidden)
		request.ExitAll()
		return
	}
	if binding == nil {
		request.Middleware.Next()
		return
	}
	deny := func() {
		request.Response.WriteStatus(http.StatusForbidden)
		request.ExitAll()
	}
	claims, ok := verifyPageBridgeGrant(s.configSvc.GetJwtSecret(request.Context()), strings.TrimSpace(request.Header.Get(pluginhost.PageBridgeGrantHeaderName)), time.Now())
	businessContext := s.userCtx.Get(request.Context())
	if !ok || businessContext == nil || claims.OwnerPluginID != binding.PluginID ||
		claims.TokenID != businessContext.TokenId || claims.UserID != int32(businessContext.UserId) || claims.TenantID != int32(businessContext.TenantId) ||
		claims.Capability != binding.PageBridgeCapability || claims.Host != binding.PageBridgeHost ||
		claims.Method != strings.ToUpper(strings.TrimSpace(request.Method)) || claims.Path != sourcePageBridgeInternalPath(binding.PluginID, request.URL.Path) {
		deny()
		return
	}
	registry, err := s.storeSvc.GetRegistry(request.Context(), claims.PluginID)
	if err != nil || registry == nil || registry.Installed != statusflag.Installed.Int() || registry.Status != statusflag.EnabledValue.Int() || registry.ReleaseId <= 0 {
		deny()
		return
	}
	provider, err := s.LoadActiveDynamicPluginManifest(request.Context(), registry)
	if err != nil || provider == nil || provider.Version != claims.ProviderReleaseVersion ||
		!surfaceDeclaresPageBridgeCapability(provider, claims.Host, claims.SurfaceKey, claims.SurfaceVersion, claims.Capability) ||
		!s.integrationSvc.CanExposeBusinessEntries(request.Context(), claims.PluginID) ||
		!s.integrationSvc.CanExposeBusinessEntries(request.Context(), claims.OwnerPluginID) {
		deny()
		return
	}
	owner, err := s.catalogSvc.GetDesiredManifest(claims.OwnerPluginID)
	if err != nil || owner == nil || owner.SourcePlugin == nil || owner.Version != claims.OwnerReleaseVersion {
		deny()
		return
	}
	request.Middleware.Next()
}

func (s *serviceImpl) findSourcePageBridgeBinding(method, actualPath string) (*pluginhost.SourceRouteBinding, bool) {
	if s == nil || s.integrationSvc == nil {
		return nil, false
	}
	var selected *pluginhost.SourceRouteBinding
	for _, binding := range s.integrationSvc.ListSourceRouteBindings() {
		if binding.PageBridgeCapability == "" || !strings.EqualFold(strings.TrimSpace(binding.Method), strings.TrimSpace(method)) {
			continue
		}
		if _, ok := matchDynamicRoutePath(binding.Path, actualPath); !ok {
			continue
		}
		if selected != nil {
			return nil, true
		}
		selectedBinding := binding
		selected = &selectedBinding
	}
	return selected, false
}

func sourcePageBridgeInternalPath(ownerPluginID, actualPath string) string {
	prefix := pluginhost.PluginAPINamespacePrefix + "/" + strings.Trim(strings.TrimSpace(ownerPluginID), "/")
	if !strings.HasPrefix(actualPath, prefix+"/") {
		return ""
	}
	return strings.TrimPrefix(actualPath, prefix)
}
