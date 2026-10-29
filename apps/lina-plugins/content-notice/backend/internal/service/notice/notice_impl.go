// notice_impl.go implements notice CRUD, publish state transitions, and
// tenant-scoped query/export behavior for the content-notice plugin. It keeps
// host capability and tenant dependencies injected so plugin-owned notice rows
// remain isolated from host-internal services.

package notice

import (
	"context"
	"strconv"
	"strings"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
	"lina-core/pkg/plugin/capability/capmodel"
	"lina-core/pkg/plugin/capability/tenantcap"
	"lina-core/pkg/plugin/capability/tenantcap/tenantspi"
	"lina-core/pkg/plugin/capability/usercap"
	"lina-plugin-content-notice/backend/internal/dao"
	"lina-plugin-content-notice/backend/internal/model/do"
)

const (
	pluginID                        = "content-notice"
	noticeCreatorCapabilityResource = "notice.creator"
	noticeCreatorSearchLimit        = 200
	maxNoticeDeleteBatch            = 100
)

// List queries notice list with pagination and filters.
func (s *serviceImpl) List(ctx context.Context, in ListInput) (*ListOutput, error) {
	if in.PageNum <= 0 {
		in.PageNum = 1
	}
	if in.PageSize <= 0 {
		in.PageSize = 10
	}
	if in.PageSize > 100 {
		in.PageSize = 100
	}
	query := s.scopedQuery(ctx)
	columns := dao.Notice.Columns()
	if title := strings.TrimSpace(in.Title); title != "" {
		query = query.WhereLike(columns.Title, "%"+title+"%")
	}
	if createdBy := strings.TrimSpace(in.CreatedBy); createdBy != "" {
		creatorIDs, err := s.searchCreatorUserIDs(ctx, createdBy)
		if err != nil {
			return nil, err
		}
		if len(creatorIDs) == 0 {
			return &ListOutput{List: []*ListItem{}, Total: 0}, nil
		}
		query = query.WhereIn(columns.CreatedBy, creatorIDs)
	}
	total, err := query.Count()
	if err != nil {
		return nil, err
	}
	list := make([]*NoticeEntity, 0, in.PageSize)
	if total > 0 {
		err = query.OrderAsc(columns.SortOrder).OrderDesc(columns.Id).Page(in.PageNum, in.PageSize).Scan(&list)
		if err != nil {
			return nil, err
		}
	}

	userNameMap, err := s.resolveCreatorNameMap(ctx, list)
	if err != nil {
		return nil, err
	}

	// Build result
	items := make([]*ListItem, 0, len(list))
	for _, n := range list {
		n.Content = sanitizeContent(n.Content)
		items = append(items, &ListItem{
			NoticeEntity:  n,
			CreatedByName: userNameMap[n.CreatedBy],
		})
	}

	return &ListOutput{
		List:  items,
		Total: int(total),
	}, nil
}

// GetById retrieves notice by ID.
func (s *serviceImpl) GetById(ctx context.Context, id int64) (*ListItem, error) {
	notice, err := s.getScopedByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if notice == nil || notice.Id == 0 {
		return nil, bizerr.NewCode(CodeNoticeNotFound)
	}

	item := &ListItem{NoticeEntity: notice}

	// Resolve creator username
	if notice.CreatedBy > 0 {
		names, err := s.resolveCreatorNameMap(ctx, []*NoticeEntity{notice})
		if err != nil {
			return nil, err
		}
		if name := names[notice.CreatedBy]; name != "" {
			item.CreatedByName = name
		}
	}

	return item, nil
}

// Create creates a new notice.
func (s *serviceImpl) Create(ctx context.Context, in CreateInput) (int64, error) {
	if s == nil || s.bizCtxSvc == nil || s.pluginTableFilter() == nil {
		return 0, gerror.New("content-notice host services are unavailable")
	}
	var (
		bizCtx    = s.bizCtxSvc.Current(ctx)
		createdBy = int64(bizCtx.UserID)
		tenantID  = s.tenantFilterContext(ctx).TenantID
	)

	id, err := dao.Notice.Ctx(ctx).Data(do.Notice{
		TenantId:  tenantID,
		Title:     in.Title,
		SortOrder: in.SortOrder,
		Content:   sanitizeContent(in.Content),
		Status:    in.Status,
		CreatedBy: createdBy,
		UpdatedBy: createdBy,
	}).InsertAndGetId()
	if err != nil {
		return 0, err
	}

	return id, nil
}

