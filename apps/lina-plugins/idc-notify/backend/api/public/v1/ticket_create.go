package v1

import "github.com/gogf/gf/v2/frame/g"

// TicketCustomFieldSubmission carries exactly one typed public custom-field value.
// The authoritative support owner validates the field definition and value.
type TicketCustomFieldSubmission struct {
	FieldNo      string   `json:"fieldNo" dc:"Public presales custom field number" eg:"SCF_service_area"`
	TextValue    *string  `json:"textValue" dc:"Text value for a text field" eg:"Dedicated server"`
	NumberValue  *string  `json:"numberValue" dc:"Decimal value for a number field" eg:"12.50"`
	DateValue    *string  `json:"dateValue" dc:"Date value in YYYY-MM-DD format" eg:"2026-08-15"`
	BooleanValue *bool    `json:"booleanValue" dc:"Boolean value for a yes or no field" eg:"true"`
	OptionNos    []string `json:"optionNos" dc:"Selected option numbers for a select field" eg:"SCO_compute"`
}

type TicketCreateReq struct {
	g.Meta          `path:"/public/support/presales/tickets" method:"post" tags:"IDC Public Presales" summary:"Create an anonymous presales ticket" dc:"Consumes one verified code, creates or recovers the authoritative presales ticket, and returns the current browser access token."`
	BrowserNonce    string                        `json:"browserNonce" v:"required|min-length:16|max-length:128" dc:"Random nonce retained by this browser" eg:"7bdb8be1-cbe4-4c32-8b83-e5122ea20a42"`
	Destination     string                        `json:"destination" v:"required|max-length:320" dc:"Previously verified email address or E.164 phone number" eg:"help@example.com"`
	DestinationKind string                        `json:"destinationKind" v:"required|in:email,sms" dc:"Destination kind" eg:"email"`
	Locale          string                        `json:"locale" v:"required|in:zh-CN,en-US" dc:"Message language" eg:"zh-CN"`
	OperationNo     string                        `json:"operationNo" v:"required|min-length:8|max-length:64" dc:"Browser-generated idempotency operation number" eg:"presales-ticket-20260815-001"`
	VerificationNo  string                        `json:"verificationNo" v:"required|max-length:48" dc:"Verification number returned by code delivery" eg:"nver_01JZ..."`
	Code            string                        `json:"code" v:"required|length:6" dc:"Six-digit verification code" eg:"123456"`
	CategoryNo      string                        `json:"categoryNo" v:"required|max-length:40" dc:"Public presales ticket category" eg:"sales_consulting"`
	Subject         string                        `json:"subject" v:"required|max-length:256" dc:"Ticket subject" eg:"Dedicated server consultation"`
	Body            string                        `json:"body" v:"required|max-length:65536" dc:"Ticket content" eg:"I need a quotation for two servers."`
	CustomFields    []TicketCustomFieldSubmission `json:"customFields" dc:"At most one hundred typed public custom field submissions validated by the support owner" eg:"SCF_service_area"`
}

type TicketCreateRes PresalesAccessProjection
