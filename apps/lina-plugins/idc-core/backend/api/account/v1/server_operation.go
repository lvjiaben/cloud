package v1

import (
	"encoding/json"

	"github.com/gogf/gf/v2/frame/g"
)

type ServerOperationSummary struct {
	OperationNo string `json:"operationNo" dc:"Stable server operation number" eg:"SOP_0123456789abcdef0123456789abcdef"`
	Action      string `json:"action" dc:"Stable server operation action code" eg:"reinstall"`
	Status      string `json:"status" dc:"Operation status: ready, dispatching, running, succeeded, failed, result_unknown, or manual_review" eg:"succeeded"`
	CreatedAt   int64  `json:"createdAt" dc:"Creation time as a Unix timestamp in milliseconds" eg:"1787184000000"`
}

type CustomerServer struct {
	ServiceNo            string                  `json:"serviceNo" dc:"Stable service instance number owned by the current customer" eg:"SVC_SERVER_0001"`
	CustomerAlias        string                  `json:"customerAlias" dc:"Customer-maintained server display alias" eg:"Production database"`
	CustomerRemark       string                  `json:"customerRemark" dc:"Customer-maintained non-sensitive server note" eg:"Used by the reporting team"`
	AutoRenewEnabled     bool                    `json:"autoRenewEnabled" dc:"Whether same-currency balance automatic renewal is enabled" eg:"true"`
	RenewalCycles        int                     `json:"renewalCycles" dc:"Number of cycles requested by the current automatic renewal policy, or zero when no policy exists" eg:"1"`
	ActiveRefundNo       string                  `json:"activeRefundNo" dc:"Current refund that locks this service, or empty when no refund is active" eg:"RFD_0123456789abcdef"`
	ProductNameZhCN      string                  `json:"productNameZhCN" dc:"Frozen Simplified Chinese product name" eg:"Production server"`
	ProductNameEnUS      string                  `json:"productNameEnUS" dc:"Frozen English product name" eg:"Production Cloud Server"`
	ProductType          string                  `json:"productType" dc:"Frozen product type code" eg:"cloud_server"`
	EntitlementStatus    string                  `json:"entitlementStatus" dc:"Entitlement status code" eg:"active"`
	RuntimeStatus        string                  `json:"runtimeStatus" dc:"Observed runtime status code" eg:"running"`
	ProviderTaskStatus   string                  `json:"providerTaskStatus" dc:"Latest Provider task status code" eg:"succeeded"`
	PeriodUnit           string                  `json:"periodUnit" dc:"Billing period unit" eg:"month"`
	PeriodCount          int                     `json:"periodCount" dc:"Number of billing period units" eg:"1"`
	EntitlementStartedAt *int64                  `json:"entitlementStartedAt" dc:"Entitlement start time as a Unix timestamp in milliseconds, or null" eg:"1787184000000"`
	EntitlementExpiresAt *int64                  `json:"entitlementExpiresAt" dc:"Entitlement expiry time as a Unix timestamp in milliseconds, or null" eg:"1818720000000"`
	UpdatedAt            int64                   `json:"updatedAt" dc:"Latest projection update time as a Unix timestamp in milliseconds" eg:"1787184000000"`
	Version              int64                   `json:"version" dc:"Current optimistic-lock version" eg:"8"`
	AvailableActions     []string                `json:"availableActions" dc:"Actions currently allowed by entitlement, runtime state, and the latest capability snapshot" eg:"[\"stop\",\"reboot\",\"reinstall\"]"`
	CapabilityObservedAt *int64                  `json:"capabilityObservedAt" dc:"Capability snapshot observation time as a Unix timestamp in milliseconds, or null" eg:"1787184000000"`
	ActiveOperation      *ServerOperationSummary `json:"activeOperation" dc:"Current mutually exclusive operation, or null" eg:"{}"`
}

