package db

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/require"
)

// ==================== Helper Functions ====================

func createRandomReview(t *testing.T, orderID, userID, merchantID int64) Review {
	arg := CreateReviewParams{
		OrderID:    orderID,
		UserID:     userID,
		MerchantID: merchantID,
		Content:    "服务很好，味道不错！",
		Images:     []string{"https://example.com/img1.jpg", "https://example.com/img2.jpg"},
		IsVisible:  true,
	}

	review, err := testStore.CreateReview(context.Background(), arg)
	require.NoError(t, err)
	require.NotZero(t, review.ID)

	return review
}

// ==================== CreateReview Tests ====================

func TestCreateReview(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	user := createRandomUser(t)

	order := createCompletedOrderForStats(t, user.ID, merchant.ID, 10000, "takeout", time.Now())
	review := createRandomReview(t, order.ID, user.ID, merchant.ID)

	require.Equal(t, order.ID, review.OrderID)
	require.Equal(t, user.ID, review.UserID)
	require.Equal(t, merchant.ID, review.MerchantID)
	require.NotEmpty(t, review.Content)
	require.Len(t, review.Images, 2)
	require.True(t, review.IsVisible)
	require.False(t, review.MerchantReply.Valid) // 初始没有回复
}

func TestCreateReview_WithoutContent(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	user := createRandomUser(t)

	order := createCompletedOrderForStats(t, user.ID, merchant.ID, 10000, "takeout", time.Now())

	arg := CreateReviewParams{
		OrderID:    order.ID,
		UserID:     user.ID,
		MerchantID: merchant.ID,
		Content:    "", // 无文字评价
		Images:     nil,
		IsVisible:  true,
	}

	review, err := testStore.CreateReview(context.Background(), arg)
	require.NoError(t, err)
	require.Empty(t, review.Content)
}

// ==================== GetReview Tests ====================

func TestGetReview(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	user := createRandomUser(t)

	order := createCompletedOrderForStats(t, user.ID, merchant.ID, 10000, "takeout", time.Now())
	created := createRandomReview(t, order.ID, user.ID, merchant.ID)

	got, err := testStore.GetReview(context.Background(), created.ID)
	require.NoError(t, err)
	require.Equal(t, created.ID, got.ID)
	require.Equal(t, created.OrderID, got.OrderID)
	require.Equal(t, created.Content, got.Content)
}

func TestGetReview_NotFound(t *testing.T) {
	_, err := testStore.GetReview(context.Background(), 99999999)
	require.Error(t, err)
}

func TestGetReviewByOrderID(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	user := createRandomUser(t)

	order := createCompletedOrderForStats(t, user.ID, merchant.ID, 10000, "takeout", time.Now())
	created := createRandomReview(t, order.ID, user.ID, merchant.ID)

	got, err := testStore.GetReviewByOrderID(context.Background(), order.ID)
	require.NoError(t, err)
	require.Equal(t, created.ID, got.ID)
}

// ==================== ListReviews Tests ====================

func TestListReviewsByMerchant(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)

	// 创建5条可见评价
	for i := 0; i < 5; i++ {
		user := createRandomUser(t)
		order := createCompletedOrderForStats(t, user.ID, merchant.ID, 10000, "takeout", time.Now())
		createRandomReview(t, order.ID, user.ID, merchant.ID)
	}

	// 创建1条不可见评价
	user := createRandomUser(t)
	order := createCompletedOrderForStats(t, user.ID, merchant.ID, 10000, "takeout", time.Now())
	_, err := testStore.CreateReview(context.Background(), CreateReviewParams{
		OrderID:    order.ID,
		UserID:     user.ID,
		MerchantID: merchant.ID,
		Content:    "差评",
		IsVisible:  false, // 不可见
	})
	require.NoError(t, err)

	reviews, err := testStore.ListReviewsByMerchant(context.Background(), ListReviewsByMerchantParams{
		MerchantID: merchant.ID,
		Limit:      10,
		Offset:     0,
	})
	require.NoError(t, err)
	require.Len(t, reviews, 5) // 只有可见的5条
}

func TestListReviewsByUser(t *testing.T) {
	user := createRandomUser(t)

	// 用户在3个商户下单并评价
	for i := 0; i < 3; i++ {
		owner := createRandomUser(t)
		merchant := createRandomMerchantWithOwner(t, owner.ID)
		order := createCompletedOrderForStats(t, user.ID, merchant.ID, 10000, "takeout", time.Now())
		createRandomReview(t, order.ID, user.ID, merchant.ID)
	}

	reviews, err := testStore.ListReviewsByUser(context.Background(), ListReviewsByUserParams{
		UserID: user.ID,
		Limit:  10,
		Offset: 0,
	})
	require.NoError(t, err)
	require.Len(t, reviews, 3)
}

