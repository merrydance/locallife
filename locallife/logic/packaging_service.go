package logic

import (
	"context"
	"errors"
	"net/http"

	"github.com/jackc/pgx/v5/pgtype"

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

type ResolveCartPackagingStateInput struct {
	UserID     int64
	MerchantID int64
	OrderType  string
	Cart       *db.Cart
}

type CartPackagingSelectionInput struct {
	UserID            int64
	MerchantID        int64
	OrderType         string
	TableID           *int64
	ReservationID     *int64
	PackagingOptionID int64
}

type CartPackagingSelectionResult struct {
	SelectedOptionID *int64
	SelectionVersion int64
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

func (s *PackagingService) ResolveCartPackagingState(ctx context.Context, input ResolveCartPackagingStateInput) (CartPackagingState, error) {
	state := CartPackagingState{
		Options: []db.MerchantPackagingOption{},
	}
	if !allowedPackagingOrderType(input.OrderType) {
		return state, nil
	}

	settings, err := s.store.GetMerchantPackagingSettings(ctx, input.MerchantID)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			return state, nil
		}
		return state, err
	}

	state.Enabled = settings.Enabled
	state.Required = settings.Required
	state.Applicable = packagingAppliesToOrderType(settings.ApplicableOrderTypes, input.OrderType)
	if !state.Enabled || !state.Applicable {
		return state, nil
	}

	options, err := s.store.ListEnabledMerchantPackagingOptions(ctx, input.MerchantID)
	if err != nil {
		return state, err
	}
	state.Options = options

	if input.Cart == nil {
		return state, nil
	}
	if err := validateCartPackagingContext(*input.Cart, input.UserID, input.MerchantID, input.OrderType, nil, nil); err != nil {
		return state, err
	}

	selection, err := s.store.GetCartPackagingSelection(ctx, input.Cart.ID)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			return state, nil
		}
		return state, err
	}
	state.SelectionVersion = selection.SelectionVersion
	if selection.PackagingOptionID.Valid && cartPackagingOptionIsEnabled(options, selection.PackagingOptionID.Int64) {
		optionID := selection.PackagingOptionID.Int64
		state.SelectedOptionID = &optionID
	}
	return state, nil
}

func (s *PackagingService) SelectCartPackagingOption(ctx context.Context, input CartPackagingSelectionInput) (CartPackagingSelectionResult, error) {
	var result CartPackagingSelectionResult
	cart, err := s.loadCartForPackagingSelection(ctx, input)
	if err != nil {
		return result, err
	}
	if err := s.validatePackagingSelectionEnabled(ctx, input.MerchantID, input.OrderType); err != nil {
		return result, err
	}

	option, err := s.store.GetMerchantPackagingOption(ctx, db.GetMerchantPackagingOptionParams{
		ID:         input.PackagingOptionID,
		MerchantID: input.MerchantID,
	})
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			return result, NewRequestError(http.StatusBadRequest, errors.New("包装方式不可用"))
		}
		return result, err
	}
	if option.MerchantID != input.MerchantID || !option.IsEnabled || option.DeletedAt.Valid {
		return result, NewRequestError(http.StatusBadRequest, errors.New("包装方式不可用"))
	}

	selection, err := s.store.UpsertCartPackagingSelection(ctx, db.UpsertCartPackagingSelectionParams{
		CartID:            cart.ID,
		PackagingOptionID: pgtype.Int8{Int64: option.ID, Valid: true},
	})
	if err != nil {
		return result, err
	}
	return cartPackagingSelectionResult(selection), nil
}

