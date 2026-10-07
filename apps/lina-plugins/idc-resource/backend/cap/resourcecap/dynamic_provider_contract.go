// This file publishes the owner-defined dynamic resource Provider extension
// contract. It is transport-neutral and never serializes channel credentials,
// operation secret handles, authentication tokens, or raw supplier responses.

package resourcecap

import (
	"encoding/json"
	"sort"
	"strings"
	"time"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/plugin/capability/capmodel"
)

const (
	// DynamicProviderPoint is the business extension point owned by idc-resource.
	DynamicProviderPoint = "resource-provider"
	// DynamicProviderContractVersion is the exact first bridge contract.
	DynamicProviderContractVersion = "v1"
	// DynamicProviderMetadataType identifies the provider descriptor metadata.
	DynamicProviderMetadataType = "resource-provider-contract"

	DynamicProviderMethodVerifyConnection         = "connection.verify"
	DynamicProviderMethodDiscoverCapabilities     = "capabilities.discover"
	DynamicProviderMethodConfigurationCandidates  = "configuration.candidates"
	DynamicProviderMethodBuildProvisionParameters = "provision.parameters.build"
	DynamicProviderMethodListProviderCapacity     = "provider-capacity.list"
	DynamicProviderMethodExecute                  = "operation.execute"
	DynamicProviderMethodQueryTask                = "task.query"
	DynamicProviderMethodQueryResource            = "resource.query"
	// Catalog methods are an optional, paired read-only extension. They grant
	// no create/update/publish permission to the supplier connector.
	DynamicProviderMethodListCatalog        = "catalog.products.list"
	DynamicProviderMethodReadCatalogProduct = "catalog.products.detail"
)

// ConnectionVerification is the deliberately small, non-sensitive proof
// returned after a connector has completed a real authenticated read against
// its configured upstream endpoint.
type ConnectionVerification struct {
	Verified   bool      `json:"verified"`
	ObservedAt time.Time `json:"observedAt"`
}

// ValidateAt rejects stale, future, or synthetic-looking verification results.
func (v ConnectionVerification) ValidateAt(now time.Time) error {
	now = now.UTC()
	observedAt := v.ObservedAt.UTC()
	if !v.Verified || observedAt.IsZero() || observedAt.Before(now.Add(-5*time.Minute)) || observedAt.After(now.Add(time.Minute)) {
		return gerror.New("resource dynamic Provider connection verification is invalid")
	}
	return nil
}

// DynamicOperationRequest is the secret-free operation projection allowed to
// enter an isolated WASM guest. ConfidentialInputNames contains canonical slot
// names only. The corresponding handles and plaintext remain in idc-resource.
type DynamicOperationRequest struct {
	RequestID              string               `json:"requestId"`
	Capability             CapabilityCode       `json:"capability"`
	ResourceRef            string               `json:"resourceRef,omitempty"`
	Parameters             json.RawMessage      `json:"parameters,omitempty"`
	ConfidentialInputNames []string             `json:"confidentialInputNames,omitempty"`
	ProviderCorrelation    *ProviderCorrelation `json:"providerCorrelation,omitempty"`
}

// ProviderCorrelation contains only non-secret callback routing facts created
// by idc-resource. The callback token is a separate opaque operation binding.
type ProviderCorrelation struct {
	DownstreamID  int64  `json:"downstreamId"`
	DownstreamURL string `json:"downstreamUrl"`
}

// DynamicProviderInvocationEnvelope gives every resource-provider method the
// same exact, non-secret interface projection from the frozen rci.
type DynamicProviderInvocationEnvelope struct {
	InterfaceSettings json.RawMessage `json:"interfaceSettings"`
	Payload           json.RawMessage `json:"payload"`
}

func NewDynamicProviderInvocationEnvelope(settings, payload json.RawMessage) (json.RawMessage, error) {
	if err := validateJSONObject("resource dynamic Provider interface settings", settings, false); err != nil ||
		len(payload) == 0 || len(payload) > 1<<20 || !json.Valid(payload) {
		return nil, gerror.New("resource dynamic Provider invocation envelope is invalid")
	}
	return json.Marshal(DynamicProviderInvocationEnvelope{InterfaceSettings: append(json.RawMessage(nil), settings...), Payload: append(json.RawMessage(nil), payload...)})
}

func DecodeDynamicProviderInvocationEnvelope(input json.RawMessage, target any) (json.RawMessage, error) {
	var envelope DynamicProviderInvocationEnvelope
	decoder := json.NewDecoder(strings.NewReader(string(input)))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&envelope) != nil || validateJSONObject("resource dynamic Provider interface settings", envelope.InterfaceSettings, false) != nil ||
		len(envelope.Payload) == 0 || len(envelope.Payload) > 1<<20 || !json.Valid(envelope.Payload) || target == nil {
		return nil, gerror.New("resource dynamic Provider invocation envelope is invalid")
	}
	payloadDecoder := json.NewDecoder(strings.NewReader(string(envelope.Payload)))
	payloadDecoder.DisallowUnknownFields()
	if payloadDecoder.Decode(target) != nil {
		return nil, gerror.New("resource dynamic Provider invocation envelope is invalid")
	}
	return append(json.RawMessage(nil), envelope.InterfaceSettings...), nil
}

