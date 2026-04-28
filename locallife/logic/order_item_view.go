package logic

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
)

type OrderItemCustomization struct {
	GroupID    int64  `json:"group_id,omitempty"`
	OptionID   int64  `json:"option_id,omitempty"`
	TagID      int64  `json:"tag_id,omitempty"`
	Name       string `json:"name,omitempty"`
	Value      string `json:"value,omitempty"`
	ExtraPrice int64  `json:"extra_price,omitempty"`
}

type OrderItemView struct {
	ID             int64                    `json:"id"`
	OrderID        int64                    `json:"order_id"`
	DishID         *int64                   `json:"dish_id,omitempty"`
	ComboID        *int64                   `json:"combo_id,omitempty"`
	Name           string                   `json:"name"`
	UnitPrice      int64                    `json:"unit_price"`
	Quantity       int16                    `json:"quantity"`
	Subtotal       int64                    `json:"subtotal"`
	SpecsText      string                   `json:"specs_text"`
	Customizations []OrderItemCustomization `json:"customizations,omitempty"`
	ImageAssetID   *int64                   `json:"-"`
}

func BuildOrderItemViews(rows []db.ListOrderItemsWithDishByOrderRow) ([]OrderItemView, error) {
	views := make([]OrderItemView, len(rows))
	for i, row := range rows {
		view, err := buildOrderItemView(row.ID, row.OrderID, row.DishID, row.ComboID, row.Name, row.UnitPrice, row.Quantity, row.Subtotal, row.Customizations, row.DishImageMediaAssetID)
		if err != nil {
			return nil, err
		}
		views[i] = view
	}
	return views, nil
}

func BuildOrderItemViewsFromOrderIDs(rows []db.ListOrderItemsWithDishByOrderIDsRow) ([]OrderItemView, error) {
	views := make([]OrderItemView, len(rows))
	for i, row := range rows {
		view, err := buildOrderItemView(row.ID, row.OrderID, row.DishID, row.ComboID, row.Name, row.UnitPrice, row.Quantity, row.Subtotal, row.Customizations, row.DishImageMediaAssetID)
		if err != nil {
			return nil, err
		}
		views[i] = view
	}
	return views, nil
}

func buildOrderItemView(id, orderID int64, dishID, comboID pgtype.Int8, name string, unitPrice int64, quantity int16, subtotal int64, rawCustomizations []byte, imageAssetID pgtype.Int8) (OrderItemView, error) {
	customizations, specsText, err := DecodeOrderItemCustomizations(rawCustomizations)
	if err != nil {
		return OrderItemView{}, fmt.Errorf("decode order item customizations for item %d: %w", id, err)
	}

	view := OrderItemView{
		ID:             id,
		OrderID:        orderID,
		Name:           name,
		UnitPrice:      unitPrice,
		Quantity:       quantity,
		Subtotal:       subtotal,
		SpecsText:      specsText,
		Customizations: customizations,
	}
	if dishID.Valid {
		view.DishID = &dishID.Int64
	}
	if comboID.Valid {
		view.ComboID = &comboID.Int64
	}
	if imageAssetID.Valid {
		view.ImageAssetID = &imageAssetID.Int64
	}
	return view, nil
}

func DecodeOrderItemCustomizations(raw []byte) ([]OrderItemCustomization, string, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return nil, "", nil
	}

	var meta map[string]any
	if err := json.Unmarshal(trimmed, &meta); err == nil {
		if specs, ok := meta["meta_specs"].(string); ok && strings.TrimSpace(specs) != "" {
			return nil, strings.TrimSpace(specs), nil
		}
	}

	var items []OrderItemCustomization
	if err := json.Unmarshal(trimmed, &items); err != nil {
		return nil, "", err
	}
	return items, orderItemSpecsText(items), nil
}

func orderItemSpecsText(items []OrderItemCustomization) string {
	parts := make([]string, 0, len(items))
	for _, item := range items {
		name := strings.TrimSpace(item.Name)
		value := strings.TrimSpace(item.Value)
		switch {
		case name != "" && value != "":
			parts = append(parts, name+"："+value)
		case value != "":
			parts = append(parts, value)
		}
	}
	return strings.Join(parts, " / ")
}
