package logic

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func assertRequestError(t *testing.T, err error) *RequestError {
	require.Error(t, err)
	var reqErr *RequestError
	require.ErrorAs(t, err, &reqErr)
	return reqErr
}
