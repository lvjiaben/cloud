// This file defines the safe electronic-contract template management projections.

package v1

// ElectronicContractTemplateItem is one latest or exact immutable-version projection.
type ElectronicContractTemplateItem struct {
	TemplateNo       string `json:"templateNo" dc:"Installation-local opaque logical template number" eg:"ctpl_0123456789abcdef0123456789abcdef"`
	VersionNo        int64  `json:"versionNo" dc:"Installation-local immutable version number" eg:"1"`
	Name             string `json:"name" dc:"Template display name" eg:"IDC service agreement"`
	ScopeCode        string `json:"scopeCode" dc:"personal, enterprise, all_customers, or empty while Tencent settings are incomplete" eg:"enterprise"`
	SigningOrderCode string `json:"signingOrderCode" dc:"customer_first, merchant_first, parallel, or empty while Tencent settings are incomplete" eg:"customer_first"`
	LanguageCode     string `json:"languageCode" dc:"zh-CN, en-US, or empty while Tencent settings are incomplete" eg:"zh-CN"`
	StatusCode       string `json:"statusCode" dc:"Local effective status including provider_disabled and provider_offline overrides" eg:"enabled"`
	OriginCode       string `json:"originCode" dc:"local or tencent_synced" eg:"tencent_synced"`
	ProviderState    string `json:"providerState,omitempty" dc:"Safe Tencent state: active, disabled, or offline" eg:"active"`
	ProviderVersion  string `json:"providerVersion,omitempty" dc:"Tencent version when supplied by the verified API" eg:"20260814001"`
	UsedCount        int64  `json:"usedCount" dc:"Number of contracts already bound to this immutable version" eg:"0"`
	LockVersion      int64  `json:"lockVersion" dc:"Optimistic local mutation version" eg:"1"`
	UpdatedAt        int64  `json:"updatedAt" dc:"Update time as Unix milliseconds" eg:"1786665600000"`
}

// ElectronicContractTemplateContentItem is the safe current immutable body projection.
type ElectronicContractTemplateContentItem struct {
	ContentRevisionNo    string `json:"contentRevisionNo" dc:"Opaque immutable content revision" eg:"ctpc_0123456789abcdef0123456789abcdef"`
	RevisionNo           int64  `json:"revisionNo" dc:"Monotonic authoring revision number" eg:"2"`
	SourceModeCode       string `json:"sourceModeCode" dc:"uploaded_file or online_editor" eg:"online_editor"`
	SourceMediaType      string `json:"sourceMediaType" dc:"Verified source media type" eg:"text/html"`
	SourceByteSize       int64  `json:"sourceByteSize" dc:"Verified source byte size" eg:"4096"`
	SourceSHA256         string `json:"sourceSha256" dc:"Lowercase source SHA-256" eg:"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"`
	ConversionStatusCode string `json:"conversionStatusCode" dc:"source_saved or ready" eg:"ready"`
	PDFSHA256            string `json:"pdfSha256,omitempty" dc:"Lowercase immutable PDF SHA-256 when ready" eg:"abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789"`
	PDFByteSize          int64  `json:"pdfByteSize,omitempty" dc:"Immutable PDF byte size when ready" eg:"8192"`
	PDFPageCount         int    `json:"pdfPageCount,omitempty" dc:"Validated PDF page count when ready" eg:"3"`
	CreatedAt            int64  `json:"createdAt" dc:"Revision creation time as Unix milliseconds" eg:"1786665600000"`
}

// ElectronicContractTemplateFieldItem is one safe normalized PDF placement.
type ElectronicContractTemplateFieldItem struct {
	FieldNo       string `json:"fieldNo" dc:"Opaque immutable field number" eg:"ctpf_0123456789abcdef0123456789abcdef"`
	SequenceNo    int    `json:"sequenceNo" dc:"One-based display order" eg:"1"`
	FieldTypeCode string `json:"fieldTypeCode" dc:"text_variable, signature, enterprise_seal, or signing_date" eg:"signature"`
	PartyCode     string `json:"partyCode" dc:"customer or merchant" eg:"customer"`
	VariableCode  string `json:"variableCode,omitempty" dc:"Fixed variable code for text_variable" eg:"customer_name"`
	PageNo        int    `json:"pageNo" dc:"One-based PDF page" eg:"1"`
	PositionXPPM  int    `json:"positionXPPM" dc:"Left position in millionths of page width" eg:"100000"`
	PositionYPPM  int    `json:"positionYPPM" dc:"Top position in millionths of page height" eg:"700000"`
	WidthPPM      int    `json:"widthPPM" dc:"Width in millionths of page width" eg:"250000"`
	HeightPPM     int    `json:"heightPPM" dc:"Height in millionths of page height" eg:"80000"`
}

// ElectronicContractTemplateProviderComponentItem is one safe Tencent position.
type ElectronicContractTemplateProviderComponentItem struct {
	ComponentRef string `json:"componentRef" dc:"Opaque provider position reference used only when saving the selection" eg:"component-1"`
	Name         string `json:"name" dc:"Provider position display name" eg:"Service details"`
	TypeCode     string `json:"typeCode" dc:"Safe provider position type" eg:"DYNAMIC_TABLE"`
	Required     bool   `json:"required" dc:"Whether Tencent requires this position" eg:"true"`
	RecipientRef string `json:"recipientRef,omitempty" dc:"Opaque owner role when Tencent supplies one" eg:"recipient-1"`
	ValueCode    string `json:"valueCode,omitempty" dc:"Confirmed fixed contract value code" eg:"service_lines"`
}

