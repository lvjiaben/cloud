// This file owns private staff uploads used by interface and product configuration.
package productautomation

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gogf/gf/v2/database/gdb"

	"lina-core/pkg/bizerr"
	"lina-core/pkg/plugin/capability/capmodel"
	"lina-core/pkg/plugin/capability/plugincap"
	"lina-core/pkg/plugin/capability/storagecap"
	"lina-plugin-idc-core/backend/internal/service/catalog"
)

const (
	managementFileReserveLifetime = time.Hour
	managementFileReadyLifetime   = 7 * 24 * time.Hour
)

type managementFileRow struct {
	UploadNo, Surface, TargetNo, FieldKey, PolicyID, PolicySnapshot, FileName, MediaType, ObjectRef, Status, ScanStatus, SHA256Hex string
	TargetRevision, DeclaredSize, ActualSize, UploaderUserID, BoundRevision, Version                                               int64
	ExpiresAt                                                                                                                      time.Time
}

func (s *serviceImpl) reconcileManagementFileObject(ctx context.Context, row *managementFileRow) (*ManagementConfigurationFileItem, bool, error) {
	if row == nil || row.Status != "uploading" {
		return nil, false, bizerr.NewCode(catalog.CodeManagementConflict)
	}
	if row.ActualSize < 1 || row.MediaType == "" || len(row.SHA256Hex) != 64 {
		return nil, false, nil
	}
	object, err := s.storage.Get(ctx, storagecap.GetInput{Path: row.ObjectRef})
	if err != nil {
		return nil, false, bizerr.WrapCode(err, catalog.CodeManagementUnavailable)
	}
	if object == nil || !object.Found {
		return nil, false, nil
	}
	if object.Body == nil {
		return nil, false, bizerr.NewCode(catalog.CodeManagementUnavailable)
	}
	hash := sha256.New()
	size, readErr := io.Copy(hash, io.LimitReader(object.Body, row.ActualSize+1))
	closeErr := object.Body.Close()
	if readErr != nil || closeErr != nil {
		return nil, false, bizerr.WrapCode(errors.Join(readErr, closeErr), catalog.CodeManagementUnavailable)
	}
	actualDigest := hex.EncodeToString(hash.Sum(nil))
	if size != row.ActualSize || actualDigest != row.SHA256Hex || (object.Object != nil && object.Object.ContentType != "" && !strings.EqualFold(object.Object.ContentType, row.MediaType)) {
		deleteErr := s.storage.Delete(ctx, storagecap.DeleteInput{Path: row.ObjectRef})
		_, updateErr := s.database.Exec(ctx, `UPDATE plugin_idc_core_management_configuration_upload SET status_code='rejected',scan_status='failed',version=version+1,updated_at=CURRENT_TIMESTAMP WHERE tenant_id=0 AND upload_no=? AND status_code='uploading' AND version=?`, row.UploadNo, row.Version)
		return nil, false, errors.Join(bizerr.NewCode(catalog.CodeManagementConflict), deleteErr, updateErr)
	}
	var completed struct {
		ExpiresAt time.Time `json:"expires_at"`
	}
	err = s.database.GetScan(ctx, &completed, `UPDATE plugin_idc_core_management_configuration_upload SET status_code='ready',completed_at=CURRENT_TIMESTAMP,expires_at=CURRENT_TIMESTAMP+(?*INTERVAL '1 second'),version=version+1,updated_at=CURRENT_TIMESTAMP WHERE tenant_id=0 AND upload_no=? AND uploader_user_id=? AND status_code='uploading' AND version=? RETURNING expires_at`, int64(managementFileReadyLifetime/time.Second), row.UploadNo, row.UploaderUserID, row.Version)
	if err != nil {
		return nil, false, bizerr.WrapCode(err, catalog.CodeManagementUnavailable)
	}
	return &ManagementConfigurationFileItem{FileToken: row.UploadNo, FileName: row.FileName, MediaType: row.MediaType, Status: "ready", SizeBytes: row.ActualSize, ExpiresAt: completed.ExpiresAt}, true, nil
}

