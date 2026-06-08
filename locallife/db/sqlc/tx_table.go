package db

import (
	"context"
	"errors"
	"fmt"
)

// CreateTableTxParams creates a table and its tag associations atomically.
type CreateTableTxParams struct {
	Table  CreateTableParams
	TagIDs []int64
}

// CreateTableTxResult contains the created table and tag associations.
type CreateTableTxResult struct {
	Table Table
	Tags  []TableTag
}

// UpdateTableTxParams updates a table and optionally replaces its tag associations atomically.
type UpdateTableTxParams struct {
	Table  UpdateTableParams
	TagIDs *[]int64
}

// UpdateTableTxResult contains the updated table and tag associations when tags were replaced.
type UpdateTableTxResult struct {
	Table Table
	Tags  []TableTag
}

// DeleteTableResult 删除桌台的结果
type DeleteTableResult struct {
	TableID int64
}

// DeleteTableParams 删除桌台的参数
type DeleteTableParams struct {
	TableID int64
}

// SetTableImagePrimaryTxParams sets a table image as primary within its table.
type SetTableImagePrimaryTxParams struct {
	TableID int64
	ImageID int64
}

// CreateTableTx creates a table and tag associations in a single transaction.
func (store *SQLStore) CreateTableTx(ctx context.Context, arg CreateTableTxParams) (CreateTableTxResult, error) {
	var result CreateTableTxResult

	err := store.execTx(ctx, func(q *Queries) error {
		var err error

		result.Table, err = q.CreateTable(ctx, arg.Table)
		if err != nil {
			return fmt.Errorf("create table: %w", err)
		}

		result.Tags = make([]TableTag, 0, len(arg.TagIDs))
		for _, tagID := range arg.TagIDs {
			tableTag, err := q.AddTableTag(ctx, AddTableTagParams{
				TableID: result.Table.ID,
				TagID:   tagID,
			})
			if err != nil {
				return fmt.Errorf("add table tag %d: %w", tagID, err)
			}
			result.Tags = append(result.Tags, tableTag)
		}

		return nil
	})

	return result, err
}

// UpdateTableTx updates a table and replaces tag associations in a single transaction.
func (store *SQLStore) UpdateTableTx(ctx context.Context, arg UpdateTableTxParams) (UpdateTableTxResult, error) {
	var result UpdateTableTxResult

	err := store.execTx(ctx, func(q *Queries) error {
		var err error

		result.Table, err = q.UpdateTable(ctx, arg.Table)
		if err != nil {
			return fmt.Errorf("update table: %w", err)
		}

		if arg.TagIDs != nil {
			if err := q.RemoveAllTableTags(ctx, arg.Table.ID); err != nil {
				return fmt.Errorf("remove table tags: %w", err)
			}

			result.Tags = make([]TableTag, 0, len(*arg.TagIDs))
			for _, tagID := range *arg.TagIDs {
				tableTag, err := q.AddTableTag(ctx, AddTableTagParams{
					TableID: arg.Table.ID,
					TagID:   tagID,
				})
				if err != nil {
					return fmt.Errorf("add table tag %d: %w", tagID, err)
				}
				result.Tags = append(result.Tags, tableTag)
			}
		}

		return nil
	})

	return result, err
}

// DeleteTableTx 事务删除桌台
// 检查是否有未来预定，如果有则返回错误
// 删除标签关联和桌台本身在同一个事务中
func (store *SQLStore) DeleteTableTx(ctx context.Context, arg DeleteTableParams) (DeleteTableResult, error) {
	var result DeleteTableResult

	err := store.execTx(ctx, func(q *Queries) error {
		var err error

		// 1. 检查是否有未来的预定
		count, err := q.CountFutureReservationsByTable(ctx, arg.TableID)
		if err != nil {
			return err
		}
		if count > 0 {
			return errors.New("cannot delete table with future reservations")
		}

		// 2. 删除所有标签关联
		err = q.RemoveAllTableTags(ctx, arg.TableID)
		if err != nil {
			return err
		}

		// 3. 删除桌台
		err = q.DeleteTable(ctx, arg.TableID)
		if err != nil {
			return err
		}

		result.TableID = arg.TableID
		return nil
	})

	return result, err
}

// SetTableImagePrimaryTx verifies the image belongs to the table, then switches the table primary image atomically.
func (store *SQLStore) SetTableImagePrimaryTx(ctx context.Context, arg SetTableImagePrimaryTxParams) (TableImage, error) {
	var result TableImage

	err := store.execTx(ctx, func(q *Queries) error {
		image, err := q.SetTableImagePrimary(ctx, SetTableImagePrimaryParams{
			TableID: arg.TableID,
			ID:      arg.ImageID,
		})
		if err != nil {
			return err
		}

		if err := q.ClearOtherPrimaryTableImages(ctx, ClearOtherPrimaryTableImagesParams{
			TableID: arg.TableID,
			ID:      arg.ImageID,
		}); err != nil {
			return err
		}

		result = image
		return nil
	})

	return result, err
}
