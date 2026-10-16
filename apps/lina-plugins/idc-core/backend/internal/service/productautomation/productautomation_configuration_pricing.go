// This file lets merchants explicitly turn frozen customer candidates into
// core-owned price dimensions without exposing plugin keys or raw values.

package productautomation

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"sort"
	"strconv"
	"strings"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
	"lina-core/pkg/plugin/capability/capmodel"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/internal/service/catalog"
)

const (
	maximumConfigurationPriceCandidates = 4096
	configurationQuantityPriceIdentity  = "quantity"
)

type configurationPriceCandidatePrivate struct {
	ConfigurationPriceCandidate
	fieldKey, candidateValue string
	configurationVersion     int64
}

type configurationPriceDimensionRow struct {
	DimensionNo        string                         `orm:"dimension_no"`
	FieldKey           string                         `orm:"field_key"`
	CandidateValue     string                         `orm:"candidate_value"`
	FieldLabelZhCN     string                         `orm:"field_label_zh_cn"`
	FieldLabelEnUS     string                         `orm:"field_label_en_us"`
	CandidateLabelZhCN string                         `orm:"candidate_label_zh_cn"`
	CandidateLabelEnUS string                         `orm:"candidate_label_en_us"`
	PricingMode        catalog.ConfigurationPriceMode `orm:"pricing_mode_code"`
}

func (s *serviceImpl) ListConfigurationPriceCandidates(ctx context.Context, versionNo string) ([]ConfigurationPriceCandidate, error) {
	configuration, err := s.GetVersionConfiguration(ctx, versionNo)
	if err != nil {
		return nil, err
	}
	if configuration == nil || configuration.Version < 1 {
		return []ConfigurationPriceCandidate{}, nil
	}
	private, err := configurationPriceCandidates(versionNo, configuration)
	if err != nil {
		return nil, bizerr.WrapCode(err, catalog.CodeManagementUnavailable)
	}
	var rows []configurationPriceDimensionRow
	if err = s.database.GetScan(ctx, &rows, `SELECT dimension_no,field_key,candidate_value,
field_label_zh_cn,field_label_en_us,candidate_label_zh_cn,candidate_label_en_us,pricing_mode_code
FROM plugin_idc_core_catalog_configuration_price_dimension
WHERE tenant_id=0 AND version_no=? AND status_code='active'
ORDER BY field_key,candidate_value,id LIMIT ?`, versionNo, maximumConfigurationPriceCandidates+1); err != nil {
		return nil, bizerr.WrapCode(err, catalog.CodeManagementUnavailable)
	}
	if len(rows) > maximumConfigurationPriceCandidates {
		return nil, bizerr.NewCode(catalog.CodeManagementUnavailable)
	}
	dimensionByIdentity := make(map[string]configurationPriceDimensionRow, len(rows))
	for _, row := range rows {
		dimensionByIdentity[row.FieldKey+"\x00"+row.CandidateValue] = row
	}
	result := make([]ConfigurationPriceCandidate, 0, len(private))
	for _, candidate := range private {
		dimension := dimensionByIdentity[candidate.fieldKey+"\x00"+candidate.candidateValue]
		candidate.DimensionNo, candidate.PricingMode = dimension.DimensionNo, dimension.PricingMode
		result = append(result, candidate.ConfigurationPriceCandidate)
	}
	return result, nil
}

