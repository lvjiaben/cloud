package customerexport

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/plugin/capability/storagecap"
	"lina-plugin-idc-core/backend/internal/service/asynctask"
)

const (
	TaskType             = "customer.data_export"
	maximumArtifactBytes = 10 * 1024 * 1024
	artifactLifetime     = 24 * time.Hour
)

type Worker interface {
	Run(context.Context, int) (int, error)
	Cleanup(context.Context, int) (int, error)
}

type workerImpl struct {
	database gdb.DB
	tasks    asynctask.Service
	storage  storagecap.Service
}

func NewWorker(database gdb.DB, tasks asynctask.Service, storage storagecap.Service) (Worker, error) {
	if database == nil || tasks == nil || storage == nil {
		return nil, gerror.New("customer export worker requires all dependencies")
	}
	return &workerImpl{database: database, tasks: tasks, storage: storage}, nil
}

func (w *workerImpl) Run(ctx context.Context, limit int) (int, error) {
	claimed, err := w.tasks.Claim(ctx, asynctask.ClaimInput{WorkerID: "customer-data-export", Limit: limit, TaskTypes: []string{TaskType}})
	if err != nil {
		return 0, err
	}
	completed := 0
	for _, task := range claimed {
		if err = w.process(ctx, task); err != nil {
			_, failErr := w.tasks.Fail(ctx, asynctask.FailureInput{Lease: task.Lease, Retryable: true, ErrorCode: "export_generation_failed", ErrorSummary: "Customer export generation failed"})
			if failErr != nil {
				return completed, gerror.Wrap(failErr, "record customer export failure")
			}
			continue
		}
		completed++
	}
	return completed, nil
}

func (w *workerImpl) Cleanup(ctx context.Context, limit int) (int, error) {
	if limit < 1 || limit > 100 {
		return 0, gerror.New("customer export cleanup limit is invalid")
	}
	type expiredRow struct{ RequestNo, ObjectRef string }
	var rows []expiredRow
	if err := w.database.GetScan(ctx, &rows, `SELECT request_no,object_ref FROM plugin_idc_core_customer_data_export_artifact WHERE tenant_id=0 AND status_code='ready' AND expires_at<=CURRENT_TIMESTAMP ORDER BY expires_at,id LIMIT ?`, limit); err != nil {
		return 0, err
	}
	cleaned := 0
	for _, row := range rows {
		if err := w.storage.Delete(ctx, storagecap.DeleteInput{Path: row.ObjectRef}); err != nil {
			continue
		}
		result, err := w.database.Exec(ctx, `UPDATE plugin_idc_core_customer_data_export_artifact SET status_code='deleted',updated_at=CURRENT_TIMESTAMP WHERE tenant_id=0 AND request_no=? AND status_code='ready' AND expires_at<=CURRENT_TIMESTAMP`, row.RequestNo)
		if err != nil {
			return cleaned, err
		}
		affected, err := result.RowsAffected()
		if err != nil {
			return cleaned, err
		}
		cleaned += int(affected)
	}
	return cleaned, nil
}

type exportHeader struct {
	RequestNo   string
	CustomerNo  string
	TaskNo      string
	RequestedAt time.Time `orm:"requested_at"`
}
type exportDocument struct {
	SchemaVersion           int              `json:"schemaVersion"`
	GeneratedAt             time.Time        `json:"generatedAt"`
	Profile                 map[string]any   `json:"profile"`
	Contacts                []map[string]any `json:"contacts"`
	Addresses               []map[string]any `json:"addresses"`
	NotificationPreferences []map[string]any `json:"notificationPreferences"`
}

