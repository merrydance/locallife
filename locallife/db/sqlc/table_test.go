package db

import (
	"context"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/merrydance/locallife/util"
	"github.com/stretchr/testify/require"
)

// ==================== Helper Functions ====================

// createRandomTable 创建一个随机的餐桌
func createRandomTable(t *testing.T, merchantID int64) Table {
	arg := CreateTableParams{
		MerchantID:   merchantID,
		TableNo:      util.RandomString(4),
		TableType:    "table",
		Capacity:     4,
		Description:  pgtype.Text{String: "靠窗位置", Valid: true},
		MinimumSpend: pgtype.Int8{Int64: 0, Valid: false},
		Status:       "available",
	}

	table, err := testStore.CreateTable(context.Background(), arg)
	require.NoError(t, err)
	require.NotZero(t, table.ID)

	return table
}

// createRandomRoom 创建一个随机的包间
func createRandomRoom(t *testing.T, merchantID int64) Table {
	arg := CreateTableParams{
		MerchantID:   merchantID,
		TableNo:      "VIP-" + util.RandomString(3),
		TableType:    "room",
		Capacity:     8,
		Description:  pgtype.Text{String: "豪华包间", Valid: true},
		MinimumSpend: pgtype.Int8{Int64: 100000, Valid: true}, // 最低消费1000元
		Status:       "available",
	}

	table, err := testStore.CreateTable(context.Background(), arg)
	require.NoError(t, err)
	require.NotZero(t, table.ID)

	return table
}

// ==================== CreateTable Tests ====================

func TestCreateTable(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)

	table := createRandomTable(t, merchant.ID)

	require.Equal(t, merchant.ID, table.MerchantID)
	require.Equal(t, "table", table.TableType)
	require.Equal(t, int16(4), table.Capacity)
	require.Equal(t, "available", table.Status)
	require.NotZero(t, table.CreatedAt)
}

func TestCreateTable_Room(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)

	room := createRandomRoom(t, merchant.ID)

	require.Equal(t, "room", room.TableType)
	require.True(t, room.MinimumSpend.Valid)
	require.Equal(t, int64(100000), room.MinimumSpend.Int64)
	require.Equal(t, int16(8), room.Capacity)
}

func TestCreateTable_WithQRCode(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)

	arg := CreateTableParams{
		MerchantID: merchant.ID,
		TableNo:    "T001",
		TableType:  "table",
		Capacity:   2,
		QrCodeUrl:  pgtype.Text{String: "https://example.com/qr/t001.png", Valid: true},
		Status:     "available",
	}

	table, err := testStore.CreateTable(context.Background(), arg)
	require.NoError(t, err)
	require.True(t, table.QrCodeUrl.Valid)
	require.Equal(t, "https://example.com/qr/t001.png", table.QrCodeUrl.String)
}

// ==================== GetTable Tests ====================

func TestGetTable(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)

	created := createRandomTable(t, merchant.ID)

	got, err := testStore.GetTable(context.Background(), created.ID)
	require.NoError(t, err)
	require.Equal(t, created.ID, got.ID)
	require.Equal(t, created.MerchantID, got.MerchantID)
	require.Equal(t, created.TableNo, got.TableNo)
	require.Equal(t, created.TableType, got.TableType)
	require.Equal(t, created.Capacity, got.Capacity)
}

func TestGetTable_NotFound(t *testing.T) {
	_, err := testStore.GetTable(context.Background(), 99999999)
	require.Error(t, err)
}

// ==================== GetTableByMerchantAndNo Tests ====================

func TestGetTableByMerchantAndNo(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)

	created := createRandomTable(t, merchant.ID)

	got, err := testStore.GetTableByMerchantAndNo(context.Background(), GetTableByMerchantAndNoParams{
		MerchantID: merchant.ID,
		TableNo:    created.TableNo,
	})
	require.NoError(t, err)
	require.Equal(t, created.ID, got.ID)
}

