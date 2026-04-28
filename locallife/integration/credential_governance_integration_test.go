package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/logic"
	"github.com/merrydance/locallife/scheduler"
	"github.com/merrydance/locallife/util"
	"github.com/merrydance/locallife/worker"
	"github.com/stretchr/testify/require"
)

func TestMerchantCredentialRestoreIntegration_DoesNotReleaseManualComplianceHold(t *testing.T) {
	_, store := initIntegrationServer(t)
	resetIntegrationData(t)

	ctx := context.Background()
	region := createIntegrationRegion(t, store)
	owner := createIntegrationUser(t, store)
	merchant := createIntegrationMerchant(t, store, owner.ID, region.ID)
	application, err := store.CreateMerchantApplicationDraft(ctx, owner.ID)
	require.NoError(t, err)
	_, err = store.CreateMerchantProfile(ctx, merchant.ID)
	require.NoError(t, err)

	err = store.SuspendMerchant(ctx, db.SuspendMerchantParams{
		MerchantID:    merchant.ID,
		SuspendReason: pgtype.Text{String: "manual compliance hold", Valid: true},
		SuspendUntil:  pgtype.Timestamptz{Time: time.Now().UTC().Add(24 * time.Hour), Valid: true},
	})
	require.NoError(t, err)
	err = store.SuspendMerchantTakeout(ctx, db.SuspendMerchantTakeoutParams{
		MerchantID:           merchant.ID,
		TakeoutSuspendReason: pgtype.Text{String: "manual compliance hold", Valid: true},
		TakeoutSuspendUntil:  pgtype.Timestamptz{Time: time.Now().UTC().Add(24 * time.Hour), Valid: true},
	})
	require.NoError(t, err)
	_, err = integrationPool.Exec(ctx, `UPDATE merchants SET status = 'suspended' WHERE id = $1`, merchant.ID)
	require.NoError(t, err)

	businessLicenseAsset := createIntegrationMediaAsset(t, store, owner.ID, "business_license")
	foodPermitAsset := createIntegrationMediaAsset(t, store, owner.ID, "food_permit")
	businessLicenseExpiresAt := time.Now().UTC().AddDate(1, 0, 0)
	foodPermitExpiresAt := time.Now().UTC().AddDate(0, 6, 0)

	governanceService := logic.NewCredentialGovernanceService(store)
	ledgers, err := governanceService.ActivateMerchantCredentials(ctx, logic.ActivateMerchantCredentialsInput{
		MerchantID:            merchant.ID,
		MerchantApplicationID: application.ID,
		Entries: []logic.CredentialActivationInput{
			{
				DocumentType: db.CredentialDocumentTypeBusinessLicense,
				MediaAssetID: businessLicenseAsset.ID,
				ExpiresAt:    &businessLicenseExpiresAt,
				NormalizedPayload: map[string]any{
					"credit_code": "integration-business-license",
				},
			},
			{
				DocumentType: db.CredentialDocumentTypeFoodPermit,
				MediaAssetID: foodPermitAsset.ID,
				ExpiresAt:    &foodPermitExpiresAt,
				NormalizedPayload: map[string]any{
					"permit_no": "integration-food-permit",
				},
			},
		},
	})
	require.NoError(t, err)
	require.Len(t, ledgers, 2)

	restoreResult, err := governanceService.RestoreMerchantIfEligible(ctx, merchant.ID)
	require.NoError(t, err)
	require.True(t, restoreResult.MatrixSatisfied)
	require.False(t, restoreResult.Released)

	merchantAfterRestore, err := store.GetMerchant(ctx, merchant.ID)
	require.NoError(t, err)
	require.Equal(t, "suspended", merchantAfterRestore.Status)

	profileAfterRestore, err := store.GetMerchantProfile(ctx, merchant.ID)
	require.NoError(t, err)
	require.True(t, profileAfterRestore.IsSuspended)
	require.True(t, profileAfterRestore.IsTakeoutSuspended)
	require.Equal(t, "manual compliance hold", profileAfterRestore.SuspendReason.String)
	require.Equal(t, "manual compliance hold", profileAfterRestore.TakeoutSuspendReason.String)

	activeLedgers, err := store.GetActiveMerchantCredentialLedgers(ctx, pgtype.Int8{Int64: merchant.ID, Valid: true})
	require.NoError(t, err)
	require.Len(t, activeLedgers, 2)
	for _, ledger := range activeLedgers {
		require.False(t, ledger.ResumedAt.Valid)
	}
}

