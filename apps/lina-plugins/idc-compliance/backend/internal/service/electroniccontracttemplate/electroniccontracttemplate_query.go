// This file implements the bounded template list and exact detail projection.

package electroniccontracttemplate

import (
	"context"
	"lina-plugin-idc-compliance/backend/internal/service/numberedpage"
	"strings"
	"time"

	"lina-core/pkg/bizerr"
	"lina-core/pkg/logger"
	"lina-plugin-idc-core/backend/cap/idccap"
)

type projectionRow struct {
	TemplateNo       string     `orm:"template_no"`
	VersionNo        int64      `orm:"version_no"`
	Name             string     `orm:"name"`
	ScopeCode        *string    `orm:"scope_code"`
	SigningOrderCode *string    `orm:"signing_order_code"`
	LanguageCode     *string    `orm:"language_code"`
	StatusCode       string     `orm:"status_code"`
	OriginCode       string     `orm:"origin_code"`
	ProviderState    *string    `orm:"provider_state_code"`
	ProviderVersion  *string    `orm:"provider_version"`
	UsedCount        int64      `orm:"used_count"`
	LockVersion      int64      `orm:"lock_version"`
	UpdatedAt        *time.Time `orm:"updated_at"`
}

const templateProjectionColumns = `
template_no, version_no, name, scope_code, signing_order_code, language_code,
CASE
    WHEN origin_code = 'tencent_synced' AND provider_state_code = 'disabled'
        THEN 'provider_disabled'
    WHEN origin_code = 'tencent_synced' AND provider_state_code = 'offline'
        THEN 'provider_offline'
    ELSE status_code
END AS status_code,
origin_code, provider_state_code, provider_version, used_count, lock_version, updated_at`

// List returns latest logical versions only and never calls Tencent implicitly.
func (s *serviceImpl) List(ctx context.Context, input ListInput) (*List, error) {
	if _, err := s.authorize(ctx, PermissionQuery, templateDirectoryTarget); err != nil {
		return nil, err
	}
	input, err := normalizeListInput(input)
	if err != nil {
		return nil, err
	}
	clauses := []string{"1 = 1"}
	arguments := []any{}
	if input.Name != "" {
		clauses = append(clauses, `name ILIKE ? ESCAPE '\'`)
		arguments = append(arguments, "%"+escapeLike(input.Name)+"%")
	}
	if input.ScopeCode != "" {
		clauses = append(clauses, "scope_code = ?")
		arguments = append(arguments, input.ScopeCode)
	}
	if input.StatusCode != "" {
		clauses = append(clauses, "status_code = ?")
		arguments = append(arguments, input.StatusCode)
	}
	projection := `
WITH latest AS (
    SELECT DISTINCT ON (version.template_no)
        version.template_no, version.version_no, version.name,
		version.scope_code, version.signing_order_code, version.language_code, version.status_code,
        template.origin_code, template.provider_state_code, version.provider_version,
        version.used_count, version.lock_version, version.updated_at,
        version.tenant_id
    FROM plugin_idc_compliance_contract_template_version AS version
    JOIN plugin_idc_compliance_contract_template AS template
      ON template.tenant_id = version.tenant_id
     AND template.template_no = version.template_no
    WHERE version.tenant_id = ?
    ORDER BY version.template_no, version.version_no DESC
), projected AS (
    SELECT ` + templateProjectionColumns + `
    FROM latest
)
`
	filtered := `FROM projected WHERE ` + strings.Join(clauses, " AND ")
	query := projection + `SELECT template_no, version_no, name, scope_code,
signing_order_code, language_code, status_code, origin_code, provider_state_code,
provider_version, used_count, lock_version, updated_at
` + filtered + `
ORDER BY updated_at DESC, template_no DESC
LIMIT ? OFFSET ?`
	allArguments := []any{idccap.DefaultTenantID}
	allArguments = append(allArguments, arguments...)
	countArgs := append([]any(nil), allArguments...)
	countSQL := projection + `SELECT COUNT(*) ` + filtered
	allArguments = append(allArguments, input.PageSize, (input.Page-1)*input.PageSize)
	var rows []projectionRow
	total, err := numberedpage.Query(ctx, s.database, &rows, countSQL, countArgs, query, allArguments)
	if err != nil {
		return nil, bizerr.WrapCode(err, CodeUnavailable)
	}
	result := &List{Items: make([]Item, 0, len(rows)), Page: input.Page, PageSize: input.PageSize, Total: total}
	for _, row := range rows {
		item, valid := projectRow(row)
		if !valid {
			logger.Warningf(ctx, "reject invalid electronic contract template projection templateNo=%s version=%d status=%s origin=%s scope=%q signingOrder=%q language=%q providerState=%q usedCount=%d lockVersion=%d updatedAtPresent=%t",
				row.TemplateNo, row.VersionNo, row.StatusCode, row.OriginCode, pointerValue(row.ScopeCode), pointerValue(row.SigningOrderCode), pointerValue(row.LanguageCode), pointerValue(row.ProviderState), row.UsedCount, row.LockVersion, row.UpdatedAt != nil)
			return nil, bizerr.NewCode(CodeUnavailable)
		}
		result.Items = append(result.Items, item)
	}
	return result, nil
}

func pointerValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

// Get returns one exact version after target authorization.
func (s *serviceImpl) Get(ctx context.Context, templateNo string, versionNo int64) (*Item, error) {
	templateNo = strings.TrimSpace(templateNo)
	if !templateNoPattern.MatchString(templateNo) || versionNo <= 0 {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	if _, err := s.authorize(ctx, PermissionQuery, templateNo); err != nil {
		return nil, err
	}
	return s.get(ctx, templateNo, versionNo, false)
}

func (s *serviceImpl) get(ctx context.Context, templateNo string, versionNo int64, lock bool) (*Item, error) {
	query := `
SELECT ` + templateProjectionColumns + `
FROM (
    SELECT version.template_no, version.version_no, version.name,
		   version.scope_code, version.signing_order_code, version.language_code, version.status_code,
           template.origin_code, template.provider_state_code, version.provider_version,
           version.used_count, version.lock_version, version.updated_at
    FROM plugin_idc_compliance_contract_template_version AS version
    JOIN plugin_idc_compliance_contract_template AS template
      ON template.tenant_id = version.tenant_id
     AND template.template_no = version.template_no
    WHERE version.tenant_id = ? AND version.template_no = ? AND version.version_no = ?
) AS exact`
	if lock {
		query += " FOR UPDATE"
	}
	var row *projectionRow
	if err := s.database.GetScan(ctx, &row, query, idccap.DefaultTenantID, templateNo, versionNo); err != nil {
		return nil, bizerr.NewCode(CodeUnavailable)
	}
	if row == nil {
		return nil, bizerr.NewCode(CodeNotAccessible)
	}
	item, valid := projectRow(*row)
	if !valid {
		return nil, bizerr.NewCode(CodeUnavailable)
	}
	return &item, nil
}

func projectRow(row projectionRow) (Item, bool) {
	if !templateNoPattern.MatchString(row.TemplateNo) || row.VersionNo <= 0 || !validName(row.Name) ||
		!validStatusFilter(row.StatusCode) ||
		(row.OriginCode != OriginLocal && row.OriginCode != OriginTencentSynced) ||
		row.UsedCount < 0 || row.LockVersion <= 0 || row.UpdatedAt == nil {
		return Item{}, false
	}
	item := Item{
		TemplateNo: row.TemplateNo, VersionNo: row.VersionNo, Name: row.Name,
		StatusCode: row.StatusCode, OriginCode: row.OriginCode, UsedCount: row.UsedCount,
		LockVersion: row.LockVersion, UpdatedAt: row.UpdatedAt.UTC(),
	}
	if row.ScopeCode != nil {
		item.ScopeCode = *row.ScopeCode
	}
	if row.SigningOrderCode != nil {
		item.SigningOrderCode = *row.SigningOrderCode
	}
	if row.LanguageCode != nil {
		item.LanguageCode = *row.LanguageCode
	}
	if !validOptionalSettings(item.ScopeCode, item.SigningOrderCode, item.LanguageCode) {
		return Item{}, false
	}
	if row.ProviderState != nil {
		item.ProviderState = *row.ProviderState
	}
	if row.ProviderVersion != nil {
		item.ProviderVersion = *row.ProviderVersion
	}
	if row.OriginCode == OriginLocal && item.ProviderState != "" ||
		row.OriginCode == OriginTencentSynced &&
			(item.ProviderState != ProviderStateActive && item.ProviderState != ProviderStateDisabled &&
				item.ProviderState != ProviderStateOffline) {
		return Item{}, false
	}
	return item, true
}

func escapeLike(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, `%`, `\%`)
	return strings.ReplaceAll(value, `_`, `\_`)
}
