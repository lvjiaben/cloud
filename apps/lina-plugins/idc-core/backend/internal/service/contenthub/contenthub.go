// Package contenthub owns news, announcements, knowledge articles, and downloads.
package contenthub

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"io"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gcode"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
	"lina-core/pkg/plugin/capability/authcap/authz"
	"lina-core/pkg/plugin/capability/bizctxcap"
	"lina-core/pkg/plugin/capability/i18ncap"
	"lina-core/pkg/plugin/capability/sessioncap"
	"lina-core/pkg/plugin/capability/storagecap"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/internal/service/principal"
)

const PermissionManage = "idc:admin:content:manage"

var (
	businessNumberPattern  = regexp.MustCompile(`^[A-Za-z0-9_-]{1,40}$`)
	CodeInvalid            = bizerr.MustDefine("IDC_CONTENT_INVALID", "The content request is invalid", gcode.CodeInvalidParameter)
	CodeNotAccessible      = bizerr.MustDefine("IDC_CONTENT_NOT_ACCESSIBLE", "The content is not accessible", gcode.CodeNotAuthorized)
	CodeNotFound           = bizerr.MustDefine("IDC_CONTENT_NOT_FOUND", "The content does not exist", gcode.CodeNotFound)
	CodeConflict           = bizerr.MustDefine("IDC_CONTENT_CONFLICT", "The content changed; reload it and try again", gcode.CodeInvalidOperation)
	CodeUploadRejected     = bizerr.MustDefine("IDC_CONTENT_UPLOAD_REJECTED", "The download file is not allowed", gcode.CodeInvalidParameter)
	CodeScannerUnavailable = bizerr.MustDefine("IDC_CONTENT_SCANNER_UNAVAILABLE", "Virus scanning is not configured", gcode.CodeInvalidOperation)
	CodeUnavailable        = bizerr.MustDefine("IDC_CONTENT_UNAVAILABLE", "The content service is temporarily unavailable", gcode.CodeInternalError)
)

type Item struct {
	ID                                                                                                                                                             int64
	ContentNo, Type, CategoryNo, CategoryNameZhCN, CategoryNameEnUS, Visibility, Title, Summary, Body, SEOTitle, SEODescription, PublicPath, Status, CurrentFileNo string
	ValidityVersion, Version                                                                                                                                       int64
	PublishedAt, OfflineAt, CreatedAt, UpdatedAt                                                                                                                   *time.Time
	TargetCustomerNos, TargetServiceNos, TargetCustomerGroupNos                                                                                                    []string
	ActiveUpload                                                                                                                                                   *ActiveUpload
	CandidateFile                                                                                                                                                  *CandidateFile
	CurrentFile                                                                                                                                                    *CurrentFile
	AvailableActions                                                                                                                                               []string
	TargetAudiences                                                                                                                                                []TargetAudience
}
type ActiveUpload struct {
	UploadNo, FileName                        string
	SizeBytes, ConfirmedBytes, ChunkSizeBytes int64
	ExpiresAt                                 time.Time
	Version                                   int64
	CanResume, CanCancel                      bool
}
type CandidateFile struct {
	FileNo, FileName, MediaType, ContentKind, GateStatus, ScanStatus string
	SizeBytes, Version                                               int64
	GateExpiresAt                                                    *time.Time
	CanCancel, CanRescan, CanEnable                                  bool
}
type CurrentFile struct {
	FileNo, FileName, MediaType, ContentKind, SHA256 string
	SizeBytes, Version                               int64
}
type TargetAudience struct {
	Kind, Number, Label string
}
type Page struct {
	Items      []*Item
	NextCursor string
}
type SearchInput struct {
	Type, CategoryNo, Status, Visibility, Query, Cursor string
	Limit                                               int
}
type Category struct {
	ID                             int64
	CategoryNo, Type, Name, Status string
	SortOrder                      int
	Version                        int64
	CreatedAt, UpdatedAt           time.Time
	Synthetic                      bool
}
type CategoryPage struct {
	Items      []Category
	NextCursor string
}
type CategorySearchInput struct {
	Type, Status, Locale, Cursor string
	Limit                        int
}
type CategoryCreateInput struct {
	Type, Name string
	SortOrder  int
}
type CategoryDeleteInput struct {
	CategoryNo string
	Version    int64
	Confirmed  bool
}
type WriteInput struct {
	ContentNo, Type, CategoryNo, Visibility, Title, Summary, Body, SEOTitle, SEODescription, Status string
	Version                                                                                         int64
	TargetCustomerNos, TargetServiceNos, TargetCustomerGroupNos                                     []string
}
type Candidate struct{ Number, Label string }
type CandidatePage struct {
	Items      []Candidate
	NextCursor string
}
type AudienceSearchInput struct {
	Keyword, Cursor string
	Limit           int
}
type UploadCreateInput struct {
	ContentNo, FileName       string
	SizeBytes, ContentVersion int64
}
type UploadSession struct {
	UploadNo, FileName                        string
	SizeBytes, ConfirmedBytes, ChunkSizeBytes int64
	ExpiresAt                                 time.Time
	Version                                   int64
}
type ChunkResult struct {
	UploadNo                            string
	ConfirmedBytes, NextOffset, Version int64
	Complete                            bool
}
type CompleteInput struct {
	UploadNo                   string
	ExpectedSizeBytes, Version int64
}
type CandidateActionInput struct {
	ContentNo, FileNo           string
	ContentVersion, FileVersion int64
	Confirmed                   bool
}
type DownloadTicket struct {
	Token     string
	ExpiresAt time.Time
}
type DownloadOpen struct {
	FileName, MediaType string
	SizeBytes           int64
	Body                io.ReadCloser
}
type UploadPolicy struct {
	OrdinaryMaxSizeBytes           int64
	InspectionRequiredMaxSizeBytes int64
	InspectionRequiredAvailable    bool
	OrdinaryExtensions             []string
	InspectionRequiredExtensions   []string
}

