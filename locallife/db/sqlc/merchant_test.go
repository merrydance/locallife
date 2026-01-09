package db

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/merrydance/locallife/util"
	"github.com/stretchr/testify/require"
)

// ==================== Helper Functions ====================

// createRandomRegion 创建测试用的区域
func createRandomRegion(t *testing.T) Region {
	arg := CreateRegionParams{
		Code:      util.RandomString(6),
		Name:      util.RandomString(10),
		Level:     1,
		ParentID:  pgtype.Int8{Valid: false},
		Longitude: pgtype.Numeric{},
		Latitude:  pgtype.Numeric{},
	}

	region, err := testStore.CreateRegion(context.Background(), arg)
	require.NoError(t, err)
	require.NotEmpty(t, region)
	return region
}

func createRandomMerchantForTest(t *testing.T) Merchant {
	user := createRandomUser(t)
	return createRandomMerchantWithOwner(t, user.ID)
}

func createRandomMerchantWithOwner(t *testing.T, ownerID int64) Merchant {
	// 首先创建一个区域用于测试
	region := createRandomRegion(t)

	appData, _ := json.Marshal(map[string]string{"test": "data"})
	arg := CreateMerchantParams{
		OwnerUserID:     ownerID,
		Name:            util.RandomString(10),
		Description:     pgtype.Text{String: util.RandomString(50), Valid: true},
		LogoUrl:         pgtype.Text{String: "https://example.com/logo.jpg", Valid: true},
		Phone:           "13800138000",
		Address:         util.RandomString(30),
		Latitude:        pgtype.Numeric{},
		Longitude:       pgtype.Numeric{},
		Status:          "approved",
		ApplicationData: appData,
		RegionID:        region.ID, // 添加区域ID
	}

	merchant, err := testStore.CreateMerchant(context.Background(), arg)
	require.NoError(t, err)
	require.NotEmpty(t, merchant)

	require.Equal(t, arg.OwnerUserID, merchant.OwnerUserID)
	require.Equal(t, arg.Name, merchant.Name)
	require.Equal(t, arg.Phone, merchant.Phone)
	require.NotZero(t, merchant.ID)
	require.NotZero(t, merchant.CreatedAt)

	return merchant
}

func createRandomMerchantApplication(t *testing.T) MerchantApplication {
	user := createRandomUser(t)
	return createRandomMerchantApplicationWithUser(t, user.ID)
}

func createRandomMerchantApplicationWithUser(t *testing.T, userID int64) MerchantApplication {
	arg := CreateMerchantApplicationParams{
		UserID:                  userID,
		MerchantName:            util.RandomString(10),
		BusinessLicenseNumber:   util.RandomString(18),
		BusinessLicenseImageUrl: "uploads/merchants/test/license.jpg",
		LegalPersonName:         util.RandomString(6),
		LegalPersonIDNumber:     "110101199001011234",
		LegalPersonIDFrontUrl:   "uploads/merchants/test/id_front.jpg",
		LegalPersonIDBackUrl:    "uploads/merchants/test/id_back.jpg",
		ContactPhone:            "13800138000",
		BusinessAddress:         util.RandomString(30),
		BusinessScope:           pgtype.Text{String: "餐饮服务", Valid: true},
	}

	application, err := testStore.CreateMerchantApplication(context.Background(), arg)
	require.NoError(t, err)
	require.NotEmpty(t, application)

	require.Equal(t, arg.UserID, application.UserID)
	require.Equal(t, arg.MerchantName, application.MerchantName)
	require.Equal(t, "draft", application.Status) // 新创建的申请默认是草稿状态
	require.NotZero(t, application.ID)

	return application
}

// ==================== Merchant Tests ====================

func TestCreateMerchant(t *testing.T) {
	createRandomMerchantForTest(t)
}

func TestGetMerchant(t *testing.T) {
	merchant1 := createRandomMerchantForTest(t)

	merchant2, err := testStore.GetMerchant(context.Background(), merchant1.ID)
	require.NoError(t, err)
	require.NotEmpty(t, merchant2)

	require.Equal(t, merchant1.ID, merchant2.ID)
	require.Equal(t, merchant1.Name, merchant2.Name)
	require.Equal(t, merchant1.OwnerUserID, merchant2.OwnerUserID)
}

