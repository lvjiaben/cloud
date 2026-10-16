// This file scopes public lifecycle progress reads to the authenticated application.
package channel

import (
	"context"
	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/channelcommercecap"
)

// GetLifecycle keeps application status checks ahead of the core-owned read.
func (s *service) GetLifecycle(ctx context.Context, channelID, serviceNo, operationNo string) (*channelcommercecap.LifecycleResult, error) {
	if _, err := s.requireChannel(ctx, channelID, false); err != nil {
		return nil, err
	}
	return s.core.GetLifecycle(ctx, channelID, serviceNo, operationNo)
}

// ReadRenewalByRequest preserves application status and bound customer checks.
func (s *service) ReadRenewalByRequest(ctx context.Context, appID, serviceNo, requestID string) (*channelcommercecap.RenewalRequestResult, error) {
	if _, err := s.requireChannel(ctx, appID, false); err != nil {
		return nil, err
	}
	runtime, ok := s.core.(channelcommercecap.RenewalRequestRuntime)
	if !ok || runtime == nil {
		return nil, bizerr.NewCode(codeLifecycleInvalid)
	}
	return runtime.ReadRenewalByRequest(ctx, appID, serviceNo, requestID)
}
