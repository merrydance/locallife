package contracts

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOfficialWithdrawQueryRequestRequiresOfficialTradeDateFormat(t *testing.T) {
	req := OfficialWithdrawQueryRequest{
		Version:       OfficialWithdrawVersion,
		TransSerialNo: "WD202605050001",
		TradeTime:     "2026-05-05",
	}
	require.NoError(t, req.Validate())

	req.TradeTime = "20260505"
	require.EqualError(t, req.Validate(), "baofu withdraw query tradeTime must use yyyy-MM-dd")

	req.TradeTime = "2026-02-30"
	require.EqualError(t, req.Validate(), "baofu withdraw query tradeTime must use yyyy-MM-dd")
}
