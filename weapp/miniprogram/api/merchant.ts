/**
 * 商户基础管理接口 (仅保留顾客端搜索和推荐)
 */

import { request } from '../utils/request'
import { normalizePaginatedResult, type PaginatedListResult, type PaginationEnvelope } from './types'
import type { CustomizationGroup } from './dish'

// ==================== 数据类型定义 ====================

/**
 * 商户摘要信息 - 对齐 api.merchantSummary（用于搜索和推荐）
 */
export interface MerchantSummary {
  id: number                                   // 商户ID
  name: string                                 // 商户名称
  address?: string                             // 商户地址
  description?: string                         // 商户描述
  logo_url?: string                            // Logo URL
  cover_image?: string                         // 门头照（列表卡片封面）
  distance?: number                            // 距离（米）
  estimated_delivery_fee?: number              // 预估配送费（分）
  total_orders?: number                        // 总销量（搜索返回）
  monthly_sales?: number                       // 近30天订单量（旧字段）
  region_id?: number                           // 区域ID
  status?: string                              // 商户状态
  is_open?: boolean                            // 是否营业（仅部分接口）
  tags?: string[]                              // 商户标签（仅部分接口）
  system_labels?: string[]                     // 商户系统标签（如：无明厨亮灶）
  created_at?: string                          // 入驻时间（用于判断新店）
  label?: string                               // 推荐 / 热销
}

/** 搜索商户返回项 - 对齐后端 searchMerchantResponse */
export interface SearchMerchantItem {
  id: number
  name: string
  description?: string
  address?: string
  phone?: string
  logo_url: string
  cover_image?: string
  status: string
  is_open?: boolean
  region_id: number
  total_orders?: number
  distance?: number
  estimated_delivery_fee?: number
  tags?: string[]
  system_labels?: string[]
  created_at?: string  // 入驻时间，用于前端判断"新店"
  label?: string       // 推荐 / 热销
}

export interface SearchMerchantsResponse {
  merchants: SearchMerchantItem[]
  total?: number
  page_id?: number
  page_size?: number
}

export interface SearchMerchantsParams {
  keyword?: string
  region_id?: number
  tag_id?: number
  sort_by?: 'distance'
  page_id?: number
  page_size?: number
  user_latitude?: number
  user_longitude?: number
}

export interface MerchantSummaryListResult extends PaginatedListResult<MerchantSummary> {
  merchants: MerchantSummary[]
}

type SearchMerchantsEnvelope = PaginationEnvelope & {
  merchants?: SearchMerchantItem[]
}

function normalizeMerchantSummary(item: SearchMerchantItem): MerchantSummary {
  return {
    id: item.id,
    name: item.name,
    address: item.address || '',
    description: item.description || '',
    logo_url: item.logo_url || '',
    cover_image: item.cover_image || '',
    distance: item.distance,
    estimated_delivery_fee: item.estimated_delivery_fee,
    total_orders: item.total_orders,
    region_id: item.region_id,
    status: item.status,
    is_open: item.is_open,
    tags: item.tags || [],
    system_labels: item.system_labels || [],
    created_at: item.created_at,
    label: item.label
  }
}

// ==================== 顾客端商户接口 ====================

/**
 * 搜索商户 - 基于 /v1/search/merchants
 */
export async function searchMerchantsWithMeta(params: SearchMerchantsParams): Promise<MerchantSummaryListResult> {
  const requestParams: Record<string, unknown> = {
    keyword: params.keyword || '',
    page_id: params.page_id || 1,
    page_size: params.page_size || 20
  }

  if (params.user_latitude !== undefined && params.user_latitude !== null) {
    requestParams.user_latitude = params.user_latitude
  }
  if (params.user_longitude !== undefined && params.user_longitude !== null) {
    requestParams.user_longitude = params.user_longitude
  }

  if (params.region_id !== undefined && params.region_id !== null) {
    requestParams.region_id = params.region_id
  }

  if (params.tag_id !== undefined && params.tag_id !== null) {
    requestParams.tag_id = params.tag_id
  }

  if (params.sort_by) {
    requestParams.sort_by = params.sort_by
  }

  const response = await request<SearchMerchantsEnvelope>({
    url: '/v1/search/merchants',
    method: 'GET',
    data: requestParams,
    useCache: true,
    cacheTTL: 2 * 60 * 1000
  })

  const merchants = (response.merchants || []).map(normalizeMerchantSummary)
  const normalized = normalizePaginatedResult(merchants, response, {
    page: params.page_id || 1,
    pageSize: params.page_size || 20
  })

  return {
    ...normalized,
    merchants
  }
}

