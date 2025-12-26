/**
 * 个人中心功能接口
 * 基于swagger.json完全重构，包含收藏、历史、评价、会员、通知、申诉等功能
 */

import { request } from '../utils/request'

// ==================== 数据类型定义 ====================

// 收藏相关类型
export interface FavoriteDishResponse {
    id: number
    dish_id: number
    dish_name: string
    dish_image_url?: string
    merchant_id: number
    merchant_name: string
    merchant_logo_url?: string
    price: number
    member_price?: number
    created_at: string
}

export interface FavoriteMerchantResponse {
    id: number
    merchant_id: number
    merchant_name: string
    merchant_logo_url?: string
    monthly_sales: number
    estimated_delivery_fee: number
    tags: string[]
    created_at: string
}

export interface ListFavoriteDishesResponse {
    dishes: FavoriteDishResponse[]
    total: number
    page: number
    page_size: number
}

export interface ListFavoriteMerchantsResponse {
    merchants: FavoriteMerchantResponse[]
    total: number
    page: number
    page_size: number
}

// 浏览历史类型 - 对齐 api.browseHistoryItem
export interface BrowseHistoryItem {
    id: number                    // 浏览记录ID
    target_type: string           // 浏览目标类型：merchant=商户, dish=菜品
    target_id: number             // 目标ID
    name: string                  // 目标名称
    image_url?: string            // 目标图片URL
    last_viewed_at: string        // 最后浏览时间
    view_count: number            // 浏览次数
}

// 对齐 api.listBrowseHistoryResponse
export interface ListBrowseHistoryResponse {
    items: BrowseHistoryItem[]    // 浏览记录列表
    total: number                 // 总数
}

// 评价相关类型 - 对齐 api.reviewResponse
export interface ReviewResponse {
    id: number
    order_id: number
    merchant_id: number
    user_id: number
    content: string
    images: string[]
    is_visible: boolean           // 是否可见
    merchant_reply?: string       // 商户回复
    replied_at?: string           // 回复时间
    created_at: string
}

export interface CreateReviewRequest extends Record<string, unknown> {
    order_id: number
    content: string
    images?: string[]
}

export interface ListReviewsResponse {
    reviews: ReviewResponse[]
    total: number
    page: number
    page_size: number
}

/** 回复评价请求 - 对齐 api.replyReviewRequest */
export interface ReplyReviewRequest extends Record<string, unknown> {
    reply: string                                // 回复内容（1-500字符）
}

// 会员相关类型 - 对齐 api.membershipResponse
export interface MembershipResponse {
    id: number
    user_id: number
    merchant_id: number
    merchant_name: string
    balance: number               // 余额（分）
    total_recharged: number       // 累计充值（分）
    total_consumed: number        // 累计消费（分）
    created_at: string
}

export interface MembershipTransactionResponse {
    id: number
    membership_id: number
    type: 'recharge' | 'consume' | 'refund' | 'points_earn' | 'points_redeem'
    amount: number
    points_change?: number
    description: string
    order_id?: number
    created_at: string
}

/** 充值请求 - 对齐 api.rechargeRequest */
export interface RechargeRequest extends Record<string, unknown> {
    membership_id: number                        // 会员ID
    payment_method: 'wechat' | 'alipay'          // 支付方式
    recharge_amount: number                      // 充值金额（分，最大100万元）
}

export interface ListMembershipsResponse {
    memberships: MembershipResponse[]
    total: number
}

export interface ListMembershipTransactionsResponse {
    transactions: MembershipTransactionResponse[]
    total: number
    page: number
    page_size: number
}

// 通知相关类型 - 对齐 api.notificationResponse
export interface NotificationResponse {
    id: number
    user_id: number
    type: string                  // 通知类型
    title: string
    content: string
    related_type?: string         // 关联类型
    related_id?: number           // 关联ID
    extra_data?: Record<string, unknown>  // 额外数据
    is_read: boolean
    read_at?: string              // 阅读时间
    is_pushed: boolean            // 是否已推送
    pushed_at?: string            // 推送时间
    expires_at?: string           // 过期时间
    created_at: string
}

// 对齐 api.notificationPreferencesResponse
export interface NotificationPreferencesResponse {
    user_id: number
    enable_order_notifications: boolean
    enable_payment_notifications: boolean
    enable_delivery_notifications: boolean
    enable_system_notifications: boolean
    do_not_disturb_start?: string  // 免打扰开始时间
    do_not_disturb_end?: string    // 免打扰结束时间
    created_at: string
    updated_at: string
}

