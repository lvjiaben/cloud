// This file restores imported opaque candidate values through frozen token evidence.
package productautomation

import (
	"context"
	"encoding/json"
	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"
	"lina-core/pkg/plugin/capability/capmodel"
	"lina-plugin-idc-core/backend/cap/channelcommercecap"
	"lina-plugin-idc-core/backend/cap/resourceinterfaceprojectioncap"
	"lina-plugin-idc-core/backend/internal/model/do"
	"lina-plugin-idc-core/backend/internal/model/entity"
	"lina-plugin-idc-core/backend/internal/service/configurationcontract"
)

const importedCandidateMethod = "peer.configuration.candidates"
const maximumImportedCandidateEvidence = configurationcontract.MaximumDynamicCandidateFields * configurationcontract.MaximumDynamicCandidatesPerField

func (s *serviceImpl) resolveImportedCandidates(ctx context.Context, exact *capmodel.BusinessExtensionProviderEvidence, schema *capmodel.ConfigurationSchemaDeclaration, values []configurationcontract.ConfigurationValue, scope dynamicCandidateScope) (configurationcontract.ResolvedCandidates, error) {
	batches, err := configurationcontract.BuildDynamicCandidateBatches(schema, values, configurationSurfaceCustomer, exact.AuthorizedMethods)
	if err != nil {
		return nil, err
	}
	expected := map[string]bool{}
	if schema != nil {
		for _, f := range schema.Fields {
			if f.DynamicCandidates != nil {
				if f.DynamicCandidates.Method != importedCandidateMethod {
					return nil, gerror.New("imported candidate method invalid")
				}
				expected[f.Key] = true
			}
		}
	}
	if len(expected) == 0 {
		return configurationcontract.ResolvedCandidates{}, nil
	}
	if s.resourceCandidates == nil {
		return nil, gerror.New("candidate source unavailable")
	}
	var c *entity.CatalogImportContract
	if err := s.database.Model(do.CatalogImportContract{}).Ctx(ctx).Where(do.CatalogImportContract{TenantId: 0, VersionNo: scope.versionNo, InterfaceInstanceNo: scope.interfaceInstanceNo}).Scan(&c); err != nil || c == nil {
		return nil, gerror.New("import contract unavailable")
	}
	var snap channelcommercecap.ImportSnapshot
	if json.Unmarshal([]byte(c.Snapshot), &snap) != nil || snap.Validate() != nil {
		return nil, gerror.New("import snapshot invalid")
	}
	var read resourceinterfaceprojectioncap.CatalogReadEvidence
	if json.Unmarshal([]byte(c.ReadEvidence), &read) != nil || read.Validate() != nil {
		return nil, gerror.New("import evidence invalid")
	}
	var rows []entity.CatalogImportCandidate
	if err := s.database.Model(do.CatalogImportCandidate{}).Ctx(ctx).Where(do.CatalogImportCandidate{TenantId: 0, VersionNo: scope.versionNo}).Limit(maximumImportedCandidateEvidence + 1).Scan(&rows); err != nil {
		return nil, err
	}
	if len(rows) > maximumImportedCandidateEvidence {
		return nil, gerror.New("import candidate evidence bound exceeded")
	}
	tokens := map[string]string{}
	for _, m := range snap.Mappings {
		for _, o := range m.Options {
			tokens[m.FieldKey+"\x00"+o.Value] = o.OptionToken
		}
	}
	for _, r := range rows {
		if old := tokens[r.FieldKey+"\x00"+r.CandidateValue]; old != "" && old != r.OptionToken {
			return nil, gerror.New("candidate token differs from frozen source")
		}
		tokens[r.FieldKey+"\x00"+r.CandidateValue] = r.OptionToken
	}
	purchase := make([]channelcommercecap.PurchaseValue, 0, len(values))
	for _, v := range values {
		// Candidate requests intentionally exclude text, secrets and files,
		// matching the existing governed dynamic candidate protocol.
		if v.OptionTokens == nil && v.TreeOptionTokens == nil && v.AddressOptionTokens == nil {
			continue
		}
		m := mappingByField(snap.Mappings, v.FieldKey)
		if m == nil {
			return nil, gerror.New("import field unavailable")
		}
		f := fieldByKey(snap.Schema, v.FieldKey)
		if f == nil {
			return nil, gerror.New("import field schema unavailable")
		}
		out := channelcommercecap.PurchaseValue{FieldToken: m.FieldToken, Type: f.Type}
		var err error
		if out.OptionTokens, err = mapTokens(v.OptionTokens, tokens, v.FieldKey); err != nil {
			return nil, err
		}
		if out.TreeOptionTokens, err = mapTokens(v.TreeOptionTokens, tokens, v.FieldKey); err != nil {
			return nil, err
		}
		if out.AddressOptionTokens, err = mapTokens(v.AddressOptionTokens, tokens, v.FieldKey); err != nil {
			return nil, err
		}
		purchase = append(purchase, out)
	}
	payload, err := json.Marshal(channelcommercecap.ImportCandidatesInput{PublicSlug: snap.Product.PublicSlug, ConfigurationRevision: snap.ConfigurationRevision, Values: purchase})
	if err != nil {
		return nil, err
	}
	result, err := s.resourceCandidates.ResolveCandidates(ctx, resourceinterfaceprojectioncap.CandidateRequest{InterfaceInstanceNo: scope.interfaceInstanceNo, ExpectedConfigurationRevision: read.InterfaceConfigurationRevision, Method: importedCandidateMethod, Values: payload})
	if err != nil {
		return nil, err
	}
	if err = validateImportedCandidateFields(snap.Schema, result); err != nil {
		return nil, err
	}
	if len(batches) != 1 {
		return nil, gerror.New("import candidate method set invalid")
	}
	return importedCandidateResult(ctx, s.database, scope.versionNo, batches[0].Queries, result)
}

