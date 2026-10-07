// Package contractpolicy owns installation-local contract requirement rules
// and the immutable decision made when an order is created.
package contractpolicy

import (
	"context"
	"errors"
	"strings"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-plugin-idc-core/backend/cap/contracttemplatecap"
)

type Policy string

const (
	PolicyInherit  Policy = "inherit"
	PolicyNone     Policy = "none"
	PolicyRequired Policy = "required"
)

type DecisionSource string

const (
	DecisionSourceDefault    DecisionSource = "default"
	DecisionSourceEnterprise DecisionSource = "enterprise"
	DecisionSourceProduct    DecisionSource = "product"
	DecisionSourceOrder      DecisionSource = "order"
)

var (
	ErrInvalidRule      = errors.New("contract policy rule is invalid")
	ErrTemplateConflict = errors.New("contract policy resolves to multiple templates")
)

type Rule struct {
	Policy     Policy
	TemplateNo string
}

type ResolveInput struct {
	CustomerType   contracttemplatecap.CustomerType
	EnterpriseRule Rule
	ProductRules   []Rule
	OrderOverride  Rule
}

type Decision struct {
	RequiresContract bool
	Source           DecisionSource
	EnterpriseRule   Rule
	OrderOverride    Rule
	Template         contracttemplatecap.Template
}

type Resolver interface {
	Resolve(context.Context, ResolveInput) (Decision, error)
}

type resolver struct{ templates contracttemplatecap.Source }

func NewResolver(templates contracttemplatecap.Source) Resolver {
	return &resolver{templates: templates}
}

func (r *resolver) Resolve(ctx context.Context, input ResolveInput) (Decision, error) {
	if ctx == nil || ctx.Err() != nil || contracttemplatecap.ValidateCustomerType(input.CustomerType) != nil ||
		len(input.ProductRules) == 0 || len(input.ProductRules) > contracttemplatecap.MaximumTemplates {
		return Decision{}, ErrInvalidRule
	}
	enterprise, err := NormalizeRule(input.EnterpriseRule, false)
	if err != nil || (input.CustomerType == contracttemplatecap.CustomerTypeIndividual && enterprise.Policy != PolicyNone) {
		return Decision{}, ErrInvalidRule
	}
	order, err := NormalizeRule(input.OrderOverride, true)
	if err != nil {
		return Decision{}, ErrInvalidRule
	}
	products := make([]Rule, len(input.ProductRules))
	for index, rule := range input.ProductRules {
		products[index], err = NormalizeRule(rule, true)
		if err != nil {
			return Decision{}, ErrInvalidRule
		}
	}
	decision := Decision{
		Source: DecisionSourceDefault, EnterpriseRule: enterprise, OrderOverride: order,
	}
	if order.Policy == PolicyNone {
		decision.Source = DecisionSourceOrder
		return decision, nil
	}
	if order.Policy == PolicyRequired {
		return r.requiredDecision(ctx, input.CustomerType, decision, DecisionSourceOrder, order.TemplateNo)
	}
	templates := make(map[string]DecisionSource, len(products))
	explicitProductNone := false
	for _, product := range products {
		switch product.Policy {
		case PolicyRequired:
			templates[product.TemplateNo] = DecisionSourceProduct
		case PolicyNone:
			explicitProductNone = true
		case PolicyInherit:
			if enterprise.Policy == PolicyRequired {
				if _, exists := templates[enterprise.TemplateNo]; !exists {
					templates[enterprise.TemplateNo] = DecisionSourceEnterprise
				}
			}
		}
	}
	if len(templates) == 0 {
		if explicitProductNone {
			decision.Source = DecisionSourceProduct
		}
		return decision, nil
	}
	if len(templates) != 1 {
		return Decision{}, ErrTemplateConflict
	}
	for templateNo, source := range templates {
		return r.requiredDecision(ctx, input.CustomerType, decision, source, templateNo)
	}
	return Decision{}, ErrInvalidRule
}

func (r *resolver) requiredDecision(
	ctx context.Context,
	customerType contracttemplatecap.CustomerType,
	decision Decision,
	source DecisionSource,
	templateNo string,
) (Decision, error) {
	selection := contracttemplatecap.Selection{
		Level: contracttemplatecap.SelectionLevelOrder, CustomerType: customerType,
	}
	if r.templates == nil {
		return Decision{}, contracttemplatecap.ErrSourceUnavailable
	}
	resolved, err := r.templates.ResolveAvailable(ctx, selection, []string{templateNo})
	if err != nil {
		return Decision{}, err
	}
	if len(resolved) != 1 || resolved[0].TemplateNo != templateNo ||
		contracttemplatecap.ValidateTemplate(resolved[0], selection) != nil {
		return Decision{}, contracttemplatecap.ErrTemplateUnavailable
	}
	decision.RequiresContract = true
	decision.Source = source
	decision.Template = resolved[0]
	return decision, nil
}

func NormalizeRule(value Rule, allowInherit bool) (Rule, error) {
	value.TemplateNo = strings.TrimSpace(value.TemplateNo)
	validPolicy := value.Policy == PolicyNone || value.Policy == PolicyRequired ||
		(allowInherit && value.Policy == PolicyInherit)
	validTemplate := (value.Policy == PolicyRequired && value.TemplateNo != "") ||
		(value.Policy != PolicyRequired && value.TemplateNo == "")
	if !validPolicy || !validTemplate {
		return Rule{}, gerror.Wrap(ErrInvalidRule, "normalize contract policy rule")
	}
	return value, nil
}
