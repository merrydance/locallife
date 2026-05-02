package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	ospcontracts "github.com/merrydance/locallife/wechat/ordinaryserviceprovider/contracts"
	"github.com/rs/zerolog/log"
)

const (
	applymentCatalogPageSize = 200
	applymentCatalogTTL      = 24 * time.Hour
	applymentOtherBankName   = "其他银行"
)

type applymentBankOption struct {
	BankAlias       string `json:"bank_alias"`
	BankAliasCode   string `json:"bank_alias_code"`
	AccountBank     string `json:"account_bank"`
	AccountBankCode int64  `json:"account_bank_code"`
	NeedBankBranch  bool   `json:"need_bank_branch"`
}

type applymentProvinceOption struct {
	ProvinceName string `json:"province_name"`
	ProvinceCode int    `json:"province_code"`
}

type applymentCityOption struct {
	CityName string `json:"city_name"`
	CityCode int    `json:"city_code"`
}

type applymentBranchOption struct {
	BankBranchName string `json:"bank_branch_name"`
	BankBranchID   string `json:"bank_branch_id"`
}

type applymentBankListResponse struct {
	Banks       []applymentBankOption `json:"banks"`
	Total       int                   `json:"total"`
	RefreshedAt time.Time             `json:"refreshed_at"`
}

type applymentBankSearchResponse struct {
	Matches     []applymentBankOption `json:"matches"`
	Total       int                   `json:"total"`
	RefreshedAt time.Time             `json:"refreshed_at"`
}

type applymentProvinceListResponse struct {
	Provinces   []applymentProvinceOption `json:"provinces"`
	Total       int                       `json:"total"`
	RefreshedAt time.Time                 `json:"refreshed_at"`
}

type applymentCityListResponse struct {
	Cities      []applymentCityOption `json:"cities"`
	Total       int                   `json:"total"`
	RefreshedAt time.Time             `json:"refreshed_at"`
}

type applymentBranchListResponse struct {
	Branches        []applymentBranchOption `json:"branches"`
	Total           int                     `json:"total"`
	AccountBank     string                  `json:"account_bank"`
	AccountBankCode int64                   `json:"account_bank_code"`
	BankAlias       string                  `json:"bank_alias"`
	BankAliasCode   string                  `json:"bank_alias_code"`
	RefreshedAt     time.Time               `json:"refreshed_at"`
}

type applymentBankQuery struct {
	AccountType string `form:"account_type" binding:"required,oneof=ACCOUNT_TYPE_BUSINESS ACCOUNT_TYPE_PRIVATE"`
}

type applymentBankSearchQuery struct {
	AccountType   string `form:"account_type" binding:"required,oneof=ACCOUNT_TYPE_BUSINESS ACCOUNT_TYPE_PRIVATE"`
	AccountNumber string `form:"account_number" binding:"required"`
}

type applymentBranchQuery struct {
	CityCode int `form:"city_code" binding:"required,min=1"`
}

type applymentCatalogCache struct {
	mu        sync.RWMutex
	banks     map[string]cachedBankEntry
	provinces cachedProvinceEntry
	cities    map[int]cachedCityEntry
	branches  map[string]cachedBranchEntry
}

type cachedBankEntry struct {
	items     []applymentBankOption
	refreshed time.Time
	expiresAt time.Time
}

type cachedProvinceEntry struct {
	items     []applymentProvinceOption
	refreshed time.Time
	expiresAt time.Time
}

type cachedCityEntry struct {
	items     []applymentCityOption
	refreshed time.Time
	expiresAt time.Time
}

type cachedBranchEntry struct {
	items           []applymentBranchOption
	accountBank     string
	accountBankCode int64
	bankAlias       string
	bankAliasCode   string
	refreshed       time.Time
	expiresAt       time.Time
}

func newApplymentCatalogCache() *applymentCatalogCache {
	return &applymentCatalogCache{
		banks:    make(map[string]cachedBankEntry),
		cities:   make(map[int]cachedCityEntry),
		branches: make(map[string]cachedBranchEntry),
	}
}

func (server *Server) getApplymentCatalogCache() *applymentCatalogCache {
	server.applymentCatalogCacheMu.Lock()
	defer server.applymentCatalogCacheMu.Unlock()

	if server.applymentCatalogCache == nil {
		server.applymentCatalogCache = newApplymentCatalogCache()
	}

	return server.applymentCatalogCache
}