func validateImportedCandidateFields(schema *capmodel.ConfigurationSchemaDeclaration, result *resourceinterfaceprojectioncap.CandidateResult) error {
	if result == nil {
		return gerror.New("import candidate result unavailable")
	}
	for _, field := range result.Fields {
		if fieldByKey(schema, field.FieldKey) == nil {
			return gerror.New("import candidate result contains unknown field")
		}
	}
	return nil
}

func mappingByField(ms []channelcommercecap.ImportFieldMapping, key string) *channelcommercecap.ImportFieldMapping {
	for i := range ms {
		if ms[i].FieldKey == key {
			return &ms[i]
		}
	}
	return nil
}
func fieldByKey(s *capmodel.ConfigurationSchemaDeclaration, key string) *capmodel.ConfigurationFieldDeclaration {
	if s != nil {
		for _, f := range s.Fields {
			if f != nil && f.Key == key {
				return f
			}
		}
	}
	return nil
}
func mapTokens(values []string, tokens map[string]string, field string) ([]string, error) {
	if values == nil {
		return nil, nil
	}
	out := make([]string, len(values))
	for i, v := range values {
		t := tokens[field+"\x00"+v]
		if t == "" {
			return nil, gerror.New("candidate token unavailable")
		}
		out[i] = t
	}
	return out, nil
}
func importedCandidateResult(ctx context.Context, db gdb.DB, version string, queries []configurationcontract.DynamicCandidateQuery, result *resourceinterfaceprojectioncap.CandidateResult) (configurationcontract.ResolvedCandidates, error) {
	if result == nil || len(result.Fields) > 128 {
		return nil, gerror.New("candidate result unavailable")
	}
	resolved := configurationcontract.ResolvedCandidates{}
	expected := make(map[string]bool, len(queries))
	for _, query := range queries {
		expected[query.FieldKey] = true
	}
	rows := []do.CatalogImportCandidate{}
	wanted := map[string]string{}
	for _, field := range result.Fields {
		if !expected[field.FieldKey] {
			continue
		}
		if _, duplicate := resolved[field.FieldKey]; duplicate || len(field.Items) > 512 {
			return nil, gerror.New("candidate fields invalid")
		}
		items := []*capmodel.ConfigurationCandidate{}
		for _, item := range field.Items {
			key := field.FieldKey + "\x00" + item.Value
			if item.Value == "" || item.OptionToken == "" || len(item.OptionToken) > 4096 || wanted[key] != "" {
				return nil, gerror.New("candidate token invalid")
			}
			wanted[key] = item.OptionToken
			items = append(items, &capmodel.ConfigurationCandidate{Value: item.Value, ParentValue: item.ParentValue, Label: &capmodel.LocalizedText{ZhCN: item.LabelZhCN, EnUS: item.LabelEnUS}})
			rows = append(rows, do.CatalogImportCandidate{TenantId: 0, VersionNo: version, FieldKey: field.FieldKey, CandidateValue: item.Value, OptionToken: item.OptionToken})
		}
		resolved[field.FieldKey] = items
	}
	if len(resolved) != len(expected) {
		return nil, gerror.New("candidate fields incomplete")
	}
	response := configurationcontract.DynamicCandidateResponse{SchemaVersion: configurationcontract.DynamicCandidateProtocolV1}
	for _, query := range queries {
		response.Items = append(response.Items, configurationcontract.DynamicCandidateItem{FieldKey: query.FieldKey, Candidates: resolved[query.FieldKey]})
	}
	payload, err := json.Marshal(response)
	if err != nil {
		return nil, err
	}
	resolved, err = configurationcontract.DecodeDynamicCandidateResponse(payload, queries)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return resolved, nil
	}
	err = db.Transaction(ctx, func(txCtx context.Context, _ gdb.TX) error {
		if _, e := db.Model(do.CatalogImportCandidate{}).Ctx(txCtx).Data(rows).InsertIgnore(); e != nil {
			return e
		}
		var stored []entity.CatalogImportCandidate
		if e := db.Model(do.CatalogImportCandidate{}).Ctx(txCtx).Where(do.CatalogImportCandidate{TenantId: 0, VersionNo: version}).Limit(maximumImportedCandidateEvidence + 1).Scan(&stored); e != nil {
			return e
		}
		if len(stored) > maximumImportedCandidateEvidence {
			return gerror.New("import candidate evidence bound exceeded")
		}
		for _, row := range stored {
			key := row.FieldKey + "\x00" + row.CandidateValue
			if t, ok := wanted[key]; ok {
				if t != row.OptionToken {
					return gerror.New("candidate token changed within imported version")
				}
				delete(wanted, key)
			}
		}
		if len(wanted) > 0 {
			return gerror.New("candidate token evidence incomplete")
		}
		return nil
	})
	return resolved, err
}
