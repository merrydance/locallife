package ocr

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

type stubProvider struct {
	name ProviderName
}

func (p stubProvider) Name() ProviderName {
	return p.name
}

func (p stubProvider) Recognize(ctx context.Context, capability Capability, req RecognizeRequest) (RecognizeResponse, error) {
	_ = ctx
	_ = capability
	_ = req
	return RecognizeResponse{}, nil
}

func TestNewAliyunPrimaryRouterRoutesAliyunCapabilities(t *testing.T) {
	router, err := NewAliyunPrimaryRouter(stubProvider{name: ProviderNameAliyun}, nil)
	if err != nil {
		t.Fatalf("NewAliyunPrimaryRouter error = %v", err)
	}
	tests := []struct {
		name       string
		docType    DocumentType
		capability Capability
	}{
		{name: "business license", docType: DocumentTypeBusinessLicense, capability: CapabilityAliyunBusinessLicense},
		{name: "id card", docType: DocumentTypeIDCard, capability: CapabilityAliyunIDCard},
		{name: "food permit", docType: DocumentTypeFoodPermit, capability: CapabilityAliyunFoodPermit},
		{name: "health cert", docType: DocumentTypeHealthCert, capability: CapabilityAliyunHealthCert},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			route, routeErr := router.Route(tc.docType)
			if routeErr != nil {
				t.Fatalf("Route error = %v", routeErr)
			}
			if route.Capability != tc.capability {
				t.Fatalf("capability = %s, want %s", route.Capability, tc.capability)
			}
			if route.Provider.Name() != ProviderNameAliyun {
				t.Fatalf("provider = %s, want %s", route.Provider.Name(), ProviderNameAliyun)
			}
		})
	}
}

func TestMarshalRoundTripNormalizedResult(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	encoded, err := MarshalNormalizedResult(NormalizedResult{
		DocumentType: DocumentTypeIDCard,
		Side:         DocumentSideFront,
		IDCard: &IDCardResult{
			Name:     "张三",
			IDNumber: "110101199001011234",
		},
		RecognizedAt: now,
	})
	if err != nil {
		t.Fatalf("MarshalNormalizedResult error = %v", err)
	}
	if !json.Valid(encoded) {
		t.Fatalf("encoded normalized result is not valid JSON")
	}
	decoded, err := UnmarshalNormalizedResult(encoded)
	if err != nil {
		t.Fatalf("UnmarshalNormalizedResult error = %v", err)
	}
	if decoded.DocumentType != DocumentTypeIDCard {
		t.Fatalf("document type = %s, want %s", decoded.DocumentType, DocumentTypeIDCard)
	}
	if decoded.IDCard == nil || decoded.IDCard.Name != "张三" {
		t.Fatalf("decoded IDCard result mismatch: %+v", decoded.IDCard)
	}
}