func (s *serviceImpl) CreateConfigurationPriceDimension(ctx context.Context, input CreateConfigurationPriceDimensionInput) (*ConfigurationPriceDimension, error) {
	authorization, err := s.authorizer.AuthorizeVersionConfiguration(ctx, input.VersionNo)
	if err != nil {
		return nil, err
	}
	if err = requireAuthorization(authorization); err != nil {
		return nil, err
	}
	configuration, err := s.GetVersionConfiguration(ctx, input.VersionNo)
	if err != nil || configuration == nil || configuration.Version < 1 {
		return nil, bizerr.NewCode(catalog.CodeManagementNotAccessible)
	}
	candidates, err := configurationPriceCandidates(input.VersionNo, configuration)
	if err != nil {
		return nil, bizerr.NewCode(catalog.CodeManagementInvalid)
	}
	var selected *configurationPriceCandidatePrivate
	for index := range candidates {
		if candidates[index].CandidateToken == input.CandidateToken {
			selected = &candidates[index]
			break
		}
	}
	if selected == nil {
		return nil, bizerr.NewCode(catalog.CodeManagementInvalid)
	}
	mode := input.PricingMode
	if mode == "" {
		mode = catalog.ConfigurationPriceFixed
	}
	if selected.Quantity {
		if mode != catalog.ConfigurationPricePerUnit && mode != catalog.ConfigurationPriceBandTotal {
			return nil, bizerr.NewCode(catalog.CodeManagementInvalid)
		}
	} else if mode != catalog.ConfigurationPriceFixed {
		return nil, bizerr.NewCode(catalog.CodeManagementInvalid)
	}
	dimensionNo, err := newConfigurationPriceDimensionNo(s.random)
	if err != nil {
		return nil, bizerr.WrapCode(err, catalog.CodeManagementUnavailable)
	}
	result := &ConfigurationPriceDimension{
		DimensionNo: dimensionNo, PricingMode: mode, FieldLabel: selected.FieldLabel, CandidateLabel: selected.CandidateLabel,
	}
	err = s.database.Transaction(ctx, func(txCtx context.Context, _ gdb.TX) error {
		var locked struct {
			StatusCode string `orm:"status_code"`
			SourceCode string `orm:"source_code"`
		}
		if lockErr := s.database.GetScan(txCtx, &locked, `SELECT status_code,source_code
FROM plugin_idc_core_catalog_product_version WHERE tenant_id=0 AND version_no=? FOR UPDATE`, input.VersionNo); lockErr != nil {
			return gerror.Wrap(lockErr, "lock configuration price version")
		}
		if locked.StatusCode != "draft" || (locked.SourceCode != "manual" && locked.SourceCode != "copy" && locked.SourceCode != "migration") {
			return bizerr.NewCode(catalog.CodeManagementNotAccessible)
		}
		var currentVersion int64
		if lockErr := s.database.GetScan(txCtx, &currentVersion, `SELECT version FROM plugin_idc_core_catalog_product_version_configuration
WHERE tenant_id=0 AND version_no=? FOR SHARE`, input.VersionNo); lockErr != nil || currentVersion != selected.configurationVersion {
			return bizerr.NewCode(catalog.CodeManagementConflict)
		}
		insert, insertErr := s.database.Exec(txCtx, `INSERT INTO plugin_idc_core_catalog_configuration_price_dimension
(tenant_id,dimension_no,version_no,configuration_version,field_key,candidate_value,field_label_zh_cn,field_label_en_us,candidate_label_zh_cn,candidate_label_en_us,pricing_mode_code,status_code)
VALUES(0,?,?,?,?,?,?,?,?,?,?,'active') ON CONFLICT(version_no,field_key,candidate_value) DO NOTHING`,
			dimensionNo, input.VersionNo, selected.configurationVersion, selected.fieldKey, selected.candidateValue,
			selected.FieldLabel.ZhCN, selected.FieldLabel.EnUS, selected.CandidateLabel.ZhCN, selected.CandidateLabel.EnUS, mode)
		if insertErr != nil {
			return gerror.Wrap(insertErr, "insert configuration price dimension")
		}
		affected, insertErr := insert.RowsAffected()
		if insertErr != nil || affected != 1 {
			return bizerr.NewCode(catalog.CodeManagementConflict)
		}
		return s.audit.AppendBusinessTransition(txCtx, idccap.BusinessAuditTransition{
			IdempotencyScope: "catalog.configuration-price-dimension.create", IdempotencyKey: dimensionNo,
			OperationCode: "catalog.configuration-price-dimension.create", TargetType: "catalog_product_version",
			TargetNo: input.VersionNo, ResultCode: "succeeded", ReasonCode: "manual_create", RiskCode: "medium",
			RequiredPermission: authorization.RequiredPermission,
			BeforeSnapshot:     map[string]any{"dimension_no": ""},
			AfterSnapshot:      map[string]any{"dimension_no": dimensionNo},
		})
	})
	if err != nil {
		return nil, normalizeManagementWriteError(err)
	}
	return result, nil
}

