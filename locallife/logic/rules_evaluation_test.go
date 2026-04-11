package logic

import (
	"context"
	"errors"
	"testing"

	"github.com/merrydance/locallife/rules"
	"github.com/stretchr/testify/require"
)

type fakeRulesEngine struct {
	decision rules.Decision
	callErr  error
	called   bool
	input    rules.Context
}

func (f *fakeRulesEngine) Evaluate(ctx context.Context, input rules.Context) (rules.Decision, error) {
	f.called = true
	f.input = input
	return f.decision, f.callErr
}

func TestEvaluateRules(t *testing.T) {
	ctx := context.Background()
	ruleContext := rules.Context{Domain: rules.DomainOrder, RegionID: 1, MerchantID: 2, UserID: 3}

	testCases := []struct {
		name  string
		input RuleEvaluationInput
		check func(t *testing.T, decision rules.Decision, err error)
	}{
		{
			name:  "DisabledEngine",
			input: RuleEvaluationInput{Enabled: false, Engine: nil, Context: ruleContext},
			check: func(t *testing.T, decision rules.Decision, err error) {
				require.NoError(t, err)
				require.True(t, decision.Allow)
				require.Equal(t, "allow", decision.Action)
			},
		},
		{
			name:  "NilEngine",
			input: RuleEvaluationInput{Enabled: true, Engine: nil, Context: ruleContext},
			check: func(t *testing.T, decision rules.Decision, err error) {
				require.NoError(t, err)
				require.True(t, decision.Allow)
				require.Equal(t, "allow", decision.Action)
			},
		},
		{
			name: "EngineError",
			input: RuleEvaluationInput{
				Enabled: true,
				Engine:  &fakeRulesEngine{callErr: errors.New("boom")},
				Context: ruleContext,
			},
			check: func(t *testing.T, _ rules.Decision, err error) {
				require.Error(t, err)
				require.Equal(t, "boom", err.Error())
			},
		},
		{
			name: "DecisionDeniedWithReason",
			input: RuleEvaluationInput{
				Enabled: true,
				Engine:  &fakeRulesEngine{decision: rules.Decision{Allow: false, Action: "deny", Reason: "blocked"}},
				Context: ruleContext,
			},
			check: func(t *testing.T, decision rules.Decision, err error) {
				require.Equal(t, "deny", decision.Action)
				reqErr := assertRequestError(t, err)
				require.Equal(t, 403, reqErr.Status)
				require.Equal(t, "blocked", reqErr.Err.Error())
			},
		},
		{
			name: "DecisionDeniedDefaultReason",
			input: RuleEvaluationInput{
				Enabled: true,
				Engine:  &fakeRulesEngine{decision: rules.Decision{Allow: false, Action: "deny"}},
				Context: ruleContext,
			},
			check: func(t *testing.T, decision rules.Decision, err error) {
				require.Equal(t, "deny", decision.Action)
				reqErr := assertRequestError(t, err)
				require.Equal(t, 403, reqErr.Status)
				require.Equal(t, "order blocked by rule", reqErr.Err.Error())
			},
		},
		{
			name: "DecisionAllowedCallback",
			input: RuleEvaluationInput{
				Enabled:   true,
				Engine:    &fakeRulesEngine{decision: rules.Decision{Allow: true, Action: "allow"}},
				Context:   ruleContext,
				ActorRole: "customer",
				OnDecision: func(input rules.Context, decision rules.Decision, actorRole string) {
					require.Equal(t, ruleContext, input)
					require.Equal(t, "allow", decision.Action)
					require.Equal(t, "customer", actorRole)
				},
			},
			check: func(t *testing.T, decision rules.Decision, err error) {
				require.NoError(t, err)
				require.True(t, decision.Allow)
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			decision, err := EvaluateRules(ctx, tc.input)
			tc.check(t, decision, err)
		})
	}
}
