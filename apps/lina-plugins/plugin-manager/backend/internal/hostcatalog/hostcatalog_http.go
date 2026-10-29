// This file dispatches bounded public summary GETs with a freshly authenticated request context.

package hostcatalog

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/gogf/gf/v2/net/ghttp"
	pluginv1 "lina-core/api/plugin/v1"
	"lina-core/pkg/bizerr"
	"net/http"
	"net/url"
	"strconv"
)

// List uses the active request's server and allowlisted headers to read one public page.
func (r *Reader) List(ctx context.Context, page int, size int, id string, name string) (*pluginv1.ListRes, error) {
	incoming := ghttp.RequestFromCtx(ctx)
	if incoming == nil || incoming.Server == nil {
		return nil, bizerr.NewCode(CodeReadFailed)
	}
	params := url.Values{"pageNum": {strconv.Itoa(page)}, "pageSize": {strconv.Itoa(size)}, "type": {pluginv1.PluginTypeDynamic.String()}, "id": {id}, "name": {name}}
	// Rebuild request scope through public middleware instead of inheriting host business context.
	childCtx, cancel := context.WithCancel(context.Background())
	stop := context.AfterFunc(ctx, cancel)
	defer cancel()
	defer stop()
	request, err := http.NewRequestWithContext(childCtx, http.MethodGet, "/api/v1/plugins?"+params.Encode(), nil)
	if err != nil {
		return nil, bizerr.WrapCode(err, CodeReadFailed)
	}
	request.Body = http.NoBody
	request.Host = incoming.Host
	request.RemoteAddr = incoming.RemoteAddr
	for _, key := range []string{"Authorization", "Accept-Language", "X-Tenant-Id"} {
		request.Header.Set(key, incoming.Header.Get(key))
	}
	response := &responseBuffer{header: make(http.Header)}
	incoming.Server.ServeHTTP(response, request)
	if response.status != http.StatusOK || response.overflow {
		return nil, bizerr.NewCode(CodeReadFailed)
	}
	var envelope struct {
		Code int               `json:"code"`
		Data *pluginv1.ListRes `json:"data"`
	}
	if err = json.Unmarshal(response.body.Bytes(), &envelope); err != nil {
		return nil, bizerr.WrapCode(err, CodeReadFailed)
	}
	if envelope.Code != 0 || envelope.Data == nil {
		return nil, bizerr.NewCode(CodeReadFailed)
	}
	return envelope.Data, nil
}

// responseBuffer bounds the internal HTTP response without using test-only recorders.
type responseBuffer struct {
	header   http.Header
	body     bytes.Buffer
	status   int
	overflow bool
}

func (w *responseBuffer) Header() http.Header { return w.header }
func (w *responseBuffer) WriteHeader(status int) {
	if w.status == 0 {
		w.status = status
	}
}
func (w *responseBuffer) Write(body []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	if w.body.Len()+len(body) > 4*1024*1024 {
		w.overflow = true
		return len(body), nil
	}
	return w.body.Write(body)
}