func TestMerchantCredentialRestoreIntegration_PartialMerchantRenewalKeepsTakeoutBlocked(t *testing.T) {
	server, store := initIntegrationServer(t)
	resetIntegrationData(t)

	ctx := context.Background()
	region := createIntegrationRegion(t, store)
	owner := createIntegrationUser(t, store)
	merchant := createIntegrationMerchant(t, store, owner.ID, region.ID)
	_, err := store.UpdateMerchantStatus(ctx, db.UpdateMerchantStatusParams{ID: merchant.ID, Status: "active"})
	require.NoError(t, err)
	_, err = store.UpdateMerchantIsOpen(ctx, db.UpdateMerchantIsOpenParams{ID: merchant.ID, IsOpen: true, AutoCloseAt: pgtype.Timestamptz{Valid: false}})
	require.NoError(t, err)
	merchant = ensureIntegrationMerchantCoords(t, store, merchant.ID)

	application, err := store.CreateMerchantApplicationDraft(ctx, owner.ID)
	require.NoError(t, err)
	_, err = store.CreateMerchantProfile(ctx, merchant.ID)
	require.NoError(t, err)

	foodPermitAsset := createIntegrationMediaAsset(t, store, owner.ID, "food_permit")
	businessLicenseAsset := createIntegrationMediaAsset(t, store, owner.ID, "business_license")
	_, err = store.CreateMerchantCredentialLedger(ctx, db.CreateMerchantCredentialLedgerParams{
		MerchantID:            pgtype.Int8{Int64: merchant.ID, Valid: true},
		DocumentType:          db.CredentialDocumentTypeFoodPermit,
		MerchantApplicationID: pgtype.Int8{Int64: application.ID, Valid: true},
		MediaAssetID:          foodPermitAsset.ID,
		NormalizedPayload:     []byte(`{"permit_no":"expired-food-permit"}`),
		ExpiresAt:             pgtype.Timestamptz{Time: time.Now().UTC().Add(-24 * time.Hour), Valid: true},
	})
	require.NoError(t, err)

	err = store.SuspendMerchantTakeout(ctx, db.SuspendMerchantTakeoutParams{
		MerchantID:           merchant.ID,
		TakeoutSuspendReason: pgtype.Text{String: db.CredentialSuspensionReasonDocumentExpired, Valid: true},
		TakeoutSuspendUntil:  pgtype.Timestamptz{Time: time.Now().UTC().Add(24 * time.Hour), Valid: true},
	})
	require.NoError(t, err)

	governanceService := logic.NewCredentialGovernanceService(store)
	_, err = governanceService.ActivateMerchantCredentials(ctx, logic.ActivateMerchantCredentialsInput{
		MerchantID:            merchant.ID,
		MerchantApplicationID: application.ID,
		Entries: []logic.CredentialActivationInput{
			{
				DocumentType: db.CredentialDocumentTypeBusinessLicense,
				MediaAssetID: businessLicenseAsset.ID,
				ExpiresAt:    timePointer(time.Now().UTC().AddDate(1, 0, 0)),
				NormalizedPayload: map[string]any{
					"credit_code": "renewed-business-license",
				},
			},
		},
	})
	require.NoError(t, err)

	restoreResult, err := governanceService.RestoreMerchantIfEligible(ctx, merchant.ID)
	require.NoError(t, err)
	require.False(t, restoreResult.MatrixSatisfied)
	require.False(t, restoreResult.Released)
	require.Contains(t, restoreResult.ExpiredDocumentTypes, db.CredentialDocumentTypeFoodPermit)

	merchantProfile, err := store.GetMerchantProfile(ctx, merchant.ID)
	require.NoError(t, err)
	require.True(t, merchantProfile.IsTakeoutSuspended)
	require.True(t, merchantProfile.TakeoutSuspendReason.Valid)
	require.Equal(t, db.CredentialSuspensionReasonDocumentExpired, merchantProfile.TakeoutSuspendReason.String)

	dish := createIntegrationDish(t, store, merchant.ID)
	customer := createIntegrationUser(t, store)
	address := createIntegrationUserAddress(t, store, customer.ID, region.ID)
	createBody := map[string]any{
		"merchant_id":           merchant.ID,
		"order_type":            "takeout",
		"address_id":            address.ID,
		"items":                 []map[string]any{{"dish_id": dish.ID, "quantity": 1}},
		"use_balance":           false,
		"delivery_fee":          int64(500),
		"delivery_distance":     int32(1000),
		"delivery_fee_discount": int64(0),
	}
	rec := doJSON(t, server, http.MethodPost, "/v1/orders", createBody, customer.ID)
	require.Equal(t, http.StatusForbidden, rec.Code)
	require.Contains(t, rec.Body.String(), "merchant takeout ordering is suspended")
}

