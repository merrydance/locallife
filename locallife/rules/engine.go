package rules

import "context"

// Domain represents the rule evaluation domain.
type Domain string

const (
	DomainOrder         Domain = "order"
	DomainReservation   Domain = "reservation"
	DomainPayment       Domain = "payment"
	DomainProfitSharing Domain = "profit_sharing"
	DomainClaim         Domain = "claim"
)

// Context carries inputs for rule evaluation.
type Context struct {
	Domain     Domain                 `json:"domain"`
	RegionID   int64                  `json:"region_id"`
	MerchantID int64                  `json:"merchant_id"`
	UserID     int64                  `json:"user_id"`
	OrderType  string                 `json:"order_type,omitempty"`
	Amount     int64                  `json:"amount,omitempty"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
}

// Decision is the rule evaluation result.
type Decision struct {
	Allow         bool                   `json:"allow"`
	Action        string                 `json:"action"`
	Reason        string                 `json:"reason,omitempty"`
	Meta          map[string]interface{} `json:"meta,omitempty"`
	RuleID        int64                  `json:"rule_id,omitempty"`
	RuleVersionID int64                  `json:"rule_version_id,omitempty"`
}

// Engine evaluates rules for a given context.
type Engine interface {
	Evaluate(ctx context.Context, input Context) (Decision, error)
}

// NoopEngine allows all requests and performs no evaluation.
type NoopEngine struct{}

// NewNoopEngine creates a no-op rule engine.
func NewNoopEngine() *NoopEngine {
	return &NoopEngine{}
}

// Evaluate always allows the request.
func (e *NoopEngine) Evaluate(ctx context.Context, input Context) (Decision, error) {
	return Decision{
		Allow:  true,
		Action: "allow",
	}, nil
}
