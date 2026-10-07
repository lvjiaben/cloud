// This file enforces authenticated HTTP, TLS, token, concurrency, and circuit boundaries.

package zjmfcloud

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"lina-plugin-idc-core/backend/cap/secretcap"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
)

type upstreamResponse struct {
	statusCode int
	body       []byte
}

type accessToken struct {
	value      []byte
	generation int64
}

func (p *Provider) authenticatedRequest(
	ctx context.Context,
	method string,
	route string,
	query url.Values,
	body []byte,
) (upstreamResponse, error) {
	token, err := p.accessToken(ctx)
	if err != nil {
		return upstreamResponse{}, err
	}
	response, err := p.perform(ctx, method, route, query, body, token.value, maximumAPIResponseBytes)
	if response.statusCode != http.StatusUnauthorized {
		clear(token.value)
		return response, err
	}

	if invalidateErr := p.invalidateToken(ctx, token.generation); invalidateErr != nil {
		clear(token.value)
		clear(response.body)
		return upstreamResponse{}, errAuthentication
	}
	clear(token.value)
	clear(response.body)
	if err != nil {
		return upstreamResponse{}, err
	}
	refreshed, err := p.accessToken(ctx)
	if err != nil {
		return upstreamResponse{}, err
	}
	defer clear(refreshed.value)
	response, err = p.perform(ctx, method, route, query, body, refreshed.value, maximumAPIResponseBytes)
	if response.statusCode == http.StatusUnauthorized {
		if invalidateErr := p.invalidateToken(ctx, refreshed.generation); invalidateErr != nil {
			clear(response.body)
			return upstreamResponse{}, errAuthentication
		}
		clear(response.body)
		return upstreamResponse{}, errAuthentication
	}
	return response, err
}

// authenticatedWriteRequest sends an external mutation exactly once. A 401
// invalidates the cached token for a future owner-controlled attempt, but it
// never refreshes and replays the current write because acceptance is not
// safely inferable from an intermediary response.
func (p *Provider) authenticatedWriteRequest(
	ctx context.Context,
	method string,
	route string,
	query url.Values,
	body []byte,
) (upstreamResponse, error) {
	token, err := p.accessToken(ctx)
	if err != nil {
		return upstreamResponse{}, err
	}
	response, err := p.perform(ctx, method, route, query, body, token.value, maximumAPIResponseBytes)
	if response.statusCode == http.StatusUnauthorized {
		if invalidateErr := p.invalidateToken(ctx, token.generation); invalidateErr != nil {
			clear(token.value)
			return response, nil
		}
	}
	clear(token.value)
	return response, err
}

func (p *Provider) accessToken(ctx context.Context) (accessToken, error) {
	var result accessToken
	err := p.env.AuthenticationSessions.Access(ctx, resourcecap.AuthenticationSessionRequest{
		ChannelRef: p.channelRef,
		TTL:        p.settings.TokenCacheTTL,
	}, func(consume resourcecap.SecretConsumer) error {
		token, err := p.authenticate(ctx)
		if err != nil {
			return err
		}
		defer clear(token)
		return consume(token)
	}, func(token []byte, generation int64) error {
		result.value = bytes.Clone(token)
		result.generation = generation
		return nil
	})
	if err != nil || len(result.value) == 0 || result.generation <= 0 {
		clear(result.value)
		return accessToken{}, normalizeAuthenticationError(err)
	}
	return result, nil
}

func (p *Provider) authenticate(ctx context.Context) ([]byte, error) {
	purpose, err := secretcap.NewPurpose("provider.authentication")
	if err != nil {
		return nil, errAuthentication
	}
	usernameRequest, err := secretcap.NewAccessRequest(p.username, purpose)
	if err != nil {
		return nil, errAuthentication
	}
	passwordRequest, err := secretcap.NewAccessRequest(p.password, purpose)
	if err != nil {
		return nil, errAuthentication
	}

	var token []byte
	err = p.env.Secrets.Access(ctx, usernameRequest, func(username []byte) error {
		return p.env.Secrets.Access(ctx, passwordRequest, func(password []byte) error {
			body, encodeErr := encodeConfidentialForm(
				confidentialFormPair{name: "username", value: username},
				confidentialFormPair{name: "password", value: password},
			)
			if encodeErr != nil {
				return errAuthentication
			}
			defer clear(body)

			response, requestErr := p.perform(
				ctx,
				http.MethodPost,
				"/login",
				nil,
				body,
				nil,
				maximumLoginResponseBytes,
			)
			defer clear(response.body)
			if requestErr != nil {
				return requestErr
			}
			if response.statusCode < 200 || response.statusCode >= 300 {
				return authenticationStatusError(response.statusCode)
			}
			decoded, decodeErr := decodeToken(response.body)
			if decodeErr != nil {
				return decodeErr
			}
			token = decoded
			return nil
		})
	})
	if err != nil {
		clear(token)
		return nil, normalizeAuthenticationError(err)
	}
	return token, nil
}

