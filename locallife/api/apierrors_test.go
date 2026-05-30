package api

import (
	"os"
	"regexp"
	"sort"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDeliveryPickupMerchantNotReadyErrorCodeIsUnique(t *testing.T) {
	apiErrors := collectAPIErrorsFromSource(t)
	matches := make([]apiErrorDefinition, 0, 1)
	for _, apiErr := range apiErrors {
		if apiErr.Code == ErrDeliveryPickupMerchantNotReady.Code {
			matches = append(matches, apiErr)
		}
	}

	require.Len(t, matches, 1)
	require.Equal(t, "ErrDeliveryPickupMerchantNotReady", matches[0].Name)
}

func TestClaimAmountBelowPayoutMinimumErrorCodeIsUnique(t *testing.T) {
	apiErrors := collectAPIErrorsFromSource(t)
	matches := make([]apiErrorDefinition, 0, 1)
	for _, apiErr := range apiErrors {
		if apiErr.Code == ErrClaimAmountBelowPayoutMinimum.Code {
			matches = append(matches, apiErr)
		}
	}

	require.Len(t, matches, 1)
	require.Equal(t, "ErrClaimAmountBelowPayoutMinimum", matches[0].Name)
}

type apiErrorDefinition struct {
	Name string
	Code int
}

func collectAPIErrorsFromSource(t *testing.T) []apiErrorDefinition {
	t.Helper()

	raw, err := os.ReadFile("apierrors.go")
	require.NoError(t, err)
	source := string(raw)
	pattern := regexp.MustCompile(`(Err[A-Za-z0-9]+)\s*=\s*apierr\((\d+),`)
	matches := pattern.FindAllStringSubmatch(source, -1)

	apiErrors := make([]apiErrorDefinition, 0, len(matches))
	for _, match := range matches {
		code, err := strconv.Atoi(match[2])
		require.NoError(t, err)
		apiErrors = append(apiErrors, apiErrorDefinition{Name: match[1], Code: code})
	}
	sort.Slice(apiErrors, func(i, j int) bool {
		if apiErrors[i].Code == apiErrors[j].Code {
			return apiErrors[i].Name < apiErrors[j].Name
		}
		return apiErrors[i].Code < apiErrors[j].Code
	})
	return apiErrors
}
