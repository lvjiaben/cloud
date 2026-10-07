// This file maps the optional upstream-principal contract to administrator v1.

package zjmfcloud

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"lina-plugin-idc-core/backend/cap/secretcap"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
)

var _ resourcecap.PrincipalProvider = (*Provider)(nil)

var principalBindingOrder = []string{
	resourcecap.PrincipalBindingUsername,
	resourcecap.PrincipalBindingPassword,
	resourcecap.PrincipalBindingPhone,
	resourcecap.PrincipalBindingEmail,
	resourcecap.PrincipalBindingRealName,
}

// LookupPrincipal performs the documented exact username check. Only an
// explicit id=0 is treated as absence; malformed or unavailable responses are
// never converted into permission to create another upstream user.
func (p *Provider) LookupPrincipal(
	ctx context.Context,
	request resourcecap.PrincipalLookupRequest,
) (*resourcecap.PrincipalLookupResult, error) {
	if err := request.Validate(); err != nil {
		return nil, err
	}
	var (
		result  *resourcecap.PrincipalLookupResult
		callErr error
	)
	err := p.accessPrincipalSecret(ctx, request.Username, func(username []byte) error {
		body, encodeErr := encodeConfidentialForm(confidentialFormPair{
			name: resourcecap.PrincipalBindingUsername, value: username,
		})
		if encodeErr != nil {
			callErr = errInvalidConfig
			return nil
		}
		defer clear(body)
		response, requestErr := p.authenticatedRequest(
			ctx, http.MethodPost, "/user/check", nil, body,
		)
		if requestErr != nil {
			callErr = normalizeReadCallError(requestErr)
			return nil
		}
		defer clear(response.body)
		if response.statusCode < 200 || response.statusCode >= 300 {
			callErr = readStatusError(response.statusCode)
			return nil
		}
		result, callErr = p.decodePrincipalLookup(request.RequestID, response.body)
		return nil
	})
	if err != nil {
		return nil, newProviderCallError(resourcecap.ProviderCallErrorCodeInternal, false, false)
	}
	return result, callErr
}

func (p *Provider) decodePrincipalLookup(
	requestID string,
	body []byte,
) (*resourcecap.PrincipalLookupResult, error) {
	object, err := decodeObject(body)
	if err != nil {
		return nil, newProviderCallError(resourcecap.ProviderCallErrorCodeInvalidResponse, false, false)
	}
	object = unwrapObject(object)
	defer clearRawObject(object)
	id, ok := metaInteger(object, "id")
	if !ok {
		return nil, newProviderCallError(resourcecap.ProviderCallErrorCodeInvalidResponse, false, false)
	}
	result := &resourcecap.PrincipalLookupResult{
		Exists:     id > 0,
		ObservedAt: p.now(),
		Evidence: []resourcecap.Evidence{{
			Kind:       "upstream_principal_lookup",
			Reference:  requestID,
			Summary:    "exact upstream username lookup returned a field-whitelisted identifier",
			ObservedAt: p.now(),
		}},
	}
	if id > 0 {
		result.ExternalPrincipalRef = strconv.Itoa(id)
		if !validNumericReference(result.ExternalPrincipalRef) {
			return nil, newProviderCallError(resourcecap.ProviderCallErrorCodeInvalidResponse, false, false)
		}
	}
	if err := result.Validate(); err != nil {
		return nil, newProviderCallError(resourcecap.ProviderCallErrorCodeInvalidResponse, false, false)
	}
	return result, nil
}

