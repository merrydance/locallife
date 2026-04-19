package db

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/merrydance/locallife/util"
	"github.com/stretchr/testify/require"
)

// ==================== Helper Functions ====================

// createRandomEcommerceApplymentForMerchant 为商户创建测试进件记录
func createRandomEcommerceApplymentForMerchant(t *testing.T) EcommerceApplyment {
	merchant := createRandomMerchantForTest(t)
	return createRandomEcommerceApplymentWithSubject(t, "merchant", merchant.ID)
}

// createRandomEcommerceApplymentWithSubject 创建指定主体的进件记录
func createRandomEcommerceApplymentWithSubject(t *testing.T, subjectType string, subjectID int64) EcommerceApplyment {
	return createRandomEcommerceApplymentWithSubjectAndOrgType(t, subjectType, subjectID, "2401")
}

func createRandomEcommerceApplymentWithSubjectAndOrgType(t *testing.T, subjectType string, subjectID int64, organizationType string) EcommerceApplyment {
	outRequestNo := util.RandomString(20)

	arg := CreateEcommerceApplymentParams{
		SubjectType:           subjectType,
		SubjectID:             subjectID,
		OutRequestNo:          outRequestNo,
		OrganizationType:      organizationType,
		BusinessLicenseNumber: pgtype.Text{String: util.RandomString(18), Valid: true},
		BusinessLicenseCopy:   pgtype.Text{String: "https://example.com/license.jpg", Valid: true},
		MerchantName:          util.RandomString(10),
		LegalPerson:           util.RandomString(6),
		IDCardNumber:          "110101199001011234",
		IDCardName:            util.RandomString(6),
		IDCardValidTime:       "长期",
		IDCardFrontCopy:       "https://example.com/id_front.jpg",
		IDCardBackCopy:        "https://example.com/id_back.jpg",
		AccountType:           "ACCOUNT_TYPE_PRIVATE",
		AccountBank:           "招商银行",
		AccountBankCode:       pgtype.Int8{Int64: 1099, Valid: true},
		BankAlias:             pgtype.Text{String: "深圳前海微众银行", Valid: true},
		BankAliasCode:         pgtype.Text{String: "1000009561", Valid: true},
		BankAddressCode:       "440300",
		BankBranchID:          pgtype.Text{String: "402584040001", Valid: true},
		BankName:              pgtype.Text{String: "招商银行深圳分行", Valid: true},
		AccountNumber:         "6214830012345678",
		AccountName:           util.RandomString(6),
		ContactName:           util.RandomString(6),
		ContactIDCardNumber:   pgtype.Text{String: "110101199001011234", Valid: true},
		MobilePhone:           "13800138000",
		ContactEmail:          pgtype.Text{String: "test@example.com", Valid: true},
		MerchantShortname:     util.RandomString(8),
		Qualifications:        []byte("[]"),
		BusinessAdditionPics:  []string{},
		BusinessAdditionDesc:  pgtype.Text{},
	}

	applyment, err := testStore.CreateEcommerceApplyment(context.Background(), arg)
	require.NoError(t, err)
	require.NotEmpty(t, applyment)

	require.Equal(t, arg.SubjectType, applyment.SubjectType)
	require.Equal(t, arg.SubjectID, applyment.SubjectID)
	require.Equal(t, arg.OutRequestNo, applyment.OutRequestNo)
	require.True(t, applyment.AccountBankCode.Valid)
	require.Equal(t, arg.AccountBankCode.Int64, applyment.AccountBankCode.Int64)
	require.True(t, applyment.BankAlias.Valid)
	require.Equal(t, arg.BankAlias.String, applyment.BankAlias.String)
	require.True(t, applyment.BankAliasCode.Valid)
	require.Equal(t, arg.BankAliasCode.String, applyment.BankAliasCode.String)
	require.True(t, applyment.BankBranchID.Valid)
	require.Equal(t, arg.BankBranchID.String, applyment.BankBranchID.String)
	require.Equal(t, "pending", applyment.Status)
	require.NotZero(t, applyment.ID)
	require.NotZero(t, applyment.CreatedAt)

	return applyment
}

// ==================== Test Cases ====================

func TestCreateEcommerceApplyment(t *testing.T) {
	createRandomEcommerceApplymentForMerchant(t)
}

