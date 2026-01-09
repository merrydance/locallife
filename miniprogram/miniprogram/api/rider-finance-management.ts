/**
 * 骑手财务管理接口重构 (Task 3.4)
 * 基于swagger.json完全重构，移除所有没有后端支持的旧功能
 * 包含：保证金管理、提现功能、积分历史
 */

import { request } from '../utils/request'

// ==================== 数据类型定义 ====================

/** 保证金操作类型枚举 */
export type DepositType = 'recharge' | 'withdraw' | 'freeze' | 'unfreeze' | 'deduct' | 'refund'

// ==================== 保证金管理相关类型 ====================

/** 保证金余额响应 - 基于swagger api.depositBalanceResponse */
export interface DepositBalanceResponse {
    total_deposit: number
    available_deposit: number
    frozen_deposit: number
}

/** 保证金记录响应 - 基于swagger api.depositResponse */
export interface DepositResponse {
    id: number
    rider_id: number
    type: DepositType
    amount: number
    balance_after: number
    remark?: string
    created_at: string
}

/** 保证金充值请求 */
export interface DepositRechargeRequest extends Record<string, unknown> {
    amount: number
    remark?: string
}

/** 提现请求 - 基于swagger api.withdrawRequest */
export interface WithdrawRequest extends Record<string, unknown> {
    amount: number
    remark?: string
}

/** 保证金流水查询参数 */
export interface DepositHistoryParams extends Record<string, unknown> {
    page: number
    limit: number
    type?: DepositType
    start_date?: string
    end_date?: string
}

// ==================== 积分历史相关类型 ====================

/** 积分历史记录 */
export interface ScoreHistoryItem {
    id: number
    rider_id: number
    order_id?: number
    score_change: number
    reason: string
    description?: string
    created_at: string
}

/** 积分历史查询参数 */
export interface ScoreHistoryParams extends Record<string, unknown> {
    page_id: number
    page_size: number
    start_date?: string
    end_date?: string
}

// ==================== 财务统计相关类型 ====================

/** 收入统计 */
export interface EarningsStats {
    today_earnings: number
    week_earnings: number
    month_earnings: number
    total_earnings: number
    avg_daily_earnings: number
    completed_orders: number
    avg_earnings_per_order: number
}

/** 保证金统计 */
export interface DepositStats {
    total_recharge: number
    total_withdraw: number
    total_deduct: number
    net_deposit: number
    recharge_count: number
    withdraw_count: number
    avg_recharge_amount: number
}

// ==================== 骑手财务管理服务类 ====================

/**
 * 骑手财务管理服务
 * 提供保证金管理、提现、财务统计等功能
 */
export class RiderFinanceManagementService {
    /**
     * 获取保证金余额
     */
    async getDepositBalance(): Promise<DepositBalanceResponse> {
        return request({
            url: '/v1/rider/deposit',
            method: 'GET'
        })
    }

    /**
     * 保证金充值
     * @param rechargeData 充值数据
     */
    async rechargeDeposit(rechargeData: DepositRechargeRequest): Promise<any> {
        return request({
            url: '/v1/rider/deposit',
            method: 'POST',
            data: rechargeData
        })
    }

    /**
     * 获取保证金流水记录
     * @param params 查询参数
     */
    async getDepositHistory(params: DepositHistoryParams): Promise<{
        deposits: DepositResponse[]
        total: number
        page: number
        limit: number
        has_more: boolean
    }> {
        const deposits = await request({
            url: '/v1/rider/deposits',
            method: 'GET',
            data: params
        })

        // 由于swagger中返回的是数组，这里需要适配成分页格式
        return {
            deposits: deposits || [],
            total: deposits?.length || 0,
            page: params.page,
            limit: params.limit,
            has_more: (deposits?.length || 0) >= params.limit
        }
    }

    /**
     * 申请提现
     * @param withdrawData 提现数据
     */
    async withdrawDeposit(withdrawData: WithdrawRequest): Promise<DepositResponse> {
        return request({
            url: '/v1/rider/withdraw',
            method: 'POST',
            data: withdrawData
        })
    }

