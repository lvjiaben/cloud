// This file reads safe upstream dynamic candidate mappings through the same
// authenticated protocol as imports, without guessing opaque provider values.

package peer

import (
	"encoding/json"
	"lina-core/pkg/plugin/capability/confidentialoutboundcap"
	"lina-plugin-idc-core/backend/cap/channelcommercecap"
	"lina-plugin-idc-core/backend/cap/resourceinterfaceprojectioncap"
	"net/http"
	"net/url"
)

func ReadImportCandidates(executor confidentialoutboundcap.Service, contextID string, payload json.RawMessage) (json.RawMessage, error) {
	var input channelcommercecap.ImportCandidatesInput
	if json.Unmarshal(payload, &input) != nil || (channelcommercecap.ConfigurationInput{PublicSlug: input.PublicSlug, ConfigurationRevision: input.ConfigurationRevision, Values: input.Values}).Validate() != nil {
		return nil, errInvalid
	}
	query, err := configurationQuery(channelcommercecap.ConfigurationInput{PublicSlug: input.PublicSlug, ConfigurationRevision: input.ConfigurationRevision, Values: input.Values})
	if err != nil {
		return nil, errInvalid
	}
	c := &client{executor: executor, contextID: contextID}
	var result channelcommercecap.ImportCandidates
	if err = c.call(http.MethodGet, "/catalog/products/"+url.PathEscape(input.PublicSlug)+"/import-candidates", query, nil, []string{"configurationRevision", "fields", "fieldKey", "fieldToken", "options", "value", "optionToken", "parentValue", "label", "zhCn", "enUs"}, &result); err != nil {
		return nil, err
	}
	if result.ConfigurationRevision != input.ConfigurationRevision || len(result.Fields) > 128 {
		return nil, errUnavailable
	}
	projection := resourceinterfaceprojectioncap.CandidateResult{Fields: make([]resourceinterfaceprojectioncap.CandidateField, len(result.Fields))}
	for index, field := range result.Fields {
		projection.Fields[index] = resourceinterfaceprojectioncap.CandidateField{FieldKey: field.FieldKey, Items: make([]resourceinterfaceprojectioncap.Candidate, len(field.Options))}
		for optionIndex, option := range field.Options {
			projection.Fields[index].Items[optionIndex] = resourceinterfaceprojectioncap.Candidate{Value: option.Value, ParentValue: option.ParentValue, OptionToken: option.OptionToken, LabelZhCN: option.Label.ZhCN, LabelEnUS: option.Label.EnUS}
		}
	}
	return json.Marshal(projection)
}
