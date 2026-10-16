// This file binds live import candidates to the same revision and opaque keys
// as the static schema. It never derives identities from encrypted token bytes.

package catalog

import (
	"context"
	"encoding/json"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/channelcommercecap"
)

// ReadImportCandidates accepts upstream purchase tokens and returns portable
// schema keys alongside authorized tokens, preserving candidate hierarchy.
func (s *publicCatalogReaderImpl) ReadImportCandidates(ctx context.Context, input channelcommercecap.ImportCandidatesInput) (*channelcommercecap.ImportCandidates, error) {
	if input.ConfigurationRevision == "" || len(input.Values) > maximumPublicStructuredConfigurationValues {
		return nil, bizerr.NewCode(CodePublicConfigurationInvalid)
	}
	snapshot, err := s.loadStructuredConfiguration(ctx, input.PublicSlug, input.ConfigurationRevision)
	if err != nil {
		return nil, err
	}
	publicValues := make([]PublicPurchaseConfigurationValue, len(input.Values))
	for i, value := range input.Values {
		publicValues[i] = PublicPurchaseConfigurationValue{FieldToken: value.FieldToken, Type: value.Type, TextValue: value.TextValue, IntegerValue: value.IntegerValue, DecimalValue: value.DecimalValue,
			BooleanValue: value.BooleanValue, DateValue: value.DateValue, DateTimeValue: value.DateTimeValue, OptionTokens: value.OptionTokens, TreeOptionTokens: value.TreeOptionTokens, AddressOptionTokens: value.AddressOptionTokens, FileTokens: value.FileTokens}
	}
	values, err := s.decodePublicStructuredValues(snapshot, publicValues)
	if err != nil {
		return nil, bizerr.NewCode(CodePublicConfigurationInvalid)
	}
	_, resolved, err := s.evaluateStructuredConfiguration(ctx, snapshot, values)
	if err != nil {
		return nil, err
	}
	projection, err := projectImportSchema(s.revision, snapshot)
	if err != nil {
		return nil, bizerr.WrapCode(err, CodePublicCatalogUnavailable)
	}
	result := &channelcommercecap.ImportCandidates{ConfigurationRevision: snapshot.Revision, Fields: []channelcommercecap.ImportCandidateField{}}
	if snapshot.Schema != nil {
		for _, field := range snapshot.Schema.Fields {
			candidates := field.StaticCandidates
			if field.DynamicCandidates != nil {
				candidates = resolved[field.Key]
			}
			fieldToken, err := s.revision.encodeStructuredField(snapshot.Revision, field.Key)
			if err != nil {
				return nil, bizerr.WrapCode(err, CodePublicCatalogUnavailable)
			}
			item := channelcommercecap.ImportCandidateField{FieldKey: projection.fields[field.Key], FieldToken: fieldToken, Options: []channelcommercecap.ImportCandidate{}}
			for _, candidate := range candidates {
				value, err := projection.optionKey(field.Key, candidate.Value)
				if err != nil {
					return nil, bizerr.WrapCode(err, CodePublicCatalogUnavailable)
				}
				token, err := s.revision.encodeStructuredOption(snapshot.Revision, field.Key, candidate.Value)
				if err != nil {
					return nil, bizerr.WrapCode(err, CodePublicCatalogUnavailable)
				}
				option := channelcommercecap.ImportCandidate{Value: value, OptionToken: token, Label: channelcommercecap.LocalizedText{ZhCN: candidate.Label.ZhCN, EnUS: candidate.Label.EnUS}}
				if candidate.ParentValue != "" {
					option.ParentValue, err = projection.optionKey(field.Key, candidate.ParentValue)
					if err != nil {
						return nil, bizerr.WrapCode(err, CodePublicCatalogUnavailable)
					}
				}
				item.Options = append(item.Options, option)
			}
			result.Fields = append(result.Fields, item)
		}
	}
	raw, err := json.Marshal(result)
	if err != nil || len(raw) > maximumPublicCatalogResponseBytes-publicCatalogEnvelopeReserve {
		return nil, bizerr.NewCode(CodePublicCatalogUnavailable)
	}
	if err = s.recheckStructuredConfiguration(ctx, snapshot); err != nil {
		return nil, err
	}
	return result, nil
}