    /**
     * 获取积分历史记录
     * @param params 查询参数
     */
    async getScoreHistory(params: ScoreHistoryParams): Promise<{
        history: ScoreHistoryItem[]
        total: number
        page_id: number
        page_size: number
        has_more: boolean
    }> {
        return request({
            url: '/v1/rider/score/history',
            method: 'GET',
            data: params
        })
    }
}

// ==================== 财务统计服务类 ====================

/**
 * 财务统计服务
 * 提供收入分析、保证金统计等功能
 */
export class FinanceStatsService {
    /**
     * 计算收入统计
     * @param deposits 保证金记录
     * @param deliveries 配送记录（需要从其他服务获取）
     */
    calculateEarningsStats(deposits: DepositResponse[], deliveries: any[] = []): EarningsStats {
        const now = new Date()
        const today = new Date(now.getFullYear(), now.getMonth(), now.getDate())
        const weekStart = new Date(today.getTime() - 7 * 24 * 60 * 60 * 1000)
        const monthStart = new Date(now.getFullYear(), now.getMonth(), 1)

        // 从保证金记录中筛选收入相关的记录
        const earningsDeposits = deposits.filter(d => d.type === 'recharge' && d.amount > 0)

        const todayEarnings = earningsDeposits
            .filter(d => new Date(d.created_at) >= today)
            .reduce((sum, d) => sum + d.amount, 0)

        const weekEarnings = earningsDeposits
            .filter(d => new Date(d.created_at) >= weekStart)
            .reduce((sum, d) => sum + d.amount, 0)

        const monthEarnings = earningsDeposits
            .filter(d => new Date(d.created_at) >= monthStart)
            .reduce((sum, d) => sum + d.amount, 0)

        const totalEarnings = earningsDeposits.reduce((sum, d) => sum + d.amount, 0)

        // 计算平均值
        const daysInMonth = now.getDate()
        const avgDailyEarnings = daysInMonth > 0 ? monthEarnings / daysInMonth : 0

        const completedOrders = deliveries.length
        const avgEarningsPerOrder = completedOrders > 0 ? totalEarnings / completedOrders : 0

        return {
            today_earnings: todayEarnings,
            week_earnings: weekEarnings,
            month_earnings: monthEarnings,
            total_earnings: totalEarnings,
            avg_daily_earnings: avgDailyEarnings,
            completed_orders: completedOrders,
            avg_earnings_per_order: avgEarningsPerOrder
        }
    }

    /**
     * 计算保证金统计
     * @param deposits 保证金记录
     */
    calculateDepositStats(deposits: DepositResponse[]): DepositStats {
        const rechargeRecords = deposits.filter(d => d.type === 'recharge')
        const withdrawRecords = deposits.filter(d => d.type === 'withdraw')
        const deductRecords = deposits.filter(d => d.type === 'deduct')

        const totalRecharge = rechargeRecords.reduce((sum, d) => sum + d.amount, 0)
        const totalWithdraw = withdrawRecords.reduce((sum, d) => sum + Math.abs(d.amount), 0)
        const totalDeduct = deductRecords.reduce((sum, d) => sum + Math.abs(d.amount), 0)

        const netDeposit = totalRecharge - totalWithdraw - totalDeduct
        const avgRechargeAmount = rechargeRecords.length > 0 ? totalRecharge / rechargeRecords.length : 0

        return {
            total_recharge: totalRecharge,
            total_withdraw: totalWithdraw,
            total_deduct: totalDeduct,
            net_deposit: netDeposit,
            recharge_count: rechargeRecords.length,
            withdraw_count: withdrawRecords.length,
            avg_recharge_amount: avgRechargeAmount
        }
    }
}

// ==================== 数据适配器 ====================

/**
 * 骑手财务管理数据适配器
 * 处理前端数据格式与后端API数据格式的转换
 */
