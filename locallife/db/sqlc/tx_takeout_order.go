package db

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

const deliveryPoolRetentionWindow = 365 * 24 * time.Hour

const (
	takeoutAvgPrepareTimeCalcDays       = 7
	defaultTakeoutRiderSpeedMetersHour  = 15000
	defaultTakeoutPrepareTimeMinutes    = 20
	minimumTakeoutDeliveryTimeInMinutes = 5
)

type AcceptTakeoutOrderTxParams struct {
	OrderID      int64
	OldStatus    string
	OperatorID   int64
	OperatorType string
}

type AcceptTakeoutOrderTxResult struct {
	Order     Order
	StatusLog OrderStatusLog
}

type MarkTakeoutOrderReadyTxParams struct {
	OrderID      int64
	OldStatus    string
	OperatorID   int64
	OperatorType string
}

type MarkTakeoutOrderReadyTxResult struct {
	Order     Order
	StatusLog OrderStatusLog
	Delivery  Delivery
	PoolItem  DeliveryPool
}

func (store *SQLStore) AcceptTakeoutOrderTx(ctx context.Context, arg AcceptTakeoutOrderTxParams) (AcceptTakeoutOrderTxResult, error) {
	var result AcceptTakeoutOrderTxResult

	err := store.execTx(ctx, func(q *Queries) error {
		var err error

		result.Order, err = q.UpdateOrderStatus(ctx, UpdateOrderStatusParams{
			ID:                arg.OrderID,
			Status:            OrderStatusPreparing,
			ExpectedStatus:    arg.OldStatus,
			FulfillmentStatus: pgtype.Text{String: FulfillmentStatusPreparing, Valid: true},
		})
		if err != nil {
			return fmt.Errorf("update takeout order to preparing: %w", err)
		}

		result.StatusLog, err = q.CreateOrderStatusLog(ctx, CreateOrderStatusLogParams{
			OrderID:      arg.OrderID,
			FromStatus:   pgtype.Text{String: arg.OldStatus, Valid: true},
			ToStatus:     OrderStatusPreparing,
			OperatorID:   pgtype.Int8{Int64: arg.OperatorID, Valid: true},
			OperatorType: pgtype.Text{String: arg.OperatorType, Valid: true},
		})
		if err != nil {
			return fmt.Errorf("create takeout accept status log: %w", err)
		}

		return nil
	})

	return result, err
}

func (store *SQLStore) MarkTakeoutOrderReadyTx(ctx context.Context, arg MarkTakeoutOrderReadyTxParams) (MarkTakeoutOrderReadyTxResult, error) {
	var result MarkTakeoutOrderReadyTxResult

	err := store.execTx(ctx, func(q *Queries) error {
		var err error

		result.Order, err = q.UpdateOrderStatus(ctx, UpdateOrderStatusParams{
			ID:                arg.OrderID,
			Status:            OrderStatusReady,
			ExpectedStatus:    arg.OldStatus,
			FulfillmentStatus: pgtype.Text{String: FulfillmentStatusReady, Valid: true},
		})
		if err != nil {
			return fmt.Errorf("update takeout order to ready: %w", err)
		}

		result.StatusLog, err = q.CreateOrderStatusLog(ctx, CreateOrderStatusLogParams{
			OrderID:      arg.OrderID,
			FromStatus:   pgtype.Text{String: arg.OldStatus, Valid: true},
			ToStatus:     OrderStatusReady,
			OperatorID:   pgtype.Int8{Int64: arg.OperatorID, Valid: true},
			OperatorType: pgtype.Text{String: arg.OperatorType, Valid: true},
		})
		if err != nil {
			return fmt.Errorf("create takeout ready status log: %w", err)
		}

		result.Delivery, err = ensureTakeoutDeliveryCreated(ctx, q, result.Order)
		if err != nil {
			return err
		}

		result.PoolItem, err = addTakeoutOrderToDeliveryPool(ctx, q, result.Order, result.Delivery)
		if err != nil {
			return err
		}

		return nil
	})

	return result, err
}

