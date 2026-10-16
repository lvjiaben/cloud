// This generated controller delegates staff product edits to the existing authorized owner workflow.
package admin

import (
	"context"
	account "lina-plugin-idc-core/backend/api/account/v1"

	"lina-plugin-idc-core/backend/api/admin/v1"
)

func (c *ControllerV1) CustomerServerResizeOptions(ctx context.Context, req *v1.CustomerServerResizeOptionsReq) (res *v1.CustomerServerResizeOptionsRes, err error) {
	value, err := c.accessoryPricing.ResizeOptionsForStaff(ctx, req.ServiceNo)
	if err != nil {
		return nil, err
	}
	result := &account.ServerResizeOptionsRes{PublicSlug: value.PublicSlug, CurrencyCode: value.CurrencyCode, Revision: value.Revision,
		Groups: make([]*account.ServerResizeGroup, 0, len(value.Groups)), Rules: make([]*account.ServerResizeRule, 0, len(value.Rules)),
		CurrentSelections: make([]*account.ServerResizeSelection, 0, len(value.CurrentSelections))}
	for _, group := range value.Groups {
		mapped := &account.ServerResizeGroup{GroupNo: group.GroupNo, SelectionMode: string(group.SelectionMode), NameZhCN: group.NameZhCN,
			NameEnUS: group.NameEnUS, MinimumSelected: group.MinimumSelected, MaximumSelected: group.MaximumSelected, Options: make([]*account.ServerResizeOption, 0, len(group.Options))}
		for _, option := range group.Options {
			mapped.Options = append(mapped.Options, &account.ServerResizeOption{OptionNo: option.OptionNo, ComponentType: option.ComponentType,
				NameZhCN: option.NameZhCN, NameEnUS: option.NameEnUS, MinimumQuantity: option.MinimumQuantity, MaximumQuantity: option.MaximumQuantity, DefaultQuantity: option.DefaultQuantity})
		}
		result.Groups = append(result.Groups, mapped)
	}
	for _, rule := range value.Rules {
		result.Rules = append(result.Rules, &account.ServerResizeRule{RuleType: string(rule.RuleType), SourceOptionNo: rule.SourceOptionNo,
			TargetOptionNo: rule.TargetOptionNo, MinimumTargetQuantity: rule.MinimumTargetQuantity, MaximumTargetQuantity: rule.MaximumTargetQuantity})
	}
	for _, selection := range value.CurrentSelections {
		result.CurrentSelections = append(result.CurrentSelections, &account.ServerResizeSelection{OptionNo: selection.OptionNo, Quantity: selection.Quantity})
	}
	return result, nil
}
