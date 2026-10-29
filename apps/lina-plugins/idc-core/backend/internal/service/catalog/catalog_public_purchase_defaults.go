// Interactive defaults are presentation convenience, not permission to alter
// a quote or order. They reuse one resolved directory and evaluate locally.
package catalog

import (
	"context"
	"slices"

	"github.com/gogf/gf/v2/errors/gerror"
	"lina-core/pkg/plugin/capability/capmodel"
	"lina-plugin-idc-core/backend/internal/service/configurationcontract"
)

// Initial reads and interactive changes may choose the first eligible option.
// Quote, cart and order freezing keep the strict evaluator and never repair a
// caller's invalid combination. The bounded loop performs no additional I/O.
func (s *publicCatalogReaderImpl) evaluateInteractiveStructuredConfiguration(ctx context.Context, snapshot *publicStructuredConfigurationSnapshot, values []configurationcontract.ConfigurationValue) (*configurationcontract.Evaluation, configurationcontract.ResolvedCandidates, error) {
	evaluation, resolved, err := s.evaluateStructuredConfiguration(ctx, snapshot, values)
	if err != nil || snapshot.Schema == nil {
		return evaluation, resolved, err
	}
	presentations, err := configurationcontract.DecodePresentations(snapshot.Target.CustomerPresentation)
	if err != nil {
		return nil, nil, err
	}
	values, err = configurationcontract.ApplyHiddenDefaults(snapshot.Schema, presentations, values)
	if err != nil {
		return nil, nil, err
	}
	changedAny := false
	for pass := 0; pass <= len(snapshot.Schema.Fields); pass++ {
		next, changed := reconcileInteractiveChoices(snapshot.Schema, presentations, evaluation, values)
		if !changed {
			// A legacy dynamic method may consume current selections. Refresh
			// once using the final values rather than attesting an old query.
			// The local fixed-point loop itself never repeats directory reads.
			if changedAny && schemaHasDynamicCandidates(snapshot.Schema) {
				return s.evaluateStructuredConfiguration(ctx, snapshot, values)
			}
			return evaluation, resolved, nil
		}
		changedAny = true
		values = next
		evaluation, err = evaluateStructuredResolved(snapshot, values, resolved)
		if err != nil {
			return nil, nil, err
		}
	}
	return nil, nil, gerror.New("interactive purchase choices did not stabilize")
}

func reconcileInteractiveChoices(schema *capmodel.ConfigurationSchemaDeclaration, presentations map[string]configurationcontract.Presentation, evaluation *configurationcontract.Evaluation, values []configurationcontract.ConfigurationValue) ([]configurationcontract.ConfigurationValue, bool) {
	var (
		current = make(map[string]configurationcontract.ConfigurationValue, len(values))
		next    = make([]configurationcontract.ConfigurationValue, 0, len(schema.Fields))
		changed bool
	)
	for _, value := range values {
		current[value.FieldKey] = value
	}
	for _, field := range schema.Fields {
		state := evaluation.States[field.Key]
		value, exists := current[field.Key]
		if !state.Visible || state.Disabled {
			changed = changed || exists
			continue
		}
		if field.Type == capmodel.ConfigurationFieldSingleSelect {
			legal := len(value.OptionTokens) == 1 && slices.Contains(state.CandidateValues, value.OptionTokens[0])
			if !legal {
				if len(state.CandidateValues) == 0 {
					changed = changed || exists
					continue
				}
				value = configurationcontract.ConfigurationValue{FieldKey: field.Key, OptionTokens: []string{state.CandidateValues[0]}}
				exists, changed = true, true
			}
		} else if !exists && (state.Required || field.Type == capmodel.ConfigurationFieldInteger || field.Type == capmodel.ConfigurationFieldDecimal) {
			value, exists = initialScalarChoice(field, presentations[field.Key])
			changed = changed || exists
		}
		if exists {
			next = append(next, value)
		}
	}
	return next, changed
}

// initialScalarChoice chooses a real first sellable value, including zero.
// It never treats optional quantities as an unpriced package override.
func initialScalarChoice(field *capmodel.ConfigurationFieldDeclaration, presentation configurationcontract.Presentation) (configurationcontract.ConfigurationValue, bool) {
	value := configurationcontract.ConfigurationValue{FieldKey: field.Key}
	switch field.Type {
	case capmodel.ConfigurationFieldInteger:
		first := field.Minimum
		if len(presentation.NumericChoices) > 0 {
			first = presentation.NumericChoices[0]
		}
		if first == "" {
			return value, false
		}
		value.IntegerTextValue = &first
	case capmodel.ConfigurationFieldDecimal:
		if field.Minimum == "" {
			return value, false
		}
		first := field.Minimum
		value.DecimalValue = &first
	case capmodel.ConfigurationFieldBoolean:
		first := false
		value.BooleanValue = &first
	default:
		return value, false
	}
	return value, true
}

// missingPurchaseChoiceIssues prevents a crafted quote/cart from omitting a
// sellable choice and buying only the base price. Interactive evaluation may
// fill the first choice, while strict quote/order validation never repairs it.
func missingPurchaseChoiceIssues(schema *capmodel.ConfigurationSchemaDeclaration, presentations map[string]configurationcontract.Presentation, evaluation *configurationcontract.Evaluation) []configurationcontract.Issue {
	if schema == nil {
		return nil
	}
	present := make(map[string]bool, len(evaluation.Values))
	for _, value := range evaluation.Values {
		present[value.FieldKey] = true
	}
	var issues []configurationcontract.Issue
	for _, field := range schema.Fields {
		state := evaluation.States[field.Key]
		if present[field.Key] || !state.Visible || state.Disabled || state.Required {
			continue
		}
		_, scalar := initialScalarChoice(field, presentations[field.Key])
		if (field.Type == capmodel.ConfigurationFieldSingleSelect && len(state.CandidateValues) > 0) || ((field.Type == capmodel.ConfigurationFieldInteger || field.Type == capmodel.ConfigurationFieldDecimal) && scalar) {
			issues = append(issues, configurationcontract.Issue{FieldKey: field.Key, Code: configurationcontract.IssueRequired})
		}
	}
	return issues
}