func TestGetTableByMerchantAndNo_NotFound(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)

	_, err := testStore.GetTableByMerchantAndNo(context.Background(), GetTableByMerchantAndNoParams{
		MerchantID: merchant.ID,
		TableNo:    "NOT_EXIST",
	})
	require.Error(t, err)
}

// ==================== GetTableForUpdate Tests ====================

func TestGetTableForUpdate(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)

	created := createRandomTable(t, merchant.ID)

	got, err := testStore.GetTableForUpdate(context.Background(), created.ID)
	require.NoError(t, err)
	require.Equal(t, created.ID, got.ID)
}

// ==================== UpdateTable Tests ====================

func TestUpdateTable(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)

	created := createRandomTable(t, merchant.ID)

	updated, err := testStore.UpdateTable(context.Background(), UpdateTableParams{
		ID:          created.ID,
		TableNo:     pgtype.Text{String: "T999", Valid: true},
		Capacity:    pgtype.Int2{Int16: 6, Valid: true},
		Description: pgtype.Text{String: "更新后的描述", Valid: true},
		Status:      pgtype.Text{String: "available", Valid: true},
	})
	require.NoError(t, err)
	require.Equal(t, "T999", updated.TableNo)
	require.Equal(t, int16(6), updated.Capacity)
	require.Equal(t, "更新后的描述", updated.Description.String)
}

func TestUpdateTable_StatusChange(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)

	created := createRandomTable(t, merchant.ID)

	// 更改状态为占用
	updated, err := testStore.UpdateTable(context.Background(), UpdateTableParams{
		ID:     created.ID,
		Status: pgtype.Text{String: "occupied", Valid: true},
	})
	require.NoError(t, err)
	require.Equal(t, "occupied", updated.Status)
}

// ==================== ListTablesByMerchant Tests ====================

func TestListTablesByMerchant(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)

	// 创建多个桌台
	_ = createRandomTable(t, merchant.ID)
	_ = createRandomTable(t, merchant.ID)
	_ = createRandomRoom(t, merchant.ID)

	tables, err := testStore.ListTablesByMerchant(context.Background(), merchant.ID)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(tables), 3)
}

func TestListTablesByMerchant_Empty(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)

	tables, err := testStore.ListTablesByMerchant(context.Background(), merchant.ID)
	require.NoError(t, err)
	require.Empty(t, tables)
}

// ==================== ListTablesByMerchantAndType Tests ====================

func TestListTablesByMerchantAndType(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)

	_ = createRandomTable(t, merchant.ID)
	_ = createRandomTable(t, merchant.ID)
	_ = createRandomRoom(t, merchant.ID)

	// 只查询普通桌台
	tables, err := testStore.ListTablesByMerchantAndType(context.Background(), ListTablesByMerchantAndTypeParams{
		MerchantID: merchant.ID,
		TableType:  "table",
	})
	require.NoError(t, err)
	for _, table := range tables {
		require.Equal(t, "table", table.TableType)
	}

	// 只查询包间
	rooms, err := testStore.ListTablesByMerchantAndType(context.Background(), ListTablesByMerchantAndTypeParams{
		MerchantID: merchant.ID,
		TableType:  "room",
	})
	require.NoError(t, err)
	for _, room := range rooms {
		require.Equal(t, "room", room.TableType)
	}
}

// ==================== ListAvailableRooms Tests ====================

func TestListAvailableRooms(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)

	// 创建可用包间
	_ = createRandomRoom(t, merchant.ID)
	_ = createRandomRoom(t, merchant.ID)

	// 创建不可用包间
	arg := CreateTableParams{
		MerchantID: merchant.ID,
		TableNo:    "VIP-OCCUPIED",
		TableType:  "room",
		Capacity:   6,
		Status:     "occupied",
	}
	_, _ = testStore.CreateTable(context.Background(), arg)

	rooms, err := testStore.ListAvailableRooms(context.Background(), merchant.ID)
	require.NoError(t, err)

	// 只返回可用包间
	for _, room := range rooms {
		require.Equal(t, "room", room.TableType)
		require.Equal(t, "available", room.Status)
	}
}

