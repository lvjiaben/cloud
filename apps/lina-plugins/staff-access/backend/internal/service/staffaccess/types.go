package staffaccess

import "time"

type Grant struct {
	GrantNo        string     `json:"grantNo"`
	UserID         int64      `json:"userId"`
	Username       string     `json:"username"`
	Nickname       string     `json:"nickname"`
	CapabilityCode string     `json:"capabilityCode"`
	ScopeType      string     `json:"scopeType"`
	ScopeKey       string     `json:"scopeKey"`
	Status         int        `json:"status"`
	Version        int64      `json:"version"`
	CreatedAt      time.Time  `json:"createdAt"`
	UpdatedAt      time.Time  `json:"updatedAt"`
	ValidFrom      *time.Time `json:"validFrom"`
	ValidUntil     *time.Time `json:"validUntil"`
}

type GrantPage struct {
	Items []Grant `json:"items"`
	Total int64   `json:"total"`
}

type SaveInput struct {
	UserID          int64
	CapabilityCode  string
	ScopeType       string
	ScopeKey        string
	Enabled         bool
	Reason          string
	ExpectedVersion int64
	ValidFrom       *time.Time
	ValidUntil      *time.Time
}

type UserCandidate struct {
	UserID   int64  `json:"userId"`
	Username string `json:"username"`
	Nickname string `json:"nickname"`
}