// @Summary 查询进件银行列表
// @Description 查询微信普通服务商支持的对公或对私开户银行列表，返回后端缓存快照
// @Tags 开户
// @Accept json
// @Produce json
// @Param account_type query string true "账户类型" Enums(ACCOUNT_TYPE_BUSINESS,ACCOUNT_TYPE_PRIVATE)
// @Success 200 {object} applymentBankListResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 503 {object} ErrorResponse
// @Router /v1/merchant/applyment/banks [get]
// @Security BearerAuth
func (server *Server) listApplymentBanks(ctx *gin.Context) {
	if server.ordinarySPClient == nil {
		err := fmt.Errorf("ordinary service provider client not configured")
		ctx.JSON(http.StatusServiceUnavailable, loggedServerError(ctx, err, "普通服务商开户银行目录暂不可用，请联系平台管理员检查微信支付普通服务商配置后重试", "list applyment banks ordinary service provider client not configured"))
		return
	}

	var req applymentBankQuery
	if err := ctx.ShouldBindQuery(&req); err != nil {
		respondApplymentCatalogClientError(ctx, err, "请选择开户账户类型后再查询银行目录", "list_applyment_banks")
		return
	}

	banks, refreshedAt, err := server.loadApplymentBanks(ctx.Request.Context(), req.AccountType)
	if err != nil {
		respondApplymentCatalogProviderError(ctx, err, "普通服务商开户银行目录查询失败，请稍后重试；如持续失败请联系平台管理员检查微信支付银行目录服务日志", "list_applyment_banks")
		return
	}

	ctx.JSON(http.StatusOK, applymentBankListResponse{
		Banks:       banks,
		Total:       len(banks),
		RefreshedAt: refreshedAt,
	})
}

// @Summary 识别对私银行卡开户银行
// @Description 根据个人银行卡号识别开户银行候选，仅适用于对私账户
// @Tags 开户
// @Accept json
// @Produce json
// @Param account_type query string true "账户类型" Enums(ACCOUNT_TYPE_BUSINESS,ACCOUNT_TYPE_PRIVATE)
// @Param account_number query string true "银行卡号"
// @Success 200 {object} applymentBankSearchResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 503 {object} ErrorResponse
// @Router /v1/merchant/applyment/banks/search-by-bank-account [get]
// @Security BearerAuth
func (server *Server) searchApplymentBanksByAccount(ctx *gin.Context) {
	if server.ordinarySPClient == nil {
		err := fmt.Errorf("ordinary service provider client not configured")
		ctx.JSON(http.StatusServiceUnavailable, loggedServerError(ctx, err, "普通服务商开户银行识别暂不可用，请联系平台管理员检查微信支付普通服务商配置后重试", "search applyment banks by account ordinary service provider client not configured"))
		return
	}

	var req applymentBankSearchQuery
	if err := ctx.ShouldBindQuery(&req); err != nil {
		respondApplymentCatalogClientError(ctx, err, "请填写开户账户类型和银行卡号后再识别开户银行", "search_applyment_banks_by_account")
		return
	}

	accountNumber := strings.TrimSpace(req.AccountNumber)
	if accountNumber == "" {
		respondApplymentCatalogClientError(ctx, errors.New("account_number is required"), "请填写开户账户类型和银行卡号后再识别开户银行", "search_applyment_banks_by_account")
		return
	}

	if req.AccountType != "ACCOUNT_TYPE_PRIVATE" {
		ctx.JSON(http.StatusOK, applymentBankSearchResponse{
			Matches:     []applymentBankOption{},
			Total:       0,
			RefreshedAt: time.Now(),
		})
		return
	}

	banks, refreshedAt, err := server.loadApplymentBanks(ctx.Request.Context(), req.AccountType)
	if err != nil {
		respondApplymentCatalogProviderError(ctx, err, "普通服务商开户银行识别失败，请稍后重试；如持续失败请联系平台管理员检查微信支付银行识别服务日志", "search_applyment_banks_by_account")
		return
	}

	ctx.JSON(http.StatusOK, applymentBankSearchResponse{
		Matches:     banks,
		Total:       len(banks),
		RefreshedAt: refreshedAt,
	})
}