type CustomerServerConfigurationItem struct {
	ComponentType   string          `json:"componentType" dc:"Frozen configuration component type" eg:"cpu"`
	GroupNameZhCN   string          `json:"groupNameZhCN" dc:"Frozen Simplified Chinese configuration group name" eg:"Processor"`
	GroupNameEnUS   string          `json:"groupNameEnUS" dc:"Frozen English configuration group name" eg:"CPU"`
	OptionNameZhCN  string          `json:"optionNameZhCN" dc:"Frozen Simplified Chinese option name" eg:"4 cores"`
	OptionNameEnUS  string          `json:"optionNameEnUS" dc:"Frozen English option name" eg:"4 cores"`
	NormalizedValue json.RawMessage `json:"normalizedValue" dc:"Frozen normalized configuration value" eg:"4"`
	Quantity        int             `json:"quantity" dc:"Frozen configuration quantity" eg:"1"`
}

type CustomerServerOperationHistory struct {
	OperationNo                 string `json:"operationNo" dc:"Stable server operation number" eg:"SOP_0123456789abcdef0123456789abcdef"`
	Action                      string `json:"action" dc:"Requested server action" eg:"reinstall"`
	Status                      string `json:"status" dc:"Current operation status code" eg:"succeeded"`
	FailureCode                 string `json:"failureCode" dc:"Stable redacted failure code, or empty" eg:""`
	CreatedAt                   int64  `json:"createdAt" dc:"Creation time as a Unix timestamp in milliseconds" eg:"1787184000000"`
	UpdatedAt                   int64  `json:"updatedAt" dc:"Latest update time as a Unix timestamp in milliseconds" eg:"1787184001000"`
	CompletedAt                 *int64 `json:"completedAt" dc:"Completion time as a Unix timestamp in milliseconds, or null" eg:"1787184001000"`
	ConfidentialOutputAvailable bool   `json:"confidentialOutputAvailable" dc:"Whether the current customer may actively redeem one confidential output" eg:"true"`
}

type CustomerServerReinstallImage struct {
	Reference  string `json:"reference" dc:"Provider image reference from the last complete channel snapshot" eg:"101"`
	Name       string `json:"name" dc:"Provider image display name" eg:"Ubuntu 24.04 LTS"`
	Group      string `json:"group" dc:"Provider image group" eg:"Linux"`
	OSType     string `json:"osType" dc:"Operating-system family" eg:"Linux"`
	Arch       string `json:"arch" dc:"Image CPU architecture" eg:"x86_64"`
	BootMode   string `json:"bootMode" dc:"Image boot mode" eg:"UEFI"`
	Selectable bool   `json:"selectable" dc:"Whether the image status is explicitly supported for reinstall" eg:"true"`
}

type CustomerServerAccessory struct {
	ObjectNo   string          `json:"objectNo" dc:"Stable local accessory number" eg:"ACO_0123456789abcdef"`
	Kind       string          `json:"kind" dc:"Accessory kind" eg:"snapshot"`
	Name       string          `json:"name" dc:"Safe accessory display name" eg:"Before upgrade"`
	State      string          `json:"state" dc:"Current normalized accessory state" eg:"available"`
	Projection json.RawMessage `json:"projection" dc:"Kind-specific safe projection without Provider identifiers" eg:"{}"`
}

