package logic

import (
	"context"
	"errors"
	"net/http"

	db "github.com/merrydance/locallife/db/sqlc"
)

type PackagingPolicyCandidateDish struct {
	ID          int64
	Name        string
	Price       int64
	IsAvailable bool
	IsOnline    bool
}

type PackagingPolicyResult struct {
	MerchantID           int64
	ApplicableOrderTypes []string
	CandidateDishIDs     []int64
	CandidateDishes      []PackagingPolicyCandidateDish
}

type UpdatePackagingPolicyInput struct {
	OwnerUserID          int64
	ApplicableOrderTypes []string
	CandidateDishIDs     []int64
}

func defaultPackagingPolicy(merchantID int64) PackagingPolicyResult {
	return PackagingPolicyResult{
		MerchantID:           merchantID,
		ApplicableOrderTypes: []string{},
		CandidateDishIDs:     []int64{},
		CandidateDishes:      []PackagingPolicyCandidateDish{},
	}
}

func allowedPackagingOrderType(orderType string) bool {
	return orderType == db.OrderTypeTakeout || orderType == "takeaway"
}

func uniquePackagingOrderTypes(orderTypes []string) ([]string, error) {
	if len(orderTypes) == 0 {
		return []string{}, nil
	}

	result := make([]string, 0, len(orderTypes))
	seen := make(map[string]struct{}, len(orderTypes))
	for _, orderType := range orderTypes {
		if !allowedPackagingOrderType(orderType) {
			return nil, errors.New("packaging policy only supports takeout and takeaway")
		}
		if _, ok := seen[orderType]; ok {
			continue
		}
		seen[orderType] = struct{}{}
		result = append(result, orderType)
	}

	return result, nil
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

func ownerMerchantForPackagingPolicy(ctx context.Context, store db.Store, ownerUserID int64) (db.Merchant, error) {
	merchant, err := store.GetMerchantByOwner(ctx, ownerUserID)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			return db.Merchant{}, NewRequestError(http.StatusNotFound, errors.New("merchant not found"))
		}
		return db.Merchant{}, err
	}
	if merchant.OwnerUserID != ownerUserID {
		return db.Merchant{}, NewRequestError(http.StatusForbidden, errors.New("merchant owner required"))
	}
	return merchant, nil
}

func packagingCandidateDishesFromRows(rows []db.GetDishesByIDsAllRow, orderedIDs []int64) []PackagingPolicyCandidateDish {
	if len(rows) == 0 || len(orderedIDs) == 0 {
		return []PackagingPolicyCandidateDish{}
	}

	byID := make(map[int64]db.GetDishesByIDsAllRow, len(rows))
	for _, row := range rows {
		byID[row.ID] = row
	}

	result := make([]PackagingPolicyCandidateDish, 0, len(orderedIDs))
	for _, dishID := range orderedIDs {
		row, ok := byID[dishID]
		if !ok {
			continue
		}
		result = append(result, PackagingPolicyCandidateDish{
			ID:          row.ID,
			Name:        row.Name,
			Price:       row.Price,
			IsAvailable: row.IsAvailable,
			IsOnline:    row.IsOnline,
		})
	}

	return result
}

func loadPackagingCandidateDishes(ctx context.Context, store db.Store, merchantID int64, dishIDs []int64, requireSelectable bool) ([]PackagingPolicyCandidateDish, error) {
	if len(dishIDs) == 0 {
		return []PackagingPolicyCandidateDish{}, nil
	}

	rows, err := store.GetDishesByIDsAll(ctx, dishIDs)
	if err != nil {
		return nil, err
	}
	if len(rows) != len(dishIDs) {
		return nil, NewRequestError(http.StatusBadRequest, errors.New("packaging candidate dishes must exist and belong to the current merchant"))
	}

	for _, row := range rows {
		if row.MerchantID != merchantID {
			return nil, NewRequestError(http.StatusBadRequest, errors.New("packaging candidate dishes must exist and belong to the current merchant"))
		}
		if requireSelectable && (!row.IsAvailable || !row.IsOnline) {
			return nil, NewRequestError(http.StatusBadRequest, errors.New("packaging candidate dishes must be currently available and online"))
		}
	}

	return packagingCandidateDishesFromRows(rows, dishIDs), nil
}

