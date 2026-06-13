package logic

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/stretchr/testify/require"
	gomock "go.uber.org/mock/gomock"
)

func riderReviewTestApplication() db.RiderApplication {
	return db.RiderApplication{
		ID:                     1,
		UserID:                 99,
		RealName:               pgtype.Text{String: "张三", Valid: true},
		Phone:                  pgtype.Text{String: "13812345678", Valid: true},
		IDCardOcr:              []byte(`{"name":"张三","id_number":"110101199001011234","valid_end":"20350101"}`),
		HealthCertMediaAssetID: pgtype.Int8{Int64: 3, Valid: true},
		HealthCertOcr:          []byte(`{"name":"张三","id_number":"110101199001011234","valid_end":"2030年12月31日"}`),
		Status:                 db.RiderApplicationStatusSubmitted,
	}
}

func riderReviewFixedNow() time.Time {
	return time.Date(2026, 4, 22, 10, 0, 0, 0, time.UTC)
}

func TestEvaluateRiderApplication_PermanentIDCardStillRequiresHealthCertValidation(t *testing.T) {
	app := riderReviewTestApplication()
	app.IDCardOcr = []byte(`{"name":"张三","id_number":"110101199001011234","valid_end":"长期"}`)
	app.HealthCertOcr = []byte(`{"name":"李四","valid_end":"2030年12月31日"}`)

	approved, rejectReason, _ := evaluateRiderApplication(app, riderReviewFixedNow())

	require.False(t, approved)
	require.Equal(t, "健康证姓名与身份证姓名不一致", rejectReason)
}

func TestEvaluateRiderApplication_IgnoresHealthCertIDNumber(t *testing.T) {
	app := riderReviewTestApplication()
	app.HealthCertOcr = []byte(`{"name":"张三","id_number":"320101199001011234","valid_end":"2030年12月31日"}`)

	approved, rejectReason, _ := evaluateRiderApplication(app, riderReviewFixedNow())

	require.True(t, approved)
	require.Empty(t, rejectReason)
}

func TestEvaluateRiderApplication_AcceptsIDCardDateRangeWithDots(t *testing.T) {
	app := riderReviewTestApplication()
	app.IDCardOcr = []byte(`{"name":"张三","id_number":"110101199001011234","valid_end":"2020.01.01-2035.01.01"}`)

	approved, rejectReason, _ := evaluateRiderApplication(app, riderReviewFixedNow())

	require.True(t, approved)
	require.Empty(t, rejectReason)
}

func TestEvaluateRiderApplication_AcceptsStringifiedHealthCertOCR(t *testing.T) {
	app := riderReviewTestApplication()
	wrapped, err := json.Marshal(`{"name":"张三","id_number":"110101199001011234","valid_end":"2030年12月31日"}`)
	require.NoError(t, err)
	app.HealthCertOcr = wrapped

	approved, rejectReason, _ := evaluateRiderApplication(app, riderReviewFixedNow())

	require.True(t, approved)
	require.Empty(t, rejectReason)
}

func TestEvaluateRiderApplication_AcceptsHealthCertDateWithDots(t *testing.T) {
	app := riderReviewTestApplication()
	app.HealthCertOcr = []byte(`{"name":"张三","id_number":"110101199001011234","valid_end":"2030.12.31"}`)

	approved, rejectReason, _ := evaluateRiderApplication(app, riderReviewFixedNow())

	require.True(t, approved)
	require.Empty(t, rejectReason)
}

func TestEvaluateRiderApplication_ExtractsNameFromNoisyHealthCertOCR(t *testing.T) {
	app := riderReviewTestApplication()
	app.RealName = pgtype.Text{String: "周松涛", Valid: true}
	app.IDCardOcr = []byte(`{"name":"周松涛","id_number":"132229197706017792","valid_end":"2025.03.01-2035.03.01"}`)
	app.HealthCertOcr = []byte(`{"name":"人员健康合格证明安康姓全名周松涛","cert_number":"1305282025D590","valid_end":"2026.12.06"}`)

	approved, rejectReason, _ := evaluateRiderApplication(app, riderReviewFixedNow())

	require.True(t, approved)
	require.Empty(t, rejectReason)
}

func TestEvaluateRiderApplication_BlocksPendingHealthCertOCR(t *testing.T) {
	app := riderReviewTestApplication()
	app.HealthCertOcr = []byte(`{"status":"pending","name":"张三","valid_end":"2030年12月31日"}`)

	approved, rejectReason, _ := evaluateRiderApplication(app, riderReviewFixedNow())

	require.False(t, approved)
	require.Equal(t, "健康证OCR处理中，请稍后再提交", rejectReason)
}

