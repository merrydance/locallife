package baofu

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestPublicEnvelopeTimestampUsesBaofooLocalTime(t *testing.T) {
	now := time.Date(2026, 5, 4, 13, 18, 38, 0, time.UTC)

	require.Equal(t, "20260504211838", PublicEnvelopeTimestamp(now))
}