export class RiderFinanceManagementAdapter {
    /**
     * 适配保证金余额响应数据
     */
    static adaptDepositBalanceResponse(data: DepositBalanceResponse): {
        totalDeposit: number
        availableDeposit: number
        frozenDeposit: number
    } {
        return {
            totalDeposit: data.total_deposit,
            availableDeposit: data.available_deposit,
            frozenDeposit: data.frozen_deposit
        }
    }

    /**
     * 适配保证金记录响应数据
     */
    static adaptDepositResponse(data: DepositResponse): {
        id: number
        riderId: number
        type: DepositType
        amount: number
        balanceAfter: number
        remark?: string
        createdAt: string
    } {
        return {
            id: data.id,
            riderId: data.rider_id,
            type: data.type,
            amount: data.amount,
            balanceAfter: data.balance_after,
            remark: data.remark,
            createdAt: data.created_at
        }
    }

    /**
     * 适配充值请求数据
     */
    static adaptDepositRechargeRequest(data: {
        amount: number
        remark?: string
    }): DepositRechargeRequest {
        return {
            amount: data.amount,
            remark: data.remark
        }
    }

    /**
     * 适配提现请求数据
     */
    static adaptWithdrawRequest(data: {
        amount: number
        remark?: string
    }): WithdrawRequest {
        return {
            amount: data.amount,
            remark: data.remark
        }
    }

    /**
     * 适配积分历史记录
     */
    static adaptScoreHistoryItem(data: ScoreHistoryItem): {
        id: number
        riderId: number
        orderId?: number
        scoreChange: number
        reason: string
        description?: string
        createdAt: string
    } {
        return {
            id: data.id,
            riderId: data.rider_id,
            orderId: data.order_id,
            scoreChange: data.score_change,
            reason: data.reason,
            description: data.description,
            createdAt: data.created_at
        }
    }
}

// ==================== 导出服务实例 ====================

export const riderFinanceManagementService = new RiderFinanceManagementService()
export const financeStatsService = new FinanceStatsService()

// ==================== 便捷函数 ====================

/**
 * 获取骑手财务工作台数据
 */
export async function getRiderFinanceDashboard(): Promise<{
    depositBalance: DepositBalanceResponse
    recentDeposits: DepositResponse[]
    earningsStats: EarningsStats
    depositStats: DepositStats
    canWithdraw: boolean
    withdrawLimits: {
        minAmount: number
        maxAmount: number
        dailyLimit: number
    }
}> {
    const [depositBalance, depositHistory] = await Promise.all([
        riderFinanceManagementService.getDepositBalance(),
        riderFinanceManagementService.getDepositHistory({ page: 1, limit: 10 })
    ])

    const earningsStats = financeStatsService.calculateEarningsStats(depositHistory.deposits)
    const depositStats = financeStatsService.calculateDepositStats(depositHistory.deposits)

    // 检查是否可以提现
    const canWithdraw = depositBalance.available_deposit >= 100 // 最小提现金额1元

    return {
        depositBalance,
        recentDeposits: depositHistory.deposits,
        earningsStats,
        depositStats,
        canWithdraw,
        withdrawLimits: {
            minAmount: 100, // 1元
            maxAmount: 5000000, // 50000元
            dailyLimit: 1000000 // 10000元（需要根据实际业务调整）
        }
    }
}

/**
 * 智能提现建议
 * @param depositBalance 保证金余额
 * @param activeDeliveries 活跃配送数量
 * @param recentEarnings 近期收入
 */