func TestGetMerchantByOwner(t *testing.T) {
	merchant1 := createRandomMerchantForTest(t)

	merchant2, err := testStore.GetMerchantByOwner(context.Background(), merchant1.OwnerUserID)
	require.NoError(t, err)
	require.NotEmpty(t, merchant2)

	require.Equal(t, merchant1.ID, merchant2.ID)
	require.Equal(t, merchant1.OwnerUserID, merchant2.OwnerUserID)
}

func TestListAllMerchants(t *testing.T) {
	// 创建多个商户
	for i := 0; i < 3; i++ {
		createRandomMerchantForTest(t)
	}

	arg := ListAllMerchantsParams{
		Limit:  10,
		Offset: 0,
	}

	merchants, err := testStore.ListAllMerchants(context.Background(), arg)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(merchants), 3)
}

func TestListMerchants(t *testing.T) {
	// 创建已批准的商户
	for i := 0; i < 3; i++ {
		createRandomMerchantForTest(t)
	}

	arg := ListMerchantsParams{
		Status: "approved",
		Limit:  10,
		Offset: 0,
	}

	merchants, err := testStore.ListMerchants(context.Background(), arg)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(merchants), 3)

	for _, m := range merchants {
		require.Equal(t, "approved", m.Status)
	}
}

func TestUpdateMerchant(t *testing.T) {
	merchant1 := createRandomMerchantForTest(t)

	newName := util.RandomString(10)
	newPhone := "13900139000"

	arg := UpdateMerchantParams{
		ID:          merchant1.ID,
		Version:     merchant1.Version, // ✅ 必须传入当前version
		Name:        pgtype.Text{String: newName, Valid: true},
		Phone:       pgtype.Text{String: newPhone, Valid: true},
		Description: pgtype.Text{}, // 不更新
		LogoUrl:     pgtype.Text{},
		Address:     pgtype.Text{},
		Latitude:    pgtype.Numeric{},
		Longitude:   pgtype.Numeric{},
	}

	merchant2, err := testStore.UpdateMerchant(context.Background(), arg)
	require.NoError(t, err)
	require.NotEmpty(t, merchant2)

	require.Equal(t, merchant1.ID, merchant2.ID)
	require.Equal(t, newName, merchant2.Name)
	require.Equal(t, newPhone, merchant2.Phone)
	require.Equal(t, merchant1.Description, merchant2.Description) // 未变
	require.Equal(t, merchant1.Version+1, merchant2.Version)       // ✅ version应该+1
}

func TestUpdateMerchantStatus(t *testing.T) {
	merchant1 := createRandomMerchantForTest(t)

	arg := UpdateMerchantStatusParams{
		ID:     merchant1.ID,
		Status: "suspended",
	}

	merchant2, err := testStore.UpdateMerchantStatus(context.Background(), arg)
	require.NoError(t, err)
	require.Equal(t, "suspended", merchant2.Status)
}

func TestDeleteMerchant(t *testing.T) {
	merchant := createRandomMerchantForTest(t)

	err := testStore.DeleteMerchant(context.Background(), merchant.ID)
	require.NoError(t, err)

	// 验证已删除
	_, err = testStore.GetMerchant(context.Background(), merchant.ID)
	require.Error(t, err)
}

func TestSearchMerchants(t *testing.T) {
	// 创建带有特定名称的商户
	user := createRandomUser(t)
	region := createRandomRegion(t)
	uniqueName := "SEARCHABLE_MERCHANT_" + util.RandomString(5)

	appData, _ := json.Marshal(map[string]string{"test": "data"})
	arg := CreateMerchantParams{
		OwnerUserID:     user.ID,
		Name:            uniqueName,
		Phone:           fmt.Sprintf("138%08d", util.RandomInt(10000000, 99999999)),
		Address:         "test address " + util.RandomString(10),
		Status:          "active",
		ApplicationData: appData,
		RegionID:        region.ID,
	}
	_, err := testStore.CreateMerchant(context.Background(), arg)
	require.NoError(t, err)

	// 搜索
	searchArg := SearchMerchantsParams{
		Offset:  0,
		Limit:   10,
		Column3: "SEARCHABLE_MERCHANT",
		Column4: 0,
		Column5: 0,
	}

	merchants, err := testStore.SearchMerchants(context.Background(), searchArg)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(merchants), 1)
}

