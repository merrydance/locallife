package db

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/merrydance/locallife/util"
	"github.com/stretchr/testify/require"
)

func createRandomTag(t *testing.T, tagType string) Tag {
	arg := CreateTagParams{
		Name:      util.RandomString(10),
		Type:      tagType,
		SortOrder: int16(util.RandomInt(1, 100)),
		Status:    "active",
	}

	tag, err := testStore.CreateTag(context.Background(), arg)
	require.NoError(t, err)
	require.NotEmpty(t, tag)

	require.Equal(t, arg.Name, tag.Name)
	require.Equal(t, arg.Type, tag.Type)
	require.Equal(t, arg.SortOrder, tag.SortOrder)
	require.Equal(t, arg.Status, tag.Status)
	require.NotZero(t, tag.ID)
	require.NotZero(t, tag.CreatedAt)

	return tag
}

func TestCreateTag(t *testing.T) {
	createRandomTag(t, "merchant")
}

func TestGetTag(t *testing.T) {
	tag1 := createRandomTag(t, "merchant")

	tag2, err := testStore.GetTag(context.Background(), tag1.ID)
	require.NoError(t, err)
	require.NotEmpty(t, tag2)

	require.Equal(t, tag1.ID, tag2.ID)
	require.Equal(t, tag1.Name, tag2.Name)
	require.Equal(t, tag1.Type, tag2.Type)
	require.Equal(t, tag1.SortOrder, tag2.SortOrder)
	require.Equal(t, tag1.Status, tag2.Status)
}

func TestListTags(t *testing.T) {
	// 创建多个标签
	tagType := "test_list_" + util.RandomString(5)
	for i := 0; i < 5; i++ {
		createRandomTag(t, tagType)
	}

	arg := ListTagsParams{
		Type:   tagType,
		Limit:  10,
		Offset: 0,
	}

	tags, err := testStore.ListTags(context.Background(), arg)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(tags), 5)

	for _, tag := range tags {
		require.NotEmpty(t, tag)
		require.Equal(t, tagType, tag.Type)
		require.Equal(t, "active", tag.Status)
	}
}

func TestListAllTagsByType(t *testing.T) {
	// 创建多个标签
	tagType := "test_all_" + util.RandomString(5)
	for i := 0; i < 3; i++ {
		createRandomTag(t, tagType)
	}

	tags, err := testStore.ListAllTagsByType(context.Background(), tagType)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(tags), 3)

	for _, tag := range tags {
		require.NotEmpty(t, tag)
		require.Equal(t, tagType, tag.Type)
	}
}

func TestSearchTags(t *testing.T) {
	// 创建一个带有特定名称的标签
	tagType := "test_search_" + util.RandomString(5)
	uniqueName := "SEARCHABLE_" + util.RandomString(5)

	arg := CreateTagParams{
		Name:      uniqueName,
		Type:      tagType,
		SortOrder: 1,
		Status:    "active",
	}

	_, err := testStore.CreateTag(context.Background(), arg)
	require.NoError(t, err)

	// 搜索标签
	searchArg := SearchTagsParams{
		Type:    tagType,
		Column2: pgtype.Text{String: "SEARCHABLE", Valid: true},
		Limit:   10,
		Offset:  0,
	}

	tags, err := testStore.SearchTags(context.Background(), searchArg)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(tags), 1)

	// 验证搜索结果包含创建的标签
	found := false
	for _, tag := range tags {
		if tag.Name == uniqueName {
			found = true
			break
		}
	}
	require.True(t, found)
}

func TestUpdateTag(t *testing.T) {
	tag1 := createRandomTag(t, "merchant")

	newName := util.RandomString(10)
	newSortOrder := int16(util.RandomInt(1, 100))

	arg := UpdateTagParams{
		ID:        tag1.ID,
		Name:      pgtype.Text{String: newName, Valid: true},
		SortOrder: pgtype.Int2{Int16: newSortOrder, Valid: true},
		Status:    pgtype.Text{}, // 不更新
	}

	tag2, err := testStore.UpdateTag(context.Background(), arg)
	require.NoError(t, err)
	require.NotEmpty(t, tag2)

	require.Equal(t, tag1.ID, tag2.ID)
	require.Equal(t, newName, tag2.Name)
	require.Equal(t, newSortOrder, tag2.SortOrder)
	require.Equal(t, tag1.Status, tag2.Status) // 状态未变
}

func TestDeleteTag(t *testing.T) {
	tag := createRandomTag(t, "merchant")

	err := testStore.DeleteTag(context.Background(), tag.ID)
	require.NoError(t, err)

	// 验证已删除
	_, err = testStore.GetTag(context.Background(), tag.ID)
	require.Error(t, err)
}
