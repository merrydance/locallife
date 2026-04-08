package logic

import (
	"context"
	"errors"

	db "github.com/merrydance/locallife/db/sqlc"
)

func allowedPackagingOrderType(orderType string) bool {
	return orderType == db.OrderTypeTakeout || orderType == "takeaway"
}

func uniqueInt64s(values []int64) []int64 {
	if len(values) == 0 {
		return []int64{}
	}

	result := make([]int64, 0, len(values))
	seen := make(map[int64]struct{}, len(values))
	for _, value := range values {
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}

	return result
}

func (s *OrderService) validatePackagingPolicy(ctx context.Context, merchantID int64, orderType string, items []db.CreateOrderItemParams) error {
	if !allowedPackagingOrderType(orderType) {
		return nil
	}

	activePackagingCount, err := s.store.CountActivePackagingDishesByMerchant(ctx, merchantID)
	if err != nil {
		return err
	}
	if activePackagingCount == 0 {
		return nil
	}

	selectedDishIDs := make([]int64, 0, len(items))
	for _, item := range items {
		if !item.DishID.Valid {
			continue
		}
		selectedDishIDs = append(selectedDishIDs, item.DishID.Int64)
	}
	selectedDishIDs = uniqueInt64s(selectedDishIDs)
	if len(selectedDishIDs) == 0 {
		return errors.New("请先选择包装方式")
	}

	selectedPackagingDishIDs := make(map[int64]struct{}, len(selectedDishIDs))
	for _, dishID := range selectedDishIDs {
		dish, err := s.store.GetDish(ctx, dishID)
		if err != nil {
			if errors.Is(err, db.ErrRecordNotFound) {
				continue
			}
			return err
		}
		if dish.MerchantID != merchantID {
			continue
		}
		if !dish.IsPackaging || !dish.IsOnline || !dish.IsAvailable {
			continue
		}
		selectedPackagingDishIDs[dish.ID] = struct{}{}
	}

	selectedCount := int64(0)
	for _, item := range items {
		if !item.DishID.Valid {
			continue
		}
		if _, ok := selectedPackagingDishIDs[item.DishID.Int64]; !ok {
			continue
		}
		selectedCount += int64(item.Quantity)
	}

	if selectedCount == 0 {
		return errors.New("请先选择包装方式")
	}
	if selectedCount > 1 {
		return errors.New("只能选择一种包装方式")
	}

	return nil
}