func TestCountSearchMerchants(t *testing.T) {
	// 创建多个带有特定前缀的商户
	prefix := "COUNT_TEST_MERCHANT_" + util.RandomString(4) + "_"
	region := createRandomRegion(t)
	appData, _ := json.Marshal(map[string]string{"test": "data"})

	for i := 0; i < 3; i++ {
		user := createRandomUser(t)
		arg := CreateMerchantParams{
			OwnerUserID:     user.ID,
			Name:            prefix + util.RandomString(4),
			Phone:           fmt.Sprintf("138%08d", util.RandomInt(10000000, 99999999)),
			Address:         "test address " + util.RandomString(10),
			Status:          "approved",
			ApplicationData: appData,
			RegionID:        region.ID,
		}
		_, err := testStore.CreateMerchant(context.Background(), arg)
		require.NoError(t, err)
	}

	// 计数
	count, err := testStore.CountSearchMerchants(context.Background(), pgtype.Text{String: prefix, Valid: true})
	require.NoError(t, err)
	require.Equal(t, int64(3), count)
}

func TestCountSearchMerchants_EmptyResult(t *testing.T) {
	count, err := testStore.CountSearchMerchants(context.Background(), pgtype.Text{String: "NonExistentMerchantName99999", Valid: true})
	require.NoError(t, err)
	require.Equal(t, int64(0), count)
}

func TestCountSearchMerchants_PartialMatch(t *testing.T) {
	// 创建商户
	prefix := "PARTIAL_MATCH_" + util.RandomString(6)
	region := createRandomRegion(t)
	user := createRandomUser(t)
	appData, _ := json.Marshal(map[string]string{"test": "data"})

	arg := CreateMerchantParams{
		OwnerUserID:     user.ID,
		Name:            prefix + "_SUFFIX",
		Phone:           fmt.Sprintf("138%08d", util.RandomInt(10000000, 99999999)),
		Address:         "test address " + util.RandomString(10),
		Status:          "approved",
		ApplicationData: appData,
		RegionID:        region.ID,
	}
	_, err := testStore.CreateMerchant(context.Background(), arg)
	require.NoError(t, err)

	// 部分匹配应该能找到
	count, err := testStore.CountSearchMerchants(context.Background(), pgtype.Text{String: prefix, Valid: true})
	require.NoError(t, err)
	require.Equal(t, int64(1), count)
}

// ==================== Merchant Application Tests ====================

func TestCreateMerchantApplication(t *testing.T) {
	createRandomMerchantApplication(t)
}

func TestGetMerchantApplication(t *testing.T) {
	app1 := createRandomMerchantApplication(t)

	app2, err := testStore.GetMerchantApplication(context.Background(), app1.ID)
	require.NoError(t, err)
	require.NotEmpty(t, app2)

	require.Equal(t, app1.ID, app2.ID)
	require.Equal(t, app1.MerchantName, app2.MerchantName)
}

func TestGetUserMerchantApplication(t *testing.T) {
	app1 := createRandomMerchantApplication(t)

	app2, err := testStore.GetUserMerchantApplication(context.Background(), app1.UserID)
	require.NoError(t, err)
	require.NotEmpty(t, app2)

	require.Equal(t, app1.ID, app2.ID)
	require.Equal(t, app1.UserID, app2.UserID)
}

func TestGetMerchantApplicationByLicenseNumber(t *testing.T) {
	app1 := createRandomMerchantApplication(t)

	app2, err := testStore.GetMerchantApplicationByLicenseNumber(context.Background(), app1.BusinessLicenseNumber)
	require.NoError(t, err)
	require.NotEmpty(t, app2)

	require.Equal(t, app1.ID, app2.ID)
	require.Equal(t, app1.BusinessLicenseNumber, app2.BusinessLicenseNumber)
}

func TestListAllMerchantApplications(t *testing.T) {
	// 创建多个申请
	for i := 0; i < 3; i++ {
		createRandomMerchantApplication(t)
	}

	arg := ListAllMerchantApplicationsParams{
		Limit:  10,
		Offset: 0,
	}

	apps, err := testStore.ListAllMerchantApplications(context.Background(), arg)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(apps), 3)
}

func TestListMerchantApplications(t *testing.T) {
	// 创建draft状态的申请（新创建的申请默认是draft状态）
	for i := 0; i < 3; i++ {
		createRandomMerchantApplication(t)
	}

	arg := ListMerchantApplicationsParams{
		Status: "draft", // 新创建的申请默认是draft状态
		Limit:  10,
		Offset: 0,
	}

	apps, err := testStore.ListMerchantApplications(context.Background(), arg)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(apps), 3)

	for _, app := range apps {
		require.Equal(t, "draft", app.Status)
	}
}

