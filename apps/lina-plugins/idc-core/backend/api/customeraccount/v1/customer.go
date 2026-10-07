package v1

import "github.com/gogf/gf/v2/frame/g"

type ProfileItem struct {
	CustomerCode     int64  `json:"customerCode"`
	Type             string `json:"type"`
	Status           string `json:"status"`
	DisplayName      string `json:"displayName"`
	CountryCode      string `json:"countryCode"`
	PreferredLocale  string `json:"preferredLocale"`
	Verification     string `json:"verification"`
	LegalNameMasked  string `json:"legalNameMasked"`
	IdentifierMasked string `json:"identifierMasked"`
	Version          int64  `json:"version"`
}

type ProfileGetReq struct {
	g.Meta `path:"/account/customer/profile" method:"get" tags:"IDC Customer Account" summary:"Read my customer profile"`
}
type ProfileGetRes struct {
	Item *ProfileItem `json:"item"`
}

type ProfileUpdateReq struct {
	g.Meta          `path:"/account/customer/profile" method:"put" tags:"IDC Customer Account" summary:"Update my ordinary profile"`
	DisplayName     string `json:"displayName" v:"required|max-length:256"`
	CountryCode     string `json:"countryCode" v:"required|min-length:2|max-length:2"`
	PreferredLocale string `json:"preferredLocale" v:"required|max-length:16"`
	ExpectedVersion int64  `json:"expectedVersion" v:"required|min:1"`
	IdempotencyKey  string `json:"idempotencyKey" v:"required|min-length:16|max-length:128"`
}
type ProfileUpdateRes struct {
	Item *ProfileItem `json:"item"`
}

type ContactItem struct {
	ContactNo        string `json:"contactNo"`
	DisplayName      string `json:"displayName"`
	Email            string `json:"email"`
	EmailVerified    bool   `json:"emailVerified"`
	PhoneCountryCode string `json:"phoneCountryCode"`
	Phone            string `json:"phone"`
	PhoneVerified    bool   `json:"phoneVerified"`
	PreferredLocale  string `json:"preferredLocale"`
	Status           string `json:"status"`
	Version          int64  `json:"version"`
	CreatedAt        int64  `json:"createdAt"`
	UpdatedAt        int64  `json:"updatedAt"`
}

type ContactListReq struct {
	g.Meta `path:"/account/customer/contacts" method:"get" tags:"IDC Customer Account" summary:"List my notification contacts"`
}
type ContactListRes struct {
	Items []*ContactItem `json:"items"`
}

type ContactSaveFields struct {
	DisplayName      string `json:"displayName" v:"required|max-length:256"`
	Email            string `json:"email" v:"max-length:320"`
	PhoneCountryCode string `json:"phoneCountryCode" v:"max-length:8"`
	Phone            string `json:"phone" v:"max-length:32"`
	PreferredLocale  string `json:"preferredLocale" v:"required|max-length:16"`
	IdempotencyKey   string `json:"idempotencyKey" v:"required|min-length:16|max-length:128"`
}

type ContactCreateReq struct {
	g.Meta `path:"/account/customer/contacts" method:"post" tags:"IDC Customer Account" summary:"Create my notification contact"`
	ContactSaveFields
}
type ContactCreateRes struct {
	Item *ContactItem `json:"item"`
}

type ContactUpdateReq struct {
	g.Meta          `path:"/account/customer/contacts/{contactNo}" method:"put" tags:"IDC Customer Account" summary:"Update my notification contact"`
	ContactNo       string `json:"contactNo" in:"path" v:"required|max-length:40"`
	ExpectedVersion int64  `json:"expectedVersion" v:"required|min:1"`
	ContactSaveFields
}
type ContactUpdateRes struct {
	Item *ContactItem `json:"item"`
}

type ContactArchiveReq struct {
	g.Meta          `path:"/account/customer/contacts/{contactNo}/archive" method:"post" tags:"IDC Customer Account" summary:"Archive my notification contact"`
	ContactNo       string `json:"contactNo" in:"path" v:"required|max-length:40"`
	ExpectedVersion int64  `json:"expectedVersion" v:"required|min:1"`
	IdempotencyKey  string `json:"idempotencyKey" v:"required|min-length:16|max-length:128"`
}
type ContactArchiveRes struct {
	Item *ContactItem `json:"item"`
}

type AddressItem struct {
	AddressNo      string `json:"addressNo"`
	Type           string `json:"type"`
	Label          string `json:"label"`
	RecipientName  string `json:"recipientName"`
	RecipientPhone string `json:"recipientPhone"`
	CountryCode    string `json:"countryCode"`
	Province       string `json:"province"`
	City           string `json:"city"`
	District       string `json:"district"`
	AddressLine1   string `json:"addressLine1"`
	AddressLine2   string `json:"addressLine2"`
	PostalCode     string `json:"postalCode"`
	Default        bool   `json:"default"`
	Status         string `json:"status"`
	Version        int64  `json:"version"`
	CreatedAt      int64  `json:"createdAt"`
	UpdatedAt      int64  `json:"updatedAt"`
}

type AddressListReq struct {
	g.Meta `path:"/account/customer/addresses" method:"get" tags:"IDC Customer Account" summary:"List my addresses"`
}
type AddressListRes struct {
	Items []*AddressItem `json:"items"`
}

