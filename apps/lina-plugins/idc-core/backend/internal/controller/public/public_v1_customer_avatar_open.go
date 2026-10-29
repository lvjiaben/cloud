// This file streams validated avatar images by opaque keys, without account IDs or filesystem paths.

package public

import (
	"context"
	"github.com/gogf/gf/v2/frame/g"
	"io"
	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/api/public/v1"
	"lina-plugin-idc-core/backend/internal/service/customeravatar"
	"strings"
)

// CustomerAvatarOpen serves an uploaded public avatar through plugin storage.
func (c *ControllerV1) CustomerAvatarOpen(ctx context.Context, req *v1.CustomerAvatarOpenReq) (res *v1.CustomerAvatarOpenRes, err error) {
	object, err := c.avatars.Open(ctx, req.Key)
	if err != nil {
		return nil, err
	}
	defer func() {
		if closeErr := object.Body.Close(); closeErr != nil && err == nil {
			err = bizerr.WrapCode(closeErr, customeravatar.CodeUnavailable)
		}
	}()
	request := g.RequestFromCtx(ctx)
	request.Response.Header().Set("Content-Type", "image/"+req.Key[strings.LastIndexByte(req.Key, '.')+1:])
	request.Response.Header().Set("X-Content-Type-Options", "nosniff")
	request.Response.Header().Set("Cache-Control", "public, max-age=86400, immutable")
	if _, err = io.Copy(request.Response.RawWriter(), object.Body); err != nil {
		return nil, bizerr.WrapCode(err, customeravatar.CodeUnavailable)
	}
	request.ExitAll()
	return &v1.CustomerAvatarOpenRes{}, nil
}
