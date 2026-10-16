// Package providerverification owns explicit, read-only upstream connection checks.
package providerverification

import (
	"context"
	"strings"
	"time"

	"github.com/gogf/gf/v2/errors/gcode"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/managementcap"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
	"lina-plugin-idc-resource/backend/internal/service/providerhealth"
)

const permissionVerify = "idc:admin:resource:configuration:write"

// Reviewed connection outcomes contain no supplier-controlled text.
const (
	statusTLSUntrusted     = "tls_untrusted"
	statusTimeout          = "timeout"
	statusDNS              = "dns"
	statusNetwork          = "network"
	statusEndpointNotFound = "endpoint_not_found"
	statusInvalidResponse  = "invalid_response"
	statusAuthRejected     = "auth_rejected"
	statusLocalAuthPolicy  = "local_auth_policy"
)

var (
	CodeNotAccessible = bizerr.MustDefine("IDC_RESOURCE_PROVIDER_VERIFICATION_NOT_ACCESSIBLE", "The Provider connection verification target is not accessible", gcode.CodeNotAuthorized)
	CodeUnavailable   = bizerr.MustDefine("IDC_RESOURCE_PROVIDER_VERIFICATION_UNAVAILABLE", "The Provider connection verification service is temporarily unavailable", gcode.CodeInternalError)
)

type Result struct {
	Verified   bool
	StatusCode string
	ObservedAt time.Time
}

type configuredConstructor interface {
	ConstructConfiguredSession(context.Context, string) (resourcecap.Provider, func(), error)
}

type connectionVerifier interface {
	VerifyConnection(context.Context) (*resourcecap.ConnectionVerification, error)
}

type Service interface {
	Verify(context.Context, string) (Result, error)
}

type service struct {
	runtime     managementcap.Runtime
	providers   configuredConstructor
	health      providerhealth.Service
	callTimeout time.Duration
}

func New(runtime managementcap.Runtime, providers configuredConstructor, health providerhealth.Service) (Service, error) {
	if managementcap.ValidateRuntime(runtime) != nil || providers == nil || health == nil {
		return nil, gerror.New("resource Provider verification dependencies are invalid")
	}
	return &service{runtime: runtime, providers: providers, health: health, callTimeout: 20 * time.Second}, nil
}

func (s *service) Verify(ctx context.Context, channelNo string) (Result, error) {
	channelNo = strings.TrimSpace(channelNo)
	if ctx == nil || ctx.Err() != nil || resourcecap.ValidateChannelRef(channelNo) != nil {
		return Result{}, bizerr.NewCode(CodeNotAccessible)
	}
	actor, err := s.runtime.Authorizer.Authorize(ctx, managementcap.AuthorizationRequest{
		Permission: permissionVerify,
		TargetNo:   channelNo,
	})
	if err != nil || actor <= 0 {
		return Result{}, bizerr.NewCode(CodeNotAccessible)
	}
	startedAt := time.Now()
	provider, cleanup, err := s.providers.ConstructConfiguredSession(ctx, channelNo)
	if err != nil || cleanup == nil {
		status := connectionFailureStatus(err)
		if status == "connection_failed" {
			status = "configuration_incomplete"
		}
		return s.failed(ctx, channelNo, status, time.Since(startedAt))
	}
	defer cleanup()
	verifier, ok := provider.(connectionVerifier)
	if !ok {
		return s.failed(ctx, channelNo, "verification_unsupported", time.Since(startedAt))
	}
	callCtx, cancel := context.WithTimeout(ctx, s.callTimeout)
	proof, callErr := verifier.VerifyConnection(callCtx)
	cancel()
	latency := time.Since(startedAt)
	if callErr != nil || proof == nil || proof.ValidateAt(time.Now().UTC()) != nil {
		status := connectionFailureStatus(callErr)
		if callErr == nil {
			status = statusInvalidResponse
		}
		return s.failed(ctx, channelNo, status, latency)
	}
	if err = s.health.Record(ctx, providerhealth.Observation{
		ChannelNo: channelNo, Operation: providerhealth.OperationAuthentication,
		Outcome: providerhealth.OutcomeSucceeded, SummaryCode: "verified", Latency: latency,
	}); err != nil {
		return Result{}, bizerr.WrapCode(err, CodeUnavailable)
	}
	return Result{Verified: true, StatusCode: "verified", ObservedAt: proof.ObservedAt.UTC()}, nil
}

func (s *service) failed(ctx context.Context, channelNo, statusCode string, latency time.Duration) (Result, error) {
	if err := s.health.Record(ctx, providerhealth.Observation{
		ChannelNo: channelNo, Operation: providerhealth.OperationAuthentication,
		Outcome: providerhealth.OutcomeFailed, SummaryCode: statusCode, Latency: latency,
	}); err != nil {
		return Result{}, bizerr.WrapCode(err, CodeUnavailable)
	}
	return Result{Verified: false, StatusCode: statusCode, ObservedAt: time.Now().UTC()}, nil
}

// connectionFailureStatus preserves only reviewed owner diagnostics, never raw
// error text. Unknown plugin failures retain the generic legacy status.
func connectionFailureStatus(err error) string {
	for _, candidate := range []struct {
		code   *bizerr.Code
		status string
	}{
		{resourcecap.CodeProviderTLSUntrusted, statusTLSUntrusted},
		{resourcecap.CodeProviderTimeout, statusTimeout},
		{resourcecap.CodeProviderDNS, statusDNS},
		{resourcecap.CodeProviderNetwork, statusNetwork},
		{resourcecap.CodeProviderEndpointNotFound, statusEndpointNotFound},
		{resourcecap.CodeProviderInvalidResponse, statusInvalidResponse},
		{resourcecap.CodeProviderAuthRejected, statusAuthRejected},
		{resourcecap.CodeProviderLocalAuthPolicy, statusLocalAuthPolicy},
	} {
		if bizerr.Is(err, candidate.code) {
			return candidate.status
		}
	}
	return "connection_failed"
}
