// This file loads one current published configuration graph with fixed query counts.

package catalog

import (
	"context"
	"database/sql"
	"errors"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"
)

type publicConfigurationSnapshot struct {
	VersionNo string
	Graph     PublicConfigurationGraph
}

type publicConfigurationTargetRow struct {
	ProductNo         string            `orm:"product_no"`
	VersionNo         string            `orm:"version_no"`
	ConfigurationMode ConfigurationMode `orm:"configuration_mode"`
}

type publicConfigurationGroupRow struct {
	GroupNo         string        `orm:"group_no"`
	SelectionMode   SelectionMode `orm:"selection_mode"`
	NameZhCN        string        `orm:"name_zh_cn"`
	NameEnUS        string        `orm:"name_en_us"`
	MinimumSelected int           `orm:"minimum_selected"`
	MaximumSelected int           `orm:"maximum_selected"`
}

type publicConfigurationOptionRow struct {
	PublicConfigurationOption
	GroupNo string `orm:"group_no"`
}

func loadPublicConfigurationSnapshot(
	ctx context.Context,
	database gdb.DB,
	publicSlug string,
) (*publicConfigurationSnapshot, error) {
	target, err := loadPublicConfigurationTarget(ctx, database, publicSlug)
	if err != nil || target == nil {
		return nil, err
	}
	return loadPublicConfigurationSnapshotForTarget(ctx, database, publicSlug, *target)
}

func loadPublicConfigurationTarget(
	ctx context.Context,
	database gdb.DB,
	publicSlug string,
) (*publicConfigurationTargetRow, error) {
	target := new(publicConfigurationTargetRow)
	if err := database.GetScan(ctx, target, publicConfigurationTargetSQL, publicSlug); errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	} else if err != nil {
		return nil, gerror.Wrap(err, "load public configuration target")
	}
	if target.VersionNo == "" {
		return nil, nil
	}
	return target, nil
}

func loadPublicConfigurationSnapshotForTarget(
	ctx context.Context,
	database gdb.DB,
	publicSlug string,
	target publicConfigurationTargetRow,
) (*publicConfigurationSnapshot, error) {
	groupRows := make([]publicConfigurationGroupRow, 0)
	if err := database.GetScan(ctx, &groupRows, publicConfigurationGroupsSQL,
		target.VersionNo, maximumPublicConfigurationGroups+1,
	); err != nil {
		return nil, gerror.Wrap(err, "load public configuration groups")
	}
	if len(groupRows) > maximumPublicConfigurationGroups {
		return nil, gerror.New("public configuration group bound exceeded")
	}
	optionRows := make([]publicConfigurationOptionRow, 0)
	if err := database.GetScan(ctx, &optionRows, publicConfigurationOptionsSQL,
		target.VersionNo, maximumPublicConfigurationOptions+1,
	); err != nil {
		return nil, gerror.Wrap(err, "load public configuration options")
	}
	if len(optionRows) > maximumPublicConfigurationOptions {
		return nil, gerror.New("public configuration option bound exceeded")
	}
	rules := make([]PublicCompatibilityRule, 0)
	if err := database.GetScan(ctx, &rules, publicCompatibilityRulesSQL,
		target.VersionNo, maximumPublicCompatibilityRules+1,
	); err != nil {
		return nil, gerror.Wrap(err, "load public compatibility rules")
	}
	if len(rules) > maximumPublicCompatibilityRules {
		return nil, gerror.New("public compatibility rule bound exceeded")
	}
	current, err := database.GetValue(ctx, publicConfigurationStillCurrentSQL,
		publicSlug, target.VersionNo,
	)
	if err != nil {
		return nil, gerror.Wrap(err, "recheck public configuration target")
	}
	if !current.Bool() {
		return nil, gerror.New("public configuration changed while loading")
	}
	groups := make([]PublicConfigurationGroup, 0, len(groupRows))
	groupIndexes := make(map[string]int, len(groupRows))
	for _, row := range groupRows {
		groupIndexes[row.GroupNo] = len(groups)
		groups = append(groups, PublicConfigurationGroup{
			GroupNo: row.GroupNo, SelectionMode: row.SelectionMode,
			NameZhCN: row.NameZhCN, NameEnUS: row.NameEnUS,
			MinimumSelected: row.MinimumSelected, MaximumSelected: row.MaximumSelected,
			Options: make([]PublicConfigurationOption, 0),
		})
	}
	for _, row := range optionRows {
		index, exists := groupIndexes[row.GroupNo]
		if !exists {
			return nil, gerror.New("public configuration option group is unavailable")
		}
		groups[index].Options = append(groups[index].Options, row.PublicConfigurationOption)
	}
	return &publicConfigurationSnapshot{
		VersionNo: target.VersionNo,
		Graph: PublicConfigurationGraph{
			ConfigurationMode: target.ConfigurationMode,
			Groups:            groups,
			Rules:             rules,
		},
	}, nil
}