func TestUpdateMerchantApplicationStatus(t *testing.T) {
	app1 := createRandomMerchantApplication(t)
	reviewer := createRandomUser(t)

	arg := UpdateMerchantApplicationStatusParams{
		ID:           app1.ID,
		Status:       "approved",
		RejectReason: pgtype.Text{},
		ReviewedBy:   pgtype.Int8{Int64: reviewer.ID, Valid: true},
		ReviewedAt:   pgtype.Timestamptz{Time: time.Now(), Valid: true},
	}

	app2, err := testStore.UpdateMerchantApplicationStatus(context.Background(), arg)
	require.NoError(t, err)
	require.Equal(t, "approved", app2.Status)
	require.True(t, app2.ReviewedBy.Valid)
	require.Equal(t, reviewer.ID, app2.ReviewedBy.Int64)
}

// ==================== Business Hours Tests ====================

func TestCreateBusinessHour(t *testing.T) {
	merchant := createRandomMerchantForTest(t)

	arg := CreateBusinessHourParams{
		MerchantID:  merchant.ID,
		DayOfWeek:   1,                                                           // Monday
		OpenTime:    pgtype.Time{Microseconds: 9 * 3600 * 1000000, Valid: true},  // 09:00
		CloseTime:   pgtype.Time{Microseconds: 21 * 3600 * 1000000, Valid: true}, // 21:00
		IsClosed:    false,
		SpecialDate: pgtype.Date{},
	}

	hour, err := testStore.CreateBusinessHour(context.Background(), arg)
	require.NoError(t, err)
	require.NotEmpty(t, hour)

	require.Equal(t, merchant.ID, hour.MerchantID)
	require.Equal(t, int32(1), hour.DayOfWeek)
	require.False(t, hour.IsClosed)
}

func TestGetBusinessHour(t *testing.T) {
	merchant := createRandomMerchantForTest(t)

	arg := CreateBusinessHourParams{
		MerchantID:  merchant.ID,
		DayOfWeek:   2,
		OpenTime:    pgtype.Time{Microseconds: 9 * 3600 * 1000000, Valid: true},
		CloseTime:   pgtype.Time{Microseconds: 21 * 3600 * 1000000, Valid: true},
		IsClosed:    false,
		SpecialDate: pgtype.Date{},
	}

	hour1, err := testStore.CreateBusinessHour(context.Background(), arg)
	require.NoError(t, err)

	hour2, err := testStore.GetBusinessHour(context.Background(), hour1.ID)
	require.NoError(t, err)

	require.Equal(t, hour1.ID, hour2.ID)
	require.Equal(t, hour1.MerchantID, hour2.MerchantID)
}

func TestGetBusinessHourByDayOfWeek(t *testing.T) {
	merchant := createRandomMerchantForTest(t)

	arg := CreateBusinessHourParams{
		MerchantID:  merchant.ID,
		DayOfWeek:   3, // Wednesday
		OpenTime:    pgtype.Time{Microseconds: 10 * 3600 * 1000000, Valid: true},
		CloseTime:   pgtype.Time{Microseconds: 22 * 3600 * 1000000, Valid: true},
		IsClosed:    false,
		SpecialDate: pgtype.Date{},
	}

	_, err := testStore.CreateBusinessHour(context.Background(), arg)
	require.NoError(t, err)

	getArg := GetBusinessHourByDayOfWeekParams{
		MerchantID: merchant.ID,
		DayOfWeek:  3,
	}

	hour, err := testStore.GetBusinessHourByDayOfWeek(context.Background(), getArg)
	require.NoError(t, err)
	require.Equal(t, merchant.ID, hour.MerchantID)
	require.Equal(t, int32(3), hour.DayOfWeek)
}

func TestListMerchantBusinessHours(t *testing.T) {
	merchant := createRandomMerchantForTest(t)

	// 创建多个营业时间
	for i := 1; i <= 5; i++ {
		arg := CreateBusinessHourParams{
			MerchantID:  merchant.ID,
			DayOfWeek:   int32(i),
			OpenTime:    pgtype.Time{Microseconds: 9 * 3600 * 1000000, Valid: true},
			CloseTime:   pgtype.Time{Microseconds: 21 * 3600 * 1000000, Valid: true},
			IsClosed:    false,
			SpecialDate: pgtype.Date{},
		}
		_, err := testStore.CreateBusinessHour(context.Background(), arg)
		require.NoError(t, err)
	}

	hours, err := testStore.ListMerchantBusinessHours(context.Background(), merchant.ID)
	require.NoError(t, err)
	require.Len(t, hours, 5)
}