export interface UpdateNotificationPreferencesRequest extends Record<string, unknown> {
    enable_order_notifications?: boolean
    enable_payment_notifications?: boolean
    enable_delivery_notifications?: boolean
    enable_system_notifications?: boolean
    do_not_disturb_start?: string
    do_not_disturb_end?: string
}

/** 通知列表响应 - 对齐 api.listNotificationsResponse */
export interface ListNotificationsResponse {
    notifications: NotificationResponse[]
    total_count: number                          // 总数
}

export interface UnreadCountResponse {
    count: number
}

// 申诉相关类型 - 对齐 api.claimResponse
export interface ClaimResponse {
    id: number
    user_id: number
    order_id?: number
    claim_type: string            // 申诉类型
    claim_amount: number          // 申诉金额（分）
    description: string
    evidence_urls: string[]       // 证据图片URL
    status: string                // 状态
    approval_type?: string        // 审批类型
    approved_amount?: number      // 批准金额（分）
    is_malicious?: boolean        // 是否恶意申诉
    review_notes?: string         // 审核备注
    reviewed_at?: string          // 审核时间
    reviewer_id?: number          // 审核人ID
    created_at: string
}

export interface CreateClaimRequest extends Record<string, unknown> {
    order_id?: number
    claim_type: string            // 申诉类型
    claim_amount: number          // 申诉金额（分）
    description: string
    evidence_urls?: string[]      // 证据图片URL
}

export interface ListClaimsResponse {
    claims: ClaimResponse[]
    total: number
    page: number
    page_size: number
}

// 优惠券相关类型 - 对齐 api.voucherResponse
export interface VoucherResponse {
    id: number
    merchant_id: number
    name: string
    description?: string
    code: string                  // 优惠券码
    amount: number                // 优惠金额（分）
    min_order_amount: number      // 最低订单金额（分）
    allowed_order_types: string[] // 允许的订单类型
    total_quantity: number        // 总数量
    claimed_quantity: number      // 已领取数量
    used_quantity: number         // 已使用数量
    valid_from: string            // 有效期开始
    valid_until: string           // 有效期结束
    is_active: boolean            // 是否激活
    created_at: string
}

// 用户优惠券响应 - 对齐 api.userVoucherResponse
export interface UserVoucherResponse {
    id: number
    user_id: number
    voucher_id: number
    merchant_id: number
    merchant_name: string
    name: string
    code: string
    amount: number                // 优惠金额（分）
    min_order_amount: number      // 最低订单金额（分）
    status: string                // 状态
    obtained_at: string           // 领取时间
    expires_at: string            // 过期时间
    used_at?: string              // 使用时间
    order_id?: number             // 使用的订单ID
}

export interface ListMyVouchersResponse {
    vouchers: UserVoucherResponse[]  // 用户优惠券
    total: number
}

export interface ListAvailableVouchersResponse {
    vouchers: VoucherResponse[]      // 可领取的优惠券
    total: number
}

/** 收藏菜品行 - 对齐 api.favoriteDishRow */
export interface FavoriteDishRow {
    dish_id: number                              // 菜品ID
    dish_name: string                            // 菜品名称
    order_count: number                          // 订单数
    total_quantity: number                       // 总数量
}

/** 添加收藏菜品请求 - 对齐 api.addFavoriteDishRequest */
export interface AddFavoriteDishRequest extends Record<string, unknown> {
    dish_id: number                              // 菜品ID（必填）
}

/** 添加收藏商户请求 - 对齐 api.addFavoriteMerchantRequest */
export interface AddFavoriteMerchantRequest extends Record<string, unknown> {
    merchant_id: number                          // 商户ID（必填）
}

/** 标记全部已读响应 - 对齐 api.markAllAsReadResponse */
export interface MarkAllAsReadResponse {
    success: boolean                             // 是否成功
}

/** 顾客详情响应 - 对齐 api.customerDetailResponse */
export interface CustomerDetailResponse {
    user_id: number                              // 用户ID
    full_name?: string                           // 姓名
    phone?: string                               // 电话
    avatar_url?: string                          // 头像URL
    total_orders: number                         // 总订单数
    total_amount: number                         // 总消费金额（分）
    avg_order_amount: number                     // 平均订单金额（分）
    first_order_at?: string                      // 首次下单时间
    last_order_at?: string                       // 最后下单时间
    favorite_dishes?: FavoriteDishRow[]          // 常点菜品
}