type CustomerServerDetail struct {
	Server                    *CustomerServer                    `json:"server" dc:"Current-customer server summary" eg:"{}"`
	Configuration             []*CustomerServerConfigurationItem `json:"configuration" dc:"Frozen configuration snapshot items" eg:"[]"`
	Operations                []*CustomerServerOperationHistory  `json:"operations" dc:"Bounded newest-first server operation history" eg:"[]"`
	ReinstallImages           []*CustomerServerReinstallImage    `json:"reinstallImages" dc:"Bounded field-whitelisted reinstall image choices" eg:"[]"`
	ReinstallImagesObservedAt *int64                             `json:"reinstallImagesObservedAt" dc:"Image snapshot observation time as a Unix timestamp in milliseconds, or null" eg:"1787184000000"`
	Accessories               []*CustomerServerAccessory         `json:"accessories" dc:"Bounded current accessory directory without Provider identifiers" eg:"[]"`
	AccessoriesObservedAt     *int64                             `json:"accessoriesObservedAt" dc:"Accessory snapshot observation time in Unix milliseconds, or null" eg:"1787184000000"`
	AccessoriesExpiresAt      *int64                             `json:"accessoriesExpiresAt" dc:"Accessory snapshot freshness deadline in Unix milliseconds, or null" eg:"1787187600000"`
}

type ServerListReq struct {
	g.Meta `path:"/account/servers" method:"get" tags:"IDC Customer Servers" summary:"List my servers" dc:"Return one bounded cursor page of service instances owned by the current customer without per-row Provider calls."`
	Limit  int    `json:"limit" in:"query" v:"min:1|max:50" d:"20" dc:"Maximum servers returned in this page" eg:"20"`
	Cursor string `json:"cursor" in:"query" v:"max-length:40" dc:"Stable service-number cursor from the previous page, or empty" eg:"SVC_SERVER_0001"`
}

type ServerListRes struct {
	Items      []*CustomerServer `json:"items" dc:"Current-customer server summaries" eg:"[]"`
	NextCursor string            `json:"nextCursor" dc:"Next-page cursor, or empty" eg:""`
}

type ServerDetailReq struct {
	g.Meta    `path:"/account/servers/{serviceNo}" method:"get" tags:"IDC Customer Servers" summary:"Get my server" dc:"Return the current customer's owned server, configuration, operation history, and latest complete reinstall image projection."`
	ServiceNo string `json:"serviceNo" in:"path" v:"required|max-length:40" dc:"Stable owned service instance number" eg:"SVC_SERVER_0001"`
}

type ServerDetailRes struct {
	Detail *CustomerServerDetail `json:"detail" dc:"Owned server detail projection" eg:"{}"`
}

type ServerResizeSelection struct {
	OptionNo string `json:"optionNo" v:"required|max-length:40" dc:"Published catalog option number" eg:"OPT_CPU_4"`
	Quantity int    `json:"quantity" v:"required|min:1|max:100000" dc:"Selected option quantity" eg:"1"`
}

