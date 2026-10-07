package customerexport

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
	"lina-core/pkg/plugin/capability/sessioncap"
	"lina-core/pkg/plugin/capability/storagecap"
	"lina-plugin-idc-core/backend/internal/service/customer"
)

type Status struct {
	RequestNo   string
	Status      string
	RequestedAt time.Time
	ExpiresAt   *time.Time
}
type Ticket struct {
	Token     string
	ExpiresAt time.Time
}
type Download struct {
	FileName, MediaType string
	Size                int64
	Body                io.ReadCloser
}

type Delivery interface {
	CurrentStatus(context.Context, string) (*Status, error)
	IssueTicket(context.Context, string) (*Ticket, error)
	Open(context.Context, string) (*Download, error)
}

type deliveryImpl struct {
	database gdb.DB
	access   *customer.AccessGuard
	sessions sessioncap.Service
	storage  storagecap.Service
}

func NewDelivery(database gdb.DB, access *customer.AccessGuard, sessions sessioncap.Service, storage storagecap.Service) (Delivery, error) {
	if database == nil || access == nil || sessions == nil || storage == nil {
		return nil, gerror.New("customer export delivery requires all dependencies")
	}
	return &deliveryImpl{database: database, access: access, sessions: sessions, storage: storage}, nil
}

type statusRow struct {
	RequestNo, TaskStatus, ArtifactStatus string
	RequestedAt                           time.Time
	ExpiresAt                             *time.Time
	DatabaseNow                           time.Time `orm:"database_now"`
}

func (d *deliveryImpl) CurrentStatus(ctx context.Context, requestNo string) (*Status, error) {
	audience, err := d.access.Resolve(ctx)
	if err != nil {
		return nil, err
	}
	requestNo = strings.TrimSpace(requestNo)
	var row *statusRow
	if err = d.database.GetScan(ctx, &row, `SELECT r.request_no,t.status_code AS task_status,COALESCE(a.status_code,'') AS artifact_status,r.requested_at,a.expires_at,CURRENT_TIMESTAMP AS database_now FROM plugin_idc_core_customer_data_export_request r JOIN plugin_idc_core_async_task t ON t.tenant_id=0 AND t.task_no=r.task_no LEFT JOIN plugin_idc_core_customer_data_export_artifact a ON a.tenant_id=0 AND a.request_no=r.request_no WHERE r.tenant_id=0 AND r.request_no=? AND r.customer_no=?`, requestNo, audience.CustomerNo); err != nil || row == nil {
		return nil, bizerr.NewCode(customer.CodeNotAccessible)
	}
	status := row.TaskStatus
	if row.ArtifactStatus == "ready" && row.ExpiresAt != nil && !row.DatabaseNow.IsZero() && row.ExpiresAt.After(row.DatabaseNow) {
		status = "ready"
	}
	return &Status{RequestNo: row.RequestNo, Status: status, RequestedAt: row.RequestedAt, ExpiresAt: row.ExpiresAt}, nil
}

func (d *deliveryImpl) IssueTicket(ctx context.Context, requestNo string) (*Ticket, error) {
	audience, err := d.access.Resolve(ctx)
	if err != nil {
		return nil, err
	}
	session, err := d.sessions.Current(ctx)
	if err != nil || session == nil || session.UserID != strconv.Itoa(audience.UserID) || string(session.ID) == "" {
		return nil, bizerr.NewCode(customer.CodeNotAccessible)
	}
	raw := make([]byte, 32)
	if _, err = io.ReadFull(rand.Reader, raw); err != nil {
		return nil, err
	}
	token := base64.RawURLEncoding.EncodeToString(raw)
	clear(raw)
	tokenHash := sha256.Sum256([]byte(token))
	sessionHash := sha256.Sum256([]byte(session.ID))
	ticketRaw := make([]byte, 16)
	if _, err = io.ReadFull(rand.Reader, ticketRaw); err != nil {
		return nil, err
	}
	ticketNo := "EDT_" + hex.EncodeToString(ticketRaw)
	clear(ticketRaw)
	nowValue, err := d.database.GetValue(ctx, `SELECT CURRENT_TIMESTAMP`)
	if err != nil {
		return nil, err
	}
	now := nowValue.Time()
	if now.IsZero() {
		return nil, gerror.New("customer export database time is unavailable")
	}
	expiresAt := now.Add(10 * time.Minute)
	result, err := d.database.Exec(ctx, `INSERT INTO plugin_idc_core_customer_data_export_ticket (tenant_id,ticket_no,token_digest,request_no,customer_no,user_id,session_digest,expires_at,created_at) SELECT 0,?,?,?,?,?,?,?,? FROM plugin_idc_core_customer_data_export_artifact a WHERE a.tenant_id=0 AND a.request_no=? AND a.customer_no=? AND a.status_code='ready' AND a.expires_at>?`, ticketNo, hex.EncodeToString(tokenHash[:]), requestNo, audience.CustomerNo, audience.UserID, hex.EncodeToString(sessionHash[:]), expiresAt, now, requestNo, audience.CustomerNo, now)
	if err != nil {
		return nil, err
	}
	affected, err := result.RowsAffected()
	if err != nil || affected != 1 {
		return nil, bizerr.NewCode(customer.CodeNotAccessible)
	}
	return &Ticket{Token: token, ExpiresAt: expiresAt}, nil
}

