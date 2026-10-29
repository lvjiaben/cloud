package contenthub

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"lina-core/pkg/bizerr"
	"lina-core/pkg/plugin/capability/storagecap"
)

const downloadAuthorizationChunkBytes = 1024 * 1024

type downloadSource struct {
	ContentNo, FileNo, Visibility, ObjectRef, FileName, MediaType string
	SizeBytes, ValidityVersion                                    int64
}

func (s *serviceImpl) IssuePublicDownload(ctx context.Context, contentNo string) (*DownloadTicket, error) {
	return s.issueDownload(ctx, contentNo, 0, "", "")
}
func (s *serviceImpl) IssueAccountDownload(ctx context.Context, contentNo string) (*DownloadTicket, error) {
	userID, customerNo, err := s.customer(ctx)
	if err != nil {
		return nil, err
	}
	session, err := s.sessions.Current(ctx)
	if err != nil || session == nil || string(session.ID) == "" || session.UserID != strconv.Itoa(userID) {
		return nil, bizerr.NewCode(CodeNotAccessible)
	}
	return s.issueDownload(ctx, contentNo, userID, customerNo, string(session.ID))
}
func (s *serviceImpl) issueDownload(ctx context.Context, contentNo string, userID int, customerNo, sessionID string) (*DownloadTicket, error) {
	if !businessNumberPattern.MatchString(contentNo) {
		return nil, bizerr.NewCode(CodeNotFound)
	}
	source, err := s.loadDownloadSource(ctx, contentNo, customerNo, userID > 0)
	if err != nil {
		return nil, err
	}
	if source.Visibility == "public" {
		userID, sessionID = 0, ""
	}
	raw := make([]byte, 32)
	if _, err = io.ReadFull(s.random, raw); err != nil {
		return nil, contentWrap(err, "generate download token")
	}
	token := base64.RawURLEncoding.EncodeToString(raw)
	clear(raw)
	digest := sha256.Sum256([]byte(token))
	tokenDigest := hex.EncodeToString(digest[:])
	ticketNo, err := s.newNo("DTK_")
	if err != nil {
		return nil, contentWrap(err, "generate download ticket")
	}
	var result *DownloadTicket
	err = s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		now, err := contentDatabaseNow(ctx, s.database)
		if err != nil {
			return err
		}
		expires := now.Add(10 * time.Minute)
		var sessionDigest any
		var authorizationRevision any
		if userID > 0 {
			sessionHash := sha256.Sum256([]byte(sessionID))
			sessionDigest = hex.EncodeToString(sessionHash[:])
			revision, err := s.database.GetValue(ctx, `INSERT INTO plugin_idc_core_content_download_authorization (tenant_id,content_no,user_id,revision,updated_at) VALUES (0,?,?,1,?) ON CONFLICT (content_no,user_id) DO UPDATE SET updated_at=EXCLUDED.updated_at RETURNING revision`, contentNo, userID, now)
			if err != nil {
				return err
			}
			authorizationRevision = revision.Int64()
		}
		_, err = s.database.Exec(ctx, `INSERT INTO plugin_idc_core_content_download_ticket (tenant_id,ticket_no,token_digest,content_no,file_no,content_validity_version,visibility_code,user_id,session_reference_digest,authorization_revision,issued_at,expires_at) VALUES (0,?,?,?,?,?,?,?,?,?,?,?)`, ticketNo, tokenDigest, source.ContentNo, source.FileNo, source.ValidityVersion, source.Visibility, contentInt(userID), sessionDigest, authorizationRevision, now, expires)
		if err == nil {
			result = &DownloadTicket{Token: token, ExpiresAt: expires}
		}
		return err
	})
	if err != nil {
		clear([]byte(token))
		return nil, contentWrap(err, "issue download ticket")
	}
	return result, nil
}
func (s *serviceImpl) loadDownloadSource(ctx context.Context, contentNo, customerNo string, account bool) (*downloadSource, error) {
	where := ` WHERE i.tenant_id=0 AND i.content_no=? AND i.content_type='download' AND i.status_code='published' AND f.file_no=i.current_file_no AND f.role_code='current' AND f.status_code='active'`
	args := []any{contentNo}
	if !account {
		where += " AND i.visibility_code='public'"
	} else {
		where += ` AND (i.visibility_code IN ('public','all_logged_in') OR (i.visibility_code='customer_group' AND EXISTS (SELECT 1 FROM plugin_idc_core_content_group_scope gs JOIN plugin_idc_core_customer_group_assignment ga ON ga.tenant_id=0 AND ga.group_no=gs.group_no AND ga.customer_no=? AND ga.status_code='active' WHERE gs.tenant_id=0 AND gs.content_no=i.content_no)))`
		args = append(args, customerNo)
	}
	var rows []downloadSource
	if err := s.database.GetScan(ctx, &rows, `SELECT i.content_no,f.file_no,i.visibility_code AS visibility,f.object_ref,f.file_name,f.media_type,f.size_bytes,i.validity_version FROM plugin_idc_core_content_item i JOIN plugin_idc_core_content_file f ON f.tenant_id=0 AND f.content_no=i.content_no`+where+` LIMIT 1`, args...); err != nil {
		return nil, contentWrap(err, "load downloadable content")
	}
	if len(rows) != 1 {
		return nil, bizerr.NewCode(CodeNotFound)
	}
	return &rows[0], nil
}
func (s *serviceImpl) OpenPublicDownload(ctx context.Context, token string) (*DownloadOpen, error) {
	return s.openDownload(ctx, token, 0, "", "")
}
func (s *serviceImpl) OpenAccountDownload(ctx context.Context, token string) (*DownloadOpen, error) {
	userID, customerNo, err := s.customer(ctx)
	if err != nil {
		return nil, err
	}
	session, err := s.sessions.Current(ctx)
	if err != nil || session == nil || session.UserID != strconv.Itoa(userID) {
		return nil, bizerr.NewCode(CodeNotAccessible)
	}
	if publicFile, publicErr := s.openDownload(ctx, token, 0, "", ""); publicErr == nil {
		return publicFile, nil
	}
	return s.openDownload(ctx, token, userID, customerNo, string(session.ID))
}
func (s *serviceImpl) openDownload(ctx context.Context, token string, userID int, customerNo, sessionID string) (*DownloadOpen, error) {
	token = strings.TrimSpace(token)
	if len(token) < 32 || len(token) > 128 {
		return nil, bizerr.NewCode(CodeNotFound)
	}
	digest := sha256.Sum256([]byte(token))
	tokenDigest := hex.EncodeToString(digest[:])
	where := ` WHERE d.tenant_id=0 AND d.token_digest=? AND d.revoked_at IS NULL AND d.expires_at>CURRENT_TIMESTAMP AND i.status_code='published' AND i.validity_version=d.content_validity_version AND i.current_file_no=d.file_no AND f.role_code='current' AND f.status_code='active'`
	args := []any{tokenDigest}
	if userID == 0 {
		where += " AND d.visibility_code='public' AND d.user_id IS NULL AND i.visibility_code='public'"
	} else {
		sessionHash := sha256.Sum256([]byte(sessionID))
		where += ` AND d.visibility_code<>'public' AND d.user_id=? AND d.session_reference_digest=? AND EXISTS (SELECT 1 FROM plugin_idc_core_content_download_authorization a WHERE a.tenant_id=0 AND a.content_no=d.content_no AND a.user_id=d.user_id AND a.revision=d.authorization_revision) AND (i.visibility_code='all_logged_in' OR (i.visibility_code='customer_group' AND EXISTS (SELECT 1 FROM plugin_idc_core_content_group_scope gs JOIN plugin_idc_core_customer_group_assignment ga ON ga.tenant_id=0 AND ga.group_no=gs.group_no AND ga.customer_no=? AND ga.status_code='active' WHERE gs.tenant_id=0 AND gs.content_no=i.content_no)))`
		args = append(args, userID, hex.EncodeToString(sessionHash[:]), customerNo)
	}
	var rows []downloadSource
	if err := s.database.GetScan(ctx, &rows, `SELECT i.content_no,f.file_no,i.visibility_code AS visibility,f.object_ref,f.file_name,f.media_type,f.size_bytes,i.validity_version FROM plugin_idc_core_content_download_ticket d JOIN plugin_idc_core_content_item i ON i.tenant_id=0 AND i.content_no=d.content_no JOIN plugin_idc_core_content_file f ON f.tenant_id=0 AND f.file_no=d.file_no`+where+` LIMIT 1`, args...); err != nil {
		return nil, contentWrap(err, "authorize download ticket")
	}
	if len(rows) != 1 {
		return nil, bizerr.NewCode(CodeNotFound)
	}
	object, err := s.storage.Get(ctx, storagecap.GetInput{Path: rows[0].ObjectRef})
	if err != nil {
		return nil, contentWrap(err, "open download file")
	}
	if !object.Found || object.Body == nil {
		return nil, bizerr.NewCode(CodeNotFound)
	}
	body := &controlledDownloadBody{
		body:      object.Body,
		remaining: downloadAuthorizationChunkBytes,
		revalidate: func() error {
			return s.revalidateDownload(ctx, tokenDigest, userID, customerNo)
		},
	}
	return &DownloadOpen{FileName: rows[0].FileName, MediaType: rows[0].MediaType, SizeBytes: rows[0].SizeBytes, Body: body}, nil
}