type AddressSaveFields struct {
	Type           string `json:"type" v:"required|in:contact,billing,registered,service"`
	Label          string `json:"label" v:"max-length:128"`
	RecipientName  string `json:"recipientName" v:"max-length:256"`
	RecipientPhone string `json:"recipientPhone" v:"max-length:32"`
	CountryCode    string `json:"countryCode" v:"required|min-length:2|max-length:2"`
	Province       string `json:"province" v:"max-length:128"`
	City           string `json:"city" v:"max-length:128"`
	District       string `json:"district" v:"max-length:128"`
	AddressLine1   string `json:"addressLine1" v:"required|max-length:512"`
	AddressLine2   string `json:"addressLine2" v:"max-length:512"`
	PostalCode     string `json:"postalCode" v:"max-length:32"`
	Default        bool   `json:"default"`
	IdempotencyKey string `json:"idempotencyKey" v:"required|min-length:16|max-length:128"`
}

type AddressCreateReq struct {
	g.Meta `path:"/account/customer/addresses" method:"post" tags:"IDC Customer Account" summary:"Create my address"`
	AddressSaveFields
}
type AddressCreateRes struct {
	Item *AddressItem `json:"item"`
}

type AddressUpdateReq struct {
	g.Meta          `path:"/account/customer/addresses/{addressNo}" method:"put" tags:"IDC Customer Account" summary:"Update my address"`
	AddressNo       string `json:"addressNo" in:"path" v:"required|max-length:40"`
	ExpectedVersion int64  `json:"expectedVersion" v:"required|min:1"`
	AddressSaveFields
}
type AddressUpdateRes struct {
	Item *AddressItem `json:"item"`
}

type AddressArchiveReq struct {
	g.Meta          `path:"/account/customer/addresses/{addressNo}/archive" method:"post" tags:"IDC Customer Account" summary:"Archive my address"`
	AddressNo       string `json:"addressNo" in:"path" v:"required|max-length:40"`
	ExpectedVersion int64  `json:"expectedVersion" v:"required|min:1"`
	IdempotencyKey  string `json:"idempotencyKey" v:"required|min-length:16|max-length:128"`
}
type AddressArchiveRes struct {
	Item *AddressItem `json:"item"`
}

type ClosureFindingItem struct {
	Code  string `json:"code"`
	Count int    `json:"count"`
}
type ClosureItem struct {
	RequestNo   string                `json:"requestNo"`
	Status      string                `json:"status"`
	Findings    []*ClosureFindingItem `json:"findings"`
	RequestedAt int64                 `json:"requestedAt"`
}
type ClosureRequestReq struct {
	g.Meta          `path:"/account/customer/closure-requests" method:"post" tags:"IDC Customer Account" summary:"Request account closure"`
	ExpectedVersion int64  `json:"expectedVersion" v:"required|min:1"`
	Confirmed       bool   `json:"confirmed" v:"required"`
	IdempotencyKey  string `json:"idempotencyKey" v:"required|min-length:16|max-length:128"`
}
type ClosureRequestRes struct {
	Item *ClosureItem `json:"item"`
}

type DataExportProofReq struct {
	g.Meta          `path:"/account/customer/data-exports/proof" method:"post" tags:"IDC Customer Account" summary:"Prove a recent login for my data export"`
	ExpectedVersion int64  `json:"expectedVersion" v:"required|min:1"`
	IdempotencyKey  string `json:"idempotencyKey" v:"required|min-length:16|max-length:128"`
	Confirmed       bool   `json:"confirmed" v:"required"`
}
type DataExportProofRes struct {
	ProofToken string `json:"proofToken"`
	ExpiresAt  int64  `json:"expiresAt"`
}

type DataExportRequestReq struct {
	g.Meta          `path:"/account/customer/data-exports" method:"post" tags:"IDC Customer Account" summary:"Request my private data export"`
	ExpectedVersion int64  `json:"expectedVersion" v:"required|min:1"`
	IdempotencyKey  string `json:"idempotencyKey" v:"required|min-length:16|max-length:128"`
	Confirmed       bool   `json:"confirmed" v:"required"`
	ProofToken      string `json:"proofToken" v:"required|max-length:4096"`
}
type DataExportRequestRes struct {
	RequestNo   string `json:"requestNo"`
	TaskNo      string `json:"taskNo"`
	RequestedAt int64  `json:"requestedAt"`
}

type DataExportStatusReq struct {
	g.Meta    `path:"/account/customer/data-exports/{requestNo}" method:"get" tags:"IDC Customer Account" summary:"Read my data export status"`
	RequestNo string `json:"requestNo" in:"path" v:"required|max-length:40"`
}
type DataExportStatusRes struct {
	RequestNo   string `json:"requestNo"`
	Status      string `json:"status"`
	RequestedAt int64  `json:"requestedAt"`
	ExpiresAt   int64  `json:"expiresAt"`
}

type DataExportTicketReq struct {
	g.Meta    `path:"/account/customer/data-exports/{requestNo}/download-ticket" method:"post" tags:"IDC Customer Account" summary:"Create a private data export download ticket"`
	RequestNo string `json:"requestNo" in:"path" v:"required|max-length:40"`
}
type DataExportTicketRes struct {
	Token     string `json:"token"`
	ExpiresAt int64  `json:"expiresAt"`
}

type DataExportOpenReq struct {
	g.Meta `path:"/account/customer/data-export-downloads/{token}" method:"get" tags:"IDC Customer Account" summary:"Download my private data export"`
	Token  string `json:"token" in:"path" v:"required|max-length:128"`
}
type DataExportOpenRes struct{}
