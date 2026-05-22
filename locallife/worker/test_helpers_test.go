package worker

import "context"

type testPublisher struct {
	channel string
	payload []byte
}

func (p *testPublisher) Publish(_ context.Context, channel string, payload []byte) error {
	p.channel = channel
	p.payload = append([]byte(nil), payload...)
	return nil
}
