// This file defines exact signed local routes for Tencent contract flows, links, and rechecks.

package processauth

const (
	// TencentContractFlowsPath creates one guarded signing flow from a frozen template snapshot.
	TencentContractFlowsPath = "/internal/v1/tencent-essbasic/contract-flows"
	// TencentContractFlowCreateScope authenticates only an explicit contract signing submission.
	TencentContractFlowCreateScope = "tencent-essbasic.contract-flow.create"
	// TencentContractSigningLinksPath creates a fresh short-lived link for one existing flow.
	TencentContractSigningLinksPath = "/internal/v1/tencent-essbasic/contract-signing-links"
	// TencentContractSigningLinkCreateScope authenticates only the read-side link projection.
	TencentContractSigningLinkCreateScope = "tencent-essbasic.contract-signing-link.create"
	// TencentContractFlowRechecksPath performs one explicit authoritative flow query.
	TencentContractFlowRechecksPath = "/internal/v1/tencent-essbasic/contract-flow-rechecks"
	// TencentContractFlowRecheckScope authenticates only an explicit read-side query.
	TencentContractFlowRecheckScope = "tencent-essbasic.contract-flow.recheck"
	// TencentContractTerminationsPath creates one guarded release agreement for a completed contract.
	TencentContractTerminationsPath = "/internal/v1/tencent-essbasic/contract-terminations"
	// TencentContractTerminationCreateScope authenticates only an explicit contract release submission.
	TencentContractTerminationCreateScope = "tencent-essbasic.contract-termination.create"
)

// TencentContractFlowRequest is the exact signed local process contract. It
// contains business values only and never accepts supplier credentials or an endpoint.
type TencentContractFlowRequest struct {
	ContractNo   string                     `json:"contractNo"`
	TemplateRef  string                     `json:"templateRef"`
	LanguageCode string                     `json:"languageCode"`
	DeadlineUnix int64                      `json:"deadlineUnix"`
	FormFields   []TencentContractFormField `json:"formFields"`
	Approvers    []TencentContractApprover  `json:"approvers"`
}

// TencentContractFormField is one frozen Tencent template field value.
type TencentContractFormField struct {
	ComponentName string `json:"componentName"`
	TypeCode      string `json:"typeCode"`
	Value         string `json:"value"`
}

// TencentContractApprover is one signer in the exact Tencent template order.
type TencentContractApprover struct {
	SequenceNo       int    `json:"sequenceNo"`
	RecipientRef     string `json:"recipientRef"`
	PartyCode        string `json:"partyCode"`
	SubjectTypeCode  string `json:"subjectTypeCode"`
	Name             string `json:"name,omitempty"`
	Mobile           string `json:"mobile,omitempty"`
	OrganizationName string `json:"organizationName,omitempty"`
}

// TencentContractFlowResponse exposes only the installation-local sealed route.
type TencentContractFlowResponse struct {
	RouteNo   string `json:"routeNo"`
	Status    string `json:"status"`
	Recovered bool   `json:"recovered"`
}

// TencentContractTerminationRequest contains the frozen release-agreement
// content and only an installation-local route for the completed source flow.
// It never accepts supplier credentials, supplier flow identifiers, a custom
// endpoint, an effective time, or replacement signers.
type TencentContractTerminationRequest struct {
	ContractNo                string `json:"contractNo"`
	OriginalRouteNo           string `json:"originalRouteNo"`
	Reason                    string `json:"reason"`
	RemainInForceItem         string `json:"remainInForceItem,omitempty"`
	OriginalExpenseSettlement string `json:"originalExpenseSettlement,omitempty"`
	OriginalOtherSettlement   string `json:"originalOtherSettlement,omitempty"`
	OtherDeals                string `json:"otherDeals,omitempty"`
	DeadlineUnix              int64  `json:"deadlineUnix"`
}

// TencentContractSigningLinkRequest contains only the local sealed route, the
// frozen customer recipient, and a server-owned return URL. It cannot create a flow.
type TencentContractSigningLinkRequest struct {
	RouteNo  string                  `json:"routeNo"`
	Approver TencentContractApprover `json:"approver"`
	JumpURL  string                  `json:"jumpUrl"`
}

// TencentContractSigningLinkResponse exposes one non-persisted 30-minute H5 link.
type TencentContractSigningLinkResponse struct {
	URL         string `json:"url"`
	ExpiresUnix int64  `json:"expiresUnix"`
}

// TencentContractFlowRecheckRequest identifies one existing local sealed route.
type TencentContractFlowRecheckRequest struct {
	RouteNo string `json:"routeNo"`
}

// TencentContractFlowRecheckResponse is the safe local authoritative projection.
type TencentContractFlowRecheckResponse struct {
	RouteNo              string `json:"routeNo"`
	Status               string `json:"status"`
	ProviderStatus       string `json:"providerStatus"`
	ReconciliationStatus string `json:"reconciliationStatus"`
	RouteVersion         int64  `json:"routeVersion"`
	SupplierObservedUnix int64  `json:"supplierObservedUnix"`
	DecisionCode         string `json:"decisionCode"`
}
