package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestBindMerchantDoesNotGrantMerchantStaffRoleWhenPending(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchant(user.ID + 100)
	merchant.BindCode = pgtype.Text{String: "1234567890abcdef1234567890abcdef", Valid: true}
	merchant.BindCodeExpiresAt = pgtype.Timestamptz{Time: time.Now().Add(time.Hour), Valid: true}
	createdStaff := db.MerchantStaff{
		ID:         11,
		MerchantID: merchant.ID,
		UserID:     user.ID,
		Role:       "pending",
		Status:     "active",
		InvitedBy:  pgtype.Int8{Int64: merchant.OwnerUserID, Valid: true},
		CreatedAt:  time.Now(),
		UpdatedAt:  pgtype.Timestamptz{Time: time.Now(), Valid: true},
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().
		GetMerchantByBindCode(gomock.Any(), gomock.Eq(pgtype.Text{String: merchant.BindCode.String, Valid: true})).
		Times(1).
		Return(merchant, nil)
	store.EXPECT().
		GetMerchantStaff(gomock.Any(), gomock.Eq(db.GetMerchantStaffParams{MerchantID: merchant.ID, UserID: user.ID})).
		Times(1).
		Return(db.MerchantStaff{}, db.ErrRecordNotFound)
	store.EXPECT().
		AddMerchantStaffTx(gomock.Any(), gomock.Eq(db.AddMerchantStaffTxParams{
			MerchantID: merchant.ID,
			UserID:     user.ID,
			Role:       "pending",
			InvitedBy:  pgtype.Int8{Int64: merchant.OwnerUserID, Valid: true},
		})).
		Times(1).
		Return(db.AddMerchantStaffTxResult{Staff: createdStaff}, nil)
	store.EXPECT().
		CreateUserRole(gomock.Any(), gomock.Any()).
		Times(0)

	server := newTestServer(t, store)
	recorder := httptest.NewRecorder()

	body, err := json.Marshal(map[string]string{"invite_code": merchant.BindCode.String})
	require.NoError(t, err)

	request, err := http.NewRequest(http.MethodPost, "/v1/bind-merchant", bytes.NewReader(body))
	require.NoError(t, err)
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	var resp staffBindResponse
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
	require.Equal(t, "pending", resp.Role)
	require.Equal(t, merchant.ID, resp.MerchantID)
}

func TestBindMerchantReappliesDisabledStaffAsPending(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchant(user.ID + 100)
	merchant.BindCode = pgtype.Text{String: "1234567890abcdef1234567890abcdef", Valid: true}
	merchant.BindCodeExpiresAt = pgtype.Timestamptz{Time: time.Now().Add(time.Hour), Valid: true}
	disabledStaff := db.MerchantStaff{
		ID:         12,
		MerchantID: merchant.ID,
		UserID:     user.ID,
		Role:       db.MerchantStaffRoleCashier,
		Status:     db.MerchantStaffStatusDisabled,
		InvitedBy:  pgtype.Int8{Int64: merchant.OwnerUserID, Valid: true},
		CreatedAt:  time.Now(),
		UpdatedAt:  pgtype.Timestamptz{Time: time.Now(), Valid: true},
	}
	reactivatedStaff := disabledStaff
	reactivatedStaff.Role = db.MerchantStaffRolePending
	reactivatedStaff.Status = db.MerchantStaffStatusActive

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().
		GetMerchantByBindCode(gomock.Any(), gomock.Eq(pgtype.Text{String: merchant.BindCode.String, Valid: true})).
		Times(1).
		Return(merchant, nil)
	store.EXPECT().
		GetMerchantStaff(gomock.Any(), gomock.Eq(db.GetMerchantStaffParams{MerchantID: merchant.ID, UserID: user.ID})).
		Times(1).
		Return(disabledStaff, nil)
	store.EXPECT().
		AddMerchantStaffTx(gomock.Any(), gomock.Eq(db.AddMerchantStaffTxParams{
			MerchantID: merchant.ID,
			UserID:     user.ID,
			Role:       db.MerchantStaffRolePending,
			InvitedBy:  pgtype.Int8{Int64: merchant.OwnerUserID, Valid: true},
		})).
		Times(1).
		Return(db.AddMerchantStaffTxResult{Staff: reactivatedStaff}, nil)
	store.EXPECT().
		CreateUserRole(gomock.Any(), gomock.Any()).
		Times(0)

	server := newTestServer(t, store)
	recorder := httptest.NewRecorder()

	body, err := json.Marshal(map[string]string{"invite_code": merchant.BindCode.String})
	require.NoError(t, err)

	request, err := http.NewRequest(http.MethodPost, "/v1/bind-merchant", bytes.NewReader(body))
	require.NoError(t, err)
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	var resp staffBindResponse
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
	require.Equal(t, db.MerchantStaffRolePending, resp.Role)
	require.Equal(t, merchant.ID, resp.MerchantID)
}

func TestBindMerchantRejectsExistingActiveOrPendingStaff(t *testing.T) {
	tests := []struct {
		name string
		role string
	}{
		{name: "active assigned", role: db.MerchantStaffRoleCashier},
		{name: "active pending", role: db.MerchantStaffRolePending},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			user, _ := randomUser(t)
			merchant := randomMerchant(user.ID + 100)
			merchant.BindCode = pgtype.Text{String: "1234567890abcdef1234567890abcdef", Valid: true}
			merchant.BindCodeExpiresAt = pgtype.Timestamptz{Time: time.Now().Add(time.Hour), Valid: true}
			existingStaff := db.MerchantStaff{
				ID:         13,
				MerchantID: merchant.ID,
				UserID:     user.ID,
				Role:       tc.role,
				Status:     db.MerchantStaffStatusActive,
				InvitedBy:  pgtype.Int8{Int64: merchant.OwnerUserID, Valid: true},
				CreatedAt:  time.Now(),
				UpdatedAt:  pgtype.Timestamptz{Time: time.Now(), Valid: true},
			}

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			store := mockdb.NewMockStore(ctrl)
			store.EXPECT().
				GetMerchantByBindCode(gomock.Any(), gomock.Eq(pgtype.Text{String: merchant.BindCode.String, Valid: true})).
				Times(1).
				Return(merchant, nil)
			store.EXPECT().
				GetMerchantStaff(gomock.Any(), gomock.Eq(db.GetMerchantStaffParams{MerchantID: merchant.ID, UserID: user.ID})).
				Times(1).
				Return(existingStaff, nil)
			store.EXPECT().
				AddMerchantStaffTx(gomock.Any(), gomock.Any()).
				Times(0)

			server := newTestServer(t, store)
			recorder := httptest.NewRecorder()

			body, err := json.Marshal(map[string]string{"invite_code": merchant.BindCode.String})
			require.NoError(t, err)

			request, err := http.NewRequest(http.MethodPost, "/v1/bind-merchant", bytes.NewReader(body))
			require.NoError(t, err)
			addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

			server.router.ServeHTTP(recorder, request)

			require.Equal(t, http.StatusConflict, recorder.Code)
		})
	}
}

