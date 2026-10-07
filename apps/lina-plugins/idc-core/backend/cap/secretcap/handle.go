// This file defines an opaque handle that cannot accidentally serialize or log its reference.

package secretcap

import (
	"fmt"
	"io"
	"log/slog"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/gogf/gf/v2/errors/gerror"
)

const (
	redactedHandleText = "[secret-handle:redacted]"
	maxHandleRefBytes  = 256
)

// Handle is an opaque reference to owner-managed encrypted secret material.
// It never contains plaintext and only OpaqueRef may reveal its reference for
// authorized owner persistence or bridge resource routing.
type Handle struct {
	reference string
}

// NewHandle validates and creates an opaque secret handle from an owner-issued
// reference. The reference itself is not secret plaintext, but remains
// sensitive routing metadata and must not be logged.
func NewHandle(reference string) (Handle, error) {
	if reference == "" ||
		reference != strings.TrimSpace(reference) ||
		len(reference) > maxHandleRefBytes ||
		!utf8.ValidString(reference) ||
		strings.IndexFunc(reference, unicode.IsControl) >= 0 {
		return Handle{}, gerror.New("secretcap: secret handle reference is invalid")
	}
	return Handle{reference: reference}, nil
}

// IsZero reports whether the handle contains no owner-issued reference.
func (h Handle) IsZero() bool {
	return h.reference == ""
}

// Equal reports whether two opaque handles reference the same owner record
// without revealing either reference.
func (h Handle) Equal(other Handle) bool {
	return h.reference == other.reference
}

// OpaqueRef reveals the routing reference for authorized owner persistence or
// bridge resourceRef mapping. Callers must never place the result in logs,
// errors, audit payloads, JSON, or client responses.
func (h Handle) OpaqueRef() string {
	return h.reference
}

// String returns a fixed redaction marker for ordinary formatting.
func (h Handle) String() string {
	return redactedHandleText
}

// GoString returns a fixed redaction marker for Go-syntax formatting.
func (h Handle) GoString() string {
	return redactedHandleText
}

// Format implements fmt.Formatter so every formatting verb and flag remains
// redacted, including diagnostic `%+v` and `%#v` output.
func (h Handle) Format(state fmt.State, _ rune) {
	if _, err := io.WriteString(state, redactedHandleText); err != nil {
		return
	}
}

// LogValue returns a redacted value for structured slog output.
func (h Handle) LogValue() slog.Value {
	return slog.StringValue(redactedHandleText)
}

// MarshalJSON rejects accidental JSON serialization instead of exposing the
// opaque routing reference.
func (h Handle) MarshalJSON() ([]byte, error) {
	return nil, gerror.New("secretcap: secret handles cannot be encoded as JSON")
}

// UnmarshalJSON rejects reconstruction from ungoverned JSON. Authorized owner
// code must validate references through NewHandle.
func (h *Handle) UnmarshalJSON(_ []byte) error {
	return gerror.New("secretcap: secret handles cannot be decoded from JSON")
}

// MarshalText rejects accidental text serialization instead of exposing the
// opaque routing reference.
func (h Handle) MarshalText() ([]byte, error) {
	return nil, gerror.New("secretcap: secret handles cannot be encoded as text")
}

// UnmarshalText rejects reconstruction from ungoverned text. Authorized owner
// code must validate references through NewHandle.
func (h *Handle) UnmarshalText(_ []byte) error {
	return gerror.New("secretcap: secret handles cannot be decoded from text")
}
