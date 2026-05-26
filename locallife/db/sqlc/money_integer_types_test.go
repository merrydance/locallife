package db

import "testing"

func TestMoneyIntegerGeneratedTypes(t *testing.T) {
	var _ int64 = GetRiderDepositStatsRow{}.TotalDeposit
	var _ int64 = GetRiderDepositStatsRow{}.TotalWithdraw
	var _ int64 = GetRiderDepositStatsRow{}.TotalDeduct

	var _ int64 = GetReservationStatsByDateRangeRow{}.TotalDeposit
	var _ int64 = GetReservationStatsByDateRangeRow{}.TotalPrepaid

	var _ int64 = GetMembershipTransactionStatsRow{}.TotalRecharge
	var _ int64 = GetMembershipTransactionStatsRow{}.TotalConsume

	var _ int64 = CreateProfitSharingOrderParams{}.PaymentFee
	var _ int32 = CreateProfitSharingOrderParams{}.PaymentFeeRateBps
}