func TestIsDuplicateKeyErrorUsesTypedPostgresCode(t *testing.T) {
	require.False(t, isDuplicateKeyError(nil))
	require.False(t, isDuplicateKeyError(db.ErrRecordNotFound))
	require.NotPanics(t, func() {
		require.False(t, isDuplicateKeyError(errors.New("dup")))
	})
	require.True(t, isDuplicateKeyError(&pgconn.PgError{
		Code:    db.UniqueViolation,
		Message: "unexpected driver message",
	}))
	require.False(t, isDuplicateKeyError(&pgconn.PgError{
		Code:    db.ForeignKeyViolation,
		Message: "duplicate key value violates unique constraint",
	}))
}

func TestAddMerchantStaffAPIUsesAtomicStaffRoleTx(t *testing.T) {
	user, _ := randomUser(t)
	owner, _ := randomUser(t)
	merchant := randomMerchant(owner.ID)
	createdStaff := db.MerchantStaff{
		ID:         21,
		MerchantID: merchant.ID,
		UserID:     user.ID,
		Role:       "cashier",
		Status:     "active",
		InvitedBy:  pgtype.Int8{Int64: owner.ID, Valid: true},
		CreatedAt:  time.Now(),
		UpdatedAt:  pgtype.Timestamptz{Time: time.Now(), Valid: true},
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	expectResolveSingleOwnedMerchant(store, owner.ID, merchant)
	store.EXPECT().
		GetMerchantStaff(gomock.Any(), gomock.Eq(db.GetMerchantStaffParams{MerchantID: merchant.ID, UserID: user.ID})).
		Times(1).
		Return(db.MerchantStaff{}, db.ErrRecordNotFound)
	store.EXPECT().
		AddMerchantStaffTx(gomock.Any(), gomock.Eq(db.AddMerchantStaffTxParams{
			MerchantID: merchant.ID,
			UserID:     user.ID,
			Role:       "cashier",
			InvitedBy:  pgtype.Int8{Int64: owner.ID, Valid: true},
		})).
		Times(1).
		Return(db.AddMerchantStaffTxResult{Staff: createdStaff}, nil)
	store.EXPECT().
		CreateMerchantStaff(gomock.Any(), gomock.Any()).
		Times(0)
	store.EXPECT().
		CreateUserRole(gomock.Any(), gomock.Any()).
		Times(0)

	server := newTestServer(t, store)
	recorder := httptest.NewRecorder()

	body, err := json.Marshal(map[string]any{"user_id": user.ID, "role": "cashier"})
	require.NoError(t, err)

	request, err := http.NewRequest(http.MethodPost, "/v1/merchant/staff", bytes.NewReader(body))
	require.NoError(t, err)
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, owner.ID, time.Minute)

	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusCreated, recorder.Code)
	var resp staffResponse
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
	require.Equal(t, createdStaff.ID, resp.ID)
	require.Equal(t, "cashier", resp.Role)
}

