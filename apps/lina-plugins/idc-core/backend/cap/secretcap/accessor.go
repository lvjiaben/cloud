// This file defines callback-scoped plaintext access without exposing an encryption implementation.

package secretcap

import (
	"context"
	"strings"

	"github.com/gogf/gf/v2/errors/gerror"
)

const (
	// maxPurposeBytes bounds the complete audit purpose code.
	maxPurposeBytes = 96
	// maxPurposeSegmentBytes bounds one namespace or purpose segment.
	maxPurposeSegmentBytes = 32
)

// Purpose identifies the non-secret business reason recorded for one access.
type Purpose string

// AccessIntent is a JSON-safe metadata projection. It deliberately carries no
// handle or plaintext and does not imply that a dynamic plaintext bridge is
// available.
type AccessIntent struct {
	// Purpose is the validated business reason for the access.
	Purpose Purpose `json:"purpose"`
}

// AccessRequest combines an opaque handle with its audited purpose for a
// source-plugin accessor call. JSON serialization fails through Handle.
type AccessRequest struct {
	// Handle identifies owner-managed encrypted secret material.
	Handle Handle `json:"handle"`
	// Purpose is the validated business reason for the access.
	Purpose Purpose `json:"purpose"`
}

// Consumer receives a plaintext byte view that is valid only during the
// callback. Consumers must not retain, log, serialize, or include the bytes in
// errors; owner implementations should clear their buffer after return.
type Consumer func(plaintext []byte) error

// Accessor provides least-privilege, callback-scoped source-plugin access to
// handles already authorized for the injected caller. It does not provide
// secret discovery, enumeration, global lookup, storage, encryption, key
// rotation, or a dynamic-plugin plaintext bridge.
type Accessor interface {
	// Access validates caller scope and purpose, invokes consume at most once,
	// and never returns plaintext from the method.
	Access(ctx context.Context, request AccessRequest, consume Consumer) error
}

// NewPurpose validates and creates a stable, low-entropy audit purpose code.
// Purpose values are namespaced codes such as provider.authentication, never
// operator prose, request headers, credentials, or arbitrary external text.
func NewPurpose(value string) (Purpose, error) {
	if !validPurposeCode(value) {
		return "", gerror.New("secretcap: access purpose is invalid")
	}
	return Purpose(value), nil
}

// validPurposeCode reports whether value is a bounded namespaced audit code.
func validPurposeCode(value string) bool {
	if value == "" || value != strings.TrimSpace(value) || len(value) > maxPurposeBytes {
		return false
	}
	segments := strings.Split(value, ".")
	if len(segments) < 2 || !allowedPurposeNamespace(segments[0]) {
		return false
	}
	for _, segment := range segments {
		if !validPurposeSegment(segment) {
			return false
		}
	}
	return true
}

// allowedPurposeNamespace reports whether value is an owner-reviewed namespace.
func allowedPurposeNamespace(value string) bool {
	switch value {
	case "core", "payment", "resource", "provider", "compliance", "notify", "channel", "ai", "portal", "migration", "system":
		return true
	default:
		return false
	}
}

// validPurposeSegment reports whether value uses the canonical segment syntax.
func validPurposeSegment(value string) bool {
	if value == "" || len(value) > maxPurposeSegmentBytes || value[0] < 'a' || value[0] > 'z' {
		return false
	}
	previousHyphen := false
	for index := 1; index < len(value); index++ {
		character := value[index]
		if character == '-' {
			if previousHyphen || index == len(value)-1 {
				return false
			}
			previousHyphen = true
			continue
		}
		if (character < 'a' || character > 'z') && (character < '0' || character > '9') {
			return false
		}
		previousHyphen = false
	}
	return true
}

// NewAccessRequest creates a validated callback-scoped access request.
func NewAccessRequest(handle Handle, purpose Purpose) (AccessRequest, error) {
	request := AccessRequest{Handle: handle, Purpose: purpose}
	if err := validateAccessRequest(request); err != nil {
		return AccessRequest{}, err
	}
	return request, nil
}

// ValidateAccess validates the reusable contract preconditions for accessor
// implementations without performing authorization or revealing plaintext.
func ValidateAccess(request AccessRequest, consume Consumer) error {
	if err := validateAccessRequest(request); err != nil {
		return err
	}
	if consume == nil {
		return gerror.New("secretcap: plaintext consumer is required")
	}
	return nil
}

// validateAccessRequest validates the handle and purpose shared by constructors
// and accessor implementations without invoking a plaintext consumer.
func validateAccessRequest(request AccessRequest) error {
	if request.Handle.IsZero() {
		return gerror.New("secretcap: a non-zero secret handle is required")
	}
	if _, err := NewPurpose(string(request.Purpose)); err != nil {
		return err
	}
	return nil
}

// Intent returns JSON-safe audit metadata while intentionally omitting the
// handle reference. A future dynamic bridge requires a separately reviewed
// confidential transport and is not enabled by this projection.
func (r AccessRequest) Intent() AccessIntent {
	return AccessIntent{Purpose: r.Purpose}
}