/** 加入会员请求 - 对齐 api.joinMembershipRequest */
export interface JoinMembershipRequest extends Record<string, unknown> {
    merchant_id: number                          // 商户ID（必填）
}

/** 交易记录响应 - 对齐 api.transactionResponse */
export interface TransactionResponse {
    id: number                                   // 交易ID
    membership_id: number                        // 会员ID
    type: string                                 // 交易类型
    amount: number                               // 金额（分）
    balance_after: number                        // 交易后余额（分）
    related_order_id?: number                    // 关联订单ID
    notes?: string                               // 备注
    created_at: string                           // 创建时间
}

// 查询参数类型
export interface PaginationParams extends Record<string, unknown> {
    page?: number
    page_size?: number
}

export interface FavoritesParams extends PaginationParams {
    // 可以添加筛选参数
}

export interface HistoryParams extends PaginationParams {
    target_type?: 'dish' | 'merchant'  // 对齐swagger字段名
}

export interface ReviewsParams extends PaginationParams {
    merchant_id?: number
}

export interface NotificationsParams extends PaginationParams {
    type?: string
    is_read?: boolean
}

export interface ClaimsParams extends PaginationParams {
    status?: 'pending' | 'processing' | 'resolved' | 'rejected'
}

export interface VouchersParams extends PaginationParams {
    status?: 'available' | 'used' | 'expired'
    merchant_id?: number
}

// ==================== 收藏功能接口 ====================

/**
 * 获取收藏的菜品列表
 */
export function getFavoriteDishes(params: FavoritesParams = {}): Promise<ListFavoriteDishesResponse> {
    return request({
        url: '/v1/favorites/dishes',
        method: 'GET',
        data: params
    })
}

/**
 * 添加菜品到收藏
 */
export function addDishToFavorites(dishId: number): Promise<void> {
    return request({
        url: '/v1/favorites/dishes',
        method: 'POST',
        data: { dish_id: dishId }
    })
}

/**
 * 从收藏中移除菜品
 */
export function removeDishFromFavorites(dishId: number): Promise<void> {
    return request({
        url: `/v1/favorites/dishes/${dishId}`,
        method: 'DELETE'
    })
}

/**
 * 获取收藏的商户列表
 */
export function getFavoriteMerchants(params: FavoritesParams = {}): Promise<ListFavoriteMerchantsResponse> {
    return request({
        url: '/v1/favorites/merchants',
        method: 'GET',
        data: params
    })
}

/**
 * 添加商户到收藏
 */
export function addMerchantToFavorites(merchantId: number): Promise<void> {
    return request({
        url: '/v1/favorites/merchants',
        method: 'POST',
        data: { merchant_id: merchantId }
    })
}

/**
 * 从收藏中移除商户
 */
export function removeMerchantFromFavorites(merchantId: number): Promise<void> {
    return request({
        url: `/v1/favorites/merchants/${merchantId}`,
        method: 'DELETE'
    })
}

// ==================== 浏览历史接口 ====================

/**
 * 获取浏览历史
 */
export function getBrowseHistory(params: HistoryParams = {}): Promise<ListBrowseHistoryResponse> {
    return request({
        url: '/v1/history/browse',
        method: 'GET',
        data: params
    })
}

/**
 * 清空浏览历史
 */
export function clearBrowseHistory(): Promise<void> {
    return request({
        url: '/v1/history/browse',
        method: 'DELETE'
    })
}

// ==================== 评价系统接口 ====================

/**
 * 创建评价
 */
export function createReview(data: CreateReviewRequest): Promise<ReviewResponse> {
    return request({
        url: '/v1/reviews',
        method: 'POST',
        data
    })
}

/**
 * 获取我的评价列表
 */
export function getMyReviews(params: ReviewsParams = {}): Promise<ListReviewsResponse> {
    return request({
        url: '/v1/reviews/me',
        method: 'GET',
        data: params
    })
}

/**
 * 获取评价详情
 */
export function getReviewDetail(reviewId: number): Promise<ReviewResponse> {
    return request({
        url: `/v1/reviews/${reviewId}`,
        method: 'GET'
    })
}

/**
 * 获取商户的评价列表
 */
