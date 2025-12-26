package maps

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
)

const (
	districtListURL     = "/ws/district/v1/list"
	districtChildrenURL = "/ws/district/v1/getchildren"
)

// District 行政区划（腾讯地图 district API）
// 注意：字段以腾讯 WebService API 返回为准，此处只取项目需要的部分。
type District struct {
	ID       string
	Name     string
	FullName string
	Location *Location
}

type districtAPIItem struct {
	ID       json.RawMessage `json:"id"`
	Name     string          `json:"name"`
	FullName string          `json:"fullname"`
	Location *struct {
		Lat float64 `json:"lat"`
		Lng float64 `json:"lng"`
	} `json:"location"`
}

func (it districtAPIItem) idString() (string, error) {
	if len(it.ID) == 0 {
		return "", fmt.Errorf("missing id")
	}

	// id 可能是数字或字符串
	var asString string
	if err := json.Unmarshal(it.ID, &asString); err == nil {
		if asString == "" {
			return "", fmt.Errorf("empty id")
		}
		return asString, nil
	}

	var asNumber json.Number
	if err := json.Unmarshal(it.ID, &asNumber); err == nil {
		// 避免科学计数法，保持原样（一般是 6 位 adcode）
		return asNumber.String(), nil
	}

	var asInt int64
	if err := json.Unmarshal(it.ID, &asInt); err == nil {
		return strconv.FormatInt(asInt, 10), nil
	}

	return "", fmt.Errorf("unsupported id format: %s", string(it.ID))
}

func (it districtAPIItem) toDistrict() (District, error) {
	id, err := it.idString()
	if err != nil {
		return District{}, err
	}

	var loc *Location
	if it.Location != nil {
		loc = &Location{Lat: it.Location.Lat, Lng: it.Location.Lng}
	}

	return District{
		ID:       id,
		Name:     it.Name,
		FullName: it.FullName,
		Location: loc,
	}, nil
}

func decodeDistrictItems(raw []byte) ([][]districtAPIItem, error) {
	// list: result 通常为二维数组
	var twoD [][]districtAPIItem
	if err := json.Unmarshal(raw, &twoD); err == nil {
		return twoD, nil
	}

	// getchildren: result 通常为一维数组
	var oneD []districtAPIItem
	if err := json.Unmarshal(raw, &oneD); err == nil {
		return [][]districtAPIItem{oneD}, nil
	}

	return nil, fmt.Errorf("unexpected district result shape")
}

// ListProvinces 获取省级行政区划列表。
func (c *TencentMapClient) ListProvinces(ctx context.Context) ([]District, error) {
	params := url.Values{}
	params.Set("key", c.key)

	reqURL := baseURL + districtListURL + "?" + params.Encode()

	body, err := c.doRequest(ctx, reqURL)
	if err != nil {
		return nil, err
	}

	levels, err := decodeDistrictItems(body)
	if err != nil {
		return nil, err
	}

	if len(levels) == 0 {
		return nil, nil
	}

	// ListProvinces 只返回第一级（省）
	items := levels[0]
	out := make([]District, 0, len(items))
	for _, it := range items {
		d, err := it.toDistrict()
		if err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, nil
}

// ListAllDistricts 获取全量行政区划列表（省、市、区）。
func (c *TencentMapClient) ListAllDistricts(ctx context.Context) ([][]District, error) {
	params := url.Values{}
	params.Set("key", c.key)

	reqURL := baseURL + districtListURL + "?" + params.Encode()

	body, err := c.doRequest(ctx, reqURL)
	if err != nil {
		return nil, err
	}

	levels, err := decodeDistrictItems(body)
	if err != nil {
		return nil, err
	}

	out := make([][]District, len(levels))
	for i, items := range levels {
		out[i] = make([]District, 0, len(items))
		for _, it := range items {
			d, err := it.toDistrict()
			if err != nil {
				return nil, err
			}
			out[i] = append(out[i], d)
		}
	}
	return out, nil
}

// GetChildren 获取指定行政区划的子级列表。
func (c *TencentMapClient) GetChildren(ctx context.Context, id string) ([]District, error) {
	params := url.Values{}
	params.Set("id", id)
	params.Set("key", c.key)

	reqURL := baseURL + districtChildrenURL + "?" + params.Encode()

	body, err := c.doRequest(ctx, reqURL)
	if err != nil {
		return nil, err
	}

	levels, err := decodeDistrictItems(body)
	if err != nil {
		return nil, err
	}

	if len(levels) == 0 {
		return nil, nil
	}

	items := levels[0]
	out := make([]District, 0, len(items))
	for _, it := range items {
		d, err := it.toDistrict()
		if err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, nil
}
