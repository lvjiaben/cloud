package contentaccount

import (
	"context"
	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
	"io"
	v1 "lina-plugin-idc-core/backend/api/contentaccount/v1"
	"lina-plugin-idc-core/backend/internal/service/contenthub"
	"strconv"
	"strings"
)

type ControllerV1 struct{ service contenthub.Service }

func NewV1(service contenthub.Service) (*ControllerV1, error) {
	if service == nil {
		return nil, gerror.New("account content controller requires service")
	}
	return &ControllerV1{service: service}, nil
}
func (c *ControllerV1) ContentList(ctx context.Context, req *v1.ContentListReq) (*v1.ContentListRes, error) {
	page, err := c.service.SearchAccount(ctx, contenthub.SearchInput{Type: req.Type, Query: req.Query, Cursor: req.Cursor, Limit: req.Limit})
	if err != nil {
		return nil, err
	}
	items := make([]*v1.ContentSummary, 0, len(page.Items))
	for _, item := range page.Items {
		items = append(items, toSummary(item))
	}
	return &v1.ContentListRes{Items: items, NextCursor: page.NextCursor}, nil
}
func (c *ControllerV1) ContentDetail(ctx context.Context, req *v1.ContentDetailReq) (*v1.ContentDetailRes, error) {
	item, err := c.service.DetailAccount(ctx, req.ContentNo)
	if err != nil {
		return nil, err
	}
	return &v1.ContentDetailRes{Item: toDetail(item)}, nil
}

func toSummary(item *contenthub.Item) *v1.ContentSummary {
	if item == nil {
		return nil
	}
	result := &v1.ContentSummary{ContentNo: item.ContentNo, PublicPath: item.PublicPath, Type: item.Type, Title: item.Title, Summary: item.Summary}
	if item.CategoryNameZhCN != "" || item.CategoryNameEnUS != "" {
		result.Category = &v1.CategoryLabel{NameZhCN: item.CategoryNameZhCN, NameEnUS: item.CategoryNameEnUS}
	}
	if item.PublishedAt != nil {
		result.PublishedAt = item.PublishedAt.UnixMilli()
	}
	if item.UpdatedAt != nil {
		result.UpdatedAt = item.UpdatedAt.UnixMilli()
	}
	return result
}

func toDetail(item *contenthub.Item) *v1.ContentDetail {
	if item == nil {
		return nil
	}
	result := &v1.ContentDetail{ContentSummary: *toSummary(item), Body: item.Body, SEOTitle: item.SEOTitle, SEODescription: item.SEODescription}
	if item.CurrentFile != nil {
		result.File = &v1.PublicFile{FileName: item.CurrentFile.FileName, MediaType: item.CurrentFile.MediaType, SizeBytes: item.CurrentFile.SizeBytes, SHA256: item.CurrentFile.SHA256}
	}
	return result
}
func (c *ControllerV1) DownloadTicketCreate(ctx context.Context, req *v1.DownloadTicketCreateReq) (*v1.DownloadTicketCreateRes, error) {
	ticket, err := c.service.IssueAccountDownload(ctx, req.ContentNo)
	if err != nil {
		return nil, err
	}
	return &v1.DownloadTicketCreateRes{Token: ticket.Token, ExpiresAt: ticket.ExpiresAt.UnixMilli()}, nil
}
func (c *ControllerV1) DownloadOpen(ctx context.Context, req *v1.DownloadOpenReq) (res *v1.DownloadOpenRes, err error) {
	request := g.RequestFromCtx(ctx)
	file, err := c.service.OpenAccountDownload(ctx, req.Token)
	if err != nil {
		return nil, err
	}
	if request == nil || file == nil || file.Body == nil {
		return nil, gerror.New("download stream is unavailable")
	}
	defer func() {
		if closeErr := file.Body.Close(); err == nil && closeErr != nil {
			err = closeErr
		}
	}()
	mediaType := strings.TrimSpace(file.MediaType)
	if mediaType == "" {
		mediaType = "application/octet-stream"
	}
	request.Response.Header().Set("Content-Type", mediaType)
	request.Response.Header().Set("Content-Length", strconv.FormatInt(file.SizeBytes, 10))
	safeName := strings.NewReplacer("\\", "_", "\"", "_", "\r", "_", "\n", "_").Replace(file.FileName)
	request.Response.Header().Set("Content-Disposition", "attachment; filename=\""+safeName+"\"")
	if _, err = io.Copy(request.Response.RawWriter(), file.Body); err != nil {
		return nil, err
	}
	request.ExitAll()
	return &v1.DownloadOpenRes{}, nil
}