func packagingPolicyResultFromModel(policy db.MerchantPackagingPolicy, candidateDishes []PackagingPolicyCandidateDish) PackagingPolicyResult {
	return PackagingPolicyResult{
		MerchantID:           policy.MerchantID,
		ApplicableOrderTypes: append([]string{}, policy.ApplicableOrderTypes...),
		CandidateDishIDs:     append([]int64{}, policy.CandidateDishIds...),
		CandidateDishes:      candidateDishes,
	}
}

func GetPackagingPolicyForOwner(ctx context.Context, store db.Store, ownerUserID int64) (PackagingPolicyResult, error) {
	merchant, err := ownerMerchantForPackagingPolicy(ctx, store, ownerUserID)
	if err != nil {
		return PackagingPolicyResult{}, err
	}

	policy, err := store.GetMerchantPackagingPolicy(ctx, merchant.ID)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			return defaultPackagingPolicy(merchant.ID), nil
		}
		return PackagingPolicyResult{}, err
	}

	candidateDishes, err := loadPackagingCandidateDishes(ctx, store, merchant.ID, policy.CandidateDishIds, false)
	if err != nil {
		var reqErr *RequestError
		if errors.As(err, &reqErr) {
			return PackagingPolicyResult{
				MerchantID:           merchant.ID,
				ApplicableOrderTypes: append([]string{}, policy.ApplicableOrderTypes...),
				CandidateDishIDs:     append([]int64{}, policy.CandidateDishIds...),
				CandidateDishes:      []PackagingPolicyCandidateDish{},
			}, nil
		}
		return PackagingPolicyResult{}, err
	}

	return packagingPolicyResultFromModel(policy, candidateDishes), nil
}

func UpdatePackagingPolicyForOwner(ctx context.Context, store db.Store, input UpdatePackagingPolicyInput) (PackagingPolicyResult, error) {
	merchant, err := ownerMerchantForPackagingPolicy(ctx, store, input.OwnerUserID)
	if err != nil {
		return PackagingPolicyResult{}, err
	}

	orderTypes, err := uniquePackagingOrderTypes(input.ApplicableOrderTypes)
	if err != nil {
		return PackagingPolicyResult{}, NewRequestError(http.StatusBadRequest, err)
	}
	dishIDs := uniqueInt64s(input.CandidateDishIDs)

	candidateDishes, err := loadPackagingCandidateDishes(ctx, store, merchant.ID, dishIDs, true)
	if err != nil {
		return PackagingPolicyResult{}, err
	}

	policy, err := store.UpsertMerchantPackagingPolicy(ctx, db.UpsertMerchantPackagingPolicyParams{
		MerchantID:           merchant.ID,
		ApplicableOrderTypes: orderTypes,
		CandidateDishIds:     dishIDs,
	})
	if err != nil {
		return PackagingPolicyResult{}, err
	}

	return packagingPolicyResultFromModel(policy, candidateDishes), nil
}

func packagingPolicyApplies(orderType string, applicableOrderTypes []string) bool {
	for _, current := range applicableOrderTypes {
		if current == orderType {
			return true
		}
	}
	return false
}

func activePackagingCandidateSet(candidateDishes []PackagingPolicyCandidateDish) map[int64]struct{} {
	if len(candidateDishes) == 0 {
		return map[int64]struct{}{}
	}

	set := make(map[int64]struct{}, len(candidateDishes))
	for _, dish := range candidateDishes {
		if !dish.IsAvailable || !dish.IsOnline {
			continue
		}
		set[dish.ID] = struct{}{}
	}
	return set
}

func (s *OrderService) validatePackagingPolicy(ctx context.Context, merchantID int64, orderType string, items []db.CreateOrderItemParams) error {
	if !allowedPackagingOrderType(orderType) {
		return nil
	}

	policy, err := s.store.GetMerchantPackagingPolicy(ctx, merchantID)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			return nil
		}
		return err
	}
	if !packagingPolicyApplies(orderType, policy.ApplicableOrderTypes) || len(policy.CandidateDishIds) == 0 {
		return nil
	}

	candidateDishes, err := loadPackagingCandidateDishes(ctx, s.store, merchantID, uniqueInt64s(policy.CandidateDishIds), false)
	if err != nil {
		var reqErr *RequestError
		if errors.As(err, &reqErr) {
			return nil
		}
		return err
	}

	activeCandidates := activePackagingCandidateSet(candidateDishes)
	if len(activeCandidates) == 0 {
		return nil
	}

	selectedCount := int64(0)
	for _, item := range items {
		if !item.DishID.Valid {
			continue
		}
		if _, ok := activeCandidates[item.DishID.Int64]; !ok {
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
