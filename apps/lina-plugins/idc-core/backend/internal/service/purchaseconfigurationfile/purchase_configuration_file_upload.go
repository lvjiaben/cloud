// This file implements durable reservation, bounded inspection, private
// storage writes, and exact-account status reads.

package purchaseconfigurationfile

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/gogf/gf/v2/database/gdb"

	"lina-core/pkg/bizerr"
	"lina-core/pkg/plugin/capability/capmodel"
	"lina-core/pkg/plugin/capability/storagecap"
	"lina-plugin-idc-core/backend/internal/service/principal"
)

const (
	reservationLifetime                = time.Hour
	readyTokenLifetime                 = 7 * 24 * time.Hour
	maximumUploadFiles                 = 100
	maximumUploadFileBytes             = int64(5 * 1024 * 1024 * 1024)
	maximumUploadTotalBytes            = int64(10 * 1024 * 1024 * 1024)
	maximumUploadPolicyChoices         = 32
	maximumConfigurationFieldsForFiles = 128
	maximumConfigurationFileTokens     = maximumConfigurationFieldsForFiles * maximumUploadFiles
)

var (
	uploadPolicyIDPattern  = regexp.MustCompile(`^[a-z][a-z0-9]*(?:[._-][a-z0-9]+)*$`)
	uploadMIMEPattern      = regexp.MustCompile(`^[a-z0-9][a-z0-9!#$&^_.+-]{0,63}/[a-z0-9][a-z0-9!#$&^_.+-]{0,63}$`)
	uploadExtensionPattern = regexp.MustCompile(`^\.[a-z0-9][a-z0-9._+-]{0,15}$`)
)