// ElectronicContractTemplateProviderRecipientItem is one safe Tencent signer role.
type ElectronicContractTemplateProviderRecipientItem struct {
	RecipientRef       string `json:"recipientRef" dc:"Opaque provider signer-role reference used only when saving the selection" eg:"recipient-1"`
	SequenceNo         int    `json:"sequenceNo" dc:"Tencent template signer sequence, starting at one" eg:"1"`
	TypeCode           string `json:"typeCode" dc:"INDIVIDUAL, ENTERPRISE, or PROMOTER" eg:"ENTERPRISE"`
	RoleName           string `json:"roleName" dc:"Provider signer-role display name" eg:"Customer"`
	RequireSign        bool   `json:"requireSign" dc:"Whether Tencent marks this role as required" eg:"true"`
	RoutingOrder       *int64 `json:"routingOrder" dc:"Tencent signing order; null blocks confirmation" eg:"1"`
	IsPromoter         bool   `json:"isPromoter" dc:"Whether Tencent marks this role as the flow promoter" eg:"false"`
	SignComponentCount int    `json:"signComponentCount" dc:"Verified signing positions owned by this role" eg:"1"`
	PartyCode          string `json:"partyCode,omitempty" dc:"Confirmed customer or merchant mapping" eg:"customer"`
}

// ElectronicContractTemplateEditorItem is the exact current authoring projection.
type ElectronicContractTemplateEditorItem struct {
	Template             *ElectronicContractTemplateItem                   `json:"template" dc:"Exact template metadata" eg:"{}"`
	Content              *ElectronicContractTemplateContentItem            `json:"content,omitempty" dc:"Current immutable body revision" eg:"{}"`
	Fields               []ElectronicContractTemplateFieldItem             `json:"fields" dc:"Current immutable placement set" eg:"[]"`
	ProviderComponents   []ElectronicContractTemplateProviderComponentItem `json:"providerComponents" dc:"Safe Tencent positions for explicit mapping" eg:"[]"`
	ProviderRecipients   []ElectronicContractTemplateProviderRecipientItem `json:"providerRecipients" dc:"Safe Tencent signer roles for explicit mapping" eg:"[]"`
	ProviderMappingSaved bool                                              `json:"providerMappingSaved" dc:"Whether the exact provider version has a confirmed mapping" eg:"true"`
	OnlineHTML           string                                            `json:"onlineHtml,omitempty" dc:"Sanitized current online-editor source; absent for uploaded files" eg:"<p>Service agreement</p>"`
	PreviewConfirmed     bool                                              `json:"previewConfirmed" dc:"Whether an administrator explicitly confirmed the exact current PDF" eg:"true"`
	EnableReady          bool                                              `json:"enableReady" dc:"Whether server-side content and signing-position gates currently pass" eg:"true"`
	EnableReasonCode     string                                            `json:"enableReasonCode,omitempty" dc:"Safe reason when enableReady is false" eg:"preview_not_confirmed"`
	SubmissionAvailable  bool                                              `json:"submissionAvailable" dc:"Whether the verified Tencent contract supports local template submission" eg:"false"`
	SubmissionReasonCode string                                            `json:"submissionReasonCode,omitempty" dc:"Safe reason when submission is unavailable" eg:"tencent_template_submission_unavailable"`
}

// ElectronicContractTemplateFieldInput carries one browser drag placement without accepting an ID.
type ElectronicContractTemplateFieldInput struct {
	FieldTypeCode string `json:"fieldTypeCode" v:"required|max-length:32" dc:"text_variable, signature, enterprise_seal, or signing_date" eg:"signature"`
	PartyCode     string `json:"partyCode" v:"required|max-length:16" dc:"customer or merchant" eg:"customer"`
	VariableCode  string `json:"variableCode" v:"max-length:64" dc:"Fixed variable code only for text_variable" eg:"customer_name"`
	PageNo        int    `json:"pageNo" v:"required|min:1|max:500" dc:"One-based PDF page" eg:"1"`
	PositionXPPM  int    `json:"positionXPPM" v:"min:0|max:1000000" dc:"Left position in millionths of page width" eg:"100000"`
	PositionYPPM  int    `json:"positionYPPM" v:"min:0|max:1000000" dc:"Top position in millionths of page height" eg:"700000"`
	WidthPPM      int    `json:"widthPPM" v:"required|min:1|max:1000000" dc:"Width in millionths of page width" eg:"250000"`
	HeightPPM     int    `json:"heightPPM" v:"required|min:1|max:1000000" dc:"Height in millionths of page height" eg:"80000"`
}

// ElectronicContractTemplateSyncResult summarizes one complete explicit Tencent snapshot.
type ElectronicContractTemplateSyncResult struct {
	ProviderTotal int   `json:"providerTotal" dc:"Templates in the complete Tencent snapshot" eg:"12"`
	NewVersions   int   `json:"newVersions" dc:"New immutable local mappings created" eg:"2"`
	MarkedOffline int   `json:"markedOffline" dc:"Previously synchronized templates no longer returned" eg:"1"`
	SyncedAt      int64 `json:"syncedAt" dc:"Local synchronization commit time as Unix milliseconds" eg:"1786665600000"`
}