export function getWithdrawSuggestion(
    depositBalance: DepositBalanceResponse,
    activeDeliveries: number = 0,
    recentEarnings: number = 0
): {
    canWithdraw: boolean
    suggestedAmount: number
    reason: string
    warnings: string[]
} {
    const warnings: string[] = []
    let canWithdraw = true
    let reason = ''
    let suggestedAmount = 0

    // 检查是否有活跃配送
    if (activeDeliveries > 0) {
        canWithdraw = false
        reason = '有进行中的配送订单，无法提现'
        return { canWithdraw, suggestedAmount, reason, warnings }
    }

    // 检查最小提现金额
    if (depositBalance.available_deposit < 100) {
        canWithdraw = false
        reason = '可用余额不足1元，无法提现'
        return { canWithdraw, suggestedAmount, reason, warnings }
    }

    // 建议保留一定的保证金用于接单
    const recommendedReserve = Math.max(50000, recentEarnings * 0.1) // 保留500元或近期收入的10%
    const availableForWithdraw = depositBalance.available_deposit - recommendedReserve

    if (availableForWithdraw <= 0) {
        warnings.push('建议保留一定保证金用于正常接单')
        suggestedAmount = Math.max(100, depositBalance.available_deposit * 0.5)
    } else {
        suggestedAmount = Math.min(availableForWithdraw, 1000000) // 最大单次提现10000元
    }

    reason = '可以提现'

    // 添加其他警告
    if (depositBalance.frozen_deposit > 0) {
        warnings.push(`有${formatAmount(depositBalance.frozen_deposit)}保证金被冻结`)
    }

    return { canWithdraw, suggestedAmount, reason, warnings }
}

/**
 * 保证金安全检查
 * @param depositBalance 保证金余额
 * @param recentDeposits 近期保证金记录
 */
export function checkDepositSecurity(
    depositBalance: DepositBalanceResponse,
    recentDeposits: DepositResponse[]
): {
    securityLevel: 'safe' | 'warning' | 'danger'
    issues: string[]
    suggestions: string[]
} {
    const issues: string[] = []
    const suggestions: string[] = []
    let securityLevel: 'safe' | 'warning' | 'danger' = 'safe'

    // 检查保证金余额
    if (depositBalance.available_deposit < 10000) { // 少于100元
        issues.push('可用保证金余额过低')
        suggestions.push('建议及时充值保证金以确保正常接单')
        securityLevel = 'warning'
    }

    // 检查冻结保证金比例
    const frozenRatio = depositBalance.total_deposit > 0
        ? (depositBalance.frozen_deposit / depositBalance.total_deposit) * 100
        : 0

    if (frozenRatio > 50) {
        issues.push('冻结保证金比例过高')
        suggestions.push('请检查是否有未完成的配送订单或违规行为')
        securityLevel = 'danger'
    }

    // 检查近期异常扣款
    const recentDeducts = recentDeposits
        .filter(d => d.type === 'deduct' && new Date(d.created_at) > new Date(Date.now() - 7 * 24 * 60 * 60 * 1000))

    if (recentDeducts.length > 3) {
        issues.push('近期扣款频繁')
        suggestions.push('请注意配送服务质量，避免违规操作')
        securityLevel = securityLevel === 'danger' ? 'danger' : 'warning'
    }

    return { securityLevel, issues, suggestions }
}

/**
 * 收入趋势分析
 * @param deposits 保证金记录
 * @param days 分析天数
 */
