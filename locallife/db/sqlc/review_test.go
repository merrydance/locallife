package db

import (
	"context"
	"fmt"
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
		IsVisible:  true,
	}

	review, err := testStore.CreateReview(context.Background(), arg)
	require.NoError(t, err)
	require.NotZero(t, review.ID)

	return review
}

func createReviewMediaAsset(t *testing.T, userID int64) MediaAsset {
	objectKey := fmt.Sprintf("user/review/%d/%d.jpg", userID, time.Now().UnixNano())
	checksum := fmt.Sprintf("%064d", time.Now().UnixNano())
	asset, err := testStore.CreateMediaAsset(context.Background(), CreateMediaAssetParams{
		ObjectKey:      objectKey,
		Visibility:     "public",
		MediaCategory:  "review",
		MimeType:       "image/jpeg",
		FileSize:       1024,
		ChecksumSha256: checksum,
		UploadedBy:     userID,
		SourceClient:   "test",
	})
	require.NoError(t, err)

	asset, err = testStore.SetMediaAssetUploadStatus(context.Background(), SetMediaAssetUploadStatusParams{
		ID:           asset.ID,
		UploadStatus: "confirmed",
	})
	require.NoError(t, err)
	return asset
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
	for _, review := range reviews {
		require.NotEmpty(t, review.OrderNo)
	}
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

func TestHiddenReviewReplyVisibilityContract(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	user := createRandomUser(t)

	visibleOrder := createCompletedOrderForStats(t, user.ID, merchant.ID, 10000, "takeout", time.Now())
	visibleReview := createRandomReview(t, visibleOrder.ID, user.ID, merchant.ID)

	hiddenOrder := createCompletedOrderForStats(t, user.ID, merchant.ID, 10000, "takeout", time.Now())
	hiddenReview, err := testStore.CreateReview(context.Background(), CreateReviewParams{
		OrderID:    hiddenOrder.ID,
		UserID:     user.ID,
		MerchantID: merchant.ID,
		Content:    "该隐藏评价需要商户跟进",
		IsVisible:  false,
	})
	require.NoError(t, err)

	reply := pgtype.Text{String: "已联系顾客处理，会持续跟进。", Valid: true}
	updatedHiddenReview, err := testStore.UpdateMerchantReply(context.Background(), UpdateMerchantReplyParams{
		ID:            hiddenReview.ID,
		MerchantReply: reply,
	})
	require.NoError(t, err)
	require.False(t, updatedHiddenReview.IsVisible)
	require.Equal(t, reply.String, updatedHiddenReview.MerchantReply.String)

	publicReviews, err := testStore.ListReviewsByMerchant(context.Background(), ListReviewsByMerchantParams{
		MerchantID: merchant.ID,
		Limit:      10,
		Offset:     0,
	})
	require.NoError(t, err)
	require.Len(t, publicReviews, 1)
	require.Equal(t, visibleReview.ID, publicReviews[0].ID)

	allMerchantReviews, err := testStore.ListAllReviewsByMerchant(context.Background(), ListAllReviewsByMerchantParams{
		MerchantID: merchant.ID,
		Limit:      10,
		Offset:     0,
	})
	require.NoError(t, err)
	require.Len(t, allMerchantReviews, 2)
	require.Contains(t, reviewIDs(allMerchantReviews), hiddenReview.ID)

	userReviews, err := testStore.ListReviewsByUser(context.Background(), ListReviewsByUserParams{
		UserID: user.ID,
		Limit:  10,
		Offset: 0,
	})
	require.NoError(t, err)
	require.Len(t, userReviews, 2)
	require.Contains(t, reviewRowIDs(userReviews), hiddenReview.ID)
}

func TestReviewListQueriesUseIDTieBreaker(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	user := createRandomUser(t)
	tiedCreatedAt := time.Now().UTC().Truncate(time.Microsecond)

	order1 := createCompletedOrderForStats(t, user.ID, merchant.ID, 10000, "takeout", time.Now())
	review1 := createRandomReview(t, order1.ID, user.ID, merchant.ID)
	otherOwner := createRandomUser(t)
	otherMerchant := createRandomMerchantWithOwner(t, otherOwner.ID)
	order2 := createCompletedOrderForStats(t, user.ID, otherMerchant.ID, 10000, "takeout", time.Now())
	review2 := createRandomReview(t, order2.ID, user.ID, otherMerchant.ID)

	merchantUser := createRandomUser(t)
	order3 := createCompletedOrderForStats(t, merchantUser.ID, merchant.ID, 10000, "takeout", time.Now())
	review3 := createRandomReview(t, order3.ID, merchantUser.ID, merchant.ID)

	_, err := testStore.(*SQLStore).connPool.Exec(context.Background(),
		`UPDATE reviews SET created_at = $1 WHERE id = ANY($2)`,
		tiedCreatedAt,
		[]int64{review1.ID, review2.ID, review3.ID},
	)
	require.NoError(t, err)

	reviewsByUser, err := testStore.ListReviewsByUser(context.Background(), ListReviewsByUserParams{
		UserID: user.ID,
		Limit:  2,
		Offset: 0,
	})
	require.NoError(t, err)
	require.Len(t, reviewsByUser, 2)
	require.Equal(t, review2.ID, reviewsByUser[0].ID)
	require.Equal(t, review1.ID, reviewsByUser[1].ID)

	reviewsByMerchant, err := testStore.ListReviewsByMerchant(context.Background(), ListReviewsByMerchantParams{
		MerchantID: merchant.ID,
		Limit:      2,
		Offset:     0,
	})
	require.NoError(t, err)
	require.Len(t, reviewsByMerchant, 2)
	require.Equal(t, review3.ID, reviewsByMerchant[0].ID)
	require.Equal(t, review1.ID, reviewsByMerchant[1].ID)

	allReviewsByMerchant, err := testStore.ListAllReviewsByMerchant(context.Background(), ListAllReviewsByMerchantParams{
		MerchantID: merchant.ID,
		Limit:      2,
		Offset:     0,
	})
	require.NoError(t, err)
	require.Len(t, allReviewsByMerchant, 2)
	require.Equal(t, review3.ID, allReviewsByMerchant[0].ID)
	require.Equal(t, review1.ID, allReviewsByMerchant[1].ID)
}

func reviewIDs(reviews []Review) []int64 {
	ids := make([]int64, 0, len(reviews))
	for _, review := range reviews {
		ids = append(ids, review.ID)
	}
	return ids
}

func reviewRowIDs(reviews []ListReviewsByUserRow) []int64 {
	ids := make([]int64, 0, len(reviews))
	for _, review := range reviews {
		ids = append(ids, review.ID)
	}
	return ids
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

func TestUpdateMerchantReplyOverwriteAndTimestampContract(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	user := createRandomUser(t)

	order := createCompletedOrderForStats(t, user.ID, merchant.ID, 10000, "takeout", time.Now())
	review, err := testStore.CreateReview(context.Background(), CreateReviewParams{
		OrderID:    order.ID,
		UserID:     user.ID,
		MerchantID: merchant.ID,
		Content:    "该评价用于验证商户回复覆盖契约",
		IsVisible:  false,
	})
	require.NoError(t, err)

	reply := pgtype.Text{String: "已联系顾客处理，会持续跟进。", Valid: true}
	first, err := testStore.UpdateMerchantReply(context.Background(), UpdateMerchantReplyParams{
		ID:            review.ID,
		MerchantReply: reply,
	})
	require.NoError(t, err)
	require.False(t, first.IsVisible)
	require.True(t, first.RepliedAt.Valid)
	require.Equal(t, reply.String, first.MerchantReply.String)

	time.Sleep(5 * time.Millisecond)

	sameReply, err := testStore.UpdateMerchantReply(context.Background(), UpdateMerchantReplyParams{
		ID:            review.ID,
		MerchantReply: reply,
	})
	require.NoError(t, err)
	require.False(t, sameReply.IsVisible)
	require.Equal(t, reply.String, sameReply.MerchantReply.String)
	require.True(t, sameReply.RepliedAt.Time.After(first.RepliedAt.Time))

	time.Sleep(5 * time.Millisecond)

	replacement := pgtype.Text{String: "已升级给店长复盘，并补发优惠券。", Valid: true}
	replaced, err := testStore.UpdateMerchantReply(context.Background(), UpdateMerchantReplyParams{
		ID:            review.ID,
		MerchantReply: replacement,
	})
	require.NoError(t, err)
	require.False(t, replaced.IsVisible)
	require.Equal(t, replacement.String, replaced.MerchantReply.String)
	require.True(t, replaced.RepliedAt.Time.After(sameReply.RepliedAt.Time))

	persisted, err := testStore.GetReview(context.Background(), review.ID)
	require.NoError(t, err)
	require.False(t, persisted.IsVisible)
	require.Equal(t, replacement.String, persisted.MerchantReply.String)
	require.Equal(t, replaced.RepliedAt.Time, persisted.RepliedAt.Time)
}

func TestUpdateReviewContent(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	user := createRandomUser(t)

	order := createCompletedOrderForStats(t, user.ID, merchant.ID, 10000, "takeout", time.Now())
	review := createRandomReview(t, order.ID, user.ID, merchant.ID)

	updated, err := testStore.UpdateReviewContent(context.Background(), UpdateReviewContentParams{
		ID:      review.ID,
		Content: "更新后的评价内容",
	})
	require.NoError(t, err)
	require.Equal(t, review.ID, updated.ID)
	require.Equal(t, review.UserID, updated.UserID)
	require.Equal(t, review.MerchantID, updated.MerchantID)
	require.Equal(t, "更新后的评价内容", updated.Content)
	require.Equal(t, review.IsVisible, updated.IsVisible)
	require.False(t, updated.MerchantReply.Valid)
}

func TestUpdateReviewTx_ReplacesContentAndImages(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	user := createRandomUser(t)

	order := createCompletedOrderForStats(t, user.ID, merchant.ID, 10000, "takeout", time.Now())
	review := createRandomReview(t, order.ID, user.ID, merchant.ID)
	oldAsset := createReviewMediaAsset(t, user.ID)
	newAssetOne := createReviewMediaAsset(t, user.ID)
	newAssetTwo := createReviewMediaAsset(t, user.ID)

	_, err := testStore.AddReviewImage(context.Background(), AddReviewImageParams{
		ReviewID:     review.ID,
		MediaAssetID: oldAsset.ID,
		SortOrder:    0,
	})
	require.NoError(t, err)

	result, err := testStore.UpdateReviewTx(context.Background(), UpdateReviewTxParams{
		ID:            review.ID,
		Content:       "更新后的评价和图片",
		MediaAssetIDs: []int64{newAssetOne.ID, newAssetTwo.ID},
	})
	require.NoError(t, err)
	require.Equal(t, review.ID, result.Review.ID)
	require.Equal(t, "更新后的评价和图片", result.Review.Content)
	require.Len(t, result.Images, 2)
	require.Equal(t, newAssetOne.ID, result.Images[0].MediaAssetID)
	require.Equal(t, int32(0), result.Images[0].SortOrder)
	require.Equal(t, newAssetTwo.ID, result.Images[1].MediaAssetID)
	require.Equal(t, int32(1), result.Images[1].SortOrder)

	images, err := testStore.ListReviewImages(context.Background(), review.ID)
	require.NoError(t, err)
	require.Len(t, images, 2)
	require.Equal(t, newAssetOne.ID, images[0].MediaAssetID)
	require.Equal(t, newAssetTwo.ID, images[1].MediaAssetID)
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
