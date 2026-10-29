// Package notice owns announcement content and each signed-in user's read state.
package notice

import (
	"context"

	"lina-core/pkg/plugin/capability/bizctxcap"
	"lina-core/pkg/plugin/capability/tenantcap"
	"lina-core/pkg/plugin/capability/usercap"
)

// Notice status values (matching sys_notice_status dictionary)
const (
	noticeStatusDraft     = 0 // Draft
	noticeStatusPublished = 1 // Published
)

// Service defines announcement management and current-user read state.
type Service interface {
	// Published lists all published announcements; read state is only loaded for the current user when signedIn is true.
	Published(ctx context.Context, pageNum, pageSize int, signedIn bool) (*PublishedPage, error)
	// PublishedByID returns one published announcement or CodeNoticeNotFound, without modifying read state.
	PublishedByID(ctx context.Context, id int64, signedIn bool) (*PublishedItem, error)
	// MarkRead idempotently persists a read record for the signed-in user; drafts and missing IDs are rejected.
	MarkRead(ctx context.Context, id int64) error
	// List returns notices visible to ctx's tenant using the supplied page and
	// fuzzy title/creator filters. It returns DAO or tenant-scope errors.
	List(ctx context.Context, in ListInput) (*ListOutput, error)
	// GetById returns one tenant-visible notice by primary key with creator
	// display metadata, or CodeNoticeNotFound when the row is outside scope or absent.
	GetById(ctx context.Context, id int64) (*ListItem, error)
	// Create stores an announcement in the current installation.
	Create(ctx context.Context, in CreateInput) (int64, error)
	// Update changes an existing announcement, including its sort order.
	Update(ctx context.Context, in UpdateInput) error
	// Delete atomically removes the requested announcements; missing IDs reject the operation.
	Delete(ctx context.Context, ids []int64) error
}

// Ensure serviceImpl implements Service.
var _ Service = (*serviceImpl)(nil)

// serviceImpl implements Service.
type serviceImpl struct {
	bizCtxSvc bizctxcap.Service // Business context bridge
	tenantSvc tenantcap.Service // Tenant capability bridge
	userSvc   usercap.Service   // User domain projection capability
}

// New creates and returns a new Service instance.
func New(
	bizCtxSvc bizctxcap.Service,
	tenantSvc tenantcap.Service,
	userSvc usercap.Service,
) Service {
	return &serviceImpl{
		bizCtxSvc: bizCtxSvc,
		tenantSvc: tenantSvc,
		userSvc:   userSvc,
	}
}

// ListInput defines input for List function.
type ListInput struct {
	PageNum   int    // Page number, starting from 1
	PageSize  int    // Page size
	Title     string // Title, supports fuzzy search
	CreatedBy string // Creator username, supports fuzzy search
}

// ListItem defines a single list item.
type ListItem struct {
	*NoticeEntity        // Notice entity
	CreatedByName string `json:"createdByName"` // Creator username
}

// ListOutput defines output for List function.
type ListOutput struct {
	List  []*ListItem // List items
	Total int         // Total count
}

// CreateInput defines input for Create function.
type CreateInput struct {
	SortOrder int    // Smaller values appear first
	Title     string // Title
	Content   string // Content
	Status    int    // Status: 0=Draft 1=Published
}

// UpdateInput defines input for Update function.
type UpdateInput struct {
	Id        int64   // Notice ID
	SortOrder *int    // Smaller values appear first
	Title     *string // Title
	Content   *string // Content
	Status    *int    // Status: 0=Draft 1=Published
}

// PublishedItem projects a published announcement and the current user's read state.
type PublishedItem struct {
	*NoticeEntity
	Read bool
}

// PublishedPage contains a bounded page and its total count.
type PublishedPage struct {
	List  []*PublishedItem
	Total int
}