const publicConfigurationTargetSQL = `
SELECT product.product_no, product.current_version_no AS version_no,
       CASE WHEN EXISTS(
           SELECT 1 FROM plugin_idc_core_catalog_configuration_group config_group
           WHERE config_group.tenant_id=product.tenant_id
             AND config_group.version_no=product.current_version_no
       ) THEN 'configurable' ELSE 'fixed' END AS configuration_mode
FROM plugin_idc_core_catalog_product product
INNER JOIN plugin_idc_core_catalog_category category
    ON category.tenant_id = product.tenant_id
   AND category.category_no = product.category_no
   AND category.deleted_at IS NULL AND category.status_code = 'active'
INNER JOIN plugin_idc_core_catalog_product_version version
    ON version.tenant_id = product.tenant_id
   AND version.product_no = product.product_no
   AND version.version_no = product.current_version_no
   AND version.status_code = 'published'
WHERE product.tenant_id = 0 AND product.deleted_at IS NULL
  AND product.status_code = 'active' AND product.public_slug = ?
LIMIT 1`

const publicConfigurationGroupsSQL = `
SELECT group_no, selection_mode_code AS selection_mode,
       name_zh_cn, name_en_us, minimum_selected, maximum_selected
FROM plugin_idc_core_catalog_configuration_group
WHERE tenant_id = 0 AND version_no = ?
ORDER BY sort_order ASC, id ASC
LIMIT ?`

const publicConfigurationOptionsSQL = `
SELECT option.group_no, option.option_no,
       option.component_type_code AS component_type,
       option.name_zh_cn, option.name_en_us,
       option.minimum_quantity, option.maximum_quantity, option.default_quantity
FROM plugin_idc_core_catalog_configuration_option option
INNER JOIN plugin_idc_core_catalog_configuration_group config_group
    ON config_group.tenant_id = option.tenant_id
   AND config_group.version_no = option.version_no
   AND config_group.group_no = option.group_no
WHERE option.tenant_id = 0 AND option.version_no = ? AND option.status_code = 'active'
ORDER BY config_group.sort_order ASC, config_group.id ASC, option.sort_order ASC, option.id ASC
LIMIT ?`

const publicCompatibilityRulesSQL = `
SELECT rule_type_code AS rule_type, source_option_no, target_option_no,
       minimum_target_quantity, maximum_target_quantity
FROM plugin_idc_core_catalog_compatibility_rule
WHERE tenant_id = 0 AND version_no = ? AND status_code = 'active'
ORDER BY id ASC
LIMIT ?`

const publicConfigurationStillCurrentSQL = `
SELECT EXISTS (
    SELECT 1
    FROM plugin_idc_core_catalog_product product
    INNER JOIN plugin_idc_core_catalog_category category
        ON category.tenant_id = product.tenant_id
       AND category.category_no = product.category_no
       AND category.deleted_at IS NULL AND category.status_code = 'active'
    INNER JOIN plugin_idc_core_catalog_product_version version
        ON version.tenant_id = product.tenant_id
       AND version.product_no = product.product_no
       AND version.version_no = product.current_version_no
       AND version.status_code = 'published'
    WHERE product.tenant_id = 0 AND product.deleted_at IS NULL
      AND product.status_code = 'active' AND product.public_slug = ?
      AND product.current_version_no = ?
)`
