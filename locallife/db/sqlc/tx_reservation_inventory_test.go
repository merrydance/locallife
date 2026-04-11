package db

import (
    "context"
    "testing"
    "time"

    "github.com/jackc/pgx/v5/pgtype"
    "github.com/stretchr/testify/require"
)

func createReservationWithItems(t *testing.T, merchant Merchant, table Table, user User, items []CreateReservationItemParams) TableReservation {
    reservation := createRandomReservation(t, user.ID, merchant.ID, table.ID, "paid")

    for _, item := range items {
        _, err := testStore.CreateReservationItem(context.Background(), CreateReservationItemParams{
            ReservationID: reservation.ID,
            DishID:        item.DishID,
            ComboID:       item.ComboID,
            Quantity:      item.Quantity,
            UnitPrice:     item.UnitPrice,
            TotalPrice:    item.TotalPrice,
        })
        require.NoError(t, err)
    }

    return reservation
}

func TestSyncReservationInventoryTx_ReserveAndRelease(t *testing.T) {
    owner := createRandomUser(t)
    merchant := createRandomMerchantWithOwner(t, owner.ID)
    table := createRandomTable(t, merchant.ID)
    user := createRandomUser(t)
    category := createRandomDishCategory(t)
    dish1 := createRandomDish(t, merchant.ID, category.ID)
    dish2 := createRandomDish(t, merchant.ID, category.ID)

    reservationDate := time.Now().Add(24 * time.Hour)
    _, err := testStore.CreateDailyInventory(context.Background(), CreateDailyInventoryParams{
        MerchantID:    merchant.ID,
        DishID:        dish1.ID,
        Date:          pgtype.Date{Time: reservationDate, Valid: true},
        TotalQuantity: 10,
        SoldQuantity:  0,
    })
    require.NoError(t, err)
    _, err = testStore.CreateDailyInventory(context.Background(), CreateDailyInventoryParams{
        MerchantID:    merchant.ID,
        DishID:        dish2.ID,
        Date:          pgtype.Date{Time: reservationDate, Valid: true},
        TotalQuantity: 10,
        SoldQuantity:  0,
    })
    require.NoError(t, err)

    reservation := createReservationWithItems(t, merchant, table, user, []CreateReservationItemParams{
        {
            DishID:     pgtype.Int8{Int64: dish1.ID, Valid: true},
            Quantity:   2,
            UnitPrice:  dish1.Price,
            TotalPrice: dish1.Price * 2,
        },
        {
            DishID:     pgtype.Int8{Int64: dish2.ID, Valid: true},
            Quantity:   1,
            UnitPrice:  dish2.Price,
            TotalPrice: dish2.Price,
        },
    })

    _, err = testStore.SyncReservationInventoryTx(context.Background(), SyncReservationInventoryTxParams{
        ReservationID: reservation.ID,
    })
    require.NoError(t, err)

    inventory1, err := testStore.GetDailyInventory(context.Background(), GetDailyInventoryParams{
        MerchantID: merchant.ID,
        DishID:     dish1.ID,
        Date:       pgtype.Date{Time: reservation.ReservationDate.Time, Valid: true},
    })
    require.NoError(t, err)
    require.Equal(t, int32(2), inventory1.ReservedQuantity)

    inventory2, err := testStore.GetDailyInventory(context.Background(), GetDailyInventoryParams{
        MerchantID: merchant.ID,
        DishID:     dish2.ID,
        Date:       pgtype.Date{Time: reservation.ReservationDate.Time, Valid: true},
    })
    require.NoError(t, err)
    require.Equal(t, int32(1), inventory2.ReservedQuantity)

    entries, err := testStore.ListReservationInventoryByReservation(context.Background(), reservation.ID)
    require.NoError(t, err)
    require.Len(t, entries, 2)

    _, err = testStore.ReplaceReservationItemsTx(context.Background(), ReplaceReservationItemsTxParams{
        ReservationID: reservation.ID,
        Items: []CreateReservationItemParams{
            {
                ReservationID: reservation.ID,
                DishID:        pgtype.Int8{Int64: dish1.ID, Valid: true},
                Quantity:      1,
                UnitPrice:     dish1.Price,
                TotalPrice:    dish1.Price,
            },
        },
    })
    require.NoError(t, err)

    _, err = testStore.SyncReservationInventoryTx(context.Background(), SyncReservationInventoryTxParams{
        ReservationID: reservation.ID,
    })
    require.NoError(t, err)

    inventory1, err = testStore.GetDailyInventory(context.Background(), GetDailyInventoryParams{
        MerchantID: merchant.ID,
        DishID:     dish1.ID,
        Date:       pgtype.Date{Time: reservation.ReservationDate.Time, Valid: true},
    })
    require.NoError(t, err)
    require.Equal(t, int32(1), inventory1.ReservedQuantity)

    inventory2, err = testStore.GetDailyInventory(context.Background(), GetDailyInventoryParams{
        MerchantID: merchant.ID,
        DishID:     dish2.ID,
        Date:       pgtype.Date{Time: reservation.ReservationDate.Time, Valid: true},
    })
    require.NoError(t, err)
    require.Equal(t, int32(0), inventory2.ReservedQuantity)

    entries, err = testStore.ListReservationInventoryByReservation(context.Background(), reservation.ID)
    require.NoError(t, err)
    require.Len(t, entries, 1)
    require.Equal(t, dish1.ID, entries[0].DishID)
    require.Equal(t, int32(1), entries[0].Quantity)
}

func TestReleaseReservationInventoryTx(t *testing.T) {
    owner := createRandomUser(t)
    merchant := createRandomMerchantWithOwner(t, owner.ID)
    table := createRandomTable(t, merchant.ID)
    user := createRandomUser(t)
    category := createRandomDishCategory(t)
    dish := createRandomDish(t, merchant.ID, category.ID)

    reservationDate := time.Now().Add(24 * time.Hour)
    _, err := testStore.CreateDailyInventory(context.Background(), CreateDailyInventoryParams{
        MerchantID:    merchant.ID,
        DishID:        dish.ID,
        Date:          pgtype.Date{Time: reservationDate, Valid: true},
        TotalQuantity: 10,
        SoldQuantity:  0,
    })
    require.NoError(t, err)

    reservation := createReservationWithItems(t, merchant, table, user, []CreateReservationItemParams{
        {
            DishID:     pgtype.Int8{Int64: dish.ID, Valid: true},
            Quantity:   2,
            UnitPrice:  dish.Price,
            TotalPrice: dish.Price * 2,
        },
    })

    _, err = testStore.SyncReservationInventoryTx(context.Background(), SyncReservationInventoryTxParams{
        ReservationID: reservation.ID,
    })
    require.NoError(t, err)

    err = testStore.ReleaseReservationInventoryTx(context.Background(), ReleaseReservationInventoryTxParams{
        ReservationID: reservation.ID,
    })
    require.NoError(t, err)

    inventory, err := testStore.GetDailyInventory(context.Background(), GetDailyInventoryParams{
        MerchantID: merchant.ID,
        DishID:     dish.ID,
        Date:       pgtype.Date{Time: reservation.ReservationDate.Time, Valid: true},
    })
    require.NoError(t, err)
    require.Equal(t, int32(0), inventory.ReservedQuantity)

    entries, err := testStore.ListReservationInventoryByReservation(context.Background(), reservation.ID)
    require.NoError(t, err)
    require.Len(t, entries, 0)
}
