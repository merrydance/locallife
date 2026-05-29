package websocket

import (
	"testing"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"
)

func TestIsUnexpectedWebSocketReadError(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		unexpected bool
	}{
		{
			name:       "normal client logout close frame is expected",
			err:        &websocket.CloseError{Code: websocket.CloseNormalClosure, Text: "Client logout"},
			unexpected: false,
		},
		{
			name:       "client navigation away is expected",
			err:        &websocket.CloseError{Code: websocket.CloseGoingAway, Text: "page closed"},
			unexpected: false,
		},
		{
			name:       "abnormal close is expected for network drops",
			err:        &websocket.CloseError{Code: websocket.CloseAbnormalClosure, Text: "unexpected EOF"},
			unexpected: false,
		},
		{
			name:       "protocol errors remain unexpected",
			err:        &websocket.CloseError{Code: websocket.CloseProtocolError, Text: "bad frame"},
			unexpected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.unexpected, isUnexpectedWebSocketReadError(tt.err))
		})
	}
}
