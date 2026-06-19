package api

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	db "github.com/merrydance/locallife/db/sqlc"
)

var errInvalidCustomizationTag = errors.New("tag is not an active customization tag")

func buildCustomizationGroupInputs(requestGroups []customizationGroupInput) ([]db.CustomizationGroupInput, error) {
	groups := make([]db.CustomizationGroupInput, 0, len(requestGroups))
	for _, g := range requestGroups {
		groupName := strings.TrimSpace(g.Name)
		if groupName == "" {
			return nil, errors.New("customization group name is required")
		}
		if len(g.Options) == 0 {
			return nil, errors.New("customization group requires at least one option")
		}

		options := make([]db.CustomizationOptionInput, 0, len(g.Options))
		optionNames := make(map[string]struct{}, len(g.Options))
		for _, o := range g.Options {
			optionName := strings.TrimSpace(o.Name)
			if o.TagID <= 0 && optionName == "" {
				return nil, errors.New("customization option requires tag_id or name")
			}
			optionKey := optionName
			if optionKey == "" {
				optionKey = fmt.Sprintf("tag_id:%d", o.TagID)
			}
			if _, exists := optionNames[optionKey]; exists {
				return nil, fmt.Errorf("duplicate customization option in group %q", groupName)
			}
			optionNames[optionKey] = struct{}{}

			options = append(options, db.CustomizationOptionInput{
				TagID:      o.TagID,
				Name:       optionName,
				ExtraPrice: o.ExtraPrice,
				SortOrder:  o.SortOrder,
			})
		}

		groups = append(groups, db.CustomizationGroupInput{
			Name:       groupName,
			IsRequired: g.IsRequired,
			SortOrder:  g.SortOrder,
			Options:    options,
		})
	}

	return groups, nil
}

func (server *Server) loadExplicitCustomizationTagNames(ctx *gin.Context, groups []db.CustomizationGroupInput) (map[int64]string, int64, error) {
	tagNameMap := make(map[int64]string)
	for _, g := range groups {
		for _, o := range g.Options {
			if o.TagID <= 0 {
				continue
			}
			if _, exists := tagNameMap[o.TagID]; exists {
				continue
			}

			tag, err := server.store.GetTag(ctx, o.TagID)
			if err != nil {
				return nil, o.TagID, err
			}
			if tag.Type != db.TagTypeCustomization || tag.Status != db.TagStatusActive {
				return nil, o.TagID, fmt.Errorf("%w: %d", errInvalidCustomizationTag, o.TagID)
			}
			tagNameMap[o.TagID] = tag.Name
		}
	}

	return tagNameMap, 0, nil
}

func mergeCustomizationTagNames(target map[int64]string, source map[int64]string) {
	for tagID, name := range source {
		target[tagID] = name
	}
}

func respondCustomizationTagLookupError(ctx *gin.Context, tagID int64, err error) {
	if isNotFoundError(err) {
		ctx.JSON(http.StatusBadRequest, errorResponse(fmt.Errorf("tag %d not found", tagID)))
		return
	}
	if errors.Is(err, errInvalidCustomizationTag) {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}
	ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("get tag %d: %w", tagID, err)))
}

func respondDishCustomizationTxError(ctx *gin.Context, operation string, err error) {
	if errors.Is(err, db.ErrCustomizationTagUnavailable) || errors.Is(err, db.ErrDuplicateCustomizationOption) {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}
	ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("%s: %w", operation, err)))
}

func buildCustomizationGroupsForDishResponse(groups []db.DishCustomizationGroupWithOptions, tagNameMap map[int64]string) []customizationGroup {
	resultGroups := make([]customizationGroup, 0, len(groups))
	for _, g := range groups {
		options := make([]customizationOption, 0, len(g.Options))
		for _, o := range g.Options {
			options = append(options, customizationOption{
				ID:         o.ID,
				TagID:      o.TagID,
				TagName:    tagNameMap[o.TagID],
				ExtraPrice: o.ExtraPrice,
				SortOrder:  o.SortOrder,
			})
		}
		resultGroups = append(resultGroups, customizationGroup{
			ID:         g.Group.ID,
			Name:       g.Group.Name,
			IsRequired: g.Group.IsRequired,
			SortOrder:  g.Group.SortOrder,
			Options:    options,
		})
	}

	return resultGroups
}

func buildDishCustomizationsResponseGroups(groups []db.DishCustomizationGroupWithOptions, tagNameMap map[int64]string) []customizationGroupResponse {
	resultGroups := make([]customizationGroupResponse, 0, len(groups))
	for _, g := range groups {
		options := make([]customizationOptionResponse, 0, len(g.Options))
		for _, o := range g.Options {
			options = append(options, customizationOptionResponse{
				ID:         o.ID,
				TagID:      o.TagID,
				TagName:    tagNameMap[o.TagID],
				ExtraPrice: o.ExtraPrice,
				SortOrder:  o.SortOrder,
			})
		}
		resultGroups = append(resultGroups, customizationGroupResponse{
			ID:         g.Group.ID,
			Name:       g.Group.Name,
			IsRequired: g.Group.IsRequired,
			SortOrder:  g.Group.SortOrder,
			Options:    options,
		})
	}

	return resultGroups
}
