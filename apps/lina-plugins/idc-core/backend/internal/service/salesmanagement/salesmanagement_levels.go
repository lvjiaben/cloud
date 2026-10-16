// This file persists named agent tiers with optimistic updates. Referenced
// tiers are suspended instead of deleted so historic quotes remain readable.
package salesmanagement

import (
	"context"
	"strings"
	"unicode/utf8"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/util/guid"
	"lina-core/pkg/bizerr"
	"lina-core/pkg/plugin/capability/authcap/authz"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/internal/dao"
	"lina-plugin-idc-core/backend/internal/model/do"
	"lina-plugin-idc-core/backend/internal/model/entity"
)

func (s *serviceImpl) ListLevels(ctx context.Context, input PageInput) (LevelPage, error) {
	return s.listLevels(ctx, input, permissionOverview)
}

// PriceLevelCandidates provides bounded tier choices to price operators.
func (s *serviceImpl) PriceLevelCandidates(ctx context.Context, input PageInput) (LevelPage, error) {
	return s.listLevels(ctx, input, permissionPricing)
}

func (s *serviceImpl) listLevels(ctx context.Context, input PageInput, permission string) (LevelPage, error) {
	result := LevelPage{Items: []Level{}}
	if _, err := s.authorize(ctx, permission, false); err != nil {
		return result, err
	}
	// An overview grant permits tier administration data, never price projection.
	// Read optional pricing grants before touching the price table; authorization
	// backend failures remain errors rather than being mistaken for a denied grant.
	includePrices := permission == permissionPricing
	if !includePrices {
		grants, err := s.authorization.BatchHasPermissions(ctx, []authz.PermissionKey{authz.PermissionKey(permissionPricing), authz.PermissionKey(idccap.HostPermissionWildcard)})
		if err != nil {
			return result, bizerr.WrapCode(err, codeDenied)
		}
		includePrices = grants[authz.PermissionKey(permissionPricing)] || grants[authz.PermissionKey(idccap.HostPermissionWildcard)]
	}
	input, err := normalizePage(input)
	if err != nil {
		return result, err
	}
	query := s.database.Model(dao.SalesAgentLevel.Table()).Ctx(ctx).Where(do.SalesAgentLevel{TenantId: 0})
	if input.Search != "" {
		query = query.Where("name_zh_cn LIKE ? OR name_en_us LIKE ?", "%"+input.Search+"%", "%"+input.Search+"%")
	}
	result.Total, err = query.Clone().Count()
	if err != nil {
		return result, managementError(err)
	}
	var rows []entity.SalesAgentLevel
	err = query.Fields("level_no,name_zh_cn,name_en_us,status_code,version").OrderAsc("sort_order").OrderAsc("id").Page(input.Page, input.PageSize).Scan(&rows)
	if err != nil {
		return result, managementError(err)
	}
	keys := make([]string, 0, len(rows))
	for _, row := range rows {
		keys = append(keys, row.LevelNo)
	}
	prices := map[string]*entity.SalesLevelPrice{}
	if includePrices && len(keys) > 0 {
		var current []entity.SalesLevelPrice
		if err = s.database.Model(dao.SalesLevelPrice.Table()+" p").Ctx(ctx).Where("p.tenant_id", 0).WhereIn("p.level_no", keys).
			Where("NOT EXISTS (SELECT 1 FROM plugin_idc_core_sales_level_price newer WHERE newer.level_no=p.level_no AND newer.version>p.version)").Scan(&current); err != nil {
			return result, managementError(err)
		}
		for i := range current {
			prices[current[i].LevelNo] = &current[i]
		}
	}
	for _, row := range rows {
		item := levelProjection(row)
		price := levelPricingProjection(row.LevelNo, prices[row.LevelNo])
		item.PercentageBasisPoints = price.PercentageBasisPoints
		item.PricingVersion = price.Version
		result.Items = append(result.Items, item)
	}
	return result, managementError(err)
}
func (s *serviceImpl) SaveLevel(ctx context.Context, input LevelInput) (Level, error) {
	if _, err := s.authorize(ctx, permissionQualification, false); err != nil {
		return Level{}, err
	}
	input.Name = strings.TrimSpace(input.Name)
	input.EnglishName = strings.TrimSpace(input.EnglishName)
	if input.EnglishName == "" {
		input.EnglishName = input.Name
	}
	if input.Name == "" || utf8.RuneCountInString(input.Name) > 128 || utf8.RuneCountInString(input.EnglishName) > 128 || (input.Status != statusActive && input.Status != statusInactive) {
		return Level{}, bizerr.NewCode(codeInvalid)
	}
	var result Level
	err := s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		query := s.database.Model(dao.SalesAgentLevel.Table()).Ctx(ctx)
		if input.LevelNo == "" {
			if input.ExpectedVersion != 0 {
				return bizerr.NewCode(codeConflict)
			}
			input.LevelNo = "ALV_" + guid.S()
			_, err := query.Data(do.SalesAgentLevel{TenantId: 0, LevelNo: input.LevelNo, LevelCode: input.LevelNo, NameZhCn: input.Name, NameEnUs: input.EnglishName, StatusCode: input.Status, Version: 1}).Insert()
			result = Level{LevelNo: input.LevelNo, Name: input.Name, EnglishName: input.EnglishName, Status: input.Status, Version: 1}
			if err != nil {
				return err
			}
			return s.auditChange(ctx, permissionQualification, "level", input.LevelNo, "", 1, nil, result)
		}
		var row *entity.SalesAgentLevel
		if err := query.Where(do.SalesAgentLevel{TenantId: 0, LevelNo: input.LevelNo}).LockUpdate().Scan(&row); err != nil {
			return err
		}
		if row == nil || row.Version != input.ExpectedVersion {
			return bizerr.NewCode(codeConflict)
		}
		_, err := s.database.Model(dao.SalesAgentLevel.Table()).Ctx(ctx).Where(do.SalesAgentLevel{TenantId: 0, LevelNo: input.LevelNo, Version: input.ExpectedVersion}).Data(do.SalesAgentLevel{NameZhCn: input.Name, NameEnUs: input.EnglishName, StatusCode: input.Status, Version: input.ExpectedVersion + 1}).Update()
		result = Level{LevelNo: input.LevelNo, Name: input.Name, EnglishName: input.EnglishName, Status: input.Status, Version: input.ExpectedVersion + 1}
		if err != nil {
			return err
		}
		return s.auditChange(ctx, permissionQualification, "level", input.LevelNo, "", result.Version, row, result)
	})
	return result, managementError(err)
}
func levelProjection(row entity.SalesAgentLevel) Level {
	return Level{LevelNo: row.LevelNo, Name: row.NameZhCn, EnglishName: row.NameEnUs, Status: row.StatusCode, Version: row.Version}
}
