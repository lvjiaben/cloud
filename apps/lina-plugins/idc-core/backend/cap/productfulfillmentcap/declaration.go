// This file defines the owner-validated provider declaration frozen in plugin metadata.

package productfulfillmentcap

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"regexp"
	"strings"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/plugin/capability/capmodel"
)

var entitlementOwnerIDPattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)
var managedServiceActionKeyPattern = regexp.MustCompile(`^[a-z][a-z0-9]*(?:[._-][a-z0-9]+)*$`)
var managedServiceProjectionFieldPattern = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_]{0,63}$`)

const (
	// ProviderMetadataType identifies the owner-defined fulfillment declaration.
	ProviderMetadataType = "product-fulfillment-contract"
	// ProviderMetadataVersion is the current declaration schema version.
	ProviderMetadataVersion = "v1"
	// GranularityPerOrderItem creates one fulfillment task per order item.
	GranularityPerOrderItem = "per_order_item"
	// GranularityPerQuantityUnit creates one task per purchased unit.
	GranularityPerQuantityUnit = "per_quantity_unit"
	// RenewalSupportSupported declares that the frozen provider release accepts renewal tasks.
	RenewalSupportSupported = "supported"
	// RenewalSupportUnsupported declares that the provider does not accept renewal tasks.
	RenewalSupportUnsupported = "unsupported"
	// ExpiryManagementProvider declares that the provider owns the external expiry date.
	ExpiryManagementProvider = "provider"
	// ExpiryManagementUnsupported declares that the provider has no external expiry date to manage.
	ExpiryManagementUnsupported = "unsupported"
	// RenewalModeProviderConfirmed advances entitlement only after a provider task succeeds.
	RenewalModeProviderConfirmed = "provider_confirmed"
	// RenewalModeLocalEntitlement advances only the local entitlement after settlement.
	RenewalModeLocalEntitlement = "local_entitlement"
	// RenewalModeUnavailable keeps an ongoing entitlement visible but closes renewal entry points.
	RenewalModeUnavailable = "unavailable"
	// ServiceActionExecutionProjection allows one read-only, bounded synchronous projection.
	ServiceActionExecutionProjection = "synchronous_projection"
	// ServiceActionExecutionAsynchronous requires owner-persisted asynchronous execution.
	ServiceActionExecutionAsynchronous = "persistent_async"
	// ServiceActionAvailabilityAlways keeps a read-only action available for the entitlement lifetime.
	ServiceActionAvailabilityAlways = "always"
	// ServiceActionAvailabilityBeforeFinal limits a mutation to provider states before final completion.
	ServiceActionAvailabilityBeforeFinal = "before_final"
	// ServiceActionAudienceCustomer allows the current owning customer to request an action.
	ServiceActionAudienceCustomer = "customer"
	// ServiceActionAudienceAdmin allows an authorized and data-scoped employee to request an action.
	ServiceActionAudienceAdmin = "admin"
)

// ProviderCapability is the bounded user-readable provider description.
type ProviderCapability struct {
	EnUS string `json:"enUS"`
	ZhCN string `json:"zhCN"`
}

// ProviderMethodRoles maps fulfillment semantics to authorized provider methods.
type ProviderMethodRoles struct {
	Provision string `json:"provision"`
	Query     string `json:"query"`
	Renewal   string `json:"renewal,omitempty"`
}

// ManagedServiceAction declares one owner-governed action on an existing long-lived entitlement.
// It carries no customer identity, supplier route, credentials, or executable configuration.
type ManagedServiceAction struct {
	Key                  string             `json:"key"`
	Title                ProviderCapability `json:"title"`
	Method               string             `json:"method"`
	QueryMethod          string             `json:"queryMethod,omitempty"`
	Execution            string             `json:"execution"`
	Availability         string             `json:"availability"`
	ConfirmedEntitlement string             `json:"confirmedEntitlement,omitempty"`
	Audiences            []string           `json:"audiences"`
}

// ManagedServiceDownload declares one host-owned supplier download. The host
// performs this request directly; neither the request credentials nor response
// bytes are delivered to the managed guest.
type ManagedServiceDownload struct {
	Key                    string             `json:"key"`
	Title                  ProviderCapability `json:"title"`
	Method                 string             `json:"method"`
	Path                   string             `json:"path"`
	EndpointField          string             `json:"endpointField"`
	ExternalReferenceField string             `json:"externalReferenceField"`
	FixedBody              map[string]string  `json:"fixedBody,omitempty"`
	SecretHeaders          map[string]string  `json:"secretHeaders"`
	Audiences              []string           `json:"audiences"`
}

// ManagedServiceProjection maps a provider's bounded existing-service facts
// into the common directory without exposing the full customer configuration.
type ManagedServiceProjection struct {
	PrimaryReferenceCustomerField string `json:"primaryReferenceCustomerField"`
	ValidationMethodCustomerField string `json:"validationMethodCustomerField"`
	StatusField                   string `json:"statusField"`
	IssuedAtField                 string `json:"issuedAtField"`
	ExpiresAtField                string `json:"expiresAtField"`
	ValidationField               string `json:"validationField"`
}

// ProviderDeclaration is the typed contract owned by idc-core.
type ProviderDeclaration struct {
	Capability         ProviderCapability        `json:"capability"`
	Granularity        string                    `json:"granularity"`
	OngoingManagement  bool                      `json:"ongoingManagement"`
	RenewalSupport     string                    `json:"renewalSupport"`
	ExpiryManagement   string                    `json:"expiryManagement"`
	EntitlementOwnerID string                    `json:"entitlementOwnerId"`
	MethodRoles        ProviderMethodRoles       `json:"methodRoles"`
	ServiceActions     []ManagedServiceAction    `json:"serviceActions,omitempty"`
	ServiceDownloads   []ManagedServiceDownload  `json:"serviceDownloads,omitempty"`
	ServiceProjection  *ManagedServiceProjection `json:"serviceProjection,omitempty"`
}

// ParseProviderDeclaration decodes and validates one frozen opaque metadata value.
func ParseProviderDeclaration(
	metadata *capmodel.BusinessExtensionProviderMetadata,
	authorizedMethods []string,
) (*ProviderDeclaration, error) {
	if metadata == nil || metadata.Type != ProviderMetadataType || metadata.Version != ProviderMetadataVersion || metadata.Values == nil {
		return nil, gerror.New("product fulfillment provider metadata is missing or uses an unsupported schema")
	}
	if _, declared := metadata.Values["ongoingManagement"]; !declared {
		return nil, gerror.New("product fulfillment provider metadata must explicitly declare ongoing management")
	}
	if _, declared := metadata.Values["renewalSupport"]; !declared {
		return nil, gerror.New("product fulfillment provider metadata must explicitly declare renewal support")
	}
	if _, declared := metadata.Values["expiryManagement"]; !declared {
		return nil, gerror.New("product fulfillment provider metadata must explicitly declare expiry management")
	}
	if _, declared := metadata.Values["entitlementOwnerId"]; !declared {
		return nil, gerror.New("product fulfillment provider metadata must explicitly declare its entitlement owner")
	}
	content, err := json.Marshal(metadata.Values)
	if err != nil {
		return nil, gerror.Wrap(err, "encode product fulfillment provider metadata")
	}
	decoder := json.NewDecoder(bytes.NewReader(content))
	decoder.DisallowUnknownFields()
	var declaration ProviderDeclaration
	if err = decoder.Decode(&declaration); err != nil {
		return nil, gerror.Wrap(err, "decode product fulfillment provider metadata")
	}
	if err = decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return nil, gerror.New("product fulfillment provider metadata contains trailing values")
	}
	if err = ValidateProviderDeclaration(&declaration, authorizedMethods); err != nil {
		return nil, err
	}
	return &declaration, nil
}

// ValidateProviderDeclaration enforces the owner-owned fulfillment contract.
func ValidateProviderDeclaration(declaration *ProviderDeclaration, authorizedMethods []string) error {
	if declaration == nil || declaration.Capability.EnUS == "" || declaration.Capability.ZhCN == "" ||
		declaration.Capability.EnUS != strings.TrimSpace(declaration.Capability.EnUS) ||
		declaration.Capability.ZhCN != strings.TrimSpace(declaration.Capability.ZhCN) ||
		len([]rune(declaration.Capability.EnUS)) > 256 || len([]rune(declaration.Capability.ZhCN)) > 256 {
		return gerror.New("product fulfillment capability requires bounded English and Chinese text")
	}
	if declaration.Granularity != GranularityPerOrderItem && declaration.Granularity != GranularityPerQuantityUnit {
		return gerror.New("product fulfillment granularity is invalid")
	}
	if declaration.MethodRoles.Provision == "" || declaration.MethodRoles.Query == "" ||
		declaration.MethodRoles.Provision == declaration.MethodRoles.Query {
		return gerror.New("product fulfillment provision and query roles require distinct methods")
	}
	allowed := make(map[string]struct{}, len(authorizedMethods))
	for _, method := range authorizedMethods {
		allowed[method] = struct{}{}
	}
	if _, ok := allowed[declaration.MethodRoles.Provision]; !ok {
		return gerror.New("product fulfillment provision role is not authorized")
	}
	if _, ok := allowed[declaration.MethodRoles.Query]; !ok {
		return gerror.New("product fulfillment query role is not authorized")
	}
	mode, err := RenewalMode(declaration)
	if err != nil {
		return err
	}
	if mode == RenewalModeProviderConfirmed {
		if declaration.MethodRoles.Renewal == "" || declaration.MethodRoles.Renewal == declaration.MethodRoles.Provision ||
			declaration.MethodRoles.Renewal == declaration.MethodRoles.Query {
			return gerror.New("provider-confirmed renewal requires one distinct method")
		}
		if _, ok := allowed[declaration.MethodRoles.Renewal]; !ok {
			return gerror.New("product fulfillment renewal role is not authorized")
		}
	} else if declaration.MethodRoles.Renewal != "" {
		return gerror.New("non-provider renewal must not declare a renewal method")
	}
	if declaration.OngoingManagement {
		if len(declaration.EntitlementOwnerID) > 64 || !entitlementOwnerIDPattern.MatchString(declaration.EntitlementOwnerID) {
			return gerror.New("ongoing fulfillment requires one valid entitlement owner")
		}
	} else if declaration.EntitlementOwnerID != "" {
		return gerror.New("one-time fulfillment must not declare an entitlement owner")
	}
	if err := validateManagedServiceActions(declaration, allowed); err != nil {
		return err
	}
	if err := validateManagedServiceDownloads(declaration, allowed); err != nil {
		return err
	}
	if declaration.ServiceProjection != nil {
		values := []string{declaration.ServiceProjection.PrimaryReferenceCustomerField, declaration.ServiceProjection.ValidationMethodCustomerField, declaration.ServiceProjection.StatusField, declaration.ServiceProjection.IssuedAtField, declaration.ServiceProjection.ExpiresAtField, declaration.ServiceProjection.ValidationField}
		for _, value := range values {
			if !managedServiceProjectionFieldPattern.MatchString(value) {
				return gerror.New("product fulfillment managed service projection is invalid")
			}
		}
	}
	return nil
}

func validateManagedServiceDownloads(declaration *ProviderDeclaration, allowed map[string]struct{}) error {
	if !declaration.OngoingManagement && len(declaration.ServiceDownloads) > 0 || len(declaration.ServiceDownloads) > 8 {
		return gerror.New("product fulfillment managed service download count is invalid")
	}
	seen := make(map[string]struct{}, len(declaration.ServiceDownloads))
	for _, download := range declaration.ServiceDownloads {
		if err := validateManagedServiceDownload(declaration, download, allowed); err != nil {
			return err
		}
		if _, duplicate := seen[download.Key]; duplicate {
			return gerror.New("product fulfillment managed service download is duplicated")
		}
		seen[download.Key] = struct{}{}
	}
	return nil
}

func validateManagedServiceDownload(declaration *ProviderDeclaration, download ManagedServiceDownload, allowed map[string]struct{}) error {
	if !managedServiceActionKeyPattern.MatchString(download.Key) || len(download.Key) > 64 ||
		download.Title.EnUS == "" || download.Title.ZhCN == "" ||
		download.Title.EnUS != strings.TrimSpace(download.Title.EnUS) || download.Title.ZhCN != strings.TrimSpace(download.Title.ZhCN) ||
		len([]rune(download.Title.EnUS)) > 128 || len([]rune(download.Title.ZhCN)) > 128 ||
		download.EndpointField != "endpoint" || !managedServiceActionKeyPattern.MatchString(download.ExternalReferenceField) ||
		len(download.ExternalReferenceField) > 64 || len(download.FixedBody) > 8 ||
		len(download.Path) < 2 || len(download.Path) > 256 || download.Path[0] != '/' ||
		strings.ContainsAny(download.Path, "?#\\") || strings.Contains(download.Path, "..") || len(download.SecretHeaders) < 1 || len(download.SecretHeaders) > 8 ||
		!validServiceActionAudiences(download.Audiences) {
		return gerror.New("product fulfillment managed service download is invalid")
	}
	if _, authorized := allowed[download.Method]; !authorized || download.Method == declaration.MethodRoles.Provision || download.Method == declaration.MethodRoles.Query {
		return gerror.New("product fulfillment managed service download method is not authorized")
	}
	if err := validateManagedServiceDownloadHeaders(download.SecretHeaders); err != nil {
		return err
	}
	return validateManagedServiceDownloadBody(download.FixedBody, download.ExternalReferenceField)
}

func validateManagedServiceDownloadHeaders(headers map[string]string) error {
	for header, slot := range headers {
		if (header != "X-API-Key" && header != "X-API-Secret") || !managedServiceActionKeyPattern.MatchString(slot) || len(slot) > 64 {
			return gerror.New("product fulfillment managed service download secret header is invalid")
		}
	}
	return nil
}

func validateManagedServiceDownloadBody(body map[string]string, externalReferenceField string) error {
	for field, value := range body {
		if !managedServiceActionKeyPattern.MatchString(field) || len(field) > 64 || strings.TrimSpace(value) != value || value == "" || len(value) > 128 || field == externalReferenceField {
			return gerror.New("product fulfillment managed service download fixed body is invalid")
		}
	}
	return nil
}

func validateManagedServiceActions(declaration *ProviderDeclaration, allowed map[string]struct{}) error {
	if !declaration.OngoingManagement && len(declaration.ServiceActions) > 0 {
		return gerror.New("one-time fulfillment must not declare managed service actions")
	}
	if len(declaration.ServiceActions) > 32 {
		return gerror.New("product fulfillment managed service action count is invalid")
	}
	seen := make(map[string]struct{}, len(declaration.ServiceActions))
	for _, action := range declaration.ServiceActions {
		if !managedServiceActionKeyPattern.MatchString(action.Key) || len(action.Key) > 64 ||
			action.Title.EnUS == "" || action.Title.ZhCN == "" || len([]rune(action.Title.EnUS)) > 128 || len([]rune(action.Title.ZhCN)) > 128 ||
			action.Title.EnUS != strings.TrimSpace(action.Title.EnUS) || action.Title.ZhCN != strings.TrimSpace(action.Title.ZhCN) {
			return gerror.New("product fulfillment managed service action identity is invalid")
		}
		if _, duplicated := seen[action.Key]; duplicated {
			return gerror.New("product fulfillment managed service action is duplicated")
		}
		seen[action.Key] = struct{}{}
		if _, authorized := allowed[action.Method]; !authorized || action.Method == declaration.MethodRoles.Provision {
			return gerror.New("product fulfillment managed service action method is not authorized")
		}
		if action.Execution != ServiceActionExecutionProjection && action.Execution != ServiceActionExecutionAsynchronous {
			return gerror.New("product fulfillment managed service action execution is invalid")
		}
		if action.Availability != ServiceActionAvailabilityAlways && action.Availability != ServiceActionAvailabilityBeforeFinal {
			return gerror.New("product fulfillment managed service action availability is invalid")
		}
		if action.Execution == ServiceActionExecutionProjection {
			if action.QueryMethod != "" || action.Availability != ServiceActionAvailabilityAlways || action.ConfirmedEntitlement != "" {
				return gerror.New("synchronous service projections cannot declare mutation recovery")
			}
		} else {
			if action.QueryMethod == "" || action.QueryMethod == action.Method {
				return gerror.New("asynchronous service actions require one distinct query method")
			}
			if _, authorized := allowed[action.QueryMethod]; !authorized {
				return gerror.New("asynchronous service action query method is not authorized")
			}
			if action.ConfirmedEntitlement != "" && action.ConfirmedEntitlement != "cancelled" {
				return gerror.New("asynchronous service action entitlement transition is invalid")
			}
		}
		if !validServiceActionAudiences(action.Audiences) {
			return gerror.New("product fulfillment managed service action audience is invalid")
		}
	}
	return nil
}

func validServiceActionAudiences(audiences []string) bool {
	if len(audiences) < 1 || len(audiences) > 2 {
		return false
	}
	seen := make(map[string]struct{}, len(audiences))
	for _, audience := range audiences {
		if audience != ServiceActionAudienceCustomer && audience != ServiceActionAudienceAdmin {
			return false
		}
		if _, duplicated := seen[audience]; duplicated {
			return false
		}
		seen[audience] = struct{}{}
	}
	return true
}

// RenewalMode derives the only safe renewal path from explicit provider facts.
// Mixed or one-time declarations remain unavailable instead of being guessed.
func RenewalMode(declaration *ProviderDeclaration) (string, error) {
	if declaration == nil {
		return "", gerror.New("product fulfillment provider declaration is nil")
	}
	if !declaration.OngoingManagement {
		if declaration.RenewalSupport != RenewalSupportUnsupported ||
			declaration.ExpiryManagement != ExpiryManagementUnsupported {
			return "", gerror.New("one-time delivery cannot declare renewal or expiry management")
		}
		return RenewalModeUnavailable, nil
	}
	switch {
	case declaration.RenewalSupport == RenewalSupportSupported &&
		declaration.ExpiryManagement == ExpiryManagementProvider:
		return RenewalModeProviderConfirmed, nil
	case declaration.RenewalSupport == RenewalSupportUnsupported &&
		declaration.ExpiryManagement == ExpiryManagementUnsupported:
		return RenewalModeLocalEntitlement, nil
	case (declaration.RenewalSupport == RenewalSupportSupported ||
		declaration.RenewalSupport == RenewalSupportUnsupported) &&
		(declaration.ExpiryManagement == ExpiryManagementProvider ||
			declaration.ExpiryManagement == ExpiryManagementUnsupported):
		return RenewalModeUnavailable, nil
	default:
		return "", gerror.New("product fulfillment renewal declaration is invalid")
	}
}

// ValidateProviderDeclarations validates the complete provider declaration
// group for one plugin release, including cross-contract-version uniqueness.
func ValidateProviderDeclarations(declarations []*capmodel.BusinessExtensionDeclaration) error {
	if len(declarations) != 1 || declarations[0] == nil {
		return gerror.New("product fulfillment provider release must declare exactly one contract")
	}
	declaration := declarations[0]
	if declaration.Point != capmodel.BusinessExtensionProductFulfillment ||
		declaration.Role != capmodel.BusinessExtensionRoleProvider {
		return gerror.New("product fulfillment provider declaration has an invalid owner contract scope")
	}
	_, err := ParseProviderDeclaration(declaration.ProviderMetadata, declaration.Methods)
	return err
}

// NewProviderMetadata encodes one typed declaration for tests and source providers.
func NewProviderMetadata(declaration *ProviderDeclaration) (*capmodel.BusinessExtensionProviderMetadata, error) {
	if declaration == nil {
		return nil, gerror.New("product fulfillment provider declaration is nil")
	}
	content, err := json.Marshal(declaration)
	if err != nil {
		return nil, gerror.Wrap(err, "encode product fulfillment provider declaration")
	}
	values := make(map[string]any)
	if err = json.Unmarshal(content, &values); err != nil {
		return nil, gerror.Wrap(err, "normalize product fulfillment provider declaration")
	}
	return &capmodel.BusinessExtensionProviderMetadata{Type: ProviderMetadataType, Version: ProviderMetadataVersion, Values: values}, nil
}
