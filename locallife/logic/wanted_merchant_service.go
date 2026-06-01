package logic

import (
	"context"
	"errors"
	"math"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/wechat"
)

const (
	WantedMerchantSourceManual = db.WantedMerchantSourceManual
	WantedMerchantSourceMap    = db.WantedMerchantSourceMap

	WantedMerchantVoteResultCreated           = db.WantedMerchantVoteResultCreated
	WantedMerchantVoteResultVoted             = db.WantedMerchantVoteResultVoted
	WantedMerchantVoteResultAlreadyVoted      = db.WantedMerchantVoteResultAlreadyVoted
	WantedMerchantVoteResultFoundInRank       = "found_in_rank"
	WantedMerchantVoteResultMerchantAvailable = "merchant_available"
)

type WantedMerchantVoteInput struct {
	UserID       int64
	WechatOpenID string
	RegionID     int64
	Source       string
	Name         string
	Address      string
	Latitude     pgtype.Numeric
	Longitude    pgtype.Numeric
}

type WantedMerchantExistingVoteInput struct {
	UserID           int64
	RegionID         int64
	WantedMerchantID int64
}

type WantedMerchantVoteResult struct {
	Result           string
	WantedMerchantID int64
	MerchantID       pgtype.Int8
	Rank             int64
	WantCount        int32
}

type wantedMerchantStore interface {
	FindActiveTakeoutMerchantByNormalizedName(ctx context.Context, arg db.FindActiveTakeoutMerchantByNormalizedNameParams) (db.Merchant, error)
	FindActiveWantedMerchantByNormalizedName(ctx context.Context, arg db.FindActiveWantedMerchantByNormalizedNameParams) (db.WantedMerchant, error)
	GetActiveWantedMerchantByID(ctx context.Context, arg db.GetActiveWantedMerchantByIDParams) (db.WantedMerchant, error)
	VoteWantedMerchantTx(ctx context.Context, arg db.WantedMerchantVoteTxParams) (db.WantedMerchantVoteTxResult, error)
}

type wantedMerchantTextChecker interface {
	CheckWantedMerchantText(ctx context.Context, openID string, content string) error
}

type WantedMerchantService struct {
	store       wantedMerchantStore
	textChecker wantedMerchantTextChecker
}

func NewWantedMerchantService(store wantedMerchantStore, textChecker wantedMerchantTextChecker) *WantedMerchantService {
	return &WantedMerchantService{store: store, textChecker: textChecker}
}

