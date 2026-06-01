package logic

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/wechat"
	"github.com/stretchr/testify/require"
)

func TestWantedMerchantServiceSubmitManualReturnsMerchantAvailable(t *testing.T) {
	store := &fakeWantedMerchantStore{
		activeMerchant: &db.Merchant{ID: 88, RegionID: 9, Name: "阿姨炸鸡", Status: db.MerchantStatusActive},
	}
	checker := &fakeWantedMerchantTextChecker{}
	service := NewWantedMerchantService(store, checker)

	result, err := service.SubmitVote(context.Background(), WantedMerchantVoteInput{
		UserID:       11,
		WechatOpenID: "openid-11",
		RegionID:     9,
		Source:       WantedMerchantSourceManual,
		Name:         "  阿姨炸鸡  ",
	})

	require.NoError(t, err)
	require.Equal(t, WantedMerchantVoteResultMerchantAvailable, result.Result)
	require.EqualValues(t, 88, result.MerchantID.Int64)
	require.False(t, checker.called, "existing merchants should be detected before text security")
}

func TestWantedMerchantServiceSubmitManualReturnsFoundInRankWithoutAutoVoting(t *testing.T) {
	store := &fakeWantedMerchantStore{
		activeWanted: &db.WantedMerchant{
			ID:             123,
			RegionID:       9,
			NormalizedName: "阿姨炸鸡",
			DisplayName:    "阿姨炸鸡",
			WantCount:      7,
		},
	}
	service := NewWantedMerchantService(store, &fakeWantedMerchantTextChecker{})

	result, err := service.SubmitVote(context.Background(), WantedMerchantVoteInput{
		UserID:       11,
		WechatOpenID: "openid-11",
		RegionID:     9,
		Source:       WantedMerchantSourceManual,
		Name:         "阿姨炸鸡",
	})

	require.NoError(t, err)
	require.Equal(t, WantedMerchantVoteResultFoundInRank, result.Result)
	require.EqualValues(t, 123, result.WantedMerchantID)
	require.EqualValues(t, 7, result.WantCount)
	require.False(t, store.voteCalled, "re-entering a ranked merchant should only focus the row")
}

func TestWantedMerchantServiceSubmitManualNormalizesCaseAndSpaces(t *testing.T) {
	store := &fakeWantedMerchantStore{}
	service := NewWantedMerchantService(store, &fakeWantedMerchantTextChecker{})

	_, err := service.SubmitVote(context.Background(), WantedMerchantVoteInput{
		UserID:       11,
		WechatOpenID: "openid-11",
		RegionID:     9,
		Source:       WantedMerchantSourceManual,
		Name:         " K F C ",
	})

	require.NoError(t, err)
	require.Equal(t, "kfc", store.lastMerchantLookup.NormalizedName)
	require.Equal(t, "kfc", store.lastWantedLookup.NormalizedName)
	require.Equal(t, "kfc", store.lastVoteArg.NormalizedName)
}

func TestWantedMerchantServiceSubmitManualBlocksRiskyText(t *testing.T) {
	store := &fakeWantedMerchantStore{}
	checker := &fakeWantedMerchantTextChecker{err: wechat.ErrRiskyTextContent}
	service := NewWantedMerchantService(store, checker)

	_, err := service.SubmitVote(context.Background(), WantedMerchantVoteInput{
		UserID:       11,
		WechatOpenID: "openid-11",
		RegionID:     9,
		Source:       WantedMerchantSourceManual,
		Name:         "风险店名",
	})

	require.Error(t, err)
	var reqErr *RequestError
	require.True(t, errors.As(err, &reqErr))
	require.Equal(t, http.StatusBadRequest, reqErr.Status)
	require.Contains(t, reqErr.Err.Error(), "店名包含暂不支持发布的内容")
	require.False(t, store.voteCalled)
}

func TestWantedMerchantServiceSubmitManualRequiresTextChecker(t *testing.T) {
	store := &fakeWantedMerchantStore{}
	service := NewWantedMerchantService(store, nil)

	_, err := service.SubmitVote(context.Background(), WantedMerchantVoteInput{
		UserID:       11,
		WechatOpenID: "openid-11",
		RegionID:     9,
		Source:       WantedMerchantSourceManual,
		Name:         "阿姨炸鸡",
	})

	require.Error(t, err)
	var reqErr *RequestError
	require.True(t, errors.As(err, &reqErr))
	require.Equal(t, http.StatusBadGateway, reqErr.Status)
	require.Contains(t, reqErr.Err.Error(), "店名审核服务暂不可用")
	require.False(t, store.voteCalled)
}