func TestRiderCredentialRestoreIntegration_RestoresGrabAvailability(t *testing.T) {
	server, store := initIntegrationServer(t)
	resetIntegrationData(t)

	ctx := context.Background()
	region := createIntegrationRegion(t, store)
	merchantOwner := createIntegrationUser(t, store)
	merchant := createIntegrationMerchant(t, store, merchantOwner.ID, region.ID)
	_, err := store.UpdateMerchantStatus(ctx, db.UpdateMerchantStatusParams{ID: merchant.ID, Status: "active"})
	require.NoError(t, err)
	_, err = store.UpdateMerchantIsOpen(ctx, db.UpdateMerchantIsOpenParams{ID: merchant.ID, IsOpen: true, AutoCloseAt: pgtype.Timestamptz{Valid: false}})
	require.NoError(t, err)
	merchant = ensureIntegrationMerchantCoords(t, store, merchant.ID)

	customer := createIntegrationUser(t, store)
	address := createIntegrationUserAddress(t, store, customer.ID, region.ID)
	order, delivery := createReadyTakeoutDeliveryCandidate(t, store, customer.ID, merchant, address)

	riderUser := createIntegrationUser(t, store)
	rider := createIntegrationRider(t, store, riderUser.ID, region.ID)
	_, err = store.CreateRiderProfile(ctx, rider.ID)
	require.NoError(t, err)
	_, err = store.UpdateRiderStatus(ctx, db.UpdateRiderStatusParams{ID: rider.ID, Status: "active"})
	require.NoError(t, err)
	_, err = store.UpdateRiderDeposit(ctx, db.UpdateRiderDepositParams{ID: rider.ID, DepositAmount: 100_000, FrozenDeposit: 0})
	require.NoError(t, err)
	_, err = store.UpdateRiderOnlineStatus(ctx, db.UpdateRiderOnlineStatusParams{ID: rider.ID, IsOnline: true})
	require.NoError(t, err)

	riderApplication, err := store.CreateRiderApplication(ctx, riderUser.ID)
	require.NoError(t, err)
	expiredHealthCertAsset := createIntegrationMediaAsset(t, store, riderUser.ID, "health_cert")
	_, err = store.CreateRiderCredentialLedger(ctx, db.CreateRiderCredentialLedgerParams{
		RiderID:            pgtype.Int8{Int64: rider.ID, Valid: true},
		DocumentType:       db.CredentialDocumentTypeHealthCert,
		RiderApplicationID: pgtype.Int8{Int64: riderApplication.ID, Valid: true},
		MediaAssetID:       expiredHealthCertAsset.ID,
		NormalizedPayload:  []byte(`{"holder_name":"expired-health-cert"}`),
		ExpiresAt:          pgtype.Timestamptz{Time: time.Now().UTC().Add(-24 * time.Hour), Valid: true},
	})
	require.NoError(t, err)
	err = store.SuspendRider(ctx, db.SuspendRiderParams{
		RiderID:       rider.ID,
		SuspendReason: pgtype.Text{String: db.CredentialSuspensionReasonDocumentExpired, Valid: true},
		SuspendUntil:  pgtype.Timestamptz{Time: time.Now().UTC().Add(24 * time.Hour), Valid: true},
	})
	require.NoError(t, err)

	governanceService := logic.NewCredentialGovernanceService(store)
	newHealthCertAsset := createIntegrationMediaAsset(t, store, riderUser.ID, "health_cert")
	_, err = governanceService.ActivateRiderCredentials(ctx, logic.ActivateRiderCredentialsInput{
		RiderID:            rider.ID,
		RiderApplicationID: riderApplication.ID,
		Entries: []logic.CredentialActivationInput{
			{
				DocumentType: db.CredentialDocumentTypeHealthCert,
				MediaAssetID: newHealthCertAsset.ID,
				ExpiresAt:    timePointer(time.Now().UTC().AddDate(0, 6, 0)),
				NormalizedPayload: map[string]any{
					"holder_name": "renewed-health-cert",
				},
			},
		},
	})
	require.NoError(t, err)

	restoreResult, err := governanceService.RestoreRiderIfEligible(ctx, rider.ID)
	require.NoError(t, err)
	require.True(t, restoreResult.MatrixSatisfied)
	require.True(t, restoreResult.Released)

	riderProfile, err := store.GetRiderProfile(ctx, rider.ID)
	require.NoError(t, err)
	require.False(t, riderProfile.IsSuspended)
	require.False(t, riderProfile.SuspendReason.Valid)

	grabURL := fmt.Sprintf("/v1/delivery/grab/%d", order.ID)
	rec := doJSON(t, server, http.MethodPost, grabURL, nil, riderUser.ID)
	require.Equal(t, http.StatusOK, rec.Code)

	updatedOrder, err := store.GetOrder(ctx, order.ID)
	require.NoError(t, err)
	require.Equal(t, db.OrderStatusCourierAccepted, updatedOrder.Status)

	updatedDelivery, err := store.GetDeliveryByOrderID(ctx, order.ID)
	require.NoError(t, err)
	require.Equal(t, delivery.ID, updatedDelivery.ID)
	require.True(t, updatedDelivery.RiderID.Valid)
	require.Equal(t, rider.ID, updatedDelivery.RiderID.Int64)
}