export async function searchMerchants(params: SearchMerchantsParams): Promise<MerchantSummary[]> {
  const result = await searchMerchantsWithMeta(params)
  return result.merchants
}

/**
 * 推荐商户请求参数
 */
export interface RecommendMerchantsParams {
  region_id?: number
  user_latitude?: number
  user_longitude?: number
  limit?: number
  page?: number
}

/**
 * 推荐商户结果（包含分页信息）
 */
export interface RecommendMerchantsResult {
  merchants: MerchantSummary[]
  has_more: boolean
  page: number
  total: number
}

export async function getRecommendedMerchantsWithMeta(params?: RecommendMerchantsParams): Promise<MerchantSummaryListResult> {
  return searchMerchantsWithMeta({
    keyword: '',
    region_id: params?.region_id,
    user_latitude: params?.user_latitude,
    user_longitude: params?.user_longitude,
    page_id: params?.page ?? 1,
    page_size: params?.limit ?? 20
  })
}

/**
 * 获取推荐商户
 */
export async function getRecommendedMerchants(params?: RecommendMerchantsParams): Promise<RecommendMerchantsResult> {
  const result = await getRecommendedMerchantsWithMeta(params)
  return {
    merchants: result.merchants,
    has_more: result.hasMore,
    page: result.page,
    total: result.total
  }
}

/**
 * 消费者端的证照/优惠规则定义
 */
export interface PublicDiscountRule {
  id: number
  name: string
  min_order_amount: number
  discount_amount: number
}

export interface PublicVoucher {
  id: number
  name: string
  amount: number
  min_order_amount: number
}

export interface PublicDeliveryPromotion {
  id: number
  name: string
  min_order_amount: number
  discount_amount: number
}

export interface PublicMerchantDetail {
  id: number
  name: string
  description?: string
  logo_url?: string
  cover_image?: string
  phone: string
  address: string
  latitude: number
  longitude: number
  region_id: number
  is_open: boolean
  is_ordering_suspended: boolean
  tags: string[]
  system_labels?: string[]
  monthly_sales: number
  avg_prep_minutes: number
  business_license_image_url?: string
  food_permit_url?: string
  business_hours?: {
    day_of_week: number
    open_time: string
    close_time: string
    is_closed: boolean
  }[]
  discount_rules?: PublicDiscountRule[]
  vouchers?: PublicVoucher[]
  delivery_promotions?: PublicDeliveryPromotion[]
}

/**
 * 获取商户详情（消费者端）
 */
export async function getPublicMerchantDetail(merchantId: number, lite = false): Promise<PublicMerchantDetail> {
  return await request({
    url: `/v1/public/merchants/${merchantId}${lite ? '?lite=true' : ''}`,
    method: 'GET',
    useCache: true,
    cacheTTL: 5 * 60 * 1000
  })
}

/**
 * 菜品分类项
 */
export interface PublicDishCategory {
  id: number
  name: string
  sort_order: number
}

/**
 * 菜品项
 */
export interface PublicDish {
  id: number
  name: string
  description?: string
  price: number
  original_price?: number
  member_price?: number
  image_url?: string
  category_id: number
  category_name: string
  monthly_sales: number
  prepare_time: number
  tags: string[]
  customization_groups?: CustomizationGroup[]
}

export type DishDTO = PublicDish

/**
 * 菜品列表响应
 */
export interface PublicMerchantDishesResponse {
  categories: PublicDishCategory[]
  dishes: PublicDish[]
}

/**
 * 获取商户菜品列表（消费者端）
 */
export async function getPublicMerchantDishes(merchantId: number): Promise<PublicMerchantDishesResponse> {
  return await request({
    url: `/v1/public/merchants/${merchantId}/dishes`,
    method: 'GET',
    useCache: true,
    cacheTTL: 5 * 60 * 1000
  })
}

/**
 * 套餐菜品项
 */
export interface ComboDishItem {
  dish_id: number
  dish_name: string
  quantity: number
}

/**
 * 套餐项
 */
export interface PublicCombo {
  id: number
  name: string
  description?: string
  image_url?: string
  combo_price: number
  original_price: number
  dishes: ComboDishItem[]
  tags?: string[]
  dish_images?: string[]
}

/**
 * 套餐列表响应
 */
export interface PublicMerchantCombosResponse {
  combos: PublicCombo[]
}

