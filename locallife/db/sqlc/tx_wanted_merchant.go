package db

import (
	"context"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5/pgtype"
)

type WantedMerchantVoteTxParams struct {
	UserID           int64
	RegionID         int64
	ExistingWantedID int64
	NormalizedName   string
	DisplayName      string
	Address          string
	Latitude         pgtype.Numeric
	Longitude        pgtype.Numeric
	Source           string
}

type WantedMerchantVoteTxResult struct {
	Result         string
	WantedMerchant WantedMerchant
	VoteInserted   bool
	Rank           int64
}

func (store *SQLStore) VoteWantedMerchantTx(ctx context.Context, arg WantedMerchantVoteTxParams) (WantedMerchantVoteTxResult, error) {
	var result WantedMerchantVoteTxResult

	err := store.execTx(ctx, func(q *Queries) error {
		var wanted WantedMerchant
		var err error

		if arg.ExistingWantedID > 0 {
			wanted, err = q.GetActiveWantedMerchantByIDForUpdate(ctx, GetActiveWantedMerchantByIDForUpdateParams{
				ID:       arg.ExistingWantedID,
				RegionID: arg.RegionID,
			})
		} else {
			wanted, err = q.CreateOrGetActiveWantedMerchant(ctx, CreateOrGetActiveWantedMerchantParams{
				RegionID:        arg.RegionID,
				NormalizedName:  arg.NormalizedName,
				DisplayName:     arg.DisplayName,
				Address:         pgtype.Text{String: arg.Address, Valid: strings.TrimSpace(arg.Address) != ""},
				Latitude:        arg.Latitude,
				Longitude:       arg.Longitude,
				Source:          arg.Source,
				CreatedByUserID: arg.UserID,
			})
		}
		if err != nil {
			return err
		}

		_, err = q.CreateWantedMerchantVote(ctx, CreateWantedMerchantVoteParams{
			WantedMerchantID: wanted.ID,
			RegionID:         arg.RegionID,
			UserID:           arg.UserID,
		})
		if err == nil {
			result.VoteInserted = true
			wanted, err = q.IncrementWantedMerchantWantCount(ctx, IncrementWantedMerchantWantCountParams{
				ID:       wanted.ID,
				RegionID: arg.RegionID,
			})
			if err != nil {
				return err
			}
			if arg.ExistingWantedID == 0 && wanted.WantCount == 1 {
				result.Result = WantedMerchantVoteResultCreated
			} else {
				result.Result = WantedMerchantVoteResultVoted
			}
		} else if errors.Is(err, ErrRecordNotFound) {
			result.Result = WantedMerchantVoteResultAlreadyVoted
		} else {
			return err
		}

		rank, err := q.GetWantedMerchantRank(ctx, GetWantedMerchantRankParams{
			RegionID: arg.RegionID,
			ID:       wanted.ID,
		})
		if err != nil {
			return err
		}

		result.WantedMerchant = wanted
		result.Rank = rank
		return nil
	})

	return result, err
}

func normalizeWantedMerchantNameForDB(name string) string {
	fields := strings.Fields(strings.TrimSpace(name))
	return strings.ToLower(strings.Join(fields, ""))
}
