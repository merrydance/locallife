package api

import (
	"context"
	"testing"

	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/rules"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestDBRulesEngineEvaluate_ReturnsErrorOnMalformedActiveRuleVersionJSON(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().
		ListActiveRuleVersions(gomock.Any()).
		Times(1).
		Return([]db.RuleVersion{{
			ID:         11,
			RuleID:     21,
			Scope:      []byte(`{"domain":"claim"`),
			Condition:  []byte(`{}`),
			Action:     []byte(`{"type":"deny"}`),
			GrayConfig: []byte(`{}`),
		}}, nil)

	engine := NewDBRulesEngine(store)
	decision, err := engine.Evaluate(context.Background(), rules.Context{Domain: rules.DomainClaim})

	require.Equal(t, rules.Decision{}, decision)
	require.Error(t, err)
	require.ErrorContains(t, err, "decode active rule version 11 field scope")
}
