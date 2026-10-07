package billing

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
	"lina-core/pkg/plugin/capability/storagecap"
	"lina-plugin-idc-core/backend/internal/service/principal"
)

const (
	maximumCreditEvidenceFiles            = 10
	maximumCreditEvidenceFileBytes  int64 = 20 * 1024 * 1024
	maximumCreditEvidenceTotalBytes int64 = 200 * 1024 * 1024
)

type CreditEvidenceReservation struct {
	UploadNo, FileName string
	SizeBytes          int64
	ExpiresAt          time.Time
}
type CreditEvidenceItem struct {
	UploadNo, FileName, MediaType string
	SizeBytes                     int64
}
type CreditEvidenceOpen struct {
	FileName, MediaType string
	SizeBytes           int64
	Body                io.ReadCloser
}

type CreditEvidenceService interface {
	ReserveAccountEvidence(context.Context, string, int64) (*CreditEvidenceReservation, error)
	UploadAccountEvidence(context.Context, string, string, io.Reader) (*CreditEvidenceItem, error)
	PrepareAccountEvidence(context.Context, []string) (string, string, error)
	BindAccountEvidence(context.Context, []string, string) error
	ListManagementEvidence(context.Context, string) ([]CreditEvidenceItem, error)
	OpenManagementEvidence(context.Context, string, string) (*CreditEvidenceOpen, error)
}

type creditEvidenceService struct {
	database   gdb.DB
	principal  principal.Reader
	authorizer ManagementCreditAuthorizer
	storage    storagecap.Service
}

func NewCreditEvidenceService(database gdb.DB, principalReader principal.Reader, authorizer ManagementCreditAuthorizer, storage storagecap.Service) (CreditEvidenceService, error) {
	for _, dependency := range []any{database, principalReader, authorizer, storage} {
		if isNilFinanceDirectoryDependency(dependency) {
			return nil, gerror.New("credit evidence requires database, identity, authorization, and private storage")
		}
	}
	return &creditEvidenceService{database: database, principal: principalReader, authorizer: authorizer, storage: storage}, nil
}

func (s *creditEvidenceService) ReserveAccountEvidence(ctx context.Context, fileName string, size int64) (*CreditEvidenceReservation, error) {
	audience, err := currentFinancialCustomer(ctx, s.principal)
	if err != nil {
		return nil, err
	}
	fileName = strings.TrimSpace(filepath.Base(fileName))
	if fileName == "" || len(fileName) > 255 || size < 1 || size > maximumCreditEvidenceFileBytes || !creditEvidenceExtensionAllowed(fileName) {
		return nil, bizerr.NewCode(CodeCreditInvalid)
	}
	var token [16]byte
	if _, err = io.ReadFull(rand.Reader, token[:]); err != nil {
		return nil, bizerr.WrapCode(err, CodeCreditUnavailable)
	}
	uploadNo := "CEU_" + strings.ToUpper(hex.EncodeToString(token[:]))
	now := time.Now().UTC()
	expires := now.Add(time.Hour)
	objectRef := "credit/evidence-uploads/" + uploadNo
	_, err = s.database.Exec(ctx, `INSERT INTO plugin_idc_core_credit_evidence_upload (tenant_id,upload_no,customer_no,uploader_user_id,file_name,declared_size_bytes,object_ref,status_code,expires_at,created_at,updated_at) VALUES (0,?,?,?,?,?,?,'reserved',?,?,?)`, uploadNo, audience.CustomerNo, audience.UserID, fileName, size, objectRef, expires, now, now)
	if err != nil {
		return nil, bizerr.WrapCode(err, CodeCreditUnavailable)
	}
	return &CreditEvidenceReservation{UploadNo: uploadNo, FileName: fileName, SizeBytes: size, ExpiresAt: expires}, nil
}