func TestEvaluateRiderApplication_FallsBackToApplicationRealName(t *testing.T) {
	app := riderReviewTestApplication()
	app.IDCardOcr = []byte(`{"id_number":"110101199001011234","valid_end":"20350101"}`)
	app.RealName = pgtype.Text{String: "张三", Valid: true}

	approved, rejectReason, _ := evaluateRiderApplication(app, riderReviewFixedNow())

	require.True(t, approved)
	require.Empty(t, rejectReason)
}

func TestEvaluateRiderApplication_RejectsMissingIDNumber(t *testing.T) {
	app := riderReviewTestApplication()
	app.IDCardOcr = []byte(`{"name":"张三","valid_end":"20350101"}`)

	approved, rejectReason, _ := evaluateRiderApplication(app, riderReviewFixedNow())

	require.False(t, approved)
	require.Equal(t, "身份证号未识别，请重新上传清晰的身份证正面照片", rejectReason)
}

func TestRiderOnboardingReviewServiceProcessSubmittedApplication_UsesDurableApprovalTx(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	service := &RiderOnboardingReviewService{
		store:                       store,
		credentialGovernanceService: &CredentialGovernanceService{now: riderReviewFixedNow},
		now:                         riderReviewFixedNow,
	}

	application := riderReviewTestApplication()
	reviewRun := db.OnboardingReviewRun{
		ID:                 77,
		ApplicationType:    "rider",
		RiderApplicationID: pgtype.Int8{Int64: application.ID, Valid: true},
		RunStatus:          db.OnboardingReviewRunStatusCompleted,
		Stage:              "review",
		Outcome:            pgtype.Text{String: db.OnboardingReviewOutcomeApproved, Valid: true},
		ReasonCode:         pgtype.Text{String: "auto_approved", Valid: true},
		CreatedAt:          riderReviewFixedNow(),
	}
	expectedLedger, _ := json.Marshal(map[string]any{
		"name":        "张三",
		"id_number":   "110101199001011234",
		"cert_number": "",
		"valid_start": "",
		"valid_end":   "2030年12月31日",
	})

	store.EXPECT().
		ApproveRiderApplicationWithReviewTx(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, arg db.ApproveRiderApplicationWithReviewTxParams) (db.ApproveRiderApplicationWithReviewTxResult, error) {
			require.Equal(t, application.ID, arg.Approval.ApplicationID)
			require.Equal(t, reviewRun.ID, arg.ReviewRunID)
			require.Equal(t, application.ID, arg.ReviewCreation.RiderApplicationID.Int64)
			require.True(t, arg.ReviewCreation.RiderApplicationID.Valid)
			require.Equal(t, db.OnboardingReviewRunStatusQueued, arg.ReviewCreation.RunStatus)
			require.Equal(t, "review", arg.ReviewCreation.Stage)
			require.Equal(t, reviewRun.ID, arg.ReviewCompletion.ID)
			require.Equal(t, db.OnboardingReviewOutcomeApproved, arg.ReviewCompletion.Outcome.String)
			require.True(t, arg.ReviewCompletion.Outcome.Valid)
			require.Len(t, arg.CredentialEntries, 1)
			require.Equal(t, db.CredentialDocumentTypeHealthCert, arg.CredentialEntries[0].DocumentType)
			require.Equal(t, application.ID, arg.CredentialEntries[0].RiderApplicationID)
			require.Equal(t, reviewRun.ID, arg.CredentialEntries[0].ReviewRunID.Int64)
			require.True(t, arg.CredentialEntries[0].ReviewRunID.Valid)
			require.JSONEq(t, string(expectedLedger), string(arg.CredentialEntries[0].NormalizedPayload))
			return db.ApproveRiderApplicationWithReviewTxResult{
				Application:       db.RiderApplication{ID: application.ID, UserID: application.UserID, Status: db.RiderApplicationStatusApproved},
				Rider:             db.Rider{ID: 501, UserID: application.UserID, Status: db.RiderStatusApproved},
				ReviewRun:         reviewRun,
				CredentialLedgers: []db.CredentialLedger{{ID: 601}},
			}, nil
		})

	result, err := service.ProcessSubmittedApplication(context.Background(), application, application.UserID, &reviewRun.ID)
	require.NoError(t, err)
	require.True(t, result.Approved)
	require.Equal(t, reviewRun.ID, result.ReviewRun.ID)
	require.Equal(t, db.RiderApplicationStatusApproved, result.Application.Status)
	require.Len(t, result.CredentialEntries, 1)
}
