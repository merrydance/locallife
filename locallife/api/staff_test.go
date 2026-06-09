package api

import (
	"bytes"
	"context"
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
		CreateMerchantStaff(gomock.Any(), gomock.Eq(db.CreateMerchantStaffParams{
			MerchantID: merchant.ID,
			UserID:     user.ID,
			Role:       "pending",
			Status:     "active",
			InvitedBy:  pgtype.Int8{Int64: merchant.OwnerUserID, Valid: true},
		})).
		Times(1).
		Return(createdStaff, nil)
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

func TestEnsureMerchantStaffUserRoleActive(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchant(user.ID + 200)

	t.Run("create missing role", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		store := mockdb.NewMockStore(ctrl)
		store.EXPECT().
			GetUserRoleByType(gomock.Any(), gomock.Eq(db.GetUserRoleByTypeParams{UserID: user.ID, Role: RoleMerchantStaff})).
			Times(1).
			Return(db.UserRole{}, db.ErrRecordNotFound)
		store.EXPECT().
			CreateUserRole(gomock.Any(), gomock.Eq(db.CreateUserRoleParams{
				UserID:          user.ID,
				Role:            RoleMerchantStaff,
				Status:          "active",
				RelatedEntityID: pgtype.Int8{Int64: merchant.ID, Valid: true},
			})).
			Times(1).
			Return(db.UserRole{}, nil)

		server := newTestServer(t, store)
		err := server.ensureMerchantStaffUserRoleActive(context.Background(), user.ID, merchant.ID)
		require.NoError(t, err)
	})

	t.Run("reactivate disabled role", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		store := mockdb.NewMockStore(ctrl)
		storedRole := db.UserRole{ID: 7, UserID: user.ID, Role: RoleMerchantStaff, Status: "disabled"}
		store.EXPECT().
			GetUserRoleByType(gomock.Any(), gomock.Eq(db.GetUserRoleByTypeParams{UserID: user.ID, Role: RoleMerchantStaff})).
			Times(1).
			Return(storedRole, nil)
		store.EXPECT().
			UpdateUserRoleStatus(gomock.Any(), gomock.Eq(db.UpdateUserRoleStatusParams{ID: storedRole.ID, Status: "active"})).
			Times(1).
			Return(db.UserRole{ID: storedRole.ID, UserID: user.ID, Role: RoleMerchantStaff, Status: "active"}, nil)

		server := newTestServer(t, store)
		err := server.ensureMerchantStaffUserRoleActive(context.Background(), user.ID, merchant.ID)
		require.NoError(t, err)
	})
}

func TestDisableMerchantStaffUserRoleIfUnused(t *testing.T) {
	user, _ := randomUser(t)
	pendingMerchant := randomMerchant(user.ID + 300)
	activeRole := db.UserRole{ID: 9, UserID: user.ID, Role: RoleMerchantStaff, Status: "active"}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().
		ListMerchantsByStaff(gomock.Any(), gomock.Eq(user.ID)).
		Times(1).
		Return([]db.Merchant{pendingMerchant}, nil)
	store.EXPECT().
		GetUserMerchantRole(gomock.Any(), gomock.Eq(db.GetUserMerchantRoleParams{MerchantID: pendingMerchant.ID, UserID: user.ID})).
		Times(1).
		Return("pending", nil)
	store.EXPECT().
		GetUserRoleByType(gomock.Any(), gomock.Eq(db.GetUserRoleByTypeParams{UserID: user.ID, Role: RoleMerchantStaff})).
		Times(1).
		Return(activeRole, nil)
	store.EXPECT().
		UpdateUserRoleStatus(gomock.Any(), gomock.Eq(db.UpdateUserRoleStatusParams{ID: activeRole.ID, Status: "disabled"})).
		Times(1).
		Return(db.UserRole{ID: activeRole.ID, UserID: user.ID, Role: RoleMerchantStaff, Status: "disabled"}, nil)

	server := newTestServer(t, store)
	err := server.disableMerchantStaffUserRoleIfUnused(context.Background(), user.ID)
	require.NoError(t, err)
}
