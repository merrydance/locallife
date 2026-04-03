package util

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRandomInt(t *testing.T) {
	for range 32 {
		value := RandomInt(3, 7)
		require.GreaterOrEqual(t, value, int64(3))
		require.LessOrEqual(t, value, int64(7))
	}

	require.Equal(t, int64(5), RandomInt(5, 5))
	require.Equal(t, int64(5), RandomInt(5, 4))
}

func TestRandomString(t *testing.T) {
	value := RandomString(32)
	require.Len(t, value, 32)

	for _, ch := range value {
		require.Contains(t, alphabet, string(ch))
	}

	require.Empty(t, RandomString(0))
}