func TestCreateEcommerceApplymentSupportedOrganizationTypes(t *testing.T) {
	t.Parallel()

	merchant := createRandomMerchantForTest(t)
	organizationTypes := []string{"2401", "2500", "4", "2", "3", "2502", "1708"}

	for _, organizationType := range organizationTypes {
		organizationType := organizationType
		t.Run(organizationType, func(t *testing.T) {
			applyment := createRandomEcommerceApplymentWithSubjectAndOrgType(t, "merchant", merchant.ID, organizationType)
			require.Equal(t, organizationType, applyment.OrganizationType)
		})
	}
}

func TestGetEcommerceApplyment(t *testing.T) {
	applyment1 := createRandomEcommerceApplymentForMerchant(t)

	applyment2, err := testStore.GetEcommerceApplyment(context.Background(), applyment1.ID)
	require.NoError(t, err)
	require.NotEmpty(t, applyment2)

	require.Equal(t, applyment1.ID, applyment2.ID)
	require.Equal(t, applyment1.SubjectType, applyment2.SubjectType)
	require.Equal(t, applyment1.SubjectID, applyment2.SubjectID)
	require.Equal(t, applyment1.OutRequestNo, applyment2.OutRequestNo)
	require.Equal(t, applyment1.Status, applyment2.Status)
}

func TestGetEcommerceApplymentByOutRequestNo(t *testing.T) {
	applyment1 := createRandomEcommerceApplymentForMerchant(t)

	applyment2, err := testStore.GetEcommerceApplymentByOutRequestNo(context.Background(), applyment1.OutRequestNo)
	require.NoError(t, err)
	require.NotEmpty(t, applyment2)

	require.Equal(t, applyment1.ID, applyment2.ID)
	require.Equal(t, applyment1.OutRequestNo, applyment2.OutRequestNo)
}

func TestGetLatestEcommerceApplymentBySubject(t *testing.T) {
	merchant := createRandomMerchantForTest(t)
	tiedCreatedAt := time.Now().UTC().Truncate(time.Microsecond)

	// 创建两个进件记录
	applyment1 := createRandomEcommerceApplymentWithSubject(t, "merchant", merchant.ID)
	applyment2 := createRandomEcommerceApplymentWithSubject(t, "merchant", merchant.ID)

	_, err := testStore.(*SQLStore).connPool.Exec(context.Background(),
		`UPDATE ecommerce_applyments SET created_at = $1 WHERE id = ANY($2)`,
		tiedCreatedAt,
		[]int64{applyment1.ID, applyment2.ID},
	)
	require.NoError(t, err)

	// 获取最新的
	latest, err := testStore.GetLatestEcommerceApplymentBySubject(context.Background(), GetLatestEcommerceApplymentBySubjectParams{
		SubjectType: "merchant",
		SubjectID:   merchant.ID,
	})
	require.NoError(t, err)
	require.NotEmpty(t, latest)

	// 应该是第二个（最新的）
	require.Equal(t, applyment2.ID, latest.ID)
	require.NotEqual(t, applyment1.ID, latest.ID)
}

func TestGetEcommerceApplymentBySubjectUsesIDTieBreaker(t *testing.T) {
	merchant := createRandomMerchantForTest(t)
	tiedCreatedAt := time.Now().UTC().Truncate(time.Microsecond)

	applyment1 := createRandomEcommerceApplymentWithSubject(t, "merchant", merchant.ID)
	applyment2 := createRandomEcommerceApplymentWithSubject(t, "merchant", merchant.ID)

	_, err := testStore.(*SQLStore).connPool.Exec(context.Background(),
		`UPDATE ecommerce_applyments SET created_at = $1 WHERE id = ANY($2)`,
		tiedCreatedAt,
		[]int64{applyment1.ID, applyment2.ID},
	)
	require.NoError(t, err)

	latest, err := testStore.GetEcommerceApplymentBySubject(context.Background(), GetEcommerceApplymentBySubjectParams{
		SubjectType: "merchant",
		SubjectID:   merchant.ID,
	})
	require.NoError(t, err)
	require.Equal(t, applyment2.ID, latest.ID)
}

func TestUpdateEcommerceApplymentToSubmitted(t *testing.T) {
	applyment := createRandomEcommerceApplymentForMerchant(t)
	require.Equal(t, "pending", applyment.Status)

	// 更新为已提交
	wxApplymentID := int64(123456789)
	updated, err := testStore.UpdateEcommerceApplymentToSubmitted(context.Background(), UpdateEcommerceApplymentToSubmittedParams{
		ID:          applyment.ID,
		ApplymentID: pgtype.Int8{Int64: wxApplymentID, Valid: true},
	})
	require.NoError(t, err)
	require.NotEmpty(t, updated)

	require.Equal(t, "submitted", updated.Status)
	require.Equal(t, wxApplymentID, updated.ApplymentID.Int64)
	require.True(t, updated.SubmittedAt.Valid)
}