func (w *workerImpl) process(ctx context.Context, task asynctask.ClaimedTask) error {
	var header *exportHeader
	if err := w.database.GetScan(ctx, &header, `SELECT request_no,customer_no,task_no,requested_at FROM plugin_idc_core_customer_data_export_request WHERE tenant_id=0 AND task_no=?`, task.TaskNo); err != nil || header == nil {
		return gerror.Wrap(err, "load customer export request")
	}
	existingValue, err := w.database.GetValue(ctx, `SELECT COUNT(1) FROM plugin_idc_core_customer_data_export_artifact WHERE tenant_id=0 AND request_no=? AND task_no=? AND status_code='ready' AND expires_at>CURRENT_TIMESTAMP`, header.RequestNo, header.TaskNo)
	if err != nil {
		return err
	}
	if existingValue.Int() == 1 {
		_, err := w.tasks.Complete(ctx, asynctask.CompleteInput{Lease: task.Lease, ResultSnapshot: json.RawMessage(fmt.Sprintf(`{"request_no":%q,"status":"ready"}`, header.RequestNo))})
		return err
	}
	document, err := w.loadDocument(ctx, header.CustomerNo, header.RequestedAt)
	if err != nil {
		return err
	}
	archive, err := buildArchive(document)
	if err != nil {
		return err
	}
	if len(archive) == 0 || len(archive) > maximumArtifactBytes {
		return gerror.New("customer export artifact exceeds the bounded size")
	}
	digestBytes := sha256.Sum256(archive)
	digest := hex.EncodeToString(digestBytes[:])
	objectRef := fmt.Sprintf("private/customer-exports/%s/%s.zip", header.RequestNo, digest)
	if _, err = w.storage.Put(ctx, storagecap.PutInput{Path: objectRef, Body: bytes.NewReader(archive), Size: int64(len(archive)), ContentType: "application/zip", Overwrite: false}); err != nil {
		stat, statErr := w.storage.Stat(ctx, storagecap.StatInput{Path: objectRef})
		if statErr != nil || !stat.Found || stat.Object == nil || stat.Object.Size != int64(len(archive)) {
			return err
		}
	}
	nowValue, err := w.database.GetValue(ctx, `SELECT CURRENT_TIMESTAMP`)
	if err != nil {
		return err
	}
	now := nowValue.Time()
	if now.IsZero() {
		return gerror.New("customer export database time is unavailable")
	}
	expiresAt := now.Add(artifactLifetime)
	_, err = w.database.Exec(ctx, `INSERT INTO plugin_idc_core_customer_data_export_artifact (tenant_id,request_no,customer_no,task_no,object_ref,content_sha256,size_bytes,status_code,expires_at,created_at,updated_at) VALUES (0,?,?,?,?,?,?,'ready',?,?,?) ON CONFLICT (request_no) DO NOTHING`, header.RequestNo, header.CustomerNo, header.TaskNo, objectRef, digest, len(archive), expiresAt, now, now)
	if err != nil {
		return err
	}
	_, err = w.tasks.Complete(ctx, asynctask.CompleteInput{Lease: task.Lease, ResultSnapshot: json.RawMessage(fmt.Sprintf(`{"request_no":%q,"status":"ready","expires_at":%q}`, header.RequestNo, expiresAt.Format(time.RFC3339)))})
	return err
}

func (w *workerImpl) loadDocument(ctx context.Context, customerNo string, generatedAt time.Time) (*exportDocument, error) {
	if generatedAt.IsZero() {
		return nil, gerror.New("customer export request time is unavailable")
	}
	document := &exportDocument{SchemaVersion: 1, GeneratedAt: generatedAt.UTC()}
	if err := w.database.GetScan(ctx, &document.Profile, `SELECT customer_no,customer_type_code,status_code,display_name,country_code,preferred_locale,version,created_at,updated_at FROM plugin_idc_core_customer_subject WHERE tenant_id=0 AND customer_no=? AND status_code<>'anonymized'`, customerNo); err != nil || document.Profile == nil {
		return nil, gerror.Wrap(err, "load customer export profile")
	}
	queries := []struct {
		target *[]map[string]any
		query  string
	}{
		{&document.Contacts, `SELECT contact_no,display_name,email,phone_country_code,phone,preferred_locale,status_code,version,created_at,updated_at FROM plugin_idc_core_customer_contact WHERE tenant_id=0 AND customer_no=? ORDER BY id LIMIT 100`},
		{&document.Addresses, `SELECT address_no,address_type_code,label,recipient_name,recipient_phone,country_code,province,city,district,address_line1,address_line2,postal_code,is_default,status_code,version,created_at,updated_at FROM plugin_idc_core_customer_address WHERE tenant_id=0 AND customer_no=? ORDER BY id LIMIT 100`},
		{&document.NotificationPreferences, `SELECT contact_no,topic_code,channel_code,enabled,version,created_at,updated_at FROM plugin_idc_core_customer_contact_preference WHERE tenant_id=0 AND customer_no=? ORDER BY id LIMIT 1000`},
	}
	for _, query := range queries {
		if err := w.database.GetScan(ctx, query.target, query.query, customerNo); err != nil {
			return nil, err
		}
	}
	return document, nil
}

func buildArchive(document *exportDocument) ([]byte, error) {
	content, err := json.MarshalIndent(document, "", "  ")
	if err != nil {
		return nil, err
	}
	var buffer bytes.Buffer
	archive := zip.NewWriter(&buffer)
	header := &zip.FileHeader{Name: "customer-data.json", Method: zip.Deflate}
	header.SetModTime(time.Unix(0, 0).UTC())
	file, err := archive.CreateHeader(header)
	if err == nil {
		_, err = file.Write(content)
	}
	if closeErr := archive.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}