// Update updates notice information.
func (s *serviceImpl) Update(ctx context.Context, in UpdateInput) error {
	if s == nil || s.bizCtxSvc == nil || s.pluginTableFilter() == nil {
		return gerror.New("content-notice host services are unavailable")
	}
	_, err := s.getScopedByID(ctx, in.Id)
	if err != nil {
		return err
	}
	updatedBy := int64(s.bizCtxSvc.Current(ctx).UserID)
	data := do.Notice{UpdatedBy: updatedBy}
	if in.SortOrder != nil {
		data.SortOrder = *in.SortOrder
	}
	if in.Title != nil {
		data.Title = *in.Title
	}
	if in.Content != nil {
		data.Content = sanitizeContent(*in.Content)
	}
	if in.Status != nil {
		data.Status = *in.Status
	}
	result, err := s.scopedQuery(ctx).
		OmitNilData().
		Where(dao.Notice.Columns().Id, in.Id).
		Data(data).
		Update()
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected != 1 {
		return bizerr.NewCode(CodeNoticeNotFound)
	}

	return nil
}

// Delete removes the complete requested announcement set after checking ownership.
func (s *serviceImpl) Delete(ctx context.Context, ids []int64) error {
	idList := normalizePositiveInt64IDs(ids)
	if len(idList) == 0 {
		return bizerr.NewCode(CodeNoticeDeleteRequired)
	}
	if len(idList) > maxNoticeDeleteBatch {
		return gerror.Newf("content-notice delete batch exceeds %d items", maxNoticeDeleteBatch)
	}
	if s == nil || s.pluginTableFilter() == nil {
		return gerror.New("content-notice tenant filter service is unavailable")
	}
	count, err := s.scopedQuery(ctx).WhereIn(dao.Notice.Columns().Id, idList).Count()
	if err != nil {
		return err
	}
	if count != len(idList) {
		return bizerr.NewCode(CodeNoticeNotFound)
	}
	_, err = s.scopedQuery(ctx).
		WhereIn(dao.Notice.Columns().Id, idList).
		Delete()
	if err != nil {
		return err
	}

	return nil
}

// normalizePositiveInt64IDs drops non-positive identifiers from a batch ID list.
func normalizePositiveInt64IDs(ids []int64) []int64 {
	if len(ids) == 0 {
		return nil
	}
	result := make([]int64, 0, len(ids))
	seen := make(map[int64]struct{}, len(ids))
	for _, id := range ids {
		if id <= 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		result = append(result, id)
	}
	return result
}

// scopedQuery applies the same tenant and soft-delete visibility rules to all
// notice reads before a key-only governed mutation is allowed.
func (s *serviceImpl) scopedQuery(ctx context.Context) *gdb.Model {
	return tenantspi.ApplyPluginTableFilter(ctx, s.pluginTableFilter(), dao.Notice.Ctx(ctx), "")
}

func (s *serviceImpl) getScopedByID(ctx context.Context, id int64) (*NoticeEntity, error) {
	if s == nil || s.pluginTableFilter() == nil || id <= 0 {
		return nil, bizerr.NewCode(CodeNoticeNotFound)
	}
	var notice *NoticeEntity
	err := s.scopedQuery(ctx).
		Where(dao.Notice.Columns().Id, id).
		Scan(&notice)
	if err != nil {
		return nil, err
	}
	if notice == nil || notice.Id == 0 {
		return nil, bizerr.NewCode(CodeNoticeNotFound)
	}
	notice.Content = sanitizeContent(notice.Content)
	return notice, nil
}

