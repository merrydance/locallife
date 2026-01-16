package api

import "github.com/merrydance/locallife/websocket"

// WSMetricsRecorder adapts websocket metrics into Prometheus counters.
type WSMetricsRecorder struct{}

func (WSMetricsRecorder) RecordSend(clientType websocket.ClientType, result string) {
	RecordWSMessage(string(clientType), result)
}

func (WSMetricsRecorder) RecordAck(clientType websocket.ClientType) {
	RecordWSAck(string(clientType))
}

func (WSMetricsRecorder) RecordRetry(clientType websocket.ClientType) {
	RecordWSRetry(string(clientType))
}

func (WSMetricsRecorder) RecordReplay(clientType websocket.ClientType) {
	RecordWSReplay(string(clientType))
}

func (WSMetricsRecorder) RecordLatency(clientType websocket.ClientType, seconds float64) {
	RecordWSAckLatency(string(clientType), seconds)
}

func (WSMetricsRecorder) RecordConnections(riders, merchants, platforms int) {
	UpdateWSMetrics(riders, merchants, platforms)
}