func TestUpdateEcommerceApplymentStatus(t *testing.T) {
	applyment := createRandomEcommerceApplymentForMerchant(t)

	testCases := []struct {
		name         string
		newStatus    string
		rejectReason string
		signURL      string
		signState    string
		subMchID     string
	}{
		{
			name:      "Update to auditing",
			newStatus: "auditing",
		},
		{
			name:      "Update to checking",
			newStatus: "checking",
		},
		{
			name:      "Update to account_need_verify",
			newStatus: "account_need_verify",
		},
		{
			name:      "Update to to_be_confirmed",
			newStatus: "to_be_confirmed",
		},
		{
			name:         "Update to rejected",
			newStatus:    "rejected",
			rejectReason: "资料不符合要求",
		},
		{
			name:      "Update to to_be_signed",
			newStatus: "to_be_signed",
			signURL:   "https://pay.weixin.qq.com/sign/xxxx",
		},
		{
			name:      "Update to signing",
			newStatus: "signing",
			signState: "SIGNING",
		},
		{
			name:      "Update to finish",
			newStatus: "finish",
			subMchID:  "1234567890",
		},
		{
			name:      "Update to canceled",
			newStatus: "canceled",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			updated, err := testStore.UpdateEcommerceApplymentStatus(context.Background(), UpdateEcommerceApplymentStatusParams{
				ID:           applyment.ID,
				Status:       tc.newStatus,
				RejectReason: pgtype.Text{String: tc.rejectReason, Valid: tc.rejectReason != ""},
				SignUrl:      pgtype.Text{String: tc.signURL, Valid: tc.signURL != ""},
				SignState:    pgtype.Text{String: tc.signState, Valid: tc.signState != ""},
				SubMchID:     pgtype.Text{String: tc.subMchID, Valid: tc.subMchID != ""},
			})
			require.NoError(t, err)
			require.NotEmpty(t, updated)
			require.Equal(t, tc.newStatus, updated.Status)

			if tc.rejectReason != "" {
				require.Equal(t, tc.rejectReason, updated.RejectReason.String)
			}
			if tc.signURL != "" {
				require.Equal(t, tc.signURL, updated.SignUrl.String)
			}
			if tc.signState != "" {
				require.Equal(t, tc.signState, updated.SignState.String)
			}
			if tc.subMchID != "" {
				require.Equal(t, tc.subMchID, updated.SubMchID.String)
			}
		})
	}
}

func TestUpdateEcommerceApplymentStatusPersistsVerificationFields(t *testing.T) {
	applyment := createRandomEcommerceApplymentForMerchant(t)
	accountValidation := []byte(`{"pay_amount":66,"destination_account_number":"6222000000001234","remark":"verify"}`)

	updated, err := testStore.UpdateEcommerceApplymentStatus(context.Background(), UpdateEcommerceApplymentStatusParams{
		ID:                 applyment.ID,
		ApplymentID:        pgtype.Int8{Int64: 55667788, Valid: true},
		Status:             "account_need_verify",
		RejectReason:       pgtype.Text{},
		SignUrl:            pgtype.Text{},
		SignState:          pgtype.Text{},
		LegalValidationUrl: pgtype.Text{String: "https://wx.example.com/legal-check", Valid: true},
		AccountValidation:  accountValidation,
		SubMchID:           pgtype.Text{},
	})
	require.NoError(t, err)
	require.Equal(t, int64(55667788), updated.ApplymentID.Int64)
	require.True(t, updated.LegalValidationUrl.Valid)
	require.Equal(t, "https://wx.example.com/legal-check", updated.LegalValidationUrl.String)
	require.JSONEq(t, string(accountValidation), string(updated.AccountValidation))

	found, err := testStore.GetEcommerceApplyment(context.Background(), applyment.ID)
	require.NoError(t, err)
	require.Equal(t, int64(55667788), found.ApplymentID.Int64)
	require.True(t, found.LegalValidationUrl.Valid)
	require.Equal(t, "https://wx.example.com/legal-check", found.LegalValidationUrl.String)
	require.JSONEq(t, string(accountValidation), string(found.AccountValidation))
}