func configurationPriceCandidates(versionNo string, configuration *VersionConfiguration) ([]configurationPriceCandidatePrivate, error) {
	if configuration == nil || configuration.CustomerDeclaration == nil || configuration.Version < 1 {
		return []configurationPriceCandidatePrivate{}, nil
	}
	result := make([]configurationPriceCandidatePrivate, 0)
	for _, field := range configuration.CustomerDeclaration.Fields {
		if field == nil || (!configurationPriceCandidateField(field.Type) && field.Type != capmodel.ConfigurationFieldInteger) || field.Label == nil {
			continue
		}
		candidates := field.StaticCandidates
		if field.Type == capmodel.ConfigurationFieldInteger {
			token := configurationPriceCandidateToken(versionNo, configuration.Version, field.Key, configurationQuantityPriceIdentity)
			result = append(result, configurationPriceCandidatePrivate{
				ConfigurationPriceCandidate: ConfigurationPriceCandidate{CandidateToken: token, FieldLabel: *field.Label, CandidateLabel: *field.Label, Quantity: true},
				fieldKey:                    field.Key, candidateValue: configurationQuantityPriceIdentity, configurationVersion: configuration.Version,
			})
			if len(result) > maximumConfigurationPriceCandidates {
				return nil, gerror.New("configuration price candidate bound exceeded")
			}
			continue
		}
		if len(candidates) == 0 {
			candidates = configuration.CustomerResolvedCandidates[field.Key]
		}
		for _, candidate := range candidates {
			if candidate == nil || candidate.Label == nil {
				return nil, gerror.New("configuration price candidate is incomplete")
			}
			token := configurationPriceCandidateToken(versionNo, configuration.Version, field.Key, candidate.Value)
			result = append(result, configurationPriceCandidatePrivate{
				ConfigurationPriceCandidate: ConfigurationPriceCandidate{CandidateToken: token,
					FieldLabel: *field.Label, CandidateLabel: *candidate.Label},
				fieldKey: field.Key, candidateValue: candidate.Value, configurationVersion: configuration.Version,
			})
			if len(result) > maximumConfigurationPriceCandidates {
				return nil, gerror.New("configuration price candidate bound exceeded")
			}
		}
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].FieldLabel.ZhCN != result[j].FieldLabel.ZhCN {
			return result[i].FieldLabel.ZhCN < result[j].FieldLabel.ZhCN
		}
		return result[i].CandidateLabel.ZhCN < result[j].CandidateLabel.ZhCN
	})
	return result, nil
}

func configurationPriceCandidateField(fieldType string) bool {
	switch fieldType {
	case capmodel.ConfigurationFieldSingleSelect, capmodel.ConfigurationFieldMultiSelect,
		capmodel.ConfigurationFieldTreeSelect, capmodel.ConfigurationFieldAddress:
		return true
	default:
		return false
	}
}

func configurationPriceCandidateToken(versionNo string, configurationVersion int64, fieldKey, value string) string {
	digest := sha256.Sum256([]byte(strings.Join([]string{versionNo, fieldKey, value, strconv.FormatInt(configurationVersion, 10)}, "\x00")))
	return "candidate_" + hex.EncodeToString(digest[:])
}

func newConfigurationPriceDimensionNo(randomReader io.Reader) (string, error) {
	bytes := make([]byte, 16)
	if _, err := io.ReadFull(randomReader, bytes); err != nil {
		return "", gerror.Wrap(err, "read configuration price dimension entropy")
	}
	return "cpd_" + hex.EncodeToString(bytes), nil
}
