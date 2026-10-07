// Package confidentialoutbound owns short-lived, invocation-scoped bindings for
// host-managed supplier calls made on behalf of isolated dynamic plugins.
package confidentialoutbound

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"io"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/plugin/capability/capmodel"
	"lina-core/pkg/plugin/pluginhost"
)

const (
	defaultContextLifetime = 10 * time.Second
	maxContextLifetime     = 30 * time.Second
	maxPayloadBytes        = 64 * 1024
	maxResultBytes         = 1 * 1024 * 1024
	maxResultFields        = 128
	maxInvocationSteps     = 8
	maxInvocationBytes     = 2 * 1024 * 1024
)

var safeNamePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$`)

var forbiddenProjectionKeys = map[string]struct{}{
	"authorization": {}, "cookie": {}, "credential": {}, "credentials": {},
	"password": {}, "secret": {}, "secrethandle": {}, "secretref": {},
	"token": {}, "accesstoken": {}, "refreshtoken": {}, "sessiontoken": {},
	"securitytoken": {}, "apikey": {}, "xapikey": {}, "accesskey": {},
	"accesskeyid": {}, "secretkey": {}, "clientsecret": {}, "privatekey": {},
	"interfaceinstanceid": {}, "outboundmethod": {}, "providerpluginid": {},
	"ownerpluginid": {}, "releasedigest": {}, "declarationdigest": {},
}

// Invocation identifies the exact dynamic provider call currently executing.
type Invocation struct {
	OwnerPluginID     string
	ProviderPluginID  string
	Point             string
	ContractVersion   string
	ProviderMethod    string
	ReleaseVersion    string
	ReleaseDigest     string
	DeclarationDigest string
	Purpose           capmodel.BusinessExtensionInvocationPurpose
	ContextID         string
}

// Registration is the normalized host-only context created by a source owner.
type Registration struct {
	OwnerPluginID string
	Input         pluginhost.ConfidentialOutboundContextInput
}

// Registry stores only short-lived callbacks and bindings in the startup-owned
// plugin service instance. Entries are consumed before the supplier call starts.
type Registry struct {
	mu      sync.Mutex
	entries map[string]*entry
	now     func() time.Time
}

type entry struct {
	registration Registration
	expiresAt    time.Time
	nextSequence uint8
	steps        int
	totalBytes   int
	inUse        bool
}

// New creates an empty process-local registry. A context never crosses a
// request or node; an unknown context therefore fails closed.
func New() *Registry {
	return &Registry{entries: make(map[string]*entry), now: time.Now}
}

// Register creates one opaque context and returns a revoker for guaranteed
// cleanup when the surrounding provider invocation ends.
func (r *Registry) Register(registration Registration) (string, func(), error) {
	if r == nil {
		return "", nil, gerror.New("confidential outbound registry is unavailable")
	}
	if err := validateRegistration(registration); err != nil {
		return "", nil, err
	}
	lifetime := registration.Input.ExpiresAfter
	if lifetime == 0 {
		lifetime = defaultContextLifetime
	}
	if lifetime < 0 || lifetime > maxContextLifetime {
		return "", nil, gerror.New("confidential outbound context lifetime is invalid")
	}
	contextID, err := newContextID()
	if err != nil {
		return "", nil, err
	}
	r.mu.Lock()
	r.entries[contextID] = &entry{
		registration: cloneRegistration(registration), expiresAt: r.now().Add(lifetime), nextSequence: 1,
	}
	r.mu.Unlock()
	return contextID, func() { r.revoke(contextID) }, nil
}

// Execute performs one strictly ordered step in an exact binding. The caller
// must advance sequence numbers; retries, concurrent calls and calls beyond the
// small invocation budget fail before the supplier callback starts.
func (r *Registry) Execute(
	ctx context.Context,
	invocation Invocation,
	request *pluginhost.ConfidentialOutboundRequest,
) (*pluginhost.ConfidentialOutboundResult, error) {
	if r == nil {
		return nil, gerror.New("confidential outbound registry is unavailable")
	}
	if request == nil || validateJSONObject(request.Payload, maxPayloadBytes, nil) != nil {
		return nil, gerror.New("confidential outbound request is invalid")
	}
	r.mu.Lock()
	bound := r.entries[invocation.ContextID]
	if bound == nil || !r.now().Before(bound.expiresAt) || !matches(bound.registration, invocation) ||
		request.Sequence == 0 || request.Sequence != bound.nextSequence || bound.inUse ||
		bound.steps >= maxInvocationSteps || bound.totalBytes+len(request.Payload) > maxInvocationBytes {
		r.mu.Unlock()
		return nil, gerror.New("confidential outbound context is unavailable")
	}
	bound.inUse = true
	bound.steps++
	bound.nextSequence++
	bound.totalBytes += len(request.Payload)
	registration := cloneRegistration(bound.registration)
	r.mu.Unlock()

	result, err := registration.Input.Handler(ctx, &pluginhost.ConfidentialOutboundRequest{
		Sequence: request.Sequence, Payload: append(json.RawMessage(nil), request.Payload...),
	})
	r.mu.Lock()
	if current := r.entries[invocation.ContextID]; current == bound {
		current.inUse = false
		if result != nil {
			current.totalBytes += len(result.Payload)
		}
		if current.totalBytes > maxInvocationBytes {
			delete(r.entries, invocation.ContextID)
		}
	}
	r.mu.Unlock()
	if err != nil {
		return nil, gerror.New("confidential outbound supplier operation failed")
	}
	allowed := normalizedSet(registration.Input.AllowedResultFields)
	if result == nil || validateJSONObject(result.Payload, maxResultBytes, allowed) != nil {
		return nil, gerror.New("confidential outbound result projection is invalid")
	}
	return &pluginhost.ConfidentialOutboundResult{Payload: append(json.RawMessage(nil), result.Payload...)}, nil
}

func (r *Registry) revoke(contextID string) {
	if r == nil || contextID == "" {
		return
	}
	r.mu.Lock()
	delete(r.entries, contextID)
	r.mu.Unlock()
}

func validateRegistration(registration Registration) error {
	input := registration.Input
	for _, value := range []string{
		registration.OwnerPluginID, input.ProviderPluginID, input.Point,
		input.ContractVersion, input.ProviderMethod, input.OutboundMethod,
		input.ReleaseVersion, input.ReleaseDigest, input.DeclarationDigest,
		input.InterfaceInstanceID,
	} {
		if !safeNamePattern.MatchString(strings.TrimSpace(value)) {
			return gerror.New("confidential outbound binding is incomplete or invalid")
		}
	}
	if len(input.ReleaseDigest) != 64 || len(input.DeclarationDigest) != 64 || input.Handler == nil {
		return gerror.New("confidential outbound evidence or handler is invalid")
	}
	if input.Purpose != capmodel.BusinessExtensionInvocationPurposeNewBusiness &&
		input.Purpose != capmodel.BusinessExtensionInvocationPurposeExistingBusiness {
		return gerror.New("confidential outbound purpose is invalid")
	}
	if len(input.AllowedResultFields) == 0 || len(input.AllowedResultFields) > maxResultFields {
		return gerror.New("confidential outbound result projection is missing")
	}
	for field := range normalizedSet(input.AllowedResultFields) {
		if !safeNamePattern.MatchString(field) || sensitiveKey(field) {
			return gerror.New("confidential outbound result projection contains a forbidden field")
		}
	}
	return nil
}

func matches(registration Registration, invocation Invocation) bool {
	input := registration.Input
	return registration.OwnerPluginID == invocation.OwnerPluginID &&
		input.ProviderPluginID == invocation.ProviderPluginID &&
		input.Point == invocation.Point && input.ContractVersion == invocation.ContractVersion &&
		input.ProviderMethod == invocation.ProviderMethod && input.ReleaseVersion == invocation.ReleaseVersion &&
		input.ReleaseDigest == invocation.ReleaseDigest && input.DeclarationDigest == invocation.DeclarationDigest &&
		input.Purpose == invocation.Purpose
}

func cloneRegistration(registration Registration) Registration {
	cloned := registration
	cloned.Input.AllowedResultFields = append([]string(nil), registration.Input.AllowedResultFields...)
	return cloned
}

func newContextID() (string, error) {
	random := make([]byte, 32)
	if _, err := rand.Read(random); err != nil {
		return "", gerror.Wrap(err, "generate confidential outbound context failed")
	}
	return "ctx_" + base64.RawURLEncoding.EncodeToString(random), nil
}

func validateJSONObject(content []byte, maxBytes int, allowedTopLevel map[string]struct{}) error {
	if len(content) == 0 || len(content) > maxBytes {
		return gerror.New("confidential outbound JSON size is invalid")
	}
	decoder := json.NewDecoder(bytes.NewReader(content))
	decoder.UseNumber()
	var value map[string]any
	if err := decoder.Decode(&value); err != nil || value == nil {
		return gerror.New("confidential outbound payload must be one JSON object")
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return gerror.New("confidential outbound payload contains trailing data")
	}
	if allowedTopLevel != nil {
		for key := range value {
			if _, ok := allowedTopLevel[key]; !ok {
				return gerror.New("confidential outbound result contains an unapproved field")
			}
		}
	}
	return validateSafeJSONValue(value, 1)
}

func validateSafeJSONValue(value any, depth int) error {
	if depth > 16 {
		return gerror.New("confidential outbound JSON is too deep")
	}
	switch typed := value.(type) {
	case map[string]any:
		if len(typed) > maxResultFields {
			return gerror.New("confidential outbound JSON has too many fields")
		}
		for key, child := range typed {
			if !safeNamePattern.MatchString(key) || sensitiveKey(key) {
				return gerror.New("confidential outbound JSON contains a forbidden field")
			}
			if err := validateSafeJSONValue(child, depth+1); err != nil {
				return err
			}
		}
	case []any:
		if len(typed) > 512 {
			return gerror.New("confidential outbound JSON array is too large")
		}
		for _, child := range typed {
			if err := validateSafeJSONValue(child, depth+1); err != nil {
				return err
			}
		}
	case string:
		if len(typed) > 8*1024 {
			return gerror.New("confidential outbound JSON string is too large")
		}
	case nil, bool, json.Number:
	default:
		return gerror.New("confidential outbound JSON contains an unsupported value")
	}
	return nil
}

func normalizedSet(values []string) map[string]struct{} {
	result := make(map[string]struct{}, len(values))
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			result[trimmed] = struct{}{}
		}
	}
	return result
}

func sensitiveKey(key string) bool {
	normalized := strings.NewReplacer("_", "", "-", "", ".", "").Replace(strings.ToLower(strings.TrimSpace(key)))
	_, forbidden := forbiddenProjectionKeys[normalized]
	return forbidden
}
