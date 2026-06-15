package main

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidateFixtureClaimabilityFlagIDs(t *testing.T) {
	require.NoError(t, validateFixtureClaimabilityFlagIDs(false, 0, 0))
	require.NoError(t, validateFixtureClaimabilityFlagIDs(true, 101, 202))

	err := validateFixtureClaimabilityFlagIDs(true, 0, 202)
	require.ErrorContains(t, err, "payment-fact-application-fixture-id must be a positive integer")

	err = validateFixtureClaimabilityFlagIDs(true, 101, -1)
	require.ErrorContains(t, err, "payment-domain-outbox-fixture-id must be a positive integer")
}
