// Customer segment actions expose simple scoped add/remove flows without internal codes.
package v1

import "github.com/gogf/gf/v2/frame/g"

type CustomerSegmentAddReq struct {
	g.Meta         `path:"/admin/customers/{customerNo}/segments" method:"post" tags:"IDC Customer Management" summary:"Add a customer group or tag" dc:"Assign an existing segment or create a named segment and assign it in one audited transaction." permission:"idc:admin:customer:update"`
	CustomerNo     string `json:"customerNo" in:"path" v:"required|max-length:40" dc:"Visible customer number" eg:"CUS_0123456789abcdef0123456789abcdef"`
	Kind           string `json:"kind" v:"required|in:group,tag" dc:"Segment kind: group or tag" eg:"tag"`
	SegmentNo      string `json:"segmentNo" v:"max-length:40" dc:"Existing segment number; omitted when creating by name" eg:"TAG_0123456789abcdef0123456789abcdef"`
	Name           string `json:"name" v:"max-length:128" dc:"Name for a new segment; required when no segment number is provided" eg:"Priority customer"`
	IdempotencyKey string `json:"idempotencyKey" v:"required|min-length:16|max-length:128" dc:"Stable retry key" eg:"550e8400-e29b-41d4-a716-446655440000"`
}
type CustomerSegmentAddRes struct {
	SegmentNo string `json:"segmentNo" dc:"Assigned segment number" eg:"TAG_0123456789abcdef0123456789abcdef"`
}

type CustomerSegmentRemoveReq struct {
	g.Meta         `path:"/admin/customers/{customerNo}/segments/{segmentNo}" method:"delete" tags:"IDC Customer Management" summary:"Remove a customer group or tag" dc:"Revoke one visible customer assignment without deleting the shared definition or history." permission:"idc:admin:customer:update"`
	CustomerNo     string `json:"customerNo" in:"path" v:"required|max-length:40" dc:"Visible customer number" eg:"CUS_0123456789abcdef0123456789abcdef"`
	SegmentNo      string `json:"segmentNo" in:"path" v:"required|max-length:40" dc:"Assigned segment number" eg:"TAG_0123456789abcdef0123456789abcdef"`
	Kind           string `json:"kind" v:"required|in:group,tag" dc:"Segment kind: group or tag" eg:"tag"`
	IdempotencyKey string `json:"idempotencyKey" v:"required|min-length:16|max-length:128" dc:"Stable retry key" eg:"550e8400-e29b-41d4-a716-446655440000"`
}
type CustomerSegmentRemoveRes struct{}
