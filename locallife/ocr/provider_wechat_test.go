package ocr

import (
	"context"
	"encoding/json"
	"errors"
	"mime/multipart"
	"testing"

	"github.com/merrydance/locallife/wechat"
)

type stubWechatOCRClient struct {
	businessLicenseResp *wechat.BusinessLicenseOCRResponse
	idCardResp          *wechat.IDCardOCRResponse
	printedTextResp     *wechat.PrintedTextOCRResponse
	err                 error
}

func (c stubWechatOCRClient) Code2Session(ctx context.Context, code string) (*wechat.Code2SessionResponse, error) {
	_ = ctx
	_ = code
	return nil, errors.New("not implemented")
}

func (c stubWechatOCRClient) ImgSecCheck(ctx context.Context, imgFile multipart.File) error {
	_ = ctx
	_ = imgFile
	return errors.New("not implemented")
}

func (c stubWechatOCRClient) MsgSecCheck(ctx context.Context, openid string, scene int, content string) error {
	_ = ctx
	_ = openid
	_ = scene
	_ = content
	return errors.New("not implemented")
}

func (c stubWechatOCRClient) MediaCheckAsync(ctx context.Context, req wechat.MediaCheckAsyncRequest) (*wechat.MediaCheckAsyncResponse, error) {
	_ = ctx
	_ = req
	return nil, errors.New("not implemented")
}

func (c stubWechatOCRClient) OCRBusinessLicense(ctx context.Context, imgFile multipart.File) (*wechat.BusinessLicenseOCRResponse, error) {
	_ = ctx
	_ = imgFile
	return c.businessLicenseResp, c.err
}

func (c stubWechatOCRClient) OCRIDCard(ctx context.Context, imgFile multipart.File, cardSide string) (*wechat.IDCardOCRResponse, error) {
	_ = ctx
	_ = imgFile
	_ = cardSide
	return c.idCardResp, c.err
}

func (c stubWechatOCRClient) OCRPrintedText(ctx context.Context, imgFile multipart.File) (*wechat.PrintedTextOCRResponse, error) {
	_ = ctx
	_ = imgFile
	return c.printedTextResp, c.err
}

func (c stubWechatOCRClient) GetWXACodeUnlimited(ctx context.Context, req *wechat.WXACodeRequest) ([]byte, error) {
	_ = ctx
	_ = req
	return nil, errors.New("not implemented")
}

func TestWechatPrintedTextProviderRecognizeFoodPermit(t *testing.T) {
	provider := NewWechatPrintedTextProvider(stubWechatOCRClient{printedTextResp: &wechat.PrintedTextOCRResponse{Items: []wechat.PrintedTextItem{{Text: "许可证编号 JY123"}, {Text: "企业名称 测试餐厅"}}}})
	resp, err := provider.Recognize(context.Background(), CapabilityWechatPrintedText, RecognizeRequest{DocumentType: DocumentTypeFoodPermit, Data: []byte("img")})
	if err != nil {
		t.Fatalf("Recognize error = %v", err)
	}
	if resp.Provider != ProviderNameWechat {
		t.Fatalf("provider = %s", resp.Provider)
	}
	if resp.Normalized.FoodPermit == nil || resp.Normalized.FoodPermit.RawText == "" {
		t.Fatalf("food permit raw text missing: %+v", resp.Normalized.FoodPermit)
	}
	if !json.Valid(resp.RawResult) {
		t.Fatalf("raw result should be valid JSON")
	}
}

func TestWechatBusinessLicenseProviderRecognize(t *testing.T) {
	provider := NewWechatBusinessLicenseProvider(stubWechatOCRClient{businessLicenseResp: &wechat.BusinessLicenseOCRResponse{
		RegNum:              "91310000123456789A",
		EnterpriseEName:     "本地生活科技有限公司",
		LegalRepresentative: "张三",
		Address:             "测试路1号",
		BusinessScope:       "餐饮服务",
		ValidPeriod:         "2020-01-01 至 2040-01-01",
		CreditCode:          "91310000123456789A",
	}})
	resp, err := provider.Recognize(context.Background(), CapabilityWechatBusinessLicense, RecognizeRequest{DocumentType: DocumentTypeBusinessLicense, Data: []byte("img")})
	if err != nil {
		t.Fatalf("Recognize error = %v", err)
	}
	if resp.Normalized.BusinessLicense == nil {
		t.Fatal("expected normalized business license result")
	}
	if resp.Normalized.BusinessLicense.EnterpriseName != "本地生活科技有限公司" {
		t.Fatalf("enterprise name = %s", resp.Normalized.BusinessLicense.EnterpriseName)
	}
	if resp.Normalized.BusinessLicense.CreditCode != "91310000123456789A" {
		t.Fatalf("credit code = %s", resp.Normalized.BusinessLicense.CreditCode)
	}
	if !json.Valid(resp.RawResult) {
		t.Fatalf("raw result should be valid JSON")
	}
}

func TestWechatIDCardProviderRecognizeFront(t *testing.T) {
	provider := NewWechatIDCardProvider(stubWechatOCRClient{idCardResp: &wechat.IDCardOCRResponse{
		Name:   "张三",
		ID:     "110101199001011234",
		Gender: "男",
		Nation: "汉",
		Addr:   "测试路1号",
	}})
	resp, err := provider.Recognize(context.Background(), CapabilityWechatIDCard, RecognizeRequest{DocumentType: DocumentTypeIDCard, Side: DocumentSideFront, Data: []byte("img")})
	if err != nil {
		t.Fatalf("Recognize error = %v", err)
	}
	if resp.Normalized.IDCard == nil {
		t.Fatal("expected normalized id card result")
	}
	if resp.Normalized.IDCard.Name != "张三" {
		t.Fatalf("name = %s", resp.Normalized.IDCard.Name)
	}
	if resp.Normalized.IDCard.IDNumber != "110101199001011234" {
		t.Fatalf("id number = %s", resp.Normalized.IDCard.IDNumber)
	}
	if !json.Valid(resp.RawResult) {
		t.Fatalf("raw result should be valid JSON")
	}
}