func TestAddMerchantStaffAPIReactivatesDisabledStaffWithOwnerConfirmedRole(t *testing.T) {
	user, _ := randomUser(t)
	owner, _ := randomUser(t)
	merchant := randomMerchant(owner.ID)
	disabledStaff := db.MerchantStaff{
		ID:         22,
		MerchantID: merchant.ID,
		UserID:     user.ID,
		Role:       db.MerchantStaffRoleCashier,
		Status:     db.MerchantStaffStatusDisabled,
		InvitedBy:  pgtype.Int8{Int64: owner.ID, Valid: true},
		CreatedAt:  time.Now(),
		UpdatedAt:  pgtype.Timestamptz{Time: time.Now(), Valid: true},
	}
	reactivatedStaff := disabledStaff
	reactivatedStaff.Role = db.MerchantStaffRoleManager
	reactivatedStaff.Status = db.MerchantStaffStatusActive

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	expectResolveSingleOwnedMerchant(store, owner.ID, merchant)
	store.EXPECT().
		GetMerchantStaff(gomock.Any(), gomock.Eq(db.GetMerchantStaffParams{MerchantID: merchant.ID, UserID: user.ID})).
		Times(1).
		Return(disabledStaff, nil)
	store.EXPECT().
		AddMerchantStaffTx(gomock.Any(), gomock.Eq(db.AddMerchantStaffTxParams{
			MerchantID: merchant.ID,
			UserID:     user.ID,
			Role:       db.MerchantStaffRoleManager,
			InvitedBy:  pgtype.Int8{Int64: owner.ID, Valid: true},
		})).
		Times(1).
		Return(db.AddMerchantStaffTxResult{Staff: reactivatedStaff}, nil)
	store.EXPECT().
		CreateMerchantStaff(gomock.Any(), gomock.Any()).
		Times(0)
	store.EXPECT().
		CreateUserRole(gomock.Any(), gomock.Any()).
		Times(0)

	server := newTestServer(t, store)
	recorder := httptest.NewRecorder()

	body, err := json.Marshal(map[string]any{"user_id": user.ID, "role": db.MerchantStaffRoleManager})
	require.NoError(t, err)

	request, err := http.NewRequest(http.MethodPost, "/v1/merchant/staff", bytes.NewReader(body))
	require.NoError(t, err)
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, owner.ID, time.Minute)

	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusCreated, recorder.Code)
	var resp staffResponse
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
	require.Equal(t, disabledStaff.ID, resp.ID)
	require.Equal(t, db.MerchantStaffRoleManager, resp.Role)
	require.Equal(t, db.MerchantStaffStatusActive, resp.Status)
}

