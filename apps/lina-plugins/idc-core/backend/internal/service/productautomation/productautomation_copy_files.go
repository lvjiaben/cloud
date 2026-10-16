// This file clones immutable product file references into a new draft scope.
// Bound objects cannot be revoked or removed; bytes stay shared while tokens,
// targets and staff ownership are independent. All writes use the owner TX.

package productautomation

import (
	"context"
	"encoding/json"
	"github.com/gogf/gf/v2/database/gdb"
	"strings"

	"lina-core/pkg/bizerr"
	"lina-core/pkg/plugin/capability/capmodel"
	"lina-plugin-idc-core/backend/internal/model/entity"
	"lina-plugin-idc-core/backend/internal/service/catalog"
)

func (s *serviceImpl) copyProductConfigurationFiles(ctx context.Context, source *entity.CatalogProductVersionConfiguration, target string, actor int) (string, string, error) {
	if source == nil || actor <= 0 || target == source.VersionNo || validateBusinessNo(target, "ver") != nil || gdb.TXFromCtx(ctx, s.database.GetGroup()) == nil {
		return "", "", bizerr.NewCode(catalog.CodeManagementInvalid)
	}
	schema, err := decodeConfigurationDeclaration(source.ProductDeclarationSnapshot)
	if err != nil {
		return "", "", err
	}
	values, err := configurationValuesFromJSON(schema, []byte(source.MerchantValueSnapshot))
	if err != nil {
		return "", "", err
	}
	var document map[string]json.RawMessage
	if err = json.Unmarshal([]byte(source.MerchantValueSnapshot), &document); err != nil {
		return "", "", err
	}
	var (
		tokens []string
		fields = make(map[string]string)
	)
	for _, value := range values {
		for _, token := range value.FileTokens {
			if !validManagementFileToken(token) || fields[token] != "" || len(tokens) >= 256 {
				return "", "", bizerr.NewCode(catalog.CodeManagementConflict)
			}
			tokens = append(tokens, token)
			fields[token] = value.FieldKey
		}
	}
	if len(tokens) == 0 {
		return source.MerchantValueSnapshot, source.MerchantValueDigest, nil
	}
	var rows []*managementFileRow
	const query = `SELECT u.upload_no,u.field_key,u.policy_id,u.policy_snapshot::text,u.actual_size_bytes AS actual_size,u.media_type,u.scan_status
 FROM plugin_idc_core_management_configuration_upload u
 JOIN plugin_idc_core_management_configuration_file_reference r ON r.tenant_id=u.tenant_id AND r.upload_no=u.upload_no AND r.target_no=u.target_no AND r.surface_code=u.surface_code
 WHERE u.tenant_id=0 AND u.target_no=? AND u.surface_code='product' AND u.status_code='bound' AND r.configuration_revision=? AND u.upload_no IN(?) ORDER BY u.upload_no FOR UPDATE OF u`
	if err = s.database.GetScan(ctx, &rows, query, source.VersionNo, source.Version, tokens); err != nil {
		return "", "", err
	}
	if len(rows) != len(tokens) {
		return "", "", bizerr.NewCode(catalog.CodeManagementNotAccessible)
	}
	var (
		replacements = make(map[string]string, len(rows))
		placeholders = make([]string, 0, len(rows))
		args         = make([]any, 0, len(rows)*2+4)
	)
	for _, row := range rows {
		if err := validateCopiedFilePolicy(schema, fields[row.UploadNo], row); err != nil {
			return "", "", err
		}

		token, tokenErr := newManagementFileToken(s.random)
		if tokenErr != nil {
			return "", "", tokenErr
		}
		replacements[row.UploadNo] = token
		placeholders = append(placeholders, "(?::varchar,?::varchar)")
		args = append(args, row.UploadNo, token)
	}
	if err = validateCopiedFileTotals(schema, rows); err != nil {
		return "", "", err
	}
	// The mapping is generated internally; no SQL text derives from a field or
	// token. Immutable source uploads keep the shared private object alive.
	insert := `WITH copies(source_token,target_token) AS (VALUES ` + strings.Join(placeholders, ",") + `)
 INSERT INTO plugin_idc_core_management_configuration_upload(tenant_id,upload_no,surface_code,target_no,target_revision,uploader_user_id,field_key,policy_id,policy_snapshot,file_name,declared_size_bytes,actual_size_bytes,media_type,sha256_hex,object_ref,status_code,scan_status,expires_at,completed_at,bound_revision,bound_at,version)
 SELECT 0,c.target_token,u.surface_code,?,?,?,u.field_key,u.policy_id,u.policy_snapshot,u.file_name,u.declared_size_bytes,u.actual_size_bytes,u.media_type,u.sha256_hex,u.object_ref,u.status_code,u.scan_status,u.expires_at,u.completed_at,?,CURRENT_TIMESTAMP,1
 FROM copies c JOIN plugin_idc_core_management_configuration_upload u ON u.tenant_id=0 AND u.upload_no=c.source_token AND u.status_code='bound'`
	args = append(args, target, source.Version, actor, source.Version)
	result, err := s.database.Exec(ctx, insert, args...)
	if err != nil {
		return "", "", err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return "", "", err
	}
	if count != int64(len(rows)) {
		return "", "", bizerr.NewCode(catalog.CodeManagementConflict)
	}
	newTokens := make([]string, 0, len(rows))
	for _, value := range values {
		if len(value.FileTokens) == 0 {
			continue
		}
		copied := make([]string, len(value.FileTokens))
		for i, token := range value.FileTokens {
			copied[i] = replacements[token]
			newTokens = append(newTokens, copied[i])
		}
		raw, marshalErr := json.Marshal(copied)
		if marshalErr != nil {
			return "", "", marshalErr
		}
		document[value.FieldKey] = raw
	}
	result, err = s.database.Exec(ctx, `INSERT INTO plugin_idc_core_management_configuration_file_reference(tenant_id,surface_code,target_no,configuration_revision,upload_no,field_key,policy_id,created_by)
 SELECT tenant_id,surface_code,target_no,?,upload_no,field_key,policy_id,? FROM plugin_idc_core_management_configuration_upload WHERE tenant_id=0 AND target_no=? AND upload_no IN(?) AND status_code='bound'`, source.Version, actor, target, newTokens)
	if err != nil {
		return "", "", err
	}
	count, err = result.RowsAffected()
	if err != nil {
		return "", "", err
	}
	if count != int64(len(rows)) {
		return "", "", bizerr.NewCode(catalog.CodeManagementConflict)
	}
	raw, err := json.Marshal(document)
	if err != nil {
		return "", "", err
	}
	return string(raw), configurationDigest(raw), nil
}