func (s *serviceImpl) ReserveManagementConfigurationFile(ctx context.Context, input ManagementConfigurationFileInput) (*ManagementConfigurationFileItem, error) {
	auth, schema, err := s.authorizeManagementFileTarget(ctx, input)
	if err != nil {
		return nil, err
	}
	policy := managementFilePolicy(schema, input.FieldKey)
	name := strings.TrimSpace(input.FileName)
	if policy == nil || !validManagementFileName(name, policy) || input.SizeBytes < 1 || input.SizeBytes > policy.MaximumFileBytes {
		return nil, bizerr.NewCode(catalog.CodeManagementInvalid)
	}
	token, err := newManagementFileToken(s.random)
	if err != nil {
		return nil, bizerr.WrapCode(err, catalog.CodeManagementUnavailable)
	}
	policyJSON, err := json.Marshal(policy)
	if err != nil {
		return nil, bizerr.WrapCode(err, catalog.CodeManagementUnavailable)
	}
	var result struct {
		ExpiresAt time.Time `json:"expires_at"`
	}
	errQuota := errors.New("management configuration file quota exceeded")
	err = s.database.Transaction(ctx, func(txCtx context.Context, _ gdb.TX) error {
		lockKey := strings.Join([]string{string(input.Surface), input.TargetNo, input.FieldKey, policy.ID, strconv.Itoa(auth.ActorUserID)}, "\x1f")
		if _, lockErr := s.database.Exec(txCtx, `SELECT pg_advisory_xact_lock(hashtextextended(?,0))`, lockKey); lockErr != nil {
			return lockErr
		}
		var usage struct{ Count, Total int64 }
		if loadErr := s.database.GetScan(txCtx, &usage, `SELECT COUNT(*) AS count,COALESCE(SUM(declared_size_bytes),0) AS total FROM plugin_idc_core_management_configuration_upload WHERE tenant_id=0 AND surface_code=? AND target_no=? AND target_revision=? AND uploader_user_id=? AND field_key=? AND policy_id=? AND status_code IN('reserved','uploading','ready') AND expires_at>CURRENT_TIMESTAMP`, input.Surface, input.TargetNo, input.TargetRevision, auth.ActorUserID, input.FieldKey, policy.ID); loadErr != nil {
			return loadErr
		}
		if usage.Count >= int64(policy.MaximumFiles) || usage.Total > policy.MaximumTotalBytes-input.SizeBytes {
			return errQuota
		}
		return s.database.GetScan(txCtx, &result, `INSERT INTO plugin_idc_core_management_configuration_upload (tenant_id,upload_no,surface_code,target_no,target_revision,uploader_user_id,field_key,policy_id,policy_snapshot,file_name,declared_size_bytes,object_ref,status_code,scan_status,expires_at,created_at,updated_at) VALUES (0,?,?,?,?,?,?,?,?,?,?,?,'reserved','pending',CURRENT_TIMESTAMP+(?*INTERVAL '1 second'),CURRENT_TIMESTAMP,CURRENT_TIMESTAMP) RETURNING expires_at`, token, input.Surface, input.TargetNo, input.TargetRevision, auth.ActorUserID, input.FieldKey, policy.ID, policyJSON, name, input.SizeBytes, "management-configuration/"+token, int64(managementFileReserveLifetime/time.Second))
	})
	if err != nil {
		if errors.Is(err, errQuota) {
			return nil, bizerr.NewCode(catalog.CodeManagementInvalid)
		}
		return nil, bizerr.WrapCode(err, catalog.CodeManagementUnavailable)
	}
	return &ManagementConfigurationFileItem{FileToken: token, FileName: name, Status: "reserved", SizeBytes: input.SizeBytes, ExpiresAt: result.ExpiresAt}, nil
}