func TestWantedMerchantServiceSubmitMapRejectsInvalidCoordinates(t *testing.T) {
	store := &fakeWantedMerchantStore{}
	service := NewWantedMerchantService(store, &fakeWantedMerchantTextChecker{})

	_, err := service.SubmitVote(context.Background(), WantedMerchantVoteInput{
		UserID:       11,
		WechatOpenID: "openid-11",
		RegionID:     9,
		Source:       WantedMerchantSourceMap,
		Name:         "阿姨炸鸡",
		Latitude:     wantedMerchantValidNumeric(91),
		Longitude:    wantedMerchantValidNumeric(116.4),
	})

	require.Error(t, err)
	var reqErr *RequestError
	require.True(t, errors.As(err, &reqErr))
	require.Equal(t, http.StatusBadRequest, reqErr.Status)
	require.Contains(t, reqErr.Err.Error(), "地图位置无效")
	require.False(t, store.voteCalled)
}

func TestWantedMerchantServiceVoteExistingAlreadyVotedDoesNotIncrement(t *testing.T) {
	store := &fakeWantedMerchantStore{
		activeWanted: &db.WantedMerchant{
			ID:             123,
			RegionID:       9,
			NormalizedName: "阿姨炸鸡",
			DisplayName:    "阿姨炸鸡",
			WantCount:      7,
		},
		voteResult: db.WantedMerchantVoteTxResult{
			Result: WantedMerchantVoteResultAlreadyVoted,
			WantedMerchant: db.WantedMerchant{
				ID:        123,
				WantCount: 7,
			},
		},
	}
	service := NewWantedMerchantService(store, &fakeWantedMerchantTextChecker{})

	result, err := service.VoteExisting(context.Background(), WantedMerchantExistingVoteInput{
		UserID:           11,
		RegionID:         9,
		WantedMerchantID: 123,
	})

	require.NoError(t, err)
	require.Equal(t, WantedMerchantVoteResultAlreadyVoted, result.Result)
	require.EqualValues(t, 7, result.WantCount)
	require.True(t, store.voteCalled)
}

type fakeWantedMerchantStore struct {
	activeMerchant     *db.Merchant
	activeWanted       *db.WantedMerchant
	voteResult         db.WantedMerchantVoteTxResult
	voteCalled         bool
	lastMerchantLookup db.FindActiveTakeoutMerchantByNormalizedNameParams
	lastWantedLookup   db.FindActiveWantedMerchantByNormalizedNameParams
	lastVoteArg        db.WantedMerchantVoteTxParams
}

func (s *fakeWantedMerchantStore) FindActiveTakeoutMerchantByNormalizedName(ctx context.Context, arg db.FindActiveTakeoutMerchantByNormalizedNameParams) (db.Merchant, error) {
	s.lastMerchantLookup = arg
	if s.activeMerchant == nil {
		return db.Merchant{}, db.ErrRecordNotFound
	}
	return *s.activeMerchant, nil
}

func (s *fakeWantedMerchantStore) FindActiveWantedMerchantByNormalizedName(ctx context.Context, arg db.FindActiveWantedMerchantByNormalizedNameParams) (db.WantedMerchant, error) {
	s.lastWantedLookup = arg
	if s.activeWanted == nil {
		return db.WantedMerchant{}, db.ErrRecordNotFound
	}
	return *s.activeWanted, nil
}

func (s *fakeWantedMerchantStore) GetActiveWantedMerchantByID(ctx context.Context, arg db.GetActiveWantedMerchantByIDParams) (db.WantedMerchant, error) {
	if s.activeWanted == nil || s.activeWanted.ID != arg.ID || s.activeWanted.RegionID != arg.RegionID {
		return db.WantedMerchant{}, db.ErrRecordNotFound
	}
	return *s.activeWanted, nil
}

func (s *fakeWantedMerchantStore) VoteWantedMerchantTx(ctx context.Context, arg db.WantedMerchantVoteTxParams) (db.WantedMerchantVoteTxResult, error) {
	s.voteCalled = true
	s.lastVoteArg = arg
	if s.voteResult.Result != "" {
		return s.voteResult, nil
	}
	return db.WantedMerchantVoteTxResult{
		Result: WantedMerchantVoteResultCreated,
		WantedMerchant: db.WantedMerchant{
			ID:        456,
			WantCount: 1,
		},
	}, nil
}

type fakeWantedMerchantTextChecker struct {
	err    error
	called bool
}

func (c *fakeWantedMerchantTextChecker) CheckWantedMerchantText(ctx context.Context, openID string, content string) error {
	c.called = true
	return c.err
}

func wantedMerchantValidNumeric(value float64) pgtype.Numeric {
	var numeric pgtype.Numeric
	_ = numeric.Scan(value)
	return numeric
}