func TestListAllReviewsByMerchant(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)

	// 创建3条可见评价
	for i := 0; i < 3; i++ {
		user := createRandomUser(t)
		order := createCompletedOrderForStats(t, user.ID, merchant.ID, 10000, "takeout", time.Now())
		createRandomReview(t, order.ID, user.ID, merchant.ID)
	}

	// 创建2条不可见评价
	for i := 0; i < 2; i++ {
		user := createRandomUser(t)
		order := createCompletedOrderForStats(t, user.ID, merchant.ID, 10000, "takeout", time.Now())
		_, err := testStore.CreateReview(context.Background(), CreateReviewParams{
			OrderID:    order.ID,
			UserID:     user.ID,
			MerchantID: merchant.ID,
			Content:    "不可见评价",
			IsVisible:  false,
		})
		require.NoError(t, err)
	}

	// 商户查看所有评价（包含不可见）
	reviews, err := testStore.ListAllReviewsByMerchant(context.Background(), ListAllReviewsByMerchantParams{
		MerchantID: merchant.ID,
		Limit:      10,
		Offset:     0,
	})
	require.NoError(t, err)
	require.Len(t, reviews, 5) // 所有5条
}

// ==================== Count Tests ====================

func TestCountReviewsByMerchant(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)

	// 创建4条可见评价
	for i := 0; i < 4; i++ {
		user := createRandomUser(t)
		order := createCompletedOrderForStats(t, user.ID, merchant.ID, 10000, "takeout", time.Now())
		createRandomReview(t, order.ID, user.ID, merchant.ID)
	}

	count, err := testStore.CountReviewsByMerchant(context.Background(), merchant.ID)
	require.NoError(t, err)
	require.Equal(t, int64(4), count)
}

func TestCountReviewsByUser(t *testing.T) {
	user := createRandomUser(t)

	// 创建3条评价
	for i := 0; i < 3; i++ {
		owner := createRandomUser(t)
		merchant := createRandomMerchantWithOwner(t, owner.ID)
		order := createCompletedOrderForStats(t, user.ID, merchant.ID, 10000, "takeout", time.Now())
		createRandomReview(t, order.ID, user.ID, merchant.ID)
	}

	count, err := testStore.CountReviewsByUser(context.Background(), user.ID)
	require.NoError(t, err)
	require.Equal(t, int64(3), count)
}

func TestCountAllReviewsByMerchant(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)

	// 创建2条可见 + 2条不可见
	for i := 0; i < 2; i++ {
		user := createRandomUser(t)
		order := createCompletedOrderForStats(t, user.ID, merchant.ID, 10000, "takeout", time.Now())
		createRandomReview(t, order.ID, user.ID, merchant.ID)
	}
	for i := 0; i < 2; i++ {
		user := createRandomUser(t)
		order := createCompletedOrderForStats(t, user.ID, merchant.ID, 10000, "takeout", time.Now())
		_, err := testStore.CreateReview(context.Background(), CreateReviewParams{
			OrderID:    order.ID,
			UserID:     user.ID,
			MerchantID: merchant.ID,
			IsVisible:  false,
		})
		require.NoError(t, err)
	}

	count, err := testStore.CountAllReviewsByMerchant(context.Background(), merchant.ID)
	require.NoError(t, err)
	require.Equal(t, int64(4), count) // 所有4条
}

// ==================== Update Tests ====================

func TestUpdateMerchantReply(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	user := createRandomUser(t)

	order := createCompletedOrderForStats(t, user.ID, merchant.ID, 10000, "takeout", time.Now())
	review := createRandomReview(t, order.ID, user.ID, merchant.ID)

	// 商户回复
	reply := pgtype.Text{String: "感谢您的评价，欢迎再次光临！", Valid: true}
	updated, err := testStore.UpdateMerchantReply(context.Background(), UpdateMerchantReplyParams{
		ID:            review.ID,
		MerchantReply: reply,
	})
	require.NoError(t, err)
	require.True(t, updated.MerchantReply.Valid)
	require.Equal(t, reply.String, updated.MerchantReply.String)
	require.True(t, updated.RepliedAt.Valid)
}

func TestUpdateReviewVisibility(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	user := createRandomUser(t)

	order := createCompletedOrderForStats(t, user.ID, merchant.ID, 10000, "takeout", time.Now())
	review := createRandomReview(t, order.ID, user.ID, merchant.ID)

	require.True(t, review.IsVisible)

	// 设置为不可见
	updated, err := testStore.UpdateReviewVisibility(context.Background(), UpdateReviewVisibilityParams{
		ID:        review.ID,
		IsVisible: false,
	})
	require.NoError(t, err)
	require.False(t, updated.IsVisible)

	// 恢复为可见
	updated, err = testStore.UpdateReviewVisibility(context.Background(), UpdateReviewVisibilityParams{
		ID:        review.ID,
		IsVisible: true,
	})
	require.NoError(t, err)
	require.True(t, updated.IsVisible)
}

// ==================== Delete Tests ====================

func TestDeleteReview(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	user := createRandomUser(t)

	order := createCompletedOrderForStats(t, user.ID, merchant.ID, 10000, "takeout", time.Now())
	review := createRandomReview(t, order.ID, user.ID, merchant.ID)

	err := testStore.DeleteReview(context.Background(), review.ID)
	require.NoError(t, err)

	// 验证已删除
	_, err = testStore.GetReview(context.Background(), review.ID)
	require.Error(t, err)
}