export function analyzeEarningsTrend(deposits: DepositResponse[], days: number = 30): {
    trend: 'up' | 'down' | 'stable'
    growthRate: number
    dailyAverage: number
    bestDay: { date: string; amount: number } | null
    worstDay: { date: string; amount: number } | null
} {
    const earningsDeposits = deposits.filter(d => d.type === 'recharge')

    if (earningsDeposits.length === 0) {
        return {
            trend: 'stable',
            growthRate: 0,
            dailyAverage: 0,
            bestDay: null,
            worstDay: null
        }
    }

    // 按日期分组计算每日收入
    const dailyEarnings = new Map<string, number>()
    earningsDeposits.forEach(deposit => {
        const date = deposit.created_at.split('T')[0]
        dailyEarnings.set(date, (dailyEarnings.get(date) || 0) + deposit.amount)
    })

    const sortedDays = Array.from(dailyEarnings.entries()).sort((a, b) => a[0].localeCompare(b[0]))

    if (sortedDays.length < 2) {
        return {
            trend: 'stable',
            growthRate: 0,
            dailyAverage: sortedDays[0]?.[1] || 0,
            bestDay: sortedDays[0] ? { date: sortedDays[0][0], amount: sortedDays[0][1] } : null,
            worstDay: sortedDays[0] ? { date: sortedDays[0][0], amount: sortedDays[0][1] } : null
        }
    }

    // 计算趋势
    const firstWeek = sortedDays.slice(0, Math.min(7, sortedDays.length))
    const lastWeek = sortedDays.slice(-Math.min(7, sortedDays.length))

    const firstWeekAvg = firstWeek.reduce((sum, [, amount]) => sum + amount, 0) / firstWeek.length
    const lastWeekAvg = lastWeek.reduce((sum, [, amount]) => sum + amount, 0) / lastWeek.length

    const growthRate = firstWeekAvg > 0 ? ((lastWeekAvg - firstWeekAvg) / firstWeekAvg) * 100 : 0

    let trend: 'up' | 'down' | 'stable' = 'stable'
    if (growthRate > 10) trend = 'up'
    else if (growthRate < -10) trend = 'down'

    // 计算平均值
    const totalEarnings = sortedDays.reduce((sum, [, amount]) => sum + amount, 0)
    const dailyAverage = totalEarnings / sortedDays.length

    // 找出最好和最差的一天
    const bestDay = sortedDays.reduce((max, current) => current[1] > max[1] ? current : max)
    const worstDay = sortedDays.reduce((min, current) => current[1] < min[1] ? current : min)

    return {
        trend,
        growthRate,
        dailyAverage,
        bestDay: { date: bestDay[0], amount: bestDay[1] },
        worstDay: { date: worstDay[0], amount: worstDay[1] }
    }
}

/**
 * 格式化保证金操作类型显示
 * @param type 操作类型
 */
export function formatDepositType(type: DepositType): string {
    const typeMap: Record<DepositType, string> = {
        recharge: '充值',
        withdraw: '提现',
        freeze: '冻结',
        unfreeze: '解冻',
        deduct: '扣款',
        refund: '退款'
    }
    return typeMap[type] || type
}

/**
 * 格式化金额显示
 * @param amount 金额（分）
 * @param showUnit 是否显示单位
 */
export function formatAmount(amount: number, showUnit: boolean = true): string {
    const yuan = (amount / 100).toFixed(2)
    return showUnit ? `¥${yuan}` : yuan
}

/**
 * 验证提现金额
 * @param amount 提现金额（分）
 * @param availableDeposit 可用保证金
 */
export function validateWithdrawAmount(amount: number, availableDeposit: number): { valid: boolean; message?: string } {
    if (amount < 100) {
        return { valid: false, message: '提现金额不能少于1元' }
    }

    if (amount > 5000000) {
        return { valid: false, message: '单次提现金额不能超过50000元' }
    }

    if (amount > availableDeposit) {
        return { valid: false, message: '提现金额不能超过可用余额' }
    }

    return { valid: true }
}

/**
 * 验证充值金额
 * @param amount 充值金额（分）
 */
export function validateRechargeAmount(amount: number): { valid: boolean; message?: string } {
    if (amount < 100) {
        return { valid: false, message: '充值金额不能少于1元' }
    }

    if (amount > 1000000) {
        return { valid: false, message: '单次充值金额不能超过10000元' }
    }

    return { valid: true }
}

/**
 * 计算提现手续费
 * @param amount 提现金额（分）
 */
export function calculateWithdrawFee(amount: number): {
    fee: number
    actualAmount: number
    feeRate: number
} {
    // 这里需要根据实际的手续费规则调整
    const feeRate = 0.001 // 0.1%
    const fee = Math.max(100, amount * feeRate) // 最低1元手续费
    const actualAmount = amount - fee

    return {
        fee,
        actualAmount,
        feeRate: feeRate * 100
    }
}