// ==================== CountTablesByMerchant Tests ====================

func TestCountTablesByMerchant(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)

	// 初始计数
	count, err := testStore.CountTablesByMerchant(context.Background(), merchant.ID)
	require.NoError(t, err)
	require.Equal(t, int64(0), count)

	// 添加桌台
	_ = createRandomTable(t, merchant.ID)
	_ = createRandomTable(t, merchant.ID)
	_ = createRandomRoom(t, merchant.ID)

	count, err = testStore.CountTablesByMerchant(context.Background(), merchant.ID)
	require.NoError(t, err)
	require.Equal(t, int64(3), count)
}

// ==================== CountAvailableTablesByMerchant Tests ====================

func TestCountAvailableTablesByMerchant(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)

	// 创建可用桌台
	_ = createRandomTable(t, merchant.ID)
	_ = createRandomRoom(t, merchant.ID)

	// 创建不可用桌台
	arg := CreateTableParams{
		MerchantID: merchant.ID,
		TableNo:    "T-OCCUPIED",
		TableType:  "table",
		Capacity:   4,
		Status:     "occupied",
	}
	_, _ = testStore.CreateTable(context.Background(), arg)

	count, err := testStore.CountAvailableTablesByMerchant(context.Background(), merchant.ID)
	require.NoError(t, err)
	require.Equal(t, int64(2), count) // 只有2个可用
}

// ==================== DeleteTable Tests ====================

func TestDeleteTable(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)

	table := createRandomTable(t, merchant.ID)

	err := testStore.DeleteTable(context.Background(), table.ID)
	require.NoError(t, err)

	// 验证已删除
	_, err = testStore.GetTable(context.Background(), table.ID)
	require.Error(t, err)
}

// ==================== Table Tags Tests ====================

func TestAddTableTag(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	table := createRandomRoom(t, merchant.ID)

	// 创建标签(使用唯一名称)
	tag, err := testStore.CreateTag(context.Background(), CreateTagParams{
		Name: "窗景包间-" + util.RandomString(6),
		Type: "room_feature",
	})
	require.NoError(t, err)

	tableTag, err := testStore.AddTableTag(context.Background(), AddTableTagParams{
		TableID: table.ID,
		TagID:   tag.ID,
	})
	require.NoError(t, err)
	require.NotZero(t, tableTag.ID)
	require.Equal(t, table.ID, tableTag.TableID)
	require.Equal(t, tag.ID, tableTag.TagID)
}

func TestListTableTags(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	table := createRandomRoom(t, merchant.ID)

	// 创建并添加多个标签(使用唯一名称)
	tag1, err := testStore.CreateTag(context.Background(), CreateTagParams{
		Name: "江景包间-" + util.RandomString(6),
		Type: "room_feature",
	})
	require.NoError(t, err)
	tag2, err := testStore.CreateTag(context.Background(), CreateTagParams{
		Name: "大包间-" + util.RandomString(6),
		Type: "room_size",
	})
	require.NoError(t, err)

	_, err = testStore.AddTableTag(context.Background(), AddTableTagParams{TableID: table.ID, TagID: tag1.ID})
	require.NoError(t, err)
	_, err = testStore.AddTableTag(context.Background(), AddTableTagParams{TableID: table.ID, TagID: tag2.ID})
	require.NoError(t, err)

	tags, err := testStore.ListTableTags(context.Background(), table.ID)
	require.NoError(t, err)
	require.Len(t, tags, 2)

	for _, tt := range tags {
		require.NotEmpty(t, tt.TagName)
		require.NotEmpty(t, tt.TagType)
	}
}

