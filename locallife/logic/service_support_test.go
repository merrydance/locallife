package logic

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestDefaultIDGeneratorPickupCodeUsesFourDigits(t *testing.T) {
	code, err := DefaultIDGenerator{}.PickupCode(time.Now())

	require.NoError(t, err)
	require.Regexp(t, `^\d{4}$`, code)
}