type ServerResizeOption struct {
	OptionNo        string `json:"optionNo" dc:"Published option number" eg:"Example"`
	ComponentType   string `json:"componentType" dc:"Catalog component type" eg:"Example"`
	NameZhCN        string `json:"nameZhCN" dc:"Chinese display name" eg:"Example"`
	NameEnUS        string `json:"nameEnUS" dc:"English display name" eg:"Example"`
	MinimumQuantity int    `json:"minimumQuantity" dc:"Minimum option quantity" eg:"1"`
	MaximumQuantity int    `json:"maximumQuantity" dc:"Maximum option quantity" eg:"1"`
	DefaultQuantity int    `json:"defaultQuantity" dc:"Default option quantity" eg:"1"`
}
type ServerResizeGroup struct {
	GroupNo         string                `json:"groupNo" dc:"Published group number" eg:"Example"`
	SelectionMode   string                `json:"selectionMode" dc:"Selection mode: single or multiple" eg:"Example"`
	NameZhCN        string                `json:"nameZhCN" dc:"Chinese display name" eg:"Example"`
	NameEnUS        string                `json:"nameEnUS" dc:"English display name" eg:"Example"`
	MinimumSelected int                   `json:"minimumSelected" dc:"Minimum selected options" eg:"1"`
	MaximumSelected int                   `json:"maximumSelected" dc:"Maximum selected options" eg:"1"`
	Options         []*ServerResizeOption `json:"options" dc:"Published option choices" eg:"[]"`
}
type ServerResizeRule struct {
	RuleType              string `json:"ruleType" dc:"Catalog compatibility rule type" eg:"Example"`
	SourceOptionNo        string `json:"sourceOptionNo" dc:"Source option number" eg:"Example"`
	TargetOptionNo        string `json:"targetOptionNo" dc:"Target option number" eg:"Example"`
	MinimumTargetQuantity int    `json:"minimumTargetQuantity" dc:"Minimum target quantity" eg:"1"`
	MaximumTargetQuantity *int   `json:"maximumTargetQuantity" dc:"Maximum target quantity, null when unrestricted" eg:"1"`
}
type ServerResizeOptionsReq struct {
	g.Meta    `path:"/account/servers/{serviceNo}/resize-options" method:"get" tags:"IDC Customer Servers" summary:"Get my server resize options" dc:"Return the current published configuration choices and the owned server's current selection."`
	ServiceNo string `json:"serviceNo" in:"path" v:"required|max-length:40" dc:"Target server number" eg:"SVC_SERVER_0001"`
}
type ServerResizeOptionsRes struct {
	PublicSlug        string                   `json:"publicSlug" dc:"Published product slug" eg:"Example"`
	CurrencyCode      string                   `json:"currencyCode" dc:"ISO currency code" eg:"CNY"`
	Revision          string                   `json:"revision" dc:"Published configuration revision" eg:"Example"`
	Groups            []*ServerResizeGroup     `json:"groups" dc:"Published configuration groups" eg:"[]"`
	Rules             []*ServerResizeRule      `json:"rules" dc:"Published compatibility rules" eg:"[]"`
	CurrentSelections []*ServerResizeSelection `json:"currentSelections" dc:"Current server option quantities" eg:"[]"`
}
type ServerResizeQuoteCreateReq struct {
	g.Meta         `path:"/account/servers/{serviceNo}/resize-quotes" method:"post" tags:"IDC Customer Servers" summary:"Quote my server configuration change" dc:"Freeze an immediate remaining-period upgrade charge or a no-refund next-renewal downgrade."`
	ServiceNo      string                   `json:"serviceNo" in:"path" v:"required|max-length:40" dc:"Target server number" eg:"SVC_SERVER_0001"`
	Revision       string                   `json:"revision" v:"required|max-length:4096" dc:"Published configuration revision" eg:"Example"`
	TermNo         string                   `json:"termNo" v:"max-length:4096" dc:"Target field value" eg:"Example"`
	CurrencyCode   string                   `json:"currencyCode" v:"required|length:3" dc:"ISO currency code" eg:"CNY"`
	Selections     []*ServerResizeSelection `json:"selections" v:"required" dc:"Target field value" eg:"[]"`
	IdempotencyKey string                   `json:"idempotencyKey" v:"required|min-length:8|max-length:256" dc:"Target field value" eg:"Example"`
}
type ServerResizeQuote struct {
	QuoteNo                     string `json:"quoteNo" dc:"Frozen configuration quote number" eg:"Example"`
	ServiceNo                   string `json:"serviceNo" dc:"Target server number" eg:"SVC_SERVER_0001"`
	Direction                   string `json:"direction" dc:"Change direction: upgrade or downgrade" eg:"Example"`
	CurrencyCode                string `json:"currencyCode" dc:"ISO currency code" eg:"CNY"`
	CurrentRecurringAmountMinor int64  `json:"currentRecurringAmountMinor" dc:"Current renewal price in currency minor units" eg:"1"`
	TargetRecurringAmountMinor  int64  `json:"targetRecurringAmountMinor" dc:"Target renewal price in currency minor units" eg:"1"`
	ChargeAmountMinor           int64  `json:"chargeAmountMinor" dc:"Current configuration change fee in currency minor units" eg:"1"`
	EffectiveAt                 int64  `json:"effectiveAt" dc:"Effective time as a Unix timestamp in milliseconds" eg:"1787184000000"`
	IssuedAt                    int64  `json:"issuedAt" dc:"Quote issue time as a Unix timestamp in milliseconds" eg:"1787184000000"`
	ExpiresAt                   int64  `json:"expiresAt" dc:"Quote expiry as a Unix timestamp in milliseconds" eg:"1787184000000"`
	Reused                      bool   `json:"reused" dc:"Whether the exact request was already accepted" eg:"false"`
}
type ServerResizeQuoteCreateRes struct {
	Quote *ServerResizeQuote `json:"quote" dc:"Frozen configuration change quote" eg:"{}"`
}
type ServerResizeCheckoutCreateReq struct {
	g.Meta         `path:"/account/server-resize-quotes/{quoteNo}/checkout" method:"post" tags:"IDC Customer Servers" summary:"Confirm my server configuration change" dc:"Create a pending bill for an upgrade or schedule a no-refund downgrade for the next renewal."`
	QuoteNo        string `json:"quoteNo" in:"path" v:"required|max-length:40" dc:"Frozen configuration quote number" eg:"Example"`
	IdempotencyKey string `json:"idempotencyKey" v:"required|min-length:8|max-length:256" dc:"Target field value" eg:"Example"`
}
type ServerResizeCheckout struct {
	OperationNo       string `json:"operationNo" dc:"Configuration change operation number" eg:"Example"`
	OrderNo           string `json:"orderNo" dc:"Pending order number, empty when no payment required" eg:"Example"`
	InvoiceNo         string `json:"invoiceNo" dc:"Pending invoice number, empty when no payment required" eg:"Example"`
	ServiceNo         string `json:"serviceNo" dc:"Target server number" eg:"SVC_SERVER_0001"`
	Direction         string `json:"direction" dc:"Change direction: upgrade or downgrade" eg:"Example"`
	Status            string `json:"status" dc:"Current owner workflow status" eg:"Example"`
	CurrencyCode      string `json:"currencyCode" dc:"ISO currency code" eg:"CNY"`
	ChargeAmountMinor int64  `json:"chargeAmountMinor" dc:"Current configuration change fee in currency minor units" eg:"1"`
	EffectiveAt       int64  `json:"effectiveAt" dc:"Effective time as a Unix timestamp in milliseconds" eg:"1787184000000"`
	Reused            bool   `json:"reused" dc:"Whether the exact request was already accepted" eg:"false"`
}
type ServerResizeCheckoutCreateRes struct {
	Checkout *ServerResizeCheckout `json:"checkout" dc:"Accepted configuration change" eg:"{}"`
}

