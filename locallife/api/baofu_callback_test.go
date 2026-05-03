package api

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgtype"
	baofunotification "github.com/merrydance/locallife/baofu/account/notification"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestBaofuAccountOpenCallbackPersistsFactBeforeAck(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	store := mockdb.NewMockStore(ctrl)
	server := newTestServer(t, store)
	server.SetBaofuAccountNotificationParserForTest(fakeBaofuOpenAccountParser{})

	store.EXPECT().CreateExternalPaymentFact(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ any, arg db.CreateExternalPaymentFactParams) (db.ExternalPaymentFact, error) {
			require.Equal(t, db.ExternalPaymentProviderBaofu, arg.Provider)
			require.Equal(t, db.PaymentChannelBaofuAggregate, arg.Channel)
			require.Equal(t, db.ExternalPaymentCapabilityBaofuAccount, arg.Capability)
			require.Equal(t, db.ExternalPaymentFactSourceCallback, arg.FactSource)
			require.Equal(t, "OPEN123", arg.ExternalObjectKey)
			require.Equal(t, "baofu:callback:account:OPEN123:1", arg.DedupeKey)
			require.Equal(t, db.ExternalPaymentTerminalStatusSuccess, arg.TerminalStatus)
			require.True(t, arg.IsTerminal)
			require.Equal(t, pgtype.Text{String: db.ExternalPaymentBusinessOwnerApplyment, Valid: true}, arg.BusinessOwner)
			require.False(t, arg.BusinessObjectType.Valid)
			require.False(t, arg.BusinessObjectID.Valid)
			return db.ExternalPaymentFact{ID: 88, DedupeKey: arg.DedupeKey}, nil
		})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/v1/webhooks/baofu/account/open", bytes.NewBufferString(`{"encrypted":true}`))
	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Contains(t, recorder.Body.String(), "SUCCESS")
}

type fakeBaofuOpenAccountParser struct{}

func (fakeBaofuOpenAccountParser) ParseOpenAccountNotification(body []byte) (*baofunotification.AccountNotification, error) {
	return &baofunotification.AccountNotification{
		OutRequestNo:  "OPEN123",
		ContractNo:    "CP123",
		SharingMerID:  "CP123",
		UpstreamState: "1",
		OpenState:     db.BaofuAccountOpenStateActive,
		OccurredAt:    time.Now().UTC(),
		Raw:           []byte(`{"outRequestNo":"OPEN123","status":"1"}`),
	}, nil
}

var _ = gin.TestMode
