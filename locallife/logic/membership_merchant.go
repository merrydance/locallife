package logic

import (
	"context"
	"errors"
	"net/http"

	db "github.com/merrydance/locallife/db/sqlc"
)

type MerchantMembersInput struct {
	MerchantID       int64
	TargetMerchantID int64
	Limit            int32
	Offset           int32
}

type MerchantMemberDetailInput struct {
	MerchantID        int64
	TargetMerchantID  int64
	UserID            int64
	TransactionsLimit int32
}

type MerchantMemberDetailResult struct {
	Membership   db.MerchantMembership
	User         db.User
	Transactions []db.MembershipTransaction
}

type MerchantMembersResult struct {
	Members []db.ListMerchantMembersRow
	Total   int64
}

func ListMerchantMembers(ctx context.Context, store db.Store, input MerchantMembersInput) (MerchantMembersResult, error) {
	var result MerchantMembersResult

	if input.MerchantID != input.TargetMerchantID {
		return result, NewRequestError(http.StatusForbidden, errors.New("not authorized for this merchant"))
	}

	members, err := store.ListMerchantMembers(ctx, db.ListMerchantMembersParams{
		MerchantID: input.TargetMerchantID,
		Limit:      input.Limit,
		Offset:     input.Offset,
	})
	if err != nil {
		return result, err
	}

	total, err := store.CountMerchantMembers(ctx, input.TargetMerchantID)
	if err != nil {
		return result, err
	}

	result.Members = members
	result.Total = total
	return result, nil
}

func GetMerchantMemberDetail(ctx context.Context, store db.Store, input MerchantMemberDetailInput) (MerchantMemberDetailResult, error) {
	var result MerchantMemberDetailResult

	if input.MerchantID != input.TargetMerchantID {
		return result, NewRequestError(http.StatusForbidden, errors.New("not authorized for this merchant"))
	}

	membership, err := store.GetMembershipByMerchantAndUser(ctx, db.GetMembershipByMerchantAndUserParams{
		MerchantID: input.TargetMerchantID,
		UserID:     input.UserID,
	})
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			return result, NewRequestError(http.StatusNotFound, errors.New("membership not found"))
		}
		return result, err
	}

	user, err := store.GetUser(ctx, input.UserID)
	if err != nil {
		return result, err
	}

	limit := input.TransactionsLimit
	if limit <= 0 {
		limit = 20
	}

	transactions, err := store.ListMembershipTransactions(ctx, db.ListMembershipTransactionsParams{
		MembershipID: membership.ID,
		Limit:        limit,
		Offset:       0,
	})
	if err != nil {
		return result, err
	}

	result.Membership = membership
	result.User = user
	result.Transactions = transactions

	return result, nil
}
