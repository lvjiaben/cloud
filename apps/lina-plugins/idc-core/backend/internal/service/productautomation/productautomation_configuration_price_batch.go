// Fixed-price table target resolution is local and bounded. It reuses frozen
// candidates and inserts missing immutable dimensions in a single SQL batch.
package productautomation

import (
	"context"
	"strings"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"
	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/internal/service/catalog"
)

func (s *serviceImpl) ResolvePriceTargets(ctx context.Context, versionNo string, tokens []string) (map[string]string, error) {
	var result map[string]string
	err := s.database.Transaction(ctx, func(txCtx context.Context, _ gdb.TX) error {
		var resolveErr error
		result, resolveErr = s.resolveFixedPriceTargets(txCtx, versionNo, tokens)
		return resolveErr
	})
	return result, err
}
func (s *serviceImpl) resolveFixedPriceTargets(ctx context.Context, versionNo string, tokens []string) (map[string]string, error) {
	if len(tokens) == 0 || len(tokens) > 256 {
		return nil, bizerr.NewCode(catalog.CodeManagementInvalid)
	}
	authorization, err := s.authorizer.AuthorizeVersionConfiguration(ctx, versionNo)
	if err != nil {
		return nil, err
	}
	if err = requireAuthorization(authorization); err != nil {
		return nil, err
	}
	configuration, _, err := s.loadPriceConfiguration(ctx, versionNo)
	if err != nil {
		return nil, err
	}
	if configuration == nil {
		return nil, bizerr.NewCode(catalog.CodeManagementConflict)
	}
	candidates, err := configurationPriceCandidates(versionNo, configuration, s.booleanPriceLabels)
	if err != nil {
		return nil, err
	}
	byToken := map[string]configurationPriceCandidatePrivate{}
	for _, candidate := range candidates {
		byToken[candidate.CandidateToken] = candidate
	}
	selected := make([]configurationPriceCandidatePrivate, 0, len(tokens))
	seen := map[string]bool{}
	for _, token := range tokens {
		candidate, exists := byToken[token]
		if !exists || candidate.Quantity {
			return nil, bizerr.NewCode(catalog.CodeManagementInvalid)
		}
		if !seen[token] {
			selected = append(selected, candidate)
			seen[token] = true
		}
	}
	if err = s.lockDraftVersion(ctx, versionNo); err != nil {
		return nil, err
	}
	current, err := s.database.GetValue(ctx, `SELECT version FROM plugin_idc_core_catalog_product_version_configuration WHERE tenant_id=0 AND version_no=? FOR SHARE`, versionNo)
	if err != nil {
		return nil, err
	}
	if current.IsNil() || current.Int64() != configuration.Version {
		return nil, bizerr.NewCode(catalog.CodeManagementConflict)
	}
	return s.insertFixedPriceTargets(ctx, versionNo, selected, authorization.RequiredPermission)
}

func (s *serviceImpl) insertFixedPriceTargets(ctx context.Context, versionNo string, selected []configurationPriceCandidatePrivate, permission string) (map[string]string, error) {
	return s.insertConfigurationPriceTargets(ctx, versionNo, selected, permission, nil)
}

func (s *serviceImpl) insertConfigurationPriceTargets(ctx context.Context, versionNo string, selected []configurationPriceCandidatePrivate, permission string, modes map[string]catalog.ConfigurationPriceMode) (map[string]string, error) {
	var existing []configurationPriceDimensionRow
	if err := s.database.GetScan(ctx, &existing, `SELECT dimension_no,field_key,candidate_value,pricing_mode_code FROM plugin_idc_core_catalog_configuration_price_dimension WHERE tenant_id=0 AND version_no=? ORDER BY id LIMIT ?`, versionNo, maximumConfigurationPriceCandidates+1); err != nil {
		return nil, err
	}
	if len(existing) > maximumConfigurationPriceCandidates {
		return nil, bizerr.NewCode(catalog.CodeManagementConflict)
	}
	byIdentity := map[string]configurationPriceDimensionRow{}
	for _, row := range existing {
		byIdentity[row.FieldKey+"\x00"+row.CandidateValue] = row
	}
	var (
		result       = make(map[string]string, len(selected))
		placeholders = []string{}
		values       = []any{}
		created      = []string{}
	)
	for _, candidate := range selected {
		mode := modes[candidate.CandidateToken]
		if mode == "" {
			mode = catalog.ConfigurationPriceFixed
		}
		row, exists := byIdentity[candidate.fieldKey+"\x00"+candidate.candidateValue]
		if exists {
			if row.PricingMode != mode {
				return nil, bizerr.NewCode(catalog.CodeManagementConflict)
			}
			result[candidate.CandidateToken] = row.DimensionNo
			continue
		}
		number, err := newConfigurationPriceDimensionNo(s.random)
		if err != nil {
			return nil, err
		}
		result[candidate.CandidateToken] = number
		created = append(created, number)
		placeholders = append(placeholders, "(0,?,?,?,?,?,?,?,?,?,?,'active')")
		values = append(values, number, versionNo, candidate.configurationVersion, candidate.fieldKey, candidate.candidateValue, candidate.FieldLabel.ZhCN, candidate.FieldLabel.EnUS, candidate.CandidateLabel.ZhCN, candidate.CandidateLabel.EnUS, mode)
	}
	if len(created) == 0 {
		return result, nil
	}
	if len(created)+len(existing) > maximumConfigurationPriceCandidates {
		return nil, bizerr.NewCode(catalog.CodeManagementConflict)
	}
	inserted, err := s.database.Exec(ctx, `INSERT INTO plugin_idc_core_catalog_configuration_price_dimension(tenant_id,dimension_no,version_no,configuration_version,field_key,candidate_value,field_label_zh_cn,field_label_en_us,candidate_label_zh_cn,candidate_label_en_us,pricing_mode_code,status_code) VALUES `+strings.Join(placeholders, ","), values...)
	if err != nil {
		return nil, gerror.Wrap(err, "insert fixed price dimensions")
	}
	affected, err := inserted.RowsAffected()
	if err != nil {
		return nil, err
	}
	if affected != int64(len(created)) {
		return nil, bizerr.NewCode(catalog.CodeManagementConflict)
	}
	err = s.audit.AppendBusinessTransition(ctx, idccap.BusinessAuditTransition{IdempotencyScope: "catalog.configuration-price-dimension.batch", IdempotencyKey: created[0], OperationCode: "catalog.configuration-price-dimension.batch", TargetType: "catalog_product_version", TargetNo: versionNo, ResultCode: "succeeded", ReasonCode: "manual_create", RiskCode: "medium", RequiredPermission: permission, BeforeSnapshot: map[string]any{}, AfterSnapshot: map[string]any{"dimension_numbers": created}})
	return result, err
}