//nolint:cyclop // The upload transition intentionally keeps inspection, durable preparation, private storage, and reconciliation in one auditable state machine.
func (s *serviceImpl) UploadManagementConfigurationFile(ctx context.Context, input ManagementConfigurationFileInput, token, declaredType string, body io.Reader) (_ *ManagementConfigurationFileItem, resultErr error) {
	auth, _, err := s.authorizeManagementFileTarget(ctx, input)
	if err != nil {
		return nil, err
	}
	row, err := s.loadManagementFile(ctx, input, token, auth.ActorUserID, false)
	if err != nil || body == nil {
		return nil, bizerr.NewCode(catalog.CodeManagementNotAccessible)
	}
	if row.Status == "uploading" {
		item, found, reconcileErr := s.reconcileManagementFileObject(ctx, row)
		if reconcileErr != nil || found {
			return item, reconcileErr
		}
		result, resetErr := s.database.Exec(ctx, `UPDATE plugin_idc_core_management_configuration_upload SET status_code='reserved',actual_size_bytes=0,media_type='',sha256_hex='',scan_status='pending',version=version+1,updated_at=CURRENT_TIMESTAMP WHERE tenant_id=0 AND upload_no=? AND uploader_user_id=? AND status_code='uploading' AND version=?`, token, auth.ActorUserID, row.Version)
		if resetErr != nil {
			return nil, bizerr.WrapCode(resetErr, catalog.CodeManagementUnavailable)
		}
		changed, _ := result.RowsAffected()
		if changed != 1 {
			return nil, bizerr.NewCode(catalog.CodeManagementConflict)
		}
		row.Status = "reserved"
		row.Version++
	}
	if row.Status != "reserved" {
		return nil, bizerr.NewCode(catalog.CodeManagementNotAccessible)
	}
	changed, err := s.database.Exec(ctx, `UPDATE plugin_idc_core_management_configuration_upload SET status_code='uploading',version=version+1,updated_at=CURRENT_TIMESTAMP WHERE tenant_id=0 AND upload_no=? AND uploader_user_id=? AND status_code='reserved' AND version=?`, token, auth.ActorUserID, row.Version)
	if err != nil {
		return nil, bizerr.WrapCode(err, catalog.CodeManagementUnavailable)
	}
	count, _ := changed.RowsAffected()
	if count != 1 {
		return nil, bizerr.NewCode(catalog.CodeManagementConflict)
	}
	row.Version++
	restore := true
	defer func() {
		if restore {
			_, restoreErr := s.database.Exec(ctx, `UPDATE plugin_idc_core_management_configuration_upload SET status_code='reserved',version=version+1,updated_at=CURRENT_TIMESTAMP WHERE tenant_id=0 AND upload_no=? AND uploader_user_id=? AND status_code='uploading' AND version=?`, token, auth.ActorUserID, row.Version)
			resultErr = errors.Join(resultErr, restoreErr)
		}
	}()
	policy := new(capmodel.ConfigurationUploadPolicyDeclaration)
	if strictManagementJSON(row.PolicySnapshot, policy) != nil || policy.ID != row.PolicyID {
		return nil, bizerr.NewCode(catalog.CodeManagementConflict)
	}
	temporary, err := os.CreateTemp("", "idc-management-file-*")
	if err != nil {
		return nil, bizerr.WrapCode(err, catalog.CodeManagementUnavailable)
	}
	defer func() {
		resultErr = errors.Join(resultErr, temporary.Close())
		if removeErr := os.Remove(temporary.Name()); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
			resultErr = errors.Join(resultErr, removeErr)
		}
	}()
	hash := sha256.New()
	written, err := io.Copy(io.MultiWriter(temporary, hash), io.LimitReader(body, row.DeclaredSize+1))
	if err != nil || written != row.DeclaredSize {
		restore = false
		_, _ = s.database.Exec(ctx, `UPDATE plugin_idc_core_management_configuration_upload SET status_code='rejected',scan_status='failed',version=version+1,updated_at=CURRENT_TIMESTAMP WHERE tenant_id=0 AND upload_no=? AND status_code='uploading' AND version=?`, token, row.Version)
		return nil, bizerr.NewCode(catalog.CodeManagementInvalid)
	}
	_, _ = temporary.Seek(0, io.SeekStart)
	probe := make([]byte, 512)
	read, readErr := io.ReadFull(temporary, probe)
	if readErr != nil && !errors.Is(readErr, io.ErrUnexpectedEOF) {
		return nil, bizerr.WrapCode(readErr, catalog.CodeManagementUnavailable)
	}
	actualType := strings.ToLower(strings.TrimSpace(strings.Split(http.DetectContentType(probe[:read]), ";")[0]))
	declaredType = strings.ToLower(strings.TrimSpace(strings.Split(declaredType, ";")[0]))
	if !managementMIMEAllowed(policy, actualType) || (declaredType != "" && declaredType != "application/octet-stream" && declaredType != actualType) {
		restore = false
		_, _ = s.database.Exec(ctx, `UPDATE plugin_idc_core_management_configuration_upload SET status_code='rejected',scan_status='failed',version=version+1,updated_at=CURRENT_TIMESTAMP WHERE tenant_id=0 AND upload_no=? AND status_code='uploading' AND version=?`, token, row.Version)
		return nil, bizerr.NewCode(catalog.CodeManagementInvalid)
	}
	_, _ = temporary.Seek(0, io.SeekStart)
	scanStatus := "not_required"
	if policy.SafetyScanRequired {
		if !s.fileScanner.Ready() {
			return nil, bizerr.NewCode(catalog.CodeManagementUnavailable)
		}
		verdict, scanErr := s.fileScanner.Scan(ctx, temporary, written)
		if scanErr != nil {
			return nil, bizerr.WrapCode(scanErr, catalog.CodeManagementUnavailable)
		}
		if verdict != "clean" {
			restore = false
			_, _ = s.database.Exec(ctx, `UPDATE plugin_idc_core_management_configuration_upload SET status_code='rejected',scan_status='risk',version=version+1,updated_at=CURRENT_TIMESTAMP WHERE tenant_id=0 AND upload_no=? AND status_code='uploading' AND version=?`, token, row.Version)
			return nil, bizerr.NewCode(catalog.CodeManagementInvalid)
		}
		scanStatus = "clean"
	}
	digest := hex.EncodeToString(hash.Sum(nil))
	prepared, prepareErr := s.database.Exec(ctx, `UPDATE plugin_idc_core_management_configuration_upload SET actual_size_bytes=?,media_type=?,sha256_hex=?,scan_status=?,version=version+1,updated_at=CURRENT_TIMESTAMP WHERE tenant_id=0 AND upload_no=? AND uploader_user_id=? AND status_code='uploading' AND version=?`, written, actualType, digest, scanStatus, token, auth.ActorUserID, row.Version)
	if prepareErr != nil {
		return nil, bizerr.WrapCode(prepareErr, catalog.CodeManagementUnavailable)
	}
	preparedCount, _ := prepared.RowsAffected()
	if preparedCount != 1 {
		return nil, bizerr.NewCode(catalog.CodeManagementConflict)
	}
	row.ActualSize = written
	row.MediaType = actualType
	row.ScanStatus = scanStatus
	row.SHA256Hex = digest
	row.Version++
	restore = false
	_, _ = temporary.Seek(0, io.SeekStart)
	_, putErr := s.storage.Put(ctx, storagecap.PutInput{Path: row.ObjectRef, Body: temporary, Size: written, ContentType: actualType, Overwrite: false})
	if putErr != nil { /* outcome is resolved below */
	}
	item, found, reconcileErr := s.reconcileManagementFileObject(ctx, row)
	if reconcileErr != nil {
		return nil, reconcileErr
	}
	if found {
		return item, nil
	}
	if putErr != nil {
		result, resetErr := s.database.Exec(ctx, `UPDATE plugin_idc_core_management_configuration_upload SET status_code='reserved',actual_size_bytes=0,media_type='',sha256_hex='',scan_status='pending',version=version+1,updated_at=CURRENT_TIMESTAMP WHERE tenant_id=0 AND upload_no=? AND uploader_user_id=? AND status_code='uploading' AND version=?`, token, auth.ActorUserID, row.Version)
		if resetErr != nil {
			return nil, bizerr.WrapCode(resetErr, catalog.CodeManagementUnavailable)
		}
		changed, _ := result.RowsAffected()
		if changed != 1 {
			return nil, bizerr.NewCode(catalog.CodeManagementConflict)
		}
		return nil, bizerr.WrapCode(putErr, catalog.CodeManagementUnavailable)
	}
	return nil, bizerr.NewCode(catalog.CodeManagementUnavailable)
}

