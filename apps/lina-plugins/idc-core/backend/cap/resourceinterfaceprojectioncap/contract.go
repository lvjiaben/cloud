// Package resourceinterfaceprojectioncap defines the secret-free resource
// interface projection and resource-owned dynamic-candidate proxy contracts.
package resourceinterfaceprojectioncap

import (
	"context"
	"encoding/json"
	"regexp"
	"strings"

	"github.com/gogf/gf/v2/errors/gerror"
)

const (
	OwnerID        = "idc-resource"
	StateAccepting = "accepting"
	StateStopped   = "stopped"
)

var (
	interfaceNoPattern           = regexp.MustCompile(`^pfi_[0-9a-f]{32}$`)
	configurationRevisionPattern = regexp.MustCompile(`^rci_[0-9a-f]{32}$`)
	channelPattern               = regexp.MustCompile(`^CHN_[A-Za-z0-9_-]{1,60}$`)
	pluginPattern                = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)
	methodPattern                = regexp.MustCompile(`^[a-z][a-z0-9]*(?:[._-][a-z0-9]+)*$`)
	digestPattern                = regexp.MustCompile(`^[0-9a-f]{64}$`)
)

// Projection is the complete secret-free identity mirrored into product
// automation. ResourceChannelRef is only an owner-local reference, never an
// endpoint, account, token, or secret handle.
type Projection struct {
	InterfaceInstanceNo string
	// InterfaceConfigurationRevision identifies the immutable resource-owner
	// endpoint/account/secret-reference revision. It is deliberately distinct
	// from the product configuration revision frozen by commerce.
	InterfaceConfigurationRevision string
	DisplayName                    string
	// ActorUserID is the authenticated staff actor that created or changed the
	// resource channel. Core must not invent a synthetic database user here.
	ActorUserID          int
	OwnerID              string
	ResourceChannelRef   string
	PluginID             string
	PluginReleaseVersion string
	ContractVersion      string
	AuthorizedMethods    []string
	ReleaseDigest        string
	DeclarationDigest    string
	NewBusinessState     string
}

// Sink idempotently receives one resource-owned interface projection.
type Sink interface {
	Upsert(context.Context, Projection) error
}

// Usage is the staff-visible current local usage of one resource channel.
// ActiveInstances follows entitlement, not power state. TotalInstances includes
// terminated and imported instances. ConfiguredProducts counts only the current
// effective product-version binding, never draft or historical bindings.
type Usage struct {
	ChannelNo          string
	ActiveInstances    int
	TotalInstances     int
	ConfiguredProducts int
}

// UsageReader supplies one bounded local projection to the resource owner.
type UsageReader interface {
	// BatchGet requires direct staff configuration-list permission and supported
	// merchant data scope. It accepts at most 100 distinct channel numbers,
	// returns zero counts for unused channels, and never contacts a provider.
	BatchGet(ctx context.Context, channelNos []string) ([]Usage, error)
}

// CandidateRequest asks the resource owner to resolve one frozen, bounded
// candidate method through its pfi-to-channel mapping.
type CandidateRequest struct {
	InterfaceInstanceNo string
	// ExpectedConfigurationRevision pins an owner-side immutable snapshot for
	// import validation. Empty preserves ordinary current-candidate reads.
	ExpectedConfigurationRevision string
	Method                        string
	Values                        json.RawMessage
}

// Candidate is a safe label/value projection without provider credentials.
type Candidate struct{ Value, LabelZhCN, LabelEnUS string }

// CandidateResult contains one bounded candidate list.
type CandidateField struct {
	FieldKey string
	Items    []Candidate
}

type CandidateResult struct{ Fields []CandidateField }

// CandidateSource is implemented by idc-resource; core must never resolve a
// resource-owned candidate using core-held endpoint or account secrets.
type CandidateSource interface {
	ResolveCandidates(context.Context, CandidateRequest) (*CandidateResult, error)
}

func ValidateProjection(value Projection) error {
	if err := validateProjectionIdentity(value); err != nil {
		return err
	}
	return validateProjectionMethods(value.AuthorizedMethods)
}

func validateProjectionIdentity(value Projection) error {
	if !interfaceNoPattern.MatchString(value.InterfaceInstanceNo) ||
		!configurationRevisionPattern.MatchString(value.InterfaceConfigurationRevision) ||
		strings.TrimSpace(value.DisplayName) != value.DisplayName || len(value.DisplayName) < 1 || len(value.DisplayName) > 128 ||
		value.ActorUserID <= 0 ||
		value.OwnerID != OwnerID || !channelPattern.MatchString(value.ResourceChannelRef) ||
		!pluginPattern.MatchString(value.PluginID) || len(value.PluginID) > 64 ||
		strings.TrimSpace(value.PluginReleaseVersion) == "" || len(value.PluginReleaseVersion) > 32 ||
		strings.TrimSpace(value.ContractVersion) == "" || len(value.ContractVersion) > 32 ||
		!digestPattern.MatchString(value.ReleaseDigest) || !digestPattern.MatchString(value.DeclarationDigest) ||
		(value.NewBusinessState != StateAccepting && value.NewBusinessState != StateStopped) ||
		len(value.AuthorizedMethods) == 0 || len(value.AuthorizedMethods) > 16 {
		return gerror.New("resource interface projection is invalid")
	}
	return nil
}

func validateProjectionMethods(methods []string) error {
	seen := make(map[string]struct{}, len(methods))
	for _, method := range methods {
		if !methodPattern.MatchString(method) {
			return gerror.New("resource interface projection method is invalid")
		}
		if _, exists := seen[method]; exists {
			return gerror.New("resource interface projection methods are not unique")
		}
		seen[method] = struct{}{}
	}
	return nil
}

func ValidateCandidateRequest(value CandidateRequest) error {
	if !interfaceNoPattern.MatchString(value.InterfaceInstanceNo) || !methodPattern.MatchString(value.Method) ||
		(value.ExpectedConfigurationRevision != "" && !configurationRevisionPattern.MatchString(value.ExpectedConfigurationRevision)) ||
		len(value.Values) == 0 || len(value.Values) > 1<<20 || !json.Valid(value.Values) {
		return gerror.New("resource interface candidate request is invalid")
	}
	var object map[string]json.RawMessage
	if json.Unmarshal(value.Values, &object) != nil || object == nil {
		return gerror.New("resource interface candidate values are invalid")
	}
	return nil
}
