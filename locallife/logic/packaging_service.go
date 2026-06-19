package logic

import (
	"context"
	"errors"
	"net/http"

	db "github.com/merrydance/locallife/db/sqlc"
)

type PackagingRequirement struct {
	Enabled        bool
	Required       bool
	Applicable     bool
	Options        []db.MerchantPackagingOption
	SelectedOption *db.MerchantPackagingOption
}

type ResolvePackagingInput struct {
	UserID            int64
	MerchantID        int64
	OrderType         string
	CartID            *int64
	PackagingOptionID *int64
}

type OrderPackagingSnapshot struct {
	PackagingOptionID *int64
	Name              string
	UnitPrice         int64
	Quantity          int16
	Subtotal          int64
}

type PackagingService struct {
	store db.Store
}

func NewPackagingService(store db.Store) *PackagingService {
	return &PackagingService{store: store}
}

func (s *PackagingService) ResolvePackagingRequirement(ctx context.Context, input ResolvePackagingInput) (PackagingRequirement, error) {
	var requirement PackagingRequirement
	if !allowedPackagingOrderType(input.OrderType) {
		return requirement, nil
	}

	settings, err := s.store.GetMerchantPackagingSettings(ctx, input.MerchantID)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			return requirement, nil
		}
		return requirement, err
	}

	requirement.Enabled = settings.Enabled
	requirement.Required = settings.Required
	requirement.Applicable = packagingAppliesToOrderType(settings.ApplicableOrderTypes, input.OrderType)
	if !requirement.Applicable || !requirement.Enabled {
		return requirement, nil
	}

	selectedOptionID, err := s.resolveSelectedPackagingOptionID(ctx, input)
	if err != nil {
		return requirement, err
	}

	options, err := s.store.ListEnabledMerchantPackagingOptions(ctx, input.MerchantID)
	if err != nil {
		return requirement, err
	}
	requirement.Options = options

	if selectedOptionID != nil {
		option, err := s.store.GetMerchantPackagingOption(ctx, db.GetMerchantPackagingOptionParams{
			ID:         *selectedOptionID,
			MerchantID: input.MerchantID,
		})
		if err != nil {
			if errors.Is(err, db.ErrRecordNotFound) {
				return requirement, NewRequestError(http.StatusBadRequest, errors.New("包装方式不可用"))
			}
			return requirement, err
		}
		if option.MerchantID != input.MerchantID || !option.IsEnabled || option.DeletedAt.Valid {
			return requirement, NewRequestError(http.StatusBadRequest, errors.New("包装方式不可用"))
		}
		requirement.SelectedOption = &option
	}

	if requirement.Required && requirement.SelectedOption == nil {
		return requirement, NewRequestError(http.StatusBadRequest, errors.New("请先选择包装方式"))
	}

	return requirement, nil
}

func (s *PackagingService) resolveSelectedPackagingOptionID(ctx context.Context, input ResolvePackagingInput) (*int64, error) {
	if input.CartID == nil {
		return input.PackagingOptionID, nil
	}

	cart, err := s.store.GetCart(ctx, *input.CartID)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			return nil, NewRequestError(http.StatusBadRequest, errors.New("购物车不存在"))
		}
		return nil, err
	}
	if cart.UserID != input.UserID {
		return nil, NewRequestError(http.StatusForbidden, errors.New("购物车不属于你"))
	}
	if cart.MerchantID != input.MerchantID || cart.OrderType != input.OrderType {
		return nil, NewRequestError(http.StatusConflict, errors.New("购物车状态已变化，请刷新后重试"))
	}
	if input.PackagingOptionID != nil {
		return input.PackagingOptionID, nil
	}

	selection, err := s.store.GetCartPackagingSelection(ctx, *input.CartID)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	if !selection.PackagingOptionID.Valid {
		return nil, nil
	}
	optionID := selection.PackagingOptionID.Int64
	return &optionID, nil
}

func BuildOrderPackagingSnapshot(option db.MerchantPackagingOption) OrderPackagingSnapshot {
	optionID := option.ID
	return OrderPackagingSnapshot{
		PackagingOptionID: &optionID,
		Name:              option.Name,
		UnitPrice:         option.Price,
		Quantity:          1,
		Subtotal:          option.Price,
	}
}

func packagingAppliesToOrderType(orderTypes []string, orderType string) bool {
	if len(orderTypes) == 0 {
		return allowedPackagingOrderType(orderType)
	}
	for _, candidate := range orderTypes {
		if candidate == orderType {
			return true
		}
	}
	return false
}
