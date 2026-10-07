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
		return s.failed(ctx, channelNo, "configuration_incomplete", time.Since(startedAt))
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
		return s.failed(ctx, channelNo, "connection_failed", latency)
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
