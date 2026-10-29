// This file wires the public announcement controller to its owning service.

package publicnotice

import (
	"github.com/gogf/gf/v2/errors/gerror"
	"lina-core/pkg/apitime"
	v1 "lina-plugin-content-notice/backend/api/publicnotice/v1"
	noticesvc "lina-plugin-content-notice/backend/internal/service/notice"
)

// ControllerV1 exposes only published announcement content.
type ControllerV1 struct{ service noticesvc.Service }

// NewV1WithService injects the same source used by announcement management.
func NewV1WithService(service noticesvc.Service) (*ControllerV1, error) {
	if service == nil {
		return nil, gerror.New("public announcement controller requires its owning service")
	}
	return &ControllerV1{service: service}, nil
}

// toItem projects public content without account or management metadata.
func toItem(item *noticesvc.PublishedItem) v1.Item {
	return v1.Item{Id: item.Id, Title: item.Title, Content: item.Content, SortOrder: item.SortOrder,
		CreatedAt: apitime.Milli(item.CreatedAt), UpdatedAt: apitime.Milli(item.UpdatedAt)}
}
