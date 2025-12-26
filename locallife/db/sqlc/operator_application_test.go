package db

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/require"
)

// ==================== Helper Functions ====================

func createRandomOperatorApplication(t *testing.T) OperatorApplication {
	user := createRandomUser(t)
	region := createRandomRegion(t)
	return createRandomOperatorApplicationWithUserAndRegion(t, user.ID, region.ID)
}

func createRandomOperatorApplicationWithUserAndRegion(t *testing.T, userID int64, regionID int64) OperatorApplication {
	app, err := testStore.CreateOperatorApplicationDraft(context.Background(), CreateOperatorApplicationDraftParams{
		UserID:   userID,
		RegionID: regionID,
	})
	require.NoError(t, err)
	require.NotEmpty(t, app)

	require.Equal(t, userID, app.UserID)
	require.Equal(t, regionID, app.RegionID)
	require.Equal(t, "draft", app.Status)
	require.NotZero(t, app.ID)
	require.NotZero(t, app.CreatedAt)

	return app
}

// ==================== Create Tests ====================

func TestCreateOperatorApplicationDraft(t *testing.T) {
	createRandomOperatorApplication(t)
}

func TestCreateOperatorApplicationDraft_DuplicateUserID(t *testing.T) {
	user := createRandomUser(t)
	region1 := createRandomRegion(t)
	region2 := createRandomRegion(t)

	// 第一次创建
	createRandomOperatorApplicationWithUserAndRegion(t, user.ID, region1.ID)

	// 同一用户重复创建应该失败（唯一约束：user_id）
	_, err := testStore.CreateOperatorApplicationDraft(context.Background(), CreateOperatorApplicationDraftParams{
		UserID:   user.ID,
		RegionID: region2.ID,
	})
	require.Error(t, err, "同一用户重复创建申请应该返回错误")
}

// ==================== Get Tests ====================

func TestGetOperatorApplicationByID(t *testing.T) {
	app1 := createRandomOperatorApplication(t)

	app2, err := testStore.GetOperatorApplicationByID(context.Background(), app1.ID)
	require.NoError(t, err)
	require.NotEmpty(t, app2)

	require.Equal(t, app1.ID, app2.ID)
	require.Equal(t, app1.UserID, app2.UserID)
	require.Equal(t, app1.RegionID, app2.RegionID)
	require.Equal(t, app1.Status, app2.Status)
	require.WithinDuration(t, app1.CreatedAt, app2.CreatedAt, time.Second)
}

func TestGetOperatorApplicationDraft(t *testing.T) {
	user := createRandomUser(t)
	region := createRandomRegion(t)
	app1 := createRandomOperatorApplicationWithUserAndRegion(t, user.ID, region.ID)

	// 获取草稿状态
	app2, err := testStore.GetOperatorApplicationDraft(context.Background(), user.ID)
	require.NoError(t, err)
	require.NotEmpty(t, app2)
	require.Equal(t, app1.ID, app2.ID)
}

func TestGetOperatorApplicationByUserID(t *testing.T) {
	user := createRandomUser(t)
	region := createRandomRegion(t)
	app1 := createRandomOperatorApplicationWithUserAndRegion(t, user.ID, region.ID)

	app2, err := testStore.GetOperatorApplicationByUserID(context.Background(), user.ID)
	require.NoError(t, err)
	require.NotEmpty(t, app2)

	require.Equal(t, app1.ID, app2.ID)
	require.Equal(t, app1.UserID, app2.UserID)
}

// ==================== Update Region Tests ====================

func TestUpdateOperatorApplicationRegion(t *testing.T) {
	user := createRandomUser(t)
	region1 := createRandomRegion(t)
	region2 := createRandomRegion(t)
	app := createRandomOperatorApplicationWithUserAndRegion(t, user.ID, region1.ID)

	// 更新区域
	updated, err := testStore.UpdateOperatorApplicationRegion(context.Background(), UpdateOperatorApplicationRegionParams{
		ID:       app.ID,
		RegionID: region2.ID,
	})
	require.NoError(t, err)
	require.Equal(t, region2.ID, updated.RegionID)
}

// ==================== Update Basic Info Tests ====================

func TestUpdateOperatorApplicationBasicInfo(t *testing.T) {
	app := createRandomOperatorApplication(t)

	operatorName := "测试运营商"
	contactName := "张三"
	contactPhone := "13812345678"

	updated, err := testStore.UpdateOperatorApplicationBasicInfo(context.Background(), UpdateOperatorApplicationBasicInfoParams{
		ID:           app.ID,
		Name:         pgtype.Text{String: operatorName, Valid: true},
		ContactName:  pgtype.Text{String: contactName, Valid: true},
		ContactPhone: pgtype.Text{String: contactPhone, Valid: true},
	})
	require.NoError(t, err)
	require.Equal(t, operatorName, updated.Name.String)
	require.Equal(t, contactName, updated.ContactName.String)
	require.Equal(t, contactPhone, updated.ContactPhone.String)
}

