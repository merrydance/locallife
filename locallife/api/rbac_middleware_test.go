package api

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/token"
	"github.com/merrydance/locallife/util"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestRoleMiddleware(t *testing.T) {
	user, _ := randomUser(t)

	testCases := []struct {
		name          string
		allowedRoles  []string
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name:         "OK_HasRole",
			allowedRoles: []string{RoleOperator},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					ListUserRoles(gomock.Any(), user.ID).
					Times(1).
					Return([]db.UserRole{
						{ID: 1, UserID: user.ID, Role: RoleOperator, Status: "active"},
					}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name:         "OK_HasOneOfMultipleRoles",
			allowedRoles: []string{RoleAdmin, RoleOperator},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					ListUserRoles(gomock.Any(), user.ID).
					Times(1).
					Return([]db.UserRole{
						{ID: 1, UserID: user.ID, Role: RoleOperator, Status: "active"},
					}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name:         "Forbidden_NoMatchingRole",
			allowedRoles: []string{RoleAdmin},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					ListUserRoles(gomock.Any(), user.ID).
					Times(1).
					Return([]db.UserRole{
						{ID: 1, UserID: user.ID, Role: RoleCustomer, Status: "active"},
					}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusForbidden, recorder.Code)
			},
		},
		{
			name:         "Forbidden_RoleSuspended",
			allowedRoles: []string{RoleOperator},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					ListUserRoles(gomock.Any(), user.ID).
					Times(1).
					Return([]db.UserRole{
						{ID: 1, UserID: user.ID, Role: RoleOperator, Status: "suspended"},
					}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusForbidden, recorder.Code)
			},
		},
		{
			name:         "Forbidden_NoRoles",
			allowedRoles: []string{RoleOperator},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					ListUserRoles(gomock.Any(), user.ID).
					Times(1).
					Return([]db.UserRole{}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusForbidden, recorder.Code)
			},
		},
	}

	for i := range testCases {
		tc := testCases[i]

		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			store := mockdb.NewMockStore(ctrl)
			tc.buildStubs(store)

			server := newTestServer(t, store)

			// 添加测试路由
			server.router.GET("/test-role",
				authMiddleware(server.tokenMaker),
				server.RoleMiddleware(tc.allowedRoles...),
				func(ctx *gin.Context) {
					ctx.JSON(http.StatusOK, gin.H{"status": "ok"})
				},
			)

			recorder := httptest.NewRecorder()
			request, err := http.NewRequest(http.MethodGet, "/test-role", nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

func TestOperatorMiddleware(t *testing.T) {
	user, _ := randomUser(t)
	operator := db.Operator{
		ID:       util.RandomInt(1, 100),
		UserID:   user.ID,
		RegionID: 1,
		Status:   "active",
	}

	testCases := []struct {
		name          string
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name: "OK",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					ListUserRoles(gomock.Any(), user.ID).
					Times(1).
					Return([]db.UserRole{
						{ID: 1, UserID: user.ID, Role: RoleOperator, Status: "active"},
					}, nil)

				store.EXPECT().
					GetOperatorByUser(gomock.Any(), user.ID).
					Times(1).
					Return(operator, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name: "Forbidden_OperatorNotFound",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					ListUserRoles(gomock.Any(), user.ID).
					Times(1).
					Return([]db.UserRole{
						{ID: 1, UserID: user.ID, Role: RoleOperator, Status: "active"},
					}, nil)

				store.EXPECT().
					GetOperatorByUser(gomock.Any(), user.ID).
					Times(1).
					Return(db.Operator{}, pgx.ErrNoRows)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusForbidden, recorder.Code)
			},
		},
		{
			name: "Forbidden_OperatorSuspended",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					ListUserRoles(gomock.Any(), user.ID).
					Times(1).
					Return([]db.UserRole{
						{ID: 1, UserID: user.ID, Role: RoleOperator, Status: "active"},
					}, nil)

				suspendedOperator := operator
				suspendedOperator.Status = "suspended"
				store.EXPECT().
					GetOperatorByUser(gomock.Any(), user.ID).
					Times(1).
					Return(suspendedOperator, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusForbidden, recorder.Code)
			},
		},
	}

	for i := range testCases {
		tc := testCases[i]

		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			store := mockdb.NewMockStore(ctrl)
			tc.buildStubs(store)

			server := newTestServer(t, store)

			// 添加测试路由（需要先经过 RoleMiddleware）
			server.router.GET("/test-operator",
				authMiddleware(server.tokenMaker),
				server.RoleMiddleware(RoleOperator),
				server.OperatorMiddleware(),
				func(ctx *gin.Context) {
					// 验证 operator 已加载到 context
					op, exists := GetOperatorFromContext(ctx)
					if exists {
						ctx.JSON(http.StatusOK, gin.H{"operator_id": op.ID})
					} else {
						ctx.JSON(http.StatusInternalServerError, gin.H{"error": "operator not in context"})
					}
				},
			)

			recorder := httptest.NewRecorder()
			request, err := http.NewRequest(http.MethodGet, "/test-operator", nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

func TestOperatorRegionMiddleware(t *testing.T) {
	user, _ := randomUser(t)
	regionID := int64(1)
	operator := db.Operator{
		ID:       util.RandomInt(1, 100),
		UserID:   user.ID,
		RegionID: regionID,
		Status:   "active",
	}

	testCases := []struct {
		name          string
		regionID      int64
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name:     "OK_ManagesRegion",
			regionID: regionID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					ListUserRoles(gomock.Any(), user.ID).
					Times(1).
					Return([]db.UserRole{
						{ID: 1, UserID: user.ID, Role: RoleOperator, Status: "active"},
					}, nil)

				store.EXPECT().
					GetOperatorByUser(gomock.Any(), user.ID).
					Times(1).
					Return(operator, nil)

				store.EXPECT().
					CheckOperatorManagesRegion(gomock.Any(), db.CheckOperatorManagesRegionParams{
						OperatorID: operator.ID,
						RegionID:   regionID,
					}).
					Times(1).
					Return(true, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name:     "Forbidden_DoesNotManageRegion",
			regionID: int64(999),
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					ListUserRoles(gomock.Any(), user.ID).
					Times(1).
					Return([]db.UserRole{
						{ID: 1, UserID: user.ID, Role: RoleOperator, Status: "active"},
					}, nil)

				store.EXPECT().
					GetOperatorByUser(gomock.Any(), user.ID).
					Times(1).
					Return(operator, nil)

				store.EXPECT().
					CheckOperatorManagesRegion(gomock.Any(), db.CheckOperatorManagesRegionParams{
						OperatorID: operator.ID,
						RegionID:   int64(999),
					}).
					Times(1).
					Return(false, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusForbidden, recorder.Code)
			},
		},
	}

	for i := range testCases {
		tc := testCases[i]

		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			store := mockdb.NewMockStore(ctrl)
			tc.buildStubs(store)

			server := newTestServer(t, store)

			// 添加测试路由
			server.router.GET("/test-region/:region_id",
				authMiddleware(server.tokenMaker),
				server.RoleMiddleware(RoleOperator),
				server.OperatorMiddleware(),
				server.OperatorRegionMiddleware("region_id"),
				func(ctx *gin.Context) {
					ctx.JSON(http.StatusOK, gin.H{"status": "ok"})
				},
			)

			recorder := httptest.NewRecorder()
			url := fmt.Sprintf("/test-region/%d", tc.regionID)
			request, err := http.NewRequest(http.MethodGet, url, nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

func TestMerchantOwnerMiddleware(t *testing.T) {
	user, _ := randomUser(t)
	merchantID := util.RandomInt(1, 100)
	merchant := db.Merchant{
		ID:          merchantID,
		OwnerUserID: user.ID,
		Name:        "Test Merchant",
		Status:      "approved",
		RegionID:    1,
	}

	testCases := []struct {
		name          string
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name: "OK",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					ListUserRoles(gomock.Any(), user.ID).
					Times(1).
					Return([]db.UserRole{
						{
							ID:              1,
							UserID:          user.ID,
							Role:            RoleMerchantOwner,
							Status:          "active",
							RelatedEntityID: pgtype.Int8{Int64: merchantID, Valid: true},
						},
					}, nil)

				store.EXPECT().
					GetUserRoleByType(gomock.Any(), db.GetUserRoleByTypeParams{
						UserID: user.ID,
						Role:   RoleMerchantOwner,
					}).
					Times(1).
					Return(db.UserRole{
						ID:              1,
						UserID:          user.ID,
						Role:            RoleMerchantOwner,
						Status:          "active",
						RelatedEntityID: pgtype.Int8{Int64: merchantID, Valid: true},
					}, nil)

				store.EXPECT().
					GetMerchant(gomock.Any(), merchantID).
					Times(1).
					Return(merchant, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name: "Forbidden_RoleNotFound",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					ListUserRoles(gomock.Any(), user.ID).
					Times(1).
					Return([]db.UserRole{
						{
							ID:              1,
							UserID:          user.ID,
							Role:            RoleMerchantOwner,
							Status:          "active",
							RelatedEntityID: pgtype.Int8{Int64: merchantID, Valid: true},
						},
					}, nil)

				store.EXPECT().
					GetUserRoleByType(gomock.Any(), db.GetUserRoleByTypeParams{
						UserID: user.ID,
						Role:   RoleMerchantOwner,
					}).
					Times(1).
					Return(db.UserRole{}, pgx.ErrNoRows)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusForbidden, recorder.Code)
			},
		},
		{
			name: "Forbidden_MerchantNotActive",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					ListUserRoles(gomock.Any(), user.ID).
					Times(1).
					Return([]db.UserRole{
						{
							ID:              1,
							UserID:          user.ID,
							Role:            RoleMerchantOwner,
							Status:          "active",
							RelatedEntityID: pgtype.Int8{Int64: merchantID, Valid: true},
						},
					}, nil)

				store.EXPECT().
					GetUserRoleByType(gomock.Any(), db.GetUserRoleByTypeParams{
						UserID: user.ID,
						Role:   RoleMerchantOwner,
					}).
					Times(1).
					Return(db.UserRole{
						ID:              1,
						UserID:          user.ID,
						Role:            RoleMerchantOwner,
						Status:          "active",
						RelatedEntityID: pgtype.Int8{Int64: merchantID, Valid: true},
					}, nil)

				suspendedMerchant := merchant
				suspendedMerchant.Status = "suspended"
				store.EXPECT().
					GetMerchant(gomock.Any(), merchantID).
					Times(1).
					Return(suspendedMerchant, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusForbidden, recorder.Code)
			},
		},
	}

	for i := range testCases {
		tc := testCases[i]

		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			store := mockdb.NewMockStore(ctrl)
			tc.buildStubs(store)

			server := newTestServer(t, store)

			// 添加测试路由
			server.router.GET("/test-merchant",
				authMiddleware(server.tokenMaker),
				server.RoleMiddleware(RoleMerchantOwner),
				server.MerchantOwnerMiddleware(),
				func(ctx *gin.Context) {
					m, exists := GetMerchantFromContext(ctx)
					if exists {
						ctx.JSON(http.StatusOK, gin.H{"merchant_id": m.ID})
					} else {
						ctx.JSON(http.StatusInternalServerError, gin.H{"error": "merchant not in context"})
					}
				},
			)

			recorder := httptest.NewRecorder()
			request, err := http.NewRequest(http.MethodGet, "/test-merchant", nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

func TestRiderMiddleware(t *testing.T) {
	user, _ := randomUser(t)
	rider := db.Rider{
		ID:       util.RandomInt(1, 100),
		UserID:   user.ID,
		Status:   "approved",
		RegionID: pgtype.Int8{Int64: 1, Valid: true},
	}

	testCases := []struct {
		name          string
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name: "OK",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					ListUserRoles(gomock.Any(), user.ID).
					Times(1).
					Return([]db.UserRole{
						{ID: 1, UserID: user.ID, Role: RoleRider, Status: "active"},
					}, nil)

				store.EXPECT().
					GetRiderByUserID(gomock.Any(), user.ID).
					Times(1).
					Return(rider, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name: "Forbidden_RiderNotFound",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					ListUserRoles(gomock.Any(), user.ID).
					Times(1).
					Return([]db.UserRole{
						{ID: 1, UserID: user.ID, Role: RoleRider, Status: "active"},
					}, nil)

				store.EXPECT().
					GetRiderByUserID(gomock.Any(), user.ID).
					Times(1).
					Return(db.Rider{}, pgx.ErrNoRows)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusForbidden, recorder.Code)
			},
		},
		{
			name: "Forbidden_RiderNotApproved",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					ListUserRoles(gomock.Any(), user.ID).
					Times(1).
					Return([]db.UserRole{
						{ID: 1, UserID: user.ID, Role: RoleRider, Status: "active"},
					}, nil)

				pendingRider := rider
				pendingRider.Status = "pending"
				store.EXPECT().
					GetRiderByUserID(gomock.Any(), user.ID).
					Times(1).
					Return(pendingRider, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusForbidden, recorder.Code)
			},
		},
	}

	for i := range testCases {
		tc := testCases[i]

		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			store := mockdb.NewMockStore(ctrl)
			tc.buildStubs(store)

			server := newTestServer(t, store)

			// 添加测试路由
			server.router.GET("/test-rider",
				authMiddleware(server.tokenMaker),
				server.RoleMiddleware(RoleRider),
				server.RiderMiddleware(),
				func(ctx *gin.Context) {
					r, exists := GetRiderFromContext(ctx)
					if exists {
						ctx.JSON(http.StatusOK, gin.H{"rider_id": r.ID})
					} else {
						ctx.JSON(http.StatusInternalServerError, gin.H{"error": "rider not in context"})
					}
				},
			)

			recorder := httptest.NewRecorder()
			request, err := http.NewRequest(http.MethodGet, "/test-rider", nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}
