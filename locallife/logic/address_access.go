package logic

import (
	"context"
	"errors"
	"net/http"

	db "github.com/merrydance/locallife/db/sqlc"
)

// loadOwnedUserAddress resolves an address and enforces ownership for the current user.
func loadOwnedUserAddress(ctx context.Context, store db.Store, userID, addressID int64) (db.UserAddress, error) {
	address, err := store.GetUserAddress(ctx, addressID)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			return db.UserAddress{}, NewRequestError(http.StatusNotFound, errors.New("address not found"))
		}
		return db.UserAddress{}, err
	}
	if address.UserID != userID {
		return db.UserAddress{}, NewRequestError(http.StatusForbidden, errors.New("address does not belong to you"))
	}
	return address, nil
}
