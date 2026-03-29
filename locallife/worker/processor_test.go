package worker

import (
	"context"
	"testing"
	"time"

	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/ocr"
	"github.com/merrydance/locallife/util"
	"go.uber.org/mock/gomock"
)

func TestNewMerchantApplicationOCRService_UsesAliyunForFoodPermit(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().
		UpsertOCRJob(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, arg db.UpsertOCRJobParams) (db.OcrJob, error) {
			if arg.DocumentType != string(ocr.DocumentTypeFoodPermit) {
				t.Fatalf("document type = %s", arg.DocumentType)
			}
			if arg.Provider != string(ocr.ProviderNameAliyun) {
				t.Fatalf("provider = %s", arg.Provider)
			}
			return db.OcrJob{ID: 1, Provider: arg.Provider, DocumentType: arg.DocumentType}, nil
		})

	service, err := newMerchantApplicationOCRService(store, nil, nil, util.Config{
		AliyunOCREnabled:         true,
		AliyunOCREndpoint:        "https://ocr-api.cn-hangzhou.aliyuncs.com",
		AliyunOCRRegion:          "cn-hangzhou",
		AliyunOCRAccessKeyID:     "test-ak",
		AliyunOCRAccessKeySecret: "test-sk",
		AliyunOCRHTTPTimeout:     time.Second,
	})
	if err != nil {
		t.Fatalf("newMerchantApplicationOCRService error = %v", err)
	}
	if service == nil {
		t.Fatal("expected OCR service")
	}

	_, err = service.CreateJob(context.Background(), ocr.CreateJobParams{
		DocumentType: ocr.DocumentTypeFoodPermit,
		MediaAssetID: 88,
		OwnerType:    ocr.OwnerTypeMerchantApplication,
		OwnerID:      66,
		RequestedBy:  1,
	})
	if err != nil {
		t.Fatalf("CreateJob error = %v", err)
	}
}
