package logic

import (
	"context"
	"errors"
	"net/http"

	"github.com/merrydance/locallife/rules"
)

// RuleEvaluationInput defines the inputs to evaluate a rule decision.
type RuleEvaluationInput struct {
	Enabled    bool
	Engine     rules.Engine
	Context    rules.Context
	ActorRole  string
	OnDecision func(input rules.Context, decision rules.Decision, actorRole string)
}

// EvaluateRules runs the rules engine and returns the decision.
func EvaluateRules(ctx context.Context, input RuleEvaluationInput) (rules.Decision, error) {
	if !input.Enabled || input.Engine == nil {
		return rules.Decision{Allow: true, Action: "allow"}, nil
	}

	decision, err := input.Engine.Evaluate(ctx, input.Context)
	if err != nil {
		return rules.Decision{}, err
	}
	if input.OnDecision != nil {
		input.OnDecision(input.Context, decision, input.ActorRole)
	}
	if !decision.Allow {
		reason := decision.Reason
		if reason == "" {
			reason = "order blocked by rule"
		}
		return decision, NewRequestError(http.StatusForbidden, errors.New(reason))
	}

	return decision, nil
}