/**
 * 获取商户套餐列表（消费者端）
 */
export async function getPublicMerchantCombos(merchantId: number): Promise<PublicMerchantCombosResponse> {
  return await request({
    url: `/v1/public/merchants/${merchantId}/combos`,
    method: 'GET',
    useCache: true,
    cacheTTL: 5 * 60 * 1000
  })
}

/**
 * 查询当前用户是否曾在该商户成功下单（用于展示"再来一单"标识）
 */
export async function getHasUserOrderedFromMerchant(merchantId: number): Promise<boolean> {
  try {
    const result = await request<{ has_ordered: boolean }>({
      url: `/v1/public/merchants/${merchantId}/has-ordered`,
      method: 'GET',
      useCache: true,
      cacheTTL: 10 * 60 * 1000
    })
    return result.has_ordered ?? false
  } catch {
    return false
  }
}

// ==================== 数据适配器 (仅保留顾客端相关) ====================

export class MerchantManagementAdapter {
  static formatDayOfWeek(dayOfWeek: number): string {
    const dayNames = ['周日', '周一', '周二', '周三', '周四', '周五', '周六']
    return dayNames[dayOfWeek] || `星期${dayOfWeek}`
  }

  static isCurrentlyOpen(businessHours: { day_of_week: number, open_time: string, close_time: string, is_closed: boolean }[]): boolean {
    const now = new Date()
    const currentDay = now.getDay()
    const currentTime = now.toTimeString().slice(0, 5)

    const todayHours = businessHours.find((hour) => hour.day_of_week === currentDay)

    if (!todayHours || todayHours.is_closed) {
      return false
    }

    return currentTime >= todayHours.open_time && currentTime <= todayHours.close_time
  }
}

export const getMerchantDishes = getPublicMerchantDishes
export const getMerchants = searchMerchants

// ==================== 商户工作台接口（商户自身管理用） ====================

/**
 * 商户详情响应（商户工作台使用，包含 version）
 */
export interface MerchantOperatorResponse {
  id: number
  owner_user_id: number
  region_id: number
  name: string
  description?: string
  logo_url?: string
  phone: string
  address: string
  latitude?: string
  longitude?: string
  status: string
  is_open: boolean
  version: number
  group_id?: number
  brand_id?: number
  created_at: string
  updated_at: string
}

export interface MerchantOpenStatusResponse {
  is_open: boolean
  auto_close_at?: string
  message: string
}

export interface MerchantBusinessHour {
  id?: number
  day_of_week: number
  day_name?: string
  open_time: string
  close_time: string
  is_closed: boolean
  special_date?: string
}

export interface MerchantBusinessHoursResponse {
  hours: MerchantBusinessHour[]
  auto_open_by_business_hours: boolean
}

export interface UpdateMyMerchantProfileRequest {
  name?: string
  description?: string
  phone?: string
  address?: string
  latitude?: string
  longitude?: string
  logo_asset_id?: number | null
  clear_logo?: boolean
  version: number
}

export interface UpdateMerchantBusinessHoursRequest {
  auto_open_by_business_hours: boolean
  hours: Array<{
    day_of_week: number
    open_time: string
    close_time: string
    is_closed: boolean
    special_date?: string
  }>
}

export type MerchantMembershipScene = 'dine_in' | 'takeout' | 'reservation'

export interface MerchantMembershipSettingsResponse {
  merchant_id: number
  balance_usable_scenes: MerchantMembershipScene[]
  bonus_usable_scenes: MerchantMembershipScene[]
  allow_with_voucher: boolean
  allow_with_discount: boolean
  max_deduction_percent: number
}

export interface UpdateMerchantMembershipSettingsRequest {
  balance_usable_scenes?: MerchantMembershipScene[]
  bonus_usable_scenes?: MerchantMembershipScene[]
  allow_with_voucher?: boolean
  allow_with_discount?: boolean
  max_deduction_percent?: number
}

/**
 * 获取当前用户可访问的全部商户（多门店切换）
 * GET /v1/merchants/my
 */
export function listMyMerchants() {
  return request<MerchantOperatorResponse[]>({
    url: '/v1/merchants/my',
    method: 'GET'
  })
}

/**
 * 获取当前登录商户信息（商户工作台）
 * GET /v1/merchants/me
 */
export function getMyMerchantProfile() {
  return request<MerchantOperatorResponse>({
    url: '/v1/merchants/me',
    method: 'GET'
  })
}

