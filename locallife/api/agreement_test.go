package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/util"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestListAgreementsAPI(t *testing.T) {
	agreements := []db.ListActiveAgreementsRow{
		{
			Type:        "USER_AGREEMENT",
			Title:       "User Agreement",
			Version:     "v1.0.0",
			PublishedOn: pgtype.Date{Time: time.Now(), Valid: true},
		},
		{
			Type:        "PRIVACY_POLICY",
			Title:       "Privacy Policy",
			Version:     "v1.0.0",
			PublishedOn: pgtype.Date{Time: time.Now(), Valid: true},
		},
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().
		ListActiveAgreements(gomock.Any()).
		Times(1).
		Return(agreements, nil)

	server := newTestServer(t, store)
	recorder := httptest.NewRecorder()

	url := "/v1/agreements"
	request, err := http.NewRequest(http.MethodGet, url, nil)
	require.NoError(t, err)

	request.Header.Set("X-Response-Envelope", "0")
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, util.RandomInt(1, 1000), time.Minute)
	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)

	var response []listActiveAgreementsResponse
	err = json.Unmarshal(recorder.Body.Bytes(), &response)
	require.NoError(t, err)
	require.Len(t, response, 2)
}

func TestGetAgreementAPI(t *testing.T) {
	agreement := db.Agreement{
		ID:          util.RandomInt(1, 1000),
		Type:        "USER_AGREEMENT",
		Title:       "User Agreement",
		Content:     "Content",
		Version:     "v1.0.0",
		PublishedOn: pgtype.Date{Time: time.Now(), Valid: true},
		IsActive:    true,
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().
		GetActiveAgreementByType(gomock.Any(), gomock.Eq(agreement.Type)).
		Times(1).
		Return(agreement, nil)

	server := newTestServer(t, store)
	recorder := httptest.NewRecorder()

	url := fmt.Sprintf("/v1/agreements/%s", agreement.Type)
	request, err := http.NewRequest(http.MethodGet, url, nil)
	require.NoError(t, err)

	request.Header.Set("X-Response-Envelope", "0")
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, util.RandomInt(1, 1000), time.Minute)
	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)

	var response db.Agreement
	err = json.Unmarshal(recorder.Body.Bytes(), &response)
	require.NoError(t, err)
	require.Equal(t, agreement.ID, response.ID)
	require.Equal(t, agreement.Type, response.Type)
}