func TestUpdateBusinessHour(t *testing.T) {
	merchant := createRandomMerchantForTest(t)

	createArg := CreateBusinessHourParams{
		MerchantID:  merchant.ID,
		DayOfWeek:   6,
		OpenTime:    pgtype.Time{Microseconds: 9 * 3600 * 1000000, Valid: true},
		CloseTime:   pgtype.Time{Microseconds: 21 * 3600 * 1000000, Valid: true},
		IsClosed:    false,
		SpecialDate: pgtype.Date{},
	}

	hour1, err := testStore.CreateBusinessHour(context.Background(), createArg)
	require.NoError(t, err)

	updateArg := UpdateBusinessHourParams{
		ID:        hour1.ID,
		OpenTime:  pgtype.Time{Microseconds: 10 * 3600 * 1000000, Valid: true},
		CloseTime: pgtype.Time{Microseconds: 22 * 3600 * 1000000, Valid: true},
		IsClosed:  true,
	}

	hour2, err := testStore.UpdateBusinessHour(context.Background(), updateArg)
	require.NoError(t, err)
	require.True(t, hour2.IsClosed)
}

func TestDeleteBusinessHour(t *testing.T) {
	merchant := createRandomMerchantForTest(t)

	arg := CreateBusinessHourParams{
		MerchantID:  merchant.ID,
		DayOfWeek:   7,
		OpenTime:    pgtype.Time{Microseconds: 9 * 3600 * 1000000, Valid: true},
		CloseTime:   pgtype.Time{Microseconds: 21 * 3600 * 1000000, Valid: true},
		IsClosed:    false,
		SpecialDate: pgtype.Date{},
	}

	hour, err := testStore.CreateBusinessHour(context.Background(), arg)
	require.NoError(t, err)

	err = testStore.DeleteBusinessHour(context.Background(), hour.ID)
	require.NoError(t, err)

	// 验证已删除
	_, err = testStore.GetBusinessHour(context.Background(), hour.ID)
	require.Error(t, err)
}

func TestDeleteMerchantBusinessHours(t *testing.T) {
	merchant := createRandomMerchantForTest(t)

	// 创建多个营业时间
	for i := 1; i <= 3; i++ {
		arg := CreateBusinessHourParams{
			MerchantID:  merchant.ID,
			DayOfWeek:   int32(i),
			OpenTime:    pgtype.Time{Microseconds: 9 * 3600 * 1000000, Valid: true},
			CloseTime:   pgtype.Time{Microseconds: 21 * 3600 * 1000000, Valid: true},
			IsClosed:    false,
			SpecialDate: pgtype.Date{},
		}
		_, err := testStore.CreateBusinessHour(context.Background(), arg)
		require.NoError(t, err)
	}

	// 删除所有营业时间
	err := testStore.DeleteMerchantBusinessHours(context.Background(), merchant.ID)
	require.NoError(t, err)

	// 验证已清空
	hours, err := testStore.ListMerchantBusinessHours(context.Background(), merchant.ID)
	require.NoError(t, err)
	require.Empty(t, hours)
}

// ==================== Merchant Tags Tests ====================

func TestAddMerchantTag(t *testing.T) {
	merchant := createRandomMerchantForTest(t)
	tag := createRandomTag(t, "merchant")

	arg := AddMerchantTagParams{
		MerchantID: merchant.ID,
		TagID:      tag.ID,
	}

	err := testStore.AddMerchantTag(context.Background(), arg)
	require.NoError(t, err)
}

func TestListMerchantTags(t *testing.T) {
	merchant := createRandomMerchantForTest(t)

	// 添加多个标签
	for i := 0; i < 3; i++ {
		tag := createRandomTag(t, "merchant")
		arg := AddMerchantTagParams{
			MerchantID: merchant.ID,
			TagID:      tag.ID,
		}
		err := testStore.AddMerchantTag(context.Background(), arg)
		require.NoError(t, err)
	}

	tags, err := testStore.ListMerchantTags(context.Background(), merchant.ID)
	require.NoError(t, err)
	require.Len(t, tags, 3)
}

func TestRemoveMerchantTag(t *testing.T) {
	merchant := createRandomMerchantForTest(t)
	tag := createRandomTag(t, "merchant")

	addArg := AddMerchantTagParams{
		MerchantID: merchant.ID,
		TagID:      tag.ID,
	}
	err := testStore.AddMerchantTag(context.Background(), addArg)
	require.NoError(t, err)

	// 移除标签
	removeArg := RemoveMerchantTagParams{
		MerchantID: merchant.ID,
		TagID:      tag.ID,
	}
	err = testStore.RemoveMerchantTag(context.Background(), removeArg)
	require.NoError(t, err)

	// 验证标签已移除
	tags, err := testStore.ListMerchantTags(context.Background(), merchant.ID)
	require.NoError(t, err)
	require.Empty(t, tags)
}

