// This file defines the bounded marketing segment detail contract.

package v1

import "github.com/gogf/gf/v2/frame/g"

type MarketingSegmentDetailReq struct {
	g.Meta    `path:"/admin/marketing/segments/{segmentNo}" method:"get" tags:"IDC Marketing Management" summary:"Get marketing segment detail" dc:"Return one bounded segment definition with typed conditions and recent immutable snapshots." permission:"idc:admin:marketing:segment:query"`
	SegmentNo string `json:"segmentNo" v:"required|max-length:128" dc:"Stable marketing segment business number" eg:"segment-loyal"`
}

type MarketingSegmentDetailRes struct {
	Segment     *MarketingSegmentSummaryItem   `json:"segment" dc:"Segment identity, lifecycle, and operational counts" eg:"{}"`
	Description *MarketingBilingualDescription `json:"description" dc:"Bilingual segment description" eg:"{}"`
	Conditions  []*MarketingSegmentCondition   `json:"conditions" dc:"At most 256 typed segment conditions in evaluation order" eg:"[]"`
	Snapshots   []*MarketingSegmentSnapshot    `json:"snapshots" dc:"At most twenty recent immutable membership snapshots" eg:"[]"`
}