// CreatePrincipal submits exactly one creation attempt. Any ambiguous transport
// or response outcome crosses the boundary as ResultUnknown so the owner can
// permanently block replay and reconcile only through LookupPrincipal.
func (p *Provider) CreatePrincipal(
	ctx context.Context,
	request resourcecap.PrincipalCreateRequest,
) (*resourcecap.PrincipalCreateResult, error) {
	if err := request.Validate(); err != nil {
		return nil, err
	}
	bindings, err := exactPrincipalBindings(request.SecretBindings)
	if err != nil {
		return nil, err
	}
	var (
		result  *resourcecap.PrincipalCreateResult
		callErr error
	)
	err = p.accessPrincipalBindings(ctx, bindings, func(values map[string][]byte) error {
		pairs := []confidentialFormPair{
			{name: "state", value: []byte{'1'}},
			{name: resourcecap.PrincipalBindingUsername, value: values[resourcecap.PrincipalBindingUsername]},
			{name: resourcecap.PrincipalBindingPassword, value: values[resourcecap.PrincipalBindingPassword]},
		}
		for _, name := range principalBindingOrder[2:] {
			if value, exists := values[name]; exists {
				pairs = append(pairs, confidentialFormPair{name: name, value: value})
			}
		}
		body, encodeErr := encodeConfidentialForm(pairs...)
		if encodeErr != nil {
			callErr = newProviderCallError(resourcecap.ProviderCallErrorCodeInternal, false, false)
			return nil
		}
		defer clear(body)
		response, requestErr := p.authenticatedWriteRequest(ctx, http.MethodPost, "/user", nil, body)
		if requestErr != nil {
			callErr = principalWriteError(requestErr)
			return nil
		}
		defer clear(response.body)
		if response.statusCode < 200 || response.statusCode >= 300 {
			callErr = principalCreateStatusError(response.statusCode)
			return nil
		}
		result, callErr = p.decodePrincipalCreate(request.RequestID, response.body)
		return nil
	})
	if err != nil {
		return nil, newProviderCallError(resourcecap.ProviderCallErrorCodeInternal, false, false)
	}
	return result, callErr
}

func (p *Provider) decodePrincipalCreate(
	requestID string,
	body []byte,
) (*resourcecap.PrincipalCreateResult, error) {
	object, err := decodeObject(body)
	if err != nil {
		return nil, newProviderCallError(resourcecap.ProviderCallErrorCodeUnknown, false, true)
	}
	object = unwrapObject(object)
	defer clearRawObject(object)
	reference, ok := firstReference(object, "id")
	if !ok {
		return nil, newProviderCallError(resourcecap.ProviderCallErrorCodeUnknown, false, true)
	}
	result := &resourcecap.PrincipalCreateResult{
		ExternalPrincipalRef: reference,
		ObservedAt:           p.now(),
		Evidence: []resourcecap.Evidence{{
			Kind:       "upstream_principal_acceptance",
			Reference:  requestID,
			Summary:    "upstream principal creation returned a field-whitelisted identifier",
			ObservedAt: p.now(),
		}},
	}
	if err := result.Validate(); err != nil {
		return nil, newProviderCallError(resourcecap.ProviderCallErrorCodeUnknown, false, true)
	}
	return result, nil
}

func exactPrincipalBindings(
	bindings []resourcecap.SecretBinding,
) (map[string]secretcap.Handle, error) {
	if len(bindings) < 2 || len(bindings) > len(principalBindingOrder) {
		return nil, errInvalidConfig
	}
	allowed := make(map[string]struct{}, len(principalBindingOrder))
	for _, name := range principalBindingOrder {
		allowed[name] = struct{}{}
	}
	result := make(map[string]secretcap.Handle, len(bindings))
	for _, binding := range bindings {
		if _, ok := allowed[binding.Name]; !ok || binding.Handle.IsZero() {
			return nil, errInvalidConfig
		}
		if _, duplicated := result[binding.Name]; duplicated {
			return nil, errInvalidConfig
		}
		result[binding.Name] = binding.Handle
	}
	if result[resourcecap.PrincipalBindingUsername].IsZero() ||
		result[resourcecap.PrincipalBindingPassword].IsZero() {
		return nil, errInvalidConfig
	}
	return result, nil
}

func principalWriteError(err error) error {
	var callError *resourcecap.ProviderCallError
	if errors.As(err, &callError) && !callError.ResultUnknown {
		return resourcecap.NormalizeProviderError(callError)
	}
	if errors.Is(err, errAuthentication) {
		return newProviderCallError(resourcecap.ProviderCallErrorCodeAuthentication, false, false)
	}
	if errors.Is(err, errCircuitOpen) {
		return newProviderCallError(resourcecap.ProviderCallErrorCodeTransport, true, false)
	}
	code := resourcecap.ProviderCallErrorCodeTransport
	if errors.Is(err, errRequestTimeout) || errors.Is(err, context.DeadlineExceeded) {
		code = resourcecap.ProviderCallErrorCodeTimeout
	}
	return newProviderCallError(code, false, true)
}

func principalCreateStatusError(statusCode int) error {
	switch statusCode {
	case http.StatusBadRequest, http.StatusForbidden, http.StatusNotFound, http.StatusMethodNotAllowed:
		return newProviderCallError(resourcecap.ProviderCallErrorCodeInvalidResponse, false, false)
	default:
		return newProviderCallError(resourcecap.ProviderCallErrorCodeUnknown, false, true)
	}
}