func TestRiderCredentialGovernanceIntegration_ClosedLoop(t *testing.T) {
	server, store := initIntegrationServer(t)
	resetIntegrationData(t)

	ctx := context.Background()
	baseNow := time.Now().UTC().Truncate(time.Second)
	initialHealthCertExpiry := baseNow.AddDate(0, 0, 8)
	reminderNow := baseNow
	renewedHealthCertExpiry := baseNow.AddDate(0, 6, 0)

	region := createIntegrationRegion(t, store)
	merchantOwner := createIntegrationUser(t, store)
	merchant := createIntegrationMerchant(t, store, merchantOwner.ID, region.ID)
	_, err := store.UpdateMerchantStatus(ctx, db.UpdateMerchantStatusParams{ID: merchant.ID, Status: "active"})
	require.NoError(t, err)
	_, err = store.UpdateMerchantIsOpen(ctx, db.UpdateMerchantIsOpenParams{ID: merchant.ID, IsOpen: true, AutoCloseAt: pgtype.Timestamptz{Valid: false}})
	require.NoError(t, err)
	merchant = ensureIntegrationMerchantCoords(t, store, merchant.ID)

	customer := createIntegrationUser(t, store)
	address := createIntegrationUserAddress(t, store, customer.ID, region.ID)
	riderUser := createIntegrationUser(t, store)
	initialApplication := createSubmittedRiderApplication(t, store, riderUser, initialHealthCertExpiry)

	governanceService := logic.NewCredentialGovernanceService(store)
	reviewService := logic.NewRiderOnboardingReviewService(store, logic.NewOnboardingReviewService(store), governanceService)
	reviewResult, err := reviewService.ProcessSubmittedApplication(ctx, initialApplication, riderUser.ID, nil)
	require.NoError(t, err)
	require.True(t, reviewResult.Approved)
	require.NotNil(t, reviewResult.Rider)
	require.NotNil(t, reviewResult.ReviewRun)
	require.NotEmpty(t, reviewResult.Application.ReviewSummary)

	rider := *reviewResult.Rider
	_, err = store.CreateRiderProfile(ctx, rider.ID)
	require.NoError(t, err)
	_, err = store.UpdateRiderStatus(ctx, db.UpdateRiderStatusParams{ID: rider.ID, Status: "active"})
	require.NoError(t, err)
	_, err = store.UpdateRiderDeposit(ctx, db.UpdateRiderDepositParams{ID: rider.ID, DepositAmount: 100_000, FrozenDeposit: 0})
	require.NoError(t, err)
	_, err = store.UpdateRiderOnlineStatus(ctx, db.UpdateRiderOnlineStatusParams{ID: rider.ID, IsOnline: true})
	require.NoError(t, err)

	activeLedgers, err := store.GetActiveRiderCredentialLedgers(ctx, pgtype.Int8{Int64: rider.ID, Valid: true})
	require.NoError(t, err)
	require.Len(t, activeLedgers, 1)
	initialLedger := activeLedgers[0]
	require.Equal(t, db.CredentialDocumentTypeHealthCert, initialLedger.DocumentType)
	require.True(t, initialLedger.ExpiresAt.Valid)
	require.Equal(t, initialHealthCertExpiry.Format("2006-01-02"), initialLedger.ExpiresAt.Time.Format("2006-01-02"))

	cleanupScheduler := scheduler.NewDataCleanupScheduler(store, worker.NoopTaskDistributor{}, nil)
	cleanupScheduler.RemindExpiringCredentialsAt(reminderNow)

	activeLedgers, err = store.GetActiveRiderCredentialLedgers(ctx, pgtype.Int8{Int64: rider.ID, Valid: true})
	require.NoError(t, err)
	require.Len(t, activeLedgers, 1)
	require.True(t, activeLedgers[0].LastRemindedAt.Valid)

	notifications, err := store.GetNotificationsByRelated(ctx, db.GetNotificationsByRelatedParams{
		RelatedType: pgtype.Text{String: "rider", Valid: true},
		RelatedID:   pgtype.Int8{Int64: rider.ID, Valid: true},
	})
	require.NoError(t, err)
	require.Len(t, notifications, 1)
	require.Contains(t, notifications[0].Title, "7 天后到期")

	cleanupScheduler.EnforceExpiredCredentialsAt(initialHealthCertExpiry.Add(time.Minute))

	riderProfile, err := store.GetRiderProfile(ctx, rider.ID)
	require.NoError(t, err)
	require.True(t, riderProfile.IsSuspended)
	require.True(t, riderProfile.SuspendReason.Valid)
	require.Equal(t, db.CredentialSuspensionReasonDocumentExpired, riderProfile.SuspendReason.String)

	initialLedgerAfterSuspension, err := store.GetCredentialLedger(ctx, initialLedger.ID)
	require.NoError(t, err)
	require.True(t, initialLedgerAfterSuspension.SuspendedAt.Valid)
	require.True(t, initialLedgerAfterSuspension.SuspensionReasonCode.Valid)
	require.Equal(t, db.CredentialSuspensionReasonDocumentExpired, initialLedgerAfterSuspension.SuspensionReasonCode.String)

	notifications, err = store.GetNotificationsByRelated(ctx, db.GetNotificationsByRelatedParams{
		RelatedType: pgtype.Text{String: "rider", Valid: true},
		RelatedID:   pgtype.Int8{Int64: rider.ID, Valid: true},
	})
	require.NoError(t, err)
	require.Len(t, notifications, 2)
	require.True(t, containsNotificationTitle(notifications, "接单资格已暂停"))

	suspendedOrder, _ := createReadyTakeoutDeliveryCandidate(t, store, customer.ID, merchant, address)
	suspendedGrabRec := doJSON(t, server, http.MethodPost, fmt.Sprintf("/v1/delivery/grab/%d", suspendedOrder.ID), nil, riderUser.ID)
	require.Equal(t, http.StatusForbidden, suspendedGrabRec.Code)
	require.Contains(t, suspendedGrabRec.Body.String(), "骑手接单已暂停")

	renewedHealthCertAsset := createIntegrationMediaAsset(t, store, riderUser.ID, "health_cert")
	newLedgers, err := governanceService.ActivateRiderCredentials(ctx, logic.ActivateRiderCredentialsInput{
		RiderID:            rider.ID,
		RiderApplicationID: reviewResult.Application.ID,
		ReviewRunID:        &reviewResult.ReviewRun.ID,
		Entries: []logic.CredentialActivationInput{
			{
				DocumentType: db.CredentialDocumentTypeHealthCert,
				MediaAssetID: renewedHealthCertAsset.ID,
				ExpiresAt:    timePointer(renewedHealthCertExpiry),
				NormalizedPayload: map[string]any{
					"name":        "测试骑手",
					"id_number":   "110101199001011234",
					"cert_number": "HC-RENEWED-001",
					"valid_start": baseNow.Format("2006-01-02"),
					"valid_end":   renewedHealthCertExpiry.Format("2006-01-02"),
				},
			},
		},
	})
	require.NoError(t, err)
	require.Len(t, newLedgers, 1)
	require.NotEqual(t, initialLedger.ID, newLedgers[0].ID)

	initialLedgerAfterRenewal, err := store.GetCredentialLedger(ctx, initialLedger.ID)
	require.NoError(t, err)
	require.True(t, initialLedgerAfterRenewal.DeactivatedAt.Valid)

	activeLedgers, err = store.GetActiveRiderCredentialLedgers(ctx, pgtype.Int8{Int64: rider.ID, Valid: true})
	require.NoError(t, err)
	require.Len(t, activeLedgers, 1)
	require.Equal(t, newLedgers[0].ID, activeLedgers[0].ID)
	require.True(t, activeLedgers[0].ExpiresAt.Valid)
	require.Equal(t, renewedHealthCertExpiry.Format("2006-01-02"), activeLedgers[0].ExpiresAt.Time.Format("2006-01-02"))

	restoreResult, err := governanceService.RestoreRiderIfEligible(ctx, rider.ID)
	require.NoError(t, err)
	require.True(t, restoreResult.MatrixSatisfied)
	require.True(t, restoreResult.Released)

	riderProfile, err = store.GetRiderProfile(ctx, rider.ID)
	require.NoError(t, err)
	require.False(t, riderProfile.IsSuspended)
	require.False(t, riderProfile.SuspendReason.Valid)

	restoredOrder, restoredDelivery := createReadyTakeoutDeliveryCandidate(t, store, customer.ID, merchant, address)
	restoredGrabRec := doJSON(t, server, http.MethodPost, fmt.Sprintf("/v1/delivery/grab/%d", restoredOrder.ID), nil, riderUser.ID)
	require.Equal(t, http.StatusOK, restoredGrabRec.Code)

	updatedDelivery, err := store.GetDeliveryByOrderID(ctx, restoredOrder.ID)
	require.NoError(t, err)
	require.Equal(t, restoredDelivery.ID, updatedDelivery.ID)
	require.True(t, updatedDelivery.RiderID.Valid)
	require.Equal(t, rider.ID, updatedDelivery.RiderID.Int64)

	updatedOrder, err := store.GetOrder(ctx, restoredOrder.ID)
	require.NoError(t, err)
	require.Equal(t, db.OrderStatusCourierAccepted, updatedOrder.Status)
}