func (s *serviceImpl) GetManagementConfigurationFile(ctx context.Context, input ManagementConfigurationFileInput, token string) (*ManagementConfigurationFileItem, error) {
	auth, _, err := s.authorizeManagementFileTarget(ctx, input)
	if err != nil {
		return nil, err
	}
	row, err := s.loadManagementFileForRead(ctx, input, token, auth.ActorUserID)
	if err != nil {
		return nil, err
	}
	return managementFileItem(row), nil
}
func (s *serviceImpl) RevokeManagementConfigurationFile(ctx context.Context, input ManagementConfigurationFileInput, token string) (*ManagementConfigurationFileItem, error) {
	auth, _, err := s.authorizeManagementFileTarget(ctx, input)
	if err != nil {
		return nil, err
	}
	row, err := s.loadManagementFile(ctx, input, token, auth.ActorUserID, true)
	if err != nil {
		return nil, err
	}
	if row.Status == "bound" {
		return nil, bizerr.NewCode(catalog.CodeManagementConflict)
	}
	if row.Status != "reserved" && row.Status != "ready" && row.Status != "revoked" {
		return nil, bizerr.NewCode(catalog.CodeManagementNotAccessible)
	}
	if row.Status != "revoked" {
		result, updateErr := s.database.Exec(ctx, `UPDATE plugin_idc_core_management_configuration_upload SET status_code='revoked',version=version+1,updated_at=CURRENT_TIMESTAMP WHERE tenant_id=0 AND upload_no=? AND uploader_user_id=? AND status_code=? AND version=?`, token, auth.ActorUserID, row.Status, row.Version)
		if updateErr != nil {
			return nil, bizerr.WrapCode(updateErr, catalog.CodeManagementUnavailable)
		}
		changed, _ := result.RowsAffected()
		if changed != 1 {
			return nil, bizerr.NewCode(catalog.CodeManagementConflict)
		}
		row.Status = "revoked"
	}
	return managementFileItem(row), nil
}

