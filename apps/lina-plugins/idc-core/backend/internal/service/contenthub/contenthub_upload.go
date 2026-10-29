// This file handles resumable content uploads and candidate promotion; expiry uses database time.

package contenthub

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
	"lina-core/pkg/logger"
	"lina-core/pkg/plugin/capability/storagecap"
)

const (
	contentChunkSize     = int64(8 * 1024 * 1024)
	maximumDownloadBytes = int64(5 * 1024 * 1024 * 1024)
	downloadProbeBytes   = int64(64 * 1024)
)

type uploadRow struct {
	UploadNo, ContentNo, FileName, ObjectRef, TransferMode, ProviderUploadID, Status string
	UploaderUserID, DeclaredSize, PersistedSize, ChunkSize, Version                  int64
	ExpiresAt                                                                        time.Time
}
type uploadPart struct {
	PartNumber              int32
	OffsetBytes             int64
	SizeBytes               int
	ETag, ObjectRef, SHA256 string
}

func (s *serviceImpl) CreateUpload(ctx context.Context, input UploadCreateInput) (*UploadSession, error) {
	userID, err := s.staff(ctx)
	if err != nil {
		return nil, err
	}
	fileName, ok := trimBound(filepath.Base(input.FileName), 255, true)
	if !ok || !businessNumberPattern.MatchString(input.ContentNo) || input.SizeBytes < 1 || input.ContentVersion <= 0 {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	if err = s.validateDownloadUpload(fileName, input.SizeBytes); err != nil {
		return nil, err
	}
	uploadNo, err := s.newNo("CUP_")
	if err != nil {
		return nil, contentWrap(err, "generate upload number")
	}
	objectPath := "content/downloads/" + input.ContentNo + "/candidates/" + uploadNo
	multipart, err := s.storage.SupportsMultipart(ctx)
	if err != nil {
		return nil, contentWrap(err, "check multipart support")
	}
	mode := "chunk_objects"
	providerUploadID := ""
	if multipart {
		created, createErr := s.storage.CreateMultipart(ctx, storagecap.MultipartCreateInput{Path: objectPath, ContentType: "application/octet-stream", Overwrite: false})
		if createErr != nil {
			return nil, contentWrap(createErr, "create multipart upload")
		}
		mode = "provider_multipart"
		providerUploadID = created.UploadID
	}
	var result *UploadSession
	err = s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		count, countErr := s.database.GetValue(ctx, `SELECT COUNT(1) FROM plugin_idc_core_content_item i WHERE i.tenant_id=0 AND i.content_no=? AND i.content_type='download' AND i.status_code<>'deleted' AND i.version=? AND NOT EXISTS (SELECT 1 FROM plugin_idc_core_content_file f WHERE f.tenant_id=0 AND f.content_no=i.content_no AND f.role_code='candidate' AND f.status_code IN ('checking','ready'))`, input.ContentNo, input.ContentVersion)
		if countErr != nil {
			return countErr
		}
		if count.Int() != 1 {
			return bizerr.NewCode(CodeConflict)
		}
		now, err := contentDatabaseNow(ctx, s.database)
		if err != nil {
			return err
		}
		expires := now.Add(24 * time.Hour)
		_, err = s.database.Exec(ctx, `INSERT INTO plugin_idc_core_content_upload_session (tenant_id,upload_no,content_no,uploader_user_id,file_name,declared_size_bytes,persisted_size_bytes,temporary_object_ref,transfer_mode,provider_upload_id,chunk_size_bytes,status_code,expires_at,created_at,updated_at) VALUES (0,?,?,?,?,?,0,?,?,?,?, 'uploading',?,?,?)`, uploadNo, input.ContentNo, userID, fileName, input.SizeBytes, objectPath, mode, contentNullable(providerUploadID), contentChunkSize, expires, now, now)
		if err == nil {
			result = &UploadSession{UploadNo: uploadNo, FileName: fileName, SizeBytes: input.SizeBytes, ConfirmedBytes: 0, ChunkSizeBytes: contentChunkSize, ExpiresAt: expires, Version: 1}
		}
		return err
	})
	if err != nil {
		if mode == "provider_multipart" {
			cleanupErr := s.storage.AbortMultipart(ctx, storagecap.MultipartAbortInput{Path: objectPath, UploadID: providerUploadID})
			err = errors.Join(err, cleanupErr)
		}
		return nil, contentWrap(err, "create content upload")
	}
	return result, nil
}

