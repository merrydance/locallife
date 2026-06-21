package logic

import (
	"context"
	"fmt"
	"net/http"

	db "github.com/merrydance/locallife/db/sqlc"
)

func validateComboChildDishesOrderable(ctx context.Context, store db.Store, comboID int64, comboName string, rejectLegacyPackaging bool) error {
	dishes, err := store.ListComboDishOrderability(ctx, comboID)
	if err != nil {
		return err
	}
	if len(dishes) == 0 {
		return NewRequestError(http.StatusBadRequest, fmt.Errorf("combo %s has no available dishes", comboName))
	}
	for _, dish := range dishes {
		dishName := dish.DishName
		if dishName == "" {
			dishName = fmt.Sprintf("%d", dish.DishID)
		}
		if !dish.DishExists.Valid || !dish.DishExists.Bool {
			return NewRequestError(http.StatusBadRequest, fmt.Errorf("combo %s contains unavailable dish %s", comboName, dishName))
		}
		if !dish.IsOnline {
			return NewRequestError(http.StatusBadRequest, fmt.Errorf("combo %s contains offline dish %s", comboName, dishName))
		}
		if !dish.IsAvailable {
			return NewRequestError(http.StatusBadRequest, fmt.Errorf("combo %s contains unavailable dish %s", comboName, dishName))
		}
		if rejectLegacyPackaging && dish.IsPackaging {
			return NewRequestError(http.StatusBadRequest, fmt.Errorf("包装已迁移到包装设置，请在包装设置中维护"))
		}
	}
	return nil
}