func (s *creditEvidenceService) UploadAccountEvidence(ctx context.Context, uploadNo, declaredType string, body io.Reader) (*CreditEvidenceItem, error) {
	audience, err := currentFinancialCustomer(ctx, s.principal)
	if err != nil {
		return nil, err
	}
	var rows []struct {
		UploadNo, FileName, ObjectRef, Status string
		SizeBytes, Version, UserID            int64
		ExpiresAt                             time.Time
	}
	err = s.database.GetScan(ctx, &rows, `SELECT upload_no,file_name,object_ref,status_code AS status,declared_size_bytes AS size_bytes,version,uploader_user_id AS user_id,expires_at FROM plugin_idc_core_credit_evidence_upload WHERE tenant_id=0 AND upload_no=? AND customer_no=? LIMIT 1`, strings.TrimSpace(uploadNo), audience.CustomerNo)
	if err != nil || len(rows) != 1 || rows[0].Status != "reserved" || rows[0].UserID != int64(audience.UserID) || !rows[0].ExpiresAt.After(time.Now().UTC()) || body == nil {
		return nil, bizerr.NewCode(CodeDirectoryNotAccessible)
	}
	content, err := io.ReadAll(io.LimitReader(body, maximumCreditEvidenceFileBytes+1))
	if err != nil || int64(len(content)) != rows[0].SizeBytes {
		return nil, bizerr.NewCode(CodeCreditInvalid)
	}
	mediaType, err := inspectCreditEvidence(rows[0].FileName, content)
	if err != nil {
		return nil, err
	}
	declaredType = strings.TrimSpace(strings.Split(declaredType, ";")[0])
	if declaredType != "" && declaredType != "application/octet-stream" && declaredType != mediaType {
		return nil, bizerr.NewCode(CodeCreditInvalid)
	}
	digest := sha256.Sum256(content)
	hash := hex.EncodeToString(digest[:])
	if _, err = s.storage.Put(ctx, storagecap.PutInput{Path: rows[0].ObjectRef, Body: bytes.NewReader(content), Size: int64(len(content)), ContentType: mediaType, Overwrite: false}); err != nil {
		return nil, bizerr.WrapCode(err, CodeCreditUnavailable)
	}
	result, err := s.database.Exec(ctx, `UPDATE plugin_idc_core_credit_evidence_upload SET media_type=?,sha256_hex=?,status_code='ready',completed_at=CURRENT_TIMESTAMP,version=version+1,updated_at=CURRENT_TIMESTAMP WHERE tenant_id=0 AND upload_no=? AND status_code='reserved' AND version=?`, mediaType, hash, uploadNo, rows[0].Version)
	if err != nil {
		return nil, errors.Join(bizerr.WrapCode(err, CodeCreditUnavailable), s.storage.Delete(ctx, storagecap.DeleteInput{Path: rows[0].ObjectRef}))
	}
	changed, _ := result.RowsAffected()
	if changed != 1 {
		return nil, errors.Join(bizerr.NewCode(CodeCreditConflict), s.storage.Delete(ctx, storagecap.DeleteInput{Path: rows[0].ObjectRef}))
	}
	return &CreditEvidenceItem{UploadNo: uploadNo, FileName: rows[0].FileName, MediaType: mediaType, SizeBytes: int64(len(content))}, nil
}

func (s *creditEvidenceService) PrepareAccountEvidence(ctx context.Context, uploadNos []string) (string, string, error) {
	audience, err := currentFinancialCustomer(ctx, s.principal)
	if err != nil {
		return "", "", err
	}
	if len(uploadNos) < 1 || len(uploadNos) > maximumCreditEvidenceFiles {
		return "", "", bizerr.NewCode(CodeCreditInvalid)
	}
	unique := make(map[string]struct{}, len(uploadNos))
	sorted := append([]string(nil), uploadNos...)
	sort.Strings(sorted)
	var total int64
	hashes := make([]string, 0, len(sorted))
	for _, uploadNo := range sorted {
		uploadNo = strings.TrimSpace(uploadNo)
		if _, exists := unique[uploadNo]; exists {
			return "", "", bizerr.NewCode(CodeCreditInvalid)
		}
		unique[uploadNo] = struct{}{}
		var row *struct {
			Hash          string `orm:"sha256_hex"`
			Size          int64  `orm:"declared_size_bytes"`
			StatusCode    string `orm:"status_code"`
			ApplicationNo string `orm:"application_no"`
		}
		if err = s.database.GetScan(ctx, &row, `SELECT sha256_hex,declared_size_bytes,status_code,application_no FROM plugin_idc_core_credit_evidence_upload WHERE tenant_id=0 AND upload_no=? AND customer_no=? AND uploader_user_id=? AND status_code IN ('ready','consumed') AND (status_code='consumed' OR expires_at>CURRENT_TIMESTAMP) FOR UPDATE`, uploadNo, audience.CustomerNo, audience.UserID); err != nil || row == nil {
			return "", "", bizerr.NewCode(CodeCreditConflict)
		}
		if row.StatusCode == "consumed" && strings.TrimSpace(row.ApplicationNo) == "" {
			return "", "", bizerr.NewCode(CodeCreditConflict)
		}
		total += row.Size
		if total > maximumCreditEvidenceTotalBytes {
			return "", "", bizerr.NewCode(CodeCreditInvalid)
		}
		hashes = append(hashes, uploadNo+":"+row.Hash)
	}
	sum := sha256.Sum256([]byte(strings.Join(hashes, "\n")))
	digest := hex.EncodeToString(sum[:])
	reference := "private:credit/evidence/" + digest[:32]
	return reference, digest, nil
}