func TestRemoveTableTag(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	table := createRandomRoom(t, merchant.ID)

	tag, _ := testStore.CreateTag(context.Background(), CreateTagParams{
		Name: "测试标签-" + util.RandomString(6),
		Type: "room_feature",
	})

	_, _ = testStore.AddTableTag(context.Background(), AddTableTagParams{TableID: table.ID, TagID: tag.ID})

	// 删除标签
	err := testStore.RemoveTableTag(context.Background(), RemoveTableTagParams{
		TableID: table.ID,
		TagID:   tag.ID,
	})
	require.NoError(t, err)

	// 验证已删除
	tags, err := testStore.ListTableTags(context.Background(), table.ID)
	require.NoError(t, err)
	require.Empty(t, tags)
}

func TestRemoveAllTableTags(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	table := createRandomRoom(t, merchant.ID)

	tag1, err := testStore.CreateTag(context.Background(), CreateTagParams{Name: "标签1-" + util.RandomString(6), Type: "room_feature"})
	require.NoError(t, err)
	tag2, err := testStore.CreateTag(context.Background(), CreateTagParams{Name: "标签2-" + util.RandomString(6), Type: "room_feature"})
	require.NoError(t, err)

	_, err = testStore.AddTableTag(context.Background(), AddTableTagParams{TableID: table.ID, TagID: tag1.ID})
	require.NoError(t, err)
	_, err = testStore.AddTableTag(context.Background(), AddTableTagParams{TableID: table.ID, TagID: tag2.ID})
	require.NoError(t, err)

	// 删除所有标签
	err = testStore.RemoveAllTableTags(context.Background(), table.ID)
	require.NoError(t, err)

	tags, err := testStore.ListTableTags(context.Background(), table.ID)
	require.NoError(t, err)
	require.Empty(t, tags)
}

func TestListTablesByTag(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	room1 := createRandomRoom(t, merchant.ID)
	room2 := createRandomRoom(t, merchant.ID)

	tag, err := testStore.CreateTag(context.Background(), CreateTagParams{
		Name: "特色包间-" + util.RandomString(6),
		Type: "room_feature",
	})
	require.NoError(t, err)

	_, err = testStore.AddTableTag(context.Background(), AddTableTagParams{TableID: room1.ID, TagID: tag.ID})
	require.NoError(t, err)
	_, err = testStore.AddTableTag(context.Background(), AddTableTagParams{TableID: room2.ID, TagID: tag.ID})
	require.NoError(t, err)

	tables, err := testStore.ListTablesByTag(context.Background(), tag.ID)
	require.NoError(t, err)
	require.Len(t, tables, 2)
}

// ==================== Edge Cases Tests ====================

func TestTable_DifferentStatuses(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)

	// 有效状态: 'available', 'occupied', 'disabled'
	statuses := []string{"available", "occupied", "disabled"}

	for _, status := range statuses {
		arg := CreateTableParams{
			MerchantID: merchant.ID,
			TableNo:    "STATUS-" + status,
			TableType:  "table",
			Capacity:   4,
			Status:     status,
		}

		table, err := testStore.CreateTable(context.Background(), arg)
		require.NoError(t, err)
		require.Equal(t, status, table.Status)
	}
}

func TestTable_LargeCapacity(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)

	// 大包间测试
	arg := CreateTableParams{
		MerchantID:   merchant.ID,
		TableNo:      "BANQUET-01",
		TableType:    "room",
		Capacity:     50,                                       // 宴会厅50人
		MinimumSpend: pgtype.Int8{Int64: 1000000, Valid: true}, // 最低消费10000元
		Status:       "available",
	}

	table, err := testStore.CreateTable(context.Background(), arg)
	require.NoError(t, err)
	require.Equal(t, int16(50), table.Capacity)
	require.Equal(t, int64(1000000), table.MinimumSpend.Int64)
}

