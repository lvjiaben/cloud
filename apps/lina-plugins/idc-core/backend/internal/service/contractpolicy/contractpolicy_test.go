package contractpolicy

import (
	"context"
	"errors"
	"testing"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/contracttemplatecap"
)

type templateSourceFake struct {
	templates      map[string]contracttemplatecap.Template
	orderContracts map[string]bool
	existenceErr   error
}

func (f templateSourceFake) ListAvailable(
	context.Context,
	contracttemplatecap.Selection,
) ([]contracttemplatecap.Template, error) {
	return nil, nil
}

func (f templateSourceFake) ResolveAvailable(
	_ context.Context,
	selection contracttemplatecap.Selection,
	templateNos []string,
) ([]contracttemplatecap.Template, error) {
	result := make([]contracttemplatecap.Template, 0, len(templateNos))
	for _, templateNo := range templateNos {
		template, ok := f.templates[templateNo]
		if !ok || contracttemplatecap.ValidateTemplate(template, selection) != nil {
			return nil, contracttemplatecap.ErrTemplateUnavailable
		}
		result = append(result, template)
	}
	return result, nil
}

func (f templateSourceFake) OrderContractExists(_ context.Context, orderNo string) (bool, error) {
	return f.orderContracts[orderNo], f.existenceErr
}

func TestOptionalContractPermanentlyLocksNoContractDecision(t *testing.T) {
	service := &managementService{templates: templateSourceFake{
		orderContracts: map[string]bool{"ORDER_OPTIONAL_LOCKED": true},
	}}
	if err := service.ensureOrderCanRequireContract(
		context.Background(), "ORDER_OPTIONAL_LOCKED",
	); !bizerr.Is(err, CodeManagementOptionalContractLocked) {
		t.Fatalf("optional contract lock error=%v", err)
	}
	if err := service.ensureOrderCanRequireContract(
		context.Background(), "ORDER_WITHOUT_CONTRACT",
	); err != nil {
		t.Fatalf("order without a contract was locked: %v", err)
	}
}

func TestResolverPrecedenceAndConflict(t *testing.T) {
	all := contractTemplate("ctpl_0123456789abcdef0123456789abcdef", "all_customers")
	second := contractTemplate("ctpl_abcdef0123456789abcdef0123456789", "all_customers")
	resolver := NewResolver(templateSourceFake{templates: map[string]contracttemplatecap.Template{
		all.TemplateNo: all, second.TemplateNo: second,
	}})
	base := ResolveInput{
		CustomerType:   contracttemplatecap.CustomerTypeEnterprise,
		EnterpriseRule: Rule{Policy: PolicyRequired, TemplateNo: all.TemplateNo},
		ProductRules:   []Rule{{Policy: PolicyInherit}, {Policy: PolicyNone}},
		OrderOverride:  Rule{Policy: PolicyInherit},
	}
	decision, err := resolver.Resolve(context.Background(), base)
	if err != nil || !decision.RequiresContract || decision.Source != DecisionSourceEnterprise ||
		decision.Template.TemplateNo != all.TemplateNo {
		t.Fatalf("enterprise decision=%+v err=%v", decision, err)
	}
	base.ProductRules = []Rule{
		{Policy: PolicyRequired, TemplateNo: all.TemplateNo},
		{Policy: PolicyRequired, TemplateNo: second.TemplateNo},
	}
	if _, err = resolver.Resolve(context.Background(), base); !errors.Is(err, ErrTemplateConflict) {
		t.Fatalf("multiple product templates error=%v", err)
	}
	base.OrderOverride = Rule{Policy: PolicyRequired, TemplateNo: second.TemplateNo}
	decision, err = resolver.Resolve(context.Background(), base)
	if err != nil || decision.Source != DecisionSourceOrder || decision.Template.TemplateNo != second.TemplateNo {
		t.Fatalf("order override decision=%+v err=%v", decision, err)
	}
	base.OrderOverride = Rule{Policy: PolicyNone}
	decision, err = resolver.Resolve(context.Background(), base)
	if err != nil || decision.RequiresContract || decision.Source != DecisionSourceOrder {
		t.Fatalf("order none decision=%+v err=%v", decision, err)
	}
}

func TestResolverFailsClosed(t *testing.T) {
	resolver := NewResolver(nil)
	input := ResolveInput{
		CustomerType:   contracttemplatecap.CustomerTypeIndividual,
		EnterpriseRule: Rule{Policy: PolicyNone},
		ProductRules: []Rule{{
			Policy: PolicyRequired, TemplateNo: "ctpl_0123456789abcdef0123456789abcdef",
		}},
		OrderOverride: Rule{Policy: PolicyInherit},
	}
	if _, err := resolver.Resolve(context.Background(), input); !errors.Is(err, contracttemplatecap.ErrSourceUnavailable) {
		t.Fatalf("missing template source error=%v", err)
	}
	input.ProductRules = []Rule{{Policy: PolicyInherit}}
	decision, err := resolver.Resolve(context.Background(), input)
	if err != nil || decision.RequiresContract {
		t.Fatalf("no-contract decision=%+v err=%v", decision, err)
	}
}

func contractTemplate(templateNo, scope string) contracttemplatecap.Template {
	return contracttemplatecap.Template{
		TemplateNo: templateNo, VersionNo: 3, Name: "Agreement", ScopeCode: scope,
		SigningOrderCode: "customer_first", LanguageCode: "en-US",
	}
}
