package db

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgtype"
)

var (
	ErrTableHasActiveReservation  = errors.New("table has active reservation")
	ErrTableHasFutureReservations = errors.New("table has future reservations")
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
	Table                       UpdateTableParams
	TagIDs                      *[]int64
	RequireNoFutureReservations bool
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

// AddTableImageTxParams adds a table image and optionally makes it the only primary image.
type AddTableImageTxParams struct {
	Image AddTableImageParams
}

// UpdateTableStatusTxParams updates table status through the table transaction boundary.
type UpdateTableStatusTxParams struct {
	ID                          int64
	Status                      string
	CurrentReservationID        pgtype.Int8
	RequireNoFutureReservations bool
}

func ensureTableHasNoFutureReservations(ctx context.Context, q *Queries, tableID int64) error {
	count, err := q.CountFutureReservationsByTable(ctx, tableID)
	if err != nil {
		return err
	}
	if count > 0 {
		return ErrTableHasFutureReservations
	}
	return nil
}

func tableUpdateChangesReservationFields(current Table, update UpdateTableParams) bool {
	if update.TableNo.Valid && update.TableNo.String != current.TableNo {
		return true
	}
	if update.TableType.Valid && update.TableType.String != current.TableType {
		return true
	}
	if update.Capacity.Valid && update.Capacity.Int16 != current.Capacity {
		return true
	}
	if update.MinimumSpend.Valid && (!current.MinimumSpend.Valid || update.MinimumSpend.Int64 != current.MinimumSpend.Int64) {
		return true
	}
	if update.Status.Valid && update.Status.String == TableStatusDisabled && current.Status != TableStatusDisabled {
		return true
	}
	return false
}

// CreateTableTx creates a table and tag associations in a single transaction.
func (store *SQLStore) CreateTableTx(ctx context.Context, arg CreateTableTxParams) (CreateTableTxResult, error) {
	var result CreateTableTxResult

	err := store.execTx(ctx, func(q *Queries) error {
		var err error

		tagIDs, err := ensureMerchantSelectableTags(ctx, q, arg.Table.MerchantID, TagTypeTable, arg.TagIDs)
		if err != nil {
			return err
		}

		result.Table, err = q.CreateTable(ctx, arg.Table)
		if err != nil {
			return fmt.Errorf("create table: %w", err)
		}

		result.Tags = make([]TableTag, 0, len(tagIDs))
		for _, tagID := range tagIDs {
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
		var table Table
		tableLocked := false
		var tagIDs []int64

		if arg.RequireNoFutureReservations {
			table, err = q.GetTableForUpdate(ctx, arg.Table.ID)
			if err != nil {
				return fmt.Errorf("lock table: %w", err)
			}
			tableLocked = true
			if tableUpdateChangesReservationFields(table, arg.Table) {
				err = ensureTableHasNoFutureReservations(ctx, q, arg.Table.ID)
			}
			if err != nil {
				return err
			}
		}

		if arg.Table.TableNo.Valid && !arg.Table.QrCodeUrl.Valid {
			if !tableLocked {
				table, err = q.GetTableForUpdate(ctx, arg.Table.ID)
				if err != nil {
					return fmt.Errorf("lock table: %w", err)
				}
				tableLocked = true
			}
			if arg.Table.TableNo.String != table.TableNo {
				arg.Table.QrCodeUrl = pgtype.Text{String: "", Valid: true}
			}
		}

		if arg.TagIDs != nil && !tableLocked {
			table, err = q.GetTableForUpdate(ctx, arg.Table.ID)
			if err != nil {
				return fmt.Errorf("lock table: %w", err)
			}
			tableLocked = true
		}
		if arg.TagIDs != nil {
			tagIDs, err = ensureMerchantSelectableTags(ctx, q, table.MerchantID, TagTypeTable, *arg.TagIDs)
			if err != nil {
				return err
			}
		}

		result.Table, err = q.UpdateTable(ctx, arg.Table)
		if err != nil {
			return fmt.Errorf("update table: %w", err)
		}

		if arg.TagIDs != nil {
			if err := q.RemoveAllTableTags(ctx, arg.Table.ID); err != nil {
				return fmt.Errorf("remove table tags: %w", err)
			}

			result.Tags = make([]TableTag, 0, len(tagIDs))
			for _, tagID := range tagIDs {
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

// UpdateTableStatusTx updates table status with optional future-reservation protection.
func (store *SQLStore) UpdateTableStatusTx(ctx context.Context, arg UpdateTableStatusTxParams) (Table, error) {
	var result Table

	err := store.execTx(ctx, func(q *Queries) error {
		if arg.RequireNoFutureReservations {
			table, err := q.GetTableForUpdate(ctx, arg.ID)
			if err != nil {
				return fmt.Errorf("lock table: %w", err)
			}
			if arg.Status == TableStatusDisabled && table.Status != TableStatusDisabled {
				err = ensureTableHasNoFutureReservations(ctx, q, arg.ID)
			}
			if err != nil {
				return err
			}
		}

		var err error
		result, err = q.UpdateTableStatus(ctx, UpdateTableStatusParams{
			ID:                   arg.ID,
			Status:               arg.Status,
			CurrentReservationID: arg.CurrentReservationID,
		})
		if err != nil {
			return fmt.Errorf("update table status: %w", err)
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

		// 1. 锁定桌台，和预订创建/桌台修改共享同一并发边界。
		table, err := q.GetTableForUpdate(ctx, arg.TableID)
		if err != nil {
			return err
		}
		if table.CurrentReservationID.Valid {
			return ErrTableHasActiveReservation
		}

		// 2. 检查是否有未来的预定
		count, err := q.CountFutureReservationsByTable(ctx, arg.TableID)
		if err != nil {
			return err
		}
		if count > 0 {
			return ErrTableHasFutureReservations
		}

		// 3. 删除所有标签关联
		err = q.RemoveAllTableTags(ctx, arg.TableID)
		if err != nil {
			return err
		}

		// 4. 删除桌台
		err = q.DeleteTable(ctx, arg.TableID)
		if err != nil {
			return err
		}

		result.TableID = arg.TableID
		return nil
	})

	return result, err
}

// AddTableImageTx adds an image and clears other primary images atomically when the new image is primary.
func (store *SQLStore) AddTableImageTx(ctx context.Context, arg AddTableImageTxParams) (TableImage, error) {
	var result TableImage

	err := store.execTx(ctx, func(q *Queries) error {
		var err error
		if arg.Image.IsPrimary {
			if _, err = q.GetTableForUpdate(ctx, arg.Image.TableID); err != nil {
				return err
			}
		}

		result, err = q.AddTableImage(ctx, arg.Image)
		if err != nil {
			return err
		}

		if result.IsPrimary {
			if err := q.ClearOtherPrimaryTableImages(ctx, ClearOtherPrimaryTableImagesParams{
				TableID: result.TableID,
				ID:      result.ID,
			}); err != nil {
				return err
			}
		}

		return nil
	})

	return result, err
}

// SetTableImagePrimaryTx verifies the image belongs to the table, then switches the table primary image atomically.
func (store *SQLStore) SetTableImagePrimaryTx(ctx context.Context, arg SetTableImagePrimaryTxParams) (TableImage, error) {
	var result TableImage

	err := store.execTx(ctx, func(q *Queries) error {
		if _, err := q.GetTableForUpdate(ctx, arg.TableID); err != nil {
			return err
		}

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
