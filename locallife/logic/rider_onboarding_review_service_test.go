package logic

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/stretchr/testify/require"
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
