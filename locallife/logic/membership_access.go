package logic

import (
	"context"
	"errors"
	"net/http"

	db "github.com/merrydance/locallife/db/sqlc"
)

type MembershipAccessInput struct {
	UserID       int64
	MembershipID int64
}

type MembershipTransactionsInput struct {
	UserID       int64
	MembershipID int64
	Limit        int32
	Offset       int32
}

func GetMembershipForUser(ctx context.Context, store db.Store, input MembershipAccessInput) (db.MerchantMembership, error) {
	membership, err := store.GetMerchantMembership(ctx, input.MembershipID)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			return db.MerchantMembership{}, NewRequestError(http.StatusNotFound, errors.New("membership not found"))
		}
		return db.MerchantMembership{}, err
	}

	if membership.UserID != input.UserID {
		return db.MerchantMembership{}, NewRequestError(http.StatusForbidden, errors.New("not authorized"))
	}

	return membership, nil
}

func ListMembershipTransactionsForUser(ctx context.Context, store db.Store, input MembershipTransactionsInput) ([]db.MembershipTransaction, error) {
	_, err := GetMembershipForUser(ctx, store, MembershipAccessInput{
		UserID:       input.UserID,
		MembershipID: input.MembershipID,
	})
	if err != nil {
		return nil, err
	}

	transactions, err := store.ListMembershipTransactions(ctx, db.ListMembershipTransactionsParams{
		MembershipID: input.MembershipID,
		Limit:        input.Limit,
		Offset:       input.Offset,
	})
	if err != nil {
		return nil, err
	}

	return transactions, nil
}
