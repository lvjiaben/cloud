package admin

import (
	"context"
	"time"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/api/admin/v1"
	"lina-plugin-idc-core/backend/internal/service/serveroperation"
)

func (c *ControllerV1) ServerOperationPolicyGet(ctx context.Context,
	_ *v1.ServerOperationPolicyGetReq) (*v1.ServerOperationPolicyGetRes, error) {
	if c.serverOperationPolicy == nil {
		return nil, bizerr.NewCode(serveroperation.CodePolicyUnavailable)
	}
	policy, err := c.serverOperationPolicy.GetPolicy(ctx)
	if err != nil {
		return nil, err
	}
	return &v1.ServerOperationPolicyGetRes{Policy: toAPIServerOperationPolicy(policy)}, nil
}

func (c *ControllerV1) ServerOperationPolicyUpdate(ctx context.Context,
	req *v1.ServerOperationPolicyUpdateReq) (*v1.ServerOperationPolicyUpdateRes, error) {
	if c.serverOperationPolicy == nil {
		return nil, bizerr.NewCode(serveroperation.CodePolicyUnavailable)
	}
	policy, err := c.serverOperationPolicy.UpdatePolicy(ctx, req.RequireHighRiskReauth,
		req.AllowCustomerTransfer, req.ExpectedVersion)
	if err != nil {
		return nil, err
	}
	return &v1.ServerOperationPolicyUpdateRes{Policy: toAPIServerOperationPolicy(policy)}, nil
}

func toAPIServerOperationPolicy(policy *serveroperation.PolicyState) *v1.ServerOperationPolicy {
	if policy == nil {
		return nil
	}
	return &v1.ServerOperationPolicy{RequireHighRiskReauth: policy.RequireHighRiskReauth,
		AllowCustomerTransfer: policy.AllowCustomerTransfer, Version: policy.Version,
		UpdatedAt: policy.UpdatedAt.UTC().Format(time.RFC3339Nano)}
}
