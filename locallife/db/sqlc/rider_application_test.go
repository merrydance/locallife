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

func createRandomRiderApplication(t *testing.T) RiderApplication {
	user := createRandomUser(t)
	return createRandomRiderApplicationWithUser(t, user.ID)
}

func createRandomRiderApplicationWithUser(t *testing.T, userID int64) RiderApplication {
	app, err := testStore.CreateRiderApplication(context.Background(), userID)
	require.NoError(t, err)
	require.NotEmpty(t, app)

	require.Equal(t, userID, app.UserID)
	require.Equal(t, "draft", app.Status)
	require.NotZero(t, app.ID)
	require.NotZero(t, app.CreatedAt)

	return app
}

// ==================== Create Tests ====================

func TestCreateRiderApplication(t *testing.T) {
	createRandomRiderApplication(t)
}

func TestCreateRiderApplication_DuplicateUserID(t *testing.T) {
	user := createRandomUser(t)

	// 第一次创建
	createRandomRiderApplicationWithUser(t, user.ID)

	// 同一用户重复创建应该失败（唯一约束：user_id）
	_, err := testStore.CreateRiderApplication(context.Background(), user.ID)
	require.Error(t, err, "同一用户重复创建申请应该返回错误")
	require.Equal(t, UniqueViolation, ErrorCode(err))
}

// ==================== Get Tests ====================

func TestGetRiderApplication(t *testing.T) {
	app1 := createRandomRiderApplication(t)

	app2, err := testStore.GetRiderApplication(context.Background(), app1.ID)
	require.NoError(t, err)
	require.NotEmpty(t, app2)

	require.Equal(t, app1.ID, app2.ID)
	require.Equal(t, app1.UserID, app2.UserID)
	require.Equal(t, app1.Status, app2.Status)
	require.WithinDuration(t, app1.CreatedAt, app2.CreatedAt, time.Second)
}

func TestGetRiderApplicationByUserID(t *testing.T) {
	user := createRandomUser(t)
	app1 := createRandomRiderApplicationWithUser(t, user.ID)

	app2, err := testStore.GetRiderApplicationByUserID(context.Background(), user.ID)
	require.NoError(t, err)
	require.NotEmpty(t, app2)

	require.Equal(t, app1.ID, app2.ID)
	require.Equal(t, app1.UserID, app2.UserID)
}

// ==================== Update Basic Info Tests ====================

func TestUpdateRiderApplicationBasicInfo(t *testing.T) {
	app := createRandomRiderApplication(t)

	realName := "张三"
	phone := "13812345678"

	updated, err := testStore.UpdateRiderApplicationBasicInfo(context.Background(), UpdateRiderApplicationBasicInfoParams{
		ID:       app.ID,
		RealName: pgtype.Text{String: realName, Valid: true},
		Phone:    pgtype.Text{String: phone, Valid: true},
	})
	require.NoError(t, err)
	require.Equal(t, realName, updated.RealName.String)
	require.Equal(t, phone, updated.Phone.String)
	require.True(t, updated.UpdatedAt.Valid)
}

func TestUpdateRiderApplicationBasicInfo_OnlyDraft(t *testing.T) {
	app := createRandomRiderApplication(t)

	// 先提交申请
	// 需要先填充必填字段
	_, err := testStore.UpdateRiderApplicationBasicInfo(context.Background(), UpdateRiderApplicationBasicInfoParams{
		ID:       app.ID,
		RealName: pgtype.Text{String: "张三", Valid: true},
		Phone:    pgtype.Text{String: "13812345678", Valid: true},
	})
	require.NoError(t, err)

	_, err = testStore.UpdateRiderApplicationIDCard(context.Background(), UpdateRiderApplicationIDCardParams{
		ID:             app.ID,
		IDCardFrontUrl: pgtype.Text{String: "front.jpg", Valid: true},
		IDCardBackUrl:  pgtype.Text{String: "back.jpg", Valid: true},
	})
	require.NoError(t, err)

	_, err = testStore.UpdateRiderApplicationHealthCert(context.Background(), UpdateRiderApplicationHealthCertParams{
		ID:            app.ID,
		HealthCertUrl: pgtype.Text{String: "health.jpg", Valid: true},
	})
	require.NoError(t, err)

	// 提交
	submitted, err := testStore.SubmitRiderApplication(context.Background(), app.ID)
	require.NoError(t, err)
	require.Equal(t, "submitted", submitted.Status)

	// 已提交状态不能更新
	_, err = testStore.UpdateRiderApplicationBasicInfo(context.Background(), UpdateRiderApplicationBasicInfoParams{
		ID:       app.ID,
		RealName: pgtype.Text{String: "李四", Valid: true},
	})
	require.Error(t, err, "已提交的申请不能修改")
}

// ==================== Update ID Card Tests ====================

