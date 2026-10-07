package supportstream

import (
	"context"
	"io"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gogf/gf/v2/frame/g"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/internal/service/supportdesk"
)

func Write(ctx context.Context, file *supportdesk.AttachmentOpen) error {
	if file == nil || file.Body == nil || file.SizeBytes < 1 {
		return bizerr.NewCode(supportdesk.CodeNotAccessible)
	}
	defer file.Body.Close()
	request := g.RequestFromCtx(ctx)
	if request == nil || request.Response == nil {
		return bizerr.NewCode(supportdesk.CodeUnavailable)
	}
	name := filepath.Base(strings.TrimSpace(file.FileName))
	if name == "" || name == "." {
		name = "attachment"
	}
	mediaType := strings.TrimSpace(file.MediaType)
	if mediaType == "" {
		mediaType = "application/octet-stream"
	}
	request.Response.Header().Set("Content-Type", mediaType)
	request.Response.Header().Set("Content-Length", strconv.FormatInt(file.SizeBytes, 10))
	request.Response.Header().Set("Content-Disposition", "attachment; filename*=UTF-8''"+url.PathEscape(name))
	request.Response.Header().Set("Cache-Control", "private, no-store")
	request.Response.Header().Set("X-Content-Type-Options", "nosniff")
	_, err := io.Copy(request.Response.Writer, file.Body)
	return err
}