// ==================== Update Business License Tests ====================

func TestUpdateOperatorApplicationBusinessLicense(t *testing.T) {
	app := createRandomOperatorApplication(t)

	ocrData := map[string]string{
		"company_name":     "测试公司",
		"legal_person":     "张三",
		"credit_code":      "91310000MA1K8F2M6A",
		"registered_date":  "2020-01-01",
		"business_scope":   "餐饮服务",
	}
	ocrJSON, _ := json.Marshal(ocrData)

	updated, err := testStore.UpdateOperatorApplicationBusinessLicense(context.Background(), UpdateOperatorApplicationBusinessLicenseParams{
		ID:                    app.ID,
		BusinessLicenseUrl:    pgtype.Text{String: "https://example.com/license.jpg", Valid: true},
		BusinessLicenseNumber: pgtype.Text{String: "91310000MA1K8F2M6A", Valid: true},
		BusinessLicenseOcr:    ocrJSON,
	})
	require.NoError(t, err)
	require.True(t, updated.BusinessLicenseUrl.Valid)
	require.Equal(t, "91310000MA1K8F2M6A", updated.BusinessLicenseNumber.String)
	require.NotEmpty(t, updated.BusinessLicenseOcr)
}

// ==================== Update ID Card Front Tests ====================

func TestUpdateOperatorApplicationIDCardFront(t *testing.T) {
	app := createRandomOperatorApplication(t)

	ocrData := map[string]string{
		"name":      "张三",
		"id_number": "110101199001011234",
		"gender":    "男",
		"nation":    "汉",
	}
	ocrJSON, _ := json.Marshal(ocrData)

	updated, err := testStore.UpdateOperatorApplicationIDCardFront(context.Background(), UpdateOperatorApplicationIDCardFrontParams{
		ID:                  app.ID,
		IDCardFrontUrl:      pgtype.Text{String: "https://example.com/id_front.jpg", Valid: true},
		LegalPersonName:     pgtype.Text{String: "张三", Valid: true},
		LegalPersonIDNumber: pgtype.Text{String: "110101199001011234", Valid: true},
		IDCardFrontOcr:      ocrJSON,
	})
	require.NoError(t, err)
	require.True(t, updated.IDCardFrontUrl.Valid)
	require.Equal(t, "张三", updated.LegalPersonName.String)
	require.Equal(t, "110101199001011234", updated.LegalPersonIDNumber.String)
	require.NotEmpty(t, updated.IDCardFrontOcr)
}

// ==================== Update ID Card Back Tests ====================

func TestUpdateOperatorApplicationIDCardBack(t *testing.T) {
	app := createRandomOperatorApplication(t)

	ocrData := map[string]string{
		"valid_start": "2020-01-01",
		"valid_end":   "2030-01-01",
		"authority":   "深圳市公安局",
	}
	ocrJSON, _ := json.Marshal(ocrData)

	updated, err := testStore.UpdateOperatorApplicationIDCardBack(context.Background(), UpdateOperatorApplicationIDCardBackParams{
		ID:            app.ID,
		IDCardBackUrl: pgtype.Text{String: "https://example.com/id_back.jpg", Valid: true},
		IDCardBackOcr: ocrJSON,
	})
	require.NoError(t, err)
	require.True(t, updated.IDCardBackUrl.Valid)
	require.NotEmpty(t, updated.IDCardBackOcr)
}

// ==================== Status Transition Tests ====================

func createCompleteOperatorApplication(t *testing.T) OperatorApplication {
	app := createRandomOperatorApplication(t)

	// 填写基本信息和合同年限
	_, err := testStore.UpdateOperatorApplicationBasicInfo(context.Background(), UpdateOperatorApplicationBasicInfoParams{
		ID:                     app.ID,
		Name:                   pgtype.Text{String: "测试运营商", Valid: true},
		ContactName:            pgtype.Text{String: "张三", Valid: true},
		ContactPhone:           pgtype.Text{String: "13812345678", Valid: true},
		RequestedContractYears: pgtype.Int4{Int32: 3, Valid: true},
	})
	require.NoError(t, err)

	// 上传营业执照
	_, err = testStore.UpdateOperatorApplicationBusinessLicense(context.Background(), UpdateOperatorApplicationBusinessLicenseParams{
		ID:                    app.ID,
		BusinessLicenseUrl:    pgtype.Text{String: "https://example.com/license.jpg", Valid: true},
		BusinessLicenseNumber: pgtype.Text{String: "91310000MA1K8F2M6A", Valid: true},
		BusinessLicenseOcr:    []byte(`{"company_name": "测试公司"}`),
	})
	require.NoError(t, err)

	// 上传身份证正面
	_, err = testStore.UpdateOperatorApplicationIDCardFront(context.Background(), UpdateOperatorApplicationIDCardFrontParams{
		ID:                  app.ID,
		IDCardFrontUrl:      pgtype.Text{String: "https://example.com/id_front.jpg", Valid: true},
		LegalPersonName:     pgtype.Text{String: "张三", Valid: true},
		LegalPersonIDNumber: pgtype.Text{String: "110101199001011234", Valid: true},
		IDCardFrontOcr:      []byte(`{"name": "张三"}`),
	})
	require.NoError(t, err)

	// 上传身份证背面
	updated, err := testStore.UpdateOperatorApplicationIDCardBack(context.Background(), UpdateOperatorApplicationIDCardBackParams{
		ID:            app.ID,
		IDCardBackUrl: pgtype.Text{String: "https://example.com/id_back.jpg", Valid: true},
		IDCardBackOcr: []byte(`{"valid_end": "20300101"}`),
	})
	require.NoError(t, err)

	return updated
}