func (s *serviceImpl) authorizeManagementFileTarget(ctx context.Context, input ManagementConfigurationFileInput) (Authorization, *capmodel.ConfigurationSchemaDeclaration, error) {
	if input.Surface == configurationSurfaceInterface {
		auth, err := s.authorizer.AuthorizeInterfaceConfiguration(ctx, input.TargetNo)
		if err != nil {
			return Authorization{}, nil, err
		}
		if err = requireAuthorization(auth); err != nil {
			return Authorization{}, nil, err
		}
		_, exact, err := s.loadExactInterfaceEvidence(ctx, input.TargetNo)
		if err != nil {
			return Authorization{}, nil, err
		}
		revision, err := s.loadLatestInterfaceConfigurationRevision(ctx, input.TargetNo, false)
		if err != nil || revision == nil || revision.Revision != input.TargetRevision {
			return Authorization{}, nil, bizerr.NewCode(catalog.CodeManagementConflict)
		}
		return auth, exact.InterfaceConfiguration, nil
	}
	if input.Surface == configurationSurfaceProduct {
		auth, err := s.authorizer.AuthorizeVersionConfiguration(ctx, input.TargetNo)
		if err != nil {
			return Authorization{}, nil, err
		}
		if err = requireAuthorization(auth); err != nil {
			return Authorization{}, nil, err
		}
		binding, err := s.loadBinding(ctx, input.TargetNo, false)
		if err != nil || binding == nil {
			return Authorization{}, nil, bizerr.NewCode(catalog.CodeManagementConflict)
		}
		exact, err := s.extensions.GetProviderExact(ctx, plugincap.BusinessExtensionProviderExactInput{PluginID: binding.PluginId, ReleaseVersion: binding.PluginReleaseVersion, Point: binding.ExtensionPointCode, ContractVersion: binding.ContractVersion, ReleaseDigest: binding.ReleaseDigest, DeclarationDigest: binding.DeclarationDigest})
		if err != nil || !sameProviderContract(bindingEvidence(binding), exact) {
			return Authorization{}, nil, bizerr.NewCode(CodeEvidenceUnavailable)
		}
		var current struct{ Version int64 }
		if loadErr := s.database.GetScan(ctx, &current, `SELECT version FROM plugin_idc_core_catalog_product_version_configuration WHERE tenant_id=0 AND version_no=? LIMIT 1`, input.TargetNo); loadErr != nil {
			return Authorization{}, nil, bizerr.WrapCode(loadErr, catalog.CodeManagementUnavailable)
		}
		if current.Version != input.TargetRevision {
			return Authorization{}, nil, bizerr.NewCode(catalog.CodeManagementConflict)
		}
		return auth, exact.ProductConfiguration, nil
	}
	return Authorization{}, nil, bizerr.NewCode(catalog.CodeManagementInvalid)
}