func (s *creditEvidenceService) BindAccountEvidence(ctx context.Context, uploadNos []string, applicationNo string) error {
	audience, err := currentFinancialCustomer(ctx, s.principal)
	if err != nil {
		return err
	}
	if len(uploadNos) < 1 || len(uploadNos) > maximumCreditEvidenceFiles || !validBusinessReference(applicationNo, 40) {
		return bizerr.NewCode(CodeCreditInvalid)
	}
	result, err := s.database.Exec(ctx, `UPDATE plugin_idc_core_credit_evidence_upload SET application_no=?,status_code='consumed',consumed_at=CURRENT_TIMESTAMP,version=version+1,updated_at=CURRENT_TIMESTAMP WHERE tenant_id=0 AND customer_no=? AND uploader_user_id=? AND upload_no IN (?) AND status_code='ready'`, applicationNo, audience.CustomerNo, audience.UserID, uploadNos)
	if err != nil {
		return err
	}
	changed, _ := result.RowsAffected()
	if changed == int64(len(uploadNos)) {
		return nil
	}
	boundValue, err := s.database.GetValue(ctx, `SELECT COUNT(*) FROM plugin_idc_core_credit_evidence_upload WHERE tenant_id=0 AND customer_no=? AND uploader_user_id=? AND upload_no IN (?) AND status_code='consumed' AND application_no=?`, audience.CustomerNo, audience.UserID, uploadNos, applicationNo)
	if err != nil || boundValue.Int64() != int64(len(uploadNos)) {
		return bizerr.NewCode(CodeCreditConflict)
	}
	return nil
}

func (s *creditEvidenceService) ListManagementEvidence(ctx context.Context, applicationNo string) ([]CreditEvidenceItem, error) {
	if _, err := s.authorizer.AuthorizeApplicationDetail(ctx, applicationNo); err != nil {
		return nil, err
	}
	var rows []struct {
		UploadNo, FileName, MediaType string
		SizeBytes                     int64
	}
	if err := s.database.GetScan(ctx, &rows, `SELECT upload_no,file_name,media_type,declared_size_bytes AS size_bytes FROM plugin_idc_core_credit_evidence_upload WHERE tenant_id=0 AND application_no=? AND status_code='consumed' ORDER BY id LIMIT 10`, applicationNo); err != nil {
		return nil, bizerr.WrapCode(err, CodeCreditUnavailable)
	}
	items := make([]CreditEvidenceItem, 0, len(rows))
	for _, row := range rows {
		items = append(items, CreditEvidenceItem(row))
	}
	return items, nil
}

func (s *creditEvidenceService) OpenManagementEvidence(ctx context.Context, applicationNo, uploadNo string) (*CreditEvidenceOpen, error) {
	if _, err := s.authorizer.AuthorizeApplicationDetail(ctx, applicationNo); err != nil {
		return nil, err
	}
	var row *struct {
		FileName, MediaType, ObjectRef string
		SizeBytes                      int64
	}
	if err := s.database.GetScan(ctx, &row, `SELECT file_name,media_type,object_ref,declared_size_bytes AS size_bytes FROM plugin_idc_core_credit_evidence_upload WHERE tenant_id=0 AND application_no=? AND upload_no=? AND status_code='consumed'`, applicationNo, uploadNo); err != nil || row == nil {
		return nil, bizerr.NewCode(CodeDirectoryNotAccessible)
	}
	object, err := s.storage.Get(ctx, storagecap.GetInput{Path: row.ObjectRef})
	if err != nil || object == nil || !object.Found || object.Body == nil {
		return nil, bizerr.NewCode(CodeDirectoryNotAccessible)
	}
	return &CreditEvidenceOpen{FileName: row.FileName, MediaType: row.MediaType, SizeBytes: row.SizeBytes, Body: object.Body}, nil
}

func creditEvidenceExtensionAllowed(name string) bool {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".pdf", ".jpg", ".jpeg", ".png":
		return true
	}
	return false
}
func inspectCreditEvidence(name string, content []byte) (string, error) {
	if !creditEvidenceExtensionAllowed(name) {
		return "", bizerr.NewCode(CodeCreditInvalid)
	}
	media := strings.Split(http.DetectContentType(content), ";")[0]
	ext := strings.ToLower(filepath.Ext(name))
	valid := (ext == ".pdf" && media == "application/pdf" && bytes.HasPrefix(content, []byte("%PDF-"))) || ((ext == ".jpg" || ext == ".jpeg") && media == "image/jpeg") || (ext == ".png" && media == "image/png")
	if !valid {
		return "", bizerr.NewCode(CodeCreditInvalid)
	}
	return media, nil
}
