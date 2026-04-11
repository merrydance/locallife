package db

import "context"

type CreateUserAddressTxParams struct {
	CreateUserAddressParams
}

type CreateUserAddressTxResult struct {
	Address UserAddress
}

func (store *SQLStore) CreateUserAddressTx(ctx context.Context, arg CreateUserAddressTxParams) (CreateUserAddressTxResult, error) {
	var result CreateUserAddressTxResult
	err := store.execTx(ctx, func(q *Queries) error {
		if arg.IsDefault {
			if err := q.SetDefaultAddress(ctx, arg.UserID); err != nil {
				return err
			}
		}
		address, err := q.CreateUserAddress(ctx, arg.CreateUserAddressParams)
		if err != nil {
			return err
		}
		result.Address = address
		return nil
	})

	return result, err
}

type SetDefaultAddressTxParams struct {
	UserID    int64
	AddressID int64
}

type SetDefaultAddressTxResult struct {
	Address UserAddress
}

func (store *SQLStore) SetDefaultAddressTx(ctx context.Context, arg SetDefaultAddressTxParams) (SetDefaultAddressTxResult, error) {
	var result SetDefaultAddressTxResult
	err := store.execTx(ctx, func(q *Queries) error {
		if err := q.SetDefaultAddress(ctx, arg.UserID); err != nil {
			return err
		}
		address, err := q.SetAddressAsDefault(ctx, SetAddressAsDefaultParams{
			ID:     arg.AddressID,
			UserID: arg.UserID,
		})
		if err != nil {
			return err
		}
		result.Address = address
		return nil
	})

	return result, err
}