func ensureTakeoutDeliveryCreated(ctx context.Context, q *Queries, order Order) (Delivery, error) {
	existingDelivery, err := q.GetDeliveryByOrderID(ctx, order.ID)
	if err == nil {
		return existingDelivery, nil
	}
	if !errors.Is(err, ErrRecordNotFound) {
		return Delivery{}, fmt.Errorf("get delivery by order id: %w", err)
	}

	if !order.AddressID.Valid {
		return Delivery{}, fmt.Errorf("takeout order missing delivery address")
	}

	orderItems, err := q.ListOrderItemsByOrder(ctx, order.ID)
	if err != nil {
		return Delivery{}, fmt.Errorf("list order items: %w", err)
	}

	merchant, err := q.GetMerchant(ctx, order.MerchantID)
	if err != nil {
		return Delivery{}, fmt.Errorf("get merchant: %w", err)
	}

	deliverySnapshot, err := resolveTakeoutDeliverySnapshot(ctx, q, order)
	if err != nil {
		return Delivery{}, err
	}

	now := time.Now()
	var maxPrepareTime int16
	for _, item := range orderItems {
		if !item.DishID.Valid {
			continue
		}

		dish, err := q.GetDish(ctx, item.DishID.Int64)
		if err == nil && dish.PrepareTime > maxPrepareTime {
			maxPrepareTime = dish.PrepareTime
		}
	}

	if maxPrepareTime == 0 {
		avgTime, err := q.GetMerchantAvgPrepareTime(ctx, GetMerchantAvgPrepareTimeParams{
			MerchantID: merchant.ID,
			StartAt:    now.AddDate(0, 0, -takeoutAvgPrepareTimeCalcDays),
		})
		if err == nil && avgTime > 0 {
			maxPrepareTime = int16(avgTime)
		}
	}

	if maxPrepareTime == 0 {
		maxPrepareTime = int16(defaultTakeoutPrepareTimeMinutes)
	}

	estimatedPickupAt := now.Add(time.Duration(maxPrepareTime) * time.Minute)

	var deliveryDistance int32
	if order.DeliveryDistance.Valid {
		deliveryDistance = order.DeliveryDistance.Int32
	}

	deliveryTimeMinutes := float64(minimumTakeoutDeliveryTimeInMinutes)
	if order.DeliveryDuration.Valid && order.DeliveryDuration.Int32 > 0 {
		deliveryTimeMinutes = float64(order.DeliveryDuration.Int32) / 60.0
	} else if deliveryDistance > 0 {
		deliveryTimeMinutes = float64(deliveryDistance) / float64(defaultTakeoutRiderSpeedMetersHour) * 60
	}
	if deliveryTimeMinutes < minimumTakeoutDeliveryTimeInMinutes {
		deliveryTimeMinutes = minimumTakeoutDeliveryTimeInMinutes
	}

	estimatedDeliveryAt := estimatedPickupAt.Add(time.Duration(deliveryTimeMinutes) * time.Minute)

	delivery, err := q.CreateDelivery(ctx, CreateDeliveryParams{
		OrderID:             order.ID,
		PickupAddress:       merchant.Address,
		PickupLongitude:     merchant.Longitude,
		PickupLatitude:      merchant.Latitude,
		PickupContact:       pgtype.Text{String: merchant.Name, Valid: true},
		PickupPhone:         pgtype.Text{String: merchant.Phone, Valid: true},
		DeliveryAddress:     deliverySnapshot.Address,
		DeliveryLongitude:   deliverySnapshot.Longitude,
		DeliveryLatitude:    deliverySnapshot.Latitude,
		DeliveryContact:     deliverySnapshot.ContactName,
		DeliveryPhone:       deliverySnapshot.ContactPhone,
		Distance:            deliveryDistance,
		DeliveryFee:         order.DeliveryFee,
		EstimatedPickupAt:   pgtype.Timestamptz{Time: estimatedPickupAt, Valid: true},
		EstimatedDeliveryAt: pgtype.Timestamptz{Time: estimatedDeliveryAt, Valid: true},
	})
	if err != nil {
		return Delivery{}, fmt.Errorf("create delivery: %w", err)
	}

	return delivery, nil
}

type takeoutDeliverySnapshot struct {
	ContactName  pgtype.Text
	ContactPhone pgtype.Text
	Address      string
	Longitude    pgtype.Numeric
	Latitude     pgtype.Numeric
}

func resolveTakeoutDeliverySnapshot(ctx context.Context, q *Queries, order Order) (takeoutDeliverySnapshot, error) {
	if order.DeliveryAddressSnapshot.Valid && order.DeliveryLongitudeSnapshot.Valid && order.DeliveryLatitudeSnapshot.Valid {
		return takeoutDeliverySnapshot{
			ContactName:  order.DeliveryContactNameSnapshot,
			ContactPhone: order.DeliveryContactPhoneSnapshot,
			Address:      order.DeliveryAddressSnapshot.String,
			Longitude:    order.DeliveryLongitudeSnapshot,
			Latitude:     order.DeliveryLatitudeSnapshot,
		}, nil
	}

	userAddress, err := q.GetUserAddress(ctx, order.AddressID.Int64)
	if err != nil {
		return takeoutDeliverySnapshot{}, fmt.Errorf("get user address: %w", err)
	}

	return takeoutDeliverySnapshot{
		ContactName:  pgtype.Text{String: userAddress.ContactName, Valid: true},
		ContactPhone: pgtype.Text{String: userAddress.ContactPhone, Valid: true},
		Address:      userAddress.DetailAddress,
		Longitude:    userAddress.Longitude,
		Latitude:     userAddress.Latitude,
	}, nil
}

func addTakeoutOrderToDeliveryPool(ctx context.Context, q *Queries, order Order, delivery Delivery) (DeliveryPool, error) {
	existingPoolItem, err := q.GetDeliveryPoolByOrderID(ctx, order.ID)
	if err == nil {
		return existingPoolItem, nil
	}
	if !errors.Is(err, ErrRecordNotFound) {
		return DeliveryPool{}, fmt.Errorf("get delivery pool by order id: %w", err)
	}

	expectedPickupAt := delivery.CreatedAt
	if delivery.EstimatedPickupAt.Valid {
		expectedPickupAt = delivery.EstimatedPickupAt.Time
	}

	poolItem, err := q.AddToDeliveryPool(ctx, AddToDeliveryPoolParams{
		OrderID:            order.ID,
		MerchantID:         order.MerchantID,
		PickupLongitude:    delivery.PickupLongitude,
		PickupLatitude:     delivery.PickupLatitude,
		DeliveryLongitude:  delivery.DeliveryLongitude,
		DeliveryLatitude:   delivery.DeliveryLatitude,
		Distance:           delivery.Distance,
		DeliveryFee:        order.DeliveryFee,
		ExpectedPickupAt:   expectedPickupAt,
		ExpectedDeliveryAt: delivery.EstimatedDeliveryAt,
		ExpiresAt:          time.Now().Add(deliveryPoolRetentionWindow),
		Priority:           deliveryPoolPriority(order.DeliveryFee),
	})
	if err != nil {
		return DeliveryPool{}, fmt.Errorf("add takeout order to delivery pool: %w", err)
	}

	return poolItem, nil
}

func deliveryPoolPriority(deliveryFee int64) int32 {
	priority := int32(1)
	if deliveryFee >= 1000 {
		priority = 2
	}
	if deliveryFee >= 2000 {
		priority = 3
	}
	return priority
}