func TestAddMerchantStaffAPIRejectsExistingActiveOrPendingStaff(t *testing.T) {
	tests := []struct {
		name string
		role string
	}{
		{name: "active assigned", role: db.MerchantStaffRoleCashier},
		{name: "active pending", role: db.MerchantStaffRolePending},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			user, _ := randomUser(t)
			owner, _ := randomUser(t)
			merchant := randomMerchant(owner.ID)
			existingStaff := db.MerchantStaff{
				ID:         23,
				MerchantID: merchant.ID,
				UserID:     user.ID,
				Role:       tc.role,
				Status:     db.MerchantStaffStatusActive,
				InvitedBy:  pgtype.Int8{Int64: owner.ID, Valid: true},
				CreatedAt:  time.Now(),
				UpdatedAt:  pgtype.Timestamptz{Time: time.Now(), Valid: true},
			}

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			store := mockdb.NewMockStore(ctrl)
			expectResolveSingleOwnedMerchant(store, owner.ID, merchant)
			store.EXPECT().
				GetMerchantStaff(gomock.Any(), gomock.Eq(db.GetMerchantStaffParams{MerchantID: merchant.ID, UserID: user.ID})).
				Times(1).
				Return(existingStaff, nil)
			store.EXPECT().
				AddMerchantStaffTx(gomock.Any(), gomock.Any()).
				Times(0)

			server := newTestServer(t, store)
			recorder := httptest.NewRecorder()

			body, err := json.Marshal(map[string]any{"user_id": user.ID, "role": db.MerchantStaffRoleManager})
			require.NoError(t, err)

			request, err := http.NewRequest(http.MethodPost, "/v1/merchant/staff", bytes.NewReader(body))
			require.NoError(t, err)
			addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, owner.ID, time.Minute)

			server.router.ServeHTTP(recorder, request)

			require.Equal(t, http.StatusConflict, recorder.Code)
		})
	}
}

func TestUpdateMerchantStaffRoleAPIUsesAtomicStaffRoleTx(t *testing.T) {
	owner, _ := randomUser(t)
	staffUser, _ := randomUser(t)
	merchant := randomMerchant(owner.ID)
	staff := db.MerchantStaff{
		ID:         31,
		MerchantID: merchant.ID,
		UserID:     staffUser.ID,
		Role:       "manager",
		Status:     "active",
		CreatedAt:  time.Now(),
		UpdatedAt:  pgtype.Timestamptz{Time: time.Now(), Valid: true},
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	expectResolveSingleOwnedMerchant(store, owner.ID, merchant)
	store.EXPECT().
		AssignMerchantStaffRoleTx(gomock.Any(), gomock.Eq(db.AssignMerchantStaffRoleTxParams{
			MerchantID: merchant.ID,
			StaffID:    staff.ID,
			Role:       "chef",
		})).
		Times(1).
		Return(db.AssignMerchantStaffRoleTxResult{Staff: db.MerchantStaff{
			ID:         staff.ID,
			MerchantID: merchant.ID,
			UserID:     staffUser.ID,
			Role:       "chef",
			Status:     "active",
			CreatedAt:  staff.CreatedAt,
			UpdatedAt:  pgtype.Timestamptz{Time: time.Now(), Valid: true},
		}}, nil)
	store.EXPECT().
		UpdateMerchantStaffRole(gomock.Any(), gomock.Any()).
		Times(0)
	store.EXPECT().
		CreateUserRole(gomock.Any(), gomock.Any()).
		Times(0)

	server := newTestServer(t, store)
	recorder := httptest.NewRecorder()

	body, err := json.Marshal(map[string]string{"role": "chef"})
	require.NoError(t, err)
	request, err := http.NewRequest(http.MethodPatch, "/v1/merchant/staff/31/role", bytes.NewReader(body))
	require.NoError(t, err)
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, owner.ID, time.Minute)

	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	var resp staffResponse
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
	require.Equal(t, "chef", resp.Role)
}

func TestDeleteMerchantStaffAPIUsesAtomicStaffRoleTx(t *testing.T) {
	owner, _ := randomUser(t)
	merchant := randomMerchant(owner.ID)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	expectResolveSingleOwnedMerchant(store, owner.ID, merchant)
	store.EXPECT().
		RemoveMerchantStaffTx(gomock.Any(), gomock.Eq(db.RemoveMerchantStaffTxParams{
			MerchantID: merchant.ID,
			StaffID:    41,
		})).
		Times(1).
		Return(db.RemoveMerchantStaffTxResult{Staff: db.MerchantStaff{
			ID:         41,
			MerchantID: merchant.ID,
			UserID:     owner.ID + 100,
			Role:       "cashier",
			Status:     "disabled",
		}}, nil)
	store.EXPECT().
		SoftDeleteMerchantStaff(gomock.Any(), gomock.Any()).
		Times(0)
	store.EXPECT().
		UpdateUserRoleStatus(gomock.Any(), gomock.Any()).
		Times(0)

	server := newTestServer(t, store)
	recorder := httptest.NewRecorder()

	request, err := http.NewRequest(http.MethodDelete, "/v1/merchant/staff/41", nil)
	require.NoError(t, err)
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, owner.ID, time.Minute)

	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
}
