// This file reads the shared published announcement source and persists isolated user read state.

package notice

import (
	"context"
	"lina-core/pkg/bizerr"
	"lina-plugin-content-notice/backend/internal/dao"
	"lina-plugin-content-notice/backend/internal/model/do"
	"lina-plugin-content-notice/backend/internal/model/entity"
)

// Published lists public announcement content and loads current-page read state in a single batch.
func (s *serviceImpl) Published(ctx context.Context, pageNum, pageSize int, signedIn bool) (*PublishedPage, error) {
	userID, err := s.readerID(ctx, signedIn)
	if err != nil {
		return nil, err
	}
	if pageNum < 1 {
		pageNum = 1
	}
	if pageSize < 1 {
		pageSize = 10
	}
	if pageSize > 100 {
		pageSize = 100
	}
	var (
		cols  = dao.Notice.Columns()
		query = dao.Notice.Ctx(ctx).Where(do.Notice{Status: noticeStatusPublished})
		rows  []*NoticeEntity
	)
	total, err := query.Count()
	if err != nil {
		return nil, err
	}
	if err = query.OrderAsc(cols.SortOrder).OrderDesc(cols.Id).Page(pageNum, pageSize).Scan(&rows); err != nil {
		return nil, err
	}
	readIDs, err := s.readIDs(ctx, userID, rows)
	if err != nil {
		return nil, err
	}
	out := &PublishedPage{List: make([]*PublishedItem, 0, len(rows)), Total: total}
	for _, row := range rows {
		row.Content = sanitizeContent(row.Content)
		out.List = append(out.List, &PublishedItem{NoticeEntity: row, Read: readIDs[row.Id]})
	}
	return out, nil
}

// PublishedByID only returns published announcements, including a user's own read state.
func (s *serviceImpl) PublishedByID(ctx context.Context, id int64, signedIn bool) (*PublishedItem, error) {
	userID, err := s.readerID(ctx, signedIn)
	if err != nil {
		return nil, err
	}
	var row *NoticeEntity
	if id <= 0 {
		return nil, bizerr.NewCode(CodeNoticeNotFound)
	}
	if err = dao.Notice.Ctx(ctx).Where(do.Notice{Id: id, Status: noticeStatusPublished}).Scan(&row); err != nil {
		return nil, err
	}
	if row == nil || row.Id == 0 {
		return nil, bizerr.NewCode(CodeNoticeNotFound)
	}
	readIDs, err := s.readIDs(ctx, userID, []*NoticeEntity{row})
	if err != nil {
		return nil, err
	}
	row.Content = sanitizeContent(row.Content)
	return &PublishedItem{NoticeEntity: row, Read: readIDs[id]}, nil
}

// MarkRead does not accept a caller-selected user and is safe to repeat.
func (s *serviceImpl) MarkRead(ctx context.Context, id int64) error {
	userID, err := s.readerID(ctx, true)
	if err != nil {
		return err
	}
	if _, err = s.PublishedByID(ctx, id, true); err != nil {
		return err
	}
	_, err = dao.NoticeRead.Ctx(ctx).Data(do.NoticeRead{NoticeId: id, UserId: userID}).InsertIgnore()
	return err
}

// readerID uses the authenticated host identity; public requests have no read state.
func (s *serviceImpl) readerID(ctx context.Context, required bool) (int64, error) {
	if !required {
		return 0, nil
	}
	if s == nil || s.bizCtxSvc == nil {
		return 0, bizerr.NewCode(CodeLoginRequired)
	}
	userID := int64(s.bizCtxSvc.Current(ctx).UserID)
	if userID <= 0 {
		return 0, bizerr.NewCode(CodeLoginRequired)
	}
	return userID, nil
}

// readIDs fetches read markers for this page in one query, scoped to the signed-in user.
func (s *serviceImpl) readIDs(ctx context.Context, userID int64, rows []*NoticeEntity) (map[int64]bool, error) {
	result := make(map[int64]bool, len(rows))
	if userID <= 0 || len(rows) == 0 {
		return result, nil
	}
	ids := make([]int64, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.Id)
	}
	var reads []*entity.NoticeRead
	if err := dao.NoticeRead.Ctx(ctx).Where(do.NoticeRead{UserId: userID}).WhereIn(dao.NoticeRead.Columns().NoticeId, ids).Scan(&reads); err != nil {
		return nil, err
	}
	for _, read := range reads {
		result[read.NoticeId] = true
	}
	return result, nil
}