func (s *PackagingService) ClearCartPackagingSelection(ctx context.Context, input CartPackagingSelectionInput) (CartPackagingSelectionResult, error) {
	var result CartPackagingSelectionResult
	cart, err := s.loadCartForPackagingSelection(ctx, input)
	if err != nil {
		return result, err
	}

	selection, err := s.store.ClearCartPackagingSelection(ctx, cart.ID)
	if err != nil {
		return result, err
	}
	return cartPackagingSelectionResult(selection), nil
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

func (s *PackagingService) loadCartForPackagingSelection(ctx context.Context, input CartPackagingSelectionInput) (db.Cart, error) {
	orderType := input.OrderType
	if orderType == "" {
		orderType = db.OrderTypeTakeout
	}
	cart, err := s.store.GetCartByUserAndMerchant(ctx, db.GetCartByUserAndMerchantParams{
		UserID:        input.UserID,
		MerchantID:    input.MerchantID,
		OrderType:     orderType,
		TableID:       pgInt8FromInt64Ptr(input.TableID),
		ReservationID: pgInt8FromInt64Ptr(input.ReservationID),
	})
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			return db.Cart{}, NewRequestError(http.StatusNotFound, errors.New("购物车不存在"))
		}
		return db.Cart{}, err
	}
	if err := validateCartPackagingContext(cart, input.UserID, input.MerchantID, orderType, input.TableID, input.ReservationID); err != nil {
		return db.Cart{}, err
	}
	return cart, nil
}

func (s *PackagingService) validatePackagingSelectionEnabled(ctx context.Context, merchantID int64, orderType string) error {
	if !allowedPackagingOrderType(orderType) {
		return NewRequestError(http.StatusBadRequest, errors.New("当前订单不需要包装方式"))
	}
	settings, err := s.store.GetMerchantPackagingSettings(ctx, merchantID)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			return NewRequestError(http.StatusBadRequest, errors.New("当前订单不需要包装方式"))
		}
		return err
	}
	if !settings.Enabled || !packagingAppliesToOrderType(settings.ApplicableOrderTypes, orderType) {
		return NewRequestError(http.StatusBadRequest, errors.New("当前订单不需要包装方式"))
	}
	return nil
}

func validateCartPackagingContext(cart db.Cart, userID, merchantID int64, orderType string, tableID, reservationID *int64) error {
	if cart.UserID != userID {
		return NewRequestError(http.StatusForbidden, errors.New("购物车不属于你"))
	}
	if cart.MerchantID != merchantID || cart.OrderType != orderType {
		return NewRequestError(http.StatusConflict, errors.New("购物车状态已变化，请刷新后重试"))
	}
	if tableID != nil || cart.TableID.Valid {
		if !pgInt8MatchesInt64Ptr(cart.TableID, tableID) {
			return NewRequestError(http.StatusConflict, errors.New("购物车状态已变化，请刷新后重试"))
		}
	}
	if reservationID != nil || cart.ReservationID.Valid {
		if !pgInt8MatchesInt64Ptr(cart.ReservationID, reservationID) {
			return NewRequestError(http.StatusConflict, errors.New("购物车状态已变化，请刷新后重试"))
		}
	}
	return nil
}

func cartPackagingOptionIsEnabled(options []db.MerchantPackagingOption, optionID int64) bool {
	for _, option := range options {
		if option.ID == optionID && option.IsEnabled && !option.DeletedAt.Valid {
			return true
		}
	}
	return false
}

func cartPackagingSelectionResult(selection db.CartPackagingSelection) CartPackagingSelectionResult {
	result := CartPackagingSelectionResult{
		SelectionVersion: selection.SelectionVersion,
	}
	if selection.PackagingOptionID.Valid {
		optionID := selection.PackagingOptionID.Int64
		result.SelectedOptionID = &optionID
	}
	return result
}

func pgInt8FromInt64Ptr(value *int64) pgtype.Int8 {
	if value == nil {
		return pgtype.Int8{}
	}
	return pgtype.Int8{Int64: *value, Valid: true}
}

func pgInt8MatchesInt64Ptr(value pgtype.Int8, expected *int64) bool {
	if expected == nil {
		return !value.Valid
	}
	return value.Valid && value.Int64 == *expected
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
