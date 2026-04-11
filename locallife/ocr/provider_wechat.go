package ocr

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/merrydance/locallife/util"
	"github.com/merrydance/locallife/wechat"
)

// WechatPrintedTextProvider wraps WeChat printed-text OCR under the unified provider abstraction.
type WechatPrintedTextProvider struct {
	client wechat.WechatClient
}

type WechatBusinessLicenseProvider struct {
	client wechat.WechatClient
}

type WechatIDCardProvider struct {
	client wechat.WechatClient
}

func NewWechatBusinessLicenseProvider(client wechat.WechatClient) *WechatBusinessLicenseProvider {
	return &WechatBusinessLicenseProvider{client: client}
}

func NewWechatIDCardProvider(client wechat.WechatClient) *WechatIDCardProvider {
	return &WechatIDCardProvider{client: client}
}

func NewWechatPrintedTextProvider(client wechat.WechatClient) *WechatPrintedTextProvider {
	return &WechatPrintedTextProvider{client: client}
}

func (p *WechatPrintedTextProvider) Name() ProviderName {
	return ProviderNameWechat
}

func (p *WechatBusinessLicenseProvider) Name() ProviderName {
	return ProviderNameWechat
}

func (p *WechatIDCardProvider) Name() ProviderName {
	return ProviderNameWechat
}

func (p *WechatBusinessLicenseProvider) Recognize(ctx context.Context, capability Capability, req RecognizeRequest) (RecognizeResponse, error) {
	if p.client == nil {
		return RecognizeResponse{}, fmt.Errorf("wechat ocr client not configured")
	}
	if capability != CapabilityWechatBusinessLicense {
		return RecognizeResponse{}, fmt.Errorf("unsupported wechat capability: %s", capability)
	}
	imgFile := util.NewBytesFile(req.Data)
	ocrResp, err := p.client.OCRBusinessLicense(ctx, imgFile)
	if err != nil {
		return RecognizeResponse{}, err
	}
	raw, err := json.Marshal(ocrResp)
	if err != nil {
		return RecognizeResponse{}, err
	}
	return RecognizeResponse{
		Provider:  ProviderNameWechat,
		RawResult: raw,
		Normalized: NormalizedResult{
			DocumentType: req.DocumentType,
			Side:         req.Side,
			BusinessLicense: &BusinessLicenseResult{
				CreditCode:          ocrResp.CreditCode,
				RegistrationNumber:  ocrResp.RegNum,
				EnterpriseName:      ocrResp.EnterpriseEName,
				LegalRepresentative: ocrResp.LegalRepresentative,
				Address:             ocrResp.Address,
				BusinessScope:       ocrResp.BusinessScope,
				ValidPeriod:         ocrResp.ValidPeriod,
			},
			RecognizedAt: time.Now().UTC(),
		},
	}, nil
}

func (p *WechatPrintedTextProvider) Recognize(ctx context.Context, capability Capability, req RecognizeRequest) (RecognizeResponse, error) {
	if p.client == nil {
		return RecognizeResponse{}, fmt.Errorf("wechat ocr client not configured")
	}
	if capability != CapabilityWechatPrintedText {
		return RecognizeResponse{}, fmt.Errorf("unsupported wechat capability: %s", capability)
	}
	imgFile := util.NewBytesFile(req.Data)
	ocrResp, err := p.client.OCRPrintedText(ctx, imgFile)
	if err != nil {
		return RecognizeResponse{}, err
	}
	raw, err := json.Marshal(ocrResp)
	if err != nil {
		return RecognizeResponse{}, err
	}
	rawText := ocrResp.GetAllText()
	normalized := NormalizedResult{
		DocumentType: req.DocumentType,
		Side:         req.Side,
		RecognizedAt: time.Now().UTC(),
	}
	if req.DocumentType == DocumentTypeHealthCert {
		normalized.HealthCert = &HealthCertResult{RawText: rawText}
	} else {
		normalized.FoodPermit = &FoodPermitResult{RawText: rawText}
	}
	return RecognizeResponse{
		Provider:   ProviderNameWechat,
		RawResult:  raw,
		Normalized: normalized,
	}, nil
}

func (p *WechatIDCardProvider) Recognize(ctx context.Context, capability Capability, req RecognizeRequest) (RecognizeResponse, error) {
	if p.client == nil {
		return RecognizeResponse{}, fmt.Errorf("wechat ocr client not configured")
	}
	if capability != CapabilityWechatIDCard {
		return RecognizeResponse{}, fmt.Errorf("unsupported wechat capability: %s", capability)
	}
	side := "Front"
	if req.Side == DocumentSideBack {
		side = "Back"
	}
	imgFile := util.NewBytesFile(req.Data)
	ocrResp, err := p.client.OCRIDCard(ctx, imgFile, side)
	if err != nil {
		return RecognizeResponse{}, err
	}
	raw, err := json.Marshal(ocrResp)
	if err != nil {
		return RecognizeResponse{}, err
	}
	result := &IDCardResult{}
	if req.Side == DocumentSideBack {
		result.ValidPeriod = ocrResp.ValidDate
	} else {
		result.Name = ocrResp.Name
		result.IDNumber = ocrResp.ID
		result.Gender = ocrResp.Gender
		result.Ethnicity = ocrResp.Nation
		result.Address = ocrResp.Addr
	}
	return RecognizeResponse{
		Provider:  ProviderNameWechat,
		RawResult: raw,
		Normalized: NormalizedResult{
			DocumentType: req.DocumentType,
			Side:         req.Side,
			IDCard:       result,
			RecognizedAt: time.Now().UTC(),
		},
	}, nil
}
