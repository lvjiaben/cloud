// Content API contracts cover bounded administration, simple publication, and guarded file workflows.

package v1

import "github.com/gogf/gf/v2/frame/g"

type ActiveUpload struct {
	UploadNo       string `json:"uploadNo"`
	FileName       string `json:"fileName"`
	SizeBytes      int64  `json:"sizeBytes"`
	ConfirmedBytes int64  `json:"confirmedBytes"`
	ChunkSizeBytes int64  `json:"chunkSizeBytes"`
	ExpiresAt      int64  `json:"expiresAt"`
	Version        int64  `json:"version"`
	CanResume      bool   `json:"canResume"`
	CanCancel      bool   `json:"canCancel"`
}
type CandidateFile struct {
	FileNo        string `json:"fileNo"`
	FileName      string `json:"fileName"`
	MediaType     string `json:"mediaType"`
	ContentKind   string `json:"contentKind"`
	GateStatus    string `json:"gateStatus"`
	ScanStatus    string `json:"scanStatus"`
	SizeBytes     int64  `json:"sizeBytes"`
	Version       int64  `json:"version"`
	GateExpiresAt int64  `json:"gateExpiresAt,omitempty"`
	CanCancel     bool   `json:"canCancel"`
	CanRescan     bool   `json:"canRescan"`
	CanEnable     bool   `json:"canEnable"`
}
type CurrentContentFile struct {
	FileNo      string `json:"fileNo"`
	FileName    string `json:"fileName"`
	MediaType   string `json:"mediaType"`
	ContentKind string `json:"contentKind"`
	SizeBytes   int64  `json:"sizeBytes"`
	SHA256      string `json:"sha256"`
	Version     int64  `json:"version"`
}
type ContentItem struct {
	ContentNo              string              `json:"contentNo"`
	Type                   string              `json:"type"`
	CategoryNo             string              `json:"categoryNo,omitempty"`
	Visibility             string              `json:"visibility"`
	Title                  string              `json:"title"`
	Summary                string              `json:"summary,omitempty"`
	Body                   string              `json:"body,omitempty"`
	SEOTitle               string              `json:"seoTitle,omitempty"`
	SEODescription         string              `json:"seoDescription,omitempty"`
	PublicPath             string              `json:"publicPath"`
	Status                 string              `json:"status"`
	CurrentFileNo          string              `json:"currentFileNo,omitempty"`
	ValidityVersion        int64               `json:"validityVersion"`
	Version                int64               `json:"version"`
	PublishedAt            int64               `json:"publishedAt,omitempty"`
	OfflineAt              int64               `json:"offlineAt,omitempty"`
	CreatedAt              int64               `json:"createdAt"`
	UpdatedAt              int64               `json:"updatedAt"`
	TargetCustomerGroupNos []string            `json:"targetCustomerGroupNos"`
	ActiveUpload           *ActiveUpload       `json:"activeUpload,omitempty"`
	CandidateFile          *CandidateFile      `json:"candidateFile,omitempty"`
	CurrentFile            *CurrentContentFile `json:"currentFile,omitempty"`
	AvailableActions       []string            `json:"availableActions"`
	TargetAudiences        []TargetAudience    `json:"targetAudiences"`
}
type TargetAudience struct {
	Kind   string `json:"kind"`
	Number string `json:"number"`
	Label  string `json:"label"`
}

// ContentListReq supports bounded administrative pages without changing public cursor readers.
type ContentListReq struct {
	g.Meta     `path:"/admin/content/items" method:"get" tags:"IDC Content" summary:"List managed content" dc:"Returns managed content and its exact filtered total; page selects numbered pagination, while cursor selects keyset pagination." permission:"idc:admin:content:manage"`
	Type       string `json:"type" dc:"Content type: news, knowledge or download; empty selects all types." eg:"news"`
	CategoryNo string `json:"categoryNo" dc:"Optional category business number; empty selects all categories." eg:"CAT_NEWS"`
	Status     string `json:"status" dc:"Optional publication status: draft, published or offline; empty selects all non-deleted items." eg:"draft"`
	Visibility string `json:"visibility" dc:"Optional audience: public, all_logged_in, customer_group or internal." eg:"public"`
	Query      string `json:"query" dc:"Optional title prefix or exact content number, up to 128 bytes." eg:"News"`
	Cursor     string `json:"cursor" dc:"Opaque cursor for keyset pagination; must be empty when page is set." eg:""`
	Limit      int    `json:"limit" d:"25" v:"min:1|max:100" dc:"Maximum rows per page, from 1 to 100; defaults to 25." eg:"10"`
	Page       int    `json:"page" v:"min:0|max:1000000" dc:"One-based page number; zero uses cursor pagination." eg:"1"`
}