/**
 * 更新当前商户 Logo（商户工作台）
 * PATCH /v1/merchants/me
 * @param logoAssetId 媒体资产 ID
 * @param version 乐观锁版本号
 */
export function updateMyMerchantLogo(logoAssetId: number | null, version: number) {
  const data: Record<string, unknown> = { version }
  if (logoAssetId === null) {
    data.clear_logo = true
  } else if (logoAssetId) {
    data.logo_asset_id = logoAssetId
  }
  return request<MerchantOperatorResponse>({
    url: '/v1/merchants/me',
    method: 'PATCH',
    data
  })
}

/**
 * 更新当前商户资料（商户工作台）
 * PATCH /v1/merchants/me
 */
export function updateMyMerchantProfile(data: UpdateMyMerchantProfileRequest) {
  return request<MerchantOperatorResponse>({
    url: '/v1/merchants/me',
    method: 'PATCH',
    data
  })
}

/**
 * 获取当前商户营业状态
 * GET /v1/merchants/me/status
 */
export function getMyMerchantOpenStatus() {
  return request<MerchantOpenStatusResponse>({
    url: '/v1/merchants/me/status',
    method: 'GET'
  })
}

/**
 * 更新当前商户营业状态
 * PATCH /v1/merchants/me/status
 */
export function updateMyMerchantOpenStatus(isOpen: boolean, autoCloseAt?: string) {
  const data: Record<string, unknown> = { is_open: isOpen }
  if (autoCloseAt) {
    data.auto_close_at = autoCloseAt
  }
  return request<MerchantOpenStatusResponse>({
    url: '/v1/merchants/me/status',
    method: 'PATCH',
    data
  })
}

/**
 * 获取当前商户营业时间
 * GET /v1/merchants/me/business-hours
 */
export function getMyMerchantBusinessHours() {
  return request<MerchantBusinessHoursResponse>({
    url: '/v1/merchants/me/business-hours',
    method: 'GET'
  })
}

/**
 * 更新当前商户营业时间
 * PUT /v1/merchants/me/business-hours
 */
export function updateMyMerchantBusinessHours(data: UpdateMerchantBusinessHoursRequest) {
  return request<MerchantBusinessHoursResponse>({
    url: '/v1/merchants/me/business-hours',
    method: 'PUT',
    data
  })
}

/**
 * 获取当前商户会员设置
 * GET /v1/merchants/me/membership-settings
 */
export function getMyMerchantMembershipSettings() {
  return request<MerchantMembershipSettingsResponse>({
    url: '/v1/merchants/me/membership-settings',
    method: 'GET'
  })
}

/**
 * 更新当前商户会员设置
 * PUT /v1/merchants/me/membership-settings
 */
export function updateMyMerchantMembershipSettings(data: UpdateMerchantMembershipSettingsRequest) {
  return request<MerchantMembershipSettingsResponse>({
    url: '/v1/merchants/me/membership-settings',
    method: 'PUT',
    data
  })
}

export interface MerchantRechargeRuleResponse {
  id: number
  merchant_id: number
  recharge_amount: number
  bonus_amount: number
  is_active: boolean
  valid_from: string
  valid_until: string
  status_code: MerchantTimedRuleStatusCode
  status_label: string
  status_theme: MerchantRuleStatusTheme
  created_at: string
  updated_at?: string
}

export type MerchantRuleStatusTheme = 'success' | 'warning' | 'danger' | 'default'
export type MerchantTimedRuleStatusCode = 'inactive' | 'expired' | 'scheduled' | 'active'

export interface MerchantTimedRuleStatusView {
  label: string
  theme: MerchantRuleStatusTheme
  code: MerchantTimedRuleStatusCode
}

function buildMerchantTimedRuleStatusView(
  rule: Pick<MerchantRechargeRuleResponse | MerchantDiscountRuleResponse, 'is_active' | 'valid_from' | 'valid_until'>,
  now: Date = new Date()
): MerchantTimedRuleStatusView {
  const ruleWithStatus = rule as Partial<Pick<MerchantRechargeRuleResponse, 'status_code' | 'status_label' | 'status_theme'>>

  if (ruleWithStatus.status_code && ruleWithStatus.status_label && ruleWithStatus.status_theme) {
    return {
      label: ruleWithStatus.status_label,
      theme: ruleWithStatus.status_theme,
      code: ruleWithStatus.status_code
    }
  }

  const validUntil = new Date(rule.valid_until)
  const validFrom = new Date(rule.valid_from)

  if (!rule.is_active) {
    return { label: '已停用', theme: 'default', code: 'inactive' }
  }

  if (now > validUntil) {
    return { label: '已过期', theme: 'danger', code: 'expired' }
  }

  if (now < validFrom) {
    return { label: '未开始', theme: 'warning', code: 'scheduled' }
  }

  return { label: '生效中', theme: 'success', code: 'active' }
}

