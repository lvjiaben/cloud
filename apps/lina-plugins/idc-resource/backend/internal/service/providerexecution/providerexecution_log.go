// This file persists only bounded invocation coordinates, never input or result bodies.
package providerexecution

import (
	"context"
	"lina-core/pkg/logger"
	"lina-core/pkg/plugin/pluginhost"
	"lina-plugin-idc-resource/backend/internal/model/do"
	"time"
)

// recordCall uses an independent bounded context so a failed business transaction
// cannot erase the call history. Log failures never change or repeat the call.
func (r *registrar) recordCall(ctx context.Context, input pluginhost.ConfidentialOutboundContextInput, started time.Time, invokeErr error, code callOutcomeCode) {
	if r.database == nil {
		return
	}
	// Error text can contain provider secrets; retain only an owner-defined class.
	if invokeErr != nil {
		code = callOutcomeRuntimeFailed
	}
	// Deliberately detach the transaction and cancellation, with a strict timeout.
	logCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, err := r.database.Model(do.ProviderApiLog{}).Ctx(logCtx).Data(do.ProviderApiLog{
		TenantId: 0, ChannelNo: input.InterfaceInstanceID, ProviderPluginId: input.ProviderPluginID,
		MethodCode: input.ProviderMethod, Succeeded: code == callOutcomeCompleted, ErrorCode: string(code),
		DurationMs: time.Since(started).Milliseconds(), StartedAt: &started,
	}).Insert()
	if err != nil {
		logger.Warningf(ctx, "provider API history could not be recorded")
	}
}