// @Summary 查询开户省份列表
// @Description 查询微信普通服务商支行检索所需的省份列表
// @Tags 开户
// @Accept json
// @Produce json
// @Success 200 {object} applymentProvinceListResponse
// @Failure 401 {object} ErrorResponse
// @Failure 503 {object} ErrorResponse
// @Router /v1/merchant/applyment/areas/provinces [get]
// @Security BearerAuth
func (server *Server) listApplymentProvinces(ctx *gin.Context) {
	if server.ordinarySPClient == nil {
		err := fmt.Errorf("ordinary service provider client not configured")
		ctx.JSON(http.StatusServiceUnavailable, loggedServerError(ctx, err, "普通服务商开户省市目录暂不可用，请联系平台管理员检查微信支付普通服务商配置后重试", "list applyment provinces ordinary service provider client not configured"))
		return
	}

	provinces, refreshedAt, err := server.loadApplymentProvinces(ctx.Request.Context())
	if err != nil {
		respondApplymentCatalogProviderError(ctx, err, "普通服务商开户省份目录查询失败，请稍后重试；如持续失败请联系平台管理员检查微信支付省市目录服务日志", "list_applyment_provinces")
		return
	}

	ctx.JSON(http.StatusOK, applymentProvinceListResponse{
		Provinces:   provinces,
		Total:       len(provinces),
		RefreshedAt: refreshedAt,
	})
}

// @Summary 查询开户城市列表
// @Description 根据省份编码查询微信普通服务商支行检索所需的城市列表
// @Tags 开户
// @Accept json
// @Produce json
// @Param province_code path int true "省份编码"
// @Success 200 {object} applymentCityListResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 503 {object} ErrorResponse
// @Router /v1/merchant/applyment/areas/provinces/{province_code}/cities [get]
// @Security BearerAuth
func (server *Server) listApplymentCities(ctx *gin.Context) {
	if server.ordinarySPClient == nil {
		err := fmt.Errorf("ordinary service provider client not configured")
		ctx.JSON(http.StatusServiceUnavailable, loggedServerError(ctx, err, "普通服务商开户城市目录暂不可用，请联系平台管理员检查微信支付普通服务商配置后重试", "list applyment cities ordinary service provider client not configured"))
		return
	}

	provinceCode, err := strconv.Atoi(ctx.Param("province_code"))
	if err != nil || provinceCode <= 0 {
		respondApplymentCatalogClientError(ctx, errors.New("invalid province_code"), "省份编码无效，请返回省份列表重新选择", "list_applyment_cities")
		return
	}

	cities, refreshedAt, err := server.loadApplymentCities(ctx.Request.Context(), provinceCode)
	if err != nil {
		respondApplymentCatalogProviderError(ctx, err, "普通服务商开户城市目录查询失败，请稍后重试；如持续失败请联系平台管理员检查微信支付城市目录服务日志", "list_applyment_cities")
		return
	}

	ctx.JSON(http.StatusOK, applymentCityListResponse{
		Cities:      cities,
		Total:       len(cities),
		RefreshedAt: refreshedAt,
	})
}

// @Summary 查询开户支行列表
// @Description 根据银行别名编码和城市编码查询微信普通服务商支行列表
// @Tags 开户
// @Accept json
// @Produce json
// @Param bank_alias_code path string true "银行别名编码"
// @Param city_code query int true "城市编码"
// @Success 200 {object} applymentBranchListResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 503 {object} ErrorResponse
// @Router /v1/merchant/applyment/banks/{bank_alias_code}/branches [get]
// @Security BearerAuth
func (server *Server) listApplymentBankBranches(ctx *gin.Context) {
	if server.ordinarySPClient == nil {
		err := fmt.Errorf("ordinary service provider client not configured")
		ctx.JSON(http.StatusServiceUnavailable, loggedServerError(ctx, err, "普通服务商开户支行目录暂不可用，请联系平台管理员检查微信支付普通服务商配置后重试", "list applyment bank branches ordinary service provider client not configured"))
		return
	}

	bankAliasCode := strings.TrimSpace(ctx.Param("bank_alias_code"))
	if bankAliasCode == "" {
		respondApplymentCatalogClientError(ctx, errors.New("bank_alias_code is required"), "银行别名编码缺失，请返回银行列表重新选择", "list_applyment_bank_branches")
		return
	}

	var req applymentBranchQuery
	if err := ctx.ShouldBindQuery(&req); err != nil {
		respondApplymentCatalogClientError(ctx, err, "城市编码无效，请返回城市列表重新选择后再查询支行", "list_applyment_bank_branches")
		return
	}

	branches, refreshedAt, err := server.loadApplymentBranches(ctx.Request.Context(), bankAliasCode, req.CityCode)
	if err != nil {
		respondApplymentCatalogProviderError(ctx, err, "普通服务商开户支行目录查询失败，请稍后重试；如持续失败请联系平台管理员检查微信支付支行目录服务日志", "list_applyment_bank_branches")
		return
	}

	ctx.JSON(http.StatusOK, applymentBranchListResponse{
		Branches:        branches.items,
		Total:           len(branches.items),
		AccountBank:     branches.accountBank,
		AccountBankCode: branches.accountBankCode,
		BankAlias:       branches.bankAlias,
		BankAliasCode:   branches.bankAliasCode,
		RefreshedAt:     refreshedAt,
	})
}

