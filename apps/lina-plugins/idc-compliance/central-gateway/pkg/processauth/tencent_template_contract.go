// This file defines the exact signed read route for buyer-owned Tencent templates.

package processauth

const (
	// TencentTemplatesPath returns one complete, bounded safe template projection.
	TencentTemplatesPath = "/internal/v1/tencent-essbasic/templates"
	// TencentTemplatesReadScope authenticates only explicit template synchronization.
	TencentTemplatesReadScope = "tencent-essbasic.templates.read"
	// TencentTemplatePreviewsPath returns one short-lived preview URL for a server-owned template ref.
	TencentTemplatePreviewsPath = "/internal/v1/tencent-essbasic/template-previews"
	// TencentTemplatePreviewReadScope authenticates only an explicit template preview read.
	TencentTemplatePreviewReadScope = "tencent-essbasic.template-preview.read"
)