func (s *serviceImpl) GetUpload(ctx context.Context, uploadNo string) (*UploadSession, error) {
	userID, err := s.staff(ctx)
	if err != nil {
		return nil, err
	}
	row, err := s.loadUpload(ctx, uploadNo)
	if err != nil {
		return nil, err
	}
	if row.UploaderUserID != int64(userID) {
		return nil, bizerr.NewCode(CodeNotAccessible)
	}
	return uploadSession(row), nil
}
func uploadSession(row *uploadRow) *UploadSession {
	if row == nil {
		return nil
	}
	return &UploadSession{UploadNo: row.UploadNo, FileName: row.FileName, SizeBytes: row.DeclaredSize, ConfirmedBytes: row.PersistedSize, ChunkSizeBytes: row.ChunkSize, ExpiresAt: row.ExpiresAt, Version: row.Version}
}
func (s *serviceImpl) loadUpload(ctx context.Context, uploadNo string) (*uploadRow, error) {
	if !businessNumberPattern.MatchString(uploadNo) {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	var rows []uploadRow
	if err := s.database.GetScan(ctx, &rows, `SELECT upload_no,content_no,uploader_user_id,file_name,declared_size_bytes AS declared_size,persisted_size_bytes AS persisted_size,temporary_object_ref AS object_ref,transfer_mode,COALESCE(provider_upload_id,'') AS provider_upload_id,chunk_size_bytes AS chunk_size,status_code AS status,expires_at,version FROM plugin_idc_core_content_upload_session WHERE tenant_id=0 AND upload_no=? LIMIT 1`, uploadNo); err != nil {
		return nil, contentWrap(err, "load content upload")
	}
	if len(rows) != 1 {
		return nil, bizerr.NewCode(CodeNotAccessible)
	}
	return &rows[0], nil
}

func (s *serviceImpl) UploadChunk(ctx context.Context, uploadNo string, offset int64, body io.Reader) (*ChunkResult, error) {
	userID, err := s.staff(ctx)
	if err != nil {
		return nil, err
	}
	if body == nil || offset < 0 {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	row, err := s.loadUpload(ctx, uploadNo)
	if err != nil {
		return nil, err
	}
	now, err := contentDatabaseNow(ctx, s.database)
	if err != nil {
		return nil, contentWrap(err, "read upload time")
	}
	if row.UploaderUserID != int64(userID) || row.Status != "uploading" || !row.ExpiresAt.After(now) {
		return nil, bizerr.NewCode(CodeNotAccessible)
	}
	if offset != row.PersistedSize || offset%row.ChunkSize != 0 {
		return nil, bizerr.NewCode(CodeConflict)
	}
	remaining := row.DeclaredSize - offset
	if remaining <= 0 {
		return nil, bizerr.NewCode(CodeConflict)
	}
	expected := row.ChunkSize
	if remaining < expected {
		expected = remaining
	}
	content, err := io.ReadAll(io.LimitReader(body, expected+1))
	if err != nil {
		return nil, contentWrap(err, "read upload chunk")
	}
	if int64(len(content)) != expected {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	partNumber := int32(offset/row.ChunkSize + 1)
	digest := sha256.Sum256(content)
	hash := hex.EncodeToString(digest[:])
	etag := ""
	objectRef := ""
	if row.TransferMode == "provider_multipart" {
		part, uploadErr := s.storage.UploadPart(ctx, storagecap.MultipartPartInput{Path: row.ObjectRef, UploadID: row.ProviderUploadID, PartNumber: partNumber, Body: bytes.NewReader(content), Size: int64(len(content))})
		if uploadErr != nil {
			return nil, contentWrap(uploadErr, "upload multipart chunk")
		}
		etag = part.ETag
	} else {
		// Keep parts beside the final object: local storage cannot replace a directory with a file.
		objectRef = row.ObjectRef + ".parts/" + strconv.FormatInt(int64(partNumber), 10)
		if _, uploadErr := s.storage.Put(ctx, storagecap.PutInput{Path: objectRef, Body: bytes.NewReader(content), Size: int64(len(content)), ContentType: "application/octet-stream", Overwrite: false}); uploadErr != nil {
			return nil, contentWrap(uploadErr, "store upload chunk")
		}
	}
	result, err := s.database.Exec(ctx, `WITH part AS (INSERT INTO plugin_idc_core_content_upload_part (tenant_id,upload_no,part_number,offset_bytes,size_bytes,etag,object_ref,sha256_hex) VALUES (0,?,?,?,?,?,?,?) ON CONFLICT (upload_no,part_number) DO NOTHING RETURNING 1) UPDATE plugin_idc_core_content_upload_session SET persisted_size_bytes=persisted_size_bytes+?,last_data_at=?,expires_at=?,version=version+1,updated_at=? WHERE tenant_id=0 AND upload_no=? AND status_code='uploading' AND version=? AND persisted_size_bytes=? AND EXISTS (SELECT 1 FROM part)`, uploadNo, partNumber, offset, len(content), contentNullable(etag), contentNullable(objectRef), hash, len(content), now, now.Add(24*time.Hour), now, uploadNo, row.Version, offset)
	if err != nil {
		if objectRef != "" {
			err = errors.Join(err, s.storage.Delete(ctx, storagecap.DeleteInput{Path: objectRef}))
		}
		return nil, contentWrap(err, "confirm upload chunk")
	}
	changed, rowsErr := result.RowsAffected()
	if rowsErr != nil {
		if objectRef != "" {
			rowsErr = errors.Join(rowsErr, s.storage.Delete(ctx, storagecap.DeleteInput{Path: objectRef}))
		}
		return nil, contentWrap(rowsErr, "confirm upload chunk")
	}
	if changed != 1 {
		if objectRef != "" {
			if cleanupErr := s.storage.Delete(ctx, storagecap.DeleteInput{Path: objectRef}); cleanupErr != nil {
				logger.Errorf(ctx, "clean up rejected content chunk: %v", cleanupErr)
			}
		}
		return nil, bizerr.NewCode(CodeConflict)
	}
	confirmed := offset + int64(len(content))
	return &ChunkResult{UploadNo: uploadNo, ConfirmedBytes: confirmed, NextOffset: confirmed, Version: row.Version + 1, Complete: confirmed == row.DeclaredSize}, nil
}

func (s *serviceImpl) CompleteUpload(ctx context.Context, input CompleteInput) (*CandidateFile, error) {
	userID, err := s.staff(ctx)
	if err != nil {
		return nil, err
	}
	row, err := s.loadUpload(ctx, input.UploadNo)
	if err != nil {
		return nil, err
	}
	if row.UploaderUserID != int64(userID) || row.Status != "uploading" || row.Version != input.Version || row.DeclaredSize != input.ExpectedSizeBytes || row.PersistedSize != row.DeclaredSize {
		return nil, bizerr.NewCode(CodeConflict)
	}
	parts, err := s.loadParts(ctx, row.UploadNo)
	if err != nil {
		return nil, err
	}
	if !validParts(parts, row) {
		return nil, bizerr.NewCode(CodeConflict)
	}
	if row.TransferMode == "provider_multipart" {
		completed := make([]storagecap.MultipartCompletedPart, 0, len(parts))
		for _, part := range parts {
			completed = append(completed, storagecap.MultipartCompletedPart{PartNumber: part.PartNumber, ETag: part.ETag})
		}
		if _, err = s.storage.CompleteMultipart(ctx, storagecap.MultipartCompleteInput{Path: row.ObjectRef, UploadID: row.ProviderUploadID, Parts: completed}); err != nil {
			return nil, contentWrap(err, "complete multipart upload")
		}
	} else if err = s.assembleChunkObjects(ctx, row, parts); err != nil {
		return nil, err
	}
	inspection, err := s.inspectDownload(ctx, row)
	if err != nil {
		return nil, err
	}
	fileNo, err := s.newNo("CFI_")
	if err != nil {
		return nil, contentWrap(err, "generate content file")
	}
	attemptNo := ""
	if inspection.ScanRequired && s.scannerReady() {
		attemptNo, err = s.newNo("CSA_")
		if err != nil {
			return nil, contentWrap(err, "generate content scan attempt")
		}
	}
	now, err := contentDatabaseNow(ctx, s.database)
	if err != nil {
		return nil, contentWrap(err, "read content upload time")
	}
	generation, err := s.database.GetValue(ctx, `SELECT COALESCE(MAX(candidate_generation),0)+1 FROM plugin_idc_core_content_file WHERE tenant_id=0 AND content_no=?`, row.ContentNo)
	if err != nil {
		return nil, contentWrap(err, "allocate candidate generation")
	}
	status, scanStatus := "ready", "passed"
	var gateAt, expires any = now, now.Add(7 * 24 * time.Hour)
	if inspection.ScanRequired {
		status, scanStatus = "checking", "not_configured"
		gateAt, expires = nil, now.Add(24*time.Hour)
		if attemptNo != "" {
			scanStatus, expires = "scanning", nil
		}
	}
	var cleanupAt any
	if row.TransferMode == "provider_multipart" {
		cleanupAt = now
	}
	err = s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		result, updateErr := s.database.Exec(ctx, `UPDATE plugin_idc_core_content_upload_session SET status_code='completed',completed_at=?,cleanup_completed_at=?,version=version+1,updated_at=? WHERE tenant_id=0 AND upload_no=? AND status_code='uploading' AND version=?`, now, cleanupAt, now, row.UploadNo, row.Version)
		if updateErr != nil {
			return updateErr
		}
		changed, rowsErr := result.RowsAffected()
		if rowsErr != nil {
			return rowsErr
		}
		if changed != 1 {
			return bizerr.NewCode(CodeConflict)
		}
		if _, insertErr := s.database.Exec(ctx, `INSERT INTO plugin_idc_core_content_file (tenant_id,file_no,content_no,upload_no,candidate_generation,role_code,status_code,file_name,media_type,content_kind,size_bytes,sha256_hex,object_ref,scan_required,scan_status,gate_completed_at,retention_expires_at,version,created_by,created_at,updated_at) VALUES (0,?,?,?,?,'candidate',?,?,?,?,?,?,?,?,?,?,?,1,?,?,?)`, fileNo, row.ContentNo, row.UploadNo, generation.Int64(), status, row.FileName, inspection.MediaType, inspection.Kind, row.DeclaredSize, inspection.SHA256, row.ObjectRef, inspection.ScanRequired, scanStatus, gateAt, expires, userID, now, now); insertErr != nil {
			return insertErr
		}
		if row.TransferMode == "provider_multipart" {
			if _, deleteErr := s.database.Exec(ctx, `DELETE FROM plugin_idc_core_content_upload_part WHERE tenant_id=0 AND upload_no=?`, row.UploadNo); deleteErr != nil {
				return deleteErr
			}
		}
		if attemptNo == "" {
			return nil
		}
		_, insertErr := s.database.Exec(ctx, `INSERT INTO plugin_idc_core_content_scan_attempt (tenant_id,attempt_no,file_no,candidate_generation,status_code,queued_at,created_by,version) VALUES (0,?,?,?,'pending',?,?,1)`, attemptNo, fileNo, generation.Int64(), now, userID)
		return insertErr
	})
	if err != nil {
		return nil, contentWrap(err, "create download candidate")
	}
	if cleanupErr := s.deletePartObjects(ctx, parts); cleanupErr != nil {
		logger.Errorf(ctx, "clean up completed content upload parts: %v", cleanupErr)
	} else if row.TransferMode == "chunk_objects" {
		if cleanupErr := markUploadCleanupComplete(ctx, s.database, row.UploadNo, row.Version+1); cleanupErr != nil {
			logger.Errorf(ctx, "confirm completed content upload cleanup: %v", cleanupErr)
		}
	}
	return &CandidateFile{FileNo: fileNo, FileName: row.FileName, MediaType: inspection.MediaType, ContentKind: inspection.Kind, GateStatus: status, ScanStatus: scanStatus, SizeBytes: row.DeclaredSize, Version: 1, GateExpiresAt: timePointer(expires), CanCancel: true, CanRescan: false, CanEnable: status == "ready"}, nil
}

func (s *serviceImpl) loadParts(ctx context.Context, uploadNo string) ([]uploadPart, error) {
	var parts []uploadPart
	if err := s.database.GetScan(ctx, &parts, `SELECT part_number,offset_bytes,size_bytes,COALESCE(etag,'') AS etag,COALESCE(object_ref,'') AS object_ref,sha256_hex AS sha256 FROM plugin_idc_core_content_upload_part WHERE tenant_id=0 AND upload_no=? ORDER BY part_number`, uploadNo); err != nil {
		return nil, contentWrap(err, "load upload parts")
	}
	return parts, nil
}
func validParts(parts []uploadPart, row *uploadRow) bool {
	if len(parts) == 0 {
		return false
	}
	var offset int64
	for index, part := range parts {
		if part.PartNumber != int32(index+1) || part.OffsetBytes != offset || part.SizeBytes < 1 {
			return false
		}
		offset += int64(part.SizeBytes)
	}
	return offset == row.DeclaredSize
}
func (s *serviceImpl) assembleChunkObjects(ctx context.Context, row *uploadRow, parts []uploadPart) (resultErr error) {
	reader, writer := io.Pipe()
	// Unblock the producer if the storage provider rejects the write before reading.
	defer func() { resultErr = errors.Join(resultErr, reader.Close()) }()
	go func() {
		var copyErr error
		defer func() {
			if closeErr := writer.CloseWithError(copyErr); closeErr != nil {
				logger.Errorf(ctx, "close content assembly stream: %v", closeErr)
			}
		}()
		for _, part := range parts {
			object, getErr := s.storage.Get(ctx, storagecap.GetInput{Path: part.ObjectRef})
			if getErr != nil {
				copyErr = getErr
				return
			}
			if !object.Found || object.Body == nil {
				copyErr = bizerr.NewCode(CodeConflict)
				return
			}
			_, copyErr = io.Copy(writer, object.Body)
			closeErr := object.Body.Close()
			if copyErr != nil {
				return
			}
			if closeErr != nil {
				copyErr = closeErr
				return
			}
		}
	}()
	_, err := s.storage.Put(ctx, storagecap.PutInput{Path: row.ObjectRef, Body: reader, Size: row.DeclaredSize, ContentType: "application/octet-stream", Overwrite: false})
	if err != nil {
		return contentWrap(err, "assemble upload chunks")
	}
	return nil
}
func (s *serviceImpl) deletePartObjects(ctx context.Context, parts []uploadPart) error {
	paths := make([]string, 0, len(parts))
	for _, part := range parts {
		if part.ObjectRef != "" {
			paths = append(paths, part.ObjectRef)
		}
	}
	for len(paths) > 0 {
		count := len(paths)
		if count > 100 {
			count = 100
		}
		if err := s.storage.DeleteMany(ctx, storagecap.DeleteManyInput{Paths: paths[:count]}); err != nil {
			return err
		}
		paths = paths[count:]
	}
	return nil
}

type downloadInspection struct {
	MediaType, Kind, SHA256 string
	ScanRequired            bool
}

func (s *serviceImpl) inspectDownload(ctx context.Context, row *uploadRow) (inspection downloadInspection, resultErr error) {
	object, err := s.storage.Get(ctx, storagecap.GetInput{Path: row.ObjectRef})
	if err != nil {
		return downloadInspection{}, contentWrap(err, "read completed download")
	}
	if !object.Found || object.Body == nil {
		return downloadInspection{}, bizerr.NewCode(CodeConflict)
	}
	defer func() {
		resultErr = errors.Join(resultErr, object.Body.Close())
	}()
	probeSize := row.DeclaredSize
	if probeSize > downloadProbeBytes {
		probeSize = downloadProbeBytes
	}
	probe := make([]byte, probeSize)
	hash := sha256.New()
	read, readErr := io.ReadFull(io.TeeReader(object.Body, hash), probe)
	if readErr != nil && !(readErr == io.ErrUnexpectedEOF && int64(read) == row.DeclaredSize) {
		return downloadInspection{}, bizerr.NewCode(CodeUploadRejected)
	}
	written, copyErr := io.Copy(hash, io.LimitReader(object.Body, row.DeclaredSize-int64(read)+1))
	if copyErr != nil || int64(read)+written != row.DeclaredSize {
		return downloadInspection{}, bizerr.NewCode(CodeUploadRejected)
	}
	var extra [1]byte
	if count, extraErr := object.Body.Read(extra[:]); count != 0 || (extraErr != nil && extraErr != io.EOF) {
		return downloadInspection{}, bizerr.NewCode(CodeUploadRejected)
	}
	inspection, err = inspectDownloadProbe(probe, row.FileName, row.DeclaredSize)
	if err != nil {
		return downloadInspection{}, err
	}
	if inspection.Kind == "7z" {
		if err = preflightSevenZip(ctx, s.storage, row.ObjectRef, row.DeclaredSize); err != nil {
			return downloadInspection{}, err
		}
	}
	inspection.SHA256 = hex.EncodeToString(hash.Sum(nil))
	return inspection, nil
}
func inspectDownloadProbe(header []byte, fileName string, size int64) (downloadInspection, error) {
	ext := strings.ToLower(filepath.Ext(fileName))
	media := strings.Split(http.DetectContentType(header), ";")[0]
	if strings.HasPrefix(media, "image/") && media != "image/svg+xml" && map[string]bool{".png": true, ".jpg": true, ".jpeg": true, ".gif": true, ".webp": true, ".bmp": true}[ext] {
		return downloadInspection{MediaType: media, Kind: "image"}, nil
	}
	if bytes.HasPrefix(header, []byte("%PDF-")) && ext == ".pdf" {
		return downloadInspection{MediaType: "application/pdf", Kind: "pdf"}, nil
	}
	if bytes.HasPrefix(header, []byte("PK\x03\x04")) {
		if len(header) < 8 || binary.LittleEndian.Uint16(header[6:8])&1 != 0 {
			return downloadInspection{}, bizerr.NewCode(CodeUploadRejected)
		}
		if map[string]bool{".docx": true, ".xlsx": true, ".pptx": true}[ext] {
			return downloadInspection{MediaType: "application/vnd.openxmlformats-officedocument", Kind: "office"}, nil
		}
		if ext == ".zip" {
			return downloadInspection{MediaType: "application/zip", Kind: "zip", ScanRequired: true}, nil
		}
		return downloadInspection{}, bizerr.NewCode(CodeUploadRejected)
	}
	if bytes.HasPrefix(header, []byte{0xd0, 0xcf, 0x11, 0xe0, 0xa1, 0xb1, 0x1a, 0xe1}) {
		if ext == ".msi" {
			return downloadInspection{MediaType: "application/x-msi", Kind: "msi", ScanRequired: true}, nil
		}
		if map[string]bool{".doc": true, ".xls": true, ".ppt": true}[ext] {
			return downloadInspection{MediaType: "application/x-ole-storage", Kind: "office"}, nil
		}
	}
	if bytes.HasPrefix(header, []byte{'M', 'Z'}) && ext == ".exe" {
		return downloadInspection{MediaType: "application/vnd.microsoft.portable-executable", Kind: "exe", ScanRequired: true}, nil
	}
	if bytes.HasPrefix(header, []byte{'7', 'z', 0xbc, 0xaf, 0x27, 0x1c}) && ext == ".7z" {
		return downloadInspection{MediaType: "application/x-7z-compressed", Kind: "7z", ScanRequired: true}, nil
	}
	if ext == ".iso" && size >= 0x8006 {
		if len(header) >= 0x8006 && bytes.Equal(header[0x8001:0x8006], []byte("CD001")) {
			return downloadInspection{MediaType: "application/x-iso9660-image", Kind: "iso", ScanRequired: true}, nil
		}
	}
	return downloadInspection{}, bizerr.NewCode(CodeUploadRejected)
}

func classifyDownloadExtension(fileName string) (dangerous bool, allowed bool) {
	return classifyDownloadExtensionName(strings.ToLower(filepath.Ext(fileName)))
}

func (s *serviceImpl) validateDownloadUpload(fileName string, size int64) error {
	dangerous, allowed := classifyDownloadExtension(fileName)
	if !allowed || size < 1 || size > maximumDownloadBytes {
		return bizerr.NewCode(CodeUploadRejected)
	}
	if !dangerous {
		return nil
	}
	if !s.scannerReady() {
		return bizerr.NewCode(CodeScannerUnavailable)
	}
	if size > s.scanner.DangerousLimitBytes() {
		return bizerr.NewCode(CodeUploadRejected)
	}
	return nil
}

func (s *serviceImpl) CancelUpload(ctx context.Context, uploadNo string, version int64, confirmed bool) error {
	if _, err := s.staff(ctx); err != nil {
		return err
	}
	if !confirmed || version <= 0 {
		return bizerr.NewCode(CodeInvalid)
	}
	row, err := s.loadUpload(ctx, uploadNo)
	if err != nil {
		return err
	}
	if row.Status != "uploading" || row.Version != version {
		return bizerr.NewCode(CodeConflict)
	}
	var parts []uploadPart
	if row.TransferMode != "provider_multipart" {
		parts, err = s.loadParts(ctx, uploadNo)
		if err != nil {
			return err
		}
	}
	result, err := s.database.Exec(ctx, `UPDATE plugin_idc_core_content_upload_session SET status_code='cancelled',version=version+1,updated_at=CURRENT_TIMESTAMP WHERE tenant_id=0 AND upload_no=? AND status_code='uploading' AND version=?`, uploadNo, version)
	if err != nil {
		return contentWrap(err, "cancel upload")
	}
	changed, rowsErr := result.RowsAffected()
	if rowsErr != nil {
		return contentWrap(rowsErr, "cancel upload")
	}
	if changed != 1 {
		return bizerr.NewCode(CodeConflict)
	}
	if row.TransferMode == "provider_multipart" {
		if cleanupErr := s.storage.AbortMultipart(ctx, storagecap.MultipartAbortInput{Path: row.ObjectRef, UploadID: row.ProviderUploadID}); cleanupErr != nil {
			logger.Errorf(ctx, "abort cancelled content multipart upload: %v", cleanupErr)
		} else if cleanupErr = s.storage.Delete(ctx, storagecap.DeleteInput{Path: row.ObjectRef}); cleanupErr != nil {
			logger.Errorf(ctx, "delete cancelled content multipart object: %v", cleanupErr)
		} else if cleanupErr = markUploadCleanupComplete(ctx, s.database, row.UploadNo, row.Version+1); cleanupErr != nil {
			logger.Errorf(ctx, "confirm cancelled content multipart cleanup: %v", cleanupErr)
		}
	} else {
		paths := appendPartObjectRefs(parts, row.ObjectRef)
		if cleanupErr := deleteUploadObjectPaths(ctx, s.storage, paths); cleanupErr != nil {
			logger.Errorf(ctx, "clean up cancelled content upload parts: %v", cleanupErr)
		} else if cleanupErr = markUploadCleanupComplete(ctx, s.database, row.UploadNo, row.Version+1); cleanupErr != nil {
			logger.Errorf(ctx, "confirm cancelled content upload cleanup: %v", cleanupErr)
		}
	}
	return nil
}

func cleanupUploadObjects(ctx context.Context, storage scanStorage, claim *uploadCleanupClaim) error {
	if claim.TransferMode == "provider_multipart" {
		if claim.ProviderUploadID == "" {
			return gerror.New("content multipart cleanup is missing its provider upload identifier")
		}
		if err := storage.AbortMultipart(ctx, storagecap.MultipartAbortInput{Path: claim.ObjectRef, UploadID: claim.ProviderUploadID}); err != nil {
			return gerror.Wrap(err, "abort expired content multipart upload")
		}
		if err := storage.Delete(ctx, storagecap.DeleteInput{Path: claim.ObjectRef}); err != nil {
			return gerror.Wrap(err, "delete expired content multipart object")
		}
		return nil
	}
	paths := claim.PartRefs
	if claim.Status != "completed" {
		paths = append(append([]string(nil), paths...), claim.ObjectRef)
	}
	return deleteUploadObjectPaths(ctx, storage, paths)
}

func appendPartObjectRefs(parts []uploadPart, finalObjectRef string) []string {
	paths := make([]string, 0, len(parts)+1)
	for _, part := range parts {
		if part.ObjectRef != "" {
			paths = append(paths, part.ObjectRef)
		}
	}
	if finalObjectRef != "" {
		paths = append(paths, finalObjectRef)
	}
	return paths
}

func deleteUploadObjectPaths(ctx context.Context, storage interface {
	DeleteMany(context.Context, storagecap.DeleteManyInput) error
}, paths []string) error {
	for len(paths) > 0 {
		count := len(paths)
		if count > storagecap.MaxBatchPathCount {
			count = storagecap.MaxBatchPathCount
		}
		if err := storage.DeleteMany(ctx, storagecap.DeleteManyInput{Paths: paths[:count]}); err != nil {
			return err
		}
		paths = paths[count:]
	}
	return nil
}

func markUploadCleanupComplete(ctx context.Context, database gdb.DB, uploadNo string, version int64) error {
	return database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		if _, err := database.Exec(ctx, `DELETE FROM plugin_idc_core_content_upload_part WHERE tenant_id=0 AND upload_no=?`, uploadNo); err != nil {
			return err
		}
		result, err := database.Exec(ctx, `UPDATE plugin_idc_core_content_upload_session SET temporary_object_ref=CASE WHEN status_code IN ('cancelled','expired') THEN '' ELSE temporary_object_ref END,cleanup_lease_expires_at=NULL,cleanup_completed_at=CURRENT_TIMESTAMP,version=version+1,updated_at=CURRENT_TIMESTAMP WHERE tenant_id=0 AND upload_no=? AND version=? AND cleanup_completed_at IS NULL AND status_code IN ('completed','cancelled','expired')`, uploadNo, version)
		if err != nil {
			return err
		}
		changed, err := result.RowsAffected()
		if err != nil || changed != 1 {
			return gerror.New("content upload cleanup completion lost its PostgreSQL fence")
		}
		return nil
	})
}

