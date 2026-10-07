package v1

import "github.com/gogf/gf/v2/frame/g"

type CategoryLabel struct {
	NameZhCN string `json:"nameZhCN"`
	NameEnUS string `json:"nameEnUS"`
}

type ContentSummary struct {
	ContentNo   string         `json:"contentNo"`
	PublicPath  string         `json:"publicPath"`
	Type        string         `json:"type"`
	Category    *CategoryLabel `json:"category,omitempty"`
	Title       string         `json:"title"`
	Summary     string         `json:"summary,omitempty"`
	PublishedAt int64          `json:"publishedAt"`
	UpdatedAt   int64          `json:"updatedAt"`
}

type PublicFile struct {
	FileName  string `json:"fileName"`
	MediaType string `json:"mediaType"`
	SizeBytes int64  `json:"sizeBytes"`
	SHA256    string `json:"sha256"`
}

type ContentDetail struct {
	ContentSummary
	Body           string      `json:"body"`
	SEOTitle       string      `json:"seoTitle,omitempty"`
	SEODescription string      `json:"seoDescription,omitempty"`
	File           *PublicFile `json:"file,omitempty"`
}

type ContentListReq struct {
	g.Meta `path:"/public/content/items" method:"get" tags:"IDC Public Content" summary:"List public content"`
	Type   string `json:"type"`
	Query  string `json:"query"`
	Cursor string `json:"cursor"`
	Limit  int    `json:"limit" d:"25" v:"min:1|max:100"`
}
type ContentListRes struct {
	Items      []*ContentSummary `json:"items"`
	NextCursor string            `json:"nextCursor"`
}
type ContentDetailReq struct {
	g.Meta    `path:"/public/content/items/{contentNo}" method:"get" tags:"IDC Public Content" summary:"Get public content"`
	ContentNo string `json:"contentNo"`
}
type ContentDetailRes struct {
	Item *ContentDetail `json:"item"`
}
type DownloadTicketCreateReq struct {
	g.Meta    `path:"/public/content/items/{contentNo}/download-tickets" method:"post" tags:"IDC Public Content" summary:"Create a ten-minute public download ticket"`
	ContentNo string `json:"contentNo"`
}
type DownloadTicketCreateRes struct {
	Token     string `json:"token"`
	ExpiresAt int64  `json:"expiresAt"`
}
type DownloadOpenReq struct {
	g.Meta `path:"/public/content/downloads/{token}" method:"get" tags:"IDC Public Content" summary:"Download with a short-lived ticket"`
	Token  string `json:"token"`
}
type DownloadOpenRes struct{}