type Service interface {
	SearchAdmin(context.Context, SearchInput) (*Page, error)
	// GetUploadPolicy requires the current direct staff actor to hold content
	// management permission and returns a fail-closed projection without scanner secrets.
	GetUploadPolicy(context.Context) (*UploadPolicy, error)
	SearchCategories(context.Context, CategorySearchInput) (*CategoryPage, error)
	CreateCategory(context.Context, CategoryCreateInput) (*Category, error)
	DeleteCategory(context.Context, CategoryDeleteInput) error
	DetailAdmin(context.Context, string) (*Item, error)
	Create(context.Context, WriteInput) (*Item, error)
	Update(context.Context, WriteInput) (*Item, error)
	Publish(context.Context, string, int64) (*Item, error)
	Unpublish(context.Context, string, int64) (*Item, error)
	Republish(context.Context, string, int64) (*Item, error)
	Delete(context.Context, string, int64) (*Item, error)
	SearchPublic(context.Context, SearchInput) (*Page, error)
	DetailPublic(context.Context, string) (*Item, error)
	SearchAccount(context.Context, SearchInput) (*Page, error)
	DetailAccount(context.Context, string) (*Item, error)
	SearchAudienceCustomers(context.Context, AudienceSearchInput) (*CandidatePage, error)
	SearchAudienceServices(context.Context, AudienceSearchInput) (*CandidatePage, error)
	SearchAudienceCustomerGroups(context.Context, AudienceSearchInput) (*CandidatePage, error)
	CreateUpload(context.Context, UploadCreateInput) (*UploadSession, error)
	UploadChunk(context.Context, string, int64, io.Reader) (*ChunkResult, error)
	GetUpload(context.Context, string) (*UploadSession, error)
	CompleteUpload(context.Context, CompleteInput) (*CandidateFile, error)
	CancelUpload(context.Context, string, int64, bool) error
	CancelCandidate(context.Context, CandidateActionInput) (*Item, error)
	RetryCandidateScan(context.Context, CandidateActionInput) (*Item, error)
	EnableCandidate(context.Context, CandidateActionInput) (*Item, error)
	IssuePublicDownload(context.Context, string) (*DownloadTicket, error)
	OpenPublicDownload(context.Context, string) (*DownloadOpen, error)
	IssueAccountDownload(context.Context, string) (*DownloadTicket, error)
	OpenAccountDownload(context.Context, string) (*DownloadOpen, error)
}

type serviceImpl struct {
	database      gdb.DB
	bizCtx        bizctxcap.Service
	principal     principal.Reader
	authorization authz.Service
	storage       storagecap.Service
	sessions      sessioncap.Service
	i18n          i18ncap.Service
	scanner       Scanner
	now           func() time.Time
	random        io.Reader
}

var _ Service = (*serviceImpl)(nil)

func New(database gdb.DB, bizCtx bizctxcap.Service, principalReader principal.Reader, authorization authz.Service, storage storagecap.Service, sessions sessioncap.Service, i18n i18ncap.Service) (Service, error) {
	scanner, err := NewClamdScanner(ClamdConfig{Timeout: 60 * time.Minute})
	if err != nil {
		return nil, err
	}
	return NewWithScanner(database, bizCtx, principalReader, authorization, storage, sessions, i18n, scanner)
}

