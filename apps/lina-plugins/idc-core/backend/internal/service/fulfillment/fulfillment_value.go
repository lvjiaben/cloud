package fulfillment

import (
	"bytes"
	"encoding/json"
	"io"
	"regexp"
)

var referencePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]*$`)
var currencyPattern = regexp.MustCompile(`^[A-Z]{3}$`)

type settlementPayload struct {
	SettlementNo     string `json:"settlementNo"`
	OrderNo          string `json:"orderNo"`
	InvoiceNo        string `json:"invoiceNo"`
	CustomerNo       string `json:"customerNo"`
	CurrencyCode     string `json:"currencyCode"`
	BasisCode        string `json:"basisCode"`
	TotalAmountMinor int64  `json:"totalAmountMinor"`
	SettledAtUnixMS  int64  `json:"settledAtUnixMs"`
}

type settlementMetadata struct {
	SourceCount int `json:"sourceCount"`
}

func decodeSettlementEvent(event outboxEventView) (settlementPayload, settlementMetadata, error) {
	if event.EventType != FinanciallySettledEventType ||
		event.EventVersion != financiallySettledVersion ||
		event.Topic != FinanciallySettledEventType || event.AggregateType != "commerce_order" ||
		!validReference(event.EventNo, 40) || !validReference(event.AggregateNo, 40) ||
		!validReference(event.CorrelationNo, 40) {
		return settlementPayload{}, settlementMetadata{}, permanent("financial_settlement_event_invalid")
	}
	var payload settlementPayload
	if err := decodeStrictObject(event.Payload, &payload); err != nil ||
		!validReference(payload.SettlementNo, 40) || !validReference(payload.OrderNo, 40) ||
		!validReference(payload.InvoiceNo, 40) || !validReference(payload.CustomerNo, 40) ||
		!currencyPattern.MatchString(payload.CurrencyCode) || payload.TotalAmountMinor < 0 ||
		payload.SettledAtUnixMS <= 0 || payload.OrderNo != event.AggregateNo ||
		payload.SettlementNo != event.CorrelationNo || !validSettlementBasis(payload.BasisCode) {
		return settlementPayload{}, settlementMetadata{}, permanent("financial_settlement_event_invalid")
	}
	var metadata settlementMetadata
	if err := decodeStrictObject(event.Metadata, &metadata); err != nil ||
		metadata.SourceCount < 1 || metadata.SourceCount > maximumSettlementSources {
		return settlementPayload{}, settlementMetadata{}, permanent("financial_settlement_event_invalid")
	}
	return payload, metadata, nil
}

func decodeStrictObject(raw []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return permanent("financial_settlement_event_invalid")
	}
	return nil
}

func validReference(value string, maximum int) bool {
	return len(value) >= 1 && len(value) <= maximum && referencePattern.MatchString(value)
}

func validSettlementBasis(value string) bool {
	switch value {
	case "funded_invoice", "credit", "zero_amount", "full_discount", "trial":
		return true
	default:
		return false
	}
}

type outboxEventView struct {
	EventNo       string
	EventType     string
	EventVersion  int
	Topic         string
	AggregateType string
	AggregateNo   string
	CorrelationNo string
	Payload       []byte
	Metadata      []byte
}
