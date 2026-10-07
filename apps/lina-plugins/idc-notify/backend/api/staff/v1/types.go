// This file defines shared safe projections for staff WeCom binding APIs.

package v1

// WeComBindingItem exposes binding state without exposing the provider UserID.
type WeComBindingItem struct {
	Bound   bool   `json:"bound" dc:"Whether the current staff member has an active WeCom binding" eg:"true"`
	Status  string `json:"status" dc:"Current binding lifecycle status" eg:"active"`
	Version int64  `json:"version" dc:"Binding optimistic-concurrency version" eg:"2"`
	BoundAt int64  `json:"boundAt" dc:"Binding time as a Unix timestamp in milliseconds, or zero when never active" eg:"1786752000000"`
	Reused  bool   `json:"reused" dc:"Whether this response reused an already completed operation" eg:"false"`
}