func TestUpdateRiderApplicationIDCard(t *testing.T) {
	app := createRandomRiderApplication(t)

	ocrData := map[string]string{
		"name":      "张三",
		"id_number": "110101199001011234",
		"gender":    "男",
		"nation":    "汉",
		"valid_end": "20300101",
	}
	ocrJSON, _ := json.Marshal(ocrData)

	updated, err := testStore.UpdateRiderApplicationIDCard(context.Background(), UpdateRiderApplicationIDCardParams{
		ID:             app.ID,
		IDCardFrontUrl: pgtype.Text{String: "front.jpg", Valid: true},
		IDCardBackUrl:  pgtype.Text{String: "back.jpg", Valid: true},
		IDCardOcr:      ocrJSON,
		RealName:       pgtype.Text{String: "张三", Valid: true},
	})
	require.NoError(t, err)
	require.Equal(t, "front.jpg", updated.IDCardFrontUrl.String)
	require.Equal(t, "back.jpg", updated.IDCardBackUrl.String)
	require.NotEmpty(t, updated.IDCardOcr)
	require.Equal(t, "张三", updated.RealName.String)
}

// ==================== Update Health Cert Tests ====================

func TestUpdateRiderApplicationHealthCert(t *testing.T) {
	app := createRandomRiderApplication(t)

	updated, err := testStore.UpdateRiderApplicationHealthCert(context.Background(), UpdateRiderApplicationHealthCertParams{
		ID:            app.ID,
		HealthCertUrl: pgtype.Text{String: "health.jpg", Valid: true},
	})
	require.NoError(t, err)
	require.Equal(t, "health.jpg", updated.HealthCertUrl.String)
}

// ==================== Submit Tests ====================

func TestSubmitRiderApplication(t *testing.T) {
	app := createRandomRiderApplication(t)

	// 填充必填字段
	_, _ = testStore.UpdateRiderApplicationBasicInfo(context.Background(), UpdateRiderApplicationBasicInfoParams{
		ID:       app.ID,
		RealName: pgtype.Text{String: "张三", Valid: true},
		Phone:    pgtype.Text{String: "13812345678", Valid: true},
	})
	_, _ = testStore.UpdateRiderApplicationIDCard(context.Background(), UpdateRiderApplicationIDCardParams{
		ID:             app.ID,
		IDCardFrontUrl: pgtype.Text{String: "front.jpg", Valid: true},
		IDCardBackUrl:  pgtype.Text{String: "back.jpg", Valid: true},
	})
	_, _ = testStore.UpdateRiderApplicationHealthCert(context.Background(), UpdateRiderApplicationHealthCertParams{
		ID:            app.ID,
		HealthCertUrl: pgtype.Text{String: "health.jpg", Valid: true},
	})

	// 提交
	submitted, err := testStore.SubmitRiderApplication(context.Background(), app.ID)
	require.NoError(t, err)
	require.Equal(t, "submitted", submitted.Status)
	require.True(t, submitted.SubmittedAt.Valid)
}

// ==================== Approve Tests ====================

func TestApproveRiderApplication(t *testing.T) {
	app := createRandomRiderApplication(t)

	// 填充并提交
	_, _ = testStore.UpdateRiderApplicationBasicInfo(context.Background(), UpdateRiderApplicationBasicInfoParams{
		ID:       app.ID,
		RealName: pgtype.Text{String: "张三", Valid: true},
		Phone:    pgtype.Text{String: "13812345678", Valid: true},
	})
	_, _ = testStore.UpdateRiderApplicationIDCard(context.Background(), UpdateRiderApplicationIDCardParams{
		ID:             app.ID,
		IDCardFrontUrl: pgtype.Text{String: "front.jpg", Valid: true},
		IDCardBackUrl:  pgtype.Text{String: "back.jpg", Valid: true},
	})
	_, _ = testStore.UpdateRiderApplicationHealthCert(context.Background(), UpdateRiderApplicationHealthCertParams{
		ID:            app.ID,
		HealthCertUrl: pgtype.Text{String: "health.jpg", Valid: true},
	})
	_, _ = testStore.SubmitRiderApplication(context.Background(), app.ID)

	// 审核通过
	approved, err := testStore.ApproveRiderApplication(context.Background(), ApproveRiderApplicationParams{
		ID: app.ID,
	})
	require.NoError(t, err)
	require.Equal(t, "approved", approved.Status)
	require.True(t, approved.ReviewedAt.Valid)
}

// ==================== Reject Tests ====================

