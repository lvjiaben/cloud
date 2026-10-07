package account

import (
	"context"

	"lina-plugin-idc-core/backend/api/account/v1"
	"lina-plugin-idc-core/backend/internal/service/renewal"
)

func (c *ControllerV1) FinancialRenewalPolicyUpdate(ctx context.Context, req *v1.FinancialRenewalPolicyUpdateReq) (res *v1.FinancialRenewalPolicyUpdateRes, err error) {
	policy, err := c.renewals.ConfigurePolicyCurrentCustomer(ctx, renewal.PolicyInput{
		EntitlementNo: req.EntitlementNo, ExpectedEntitlementVersion: req.ExpectedEntitlementVersion,
		AutoRenewEnabled: req.AutoRenewEnabled, RenewalCycles: req.RenewalCycles,
		LeadSeconds: req.LeadSeconds, MaximumAttempts: req.MaximumAttempts,
		RetryIntervalSeconds:       req.RetryIntervalSeconds,
		FailureNotificationEnabled: req.FailureNotificationEnabled,
	})
	if err != nil {
		return nil, err
	}
	return &v1.FinancialRenewalPolicyUpdateRes{Policy: toAPIFinancialRenewalPolicy(policy)}, nil
}
