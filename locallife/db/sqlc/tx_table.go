package db

import (
	"context"
	"errors"
)

// DeleteTableResult 删除桌台的结果
type DeleteTableResult struct {
	TableID int64
}

// DeleteTableParams 删除桌台的参数
type DeleteTableParams struct {
	TableID int64
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