// DynamicProviderMethods returns the deterministic required v1 method set.
// Optional catalog reads must be declared together in addition to this set.
func DynamicProviderMethods() []string {
	return []string{
		DynamicProviderMethodVerifyConnection,
		DynamicProviderMethodDiscoverCapabilities,
		DynamicProviderMethodConfigurationCandidates,
		DynamicProviderMethodBuildProvisionParameters,
		DynamicProviderMethodListProviderCapacity,
		DynamicProviderMethodExecute,
		DynamicProviderMethodQueryTask,
		DynamicProviderMethodQueryResource,
	}
}

// ValidateDynamicProviderDeclarations validates one provider's complete
// normalized declaration group. The generic host only governs lifecycle and
// dispatch; idc-resource remains the sole owner of Provider semantics.
func ValidateDynamicProviderDeclarations(declarations []*capmodel.BusinessExtensionDeclaration) error {
	if len(declarations) != 1 || declarations[0] == nil {
		return gerror.New("resource dynamic Provider declaration is incomplete")
	}
	declaration := declarations[0]
	if declaration.Point != DynamicProviderPoint || declaration.Version != DynamicProviderContractVersion ||
		declaration.Role != capmodel.BusinessExtensionRoleProvider || declaration.Owner != OwnerPluginID ||
		declaration.InterfaceConfiguration == nil ||
		declaration.InterfaceConfigurationCardinality != capmodel.InterfaceConfigurationCardinalityMultiple ||
		declaration.ProductConfiguration != nil ||
		declaration.CustomerConfiguration != nil {
		return gerror.New("resource dynamic Provider declaration is invalid")
	}
	want := DynamicProviderMethods()
	got := append([]string(nil), declaration.Methods...)
	sort.Strings(want)
	sort.Strings(got)
	if len(got) == len(want)+2 {
		want = append(want, DynamicProviderMethodListCatalog, DynamicProviderMethodReadCatalogProduct)
		sort.Strings(want)
	}
	if len(got) != len(want) {
		return gerror.New("resource dynamic Provider method declaration is incomplete")
	}
	for index := range want {
		if got[index] != want[index] {
			return gerror.New("resource dynamic Provider method declaration is incomplete")
		}
	}
	metadata := declaration.ProviderMetadata
	if ValidateProviderMetadata(metadata, declaration.InterfaceConfiguration) != nil {
		return gerror.New("resource dynamic Provider metadata is invalid")
	}
	providerID, ok := metadata.Values["providerId"].(string)
	if !ok || ValidateProviderID(strings.TrimSpace(providerID)) != nil || providerID != strings.TrimSpace(providerID) {
		return gerror.New("resource dynamic Provider metadata is invalid")
	}
	return nil
}

// Validate converts the WASM-visible operation into the existing normalized
// request validator without ever constructing or serializing a secret handle.
func (r DynamicOperationRequest) Validate() error {
	if err := (OperationRequest{
		RequestID: r.RequestID, Capability: r.Capability,
		ResourceRef: r.ResourceRef, Parameters: r.Parameters,
	}).Validate(); err != nil {
		return err
	}
	if len(r.ConfidentialInputNames) > MaximumSecretBindingCount {
		return gerror.New("resource dynamic Provider confidential input list is too large")
	}
	if r.ProviderCorrelation != nil && ValidateProviderCorrelation(r.ProviderCorrelation) != nil {
		return gerror.New("resource dynamic Provider correlation is invalid")
	}
	seen := make(map[string]struct{}, len(r.ConfidentialInputNames))
	for _, name := range r.ConfidentialInputNames {
		if err := validateSafeName("confidential input name", name, maximumNameBytes); err != nil {
			return err
		}
		if _, exists := seen[name]; exists {
			return gerror.New("resource dynamic Provider confidential input is duplicated")
		}
		seen[name] = struct{}{}
	}
	return nil
}

// ProjectDynamicOperationRequest strips opaque handles from one validated
// owner request before the payload is sent to the WASM provider.
func ProjectDynamicOperationRequest(request OperationRequest) (DynamicOperationRequest, error) {
	if err := request.Validate(); err != nil {
		return DynamicOperationRequest{}, err
	}
	names := make([]string, 0, len(request.SecretBindings))
	for _, binding := range request.SecretBindings {
		names = append(names, binding.Name)
	}
	sort.Strings(names)
	return DynamicOperationRequest{
		RequestID: request.RequestID, Capability: request.Capability,
		ResourceRef: request.ResourceRef, Parameters: append(json.RawMessage(nil), request.Parameters...),
		ConfidentialInputNames: names,
		ProviderCorrelation:    request.ProviderCorrelation,
	}, nil
}
