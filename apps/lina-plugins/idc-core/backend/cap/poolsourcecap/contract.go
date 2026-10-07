// Package poolsourcecap defines the catalog requester's owner-implemented pool candidate port.
package poolsourcecap

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/gogf/gf/v2/errors/gerror"
)

const (
	// MaximumCandidates bounds one owner projection and its catalog response.
	MaximumCandidates = 500
	// MaximumPayloadBytes bounds one serialized candidate response.
	MaximumPayloadBytes = 512 * 1024
)

var (
	// ErrCandidateUnavailable means the exact candidate is absent, disabled, changed, or incompatible.
	ErrCandidateUnavailable = errors.New("resource pool candidate is unavailable")
	// ErrSourceUnavailable means the resource owner cannot serve the requester port.
	ErrSourceUnavailable = errors.New("resource pool candidate source is unavailable")
	poolRevisionPattern  = regexp.MustCompile(`^[0-9a-f]{64}$`)
)

// ResourceKind is the normalized server family shared at the owner boundary.
type ResourceKind string

const (
	ResourceKindVirtualServer ResourceKind = "virtual_server"
	ResourceKindBareMetal     ResourceKind = "bare_metal"
)

// Candidate is a bounded, credential-free projection of one currently eligible owner pool.
type Candidate struct {
	PoolNo           string       `json:"poolNo"`
	DisplayName      string       `json:"displayName"`
	ChannelName      string       `json:"channelName"`
	ProviderID       string       `json:"providerId"`
	ProviderPluginID string       `json:"providerPluginId"`
	ResourceKind     ResourceKind `json:"resourceKind"`
	Revision         string       `json:"revision"`
}

// ListRequest selects eligible pools by normalized server family.
type ListRequest struct {
	ResourceKind ResourceKind
}

// VerifyRequest rechecks the exact owner candidate selected by an administrator.
type VerifyRequest struct {
	PoolNo       string
	ResourceKind ResourceKind
	Revision     string
}

// Source is implemented by the resource owner and explicitly injected into catalog management.
type Source interface {
	ListEligible(ctx context.Context, request ListRequest) ([]Candidate, error)
	VerifyEligible(ctx context.Context, request VerifyRequest) (*Candidate, error)
}

// ValidateListRequest rejects unsupported resource families.
func ValidateListRequest(request ListRequest) error {
	return validateResourceKind(request.ResourceKind)
}

// ValidateVerifyRequest rejects malformed selections before owner persistence is accessed.
func ValidateVerifyRequest(request VerifyRequest) error {
	if validatePoolNo(request.PoolNo) != nil || validateResourceKind(request.ResourceKind) != nil ||
		!poolRevisionPattern.MatchString(request.Revision) {
		return gerror.New("resource pool candidate verification request is invalid")
	}
	return nil
}

// ValidateCandidate validates one detached owner projection.
func ValidateCandidate(candidate Candidate) error {
	if validatePoolNo(candidate.PoolNo) != nil ||
		validateResourceKind(candidate.ResourceKind) != nil ||
		!poolRevisionPattern.MatchString(candidate.Revision) ||
		validateText(candidate.DisplayName, 128) != nil ||
		validateText(candidate.ChannelName, 128) != nil ||
		validateMachineID(candidate.ProviderID, 128) != nil ||
		validateMachineID(candidate.ProviderPluginID, 128) != nil {
		return gerror.New("resource pool candidate is invalid")
	}
	return nil
}

// ValidateCandidates validates bounded uniqueness and deterministic owner projections.
func ValidateCandidates(candidates []Candidate) error {
	if len(candidates) > MaximumCandidates {
		return gerror.New("resource pool candidate collection exceeds limit")
	}
	seen := make(map[string]struct{}, len(candidates))
	for _, candidate := range candidates {
		if err := ValidateCandidate(candidate); err != nil {
			return err
		}
		if _, exists := seen[candidate.PoolNo]; exists {
			return gerror.New("resource pool candidate collection contains a duplicate")
		}
		seen[candidate.PoolNo] = struct{}{}
	}
	return nil
}

func validateResourceKind(kind ResourceKind) error {
	if kind != ResourceKindVirtualServer && kind != ResourceKindBareMetal {
		return gerror.New("resource pool candidate kind is invalid")
	}
	return nil
}

func validatePoolNo(value string) error {
	if validateText(value, 64) != nil || strings.IndexFunc(value, unicode.IsSpace) >= 0 {
		return gerror.New("resource pool candidate number is invalid")
	}
	return nil
}

func validateMachineID(value string, maximum int) error {
	if validateText(value, maximum) != nil || strings.IndexFunc(value, unicode.IsSpace) >= 0 {
		return gerror.New("resource pool candidate owner identity is invalid")
	}
	return nil
}

func validateText(value string, maximum int) error {
	if value == "" || value != strings.TrimSpace(value) || !utf8.ValidString(value) ||
		utf8.RuneCountInString(value) > maximum || strings.IndexFunc(value, unicode.IsControl) >= 0 {
		return gerror.New("resource pool candidate text is invalid")
	}
	return nil
}