export function buildMerchantRechargeRuleStatusView(
  rule: Pick<MerchantRechargeRuleResponse, 'is_active' | 'valid_from' | 'valid_until'>,
  now: Date = new Date()
): MerchantTimedRuleStatusView {
  return buildMerchantTimedRuleStatusView(rule, now)
}

export interface CreateMerchantRechargeRuleRequest {
  recharge_amount: number
  bonus_amount: number
  valid_from: string
  valid_until: string
}

export interface UpdateMerchantRechargeRuleRequest {
  recharge_amount?: number
  bonus_amount?: number
  is_active?: boolean
  valid_from?: string
  valid_until?: string
}

export interface MerchantMembershipTransaction {
  id: number
  membership_id: number
  type: string
  amount: number
  balance_after: number
  related_order_id?: number
  notes?: string
  created_at: string
}

export interface MerchantMemberSummary {
  user_id: number
  full_name: string
  phone: string
  avatar_url: string
  membership_id: number
  balance: number
  total_recharged: number
  total_consumed: number
  created_at: string
}

export interface MerchantMemberDetail extends MerchantMemberSummary {
  transactions: MerchantMembershipTransaction[]
}

export interface ListMerchantMembersResponse {
  members: MerchantMemberSummary[]
  total: number
  page_id: number
  page_size: number
}

export interface AdjustMerchantMemberBalanceRequest {
  amount: number
  notes: string
}

export interface MerchantDiscountRuleResponse {
  id: number
  merchant_id: number
  name: string
  description?: string
  min_order_amount: number
  discount_amount: number
  can_stack_with_voucher: boolean
  can_stack_with_membership: boolean
  stacking_group?: string
  valid_from: string
  valid_until: string
  is_active: boolean
  status_code: MerchantTimedRuleStatusCode
  status_label: string
  status_theme: MerchantRuleStatusTheme
  created_at: string
}

export function buildMerchantDiscountRuleStatusView(
  rule: Pick<MerchantDiscountRuleResponse, 'is_active' | 'valid_from' | 'valid_until'>,
  now: Date = new Date()
): MerchantTimedRuleStatusView {
  return buildMerchantTimedRuleStatusView(rule, now)
}

export interface ListMerchantDiscountRulesResponse {
  rules: MerchantDiscountRuleResponse[]
  total: number
  page_id: number
  page_size: number
}

export interface CreateMerchantDiscountRuleRequest {
  name: string
  description?: string
  min_order_amount: number
  discount_amount: number
  can_stack_with_voucher?: boolean
  can_stack_with_membership?: boolean
  stacking_group?: string
  valid_from: string
  valid_until: string
}

export interface UpdateMerchantDiscountRuleRequest {
  name?: string
  description?: string
  min_order_amount?: number
  discount_amount?: number
  can_stack_with_voucher?: boolean
  can_stack_with_membership?: boolean
  stacking_group?: string
  valid_from?: string
  valid_until?: string
  is_active?: boolean
}

/**
 * 获取商户充值规则列表
 * GET /v1/merchants/{id}/recharge-rules
 */
export function listMerchantRechargeRules(merchantId: number) {
  return request<MerchantRechargeRuleResponse[]>({
    url: `/v1/merchants/${merchantId}/recharge-rules`,
    method: 'GET'
  })
}

/**
 * 创建商户充值规则
 * POST /v1/merchants/{id}/recharge-rules
 */
export function createMerchantRechargeRule(merchantId: number, data: CreateMerchantRechargeRuleRequest) {
  return request<MerchantRechargeRuleResponse>({
    url: `/v1/merchants/${merchantId}/recharge-rules`,
    method: 'POST',
    data
  })
}

/**
 * 更新商户充值规则
 * PATCH /v1/merchants/{id}/recharge-rules/{rule_id}
 */
export function updateMerchantRechargeRule(merchantId: number, ruleId: number, data: UpdateMerchantRechargeRuleRequest) {
  return request<MerchantRechargeRuleResponse>({
    url: `/v1/merchants/${merchantId}/recharge-rules/${ruleId}`,
    method: 'PATCH',
    data
  })
}

/**
 * 删除商户充值规则
 * DELETE /v1/merchants/{id}/recharge-rules/{rule_id}
 */
