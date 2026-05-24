package db

import (
	"context"
	"fmt"
)

type UpdateReviewTxParams struct {
	ID            int64
	Content       string
	MediaAssetIDs []int64
}

type UpdateReviewTxResult struct {
	Review Review
	Images []ReviewImage
}

func (store *SQLStore) UpdateReviewTx(ctx context.Context, arg UpdateReviewTxParams) (UpdateReviewTxResult, error) {
	var result UpdateReviewTxResult

	err := store.execTx(ctx, func(q *Queries) error {
		var err error
		result.Review, err = q.UpdateReviewContent(ctx, UpdateReviewContentParams{
			ID:      arg.ID,
			Content: arg.Content,
		})
		if err != nil {
			return fmt.Errorf("update review content: %w", err)
		}

		if err := q.DeleteReviewImages(ctx, arg.ID); err != nil {
			return fmt.Errorf("delete review images: %w", err)
		}

		result.Images = make([]ReviewImage, 0, len(arg.MediaAssetIDs))
		for i, assetID := range arg.MediaAssetIDs {
			image, err := q.AddReviewImage(ctx, AddReviewImageParams{
				ReviewID:     arg.ID,
				MediaAssetID: assetID,
				SortOrder:    int32(i),
			})
			if err != nil {
				return fmt.Errorf("add review image %d: %w", assetID, err)
			}
			result.Images = append(result.Images, image)
		}

		return nil
	})

	return result, err
}
