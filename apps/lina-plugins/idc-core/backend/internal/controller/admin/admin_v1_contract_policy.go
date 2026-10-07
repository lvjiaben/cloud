package admin

import (
	"context"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/api/admin/v1"
	"lina-plugin-idc-core/backend/cap/contracttemplatecap"
	"lina-plugin-idc-core/backend/internal/service/commerce"
	"lina-plugin-idc-core/backend/internal/service/contractpolicy"
)

func (c *ControllerV1) ContractPolicyTemplateList(
	ctx context.Context,
	req *v1.ContractPolicyTemplateListReq,
) (*v1.ContractPolicyTemplateListRes, error) {
	if c.contractPolicies == nil {
		return nil, bizerr.NewCode(contractpolicy.CodeManagementUnavailable)
	}
	items, err := c.contractPolicies.ListTemplates(ctx, contractpolicy.TemplateListInput{
		Level: contracttemplatecap.SelectionLevel(req.Level), TargetNo: req.TargetNo,
		CustomerNo: req.CustomerNo,
	})
	if err != nil {
		return nil, err
	}
	templates := make([]*v1.ContractPolicyTemplate, 0, len(items))
	for _, item := range items {
		templates = append(templates, toAPIContractPolicyTemplate(item))
	}
	return &v1.ContractPolicyTemplateListRes{Templates: templates}, nil
}

func (c *ControllerV1) CustomerContractPolicyUpdate(
	ctx context.Context,
	req *v1.CustomerContractPolicyUpdateReq,
) (*v1.CustomerContractPolicyUpdateRes, error) {
	if c.contractPolicies == nil {
		return nil, bizerr.NewCode(contractpolicy.CodeManagementUnavailable)
	}
	state, err := c.contractPolicies.UpdateEnterpriseRule(ctx, contractpolicy.RuleUpdateInput{
		TargetNo: req.CustomerNo, ExpectedVersion: req.ExpectedVersion,
		Rule: contractpolicy.Rule{Policy: contractpolicy.Policy(req.PolicyCode), TemplateNo: req.TemplateNo},
	})
	if err != nil {
		return nil, err
	}
	return &v1.CustomerContractPolicyUpdateRes{Rule: toAPIContractPolicyRule(state)}, nil
}

func (c *ControllerV1) CatalogContractPolicyUpdate(
	ctx context.Context,
	req *v1.CatalogContractPolicyUpdateReq,
) (*v1.CatalogContractPolicyUpdateRes, error) {
	if c.contractPolicies == nil {
		return nil, bizerr.NewCode(contractpolicy.CodeManagementUnavailable)
	}
	state, err := c.contractPolicies.UpdateProductRule(ctx, contractpolicy.RuleUpdateInput{
		TargetNo: req.VersionNo, ExpectedVersion: req.ExpectedVersion,
		Rule: contractpolicy.Rule{Policy: contractpolicy.Policy(req.PolicyCode), TemplateNo: req.TemplateNo},
	})
	if err != nil {
		return nil, err
	}
	return &v1.CatalogContractPolicyUpdateRes{Rule: toAPIContractPolicyRule(state)}, nil
}

func (c *ControllerV1) CommerceOrderContractPolicyUpdate(
	ctx context.Context,
	req *v1.CommerceOrderContractPolicyUpdateReq,
) (*v1.CommerceOrderContractPolicyUpdateRes, error) {
	if c.contractPolicies == nil {
		return nil, bizerr.NewCode(contractpolicy.CodeManagementUnavailable)
	}
	state, err := c.contractPolicies.UpdateOrderRule(ctx, contractpolicy.RuleUpdateInput{
		TargetNo: req.OrderNo, ExpectedVersion: req.ExpectedVersion,
		Rule: contractpolicy.Rule{Policy: contractpolicy.Policy(req.PolicyCode), TemplateNo: req.TemplateNo},
	})
	if err != nil {
		return nil, err
	}
	result := &v1.ContractPolicyOrderRule{
		ContractPolicyRule: *toAPIContractPolicyRule(&state.RuleState),
		RequiresContract:   state.RequiresContract, DecisionSource: string(state.DecisionSource),
	}
	if state.RequiresContract {
		result.Template = toAPIContractPolicyTemplate(state.Template)
	}
	return &v1.CommerceOrderContractPolicyUpdateRes{Rule: result}, nil
}

func toAPIContractPolicyRule(state *contractpolicy.RuleState) *v1.ContractPolicyRule {
	if state == nil {
		return nil
	}
	return &v1.ContractPolicyRule{
		PolicyCode: string(state.Rule.Policy), TemplateNo: state.Rule.TemplateNo, Version: state.Version,
	}
}

func toAPIContractPolicyTemplate(item contracttemplatecap.Template) *v1.ContractPolicyTemplate {
	return &v1.ContractPolicyTemplate{
		TemplateNo: item.TemplateNo, VersionNo: item.VersionNo, Name: item.Name,
		ScopeCode: item.ScopeCode, SigningOrderCode: item.SigningOrderCode,
		LanguageCode: item.LanguageCode,
	}
}

func toAPIOrderContractRule(summary commerce.OrderSummary) *v1.ContractPolicyOrderRule {
	result := &v1.ContractPolicyOrderRule{
		ContractPolicyRule: v1.ContractPolicyRule{
			PolicyCode: summary.ContractOverridePolicyCode,
			TemplateNo: summary.ContractOverrideTemplateNo,
			Version:    summary.Version,
		},
		RequiresContract: summary.RequiresContract,
		DecisionSource:   summary.ContractDecisionSourceCode,
	}
	if summary.RequiresContract {
		result.Template = &v1.ContractPolicyTemplate{
			TemplateNo:       summary.ContractTemplateNo,
			VersionNo:        summary.ContractTemplateVersionNo,
			Name:             summary.ContractTemplateName,
			ScopeCode:        summary.ContractScopeCode,
			SigningOrderCode: summary.ContractSigningOrderCode,
			LanguageCode:     summary.ContractLanguageCode,
		}
	}
	return result
}