// validateCopiedFileTotals enforces each field's frozen aggregate limits before
// any target upload is inserted. Subtraction avoids overflowing byte totals.
func validateCopiedFileTotals(schema *capmodel.ConfigurationSchemaDeclaration, rows []*managementFileRow) error {
	counts := make(map[string]int)
	totals := make(map[string]int64)
	for _, row := range rows {
		policy := managementFilePolicy(schema, row.FieldKey)
		if policy == nil || counts[row.FieldKey] >= policy.MaximumFiles || row.ActualSize < 1 || row.ActualSize > policy.MaximumTotalBytes-totals[row.FieldKey] {
			return bizerr.NewCode(catalog.CodeManagementConflict)
		}
		counts[row.FieldKey]++
		totals[row.FieldKey] += row.ActualSize
	}
	return nil
}

// validateCopiedFilePolicy rejects changed policy or unsafe immutable evidence.
func validateCopiedFilePolicy(schema *capmodel.ConfigurationSchemaDeclaration, field string, row *managementFileRow) error {
	policy := managementFilePolicy(schema, field)
	snapshot := new(capmodel.ConfigurationUploadPolicyDeclaration)
	if policy == nil || row.FieldKey != field || row.PolicyID != policy.ID || strictManagementJSON(row.PolicySnapshot, snapshot) != nil || !sameManagementPolicy(snapshot, policy) || row.ActualSize < 1 || row.ActualSize > policy.MaximumFileBytes || !managementMIMEAllowed(policy, row.MediaType) || (row.ScanStatus != "clean" && row.ScanStatus != "not_required") {
		return bizerr.NewCode(catalog.CodeManagementConflict)
	}
	return nil
}