func (s *WantedMerchantService) SubmitVote(ctx context.Context, input WantedMerchantVoteInput) (WantedMerchantVoteResult, error) {
	if s == nil || s.store == nil {
		return WantedMerchantVoteResult{}, NewRequestError(http.StatusServiceUnavailable, errors.New("想吃榜服务暂不可用，请稍后重试"))
	}
	normalizedName := normalizeWantedMerchantName(input.Name)
	if input.RegionID <= 0 || normalizedName == "" {
		return WantedMerchantVoteResult{}, NewRequestError(http.StatusBadRequest, errors.New("请选择有效区县并填写店名"))
	}
	source := strings.TrimSpace(input.Source)
	if source == "" {
		source = WantedMerchantSourceManual
	}

	merchant, err := s.store.FindActiveTakeoutMerchantByNormalizedName(ctx, db.FindActiveTakeoutMerchantByNormalizedNameParams{
		RegionID:       input.RegionID,
		NormalizedName: normalizedName,
	})
	if err == nil {
		return WantedMerchantVoteResult{
			Result:     WantedMerchantVoteResultMerchantAvailable,
			MerchantID: pgtype.Int8{Int64: merchant.ID, Valid: true},
		}, nil
	}
	if err != nil && !errors.Is(err, db.ErrRecordNotFound) {
		return WantedMerchantVoteResult{}, err
	}

	candidate, err := s.store.FindActiveWantedMerchantByNormalizedName(ctx, db.FindActiveWantedMerchantByNormalizedNameParams{
		RegionID:       input.RegionID,
		NormalizedName: normalizedName,
	})
	if err == nil {
		return WantedMerchantVoteResult{
			Result:           WantedMerchantVoteResultFoundInRank,
			WantedMerchantID: candidate.ID,
			WantCount:        candidate.WantCount,
		}, nil
	}
	if err != nil && !errors.Is(err, db.ErrRecordNotFound) {
		return WantedMerchantVoteResult{}, err
	}

	switch source {
	case WantedMerchantSourceManual:
	case WantedMerchantSourceMap:
		if !validWantedMerchantCoordinate(input.Latitude, -90, 90) ||
			!validWantedMerchantCoordinate(input.Longitude, -180, 180) {
			return WantedMerchantVoteResult{}, NewRequestError(http.StatusBadRequest, errors.New("地图位置无效，请重新选择店铺"))
		}
	default:
		return WantedMerchantVoteResult{}, NewRequestError(http.StatusBadRequest, errors.New("请选择有效提交方式"))
	}

	if source == WantedMerchantSourceManual && s.textChecker != nil {
		if err := s.textChecker.CheckWantedMerchantText(ctx, input.WechatOpenID, normalizedName); err != nil {
			if errors.Is(err, wechat.ErrRiskyTextContent) {
				return WantedMerchantVoteResult{}, NewRequestError(http.StatusBadRequest, errors.New("店名包含暂不支持发布的内容，请修改后再试"))
			}
			return WantedMerchantVoteResult{}, NewRequestErrorWithCause(http.StatusBadGateway, errors.New("店名审核服务暂不可用，请稍后重试"), err)
		}
	} else if source == WantedMerchantSourceManual {
		return WantedMerchantVoteResult{}, NewRequestErrorWithCause(
			http.StatusBadGateway,
			errors.New("店名审核服务暂不可用，请稍后重试"),
			errors.New("wanted merchant text checker is not configured"),
		)
	}

	txResult, err := s.store.VoteWantedMerchantTx(ctx, db.WantedMerchantVoteTxParams{
		UserID:         input.UserID,
		RegionID:       input.RegionID,
		NormalizedName: normalizedName,
		DisplayName:    strings.TrimSpace(input.Name),
		Address:        strings.TrimSpace(input.Address),
		Latitude:       input.Latitude,
		Longitude:      input.Longitude,
		Source:         source,
	})
	if err != nil {
		return WantedMerchantVoteResult{}, err
	}
	return newWantedMerchantVoteResult(txResult), nil
}

func (s *WantedMerchantService) VoteExisting(ctx context.Context, input WantedMerchantExistingVoteInput) (WantedMerchantVoteResult, error) {
	if s == nil || s.store == nil {
		return WantedMerchantVoteResult{}, NewRequestError(http.StatusServiceUnavailable, errors.New("想吃榜服务暂不可用，请稍后重试"))
	}
	if input.RegionID <= 0 || input.WantedMerchantID <= 0 {
		return WantedMerchantVoteResult{}, NewRequestError(http.StatusBadRequest, errors.New("请选择有效榜单商户"))
	}
	_, err := s.store.GetActiveWantedMerchantByID(ctx, db.GetActiveWantedMerchantByIDParams{
		ID:       input.WantedMerchantID,
		RegionID: input.RegionID,
	})
	if errors.Is(err, db.ErrRecordNotFound) {
		return WantedMerchantVoteResult{}, NewRequestError(http.StatusNotFound, errors.New("榜单商户不存在"))
	}
	if err != nil {
		return WantedMerchantVoteResult{}, err
	}
	txResult, err := s.store.VoteWantedMerchantTx(ctx, db.WantedMerchantVoteTxParams{
		UserID:           input.UserID,
		RegionID:         input.RegionID,
		ExistingWantedID: input.WantedMerchantID,
	})
	if err != nil {
		return WantedMerchantVoteResult{}, err
	}
	return newWantedMerchantVoteResult(txResult), nil
}

func normalizeWantedMerchantName(name string) string {
	fields := strings.Fields(strings.TrimSpace(name))
	return strings.ToLower(strings.Join(fields, ""))
}

func validWantedMerchantCoordinate(value pgtype.Numeric, minValue float64, maxValue float64) bool {
	if !value.Valid {
		return false
	}
	coordinate, err := value.Float64Value()
	if err != nil || !coordinate.Valid {
		return false
	}
	return !math.IsNaN(coordinate.Float64) && coordinate.Float64 >= minValue && coordinate.Float64 <= maxValue
}

func newWantedMerchantVoteResult(txResult db.WantedMerchantVoteTxResult) WantedMerchantVoteResult {
	return WantedMerchantVoteResult{
		Result:           txResult.Result,
		WantedMerchantID: txResult.WantedMerchant.ID,
		Rank:             txResult.Rank,
		WantCount:        txResult.WantedMerchant.WantCount,
	}
}
