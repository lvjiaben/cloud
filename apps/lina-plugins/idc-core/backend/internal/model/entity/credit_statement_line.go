// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// CreditStatementLine is the golang structure for table credit_statement_line.
type CreditStatementLine struct {
	Id              int64      `json:"id"              orm:"id"                description:""`
	TenantId        int        `json:"tenantId"        orm:"tenant_id"         description:""`
	StatementLineNo string     `json:"statementLineNo" orm:"statement_line_no" description:""`
	StatementNo     string     `json:"statementNo"     orm:"statement_no"      description:""`
	CreditAccountNo string     `json:"creditAccountNo" orm:"credit_account_no" description:""`
	CustomerNo      string     `json:"customerNo"      orm:"customer_no"       description:""`
	CurrencyCode    string     `json:"currencyCode"    orm:"currency_code"     description:""`
	LineNumber      int        `json:"lineNumber"      orm:"line_number"       description:""`
	LineTypeCode    string     `json:"lineTypeCode"    orm:"line_type_code"    description:""`
	ExposureNo      string     `json:"exposureNo"      orm:"exposure_no"       description:""`
	SourceTypeCode  string     `json:"sourceTypeCode"  orm:"source_type_code"  description:""`
	SourceReference string     `json:"sourceReference" orm:"source_reference"  description:""`
	AmountMinor     int64      `json:"amountMinor"     orm:"amount_minor"      description:""`
	ContentDigest   string     `json:"contentDigest"   orm:"content_digest"    description:""`
	CreatedAt       *time.Time `json:"createdAt"       orm:"created_at"        description:""`
}
