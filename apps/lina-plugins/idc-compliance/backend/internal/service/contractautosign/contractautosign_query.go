package contractautosign

import (
	"context"
	"lina-plugin-idc-compliance/backend/internal/service/numberedpage"
	"strings"
	"time"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/idccap"
)

type authorizationRow struct {
	AuthorizationNo     string     `json:"authorization_no"`
	CurrencyCode        string     `json:"currency_code"`
	AllowRequiredOrder  bool       `json:"allow_required_order"`
	AllowManualOptional bool       `json:"allow_manual_optional"`
	MaxAmountMinor      int64      `json:"max_amount_minor"`
	SuccessLimit        int64      `json:"success_limit"`
	SuccessCount        int64      `json:"success_count"`
	StatusCode          string     `json:"status_code"`
	ExpiresAt           *time.Time `json:"expires_at"`
	CreatedByUserID     int        `json:"created_by_user_id"`
	RevokedByUserID     *int       `json:"revoked_by_user_id"`
	RevokedAt           *time.Time `json:"revoked_at"`
	Version             int64      `json:"version"`
	CreatedAt           *time.Time `json:"created_at"`
	CreationFingerprint string     `json:"creation_fingerprint"`
}

type templateRow struct {
	AuthorizationNo string `json:"authorization_no"`
	TemplateNo      string `json:"template_no"`
	VersionNo       int64  `json:"version_no"`
	Name            string `json:"name"`
}

func (s *serviceImpl) List(ctx context.Context, page int, requestedSize ...int) (*List, error) {
	if _, err := s.authorize(ctx, "contract-auto-sign"); err != nil {
		return nil, err
	}
	if page <= 0 {
		page = 1
	}
	pageSize := PageSize
	if len(requestedSize) > 0 {
		pageSize = requestedSize[0]
	}
	if page > 1000000 || pageSize < 1 || pageSize > 100 {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	var (
		rows       []authorizationRow
		total      int64
		candidates []TemplateRef
	)
	var err error
	total, err = numberedpage.Query(ctx, s.database, &rows,
		`SELECT COUNT(*) FROM plugin_idc_compliance_auto_sign_auth WHERE tenant_id = ?`, []any{idccap.DefaultTenantID},
		`
SELECT authorization_no, currency_code, allow_required_order, allow_manual_optional,
       max_amount_minor, success_limit, success_count,
       CASE WHEN status_code = 'active' AND expires_at <= CURRENT_TIMESTAMP THEN 'expired'
            ELSE status_code END AS status_code,
       expires_at, created_by_user_id, revoked_by_user_id, revoked_at, version, created_at
FROM plugin_idc_compliance_auto_sign_auth
WHERE tenant_id = ?
ORDER BY created_at DESC, id DESC
LIMIT ? OFFSET ?`, []any{idccap.DefaultTenantID, pageSize, (page - 1) * pageSize})
	if err != nil {
		return nil, bizerr.NewCode(CodeUnavailable)
	}
	items := make([]Item, 0, len(rows))
	index := make(map[string]int, len(rows))
	for _, row := range rows {
		item, valid := projectAuthorization(row)
		if !valid {
			return nil, bizerr.NewCode(CodeUnavailable)
		}
		index[item.AuthorizationNo] = len(items)
		items = append(items, item)
	}
	if len(rows) > 0 {
		placeholders := make([]string, 0, len(rows))
		args := make([]any, 0, len(rows)+1)
		args = append(args, idccap.DefaultTenantID)
		for _, row := range rows {
			placeholders = append(placeholders, "?")
			args = append(args, row.AuthorizationNo)
		}
		var refs []templateRow
		query := `
SELECT relation.authorization_no, relation.template_no, relation.version_no, version.name
FROM plugin_idc_compliance_auto_sign_auth_template AS relation
JOIN plugin_idc_compliance_contract_template_version AS version
  ON version.tenant_id = relation.tenant_id AND version.template_no = relation.template_no
 AND version.version_no = relation.version_no
WHERE relation.tenant_id = ? AND relation.authorization_no IN (` + strings.Join(placeholders, ",") + `)
ORDER BY relation.authorization_no, relation.id`
		if err = s.database.GetScan(ctx, &refs, query, args...); err != nil {
			return nil, bizerr.NewCode(CodeUnavailable)
		}
		for _, ref := range refs {
			position, ok := index[ref.AuthorizationNo]
			if !ok || !templateNoPattern.MatchString(ref.TemplateNo) || ref.VersionNo <= 0 || strings.TrimSpace(ref.Name) == "" {
				return nil, bizerr.NewCode(CodeUnavailable)
			}
			items[position].Templates = append(items[position].Templates, TemplateRef{TemplateNo: ref.TemplateNo, VersionNo: ref.VersionNo, Name: ref.Name})
		}
	}
	if err = s.database.GetScan(ctx, &candidates, `
SELECT version.template_no, version.version_no, version.name
FROM plugin_idc_compliance_contract_template_version AS version
WHERE version.tenant_id = ? AND version.status_code = 'enabled'
  AND version.scope_code IN ('enterprise', 'all_customers')
  AND (
    version.provider_merchant_seal_count > 0 OR EXISTS (
      SELECT 1
      FROM plugin_idc_compliance_contract_template_field_set AS field_set
      JOIN plugin_idc_compliance_contract_template_field AS field
        ON field.tenant_id = field_set.tenant_id AND field.field_set_no = field_set.field_set_no
      WHERE field_set.tenant_id = version.tenant_id
        AND field_set.field_set_no = version.current_field_set_no
        AND field.party_code = 'merchant' AND field.field_type_code = 'enterprise_seal'
    )
  )
ORDER BY version.updated_at DESC, version.id DESC
LIMIT ?`, idccap.DefaultTenantID, MaxTemplates); err != nil {
		return nil, bizerr.NewCode(CodeUnavailable)
	}
	return &List{Items: items, Candidates: candidates, Page: page, PageSize: pageSize, Total: total}, nil
}

func projectAuthorization(row authorizationRow) (Item, bool) {
	if !templateNoPattern.MatchString(row.AuthorizationNo) || !currencyPattern.MatchString(row.CurrencyCode) ||
		row.MaxAmountMinor <= 0 || row.SuccessLimit <= 0 || row.SuccessCount < 0 || row.SuccessCount > row.SuccessLimit ||
		row.ExpiresAt == nil || row.CreatedAt == nil || row.CreatedByUserID <= 0 || row.Version <= 0 {
		return Item{}, false
	}
	if row.StatusCode != StatusActive && row.StatusCode != StatusExpired && row.StatusCode != StatusRevoked {
		return Item{}, false
	}
	return Item{
		AuthorizationNo: row.AuthorizationNo, CurrencyCode: row.CurrencyCode,
		AllowRequiredOrder: row.AllowRequiredOrder, AllowManualOptional: row.AllowManualOptional,
		MaxAmountMinor: row.MaxAmountMinor, SuccessLimit: row.SuccessLimit, SuccessCount: row.SuccessCount,
		StatusCode: row.StatusCode, ExpiresAt: row.ExpiresAt.UTC(), CreatedByUserID: row.CreatedByUserID,
		RevokedByUserID: row.RevokedByUserID, RevokedAt: row.RevokedAt, Version: row.Version,
		CreatedAt: row.CreatedAt.UTC(), Templates: []TemplateRef{},
	}, true
}
