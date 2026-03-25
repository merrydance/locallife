package ocr

import (
	"context"
	"encoding/json"
)

// RecognizeRequest is the provider input payload after routing.
type RecognizeRequest struct {
	DocumentType DocumentType
	Side         DocumentSide
	MediaAssetID int64
	ContentType  string
	Data         []byte
}

// BinaryReader reads media asset bytes for internal OCR execution.
type BinaryReader interface {
	ReadMediaAsset(ctx context.Context, mediaAssetID int64) ([]byte, string, error)
}

// RecognizeResponse is the provider output before projector handling.
type RecognizeResponse struct {
	Provider       ProviderName
	ProviderTaskID string
	RawResult      json.RawMessage
	Normalized     NormalizedResult
}

// Provider performs OCR against a specific provider implementation.
type Provider interface {
	Name() ProviderName
	Recognize(ctx context.Context, capability Capability, req RecognizeRequest) (RecognizeResponse, error)
}

// Route binds a provider instance to a capability selected for a document type.
type Route struct {
	Provider   Provider
	Capability Capability
}