func (s *serviceImpl) revalidateDownload(ctx context.Context, tokenDigest string, userID int, customerNo string) error {
	where := ` WHERE d.tenant_id=0 AND d.token_digest=? AND d.revoked_at IS NULL
        AND i.status_code='published' AND i.validity_version=d.content_validity_version
        AND i.current_file_no=d.file_no AND f.role_code='current' AND f.status_code='active'`
	args := []any{tokenDigest}
	if userID == 0 {
		where += " AND d.visibility_code='public' AND d.user_id IS NULL AND i.visibility_code='public'"
	} else {
		where += ` AND d.visibility_code<>'public' AND d.user_id=?
            AND EXISTS (SELECT 1 FROM plugin_idc_core_content_download_authorization a
                        WHERE a.tenant_id=0 AND a.content_no=d.content_no
                          AND a.user_id=d.user_id AND a.revision=d.authorization_revision)
            AND (i.visibility_code='all_logged_in'
                 OR (i.visibility_code='customer_group' AND EXISTS (
                        SELECT 1 FROM plugin_idc_core_content_group_scope gs
                        JOIN plugin_idc_core_customer_group_assignment ga
                          ON ga.tenant_id=0 AND ga.group_no=gs.group_no
                         AND ga.customer_no=? AND ga.status_code='active'
                        WHERE gs.tenant_id=0 AND gs.content_no=i.content_no))
)`
		args = append(args, userID, customerNo)
	}
	count, err := s.database.GetValue(ctx, `SELECT COUNT(1)
        FROM plugin_idc_core_content_download_ticket d
        JOIN plugin_idc_core_content_item i
          ON i.tenant_id=0 AND i.content_no=d.content_no
        JOIN plugin_idc_core_content_file f
	          ON f.tenant_id=0 AND f.file_no=d.file_no`+where, args...)
	if err != nil {
		return contentWrap(err, "revalidate open download")
	}
	if count.Int() != 1 {
		return bizerr.NewCode(CodeNotFound)
	}
	return nil
}

type controlledDownloadBody struct {
	body       io.ReadCloser
	remaining  int
	revalidate func() error
	terminal   error
	closed     bool
}

func (r *controlledDownloadBody) Read(buffer []byte) (int, error) {
	if len(buffer) == 0 {
		return 0, nil
	}
	if r.terminal != nil {
		return 0, r.terminal
	}
	if r.remaining == 0 {
		if err := r.revalidate(); err != nil {
			r.terminal = r.closeWithError(err)
			return 0, r.terminal
		}
		r.remaining = downloadAuthorizationChunkBytes
	}
	if len(buffer) > r.remaining {
		buffer = buffer[:r.remaining]
	}
	read, err := r.body.Read(buffer)
	r.remaining -= read
	return read, err
}

func (r *controlledDownloadBody) Close() error {
	return r.closeWithError(nil)
}

func (r *controlledDownloadBody) closeWithError(cause error) error {
	if r.closed {
		return cause
	}
	r.closed = true
	return errors.Join(cause, r.body.Close())
}

func contentInt(value int) any {
	if value <= 0 {
		return nil
	}
	return value
}