func normalizeAuthenticationError(err error) error {
	var callError *resourcecap.ProviderCallError
	if errors.As(err, &callError) {
		return resourcecap.NormalizeProviderError(callError)
	}
	switch {
	case errors.Is(err, context.Canceled):
		return context.Canceled
	case errors.Is(err, errRequestTimeout),
		errors.Is(err, errCircuitOpen),
		errors.Is(err, errTransportFailure),
		errors.Is(err, errTransportPolicy),
		errors.Is(err, errInvalidResponse),
		errors.Is(err, errResponseTooLarge),
		errors.Is(err, context.DeadlineExceeded):
		return normalizeReadCallError(err)
	default:
		return errAuthentication
	}
}

func authenticationStatusError(statusCode int) error {
	switch {
	case statusCode == http.StatusRequestTimeout:
		return newProviderCallError(resourcecap.ProviderCallErrorCodeTimeout, true, false)
	case statusCode == http.StatusTooEarly, statusCode == http.StatusTooManyRequests,
		statusCode >= http.StatusInternalServerError:
		return newProviderCallError(resourcecap.ProviderCallErrorCodeTransport, true, false)
	case statusCode >= http.StatusBadRequest && statusCode < http.StatusInternalServerError:
		return errAuthentication
	default:
		return newProviderCallError(resourcecap.ProviderCallErrorCodeInvalidResponse, false, false)
	}
}

func decodeToken(body []byte) ([]byte, error) {
	var token string
	decoder := json.NewDecoder(bytes.NewReader(body))
	if err := decoder.Decode(&token); err != nil || decoder.Decode(&struct{}{}) != io.EOF {
		return nil, errInvalidResponse
	}
	if token == "" || len(token) > maximumTokenBytes || token != strings.TrimSpace(token) ||
		strings.ContainsAny(token, "\r\n\x00") {
		return nil, errInvalidResponse
	}
	return []byte(token), nil
}

func (p *Provider) perform(
	ctx context.Context,
	method string,
	route string,
	query url.Values,
	body []byte,
	token []byte,
	responseLimit int64,
) (upstreamResponse, error) {
	if err := p.beforeCall(blockedByOpenCircuit(method, route)); err != nil {
		return upstreamResponse{}, err
	}
	callContext, cancel := context.WithTimeout(ctx, p.settings.RequestTimeout)
	defer cancel()
	if err := p.acquire(callContext); err != nil {
		return upstreamResponse{}, err
	}
	defer p.release()

	requestURL, err := p.buildURL(route, query)
	if err != nil {
		return upstreamResponse{}, errTransportPolicy
	}
	request, err := http.NewRequestWithContext(callContext, method, requestURL.String(), bytes.NewReader(body))
	if err != nil {
		return upstreamResponse{}, errTransportPolicy
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("User-Agent", "LinaPro-ZJMFCloud/1")
	if len(body) > 0 {
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	if len(token) > 0 {
		request.Header.Set("Access-Token", string(token))
		defer request.Header.Del("Access-Token")
	}

	response, err := p.env.HTTPClient.Do(request)
	if err != nil {
		p.afterCall(true)
		if errors.Is(callContext.Err(), context.DeadlineExceeded) ||
			errors.Is(err, context.DeadlineExceeded) {
			return upstreamResponse{}, errRequestTimeout
		}
		if ctx.Err() != nil {
			return upstreamResponse{}, ctx.Err()
		}
		return upstreamResponse{}, errTransportFailure
	}
	if response == nil || response.Body == nil {
		p.afterCall(true)
		return upstreamResponse{}, errInvalidResponse
	}
	defer response.Body.Close()
	if !p.validTransportResponse(requestURL, response) {
		p.afterCall(true)
		return upstreamResponse{}, errTransportPolicy
	}

	limited := io.LimitReader(response.Body, responseLimit+1)
	responseBody, err := io.ReadAll(limited)
	if err != nil {
		p.afterCall(true)
		return upstreamResponse{}, errInvalidResponse
	}
	if int64(len(responseBody)) > responseLimit {
		clear(responseBody)
		p.afterCall(true)
		return upstreamResponse{}, errResponseTooLarge
	}
	failed := response.StatusCode == http.StatusTooManyRequests || response.StatusCode >= 500
	p.afterCall(failed)
	return upstreamResponse{
		statusCode: response.StatusCode,
		body:       responseBody,
	}, nil
}

func normalizeReadCallError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, context.Canceled) {
		return context.Canceled
	}
	var callError *resourcecap.ProviderCallError
	if errors.As(err, &callError) {
		return resourcecap.NormalizeProviderError(callError)
	}
	switch {
	case errors.Is(err, errAuthentication):
		return newProviderCallError(resourcecap.ProviderCallErrorCodeAuthentication, false, false)
	case errors.Is(err, errRequestTimeout), errors.Is(err, context.DeadlineExceeded):
		return newProviderCallError(resourcecap.ProviderCallErrorCodeTimeout, true, false)
	case errors.Is(err, errCircuitOpen), errors.Is(err, errTransportFailure):
		return newProviderCallError(resourcecap.ProviderCallErrorCodeTransport, true, false)
	case errors.Is(err, errTransportPolicy):
		return newProviderCallError(resourcecap.ProviderCallErrorCodeTransport, false, false)
	case errors.Is(err, errInvalidResponse), errors.Is(err, errResponseTooLarge),
		errors.Is(err, errUnexpectedStatus):
		return newProviderCallError(resourcecap.ProviderCallErrorCodeInvalidResponse, false, false)
	default:
		return newProviderCallError(resourcecap.ProviderCallErrorCodeInternal, false, false)
	}
}