func (s *serviceImpl) CancelCandidate(ctx context.Context, input CandidateActionInput) (*Item, error) {
	if _, err := s.staff(ctx); err != nil {
		return nil, err
	}
	if !input.Confirmed {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	var objectRef string
	err := s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		var rows []struct{ ObjectRef string }
		if err := s.database.GetScan(ctx, &rows, `SELECT object_ref FROM plugin_idc_core_content_file WHERE tenant_id=0 AND file_no=? AND content_no=? AND role_code='candidate' AND status_code IN ('checking','ready') AND version=? FOR UPDATE`, input.FileNo, input.ContentNo, input.FileVersion); err != nil {
			return err
		}
		if len(rows) != 1 {
			return bizerr.NewCode(CodeConflict)
		}
		objectRef = rows[0].ObjectRef
		result, err := s.database.Exec(ctx, `UPDATE plugin_idc_core_content_file SET status_code='cancelled',object_ref=NULL,cancelled_at=CURRENT_TIMESTAMP,version=version+1,updated_at=CURRENT_TIMESTAMP WHERE tenant_id=0 AND file_no=? AND version=?`, input.FileNo, input.FileVersion)
		if err != nil {
			return err
		}
		changed, rowsErr := result.RowsAffected()
		if rowsErr != nil {
			return rowsErr
		}
		if changed != 1 {
			return bizerr.NewCode(CodeConflict)
		}
		return nil
	})
	if err != nil {
		return nil, contentWrap(err, "cancel candidate")
	}
	if cleanupErr := s.storage.Delete(ctx, storagecap.DeleteInput{Path: objectRef}); cleanupErr != nil {
		logger.Errorf(ctx, "delete cancelled content candidate: %v", cleanupErr)
	}
	return s.DetailAdmin(ctx, input.ContentNo)
}
func (s *serviceImpl) RetryCandidateScan(ctx context.Context, input CandidateActionInput) (*Item, error) {
	userID, err := s.staff(ctx)
	if err != nil {
		return nil, err
	}
	if !s.scannerReady() {
		return nil, bizerr.NewCode(CodeScannerUnavailable)
	}
	if !input.Confirmed || input.ContentVersion <= 0 || input.FileVersion <= 0 || !businessNumberPattern.MatchString(input.ContentNo) || !businessNumberPattern.MatchString(input.FileNo) {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	attemptNo, err := s.newNo("CSA_")
	if err != nil {
		return nil, contentWrap(err, "generate content scan attempt")
	}
	err = s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		now, timeErr := contentDatabaseNow(ctx, s.database)
		if timeErr != nil {
			return timeErr
		}
		var contentRows []struct{ ContentNo string }
		if scanErr := s.database.GetScan(ctx, &contentRows, `SELECT content_no FROM plugin_idc_core_content_item WHERE tenant_id=0 AND content_no=? AND content_type='download' AND status_code<>'deleted' AND version=? FOR UPDATE`, input.ContentNo, input.ContentVersion); scanErr != nil {
			return scanErr
		}
		if len(contentRows) != 1 {
			return bizerr.NewCode(CodeConflict)
		}
		result, updateErr := s.database.Exec(ctx, `UPDATE plugin_idc_core_content_file SET scan_status='scanning',version=version+1,updated_at=? WHERE tenant_id=0 AND file_no=? AND content_no=? AND role_code='candidate' AND status_code='checking' AND scan_status IN ('not_configured','failed') AND version=? AND (scan_status<>'not_configured' OR retention_expires_at>?)`, now, input.FileNo, input.ContentNo, input.FileVersion, now)
		if updateErr != nil {
			return updateErr
		}
		changed, rowsErr := result.RowsAffected()
		if rowsErr != nil || changed != 1 {
			return bizerr.NewCode(CodeConflict)
		}
		_, insertErr := s.database.Exec(ctx, `INSERT INTO plugin_idc_core_content_scan_attempt (tenant_id,attempt_no,file_no,candidate_generation,status_code,queued_at,created_by,version) SELECT 0,?,file_no,candidate_generation,'pending',?,?,1 FROM plugin_idc_core_content_file WHERE tenant_id=0 AND file_no=? AND content_no=? AND version=?`, attemptNo, now, userID, input.FileNo, input.ContentNo, input.FileVersion+1)
		return insertErr
	})
	if err != nil {
		return nil, contentWrap(err, "queue candidate rescan")
	}
	return s.DetailAdmin(ctx, input.ContentNo)
}
func (s *serviceImpl) EnableCandidate(ctx context.Context, input CandidateActionInput) (*Item, error) {
	userID, err := s.staff(ctx)
	if err != nil {
		return nil, err
	}
	if !input.Confirmed || input.ContentVersion <= 0 || input.FileVersion <= 0 {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	actionNo, err := s.newNo("CAH_")
	if err != nil {
		return nil, contentWrap(err, "generate content history")
	}
	err = s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		now, err := contentDatabaseNow(ctx, s.database)
		if err != nil {
			return err
		}
		var rows []struct {
			CurrentFileNo string
			Status        string
		}
		if err = s.database.GetScan(ctx, &rows, `SELECT COALESCE(current_file_no,'') AS current_file_no,status_code AS status FROM plugin_idc_core_content_item WHERE tenant_id=0 AND content_no=? AND content_type='download' AND status_code<>'deleted' AND version=? FOR UPDATE`, input.ContentNo, input.ContentVersion); err != nil {
			return err
		}
		if len(rows) != 1 {
			return bizerr.NewCode(CodeConflict)
		}
		count, countErr := s.database.GetValue(ctx, `SELECT COUNT(1) FROM plugin_idc_core_content_file WHERE tenant_id=0 AND content_no=? AND file_no=? AND role_code='candidate' AND status_code='ready' AND scan_status='passed' AND retention_expires_at>CURRENT_TIMESTAMP AND version=?`, input.ContentNo, input.FileNo, input.FileVersion)
		if countErr != nil {
			return countErr
		}
		if count.Int() != 1 {
			return bizerr.NewCode(CodeConflict)
		}
		if rows[0].CurrentFileNo != "" {
			if _, err = s.database.Exec(ctx, `UPDATE plugin_idc_core_content_file SET role_code='retired',status_code='replaced',version=version+1,updated_at=? WHERE tenant_id=0 AND file_no=? AND role_code='current' AND status_code='active'`, now, rows[0].CurrentFileNo); err != nil {
				return err
			}
		}
		if _, err = s.database.Exec(ctx, `UPDATE plugin_idc_core_content_file SET role_code='current',status_code='active',enabled_at=?,version=version+1,updated_at=? WHERE tenant_id=0 AND file_no=? AND version=?`, now, now, input.FileNo, input.FileVersion); err != nil {
			return err
		}
		result, err := s.database.Exec(ctx, `UPDATE plugin_idc_core_content_item SET current_file_no=?,validity_version=validity_version+1,version=version+1,updated_by=?,updated_at=? WHERE tenant_id=0 AND content_no=? AND version=?`, input.FileNo, userID, now, input.ContentNo, input.ContentVersion)
		if err != nil {
			return err
		}
		changed, _ := result.RowsAffected()
		if changed != 1 {
			return bizerr.NewCode(CodeConflict)
		}
		_, err = s.database.Exec(ctx, `INSERT INTO plugin_idc_core_content_action_history (tenant_id,action_no,content_no,action_code,actor_user_id,content_version,occurred_at) VALUES (0,?,?, 'file_enabled',?,?,?)`, actionNo, input.ContentNo, userID, input.ContentVersion+1, now)
		return err
	})
	if err != nil {
		return nil, contentWrap(err, "enable download candidate")
	}
	return s.DetailAdmin(ctx, input.ContentNo)
}

func timePointer(value any) *time.Time {
	if typed, ok := value.(time.Time); ok {
		return &typed
	}
	return nil
}