func respondApplymentCatalogClientError(ctx *gin.Context, err error, publicMessage string, operation string) {
	_ = ctx.Error(err)
	log.Warn().
		Err(err).
		Str("request_id", GetRequestID(ctx)).
		Str("operation", operation).
		Str("path", ctx.Request.URL.Path).
		Msg("applyment catalog request rejected")
	ctx.JSON(http.StatusBadRequest, errorResponse(errors.New(publicMessage)))
}

func respondApplymentCatalogProviderError(ctx *gin.Context, err error, publicMessage string, operation string) {
	ctx.JSON(http.StatusBadGateway, loggedServerError(ctx, err, publicMessage, operation+" ordinary service provider catalog request failed"))
}

func (server *Server) loadApplymentBanks(ctx context.Context, accountType string) ([]applymentBankOption, time.Time, error) {
	cache := server.getApplymentCatalogCache()
	now := time.Now()

	cache.mu.RLock()
	entry, ok := cache.banks[accountType]
	cache.mu.RUnlock()
	if ok && now.Before(entry.expiresAt) {
		return entry.items, entry.refreshed, nil
	}

	var (
		fetch func(context.Context, int, int) (*ospcontracts.CapitalBankListResponse, error)
		items []applymentBankOption
	)

	if accountType == "ACCOUNT_TYPE_PRIVATE" {
		fetch = server.ordinarySPClient.ListPersonalBankingBanks
	} else {
		fetch = server.ordinarySPClient.ListCorporateBankingBanks
	}

	for offset := 0; ; {
		resp, err := fetch(ctx, offset, applymentCatalogPageSize)
		if err != nil {
			return nil, time.Time{}, fmt.Errorf("load applyment banks: %w", err)
		}
		for _, bank := range resp.Data {
			option := mapCapitalBankOption(bank)
			if shouldExcludeApplymentBankOption(option) {
				continue
			}
			items = append(items, option)
		}
		count := int(resp.Count)
		if len(resp.Data) == 0 || int64(offset)+resp.Count >= resp.TotalCount || count <= 0 {
			break
		}
		offset += count
	}

	sort.SliceStable(items, func(i, j int) bool {
		return items[i].BankAlias < items[j].BankAlias
	})

	entry = cachedBankEntry{items: items, refreshed: now, expiresAt: now.Add(applymentCatalogTTL)}
	cache.mu.Lock()
	cache.banks[accountType] = entry
	cache.mu.Unlock()

	return entry.items, entry.refreshed, nil
}

func (server *Server) loadApplymentProvinces(ctx context.Context) ([]applymentProvinceOption, time.Time, error) {
	cache := server.getApplymentCatalogCache()
	now := time.Now()

	cache.mu.RLock()
	entry := cache.provinces
	cache.mu.RUnlock()
	if now.Before(entry.expiresAt) {
		return entry.items, entry.refreshed, nil
	}

	resp, err := server.ordinarySPClient.ListProvinceAreas(ctx)
	if err != nil {
		return nil, time.Time{}, fmt.Errorf("load applyment provinces: %w", err)
	}

	items := make([]applymentProvinceOption, 0, len(resp.Data))
	for _, province := range resp.Data {
		items = append(items, applymentProvinceOption{
			ProvinceName: province.ProvinceName,
			ProvinceCode: province.ProvinceCode,
		})
	}

	sort.SliceStable(items, func(i, j int) bool {
		return items[i].ProvinceCode < items[j].ProvinceCode
	})

	entry = cachedProvinceEntry{items: items, refreshed: now, expiresAt: now.Add(applymentCatalogTTL)}
	cache.mu.Lock()
	cache.provinces = entry
	cache.mu.Unlock()

	return entry.items, entry.refreshed, nil
}

