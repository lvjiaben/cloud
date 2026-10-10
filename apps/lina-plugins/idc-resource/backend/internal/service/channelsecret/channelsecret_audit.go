// This file records secret-free channel lifecycle evidence in the owner transaction.

package channelsecret

import (
	"context"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/cap/managementcap"
	"lina-plugin-idc-core/backend/cap/secretcap"
	"strconv"
)

const (
	auditScopePrefix         = "resource.channel-secret."
	auditCompleteSuffix      = ".complete"
	auditTargetChannelSecret = "resource_channel_secret"
	auditResultSucceeded     = "succeeded"
	auditRiskHigh            = "high"
)

// appendBindingAudit persists only lifecycle metadata, never a secret handle or value.
func (s *serviceImpl) appendBindingAudit(ctx context.Context, operation Operation, before, after *bindingRow) error {
	key := after.BindingNo + ":" + strconv.FormatInt(after.Version, 10)
	return s.runtime.Audit.AppendBusinessTransition(ctx, idccap.BusinessAuditTransition{
		IdempotencyScope: auditScopePrefix + string(operation), IdempotencyKey: key,
		OperationCode: auditScopePrefix + string(operation) + auditCompleteSuffix,
		TargetType:    auditTargetChannelSecret, TargetNo: after.ChannelNo,
		ResultCode: auditResultSucceeded, RiskCode: auditRiskHigh, RequiredPermission: PermissionManage,
		BeforeSnapshot: bindingAuditSnapshot(before), AfterSnapshot: bindingAuditSnapshot(after),
	})
}

func bindingAuditSnapshot(row *bindingRow) map[string]any {
	if row == nil {
		return map[string]any{}
	}
	return map[string]any{"channelNo": row.ChannelNo, "bindingName": row.BindingName, "statusCode": row.StatusCode, "version": row.Version}
}

// appendRotationAudit records aggregate progress under the consumed operation proof.
func (s *serviceImpl) appendRotationAudit(ctx context.Context, channelNo, accessContext string, result secretcap.RotationResult) error {
	return s.runtime.Audit.AppendBusinessTransition(ctx, idccap.BusinessAuditTransition{
		IdempotencyScope: auditScopePrefix + string(OperationRotate), IdempotencyKey: managementcap.DigestSensitiveAccessTarget([]byte(accessContext)),
		OperationCode: auditScopePrefix + string(OperationRotate) + auditCompleteSuffix, TargetType: auditTargetChannelSecret, TargetNo: channelNo,
		ResultCode: auditResultSucceeded, RiskCode: auditRiskHigh, RequiredPermission: PermissionManage,
		BeforeSnapshot: map[string]any{}, AfterSnapshot: map[string]any{"rewrapped": result.Rewrapped, "remaining": result.Remaining},
	})
}