func TestClearMerchantTags(t *testing.T) {
	merchant := createRandomMerchantForTest(t)

	// 添加多个标签
	for i := 0; i < 3; i++ {
		tag := createRandomTag(t, "merchant")
		arg := AddMerchantTagParams{
			MerchantID: merchant.ID,
			TagID:      tag.ID,
		}
		err := testStore.AddMerchantTag(context.Background(), arg)
		require.NoError(t, err)
	}

	// 清除所有标签
	err := testStore.ClearMerchantTags(context.Background(), merchant.ID)
	require.NoError(t, err)

	// 验证已清空
	tags, err := testStore.ListMerchantTags(context.Background(), merchant.ID)
	require.NoError(t, err)
	require.Empty(t, tags)
}

func TestGetMerchantWithTags(t *testing.T) {
	merchant := createRandomMerchantForTest(t)

	// 添加标签
	tag := createRandomTag(t, "merchant")
	addArg := AddMerchantTagParams{
		MerchantID: merchant.ID,
		TagID:      tag.ID,
	}
	err := testStore.AddMerchantTag(context.Background(), addArg)
	require.NoError(t, err)

	// 获取商户带标签
	result, err := testStore.GetMerchantWithTags(context.Background(), merchant.ID)
	require.NoError(t, err)
	require.Equal(t, merchant.ID, result.Merchant.ID)
	require.NotEmpty(t, result.Tags)
}

func TestListMerchantsByTag(t *testing.T) {
	tag := createRandomTag(t, "merchant")

	// 创建多个商户并关联标签
	for i := 0; i < 3; i++ {
		merchant := createRandomMerchantForTest(t)
		addArg := AddMerchantTagParams{
			MerchantID: merchant.ID,
			TagID:      tag.ID,
		}
		err := testStore.AddMerchantTag(context.Background(), addArg)
		require.NoError(t, err)
	}

	arg := ListMerchantsByTagParams{
		TagID:  tag.ID,
		Limit:  10,
		Offset: 0,
	}

	merchants, err := testStore.ListMerchantsByTag(context.Background(), arg)
	require.NoError(t, err)
	require.Len(t, merchants, 3)
}

func TestListMerchantsWithTagCount(t *testing.T) {
	// 创建商户并添加不同数量的标签
	for i := 0; i < 3; i++ {
		merchant := createRandomMerchantForTest(t)
		for j := 0; j <= i; j++ {
			tag := createRandomTag(t, "merchant")
			addArg := AddMerchantTagParams{
				MerchantID: merchant.ID,
				TagID:      tag.ID,
			}
			err := testStore.AddMerchantTag(context.Background(), addArg)
			require.NoError(t, err)
		}
	}

	arg := ListMerchantsWithTagCountParams{
		Status: "approved",
		Limit:  10,
		Offset: 0,
	}

	merchants, err := testStore.ListMerchantsWithTagCount(context.Background(), arg)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(merchants), 3)
}

func TestGetPopularMerchants(t *testing.T) {
	// 这个测试的价值在于：确保查询能在真实 DB 上执行成功。
	// sqlc 生成代码时不会校验 SQL 是否引用了不存在的列，所以必须通过集成测试兜底。
	_ = createRandomMerchantForTest(t)

	rows, err := testStore.GetPopularMerchants(context.Background(), GetPopularMerchantsParams{
		Limit:   10,
		Column2: 0, // Lat
		Column3: 0, // Lng
	})
	require.NoError(t, err)
	// 至少应返回 1 条（刚创建的是 approved 商户）
	require.NotEmpty(t, rows)
}

// ============================================
// GetMerchantsWithStatsByIDs 测试（推荐流用）
// ============================================