func TestSubmitOperatorApplication(t *testing.T) {
	app := createCompleteOperatorApplication(t)

	// 提交申请
	submitted, err := testStore.SubmitOperatorApplication(context.Background(), app.ID)
	require.NoError(t, err)
	require.Equal(t, "submitted", submitted.Status)
	require.True(t, submitted.SubmittedAt.Valid)
}

func TestApproveOperatorApplication(t *testing.T) {
	app := createCompleteOperatorApplication(t)

	// 提交
	submitted, err := testStore.SubmitOperatorApplication(context.Background(), app.ID)
	require.NoError(t, err)
	require.Equal(t, "submitted", submitted.Status)

	// 审核员
	reviewer := createRandomUser(t)

	// 审核通过
	approved, err := testStore.ApproveOperatorApplication(context.Background(), ApproveOperatorApplicationParams{
		ID:         app.ID,
		ReviewedBy: pgtype.Int8{Int64: reviewer.ID, Valid: true},
	})
	require.NoError(t, err)
	require.Equal(t, "approved", approved.Status)
	require.True(t, approved.ReviewedAt.Valid)
	require.Equal(t, reviewer.ID, approved.ReviewedBy.Int64)
}

func TestRejectOperatorApplication(t *testing.T) {
	app := createCompleteOperatorApplication(t)

	// 提交
	submitted, err := testStore.SubmitOperatorApplication(context.Background(), app.ID)
	require.NoError(t, err)
	require.Equal(t, "submitted", submitted.Status)

	// 审核员
	reviewer := createRandomUser(t)

	// 审核拒绝
	rejected, err := testStore.RejectOperatorApplication(context.Background(), RejectOperatorApplicationParams{
		ID:           app.ID,
		RejectReason: pgtype.Text{String: "资质不符合要求", Valid: true},
		ReviewedBy:   pgtype.Int8{Int64: reviewer.ID, Valid: true},
	})
	require.NoError(t, err)
	require.Equal(t, "rejected", rejected.Status)
	require.Equal(t, "资质不符合要求", rejected.RejectReason.String)
	require.True(t, rejected.ReviewedAt.Valid)
}

func TestResetOperatorApplicationToDraft(t *testing.T) {
	app := createCompleteOperatorApplication(t)

	// 提交
	submitted, err := testStore.SubmitOperatorApplication(context.Background(), app.ID)
	require.NoError(t, err)
	require.Equal(t, "submitted", submitted.Status)

	// 审核员
	reviewer := createRandomUser(t)

	// 拒绝
	rejected, err := testStore.RejectOperatorApplication(context.Background(), RejectOperatorApplicationParams{
		ID:           app.ID,
		RejectReason: pgtype.Text{String: "资质不符合要求", Valid: true},
		ReviewedBy:   pgtype.Int8{Int64: reviewer.ID, Valid: true},
	})
	require.NoError(t, err)
	require.Equal(t, "rejected", rejected.Status)

	// 重置为草稿
	reset, err := testStore.ResetOperatorApplicationToDraft(context.Background(), app.ID)
	require.NoError(t, err)
	require.Equal(t, "draft", reset.Status)
	// 注意：submitted_at 保留为历史记录，不会被清除
	require.False(t, reset.RejectReason.Valid, "reject_reason应该被清除")
}

// ==================== List Tests ====================

func TestListPendingOperatorApplications(t *testing.T) {
	// 创建多个待审核的申请
	for i := 0; i < 3; i++ {
		app := createCompleteOperatorApplication(t)
		_, err := testStore.SubmitOperatorApplication(context.Background(), app.ID)
		require.NoError(t, err)
	}

	// 列出待审核的申请
	apps, err := testStore.ListPendingOperatorApplications(context.Background(), ListPendingOperatorApplicationsParams{
		Limit:  10,
		Offset: 0,
	})
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(apps), 3, "应该至少有3个待审核的申请")

	// 验证返回的都是submitted状态
	for _, app := range apps {
		require.Equal(t, "submitted", app.Status)
	}
}