export function getMerchantReviews(merchantId: number, params: PaginationParams = {}): Promise<ListReviewsResponse> {
    return request({
        url: `/v1/reviews/merchants/${merchantId}`,
        method: 'GET',
        data: params
    })
}

/**
 * 获取商户的所有评价（包括历史）
 */
export function getAllMerchantReviews(merchantId: number, params: PaginationParams = {}): Promise<ListReviewsResponse> {
    return request({
        url: `/v1/reviews/merchants/${merchantId}/all`,
        method: 'GET',
        data: params
    })
}

/**
 * 商户回复评价
 */
export function replyToReview(reviewId: number, data: ReplyReviewRequest): Promise<ReviewResponse> {
    return request({
        url: `/v1/reviews/${reviewId}/reply`,
        method: 'POST',
        data
    })
}

// ==================== 会员系统接口 ====================

/**
 * 获取我的会员卡列表
 */
export function getMyMemberships(): Promise<ListMembershipsResponse> {
    return request({
        url: '/v1/memberships',
        method: 'GET'
    })
}

/**
 * 获取会员卡详情
 */
export function getMembershipDetail(membershipId: number): Promise<MembershipResponse> {
    return request({
        url: `/v1/memberships/${membershipId}`,
        method: 'GET'
    })
}

/**
 * 会员卡充值
 */
export function rechargeMembership(data: RechargeRequest): Promise<any> {
    return request({
        url: '/v1/memberships/recharge',
        method: 'POST',
        data
    })
}

/**
 * 获取会员卡交易记录
 */
export function getMembershipTransactions(
    membershipId: number,
    params: PaginationParams = {}
): Promise<ListMembershipTransactionsResponse> {
    return request({
        url: `/v1/memberships/${membershipId}/transactions`,
        method: 'GET',
        data: params
    })
}

// ==================== 通知系统接口 ====================

/**
 * 获取通知列表
 */
export function getNotifications(params: NotificationsParams = {}): Promise<ListNotificationsResponse> {
    return request({
        url: '/v1/notifications',
        method: 'GET',
        data: params
    })
}

/**
 * 获取未读通知数量
 */
export function getUnreadNotificationCount(): Promise<UnreadCountResponse> {
    return request({
        url: '/v1/notifications/unread/count',
        method: 'GET'
    })
}

/**
 * 标记通知为已读
 */
export function markNotificationAsRead(notificationId: number): Promise<void> {
    return request({
        url: `/v1/notifications/${notificationId}/read`,
        method: 'PUT'
    })
}

/**
 * 标记所有通知为已读
 */
export function markAllNotificationsAsRead(): Promise<void> {
    return request({
        url: '/v1/notifications/read-all',
        method: 'PUT'
    })
}

/**
 * 删除通知
 */
export function deleteNotification(notificationId: number): Promise<void> {
    return request({
        url: `/v1/notifications/${notificationId}`,
        method: 'DELETE'
    })
}

/**
 * 获取通知偏好设置
 */
export function getNotificationPreferences(): Promise<NotificationPreferencesResponse> {
    return request({
        url: '/v1/notifications/preferences',
        method: 'GET'
    })
}

/**
 * 更新通知偏好设置
 */
export function updateNotificationPreferences(data: UpdateNotificationPreferencesRequest): Promise<NotificationPreferencesResponse> {
    return request({
        url: '/v1/notifications/preferences',
        method: 'PUT',
        data
    })
}

// ==================== 申诉功能接口 ====================

/**
 * 创建申诉
 */
export function createClaim(data: CreateClaimRequest): Promise<ClaimResponse> {
    return request({
        url: '/v1/claims',
        method: 'POST',
        data
    })
}

/**
 * 获取我的申诉列表
 */
export function getMyClaims(params: ClaimsParams = {}): Promise<ListClaimsResponse> {
    return request({
        url: '/v1/claims',
        method: 'GET',
        data: params
    })
}

/**
 * 获取申诉详情
 */
export function getClaimDetail(claimId: number): Promise<ClaimResponse> {
    return request({
        url: `/v1/claims/${claimId}`,
        method: 'GET'
    })
}

// ==================== 优惠券接口 ====================

/**
 * 获取我的优惠券列表
 */
export function getMyVouchers(params: VouchersParams = {}): Promise<ListMyVouchersResponse> {
    return request({
        url: '/v1/vouchers/me',
        method: 'GET',
        data: params
    })
}

/**
 * 获取我的可用优惠券
 */