func TestGetMerchantsWithStatsByIDs(t *testing.T) {
	// 创建商户
	merchant1 := createRandomMerchantForTest(t)
	merchant2 := createRandomMerchantForTest(t)

	// 批量查询
	merchantIDs := []int64{merchant1.ID, merchant2.ID}
	results, err := testStore.GetMerchantsWithStatsByIDs(context.Background(), merchantIDs)
	require.NoError(t, err)
	require.Len(t, results, 2)

	// 验证返回数据结构
	for _, r := range results {
		require.NotZero(t, r.ID)
		require.NotEmpty(t, r.Name)
		require.NotEmpty(t, r.Address)
		require.NotZero(t, r.RegionID)
		require.Equal(t, "approved", r.Status)
		// TrustScore 默认是500
		require.GreaterOrEqual(t, r.TrustScore, int16(0))
		// MonthlyOrders 新商户应该是0
		require.GreaterOrEqual(t, r.MonthlyOrders, int32(0))
	}
}

func TestGetMerchantsWithStatsByIDs_WithTrustScore(t *testing.T) {
	merchant := createRandomMerchantForTest(t)

	// 创建商户档案（包含信任分）
	trustScore := int16(800)
	_, err := testStore.CreateMerchantProfile(context.Background(), CreateMerchantProfileParams{
		MerchantID: merchant.ID,
		TrustScore: trustScore,
	})
	require.NoError(t, err)

	// 查询
	results, err := testStore.GetMerchantsWithStatsByIDs(context.Background(), []int64{merchant.ID})
	require.NoError(t, err)
	require.Len(t, results, 1)

	// 验证信任分
	require.Equal(t, trustScore, results[0].TrustScore)
}

func TestGetMerchantsWithStatsByIDs_EmptyIDs(t *testing.T) {
	results, err := testStore.GetMerchantsWithStatsByIDs(context.Background(), []int64{})
	require.NoError(t, err)
	require.Empty(t, results)
}

func TestGetMerchantsWithStatsByIDs_NonExistentIDs(t *testing.T) {
	results, err := testStore.GetMerchantsWithStatsByIDs(context.Background(), []int64{999999999})
	require.NoError(t, err)
	require.Empty(t, results)
}

func TestGetMerchantsWithStatsByIDs_FilterNonApproved(t *testing.T) {
	user := createRandomUser(t)
	region := createRandomRegion(t)

	// 创建一个待审核的商户
	arg := CreateMerchantParams{
		OwnerUserID: user.ID,
		Name:        "待审核商户_" + util.RandomString(6),
		Phone:       "13800138001",
		Address:     util.RandomString(30), // 使用随机地址避免唯一约束冲突
		Status:      "pending",             // 待审核
		RegionID:    region.ID,
	}
	pendingMerchant, err := testStore.CreateMerchant(context.Background(), arg)
	require.NoError(t, err)

	// 查询应该过滤掉非approved的商户
	results, err := testStore.GetMerchantsWithStatsByIDs(context.Background(), []int64{pendingMerchant.ID})
	require.NoError(t, err)
	require.Empty(t, results, "非approved商户不应被返回")
}

func TestGetMerchantsWithStatsByIDs_VerifyRegionID(t *testing.T) {
	// 确保RegionID正确返回（用于运费计算）
	merchant := createRandomMerchantForTest(t)

	results, err := testStore.GetMerchantsWithStatsByIDs(context.Background(), []int64{merchant.ID})
	require.NoError(t, err)
	require.Len(t, results, 1)

	// RegionID必须存在且大于0（用于运费计算）
	require.NotZero(t, results[0].RegionID, "RegionID必须存在，用于运费计算")
}

// ==================== 运营商商户管理集成测试 ====================

func createMerchantInRegion(t *testing.T, regionID int64, status string) Merchant {
	user := createRandomUser(t)
	appData, _ := json.Marshal(map[string]string{"test": "data"})
	arg := CreateMerchantParams{
		OwnerUserID:     user.ID,
		Name:            util.RandomString(10),
		Description:     pgtype.Text{String: util.RandomString(50), Valid: true},
		LogoUrl:         pgtype.Text{String: "https://example.com/logo.jpg", Valid: true},
		Phone:           "13800138000",
		Address:         util.RandomString(30),
		Latitude:        pgtype.Numeric{},
		Longitude:       pgtype.Numeric{},
		Status:          status,
		ApplicationData: appData,
		RegionID:        regionID,
	}

	merchant, err := testStore.CreateMerchant(context.Background(), arg)
	require.NoError(t, err)
	require.NotEmpty(t, merchant)
	return merchant
}

