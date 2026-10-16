// This file represents the public millisecond delivery wire format separately
// from internal core capability time values.
package peer

type peerOrderDeliveries struct {
	OrderNo string              `json:"orderNo"`
	Items   []peerOrderDelivery `json:"items"`
}

type peerOrderDelivery struct {
	DeliveryNo, Status     string
	Quantity               int
	Ongoing                bool
	StartedAt, CompletedAt *int64
	Services               []peerDeliveryService
}

type peerDeliveryService struct {
	ServiceNo, Status, RenewalMode string
	StartedAt, ExpiresAt           *int64
}