func TestUpdateEcommerceApplymentSubMchID(t *testing.T) {
	applyment := createRandomEcommerceApplymentForMerchant(t)

	subMchID := "1234567890"
	updated, err := testStore.UpdateEcommerceApplymentSubMchID(context.Background(), UpdateEcommerceApplymentSubMchIDParams{
		ID:       applyment.ID,
		SubMchID: pgtype.Text{String: subMchID, Valid: true},
	})
	require.NoError(t, err)
	require.NotEmpty(t, updated)

	require.Equal(t, "finish", updated.Status)
	require.Equal(t, subMchID, updated.SubMchID.String)
	require.True(t, updated.AuditedAt.Valid)
}

func TestListEcommerceApplymentsBySubject(t *testing.T) {
	merchant := createRandomMerchantForTest(t)

	before, err := testStore.ListEcommerceApplymentsBySubject(context.Background(), ListEcommerceApplymentsBySubjectParams{
		SubjectType: "merchant",
		SubjectID:   merchant.ID,
	})
	require.NoError(t, err)

	// 创建多个进件记录
	n := 3
	createdIDs := make(map[int64]struct{}, n)
	for i := 0; i < n; i++ {
		created := createRandomEcommerceApplymentWithSubject(t, "merchant", merchant.ID)
		createdIDs[created.ID] = struct{}{}
	}

	applyments, err := testStore.ListEcommerceApplymentsBySubject(context.Background(), ListEcommerceApplymentsBySubjectParams{
		SubjectType: "merchant",
		SubjectID:   merchant.ID,
	})
	require.NoError(t, err)
	require.Len(t, applyments, len(before)+n)

	// 验证所有记录都属于该商户
	for _, a := range applyments {
		require.Equal(t, "merchant", a.SubjectType)
		require.Equal(t, merchant.ID, a.SubjectID)
	}

	// 验证新创建记录都在结果中
	for _, a := range applyments {
		delete(createdIDs, a.ID)
	}
	require.Empty(t, createdIDs)
}

func TestListEcommerceApplymentsByStatus(t *testing.T) {
	// 创建几个不同状态的进件记录
	applyment := createRandomEcommerceApplymentForMerchant(t)

	// 更新状态
	_, err := testStore.UpdateEcommerceApplymentToSubmitted(context.Background(), UpdateEcommerceApplymentToSubmittedParams{
		ID:          applyment.ID,
		ApplymentID: pgtype.Int8{Int64: 123456789, Valid: true},
	})
	require.NoError(t, err)

	applyments, err := testStore.ListEcommerceApplymentsByStatus(context.Background(), ListEcommerceApplymentsByStatusParams{
		Status: "submitted",
		Limit:  10,
		Offset: 0,
	})
	require.NoError(t, err)
	require.NotEmpty(t, applyments)

	// 验证所有记录状态都是 submitted
	for _, a := range applyments {
		require.Equal(t, "submitted", a.Status)
	}
}