func createIntegrationMediaAsset(t *testing.T, store *db.SQLStore, uploadedBy int64, mediaCategory string) db.MediaAsset {
	t.Helper()

	asset, err := store.CreateMediaAsset(context.Background(), db.CreateMediaAssetParams{
		ObjectKey:      fmt.Sprintf("integration/%s/%s.jpg", mediaCategory, util.RandomString(10)),
		Visibility:     "private",
		MediaCategory:  mediaCategory,
		MimeType:       "image/jpeg",
		FileSize:       1024,
		ChecksumSha256: util.RandomString(32),
		UploadedBy:     uploadedBy,
		SourceClient:   "integration_test",
	})
	require.NoError(t, err)
	return asset
}

func createReadyTakeoutDeliveryCandidate(t *testing.T, store *db.SQLStore, customerID int64, merchant db.Merchant, address db.UserAddress) (db.Order, db.Delivery) {
	t.Helper()

	ctx := context.Background()
	order, err := store.CreateOrder(ctx, db.CreateOrderParams{
		OrderNo:                      "order_" + util.RandomString(12),
		UserID:                       customerID,
		MerchantID:                   merchant.ID,
		OrderType:                    "takeout",
		AddressID:                    pgtype.Int8{Int64: address.ID, Valid: true},
		DeliveryContactNameSnapshot:  pgtype.Text{String: address.ContactName, Valid: true},
		DeliveryContactPhoneSnapshot: pgtype.Text{String: address.ContactPhone, Valid: true},
		DeliveryAddressSnapshot:      pgtype.Text{String: address.DetailAddress, Valid: true},
		DeliveryLongitudeSnapshot:    address.Longitude,
		DeliveryLatitudeSnapshot:     address.Latitude,
		DeliveryFee:                  500,
		DeliveryDistance:             pgtype.Int4{Int32: 1000, Valid: true},
		DeliveryDuration:             pgtype.Int4{Int32: 30, Valid: true},
		TableID:                      pgtype.Int8{Valid: false},
		ReservationID:                pgtype.Int8{Valid: false},
		Subtotal:                     2000,
		DiscountAmount:               0,
		DeliveryFeeDiscount:          0,
		TotalAmount:                  2500,
		Status:                       db.OrderStatusReady,
		FulfillmentStatus:            "ready",
		Notes:                        pgtype.Text{Valid: false},
		UserVoucherID:                pgtype.Int8{Valid: false},
		VoucherAmount:                0,
		BalancePaid:                  0,
		MembershipID:                 pgtype.Int8{Valid: false},
		ReplacedByOrderID:            pgtype.Int8{Valid: false},
		PickupCode:                   pgtype.Text{Valid: false},
	})
	require.NoError(t, err)

	estimatedPickupAt := time.Now().UTC().Add(10 * time.Minute)
	estimatedDeliveryAt := estimatedPickupAt.Add(30 * time.Minute)
	delivery, err := store.CreateDelivery(ctx, db.CreateDeliveryParams{
		OrderID:             order.ID,
		PickupAddress:       merchant.Address,
		PickupLongitude:     merchant.Longitude,
		PickupLatitude:      merchant.Latitude,
		PickupContact:       pgtype.Text{String: merchant.Name, Valid: true},
		PickupPhone:         pgtype.Text{String: merchant.Phone, Valid: true},
		DeliveryAddress:     address.DetailAddress,
		DeliveryLongitude:   address.Longitude,
		DeliveryLatitude:    address.Latitude,
		DeliveryContact:     pgtype.Text{String: address.ContactName, Valid: true},
		DeliveryPhone:       pgtype.Text{String: address.ContactPhone, Valid: true},
		Distance:            1000,
		DeliveryFee:         500,
		EstimatedPickupAt:   pgtype.Timestamptz{Time: estimatedPickupAt, Valid: true},
		EstimatedDeliveryAt: pgtype.Timestamptz{Time: estimatedDeliveryAt, Valid: true},
	})
	require.NoError(t, err)

	_, err = store.AddToDeliveryPool(ctx, db.AddToDeliveryPoolParams{
		OrderID:            order.ID,
		MerchantID:         merchant.ID,
		PickupLongitude:    merchant.Longitude,
		PickupLatitude:     merchant.Latitude,
		DeliveryLongitude:  address.Longitude,
		DeliveryLatitude:   address.Latitude,
		Distance:           1000,
		DeliveryFee:        500,
		ExpectedPickupAt:   estimatedPickupAt,
		ExpectedDeliveryAt: pgtype.Timestamptz{Time: estimatedDeliveryAt, Valid: true},
		ExpiresAt:          time.Now().UTC().Add(time.Hour),
		Priority:           1,
	})
	require.NoError(t, err)

	return order, delivery
}

