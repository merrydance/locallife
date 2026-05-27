package api

import (
	"errors"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

type platformEntityListRequest struct {
	Page  int32 `form:"page" binding:"omitempty,min=1"`
	Limit int32 `form:"limit" binding:"omitempty,min=1,max=100"`
}

type platformRiderIDRequest struct {
	RiderID int64 `uri:"rider_id" binding:"required,min=1"`
}

type platformOperatorIDRequest struct {
	OperatorID int64 `uri:"operator_id" binding:"required,min=1"`
}

type platformMerchantIDRequest struct {
	MerchantID int64 `uri:"merchant_id" binding:"required,min=1"`
}

type platformComplaintCategory struct {
	Category string `json:"category"`
	Count    int64  `json:"count"`
}

type platformOrderStats struct {
	TotalOrders     int32 `json:"total_orders"`
	TotalIncome     int64 `json:"total_income"`
	LastMonthOrders int32 `json:"last_month_orders"`
	LastMonthIncome int64 `json:"last_month_income"`
}

type platformServiceStats struct {
	ComplaintCount      int64                       `json:"complaint_count"`
	ComplaintCategories []platformComplaintCategory `json:"complaint_categories"`
}

type platformRiderCard struct {
	ID             int64  `json:"id"`
	Name           string `json:"name"`
	RegionID       *int64 `json:"region_id,omitempty"`
	RegionName     string `json:"region_name"`
	Status         string `json:"status"`
	Active         bool   `json:"active"`
	ComplaintCount int64  `json:"complaint_count"`
}

type platformRiderListResponse struct {
	Riders  []platformRiderCard `json:"riders"`
	Total   int64               `json:"total"`
	Page    int32               `json:"page"`
	Limit   int32               `json:"limit"`
	HasMore bool                `json:"has_more"`
}

type platformRiderBasicInfo struct {
	Name       string `json:"name"`
	RegionID   *int64 `json:"region_id,omitempty"`
	RegionName string `json:"region_name"`
	Age        *int32 `json:"age,omitempty"`
	Gender     string `json:"gender"`
	Status     string `json:"status"`
	Active     bool   `json:"active"`
}

type platformRiderDetailResponse struct {
	ID                 int64                  `json:"id"`
	Name               string                 `json:"name"`
	Basic              platformRiderBasicInfo `json:"basic"`
	OrderStats         platformOrderStats     `json:"order_stats"`
	Service            platformServiceStats   `json:"service"`
	CreatedAt          time.Time              `json:"created_at"`
	LocationUpdatedAt  *time.Time             `json:"location_updated_at,omitempty"`
	CanPauseAccepting  bool                   `json:"can_pause_accepting"`
	CanResumeAccepting bool                   `json:"can_resume_accepting"`
}

type platformRiderStatusResponse struct {
	ID     int64  `json:"id"`
	Status string `json:"status"`
}

type platformOperatorCard struct {
	ID             int64  `json:"id"`
	Name           string `json:"name"`
	Status         string `json:"status"`
	RegionCount    int64  `json:"region_count"`
	MerchantCount  int64  `json:"merchant_count"`
	ComplaintCount int64  `json:"complaint_count"`
}

type platformOperatorListResponse struct {
	Operators []platformOperatorCard `json:"operators"`
	Total     int64                  `json:"total"`
	Page      int32                  `json:"page"`
	Limit     int32                  `json:"limit"`
	HasMore   bool                   `json:"has_more"`
}

type platformOperatorRegion struct {
	RegionID   int64  `json:"region_id"`
	RegionName string `json:"region_name"`
	Status     string `json:"status"`
}

type platformOperatorDetailResponse struct {
	ID            int64                    `json:"id"`
	Name          string                   `json:"name"`
	ContactName   string                   `json:"contact_name"`
	ContactPhone  string                   `json:"contact_phone"`
	Status        string                   `json:"status"`
	RegionID      int64                    `json:"region_id"`
	RegionName    string                   `json:"region_name"`
	RegionCount   int64                    `json:"region_count"`
	MerchantCount int64                    `json:"merchant_count"`
	Regions       []platformOperatorRegion `json:"regions"`
	OrderStats    platformOrderStats       `json:"order_stats"`
	Service       platformServiceStats     `json:"service"`
	CreatedAt     time.Time                `json:"created_at"`
	CanSuspend    bool                     `json:"can_suspend"`
	CanResume     bool                     `json:"can_resume"`
}

type platformMerchantCard struct {
	ID             int64  `json:"id"`
	Name           string `json:"name"`
	RegionID       int64  `json:"region_id"`
	RegionName     string `json:"region_name"`
	Status         string `json:"status"`
	IsOpen         bool   `json:"is_open"`
	MonthOrders    int32  `json:"month_orders"`
	ComplaintCount int64  `json:"complaint_count"`
}

type platformMerchantListResponse struct {
	Merchants []platformMerchantCard `json:"merchants"`
	Total     int64                  `json:"total"`
	Page      int32                  `json:"page"`
	Limit     int32                  `json:"limit"`
	HasMore   bool                   `json:"has_more"`
}

type platformMerchantBasicInfo struct {
	Name       string `json:"name"`
	Phone      string `json:"phone"`
	Address    string `json:"address"`
	RegionID   int64  `json:"region_id"`
	RegionName string `json:"region_name"`
	Status     string `json:"status"`
	IsOpen     bool   `json:"is_open"`
}

type platformMerchantDetailResponse struct {
	ID         int64                     `json:"id"`
	Name       string                    `json:"name"`
	Basic      platformMerchantBasicInfo `json:"basic"`
	OrderStats platformOrderStats        `json:"order_stats"`
	Service    platformServiceStats      `json:"service"`
	CreatedAt  time.Time                 `json:"created_at"`
	CanSuspend bool                      `json:"can_suspend"`
	CanResume  bool                      `json:"can_resume"`
}

type platformMerchantStatusResponse struct {
	ID     int64  `json:"id"`
	Status string `json:"status"`
}

func normalizePlatformPagination(req *platformEntityListRequest) {
	if req.Page == 0 {
		req.Page = 1
	}
	if req.Limit == 0 {
		req.Limit = 20
	}
}

func platformHasMore(page int32, limit int32, loaded int, total int64) bool {
	return int64(pageOffset(page, limit))+int64(loaded) < total
}

func stringFromPgText(value pgtype.Text) string {
	if value.Valid {
		return value.String
	}
	return ""
}

func activeRiderByPlatformRule(acceptedIn3d bool) bool {
	return acceptedIn3d
}

func platformRegionIDFromPgInt8(regionID pgtype.Int8) *int64 {
	if regionID.Valid {
		value := regionID.Int64
		return &value
	}
	return nil
}

var errPlatformEntityStatusConflict = errors.New("entity status does not allow this operation")