export function deleteMerchantRechargeRule(merchantId: number, ruleId: number) {
  return request<{ message?: string }>({
    url: `/v1/merchants/${merchantId}/recharge-rules/${ruleId}`,
    method: 'DELETE'
  })
}

/**
 * 获取商户会员列表
 * GET /v1/merchants/{id}/members
 */
export function listMerchantMembers(merchantId: number, pageId: number = 1, pageSize: number = 20) {
  return request<ListMerchantMembersResponse>({
    url: `/v1/merchants/${merchantId}/members`,
    method: 'GET',
    data: { page_id: pageId, page_size: pageSize }
  })
}

/**
 * 获取商户会员详情
 * GET /v1/merchants/{id}/members/{user_id}
 */
export function getMerchantMemberDetail(merchantId: number, userId: number) {
  return request<MerchantMemberDetail>({
    url: `/v1/merchants/${merchantId}/members/${userId}`,
    method: 'GET'
  })
}

/**
 * 调整商户会员余额
 * POST /v1/merchants/{id}/members/{user_id}/balance
 */
export function adjustMerchantMemberBalance(merchantId: number, userId: number, data: AdjustMerchantMemberBalanceRequest) {
  return request<MerchantMemberSummary>({
    url: `/v1/merchants/${merchantId}/members/${userId}/balance`,
    method: 'POST',
    data
  })
}

/**
 * 获取商户满减规则列表
 * GET /v1/merchants/{id}/discounts
 */
export function listMerchantDiscountRules(merchantId: number, pageId: number = 1, pageSize: number = 20) {
  return request<ListMerchantDiscountRulesResponse>({
    url: `/v1/merchants/${merchantId}/discounts`,
    method: 'GET',
    data: { page_id: pageId, page_size: pageSize }
  })
}

/**
 * 获取单条商户满减规则
 * GET /v1/merchants/{id}/discounts/{rule_id}
 */
export function getMerchantDiscountRule(merchantId: number, ruleId: number) {
  return request<MerchantDiscountRuleResponse>({
    url: `/v1/merchants/${merchantId}/discounts/${ruleId}`,
    method: 'GET'
  })
}

/**
 * 创建商户满减规则
 * POST /v1/merchants/{id}/discounts
 */
export function createMerchantDiscountRule(merchantId: number, data: CreateMerchantDiscountRuleRequest) {
  return request<MerchantDiscountRuleResponse>({
    url: `/v1/merchants/${merchantId}/discounts`,
    method: 'POST',
    data
  })
}

/**
 * 更新商户满减规则
 * PATCH /v1/merchants/{id}/discounts/{rule_id}
 */
export function updateMerchantDiscountRule(merchantId: number, ruleId: number, data: UpdateMerchantDiscountRuleRequest) {
  return request<MerchantDiscountRuleResponse>({
    url: `/v1/merchants/${merchantId}/discounts/${ruleId}`,
    method: 'PATCH',
    data: {
      id: ruleId,
      ...data
    }
  })
}

/**
 * 删除商户满减规则
 * DELETE /v1/merchants/{id}/discounts/{rule_id}
 */
export function deleteMerchantDiscountRule(merchantId: number, ruleId: number) {
  return request<{ message?: string }>({
    url: `/v1/merchants/${merchantId}/discounts/${ruleId}`,
    method: 'DELETE'
  })
}

// ==================== 商户经营类目 ====================

export interface MerchantCategoryTag {
  id: number
  name: string
  type: string
  sort_order: number
}

/**
 * 获取当前商户已选的经营类目标签
 * GET /v1/merchants/me/tags
 */
export function getMyMerchantTags() {
  return request<{ tags: MerchantCategoryTag[] }>({
    url: '/v1/merchants/me/tags',
    method: 'GET'
  })
}

/**
 * 获取平台所有可选的商户类目标签
 * GET /v1/tags?type=merchant
 */
export function getAvailableMerchantTags() {
  return request<{ tags: MerchantCategoryTag[] }>({
    url: '/v1/tags',
    method: 'GET',
    data: { type: 'merchant' }
  })
}

/**
 * 替换当前商户的经营类目标签（最多5个）
 * PUT /v1/merchants/me/tags
 */
export function setMyMerchantTags(tagIds: number[]) {
  return request<{ tags: MerchantCategoryTag[] }>({
    url: '/v1/merchants/me/tags',
    method: 'PUT',
    data: { tag_ids: tagIds }
  })
}
