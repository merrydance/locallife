package logic

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDecodeOrderItemCustomizations_UsesMetaSpecsFromNormalizedSelection(t *testing.T) {
	customizations, specsText, err := DecodeOrderItemCustomizations([]byte(`{"501":601,"502":602,"meta_specs":"大份 / 少辣"}`))
	require.NoError(t, err)
	require.Empty(t, customizations)
	require.Equal(t, "大份 / 少辣", specsText)
}

func TestDecodeOrderItemCustomizations_DerivesSpecsFromStructuredItems(t *testing.T) {
	customizations, specsText, err := DecodeOrderItemCustomizations([]byte(`[{"name":"规格","value":"大份"},{"name":"辣度","value":"少辣"}]`))
	require.NoError(t, err)
	require.Len(t, customizations, 2)
	require.Equal(t, "规格：大份 / 辣度：少辣", specsText)
}