func (s *serviceImpl) Reserve(ctx context.Context, input ReserveInput) (*Reservation, error) {
	audience, err := s.currentCustomer(ctx)
	if err != nil {
		return nil, err
	}
	target, err := s.resolver.ResolvePurchaseFileTarget(ctx, strings.TrimSpace(input.PublicSlug), strings.TrimSpace(input.ConfigurationRevision), strings.TrimSpace(input.FieldToken))
	if err != nil || !validTarget(target) {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	rawFileName := strings.TrimSpace(input.FileName)
	fileName := filepath.Base(rawFileName)
	if fileName != rawFileName || fileName == "." || fileName == "" || len(fileName) > 255 || strings.ContainsAny(fileName, "\x00\r\n") || input.SizeBytes < 1 || input.SizeBytes > target.Policy.MaximumFileBytes || !extensionAllowed(target.Policy, fileName) {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	uploadToken, err := newUploadToken(s.random)
	if err != nil {
		return nil, bizerr.WrapCode(err, CodeUnavailable)
	}
	policySnapshot, err := json.Marshal(target.Policy)
	if err != nil {
		return nil, bizerr.WrapCode(err, CodeUnavailable)
	}
	var inserted struct {
		ExpiresAt time.Time `json:"expires_at"`
	}
	errReservationQuota := errors.New("purchase configuration upload policy quota exceeded")
	err = s.database.Transaction(ctx, func(txCtx context.Context, _ gdb.TX) error {
		lockKey := strings.Join([]string{audience.CustomerNo, target.ProductVersionNo, target.ConfigurationRevision, target.FieldKey, target.Policy.ID}, "\x1f")
		if _, lockErr := s.database.Exec(txCtx, `SELECT pg_advisory_xact_lock(hashtextextended(?, 0))`, lockKey); lockErr != nil {
			return lockErr
		}
		var usage struct {
			FileCount int64 `json:"file_count"`
			TotalSize int64 `json:"total_size"`
		}
		if usageErr := s.database.GetScan(txCtx, &usage, `SELECT COUNT(*) AS file_count,COALESCE(SUM(declared_size_bytes),0) AS total_size FROM plugin_idc_core_purchase_configuration_upload WHERE tenant_id=0 AND customer_no=? AND uploader_user_id=? AND product_version_no=? AND configuration_revision=? AND field_key=? AND policy_id=? AND status_code IN('reserved','uploading','ready') AND expires_at>CURRENT_TIMESTAMP`, audience.CustomerNo, audience.UserID, target.ProductVersionNo, target.ConfigurationRevision, target.FieldKey, target.Policy.ID); usageErr != nil {
			return usageErr
		}
		if usage.FileCount >= int64(target.Policy.MaximumFiles) || usage.TotalSize > target.Policy.MaximumTotalBytes-input.SizeBytes {
			return errReservationQuota
		}
		insertErr := s.database.GetScan(txCtx, &inserted, `INSERT INTO plugin_idc_core_purchase_configuration_upload
        (tenant_id,upload_no,customer_no,uploader_user_id,product_version_no,configuration_revision,field_key,policy_id,policy_snapshot,file_name,declared_size_bytes,object_ref,status_code,scan_status,expires_at,created_at,updated_at)
		VALUES (0,?,?,?,?,?,?,?,?,?,?,?,'reserved','pending',CURRENT_TIMESTAMP + (? * INTERVAL '1 second'),CURRENT_TIMESTAMP,CURRENT_TIMESTAMP)
		RETURNING expires_at`,
			uploadToken, audience.CustomerNo, audience.UserID, target.ProductVersionNo, target.ConfigurationRevision,
			target.FieldKey, target.Policy.ID, policySnapshot, fileName, input.SizeBytes,
			"purchase-configuration/"+uploadToken, int64(reservationLifetime/time.Second))
		return insertErr
	})
	if err != nil {
		if errors.Is(err, errReservationQuota) {
			return nil, bizerr.NewCode(CodeInvalid)
		}
		return nil, bizerr.WrapCode(err, CodeUnavailable)
	}
	return &Reservation{UploadToken: uploadToken, ExpiresAt: inserted.ExpiresAt}, nil
}

func (s *serviceImpl) Upload(ctx context.Context, token, declaredType string, body io.Reader) (_ *Item, resultErr error) {
	audience, err := s.currentCustomer(ctx)
	if err != nil {
		return nil, err
	}
	row, err := s.loadOwned(ctx, token, audience)
	if err != nil || row.Status != StatusReserved || body == nil {
		return nil, bizerr.NewCode(CodeNotAccessible)
	}
	claimed, err := s.claim(ctx, row, audience)
	if err != nil {
		return nil, err
	}
	if !claimed {
		return nil, bizerr.NewCode(CodeConflict)
	}
	row.Status = StatusUploading
	row.Version++
	releaseClaim := true
	defer func() {
		if releaseClaim {
			resultErr = errors.Join(resultErr, s.restoreReservation(ctx, row))
		}
	}()
	policy := new(capmodel.ConfigurationUploadPolicyDeclaration)
	if err = strictJSON(row.PolicySnapshot, policy); err != nil || !validPolicy(policy) || policy.ID != row.PolicyID || row.DeclaredSize > policy.MaximumFileBytes {
		return nil, bizerr.NewCode(CodeConflict)
	}
	temporary, err := os.CreateTemp("", "idc-purchase-file-*")
	if err != nil {
		return nil, bizerr.WrapCode(err, CodeUnavailable)
	}
	defer func() {
		if closeErr := temporary.Close(); closeErr != nil {
			resultErr = errors.Join(resultErr, bizerr.WrapCode(closeErr, CodeUnavailable))
		}
		if removeErr := os.Remove(temporary.Name()); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
			resultErr = errors.Join(resultErr, bizerr.WrapCode(removeErr, CodeUnavailable))
		}
	}()
	hash := sha256.New()
	written, err := io.Copy(io.MultiWriter(temporary, hash), io.LimitReader(body, row.DeclaredSize+1))
	if err != nil || written != row.DeclaredSize {
		releaseClaim = false
		return s.reject(ctx, row, "failed", bizerr.NewCode(CodeRejected))
	}
	if _, err = temporary.Seek(0, io.SeekStart); err != nil {
		return nil, bizerr.WrapCode(err, CodeUnavailable)
	}
	probe := make([]byte, 512)
	read, readErr := io.ReadFull(temporary, probe)
	if readErr != nil && !errors.Is(readErr, io.ErrUnexpectedEOF) {
		return nil, bizerr.WrapCode(readErr, CodeUnavailable)
	}
	actualType := strings.ToLower(strings.TrimSpace(strings.Split(http.DetectContentType(probe[:read]), ";")[0]))
	declaredType = strings.ToLower(strings.TrimSpace(strings.Split(declaredType, ";")[0]))
	if !mimeAllowed(policy, actualType) || (declaredType != "" && declaredType != "application/octet-stream" && declaredType != actualType) {
		releaseClaim = false
		return s.reject(ctx, row, "failed", bizerr.NewCode(CodeRejected))
	}
	if _, err = temporary.Seek(0, io.SeekStart); err != nil {
		return nil, bizerr.WrapCode(err, CodeUnavailable)
	}
	scanStatus := "not_required"
	if policy.SafetyScanRequired {
		if !s.scanner.Ready() {
			return nil, bizerr.NewCode(CodeUnavailable)
		}
		verdict, scanErr := s.scanner.Scan(ctx, temporary, written)
		if scanErr != nil {
			return nil, bizerr.WrapCode(scanErr, CodeUnavailable)
		}
		if verdict != "clean" {
			releaseClaim = false
			return s.reject(ctx, row, "risk", bizerr.NewCode(CodeRejected))
		}
		scanStatus = "clean"
	}
	if _, err = temporary.Seek(0, io.SeekStart); err != nil {
		return nil, bizerr.WrapCode(err, CodeUnavailable)
	}
	if _, err = s.storage.Put(ctx, storagecap.PutInput{Path: row.ObjectRef, Body: temporary, Size: written, ContentType: actualType, Overwrite: false}); err != nil {
		if deleteErr := s.storage.Delete(ctx, storagecap.DeleteInput{Path: row.ObjectRef}); deleteErr != nil {
			releaseClaim = false
			return nil, errors.Join(bizerr.WrapCode(err, CodeUnavailable), deleteErr)
		}
		return nil, bizerr.WrapCode(err, CodeUnavailable)
	}
	digest := hex.EncodeToString(hash.Sum(nil))
	// A transport error after the final update has an unknown database outcome.
	// Never reopen the reservation or delete an object a successful commit may reference.
	releaseClaim = false
	var completed struct {
		ExpiresAt time.Time `json:"expires_at"`
	}
	err = s.database.GetScan(ctx, &completed, `UPDATE plugin_idc_core_purchase_configuration_upload SET actual_size_bytes=?,media_type=?,sha256_hex=?,status_code='ready',scan_status=?,completed_at=CURRENT_TIMESTAMP,expires_at=CURRENT_TIMESTAMP + (? * INTERVAL '1 second'),version=version+1,updated_at=CURRENT_TIMESTAMP WHERE tenant_id=0 AND upload_no=? AND customer_no=? AND uploader_user_id=? AND status_code='uploading' AND version=? RETURNING expires_at`, written, actualType, digest, scanStatus, int64(readyTokenLifetime/time.Second), row.UploadNo, audience.CustomerNo, audience.UserID, row.Version)
	if err != nil {
		// The database outcome is unknown. Keeping a private orphan is safer than
		// deleting an object that a successful commit may already reference.
		return nil, bizerr.WrapCode(err, CodeUnavailable)
	}
	return &Item{FileToken: row.UploadNo, FileName: row.FileName, MediaType: actualType, SizeBytes: written, Status: StatusReady, ExpiresAt: completed.ExpiresAt}, nil
}

func (s *serviceImpl) Get(ctx context.Context, token string) (*Item, error) {
	audience, err := s.currentCustomer(ctx)
	if err != nil {
		return nil, err
	}
	row, err := s.loadOwned(ctx, token, audience)
	if err != nil {
		return nil, err
	}
	return &Item{FileToken: row.UploadNo, FileName: row.FileName, MediaType: row.MediaType, SizeBytes: row.ActualSize, Status: row.Status, ExpiresAt: row.ExpiresAt}, nil
}

func (s *serviceImpl) Revoke(ctx context.Context, token string) (*Item, error) {
	audience, err := s.currentCustomer(ctx)
	if err != nil {
		return nil, err
	}
	if !validUploadToken(token) {
		return nil, bizerr.NewCode(CodeNotAccessible)
	}
	var revoked uploadRow
	err = s.database.Transaction(ctx, func(txCtx context.Context, _ gdb.TX) error {
		var current uploadRow
		if loadErr := s.database.GetScan(txCtx, &current, `SELECT upload_no,file_name,media_type,object_ref,status_code AS status,policy_id,policy_snapshot,declared_size_bytes AS declared_size,actual_size_bytes AS actual_size,version,expires_at FROM plugin_idc_core_purchase_configuration_upload WHERE tenant_id=0 AND upload_no=? AND customer_no=? AND uploader_user_id=? LIMIT 1 FOR UPDATE`, token, audience.CustomerNo, audience.UserID); loadErr != nil || current.UploadNo == "" {
			return bizerr.NewCode(CodeNotAccessible)
		}
		if current.Status == StatusRevoked {
			revoked = current
			return nil
		}
		if current.Status == StatusConsumed {
			return bizerr.NewCode(CodeConflict)
		}
		if current.Status != StatusReserved && current.Status != StatusReady {
			return bizerr.NewCode(CodeNotAccessible)
		}
		result, updateErr := s.database.Exec(txCtx, `UPDATE plugin_idc_core_purchase_configuration_upload SET status_code='revoked',version=version+1,updated_at=CURRENT_TIMESTAMP WHERE tenant_id=0 AND upload_no=? AND customer_no=? AND uploader_user_id=? AND status_code=? AND version=?`, token, audience.CustomerNo, audience.UserID, current.Status, current.Version)
		if updateErr != nil {
			return updateErr
		}
		changed, rowsErr := result.RowsAffected()
		if rowsErr != nil || changed != 1 {
			return bizerr.NewCode(CodeConflict)
		}
		current.Status = StatusRevoked
		current.Version++
		revoked = current
		return nil
	})
	if err != nil {
		if bizerr.Is(err, CodeConflict) || bizerr.Is(err, CodeNotAccessible) {
			return nil, err
		}
		return nil, bizerr.WrapCode(err, CodeUnavailable)
	}
	return &Item{FileToken: revoked.UploadNo, FileName: revoked.FileName, MediaType: revoked.MediaType, SizeBytes: revoked.ActualSize, Status: revoked.Status, ExpiresAt: revoked.ExpiresAt}, nil
}

func (s *serviceImpl) VerifyBatch(ctx context.Context, input VerifyInput) ([]VerifiedFile, error) {
	result, err := s.VerifyFields(ctx, []VerifyInput{input})
	if err != nil {
		return nil, err
	}
	return result[0].Files, nil
}

func (s *serviceImpl) VerifyFields(ctx context.Context, inputs []VerifyInput) ([]VerifiedField, error) {
	audience, err := s.currentCustomer(ctx)
	if err != nil {
		return nil, err
	}
	plan, err := prepareVerification(inputs)
	if err != nil {
		return nil, err
	}
	rows := make([]verifiedRow, 0, len(plan.tokens))
	err = s.database.GetScan(ctx, &rows, `SELECT upload_no,file_name,media_type,sha256_hex,object_ref,product_version_no,configuration_revision,field_key,policy_id,policy_snapshot,scan_status,actual_size_bytes AS size_bytes,expires_at FROM plugin_idc_core_purchase_configuration_upload WHERE tenant_id=0 AND customer_no=? AND uploader_user_id=? AND upload_no IN(?) AND status_code='ready' AND expires_at>CURRENT_TIMESTAMP`, audience.CustomerNo, audience.UserID, plan.tokens)
	if err != nil {
		return nil, bizerr.WrapCode(err, CodeUnavailable)
	}
	return assembleVerified(inputs, plan, rows)
}

func (s *serviceImpl) ConsumeForOrder(ctx context.Context, orderNo string, inputs []VerifyInput) ([]VerifiedField, error) {
	var verified []VerifiedField
	err := s.database.Transaction(ctx, func(txCtx context.Context, tx gdb.TX) error {
		var consumeErr error
		verified, consumeErr = s.ConsumeForOrderTransaction(txCtx, tx, orderNo, inputs)
		return consumeErr
	})
	if err != nil {
		if bizerr.Is(err, CodeInvalid) || bizerr.Is(err, CodeConflict) || bizerr.Is(err, CodeNotAccessible) {
			return nil, err
		}
		return nil, bizerr.WrapCode(err, CodeUnavailable)
	}
	return verified, nil
}

func (s *serviceImpl) ConsumeForOrderTransaction(ctx context.Context, tx gdb.TX, orderNo string, inputs []VerifyInput) ([]VerifiedField, error) {
	audience, err := s.currentCustomer(ctx)
	if err != nil {
		return nil, err
	}
	if tx == nil || !validBoundOwnerNo(orderNo) {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	plan, err := prepareVerification(inputs)
	if err != nil {
		return nil, err
	}
	rows := make([]verifiedRow, 0, len(plan.tokens))
	err = tx.GetScan(&rows, `SELECT upload_no,file_name,media_type,sha256_hex,object_ref,product_version_no,configuration_revision,field_key,policy_id,policy_snapshot,scan_status,actual_size_bytes AS size_bytes,expires_at FROM plugin_idc_core_purchase_configuration_upload WHERE tenant_id=0 AND customer_no=? AND uploader_user_id=? AND upload_no IN(?) AND ((status_code='ready' AND expires_at>CURRENT_TIMESTAMP) OR (status_code='consumed' AND bound_owner_type='order' AND bound_owner_no=?)) ORDER BY upload_no FOR UPDATE`, audience.CustomerNo, audience.UserID, plan.tokens, orderNo)
	if err != nil {
		return nil, err
	}
	verified, err := assembleVerified(inputs, plan, rows)
	if err != nil {
		return nil, err
	}
	result, err := tx.Exec(`UPDATE plugin_idc_core_purchase_configuration_upload SET status_code='consumed',bound_owner_type='order',bound_owner_no=?,consumed_at=COALESCE(consumed_at,CURRENT_TIMESTAMP),version=CASE WHEN status_code='ready' THEN version+1 ELSE version END,updated_at=CASE WHEN status_code='ready' THEN CURRENT_TIMESTAMP ELSE updated_at END WHERE tenant_id=0 AND customer_no=? AND uploader_user_id=? AND upload_no IN(?) AND ((status_code='ready' AND expires_at>CURRENT_TIMESTAMP) OR (status_code='consumed' AND bound_owner_type='order' AND bound_owner_no=?))`, orderNo, audience.CustomerNo, audience.UserID, plan.tokens, orderNo)
	if err != nil {
		return nil, err
	}
	changed, rowsErr := result.RowsAffected()
	if rowsErr != nil || changed != int64(len(plan.tokens)) {
		return nil, bizerr.NewCode(CodeConflict)
	}
	return verified, nil
}

func (s *serviceImpl) VerifyOrderEvidence(ctx context.Context, owner OrderEvidenceOwner, inputs []VerifyInput) ([]VerifiedField, error) {
	result, err := s.VerifyOrderEvidenceBatch(ctx, []OrderEvidenceBatchInput{{Owner: owner, Fields: inputs}})
	return singleOrderEvidenceResult(result, err)
}

func singleOrderEvidenceResult(result [][]VerifiedField, err error) ([]VerifiedField, error) {
	if err != nil {
		return nil, err
	}
	if len(result) != 1 {
		return nil, bizerr.NewCode(CodeUnavailable)
	}
	return result[0], nil
}

func (s *serviceImpl) VerifyOrderEvidenceBatch(ctx context.Context, inputs []OrderEvidenceBatchInput) ([][]VerifiedField, error) {
	if len(inputs) < 1 || len(inputs) > 100 {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	tokens := make([]string, 0)
	plans := make([]*verificationPlan, len(inputs))
	for index, input := range inputs {
		if !validBoundOwnerNo(input.Owner.OrderNo) || input.Owner.CustomerNo == "" || len(input.Owner.CustomerNo) > 40 || input.Owner.UploaderUserID < 1 {
			return nil, bizerr.NewCode(CodeInvalid)
		}
		plan, err := prepareVerification(input.Fields)
		if err != nil {
			return nil, err
		}
		plans[index] = plan
		tokens = append(tokens, plan.tokens...)
		if len(tokens) > maximumConfigurationFileTokens {
			return nil, bizerr.NewCode(CodeInvalid)
		}
	}
	rows := make([]verifiedRow, 0, len(tokens))
	err := s.database.GetScan(ctx, &rows, `SELECT upload_no,file_name,media_type,sha256_hex,object_ref,product_version_no,configuration_revision,field_key,policy_id,policy_snapshot,scan_status,actual_size_bytes AS size_bytes,expires_at,customer_no,uploader_user_id,bound_owner_no FROM plugin_idc_core_purchase_configuration_upload WHERE tenant_id=0 AND upload_no IN(?) AND status_code='consumed' AND bound_owner_type='order'`, tokens)
	if err != nil {
		return nil, bizerr.WrapCode(err, CodeUnavailable)
	}
	result := make([][]VerifiedField, len(inputs))
	for index, input := range inputs {
		owned := make([]verifiedRow, 0, len(plans[index].tokens))
		for _, row := range rows {
			if row.CustomerNo == input.Owner.CustomerNo && row.UploaderUserID == input.Owner.UploaderUserID && row.BoundOwnerNo == input.Owner.OrderNo {
				owned = append(owned, row)
			}
		}
		result[index], err = assembleVerified(input.Fields, plans[index], owned)
		if err != nil {
			return nil, err
		}
	}
	return result, nil
}

type verificationPlan struct {
	tokens   []string
	expected map[string]*Target
}

type verifiedRow struct {
	UploadNo, FileName, MediaType, SHA256Hex, ObjectRef, ProductVersionNo, ConfigurationRevision, FieldKey, PolicyID, ScanStatus string
	CustomerNo, BoundOwnerNo                                                                                                     string
	UploaderUserID                                                                                                               int
	PolicySnapshot                                                                                                               []byte
	SizeBytes                                                                                                                    int64
	ExpiresAt                                                                                                                    time.Time
}

func prepareVerification(inputs []VerifyInput) (*verificationPlan, error) {
	if len(inputs) < 1 || len(inputs) > maximumConfigurationFieldsForFiles {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	totalTokens := 0
	unique := make(map[string]struct{})
	expected := make(map[string]*Target)
	for _, input := range inputs {
		if !validTarget(input.Target) || len(input.FileTokens) < 1 || len(input.FileTokens) > input.Target.Policy.MaximumFiles {
			return nil, bizerr.NewCode(CodeInvalid)
		}
		totalTokens += len(input.FileTokens)
		if totalTokens > maximumConfigurationFileTokens {
			return nil, bizerr.NewCode(CodeInvalid)
		}
		for _, token := range input.FileTokens {
			if !validUploadToken(token) {
				return nil, bizerr.NewCode(CodeInvalid)
			}
			if _, exists := unique[token]; exists {
				return nil, bizerr.NewCode(CodeInvalid)
			}
			unique[token] = struct{}{}
			expected[token] = input.Target
		}
	}
	allTokens := make([]string, 0, len(unique))
	for token := range unique {
		allTokens = append(allTokens, token)
	}
	sort.Strings(allTokens)
	return &verificationPlan{tokens: allTokens, expected: expected}, nil
}

func assembleVerified(inputs []VerifyInput, plan *verificationPlan, rows []verifiedRow) ([]VerifiedField, error) {
	if plan == nil || len(rows) != len(plan.tokens) {
		return nil, bizerr.NewCode(CodeNotAccessible)
	}
	byToken := make(map[string]VerifiedFile, len(rows))
	totalByTarget := make(map[*Target]int64, len(inputs))
	for _, row := range rows {
		target := plan.expected[row.UploadNo]
		storedPolicy := new(capmodel.ConfigurationUploadPolicyDeclaration)
		if target == nil || strictJSON(row.PolicySnapshot, storedPolicy) != nil || !samePolicy(storedPolicy, target.Policy) || row.ProductVersionNo != target.ProductVersionNo || row.ConfigurationRevision != target.ConfigurationRevision || row.FieldKey != target.FieldKey || row.PolicyID != target.Policy.ID ||
			(row.ScanStatus != "clean" && row.ScanStatus != "not_required") || row.SizeBytes < 1 || row.SizeBytes > target.Policy.MaximumFileBytes || !mimeAllowed(target.Policy, row.MediaType) || !extensionAllowed(target.Policy, row.FileName) {
			return nil, bizerr.NewCode(CodeConflict)
		}
		totalByTarget[target] += row.SizeBytes
		if totalByTarget[target] > target.Policy.MaximumTotalBytes {
			return nil, bizerr.NewCode(CodeInvalid)
		}
		byToken[row.UploadNo] = VerifiedFile{FileToken: row.UploadNo, FileName: row.FileName, MediaType: row.MediaType, SHA256Hex: row.SHA256Hex, ObjectRef: row.ObjectRef, SizeBytes: row.SizeBytes}
	}
	result := make([]VerifiedField, 0, len(inputs))
	for _, input := range inputs {
		field := VerifiedField{Target: input.Target, Files: make([]VerifiedFile, 0, len(input.FileTokens))}
		for _, token := range input.FileTokens {
			field.Files = append(field.Files, byToken[token])
		}
		result = append(result, field)
	}
	return result, nil
}

func samePolicy(left, right *capmodel.ConfigurationUploadPolicyDeclaration) bool {
	if left == nil || right == nil || left.ID != right.ID || left.MaximumFiles != right.MaximumFiles || left.MaximumFileBytes != right.MaximumFileBytes || left.MaximumTotalBytes != right.MaximumTotalBytes || left.SafetyScanRequired != right.SafetyScanRequired || len(left.AllowedMIMETypes) != len(right.AllowedMIMETypes) || len(left.AllowedExtensions) != len(right.AllowedExtensions) {
		return false
	}
	for index := range left.AllowedMIMETypes {
		if left.AllowedMIMETypes[index] != right.AllowedMIMETypes[index] {
			return false
		}
	}
	for index := range left.AllowedExtensions {
		if left.AllowedExtensions[index] != right.AllowedExtensions[index] {
			return false
		}
	}
	return true
}

type uploadRow struct {
	UploadNo, FileName, MediaType, ObjectRef, Status, PolicyID string
	PolicySnapshot                                             []byte
	DeclaredSize, ActualSize, Version                          int64
	ExpiresAt                                                  time.Time
}

func (s *serviceImpl) loadOwned(ctx context.Context, token string, audience *principal.Audience) (*uploadRow, error) {
	if !validUploadToken(token) {
		return nil, bizerr.NewCode(CodeNotAccessible)
	}
	query := `SELECT upload_no,file_name,media_type,object_ref,status_code AS status,policy_id,policy_snapshot,declared_size_bytes AS declared_size,actual_size_bytes AS actual_size,version,expires_at FROM plugin_idc_core_purchase_configuration_upload WHERE tenant_id=0 AND upload_no=? AND customer_no=? AND uploader_user_id=? AND expires_at>CURRENT_TIMESTAMP LIMIT 1`
	var row *uploadRow
	if err := s.database.GetScan(ctx, &row, query, token, audience.CustomerNo, audience.UserID); err != nil || row == nil {
		return nil, bizerr.NewCode(CodeNotAccessible)
	}
	return row, nil
}

func (s *serviceImpl) currentCustomer(ctx context.Context) (*principal.Audience, error) {
	audience, err := s.principal.Current(ctx)
	if err != nil || audience == nil || audience.UserID <= 0 || audience.CustomerNo == "" || audience.Code != principal.AudienceCustomer || audience.Status != principal.AudienceStatusActive {
		return nil, bizerr.NewCode(CodeNotAccessible)
	}
	return audience, nil
}

func (s *serviceImpl) claim(ctx context.Context, row *uploadRow, audience *principal.Audience) (bool, error) {
	result, err := s.database.Exec(ctx, `UPDATE plugin_idc_core_purchase_configuration_upload SET status_code='uploading',version=version+1,updated_at=CURRENT_TIMESTAMP WHERE tenant_id=0 AND upload_no=? AND customer_no=? AND uploader_user_id=? AND status_code='reserved' AND version=?`, row.UploadNo, audience.CustomerNo, audience.UserID, row.Version)
	if err != nil {
		return false, bizerr.WrapCode(err, CodeUnavailable)
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return false, bizerr.WrapCode(err, CodeUnavailable)
	}
	return changed == 1, nil
}

func (s *serviceImpl) restoreReservation(ctx context.Context, row *uploadRow) error {
	result, err := s.database.Exec(ctx, `UPDATE plugin_idc_core_purchase_configuration_upload SET status_code='reserved',version=version+1,updated_at=CURRENT_TIMESTAMP WHERE tenant_id=0 AND upload_no=? AND status_code='uploading' AND version=?`, row.UploadNo, row.Version)
	if err != nil {
		return bizerr.WrapCode(err, CodeUnavailable)
	}
	changed, err := result.RowsAffected()
	if err != nil || changed != 1 {
		return bizerr.NewCode(CodeConflict)
	}
	return nil
}

func (s *serviceImpl) reject(ctx context.Context, row *uploadRow, scanStatus string, cause error) (*Item, error) {
	result, updateErr := s.database.Exec(ctx, `UPDATE plugin_idc_core_purchase_configuration_upload SET status_code='rejected',scan_status=?,version=version+1,updated_at=CURRENT_TIMESTAMP WHERE tenant_id=0 AND upload_no=? AND status_code='uploading' AND version=?`, scanStatus, row.UploadNo, row.Version)
	if updateErr != nil {
		return nil, errors.Join(cause, bizerr.WrapCode(updateErr, CodeUnavailable))
	}
	changed, rowsErr := result.RowsAffected()
	if rowsErr != nil || changed != 1 {
		return nil, errors.Join(cause, bizerr.NewCode(CodeConflict))
	}
	return nil, cause
}

func newUploadToken(reader io.Reader) (string, error) {
	var value [16]byte
	if _, err := io.ReadFull(reader, value[:]); err != nil {
		return "", err
	}
	return "CFU_" + strings.ToUpper(hex.EncodeToString(value[:])), nil
}

func validUploadToken(value string) bool {
	if len(value) != 36 || !strings.HasPrefix(value, "CFU_") {
		return false
	}
	_, err := hex.DecodeString(value[4:])
	return err == nil
}

func validBoundOwnerNo(value string) bool {
	if len(value) < 1 || len(value) > 64 {
		return false
	}
	for _, character := range value {
		if character >= 'A' && character <= 'Z' || character >= 'a' && character <= 'z' || character >= '0' && character <= '9' || character == '_' || character == '-' {
			continue
		}
		return false
	}
	return true
}

func validTarget(target *Target) bool {
	return target != nil && target.ProductVersionNo != "" && target.ConfigurationRevision != "" && target.FieldKey != "" && validPolicy(target.Policy)
}

func validPolicy(policy *capmodel.ConfigurationUploadPolicyDeclaration) bool {
	if policy == nil || !uploadPolicyIDPattern.MatchString(policy.ID) || policy.MaximumFiles < 1 || policy.MaximumFiles > maximumUploadFiles || policy.MaximumFileBytes < 1 || policy.MaximumFileBytes > maximumUploadFileBytes || policy.MaximumTotalBytes < policy.MaximumFileBytes || policy.MaximumTotalBytes > maximumUploadTotalBytes || len(policy.AllowedMIMETypes) < 1 || len(policy.AllowedMIMETypes) > maximumUploadPolicyChoices || len(policy.AllowedExtensions) < 1 || len(policy.AllowedExtensions) > maximumUploadPolicyChoices {
		return false
	}
	seenMIME := make(map[string]struct{}, len(policy.AllowedMIMETypes))
	for _, value := range policy.AllowedMIMETypes {
		if value != strings.ToLower(strings.TrimSpace(value)) || !uploadMIMEPattern.MatchString(value) {
			return false
		}
		if _, exists := seenMIME[value]; exists {
			return false
		}
		seenMIME[value] = struct{}{}
	}
	seenExtensions := make(map[string]struct{}, len(policy.AllowedExtensions))
	for _, value := range policy.AllowedExtensions {
		if value != strings.ToLower(strings.TrimSpace(value)) || !uploadExtensionPattern.MatchString(value) {
			return false
		}
		if _, exists := seenExtensions[value]; exists {
			return false
		}
		seenExtensions[value] = struct{}{}
	}
	return true
}

func extensionAllowed(policy *capmodel.ConfigurationUploadPolicyDeclaration, name string) bool {
	extension := strings.ToLower(filepath.Ext(name))
	for _, allowed := range policy.AllowedExtensions {
		if extension == strings.ToLower(strings.TrimSpace(allowed)) {
			return true
		}
	}
	return false
}

func mimeAllowed(policy *capmodel.ConfigurationUploadPolicyDeclaration, value string) bool {
	for _, allowed := range policy.AllowedMIMETypes {
		if value == strings.ToLower(strings.TrimSpace(allowed)) {
			return true
		}
	}
	return false
}

func strictJSON(payload []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("multiple JSON values")
		}
		return err
	}
	return nil
}