func readStatusError(statusCode int) error {
	switch {
	case statusCode == http.StatusRequestTimeout:
		return newProviderCallError(resourcecap.ProviderCallErrorCodeTimeout, true, false)
	case statusCode == http.StatusTooEarly, statusCode == http.StatusTooManyRequests,
		statusCode >= http.StatusInternalServerError:
		return newProviderCallError(resourcecap.ProviderCallErrorCodeTransport, true, false)
	default:
		return newProviderCallError(resourcecap.ProviderCallErrorCodeInvalidResponse, false, false)
	}
}

func (p *Provider) buildURL(route string, query url.Values) (*url.URL, error) {
	if route == "" || route[0] != '/' || strings.Contains(route, "..") || strings.ContainsAny(route, "?#") {
		return nil, errTransportPolicy
	}
	result := *p.endpoint
	result.Path = p.endpoint.Path + route
	result.RawQuery = query.Encode()
	return &result, nil
}

func (p *Provider) validTransportResponse(expected *url.URL, response *http.Response) bool {
	if response.Request == nil || response.Request.URL == nil || response.TLS == nil ||
		len(response.TLS.VerifiedChains) == 0 || len(response.TLS.PeerCertificates) == 0 ||
		response.TLS.PeerCertificates[0] == nil {
		return false
	}
	actual := response.Request.URL
	leaf := response.TLS.PeerCertificates[0]
	if leaf.VerifyHostname(expected.Hostname()) != nil {
		return false
	}
	chainMatchesPeer := false
	for _, chain := range response.TLS.VerifiedChains {
		if len(chain) > 0 && chain[0] != nil && bytes.Equal(chain[0].Raw, leaf.Raw) {
			chainMatchesPeer = true
			break
		}
	}
	return chainMatchesPeer && actual.Scheme == "https" &&
		strings.EqualFold(actual.Host, expected.Host) &&
		actual.EscapedPath() == expected.EscapedPath() &&
		actual.RawQuery == expected.RawQuery &&
		actual.Fragment == "" && actual.User == nil
}

func (p *Provider) acquire(ctx context.Context) error {
	select {
	case p.concurrency <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (p *Provider) release() {
	<-p.concurrency
}

func (p *Provider) beforeCall(blockWhenOpen bool) error {
	p.breakerMu.Lock()
	defer p.breakerMu.Unlock()
	if p.circuitOpenUntil.IsZero() {
		return nil
	}
	if p.now().Before(p.circuitOpenUntil) {
		if blockWhenOpen {
			return errCircuitOpen
		}
		return nil
	}
	p.circuitOpenUntil = time.Time{}
	p.consecutiveFailure = 0
	return nil
}

// blockedByOpenCircuit keeps authentication and read-only reconciliation
// available while preventing new upstream mutations during a protection window.
func blockedByOpenCircuit(method string, route string) bool {
	if route == "/login" {
		return false
	}
	// Although the upstream exposes exact username lookup as POST, it is a
	// read-only reconciliation call and must remain available while writes are
	// blocked by the circuit breaker.
	if method == http.MethodPost && route == "/user/check" {
		return false
	}
	if method != http.MethodGet {
		return true
	}
	if reference, ok := strings.CutPrefix(route, "/tasks/"); ok {
		return !validNumericReference(reference)
	}
	if reference, ok := strings.CutPrefix(route, "/clouds/"); ok {
		return !validNumericReference(reference)
	}
	return true
}

func (p *Provider) afterCall(failed bool) {
	p.breakerMu.Lock()
	defer p.breakerMu.Unlock()
	if !failed {
		if !p.circuitOpenUntil.IsZero() && p.now().Before(p.circuitOpenUntil) {
			return
		}
		p.consecutiveFailure = 0
		p.circuitOpenUntil = time.Time{}
		return
	}
	p.consecutiveFailure++
	if p.consecutiveFailure >= p.settings.CircuitFailures {
		p.circuitOpenUntil = p.now().Add(p.settings.CircuitOpenDuration)
	}
}