func TestListPendingEcommerceApplyments(t *testing.T) {
	sqlStore, ok := testStore.(*SQLStore)
	require.True(t, ok)

	var pendingCountBefore int64
	err := sqlStore.connPool.QueryRow(context.Background(), `
		SELECT count(*)
		FROM ecommerce_applyments
		WHERE status IN ('submitted', 'checking', 'auditing', 'account_need_verify', 'to_be_confirmed', 'to_be_signed', 'signing')
	`).Scan(&pendingCountBefore)
	require.NoError(t, err)

	testCases := []struct {
		status    string
		signURL   string
		signState string
	}{
		{status: "submitted"},
		{status: "checking"},
		{status: "auditing"},
		{status: "account_need_verify"},
		{status: "to_be_confirmed"},
		{status: "to_be_signed", signURL: "https://pay.weixin.qq.com/sign/xxxx"},
		{status: "signing", signState: "SIGNING"},
	}

	createdStatuses := make(map[int64]string, len(testCases))
	for index, tc := range testCases {
		applyment := createRandomEcommerceApplymentForMerchant(t)
		createdStatuses[applyment.ID] = tc.status

		if tc.status == "submitted" {
			_, err = testStore.UpdateEcommerceApplymentToSubmitted(context.Background(), UpdateEcommerceApplymentToSubmittedParams{
				ID:          applyment.ID,
				ApplymentID: pgtype.Int8{Int64: int64(123456789 + index), Valid: true},
			})
			require.NoError(t, err)
			continue
		}

		_, err = testStore.UpdateEcommerceApplymentStatus(context.Background(), UpdateEcommerceApplymentStatusParams{
			ID:           applyment.ID,
			Status:       tc.status,
			RejectReason: pgtype.Text{},
			SignUrl:      pgtype.Text{String: tc.signURL, Valid: tc.signURL != ""},
			SignState:    pgtype.Text{String: tc.signState, Valid: tc.signState != ""},
			SubMchID:     pgtype.Text{},
		})
		require.NoError(t, err)
	}

	applyments, err := testStore.ListPendingEcommerceApplyments(context.Background(), ListPendingEcommerceApplymentsParams{
		Limit:  int32(len(testCases)),
		Offset: int32(pendingCountBefore),
	})
	require.NoError(t, err)

	expectedStatuses := map[string]struct{}{
		"submitted":           {},
		"checking":            {},
		"auditing":            {},
		"account_need_verify": {},
		"to_be_confirmed":     {},
		"to_be_signed":        {},
		"signing":             {},
	}

	foundStatuses := make(map[int64]string, len(createdStatuses))
	for _, a := range applyments {
		_, ok := expectedStatuses[a.Status]
		require.True(t, ok, "unexpected pending applyment status: %s", a.Status)
		if _, exists := createdStatuses[a.ID]; exists {
			foundStatuses[a.ID] = a.Status
		}
	}

	require.Len(t, foundStatuses, len(createdStatuses))
	for id, status := range createdStatuses {
		require.Equal(t, status, foundStatuses[id])
	}
}

func TestListPendingEcommerceApplymentsUsesIDTieBreaker(t *testing.T) {
	sqlStore, ok := testStore.(*SQLStore)
	require.True(t, ok)

	var pendingCountBefore int64
	err := sqlStore.connPool.QueryRow(context.Background(), `
		SELECT count(*)
		FROM ecommerce_applyments
		WHERE status IN ('submitted', 'checking', 'auditing', 'account_need_verify', 'to_be_confirmed', 'to_be_signed', 'signing')
	`).Scan(&pendingCountBefore)
	require.NoError(t, err)

	tiedCreatedAt := time.Now().UTC().Truncate(time.Microsecond)
	applyment1 := createRandomEcommerceApplymentForMerchant(t)
	applyment2 := createRandomEcommerceApplymentForMerchant(t)

	for index, applyment := range []EcommerceApplyment{applyment1, applyment2} {
		_, err = testStore.UpdateEcommerceApplymentToSubmitted(context.Background(), UpdateEcommerceApplymentToSubmittedParams{
			ID:          applyment.ID,
			ApplymentID: pgtype.Int8{Int64: int64(223456789 + index), Valid: true},
		})
		require.NoError(t, err)
	}

	_, err = sqlStore.connPool.Exec(context.Background(),
		`UPDATE ecommerce_applyments SET created_at = $1 WHERE id = ANY($2)`,
		tiedCreatedAt,
		[]int64{applyment1.ID, applyment2.ID},
	)
	require.NoError(t, err)

	applyments, err := testStore.ListPendingEcommerceApplyments(context.Background(), ListPendingEcommerceApplymentsParams{
		Limit:  2,
		Offset: int32(pendingCountBefore),
	})
	require.NoError(t, err)
	require.Len(t, applyments, 2)
	require.Equal(t, applyment1.ID, applyments[0].ID)
	require.Equal(t, applyment2.ID, applyments[1].ID)
}

func TestCountEcommerceApplymentsByStatus(t *testing.T) {
	// 创建一些记录
	createRandomEcommerceApplymentForMerchant(t)
	createRandomEcommerceApplymentForMerchant(t)

	count, err := testStore.CountEcommerceApplymentsByStatus(context.Background(), "pending")
	require.NoError(t, err)
	require.GreaterOrEqual(t, count, int64(2))
}