func TestTable_MultiMerchant(t *testing.T) {
	owner1 := createRandomUser(t)
	owner2 := createRandomUser(t)
	merchant1 := createRandomMerchantWithOwner(t, owner1.ID)
	merchant2 := createRandomMerchantWithOwner(t, owner2.ID)

	// 两个商户可以有相同桌号
	arg1 := CreateTableParams{
		MerchantID: merchant1.ID,
		TableNo:    "A01",
		TableType:  "table",
		Capacity:   4,
		Status:     "available",
	}
	arg2 := CreateTableParams{
		MerchantID: merchant2.ID,
		TableNo:    "A01",
		TableType:  "table",
		Capacity:   4,
		Status:     "available",
	}

	table1, err := testStore.CreateTable(context.Background(), arg1)
	require.NoError(t, err)

	table2, err := testStore.CreateTable(context.Background(), arg2)
	require.NoError(t, err)

	require.NotEqual(t, table1.ID, table2.ID)
	require.Equal(t, table1.TableNo, table2.TableNo)
}

// ==================== 包间图片管理测试 ====================

func TestAddTableImage(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	table := createRandomRoom(t, merchant.ID)

	arg := AddTableImageParams{
		TableID:   table.ID,
		ImageUrl:  "https://example.com/room1.jpg",
		SortOrder: 1,
		IsPrimary: true,
	}

	image, err := testStore.AddTableImage(context.Background(), arg)
	require.NoError(t, err)
	require.NotZero(t, image.ID)
	require.Equal(t, table.ID, image.TableID)
	require.Equal(t, arg.ImageUrl, image.ImageUrl)
	require.Equal(t, arg.SortOrder, image.SortOrder)
	require.True(t, image.IsPrimary)
}

func TestListTableImages(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	table := createRandomRoom(t, merchant.ID)

	// 添加多张图片
	for i := 1; i <= 3; i++ {
		arg := AddTableImageParams{
			TableID:   table.ID,
			ImageUrl:  fmt.Sprintf("https://example.com/room%d.jpg", i),
			SortOrder: int32(i),
			IsPrimary: i == 1,
		}
		_, err := testStore.AddTableImage(context.Background(), arg)
		require.NoError(t, err)
	}

	images, err := testStore.ListTableImages(context.Background(), table.ID)
	require.NoError(t, err)
	require.Len(t, images, 3)

	// 验证按 sort_order 排序
	for i, img := range images {
		require.Equal(t, int32(i+1), img.SortOrder)
	}
}

func TestGetPrimaryTableImage(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	table := createRandomRoom(t, merchant.ID)

	// 添加非主图
	_, err := testStore.AddTableImage(context.Background(), AddTableImageParams{
		TableID:   table.ID,
		ImageUrl:  "https://example.com/non-primary.jpg",
		SortOrder: 1,
		IsPrimary: false,
	})
	require.NoError(t, err)

	// 添加主图
	_, err = testStore.AddTableImage(context.Background(), AddTableImageParams{
		TableID:   table.ID,
		ImageUrl:  "https://example.com/primary.jpg",
		SortOrder: 2,
		IsPrimary: true,
	})
	require.NoError(t, err)

	primaryImage, err := testStore.GetPrimaryTableImage(context.Background(), table.ID)
	require.NoError(t, err)
	require.Equal(t, "https://example.com/primary.jpg", primaryImage.ImageUrl)
	require.True(t, primaryImage.IsPrimary)
}

func TestSetPrimaryTableImage(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	table := createRandomRoom(t, merchant.ID)

	// 添加一张主图
	img1, err := testStore.AddTableImage(context.Background(), AddTableImageParams{
		TableID:   table.ID,
		ImageUrl:  "https://example.com/img1.jpg",
		SortOrder: 1,
		IsPrimary: true,
	})
	require.NoError(t, err)
	require.True(t, img1.IsPrimary)

	// 清除所有主图标记
	err = testStore.SetPrimaryTableImage(context.Background(), table.ID)
	require.NoError(t, err)

	// 验证没有主图了
	images, err := testStore.ListTableImages(context.Background(), table.ID)
	require.NoError(t, err)
	for _, img := range images {
		require.False(t, img.IsPrimary)
	}
}

