package pluginhost

import (
	"context"
	"encoding/json"
	"time"

	"lina-core/pkg/plugin/capability/capmodel"
	"lina-core/pkg/plugin/capability/plugincap"
)

// BusinessExtensionRegistrar is the source-business-owner-only facade for
// provider discovery and atomic new-binding admission.
type BusinessExtensionRegistrar interface {
	ListProviders(ctx context.Context, input plugincap.BusinessExtensionProviderListInput) (*capmodel.PageResult[*capmodel.BusinessExtensionProviderEvidence], error)
	GetProviderExact(ctx context.Context, input plugincap.BusinessExtensionProviderExactInput) (*capmodel.BusinessExtensionProviderEvidence, error)
	GetProvidersExact(ctx context.Context, inputs []plugincap.BusinessExtensionProviderExactInput) ([]*capmodel.BusinessExtensionProviderEvidence, error)
	WithNewBusinessAdmission(
		ctx context.Context,
		input BusinessExtensionAdmissionInput,
		action capmodel.BusinessExtensionAdmissionAction,
	) error
	InvokeProviderMethod(ctx context.Context, input BusinessExtensionMethodInvokeInput) (*capmodel.BusinessExtensionMethodResult, error)
	WithConfidentialOutboundContext(
		ctx context.Context,
		input ConfidentialOutboundContextInput,
		action ConfidentialOutboundAction,
	) error
}

// ConfidentialOutboundHandler executes one host-owned supplier operation. The
// implementation may resolve interface credentials and inspect the raw upstream
// response, but it must return only the explicitly allowed safe projection.
type ConfidentialOutboundHandler func(
	ctx context.Context,
	request *ConfidentialOutboundRequest,
) (*ConfidentialOutboundResult, error)

// ConfidentialOutboundAction receives one opaque, short-lived runtime context
// that can be attached to exactly one provider invocation.
type ConfidentialOutboundAction func(
	ctx context.Context,
	runtimeContext *capmodel.BusinessExtensionRuntimeContext,
) error

// ConfidentialOutboundContextInput binds one host-owned supplier operation to
// exact provider evidence, interface instance, purpose, and provider method.
type ConfidentialOutboundContextInput struct {
	ProviderPluginID    string
	Point               string
	ContractVersion     string
	ProviderMethod      string
	OutboundMethod      string
	ReleaseVersion      string
	ReleaseDigest       string
	DeclarationDigest   string
	InterfaceInstanceID string
	Purpose             capmodel.BusinessExtensionInvocationPurpose
	ExpiresAfter        time.Duration
	AllowedResultFields []string
	Handler             ConfidentialOutboundHandler
}

// ConfidentialOutboundRequest carries only the bounded non-secret business
// input supplied by the isolated provider.
type ConfidentialOutboundRequest struct {
	Sequence uint8           `json:"sequence"`
	Payload  json.RawMessage `json:"payload"`
}

// ConfidentialOutboundResult carries the host-approved safe result projection.
type ConfidentialOutboundResult struct {
	Payload json.RawMessage `json:"payload"`
}

// SourcePluginBusinessExtensionMethodHandler executes one host-authorized
// provider method without receiving host credentials or private runtime state.
type SourcePluginBusinessExtensionMethodHandler func(
	ctx context.Context,
	invocation *capmodel.BusinessExtensionMethodInvocation,
) (*capmodel.BusinessExtensionMethodResult, error)

// BusinessExtensionMethodInvokeInput identifies one exact frozen provider
// release and carries one bounded JSON object request.
type BusinessExtensionMethodInvokeInput struct {
	PluginID          string
	Point             string
	ContractVersion   string
	Method            string
	ReleaseVersion    string
	ReleaseDigest     string
	DeclarationDigest string
	Purpose           capmodel.BusinessExtensionInvocationPurpose
	RuntimeContext    *capmodel.BusinessExtensionRuntimeContext
	Payload           []byte
}

// BusinessExtensionAdmissionInput identifies the provider contract and method
// that an owner is about to freeze in its own transaction.
type BusinessExtensionAdmissionInput struct {
	PluginID          string
	Point             string
	ContractVersion   string
	Method            string
	ReleaseVersion    string
	ReleaseDigest     string
	DeclarationDigest string
}