type ContentCategory struct {
	CategoryNo string `json:"categoryNo"`
	Type       string `json:"type"`
	Name       string `json:"name"`
	SortOrder  int    `json:"sortOrder"`
	Status     string `json:"status"`
	Version    int64  `json:"version"`
	CreatedAt  int64  `json:"createdAt,omitempty"`
	UpdatedAt  int64  `json:"updatedAt,omitempty"`
	Synthetic  bool   `json:"synthetic"`
}
type ContentCategoryListReq struct {
	g.Meta                       `path:"/admin/content/categories" method:"get" tags:"IDC Content" summary:"List content categories" permission:"idc:admin:content:manage"`
	Type, Status, Locale, Cursor string
	Limit                        int `d:"50" v:"min:1|max:100"`
}
type ContentCategoryListRes struct {
	Items      []ContentCategory `json:"items"`
	NextCursor string            `json:"nextCursor"`
}
type ContentCategoryCreateReq struct {
	g.Meta    `path:"/admin/content/categories" method:"post" tags:"IDC Content" summary:"Create a content category" permission:"idc:admin:content:manage"`
	Type      string `json:"type"`
	Name      string `json:"name"`
	SortOrder int    `json:"sortOrder"`
}
type ContentCategoryCreateRes struct {
	Item *ContentCategory `json:"item"`
}
type ContentCategoryDeleteReq struct {
	g.Meta     `path:"/admin/content/categories/{categoryNo}" method:"delete" tags:"IDC Content" summary:"Delete a category and move its content to uncategorized" permission:"idc:admin:content:manage"`
	CategoryNo string `json:"categoryNo"`
	Version    int64  `json:"version"`
	Confirmed  bool   `json:"confirmed"`
}
type ContentCategoryDeleteRes struct{}
type ContentListRes struct {
	Items      []*ContentItem `json:"items"`
	NextCursor string         `json:"nextCursor"`
	Total      int            `json:"total" dc:"Exact count of non-deleted content matching the same administrative filters and authorization boundary." eg:"42"`
}
type ContentUploadPolicyReq struct {
	g.Meta `path:"/admin/content/upload-policy" method:"get" tags:"IDC Content" summary:"Get content upload policy" dc:"Returns the current fail-closed size and extension policy used before an administrator starts a download-file upload." permission:"idc:admin:content:manage"`
}
type ContentUploadPolicyRes struct {
	OrdinaryMaxSizeBytes           int64    `json:"ordinaryMaxSizeBytes" dc:"Maximum accepted size in bytes for ordinary image, PDF, and office-document uploads" eg:"5368709120"`
	InspectionRequiredMaxSizeBytes int64    `json:"inspectionRequiredMaxSizeBytes" dc:"Maximum accepted size in bytes for inspection-required uploads; zero when that upload class is unavailable" eg:"2147483648"`
	InspectionRequiredAvailable    bool     `json:"inspectionRequiredAvailable" dc:"Whether inspection-required uploads may currently be started after the local scanner passed its deployment self-test" eg:"true"`
	OrdinaryExtensions             []string `json:"ordinaryExtensions" dc:"Lowercase filename extensions accepted through the ordinary upload path" eg:"[\".pdf\",\".docx\"]"`
	InspectionRequiredExtensions   []string `json:"inspectionRequiredExtensions" dc:"Lowercase filename extensions accepted only through the inspection-required upload path" eg:"[\".zip\",\".7z\"]"`
}
type ContentDetailReq struct {
	g.Meta    `path:"/admin/content/items/{contentNo}" method:"get" tags:"IDC Content" summary:"Get managed content" permission:"idc:admin:content:manage"`
	ContentNo string `json:"contentNo"`
}
type ContentDetailRes struct {
	Item *ContentItem `json:"item"`
}
type ContentWriteFields struct {
	ContentNo              string   `json:"contentNo,omitempty"`
	Type                   string   `json:"type"`
	CategoryNo             string   `json:"categoryNo,omitempty"`
	Visibility             string   `json:"visibility"`
	Title                  string   `json:"title"`
	Summary                string   `json:"summary,omitempty"`
	Body                   string   `json:"body"`
	SEOTitle               string   `json:"seoTitle,omitempty"`
	SEODescription         string   `json:"seoDescription,omitempty"`
	Status                 string   `json:"status,omitempty"`
	Version                int64    `json:"version,omitempty"`
	TargetCustomerGroupNos []string `json:"targetCustomerGroupNos"`
}
type ContentCreateReq struct {
	g.Meta `path:"/admin/content/items" method:"post" tags:"IDC Content" summary:"Create content" permission:"idc:admin:content:manage"`
	ContentWriteFields
}
type ContentCreateRes struct {
	Item *ContentItem `json:"item"`
}
type ContentUpdateReq struct {
	g.Meta `path:"/admin/content/items/{contentNo}" method:"put" tags:"IDC Content" summary:"Update content" permission:"idc:admin:content:manage"`
	ContentWriteFields
}
type ContentUpdateRes struct {
	Item *ContentItem `json:"item"`
}
type VersionCommand struct {
	ContentNo string `json:"contentNo"`
	Version   int64  `json:"version"`
	Confirmed bool   `json:"confirmed"`
}
type ContentPublishReq struct {
	g.Meta `path:"/admin/content/items/{contentNo}/publications" method:"post" tags:"IDC Content" summary:"Publish content" permission:"idc:admin:content:manage"`
	VersionCommand
}
type ContentPublishRes struct {
	Item *ContentItem `json:"item"`
}
type ContentUnpublishReq struct {
	g.Meta `path:"/admin/content/items/{contentNo}/unpublications" method:"post" tags:"IDC Content" summary:"Take content offline" permission:"idc:admin:content:manage"`
	VersionCommand
}
type ContentUnpublishRes struct {
	Item *ContentItem `json:"item"`
}
type ContentRepublishReq struct {
	g.Meta `path:"/admin/content/items/{contentNo}/republications" method:"post" tags:"IDC Content" summary:"Republish content" permission:"idc:admin:content:manage"`
	VersionCommand
}
type ContentRepublishRes struct {
	Item *ContentItem `json:"item"`
}
type ContentDeleteReq struct {
	g.Meta `path:"/admin/content/items/{contentNo}" method:"delete" tags:"IDC Content" summary:"Delete content" permission:"idc:admin:content:manage"`
	VersionCommand
}
type ContentDeleteRes struct {
	Item *ContentItem `json:"item"`
}
type AudienceSearchReq struct {
	Keyword, Cursor string
	Limit           int `d:"20" v:"min:1|max:50"`
}
type AudienceCandidate struct {
	GroupNo string `json:"groupNo,omitempty"`
	Label   string `json:"label"`
}
type AudienceSearchRes struct {
	Items      []AudienceCandidate `json:"items"`
	NextCursor string              `json:"nextCursor"`
}
type AudienceCustomerGroupSearchReq struct {
	g.Meta `path:"/admin/content/audience-customer-groups" method:"get" tags:"IDC Content" summary:"Search customer-group audience" permission:"idc:admin:content:manage"`
	AudienceSearchReq
}
type AudienceCustomerGroupSearchRes AudienceSearchRes
type UploadCreateReq struct {
	g.Meta         `path:"/admin/content/items/{contentNo}/upload-sessions" method:"post" tags:"IDC Content" summary:"Create resumable download upload" permission:"idc:admin:content:manage"`
	ContentNo      string `json:"contentNo"`
	FileName       string `json:"fileName"`
	SizeBytes      int64  `json:"sizeBytes"`
	ContentVersion int64  `json:"contentVersion"`
}
type UploadSession struct {
	UploadNo       string `json:"uploadNo"`
	FileName       string `json:"fileName"`
	SizeBytes      int64  `json:"sizeBytes"`
	ConfirmedBytes int64  `json:"confirmedBytes"`
	ChunkSizeBytes int64  `json:"chunkSizeBytes"`
	ExpiresAt      int64  `json:"expiresAt"`
	Version        int64  `json:"version"`
}
type UploadCreateRes struct {
	Item *UploadSession `json:"item"`
}
type UploadDetailReq struct {
	g.Meta   `path:"/admin/content/upload-sessions/{uploadNo}" method:"get" tags:"IDC Content" summary:"Get resumable upload progress" permission:"idc:admin:content:manage"`
	UploadNo string `json:"uploadNo"`
}
type UploadDetailRes struct {
	Item *UploadSession `json:"item"`
}
type UploadChunkReq struct {
	g.Meta   `path:"/admin/content/upload-sessions/{uploadNo}/chunks/{offset}" method:"put" mime:"application/octet-stream" tags:"IDC Content" summary:"Upload one resumable chunk" permission:"idc:admin:content:manage"`
	UploadNo string `json:"uploadNo"`
	Offset   int64  `json:"offset"`
}
type UploadChunkRes struct {
	UploadNo       string `json:"uploadNo"`
	ConfirmedBytes int64  `json:"confirmedBytes"`
	NextOffset     int64  `json:"nextOffset"`
	Version        int64  `json:"version"`
	Complete       bool   `json:"complete"`
}
type UploadCompleteReq struct {
	g.Meta            `path:"/admin/content/upload-sessions/{uploadNo}/completion" method:"post" tags:"IDC Content" summary:"Complete resumable upload" permission:"idc:admin:content:manage"`
	UploadNo          string `json:"uploadNo"`
	ExpectedSizeBytes int64  `json:"expectedSizeBytes"`
	Version           int64  `json:"version"`
}
type UploadCompleteRes struct {
	Item *CandidateFile `json:"item"`
}
type UploadCancelReq struct {
	g.Meta    `path:"/admin/content/upload-sessions/{uploadNo}/cancellation" method:"post" tags:"IDC Content" summary:"Cancel resumable upload" permission:"idc:admin:content:manage"`
	UploadNo  string `json:"uploadNo"`
	Version   int64  `json:"version"`
	Confirmed bool   `json:"confirmed"`
}
type UploadCancelRes struct{}
type CandidateActionFields struct {
	ContentNo      string `json:"contentNo"`
	FileNo         string `json:"fileNo"`
	ContentVersion int64  `json:"contentVersion"`
	FileVersion    int64  `json:"fileVersion"`
	Confirmed      bool   `json:"confirmed"`
}
type CandidateCancelReq struct {
	g.Meta `path:"/admin/content/items/{contentNo}/file-candidates/{fileNo}/cancellation" method:"post" tags:"IDC Content" summary:"Cancel download candidate" permission:"idc:admin:content:manage"`
	CandidateActionFields
}
type CandidateCancelRes struct {
	Item *ContentItem `json:"item"`
}
type CandidateScanRetryReq struct {
	g.Meta `path:"/admin/content/items/{contentNo}/file-candidates/{fileNo}/scan-retries" method:"post" tags:"IDC Content" summary:"Retry candidate scan" permission:"idc:admin:content:manage"`
	CandidateActionFields
}
type CandidateScanRetryRes struct {
	Item *ContentItem `json:"item"`
}
type CandidateEnableReq struct {
	g.Meta `path:"/admin/content/items/{contentNo}/file-candidates/{fileNo}/enablements" method:"post" tags:"IDC Content" summary:"Enable download candidate" permission:"idc:admin:content:manage"`
	CandidateActionFields
}
type CandidateEnableRes struct {
	Item *ContentItem `json:"item"`
}