func TestCountPendingOperatorApplications(t *testing.T) {
	// 创建一个待审核的申请
	app := createCompleteOperatorApplication(t)
	_, err := testStore.SubmitOperatorApplication(context.Background(), app.ID)
	require.NoError(t, err)

	// 统计待审核的申请
	count, err := testStore.CountPendingOperatorApplications(context.Background())
	require.NoError(t, err)
	require.GreaterOrEqual(t, count, int64(1))
}

// ==================== Region Exclusivity Tests ====================

func TestGetPendingOperatorApplicationByRegion(t *testing.T) {
	user := createRandomUser(t)
	region := createRandomRegion(t)
	app := createRandomOperatorApplicationWithUserAndRegion(t, user.ID, region.ID)

	// 填写并提交申请
	_, err := testStore.UpdateOperatorApplicationBasicInfo(context.Background(), UpdateOperatorApplicationBasicInfoParams{
		ID:           app.ID,
		Name:         pgtype.Text{String: "测试运营商", Valid: true},
		ContactName:  pgtype.Text{String: "张三", Valid: true},
		ContactPhone: pgtype.Text{String: "13812345678", Valid: true},
	})
	require.NoError(t, err)

	_, err = testStore.UpdateOperatorApplicationRegion(context.Background(), UpdateOperatorApplicationRegionParams{
		ID:       app.ID,
		RegionID: region.ID,
	})
	require.NoError(t, err)

	_, err = testStore.UpdateOperatorApplicationBusinessLicense(context.Background(), UpdateOperatorApplicationBusinessLicenseParams{
		ID:                    app.ID,
		BusinessLicenseUrl:    pgtype.Text{String: "https://example.com/license.jpg", Valid: true},
		BusinessLicenseNumber: pgtype.Text{String: "91310000MA1K8F2M6A", Valid: true},
		BusinessLicenseOcr:    []byte(`{}`),
	})
	require.NoError(t, err)

	_, err = testStore.UpdateOperatorApplicationIDCardFront(context.Background(), UpdateOperatorApplicationIDCardFrontParams{
		ID:                  app.ID,
		IDCardFrontUrl:      pgtype.Text{String: "https://example.com/id_front.jpg", Valid: true},
		LegalPersonName:     pgtype.Text{String: "张三", Valid: true},
		LegalPersonIDNumber: pgtype.Text{String: "110101199001011234", Valid: true},
		IDCardFrontOcr:      []byte(`{}`),
	})
	require.NoError(t, err)

	_, err = testStore.UpdateOperatorApplicationIDCardBack(context.Background(), UpdateOperatorApplicationIDCardBackParams{
		ID:            app.ID,
		IDCardBackUrl: pgtype.Text{String: "https://example.com/id_back.jpg", Valid: true},
		IDCardBackOcr: []byte(`{}`),
	})
	require.NoError(t, err)

	_, err = testStore.SubmitOperatorApplication(context.Background(), app.ID)
	require.NoError(t, err)

	// 检查该区域是否有待审核的申请
	pendingApp, err := testStore.GetPendingOperatorApplicationByRegion(context.Background(), region.ID)
	require.NoError(t, err)
	require.Equal(t, app.ID, pendingApp.ID)
	require.Equal(t, region.ID, pendingApp.RegionID)
	require.Equal(t, "submitted", pendingApp.Status)
}

// ==================== Approved Application for Bindbank ====================

func TestGetApprovedOperatorApplicationByUserID(t *testing.T) {
	app := createCompleteOperatorApplication(t)

	// 提交
	submitted, err := testStore.SubmitOperatorApplication(context.Background(), app.ID)
	require.NoError(t, err)
	require.Equal(t, "submitted", submitted.Status)

	// 审核员
	reviewer := createRandomUser(t)

	// 审核通过
	approved, err := testStore.ApproveOperatorApplication(context.Background(), ApproveOperatorApplicationParams{
		ID:         app.ID,
		ReviewedBy: pgtype.Int8{Int64: reviewer.ID, Valid: true},
	})
	require.NoError(t, err)
	require.Equal(t, "approved", approved.Status)

	// 获取审核通过的申请（用于绑卡）
	fetched, err := testStore.GetApprovedOperatorApplicationByUserID(context.Background(), approved.UserID)
	require.NoError(t, err)
	require.Equal(t, app.ID, fetched.ID)
	require.Equal(t, "approved", fetched.Status)
	require.True(t, fetched.LegalPersonName.Valid)
	require.True(t, fetched.LegalPersonIDNumber.Valid)
}
