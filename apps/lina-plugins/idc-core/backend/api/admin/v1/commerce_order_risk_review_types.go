package v1

// CommerceOrderRiskReviewDecisionItem is the bounded result of one committed
// staff decision. Raw review evidence is intentionally never returned.
type CommerceOrderRiskReviewDecisionItem struct {
	ReviewNo    string `json:"reviewNo" dc:"Stable order risk-review business number" eg:"review-risk-001"`
	OrderNo     string `json:"orderNo" dc:"Order business number governed by this risk review" eg:"order-001"`
	CustomerNo  string `json:"customerNo" dc:"Customer business number that owns the order" eg:"customer-001"`
	StatusCode  string `json:"statusCode" dc:"Committed review decision: approved or rejected" eg:"approved"`
	ReasonCode  string `json:"reasonCode" dc:"Stable decision reason: risk_approved or risk_rejected" eg:"risk_approved"`
	ReviewedBy  int    `json:"reviewedBy" dc:"Trusted staff user identifier that committed the decision" eg:"1001"`
	ReviewedAt  int64  `json:"reviewedAt" dc:"Decision commit time as a Unix millisecond timestamp" eg:"1786320000000"`
	OrderStatus string `json:"orderStatus" dc:"Resulting order status after all independent gates are evaluated" eg:"pending_payment"`
	Reused      bool   `json:"reused" dc:"Whether the response reused the exact previously committed decision" eg:"false"`
}