func createSubmittedRiderApplication(t *testing.T, store *db.SQLStore, user db.User, healthCertExpiry time.Time) db.RiderApplication {
	t.Helper()

	ctx := context.Background()
	application, err := store.CreateRiderApplication(ctx, user.ID)
	require.NoError(t, err)

	application, err = store.UpdateRiderApplicationBasicInfo(ctx, db.UpdateRiderApplicationBasicInfoParams{
		ID:       application.ID,
		RealName: pgtype.Text{String: "测试骑手", Valid: true},
		Phone:    user.Phone,
	})
	require.NoError(t, err)

	idCardFrontAsset := createIntegrationMediaAsset(t, store, user.ID, "id_card_front")
	idCardBackAsset := createIntegrationMediaAsset(t, store, user.ID, "id_card_back")
	idCardOCR, err := json.Marshal(map[string]any{
		"name":       "测试骑手",
		"id_number":  "110101199001011234",
		"valid_end":  healthCertExpiry.AddDate(10, 0, 0).Format("2006-01-02"),
		"ocr_job_id": util.RandomInt(1000, 9999),
	})
	require.NoError(t, err)
	application, err = store.UpdateRiderApplicationIDCard(ctx, db.UpdateRiderApplicationIDCardParams{
		ID:                      application.ID,
		IDCardFrontMediaAssetID: pgtype.Int8{Int64: idCardFrontAsset.ID, Valid: true},
		IDCardBackMediaAssetID:  pgtype.Int8{Int64: idCardBackAsset.ID, Valid: true},
		IDCardOcr:               idCardOCR,
		RealName:                pgtype.Text{String: "测试骑手", Valid: true},
	})
	require.NoError(t, err)

	healthCertAsset := createIntegrationMediaAsset(t, store, user.ID, "health_cert")
	healthCertOCR, err := json.Marshal(map[string]any{
		"name":        "测试骑手",
		"id_number":   "110101199001011234",
		"cert_number": "HC-INITIAL-001",
		"valid_start": time.Now().UTC().AddDate(-1, 0, 0).Format("2006-01-02"),
		"valid_end":   healthCertExpiry.Format("2006-01-02"),
		"ocr_job_id":  util.RandomInt(1000, 9999),
	})
	require.NoError(t, err)
	application, err = store.UpdateRiderApplicationHealthCert(ctx, db.UpdateRiderApplicationHealthCertParams{
		ID:                     application.ID,
		HealthCertMediaAssetID: pgtype.Int8{Int64: healthCertAsset.ID, Valid: true},
		HealthCertOcr:          healthCertOCR,
	})
	require.NoError(t, err)

	application, err = store.SubmitRiderApplication(ctx, application.ID)
	require.NoError(t, err)
	return application
}

func containsNotificationTitle(notifications []db.Notification, fragment string) bool {
	for _, notification := range notifications {
		if strings.Contains(notification.Title, fragment) {
			return true
		}
	}
	return false
}

func timePointer(value time.Time) *time.Time {
	return &value
}