export function getMyAvailableVouchers(): Promise<ListAvailableVouchersResponse> {
    return request({
        url: '/v1/vouchers/me/available',
        method: 'GET'
    })
}

/**
 * 获取商户可用优惠券
 */
export function getAvailableVouchersForMerchant(merchantId: number): Promise<ListAvailableVouchersResponse> {
    return request({
        url: `/v1/vouchers/available/${merchantId}`,
        method: 'GET'
    })
}

/**
 * 领取优惠券
 */
export function claimVoucher(voucherId: number): Promise<void> {
    return request({
        url: `/v1/vouchers/${voucherId}/claim`,
        method: 'POST'
    })
}

// ==================== 便捷方法 ====================

/**
 * 检查菜品是否已收藏
 */
export async function isDishFavorited(dishId: number): Promise<boolean> {
    try {
        const response = await getFavoriteDishes({ page: 1, page_size: 1000 })
        return response.dishes.some(dish => dish.dish_id === dishId)
    } catch (error) {
        console.error('检查菜品收藏状态失败:', error)
        return false
    }
}

/**
 * 检查商户是否已收藏
 */
export async function isMerchantFavorited(merchantId: number): Promise<boolean> {
    try {
        const response = await getFavoriteMerchants({ page: 1, page_size: 1000 })
        return response.merchants.some(merchant => merchant.merchant_id === merchantId)
    } catch (error) {
        console.error('检查商户收藏状态失败:', error)
        return false
    }
}

/**
 * 切换菜品收藏状态
 */
export async function toggleDishFavorite(dishId: number): Promise<boolean> {
    const isFavorited = await isDishFavorited(dishId)
    if (isFavorited) {
        await removeDishFromFavorites(dishId)
        return false
    } else {
        await addDishToFavorites(dishId)
        return true
    }
}

/**
 * 切换商户收藏状态
 */
export async function toggleMerchantFavorite(merchantId: number): Promise<boolean> {
    const isFavorited = await isMerchantFavorited(merchantId)
    if (isFavorited) {
        await removeMerchantFromFavorites(merchantId)
        return false
    } else {
        await addMerchantToFavorites(merchantId)
        return true
    }
}

/**
 * 获取个人中心概览数据
 */
export async function getPersonalCenterOverview() {
    try {
        const [
            favoriteDishes,
            favoriteMerchants,
            unreadCount,
            availableVouchers,
            memberships
        ] = await Promise.all([
            getFavoriteDishes({ page: 1, page_size: 1 }),
            getFavoriteMerchants({ page: 1, page_size: 1 }),
            getUnreadNotificationCount(),
            getMyAvailableVouchers(),
            getMyMemberships()
        ])

        return {
            favoriteCount: {
                dishes: favoriteDishes.total,
                merchants: favoriteMerchants.total
            },
            unreadNotifications: unreadCount.count,
            availableVouchers: availableVouchers.total,
            membershipCount: memberships.total
        }
    } catch (error) {
        console.error('获取个人中心概览失败:', error)
        return {
            favoriteCount: { dishes: 0, merchants: 0 },
            unreadNotifications: 0,
            availableVouchers: 0,
            membershipCount: 0
        }
    }
}

// 兼容性导出
export default {
    // 收藏功能
    getFavoriteDishes,
    addDishToFavorites,
    removeDishFromFavorites,
    getFavoriteMerchants,
    addMerchantToFavorites,
    removeMerchantFromFavorites,

    // 浏览历史
    getBrowseHistory,
    clearBrowseHistory,

    // 评价系统
    createReview,
    getMyReviews,
    getReviewDetail,
    getMerchantReviews,
    getAllMerchantReviews,
    replyToReview,

    // 会员系统
    getMyMemberships,
    getMembershipDetail,
    rechargeMembership,
    getMembershipTransactions,

    // 通知系统
    getNotifications,
    getUnreadNotificationCount,
    markNotificationAsRead,
    markAllNotificationsAsRead,
    deleteNotification,
    getNotificationPreferences,
    updateNotificationPreferences,

    // 申诉功能
    createClaim,
    getMyClaims,
    getClaimDetail,

    // 优惠券
    getMyVouchers,
    getMyAvailableVouchers,
    getAvailableVouchersForMerchant,
    claimVoucher,

    // 便捷方法
    isDishFavorited,
    isMerchantFavorited,
    toggleDishFavorite,
    toggleMerchantFavorite,
    getPersonalCenterOverview
}