func TestSetTableImagePrimary(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	table := createRandomRoom(t, merchant.ID)

	// 添加一张非主图
	img, err := testStore.AddTableImage(context.Background(), AddTableImageParams{
		TableID:   table.ID,
		ImageUrl:  "https://example.com/img.jpg",
		SortOrder: 1,
		IsPrimary: false,
	})
	require.NoError(t, err)
	require.False(t, img.IsPrimary)

	// 设置为主图
	updatedImg, err := testStore.SetTableImagePrimary(context.Background(), img.ID)
	require.NoError(t, err)
	require.True(t, updatedImg.IsPrimary)
}

func TestDeleteTableImage(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	table := createRandomRoom(t, merchant.ID)

	img, err := testStore.AddTableImage(context.Background(), AddTableImageParams{
		TableID:   table.ID,
		ImageUrl:  "https://example.com/to-delete.jpg",
		SortOrder: 1,
		IsPrimary: false,
	})
	require.NoError(t, err)

	// 删除图片
	err = testStore.DeleteTableImage(context.Background(), img.ID)
	require.NoError(t, err)

	// 验证删除成功
	images, err := testStore.ListTableImages(context.Background(), table.ID)
	require.NoError(t, err)
	for _, i := range images {
		require.NotEqual(t, img.ID, i.ID)
	}
}

// ==================== 客户端包间查询测试 ====================

func TestListMerchantRoomsForCustomer(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)

	// 创建多个包间
	for i := 1; i <= 3; i++ {
		createRandomRoom(t, merchant.ID)
	}

	rooms, err := testStore.ListMerchantRoomsForCustomer(context.Background(), merchant.ID)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(rooms), 3)

	for _, room := range rooms {
		require.Equal(t, merchant.ID, room.MerchantID)
	}
}

func TestSearchRoomsWithImage(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	room := createRandomRoom(t, merchant.ID)

	// 添加主图
	_, err := testStore.AddTableImage(context.Background(), AddTableImageParams{
		TableID:   room.ID,
		ImageUrl:  "https://example.com/searchtest-primary.jpg",
		SortOrder: 1,
		IsPrimary: true,
	})
	require.NoError(t, err)

	// 搜索 - 根据此商户的 region_id 筛选
	result, err := testStore.SearchRoomsWithImage(context.Background(), SearchRoomsWithImageParams{
		PageSize:   100,
		PageOffset: 0,
		RegionID:   pgtype.Int8{Int64: merchant.RegionID, Valid: true},
	})
	require.NoError(t, err)
	require.NotEmpty(t, result, "should find at least one room")

	// 验证我们创建的房间在结果中
	found := false
	for _, r := range result {
		if r.ID == room.ID {
			found = true
			require.Equal(t, "room", r.TableType)
			require.Equal(t, merchant.ID, r.MerchantID)
			break
		}
	}
	require.True(t, found, "created room should be in search results")
}

func TestExploreNearbyRooms(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	room := createRandomRoom(t, merchant.ID)

	// 添加主图
	_, err := testStore.AddTableImage(context.Background(), AddTableImageParams{
		TableID:   room.ID,
		ImageUrl:  "https://example.com/explore.jpg",
		SortOrder: 1,
		IsPrimary: true,
	})
	require.NoError(t, err)

	// 探索附近包间
	result, err := testStore.ExploreNearbyRooms(context.Background(), ExploreNearbyRoomsParams{
		PageSize:   10,
		PageOffset: 0,
	})
	require.NoError(t, err)
	require.NotEmpty(t, result)

	// 验证结果包含必要字段
	for _, r := range result {
		require.NotZero(t, r.ID)
		require.NotEmpty(t, r.TableNo)
		require.NotEmpty(t, r.MerchantName)
	}
}

func TestCountExploreNearbyRooms(t *testing.T) {
	count, err := testStore.CountExploreNearbyRooms(context.Background(), CountExploreNearbyRoomsParams{})
	require.NoError(t, err)
	require.GreaterOrEqual(t, count, int64(0))
}