func NewWithScanner(database gdb.DB, bizCtx bizctxcap.Service, principalReader principal.Reader, authorization authz.Service, storage storagecap.Service, sessions sessioncap.Service, i18n i18ncap.Service, scanner Scanner) (Service, error) {
	if database == nil || bizCtx == nil || principalReader == nil || authorization == nil || storage == nil || sessions == nil || i18n == nil {
		return nil, gerror.New("content hub requires database, identity, authorization, storage, session, and i18n services")
	}
	if scanner == nil {
		return nil, gerror.New("content hub requires an explicit content scanner boundary")
	}
	return &serviceImpl{database: database, bizCtx: bizCtx, principal: principalReader, authorization: authorization, storage: storage, sessions: sessions, i18n: i18n, scanner: scanner, now: time.Now, random: rand.Reader}, nil
}

func (s *serviceImpl) scannerReady() bool {
	return s != nil && s.scanner != nil && s.scanner.Ready()
}

func (s *serviceImpl) staff(ctx context.Context) (int, error) {
	current := s.bizCtx.Current(ctx)
	if current.UserID <= 0 || current.TenantID != idccap.DefaultTenantID || current.ActingUserID != 0 || current.IsImpersonation || current.ActingAsTenant || current.DataScopeUnsupported || (current.DataScope != 1 && current.DataScope != 2) {
		return 0, bizerr.NewCode(CodeNotAccessible)
	}
	audience, err := s.principal.Current(ctx)
	if err != nil || audience == nil || audience.UserID != current.UserID || audience.Code != principal.AudienceStaff || audience.Status != principal.AudienceStatusActive {
		return 0, bizerr.NewCode(CodeNotAccessible)
	}
	keys := []authz.PermissionKey{PermissionManage, idccap.HostPermissionWildcard}
	grants, err := s.authorization.BatchHasPermissions(ctx, keys)
	if err != nil {
		return 0, bizerr.WrapCode(err, CodeNotAccessible)
	}
	admin, err := s.authorization.IsPlatformAdmin(ctx, authz.UserID(strconv.Itoa(current.UserID)))
	if err != nil {
		return 0, bizerr.WrapCode(err, CodeNotAccessible)
	}
	if !grants[PermissionManage] && !grants[idccap.HostPermissionWildcard] && !admin {
		return 0, bizerr.NewCode(CodeNotAccessible)
	}
	return current.UserID, nil
}
func (s *serviceImpl) customer(ctx context.Context) (int, string, error) {
	current := s.bizCtx.Current(ctx)
	directCustomer := current.ActingUserID == 0 && !current.IsImpersonation && !current.ActingAsTenant
	delegatedCustomer := current.IsImpersonation && current.ActingUserID > 0 && current.ActingUserID != current.UserID && !current.ActingAsTenant
	if current.UserID <= 0 || current.TenantID != 0 || (!directCustomer && !delegatedCustomer) {
		return 0, "", bizerr.NewCode(CodeNotAccessible)
	}
	audience, err := s.principal.Current(ctx)
	if err != nil || audience == nil || audience.UserID != current.UserID || audience.Code != principal.AudienceCustomer || audience.Status != principal.AudienceStatusActive {
		return 0, "", bizerr.NewCode(CodeNotAccessible)
	}
	return current.UserID, audience.CustomerNo, nil
}
func (s *serviceImpl) newNo(prefix string) (string, error) {
	buf := make([]byte, 16)
	if _, err := io.ReadFull(s.random, buf); err != nil {
		return "", err
	}
	return prefix + hex.EncodeToString(buf), nil
}
func cursorEncode(id int64) string {
	if id <= 0 {
		return ""
	}
	return base64.RawURLEncoding.EncodeToString([]byte(strconv.FormatInt(id, 10)))
}
func cursorDecode(value string) (int64, error) {
	if value == "" {
		return 0, nil
	}
	raw, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return 0, bizerr.NewCode(CodeInvalid)
	}
	id, err := strconv.ParseInt(string(raw), 10, 64)
	if err != nil || id <= 0 {
		return 0, bizerr.NewCode(CodeInvalid)
	}
	return id, nil
}
func contentTypeValid(value string) bool {
	return value == "news" || value == "announcement" || value == "knowledge" || value == "download"
}
func visibilityValid(contentType, value string) bool {
	if value == "public" || value == "all_logged_in" || value == "customer_group" || value == "internal" {
		return true
	}
	return contentType == "announcement" && (value == "specific_customers" || value == "specific_services")
}
func trimBound(value string, maxBytes int, required bool) (string, bool) {
	value = strings.TrimSpace(value)
	if required && value == "" {
		return "", false
	}
	return value, len([]byte(value)) <= maxBytes
}
func uniqueBusinessNos(values []string, maxItems int) ([]string, bool) {
	if len(values) > maxItems {
		return nil, false
	}
	result := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if !businessNumberPattern.MatchString(value) {
			return nil, false
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result, true
}
func contentWrap(err error, operation string) error {
	if err == nil {
		return nil
	}
	if _, ok := bizerr.As(err); ok {
		return err
	}
	return bizerr.WrapCode(gerror.Wrap(err, operation), CodeUnavailable)
}