func TestListMerchantsByRegion(t *testing.T) {
	region := createRandomRegion(t)

	// 在该区域创建3个商户
	for i := 0; i < 3; i++ {
		createMerchantInRegion(t, region.ID, "approved")
	}

	// 查询该区域商户
	merchants, err := testStore.ListMerchantsByRegion(context.Background(), ListMerchantsByRegionParams{
		RegionID: region.ID,
		Limit:    10,
		Offset:   0,
	})
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(merchants), 3)

	// 验证所有商户都属于该区域
	for _, m := range merchants {
		require.Equal(t, region.ID, m.RegionID)
	}
}

func TestListMerchantsByRegion_Pagination(t *testing.T) {
	region := createRandomRegion(t)

	// 在该区域创建5个商户
	for i := 0; i < 5; i++ {
		createMerchantInRegion(t, region.ID, "approved")
	}

	// 第一页
	page1, err := testStore.ListMerchantsByRegion(context.Background(), ListMerchantsByRegionParams{
		RegionID: region.ID,
		Limit:    2,
		Offset:   0,
	})
	require.NoError(t, err)
	require.Len(t, page1, 2)

	// 第二页
	page2, err := testStore.ListMerchantsByRegion(context.Background(), ListMerchantsByRegionParams{
		RegionID: region.ID,
		Limit:    2,
		Offset:   2,
	})
	require.NoError(t, err)
	require.Len(t, page2, 2)

	// 验证两页不重复
	require.NotEqual(t, page1[0].ID, page2[0].ID)
}

func TestCountMerchantsByRegion(t *testing.T) {
	region := createRandomRegion(t)

	// 创建3个商户
	for i := 0; i < 3; i++ {
		createMerchantInRegion(t, region.ID, "approved")
	}

	count, err := testStore.CountMerchantsByRegion(context.Background(), region.ID)
	require.NoError(t, err)
	require.GreaterOrEqual(t, count, int64(3))
}

func TestListMerchantsByRegionWithStatus(t *testing.T) {
	region := createRandomRegion(t)

	// 创建不同状态的商户
	createMerchantInRegion(t, region.ID, "approved")
	createMerchantInRegion(t, region.ID, "approved")
	createMerchantInRegion(t, region.ID, "suspended")

	// 只查询approved状态
	approvedMerchants, err := testStore.ListMerchantsByRegionWithStatus(context.Background(), ListMerchantsByRegionWithStatusParams{
		RegionID: region.ID,
		Column2:  "approved",
		Limit:    10,
		Offset:   0,
	})
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(approvedMerchants), 2)
	for _, m := range approvedMerchants {
		require.Equal(t, "approved", m.Status)
	}

	// 只查询suspended状态
	suspendedMerchants, err := testStore.ListMerchantsByRegionWithStatus(context.Background(), ListMerchantsByRegionWithStatusParams{
		RegionID: region.ID,
		Column2:  "suspended",
		Limit:    10,
		Offset:   0,
	})
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(suspendedMerchants), 1)
	for _, m := range suspendedMerchants {
		require.Equal(t, "suspended", m.Status)
	}
}

func TestCountMerchantsByRegionWithStatus(t *testing.T) {
	region := createRandomRegion(t)

	// 创建不同状态的商户
	createMerchantInRegion(t, region.ID, "approved")
	createMerchantInRegion(t, region.ID, "approved")
	createMerchantInRegion(t, region.ID, "suspended")

	approvedCount, err := testStore.CountMerchantsByRegionWithStatus(context.Background(), CountMerchantsByRegionWithStatusParams{
		RegionID: region.ID,
		Column2:  "approved",
	})
	require.NoError(t, err)
	require.GreaterOrEqual(t, approvedCount, int64(2))

	suspendedCount, err := testStore.CountMerchantsByRegionWithStatus(context.Background(), CountMerchantsByRegionWithStatusParams{
		RegionID: region.ID,
		Column2:  "suspended",
	})
	require.NoError(t, err)
	require.GreaterOrEqual(t, suspendedCount, int64(1))
}

func TestListMerchantsByRegion_ExcludesDeleted(t *testing.T) {
	region := createRandomRegion(t)

	// 创建一个商户
	merchant := createMerchantInRegion(t, region.ID, "approved")

	// 软删除该商户
	err := testStore.DeleteMerchant(context.Background(), merchant.ID)
	require.NoError(t, err)

	// 查询不应包含已删除商户
	merchants, err := testStore.ListMerchantsByRegion(context.Background(), ListMerchantsByRegionParams{
		RegionID: region.ID,
		Limit:    100,
		Offset:   0,
	})
	require.NoError(t, err)

	for _, m := range merchants {
		require.NotEqual(t, merchant.ID, m.ID, "软删除商户不应被返回")
	}
}