type ServerCustomerMetadataUpdateReq struct {
	g.Meta          `path:"/account/servers/{serviceNo}/metadata" method:"put" tags:"IDC Account Servers" summary:"Update current customer server alias and note"`
	ServiceNo       string `json:"serviceNo" v:"required|max-length:40" dc:"Stable service number" eg:"SVC_SERVER_0001"`
	Alias           string `json:"alias" v:"max-length:128" dc:"Trimmed customer display alias" eg:"Production database"`
	Remark          string `json:"remark" v:"max-length:1000" dc:"Trimmed non-sensitive customer note" eg:"Used by the reporting team"`
	ExpectedVersion int64  `json:"expectedVersion" v:"required|min:1" dc:"Last server version read by the customer" eg:"8"`
}

type ServerCustomerMetadataUpdateRes struct {
	Metadata *ServerCustomerMetadata `json:"metadata" dc:"Safe server alias, note and version" eg:"{}"`
}

type ServerCustomerMetadata struct {
	Alias   string `json:"alias" dc:"Server display alias" eg:"Example"`
	Remark  string `json:"remark" dc:"Non-sensitive server note" eg:"Example"`
	Version int64  `json:"version" dc:"Current server version" eg:"1"`
}

type ServerOperationIntent struct {
	RequestNo  string          `json:"requestNo" v:"required|min-length:8|max-length:256" dc:"Client-generated idempotency key reused only for the exact same operation" eg:"server-550e8400-e29b-41d4-a716-446655440000"`
	Action     string          `json:"action" v:"required|in:start,stop,reboot,reset_password,console,reinstall,resize,ip_change,snapshot_create,snapshot_delete,snapshot_restore,backup_create,backup_delete,backup_restore,security_group_attach,security_group_detach,nat_create,nat_delete,virtual_media_mount,virtual_media_unmount,traffic_add" dc:"Requested server or accessory action code" eg:"snapshot_create"`
	Parameters json.RawMessage `json:"parameters" v:"required" dc:"Action-specific JSON object validated against the frozen Provider capability snapshot" eg:"{\"os\":101}"`
}

