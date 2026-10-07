// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// ServicePriceComponentSnapshot is the golang structure for table service_price_component_snapshot.
type ServicePriceComponentSnapshot struct {
	Id                            int64      `json:"id"                            orm:"id"                               description:""`
	TenantId                      int        `json:"tenantId"                      orm:"tenant_id"                        description:""`
	ComponentSnapshotNo           string     `json:"componentSnapshotNo"           orm:"component_snapshot_no"            description:""`
	PriceSnapshotNo               string     `json:"priceSnapshotNo"               orm:"price_snapshot_no"                description:""`
	ServiceNo                     string     `json:"serviceNo"                     orm:"service_no"                       description:""`
	LineNumber                    int        `json:"lineNumber"                    orm:"line_number"                      description:""`
	ComponentTypeCode             string     `json:"componentTypeCode"             orm:"component_type_code"              description:""`
	ChargePhaseCode               string     `json:"chargePhaseCode"               orm:"charge_phase_code"                description:""`
	OptionNo                      *string    `json:"optionNo"                      orm:"option_no"                        description:""`
	SourceReference               string     `json:"sourceReference"               orm:"source_reference"                 description:""`
	Quantity                      int        `json:"quantity"                      orm:"quantity"                         description:""`
	UnitAmountMinor               int64      `json:"unitAmountMinor"               orm:"unit_amount_minor"                description:""`
	AmountMinor                   int64      `json:"amountMinor"                   orm:"amount_minor"                     description:""`
	SourceDigest                  string     `json:"sourceDigest"                  orm:"source_digest"                    description:""`
	ContentDigest                 string     `json:"contentDigest"                 orm:"content_digest"                   description:""`
	CreatedAt                     *time.Time `json:"createdAt"                     orm:"created_at"                       description:""`
	SubjectTypeCode               string     `json:"subjectTypeCode"               orm:"subject_type_code"                description:""`
	SubjectNo                     string     `json:"subjectNo"                     orm:"subject_no"                       description:""`
	ConfigurationPriceDimensionNo string     `json:"configurationPriceDimensionNo" orm:"configuration_price_dimension_no" description:""`
	ConfigurationSnapshotNo       string     `json:"configurationSnapshotNo"       orm:"configuration_snapshot_no"        description:""`
}