func (s *serviceImpl) loadManagementFileForRead(ctx context.Context, input ManagementConfigurationFileInput, token string, uploader int) (*managementFileRow, error) {
	if !validManagementFileToken(token) {
		return nil, bizerr.NewCode(catalog.CodeManagementNotAccessible)
	}
	var row *managementFileRow
	err := s.database.GetScan(ctx, &row, `SELECT upload.upload_no,upload.surface_code AS surface,upload.target_no,upload.target_revision,upload.uploader_user_id,upload.field_key,upload.policy_id,upload.file_name,upload.actual_size_bytes AS actual_size,upload.media_type,upload.status_code AS status,COALESCE(upload.bound_revision,0) AS bound_revision,upload.version,upload.expires_at FROM plugin_idc_core_management_configuration_upload upload WHERE upload.tenant_id=0 AND upload.upload_no=? AND upload.surface_code=? AND upload.target_no=? AND ((upload.status_code='ready' AND upload.target_revision=? AND upload.uploader_user_id=? AND upload.expires_at>CURRENT_TIMESTAMP) OR (upload.status_code='bound' AND EXISTS (SELECT 1 FROM plugin_idc_core_management_configuration_file_reference reference WHERE reference.tenant_id=upload.tenant_id AND reference.surface_code=upload.surface_code AND reference.target_no=upload.target_no AND reference.configuration_revision=? AND reference.upload_no=upload.upload_no))) LIMIT 1`, token, input.Surface, input.TargetNo, input.TargetRevision, uploader, input.TargetRevision)
	if err != nil {
		return nil, bizerr.WrapCode(err, catalog.CodeManagementUnavailable)
	}
	if row == nil {
		return nil, bizerr.NewCode(catalog.CodeManagementNotAccessible)
	}
	return row, nil
}

func (s *serviceImpl) loadManagementFile(ctx context.Context, input ManagementConfigurationFileInput, token string, uploader int, includeExpired bool) (*managementFileRow, error) {
	if !validManagementFileToken(token) {
		return nil, bizerr.NewCode(catalog.CodeManagementNotAccessible)
	}
	query := `SELECT upload_no,surface_code AS surface,target_no,target_revision,uploader_user_id,field_key,policy_id,policy_snapshot::text,file_name,declared_size_bytes AS declared_size,actual_size_bytes AS actual_size,media_type,sha256_hex,object_ref,status_code AS status,scan_status,COALESCE(bound_revision,0) AS bound_revision,version,expires_at FROM plugin_idc_core_management_configuration_upload WHERE tenant_id=0 AND upload_no=? AND surface_code=? AND target_no=? AND target_revision=? AND uploader_user_id=?`
	if !includeExpired {
		query += ` AND (status_code='bound' OR expires_at>CURRENT_TIMESTAMP)`
	}
	query += ` LIMIT 1`
	var row *managementFileRow
	if err := s.database.GetScan(ctx, &row, query, token, input.Surface, input.TargetNo, input.TargetRevision, uploader); err != nil || row == nil {
		return nil, bizerr.NewCode(catalog.CodeManagementNotAccessible)
	}
	return row, nil
}

