// Package catalog owns configurable business grouping over the public host catalog.
package catalog

import (
	"context"
	"github.com/gogf/gf/v2/errors/gerror"
	pluginv1 "lina-core/api/plugin/v1"
	v1 "lina-plugin-plugin-manager/backend/api/management/v1"
)

// Category configures a tab independently of plugin runtime types and capabilities.
type Category struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	Plugins  []string `json:"plugins"`
	Prefixes []string `json:"prefixes"`
}

// Config is ordered, open-ended business presentation configuration.
type Config struct {
	Fallback   string     `json:"fallback"`
	Categories []Category `json:"categories"`
}

// Reader reads bounded public summaries within the current caller's host authorization.
type Reader interface {
	// List reads one host page. Authorization failure or incomplete data is an error.
	List(ctx context.Context, page int, size int, id string, name string) (*pluginv1.ListRes, error)
}

// Service provides exact category totals and server-side pagination without detail fan-out.
type Service interface {
	// Categories returns configured order and names without exposing classification internals.
	Categories(ctx context.Context) []v1.Category
	// List filters only authorized dynamic managed summaries; unknown categories and overflow fail closed.
	List(ctx context.Context, input *v1.ListReq) (*v1.ListRes, error)
}

type serviceImpl struct {
	config Config
	reader Reader
}

var _ Service = (*serviceImpl)(nil)

// New validates startup configuration and binds the one public catalog reader.
func New(config Config, reader Reader) (Service, error) {
	if reader == nil {
		return nil, gerror.New("plugin catalog reader is required")
	}
	if err := validateConfig(config); err != nil {
		return nil, err
	}
	return &serviceImpl{config: config, reader: reader}, nil
}
