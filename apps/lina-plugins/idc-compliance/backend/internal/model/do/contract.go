// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// Contract is the golang structure of table plugin_idc_compliance_contract for DAO operations like Where/Data.
type Contract struct {
	g.Meta                    `orm:"table:plugin_idc_compliance_contract, do:true"`
	Id                        any        //
	TenantId                  any        //
	ContractNo                any        //
	CustomerNo                any        //
	PrincipalNo               any        //
	OrderNo                   any        //
	ServiceNo                 *string    //
	ContractTypeCode          any        //
	RequiredForFulfillment    any        //
	TemplateNo                any        //
	TemplateVersion           any        //
	VariableSnapshotRef       any        //
	VariableSnapshotSha256    any        //
	SignerSnapshotSha256      any        //
	CurrencyCode              any        //
	AmountMinor               any        //
	StatusCode                any        //
	LaunchedAt                *time.Time //
	CompletedAt               *time.Time //
	TerminatedAt              *time.Time //
	Version                   any        //
	CreatedAt                 *time.Time //
	UpdatedAt                 *time.Time //
	SubjectTypeCode           any        //
	SigningOrderCode          any        //
	SnapshotKeyRef            any        // Opaque secretcap handle for the encrypted contract preparation bundle
	OrderSnapshotSha256       any        // Digest of the canonical immutable core order, service, identity, and amount snapshot
	CreationIdempotencyDigest any        //
	CreationFingerprint       any        //
	SigningExpiresAt          *time.Time // Fixed signing deadline exactly seven days after authoritative contract creation
}