func (s *serviceImpl) bindManagementConfigurationFiles(ctx context.Context, surface ConfigurationSurface, target string, sourceRevision, boundRevision int64, actor int, schema *capmodel.ConfigurationSchemaDeclaration, values []ConfigurationValue) error {
	policies := map[string]*capmodel.ConfigurationUploadPolicyDeclaration{}
	if schema != nil {
		for _, policy := range schema.UploadPolicies {
			if policy != nil {
				policies[policy.ID] = policy
			}
		}
	}
	tokens := []string{}
	expected := map[string]struct {
		field  string
		policy *capmodel.ConfigurationUploadPolicyDeclaration
	}{}
	for _, value := range values {
		if len(value.FileTokens) == 0 {
			continue
		}
		field := managementFileField(schema, value.FieldKey)
		if field == nil {
			return bizerr.NewCode(catalog.CodeManagementInvalid)
		}
		policy := policies[field.UploadPolicy]
		for _, token := range value.FileTokens {
			if !validManagementFileToken(token) {
				return bizerr.NewCode(catalog.CodeManagementInvalid)
			}
			if _, exists := expected[token]; exists {
				return bizerr.NewCode(catalog.CodeManagementInvalid)
			}
			tokens = append(tokens, token)
			expected[token] = struct {
				field  string
				policy *capmodel.ConfigurationUploadPolicyDeclaration
			}{field.Key, policy}
		}
	}
	if len(tokens) == 0 {
		return nil
	}
	sort.Strings(tokens)
	var rows []*managementFileRow
	const evidenceQuery = `SELECT upload.upload_no,upload.surface_code AS surface,upload.target_no,upload.target_revision,
upload.uploader_user_id,upload.field_key,upload.policy_id,upload.policy_snapshot::text,upload.file_name,
upload.declared_size_bytes AS declared_size,upload.actual_size_bytes AS actual_size,upload.media_type,
upload.object_ref,upload.status_code AS status,upload.scan_status,COALESCE(upload.bound_revision,0) AS bound_revision,
upload.version,upload.expires_at FROM plugin_idc_core_management_configuration_upload upload
WHERE upload.tenant_id=0 AND upload.upload_no IN(?) AND upload.surface_code=? AND upload.target_no=? AND
((upload.status_code='ready' AND upload.target_revision=? AND upload.uploader_user_id=? AND upload.expires_at>CURRENT_TIMESTAMP)
OR (upload.status_code='bound' AND EXISTS (SELECT 1 FROM plugin_idc_core_management_configuration_file_reference reference
WHERE reference.tenant_id=upload.tenant_id AND reference.surface_code=upload.surface_code AND
reference.target_no=upload.target_no AND reference.configuration_revision=? AND reference.upload_no=upload.upload_no)))
ORDER BY upload.upload_no FOR UPDATE`
	if err := s.database.GetScan(ctx, &rows, evidenceQuery, tokens, surface, target, sourceRevision, actor, sourceRevision); err != nil {
		return err
	}
	if len(rows) != len(tokens) {
		return bizerr.NewCode(catalog.CodeManagementNotAccessible)
	}
	ready := []string{}
	for _, row := range rows {
		want := expected[row.UploadNo]
		snapshot := new(capmodel.ConfigurationUploadPolicyDeclaration)
		if want.policy == nil || row.FieldKey != want.field || row.PolicyID != want.policy.ID || strictManagementJSON(row.PolicySnapshot, snapshot) != nil || !sameManagementPolicy(snapshot, want.policy) || row.ActualSize < 1 || row.ActualSize > want.policy.MaximumFileBytes || !managementMIMEAllowed(want.policy, row.MediaType) || (row.ScanStatus != "clean" && row.ScanStatus != "not_required") {
			return bizerr.NewCode(catalog.CodeManagementConflict)
		}
		if row.Status == "ready" {
			ready = append(ready, row.UploadNo)
		} else if row.BoundRevision < 1 {
			return bizerr.NewCode(catalog.CodeManagementConflict)
		}
	}
	if len(ready) > 0 {
		result, err := s.database.Exec(ctx, `UPDATE plugin_idc_core_management_configuration_upload SET status_code='bound',bound_revision=?,bound_at=CURRENT_TIMESTAMP,version=version+1,updated_at=CURRENT_TIMESTAMP WHERE tenant_id=0 AND upload_no IN(?) AND status_code='ready' AND uploader_user_id=? AND expires_at>CURRENT_TIMESTAMP`, boundRevision, ready, actor)
		if err != nil {
			return err
		}
		changed, _ := result.RowsAffected()
		if changed != int64(len(ready)) {
			return bizerr.NewCode(catalog.CodeManagementConflict)
		}
	}
	for _, row := range rows {
		if _, err := s.database.Exec(ctx, `INSERT INTO plugin_idc_core_management_configuration_file_reference (tenant_id,surface_code,target_no,configuration_revision,upload_no,field_key,policy_id,created_by,created_at) VALUES (0,?,?,?,?,?,?,?,CURRENT_TIMESTAMP) ON CONFLICT (tenant_id,surface_code,target_no,configuration_revision,upload_no) DO NOTHING`, surface, target, boundRevision, row.UploadNo, row.FieldKey, row.PolicyID, actor); err != nil {
			return err
		}
	}
	return nil
}

