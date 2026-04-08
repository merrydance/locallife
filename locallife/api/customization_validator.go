package api

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

type customizationSelection struct {
	GroupID  int64
	OptionID int64
}

type customizationGroupMeta struct {
	ID         int64
	Name       string
	SortOrder  int32
	IsRequired bool
	Options    map[int64]customizationOptionMeta
}

type customizationOptionMeta struct {
	ID         int64
	TagID      int64
	TagName    string
	ExtraPrice int64
	SortOrder  int32
}

type customizationGroupJSON struct {
	ID         int64  `json:"id"`
	Name       string `json:"name"`
	SortOrder  int32  `json:"sort_order"`
	IsRequired bool   `json:"is_required"`
	Options    []struct {
		ID         int64  `json:"id"`
		TagID      int64  `json:"tag_id"`
		TagName    string `json:"tag_name"`
		ExtraPrice int64  `json:"extra_price"`
		SortOrder  int32  `json:"sort_order"`
	} `json:"options"`
}

type customizationSummaryPart struct {
	GroupID     int64
	GroupOrder  int32
	OptionID    int64
	OptionOrder int32
	TagName     string
}

func parseCustomizationSelections(customizations map[string]interface{}) ([]customizationSelection, error) {
	if len(customizations) == 0 {
		return nil, nil
	}
	selections := make([]customizationSelection, 0, len(customizations))
	for groupKey, raw := range customizations {
		// allow meta keys
		if len(groupKey) > 5 && groupKey[:5] == "meta_" {
			continue
		}

		groupID, err := strconv.ParseInt(groupKey, 10, 64)
		if err != nil || groupID <= 0 {
			return nil, fmt.Errorf("invalid customization group id: %s", groupKey)
		}
		optionID, err := parseInt64Value(raw)
		if err != nil || optionID <= 0 {
			return nil, fmt.Errorf("invalid customization option id for group %d", groupID)
		}
		selections = append(selections, customizationSelection{GroupID: groupID, OptionID: optionID})
	}
	return selections, nil
}

func buildCustomizationSummary(groups map[int64]customizationGroupMeta, selectedByGroup map[int64]int64) string {
	parts := make([]customizationSummaryPart, 0, len(selectedByGroup))
	for groupID, optionID := range selectedByGroup {
		group, exists := groups[groupID]
		if !exists {
			continue
		}
		option, exists := group.Options[optionID]
		if !exists || option.TagName == "" {
			continue
		}

		parts = append(parts, customizationSummaryPart{
			GroupID:     groupID,
			GroupOrder:  group.SortOrder,
			OptionID:    optionID,
			OptionOrder: option.SortOrder,
			TagName:     option.TagName,
		})
	}

	sort.Slice(parts, func(i, j int) bool {
		if parts[i].GroupOrder != parts[j].GroupOrder {
			return parts[i].GroupOrder < parts[j].GroupOrder
		}
		if parts[i].GroupID != parts[j].GroupID {
			return parts[i].GroupID < parts[j].GroupID
		}
		if parts[i].OptionOrder != parts[j].OptionOrder {
			return parts[i].OptionOrder < parts[j].OptionOrder
		}
		return parts[i].OptionID < parts[j].OptionID
	})

	names := make([]string, 0, len(parts))
	for _, part := range parts {
		names = append(names, part.TagName)
	}

	return strings.Join(names, " / ")
}

func parseInt64Value(raw interface{}) (int64, error) {
	switch v := raw.(type) {
	case float64:
		return int64(v), nil
	case float32:
		return int64(v), nil
	case int64:
		return v, nil
	case int:
		return int64(v), nil
	case json.Number:
		return v.Int64()
	case string:
		return strconv.ParseInt(v, 10, 64)
	default:
		return 0, fmt.Errorf("unsupported type")
	}
}

func (server *Server) loadDishCustomizationMeta(ctx *gin.Context, dishID int64) (map[int64]customizationGroupMeta, error) {
	dish, err := server.store.GetDishWithCustomizations(ctx, dishID)
	if err != nil {
		if isNotFoundError(err) {
			return nil, fmt.Errorf("dish not found")
		}
		return nil, err
	}

	if dish.CustomizationGroups == nil {
		return map[int64]customizationGroupMeta{}, nil
	}

	groupsJSON, err := json.Marshal(dish.CustomizationGroups)
	if err != nil {
		return nil, fmt.Errorf("marshal customization groups: %w", err)
	}

	var groups []customizationGroupJSON
	if err := json.Unmarshal(groupsJSON, &groups); err != nil {
		return nil, fmt.Errorf("unmarshal customization groups: %w", err)
	}

	meta := make(map[int64]customizationGroupMeta, len(groups))
	for _, g := range groups {
		options := make(map[int64]customizationOptionMeta, len(g.Options))
		for _, o := range g.Options {
			options[o.ID] = customizationOptionMeta{
				ID:         o.ID,
				TagID:      o.TagID,
				TagName:    o.TagName,
				ExtraPrice: o.ExtraPrice,
				SortOrder:  o.SortOrder,
			}
		}
		meta[g.ID] = customizationGroupMeta{
			ID:         g.ID,
			Name:       g.Name,
			SortOrder:  g.SortOrder,
			IsRequired: g.IsRequired,
			Options:    options,
		}
	}

	return meta, nil
}

func (server *Server) normalizeDishCustomizations(ctx *gin.Context, dishID int64, customizations map[string]interface{}) ([]orderCustomizationItem, int64, map[string]interface{}, error) {
	groups, err := server.loadDishCustomizationMeta(ctx, dishID)
	if err != nil {
		return nil, 0, nil, err
	}

	selections, err := parseCustomizationSelections(customizations)
	if err != nil {
		return nil, 0, nil, err
	}
	if len(customizations) > 0 && len(selections) == 0 {
		return nil, 0, nil, fmt.Errorf("customizations must include at least one valid selection")
	}

	selectedByGroup := make(map[int64]int64, len(selections))
	for _, s := range selections {
		if _, exists := selectedByGroup[s.GroupID]; exists {
			return nil, 0, nil, fmt.Errorf("duplicate customization group %d", s.GroupID)
		}
		selectedByGroup[s.GroupID] = s.OptionID
	}

	for _, group := range groups {
		if group.IsRequired {
			if _, exists := selectedByGroup[group.ID]; !exists {
				return nil, 0, nil, fmt.Errorf("missing required customization group %s", group.Name)
			}
		}
	}

	normalized := make(map[string]interface{}, len(selectedByGroup))
	items := make([]orderCustomizationItem, 0, len(selectedByGroup))
	var extraPrice int64

	for groupID, optionID := range selectedByGroup {
		group, exists := groups[groupID]
		if !exists {
			return nil, 0, nil, fmt.Errorf("customization group %d not found", groupID)
		}
		option, exists := group.Options[optionID]
		if !exists {
			return nil, 0, nil, fmt.Errorf("customization option %d not found in group %s", optionID, group.Name)
		}
		normalized[strconv.FormatInt(groupID, 10)] = optionID
		extraPrice += option.ExtraPrice
		items = append(items, orderCustomizationItem{
			GroupID:    groupID,
			OptionID:   optionID,
			TagID:      option.TagID,
			Name:       group.Name,
			Value:      option.TagName,
			ExtraPrice: option.ExtraPrice,
		})
	}

	if summary := buildCustomizationSummary(groups, selectedByGroup); summary != "" {
		normalized["meta_specs"] = summary
	}

	return items, extraPrice, normalized, nil
}