func TestGetEcommerceApplymentByApplymentID(t *testing.T) {
	applyment := createRandomEcommerceApplymentForMerchant(t)

	// 使用随机 ApplymentID 避免与其他测试冲突
	wxApplymentID := util.RandomInt(10000000000, 99999999999)
	_, err := testStore.UpdateEcommerceApplymentToSubmitted(context.Background(), UpdateEcommerceApplymentToSubmittedParams{
		ID:          applyment.ID,
		ApplymentID: pgtype.Int8{Int64: wxApplymentID, Valid: true},
	})
	require.NoError(t, err)

	found, err := testStore.GetEcommerceApplymentByApplymentID(context.Background(), pgtype.Int8{Int64: wxApplymentID, Valid: true})
	require.NoError(t, err)
	require.NotEmpty(t, found)
	require.Equal(t, applyment.ID, found.ID)
	require.Equal(t, wxApplymentID, found.ApplymentID.Int64)
}

// ==================== 边界情况测试 ====================

func TestGetEcommerceApplymentNotFound(t *testing.T) {
	_, err := testStore.GetEcommerceApplyment(context.Background(), 999999999)
	require.Error(t, err)
}

func TestGetEcommerceApplymentByOutRequestNoNotFound(t *testing.T) {
	_, err := testStore.GetEcommerceApplymentByOutRequestNo(context.Background(), "non_existent_request_no")
	require.Error(t, err)
}

func TestGetLatestEcommerceApplymentBySubjectNotFound(t *testing.T) {
	_, err := testStore.GetLatestEcommerceApplymentBySubject(context.Background(), GetLatestEcommerceApplymentBySubjectParams{
		SubjectType: "merchant",
		SubjectID:   999999999,
	})
	require.Error(t, err)
}

// ==================== 并发测试 ====================

func TestConcurrentCreateEcommerceApplyment(t *testing.T) {
	merchant := createRandomMerchantForTest(t)

	// 并发创建多个进件记录
	n := 5
	errs := make(chan error, n)

	for i := 0; i < n; i++ {
		go func() {
			arg := CreateEcommerceApplymentParams{
				SubjectType:           "merchant",
				SubjectID:             merchant.ID,
				OutRequestNo:          util.RandomString(20),
				OrganizationType:      "2401",
				BusinessLicenseNumber: pgtype.Text{},
				BusinessLicenseCopy:   pgtype.Text{},
				MerchantName:          util.RandomString(10),
				LegalPerson:           util.RandomString(6),
				IDCardNumber:          "110101199001011234",
				IDCardName:            util.RandomString(6),
				IDCardValidTime:       "长期",
				IDCardFrontCopy:       "https://example.com/id_front.jpg",
				IDCardBackCopy:        "https://example.com/id_back.jpg",
				AccountType:           "ACCOUNT_TYPE_PRIVATE",
				AccountBank:           "招商银行",
				BankAddressCode:       "440300",
				BankName:              pgtype.Text{},
				AccountNumber:         "6214830012345678",
				AccountName:           util.RandomString(6),
				ContactName:           util.RandomString(6),
				ContactIDCardNumber:   pgtype.Text{},
				MobilePhone:           "13800138000",
				ContactEmail:          pgtype.Text{},
				MerchantShortname:     util.RandomString(8),
				Qualifications:        []byte("[]"),
				BusinessAdditionPics:  []string{},
				BusinessAdditionDesc:  pgtype.Text{},
			}

			_, err := testStore.CreateEcommerceApplyment(context.Background(), arg)
			errs <- err
		}()
	}

	// 验证所有创建都成功
	for i := 0; i < n; i++ {
		err := <-errs
		require.NoError(t, err)
	}
}

// ==================== 被拒后重新提交测试 ====================

func TestResubmitAfterRejection(t *testing.T) {
	merchant := createRandomMerchantForTest(t)

	// 创建第一个进件记录
	applyment1 := createRandomEcommerceApplymentWithSubject(t, "merchant", merchant.ID)

	// 更新为被拒
	_, err := testStore.UpdateEcommerceApplymentStatus(context.Background(), UpdateEcommerceApplymentStatusParams{
		ID:           applyment1.ID,
		Status:       "rejected",
		RejectReason: pgtype.Text{String: "资料不完整", Valid: true},
	})
	require.NoError(t, err)

	// 创建新的进件记录（重新提交）
	applyment2 := createRandomEcommerceApplymentWithSubject(t, "merchant", merchant.ID)
	require.NotEqual(t, applyment1.ID, applyment2.ID)

	// 获取最新的应该是新的
	latest, err := testStore.GetLatestEcommerceApplymentBySubject(context.Background(), GetLatestEcommerceApplymentBySubjectParams{
		SubjectType: "merchant",
		SubjectID:   merchant.ID,
	})
	require.NoError(t, err)
	require.Equal(t, applyment2.ID, latest.ID)
}