func managementFilePolicy(schema *capmodel.ConfigurationSchemaDeclaration, key string) *capmodel.ConfigurationUploadPolicyDeclaration {
	field := managementFileField(schema, key)
	if field == nil {
		return nil
	}
	for _, policy := range schema.UploadPolicies {
		if policy != nil && policy.ID == field.UploadPolicy {
			return policy
		}
	}
	return nil
}
func managementFileField(schema *capmodel.ConfigurationSchemaDeclaration, key string) *capmodel.ConfigurationFieldDeclaration {
	if schema == nil {
		return nil
	}
	for _, field := range schema.Fields {
		if field != nil && field.Key == key && field.Type == capmodel.ConfigurationFieldFile {
			return field
		}
	}
	return nil
}
func validManagementFileName(name string, policy *capmodel.ConfigurationUploadPolicyDeclaration) bool {
	return name != "" && len(name) <= 255 && filepath.Base(name) == name && !strings.ContainsAny(name, "\x00\r\n") && managementExtensionAllowed(policy, name)
}
func managementExtensionAllowed(policy *capmodel.ConfigurationUploadPolicyDeclaration, name string) bool {
	ext := strings.ToLower(filepath.Ext(name))
	for _, allowed := range policy.AllowedExtensions {
		if strings.ToLower(allowed) == ext {
			return true
		}
	}
	return false
}
func managementMIMEAllowed(policy *capmodel.ConfigurationUploadPolicyDeclaration, value string) bool {
	for _, allowed := range policy.AllowedMIMETypes {
		if strings.EqualFold(allowed, value) {
			return true
		}
	}
	return false
}
func sameManagementPolicy(a, b *capmodel.ConfigurationUploadPolicyDeclaration) bool {
	left, leftErr := json.Marshal(a)
	right, rightErr := json.Marshal(b)
	if leftErr != nil || rightErr != nil {
		return false
	}
	return string(left) == string(right)
}
func strictManagementJSON(raw string, target any) error {
	decoder := json.NewDecoder(strings.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var extra any
	if decoder.Decode(&extra) != io.EOF {
		return errors.New("extra JSON value")
	}
	return nil
}
func validManagementFileToken(token string) bool {
	if len(token) != 36 || !strings.HasPrefix(token, "MCF_") {
		return false
	}
	for _, char := range token[4:] {
		if !(char >= '0' && char <= '9' || char >= 'A' && char <= 'F') {
			return false
		}
	}
	return true
}
func newManagementFileToken(random io.Reader) (string, error) {
	var value [16]byte
	if _, err := io.ReadFull(random, value[:]); err != nil {
		return "", err
	}
	return "MCF_" + strings.ToUpper(hex.EncodeToString(value[:])), nil
}
func managementFileItem(row *managementFileRow) *ManagementConfigurationFileItem {
	return &ManagementConfigurationFileItem{FileToken: row.UploadNo, FileName: row.FileName, MediaType: row.MediaType, Status: row.Status, SizeBytes: row.ActualSize, ExpiresAt: row.ExpiresAt}
}