type ServerOperationAuthorizationReq struct {
	g.Meta    `path:"/account/servers/{serviceNo}/operation-authorizations" method:"post" tags:"IDC Customer Servers" summary:"Authorize my server operation" dc:"Evaluate the exact current-customer operation and issue a short-lived single-use confirmation proof only when merchant policy requires it."`
	ServiceNo string `json:"serviceNo" in:"path" v:"required|max-length:40" dc:"Stable owned service instance number" eg:"SVC_SERVER_0001"`
	ServerOperationIntent
}

type ServerOperationAuthorizationRes struct {
	Required   bool   `json:"required" dc:"Whether the customer must click the explicit second confirmation" eg:"true"`
	ProofToken string `json:"proofToken" dc:"Opaque single-use proof bound to customer, session, server, action, parameters, and request number" eg:"eyJ..."`
	ExpiresAt  *int64 `json:"expiresAt" dc:"Proof expiry time as a Unix timestamp in milliseconds, or null when no proof is required" eg:"1787184120000"`
}

type ServerOperationCreateReq struct {
	g.Meta    `path:"/account/servers/{serviceNo}/operations" method:"post" tags:"IDC Customer Servers" summary:"Operate my server" dc:"Create one mutually exclusive current-customer server operation after capability, state, ownership, and optional confirmation checks."`
	ServiceNo string `json:"serviceNo" in:"path" v:"required|max-length:40" dc:"Stable owned service instance number" eg:"SVC_SERVER_0001"`
	ServerOperationIntent
	Password   string `json:"password" v:"max-length:1024" dc:"Accepted only for reset_password and never returned or persisted as plaintext" eg:"Strong-password-2026"`
	ProofToken string `json:"proofToken" v:"max-length:4096" dc:"Single-use confirmation proof when the authorization response required one" eg:"eyJ..."`
}

type ServerOperationCreateRes struct {
	Operation *ServerOperationSummary `json:"operation" dc:"Created or idempotently reused server operation" eg:"{}"`
}

type ServerOperationOutputRedeemReq struct {
	g.Meta      `path:"/account/servers/{serviceNo}/operations/{operationNo}/confidential-output" method:"post" tags:"IDC Customer Servers" summary:"Reveal my completed server operation output once" dc:"After explicit customer confirmation, atomically consume and directly deliver the exact completed operation's confidential output in a no-store response."`
	ServiceNo   string `json:"serviceNo" in:"path" v:"required|max-length:40" dc:"Stable owned service instance number" eg:"SVC_SERVER_0001"`
	OperationNo string `json:"operationNo" in:"path" v:"required|max-length:40" dc:"Stable completed operation number belonging to the service" eg:"SOP_0123456789abcdef0123456789abcdef"`
	Confirmed   bool   `json:"confirmed" v:"required" dc:"Explicit customer confirmation that this one-time output will be consumed" eg:"true"`
}

type ServerOperationOutputRedeemRes struct {
	Payload json.RawMessage `json:"payload" dc:"One-time credential or short-lived console ticket JSON written only to this no-store response" eg:"{\"username\":\"root\",\"password\":\"temporary\",\"port\":22}"`
}