func (server *Server) loadApplymentCities(ctx context.Context, provinceCode int) ([]applymentCityOption, time.Time, error) {
	cache := server.getApplymentCatalogCache()
	now := time.Now()

	cache.mu.RLock()
	entry, ok := cache.cities[provinceCode]
	cache.mu.RUnlock()
	if ok && now.Before(entry.expiresAt) {
		return entry.items, entry.refreshed, nil
	}

	resp, err := server.ordinarySPClient.ListCityAreas(ctx, provinceCode)
	if err != nil {
		return nil, time.Time{}, fmt.Errorf("load applyment cities: %w", err)
	}

	items := make([]applymentCityOption, 0, len(resp.Data))
	for _, city := range resp.Data {
		items = append(items, applymentCityOption{
			CityName: city.CityName,
			CityCode: city.CityCode,
		})
	}

	sort.SliceStable(items, func(i, j int) bool {
		return items[i].CityCode < items[j].CityCode
	})

	entry = cachedCityEntry{items: items, refreshed: now, expiresAt: now.Add(applymentCatalogTTL)}
	cache.mu.Lock()
	cache.cities[provinceCode] = entry
	cache.mu.Unlock()

	return entry.items, entry.refreshed, nil
}

func (server *Server) loadApplymentBranches(ctx context.Context, bankAliasCode string, cityCode int) (cachedBranchEntry, time.Time, error) {
	cache := server.getApplymentCatalogCache()
	now := time.Now()
	cacheKey := fmt.Sprintf("%s:%d", bankAliasCode, cityCode)

	cache.mu.RLock()
	entry, ok := cache.branches[cacheKey]
	cache.mu.RUnlock()
	if ok && now.Before(entry.expiresAt) {
		return entry, entry.refreshed, nil
	}

	items := make([]applymentBranchOption, 0)
	var accountBank string
	var accountBankCode int64
	var bankAlias string

	for offset := 0; ; {
		resp, err := server.ordinarySPClient.ListBankBranches(ctx, bankAliasCode, cityCode, offset, applymentCatalogPageSize)
		if err != nil {
			return cachedBranchEntry{}, time.Time{}, fmt.Errorf("load applyment branches: %w", err)
		}
		accountBank = resp.AccountBank
		accountBankCode = resp.AccountBankCode
		bankAlias = resp.BankAlias
		for _, branch := range resp.Data {
			items = append(items, applymentBranchOption{
				BankBranchName: branch.BankBranchName,
				BankBranchID:   branch.BankBranchID,
			})
		}
		count := int(resp.Count)
		if len(resp.Data) == 0 || int64(offset)+resp.Count >= resp.TotalCount || count <= 0 {
			break
		}
		offset += count
	}

	sort.SliceStable(items, func(i, j int) bool {
		return items[i].BankBranchName < items[j].BankBranchName
	})

	entry = cachedBranchEntry{
		items:           items,
		accountBank:     accountBank,
		accountBankCode: accountBankCode,
		bankAlias:       bankAlias,
		bankAliasCode:   bankAliasCode,
		refreshed:       now,
		expiresAt:       now.Add(applymentCatalogTTL),
	}
	cache.mu.Lock()
	cache.branches[cacheKey] = entry
	cache.mu.Unlock()

	return entry, entry.refreshed, nil
}

func mapCapitalBankOption(bank ospcontracts.CapitalBank) applymentBankOption {
	return applymentBankOption{
		BankAlias:       bank.BankAlias,
		BankAliasCode:   bank.BankAliasCode,
		AccountBank:     bank.AccountBank,
		AccountBankCode: bank.AccountBankCode,
		NeedBankBranch:  bank.NeedBankBranch,
	}
}

func shouldExcludeApplymentBankOption(bank applymentBankOption) bool {
	return strings.TrimSpace(bank.AccountBank) == applymentOtherBankName
}