// pluginTableFilter returns the tenant table-filter slice from the injected tenant service.
func (s *serviceImpl) pluginTableFilter() tenantcap.FilterService {
	if s == nil || s.tenantSvc == nil {
		return nil
	}
	return s.tenantSvc.Filter()
}

// tenantFilterContext returns current tenant metadata for write ownership fields.
func (s *serviceImpl) tenantFilterContext(ctx context.Context) tenantcap.TenantFilterContext {
	if filter := s.pluginTableFilter(); filter != nil {
		return filter.Context(ctx)
	}
	return tenantcap.TenantFilterContext{}
}

// searchCreatorUserIDs resolves a creator keyword through the host user domain
// capability before filtering plugin-owned notice rows by creator ID.
func (s *serviceImpl) searchCreatorUserIDs(ctx context.Context, keyword string) ([]int64, error) {
	if s.userSvc == nil {
		return nil, gerror.New("content-notice requires host user capability")
	}
	result, err := s.userSvc.List(ctx, usercap.ListInput{
		Keyword: strings.TrimSpace(keyword),
		Page: capmodel.PageRequest{
			PageNum:  1,
			PageSize: noticeCreatorSearchLimit,
			Limit:    noticeCreatorSearchLimit,
		},
	})
	if err != nil || result == nil {
		return nil, err
	}
	return userInfoStorageIDs(result.Items), nil
}

// resolveCreatorNameMap resolves current-page creator display names through one
// user-domain batch call and leaves invisible or missing users blank.
func (s *serviceImpl) resolveCreatorNameMap(ctx context.Context, notices []*NoticeEntity) (map[int64]string, error) {
	names := make(map[int64]string)
	userIDs := creatorDomainIDs(notices)
	if len(userIDs) == 0 {
		return names, nil
	}
	if s.userSvc == nil {
		return nil, gerror.New("content-notice requires host user capability")
	}
	result, err := s.userSvc.BatchGet(ctx, userIDs)
	if err != nil || result == nil {
		return names, err
	}
	for id, projection := range result.Items {
		storageID, ok := userDomainIDStorageID(id)
		if !ok {
			continue
		}
		names[storageID] = userInfoDisplayName(projection)
	}
	return names, nil
}

// creatorDomainIDs converts plugin-owned notice creator storage values to
// user-domain IDs for host capability calls while de-duplicating the current page.
func creatorDomainIDs(notices []*NoticeEntity) []usercap.UserID {
	ids := make([]usercap.UserID, 0, len(notices))
	seen := make(map[int64]struct{}, len(notices))
	for _, notice := range notices {
		if notice == nil || notice.CreatedBy <= 0 {
			continue
		}
		if _, ok := seen[notice.CreatedBy]; ok {
			continue
		}
		seen[notice.CreatedBy] = struct{}{}
		ids = append(ids, usercap.UserID(strconv.FormatInt(notice.CreatedBy, 10)))
	}
	return ids
}

// userInfoStorageIDs converts visible user projections back to plugin
// notice creator IDs for database-side notice filtering.
func userInfoStorageIDs(users []*usercap.UserInfo) []int64 {
	ids := make([]int64, 0, len(users))
	for _, user := range users {
		if user == nil {
			continue
		}
		id, ok := userDomainIDStorageID(user.ID)
		if ok {
			ids = append(ids, id)
		}
	}
	return ids
}

// userDomainIDStorageID parses the current host user-domain ID encoding used by
// existing plugin-owned notice creator columns.
func userDomainIDStorageID(id usercap.UserID) (int64, bool) {
	storageID, err := strconv.ParseInt(strings.TrimSpace(string(id)), 10, 64)
	return storageID, err == nil && storageID > 0
}

// userInfoDisplayName chooses the stable notice creator display field
// from the current user-domain projection.
func userInfoDisplayName(user *usercap.UserInfo) string {
	if user == nil {
		return ""
	}
	if user.Username != "" {
		return user.Username
	}
	if user.Nickname != "" {
		return user.Nickname
	}
	if user.Label != "" {
		return user.Label
	}
	return string(user.ID)
}