type artifactRow struct {
	RequestNo, ObjectRef, ContentSHA256 string
	SizeBytes                           int64
}

func (d *deliveryImpl) Open(ctx context.Context, token string) (*Download, error) {
	audience, err := d.access.Resolve(ctx)
	if err != nil {
		return nil, err
	}
	session, err := d.sessions.Current(ctx)
	if err != nil || session == nil || session.UserID != strconv.Itoa(audience.UserID) {
		return nil, bizerr.NewCode(customer.CodeNotAccessible)
	}
	token = strings.TrimSpace(token)
	if len(token) < 32 || len(token) > 128 {
		return nil, bizerr.NewCode(customer.CodeNotAccessible)
	}
	tokenHash := sha256.Sum256([]byte(token))
	sessionHash := sha256.Sum256([]byte(session.ID))
	var artifact *artifactRow
	err = d.database.Transaction(ctx, func(_ context.Context, tx gdb.TX) error {
		return tx.GetScan(&artifact, `UPDATE plugin_idc_core_customer_data_export_ticket t SET consumed_at=CURRENT_TIMESTAMP FROM plugin_idc_core_customer_data_export_artifact a WHERE t.tenant_id=0 AND t.token_digest=? AND t.customer_no=? AND t.user_id=? AND t.session_digest=? AND t.consumed_at IS NULL AND t.expires_at>CURRENT_TIMESTAMP AND a.tenant_id=0 AND a.request_no=t.request_no AND a.customer_no=t.customer_no AND a.status_code='ready' AND a.expires_at>CURRENT_TIMESTAMP RETURNING a.request_no,a.object_ref,a.content_sha256,a.size_bytes`, hex.EncodeToString(tokenHash[:]), audience.CustomerNo, audience.UserID, hex.EncodeToString(sessionHash[:]))
	})
	if err != nil || artifact == nil {
		return nil, bizerr.NewCode(customer.CodeNotAccessible)
	}
	object, err := d.storage.Get(ctx, storagecap.GetInput{Path: artifact.ObjectRef})
	if err != nil || object == nil || !object.Found || object.Body == nil || object.Object == nil || object.Object.Size != artifact.SizeBytes {
		if object != nil && object.Body != nil {
			_ = object.Body.Close()
		}
		return nil, bizerr.NewCode(customer.CodeUnavailable)
	}
	defer object.Body.Close()
	content, err := io.ReadAll(io.LimitReader(object.Body, maximumArtifactBytes+1))
	if err != nil || int64(len(content)) != artifact.SizeBytes || len(content) > maximumArtifactBytes {
		return nil, bizerr.NewCode(customer.CodeUnavailable)
	}
	digest := sha256.Sum256(content)
	if hex.EncodeToString(digest[:]) != artifact.ContentSHA256 {
		return nil, bizerr.NewCode(customer.CodeUnavailable)
	}
	return &Download{FileName: artifact.RequestNo + ".zip", MediaType: "application/zip", Size: artifact.SizeBytes, Body: io.NopCloser(bytes.NewReader(content))}, nil
}
