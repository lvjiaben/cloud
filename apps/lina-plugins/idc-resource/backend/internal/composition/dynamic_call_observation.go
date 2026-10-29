// This file holds invocation-local owner observations. It captures only reviewed
// classifications and authenticated observation time before guest error masking.
package composition

import (
	"context"
	"lina-core/pkg/bizerr"
	"lina-core/pkg/logger"
	"lina-core/pkg/plugin/pluginhost"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
	"sync"
	"time"
)

type dynamicCallObservation struct {
	mu                 sync.Mutex
	failure            error
	authenticatedAt    time.Time
	handedOff          bool
	evidence           []resourcecap.Evidence
	anyBusinessAttempt bool
}

// businessHTTPAttemptObserved is an owner-internal signal, never persisted or
// exposed to a guest. It cannot be dropped when the diagnostic list is full.
const businessHTTPAttemptObserved = "owner_business_http_attempt_observed"

func (o *dynamicCallObservation) observe(item resourcecap.Evidence) {
	o.mu.Lock()
	defer o.mu.Unlock()
	if item.Kind == businessHTTPAttemptObserved {
		o.anyBusinessAttempt = true
		return
	}
	if len(o.evidence) < 4 && item.Validate() == nil {
		o.evidence = append(o.evidence, item)
	}
}

func (o *dynamicCallObservation) execute(method string, handler pluginhost.ConfidentialOutboundHandler, callCtx context.Context, request *pluginhost.ConfidentialOutboundRequest) (*pluginhost.ConfidentialOutboundResult, error) {
	if isBusinessHTTPInstruction(request) {
		o.mu.Lock()
		o.handedOff = true
		o.mu.Unlock()
	}
	result, callErr := handler(callCtx, request)
	if method == resourcecap.DynamicProviderMethodVerifyConnection {
		logger.Debugf(callCtx, "Provider owner connection check completed success=%t authenticated=%t", callErr == nil, confirmedAuthenticationStep(request, result))
		if metadata, ok := bizerr.As(callErr); ok {
			logger.Debugf(callCtx, "Provider owner connection check rejected code=%s", metadata.RuntimeCode())
		}
	}
	// The guest clock is isolated and cannot establish wall-clock freshness.
	// Only an owner-confirmed successful authentication in this invocation
	// can supply the observation time for a connection proof.
	if method == resourcecap.DynamicProviderMethodVerifyConnection && callErr == nil && confirmedAuthenticationStep(request, result) {
		o.mu.Lock()
		o.authenticatedAt = time.Now().UTC()
		o.mu.Unlock()
	}
	if safe := safeConfidentialProviderError(callErr); safe != nil {
		o.mu.Lock()
		o.failure = safe
		o.mu.Unlock()
	}
	return result, callErr
}
