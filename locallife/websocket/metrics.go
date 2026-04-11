package websocket

// MetricsRecorder captures WebSocket delivery metrics.
type MetricsRecorder interface {
	RecordSend(clientType ClientType, result string)
	RecordAck(clientType ClientType)
	RecordRetry(clientType ClientType)
	RecordReplay(clientType ClientType)
	RecordLatency(clientType ClientType, seconds float64)
	RecordConnections(riders, merchants, platforms int)
}

type noopMetricsRecorder struct{}

func (noopMetricsRecorder) RecordSend(clientType ClientType, result string) {}
func (noopMetricsRecorder) RecordAck(clientType ClientType)                 {}
func (noopMetricsRecorder) RecordRetry(clientType ClientType)               {}
func (noopMetricsRecorder) RecordReplay(clientType ClientType)              {}
func (noopMetricsRecorder) RecordLatency(clientType ClientType, seconds float64) {
}
func (noopMetricsRecorder) RecordConnections(riders, merchants, platforms int) {
}