func TestRejectRiderApplication(t *testing.T) {
	app := createRandomRiderApplication(t)

	// 填充并提交
	_, _ = testStore.UpdateRiderApplicationBasicInfo(context.Background(), UpdateRiderApplicationBasicInfoParams{
		ID:       app.ID,
		RealName: pgtype.Text{String: "张三", Valid: true},
		Phone:    pgtype.Text{String: "13812345678", Valid: true},
	})
	_, _ = testStore.UpdateRiderApplicationIDCard(context.Background(), UpdateRiderApplicationIDCardParams{
		ID:             app.ID,
		IDCardFrontUrl: pgtype.Text{String: "front.jpg", Valid: true},
		IDCardBackUrl:  pgtype.Text{String: "back.jpg", Valid: true},
	})
	_, _ = testStore.UpdateRiderApplicationHealthCert(context.Background(), UpdateRiderApplicationHealthCertParams{
		ID:            app.ID,
		HealthCertUrl: pgtype.Text{String: "health.jpg", Valid: true},
	})
	_, _ = testStore.SubmitRiderApplication(context.Background(), app.ID)

	// 拒绝
	rejectReason := "身份证照片不清晰"
	rejected, err := testStore.RejectRiderApplication(context.Background(), RejectRiderApplicationParams{
		ID:           app.ID,
		RejectReason: pgtype.Text{String: rejectReason, Valid: true},
	})
	require.NoError(t, err)
	require.Equal(t, "rejected", rejected.Status)
	require.Equal(t, rejectReason, rejected.RejectReason.String)
}

// ==================== Reset Tests ====================

func TestResetRiderApplicationToDraft(t *testing.T) {
	app := createRandomRiderApplication(t)

	// 填充、提交、拒绝
	_, _ = testStore.UpdateRiderApplicationBasicInfo(context.Background(), UpdateRiderApplicationBasicInfoParams{
		ID:       app.ID,
		RealName: pgtype.Text{String: "张三", Valid: true},
		Phone:    pgtype.Text{String: "13812345678", Valid: true},
	})
	_, _ = testStore.UpdateRiderApplicationIDCard(context.Background(), UpdateRiderApplicationIDCardParams{
		ID:             app.ID,
		IDCardFrontUrl: pgtype.Text{String: "front.jpg", Valid: true},
		IDCardBackUrl:  pgtype.Text{String: "back.jpg", Valid: true},
	})
	_, _ = testStore.UpdateRiderApplicationHealthCert(context.Background(), UpdateRiderApplicationHealthCertParams{
		ID:            app.ID,
		HealthCertUrl: pgtype.Text{String: "health.jpg", Valid: true},
	})
	_, _ = testStore.SubmitRiderApplication(context.Background(), app.ID)
	_, _ = testStore.RejectRiderApplication(context.Background(), RejectRiderApplicationParams{
		ID:           app.ID,
		RejectReason: pgtype.Text{String: "照片不清晰", Valid: true},
	})

	// 重置为草稿
	reset, err := testStore.ResetRiderApplicationToDraft(context.Background(), app.ID)
	require.NoError(t, err)
	require.Equal(t, "draft", reset.Status)
	require.False(t, reset.RejectReason.Valid)
	require.False(t, reset.SubmittedAt.Valid)
}

// ==================== List Tests ====================

func TestListRiderApplications(t *testing.T) {
	// 创建几个申请
	for i := 0; i < 3; i++ {
		createRandomRiderApplication(t)
	}

	apps, err := testStore.ListRiderApplications(context.Background(), ListRiderApplicationsParams{
		Limit:  10,
		Offset: 0,
	})
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(apps), 3)
}

func TestListRiderApplicationsByStatus(t *testing.T) {
	// 创建一个提交的申请
	app := createRandomRiderApplication(t)
	_, _ = testStore.UpdateRiderApplicationBasicInfo(context.Background(), UpdateRiderApplicationBasicInfoParams{
		ID:       app.ID,
		RealName: pgtype.Text{String: "测试", Valid: true},
		Phone:    pgtype.Text{String: "13900000000", Valid: true},
	})
	_, _ = testStore.UpdateRiderApplicationIDCard(context.Background(), UpdateRiderApplicationIDCardParams{
		ID:             app.ID,
		IDCardFrontUrl: pgtype.Text{String: "f.jpg", Valid: true},
		IDCardBackUrl:  pgtype.Text{String: "b.jpg", Valid: true},
	})
	_, _ = testStore.UpdateRiderApplicationHealthCert(context.Background(), UpdateRiderApplicationHealthCertParams{
		ID:            app.ID,
		HealthCertUrl: pgtype.Text{String: "h.jpg", Valid: true},
	})
	_, _ = testStore.SubmitRiderApplication(context.Background(), app.ID)

	// 按状态筛选
	apps, err := testStore.ListRiderApplications(context.Background(), ListRiderApplicationsParams{
		Status: pgtype.Text{String: "submitted", Valid: true},
		Limit:  10,
		Offset: 0,
	})
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(apps), 1)

	for _, a := range apps {
		require.Equal(t, "submitted", a.Status)
	}
}

// ==================== Count Tests ====================

func TestCountRiderApplicationsByStatus(t *testing.T) {
	createRandomRiderApplication(t) // draft

	count, err := testStore.CountRiderApplicationsByStatus(context.Background(), "draft")
	require.NoError(t, err)
	require.GreaterOrEqual(t, count, int64(1))
}
