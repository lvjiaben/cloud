// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// ContractTemplateField is the golang structure for table contract_template_field.
type ContractTemplateField struct {
	Id            int64      `json:"id"            orm:"id"              description:""`
	TenantId      int        `json:"tenantId"      orm:"tenant_id"       description:""`
	FieldNo       string     `json:"fieldNo"       orm:"field_no"        description:""`
	FieldSetNo    string     `json:"fieldSetNo"    orm:"field_set_no"    description:""`
	TemplateNo    string     `json:"templateNo"    orm:"template_no"     description:""`
	VersionNo     int64      `json:"versionNo"     orm:"version_no"      description:""`
	SequenceNo    int        `json:"sequenceNo"    orm:"sequence_no"     description:""`
	FieldTypeCode string     `json:"fieldTypeCode" orm:"field_type_code" description:""`
	PartyCode     string     `json:"partyCode"     orm:"party_code"      description:""`
	VariableCode  *string    `json:"variableCode"  orm:"variable_code"   description:""`
	PageNo        int        `json:"pageNo"        orm:"page_no"         description:""`
	PositionXPpm  int        `json:"positionXPpm"  orm:"position_x_ppm"  description:""`
	PositionYPpm  int        `json:"positionYPpm"  orm:"position_y_ppm"  description:""`
	WidthPpm      int        `json:"widthPpm"      orm:"width_ppm"       description:""`
	HeightPpm     int        `json:"heightPpm"     orm:"height_ppm"      description:""`
	CreatedAt     *time.Time `json:"createdAt"     orm:"created_at"      description:""`
}
